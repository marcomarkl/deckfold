package main

import (
	"bytes"
	"deckfold/internal/config"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"deckfold/internal/htmlscan"
)

func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func put(t *testing.T, p, s string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExitCodes(t *testing.T) {
	dir := t.TempDir()
	html := filepath.Join(dir, "plain.html")
	put(t, html, `<!doctype html><html><head><meta charset="utf-8"><title>P</title></head><body><img src="ok.png"></body></html>`)
	put(t, filepath.Join(dir, "ok.png"), "\x89PNG")

	if c, _, e := runCLI(t, html); c != 2 || !strings.Contains(e, "format not recognized") {
		t.Fatalf("unknown format: exit %d, %s", c, e)
	}
	if c, _, e := runCLI(t, html, "--generic"); c != 0 {
		t.Fatalf("--generic: exit %d, %s", c, e)
	}
	out := filepath.Join(dir, "plain.standalone.html")
	if _, err := os.Stat(out); err != nil {
		t.Fatal("default output name missing")
	}
	if c, _, e := runCLI(t, html, "--generic"); c != 3 || !strings.Contains(e, "--force") {
		t.Fatalf("existing output: exit %d, %s", c, e)
	}
	if c, _, _ := runCLI(t, html, "--generic", "--force"); c != 0 {
		t.Fatal("--force")
	}
	if c, _, e := runCLI(t, html, "--generic", "-o", html); c != 3 || !strings.Contains(e, "overwrite the input") {
		t.Fatalf("overwriting the input: exit %d, %s", c, e)
	}
	// Missing asset
	miss := filepath.Join(dir, "miss.html")
	put(t, miss, `<html><head><link rel="stylesheet" href="missing.css"></head><body><img src="missing.png"></body></html>`)
	c, _, e := runCLI(t, miss, "--generic")
	if c != 1 || !strings.Contains(e, "missing.png") || !strings.Contains(e, "missing.css") || !strings.Contains(e, "miss.html") {
		t.Fatalf("missing asset: exit %d, %s", c, e)
	}
	c, o, _ := runCLI(t, miss, "--generic", "--lenient")
	if c != 0 || !strings.Contains(o, "not embedded") {
		t.Fatalf("--lenient: exit %d, %s", c, o)
	}
	// File outside the input folder
	sub := filepath.Join(dir, "deck")
	put(t, filepath.Join(sub, "index.html"), `<html><body><img src="../ok.png"></body></html>`)
	if c, _, e := runCLI(t, sub, "--generic"); c != 1 || !strings.Contains(e, "outside the input folder") {
		t.Fatalf("../: exit %d, %s", c, e)
	}
	if c, _, e := runCLI(t, sub, "--generic", "--allow-outside", dir, "-o", filepath.Join(dir, "x.html")); c != 0 {
		t.Fatalf("--allow-outside: exit %d, %s", c, e)
	}
	// Already converted, and --refresh
	if c, _, e := runCLI(t, out, "--generic", "-o", filepath.Join(dir, "y.html")); c != 3 || !strings.Contains(e, "--refresh") {
		t.Fatalf("already converted: exit %d, %s", c, e)
	}
	before, _ := os.ReadFile(out)
	if c, o, e := runCLI(t, out, "--refresh", "--notes-font-size", "40"); c != 0 || !strings.Contains(o, "runtime updated") {
		t.Fatalf("--refresh: exit %d, %s %s", c, o, e)
	}
	after, _ := os.ReadFile(out)
	if !bytes.Contains(after, []byte(`"notesFontSize":40`)) {
		t.Fatal("--refresh did not apply the new defaults")
	}
	// Everything outside the runtime block stays the same
	strip := func(b []byte) []byte {
		i := bytes.Index(b, []byte("<!--deckfold:runtime:begin-->"))
		j := bytes.Index(b, []byte("<!--deckfold:runtime:end-->"))
		return append(append([]byte{}, b[:i]...), b[j:]...)
	}
	if !bytes.Equal(strip(before), strip(after)) {
		t.Fatal("--refresh changed content outside the runtime block")
	}
	// Configuration error
	cfg := filepath.Join(dir, "c.json")
	put(t, cfg, `{"presenter":{"elements":["x"]}}`)
	if c, _, _ := runCLI(t, html, "--generic", "--force", "--config", cfg); c != 3 {
		t.Fatal("invalid configuration")
	}
}

func TestDuplicateIncludesRemoved(t *testing.T) {
	dir := t.TempDir()
	put(t, filepath.Join(dir, "a.css"), "body{background:url(bg.png)}")
	put(t, filepath.Join(dir, "bg.png"), "PNGDATA")
	put(t, filepath.Join(dir, "x.js"), "window.x=(window.x||0)+1")
	html := filepath.Join(dir, "d.html")
	put(t, html, `<html><head><link rel=stylesheet href="a.css"><link rel="stylesheet" href="./a.css"><script src="x.js"></script><script src="X.js"></script></head><body><img src="bg.png"><img src="./BG.png"></body></html>`)
	c, o, e := runCLI(t, html, "--generic")
	if c != 0 {
		t.Fatalf("exit %d %s", c, e)
	}
	if strings.Count(o, "duplicate") != 2 || !strings.Contains(o, "ignoring upper and lower case") {
		t.Fatalf("report: %s", o)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "d.standalone.html"))
	count := map[string]int{}
	for _, tg := range htmlscan.Scan(b).Tags {
		if tg.Name == "script" {
			count[tg.Value("type")]++
		}
	}
	if count["text/df-css"] != 1 || count["text/df-deferred"] != 1 {
		t.Fatalf("stylesheets/scripts after deduplication: %v", count)
	}
	// bg.png is referenced twice and stored once; plus x.js
	if count["application/octet-stream"] != 2 {
		t.Fatalf("%d asset blocks instead of 2 (png, js)", count["application/octet-stream"])
	}
}

// ---------- Golden tests on the local corpus ----------
//
// The corpus lives in testdata/ and is not part of the repository (it holds private decks).
// Expected layout: testdata/claude/*.html (claude.ai decks) and testdata/keynote/<export>/
// (Keynote HTML exports). Missing corpus entries are skipped.

var extRef = regexp.MustCompile(`(?i)(?:src|href|poster|data|srcset)\s*=\s*["']?\s*(?:https?:)?//`)
var cssExt = regexp.MustCompile(`(?i)url\(\s*["']?(?:https?:)?//`)

func checkGolden(t *testing.T, src, out []byte) {
	t.Helper()
	d := htmlscan.Scan(out)
	ids := map[string]int{}
	for _, tg := range d.Tags {
		if tg.Name == "script" && tg.Value("type") == "application/octet-stream" {
			ids[tg.Value("id")]++
			continue
		}
		if tg.Name == "iframe" || tg.Name == "a" {
			continue // live embeds are reported, links stay links
		}
		raw := out[tg.Start:tg.End]
		if extRef.Match(raw) {
			t.Errorf("external reference: %s", raw)
		}
		if tg.Name == "script" && tg.Value("type") == "text/df-css" && cssExt.Match(d.Content(tg)) {
			t.Errorf("external url() in embedded CSS")
		}
	}
	for id, n := range ids {
		if n != 1 {
			t.Errorf("asset block %s appears %d times", id, n)
		}
	}
	// No two blocks with the same content (deduplication)
	seen := map[string]bool{}
	for _, tg := range d.Tags {
		if tg.Name == "script" && tg.Value("type") == "application/octet-stream" && strings.HasSuffix(tg.Value("id"), "-0") && tg.Value("data-parts") == "1" {
			c := string(d.Content(tg))
			if seen[c] {
				t.Errorf("duplicate asset content in %s", tg.Value("id"))
			}
			seen[c] = true
		}
	}
	// Inline scripts of the source (player code, deck JSON) are byte-identical
	sd := htmlscan.Scan(src)
	var srcScripts, outScripts []string
	for _, tg := range sd.Tags {
		if tg.Name == "script" && tg.Value("src") == "" {
			srcScripts = append(srcScripts, string(sd.Content(tg)))
		}
	}
	for _, tg := range d.Tags {
		if tg.Name == "script" && tg.Value("data-df-src") == "" && tg.Value("id") != "df-runtime" && tg.Value("id") != "df-config" && tg.Value("id") != "df-meta" && tg.Value("id") != "df-content" &&
			tg.Value("type") != "application/octet-stream" && tg.Value("type") != "text/df-css" {
			outScripts = append(outScripts, string(d.Content(tg)))
		}
	}
	if len(srcScripts) != len(outScripts) {
		t.Fatalf("%d inline scripts in the source, %d in the output", len(srcScripts), len(outScripts))
	}
	for i := range srcScripts {
		if srcScripts[i] != outScripts[i] {
			t.Errorf("inline script %d changed", i)
		}
	}
	if !bytes.Contains(out[:1024], []byte("charset")) {
		t.Error("charset declaration is not within the first 1024 bytes")
	}
}

func assetBytes(t *testing.T, out []byte, hash string) []byte {
	t.Helper()
	d := htmlscan.Scan(out)
	var b []byte
	for i := 0; ; i++ {
		var found *htmlscan.Tag
		for _, tg := range d.Tags {
			if tg.Value("id") == "df-a-"+hash+"-"+strconv.Itoa(i) {
				found = tg
			}
		}
		if found == nil {
			break
		}
		p, err := base64.StdEncoding.DecodeString(string(d.Content(found)))
		if err != nil {
			t.Fatal(err)
		}
		b = append(b, p...)
	}
	return b
}

func corpus(t *testing.T, pattern string) []string {
	t.Helper()
	m, _ := filepath.Glob(filepath.Join("..", "..", "testdata", pattern))
	if len(m) == 0 {
		t.Skip("local test corpus not present (testdata/ is not part of the repository)")
	}
	return m
}

func TestGoldenKeynote(t *testing.T) {
	for _, in := range corpus(t, filepath.Join("keynote", "*", "assets", "player", "main.js")) {
		in = filepath.Dir(filepath.Dir(filepath.Dir(in)))
		t.Run(filepath.Base(in), func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "k.html")
			if c, o, e := runCLI(t, in, "-o", out); c != 0 {
				t.Fatalf("exit %d %s %s", c, o, e)
			}
			src, _ := os.ReadFile(filepath.Join(in, "index.html"))
			b, _ := os.ReadFile(out)
			checkGolden(t, src, b)
			// Player code is embedded byte-identical (found through the shim table)
			d := htmlscan.Scan(b)
			var meta struct {
				Shim map[string]string `json:"shim"`
				Deck struct {
					Slides []struct {
						Thumb string `json:"thumb"`
					} `json:"slides"`
				} `json:"deck"`
			}
			for _, tg := range d.Tags {
				if tg.Value("id") == "df-meta" {
					json.Unmarshal(d.Content(tg), &meta)
				}
			}
			want, _ := os.ReadFile(filepath.Join(in, "assets", "player", "main.js"))
			if h := meta.Shim["assets/player/main.js"]; h == "" || !bytes.Equal(assetBytes(t, b, h), want) {
				t.Fatal("main.js is not embedded byte-identical")
			}
			var header struct {
				SlideList []string `json:"slideList"`
			}
			hb, _ := os.ReadFile(filepath.Join(in, "assets", "header.json"))
			json.Unmarshal(hb, &header)
			if len(meta.Deck.Slides) != len(header.SlideList) || len(meta.Deck.Slides) == 0 || meta.Deck.Slides[0].Thumb == "" {
				t.Fatalf("slides/thumbnails: %d of %d", len(meta.Deck.Slides), len(header.SlideList))
			}
		})
	}
}

func TestGoldenClaude(t *testing.T) {
	files := corpus(t, filepath.Join("claude", "*.html"))
	if os.Getenv("DECKFOLD_OFFLINE") != "" {
		t.Skip("needs network access for remote fonts")
	}
	for _, in := range files {
		t.Run(filepath.Base(in), func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "c.html")
			c, o, e := runCLI(t, in, "-o", out)
			if c != 0 {
				t.Fatalf("exit %d %s %s", c, o, e)
			}
			src, _ := os.ReadFile(in)
			b, _ := os.ReadFile(out)
			checkGolden(t, src, b)
		})
	}
}

// A file made by an older version keeps its own values on --refresh, while fields added since then
// get their defaults (the layout of the presenter view), targetMinutes becomes targetSeconds, and
// actions retired since then are dropped (the diagnostics key of the v0.4 drafts), so that the
// result passes validation.
func TestRefreshKeepsOldDefaultsAndAddsNew(t *testing.T) {
	old := config.Default()
	old.Presenter.NotesFontSize = 30
	old.Presenter.Layout = config.Layout{}
	old.Keys["diagnostics"] = []string{"D"}
	old.Keys["next"] = []string{"n"}
	delete(old.Keys, "find") // a file from before the search existed
	old.Presenter.TargetMinutes = 20
	c := mergeKnown(config.Default(), old)
	if c.Presenter.TargetSeconds != 1200 || c.Presenter.TargetMinutes != 0 {
		t.Errorf("target: %d s, %d min; want 1200 s", c.Presenter.TargetSeconds, c.Presenter.TargetMinutes)
	}
	if c.Presenter.NotesFontSize != 30 || c.Keys["next"][0] != "n" {
		t.Errorf("old values lost: notes %d, next %v", c.Presenter.NotesFontSize, c.Keys["next"])
	}
	if !reflect.DeepEqual(c.Keys["find"], config.Default().Keys["find"]) || len(c.Keys["find"]) == 0 {
		t.Errorf("find %q, want the default %q for a file that does not know the action", c.Keys["find"], config.Default().Keys["find"])
	}
	if _, ok := c.Keys["diagnostics"]; ok {
		t.Error("the retired diagnostics key is still there after --refresh")
	}
	if c.Presenter.Layout != config.Default().Presenter.Layout {
		t.Errorf("layout %+v, want the default", c.Presenter.Layout)
	}
	if err := c.Validate(); err != nil {
		t.Error(err)
	}
	// a file with the default keys of v0.3 (Enter moved on) gets the current default; a list the
	// file chose itself stays
	old = config.Default()
	old.Keys["next"] = []string{"ArrowRight", "ArrowDown", "PageDown", " ", "Enter"}
	old.Keys["prev"] = []string{"ArrowLeft", "Enter"}
	c = mergeKnown(config.Default(), old)
	if !reflect.DeepEqual(c.Keys["next"], config.Default().Keys["next"]) {
		t.Errorf("next %q, want the current default %q", c.Keys["next"], config.Default().Keys["next"])
	}
	if !reflect.DeepEqual(c.Keys["prev"], []string{"ArrowLeft", "Enter"}) {
		t.Errorf("prev %q, want the file's own list", c.Keys["prev"])
	}
	// the element list of v0.3 gets the audience pane; a list the file chose itself stays. The
	// layout of v0.3 (main and side only) keeps its shares and gets the default for the audience.
	old = config.Default()
	old.Presenter.Elements = []string{"current", "next", "notes", "time", "overview"}
	old.Presenter.Layout = config.Layout{Main: 0.7, Side: 0.3}
	c = mergeKnown(config.Default(), old)
	if !reflect.DeepEqual(c.Presenter.Elements, config.Default().Presenter.Elements) {
		t.Errorf("elements %q, want the current default", c.Presenter.Elements)
	}
	if want := (config.Layout{Main: 0.7, Side: 0.3, Audience: config.Default().Presenter.Layout.Audience}); c.Presenter.Layout != want {
		t.Errorf("layout %+v, want %+v", c.Presenter.Layout, want)
	}
	old.Presenter.Elements = []string{"current", "notes"}
	c = mergeKnown(config.Default(), old)
	if !reflect.DeepEqual(c.Presenter.Elements, []string{"current", "notes"}) {
		t.Errorf("elements %q, want the file's own list", c.Presenter.Elements)
	}
	if err := c.Validate(); err != nil {
		t.Error(err)
	}
}
