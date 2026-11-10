package claude

import (
	"reflect"
	"testing"

	"deckfold/internal/adapter"
	"deckfold/internal/htmlscan"
)

// A deck with the structure the adapter reads; the text is made up for this test.
const deckHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="hz:kind" content="deck"><title>Deck</title></head><body>
<div class="deck-pager">1 / 2</div>
<section class="deck-slide" data-label="First" data-speaker-notes="say hello">
 <div data-frame-id="a1"><div data-text-path="0">Hello <b>bold</b> world<br>next line</div></div>
 <div data-frame-id="a2"><div data-text-path="1">Intro<ul><li>One</li><li data-frame-id="a3">Two<ul><li>Deep</li></ul></li></ul>Tail</div></div>
 <script>var x = "not text";</script><style>.a { color: red }</style>
 <img alt="A chart" src="x.png">
 <div hidden>not shown</div>
 <div data-frame-id="a4">Never</div>
</section>
<section class="deck-slide" data-label="Second"><div>Plain &amp; simple</div></section>
<script type="application/json" id="deck-motion">{"w":1920,"h":1080,"slides":[{"steps":[{"hidden":["a2","a3","a4"]},{"hidden":["a3","a4"]},{"hidden":["a4"]}]},{"steps":[{"hidden":[]}]}]}</script>
</body></html>`

func TestContent(t *testing.T) {
	src := []byte(deckHTML)
	in := &adapter.Input{Src: src, Doc: htmlscan.Scan(src)}
	meta, _, err := Adapter{}.Meta(in)
	if err != nil {
		t.Fatal(err)
	}
	c, _, err := Adapter{}.Content(adapter.SourceOf(in), meta)
	if err != nil {
		t.Fatal(err)
	}
	got := adapter.Finish(c, meta)
	want := []adapter.SlideContent{
		{Title: "First", Blocks: []adapter.Block{
			{Role: "text", Text: "Hello bold world next line"},
			{Role: "text", Text: "Intro", Step: 1},
			{Role: "item", Text: "One", Step: 1},
			// inside something that appears in step 1 and itself appearing in step 2: step 2
			{Role: "item", Text: "Two", Step: 2},
			{Role: "item", Text: "Deep", Step: 2, Level: 1},
			{Role: "text", Text: "Tail", Step: 1},
			{Role: "alt", Text: "A chart"},
			// hidden in every step: shows in no step
			{Role: "text", Text: "Never", Step: -1},
		}},
		{Title: "Second", Blocks: []adapter.Block{{Role: "text", Text: "Plain & simple"}}},
	}
	if !reflect.DeepEqual(got.Slides, want) {
		for i := range want {
			if i < len(got.Slides) && !reflect.DeepEqual(got.Slides[i], want[i]) {
				t.Errorf("slide %d:\n got %+v\nwant %+v", i+1, got.Slides[i], want[i])
			}
		}
		if len(got.Slides) != len(want) {
			t.Errorf("%d slides, want %d", len(got.Slides), len(want))
		}
	}
}

// A document whose slide sections and deck JSON disagree is an unknown structure.
func TestContentUnknownStructure(t *testing.T) {
	src := []byte(`<section class="deck-slide"></section><script type="application/json" id="deck-motion">{"w":1,"h":1,"slides":[]}</script>`)
	in := &adapter.Input{Src: src, Doc: htmlscan.Scan(src)}
	if _, _, err := (Adapter{}).Content(adapter.SourceOf(in), adapter.Meta{Slides: []adapter.Slide{{}}}); err == nil {
		t.Error("expected an error")
	}
}
