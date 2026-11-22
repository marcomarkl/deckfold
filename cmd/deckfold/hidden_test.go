package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"deckfold/internal/config"
	"deckfold/internal/htmlscan"
	"deckfold/internal/pptx/pptxgen"
)

// hiddenOutput converts the basic deck (its third slide is hidden in PowerPoint) with the given
// arguments and returns the report, the hidden flags of the slides in the metadata and the
// defaults in #df-config.
func hiddenOutput(t *testing.T, args ...string) (string, []bool, string) {
	t.Helper()
	t.Setenv("DECKFOLD_BUNDLED_FONTS", "1")
	dir := t.TempDir()
	src, out := filepath.Join(dir, "in.pptx"), filepath.Join(dir, "out.html")
	if err := pptxgen.Basic().Write(src); err != nil {
		t.Fatal(err)
	}
	c, o, e := runCLI(t, append([]string{src, "-o", out}, args...)...)
	if c != 0 {
		t.Fatalf("exit %d %s %s", c, o, e)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	d := htmlscan.Scan(b)
	var meta struct {
		Deck struct {
			Slides []struct {
				Hidden bool `json:"hidden"`
			} `json:"slides"`
		} `json:"deck"`
	}
	mb := blocksOf(d, "df-meta")
	if len(mb) != 1 {
		t.Fatalf("%d metadata blocks", len(mb))
	}
	if err := json.Unmarshal(d.Content(mb[0]), &meta); err != nil {
		t.Fatal(err)
	}
	var hidden []bool
	for _, s := range meta.Deck.Slides {
		hidden = append(hidden, s.Hidden)
	}
	cfg := ""
	if cb := blocksOf(d, "df-config"); len(cb) == 1 {
		cfg = string(d.Content(cb[0]))
	}
	return o, hidden, cfg
}

// By default a slide hidden in PowerPoint is written and marked as hidden (E21), and the report
// says so; --drop-hidden and the configuration key dropHidden leave it out as before, and the
// switch is not stored in the output.
func TestDropHidden(t *testing.T) {
	o, hidden, cfg := hiddenOutput(t)
	if !strings.Contains(o, "4 slides, 1 hidden slide included, skipped in the show (--drop-hidden leaves them out)") {
		t.Errorf("report:\n%s", o)
	}
	if len(hidden) != 4 || !hidden[2] || hidden[0] || hidden[1] || hidden[3] {
		t.Errorf("hidden flags in the metadata: %v, want the third of 4", hidden)
	}
	if strings.Contains(cfg, "dropHidden") || strings.Contains(cfg, "DropHidden") {
		t.Errorf("the conversion switch is stored in the output: %s", cfg)
	}
	for _, how := range []string{"flag", "configuration"} {
		var o, cfg string
		var hidden []bool
		if how == "flag" {
			o, hidden, cfg = hiddenOutput(t, "--drop-hidden")
		} else {
			o, hidden, cfg = hiddenWithConfig(t, `{"dropHidden": true}`)
		}
		if !strings.Contains(o, "3 slides, 1 hidden slide skipped") {
			t.Errorf("%s: report:\n%s", how, o)
		}
		if len(hidden) != 3 || hidden[0] || hidden[1] || hidden[2] {
			t.Errorf("%s: hidden flags in the metadata: %v, want 3 shown slides", how, hidden)
		}
		if strings.Contains(cfg, "dropHidden") {
			t.Errorf("%s: the switch is stored in the output: %s", how, cfg)
		}
	}
	// the flag wins over the configuration file
	if o, hidden, _ := hiddenWithConfig(t, `{"dropHidden": true}`, "--drop-hidden=false"); len(hidden) != 4 {
		t.Errorf("--drop-hidden=false does not override the configuration file: %d slides\n%s", len(hidden), o)
	}
}

// hiddenWithConfig is hiddenOutput with a configuration file of the given content.
func hiddenWithConfig(t *testing.T, content string, args ...string) (string, []bool, string) {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "cfg.json")
	put(t, cfg, content)
	return hiddenOutput(t, append([]string{"--config", cfg}, args...)...)
}

// TestPrintConfigHasDropHidden: the sample configuration shows the key, and it loads.
func TestPrintConfigHasDropHidden(t *testing.T) {
	c, o, _ := runCLI(t, "--print-config")
	if c != 0 || !strings.Contains(o, `"dropHidden": false`) {
		t.Fatalf("exit %d, sample configuration:\n%s", c, o)
	}
	if _, err := config.Merge([]byte(o), config.Default()); err != nil {
		t.Errorf("the sample configuration does not load: %v", err)
	}
}
