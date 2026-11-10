package main

import (
	"archive/zip"
	"fmt"
	"html"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"deckfold/internal/pptx/fonts"
	"deckfold/internal/pptx/kernprobe"
)

// substitutePairs are the families deckfold replaces by a bundled font with the same metrics
// (internal/pptx/fonts), where the original has a kern table PowerPoint applies.
var substitutePairs = []struct{ original, bundled, name string }{
	{"Calibri", "Carlito", "Carlito"},
	{"Arial", "Liberation Sans", "LiberationSans"},
	{"Helvetica", "Liberation Sans", "LiberationSans"},
	{"Times New Roman", "Liberation Serif", "LiberationSerif"},
}

var (
	paraRe = regexp.MustCompile(`(?s)<a:p>.*?</a:p>`)
	textRe = regexp.MustCompile(`(?s)<a:t>(.*?)</a:t>`)
)

// deckLines returns the paragraphs of the public feature decks: the text a substitute has to
// set like the original.
func deckLines(dir string) ([]string, error) {
	decks, err := filepath.Glob(filepath.Join(dir, "*.pptx"))
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, deck := range decks {
		z, err := zip.OpenReader(deck)
		if err != nil {
			return nil, err
		}
		for _, f := range z.File {
			if !strings.HasPrefix(f.Name, "ppt/slides/slide") {
				continue
			}
			r, err := f.Open()
			if err != nil {
				z.Close()
				return nil, err
			}
			data, _ := io.ReadAll(r)
			r.Close()
			for _, p := range paraRe.FindAll(data, -1) {
				var t strings.Builder
				for _, m := range textRe.FindAllSubmatch(p, -1) {
					t.WriteString(html.UnescapeString(string(m[1])))
				}
				line := strings.Join(strings.Fields(t.String()), " ")
				// not the Hebrew and Arabic paragraphs: they are shaped, and the bundled fonts
				// have other advance widths there than the originals
				if len([]rune(line)) >= 12 && !seen[line] && !fonts.Complex(line) {
					seen[line] = true
					out = append(out, line)
				}
			}
		}
		z.Close()
	}
	sort.Strings(out)
	return out, nil
}

// substitutes compares, for every paragraph of the public decks, the width PowerPoint gives it
// in the original font (advance widths plus the kern table) with what deckfold can compute for
// the bundled substitute: without kerning, with the substitute's kern table, and with the
// kerning of its GPOS.
func substitutes(srcDir, out string) error {
	lines, err := deckLines(srcDir)
	if err != nil {
		return err
	}
	work := filepath.Join(out, "fonts")
	if err := os.MkdirAll(work, 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("original\tfile\tsubstitute\tlines\tsame advance widths\tsubstitute without kerning: mean %\tmax %\tmax px at 28 pt\twith its kern table: mean %\tmax %\tmax px at 28 pt\twith its GPOS: mean %\tmax %\tmax px at 28 pt\n")
	for _, pair := range substitutePairs {
		// the bundled font as a file, so that CoreText can be asked about its GPOS
		path := filepath.Join(work, pair.name+"-Regular.ttf")
		if err := os.WriteFile(path, fonts.BundledFile(pair.name, false, false), 0o644); err != nil {
			return err
		}
		sub, err := kernprobe.Open(path, 0)
		if err != nil {
			return err
		}
		originals := kernprobe.Find(pair.original, "Regular")
		if len(originals) == 0 {
			fmt.Fprintf(&b, "%s\tnot installed\t%s\n", pair.original, pair.bundled)
			continue
		}
		for _, orig := range originals {
			var use []string
			for _, l := range lines {
				if orig.Has(l) && sub.Has(l) {
					use = append(use, l)
				}
			}
			if sub.GPOSKern && kernprobe.CoreTextAvailable() {
				if err := sub.CoreText(use); err != nil {
					return err
				}
			}
			type stat struct{ sum, max, maxPx float64 }
			var plain, kern, gpos stat
			same := true
			add := func(s *stat, got, want float64) {
				d := math.Abs(got-want) / want
				s.sum += d
				s.max = math.Max(s.max, d)
				s.maxPx = math.Max(s.maxPx, math.Abs(got-want)*28*96/72)
			}
			for _, l := range use {
				want := orig.Plain(l) + orig.Kern(l)
				p := sub.Plain(l)
				if math.Abs(p-orig.Plain(l)) > 0.0005 {
					same = false
				}
				add(&plain, p, want)
				add(&kern, p+sub.Kern(l), want)
				add(&gpos, p+sub.GPOS(l), want)
			}
			n := float64(len(use))
			cell := func(s stat, has bool) string {
				if !has {
					return "-\t-\t-"
				}
				return fmt.Sprintf("%.2f\t%.2f\t%.1f", 100*s.sum/n, 100*s.max, s.maxPx)
			}
			fmt.Fprintf(&b, "%s\t%s\t%s\t%d\t%v\t%s\t%s\t%s\n", pair.original, orig.Path, pair.bundled, len(use), same,
				cell(plain, true), cell(kern, sub.KernVersion >= 0), cell(gpos, sub.GPOSKern))
		}
	}
	// every pair of printable ASCII characters, not only the ones the decks happen to use
	b.WriteString("\noriginal\tfile\tsubstitute\tpairs of printable ASCII\tkerned in the original\tsubstitute differs (more than 0.001 em)\tlargest difference em\tcharacters with another advance width\n")
	var ascii []rune
	for r := rune(0x20); r < 0x7f; r++ {
		ascii = append(ascii, r)
	}
	var texts []string
	for _, a := range ascii {
		for _, c := range ascii {
			texts = append(texts, string([]rune{a, c}))
		}
	}
	styles := []struct {
		name, file   string
		bold, italic bool
	}{{"Regular", "Regular", false, false}, {"Bold", "Bold", true, false}, {"Italic", "Italic", false, true}, {"Bold Italic", "BoldItalic", true, true}}
	for _, pair := range substitutePairs {
		for _, st := range styles {
			path := filepath.Join(work, pair.name+"-"+st.file+".ttf")
			if err := os.WriteFile(path, fonts.BundledFile(pair.name, st.bold, st.italic), 0o644); err != nil {
				return err
			}
			sub, err := kernprobe.Open(path, 0)
			if err != nil {
				return err
			}
			// the substitute's own kerning: its kern table, or GPOS where it has none (Carlito)
			own := sub.Kern
			if sub.KernVersion < 0 {
				if kernprobe.CoreTextAvailable() {
					if err := sub.CoreText(texts); err != nil {
						return err
					}
				}
				own = sub.GPOS
			}
			originals := kernprobe.Find(pair.original, st.name)
			if st.name == "Italic" && len(originals) == 0 {
				originals = kernprobe.Find(pair.original, "Oblique") // Helvetica
			}
			if st.name == "Bold Italic" && len(originals) == 0 {
				originals = kernprobe.Find(pair.original, "Bold Oblique")
			}
			if len(originals) == 0 {
				fmt.Fprintf(&b, "%s %s\tnot installed\t%s\n", pair.original, st.name, pair.bundled)
			}
			for _, orig := range originals {
				kerned, differ, worst, widths := 0, 0, 0.0, 0
				for _, t := range texts {
					want := orig.Kern(t)
					if want != 0 {
						kerned++
					}
					if d := math.Abs(own(t) - want); d > 0.001 {
						differ++
						worst = math.Max(worst, d)
					}
				}
				for _, r := range ascii {
					if math.Abs(orig.Plain(string(r))-sub.Plain(string(r))) > 0.0005 {
						widths++
					}
				}
				fmt.Fprintf(&b, "%s %s\t%s\t%s\t%d\t%d\t%d\t%.4f\t%d\n", pair.original, st.name, orig.Path, pair.bundled, len(texts), kerned, differ, worst, widths)
			}
		}
	}
	fmt.Print(b.String())
	return os.WriteFile(filepath.Join(out, "substitutes.tsv"), []byte(b.String()), 0o644)
}
