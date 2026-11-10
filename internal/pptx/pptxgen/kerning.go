package pptxgen

import (
	"fmt"
	"strings"
)

// The kerning deck measures when PowerPoint kerns text. Every row is one line of the same words
// in its own text box, set in one font with one combination of size and kern. kern is the run
// attribute that says from which font size on PowerPoint kerns (in hundredths of a point; 0
// switches kerning off). The words are of two kinds: some have letter pairs the fonts kern, the
// others have none and show how exact the measurement is.
//
// Every row that could be kerned has a twin: the same line in the same font, size and style
// with kern="0". The difference between the width of a word in the two rows is the kerning
// PowerPoint applied, free of whatever else PowerPoint does to the width of a word.
//
// Liberation Sans and Liberation Serif have their pairs in the legacy kern table and, with the
// same values, in GPOS. Carlito has them in GPOS only. A row in Carlito therefore tells whether
// PowerPoint reads GPOS at all.

// KerningWords are the words of every row of the kerning deck, in this order. The gaps count
// too: Liberation kerns a space before T.
var KerningWords = []string{"noon", "AVATAR", "Text", "mind", "Toy", "Year", "PAVE", "dune"}

// KerningKerned and KerningControl split KerningWords: words with pairs that make up more than
// 1 % of the word in every bundled text font, and words without a pair in any of them
// (TestKerningWords checks both against the fonts).
var (
	KerningKerned  = []string{"AVATAR", "Text", "Toy", "Year", "PAVE"}
	KerningControl = []string{"noon", "mind", "dune"}
)

// kerningWrapWords fill the paragraphs that wrap: enough kerned words that a line of them is
// clearly shorter with kerning than without.
var kerningWrapWords = []string{
	"Tomato", "Table", "AVATAR", "Text", "Toy", "Water", "Wave", "Voyage", "Year", "Yoyo", "Vault", "Today",
	"Taxes", "Tower", "Vote", "PAVE", "WAVY", "Tea", "Typeface", "Tornado", "Yacht", "Valve", "Yawn", "Teapot",
}

// KerningRow describes one text box of a kerning deck.
type KerningRow struct {
	ID        string   `json:"id"`                  // what the row varies, for example "kern1200-10pt"
	Twin      string   `json:"twin,omitempty"`      // id of the row in the same font that differs only by kern="0"
	Slide     int      `json:"slide"`               // 1-based
	Font      string   `json:"font"`                // typeface of the runs
	Bold      bool     `json:"bold,omitempty"`      //
	SizePt    float64  `json:"sizePt"`              // nominal size
	Kern      string   `json:"kern"`                // "0", "1200", "2000", "none" (no kern anywhere) or "inherited" (kern="1200" in the list style of the shape only)
	SpcPt     float64  `json:"spcPt,omitempty"`     // letter spacing
	Runs      []string `json:"runs"`                // text of each run; joined they are the paragraph
	Wrap      bool     `json:"wrap,omitempty"`      // the box has a fixed width and the text wraps
	FontScale int      `json:"fontScale,omitempty"` // stored shrink on overflow, in percent
	Kerned    []string `json:"kerned"`              // words with pairs, in this font
	Control   []string `json:"control"`             // words without pairs, in this font
	Differ    []string `json:"differ,omitempty"`    // words for which the kern table and GPOS of this font disagree
	Box       [4]int   `json:"box"`                 // x, y, width, height in px at 96 dpi
}

// KerningSpec lists the rows of a kerning deck in reading order (slide, then top to bottom).
type KerningSpec struct {
	Rows []KerningRow `json:"rows"`
}

// KerningFont is a font of a kerning deck with the words chosen for it.
type KerningFont struct {
	Family  string
	Words   []string // the line, in order
	Kerned  []string
	Control []string
	Differ  []string // words for which the kern table and GPOS disagree
	Bold    bool     // the family has a bold face to measure
}

type kerningVariant struct {
	id     string
	twin   string
	size   float64
	kern   string
	bold   bool
	spc    float64
	split  bool
	skipIf func(f KerningFont) bool
}

// The matrix of the public deck (see docs/spikes/S4-powerpoint-reference.md).
var kerningFull = []kerningVariant{
	{id: "kern0", size: 28, kern: "0"},
	{id: "kern1200", twin: "kern0", size: 28, kern: "1200"},
	{id: "kern2000-28pt", twin: "kern0", size: 28, kern: "2000"},
	{id: "none", twin: "kern0", size: 28, kern: "none"},
	{id: "inherited", twin: "kern0", size: 28, kern: "inherited"},
	{id: "runs", twin: "kern0", size: 28, kern: "1200", split: true},
	{id: "bold-kern0", size: 28, kern: "0", bold: true},
	{id: "bold", twin: "bold-kern0", size: 28, kern: "1200", bold: true},
	{id: "spacing-kern0", size: 28, kern: "0", spc: 3},
	{id: "spacing", twin: "spacing-kern0", size: 28, kern: "1200", spc: 3},
	{id: "kern0-12pt", size: 12, kern: "0"},
	{id: "kern1200-12pt", twin: "kern0-12pt", size: 12, kern: "1200"},
	{id: "kern0-10pt", size: 10, kern: "0"},
	{id: "kern1200-10pt", twin: "kern0-10pt", size: 10, kern: "1200"},
	{id: "kern0-18pt", size: 18, kern: "0"},
	{id: "kern2000-18pt", twin: "kern0-18pt", size: 18, kern: "2000"},
}

// The short matrix of the local deck with fonts that are not bundled.
var kerningShort = []kerningVariant{
	{id: "kern0", size: 28, kern: "0"},
	{id: "kern1200", twin: "kern0", size: 28, kern: "1200"},
	{id: "bold-kern0", size: 28, kern: "0", bold: true, skipIf: func(f KerningFont) bool { return !f.Bold }},
	{id: "bold", twin: "bold-kern0", size: 28, kern: "1200", bold: true, skipIf: func(f KerningFont) bool { return !f.Bold }},
	{id: "kern0-10pt", size: 10, kern: "0"},
	{id: "kern1200-10pt", twin: "kern0-10pt", size: 10, kern: "1200"},
}

// Kerning is the public kerning deck.
func Kerning() *Deck {
	d, _ := KerningWithSpec()
	return d
}

// KerningWithSpec returns the public kerning deck and the description of its rows.
func KerningWithSpec() (*Deck, KerningSpec) {
	var fs []KerningFont
	for _, fam := range []string{"Liberation Sans", "Liberation Serif", "Carlito"} {
		fs = append(fs, KerningFont{Family: fam, Words: KerningWords, Kerned: KerningKerned, Control: KerningControl, Bold: true})
	}
	d := New("Kerning feature deck")
	d.NoStyleKern = true
	var spec KerningSpec
	for _, f := range fs {
		kerningSlide(d, &spec, f, kerningFull)
	}
	// paragraphs that wrap, one per font: a slide with kerning and a slide of twins without
	for _, v := range []struct{ id, twin, kern string }{{"wrap", "wrap-kern0", "1200"}, {"wrap-kern0", "", "0"}} {
		s := d.Add(LayoutBlank)
		for i, f := range fs {
			row := KerningRow{ID: v.id, Twin: v.twin, Slide: len(d.Slides), Font: f.Family, SizePt: 28, Kern: v.kern,
				Runs: []string{strings.Join(kerningWrapWords, " ")}, Wrap: true, Kerned: kerningWrapWords,
				Box: [4]int{60, 30 + 225*i, 1000, 200}}
			kerningBox(s, row)
			spec.Rows = append(spec.Rows, row)
		}
	}
	// stored shrink on overflow: 28 pt at 40 % is 11.2 pt, below kern="1200"
	s := d.Add(LayoutBlank)
	for i, f := range fs {
		for j, v := range []struct{ id, twin, kern string }{{"fontscale-kern0", "", "0"}, {"fontscale", "fontscale-kern0", "1200"}} {
			row := KerningRow{ID: v.id, Twin: v.twin, Slide: len(d.Slides), Font: f.Family, SizePt: 28, Kern: v.kern,
				Runs: []string{strings.Join(f.Words, " ")}, Wrap: true, FontScale: 40, Kerned: f.Kerned, Control: f.Control,
				Box: [4]int{60, 40 + 100*(2*i+j), 1100, 60}}
			kerningBox(s, row)
			spec.Rows = append(spec.Rows, row)
		}
	}
	return d, spec
}

// KerningLocal returns a kerning deck for other fonts, with the short matrix. The reference
// tools write it for fonts that are installed on the machine but cannot go into the repository
// (go run ./scripts/pptxgen -kernlocal).
func KerningLocal(fs []KerningFont) (*Deck, KerningSpec) {
	d := New("Kerning measurement, local fonts")
	d.NoStyleKern = true
	var spec KerningSpec
	const perSlide = 2
	for i := 0; i < len(fs); i += perSlide {
		s := d.Add(LayoutBlank)
		y := 20
		for _, f := range fs[i:min(i+perSlide, len(fs))] {
			y = kerningRows(d, s, &spec, f, kerningShort, y) + 12
		}
	}
	return d, spec
}

func kerningSlide(d *Deck, spec *KerningSpec, f KerningFont, vs []kerningVariant) {
	kerningRows(d, d.Add(LayoutBlank), spec, f, vs, 20)
}

// kerningRows adds one row per variant below y and returns the y after the last row. A row of
// large text takes 50 px, a row of small text 30 px.
func kerningRows(d *Deck, s *Slide, spec *KerningSpec, f KerningFont, vs []kerningVariant, y int) int {
	for _, v := range vs {
		if v.skipIf != nil && v.skipIf(f) {
			continue
		}
		runs := []string{strings.Join(f.Words, " ")}
		if v.split {
			runs = splitInsideKerned(f)
		}
		step := 50
		if v.size < 24 {
			step = 30
		}
		row := KerningRow{ID: v.id, Twin: v.twin, Slide: len(d.Slides), Font: f.Family, Bold: v.bold, SizePt: v.size, Kern: v.kern, SpcPt: v.spc,
			Runs: runs, Kerned: f.Kerned, Control: f.Control, Differ: f.Differ, Box: [4]int{60, y, 1160, step - 6}}
		kerningBox(s, row)
		spec.Rows = append(spec.Rows, row)
		y += step
	}
	return y
}

// splitInsideKerned cuts the line after the first letter of every kerned word, so that the
// first pair of each of them lies across a run boundary.
func splitInsideKerned(f KerningFont) []string {
	kerned := map[string]bool{}
	for _, w := range f.Kerned {
		kerned[w] = true
	}
	var runs []string
	cur := ""
	for i, w := range f.Words {
		if i > 0 {
			cur += " "
		}
		if kerned[w] && len(w) > 1 {
			runs = append(runs, cur+w[:1])
			cur = w[1:]
		} else {
			cur += w
		}
	}
	return append(runs, cur)
}

// kerningBox writes the text box of a row: no insets, so that the text starts at the box.
func kerningBox(s *Slide, r KerningRow) {
	body := `<a:bodyPr wrap="none" lIns="0" tIns="0" rIns="0" bIns="0" rtlCol="0"><a:noAutofit/></a:bodyPr>`
	switch {
	case r.FontScale > 0:
		body = fmt.Sprintf(`<a:bodyPr wrap="square" lIns="0" tIns="0" rIns="0" bIns="0" rtlCol="0"><a:normAutofit fontScale="%d"/></a:bodyPr>`, r.FontScale*1000)
	case r.Wrap:
		body = `<a:bodyPr wrap="square" lIns="0" tIns="0" rIns="0" bIns="0" rtlCol="0"><a:noAutofit/></a:bodyPr>`
	}
	lst := `<a:lstStyle/>`
	attrs := fmt.Sprintf(`sz="%d"`, int(r.SizePt*100))
	switch r.Kern {
	case "none":
	case "inherited":
		lst = `<a:lstStyle><a:lvl1pPr><a:defRPr kern="1200"/></a:lvl1pPr></a:lstStyle>`
	default:
		attrs += ` kern="` + r.Kern + `"`
	}
	if r.Bold {
		attrs += ` b="1"`
	}
	if r.SpcPt != 0 {
		attrs += fmt.Sprintf(` spc="%d"`, int(r.SpcPt*100))
	}
	var p strings.Builder
	p.WriteString(`<a:p>`)
	for i, t := range r.Runs {
		// runs alternate between two colors, so that PowerPoint keeps them apart when it saves
		clr := "000000"
		if i%2 == 1 {
			clr = "1F4E79"
		}
		p.WriteString(`<a:r><a:rPr lang="en-US" ` + attrs + ` dirty="0"><a:solidFill><a:srgbClr val="` + clr + `"/></a:solidFill><a:latin typeface="` + Esc(r.Font) + `"/></a:rPr><a:t>` + Esc(t) + `</a:t></a:r>`)
	}
	p.WriteString(`</a:p>`)
	id := s.ID()
	s.Shapes = append(s.Shapes, fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="%s"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr><p:spPr>%s<a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/></p:spPr><p:txBody>%s%s%s</p:txBody></p:sp>`,
		id, Esc(r.Font+" "+r.ID), Box{float64(r.Box[0]), float64(r.Box[1]), float64(r.Box[2]), float64(r.Box[3])}.xfrm(), body, lst, p.String()))
}
