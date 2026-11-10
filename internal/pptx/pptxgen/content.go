package pptxgen

import (
	"fmt"
	"strings"
)

// ContentSpec is the content model the content deck is expected to have: what a reader of the
// deck would list, written down next to the shapes it comes from. The tests compare it with
// what the converter reads (fixtures/pptx/src/content.spec.json).
type ContentSpec struct {
	Slides []ContentSlide `json:"slides"`
}

// ContentSlide is one shown slide of ContentSpec.
type ContentSlide struct {
	Title  string         `json:"title,omitempty"`
	Notes  string         `json:"notes,omitempty"`
	Blocks []ContentBlock `json:"blocks"`
}

// ContentBlock is one text block: its role, the step in which it shows (-1: in no step of the
// click order) and, for a link to another slide, its target.
type ContentBlock struct {
	Role  string       `json:"role"`
	Text  string       `json:"text"`
	Step  int          `json:"step"`
	Level int          `json:"level,omitempty"`
	Jump  *ContentJump `json:"jump,omitempty"`
}

// ContentJump is the target of a link: a slide (index among the shown slides) or a direction.
type ContentJump struct {
	Slide *int   `json:"slide,omitempty"`
	To    string `json:"to,omitempty"`
}

const relSlide = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide"

// showJump is the link PowerPoint writes for "next slide", "end show" and the like.
func showJump(jump string) string {
	return `<a:hlinkClick r:id="" action="ppaction://hlinkshowjump?jump=` + jump + `"/>`
}

// slideJump is a link to the slide with the given position in the file (1-based).
func (s *Slide) slideJump(position int) string {
	return `<a:hlinkClick r:id="` + s.Rel(relSlide, fmt.Sprintf("slide%d.xml", position), "") + `" action="ppaction://hlinksldjump"/>`
}

// Content is the deck for the content model: titles and a subtitle, a list with two levels, a
// picture with a description, a table, text in a group, builds by paragraph and by group, a
// shape only a trigger shows, footer and slide number (which are not content), a slide without
// a title placeholder, a hidden slide, and links to other slides on text and on a shape.
func Content() *Deck {
	d, _ := ContentWithSpec()
	return d
}

// ContentWithSpec returns the deck and its expected content model.
func ContentWithSpec() (*Deck, ContentSpec) {
	d := New("Content model deck")
	var spec ContentSpec
	entr := func(spid, para int) Group {
		return Group{Subs: []Sub{{Effects: []Eff{{Spid: spid, Para: para, Class: "entr", Preset: 10, NodeType: "clickEffect", Bhv: FadeIn}}}}}
	}
	at := func(n int) *int { return &n }

	// slide 1: title and subtitle, footer and slide number, notes
	s := d.Add(LayoutTitle)
	s.Placeholder("ctrTitle", Para{Text: "Content model deck"})
	s.Placeholder("subTitle", Para{Text: "Titles, lists, links and builds"})
	s.Raw(fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="Footer Placeholder 4"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph type="ftr" sz="quarter" idx="11"/></p:nvPr></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US" dirty="0"/><a:t>Footer text</a:t></a:r></a:p></p:txBody></p:sp>`, s.ID()))
	s.Raw(fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="Slide Number Placeholder 5"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph type="sldNum" sz="quarter" idx="12"/></p:nvPr></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:fld id="{B6F15528-21DE-4FAA-801E-634DDDAF4B2B}" type="slidenum"><a:rPr lang="en-US" dirty="0"/><a:t>1</a:t></a:fld><a:endParaRPr lang="en-US" dirty="0"/></a:p></p:txBody></p:sp>`, s.ID()))
	s.Notes = "The notes of the first slide mention a walrus."
	spec.Slides = append(spec.Slides, ContentSlide{Title: "Content model deck", Notes: s.Notes, Blocks: []ContentBlock{
		{Role: "title", Text: "Content model deck"},
		{Role: "subtitle", Text: "Titles, lists, links and builds"},
	}})

	// slide 2: a list with two levels that builds by paragraph, and a plain paragraph
	s = d.Add(LayoutContent)
	s.Placeholder("title", Para{Text: "Lists and levels"})
	body := s.nextID
	s.Placeholder("body",
		Para{Text: "The first entry is there from the start"},
		Para{Text: "The second entry appears on the first click"},
		Para{Text: "A nested entry appears on the second click", Level: 1},
		Para{Text: "The last entry appears on the third click"})
	s.TextBox("Plain", Box{X: 120, Y: 560, W: 900, H: 50}, "", Para{Text: "A plain paragraph below the list"})
	tl := &Timeline{}
	tl.Add(entr(body, 1))
	tl.Add(entr(body, 2))
	tl.Add(entr(body, 3))
	s.Timing = tl.XML()
	s.Notes = "Three clicks build the list."
	spec.Slides = append(spec.Slides, ContentSlide{Title: "Lists and levels", Notes: s.Notes, Blocks: []ContentBlock{
		{Role: "title", Text: "Lists and levels"},
		{Role: "item", Text: "The first entry is there from the start"},
		{Role: "item", Text: "The second entry appears on the first click", Step: 1},
		{Role: "item", Text: "A nested entry appears on the second click", Step: 2, Level: 1},
		{Role: "item", Text: "The last entry appears on the third click", Step: 3},
		{Role: "text", Text: "A plain paragraph below the list"},
	}})

	// slide 3: no title placeholder; a picture with a description, a table, a group that
	// appears on a click, a shape only a trigger shows
	s = d.Add(LayoutBlank)
	s.TextBox("Free text", Box{X: 60, Y: 40, W: 1100, H: 60}, "", Para{Text: "Free text on a slide without a title placeholder", RPr: `sz="2800"`})
	pic := s.Picture(d, "Gradient", Box{X: 60, Y: 140, W: 240, H: 180}, "png", "image/png", TestPNG(240, 180), "", "")
	last := len(s.Shapes) - 1
	s.Shapes[last] = strings.Replace(s.Shapes[last], fmt.Sprintf(`<p:cNvPr id="%d" name="Gradient"/>`, pic), fmt.Sprintf(`<p:cNvPr id="%d" name="Gradient" descr="A gradient picture with a circle"/>`, pic), 1)
	cell := func(text string) string {
		return `<a:tc><a:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US" sz="1600" dirty="0"/><a:t>` + Esc(text) + `</a:t></a:r></a:p></a:txBody><a:tcPr/></a:tc>`
	}
	s.Raw(fmt.Sprintf(`<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="%d" name="Table"/><p:cNvGraphicFramePr><a:graphicFrameLocks noGrp="1"/></p:cNvGraphicFramePr><p:nvPr/></p:nvGraphicFramePr>`+
		`<p:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></p:xfrm><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table"><a:tbl><a:tblPr/>`+
		`<a:tblGrid><a:gridCol w="%d"/><a:gridCol w="%d"/></a:tblGrid><a:tr h="%d">%s%s</a:tr><a:tr h="%d">%s%s</a:tr></a:tbl></a:graphicData></a:graphic></p:graphicFrame>`,
		s.ID(), Px(360), Px(140), Px(500), Px(80), Px(250), Px(250), Px(40), cell("Cell one"), cell("Cell two"), Px(40), cell("Cell three"), cell("Cell four")))
	grp, g1, g2 := s.ID(), s.ID(), s.ID()
	inGroup := func(id int, name string, x int64, text string) string {
		return fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="%s"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="%d" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom>%s<a:ln><a:noFill/></a:ln></p:spPr>`+
			`<p:txBody><a:bodyPr rtlCol="0" anchor="ctr"/><a:lstStyle/>%s</p:txBody></p:sp>`, id, name, x, Px(200), Px(80), solid("D9E2F3"), paras([]Para{{Text: text, RPr: `sz="1600"`, RKids: solid("000000")}}))
	}
	s.Raw(fmt.Sprintf(`<p:grpSp><p:nvGrpSpPr><p:cNvPr id="%d" name="Group"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/><a:chOff x="0" y="0"/><a:chExt cx="%d" cy="%d"/></a:xfrm></p:grpSpPr>%s%s</p:grpSp>`,
		grp, Px(60), Px(380), Px(440), Px(80), Px(440), Px(80), inGroup(g1, "Grouped left", 0, "Grouped left"), inGroup(g2, "Grouped right", Px(240), "Grouped right")))
	btn := s.Shape("Button", "roundRect", Box{X: 600, Y: 380, W: 240, H: 80}, solid("4472C4"), "", Para{Text: "Show the hint", RPr: `sz="1600"`})
	hint := s.Shape("Hint", "rect", Box{X: 880, Y: 380, W: 300, H: 80}, solid("FFC000"), "", Para{Text: "A hint only the button shows", RPr: `sz="1600"`, RKids: solid("000000")})
	tl = &Timeline{}
	tl.Add(entr(grp, -1))
	tl.Trigger(btn, entr(hint, -1))
	s.Timing = tl.XML()
	spec.Slides = append(spec.Slides, ContentSlide{Blocks: []ContentBlock{
		{Role: "text", Text: "Free text on a slide without a title placeholder"},
		{Role: "alt", Text: "A gradient picture with a circle"},
		{Role: "text", Text: "Cell one"}, {Role: "text", Text: "Cell two"}, {Role: "text", Text: "Cell three"}, {Role: "text", Text: "Cell four"},
		{Role: "text", Text: "Grouped left", Step: 1}, {Role: "text", Text: "Grouped right", Step: 1},
		{Role: "text", Text: "Show the hint"},
		{Role: "text", Text: "A hint only the button shows", Step: -1},
	}})

	// slide 4 is hidden: it is not in the content model, and a link to it is no link
	s = d.Add(LayoutContent)
	s.Hidden = true
	s.Placeholder("title", Para{Text: "This slide is hidden"})

	// slide 5: links to other slides, one per paragraph, and one on a shape
	s = d.Add(LayoutContent)
	s.Placeholder("title", Para{Text: "Links to other slides"})
	link := func(text, kids string) Para { return Para{Text: text, RPr: `sz="2000"`, RKids: kids} }
	s.TextBox("Links", Box{X: 120, Y: 180, W: 500, H: 400}, "",
		link("Next slide", showJump("nextslide")),
		link("Previous slide", showJump("previousslide")),
		link("First slide", showJump("firstslide")),
		link("Last slide", showJump("lastslide")),
		link("Last slide viewed", showJump("lastslideviewed")),
		link("End the show", showJump("endshow")),
		link("The lists slide", s.slideJump(2)),
		link("The hidden slide", s.slideJump(4)))
	jb := s.Shape("Jump button", "roundRect", Box{X: 760, Y: 260, W: 300, H: 100}, solid("4472C4"), "", Para{Text: "To the free text", RPr: `sz="2000"`})
	last = len(s.Shapes) - 1
	s.Shapes[last] = strings.Replace(s.Shapes[last], fmt.Sprintf(`<p:cNvPr id="%d" name="Jump button"/>`, jb), fmt.Sprintf(`<p:cNvPr id="%d" name="Jump button">%s</p:cNvPr>`, jb, s.slideJump(3)), 1)
	spec.Slides = append(spec.Slides, ContentSlide{Title: "Links to other slides", Blocks: []ContentBlock{
		{Role: "title", Text: "Links to other slides"},
		{Role: "text", Text: "Next slide", Jump: &ContentJump{To: "next"}},
		{Role: "text", Text: "Previous slide", Jump: &ContentJump{To: "prev"}},
		{Role: "text", Text: "First slide", Jump: &ContentJump{To: "first"}},
		{Role: "text", Text: "Last slide", Jump: &ContentJump{To: "last"}},
		{Role: "text", Text: "Last slide viewed", Jump: &ContentJump{To: "back"}},
		{Role: "text", Text: "End the show", Jump: &ContentJump{To: "end"}},
		{Role: "text", Text: "The lists slide", Jump: &ContentJump{Slide: at(1)}},
		{Role: "text", Text: "The hidden slide"},
		{Role: "text", Text: "To the free text", Jump: &ContentJump{Slide: at(2)}},
	}})

	// slide 6: the last slide
	s = d.Add(LayoutContent)
	s.Placeholder("title", Para{Text: "The last slide"})
	s.Placeholder("body", Para{Text: "Nothing follows this slide"})
	spec.Slides = append(spec.Slides, ContentSlide{Title: "The last slide", Blocks: []ContentBlock{
		{Role: "title", Text: "The last slide"},
		{Role: "item", Text: "Nothing follows this slide"},
	}})
	return d, spec
}
