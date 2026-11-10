package claude

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/net/html"

	"deckfold/internal/adapter"
)

// The content of a claude.ai deck is read from the slide sections with the tokenizer of
// golang.org/x/net/html. htmlscan is not enough here: it records start tags only, and the
// content needs the text and the nesting (which element a text sits in, and which elements
// around it appear in a later step). The tokenizer only reads; the output is still written
// through patches on the original bytes.

// inline elements belong to the block of the element around them; every other element starts
// a block of its own.
var inline = map[string]bool{"a": true, "abbr": true, "b": true, "bdi": true, "bdo": true, "br": true, "cite": true,
	"code": true, "em": true, "font": true, "i": true, "img": true, "kbd": true, "mark": true, "q": true, "s": true,
	"samp": true, "small": true, "span": true, "strong": true, "sub": true, "sup": true, "time": true, "u": true,
	"var": true, "wbr": true}

// void elements have no end tag.
var void = map[string]bool{"area": true, "base": true, "br": true, "col": true, "embed": true, "hr": true, "img": true,
	"input": true, "link": true, "meta": true, "source": true, "track": true, "wbr": true}

// skipped elements hold no slide text: code, styles, and embedded documents.
var skipped = map[string]bool{"script": true, "style": true, "iframe": true, "noscript": true, "template": true, "textarea": true}

// never is the step of something that shows in no step.
const never = -1

// later returns the later of two first steps; never beats every step.
func later(a, b int) int {
	if a == never || b == never {
		return never
	}
	if b > a {
		return b
	}
	return a
}

// firstSteps returns, for the slides of the deck JSON, the first step in which each element
// named in a step's hidden list shows: the first step whose list no longer holds its id (never
// when every list holds it). An element in no list shows from the start and is not in the map.
func firstSteps(deckJSON []byte) ([]map[string]int, error) {
	var dm struct {
		Slides []struct {
			Steps []struct {
				Hidden []json.RawMessage `json:"hidden"`
			} `json:"steps"`
		} `json:"slides"`
	}
	if err := json.Unmarshal(deckJSON, &dm); err != nil {
		return nil, fmt.Errorf("claude.ai deck: cannot read the deck JSON: %w", err)
	}
	out := make([]map[string]int, len(dm.Slides))
	for i, sl := range dm.Slides {
		first := map[string]int{}
		lists := make([]map[string]bool, len(sl.Steps))
		for n, st := range sl.Steps {
			lists[n] = map[string]bool{}
			for _, raw := range st.Hidden {
				var id string
				if json.Unmarshal(raw, &id) == nil {
					lists[n][id] = true
					first[id] = never
				}
			}
		}
		for id := range first {
			for n := range lists {
				if !lists[n][id] {
					first[id] = n
					break
				}
			}
		}
		out[i] = first
	}
	return out, nil
}

type elem struct {
	name  string
	block bool
	step  int // first step of this element together with everything around it
	list  int // number of lists around it
	item  bool
	skip  bool
	text  strings.Builder
}

// Content reads the text blocks of the slide sections.
func (Adapter) Content(src adapter.ContentSource, meta adapter.Meta) (*adapter.Content, []string, error) {
	t := adapter.FindByID(src.Doc, "deck-motion")
	if t == nil {
		return nil, nil, fmt.Errorf("claude.ai deck: the deck JSON is missing")
	}
	steps, err := firstSteps(src.Doc.Content(t))
	if err != nil {
		return nil, nil, err
	}
	// only the part of the document that holds the slides is read
	from, sections := -1, 0
	for _, tg := range src.Doc.Tags {
		if tg.Name == "section" && hasClass(tg.Value("class"), "deck-slide") {
			if from < 0 {
				from = tg.Start
			}
			sections++
		}
	}
	if sections != len(meta.Slides) || sections != len(steps) {
		return nil, nil, fmt.Errorf("claude.ai deck: %d slide sections, %d slides in the deck JSON; unknown structure", sections, len(steps))
	}
	c := &adapter.Content{Slides: make([]adapter.SlideContent, sections)}
	if sections == 0 {
		return c, nil, nil
	}

	z := html.NewTokenizer(bytes.NewReader(src.Doc.Src[from:]))
	var stack []*elem // the open elements of the slide being read; stack[0] is its section
	done := 0         // slides read so far; the one being read has this index
	emit := func(role, text string, e *elem) {
		b := adapter.Block{Role: role, Text: text, Step: e.step}
		if role == adapter.RoleItem && e.list > 1 {
			b.Level = e.list - 1
		}
		c.Slides[done].Blocks = append(c.Slides[done].Blocks, b)
	}
	// owner is the innermost open block element: the one a text belongs to
	owner := func() *elem {
		for i := len(stack) - 1; i >= 0; i-- {
			if stack[i].block {
				return stack[i]
			}
		}
		return nil
	}
	// flush turns the text an element has gathered so far into a block
	flush := func(e *elem) {
		if e == nil {
			return
		}
		role := adapter.RoleText
		if e.item {
			role = adapter.RoleItem
		}
		if s := adapter.CleanText(e.text.String()); s != "" && !e.skip {
			emit(role, s, e)
		}
		e.text.Reset()
	}
	for done < sections {
		tt := z.Next()
		if tt == html.ErrorToken {
			break // end of the document
		}
		switch tt {
		case html.StartTagToken, html.SelfClosingTagToken:
			tok := z.Token()
			name := tok.Data
			attr := func(k string) (string, bool) {
				for _, a := range tok.Attr {
					if a.Key == k {
						return a.Val, true
					}
				}
				return "", false
			}
			if len(stack) == 0 {
				// between two slides: only a slide section starts something
				if cls, _ := attr("class"); name == "section" && hasClass(cls, "deck-slide") {
					stack = append(stack, &elem{name: name, block: true})
				}
				continue
			}
			parent := stack[len(stack)-1]
			e := &elem{name: name, block: !inline[name], step: parent.step, list: parent.list, item: parent.item, skip: parent.skip || skipped[name]}
			if _, ok := attr("hidden"); ok {
				e.skip = true
			}
			if id, ok := attr("data-frame-id"); ok {
				if st, ok := steps[done][id]; ok {
					e.step = later(e.step, st)
				}
			}
			if e.block {
				// the text the block around it has so far comes before this block
				flush(owner())
			}
			switch name {
			case "ul", "ol":
				e.list++
			case "li":
				e.item = true
			case "br":
				if o := owner(); o != nil {
					o.text.WriteByte(' ')
				}
			case "img":
				if alt, ok := attr("alt"); ok && !e.skip {
					if s := adapter.CleanText(alt); s != "" {
						flush(owner()) // the text before the picture comes first
						emit(adapter.RoleAlt, s, e)
					}
				}
			}
			if tt == html.StartTagToken && !void[name] {
				stack = append(stack, e)
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			at := -1
			for i := len(stack) - 1; i >= 0; i-- {
				if stack[i].name == string(name) {
					at = i
					break
				}
			}
			if at < 0 {
				continue // an end tag without its start tag
			}
			// close the element and everything left open inside it
			for i := len(stack) - 1; i >= at; i-- {
				if stack[i].block {
					flush(stack[i])
				}
			}
			stack = stack[:at]
			if at == 0 {
				done++ // the slide section is closed
			}
		case html.TextToken:
			if len(stack) == 0 || stack[len(stack)-1].skip {
				continue
			}
			if o := owner(); o != nil {
				o.text.Write(z.Text())
			}
		}
	}
	if done != sections {
		return nil, nil, fmt.Errorf("claude.ai deck: %d of %d slide sections could be read; unknown structure", done, sections)
	}
	return c, nil, nil
}
