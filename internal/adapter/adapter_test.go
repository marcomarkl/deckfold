package adapter

import "testing"

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
