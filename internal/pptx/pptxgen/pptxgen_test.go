package pptxgen

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
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
