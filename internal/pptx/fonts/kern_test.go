package fonts_test

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"deckfold/internal/pptx/fonts"
	"deckfold/internal/pptx/kernprobe"
)

func bundledOnly() *fonts.Resolver {
	r := fonts.NewResolver(nil)
	r.BundledOnly = true
	return r
}

// Where the kerning of a face comes from: its own kern table, the measured substitutes, or
// nothing.
func TestKernSources(t *testing.T) {
	r := bundledOnly()
	av := func(family string, bold, italic bool) float64 { return r.Face(family, bold, italic).Kern('A', 'V') }
	sans := av("Liberation Sans", false, false)
	if sans >= 0 {
		t.Fatalf("Liberation Sans: AV is %v em, want a negative pair from its kern table", sans)
	}
	for _, c := range []struct {
		family       string
		bold, italic bool
		kerned       bool
		why          string
	}{
		{"Liberation Sans", false, false, true, "its own kern table"},
		{"Liberation Serif", true, false, true, "its own kern table"},
		{"Carlito", false, false, false, "no kern table, and PowerPoint does not read GPOS"},
		{"Carlito", true, false, false, "no kern table"},
		{"Calibri", false, false, true, "Carlito's GPOS has the pairs of Calibri's kern table"},
		{"Calibri", false, true, true, "the same for italic"},
		{"Calibri", true, true, true, "the same for bold italic"},
		{"Calibri", true, false, true, "Carlito Bold has the pairs of Calibri Bold, and 51 more"},
		{"Calibri Light", false, false, false, "another font than Calibri, not compared"},
		{"Arial", false, false, true, "Liberation Sans has the pairs of Arial"},
		{"Arial", true, true, true, "in every style"},
		{"Helvetica", false, false, true, "Liberation Sans has the pairs of Helvetica"},
		{"Helvetica", false, true, false, "Helvetica Oblique has other pairs"},
		{"Times New Roman", true, false, true, "Liberation Serif has the pairs of Times New Roman"},
		{"Helvetica Neue", false, false, false, "a substitute nobody compared"},
		{"Verdana", false, false, false, "a font deckfold has no match for"},
		{"Liberation Mono", false, false, false, "no kerning in a monospaced font"},
	} {
		f := r.Face(c.family, c.bold, c.italic)
		if got := f.Kern('A', 'V') != 0; got != c.kerned || f.Kerns() != c.kerned {
			t.Errorf("%s bold=%v italic=%v: AV %v em, Kerns %v; want kerned %v (%s)", c.family, c.bold, c.italic, f.Kern('A', 'V'), f.Kerns(), c.kerned, c.why)
		}
	}
	if got := av("Arial", false, false); got != sans {
		t.Errorf("Arial is drawn in Liberation Sans: AV %v em, Liberation Sans itself %v em", got, sans)
	}
	if f := r.Face("Liberation Sans", false, false); f.Kern('n', 'o') != 0 || f.Kern('A', 0x4e2d) != 0 {
		t.Errorf("pairs the table does not have must be 0")
	}
}

var ascii = func() (out []rune) {
	for r := rune(0x20); r < 0x7f; r++ {
		out = append(out, r)
	}
	return
}()

// Carlito's kerning comes out of GPOS through go-text/typesetting, which has missed pairs in
// other fonts. These sums were taken when it agreed with CoreText pair by pair (the test
// below); they catch a library version that reads Carlito differently, on every platform.
func TestCarlitoKerningForCalibri(t *testing.T) {
	r := bundledOnly()
	for _, c := range []struct {
		bold, italic bool
		pairs        int
		sum          float64 // of the absolute values, in em
	}{
		{false, false, wantRegularPairs, wantRegularSum},
		{true, false, wantBoldPairs, wantBoldSum},
		{false, true, wantItalicPairs, wantItalicSum},
		{true, true, wantBoldItalicPairs, wantBoldItalicSum},
	} {
		f := r.Face("Calibri", c.bold, c.italic)
		pairs, sum := 0, 0.0
		for _, a := range ascii {
			for _, b := range ascii {
				if k := f.Kern(a, b); k != 0 {
					pairs++
					sum += math.Abs(k)
				}
			}
		}
		if pairs != c.pairs || math.Abs(sum-c.sum) > 0.0005 {
			t.Errorf("Carlito bold=%v italic=%v for Calibri: %d kerned pairs of printable ASCII, %.4f em in sum; want %d and %.4f", c.bold, c.italic, pairs, sum, c.pairs, c.sum)
		}
	}
}

// The same pairs against CoreText, where CoreText is at hand (macOS with swift).
func TestCarlitoKerningAgainstCoreText(t *testing.T) {
	if !kernprobe.CoreTextAvailable() {
		t.Skip("needs CoreText (macOS with the swift command)")
	}
	var texts []string
	for _, a := range ascii {
		for _, b := range ascii {
			texts = append(texts, string([]rune{a, b}))
		}
	}
	r := bundledOnly()
	dir := t.TempDir()
	for _, c := range []struct {
		file         string
		bold, italic bool
	}{{"Regular", false, false}, {"Bold", true, false}, {"Italic", false, true}, {"BoldItalic", true, true}} {
		path := filepath.Join(dir, "Carlito-"+c.file+".ttf")
		if err := os.WriteFile(path, fonts.BundledFile("Carlito", c.bold, c.italic), 0o644); err != nil {
			t.Fatal(err)
		}
		ct, err := kernprobe.Open(path, 0)
		if err != nil {
			t.Fatal(err)
		}
		if err := ct.CoreText(texts); err != nil {
			t.Fatal(err)
		}
		f := r.Face("Calibri", c.bold, c.italic)
		bad := 0
		for _, s := range texts {
			rs := []rune(s)
			if got, want := f.Kern(rs[0], rs[1]), ct.GPOS(s); math.Abs(got-want) > 0.0005 {
				if bad++; bad <= 5 {
					t.Errorf("Carlito %s %q: %+.4f em, CoreText %+.4f em", c.file, s, got, want)
				}
			}
		}
		if bad > 5 {
			t.Errorf("Carlito %s: %d pairs differ from CoreText", c.file, bad)
		}
	}
}

// filled in from a run that agreed with CoreText (2026-11-10)
const (
	wantRegularPairs, wantRegularSum       = 611, 14.9705
	wantBoldPairs, wantBoldSum             = 660, 16.0558
	wantItalicPairs, wantItalicSum         = 565, 14.2993
	wantBoldItalicPairs, wantBoldItalicSum = 562, 13.4683
)
