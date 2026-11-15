package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"deckfold/internal/config"
	"deckfold/internal/htmlscan"
	"deckfold/internal/pptx/fonts"
)

// mediaDeck writes an HTML deck that uses one PNG picture in every way a page can (picture
// element, stylesheet, icon, plugin) and one TrueType font, and returns the HTML file.
func mediaDeck(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	img := image.NewNRGBA(image.Rect(0, 0, 200, 150))
	for y := 0; y < 150; y++ {
		for x := 0; x < 200; x++ {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x), uint8(y / 10 * 10), 90, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(dir, "pic.png"), b.String())
	put(t, filepath.Join(dir, "f.ttf"), string(fonts.BundledFile("Carlito", false, false)))
	put(t, filepath.Join(dir, "s.css"), `@font-face{font-family:T;src:url("f.ttf") format("truetype")} body{background:url(pic.png)}`)
	html := filepath.Join(dir, "d.html")
	put(t, html, `<!doctype html><html><head><meta charset="utf-8"><title>M</title><link rel="icon" href="pic.png"><link rel="stylesheet" href="s.css"></head><body>
<picture><source type="image/png" srcset="pic.png 1x, pic.png 2x"><img src="pic.png"></picture><object data="pic.png"></object></body></html>`)
	return html
}

// mediaOutput reads what a conversion made of mediaDeck: the asset blocks by MIME type, the type
// of the <source>, the stylesheet, and the stored configuration.
func mediaOutput(t *testing.T, html string) (mimes map[string]int, sourceType, css, cfg string) {
	t.Helper()
	b, err := os.ReadFile(strings.TrimSuffix(html, ".html") + ".standalone.html")
	if err != nil {
		t.Fatal(err)
	}
	doc := htmlscan.Scan(b)
	mimes = map[string]int{}
	for _, tg := range doc.Tags {
		switch {
		case tg.Name == "script" && tg.Value("type") == "application/octet-stream" && strings.HasSuffix(tg.Value("id"), "-0"):
			mimes[tg.Value("data-mime")]++
		case tg.Name == "script" && tg.Value("type") == "text/df-css":
			css += string(doc.Content(tg))
		case tg.Name == "script" && tg.Value("id") == "df-config":
			cfg = string(doc.Content(tg))
		case tg.Name == "source":
			sourceType = tg.Value("type")
		}
	}
	return
}

// TestMediaInHTML: what a page shows as a picture or uses as a font is converted, and the type
// and format hints follow. An icon and the file of a plugin stay as they are (decision E17).
func TestMediaInHTML(t *testing.T) {
	html := mediaDeck(t)
	c, o, e := runCLI(t, html, "--generic")
	if c != 0 {
		t.Fatalf("exit %d %s", c, e)
	}
	if !strings.Contains(o, "Converted: 1 picture and 1 font, ") {
		t.Errorf("the report does not name the converted assets:\n%s", o)
	}
	mimes, sourceType, css, cfg := mediaOutput(t, html)
	// the picture once as WebP (picture element and stylesheet) and once unchanged (icon and
	// plugin); the font as WOFF2
	if mimes["image/webp"] != 1 || mimes["image/png"] != 1 || mimes["font/woff2"] != 1 || mimes["font/ttf"] != 0 {
		t.Errorf("asset blocks by type: %v", mimes)
	}
	if sourceType != "image/webp" {
		t.Errorf(`<source type=%q>, want "image/webp"`, sourceType)
	}
	if !strings.Contains(css, `format("woff2")`) || strings.Contains(css, "truetype") {
		t.Errorf("the format hint of the font was not changed: %s", css)
	}
	if strings.Contains(cfg, "keepAssets") || strings.Contains(cfg, "KeepAssets") {
		t.Errorf("the conversion switch is stored in the output: %s", cfg)
	}
}

// TestKeepAssets: the switch and the configuration key leave every asset as it is.
func TestKeepAssets(t *testing.T) {
	for _, how := range []string{"flag", "configuration"} {
		html := mediaDeck(t)
		args := []string{html, "--generic", "--keep-assets"}
		if how == "configuration" {
			cfg := filepath.Join(filepath.Dir(html), "cfg.json")
			put(t, cfg, `{"keepAssets": true}`)
			args = []string{html, "--generic", "--config", cfg}
		}
		c, o, e := runCLI(t, args...)
		if c != 0 {
			t.Fatalf("%s: exit %d %s", how, c, e)
		}
		if strings.Contains(o, "Converted:") {
			t.Errorf("%s: the report names converted assets:\n%s", how, o)
		}
		mimes, sourceType, css, cfg := mediaOutput(t, html)
		if mimes["image/png"] != 1 || mimes["font/ttf"] != 1 || mimes["image/webp"] != 0 || mimes["font/woff2"] != 0 {
			t.Errorf("%s: asset blocks by type: %v", how, mimes)
		}
		if sourceType != "image/png" || !strings.Contains(css, `format("truetype")`) {
			t.Errorf("%s: <source type=%q>, stylesheet %s", how, sourceType, css)
		}
		if strings.Contains(cfg, "keepAssets") {
			t.Errorf("%s: the switch is stored in the output: %s", how, cfg)
		}
	}
	// the flag wins over the configuration file
	html := mediaDeck(t)
	cfg := filepath.Join(filepath.Dir(html), "cfg.json")
	put(t, cfg, `{"keepAssets": true}`)
	if c, _, e := runCLI(t, html, "--generic", "--config", cfg, "--keep-assets=false"); c != 0 {
		t.Fatalf("exit %d %s", c, e)
	}
	if mimes, _, _, _ := mediaOutput(t, html); mimes["image/webp"] != 1 {
		t.Errorf("--keep-assets=false does not override the configuration file: %v", mimes)
	}
}

// TestPrintConfigHasKeepAssets: the sample configuration shows the key, and it loads.
func TestPrintConfigHasKeepAssets(t *testing.T) {
	c, o, _ := runCLI(t, "--print-config")
	if c != 0 || !strings.Contains(o, `"keepAssets": false`) {
		t.Fatalf("exit %d, sample configuration:\n%s", c, o)
	}
	if _, err := config.Merge([]byte(o), config.Default()); err != nil {
		t.Errorf("the sample configuration does not load: %v", err)
	}
}
