package keynote

import (
	"encoding/json"
	"fmt"
	"strings"

	"deckfold/internal/adapter"
)

// The Keynote export lists the text of a slide for screen readers: every event of the slide JSON
// has "accessibility" entries with a text and its rectangle. That is the only text in the export
// (the slides themselves are PDF textures). The entries carry no role, and their order is the
// order of the export, which is not always top to bottom; deckfold keeps it.

type slideText struct {
	Events []struct {
		Accessibility []struct {
			Text string `json:"text"`
			Rect struct {
				X, Y, Width, Height float64
			} `json:"targetRectangle"`
		} `json:"accessibility"`
	} `json:"events"`
}

// loadJSONOrP loads ref or, if that is missing, ref+"p" without its fn( ... ) wrapper.
func loadJSONOrP(load func(string) ([]byte, error), ref string) ([]byte, error) {
	if b, err := load(ref); err == nil {
		return b, nil
	}
	b, err := load(ref + "p")
	if err != nil {
		return nil, err
	}
	s := string(b)
	i, j := strings.Index(s, "("), strings.LastIndex(s, ")")
	if i < 0 || j < i {
		return nil, fmt.Errorf("%sp: cannot read JSONP", ref)
	}
	return []byte(s[i+1 : j]), nil
}

// Content reads the text of every slide. A text belongs to the first event that lists it: one
// event is one click, so the index of the event is the step in which the text shows.
func (Adapter) Content(src adapter.ContentSource, meta adapter.Meta) (*adapter.Content, []string, error) {
	hb, err := loadJSONOrP(src.Load, "assets/header.json")
	if err != nil {
		return nil, nil, fmt.Errorf("Keynote export: header: %w", err)
	}
	var h header
	if err := json.Unmarshal(hb, &h); err != nil {
		return nil, nil, fmt.Errorf("Keynote export: cannot read header.json: %w", err)
	}
	if len(h.SlideList) != len(meta.Slides) {
		return nil, nil, fmt.Errorf("Keynote export: %d slides in the header, %d in the deck metadata", len(h.SlideList), len(meta.Slides))
	}
	c := &adapter.Content{Slides: make([]adapter.SlideContent, len(h.SlideList))}
	for i, id := range h.SlideList {
		b, err := loadJSONOrP(src.Load, "assets/"+id+"/"+id+".json")
		if err != nil {
			return nil, nil, fmt.Errorf("Keynote export: slide %s: %w", id, err)
		}
		var st slideText
		// .jsonp wraps {"name": ..., "json": {...}}
		var wrap struct {
			JSON *slideText `json:"json"`
		}
		if err := json.Unmarshal(b, &wrap); err == nil && wrap.JSON != nil {
			st = *wrap.JSON
		} else if err := json.Unmarshal(b, &st); err != nil {
			return nil, nil, fmt.Errorf("Keynote export: cannot read slide %s: %w", id, err)
		}
		last := meta.Slides[i].Steps - 1
		seen := map[string]bool{}
		for n, ev := range st.Events {
			step := n
			if step > last {
				step = last // the event of the transition to the next slide
			}
			for _, a := range ev.Accessibility {
				key := fmt.Sprintf("%g,%g,%g,%g\x00%s", a.Rect.X, a.Rect.Y, a.Rect.Width, a.Rect.Height, a.Text)
				if seen[key] {
					continue
				}
				seen[key] = true
				c.Slides[i].Blocks = append(c.Slides[i].Blocks, adapter.Block{Role: adapter.RoleText, Text: a.Text, Step: step})
			}
		}
	}
	return c, nil, nil
}
