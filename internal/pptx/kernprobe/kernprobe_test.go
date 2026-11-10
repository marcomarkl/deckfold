package kernprobe

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	tfont "github.com/tdewolff/font"

	"deckfold/internal/pptx/fonts"
)

// The kern table is read here and not through tdewolff/font, because that library refuses some
// fonts as a whole. For the fonts it does read, both must give the same pairs and the same
// advance widths: the converter measures text with tdewolff/font.
func TestSameValuesAsTheConverter(t *testing.T) {
	for _, name := range []string{"LiberationSans", "LiberationSerif", "Carlito"} {
		for _, bold := range []bool{false, true} {
			data := fonts.BundledFile(name, bold, false)
			f, err := Parse(data, 0)
			if err != nil {
				t.Fatal(err)
			}
			s, err := tfont.ParseSFNT(data, 0)
			if err != nil {
				t.Fatal(err)
			}
			upm := float64(s.UnitsPerEm())
			pairs := 0
			// Only characters the Unicode cmap has: for the others tdewolff/font answers from the
			// Mac Roman subtable (U+0080 becomes the glyph of byte 0x80 there), go-text does not.
			var runes []rune
			for r := rune(0x20); r < 0x250; r++ {
				if f.Has(string(r)) {
					runes = append(runes, r)
				}
			}
			if len(runes) < 300 {
				t.Fatalf("%s bold=%v: only %d characters between U+0020 and U+024F", name, bold, len(runes))
			}
			for _, a := range runes {
				ga := s.GlyphIndex(a)
				if want, got := float64(s.GlyphAdvance(ga))/upm, f.Plain(string(a)); math.Abs(want-got) > 1e-9 {
					t.Fatalf("%s bold=%v: advance of %q is %v, the converter has %v", name, bold, a, got, want)
				}
				for _, b := range runes {
					want, got := float64(s.Kerning(ga, s.GlyphIndex(b)))/upm, f.Pair(a, b)
					if want != got {
						t.Fatalf("%s bold=%v: pair %q %q is %v, the converter has %v", name, bold, a, b, got, want)
					}
					if got != 0 {
						pairs++
					}
				}
			}
			if hasTable := f.KernVersion >= 0; hasTable != (pairs > 0) {
				t.Errorf("%s bold=%v: kern table %v, but %d pairs between U+0020 and U+024F", name, bold, hasTable, pairs)
			}
		}
	}
}

func TestBundledFonts(t *testing.T) {
	sans, err := Bundled("LiberationSans", false)
	if err != nil {
		t.Fatal(err)
	}
	if sans.Family != "Liberation Sans" || sans.Style != "Regular" || sans.PostScript != "LiberationSans" {
		t.Errorf("names: %q %q %q", sans.Family, sans.Style, sans.PostScript)
	}
	if sans.KernVersion != 0 || sans.KernPairs == 0 || !sans.GPOSKern {
		t.Errorf("Liberation Sans: kern version %d with %d pairs, GPOS kern %v", sans.KernVersion, sans.KernPairs, sans.GPOSKern)
	}
	if k, g := sans.Kern("AVATAR"), sans.GPOS("AVATAR"); k >= 0 || math.Abs(k-g) > 1e-4 {
		t.Errorf("AVATAR in Liberation Sans: kern table %v em, GPOS %v em", k, g)
	}
	carlito, err := Bundled("Carlito", false)
	if err != nil {
		t.Fatal(err)
	}
	if carlito.KernVersion != -1 || !carlito.GPOSKern {
		t.Errorf("Carlito: kern version %d, GPOS kern %v", carlito.KernVersion, carlito.GPOSKern)
	}
	if k, g := carlito.Kern("AVATAR"), carlito.GPOS("AVATAR"); k != 0 || g >= 0 {
		t.Errorf("AVATAR in Carlito: kern table %v em, GPOS %v em", k, g)
	}
	c := Choose([]*Font{sans, carlito}, Words, 0.01)
	if len(c.Kerned) == 0 || len(c.Control) == 0 {
		t.Errorf("Choose found %d kerned and %d control words", len(c.Kerned), len(c.Control))
	}
}

// The tests of the kerning deck run everywhere and read GPOS through go-text/typesetting. For
// the bundled fonts that has to give what CoreText gives, which is checked here where CoreText
// is at hand (macOS with swift).
func TestGPOSOfTheBundledFontsAgainstCoreText(t *testing.T) {
	if !CoreTextAvailable() {
		t.Skip("needs CoreText (macOS with the swift command)")
	}
	texts := append(append([]string{}, Words...), "A V", "e T", "T o", "y Y")
	dir := t.TempDir()
	for _, name := range []string{"LiberationSans", "LiberationSerif", "Carlito"} {
		for _, bold := range []bool{false, true} {
			path := filepath.Join(dir, name+map[bool]string{false: "-Regular", true: "-Bold"}[bold]+".ttf")
			if err := os.WriteFile(path, fonts.BundledFile(name, bold, false), 0o644); err != nil {
				t.Fatal(err)
			}
			viaGoText, err := Open(path, 0)
			if err != nil {
				t.Fatal(err)
			}
			viaCoreText, _ := Open(path, 0)
			if err := viaCoreText.CoreText(texts); err != nil {
				t.Fatal(err)
			}
			for _, w := range texts {
				if a, b := viaGoText.GPOS(w), viaCoreText.GPOS(w); math.Abs(a-b) > 0.0005 {
					t.Errorf("%s bold=%v %q: go-text %+.4f em, CoreText %+.4f em", name, bold, w, a, b)
				}
			}
		}
	}
}
