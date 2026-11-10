package pptxgen

import "fmt"

// The baseline deck measures where PowerPoint puts the baselines of a text: boxes of three
// lines each, without insets, in one font with one line spacing and one vertical anchor. It is
// not one of the feature decks. The reference tools write it locally (go run ./scripts/pptxgen
// -baselinelocal), PowerPoint exports it as PDF, and the baselines read from that PDF stand in
// the test of the renderer (internal/pptx/baseline_test.go).

// BaselineBox describes one text box of the baseline deck.
type BaselineBox struct {
	Slide     int     `json:"slide"` // 1-based
	ID        string  `json:"id"`
	Font      string  `json:"font"`
	SizePt    float64 `json:"sizePt"`
	LnPct     int     `json:"lnPct,omitempty"`     // line spacing in percent
	LnPts     float64 `json:"lnPts,omitempty"`     // exact line spacing in points
	Reduce    int     `json:"reduce,omitempty"`    // stored line spacing reduction in percent
	FontScale float64 `json:"fontScale,omitempty"` // stored font scale in percent
	Anchor    string  `json:"anchor"`              // t, ctr or b
	Box       [4]int  `json:"box"`                 // x, y, width, height in px at 96 dpi
}

type baselineVariant struct {
	id     string
	size   float64
	pct    int
	pts    float64
	reduce int
	scale  float64
	anchor string
}

func baselineVariants() []baselineVariant {
	var vs []baselineVariant
	for _, p := range []int{70, 80, 90, 100, 110, 120, 150, 200} {
		vs = append(vs, baselineVariant{id: fmt.Sprintf("pct%d", p), size: 28, pct: p, anchor: "t"})
	}
	for _, p := range []float64{20, 28, 40, 50} {
		vs = append(vs, baselineVariant{id: fmt.Sprintf("pts%g", p), size: 28, pts: p, anchor: "t"})
	}
	return append(vs,
		baselineVariant{id: "reduce10", size: 28, pct: 100, reduce: 10, anchor: "t"},
		baselineVariant{id: "reduce20", size: 28, pct: 100, reduce: 20, anchor: "t"},
		baselineVariant{id: "reduce20-scale625", size: 24, pct: 100, reduce: 20, scale: 62.5, anchor: "t"},
		baselineVariant{id: "reduce10-pct90", size: 28, pct: 90, reduce: 10, anchor: "t"},
		baselineVariant{id: "pct90-18pt", size: 18, pct: 90, anchor: "t"},
		baselineVariant{id: "pct150-18pt", size: 18, pct: 150, anchor: "t"},
		baselineVariant{id: "pct100-ctr", size: 28, pct: 100, anchor: "ctr"},
		baselineVariant{id: "pct100-b", size: 28, pct: 100, anchor: "b"},
		baselineVariant{id: "pct90-ctr", size: 28, pct: 90, anchor: "ctr"},
		baselineVariant{id: "pct150-b", size: 28, pct: 150, anchor: "b"},
	)
}

// Baselines returns the baseline deck for the given fonts and the description of its boxes.
func Baselines(families ...string) (*Deck, []BaselineBox) {
	d := New("Baseline measurement")
	d.NoStyleKern = true
	var spec []BaselineBox
	const perSlide = 4
	for _, font := range families {
		for i, v := range baselineVariants() {
			if i%perSlide == 0 {
				d.Add(LayoutBlank)
			}
			s := d.Slides[len(d.Slides)-1]
			b := BaselineBox{Slide: len(d.Slides), ID: v.id, Font: font, SizePt: v.size, LnPct: v.pct, LnPts: v.pts, Reduce: v.reduce, FontScale: v.scale, Anchor: v.anchor,
				Box: [4]int{40 + 310*(i%perSlide), 40, 280, 600}}
			spec = append(spec, b)
			fit := `<a:noAutofit/>`
			if v.reduce > 0 {
				scale := ""
				if v.scale > 0 {
					scale = fmt.Sprintf(` fontScale="%d"`, int(v.scale*1000))
				}
				fit = fmt.Sprintf(`<a:normAutofit%s lnSpcReduction="%d"/>`, scale, v.reduce*1000)
			}
			ln := ""
			switch {
			case v.pts > 0:
				ln = fmt.Sprintf(`<a:lnSpc><a:spcPts val="%d"/></a:lnSpc>`, int(v.pts*100))
			case v.pct != 100:
				ln = fmt.Sprintf(`<a:lnSpc><a:spcPct val="%d"/></a:lnSpc>`, v.pct*1000)
			}
			rpr := fmt.Sprintf(`<a:rPr lang="en-US" sz="%d" kern="0" dirty="0"><a:latin typeface="%s"/></a:rPr>`, int(v.size*100), Esc(font))
			run := func(t string) string { return `<a:r>` + rpr + `<a:t>` + t + `</a:t></a:r>` }
			s.Raw(fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="%s"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr><p:spPr>%s<a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/><a:ln w="6350"><a:solidFill><a:srgbClr val="BBBBBB"/></a:solidFill></a:ln></p:spPr><p:txBody><a:bodyPr wrap="square" lIns="0" tIns="0" rIns="0" bIns="0" rtlCol="0" anchor="%s">%s</a:bodyPr><a:lstStyle/><a:p><a:pPr>%s</a:pPr>%s<a:br>%s</a:br>%s<a:br>%s</a:br>%s</a:p></p:txBody></p:sp>`,
				s.ID(), Esc(font+" "+v.id), Box{float64(b.Box[0]), float64(b.Box[1]), float64(b.Box[2]), float64(b.Box[3])}.xfrm(), v.anchor, fit, ln,
				run("Hxg one"), rpr, run("Hxg two"), rpr, run("Hxg three")))
		}
	}
	return d, spec
}
