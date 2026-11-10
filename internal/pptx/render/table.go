package render

import (
	"fmt"
	"strings"

	"deckfold/internal/pptx/model"
	"deckfold/internal/pptx/xmltree"
)

// table draws an a:tbl: cell fills, borders and text. Row heights grow to fit the text, like
// PowerPoint does when it lays out a table.
func (r *slideRenderer) table(b *strings.Builder, tbl *xmltree.Node, owner ownerKind, part model.Part, box rect) {
	cx := r.colorCtx()
	var cols []float64
	for _, g := range tbl.Path("tblGrid").Children("gridCol") {
		cols = append(cols, float64(g.Int("w", 0))/emuPx)
	}
	rows := tbl.Children("tr")
	if len(cols) == 0 || len(rows) == 0 {
		return
	}
	style := r.tableStyle(tbl.Path("tblPr/tableStyleId").AllText())
	tp := tbl.Child("tblPr")
	firstRow, bandRow := tp.Bool("firstRow", false), tp.Bool("bandRow", false)
	firstCol, lastRow := tp.Bool("firstCol", false), tp.Bool("lastRow", false)

	type cellLayout struct {
		segs []textSeg
		h    float64
	}
	// first pass: text layout per cell at y=0 to find the row heights
	heights := make([]float64, len(rows))
	layouts := make([][]cellLayout, len(rows))
	for ri, tr := range rows {
		heights[ri] = float64(tr.Int("h", 0)) / emuPx
		cells := tr.Children("tc")
		layouts[ri] = make([]cellLayout, len(cells))
		for ci, tc := range cells {
			if tc.Bool("hMerge", false) || tc.Bool("vMerge", false) {
				continue
			}
			w := spanWidth(cols, ci, int(tc.Int("gridSpan", 1)))
			cell := rect{0, 0, w, 10000}
			segs, used := r.cellText(tc, cell, cx, part, style, ri, ci, len(rows), firstRow, lastRow, firstCol)
			layouts[ri][ci] = cellLayout{segs, used}
			if int(tc.Int("rowSpan", 1)) == 1 && used > heights[ri] {
				heights[ri] = used
			}
		}
	}
	y := box.y
	for ri, tr := range rows {
		x := box.x
		for ci, tc := range tr.Children("tc") {
			w := spanWidth(cols, ci, int(tc.Int("gridSpan", 1)))
			if tc.Bool("hMerge", false) || tc.Bool("vMerge", false) {
				x += colWidth(cols, ci)
				continue
			}
			h := 0.0
			for k := 0; k < int(tc.Int("rowSpan", 1)) && ri+k < len(rows); k++ {
				h += heights[ri+k]
			}
			cell := rect{x, y, w, h}
			tcPr := tc.Child("tcPr")
			fill := ""
			for _, k := range tcPr.Kids {
				if s, ok := r.fillOf(k, cx, part, cell); ok {
					fill = s
					break
				}
			}
			if fill == "" && style != nil {
				fill = r.styleCellFill(style, cx, part, cell, ri, ci, len(rows), firstRow, bandRow, lastRow, firstCol)
			}
			if fill != "" && fill != `fill="none"` {
				fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" %s/>`, f(cell.x), f(cell.y), f(cell.w), f(cell.h), fill)
			}
			// borders
			for _, side := range []struct{ name, d string }{
				{"lnL", fmt.Sprintf("M%s %sV%s", f(cell.x), f(cell.y), f(cell.y+cell.h))},
				{"lnR", fmt.Sprintf("M%s %sV%s", f(cell.x+cell.w), f(cell.y), f(cell.y+cell.h))},
				{"lnT", fmt.Sprintf("M%s %sH%s", f(cell.x), f(cell.y), f(cell.x+cell.w))},
				{"lnB", fmt.Sprintf("M%s %sH%s", f(cell.x), f(cell.y+cell.h), f(cell.x+cell.w))},
			} {
				ln := tcPr.Child(side.name)
				if ln == nil && style != nil {
					ln = r.styleBorder(style, side.name, ri, ci, len(rows), len(cols))
				}
				if ln == nil {
					continue
				}
				if s := r.strokeAttr([]*xmltree.Node{{Name: "spPr", Kids: []*xmltree.Node{renamed(ln, "ln")}}}, nil, cx); s != "" {
					fmt.Fprintf(b, `<path d="%s" fill="none"%s/>`, side.d, s)
				}
			}
			// text: vertical anchor inside the cell
			cl := layouts[ri][ci]
			dy := 0.0
			switch tcPr.Attr("anchor") {
			case "ctr":
				dy = (h - cl.h) / 2
			case "b":
				dy = h - cl.h
			}
			for _, s := range cl.segs {
				s.x += x
				s.y += y + dy
				r.writeText(b, []textSeg{s})
			}
			x += colWidth(cols, ci)
		}
		y += heights[ri]
	}
}

func renamed(n *xmltree.Node, name string) *xmltree.Node {
	c := *n
	c.Name = name
	return &c
}

func colWidth(cols []float64, i int) float64 {
	if i < len(cols) {
		return cols[i]
	}
	return 0
}

func spanWidth(cols []float64, i, span int) float64 {
	w := 0.0
	for k := 0; k < span; k++ {
		w += colWidth(cols, i+k)
	}
	return w
}

// cellText lays out the text of a cell with its margins and returns the segments (relative to
// the cell) and the height the text needs including margins.
func (r *slideRenderer) cellText(tc *xmltree.Node, cell rect, cx colorCtx, part model.Part, style *xmltree.Node, ri, ci, nrows int, firstRow, lastRow, firstCol bool) ([]textSeg, float64) {
	tcPr := tc.Child("tcPr")
	l := float64(tcPr.Int("marL", 91440)) / emuPx
	rr := float64(tcPr.Int("marR", 91440)) / emuPx
	t := float64(tcPr.Int("marT", 45720)) / emuPx
	bm := float64(tcPr.Int("marB", 45720)) / emuPx
	// a synthetic shape: the cell's text body with zero insets (the margins are applied here)
	body := tc.Child("txBody")
	if body == nil {
		return nil, t + bm
	}
	ts := &textStyle{}
	ts.addList(r.pres.DefaultTextStyle)
	ts.addList(r.slide.Layout.Master.Other)
	// the text color and weight of the table style (white bold header, ...) rank above the
	// presentation's default text styles and below the cell's own formatting
	if style != nil {
		ts.addList(r.styleTextFor(style, ri, ci, nrows, firstRow, lastRow, firstCol))
	}
	ts.addList(body.Child("lstStyle"))
	ts.bodyPr = []*xmltree.Node{{Name: "bodyPr", Attrs: attrs("lIns", "0", "tIns", "0", "rIns", "0", "bIns", "0")}}
	sp := &xmltree.Node{Name: "sp", Kids: []*xmltree.Node{body}}
	if r.frame != "" {
		r.collectText(body, ts, r.frame, "", false, true, relLookup{part, r.Renderer}, "")
	}
	inner := rect{cell.x + l, cell.y + t, cell.w - l - rr, 100000}
	segs := r.layoutText(sp, ts, inner, cx, relLookup{part, r.Renderer})
	maxY := cell.y + t
	for _, s := range segs {
		if d := s.y + s.face.Descent()*s.size; d > maxY {
			maxY = d
		}
	}
	return segs, maxY - cell.y + bm
}

// tableStyle returns the a:tblStyle for an id from ppt/tableStyles.xml, or nil.
func (r *slideRenderer) tableStyle(id string) *xmltree.Node {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	n, err := r.pres.Pkg.XML("ppt/tableStyles.xml")
	if err != nil {
		return nil
	}
	for _, s := range n.Children("tblStyle") {
		if s.Attr("styleId") == id {
			return s
		}
	}
	return nil
}

// stylePart picks the table style part for a cell, most specific first.
func stylePart(style *xmltree.Node, ri, ci, nrows int, firstRow, bandRow, lastRow, firstCol bool) []*xmltree.Node {
	var out []*xmltree.Node
	add := func(name string) {
		if n := style.Child(name); n != nil {
			out = append(out, n)
		}
	}
	if firstRow && ri == 0 {
		add("firstRow")
	}
	if lastRow && ri == nrows-1 {
		add("lastRow")
	}
	if firstCol && ci == 0 {
		add("firstCol")
	}
	if bandRow {
		bi := ri
		if firstRow {
			bi--
		}
		if bi >= 0 && bi%2 == 0 {
			add("band1H")
		}
	}
	add("wholeTbl")
	return out
}

func (r *slideRenderer) styleCellFill(style *xmltree.Node, cx colorCtx, part model.Part, cell rect, ri, ci, nrows int, firstRow, bandRow, lastRow, firstCol bool) string {
	for _, p := range stylePart(style, ri, ci, nrows, firstRow, bandRow, lastRow, firstCol) {
		fill := p.Path("tcStyle/fill")
		if fill == nil {
			continue
		}
		for _, k := range fill.Kids {
			if s, ok := r.fillOf(k, cx, part, cell); ok {
				return s
			}
		}
	}
	return ""
}

func (r *slideRenderer) styleBorder(style *xmltree.Node, side string, ri, ci, nrows, ncols int) *xmltree.Node {
	name := map[string]string{"lnL": "left", "lnR": "right", "lnT": "top", "lnB": "bottom"}[side]
	inner := (side == "lnL" && ci > 0) || (side == "lnR" && ci < ncols-1) || (side == "lnT" && ri > 0) || (side == "lnB" && ri < nrows-1)
	if inner {
		if side == "lnL" || side == "lnR" {
			name = "insideV"
		} else {
			name = "insideH"
		}
	}
	if b := style.Path("wholeTbl/tcStyle/tcBdr/" + name + "/ln"); b != nil {
		return b
	}
	return nil
}

// styleTextFor returns an a:lstStyle-like node with the text color, bold and italic of the table
// style for a cell (from a:tcTxStyle), or nil.
func (r *slideRenderer) styleTextFor(style *xmltree.Node, ri, ci, nrows int, firstRow, lastRow, firstCol bool) *xmltree.Node {
	for _, p := range stylePart(style, ri, ci, nrows, firstRow, false, lastRow, firstCol) {
		tx := p.Child("tcTxStyle")
		if tx == nil {
			continue
		}
		def := &xmltree.Node{Name: "defRPr"}
		for _, a := range []string{"b", "i"} {
			switch tx.Attr(a) {
			case "on":
				def.Attrs = append(def.Attrs, attrs(a, "1")...)
			case "off":
				def.Attrs = append(def.Attrs, attrs(a, "0")...)
			}
		}
		for _, k := range tx.Kids {
			if r.colorCtx().colorOf(k).OK {
				def.Kids = append(def.Kids, &xmltree.Node{Name: "solidFill", Kids: []*xmltree.Node{k}})
				break
			}
		}
		lst := &xmltree.Node{Name: "lstStyle"}
		for l := 1; l <= 9; l++ {
			lst.Kids = append(lst.Kids, &xmltree.Node{Name: fmt.Sprintf("lvl%dpPr", l), Kids: []*xmltree.Node{def}})
		}
		return lst
	}
	return nil
}
