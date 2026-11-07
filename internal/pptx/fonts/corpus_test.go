package fonts

import (
	"archive/zip"
	"io"
	"path/filepath"
	"strings"
	"testing"

	tfont "github.com/tdewolff/font"
)

// TestEmbeddedFontsInCorpus decodes every font embedded in the private decks under
// testdata/pptx (not part of the repository) and parses every glyph of the result. There is no
// MTX encoder to make public test data with; spike S7 describes the comparison with installed
// copies of the same fonts.
func TestEmbeddedFontsInCorpus(t *testing.T) {
	decks, _ := filepath.Glob(filepath.Join("..", "..", "..", "testdata", "pptx", "*.pptx"))
	if len(decks) == 0 {
		t.Skip("local test corpus not present (testdata/ is not part of the repository)")
	}
	n := 0
	for i, deck := range decks {
		z, err := zip.OpenReader(deck)
		if err != nil {
			continue
		}
		for _, f := range z.File {
			if !strings.HasPrefix(f.Name, "ppt/fonts/") {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, _ := io.ReadAll(rc)
			rc.Close()
			out, err := Deobfuscate(data, f.Name)
			if err != nil {
				t.Errorf("deck %d, %s: %v", i+1, f.Name, err)
				continue
			}
			s, err := tfont.ParseSFNT(out, 0)
			if err != nil {
				t.Errorf("deck %d, %s: result does not parse: %v", i+1, f.Name, err)
				continue
			}
			if s.Glyf != nil {
				for g := uint16(0); g < s.NumGlyphs(); g++ {
					if _, err := s.Glyf.Contour(g); err != nil {
						t.Errorf("deck %d, %s: glyph %d: %v", i+1, f.Name, g, err)
						break
					}
				}
			}
			n++
		}
		z.Close()
	}
	t.Logf("%d embedded fonts decoded", n)
}
