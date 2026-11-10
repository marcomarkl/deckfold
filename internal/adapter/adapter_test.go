package adapter

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Finish gives every slide of the metadata an entry, takes the title from the slide label, cleans
// the text and drops empty blocks.
func TestFinish(t *testing.T) {
	meta := Meta{Slides: []Slide{{Label: " One  title "}, {}, {Label: "Three"}}}
	c := &Content{Slides: []SlideContent{
		{Blocks: []Block{{Role: RoleText, Text: "  a\n b\t c "}, {Role: RoleText, Text: " \n "}}},
		{Title: "not from the adapter", Blocks: []Block{{Role: RoleItem, Text: "x", Step: 2, Level: 1}}},
	}}
	got := Finish(c, meta)
	if got.Version != ContentVersion || len(got.Slides) != 3 {
		t.Fatalf("version %d, %d slides", got.Version, len(got.Slides))
	}
	if got.Slides[0].Title != "One title" || got.Slides[1].Title != "" || got.Slides[2].Title != "Three" {
		t.Errorf("titles: %q %q %q", got.Slides[0].Title, got.Slides[1].Title, got.Slides[2].Title)
	}
	if b := got.Slides[0].Blocks; len(b) != 1 || b[0].Text != "a b c" {
		t.Errorf("blocks of slide 1: %+v", b)
	}
	if b := got.Slides[1].Blocks; len(b) != 1 || b[0].Step != 2 || b[0].Level != 1 {
		t.Errorf("blocks of slide 2: %+v", b)
	}
	if got.Slides[2].Blocks == nil || len(got.Slides[2].Blocks) != 0 {
		t.Errorf("a slide without text needs an empty list, got %v", got.Slides[2].Blocks)
	}
	if Finish(nil, meta) != nil {
		t.Error("no content must stay no content")
	}
}

// A block whose step is unknown has no "step" in the JSON, and reads back as unknown; every
// other step, 0 and -1 included, is written.
func TestBlockStepJSON(t *testing.T) {
	in := []Block{{Role: RoleText, Text: "a"}, {Role: RoleText, Text: "b", Step: -1}, {Role: RoleText, Text: "c", Step: StepUnknown}, {Role: RoleItem, Text: "d", Step: 2, Level: 1}}
	js, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"role":"text","text":"a","step":0},{"role":"text","text":"b","step":-1},{"role":"text","text":"c"},{"role":"item","text":"d","step":2,"level":1}]`
	if string(js) != want {
		t.Errorf("got  %s\nwant %s", js, want)
	}
	var out []Block
	if err := json.Unmarshal(js, &out); err != nil || !reflect.DeepEqual(out, in) {
		t.Errorf("read back %+v, %v", out, err)
	}
	// the object replacement character is not text
	if got := CleanText("a \uFFFC b"); got != "a b" {
		t.Errorf("CleanText: %q", got)
	}
	if c := Finish(&Content{Slides: []SlideContent{{Blocks: []Block{{Role: RoleText, Text: "\uFFFC"}}}}}, Meta{Slides: []Slide{{}}}); len(c.Slides[0].Blocks) != 0 {
		t.Errorf("a block of an object placeholder only: %+v", c.Slides[0].Blocks)
	}
}
