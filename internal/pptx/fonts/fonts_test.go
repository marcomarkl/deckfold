package fonts

import (
	"bytes"
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
