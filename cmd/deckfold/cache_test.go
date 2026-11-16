package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cacheFiles lists the entries in the cache folder beside an output.
func cacheFiles(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	filepath.Walk(filepath.Join(dir, cacheFolder), func(p string, fi os.FileInfo, err error) error {
		if err == nil && fi.Mode().IsRegular() {
			out = append(out, p)
		}
		return nil
	})
	return out
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestCacheBesideOutput: the first conversion fills the folder beside the output, the second
// takes the picture from it and says so, and the output is the same with an empty cache, a
// filled one and none.
func TestCacheBesideOutput(t *testing.T) {
	html := mediaDeck(t)
	dir := filepath.Dir(html)
	out := strings.TrimSuffix(html, ".html") + ".standalone.html"

	c, o, e := runCLI(t, html, "--generic")
	if c != 0 {
		t.Fatalf("exit %d %s", c, e)
	}
	if strings.Contains(o, cacheFolder) {
		t.Errorf("the first conversion says it took something from the cache:\n%s", o)
	}
	cold := mustRead(t, out)
	if n := len(cacheFiles(t, dir)); n != 1 {
		t.Fatalf("%d entries beside the output after the first conversion, want 1 (the font is not kept)", n)
	}

	c, o, e = runCLI(t, html, "--generic", "--force")
	if c != 0 {
		t.Fatalf("exit %d %s", c, e)
	}
	if !strings.Contains(o, "; 1 encoder run saved by "+cacheFolder) {
		t.Errorf("the second conversion does not say what came from the cache:\n%s", o)
	}
	if !bytes.Equal(mustRead(t, out), cold) {
		t.Error("the output with a filled cache differs from the one with an empty cache")
	}

	if err := os.RemoveAll(filepath.Join(dir, cacheFolder)); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "cfg.json")
	put(t, cfg, `{"cache": false}`)
	for how, args := range map[string][]string{
		"--no-cache":           {html, "--generic", "--force", "--no-cache"},
		`"cache": false`:       {html, "--generic", "--force", "--config", cfg},
		"--keep-assets":        {html, "--generic", "--force", "--keep-assets"},
		"--no-cache over file": {html, "--generic", "--force", "--config", cfg, "--no-cache=true"},
	} {
		if c, _, e := runCLI(t, args...); c != 0 {
			t.Fatalf("%s: exit %d %s", how, c, e)
		}
		if _, err := os.Stat(filepath.Join(dir, cacheFolder)); err == nil {
			t.Errorf("%s: a cache folder was made", how)
			os.RemoveAll(filepath.Join(dir, cacheFolder))
		}
		if how != "--keep-assets" && !bytes.Equal(mustRead(t, out), cold) {
			t.Errorf("%s: the output differs from the one with a cache", how)
		}
	}
	// the flag wins over the configuration file
	if c, _, e := runCLI(t, html, "--generic", "--force", "--config", cfg, "--no-cache=false"); c != 0 {
		t.Fatalf("exit %d %s", c, e)
	}
	if n := len(cacheFiles(t, dir)); n != 1 {
		t.Errorf("--no-cache=false does not override the configuration file: %d entries", n)
	}
	// the switch steers the conversion and is not stored in the output
	if _, _, _, stored := mediaOutput(t, html); strings.Contains(stored, "cache") || strings.Contains(stored, "Cache") {
		t.Errorf("the cache switch is stored in the output: %s", stored)
	}
}

// TestPrintConfigHasCache: the sample configuration shows the key.
func TestPrintConfigHasCache(t *testing.T) {
	c, o, _ := runCLI(t, "--print-config")
	if c != 0 || !strings.Contains(o, `"cache": true`) {
		t.Fatalf("exit %d, sample configuration:\n%s", c, o)
	}
}

// TestCacheWithOutputElsewhere: with -o the folder is beside the output, not beside the source.
func TestCacheWithOutputElsewhere(t *testing.T) {
	html := mediaDeck(t)
	elsewhere := t.TempDir()
	if c, _, e := runCLI(t, html, "--generic", "-o", filepath.Join(elsewhere, "x.html")); c != 0 {
		t.Fatalf("exit %d %s", c, e)
	}
	if n := len(cacheFiles(t, elsewhere)); n != 1 {
		t.Errorf("%d entries beside the output, want 1", n)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(html), cacheFolder)); err == nil {
		t.Error("a cache folder was made beside the source")
	}
}

// TestCacheUnwritableFolder: when the folder cannot be made the conversion still succeeds, gives
// the same output and says that nothing was kept.
func TestCacheUnwritableFolder(t *testing.T) {
	html := mediaDeck(t)
	dir := filepath.Dir(html)
	out := strings.TrimSuffix(html, ".html") + ".standalone.html"
	if c, _, e := runCLI(t, html, "--generic", "--no-cache"); c != 0 {
		t.Fatalf("exit %d %s", c, e)
	}
	want := mustRead(t, out)
	put(t, filepath.Join(dir, cacheFolder), "a file where the folder should be")
	c, o, e := runCLI(t, html, "--generic", "--force")
	if c != 0 {
		t.Fatalf("exit %d %s", c, e)
	}
	if !strings.Contains(o+e, "were not kept for the next run") {
		t.Errorf("the report does not say that the cache could not be written:\n%s%s", o, e)
	}
	if !bytes.Equal(mustRead(t, out), want) {
		t.Error("the output differs when the cache cannot be written")
	}
}

// TestCachePowerPoint: the public decks with pictures give the same file with an empty cache, a
// filled one and none, and a cache that somebody else filled changes nothing: every entry is
// checked against the picture of the deck before it is used.
func TestCachePowerPoint(t *testing.T) {
	t.Setenv("DECKFOLD_BUNDLED_FONTS", "1")
	// (the deck wmf has no bitmap that goes through the encoder; internal/media tests that path)
	for _, name := range []string{"media", "gifs"} {
		src, err := filepath.Abs(filepath.Join("..", "..", "fixtures", "pptx", "src", name+".pptx"))
		if err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		out := filepath.Join(dir, "deck.html")
		convert := func(extra ...string) (string, []byte) {
			t.Helper()
			c, o, e := runCLI(t, append([]string{src, "-o", out, "--force"}, extra...)...)
			if c != 0 {
				t.Fatalf("%s: exit %d %s", name, c, e)
			}
			return o, mustRead(t, out)
		}
		_, none := convert("--no-cache")
		_, cold := convert()
		files := cacheFiles(t, dir)
		if len(files) == 0 {
			t.Fatalf("%s: the deck filled no cache entry, so this test shows nothing for it", name)
		}
		report, warm := convert()
		if !bytes.Equal(none, cold) || !bytes.Equal(none, warm) {
			t.Errorf("%s: outputs differ: no cache %d bytes, empty cache %d, filled cache %d", name, len(none), len(cold), len(warm))
		}
		if !strings.Contains(report, " saved by "+cacheFolder) {
			t.Errorf("%s: the second conversion does not say what came from the cache:\n%s", name, report)
		}
		// somebody else's cache: every entry holds the content of another one, or is no picture
		first := mustRead(t, files[0])
		for i, f := range files {
			content := []byte("<script>alert(1)</script>")
			if i > 0 {
				content = first
			} else if len(files) > 1 {
				content = mustRead(t, files[1])
			}
			if err := os.WriteFile(f, content, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		report, tampered := convert()
		if !bytes.Equal(none, tampered) {
			t.Errorf("%s: a cache with foreign entries changed the output", name)
		}
		if strings.Contains(report, " saved by "+cacheFolder) && len(files) == 1 {
			t.Errorf("%s: a foreign entry counted as used:\n%s", name, report)
		}
	}
}
