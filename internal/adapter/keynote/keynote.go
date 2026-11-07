// Package keynote is the adapter for Keynote HTML exports (tested: Keynote 15.4, export format 1.2).
// On file:// the Keynote player loads header, slides and PDFs as JSONP through dynamic <script>
// elements (header.jsonp, <id>.jsonp, <id>.pdfp); see docs/spikes/S1-keynote.md.
package keynote

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"deckfold/internal/adapter"
)

// Adapter for Keynote exports.
type Adapter struct{}

// Tested lists the export versions checked against the test corpus (major.minor from header.json).
var Tested = map[string]bool{"1.2": true}

// Name returns the format name.
func (Adapter) Name() string { return "keynote" }

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// Detect: index.html + assets/header.json(p) + assets/player/main.js.
func (Adapter) Detect(in *adapter.Input) bool {
	d := in.HTMLDir
	return (exists(filepath.Join(d, "assets", "header.json")) || exists(filepath.Join(d, "assets", "header.jsonp"))) &&
		exists(filepath.Join(d, "assets", "player", "main.js"))
}

// SkipStaticRef: nothing to skip (all files come through Collect).
func (Adapter) SkipStaticRef(string) bool { return false }

type header struct {
	SlideList               []string `json:"slideList"`
	SlideCount              int      `json:"slideCount"`
	SlideWidth              float64  `json:"slideWidth"`
	SlideHeight             float64  `json:"slideHeight"`
	ShowMode                int      `json:"showMode"`
	AutoplayTransitionDelay float64  `json:"autoplayTransitionDelay"`
	AutoplayBuildDelay      float64  `json:"autoplayBuildDelay"`
	LoopSlideshow           int      `json:"loopSlideshow"`
	Title                   string   `json:"title"`
	Creator                 string   `json:"creator"`
	MajorVersion            int      `json:"majorVersion"`
	MinorVersion            int      `json:"minorVersion"`
}

type slideJSON struct {
	Events []struct {
		Effects []struct {
			Type string `json:"type"`
		} `json:"effects"`
		Automatic bool `json:"automatic"`
	} `json:"events"`
}

// readJSONOrP reads .json or, if only that exists, .jsonp (stripping the fn( ... ) wrapper).
func readJSONOrP(p string) ([]byte, error) {
	if b, err := os.ReadFile(p); err == nil {
		return b, nil
	}
	b, err := os.ReadFile(p + "p")
	if err != nil {
		return nil, err
	}
	s := string(b)
	i, j := strings.Index(s, "("), strings.LastIndex(s, ")")
	if i < 0 || j < i {
		return nil, fmt.Errorf("%s: cannot read JSONP", p+"p")
	}
	return []byte(s[i+1 : j]), nil
}

func (Adapter) header(in *adapter.Input) (header, error) {
	var h header
	b, err := readJSONOrP(filepath.Join(in.HTMLDir, "assets", "header.json"))
	if err != nil {
		return h, err
	}
	if err := json.Unmarshal(b, &h); err != nil {
		return h, fmt.Errorf("Keynote export: cannot read header.json: %w", err)
	}
	// header.jsonp wraps the header directly, slide JSONP wraps it under "json"
	if len(h.SlideList) == 0 {
		return h, fmt.Errorf("Keynote export: header.json has no slideList; unknown structure")
	}
	return h, nil
}

// Collect returns every file under assets/: header, slides (JSON and JSONP), textures (PDF and PDFP),
// media, thumbnails and the player including CMaps. On file:// the player uses JSONP/PDFP; in the
// srcdoc previews of the presenter view the protocol is not file:, so it loads JSON/PDF and CMaps
// through XHR/fetch (see docs/spikes/S1-keynote.md).
func (a Adapter) Collect(in *adapter.Input) ([]adapter.ExtraAsset, error) {
	h, err := a.header(in)
	if err != nil {
		return nil, err
	}
	thumbs := map[string]int{}
	for i, id := range h.SlideList {
		thumbs[path.Join("assets", id, "thumbnail.jpeg")] = i
		if !exists(filepath.Join(in.HTMLDir, "assets", id, id+".jsonp")) {
			return nil, fmt.Errorf("Keynote export incomplete: assets/%s/%s.jsonp is missing", id, id)
		}
	}
	var out []adapter.ExtraAsset
	err = filepath.WalkDir(filepath.Join(in.HTMLDir, "assets"), func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || strings.HasPrefix(d.Name(), ".") || d.Name() == "Acknowledgements.pdf" {
			return nil
		}
		relHTML, _ := filepath.Rel(in.HTMLDir, p)
		relRoot, _ := filepath.Rel(in.Root, p)
		ref := filepath.ToSlash(relHTML)
		t, ok := thumbs[ref]
		if !ok {
			t = -1
		}
		out = append(out, adapter.ExtraAsset{Path: filepath.ToSlash(relRoot), Ref: ref, Thumb: t})
		return nil
	})
	return out, err
}

// Meta reads slides, steps and self-playing timings.
func (a Adapter) Meta(in *adapter.Input) (adapter.Meta, []string, error) {
	h, err := a.header(in)
	if err != nil {
		return adapter.Meta{}, nil, err
	}
	var warns []string
	ver := fmt.Sprintf("%d.%d", h.MajorVersion, h.MinorVersion)
	if !Tested[ver] {
		warns = append(warns, fmt.Sprintf("Keynote export format %s (%s) has not been checked against the test corpus; tested: 1.2 (Keynote 15.4)", ver, h.Creator))
	}
	m := adapter.Meta{Format: "keynote", Title: h.Title, Width: int(h.SlideWidth), Height: int(h.SlideHeight), Version: ver,
		Extra: map[string]any{"showMode": h.ShowMode, "autoplayTransitionDelay": h.AutoplayTransitionDelay,
			"autoplayBuildDelay": h.AutoplayBuildDelay, "loop": h.LoopSlideshow != 0, "creator": h.Creator}}
	for _, id := range h.SlideList {
		b, err := readJSONOrP(filepath.Join(in.HTMLDir, "assets", id, id+".json"))
		if err != nil {
			return adapter.Meta{}, nil, fmt.Errorf("Keynote export: slide %s: %w", id, err)
		}
		var sj slideJSON
		if err := json.Unmarshal(b, &sj); err != nil {
			// .jsonp wraps {"name": ..., "json": {...}}
			var wrap struct {
				JSON slideJSON `json:"json"`
			}
			if err2 := json.Unmarshal(b, &wrap); err2 != nil {
				return adapter.Meta{}, nil, fmt.Errorf("Keynote export: cannot read slide %s: %w", id, err)
			}
			sj = wrap.JSON
		}
		// Every event is one click; the last event of a slide is the transition to the next one.
		// Steps = states of the slide = number of builds + 1.
		builds := 0
		var delays []float64
		for _, e := range sj.Events {
			isTransition := len(e.Effects) > 0 && e.Effects[0].Type == "transition"
			if !isTransition {
				builds++
				delays = append(delays, h.AutoplayBuildDelay)
			}
		}
		sl := adapter.Slide{Steps: builds + 1}
		if h.ShowMode == 1 { // 1 = self-playing (2 = links only)
			// self-playing: time until the next build or the next slide
			sl.Delay = append(delays, h.AutoplayTransitionDelay)
		}
		m.Slides = append(m.Slides, sl)
	}
	if len(m.Slides) != h.SlideCount {
		warns = append(warns, fmt.Sprintf("Keynote export: slideCount is %d but slideList has %d slides", h.SlideCount, len(m.Slides)))
	}
	return m, warns, nil
}
