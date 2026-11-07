package fonts

import (
	"bytes"
	"fmt"
	"testing"

	tfont "github.com/tdewolff/font"
)

func TestBundledSubsetRenamed(t *testing.T) {
	r := NewResolver(nil)
	r.BundledOnly = true
	f := r.Face("Calibri", false, false)
	if f.Source != Bundled || f.Subst {
		t.Fatalf("Calibri: source %s, subst %v; want bundled Carlito with the same metrics", f.Source, f.Subst)
	}
	if a := f.Advance('W'); a <= 0.3 || a > 1.5 {
		t.Fatalf("advance of W = %v em", a)
	}
	f.Use("Hello, world")
	data, family, err := f.Subset()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > 60000 {
		t.Errorf("subset is %d bytes, expected a small file", len(data))
	}
	sub, err := tfont.ParseSFNT(data, 0)
	if err != nil {
		t.Fatalf("subset does not parse: %v", err)
	}
	for _, c := range "Helo,wrd" {
		if sub.GlyphIndex(c) == 0 {
			t.Errorf("subset lost %q", c)
		}
	}
	if sub.GlyphIndex('Z') != 0 {
		t.Errorf("subset kept an unused glyph")
	}
	names := sub.Name.Get(1)
	if len(names) == 0 || names[0].String() != family {
		t.Errorf("family name of the subset = %v, want %s", names, family)
	}
	if bytes.Contains(data, []byte("C\x00a\x00r\x00l\x00i\x00t\x00o\x00 \x00R")) {
		t.Error("subset still names the reserved family")
	}
	again, _, _ := f.Subset()
	if !bytes.Equal(data, again) {
		t.Error("subset output differs between runs")
	}
}

func TestSubstituteFlags(t *testing.T) {
	r := NewResolver(nil)
	r.BundledOnly = true
	if f := r.Face("Aptos", true, false); !f.Subst || f.Source != Bundled {
		t.Errorf("Aptos should be a bundled substitute, got %s subst=%v", f.Source, f.Subst)
	}
	if f := r.Face("Times New Roman", false, true); f.Subst {
		t.Errorf("Times New Roman has a metric-compatible bundled face")
	}
}

// A subset of spaces only must still have outlines (Chrome rejects an empty glyf table).
func TestSubsetOfSpaces(t *testing.T) {
	r := NewResolver(nil)
	r.BundledOnly = true
	f := r.Face("Arial", false, false)
	f.Use("   ")
	data, _, err := f.Subset()
	if err != nil {
		t.Fatal(err)
	}
	sub, err := tfont.ParseSFNT(data, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(sub.Tables["glyf"]) == 0 {
		t.Fatal("glyf table is empty")
	}
}

// Installed fonts are found in the platform folders, collections included.
func TestInstalledArial(t *testing.T) {
	r := NewResolver(nil)
	f := r.Face("Arial", true, false)
	if f.Source != Installed {
		t.Skipf("Arial is not installed here (got %s)", f.Source)
	}
	if !f.Bold {
		t.Errorf("bold face requested")
	}
	if f.sfnt.OS2 == nil || f.sfnt.OS2.UsWeightClass < 600 {
		t.Errorf("Arial bold resolved to %s, which is not a bold face", f.File)
	}
}

// TestEmbeddedRoundTrip: a bundled font, wrapped as PowerPoint embeds fonts (EOT with MTX),
// comes back from Deobfuscate with the same outlines, programs and metrics.
func TestEmbeddedRoundTrip(t *testing.T) {
	chars := "Embedded font: the quick brown fox jumps over the lazy dog. \u00c4\u00d6\u00dc\u00e4\u00f6\u00fc\u00e9\u00e8\u00e7\u00f1 0123456789"
	orig, err := BundledSubset("LiberationSerif", "Deckfold Embedded Test", chars)
	if err != nil {
		t.Fatal(err)
	}
	eot, err := EOT(orig)
	if err != nil {
		t.Fatal(err)
	}
	back, err := Deobfuscate(eot, "font1.fntdata")
	if err != nil {
		t.Fatal(err)
	}
	a, err := tfont.ParseSFNT(orig, 0)
	if err != nil {
		t.Fatal(err)
	}
	b, err := tfont.ParseSFNT(back, 0)
	if err != nil {
		t.Fatalf("decoded font does not parse: %v", err)
	}
	if a.NumGlyphs() != b.NumGlyphs() {
		t.Fatalf("%d glyphs, want %d", b.NumGlyphs(), a.NumGlyphs())
	}
	composite := 0
	for g := uint16(0); g < a.NumGlyphs(); g++ {
		if a.Glyf.IsComposite(g) {
			composite++
		}
		ca, err1 := a.Glyf.Contour(g)
		cb, err2 := b.Glyf.Contour(g)
		if err1 != nil || err2 != nil {
			t.Fatalf("glyph %d: %v, %v", g, err1, err2)
		}
		if fmt.Sprint(ca) != fmt.Sprint(cb) || a.GlyphAdvance(g) != b.GlyphAdvance(g) {
			t.Errorf("glyph %d differs after the round trip", g)
		}
	}
	if composite == 0 {
		t.Error("the test font has no composite glyph")
	}
	for _, r := range "E\u00e99" {
		if b.GlyphIndex(r) != a.GlyphIndex(r) {
			t.Errorf("cmap of %q differs", r)
		}
	}
}

// TestSubsetComposite: in a subset every character keeps its glyph, also when composite glyphs
// bring in components that tdewolff/font's reverse cmap lookup attributes to other characters
// (with a c cedilla in the text, the u with diaeresis used to show the cedilla).
func TestSubsetComposite(t *testing.T) {
	text := "Gr\u00fc\u00dfe aus \u00c4gypten: fa\u00e7ade, na\u00efve, \u00e4\u00f6\u00fc \u00c4\u00d6\u00dc \u00e9\u00e8\u00ea \u00e7 \u00f1 \u00a8\u00b8\u00b4`~^"
	for _, name := range []string{"Carlito", "LiberationSans", "LiberationSerif", "LiberationMono"} {
		r := NewResolver(nil)
		r.BundledOnly = true
		f := r.Face(map[string]string{"Carlito": "Calibri", "LiberationSans": "Arial", "LiberationSerif": "Times New Roman", "LiberationMono": "Courier New"}[name], false, false)
		f.Use(text)
		data, _, err := f.Subset()
		if err != nil {
			t.Fatal(err)
		}
		sub, err := tfont.ParseSFNT(data, 0)
		if err != nil {
			t.Fatal(err)
		}
		full, _ := tfont.ParseSFNT(bundledData(name, false, false), 0)
		for _, c := range text {
			a, err1 := full.Glyf.Contour(full.GlyphIndex(c))
			b, err2 := sub.Glyf.Contour(sub.GlyphIndex(c))
			if err1 != nil || err2 != nil || fmt.Sprint(a.XCoordinates, a.YCoordinates) != fmt.Sprint(b.XCoordinates, b.YCoordinates) {
				t.Errorf("%s: %q has another glyph in the subset", name, c)
			}
		}
	}
}
