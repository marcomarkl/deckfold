package render

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"deckfold/internal/pptx/model"
	"deckfold/internal/pptx/xmltree"
)

// chart draws a chart from the data cache PowerPoint stores with it (c:numCache, c:strCache):
// column and bar charts (clustered, stacked, 100 %), line, area, pie and doughnut charts, with
// axes, gridlines, legend, data labels and the title. Other types become a placeholder.
type series struct {
	name   string
	cats   []string
	vals   []float64
	color  Color
	sppr   *xmltree.Node
	node   *xmltree.Node // c:ser
	labels bool
	xs     []float64 // x values (scatter, bubble)
	sizes  []float64 // bubble sizes
}

func (r *slideRenderer) chart(b *strings.Builder, gf, gd *xmltree.Node, part model.Part, box rect) bool {
	rel, ok := part.Rels[gd.Child("chart").RID("id")]
	if !ok || rel.External {
		return false
	}
	cs, err := r.pres.Pkg.XML(rel.Target)
	if err != nil {
		return false
	}
	chart := cs.Child("chart")
	plot := chart.Child("plotArea")
	if plot == nil {
		return false
	}
	var kind *xmltree.Node
	for _, k := range plot.Kids {
		switch k.Name {
		case "barChart", "bar3DChart", "lineChart", "line3DChart", "areaChart", "area3DChart", "pieChart", "pie3DChart", "doughnutChart",
			"scatterChart", "radarChart", "bubbleChart", "stockChart", "surfaceChart", "surface3DChart", "ofPieChart":
			if kind == nil {
				kind = k
			}
		}
	}
	if kind == nil {
		return false
	}
	r.curChart = cs
	cx := r.colorCtx()
	var ss []series
	for i, s := range kind.Children("ser") {
		se := series{cats: cacheStrings(s.Child("cat")), vals: cacheNums(s.Child("val")), sppr: s.Child("spPr"), node: s}
		if s.Child("yVal") != nil {
			se.vals, se.xs, se.cats = cacheNums(s.Child("yVal")), cacheNums(s.Child("xVal")), cacheStrings(s.Child("xVal"))
			se.sizes = cacheNums(s.Child("bubbleSize"))
		}
		if names := cacheStrings(s.Child("tx")); len(names) > 0 {
			se.name = strings.TrimSpace(names[0])
		}
		idx := int(s.Child("idx").Int("val", int64(i)))
		se.color = r.seriesColor(se.sppr, idx, cx)
		se.labels = s.Path("dLbls/showVal").Bool("val", false) || kind.Path("dLbls/showVal").Bool("val", false)
		ss = append(ss, se)
	}
	if len(ss) == 0 {
		return false
	}
	face := r.fonts.Face(r.typeface("+mn-lt"), false, false)
	fs := 10.0 * ptPx
	if sz := cs.Path("txPr/p/pPr/defRPr"); sz != nil && sz.Int("sz", 0) > 0 {
		fs = float64(sz.Int("sz", 1000)) / 100 * ptPx
	}
	text := hexColor("595959")
	label := func(x, y float64, s string, anchor string, size float64, col Color) {
		w := measure(face, s, size)
		switch anchor {
		case "middle":
			x -= w / 2
		case "end":
			x -= w
		}
		face.Use(s)
		r.writeText(b, []textSeg{{x: x, y: y, text: s, face: face, size: size, color: col}})
	}
	inner := rect{box.x + 8, box.y + 8, box.w - 16, box.h - 16}
	// title
	if t := chart.Child("title"); t != nil && !chart.Child("autoTitleDeleted").Bool("val", false) {
		title := strings.TrimSpace(plainText(t.Path("tx/rich")))
		if title == "" && len(ss) == 1 {
			title = ss[0].name
		}
		if title != "" {
			ts := 18 * ptPx // PowerPoint's default title size
			if sz := t.Path("tx/rich/p/pPr/defRPr"); sz != nil && sz.Int("sz", 0) > 0 {
				ts = float64(sz.Int("sz", 1800)) / 100 * ptPx
			} else if sz := t.Find("rPr"); sz != nil && sz.Int("sz", 0) > 0 {
				ts = float64(sz.Int("sz", 1800)) / 100 * ptPx
			}
			label(inner.x+inner.w/2, inner.y+ts, title, "middle", ts, hexColor("000000"))
			inner.y += ts * 1.6
			inner.h -= ts * 1.6
		}
	}
	// legend
	if lg := chart.Child("legend"); lg != nil {
		pos := lg.Child("legendPos").Attr("val")
		if pos == "" {
			pos = "r"
		}
		names := []string{}
		cols := []Color{}
		if strings.HasPrefix(kind.Name, "pie") || kind.Name == "doughnutChart" || kind.Name == "ofPieChart" {
			for i, c := range ss[0].cats {
				names = append(names, c)
				cols = append(cols, r.pointColor(kind.Children("ser")[0], i, cx))
			}
		} else {
			for _, s := range ss {
				names = append(names, s.name)
				cols = append(cols, s.color)
			}
		}
		maxW := 0.0
		for _, nm := range names {
			maxW = math.Max(maxW, measure(face, nm, fs))
		}
		item := func(x, y float64, i int) {
			fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" %s/>`, f(x), f(y-fs*0.7), f(fs*0.7), f(fs*0.7), paint("fill", cols[i]))
			label(x+fs, y, names[i], "", fs, text)
		}
		switch pos {
		case "b", "t":
			total := 0.0
			for _, nm := range names {
				total += fs*2 + measure(face, nm, fs)
			}
			x := inner.x + (inner.w-total)/2
			y := inner.y + inner.h - fs*0.2
			if pos == "t" {
				y = inner.y + fs
				inner.y += fs * 1.8
			}
			inner.h -= fs * 1.8
			for i, nm := range names {
				item(x, y, i)
				x += fs*2 + measure(face, nm, fs)
			}
		default:
			w := maxW + fs*2
			x := inner.x + inner.w - w
			if pos == "l" {
				x = inner.x
				inner.x += w
			}
			inner.w -= w + fs
			y := inner.y + inner.h/2 - float64(len(names))*fs*0.75 + fs
			for i := range names {
				item(x, y, i)
				y += fs * 1.5
			}
		}
	}
	switch {
	case kind.Name == "pie3DChart":
		cols := []Color{}
		for i := range ss[0].vals {
			cols = append(cols, r.pointColor(kind.Children("ser")[0], i, cx))
		}
		fmt.Fprintf(b, `<g%s>`, r.chart3DAttr(cs, "pie", inner, 0, 0, ss[:1], cols, false, false))
		r.pie(b, kind, ss[0], inner, cx, label, fs)
		b.WriteString("</g>")
	case strings.HasPrefix(kind.Name, "pie") || kind.Name == "doughnutChart":
		r.pie(b, kind, ss[0], inner, cx, label, fs)
	case kind.Name == "ofPieChart":
		r.ofPie(b, kind, ss[0], inner, label, fs)
	case kind.Name == "scatterChart" || kind.Name == "bubbleChart":
		r.scatter(b, kind, plot, ss, inner, label, fs, text)
	case kind.Name == "radarChart":
		r.radar(b, kind, plot, ss, inner, label, fs, text)
	case kind.Name == "surfaceChart" || kind.Name == "surface3DChart":
		if kind.Name == "surface3DChart" {
			lo, hi := math.Inf(1), math.Inf(-1)
			for _, s := range ss {
				for _, v := range s.vals {
					lo, hi = math.Min(lo, v), math.Max(hi, v)
				}
			}
			lo, hi, _ = niceRange(lo, hi, false)
			cols := []Color{}
			for i := 0; i < 12; i++ {
				cols = append(cols, r.accent(i, cx))
			}
			fmt.Fprintf(b, `<g%s>`, r.chart3DAttr(cs, "surface", inner, lo, hi, ss, cols, false, false))
			r.surface(b, kind, ss, inner, fs)
			b.WriteString("</g>")
		} else {
			r.surface(b, kind, ss, inner, fs)
		}
	default:
		r.xyChart(b, kind, plot, ss, inner, label, fs, text)
	}
	return true
}

type labelFn func(x, y float64, s, anchor string, size float64, col Color)

func (r *slideRenderer) xyChart(b *strings.Builder, kind, plot *xmltree.Node, ss []series, box rect, label labelFn, fs float64, text Color) {
	horizontal := kind.Child("barDir").Attr("val") == "bar"
	grouping := kind.Child("grouping").Attr("val")
	stacked := grouping == "stacked" || grouping == "percentStacked"
	percent := grouping == "percentStacked"
	isBar := strings.HasPrefix(kind.Name, "bar")
	ncat := 0
	for _, s := range ss {
		ncat = max(ncat, len(s.vals))
	}
	if ncat == 0 {
		return
	}
	cats := ss[0].cats
	// value range
	lo, hi := 0.0, 0.0
	for i := 0; i < ncat; i++ {
		pos, neg := 0.0, 0.0
		for _, s := range ss {
			v := at(s.vals, i)
			if percent {
				t := 0.0
				for _, s2 := range ss {
					t += math.Abs(at(s2.vals, i))
				}
				if t > 0 {
					v = v / t * 100
				}
			}
			if stacked {
				if v >= 0 {
					pos += v
				} else {
					neg += v
				}
			} else {
				hi, lo = math.Max(hi, v), math.Min(lo, v)
			}
		}
		if stacked {
			hi, lo = math.Max(hi, pos), math.Min(lo, neg)
		}
	}
	valAx := plot.Child("valAx")
	if v := valAx.Path("scaling/max"); v != nil {
		hi = parseF(v.Attr("val"), hi)
	}
	if v := valAx.Path("scaling/min"); v != nil {
		lo = parseF(v.Attr("val"), lo)
	}
	step := axisStep(lo, hi)
	if valAx.Path("scaling/max") == nil {
		hi = math.Ceil((hi+0.05*(hi-lo))/step) * step
	}
	if valAx.Path("scaling/min") == nil {
		lo = math.Floor(lo/step) * step
	}
	if hi <= lo {
		hi = lo + 1
	}
	if percent {
		lo, hi, step = 0, 100, 10
	}
	// axis labels take room on the left (value axis) and bottom (categories)
	lw := 0.0
	for v := lo; v <= hi+step/2; v += step {
		lw = math.Max(lw, measure(r.fonts.Face(r.typeface("+mn-lt"), false, false), fmtNum(v, percent), fs))
	}
	plotR := rect{box.x + lw + fs*0.6, box.y + fs*0.5, box.w - lw - fs*0.6, box.h - fs*2}
	if horizontal {
		plotR = rect{box.x + fs*6, box.y + fs*0.5, box.w - fs*6, box.h - fs*2}
	}
	valPos := func(v float64) float64 { // along the value axis, in px from the axis start
		len := plotR.h
		if horizontal {
			len = plotR.w
		}
		return (v - lo) / (hi - lo) * len
	}
	// gridlines and value labels; lines without c:spPr are black, as PowerPoint draws them
	grid := valAx.Child("majorGridlines")
	gridStroke := chartStroke(grid, r.colorCtx(), hexColor("000000"), 1)
	minorTicks := tickMark(valAx.Child("minorTickMark"))
	majorTicks := tickMark(valAx.Child("majorTickMark"))
	for v := lo; v <= hi+step/2; v += step {
		p := valPos(v)
		if horizontal {
			x := plotR.x + p
			if grid != nil && gridStroke != "" {
				fmt.Fprintf(b, `<path d="M%s %sV%s" fill="none"%s/>`, f(x), f(plotR.y), f(plotR.y+plotR.h), gridStroke)
			}
			if !valAx.Child("delete").Bool("val", false) {
				label(x, plotR.y+plotR.h+fs*1.5, fmtNum(v, percent), "middle", fs, text)
			}
		} else {
			y := plotR.y + plotR.h - p
			if grid != nil && gridStroke != "" {
				fmt.Fprintf(b, `<path d="M%s %sH%s" fill="none"%s/>`, f(plotR.x), f(y), f(plotR.x+plotR.w), gridStroke)
			}
			if !valAx.Child("delete").Bool("val", false) {
				label(plotR.x-fs*0.6, y+fs*0.35, fmtNum(v, percent), "end", fs, text)
			}
		}
	}
	valLine := chartStroke(valAx, r.colorCtx(), hexColor("000000"), 1)
	if !valAx.Child("delete").Bool("val", false) && valLine != "" {
		var d strings.Builder
		for v := lo; v <= hi+step/2; v += step {
			for k := 0; k < 5; k++ {
				ln, kind := 0.0, majorTicks
				if k > 0 {
					kind = minorTicks
				}
				switch kind {
				case "cross":
					ln = 4
				case "out", "in":
					ln = 4
				}
				if k > 0 {
					ln /= 2
				}
				if ln == 0 || (k > 0 && v+step*float64(k)/5 > hi+1e-9) {
					continue
				}
				p := valPos(v + step*float64(k)/5)
				a, c := -ln, ln
				if kind == "out" {
					c = 0
				} else if kind == "in" {
					a = 0
				}
				if horizontal {
					x := plotR.x + p
					fmt.Fprintf(&d, "M%s %sV%s", f(x), f(plotR.y+plotR.h-a), f(plotR.y+plotR.h-c))
				} else {
					y := plotR.y + plotR.h - p
					fmt.Fprintf(&d, "M%s %sH%s", f(plotR.x+a), f(y), f(plotR.x+c))
				}
			}
		}
		if horizontal {
			fmt.Fprintf(&d, "M%s %sH%s", f(plotR.x), f(plotR.y+plotR.h), f(plotR.x+plotR.w))
		} else {
			fmt.Fprintf(&d, "M%s %sV%s", f(plotR.x), f(plotR.y), f(plotR.y+plotR.h))
		}
		fmt.Fprintf(b, `<path d="%s" fill="none"%s/>`, d.String(), valLine)
	}
	// category axis line, its tick marks between the categories, and labels
	zero := valPos(math.Max(lo, math.Min(0, hi)))
	band := plotR.w / float64(ncat)
	catAxis := plot.Child("catAx")
	// area charts put the categories on the data points (the axis crosses between them is
	// "midCat"), the others in the middle of a band
	onPoints := strings.HasPrefix(kind.Name, "area") && valAx.Child("crossBetween").Attr("val") != "between" && ncat > 1
	catPos := func(i float64) float64 {
		if onPoints {
			return i * plotR.w / float64(ncat-1)
		}
		return (i + 0.5) * band
	}
	catLine := chartStroke(catAxis, r.colorCtx(), hexColor("000000"), 1)
	catTicks := tickMark(catAxis.Child("majorTickMark"))
	if horizontal {
		band = plotR.h / float64(ncat)
	}
	if catLine != "" && !catAxis.Child("delete").Bool("val", false) {
		var d strings.Builder
		if horizontal {
			fmt.Fprintf(&d, "M%s %sV%s", f(plotR.x+zero), f(plotR.y), f(plotR.y+plotR.h))
		} else {
			fmt.Fprintf(&d, "M%s %sH%s", f(plotR.x), f(plotR.y+plotR.h-zero), f(plotR.x+plotR.w))
		}
		for i := 0; i <= ncat && catTicks != "none"; i++ {
			if horizontal {
				y := plotR.y + float64(i)*band
				fmt.Fprintf(&d, "M%s %sH%s", f(plotR.x+zero-4), f(y), f(plotR.x+zero+4))
			} else {
				x := plotR.x + float64(i)*band
				if onPoints {
					if i == ncat {
						break
					}
					x = plotR.x + catPos(float64(i))
				}
				fmt.Fprintf(&d, "M%s %sV%s", f(x), f(plotR.y+plotR.h-zero-4), f(plotR.y+plotR.h-zero+4))
			}
		}
		fmt.Fprintf(b, `<path d="%s" fill="none"%s/>`, d.String(), catLine)
	}
	catAx := plot.Child("catAx")
	for i := 0; i < ncat && !catAx.Child("delete").Bool("val", false); i++ {
		name := ""
		if i < len(cats) {
			name = cats[i]
		}
		if horizontal {
			// categories run from the bottom up, as in PowerPoint
			y := plotR.y + plotR.h - (float64(i)+0.5)*band
			label(plotR.x-fs*0.4, y+fs*0.35, name, "end", fs, text)
		} else {
			label(plotR.x+catPos(float64(i)), plotR.y+plotR.h+fs*1.5, name, "middle", fs, text)
		}
	}
	gap := float64(kind.Child("gapWidth").Int("val", 150)) / 100
	overlap := float64(kind.Child("overlap").Int("val", 0)) / 100
	if stacked {
		overlap = 1
	}
	nser := float64(len(ss))
	// bar width w with n bars per category: band = w*(n - (n-1)*overlap) + gap*w
	w := band / (nser - (nser-1)*overlap + gap)
	if kind.Name == "stockChart" {
		r.stockMarks(b, kind, ss, plotR, band, valPos)
		return
	}
	if strings.Contains(kind.Name, "3D") {
		// the plot of a 3D chart: the player draws it with three.js
		cols := []Color{}
		for _, s := range ss {
			cols = append(cols, s.color)
		}
		typ := map[string]string{"bar3DChart": "bar", "line3DChart": "line", "area3DChart": "area"}[kind.Name]
		fmt.Fprintf(b, `<g%s>`, r.chart3DAttr(r.curChart, typ, plotR, lo, hi, ss, cols, stacked, horizontal))
		defer b.WriteString("</g>")
	}
	if !isBar {
		r.lines(b, kind, ss, plotR, band, valPos, horizontal, percent, stacked)
		return
	}
	for i := 0; i < ncat; i++ {
		pos, neg := 0.0, 0.0
		total := 0.0
		for _, s := range ss {
			total += math.Abs(at(s.vals, i))
		}
		for si, s := range ss {
			v := at(s.vals, i)
			if percent && total > 0 {
				v = v / total * 100
			}
			start := 0.0
			if stacked {
				if v >= 0 {
					start = pos
					pos += v
				} else {
					start = neg
					neg += v
				}
			}
			a, c := valPos(start), valPos(start+v)
			off := gap * w / 2
			if !stacked {
				off += float64(si) * w * (1 - overlap)
			}
			var x, y, bw, bh float64
			if horizontal {
				x, bw = plotR.x+math.Min(a, c), math.Abs(c-a)
				y, bh = plotR.y+plotR.h-float64(i+1)*band+off, w
			} else {
				x, bw = plotR.x+float64(i)*band+off, w
				y, bh = plotR.y+plotR.h-math.Max(a, c), math.Abs(c-a)
			}
			fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" %s/>`, f(x), f(y), f(bw), f(bh), paint("fill", s.color))
			if s.labels {
				t := fmtNum(at(s.vals, i), false)
				if horizontal {
					label(x+bw+fs*0.3, y+bh/2+fs*0.35, t, "", fs, text)
				} else {
					label(x+bw/2, y-fs*0.3, t, "middle", fs, text)
				}
			}
		}
	}
}

func (r *slideRenderer) lines(b *strings.Builder, kind *xmltree.Node, ss []series, plotR rect, band float64, valPos func(float64) float64, horizontal, percent, stacked bool) {
	area := strings.HasPrefix(kind.Name, "area")
	ncat := len(ss[0].vals)
	acc := make([]float64, ncat)
	for si, s := range ss {
		var d strings.Builder
		var pts [][2]float64
		for i := 0; i < ncat; i++ {
			v := at(s.vals, i)
			if stacked {
				v += acc[i]
				acc[i] = v
			}
			x := plotR.x + (float64(i)+0.5)*band
			if area {
				x = plotR.x + float64(i)*plotR.w/math.Max(1, float64(ncat-1))
			}
			y := plotR.y + plotR.h - valPos(v)
			pts = append(pts, [2]float64{x, y})
		}
		// lines are smoothed unless c:smooth says otherwise (PowerPoint's reading of a missing element)
		smooth := !area && s.node.Child("smooth").Bool("val", true)
		for i, p := range pts {
			switch {
			case i == 0:
				d.WriteString("M" + f(p[0]) + " " + f(p[1]))
			case smooth:
				// Catmull-Rom through the points, as cubic Bezier segments
				p0, p3 := pts[max(0, i-2)], pts[min(len(pts)-1, i+1)]
				p1 := pts[i-1]
				c1 := [2]float64{p1[0] + (p[0]-p0[0])/6, p1[1] + (p[1]-p0[1])/6}
				c2 := [2]float64{p[0] - (p3[0]-p1[0])/6, p[1] - (p3[1]-p1[1])/6}
				d.WriteString("C" + f(c1[0]) + " " + f(c1[1]) + " " + f(c2[0]) + " " + f(c2[1]) + " " + f(p[0]) + " " + f(p[1]))
			default:
				d.WriteString("L" + f(p[0]) + " " + f(p[1]))
			}
		}
		if area {
			base := plotR.y + plotR.h - valPos(0)
			d.WriteString("L" + f(pts[len(pts)-1][0]) + " " + f(base) + "L" + f(pts[0][0]) + " " + f(base) + "Z")
			fmt.Fprintf(b, `<path d="%s" %s/>`, d.String(), paint("fill", s.color))
			continue
		}
		fmt.Fprintf(b, `<path d="%s" fill="none" %s stroke-width="2.67" stroke-linejoin="round"/>`, d.String(), paint("stroke", s.color))
		if kind.Child("marker").Bool("val", true) {
			sym := s.node.Path("marker/symbol").Attr("val")
			if sym == "" {
				// PowerPoint's automatic marker symbols, one per series
				sym = []string{"diamond", "square", "triangle", "x", "star", "circle", "plus", "dash"}[si%8]
			}
			if sym != "none" {
				for _, p := range pts {
					b.WriteString(markerShape(sym, p[0], p[1], 4.5, s.color))
				}
			}
		}
	}
}

func markerShape(sym string, x, y, r float64, c Color) string {
	fill := paint("fill", c)
	switch sym {
	case "diamond":
		return fmt.Sprintf(`<path d="M%s %sL%s %sL%s %sL%s %sZ" %s/>`, f(x), f(y-r), f(x+r), f(y), f(x), f(y+r), f(x-r), f(y), fill)
	case "square":
		return fmt.Sprintf(`<rect x="%s" y="%s" width="%s" height="%s" %s/>`, f(x-r*0.8), f(y-r*0.8), f(r*1.6), f(r*1.6), fill)
	case "triangle":
		return fmt.Sprintf(`<path d="M%s %sL%s %sL%s %sZ" %s/>`, f(x), f(y-r), f(x+r), f(y+r*0.8), f(x-r), f(y+r*0.8), fill)
	case "x", "plus", "star", "dash", "dot":
		return fmt.Sprintf(`<path d="M%s %sL%s %sM%s %sL%s %s" fill="none" %s stroke-width="1.5"/>`, f(x-r), f(y-r), f(x+r), f(y+r), f(x-r), f(y+r), f(x+r), f(y-r), paint("stroke", c))
	}
	return fmt.Sprintf(`<circle cx="%s" cy="%s" r="%s" %s/>`, f(x), f(y), f(r*0.8), fill)
}

// chartStroke returns stroke attributes for the c:spPr/a:ln of a chart element, or the default.
func chartStroke(n *xmltree.Node, cx colorCtx, def Color, w float64) string {
	ln := n.Path("spPr/ln")
	if ln != nil {
		if ln.Child("noFill") != nil {
			return ""
		}
		if c := cx.colorChild(ln.Child("solidFill")); c.OK {
			def = c
		}
		if v := ln.Int("w", 0); v > 0 {
			w = float64(v) / emuPx
		}
	}
	return ` ` + paint("stroke", def) + ` stroke-width="` + f(w) + `"`
}

// tickMark returns the tick mark type of an axis ("cross" when the element is missing, the
// schema default).
func tickMark(n *xmltree.Node) string {
	if n == nil {
		return "cross"
	}
	if v := n.Attr("val"); v != "" {
		return v
	}
	return "cross"
}

func (r *slideRenderer) pie(b *strings.Builder, kind *xmltree.Node, s series, box rect, cx colorCtx, label labelFn, fs float64) {
	total := 0.0
	for _, v := range s.vals {
		if v > 0 {
			total += v
		}
	}
	if total <= 0 {
		return
	}
	rad := math.Min(box.w, box.h)/2 - fs
	cxp, cyp := box.x+box.w/2, box.y+box.h/2
	hole := 0.0
	if kind.Name == "doughnutChart" {
		// a missing c:holeSize is drawn without a hole, as PowerPoint does (it saves it as 0)
		hole = rad * float64(kind.Child("holeSize").Int("val", 0)) / 100
	}
	ang := float64(kind.Child("firstSliceAng").Int("val", 0))*math.Pi/180 - math.Pi/2
	ser := kind.Children("ser")[0]
	for i, v := range s.vals {
		if v <= 0 {
			continue
		}
		sweep := v / total * 2 * math.Pi
		a0, a1 := ang, ang+sweep
		large := "0"
		if sweep > math.Pi {
			large = "1"
		}
		p := func(a, rr float64) string { return f(cxp+rr*math.Cos(a)) + " " + f(cyp+rr*math.Sin(a)) }
		var d string
		if sweep >= 2*math.Pi-1e-6 {
			d = "M" + p(0, rad) + "A" + f(rad) + " " + f(rad) + " 0 1 1 " + p(math.Pi, rad) + "A" + f(rad) + " " + f(rad) + " 0 1 1 " + p(0, rad) + "Z"
		} else if hole > 0 {
			d = "M" + p(a0, rad) + "A" + f(rad) + " " + f(rad) + " 0 " + large + " 1 " + p(a1, rad) + "L" + p(a1, hole) + "A" + f(hole) + " " + f(hole) + " 0 " + large + " 0 " + p(a0, hole) + "Z"
		} else {
			d = "M" + f(cxp) + " " + f(cyp) + "L" + p(a0, rad) + "A" + f(rad) + " " + f(rad) + " 0 " + large + " 1 " + p(a1, rad) + "Z"
		}
		fmt.Fprintf(b, `<path d="%s" %s stroke="#ffffff" stroke-width="1"/>`, d, paint("fill", r.pointColor(ser, i, cx)))
		if s.labels {
			m := (a0 + a1) / 2
			lr := (rad + hole) / 2
			if hole == 0 {
				lr = rad * 0.65
			}
			label(cxp+lr*math.Cos(m), cyp+lr*math.Sin(m)+fs*0.35, fmtNum(v, false), "middle", fs, hexColor("FFFFFF"))
		}
		ang = a1
	}
}

// seriesColor: the series fill, otherwise the theme accents in order (darker for later rounds).
func (r *slideRenderer) seriesColor(sppr *xmltree.Node, idx int, cx colorCtx) Color {
	if c := cx.colorChild(sppr.Child("solidFill")); c.OK {
		return c
	}
	if g := sppr.Child("gradFill"); g != nil {
		if c := cx.colorChild(g.Path("gsLst/gs")); c.OK {
			return c
		}
	}
	return r.accent(idx, cx)
}

func (r *slideRenderer) accent(idx int, cx colorCtx) Color {
	n := &xmltree.Node{Name: "schemeClr", Attrs: attrs("val", "accent"+strconv.Itoa(idx%6+1))}
	if round := idx / 6; round > 0 {
		n.Kids = append(n.Kids, &xmltree.Node{Name: "lumMod", Attrs: attrs("val", strconv.Itoa(100000-round*25000))})
	}
	return cx.colorOf(n)
}

// pointColor: a data point fill (c:dPt), otherwise one accent per point (pie and doughnut).
func (r *slideRenderer) pointColor(ser *xmltree.Node, i int, cx colorCtx) Color {
	for _, dp := range ser.Children("dPt") {
		if int(dp.Child("idx").Int("val", -1)) == i {
			if c := cx.colorChild(dp.Path("spPr/solidFill")); c.OK {
				return c
			}
		}
	}
	return r.accent(i, cx)
}

func at(v []float64, i int) float64 {
	if i < len(v) {
		return v[i]
	}
	return 0
}

func parseF(s string, def float64) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return def
	}
	return v
}

// axisStep picks the major unit of a value axis the way Excel and PowerPoint do it for automatic
// axes: from the power of ten of the maximum plus 5 % and the ratio to it.
func axisStep(lo, hi float64) float64 {
	top := math.Max(math.Abs(hi), math.Abs(lo))
	top += 0.05 * (hi - lo)
	if top <= 0 {
		return 1
	}
	p := math.Pow(10, math.Floor(math.Log10(top)))
	switch r := top / p; {
	case r > 5:
		return p
	case r > 2:
		return p / 2
	default:
		return p / 5
	}
}

func fmtNum(v float64, percent bool) string {
	s := strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64)
	if percent {
		s += "%"
	}
	return s
}

type strs []string

// cacheStrings reads the cached strings (or numbers as text) of a c:tx, c:cat or c:val.
func cacheStrings(n *xmltree.Node) strs {
	if n == nil {
		return nil
	}
	if v := n.Child("v"); v != nil {
		return strs{v.AllText()}
	}
	c := n.Find("strCache")
	if c == nil {
		c = n.Find("numCache")
	}
	if c == nil {
		if lit := n.Find("strLit"); lit != nil {
			c = lit
		}
	}
	if c == nil {
		return nil
	}
	count := int(c.Child("ptCount").Int("val", 0))
	out := make(strs, count)
	for _, pt := range c.Children("pt") {
		i := int(pt.Int("idx", -1))
		if i >= 0 && i < count {
			out[i] = pt.Child("v").AllText()
		} else if i >= count {
			out = append(out, pt.Child("v").AllText())
		}
	}
	return out
}

func cacheNums(n *xmltree.Node) []float64 {
	var out []float64
	for _, s := range cacheStrings(n) {
		out = append(out, parseF(s, 0))
	}
	return out
}
