// Package adapter defines the interface of the format adapters (claude.ai, Keynote, generic).
// The shared HTML and asset processing lives in package embed; adapters provide detection,
// the assets loaded at runtime and the deck metadata.
package adapter

import (
	"strings"

	"golang.org/x/net/html"

	"deckfold/internal/htmlscan"
)

// Input is the loaded input.
type Input struct {
	Arg      string // argument as given
	Root     string // root folder (boundary for local files)
	HTMLPath string // path of the entry HTML
	HTMLDir  string // folder of the entry HTML
	IsDir    bool
	Src      []byte
	Doc      *htmlscan.Doc
}

// Slide describes a slide for the runtime.
type Slide struct {
	Label string    `json:"label,omitempty"`
	Notes string    `json:"notes,omitempty"`
	Steps int       `json:"steps"`          // number of steps (states) of the slide, at least 1
	Auto  []bool    `json:"auto,omitempty"` // per step: the source advances by itself
	Delay []float64 `json:"delay,omitempty"`
	Thumb string    `json:"thumb,omitempty"` // asset hash of a thumbnail
}

// Meta describes the deck.
type Meta struct {
	Format  string  `json:"format"`
	Title   string  `json:"title"`
	Width   int     `json:"width"`
	Height  int     `json:"height"`
	Slides  []Slide `json:"slides"`
	Version string  `json:"version,omitempty"`
	// SlideSelector is the CSS selector of the slide elements in the source document, in order
	// (default ".deck-slide"). The presenter overview clones them, embed uses them to name
	// the slide of an asset.
	SlideSelector string         `json:"slideSelector,omitempty"`
	Extra         map[string]any `json:"extra,omitempty"`
}

// ExtraAsset is an asset the source player loads at runtime through a computed path.
type ExtraAsset struct {
	Path  string // file relative to the root folder
	Ref   string // path as the player requests it (relative to the entry HTML, normalized)
	Thumb int    // slide index if it is a thumbnail, otherwise -1
}

// Adapter is a format adapter.
type Adapter interface {
	Name() string
	Detect(in *Input) bool
	// Collect returns the assets loaded at runtime (computed paths).
	Collect(in *Input) ([]ExtraAsset, error)
	// Meta returns slides, steps, notes and timings.
	Meta(in *Input) (Meta, []string, error) // []string: warnings
	// SkipStaticRef reports whether a static reference should be left as is.
	SkipStaticRef(ref string) bool
}

// FindByID finds a tag by id.
func FindByID(d *htmlscan.Doc, id string) *htmlscan.Tag {
	for _, t := range d.Tags {
		if t.Value("id") == id {
			return t
		}
	}
	return nil
}

// Title returns the content of <title>.
func Title(d *htmlscan.Doc) string {
	for _, t := range d.Tags {
		if t.Name == "title" {
			return strings.TrimSpace(html.UnescapeString(string(d.Content(t))))
		}
	}
	return ""
}
