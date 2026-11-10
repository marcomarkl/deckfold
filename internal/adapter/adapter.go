// Package adapter defines the interface of the format adapters (claude.ai, Keynote, generic).
// The shared HTML and asset processing lives in package embed; adapters provide detection,
// the assets loaded at runtime and the deck metadata.
package adapter

import (
	"errors"
	"os"
	"path/filepath"
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
	// Content returns what is written on the slides (docs/content-model.md), or nil when the
	// format has no slide model. src is the input of a conversion or a finished deckfold file.
	Content(src ContentSource, meta Meta) (*Content, []string, error) // []string: warnings
}

// ContentVersion is the version of the content model in the output (#df-content).
const ContentVersion = 1

// Block roles. A role comes from the structure of the source, never from a guess.
const (
	RoleTitle    = "title"
	RoleSubtitle = "subtitle"
	RoleText     = "text"
	RoleItem     = "item"
	RoleAlt      = "alt"
)

// Content is the content model of a deck: for every slide of Meta.Slides, in the same order,
// its text blocks.
type Content struct {
	Version int            `json:"version"`
	Slides  []SlideContent `json:"slides"`
}

// SlideContent is the content of one slide.
type SlideContent struct {
	Title  string  `json:"title,omitempty"` // only when the source marks a title
	Blocks []Block `json:"blocks"`
}

// Block is one piece of text on a slide.
type Block struct {
	Role string `json:"role"`
	Text string `json:"text"`
	// Step is the first step of the slide in which the block shows (0: from the start); -1 when
	// it shows in no step of the click order (only on a trigger, or never).
	Step  int   `json:"step"`
	Level int   `json:"level,omitempty"` // list level of an item, 0 for the first
	Jump  *Jump `json:"jump,omitempty"`
}

// Jump is a link to another slide: either Slide (index into Content.Slides) or To ("next",
// "prev", "first", "last", "back", "end").
type Jump struct {
	Slide *int   `json:"slide,omitempty"`
	To    string `json:"to,omitempty"`
}

// ErrConvertAgain is what Content returns for a finished file that does not hold what the
// content is read from: the source has to be converted again.
var ErrConvertAgain = errors.New("convert the presentation again to get search and outline")

// ContentSource is where an adapter reads the content from.
type ContentSource struct {
	// Doc is the HTML: the source document, or the finished deckfold file.
	Doc *htmlscan.Doc
	// Load returns a file the source player loads through a computed path (ref as in
	// ExtraAsset.Ref). For a finished file it reads the embedded asset, never the file system.
	Load func(ref string) ([]byte, error)
}

// SourceOf is the content source of a conversion: the input document and its folder.
func SourceOf(in *Input) ContentSource {
	return ContentSource{Doc: in.Doc, Load: func(ref string) ([]byte, error) {
		return os.ReadFile(filepath.Join(in.HTMLDir, filepath.FromSlash(ref)))
	}}
}

// CleanText collapses white space (line breaks included) and trims the ends.
func CleanText(s string) string { return strings.Join(strings.Fields(s), " ") }

// Finish brings the content an adapter returned into its final form: one entry per slide of
// meta, titles from the slide labels (the same source as the labels of the slide tiles), cleaned
// text, no empty blocks. It returns nil for nil.
func Finish(c *Content, meta Meta) *Content {
	if c == nil {
		return nil
	}
	out := &Content{Version: ContentVersion, Slides: make([]SlideContent, len(meta.Slides))}
	for i := range out.Slides {
		sc := SlideContent{Title: CleanText(meta.Slides[i].Label), Blocks: []Block{}}
		if i < len(c.Slides) {
			for _, b := range c.Slides[i].Blocks {
				if b.Text = CleanText(b.Text); b.Text != "" {
					sc.Blocks = append(sc.Blocks, b)
				}
			}
		}
		out.Slides[i] = sc
	}
	return out
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
