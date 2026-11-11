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
			Type      string  `json:"type"`
			BeginTime float64 `json:"beginTime"` // seconds after the click
			Duration  float64 `json:"duration"`  // seconds
		} `json:"effects"`
		Automatic  bool        `json:"automatic"`
		Hyperlinks []hyperlink `json:"hyperlinks"`
	} `json:"events"`
}

// Link is a link to another slide for the runtime (deck.extra.links, one list per slide): its
// rectangle in slide units, the step in which it can be clicked and its target. The Keynote
// player draws the slides on a canvas, so the runtime finds the link under a click by its
// rectangle.
type Link struct {
	Step int     `json:"step"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	W    float64 `json:"w"`
	H    float64 `json:"h"`
	To   string  `json:"to"`
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
	var links [][]Link
	anyLink, lost := false, 0
	// builds: per slide, how long each of its builds takes, in seconds. The player draws a build
	// itself and loses a key that comes while it is still drawing one, so the runtime waits this
	// long after the key of a build before it sends the next (keynote.jump in 20-model.js).
	var buildTimes [][]float64
	anyBuild := false
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
		var delays, times []float64
		for _, e := range sj.Events {
			isTransition := len(e.Effects) > 0 && e.Effects[0].Type == "transition"
			if !isTransition {
				builds++
				delays = append(delays, h.AutoplayBuildDelay)
				// a build ends when the last of its effects does
				end := 0.0
				for _, ef := range e.Effects {
					if t := ef.BeginTime + ef.Duration; t > end {
						end = t
					}
				}
				times = append(times, end)
			}
		}
		buildTimes = append(buildTimes, times)
		anyBuild = anyBuild || len(times) > 0
		sl := adapter.Slide{Steps: builds + 1}
		// links to other slides: an event is one step; the event of the transition is the last
		var ls []Link
		for n, e := range sj.Events {
			step := n
			if step > builds {
				step = builds
			}
			for _, l := range e.Hyperlinks {
				to, ok := linkTarget(l.URL, h.SlideList)
				if !ok {
					if strings.HasPrefix(l.URL, "?slideid=") {
						lost++
					}
					continue
				}
				ls = append(ls, Link{Step: step, X: l.Rect.X, Y: l.Rect.Y, W: l.Rect.Width, H: l.Rect.Height, To: to})
			}
		}
		links = append(links, ls)
		anyLink = anyLink || len(ls) > 0
		if h.ShowMode == 1 { // 1 = self-playing (2 = links only)
			// self-playing: time until the next build or the next slide
			sl.Delay = append(delays, h.AutoplayTransitionDelay)
		}
		m.Slides = append(m.Slides, sl)
	}
	if anyLink {
		m.Extra["links"] = links
	}
	if anyBuild {
		m.Extra["builds"] = buildTimes
	}
	if lost > 0 {
		warns = append(warns, fmt.Sprintf("%d link(s) lead to a slide that is not in the export (skipped in Keynote); a click on them moves on like any click", lost))
	}
	if len(m.Slides) != h.SlideCount {
		warns = append(warns, fmt.Sprintf("Keynote export: slideCount is %d but slideList has %d slides", h.SlideCount, len(m.Slides)))
	}
	return m, warns, nil
}
