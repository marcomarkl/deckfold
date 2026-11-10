package pptxgen

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"

	"deckfold/internal/pptx/kernprobe"
)

// Every XML part of every fixture must be well-formed, every relationship target inside the
// package must exist, and the output must be byte-identical between runs.
func TestFixturesWellFormed(t *testing.T) {
	decks := Fixtures()
	decks["big"] = Big(3)
	for name, d := range decks {
		b, err := d.Bytes()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		b2, _ := d.Bytes()
		if !bytes.Equal(b, b2) {
			t.Errorf("%s: output differs between runs", name)
		}
		z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		parts := map[string]bool{}
		for _, f := range z.File {
			parts[f.Name] = true
		}
		for _, f := range z.File {
			if !strings.HasSuffix(f.Name, ".xml") && !strings.HasSuffix(f.Name, ".rels") {
				continue
			}
			rc, _ := f.Open()
			data, _ := io.ReadAll(rc)
			rc.Close()
			dec := xml.NewDecoder(bytes.NewReader(data))
			for {
				_, err := dec.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("%s: %s: %v", name, f.Name, err)
				}
			}
			if strings.HasSuffix(f.Name, ".rels") {
				checkRels(t, name, f.Name, data, parts)
			}
		}
	}
}

func checkRels(t *testing.T, deck, relsName string, data []byte, parts map[string]bool) {
	var rels struct {
		Rel []struct {
			Target string `xml:"Target,attr"`
			Mode   string `xml:"TargetMode,attr"`
		} `xml:"Relationship"`
	}
	if err := xml.Unmarshal(data, &rels); err != nil {
		t.Fatalf("%s: %s: %v", deck, relsName, err)
	}
	// source folder: "ppt/slides/_rels/slide1.xml.rels" -> "ppt/slides"
	dir := relsName[:strings.LastIndex(relsName, "_rels/")]
	dir = strings.TrimSuffix(dir, "/")
	for _, r := range rels.Rel {
		if r.Mode == "External" {
			continue
		}
		p := r.Target
		if !strings.HasPrefix(p, "/") {
			p = join(dir, p)
		}
		p = strings.TrimPrefix(p, "/")
		if !parts[p] {
			t.Errorf("%s: %s: target %s (%s) missing", deck, relsName, r.Target, p)
		}
	}
}

func join(dir, rel string) string {
	segs := []string{}
	if dir != "" {
		segs = strings.Split(dir, "/")
	}
	for _, s := range strings.Split(rel, "/") {
		switch s {
		case "..":
			if len(segs) > 0 {
				segs = segs[:len(segs)-1]
			}
		case ".", "":
		default:
			segs = append(segs, s)
		}
	}
	return strings.Join(segs, "/")
}

// No element may appear twice where the schema allows it once (a:pPr, a:rPr): PowerPoint would
// repair such a file.
func TestNoDuplicateProps(t *testing.T) {
	for name, d := range Fixtures() {
		for i, s := range d.Slides {
			for _, sh := range s.Shapes {
				if strings.Contains(sh, "</a:pPr><a:pPr") || strings.Contains(sh, "</a:rPr><a:rPr") {
					t.Errorf("%s slide %d: duplicated a:pPr or a:rPr", name, i+1)
				}
			}
		}
	}
}

// TestKerningWords checks the words of the kerning deck against the bundled fonts, so that the
// deck keeps measuring something: every kerned word has pairs worth more than 1 % of its width
// in every face (kern table for Liberation, GPOS for Carlito), and no control word has a pair
// in any table.
func TestKerningWords(t *testing.T) {
	var faces []*kernprobe.Font
	for _, name := range []string{"LiberationSans", "LiberationSerif", "Carlito"} {
		for _, bold := range []bool{false, true} {
			f, err := kernprobe.Bundled(name, bold)
			if err != nil {
				t.Fatal(err)
			}
			faces = append(faces, f)
		}
	}
	// what the comment on the deck says about the fonts
	for _, f := range faces {
		liberation := strings.HasPrefix(f.Family, "Liberation")
		if got := f.KernVersion >= 0; got != liberation {
			t.Errorf("%s %s: kern table %v, want %v", f.Family, f.Style, got, liberation)
		}
		if !f.GPOSKern {
			t.Errorf("%s %s: GPOS has no kern feature", f.Family, f.Style)
		}
	}
	all := map[string]bool{}
	for _, w := range KerningWords {
		all[w] = true
	}
	if len(KerningKerned)+len(KerningControl) != len(KerningWords) {
		t.Errorf("kerned and control words do not add up to the line")
	}
	for _, w := range KerningKerned {
		if !all[w] {
			t.Errorf("kerned word %q is not in the line", w)
		}
		for _, f := range faces {
			if s := f.Share(w); s > -0.01 {
				t.Errorf("%q in %s %s: kerning is %.2f %% of the width, want more than 1 %%", w, f.Family, f.Style, -100*s)
			}
		}
	}
	for _, w := range KerningControl {
		if !all[w] {
			t.Errorf("control word %q is not in the line", w)
		}
		for _, f := range faces {
			if k, g := f.Kern(w), f.GPOS(w); k != 0 || g != 0 {
				t.Errorf("control word %q in %s %s: kern table %+.4f em, GPOS %+.4f em", w, f.Family, f.Style, k, g)
			}
		}
	}
	// Liberation has the same values in both tables, so its rows cannot tell which one
	// PowerPoint read; the rows in Carlito can. If a later font version differs, say so.
	if c := kernprobe.Choose(faces, KerningWords, 0.01); len(c.Differ) != 0 {
		t.Logf("kern table and GPOS disagree for %v: the deck could now tell the tables apart", c.Differ)
	}
	for _, w := range kerningWrapWords {
		for _, f := range faces {
			if !f.Has(w) || f.Share(w) > -0.01 {
				t.Errorf("wrap word %q in %s %s: kerning is %.2f %% of the width", w, f.Family, f.Style, -100*f.Share(w))
			}
		}
	}
}
