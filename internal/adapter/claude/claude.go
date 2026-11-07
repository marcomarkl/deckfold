// Package claude is the adapter for slide exports from claude.ai.
// Detection: meta[name^="hz:"] plus the deck JSON in <script type="application/json" id="deck-motion">.
package claude

import (
	"bytes"
	"encoding/json"
	"fmt"

	"deckfold/internal/adapter"
)

// Adapter for claude.ai decks.
type Adapter struct{}

// Name returns the format name.
func (Adapter) Name() string { return "claude" }

// Detect looks for the hz: metadata and the deck JSON block.
func (Adapter) Detect(in *adapter.Input) bool {
	if in.Doc == nil {
		return false
	}
	hasMeta := false
	for _, t := range in.Doc.Tags {
		if t.Name == "meta" && bytes.HasPrefix([]byte(t.Value("name")), []byte("hz:")) {
			hasMeta = true
			break
		}
	}
	return hasMeta && adapter.FindByID(in.Doc, "deck-motion") != nil
}

// Collect: claude.ai decks load nothing through computed paths at runtime.
func (Adapter) Collect(in *adapter.Input) ([]adapter.ExtraAsset, error) { return nil, nil }

// SkipStaticRef: nothing to skip.
func (Adapter) SkipStaticRef(string) bool { return false }

// deckMotion is the part of the deck JSON deckfold needs. Unknown fields are allowed;
// the required fields are checked strictly (version detection).
type deckMotion struct {
	W       *float64 `json:"w"`
	H       *float64 `json:"h"`
	BuildMs *float64 `json:"buildMs"`
	TransMs map[string]float64
	Slides  *[]struct {
		Transition string `json:"transition"`
		Steps      *[]struct {
			Auto   *bool             `json:"auto"`
			Hidden []json.RawMessage `json:"hidden"`
		} `json:"steps"`
		Embeds []struct {
			ID     string `json:"id"`
			Srcdoc string `json:"srcdoc"`
		} `json:"embeds"`
	} `json:"slides"`
}

// Meta reads slides, steps, auto steps and notes.
func (Adapter) Meta(in *adapter.Input) (adapter.Meta, []string, error) {
	t := adapter.FindByID(in.Doc, "deck-motion")
	var dm deckMotion
	if err := json.Unmarshal(in.Doc.Content(t), &dm); err != nil {
		return adapter.Meta{}, nil, fmt.Errorf("claude.ai deck: cannot read the deck JSON: %w", err)
	}
	if dm.W == nil || dm.H == nil || dm.Slides == nil {
		return adapter.Meta{}, nil, fmt.Errorf("claude.ai deck: unknown deck JSON structure (w, h or slides missing); deckfold does not know this format yet")
	}
	// notes and labels from the slide sections, in document order
	var notes, labels []string
	for _, tg := range in.Doc.Tags {
		if tg.Name == "section" && hasClass(tg.Value("class"), "deck-slide") {
			notes = append(notes, tg.Value("data-speaker-notes"))
			labels = append(labels, tg.Value("data-label"))
		}
	}
	if len(notes) != len(*dm.Slides) {
		return adapter.Meta{}, nil, fmt.Errorf("claude.ai deck: %d slides in the deck JSON but %d slide sections in the HTML; unknown structure", len(*dm.Slides), len(notes))
	}
	m := adapter.Meta{Format: "claude", Title: adapter.Title(in.Doc), Width: int(*dm.W), Height: int(*dm.H),
		Extra: map[string]any{}}
	if dm.BuildMs != nil {
		m.Extra["buildMs"] = *dm.BuildMs
	}
	var warns []string
	var netSlides []string
	embeds := 0
	for i, s := range *dm.Slides {
		if s.Steps == nil || len(*s.Steps) == 0 {
			return adapter.Meta{}, nil, fmt.Errorf("claude.ai deck: slide %d has no steps; unknown structure", i+1)
		}
		sl := adapter.Slide{Label: labels[i], Notes: notes[i], Steps: len(*s.Steps)}
		anyAuto := false
		for _, st := range *s.Steps {
			a := st.Auto != nil && *st.Auto
			anyAuto = anyAuto || a
			sl.Auto = append(sl.Auto, a)
		}
		if !anyAuto {
			sl.Auto = nil
		}
		embeds += len(s.Embeds)
		for _, e := range s.Embeds {
			if bytes.Contains([]byte(e.Srcdoc), []byte("http://")) || bytes.Contains([]byte(e.Srcdoc), []byte("https://")) {
				netSlides = append(netSlides, fmt.Sprintf("slide %d: web embed %s references network content", i+1, e.ID))
			}
		}
		m.Slides = append(m.Slides, sl)
	}
	m.Extra["embeds"] = embeds
	if len(netSlides) > 0 {
		m.Extra["netSlides"] = netSlides
	}
	return m, warns, nil
}

func hasClass(cls, want string) bool {
	for _, c := range bytes.Fields([]byte(cls)) {
		if string(c) == want {
			return true
		}
	}
	return false
}
