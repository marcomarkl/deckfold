package keynote

import (
	"fmt"
	"os"
	"reflect"
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

func TestContent(t *testing.T) {
	files := map[string]string{
		"assets/header.json": `{"slideList":["A","B","C"],"slideCount":3}`,
		// one event: the transition; its texts show from the start, in the order of the export
		"assets/A/A.json": `{"events":[{"effects":[{"type":"transition"}],"accessibility":[` + acc("Lower text", 300) + `,` + acc("Upper text", 20) + `]}]}`,
		// builds, every event listing all the text shown so far; only as JSONP, as on file://
		"assets/B/B.jsonp": `local_slide({"name":"B","json":{"events":[` +
			`{"effects":[{"type":"build"}],"accessibility":[` + acc("Title", 20) + `]},` +
			`{"effects":[{"type":"build"}],"accessibility":[` + acc("Title", 20) + `,` + acc("First", 100) + `]},` +
			`{"effects":[{"type":"transition"}],"accessibility":[` + acc("Title", 20) + `,` + acc("First", 100) + `,` + acc("Second", 200) + `]}]}})`,
		// builds, every event listing only what it adds; the same text twice at two places
		"assets/C/C.json": `{"events":[` +
			`{"effects":[{"type":"build"}],"accessibility":[` + acc("Same", 20) + `]},` +
			`{"effects":[{"type":"transition"}],"accessibility":[` + acc("Same", 120) + `]}]}`,
	}
	meta := adapter.Meta{Slides: []adapter.Slide{{Steps: 1}, {Steps: 3}, {Steps: 2}}}
	c, _, err := Adapter{}.Content(source(files), meta)
	if err != nil {
		t.Fatal(err)
	}
	got := adapter.Finish(c, meta)
	want := []adapter.SlideContent{
		{Blocks: []adapter.Block{{Role: "text", Text: "Lower text"}, {Role: "text", Text: "Upper text"}}},
		{Blocks: []adapter.Block{{Role: "text", Text: "Title"}, {Role: "text", Text: "First", Step: 1}, {Role: "text", Text: "Second", Step: 2}}},
		{Blocks: []adapter.Block{{Role: "text", Text: "Same"}, {Role: "text", Text: "Same", Step: 1}}},
	}
	if !reflect.DeepEqual(got.Slides, want) {
		t.Errorf("got  %+v\nwant %+v", got.Slides, want)
	}
}

// A slide whose JSON is missing is an error, not a slide without text.
func TestContentMissingSlide(t *testing.T) {
	files := map[string]string{"assets/header.json": `{"slideList":["A"]}`}
	if _, _, err := (Adapter{}).Content(source(files), adapter.Meta{Slides: []adapter.Slide{{Steps: 1}}}); err == nil {
		t.Error("expected an error")
	}
}
