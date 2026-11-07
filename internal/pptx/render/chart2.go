package render

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"deckfold/internal/pptx/xmltree"
)

// Chart types beyond column, bar, line, area and pie: scatter, bubble, radar, stock, pie of pie
// and bar of pie, and surface charts (as value bands). 3D charts keep a flat drawing; the
// player draws their plot with three.js when WebGL is available (chart3DAttr).

// niceRange extends lo..hi to whole axis steps, as for automatic axes.
func niceRange(lo, hi float64, fromZero bool) (float64, float64, float64) {
	if fromZero {
		lo, hi = math.Min(lo, 0), math.Max(hi, 0)
	}
	if hi <= lo {
		hi = lo + 1
	}
	step := axisStep(lo, hi)
	hi = math.Ceil((hi+0.05*(hi-lo))/step) * step
	lo = math.Floor(lo/step) * step
	return lo, hi, step
}

// valueAxis draws gridlines, labels and the line of a value axis along one side of plotR.
// vertical axes run from the bottom up.
func (r *slideRenderer) valueAxis(b *strings.Builder, ax *xmltree.Node, plotR rect, lo, hi, step float64, vertical bool, label labelFn, fs float64, text Color) {
	pos := func(v float64) float64 {
		if vertical {
			return plotR.y + plotR.h - (v-lo)/(hi-lo)*plotR.h
		}
		return plotR.x + (v-lo)/(hi-lo)*plotR.w
	}
	grid := ax.Child("majorGridlines")
	gs := chartStroke(grid, r.colorCtx(), hexColor("000000"), 1)
	del := ax.Child("delete").Bool("val", false)
	for v := lo; v <= hi+step/2; v += step {
		p := pos(v)
		if grid != nil && gs != "" {
			if vertical {
				fmt.Fprintf(b, `<path d="M%s %sH%s" fill="none"%s/>`, f(plotR.x), f(p), f(plotR.x+plotR.w), gs)
			} else {
				fmt.Fprintf(b, `<path d="M%s %sV%s" fill="none"%s/>`, f(p), f(plotR.y), f(plotR.y+plotR.h), gs)
			}
		}
		if !del {
			if vertical {
				label(plotR.x-fs*0.6, p+fs*0.35, fmtNum(v, false), "end", fs, text)
			} else {
				label(p, plotR.y+plotR.h+fs*1.5, fmtNum(v, false), "middle", fs, text)
			}
		}
	}
	if line := chartStroke(ax, r.colorCtx(), hexColor("000000"), 1); line != "" && !del {
		if vertical {
			fmt.Fprintf(b, `<path d="M%s %sV%s" fill="none"%s/>`, f(plotR.x), f(plotR.y), f(plotR.y+plotR.h), line)
		} else {
			fmt.Fprintf(b, `<path d="M%s %sH%s" fill="none"%s/>`, f(plotR.x), f(plotR.y+plotR.h), f(plotR.x+plotR.w), line)
		}
	}
}

// scatter draws scatter and bubble charts: x and y values on two value axes.
func (r *slideRenderer) scatter(b *strings.Builder, kind, plot *xmltree.Node, ss []series, box rect, label labelFn, fs float64, text Color) {
	bubble := kind.Name == "bubbleChart"
	xlo, xhi, ylo, yhi := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
	maxSize := 0.0
	for i := range ss {
		if len(ss[i].xs) == 0 {
			for k := range ss[i].vals {
				ss[i].xs = append(ss[i].xs, float64(k+1))
			}
		}
		for k, y := range ss[i].vals {
			x := at(ss[i].xs, k)
			xlo, xhi, ylo, yhi = math.Min(xlo, x), math.Max(xhi, x), math.Min(ylo, y), math.Max(yhi, y)
			maxSize = math.Max(maxSize, at(ss[i].sizes, k))
		}
	}
	if math.IsInf(xlo, 0) {
		return
	}
	xlo, xhi, xstep := niceRange(xlo, xhi, true)
	ylo, yhi, ystep := niceRange(ylo, yhi, true)
	var xAx, yAx *xmltree.Node
	for _, ax := range plot.Children("valAx") {
		switch ax.Child("axPos").Attr("val") {
		case "b", "t":
			xAx = ax
		default:
			yAx = ax
		}
	}
	face := r.fonts.Face(r.typeface("+mn-lt"), false, false)
	lw := 0.0
	for v := ylo; v <= yhi+ystep/2; v += ystep {
		lw = math.Max(lw, measure(face, fmtNum(v, false), fs))
	}
	plotR := rect{box.x + lw + fs*0.6, box.y + fs*0.5, box.w - lw - fs*1.2, box.h - fs*2}
	r.valueAxis(b, yAx, plotR, ylo, yhi, ystep, true, label, fs, text)
	r.valueAxis(b, xAx, plotR, xlo, xhi, xstep, false, label, fs, text)
	px := func(x float64) float64 { return plotR.x + (x-xlo)/(xhi-xlo)*plotR.w }
	py := func(y float64) float64 { return plotR.y + plotR.h - (y-ylo)/(yhi-ylo)*plotR.h }
	style := kind.Child("scatterStyle").Attr("val")
	scale := float64(kind.Child("bubbleScale").Int("val", 100)) / 100
	maxR := math.Min(plotR.w, plotR.h) * 0.125 * scale
	for _, s := range ss {
		if bubble {
			for k, y := range s.vals {
				sz := at(s.sizes, k)
				if sz <= 0 || maxSize <= 0 {
					continue
				}
				rr := math.Sqrt(sz/maxSize) * maxR // area proportional to the size
				fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="%s" %s/>`, f(px(at(s.xs, k))), f(py(y)), f(rr), paint("fill", s.color))
			}
			continue
		}
		noLine := style == "marker" || s.sppr.Path("ln/noFill") != nil
		if !noLine && len(s.vals) > 1 {
			var d strings.Builder
			for k, y := range s.vals {
				cmd := "L"
				if k == 0 {
					cmd = "M"
				}
				fmt.Fprintf(&d, "%s%s %s", cmd, f(px(at(s.xs, k))), f(py(y)))
			}
			fmt.Fprintf(b, `<path d="%s" fill="none" stroke="%s" stroke-width="%s" stroke-linejoin="round"/>`, d.String(), s.color.Hex(), f(2.25*ptPx))
		}
		sym := s.node.Path("marker/symbol").Attr("val")
		if sym == "" {
			sym = autoMarker(int(s.node.Child("idx").Int("val", 0)))
		}
		if sym != "none" && style != "line" && style != "smooth" {
			for k, y := range s.vals {
				b.WriteString(markerShape(sym, px(at(s.xs, k)), py(y), 4.5, s.color))
			}
		}
	}
}

// radar draws radar charts: one spoke per category, rings at the axis steps.
func (r *slideRenderer) radar(b *strings.Builder, kind, plot *xmltree.Node, ss []series, box rect, label labelFn, fs float64, text Color) {
	n := 0
	lo, hi := 0.0, 0.0
	for _, s := range ss {
		n = max(n, len(s.vals))
		for _, v := range s.vals {
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
	}
	if n < 3 {
		return
	}
	lo, hi, step := niceRange(lo, hi, true)
	cx0, cy0 := box.x+box.w/2, box.y+box.h/2
	rad := math.Min(box.w, box.h)/2 - fs*2
	pt := func(i int, v float64) (float64, float64) {
		a := -math.Pi/2 + 2*math.Pi*float64(i)/float64(n)
		rr := (v - lo) / (hi - lo) * rad
		return cx0 + rr*math.Cos(a), cy0 + rr*math.Sin(a)
	}
	valAx := plot.Child("valAx")
	gs := chartStroke(valAx.Child("majorGridlines"), r.colorCtx(), hexColor("D9D9D9"), 1)
	if gs == "" {
		gs = ` stroke="#D9D9D9" stroke-width="1"`
	}
	for v := lo + step; v <= hi+step/2; v += step {
		var d strings.Builder
		for i := 0; i <= n; i++ {
			x, y := pt(i%n, v)
			cmd := "L"
			if i == 0 {
				cmd = "M"
			}
			fmt.Fprintf(&d, "%s%s %s", cmd, f(x), f(y))
		}
		fmt.Fprintf(b, `<path d="%s" fill="none"%s/>`, d.String(), gs)
		label(cx0-fs*0.3, cy0-(v-lo)/(hi-lo)*rad+fs*0.35, fmtNum(v, false), "end", fs, text)
	}
	cats := ss[0].cats
	for i := 0; i < n; i++ {
		x, y := pt(i, hi)
		fmt.Fprintf(b, `<path d="M%s %sL%s %s" fill="none"%s/>`, f(cx0), f(cy0), f(x), f(y), gs)
		name := ""
		if i < len(cats) {
			name = cats[i]
		}
		lx, ly := pt(i, hi+(hi-lo)*0.08)
		anchor := "middle"
		if lx < cx0-1 {
			anchor = "end"
		} else if lx > cx0+1 {
			anchor = ""
		}
		label(lx, ly+fs*0.35, name, anchor, fs, text)
	}
	filled := kind.Child("radarStyle").Attr("val") == "filled"
	for _, s := range ss {
		var d strings.Builder
		for i := 0; i < n; i++ {
			x, y := pt(i, at(s.vals, i))
			cmd := "L"
			if i == 0 {
				cmd = "M"
			}
			fmt.Fprintf(&d, "%s%s %s", cmd, f(x), f(y))
		}
		d.WriteString("Z")
		if filled {
			fmt.Fprintf(b, `<path d="%s" %s/>`, d.String(), paint("fill", s.color))
			continue
		}
		fmt.Fprintf(b, `<path d="%s" fill="none" stroke="%s" stroke-width="%s" stroke-linejoin="round"/>`, d.String(), s.color.Hex(), f(2.25*ptPx))
		if sym := s.node.Path("marker/symbol").Attr("val"); sym != "none" && kind.Child("radarStyle").Attr("val") == "marker" {
			if sym == "" {
				sym = autoMarker(int(s.node.Child("idx").Int("val", 0)))
			}
			for i := 0; i < n; i++ {
				x, y := pt(i, at(s.vals, i))
				b.WriteString(markerShape(sym, x, y, 3.5, s.color))
			}
		}
	}
}

// stockMarks draws the high-low lines and the close marks of a stock chart (series order:
// [open,] high, low, close), and up-down bars when there is an open series.
func (r *slideRenderer) stockMarks(b *strings.Builder, kind *xmltree.Node, ss []series, plotR rect, band float64, valPos func(float64) float64) {
	if len(ss) < 3 {
		return
	}
	o := 0
	if len(ss) >= 4 {
		o = 1
	}
	hiS, loS, clS := ss[o], ss[o+1], ss[o+2]
	y := func(v float64) float64 { return plotR.y + plotR.h - valPos(v) }
	for i := range clS.vals {
		x := plotR.x + (float64(i)+0.5)*band
		fmt.Fprintf(b, `<path d="M%s %sV%s" fill="none" stroke="#000000" stroke-width="1"/>`, f(x), f(y(at(hiS.vals, i))), f(y(at(loS.vals, i))))
		if o == 1 {
			op, cl := at(ss[0].vals, i), at(clS.vals, i)
			fill := "#FFFFFF"
			if cl < op {
				fill = "#000000"
			}
			w := band * 0.4
			fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" fill="%s" stroke="#000000" stroke-width="1"/>`,
				f(x-w/2), f(y(math.Max(op, cl))), f(w), f(math.Max(1, math.Abs(y(op)-y(cl)))), fill)
			continue
		}
		fmt.Fprintf(b, `<path d="M%s %sH%s" fill="none" stroke="%s" stroke-width="2"/>`, f(x), f(y(at(clS.vals, i))), f(x+band*0.2), clS.color.Hex())
	}
}

// ofPie draws a pie of pie or bar of pie: the last splitPos points (by position) go into a
// second pie or a stacked bar on the right, joined to their summary slice by two lines.
func (r *slideRenderer) ofPie(b *strings.Builder, kind *xmltree.Node, s series, box rect, label labelFn, fs float64) {
	n := len(s.vals)
	k := int(kind.Child("splitPos").Int("val", 2))
	if k < 1 || k >= n {
		k = min(2, n-1)
	}
	if k < 1 {
		return
	}
	cx := r.colorCtx()
	ser := kind.Children("ser")[0]
	main, other := s.vals[:n-k], s.vals[n-k:]
	sumOther := 0.0
	for _, v := range other {
		sumOther += math.Max(0, v)
	}
	total := sumOther
	for _, v := range main {
		total += math.Max(0, v)
	}
	if total <= 0 {
		return
	}
	rad := math.Min(box.w*0.3, box.h/2-fs)
	mx, my := box.x+box.w*0.3, box.y+box.h/2
	// the summary slice sits on the right, centered on the horizontal
	ang := -sumOther / total * math.Pi
	sector := func(cx0, cy0, rr, a0, a1 float64, col Color) {
		large := "0"
		if a1-a0 > math.Pi {
			large = "1"
		}
		p := func(a float64) string { return f(cx0+rr*math.Cos(a)) + " " + f(cy0+rr*math.Sin(a)) }
		fmt.Fprintf(b, `<path d="M%s %sL%sA%s %s 0 %s 1 %sZ" %s stroke="#ffffff" stroke-width="1"/>`,
			f(cx0), f(cy0), p(a0), f(rr), f(rr), large, p(a1), paint("fill", col))
	}
	otherA0 := ang
	otherA1 := ang + sumOther/total*2*math.Pi
	sector(mx, my, rad, otherA0, otherA1, r.accent(n-k, cx))
	ang = otherA1
	for i, v := range main {
		if v <= 0 {
			continue
		}
		a1 := ang + v/total*2*math.Pi
		sector(mx, my, rad, ang, a1, r.pointColor(ser, i, cx))
		ang = a1
	}
	size := float64(kind.Child("secondPieSize").Int("val", 75)) / 100
	sx, sy := box.x+box.w*0.78, my
	bar := kind.Child("ofPieType").Attr("val") == "bar"
	p0x, p0y := mx+rad*math.Cos(otherA0), my+rad*math.Sin(otherA0)
	p1x, p1y := mx+rad*math.Cos(otherA1), my+rad*math.Sin(otherA1)
	if bar {
		bh, bw := rad*2*size, rad*0.5*size
		y := sy + bh/2
		for i, v := range other {
			h := math.Max(0, v) / sumOther * bh
			fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" %s stroke="#ffffff" stroke-width="1"/>`, f(sx-bw/2), f(y-h), f(bw), f(h), paint("fill", r.pointColor(ser, n-k+i, cx)))
			y -= h
		}
		fmt.Fprintf(b, `<path d="M%s %sL%s %sM%s %sL%s %s" fill="none" stroke="#7F7F7F" stroke-width="1"/>`,
			f(p0x), f(p0y), f(sx-bw/2), f(sy-bh/2), f(p1x), f(p1y), f(sx-bw/2), f(sy+bh/2))
		return
	}
	r2 := rad * size
	a := math.Pi - sumOther/total*math.Pi
	for i, v := range other {
		if v <= 0 {
			continue
		}
		a1 := a + v/sumOther*2*math.Pi
		sector(sx, sy, r2, a, a1, r.pointColor(ser, n-k+i, cx))
		a = a1
	}
	fmt.Fprintf(b, `<path d="M%s %sL%s %sM%s %sL%s %s" fill="none" stroke="#7F7F7F" stroke-width="1"/>`,
		f(p0x), f(p0y), f(sx), f(sy-r2), f(p1x), f(p1y), f(sx), f(sy+r2))
}

// surface draws a surface chart seen from above: categories across, series down, and the
// values as bands in the colors of the value levels.
func (r *slideRenderer) surface(b *strings.Builder, kind *xmltree.Node, ss []series, box rect, fs float64) {
	n := 0
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, s := range ss {
		n = max(n, len(s.vals))
		for _, v := range s.vals {
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
	}
	if n < 2 || len(ss) < 2 || math.IsInf(lo, 0) {
		return
	}
	lo, hi, step := niceRange(lo, hi, false)
	cx := r.colorCtx()
	cw, ch := box.w/float64(n-1), box.h/float64(len(ss)-1)
	for j := 0; j+1 < len(ss); j++ {
		for i := 0; i+1 < n; i++ {
			v := (at(ss[j].vals, i) + at(ss[j].vals, i+1) + at(ss[j+1].vals, i) + at(ss[j+1].vals, i+1)) / 4
			band := int(math.Floor((v - lo) / step))
			fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" %s/>`,
				f(box.x+float64(i)*cw), f(box.y+box.h-float64(j+1)*ch), f(cw+0.5), f(ch+0.5), paint("fill", r.accent(band, cx)))
		}
	}
}

// chart3D is the description of a 3D chart plot that the player draws with three.js.
type chart3D struct {
	Type    string      `json:"type"` // bar, line, area, pie, surface
	Plot    [4]float64  `json:"plot"` // x, y, w, h of the plot area (slide px)
	Lo      float64     `json:"lo"`
	Hi      float64     `json:"hi"`
	Series  [][]float64 `json:"series"`
	Colors  []string    `json:"colors"`
	Stacked bool        `json:"stacked,omitempty"`
	Horiz   bool        `json:"horiz,omitempty"`
	RotX    float64     `json:"rotX"`
	RotY    float64     `json:"rotY"`
	Persp   float64     `json:"persp"`
}

// chart3DAttr returns the data attribute of a 3D chart's plot (the player replaces the flat
// drawing inside the group with a picture rendered with three.js).
func (r *slideRenderer) chart3DAttr(cs *xmltree.Node, kind string, plot rect, lo, hi float64, ss []series, colors []Color, stacked, horiz bool) string {
	v := cs.Path("chart/view3D")
	c := chart3D{Type: kind, Plot: [4]float64{plot.x, plot.y, plot.w, plot.h}, Lo: lo, Hi: hi, Stacked: stacked, Horiz: horiz,
		RotX: float64(v.Child("rotX").Int("val", 15)), RotY: float64(v.Child("rotY").Int("val", 20)), Persp: float64(v.Child("perspective").Int("val", 30))}
	if kind == "pie" {
		c.RotX = float64(v.Child("rotX").Int("val", 30))
		c.RotY = float64(v.Child("rotY").Int("val", 0))
	}
	for _, s := range ss {
		c.Series = append(c.Series, s.vals)
	}
	for _, col := range colors {
		c.Colors = append(c.Colors, col.Hex())
	}
	r.webGL = true
	js, _ := json.Marshal(c)
	return ` data-df-pptx-chart3d="` + esc(string(js)) + `"`
}

// autoMarker returns the marker PowerPoint gives a series without c:marker: diamond, square,
// triangle, x, star, circle, plus, in turn.
func autoMarker(idx int) string {
	list := []string{"diamond", "square", "triangle", "x", "star", "circle", "plus"}
	return list[(idx%len(list)+len(list))%len(list)]
}
