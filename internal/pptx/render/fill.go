package render

import (
	"fmt"
	"math"
	"strings"

	"deckfold/internal/pptx/model"
	"deckfold/internal/pptx/xmltree"
)

// fillAttr returns the SVG fill attributes for a shape: the first fill element in the spPr
// chain, otherwise the style reference.
func (r *slideRenderer) fillAttr(sppr []*xmltree.Node, ref *xmltree.Node, cx colorCtx, part model.Part, box rect) string {
	for _, n := range sppr {
		if n == nil {
			continue
		}
		np := part
		if p, ok := r.nodePart[n]; ok {
			np = p
		}
		for _, k := range n.Kids {
			if s, ok := r.fillOf(k, cx, np, box); ok {
				return s
			}
		}
	}
	if ref != nil {
		return r.styleFill(ref, cx, part, box, false)
	}
	return `fill="none"`
}

// styleFill resolves a:fillRef / p:bgRef against the theme's fill styles.
func (r *slideRenderer) styleFill(ref *xmltree.Node, cx colorCtx, part model.Part, box rect, bg bool) string {
	idx := int(ref.Int("idx", 0))
	th := r.slide.Layout.Master.Theme
	var list []*xmltree.Node
	switch {
	case idx >= 1001:
		list, idx = th.BgFillStyles, idx-1001
	case idx >= 1:
		list, idx = th.FillStyles, idx-1
	default:
		return `fill="none"`
	}
	if idx < 0 || idx >= len(list) {
		return `fill="none"`
	}
	ph := cx.colorChild(ref)
	cx2 := cx
	cx2.ph = &ph
	if s, ok := r.fillOf(list[idx], cx2, part, box); ok {
		return s
	}
	return `fill="none"`
}

// fillOf turns one fill element into SVG attributes.
func (r *slideRenderer) fillOf(k *xmltree.Node, cx colorCtx, part model.Part, box rect) (string, bool) {
	switch k.Name {
	case "noFill":
		return `fill="none"`, true
	case "solidFill":
		c := cx.colorChild(k)
		if !c.OK {
			return `fill="none"`, true
		}
		return paint("fill", c), true
	case "gradFill":
		return `fill="url(#` + r.gradient(k, cx, box) + `)"`, true
	case "pattFill":
		fg, bgc := cx.colorChild(k.Child("fgClr")), cx.colorChild(k.Child("bgClr"))
		return `fill="url(#` + r.pattern(k.Attr("prst"), fg, bgc) + `)"`, true
	case "blipFill":
		name, ok := r.blipAsset(k.Child("blip"), part)
		if !ok {
			return `fill="none"`, true
		}
		id := r.newID("p")
		if tile := k.Child("tile"); tile != nil {
			// tiled picture: the image at its own size, scaled by sx/sy
			sx, sy := float64(tile.Int("sx", 100000))/100000, float64(tile.Int("sy", 100000))/100000
			sz := r.sizes[name]
			w, h := sz[0], sz[1]
			w, h = w*sx, h*sy
			if w <= 0 || h <= 0 {
				w, h = box.w, box.h
			}
			tx, ty := float64(tile.Int("tx", 0))/emuPx, float64(tile.Int("ty", 0))/emuPx
			fmt.Fprintf(&r.defs, `<pattern id="%s" patternUnits="userSpaceOnUse" x="%s" y="%s" width="%s" height="%s"><image href="assets/%s" width="%s" height="%s" preserveAspectRatio="none"/></pattern>`,
				id, f(box.x+tx), f(box.y+ty), f(w), f(h), name, f(w), f(h))
		} else {
			img := r.imageRect(k, box)
			fmt.Fprintf(&r.defs, `<pattern id="%s" patternUnits="userSpaceOnUse" x="0" y="0" width="%s" height="%s"><image href="assets/%s" x="%s" y="%s" width="%s" height="%s" preserveAspectRatio="none"/></pattern>`,
				id, f(float64(r.pres.Width)/emuPx*4), f(float64(r.pres.Height)/emuPx*4), name, f(img.x), f(img.y), f(img.w), f(img.h))
		}
		return `fill="url(#` + id + `)"`, true
	case "grpFill":
		return `fill="none"`, true
	}
	return "", false
}

func paint(attr string, c Color) string {
	s := attr + `="` + c.Hex() + `"`
	if c.A < 1 {
		s += " " + attr + `-opacity="` + f(c.A) + `"`
	}
	return s
}

func (r *slideRenderer) gradient(g *xmltree.Node, cx colorCtx, box rect) string {
	id := r.newID("g")
	var stops strings.Builder
	type stop struct {
		pos float64
		c   Color
	}
	var st []stop
	for _, gs := range g.Path("gsLst").Children("gs") {
		st = append(st, stop{float64(gs.Int("pos", 0)) / 100000, cx.colorChild(gs)})
	}
	for i := 1; i < len(st); i++ { // stable insertion sort by position
		for j := i; j > 0 && st[j].pos < st[j-1].pos; j-- {
			st[j], st[j-1] = st[j-1], st[j]
		}
	}
	for _, s := range st {
		op := ""
		if s.c.A < 1 {
			op = ` stop-opacity="` + f(s.c.A) + `"`
		}
		fmt.Fprintf(&stops, `<stop offset="%s" stop-color="%s"%s/>`, f(s.pos), s.c.Hex(), op)
	}
	if p := g.Child("path"); p != nil {
		// radial (circle, rect, shape): centered on fillToRect
		ft := p.Child("fillToRect")
		l, t, rr, bb := float64(ft.Int("l", 50000))/100000, float64(ft.Int("t", 50000))/100000, float64(ft.Int("r", 50000))/100000, float64(ft.Int("b", 50000))/100000
		cxp, cyp := box.x+box.w*(l+(1-l-rr)/2), box.y+box.h*(t+(1-t-bb)/2)
		rad := math.Hypot(math.Max(cxp-box.x, box.x+box.w-cxp), math.Max(cyp-box.y, box.y+box.h-cyp))
		fmt.Fprintf(&r.defs, `<radialGradient id="%s" gradientUnits="userSpaceOnUse" cx="%s" cy="%s" r="%s">%s</radialGradient>`, id, f(cxp), f(cyp), f(rad), stops.String())
		return id
	}
	ang := 0.0
	if lin := g.Child("lin"); lin != nil {
		ang = float64(lin.Int("ang", 0)) / 60000
	}
	// the gradient line runs through the box center at the angle, long enough to cover the box
	rad := ang * math.Pi / 180
	dx, dy := math.Cos(rad), math.Sin(rad)
	half := (math.Abs(box.w*dx) + math.Abs(box.h*dy)) / 2
	cxp, cyp := box.x+box.w/2, box.y+box.h/2
	fmt.Fprintf(&r.defs, `<linearGradient id="%s" gradientUnits="userSpaceOnUse" x1="%s" y1="%s" x2="%s" y2="%s">%s</linearGradient>`,
		id, f(cxp-dx*half), f(cyp-dy*half), f(cxp+dx*half), f(cyp+dy*half), stops.String())
	return id
}

// pattern draws a preset pattern as an 8x8 tile; unknown presets become a 50 % mix.
func (r *slideRenderer) pattern(prst string, fg, bg Color) string {
	id := r.newID("pt")
	if !fg.OK {
		fg = hexColor("000000")
	}
	if !bg.OK {
		bg = hexColor("FFFFFF")
	}
	var shapes string
	switch {
	case strings.HasPrefix(prst, "pct"):
		shapes = `<rect width="1" height="1" fill="FG"/><rect x="4" y="4" width="1" height="1" fill="FG"/>`
	case strings.Contains(prst, "Horz") || prst == "horzBrick":
		shapes = `<rect width="8" height="1" fill="FG"/><rect y="4" width="8" height="1" fill="FG"/>`
	case strings.Contains(prst, "Vert"):
		shapes = `<rect width="1" height="8" fill="FG"/><rect x="4" width="1" height="8" fill="FG"/>`
	case strings.Contains(prst, "Diag") || strings.Contains(prst, "diag"):
		shapes = `<path d="M0 8L8 0" stroke="FG" stroke-width="1"/>`
	case strings.Contains(prst, "Grid") || strings.Contains(prst, "Cross") || prst == "cross":
		shapes = `<rect width="8" height="1" fill="FG"/><rect width="1" height="8" fill="FG"/>`
	default:
		shapes = `<rect width="4" height="4" fill="FG"/><rect x="4" y="4" width="4" height="4" fill="FG"/>`
	}
	shapes = strings.ReplaceAll(shapes, "FG", fg.Hex())
	fmt.Fprintf(&r.defs, `<pattern id="%s" patternUnits="userSpaceOnUse" width="8" height="8"><rect width="8" height="8" fill="%s"/>%s</pattern>`, id, bg.Hex(), shapes)
	return id
}

// strokeAttr returns the SVG stroke attributes for the outline (a:ln) or "" for none.
func (r *slideRenderer) strokeAttr(sppr []*xmltree.Node, ref *xmltree.Node, cx colorCtx) string {
	ln := newProps()
	var styleLn *xmltree.Node
	if ref != nil {
		idx := int(ref.Int("idx", 0))
		th := r.slide.Layout.Master.Theme
		if idx >= 1 && idx <= len(th.LineStyles) {
			styleLn = th.LineStyles[idx-1]
			ph := cx.colorChild(ref)
			cx.ph = &ph
		}
	}
	ln.add(styleLn)
	for i := len(sppr) - 1; i >= 0; i-- {
		ln.add(sppr[i].Child("ln"))
	}
	fill := ln.elem("solidFill")
	if fill == nil || fill.Name == "noFill" {
		return ""
	}
	var c Color
	switch fill.Name {
	case "solidFill":
		c = cx.colorChild(fill)
	case "gradFill":
		c = cx.colorChild(fill.Path("gsLst/gs"))
	default:
		return ""
	}
	if !c.OK {
		return ""
	}
	w := float64(ln.int("w", 9525)) / emuPx
	if w <= 0 {
		w = 0.75
	}
	s := " " + paint("stroke", c) + ` stroke-width="` + f(w) + `"`
	if d := ln.elem("prstDash"); d != nil {
		if da := dashArray(d.Attr("val"), w); da != "" {
			s += ` stroke-dasharray="` + da + `"`
		}
	}
	switch ln.attr("cap") {
	case "rnd":
		s += ` stroke-linecap="round"`
	case "sq":
		s += ` stroke-linecap="square"`
	}
	if ln.elem("round") != nil {
		s += ` stroke-linejoin="round"`
	} else if ln.elem("bevel") != nil {
		s += ` stroke-linejoin="bevel"`
	}
	return s
}

func dashArray(v string, w float64) string {
	pat := map[string][]float64{
		"dot": {1, 1}, "sysDot": {1, 1}, "dash": {4, 3}, "sysDash": {3, 1}, "lgDash": {8, 3},
		"dashDot": {4, 3, 1, 3}, "sysDashDot": {3, 1, 1, 1}, "lgDashDot": {8, 3, 1, 3},
		"lgDashDotDot": {8, 3, 1, 3, 1, 3}, "sysDashDotDot": {3, 1, 1, 1, 1, 1},
	}[v]
	if pat == nil {
		return ""
	}
	parts := make([]string, len(pat))
	for i, p := range pat {
		parts[i] = f(p * w)
	}
	return strings.Join(parts, " ")
}
