// Package pptxgen writes small PowerPoint files for tests and for the public feature decks in
// fixtures/pptx. It produces the parts PowerPoint itself writes for a new presentation (master,
// one layout per placeholder arrangement, theme, notes master) and lets a deck add shapes,
// pictures, notes, timing and transitions as raw PresentationML.
//
// All text uses the fonts deckfold bundles (Carlito, Liberation), so that a reference image made
// in PowerPoint and deckfold's own rendering use the same fonts.
package pptxgen

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"strings"
	"time"
)

// EMU per pixel at 96 dpi.
const EMU = 9525

// Px converts pixels at 96 dpi to EMU.
func Px(v float64) int64 { return int64(v * EMU) }

// Layout names the placeholder arrangement of a slide.
type Layout int

const (
	LayoutTitle   Layout = iota // title slide: centered title and subtitle
	LayoutContent               // title and content
	LayoutBlank                 // no placeholders
)

// Deck is a presentation under construction.
type Deck struct {
	Width, Height int64 // slide size in EMU (default 12192000 x 6858000, 16:9)
	Title         string
	HeadFont      string // theme major font (default Carlito)
	BodyFont      string // theme minor font (default Carlito)
	FirstSlideNum int    // first slide number (0 = default 1)
	ShowLoop      bool   // presentation property: loop until Escape
	TableStyles   string // raw a:tblStyle elements for ppt/tableStyles.xml
	UseTimings    *bool  // presentation property: use rehearsed timings (nil = default true)
	// NoStyleKern leaves kern out of every text style (master, other and default text style),
	// where PowerPoint and the other decks have kern="1200". The kerning deck sets kern on its
	// runs itself, and needs one row that inherits none.
	NoStyleKern bool
	// Fonts are embedded as ppt/fonts/fontN.fntdata (EOT data, see fonts.EOT).
	Fonts  []Font
	Slides []*Slide
	media  []media
	extra  []extraPart
}

type extraPart struct{ name, contentType, data string }

// Font is an embedded font: the typeface name and the regular style as EOT data.
type Font struct {
	Typeface string
	Regular  []byte
}

// Part adds an XML part (for example ppt/charts/chart1.xml) with its content type.
func (d *Deck) Part(name, contentType, data string) {
	d.extra = append(d.extra, extraPart{name, contentType, data})
}

type media struct {
	name, contentType string
	data              []byte
}

// Slide is one slide.
type Slide struct {
	Layout     Layout
	Hidden     bool
	Notes      string // plain text, paragraphs separated by "\n"
	Shapes     []string
	Timing     string // raw <p:timing> element
	Transition string // raw <p:transition> element (or mc:AlternateContent wrapping it)
	Background string // raw <p:bg> element
	rels       []rel
	nextID     int
}

type rel struct {
	id, typ, target, mode string
}

const (
	relImage     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"
	relHyperlink = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink"
	relLayout    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout"
	relNotes     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide"
	relVideo     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/video"
	relAudio     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/audio"
	relMedia     = "http://schemas.microsoft.com/office/2007/relationships/media"
)

// New returns a 16:9 deck.
func New(title string) *Deck {
	return &Deck{Width: 12192000, Height: 6858000, Title: title, HeadFont: "Carlito", BodyFont: "Carlito"}
}

// Add appends a slide with the given layout.
func (d *Deck) Add(l Layout) *Slide {
	s := &Slide{Layout: l, nextID: 2}
	d.Slides = append(d.Slides, s)
	return s
}

// ID returns the next free shape id on the slide.
func (s *Slide) ID() int { id := s.nextID; s.nextID++; return id }

// Rel adds a relationship and returns its id.
func (s *Slide) Rel(typ, target, mode string) string {
	id := fmt.Sprintf("rId%d", len(s.rels)+2) // rId1 is the layout
	s.rels = append(s.rels, rel{id, typ, target, mode})
	return id
}

// Media stores a media file in ppt/media and returns its name there (deduplicated by content).
func (d *Deck) Media(ext, contentType string, data []byte) string {
	sum := sha256.Sum256(data)
	name := "m" + hex.EncodeToString(sum[:6]) + "." + ext
	for _, m := range d.media {
		if m.name == name {
			return name
		}
	}
	d.media = append(d.media, media{name, contentType, data})
	return name
}

// Esc escapes text for XML.
func Esc(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Para is one paragraph of plain text. RPr holds attributes of a:rPr (e.g. `sz="2400" b="1"`) and
// RKids its child elements (fills, fonts); PPr and PKids do the same for a:pPr.
type Para struct {
	Text  string
	RPr   string
	RKids string
	PPr   string
	PKids string
	Level int
}

func paras(ps []Para) string {
	var b strings.Builder
	for _, p := range ps {
		ppr := p.PPr
		if p.Level > 0 {
			ppr += fmt.Sprintf(` lvl="%d"`, p.Level)
		}
		b.WriteString("<a:p>")
		switch {
		case p.PKids != "":
			b.WriteString("<a:pPr " + strings.TrimSpace(ppr) + ">" + p.PKids + "</a:pPr>")
		case ppr != "":
			b.WriteString("<a:pPr " + strings.TrimSpace(ppr) + "/>")
		}
		if p.Text != "" {
			rpr := `<a:rPr lang="en-US" dirty="0" ` + p.RPr + `/>`
			if p.RKids != "" {
				rpr = `<a:rPr lang="en-US" dirty="0" ` + p.RPr + `>` + p.RKids + `</a:rPr>`
			}
			b.WriteString(`<a:r>` + rpr + `<a:t>` + Esc(p.Text) + `</a:t></a:r>`)
		}
		b.WriteString(`<a:endParaRPr lang="en-US" dirty="0"/></a:p>`)
	}
	return b.String()
}

// Placeholder adds a placeholder shape that inherits position and style from the layout.
// typ is "title", "ctrTitle", "subTitle" or "body" (idx 1).
func (s *Slide) Placeholder(typ string, ps ...Para) {
	id := s.ID()
	ph := `<p:ph type="` + typ + `"/>`
	switch typ {
	case "body":
		ph = `<p:ph idx="1"/>`
	case "subTitle":
		ph = `<p:ph type="subTitle" idx="1"/>`
	}
	s.Shapes = append(s.Shapes, fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="%s %d"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr>%s</p:nvPr></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/>%s</p:txBody></p:sp>`,
		id, typ, id, ph, paras(ps)))
}

// Box is a position in pixels at 96 dpi.
type Box struct{ X, Y, W, H float64 }

func (b Box) xfrm() string { return `<a:xfrm>` + b.xfrmInner() + `</a:xfrm>` }

func (b Box) xfrmInner() string {
	return fmt.Sprintf(`<a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/>`, Px(b.X), Px(b.Y), Px(b.W), Px(b.H))
}

// Shape adds a preset shape. fill and line are raw DrawingML (e.g. `<a:solidFill><a:srgbClr val="FF0000"/></a:solidFill>`);
// an empty line means no outline.
func (s *Slide) Shape(name, prst string, b Box, fill, line string, ps ...Para) int {
	id := s.ID()
	if line == "" {
		line = `<a:ln><a:noFill/></a:ln>`
	}
	tx := ""
	if len(ps) > 0 {
		tx = `<p:txBody><a:bodyPr rtlCol="0" anchor="ctr"/><a:lstStyle/>` + paras(ps) + `</p:txBody>`
	}
	s.Shapes = append(s.Shapes, fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="%s"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr>%s<a:prstGeom prst="%s"><a:avLst/></a:prstGeom>%s%s</p:spPr>%s</p:sp>`,
		id, Esc(name), b.xfrm(), prst, fill, line, tx))
	return id
}

// TextBox adds a text box without fill.
func (s *Slide) TextBox(name string, b Box, bodyPr string, ps ...Para) int {
	id := s.ID()
	if bodyPr == "" {
		bodyPr = `<a:bodyPr wrap="square" rtlCol="0"><a:spAutoFit/></a:bodyPr>`
	}
	s.Shapes = append(s.Shapes, fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="%s"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr><p:spPr>%s<a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/></p:spPr><p:txBody>%s<a:lstStyle/>%s</p:txBody></p:sp>`,
		id, Esc(name), b.xfrm(), bodyPr, paras(ps)))
	return id
}

// Picture adds a picture. blipExtra is raw content after a:blip (for example a:srcRect),
// fill is the fill mode (`<a:stretch><a:fillRect/></a:stretch>` when empty).
func (s *Slide) Picture(d *Deck, name string, b Box, ext, contentType string, data []byte, srcRect, fill string) int {
	id := s.ID()
	rid := s.Rel(relImage, "../media/"+d.Media(ext, contentType, data), "")
	if fill == "" {
		fill = `<a:stretch><a:fillRect/></a:stretch>`
	}
	s.Shapes = append(s.Shapes, fmt.Sprintf(`<p:pic><p:nvPicPr><p:cNvPr id="%d" name="%s"/><p:cNvPicPr><a:picLocks noChangeAspect="1"/></p:cNvPicPr><p:nvPr/></p:nvPicPr><p:blipFill><a:blip r:embed="%s"/>%s%s</p:blipFill><p:spPr>%s<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>`,
		id, Esc(name), rid, srcRect, fill, b.xfrm()))
	return id
}

// Raw adds raw shape-tree XML.
func (s *Slide) Raw(xml string) { s.Shapes = append(s.Shapes, xml) }

const nsDecl = `xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`

const xmlHead = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"

// Bytes returns the PPTX file.
func (d *Deck) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	// Fixed timestamps keep the output byte-identical between runs.
	stamp := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	put := func(name, content string) error {
		w, err := z.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: stamp})
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(content))
		return err
	}
	files := d.parts()
	for _, f := range files {
		if err := put(f[0], f[1]); err != nil {
			return nil, err
		}
	}
	for i, f := range d.Fonts {
		w, err := z.CreateHeader(&zip.FileHeader{Name: fmt.Sprintf("ppt/fonts/font%d.fntdata", i+1), Method: zip.Store, Modified: stamp})
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(f.Regular); err != nil {
			return nil, err
		}
	}
	for _, m := range d.media {
		w, err := z.CreateHeader(&zip.FileHeader{Name: "ppt/media/" + m.name, Method: zip.Store, Modified: stamp})
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(m.data); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Write writes the PPTX file.
func (d *Deck) Write(path string) error {
	b, err := d.Bytes()
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func (d *Deck) parts() [][2]string {
	var out [][2]string
	add := func(name, content string) { out = append(out, [2]string{name, content}) }

	// content types
	var ct strings.Builder
	ct.WriteString(xmlHead + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`)
	ct.WriteString(`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/>`)
	exts := map[string]bool{}
	if len(d.Fonts) > 0 {
		ct.WriteString(`<Default Extension="fntdata" ContentType="application/x-fontdata"/>`)
	}
	for _, m := range d.media {
		ext := m.name[strings.LastIndex(m.name, ".")+1:]
		if !exts[ext] {
			exts[ext] = true
			ct.WriteString(`<Default Extension="` + ext + `" ContentType="` + m.contentType + `"/>`)
		}
	}
	ov := func(part, typ string) {
		ct.WriteString(`<Override PartName="/` + part + `" ContentType="application/vnd.openxmlformats-` + typ + `"/>`)
	}
	ov("ppt/presentation.xml", "officedocument.presentationml.presentation.main+xml")
	ov("ppt/slideMasters/slideMaster1.xml", "officedocument.presentationml.slideMaster+xml")
	for i := range layouts {
		ov(fmt.Sprintf("ppt/slideLayouts/slideLayout%d.xml", i+1), "officedocument.presentationml.slideLayout+xml")
	}
	ov("ppt/theme/theme1.xml", "officedocument.theme+xml")
	ov("ppt/theme/theme2.xml", "officedocument.theme+xml")
	ov("ppt/notesMasters/notesMaster1.xml", "officedocument.presentationml.notesMaster+xml")
	ov("ppt/presProps.xml", "officedocument.presentationml.presProps+xml")
	ov("ppt/viewProps.xml", "officedocument.presentationml.viewProps+xml")
	ov("ppt/tableStyles.xml", "officedocument.presentationml.tableStyles+xml")
	ov("docProps/core.xml", "package.core-properties+xml")
	ov("docProps/app.xml", "officedocument.extended-properties+xml")
	for i, s := range d.Slides {
		ov(fmt.Sprintf("ppt/slides/slide%d.xml", i+1), "officedocument.presentationml.slide+xml")
		if s.Notes != "" {
			ov(fmt.Sprintf("ppt/notesSlides/notesSlide%d.xml", i+1), "officedocument.presentationml.notesSlide+xml")
		}
	}
	for _, e := range d.extra {
		ct.WriteString(`<Override PartName="/` + e.name + `" ContentType="` + e.contentType + `"/>`)
	}
	ct.WriteString(`</Types>`)
	add("[Content_Types].xml", ct.String())
	for _, e := range d.extra {
		add(e.name, e.data)
	}

	add("_rels/.rels", xmlHead+`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>`+
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>`+
		`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>`+
		`</Relationships>`)
	add("docProps/core.xml", xmlHead+`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><dc:title>`+Esc(d.Title)+`</dc:title><dc:creator>deckfold pptxgen</dc:creator><dcterms:created xsi:type="dcterms:W3CDTF">2026-01-01T00:00:00Z</dcterms:created></cp:coreProperties>`)
	add("docProps/app.xml", xmlHead+`<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties"><Application>deckfold pptxgen</Application><Slides>`+fmt.Sprint(len(d.Slides))+`</Slides></Properties>`)

	// presentation
	var pres, presRels strings.Builder
	presRels.WriteString(xmlHead + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	presRels.WriteString(`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>`)
	presRels.WriteString(`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesMaster" Target="notesMasters/notesMaster1.xml"/>`)
	presRels.WriteString(`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>`)
	presRels.WriteString(`<Relationship Id="rId4" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/presProps" Target="presProps.xml"/>`)
	presRels.WriteString(`<Relationship Id="rId5" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/viewProps" Target="viewProps.xml"/>`)
	presRels.WriteString(`<Relationship Id="rId6" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/tableStyles" Target="tableStyles.xml"/>`)
	first := ""
	if d.FirstSlideNum > 1 {
		first = fmt.Sprintf(` firstSlideNum="%d"`, d.FirstSlideNum)
	}
	embed := ""
	if len(d.Fonts) > 0 {
		embed = ` embedTrueTypeFonts="1"`
	}
	pres.WriteString(xmlHead + `<p:presentation ` + nsDecl + ` saveSubsetFonts="1"` + first + embed + `><p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst><p:notesMasterIdLst><p:notesMasterId r:id="rId2"/></p:notesMasterIdLst><p:sldIdLst>`)
	for i := range d.Slides {
		rid := fmt.Sprintf("rId%d", 100+i)
		pres.WriteString(fmt.Sprintf(`<p:sldId id="%d" r:id="%s"/>`, 256+i, rid))
		presRels.WriteString(`<Relationship Id="` + rid + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide` + fmt.Sprint(i+1) + `.xml"/>`)
	}
	pres.WriteString(fmt.Sprintf(`</p:sldIdLst><p:sldSz cx="%d" cy="%d"/><p:notesSz cx="6858000" cy="9144000"/>`, d.Width, d.Height))
	if len(d.Fonts) > 0 {
		pres.WriteString(`<p:embeddedFontLst>`)
		for i, f := range d.Fonts {
			rid := fmt.Sprintf("rId%d", 50+i)
			pres.WriteString(`<p:embeddedFont><p:font typeface="` + Esc(f.Typeface) + `" pitchFamily="18" charset="0"/><p:regular r:id="` + rid + `"/></p:embeddedFont>`)
			presRels.WriteString(`<Relationship Id="` + rid + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/font" Target="fonts/font` + fmt.Sprint(i+1) + `.fntdata"/>`)
		}
		pres.WriteString(`</p:embeddedFontLst>`)
	}
	pres.WriteString(`<p:defaultTextStyle>` + d.levels("", 1800) + `</p:defaultTextStyle></p:presentation>`)
	presRels.WriteString(`</Relationships>`)
	add("ppt/presentation.xml", pres.String())
	add("ppt/_rels/presentation.xml.rels", presRels.String())

	show := `<p:showPr showNarration="1"`
	if d.ShowLoop {
		show += ` loop="1"`
	}
	if d.UseTimings != nil && !*d.UseTimings {
		show += ` useTimings="0"`
	}
	show += `><p:present/><p:sldAll/></p:showPr>`
	add("ppt/presProps.xml", xmlHead+`<p:presentationPr `+nsDecl+`>`+show+`</p:presentationPr>`)
	add("ppt/viewProps.xml", xmlHead+`<p:viewPr `+nsDecl+`><p:normalViewPr><p:restoredLeft sz="15620"/><p:restoredTop sz="94660"/></p:normalViewPr><p:gridSpacing cx="76200" cy="76200"/></p:viewPr>`)
	tblStyles := `<a:tblStyleLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" def="{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}"/>`
	if d.TableStyles != "" {
		tblStyles = strings.TrimSuffix(tblStyles, "/>") + ">" + d.TableStyles + "</a:tblStyleLst>"
	}
	add("ppt/tableStyles.xml", xmlHead+tblStyles)

	add("ppt/theme/theme1.xml", theme(d.HeadFont, d.BodyFont))
	add("ppt/theme/theme2.xml", theme(d.HeadFont, d.BodyFont))
	add("ppt/slideMasters/slideMaster1.xml", master(d))
	var mr strings.Builder
	mr.WriteString(xmlHead + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for i := range layouts {
		mr.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="%s" Target="../slideLayouts/slideLayout%d.xml"/>`, i+1, relLayout, i+1))
	}
	mr.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/></Relationships>`, len(layouts)+1))
	add("ppt/slideMasters/_rels/slideMaster1.xml.rels", mr.String())
	for i, l := range layouts {
		add(fmt.Sprintf("ppt/slideLayouts/slideLayout%d.xml", i+1), l(d))
		add(fmt.Sprintf("ppt/slideLayouts/_rels/slideLayout%d.xml.rels", i+1), xmlHead+`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>`)
	}
	add("ppt/notesMasters/notesMaster1.xml", notesMaster(d))
	add("ppt/notesMasters/_rels/notesMaster1.xml.rels", xmlHead+`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme2.xml"/></Relationships>`)

	for i, s := range d.Slides {
		n := i + 1
		var sx strings.Builder
		show := ""
		if s.Hidden {
			show = ` show="0"`
		}
		sx.WriteString(xmlHead + `<p:sld ` + nsDecl + ` xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main"` + show + `><p:cSld>`)
		sx.WriteString(s.Background)
		sx.WriteString(`<p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>`)
		for _, sh := range s.Shapes {
			sx.WriteString(sh)
		}
		sx.WriteString(`</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>`)
		sx.WriteString(s.Transition)
		sx.WriteString(s.Timing)
		sx.WriteString(`</p:sld>`)
		add(fmt.Sprintf("ppt/slides/slide%d.xml", n), sx.String())

		var rx strings.Builder
		rx.WriteString(xmlHead + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
		rx.WriteString(fmt.Sprintf(`<Relationship Id="rId1" Type="%s" Target="../slideLayouts/slideLayout%d.xml"/>`, relLayout, int(s.Layout)+1))
		for _, r := range s.rels {
			mode := ""
			if r.mode != "" {
				mode = ` TargetMode="` + r.mode + `"`
			}
			rx.WriteString(`<Relationship Id="` + r.id + `" Type="` + r.typ + `" Target="` + Esc(r.target) + `"` + mode + `/>`)
		}
		if s.Notes != "" {
			rx.WriteString(fmt.Sprintf(`<Relationship Id="rIdN" Type="%s" Target="../notesSlides/notesSlide%d.xml"/>`, relNotes, n))
		}
		rx.WriteString(`</Relationships>`)
		add(fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", n), rx.String())

		if s.Notes != "" {
			var ps []Para
			for _, line := range strings.Split(s.Notes, "\n") {
				ps = append(ps, Para{Text: line})
			}
			add(fmt.Sprintf("ppt/notesSlides/notesSlide%d.xml", n), xmlHead+`<p:notes `+nsDecl+`><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/>`+
				`<p:sp><p:nvSpPr><p:cNvPr id="2" name="Slide Image Placeholder 1"/><p:cNvSpPr><a:spLocks noGrp="1" noRot="1" noChangeAspect="1"/></p:cNvSpPr><p:nvPr><p:ph type="sldImg"/></p:nvPr></p:nvSpPr><p:spPr/></p:sp>`+
				`<p:sp><p:nvSpPr><p:cNvPr id="3" name="Notes Placeholder 2"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/>`+paras(ps)+`</p:txBody></p:sp>`+
				`</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:notes>`)
			add(fmt.Sprintf("ppt/notesSlides/_rels/notesSlide%d.xml.rels", n), xmlHead+`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+
				`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesMaster" Target="../notesMasters/notesMaster1.xml"/>`+
				fmt.Sprintf(`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="../slides/slide%d.xml"/>`, n)+
				`</Relationships>`)
		}
	}
	return out
}

// styleKern returns the kern attribute of the text styles (with its leading space).
func (d *Deck) styleKern() string {
	if d.NoStyleKern {
		return ""
	}
	return ` kern="1200"`
}

// levels returns a:lvl1pPr..a:lvl9pPr with the given size (in hundredths of a point).
func (d *Deck) levels(extra string, size int) string {
	var b strings.Builder
	b.WriteString(`<a:defPPr><a:defRPr lang="en-US"/></a:defPPr>`)
	for l := 1; l <= 9; l++ {
		b.WriteString(fmt.Sprintf(`<a:lvl%dpPr marL="%d" algn="l" defTabSz="914400" rtl="0" eaLnBrk="1" latinLnBrk="0" hangingPunct="1"%s><a:defRPr sz="%d"`+d.styleKern()+`><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="+mn-lt"/><a:ea typeface="+mn-ea"/><a:cs typeface="+mn-cs"/></a:defRPr></a:lvl%dpPr>`,
			l, (l-1)*457200, extra, size, l))
	}
	return b.String()
}

// MediaShape adds a video or audio shape ("video" or "audio") with a poster picture. The medium
// is stored in ppt/media and referenced the way PowerPoint 2010 and later do it (a:videoFile or
// a:audioFile with r:link, plus p14:media with r:embed).
func (s *Slide) MediaShape(d *Deck, kind, name string, b Box, ext, contentType string, data, poster []byte) int {
	id := s.ID()
	m := "../media/" + d.Media(ext, contentType, data)
	typ, el := relVideo, "videoFile"
	if kind == "audio" {
		typ, el = relAudio, "audioFile"
	}
	link := s.Rel(typ, m, "")
	emb := s.Rel(relMedia, m, "")
	img := s.Rel(relImage, "../media/"+d.Media("png", "image/png", poster), "")
	s.Shapes = append(s.Shapes, fmt.Sprintf(`<p:pic><p:nvPicPr><p:cNvPr id="%d" name="%s"><a:hlinkClick r:id="" action="ppaction://media"/></p:cNvPr><p:cNvPicPr><a:picLocks noChangeAspect="1"/></p:cNvPicPr>`+
		`<p:nvPr><a:%s r:link="%s"/><p:extLst><p:ext uri="{DAA4B4D4-6D71-4841-9C94-3DE7FCFB9230}"><p14:media xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main" r:embed="%s"/></p:ext></p:extLst></p:nvPr></p:nvPicPr>`+
		`<p:blipFill><a:blip r:embed="%s"/><a:stretch><a:fillRect/></a:stretch></p:blipFill><p:spPr>%s<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>`,
		id, Esc(name), el, link, emb, img, b.xfrm()))
	return id
}

// AutoPlay returns a p:timing element that starts the media shape when the slide is shown
// (an after-previous media call, as PowerPoint writes for "Start: Automatically").
func AutoPlay(spid, ms int) string {
	return fmt.Sprintf(`<p:timing><p:tnLst><p:par><p:cTn id="1" dur="indefinite" restart="never" nodeType="tmRoot"><p:childTnLst>`+
		`<p:seq concurrent="1" nextAc="seek"><p:cTn id="2" dur="indefinite" nodeType="mainSeq"><p:childTnLst>`+
		`<p:par><p:cTn id="3" fill="hold"><p:stCondLst><p:cond delay="indefinite"/><p:cond evt="onBegin" delay="0"><p:tn val="2"/></p:cond></p:stCondLst><p:childTnLst>`+
		`<p:par><p:cTn id="4" fill="hold"><p:stCondLst><p:cond delay="0"/></p:stCondLst><p:childTnLst>`+
		`<p:par><p:cTn id="5" presetID="1" presetClass="mediacall" presetSubtype="0" fill="hold" nodeType="afterEffect"><p:stCondLst><p:cond delay="0"/></p:stCondLst><p:childTnLst>`+
		`<p:cmd type="call" cmd="playFrom(0.0)"><p:cBhvr><p:cTn id="6" dur="%d" fill="hold"/><p:tgtEl><p:spTgt spid="%d"/></p:tgtEl></p:cBhvr></p:cmd>`+
		`</p:childTnLst></p:cTn></p:par></p:childTnLst></p:cTn></p:par></p:childTnLst></p:cTn></p:par>`+
		`</p:childTnLst></p:cTn><p:prevCondLst><p:cond evt="onPrev" delay="0"><p:tgtEl><p:sldTgt/></p:tgtEl></p:cond></p:prevCondLst>`+
		`<p:nextCondLst><p:cond evt="onNext" delay="0"><p:tgtEl><p:sldTgt/></p:tgtEl></p:cond></p:nextCondLst></p:seq>`+
		`</p:childTnLst></p:cTn></p:par></p:tnLst></p:timing>`, ms, spid)
}

// WAV returns a mono 8 kHz WAV file with a 440 Hz tone.
func WAV(seconds float64) []byte {
	rate := 8000
	n := int(float64(rate) * seconds)
	b := make([]byte, 44+2*n)
	copy(b, "RIFF")
	le32(b[4:], uint32(36+2*n))
	copy(b[8:], "WAVEfmt ")
	le32(b[16:], 16)
	b[20], b[22] = 1, 1
	le32(b[24:], uint32(rate))
	le32(b[28:], uint32(2*rate))
	b[32], b[34] = 2, 16
	copy(b[36:], "data")
	le32(b[40:], uint32(2*n))
	for i := 0; i < n; i++ {
		v := int16(3000 * math.Sin(float64(i)/float64(rate)*2*math.Pi*440))
		b[44+2*i], b[45+2*i] = byte(v), byte(uint16(v)>>8)
	}
	return b
}

func le32(b []byte, v uint32) { b[0], b[1], b[2], b[3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24) }

// ChartSeries is one series of a generated chart.
type ChartSeries struct {
	Name string
	Vals []float64
}

// Chart adds a chart frame. kind is the plot element (barChart, lineChart, areaChart, pieChart,
// doughnutChart); extra holds elements of the plot element (barDir, grouping, ...), written
// before the series.
func (s *Slide) Chart(d *Deck, name string, b Box, kind, extra, title string, cats []string, ser []ChartSeries) int {
	id := s.ID()
	n := len(d.extra) + 1
	part := fmt.Sprintf("ppt/charts/chart%d.xml", n)
	rid := s.Rel("http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart", fmt.Sprintf("../charts/chart%d.xml", n), "")
	var sx strings.Builder
	for i, se := range ser {
		fmt.Fprintf(&sx, `<c:ser><c:idx val="%d"/><c:order val="%d"/><c:tx><c:strRef><c:f>Sheet1!$%c$1</c:f><c:strCache><c:ptCount val="1"/><c:pt idx="0"><c:v>%s</c:v></c:pt></c:strCache></c:strRef></c:tx>`, i, i, 'B'+i, Esc(se.Name))
		fmt.Fprintf(&sx, `<c:cat><c:strRef><c:f>Sheet1!$A$2:$A$%d</c:f><c:strCache><c:ptCount val="%d"/>`, len(cats)+1, len(cats))
		for j, c := range cats {
			fmt.Fprintf(&sx, `<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, j, Esc(c))
		}
		fmt.Fprintf(&sx, `</c:strCache></c:strRef></c:cat><c:val><c:numRef><c:f>Sheet1!$%c$2:$%c$%d</c:f><c:numCache><c:formatCode>General</c:formatCode><c:ptCount val="%d"/>`, 'B'+i, 'B'+i, len(cats)+1, len(se.Vals))
		for j, v := range se.Vals {
			fmt.Fprintf(&sx, `<c:pt idx="%d"><c:v>%g</c:v></c:pt>`, j, v)
		}
		sx.WriteString(`</c:numCache></c:numRef></c:val></c:ser>`)
	}
	axes := `<c:axId val="1001"/><c:axId val="1002"/>`
	axDefs := `<c:catAx><c:axId val="1001"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="b"/><c:crossAx val="1002"/></c:catAx>` +
		`<c:valAx><c:axId val="1002"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="l"/><c:majorGridlines/><c:crossAx val="1001"/></c:valAx>`
	if kind == "pieChart" || kind == "doughnutChart" {
		axes, axDefs = "", ""
	}
	titleXML := `<c:autoTitleDeleted val="1"/>`
	if title != "" {
		titleXML = `<c:title><c:tx><c:rich><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>` + Esc(title) + `</a:t></a:r></a:p></c:rich></c:tx><c:overlay val="0"/></c:title><c:autoTitleDeleted val="0"/>`
	}
	d.Part(part, "application/vnd.openxmlformats-officedocument.drawingml.chart+xml", xmlHead+
		`<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`+
		`<c:chart>`+titleXML+`<c:plotArea><c:layout/><c:`+kind+`>`+extra+`<c:varyColors val="`+map[bool]string{true: "1", false: "0"}[axes == ""]+`"/>`+sx.String()+axes+`</c:`+kind+`>`+axDefs+`</c:plotArea>`+
		`<c:legend><c:legendPos val="b"/><c:overlay val="0"/></c:legend><c:plotVisOnly val="1"/></c:chart></c:chartSpace>`)
	s.Shapes = append(s.Shapes, fmt.Sprintf(`<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="%d" name="%s"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr><p:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></p:xfrm>`+
		`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" r:id="%s"/></a:graphicData></a:graphic></p:graphicFrame>`,
		id, Esc(name), Px(b.X), Px(b.Y), Px(b.W), Px(b.H), rid))
	return id
}
