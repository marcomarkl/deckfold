package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"deckfold/internal/adapter"
	"deckfold/internal/htmlscan"
	"deckfold/internal/pptx/pptxgen"
)

// convertDeck converts a generated PowerPoint deck with the bundled fonts and returns the output.
func convertDeck(t *testing.T, d *pptxgen.Deck) []byte {
	t.Helper()
	t.Setenv("DECKFOLD_BUNDLED_FONTS", "1")
	dir := t.TempDir()
	src, out := filepath.Join(dir, "in.pptx"), filepath.Join(dir, "out.html")
	if err := d.Write(src); err != nil {
		t.Fatal(err)
	}
	if c, o, e := runCLI(t, src, "-o", out); c != 0 {
		t.Fatalf("exit %d %s %s", c, o, e)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// blocksOf returns the script elements with the given id.
func blocksOf(d *htmlscan.Doc, id string) []*htmlscan.Tag {
	var out []*htmlscan.Tag
	for _, tg := range d.Tags {
		if tg.Name == "script" && tg.Value("id") == id {
			out = append(out, tg)
		}
	}
	return out
}

// readContent returns the content model and the number of slides in the deck metadata.
func readContent(t *testing.T, out []byte) (adapter.Content, int) {
	t.Helper()
	d := htmlscan.Scan(out)
	cb, mb := blocksOf(d, "df-content"), blocksOf(d, "df-meta")
	if len(cb) != 1 || len(mb) != 1 {
		t.Fatalf("%d content blocks and %d metadata blocks, want one of each", len(cb), len(mb))
	}
	if cb[0].Value("type") != "application/json" {
		t.Errorf("the content block has type %q", cb[0].Value("type"))
	}
	var c adapter.Content
	if err := json.Unmarshal(d.Content(cb[0]), &c); err != nil {
		t.Fatalf("the content block is not JSON: %v", err)
	}
	var meta struct {
		Deck struct {
			Slides []json.RawMessage `json:"slides"`
		} `json:"deck"`
	}
	if err := json.Unmarshal(d.Content(mb[0]), &meta); err != nil {
		t.Fatal(err)
	}
	return c, len(meta.Deck.Slides)
}

// specContent is the generator's expectation in the form of the content model.
func specContent(spec pptxgen.ContentSpec) []adapter.SlideContent {
	out := make([]adapter.SlideContent, len(spec.Slides))
	for i, s := range spec.Slides {
		out[i] = adapter.SlideContent{Title: s.Title, Blocks: []adapter.Block{}}
		for _, b := range s.Blocks {
			nb := adapter.Block{Role: b.Role, Text: b.Text, Step: b.Step, Level: b.Level}
			if b.Jump != nil {
				nb.Jump = &adapter.Jump{Slide: b.Jump.Slide, To: b.Jump.To}
			}
			out[i].Blocks = append(out[i].Blocks, nb)
		}
	}
	return out
}

// The output of a deck holds one content block next to the metadata, with an entry for every
// shown slide, and it says what the generator put on the slides.
func TestContentBlockInOutput(t *testing.T) {
	d, spec := pptxgen.ContentWithSpec()
	c, slides := readContent(t, convertDeck(t, d))
	if c.Version != adapter.ContentVersion || len(c.Slides) != slides || slides != len(spec.Slides) {
		t.Fatalf("version %d, %d slides in the content, %d in the metadata, %d expected", c.Version, len(c.Slides), slides, len(spec.Slides))
	}
	want := specContent(spec)
	for i := range want {
		if !reflect.DeepEqual(c.Slides[i], want[i]) {
			g, _ := json.Marshal(c.Slides[i])
			w, _ := json.Marshal(want[i])
			t.Errorf("slide %d:\n got %s\nwant %s", i+1, g, w)
		}
	}
}

// Text from a hostile deck stays data in the content block: the block reads back with the text
// as it was written, and it adds one script element to the output and nothing else.
func TestContentBlockHostile(t *testing.T) {
	out := convertDeck(t, pptxgen.Hostile())
	c, _ := readContent(t, out)
	const text = `<img src=x onerror=alert(1)> & "quotes" 'apostrophes'`
	found := false
	for _, s := range c.Slides {
		for _, b := range s.Blocks {
			if b.Text == text {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("the slide text is not in the content block as written: %+v", c.Slides)
	}
	d := htmlscan.Scan(out)
	cb := blocksOf(d, "df-content")[0]
	without := append(append([]byte{}, out[:cb.Start]...), out[cb.CloseEnd:]...)
	count := func(doc *htmlscan.Doc) map[string]int {
		m := map[string]int{}
		for _, tg := range doc.Tags {
			m[tg.Name]++
		}
		return m
	}
	with, base := count(d), count(htmlscan.Scan(without))
	base["script"]++
	if !reflect.DeepEqual(with, base) {
		t.Errorf("the content block changes the elements of the output:\n with    %v\n without %v (+1 script)", with, base)
	}
}

// writeOut converts a generated deck into dir and returns the path of the output.
func writeOut(t *testing.T, dir string, d *pptxgen.Deck) string {
	t.Helper()
	p := filepath.Join(dir, "deck.html")
	if err := os.WriteFile(p, convertDeck(t, d), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// --content as JSON: the slides, titles, notes and blocks the generator put into the deck.
func TestContentJSON(t *testing.T) {
	dir := t.TempDir()
	d, spec := pptxgen.ContentWithSpec()
	deckFile, target := writeOut(t, dir, d), filepath.Join(dir, "deck.json")
	if c, o, e := runCLI(t, deckFile, "--content", target); c != 0 {
		t.Fatalf("exit %d %s %s", c, o, e)
	}
	b, _ := os.ReadFile(target)
	var got struct {
		Version int    `json:"version"`
		Format  string `json:"format"`
		Title   string `json:"title"`
		Width   int    `json:"width"`
		Slides  []struct {
			Title  string          `json:"title"`
			Notes  string          `json:"notes"`
			Blocks []adapter.Block `json:"blocks"`
		} `json:"slides"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Version != adapter.ContentVersion || got.Format != "pptx" || got.Title != "Content model deck" || got.Width != 1280 || len(got.Slides) != len(spec.Slides) {
		t.Fatalf("header: %+v", got)
	}
	want := specContent(spec)
	for i, s := range got.Slides {
		if s.Title != want[i].Title || s.Notes != spec.Slides[i].Notes || !reflect.DeepEqual(s.Blocks, want[i].Blocks) {
			g, _ := json.Marshal(s)
			t.Errorf("slide %d: %s", i+1, g)
		}
	}
	// an existing file is only overwritten with --force
	if c, _, _ := runCLI(t, deckFile, "--content", target); c != 3 {
		t.Errorf("writing over an existing file: exit %d, want 3", c)
	}
	if c, _, e := runCLI(t, deckFile, "--content", target, "--force"); c != 0 {
		t.Errorf("--force: exit %d %s", c, e)
	}
}

// --content as Markdown, compared with a file that was read and checked in
// (DECKFOLD_UPDATE_GOLDEN=1 writes it again after a change that is meant).
func TestContentMarkdown(t *testing.T) {
	dir := t.TempDir()
	deckFile, target := writeOut(t, dir, pptxgen.Content()), filepath.Join(dir, "deck.md")
	if c, o, e := runCLI(t, deckFile, "--content", target); c != 0 {
		t.Fatalf("exit %d %s %s", c, o, e)
	}
	got, _ := os.ReadFile(target)
	golden := filepath.Join("..", "..", "fixtures", "pptx", "src", "content.md")
	if os.Getenv("DECKFOLD_UPDATE_GOLDEN") != "" {
		os.MkdirAll(filepath.Dir(golden), 0o755)
		os.WriteFile(golden, got, 0o644)
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("Markdown differs from %s:\n%s", golden, got)
	}
}

// The Markdown of a hostile deck: no slide text opens an element, a link or a heading.
func TestContentMarkdownHostile(t *testing.T) {
	dir := t.TempDir()
	deckFile, target := writeOut(t, dir, pptxgen.Hostile()), filepath.Join(dir, "deck.md")
	if c, o, e := runCLI(t, deckFile, "--content", target); c != 0 {
		t.Fatalf("exit %d %s %s", c, o, e)
	}
	got, _ := os.ReadFile(target)
	headings := 0
	for i, line := range strings.Split(string(got), "\n") {
		if strings.HasPrefix(line, "#") {
			headings++
		}
		for k := 0; k < len(line); k++ {
			// "<" and "[" only escaped; ">" only as the quote mark of the notes or escaped
			if (line[k] == '<' || line[k] == '[' || (line[k] == '>' && k > 0)) && (k == 0 || line[k-1] != '\\') {
				t.Errorf("line %d: unescaped %q in %q", i+1, line[k], line)
			}
		}
	}
	// the deck title and one slide: two headings, none from slide text
	if headings != 2 {
		t.Errorf("%d headings, want 2:\n%s", headings, got)
	}
	if !strings.Contains(string(got), `\<img src=x onerror=alert(1)\>`) {
		t.Errorf("the slide text is missing:\n%s", got)
	}
}

// --content needs a deckfold output, a target named .json or .md, and a file with a model.
func TestContentErrors(t *testing.T) {
	dir := t.TempDir()
	deckFile := writeOut(t, dir, pptxgen.Basic())
	plain := filepath.Join(dir, "plain.html")
	put(t, plain, "<!doctype html><html><head><meta charset=\"utf-8\"></head><body>no deck</body></html>")
	for _, c := range []struct {
		name string
		args []string
		want int
	}{
		{"with --refresh", []string{deckFile, "--content", filepath.Join(dir, "a.json"), "--refresh"}, 3},
		{"another extension", []string{deckFile, "--content", filepath.Join(dir, "a.txt")}, 3},
		{"not a deckfold output", []string{plain, "--content", filepath.Join(dir, "b.json")}, 3},
		{"missing input", []string{filepath.Join(dir, "none.html"), "--content", filepath.Join(dir, "c.json")}, 3},
	} {
		if code, _, _ := runCLI(t, c.args...); code != c.want {
			t.Errorf("%s: exit %d, want %d", c.name, code, c.want)
		}
	}
	// a file from before the content model: exit 1 with the way to get one
	out, _ := os.ReadFile(deckFile)
	d := htmlscan.Scan(out)
	cb := blocksOf(d, "df-content")[0]
	old := filepath.Join(dir, "old.html")
	os.WriteFile(old, append(append([]byte{}, out[:cb.Start]...), out[cb.CloseEnd:]...), 0o644)
	code, _, e := runCLI(t, old, "--content", filepath.Join(dir, "d.json"))
	if code != 1 || !strings.Contains(e, "convert the presentation again") {
		t.Errorf("file without a model: exit %d, %q", code, e)
	}
	// a model of a later version
	newer := filepath.Join(dir, "newer.html")
	os.WriteFile(newer, []byte(strings.Replace(string(out), `id="df-content">{"version":1,`, `id="df-content">{"version":2,`, 1)), 0o644)
	code, _, e = runCLI(t, newer, "--content", filepath.Join(dir, "e.json"))
	if code != 1 || !strings.Contains(e, "newer deckfold") {
		t.Errorf("newer model: exit %d, %q", code, e)
	}
}
