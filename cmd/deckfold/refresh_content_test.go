package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"deckfold/internal/htmlscan"
	"deckfold/internal/inject"
	"deckfold/internal/pptx/pptxgen"
)

// The golden tests for claude.ai and Keynote need the private corpus and are skipped without it.
// These two decks carry only the structure the adapters read, with made-up text, so that
// --refresh is checked for all three formats wherever the tests run.

const claudeDeck = `<!doctype html><html><head><meta charset="utf-8"><meta name="hz:kind" content="deck"><title>Made-up deck</title></head><body>
<section class="deck-slide" data-label="First" data-speaker-notes="notes of the first slide">
 <div data-frame-id="a1"><div data-text-path="0">Shown from the start</div></div>
 <div data-frame-id="a2"><div data-text-path="1"><ul><li>Shown on the first click</li></ul></div></div>
</section>
<section class="deck-slide" data-label="Second"><div data-text-path="0">The second slide</div></section>
<script type="application/json" id="deck-motion">{"w":1920,"h":1080,"slides":[{"steps":[{"hidden":["a2"]},{"hidden":[]}]},{"steps":[{"hidden":[]}]}]}</script>
</body></html>`

func writeClaudeDeck(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "claude.html")
	put(t, p, claudeDeck)
	return p
}

func writeKeynoteDeck(t *testing.T, dir string) string {
	t.Helper()
	root := filepath.Join(dir, "keynote")
	put(t, filepath.Join(root, "index.html"), `<!doctype html><html><head><meta charset="utf-8"><title>Made-up export</title></head><body><script src="assets/player/main.js"></script></body></html>`)
	put(t, filepath.Join(root, "assets", "player", "main.js"), "")
	header := `{"slideList":["A","B"],"slideCount":2,"slideWidth":720,"slideHeight":405,"title":"Made-up export","creator":"test","majorVersion":1,"minorVersion":2}`
	put(t, filepath.Join(root, "assets", "header.json"), header)
	put(t, filepath.Join(root, "assets", "header.jsonp"), "local_header("+header+")")
	for id, text := range map[string]string{"A": "Text of the first slide", "B": "Text of the second slide"} {
		js := fmt.Sprintf(`{"events":[{"effects":[{"type":"transition"}],"accessibility":[{"text":%q,"targetRectangle":{"x":1,"y":2,"width":3,"height":4}}]}]}`, text)
		put(t, filepath.Join(root, "assets", id, id+".json"), js)
		put(t, filepath.Join(root, "assets", id, id+".jsonp"), fmt.Sprintf(`local_slide({"name":%q,"json":%s})`, id, js))
	}
	return root
}

// contentBlock returns the raw content block of an output, nil when it has none.
func contentBlock(out []byte) []byte {
	d := htmlscan.Scan(out)
	if b := blocksOf(d, "df-content"); len(b) == 1 {
		return out[b[0].Start:b[0].CloseEnd]
	}
	return nil
}

// withoutContent returns the output without its content block: a file from before the model.
func withoutContent(t *testing.T, out []byte) []byte {
	t.Helper()
	d := htmlscan.Scan(out)
	b := blocksOf(d, "df-content")
	if len(b) != 1 {
		t.Fatalf("%d content blocks", len(b))
	}
	return append(append([]byte{}, out[:b[0].Start]...), out[b[0].CloseEnd:]...)
}

func refreshFile(t *testing.T, dir, name string, data []byte) (out []byte, stdout, stderr string) {
	t.Helper()
	in, target := filepath.Join(dir, name+"-in.html"), filepath.Join(dir, name+"-out.html")
	if err := os.WriteFile(in, data, 0o644); err != nil {
		t.Fatal(err)
	}
	code, o, e := runCLI(t, "--refresh", in, "-o", target, "--force")
	if code != 0 {
		t.Fatalf("--refresh: exit %d %s %s", code, o, e)
	}
	out, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	return out, o, e
}

// --refresh gives a file from before the content model the block a fresh conversion writes, for
// every format, and a second --refresh changes nothing.
func TestRefreshAddsContent(t *testing.T) {
	t.Setenv("DECKFOLD_BUNDLED_FONTS", "1")
	dir := t.TempDir()
	pptxSrc := filepath.Join(dir, "content.pptx")
	if err := pptxgen.Content().Write(pptxSrc); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, input string }{
		{"claude", writeClaudeDeck(t, dir)}, {"keynote", writeKeynoteDeck(t, dir)}, {"pptx", pptxSrc},
	} {
		t.Run(c.name, func(t *testing.T) {
			fresh := filepath.Join(dir, c.name+"-fresh.html")
			if code, o, e := runCLI(t, c.input, "-o", fresh); code != 0 {
				t.Fatalf("exit %d %s %s", code, o, e)
			}
			out, _ := os.ReadFile(fresh)
			want := contentBlock(out)
			if want == nil {
				t.Fatal("the fresh conversion has no content block")
			}
			once, _, e := refreshFile(t, dir, c.name+"-1", withoutContent(t, out))
			if got := contentBlock(once); !bytes.Equal(got, want) {
				t.Errorf("after --refresh:\n got %s\nwant %s\n%s", got, want, e)
			}
			// the block sits right after the metadata block
			d := htmlscan.Scan(once)
			if m, cb := blocksOf(d, "df-meta"), blocksOf(d, "df-content"); len(m) != 1 || len(cb) != 1 || string(once[m[0].CloseEnd:cb[0].Start]) != "\n" {
				t.Error("the content block does not follow the metadata block")
			}
			twice, _, _ := refreshFile(t, dir, c.name+"-2", once)
			if !bytes.Equal(twice, once) {
				t.Error("a second --refresh changes the file")
			}
			// a file that has the block keeps one block with the same content
			again, _, _ := refreshFile(t, dir, c.name+"-3", out)
			if got := contentBlock(again); !bytes.Equal(got, want) {
				t.Errorf("--refresh of a file with a block: %s", got)
			}
		})
	}
}

// A PowerPoint output from before the content model has no text blocks in its deck data:
// --refresh updates the runtime, adds no block and says that the presentation has to be
// converted again.
func TestRefreshOldPowerPointOutput(t *testing.T) {
	dir := t.TempDir()
	out := withoutContent(t, convertDeck(t, pptxgen.Content()))
	d := htmlscan.Scan(out)
	data := blocksOf(d, "df-pptx-deck")
	if len(data) != 1 {
		t.Fatalf("%d deck data blocks", len(data))
	}
	var deck map[string]json.RawMessage
	if err := json.Unmarshal(d.Content(data[0]), &deck); err != nil {
		t.Fatal(err)
	}
	deck["version"] = json.RawMessage("1")
	js, _ := json.Marshal(deck)
	old := append(append(append([]byte{}, out[:data[0].ContentStart]...), js...), out[data[0].ContentEnd:]...)
	got, stdout, stderr := refreshFile(t, dir, "old", old)
	if contentBlock(got) != nil {
		t.Error("an old PowerPoint output got a content block")
	}
	if !strings.Contains(stdout+stderr, "convert the presentation again") {
		t.Errorf("no hint to convert again: %q %q", stdout, stderr)
	}
	if !strings.Contains(stdout, "runtime updated") {
		t.Errorf("the runtime was not updated: %q", stdout)
	}
}

// setAsset replaces the data of the single-part asset with the given hash.
func setAsset(t *testing.T, out []byte, hash, data string) []byte {
	t.Helper()
	d := htmlscan.Scan(out)
	b := blocksOf(d, "df-a-"+hash+"-0")
	if len(b) != 1 {
		t.Fatalf("asset %s: %d blocks", hash, len(b))
	}
	return append(append(append([]byte{}, out[:b[0].ContentStart]...), data...), out[b[0].ContentEnd:]...)
}

func shimOf(t *testing.T, out []byte) map[string]string {
	t.Helper()
	d := htmlscan.Scan(out)
	var meta struct {
		Shim map[string]string `json:"shim"`
	}
	if err := json.Unmarshal(d.Content(blocksOf(d, "df-meta")[0]), &meta); err != nil {
		t.Fatal(err)
	}
	return meta.Shim
}

// A file whose slide data is damaged still gets the new runtime; the content block it has stays,
// and a warning says that the slide text could not be read.
func TestRefreshWithDamagedSlideData(t *testing.T) {
	dir := t.TempDir()
	fresh := filepath.Join(dir, "k.html")
	if code, o, e := runCLI(t, writeKeynoteDeck(t, dir), "-o", fresh); code != 0 {
		t.Fatalf("exit %d %s %s", code, o, e)
	}
	out, _ := os.ReadFile(fresh)
	want := contentBlock(out)
	shim := shimOf(t, out)
	damaged := setAsset(t, out, shim["assets/A/A.json"], "!!! not base64 !!!")
	damaged = setAsset(t, damaged, shim["assets/A/A.jsonp"], base64.StdEncoding.EncodeToString([]byte("no jsonp here")))
	got, stdout, stderr := refreshFile(t, dir, "damaged", damaged)
	if !strings.Contains(stdout, "runtime updated") || !strings.Contains(stderr, "slide text could not be read") {
		t.Errorf("stdout %q, stderr %q", stdout, stderr)
	}
	if !bytes.Equal(contentBlock(got), want) {
		t.Error("the content block of the file changed although the slide text could not be read")
	}
	if _, _, err := inject.Find(got); err != nil {
		t.Error(err)
	}
}

// Reading the content of a finished file never touches the file system: a reference resolves
// through the shim table to an embedded asset, or not at all.
func TestOutputSourceReadsOnlyTheFile(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "secret.json")
	put(t, secret, `{"leak":true}`)
	fresh := filepath.Join(dir, "k.html")
	if code, o, e := runCLI(t, writeKeynoteDeck(t, dir), "-o", fresh); code != 0 {
		t.Fatalf("exit %d %s %s", code, o, e)
	}
	out, _ := os.ReadFile(fresh)
	shim := shimOf(t, out)
	src := outputSource(htmlscan.Scan(out), shim)
	for _, ref := range []string{secret, "../secret.json", "secret.json", "assets/../secret.json", "assets/header.json/../../secret.json"} {
		if b, err := src.Load(ref); err == nil {
			t.Errorf("Load(%q) returned %q", ref, b)
		}
	}
	// a shim entry with ".." in its name is a name like any other: it yields the embedded asset
	shim["../secret.json"] = shim["assets/header.json"]
	b, err := outputSource(htmlscan.Scan(out), shim).Load("../secret.json")
	if err != nil || bytes.Contains(b, []byte("leak")) || !bytes.Contains(b, []byte("slideList")) {
		t.Errorf("Load through the shim table: %q, %v", b, err)
	}
	// a hostile header that names a slide outside the export: no slide data, so a warning, and
	// nothing from outside the file in the result
	hostile := `{"slideList":["../secret","B"],"slideCount":2,"slideWidth":720,"slideHeight":405,"majorVersion":1,"minorVersion":2}`
	bad := setAsset(t, out, shimOf(t, out)["assets/header.json"], base64.StdEncoding.EncodeToString([]byte(hostile)))
	got, _, stderr := refreshFile(t, dir, "hostile", bad)
	if bytes.Contains(got, []byte("leak")) || !strings.Contains(stderr, "slide text could not be read") {
		t.Errorf("stderr %q", stderr)
	}
}
