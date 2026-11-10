// Package content reads the content model of a finished deckfold file (docs/content-model.md)
// and writes it for other programs: as JSON with the deck's title, size and notes, or as
// Markdown for people and for tools that read text.
package content

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"deckfold/internal/adapter"
	"deckfold/internal/htmlscan"
	"deckfold/internal/inject"
)

// Errors of Read.
var (
	// ErrNoModel: the file was made before the content model existed.
	ErrNoModel = errors.New("the file has no content model")
	// ErrGeneric: decks embedded with --generic have no slide model.
	ErrGeneric = errors.New("the file has no content model: it was embedded with --generic, without a slide model")
	// ErrNewer: the model has a version this deckfold does not know.
	ErrNewer = errors.New("the content model of the file was written by a newer deckfold")
)

// Deck is the content of a deck as --content writes it.
type Deck struct {
	Version int     `json:"version"`
	Format  string  `json:"format"`
	Title   string  `json:"title"`
	Width   int     `json:"width"`
	Height  int     `json:"height"`
	Slides  []Slide `json:"slides"`
}

// Slide is one slide: the entry of the content model plus the presenter notes.
type Slide struct {
	Title  string          `json:"title,omitempty"`
	Notes  string          `json:"notes,omitempty"`
	Blocks []adapter.Block `json:"blocks"`
}

// block returns the content of the JSON script element with the given id.
func block(d *htmlscan.Doc, id string) []byte {
	for _, t := range d.Tags {
		if t.Name == "script" && t.Value("id") == id && t.Value("type") == "application/json" {
			return d.Content(t)
		}
	}
	return nil
}

// Meta reads the deck metadata of a finished deckfold file.
func Meta(d *htmlscan.Doc) (adapter.Meta, map[string]string, error) {
	var m struct {
		Deck adapter.Meta      `json:"deck"`
		Shim map[string]string `json:"shim"`
	}
	b := block(d, "df-meta")
	if b == nil {
		return adapter.Meta{}, nil, fmt.Errorf("the file has no deck metadata")
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return adapter.Meta{}, nil, fmt.Errorf("the deck metadata cannot be read: %w", err)
	}
	return m.Deck, m.Shim, nil
}

// Read reads the content model and the deck metadata from a finished deckfold file.
func Read(src []byte) (*Deck, error) {
	d := htmlscan.Scan(src)
	meta, _, err := Meta(d)
	if err != nil {
		return nil, err
	}
	if meta.Format == "generic" {
		return nil, ErrGeneric
	}
	b := block(d, inject.ContentID)
	if b == nil {
		return nil, ErrNoModel
	}
	var c adapter.Content
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("the content model cannot be read: %w", err)
	}
	if c.Version > adapter.ContentVersion {
		return nil, ErrNewer
	}
	if len(c.Slides) != len(meta.Slides) {
		return nil, fmt.Errorf("the content model has %d slides, the deck metadata %d", len(c.Slides), len(meta.Slides))
	}
	out := &Deck{Version: c.Version, Format: meta.Format, Title: meta.Title, Width: meta.Width, Height: meta.Height}
	for i, s := range c.Slides {
		sl := Slide{Title: s.Title, Notes: meta.Slides[i].Notes, Blocks: s.Blocks}
		if sl.Blocks == nil {
			sl.Blocks = []adapter.Block{}
		}
		out.Slides = append(out.Slides, sl)
	}
	return out, nil
}

// JSON returns the deck as indented JSON.
func (d *Deck) JSON() ([]byte, error) {
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// Markdown returns the deck as Markdown: the deck title, then every slide with its title (or
// its number), its text, its lists and its notes. The steps are only in the JSON.
//
// Text from the presentation is data here as well: every character Markdown gives a meaning is
// escaped, so that slide text cannot add a heading, a link or HTML to the document.
func (d *Deck) Markdown() []byte {
	var b strings.Builder
	title := d.Title
	if title == "" {
		title = "Presentation"
	}
	b.WriteString("# " + escape(title) + "\n")
	for i, s := range d.Slides {
		if s.Title != "" {
			fmt.Fprintf(&b, "\n## %d. %s\n", i+1, escape(s.Title))
		} else {
			fmt.Fprintf(&b, "\n## Slide %d\n", i+1)
		}
		inList := false
		for _, bl := range s.Blocks {
			text := escape(bl.Text)
			switch bl.Role {
			case adapter.RoleItem:
				if !inList {
					b.WriteString("\n")
				}
				level := bl.Level
				if level < 0 {
					level = 0
				}
				b.WriteString(strings.Repeat("  ", level) + "- " + text + "\n")
				inList = true
				continue
			case adapter.RoleTitle:
				if bl.Text == s.Title {
					continue // already the heading
				}
				b.WriteString("\n" + text + "\n")
			case adapter.RoleAlt:
				b.WriteString("\nImage: " + text + "\n")
			default:
				b.WriteString("\n" + text + "\n")
			}
			inList = false
		}
		if notes := strings.TrimSpace(s.Notes); notes != "" {
			b.WriteString("\n> Notes\n>\n")
			for _, line := range strings.Split(notes, "\n") {
				if line = adapter.CleanText(line); line == "" {
					b.WriteString(">\n")
				} else {
					b.WriteString("> " + escape(line) + "\n")
				}
			}
		}
	}
	return []byte(b.String())
}

// escape makes one line of text literal in Markdown.
func escape(s string) string {
	s = adapter.CleanText(s)
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\', '`', '*', '_', '#', '[', ']', '<', '>', '|', '~', '&', '!':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	out := b.String()
	// what would start a list, a rule or a numbered list at the beginning of a line
	if out != "" {
		switch out[0] {
		case '-', '+', '=':
			out = `\` + out
		default:
			i := 0
			for i < len(out) && out[i] >= '0' && out[i] <= '9' {
				i++
			}
			if i > 0 && i < len(out) && (out[i] == '.' || out[i] == ')') {
				out = out[:i] + `\` + out[i:]
			}
		}
	}
	return out
}
