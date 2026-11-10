package render

import (
	"strings"

	"deckfold/internal/pptx/xmltree"
)

// Block is one piece of slide text for the content model (docs/content-model.md): a paragraph
// of a shape or of a table cell, or the description of a picture. Only the slide's own content
// becomes blocks: text from the layout and the master would be on every slide, and the
// placeholders for date, footer and slide number are not content either.
type Block struct {
	// Shape is the data-sid of the shape; for text in a table or a SmartArt graphic, of its frame.
	Shape string
	// Para is the paragraph number inside the shape (its data-p group), -1 when the block has
	// no paragraph group of its own.
	Para int
	// Groups are the data-sid values of the groups around the shape.
	Groups []string
	Role   string // title, subtitle, text, item, alt
	Level  int    // list level of an item
	Text   string
	// Jump is the target of a link to another slide on the block (see relLookup.jumpTarget).
	Jump string
}

// paraText returns the text of a paragraph; a line break is a space.
func (r *slideRenderer) paraText(p *xmltree.Node) string {
	var s strings.Builder
	for _, k := range p.Kids {
		switch k.Name {
		case "r":
			s.WriteString(k.Child("t").AllText())
		case "fld":
			s.WriteString(r.fieldText(k))
		case "br":
			s.WriteByte(' ')
		}
	}
	return s.String()
}

// collectText adds the paragraphs of a text body as blocks. ph is the placeholder type of the
// shape ("" for none); own tells whether the paragraphs have groups of their own under shape
// (a shape on the slide) or not (a table cell, a shape of a SmartArt graphic); plain keeps
// every paragraph "text" (table cells).
func (r *slideRenderer) collectText(body *xmltree.Node, ts *textStyle, shape, ph string, own, plain bool, rels relLookup, shapeJump string) {
	if body == nil {
		return
	}
	role := ""
	switch ph {
	case "title", "ctrTitle":
		role = "title"
	case "subTitle":
		role = "subtitle"
	case "dt", "ftr", "sldNum":
		return
	}
	for pi, p := range body.Children("p") {
		text := r.paraText(p)
		if strings.TrimSpace(text) == "" {
			continue
		}
		b := Block{Shape: shape, Para: -1, Groups: append([]string(nil), r.groups...), Role: role, Text: text, Jump: shapeJump}
		// a link on the text of the paragraph: the first one counts
		for _, k := range p.Kids {
			if h := k.Path("rPr/hlinkClick"); h != nil && b.Jump == "" && (k.Name == "r" || k.Name == "fld") {
				b.Jump = rels.jumpTarget(h)
			}
		}
		if own {
			b.Para = pi
		}
		if b.Role == "" {
			b.Role = "text"
			ppNode := p.Child("pPr")
			lvl := int(ppNode.Int("lvl", 0))
			if lvl < 0 || lvl > 8 {
				lvl = 0
			}
			// the bullet with everything the paragraph inherits, as the layout resolves it
			pp := newProps()
			for _, src := range ts.levels[lvl] {
				pp.add(src)
			}
			pp.add(ppNode)
			if bt := pp.elem("buChar"); !plain && bt != nil && (bt.Name == "buChar" || bt.Name == "buAutoNum" || bt.Name == "buBlip") {
				b.Role, b.Level = "item", lvl
			}
		}
		r.blocks = append(r.blocks, b)
	}
}
