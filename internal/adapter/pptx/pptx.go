// Package pptx is the adapter for the HTML decks deckfold generates from PowerPoint files
// (internal/pptx). Detection: <meta name="deckfold:source" content="pptx"> plus the deck data
// in <script type="application/json" id="df-pptx-deck">.
package pptx

import (
	"encoding/json"
	"fmt"

	"deckfold/internal/adapter"
	"deckfold/internal/pptx/deck"
)

// Adapter for generated PowerPoint decks.
type Adapter struct{}

// Name returns the format name.
func (Adapter) Name() string { return "pptx" }

// Detect looks for the source marker and the deck data.
func (Adapter) Detect(in *adapter.Input) bool {
	if in.Doc == nil {
		return false
	}
	marked := false
	for _, t := range in.Doc.Tags {
		if t.Name == "meta" && t.Value("name") == "deckfold:source" && t.Value("content") == "pptx" {
			marked = true
			break
		}
	}
	return marked && adapter.FindByID(in.Doc, deck.DataID) != nil
}

// Collect: the generated deck references all files statically.
func (Adapter) Collect(*adapter.Input) ([]adapter.ExtraAsset, error) { return nil, nil }

// SkipStaticRef: nothing to skip.
func (Adapter) SkipStaticRef(string) bool { return false }

// Meta reads slides, steps, notes and timings from the deck data.
func (Adapter) Meta(in *adapter.Input) (adapter.Meta, []string, error) {
	t := adapter.FindByID(in.Doc, deck.DataID)
	var d deck.Data
	if err := json.Unmarshal(in.Doc.Content(t), &d); err != nil {
		return adapter.Meta{}, nil, fmt.Errorf("deck data of the generated PowerPoint deck: %w", err)
	}
	m := adapter.Meta{Format: "pptx", Title: d.Title, Width: int(d.Width + 0.5), Height: int(d.Height + 0.5),
		SlideSelector: "." + deck.SlideClass, Extra: map[string]any{"loop": d.Loop}}
	for _, s := range d.Slides {
		sl := adapter.Slide{Label: s.Label, Notes: s.Notes, Steps: s.Steps, Auto: s.Auto, Delay: s.Delay}
		if sl.Steps < 1 {
			sl.Steps = 1
		}
		if d.UseTimings && s.Advance != nil {
			// advance after the last step: the source timing for auto mode (E7); the other steps
			// have no source timing (-1)
			sl.Delay = make([]float64, sl.Steps)
			for i := range sl.Delay {
				sl.Delay[i] = -1
			}
			sl.Delay[sl.Steps-1] = *s.Advance
		}
		m.Slides = append(m.Slides, sl)
	}
	return m, nil, nil
}
