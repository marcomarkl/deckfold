package pptx

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"deckfold/internal/htmlscan"
	"deckfold/internal/media"
	"deckfold/internal/pptx/pptxgen"
)

// convertMedia converts a generated deck with the conversion of pictures and fonts switched on
// and returns the folder of the deck and its HTML.
func convertMedia(t *testing.T, d *pptxgen.Deck) (string, *htmlscan.Doc, *media.Optimizer) {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "in.pptx")
	if err := d.Write(src); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "deck")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	m := media.New()
	if _, err := Convert(src, out, Options{BundledFontsOnly: true, Media: m}); err != nil {
		t.Fatal(err)
	}
	html, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	return out, htmlscan.Scan(html), m
}

// asset reads the file an href or src of the deck points to.
func asset(t *testing.T, dir, ref string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(ref)))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestMediaConversion: with an optimizer, the pictures of a deck are WebP files and its fonts
// WOFF2 files, and the page names them as such (decision E17).
func TestMediaConversion(t *testing.T) {
	dir, doc, m := convertMedia(t, pptxgen.Basic())
	pictures := 0
	for _, tg := range doc.Tags {
		if tg.Name != "image" {
			continue
		}
		href := tg.Value("href")
		if strings.HasSuffix(href, ".png") {
			t.Errorf("a PNG picture was not converted: %s", href)
		}
		if strings.HasSuffix(href, ".webp") {
			pictures++
			if b := asset(t, dir, href); !bytes.HasPrefix(b, []byte("RIFF")) || string(b[8:12]) != "WEBP" {
				t.Errorf("%s is no WebP file", href)
			}
		}
	}
	if pictures == 0 {
		t.Error("the deck has no WebP picture")
	}
	fontFiles := 0
	for _, tg := range doc.Tags {
		if tg.Name != "style" {
			continue
		}
		css := string(doc.Content(tg))
		if strings.Contains(css, `format("truetype")`) {
			t.Errorf("a font is still declared as TrueType: %s", css)
		}
		for _, part := range strings.Split(css, `url("`)[1:] {
			ref := part[:strings.Index(part, `"`)]
			if !strings.HasSuffix(ref, ".woff2") || !strings.Contains(part, `format("woff2")`) {
				t.Errorf("font source %s is not declared as WOFF2", ref)
				continue
			}
			fontFiles++
			if b := asset(t, dir, ref); !bytes.HasPrefix(b, []byte("wOF2")) {
				t.Errorf("%s is no WOFF2 file", ref)
			}
		}
	}
	if fontFiles == 0 {
		t.Error("the deck has no font file")
	}
	if s := m.Stats(); s.Pictures != pictures || s.Fonts != fontFiles || s.After >= s.Before {
		t.Errorf("stats %+v, want %d pictures, %d fonts and fewer bytes after than before", s, pictures, fontFiles)
	}
}

// TestMediaAnimatedPicture: a GIF that became an animated WebP picture keeps the mark by which
// the player starts it again when its slide is entered.
func TestMediaAnimatedPicture(t *testing.T) {
	dir, doc, _ := convertMedia(t, pptxgen.GIFs())
	marked := 0
	for _, tg := range doc.Tags {
		if tg.Name != "image" {
			continue
		}
		if _, ok := tg.Get("data-df-pptx-gif"); !ok {
			continue
		}
		marked++
		href := tg.Value("href")
		if !strings.HasSuffix(href, ".webp") {
			t.Errorf("the animated picture is %s, want a WebP file", href)
			continue
		}
		if b := asset(t, dir, href); !bytes.Contains(b[:64], []byte("VP8X")) || !bytes.Contains(b, []byte("ANMF")) {
			t.Errorf("%s is no animated WebP file", href)
		}
	}
	if marked != 1 {
		t.Errorf("%d pictures are marked as animated, want 1", marked)
	}
}

// TestMediaHostileDeck: the deck with hostile content converts with the conversion of pictures
// and fonts switched on, too.
func TestMediaHostileDeck(t *testing.T) {
	_, doc, _ := convertMedia(t, pptxgen.Hostile())
	if len(doc.Tags) == 0 {
		t.Fatal("no deck was written")
	}
}
