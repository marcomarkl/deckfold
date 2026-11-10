package keynote

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"deckfold/internal/adapter"
)

// source serves the files of a made-up export from memory.
func source(files map[string]string) adapter.ContentSource {
	return adapter.ContentSource{Load: func(ref string) ([]byte, error) {
		if s, ok := files[ref]; ok {
			return []byte(s), nil
		}
		return nil, fmt.Errorf("%s: %w", ref, os.ErrNotExist)
	}}
}

func acc(text string, y int) string {
	return fmt.Sprintf(`{"text":%q,"targetRectangle":{"x":10,"y":%d,"width":100,"height":20}}`, text, y)
}

func link(url string, y int) string {
	return fmt.Sprintf(`{"url":%q,"targetRectangle":{"x":10,"y":%d,"width":100,"height":20}}`, url, y)
}

func TestContent(t *testing.T) {
	files := map[string]string{
		"assets/header.json": `{"slideList":["A","B","C"],"slideCount":3}`,
		// one event: the transition; the texts in the order of the export; an object placeholder
		"assets/A/A.json": `{"events":[{"effects":[{"type":"transition"}],"accessibility":[` + acc("Lower text", 300) + `,` + acc("Upper text", 20) + `,` + acc("\uFFFC", 500) + `]}]}`,
		// builds; only as JSONP, as on file://. The last event lists the whole slide and gives
		// the order; an event in between leaves text out, and one lists a text of its own.
		"assets/B/B.jsonp": `local_slide({"name":"B","json":{"events":[` +
			`{"effects":[{"type":"buildIn"}],"accessibility":[` + acc("First", 100) + `,` + acc("Title", 20) + `,` + acc("Only early", 400) + `]},` +
			`{"effects":[{"type":"buildIn"}],"accessibility":[` + acc("Title", 20) + `]},` +
			`{"effects":[{"type":"transition"}],"accessibility":[` + acc("Title", 20) + `,` + acc("First", 100) + `,` + acc("Second", 200) + `]}]}})`,
		// links: on the text with the same rectangle; a link on a shape has no text of its own
		// rectangle; a link to a slide that is not in the export is none; the same text twice
		"assets/C/C.json": `{"events":[{"effects":[{"type":"transition"}],"accessibility":[` + acc("Next", 20) + `,` + acc("To A", 60) + `,` + acc("Gone", 100) + `,` + acc("In a shape", 205) + `,` + acc("Next", 300) + `],` +
			`"hyperlinks":[` + link("?slide=next", 20) + `,` + link("?slideid=A", 60) + `,` + link("?slideid=ZZ", 100) + `,` +
			`{"url":"?action=retreat","targetRectangle":{"x":0,"y":200,"width":300,"height":30}}]}]}`,
	}
	meta := adapter.Meta{Slides: []adapter.Slide{{Steps: 1}, {Steps: 3}, {Steps: 1}}}
	c, _, err := Adapter{}.Content(source(files), meta)
	if err != nil {
		t.Fatal(err)
	}
	got := adapter.Finish(c, meta)
	// the export does not tell the step of a text: no block has one
	text := func(s string) adapter.Block { return adapter.Block{Role: "text", Text: s, Step: adapter.StepUnknown} }
	zero := 0
	next, toA := text("Next"), text("To A")
	next.Jump, toA.Jump = &adapter.Jump{To: "next"}, &adapter.Jump{Slide: &zero}
	want := []adapter.SlideContent{
		{Blocks: []adapter.Block{text("Lower text"), text("Upper text")}},
		{Blocks: []adapter.Block{text("Title"), text("First"), text("Second"), text("Only early")}},
		{Blocks: []adapter.Block{next, toA, text("Gone"), text("In a shape"), text("Next")}},
	}
	if !reflect.DeepEqual(got.Slides, want) {
		for i := range want {
			if !reflect.DeepEqual(got.Slides[i], want[i]) {
				g, _ := json.Marshal(got.Slides[i])
				w, _ := json.Marshal(want[i])
				t.Errorf("slide %d:\n got %s\nwant %s", i+1, g, w)
			}
		}
	}
	js, _ := json.Marshal(got.Slides[0].Blocks[0])
	if string(js) != `{"role":"text","text":"Lower text"}` {
		t.Errorf("a Keynote block in the JSON: %s", js)
	}
}

// The deck metadata carries the links for the runtime: rectangle, step and target, per slide.
func TestMetaLinks(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, s string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("assets/header.json", `{"slideList":["A","B"],"slideCount":2,"slideWidth":960,"slideHeight":540,"majorVersion":1,"minorVersion":2}`)
	write("assets/A/A.json", `{"events":[{"effects":[{"type":"buildIn"}],"hyperlinks":[`+link("?slide=last", 20)+`]},`+
		`{"effects":[{"type":"transition"}],"hyperlinks":[`+link("?slide=last", 20)+`,`+link("?slideid=B", 60)+`,`+link("?slideid=ZZ", 90)+`,`+link("https://example.com", 120)+`]}]}`)
	write("assets/B/B.json", `{"events":[{"effects":[{"type":"transition"}]}]}`)
	m, warns, err := Adapter{}.Meta(&adapter.Input{HTMLDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	want := [][]Link{{{Step: 0, X: 10, Y: 20, W: 100, H: 20, To: "last"}, {Step: 1, X: 10, Y: 20, W: 100, H: 20, To: "last"}, {Step: 1, X: 10, Y: 60, W: 100, H: 20, To: "1"}}, nil}
	if !reflect.DeepEqual(m.Extra["links"], want) {
		t.Errorf("links %+v, want %+v", m.Extra["links"], want)
	}
	lost := false
	for _, w := range warns {
		lost = lost || strings.Contains(w, "not in the export")
	}
	if !lost {
		t.Errorf("no warning for the link to a slide that is not in the export: %q", warns)
	}
}

// A slide whose JSON is missing is an error, not a slide without text.
func TestContentMissingSlide(t *testing.T) {
	files := map[string]string{"assets/header.json": `{"slideList":["A"]}`}
	if _, _, err := (Adapter{}).Content(source(files), adapter.Meta{Slides: []adapter.Slide{{Steps: 1}}}); err == nil {
		t.Error("expected an error")
	}
}
