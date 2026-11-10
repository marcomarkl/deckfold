package keynote

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"deckfold/internal/adapter"
)

// The Keynote export lists the text of a slide for screen readers: every event of the slide JSON
// has "accessibility" entries with a text and its rectangle. That is the only text in the export
// (the slides themselves are PDF textures). The entries carry no role, and their order is the
// order of the export, which is not always top to bottom; deckfold keeps it.

type rect struct{ X, Y, Width, Height float64 }

type slideText struct {
	Events []struct {
		Accessibility []struct {
			Text string `json:"text"`
			Rect rect   `json:"targetRectangle"`
		} `json:"accessibility"`
		Hyperlinks []hyperlink `json:"hyperlinks"`
	} `json:"events"`
}

// hyperlink is a link of a slide as the export stores it: the place on the slide (in slide
// units, see slideWidth and slideHeight of the header) and the target as a query string.
type hyperlink struct {
	URL  string `json:"url"`
	Rect rect   `json:"targetRectangle"`
}

// linkTarget returns the target of a link to another slide in the form of the content model:
// "next", "prev", "first", "last", "back" (the slide viewed before), "end", or the index of the
// slide. ok is false for every other link (a web address) and for a link to a slide that is not
// in the export, such as one skipped in Keynote. Measured on an export of Keynote 15.4, see
// docs/spikes/S1-keynote.md.
func linkTarget(url string, slides []string) (to string, ok bool) {
	switch url {
	case "?slide=next":
		return "next", true
	case "?slide=previous":
		return "prev", true
	case "?slide=first":
		return "first", true
	case "?slide=last":
		return "last", true
	case "?action=retreat":
		return "back", true
	case "?action=exitpresentation":
		return "end", true
	}
	if id, found := strings.CutPrefix(url, "?slideid="); found {
		for i, s := range slides {
			if s == id {
				return strconv.Itoa(i), true
			}
		}
	}
	return "", false
}

// readSlide parses a slide JSON; the JSONP form wraps it as {"name": ..., "json": {...}}.
func readSlide(b []byte) (slideText, error) {
	var wrap struct {
		JSON *slideText `json:"json"`
	}
	if err := json.Unmarshal(b, &wrap); err == nil && wrap.JSON != nil {
		return *wrap.JSON, nil
	}
	var st slideText
	err := json.Unmarshal(b, &st)
	return st, err
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

// Content reads the text of every slide: every text the export lists, once. The order is the
// one of the last event, which lists the whole slide; what only an earlier event lists follows.
//
// The blocks carry no step. What the export lists for an event is not what is visible in that
// step: objects that a build shows later are listed from the start, and an event can leave out
// paragraphs that are visible (measured on an export with builds, docs/spikes/S1-keynote.md).
//
// A link to another slide is on the block that has exactly the rectangle of the link.
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
		st, err := readSlide(b)
		if err != nil {
			return nil, nil, fmt.Errorf("Keynote export: cannot read slide %s: %w", id, err)
		}
		jumps := map[rect]string{}
		for _, ev := range st.Events {
			for _, l := range ev.Hyperlinks {
				if to, ok := linkTarget(l.URL, h.SlideList); ok {
					jumps[l.Rect] = to
				}
			}
		}
		type entry struct {
			text string
			at   rect
		}
		seen := map[entry]bool{}
		// the last event first, then the earlier ones in their own order
		order := make([]int, 0, len(st.Events))
		if len(st.Events) > 0 {
			order = append(order, len(st.Events)-1)
			for n := 0; n < len(st.Events)-1; n++ {
				order = append(order, n)
			}
		}
		for _, n := range order {
			for _, a := range st.Events[n].Accessibility {
				e := entry{a.Text, a.Rect}
				if seen[e] {
					continue
				}
				seen[e] = true
				bl := adapter.Block{Role: adapter.RoleText, Text: a.Text, Step: adapter.StepUnknown}
				if to, ok := jumps[a.Rect]; ok {
					if k, err := strconv.Atoi(to); err == nil {
						bl.Jump = &adapter.Jump{Slide: &k}
					} else {
						bl.Jump = &adapter.Jump{To: to}
					}
				}
				c.Slides[i].Blocks = append(c.Slides[i].Blocks, bl)
			}
		}
	}
	return c, nil, nil
}
