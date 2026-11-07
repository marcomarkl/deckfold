// Package model reads a presentation into slides, layouts, masters and themes and resolves the
// placeholder inheritance (slide -> layout -> master).
package model

import (
	"errors"
	"fmt"
	"strings"

	"deckfold/internal/pptx/opc"
	"deckfold/internal/pptx/xmltree"
)

// Part is an XML part with its relationships.
type Part struct {
	Name string
	Root *xmltree.Node
	Rels map[string]opc.Rel
}

// Theme holds the parts of a theme the renderer needs.
type Theme struct {
	Colors       map[string]string // dk1, lt1, dk2, lt2, accent1..6, hlink, folHlink -> RRGGBB
	MajorLatin   string
	MinorLatin   string
	MajorEA      string
	MinorEA      string
	MajorCS      string
	MinorCS      string
	FillStyles   []*xmltree.Node // a:fillStyleLst children (idx 1..)
	BgFillStyles []*xmltree.Node // a:bgFillStyleLst children (idx 1001..)
	LineStyles   []*xmltree.Node
	EffectStyles []*xmltree.Node
}

// Master is a slide master.
type Master struct {
	Part
	Theme  *Theme
	ClrMap map[string]string
	Title  *xmltree.Node // p:titleStyle
	Body   *xmltree.Node // p:bodyStyle
	Other  *xmltree.Node // p:otherStyle
}

// Layout is a slide layout.
type Layout struct {
	Part
	Master *Master
}

// Slide is a slide.
type Slide struct {
	Part
	Number int  // position in the file, 1-based
	Hidden bool // show="0"
	Layout *Layout
	Notes  string
	// ClrMap is the effective color map (master map or the slide's override).
	ClrMap map[string]string
}

// Presentation is the read presentation.
type Presentation struct {
	Pkg              *opc.Package
	Width, Height    int64 // EMU
	FirstSlideNum    int
	Slides           []*Slide
	DefaultTextStyle *xmltree.Node
	Loop             bool
	UseTimings       bool
	EmbeddedFonts    []EmbeddedFont
	Title            string
}

// EmbeddedFont is a font stored in the presentation (p:embeddedFontLst).
type EmbeddedFont struct {
	Typeface string
	Style    string // regular, bold, italic, boldItalic
	Part     string
}

// ErrStructure marks a package without the parts a presentation needs.
var ErrStructure = errors.New("not a PowerPoint presentation")

// Read reads the presentation of a package.
func Read(pkg *opc.Package) (*Presentation, error) {
	root, err := pkg.Rels("")
	if err != nil {
		return nil, err
	}
	docs := opc.ByType(root, "officeDocument")
	if len(docs) == 0 {
		return nil, fmt.Errorf("%w: no main document", ErrStructure)
	}
	presName := docs[0].Target
	ct := pkg.ContentType(presName)
	if !strings.Contains(ct, "presentationml") {
		return nil, fmt.Errorf("%w: main document has content type %q", ErrStructure, ct)
	}
	pres, err := pkg.XML(presName)
	if err != nil {
		return nil, err
	}
	presRels, err := pkg.Rels(presName)
	if err != nil {
		return nil, err
	}
	p := &Presentation{Pkg: pkg, Width: 12192000, Height: 6858000, FirstSlideNum: 1, UseTimings: true}
	if sz := pres.Child("sldSz"); sz != nil {
		p.Width, p.Height = sz.Int("cx", p.Width), sz.Int("cy", p.Height)
	}
	if p.Width <= 0 || p.Height <= 0 || p.Width > 51206400 || p.Height > 51206400 {
		return nil, fmt.Errorf("%w: invalid slide size", ErrStructure)
	}
	p.FirstSlideNum = int(pres.Int("firstSlideNum", 1))
	p.DefaultTextStyle = pres.Child("defaultTextStyle")
	if c, ok := pkg.Rels(""); ok == nil {
		for _, r := range opc.ByType(c, "core-properties") {
			if n, err := pkg.XML(r.Target); err == nil {
				p.Title = strings.TrimSpace(n.Child("title").AllText())
			}
		}
	}
	for _, r := range opc.ByType(presRels, "presProps") {
		if n, err := pkg.XML(r.Target); err == nil {
			if sp := n.Child("showPr"); sp != nil {
				p.Loop = sp.Bool("loop", false)
				p.UseTimings = sp.Bool("useTimings", true)
			}
		}
	}
	for _, ef := range pres.Path("embeddedFontLst").Children("embeddedFont") {
		face := ef.Child("font").Attr("typeface")
		for _, st := range []string{"regular", "bold", "italic", "boldItalic"} {
			if n := ef.Child(st); n != nil {
				if r, ok := presRels[n.RID("id")]; ok && !r.External {
					p.EmbeddedFonts = append(p.EmbeddedFonts, EmbeddedFont{Typeface: face, Style: st, Part: r.Target})
				}
			}
		}
	}

	c := &cache{pkg: pkg, masters: map[string]*Master{}, layouts: map[string]*Layout{}, themes: map[string]*Theme{}}
	for i, id := range pres.Path("sldIdLst").Children("sldId") {
		r, ok := presRels[id.RID("id")]
		if !ok || r.External {
			return nil, fmt.Errorf("%w: slide %d has no part", ErrStructure, i+1)
		}
		s, err := c.slide(r.Target, i+1)
		if err != nil {
			return nil, err
		}
		p.Slides = append(p.Slides, s)
	}
	return p, nil
}

type cache struct {
	pkg     *opc.Package
	masters map[string]*Master
	layouts map[string]*Layout
	themes  map[string]*Theme
}

func (c *cache) part(name string) (Part, error) {
	n, err := c.pkg.XML(name)
	if err != nil {
		return Part{}, err
	}
	rels, err := c.pkg.Rels(name)
	if err != nil {
		return Part{}, err
	}
	return Part{Name: name, Root: n, Rels: rels}, nil
}

func (c *cache) slide(name string, num int) (*Slide, error) {
	pt, err := c.part(name)
	if err != nil {
		return nil, err
	}
	s := &Slide{Part: pt, Number: num, Hidden: !pt.Root.Bool("show", true)}
	ls := opc.ByType(pt.Rels, "slideLayout")
	if len(ls) == 0 {
		return nil, fmt.Errorf("%w: slide %d has no layout", ErrStructure, num)
	}
	if s.Layout, err = c.layout(ls[0].Target); err != nil {
		return nil, err
	}
	s.ClrMap = s.Layout.Master.ClrMap
	if ov := pt.Root.Path("clrMapOvr/overrideClrMapping"); ov != nil {
		s.ClrMap = clrMap(ov)
	}
	for _, r := range opc.ByType(pt.Rels, "notesSlide") {
		if n, err := c.pkg.XML(r.Target); err == nil {
			s.Notes = notesText(n)
		}
	}
	return s, nil
}

func (c *cache) layout(name string) (*Layout, error) {
	if l, ok := c.layouts[name]; ok {
		return l, nil
	}
	pt, err := c.part(name)
	if err != nil {
		return nil, err
	}
	l := &Layout{Part: pt}
	ms := opc.ByType(pt.Rels, "slideMaster")
	if len(ms) == 0 {
		return nil, fmt.Errorf("%w: layout %s has no master", ErrStructure, name)
	}
	if l.Master, err = c.master(ms[0].Target); err != nil {
		return nil, err
	}
	c.layouts[name] = l
	return l, nil
}

func (c *cache) master(name string) (*Master, error) {
	if m, ok := c.masters[name]; ok {
		return m, nil
	}
	pt, err := c.part(name)
	if err != nil {
		return nil, err
	}
	m := &Master{Part: pt, ClrMap: clrMap(pt.Root.Child("clrMap"))}
	ts := pt.Root.Child("txStyles")
	m.Title, m.Body, m.Other = ts.Child("titleStyle"), ts.Child("bodyStyle"), ts.Child("otherStyle")
	th := opc.ByType(pt.Rels, "theme")
	if len(th) == 0 {
		return nil, fmt.Errorf("%w: master %s has no theme", ErrStructure, name)
	}
	if m.Theme, err = c.theme(th[0].Target); err != nil {
		return nil, err
	}
	c.masters[name] = m
	return m, nil
}

func (c *cache) theme(name string) (*Theme, error) {
	if t, ok := c.themes[name]; ok {
		return t, nil
	}
	n, err := c.pkg.XML(name)
	if err != nil {
		return nil, err
	}
	t := &Theme{Colors: map[string]string{}}
	el := n.Child("themeElements")
	if cs := el.Child("clrScheme"); cs != nil {
		for _, k := range cs.Kids {
			if len(k.Kids) == 0 {
				continue
			}
			v := k.Kids[0]
			switch v.Name {
			case "srgbClr":
				t.Colors[k.Name] = strings.ToUpper(v.Attr("val"))
			case "sysClr":
				t.Colors[k.Name] = strings.ToUpper(v.Attr("lastClr"))
				if t.Colors[k.Name] == "" {
					if v.Attr("val") == "window" {
						t.Colors[k.Name] = "FFFFFF"
					} else {
						t.Colors[k.Name] = "000000"
					}
				}
			}
		}
	}
	if fs := el.Child("fontScheme"); fs != nil {
		t.MajorLatin = fs.Path("majorFont/latin").Attr("typeface")
		t.MinorLatin = fs.Path("minorFont/latin").Attr("typeface")
		t.MajorEA = fs.Path("majorFont/ea").Attr("typeface")
		t.MinorEA = fs.Path("minorFont/ea").Attr("typeface")
		t.MajorCS = fs.Path("majorFont/cs").Attr("typeface")
		t.MinorCS = fs.Path("minorFont/cs").Attr("typeface")
	}
	if fm := el.Child("fmtScheme"); fm != nil {
		t.FillStyles = fm.Child("fillStyleLst").Elems()
		t.BgFillStyles = fm.Child("bgFillStyleLst").Elems()
		t.LineStyles = fm.Child("lnStyleLst").Elems()
		t.EffectStyles = fm.Child("effectStyleLst").Elems()
	}
	c.themes[name] = t
	return t, nil
}

func clrMap(n *xmltree.Node) map[string]string {
	m := map[string]string{"bg1": "lt1", "tx1": "dk1", "bg2": "lt2", "tx2": "dk2"}
	if n == nil {
		return m
	}
	for _, a := range n.Attrs {
		m[a.Name.Local] = a.Value
	}
	return m
}

// notesText returns the text of the body placeholder of a notes slide, paragraphs on lines.
func notesText(n *xmltree.Node) string {
	for _, sp := range n.Path("cSld/spTree").Children("sp") {
		ph := sp.Path("nvSpPr/nvPr/ph")
		if ph == nil || ph.Attr("type") != "body" {
			continue
		}
		var lines []string
		for _, p := range sp.Path("txBody").Children("p") {
			var b strings.Builder
			for _, k := range p.Kids {
				switch k.Name {
				case "r", "fld":
					b.WriteString(k.Child("t").AllText())
				case "br":
					b.WriteString("\n")
				}
			}
			lines = append(lines, b.String())
		}
		return strings.TrimRight(strings.Join(lines, "\n"), "\n ")
	}
	return ""
}

// PhType returns the effective placeholder type of a p:ph element ("obj" when not given).
func PhType(ph *xmltree.Node) string {
	if t := ph.Attr("type"); t != "" {
		return t
	}
	return "obj"
}

// masterType maps a placeholder type to the type used on the master.
func masterType(t string) string {
	switch t {
	case "ctrTitle", "title":
		return "title"
	case "dt", "ftr", "sldNum", "sldImg", "hdr":
		return t
	}
	return "body"
}

// findPh finds a placeholder shape in a shape tree: by idx when given, then by type.
func findPh(tree *xmltree.Node, typ, idx string, byIdxOnly bool) *xmltree.Node {
	var byType *xmltree.Node
	for _, sp := range shapes(tree) {
		ph := phOf(sp)
		if ph == nil {
			continue
		}
		if idx != "" && idx != "0" && ph.Attr("idx") == idx {
			return sp
		}
		if !byIdxOnly && byType == nil && PhType(ph) == typ {
			byType = sp
		}
	}
	return byType
}

func shapes(tree *xmltree.Node) []*xmltree.Node {
	var out []*xmltree.Node
	for _, k := range tree.Kids {
		switch k.Name {
		case "sp", "pic", "graphicFrame", "cxnSp":
			out = append(out, k)
		case "grpSp":
			out = append(out, shapes(k)...)
		}
	}
	return out
}

// phOf returns the p:ph element of a shape or nil.
func phOf(sp *xmltree.Node) *xmltree.Node {
	for _, nv := range []string{"nvSpPr", "nvPicPr", "nvGraphicFramePr", "nvCxnSpPr"} {
		if n := sp.Path(nv + "/nvPr/ph"); n != nil {
			return n
		}
	}
	return nil
}

// Ph returns the p:ph element of a shape or nil.
func Ph(sp *xmltree.Node) *xmltree.Node { return phOf(sp) }

// Inherited returns the layout and master placeholder shapes a slide placeholder inherits from
// (each may be nil).
func (s *Slide) Inherited(ph *xmltree.Node) (layout, master *xmltree.Node) {
	typ, idx := PhType(ph), ph.Attr("idx")
	layout = findPh(s.Layout.Root.Path("cSld/spTree"), typ, idx, false)
	if layout == nil && typ == "obj" {
		layout = findPh(s.Layout.Root.Path("cSld/spTree"), "body", idx, false)
	}
	mtyp := masterType(typ)
	if layout != nil {
		mtyp = masterType(PhType(phOf(layout)))
	}
	master = findPh(s.Layout.Master.Root.Path("cSld/spTree"), mtyp, "", false)
	return layout, master
}

// LayoutInherited returns the master placeholder a layout placeholder inherits from.
func (l *Layout) Inherited(ph *xmltree.Node) *xmltree.Node {
	return findPh(l.Master.Root.Path("cSld/spTree"), masterType(PhType(ph)), "", false)
}

// TextStyle returns the master text style for a placeholder type ("" for shapes that are no
// placeholder): titleStyle, bodyStyle or otherStyle.
func (m *Master) TextStyle(phType string) *xmltree.Node {
	switch phType {
	case "":
		return nil
	case "title", "ctrTitle":
		return m.Title
	case "body", "subTitle", "obj", "tbl", "chart", "dgm", "media", "clipArt", "pic":
		return m.Body
	}
	return m.Other
}

// Visible returns the slides that are shown, in order.
func (p *Presentation) Visible() []*Slide {
	var out []*Slide
	for _, s := range p.Slides {
		if !s.Hidden {
			out = append(out, s)
		}
	}
	return out
}
