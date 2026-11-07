package render

import (
	"fmt"
	"math"
	"strings"

	"deckfold/internal/pptx/xmltree"
)

// effects returns a filter attribute for the shape effects (outer shadow, glow). Effects
// without a counterpart in SVG are reported once and left out.
func (r *slideRenderer) effects(sppr []*xmltree.Node, ref *xmltree.Node, cx colorCtx) string {
	var lst *xmltree.Node
	for _, n := range sppr {
		if e := n.Child("effectLst"); e != nil {
			lst = e
			break
		}
		if n.Child("effectDag") != nil {
			r.warnOnce("effect graphs (effectDag) are not supported and left out")
			return ""
		}
	}
	if lst == nil && ref != nil {
		idx := int(ref.Int("idx", 0))
		th := r.slide.Layout.Master.Theme
		if idx >= 1 && idx <= len(th.EffectStyles) {
			lst = th.EffectStyles[idx-1].Child("effectLst")
			ph := cx.colorChild(ref)
			cx.ph = &ph
		}
	}
	for _, n := range sppr {
		if n.Child("scene3d") != nil || n.Child("sp3d") != nil {
			r.warnOnce("3D effects (bevel, extrusion, perspective) are not shown")
			break
		}
	}
	if lst == nil || len(lst.Kids) == 0 {
		return ""
	}
	var prims string
	for _, e := range lst.Kids {
		switch e.Name {
		case "outerShdw":
			c := cx.colorChild(e)
			if !c.OK {
				continue
			}
			dist := float64(e.Int("dist", 0)) / emuPx
			dir := float64(e.Int("dir", 0)) / 60000 * math.Pi / 180
			blur := float64(e.Int("blurRad", 0)) / emuPx
			prims += fmt.Sprintf(`<feDropShadow dx="%s" dy="%s" stdDeviation="%s" flood-color="%s" flood-opacity="%s"/>`,
				f(dist*math.Cos(dir)), f(dist*math.Sin(dir)), f(blur/2), c.Hex(), f(c.A))
		case "glow":
			c := cx.colorChild(e)
			if !c.OK {
				continue
			}
			rad := float64(e.Int("rad", 0)) / emuPx
			prims += fmt.Sprintf(`<feDropShadow dx="0" dy="0" stdDeviation="%s" flood-color="%s" flood-opacity="%s"/>`, f(rad/2), c.Hex(), f(c.A))
		case "innerShdw", "reflection", "softEdge", "prstShdw", "blur":
			r.warnOnce("the %s effect is not shown", e.Name)
		}
	}
	if prims == "" {
		return ""
	}
	id := r.newID("fx")
	fmt.Fprintf(&r.defs, `<filter id="%s" x="-50%%" y="-50%%" width="200%%" height="200%%">%s</filter>`, id, prims)
	return ` filter="url(#` + id + `)"`
}

// lineEnds returns marker attributes for arrowheads (a:headEnd, a:tailEnd).
func (r *slideRenderer) lineEnds(sppr []*xmltree.Node, ref *xmltree.Node, cx colorCtx) string {
	ln := newProps()
	if ref != nil {
		idx := int(ref.Int("idx", 0))
		th := r.slide.Layout.Master.Theme
		if idx >= 1 && idx <= len(th.LineStyles) {
			ln.add(th.LineStyles[idx-1])
			ph := cx.colorChild(ref)
			cx.ph = &ph
		}
	}
	for i := len(sppr) - 1; i >= 0; i-- {
		ln.add(sppr[i].Child("ln"))
	}
	var c Color
	if fill := ln.elem("solidFill"); fill != nil && fill.Name == "solidFill" {
		c = cx.colorChild(fill)
	}
	if !c.OK {
		return ""
	}
	out := ""
	for _, end := range []struct{ el, attr string }{{"headEnd", "marker-start"}, {"tailEnd", "marker-end"}} {
		e := ln.elem(end.el)
		if e == nil || e.Attr("type") == "" || e.Attr("type") == "none" {
			continue
		}
		out += fmt.Sprintf(` %s="url(#%s)"`, end.attr, r.marker(e, c))
	}
	return out
}

func (r *slideRenderer) marker(e *xmltree.Node, c Color) string {
	size := func(v string) float64 {
		switch v {
		case "sm":
			return 2
		case "lg":
			return 5
		}
		return 3
	}
	mw, ml := size(e.Attr("w")), size(e.Attr("len"))
	id := r.newID("m")
	var shape string
	switch e.Attr("type") {
	case "arrow":
		shape = `<path d="M0 0L10 5L0 10" fill="none" stroke="COL" stroke-width="1.5"/>`
	case "stealth":
		shape = `<path d="M0 0L10 5L0 10L3 5Z" fill="COL"/>`
	case "diamond":
		shape = `<path d="M0 5L5 0L10 5L5 10Z" fill="COL"/>`
	case "oval":
		shape = `<ellipse cx="5" cy="5" rx="5" ry="5" fill="COL"/>`
	default: // triangle
		shape = `<path d="M0 0L10 5L0 10Z" fill="COL"/>`
	}
	shape = strings.ReplaceAll(shape, "COL", c.Hex())
	fmt.Fprintf(&r.defs, `<marker id="%s" viewBox="0 0 10 10" refX="5" refY="5" markerUnits="strokeWidth" markerWidth="%s" markerHeight="%s" orient="auto-start-reverse">%s</marker>`,
		id, f(ml), f(mw), shape)
	return id
}
