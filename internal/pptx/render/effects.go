package render

import (
	"fmt"
	"math"
	"strings"

	"deckfold/internal/pptx/xmltree"
)

// effects returns a filter attribute for the shape effects and the reflection, which the caller
// draws as a mirrored copy (see reflection). td is the 3D description (may be nil); its bevel
// approximation comes first. Then come the effects on the shape itself (soft edge, inner
// shadow, blur), then glow and shadows behind it in the order of the file. An effect graph
// (a:effectDag) is approximated by the effects it contains.
func (r *slideRenderer) effects(sppr []*xmltree.Node, ref *xmltree.Node, cx colorCtx, td *threeD) (string, *xmltree.Node) {
	var kids []*xmltree.Node
	found := false
	for _, n := range sppr {
		if e := n.Child("effectLst"); e != nil {
			kids, found = e.Kids, true
			break
		}
		if dag := n.Child("effectDag"); dag != nil {
			kids, found = dagEffects(dag), true
			r.warnOnce("effect graphs (effectDag) are approximated by the shadows, glows and edges they contain")
			break
		}
	}
	if !found && ref != nil {
		idx := int(ref.Int("idx", 0))
		th := r.slide.Layout.Master.Theme
		if idx >= 1 && idx <= len(th.EffectStyles) {
			kids = th.EffectStyles[idx-1].Child("effectLst").Elems()
			ph := cx.colorChild(ref)
			cx.ph = &ph
		}
	}
	var prims string
	if td != nil {
		prims = td.bevelFilter()
	}
	n := 0
	// name gives the result of the primitives so far a name, so that a block can use it twice
	name := func() string {
		n++
		id := fmt.Sprintf("fx%d", n)
		prims += `<feOffset dx="0" dy="0" result="` + id + `"/>`
		return id
	}
	var refl *xmltree.Node
	for _, e := range kids {
		switch e.Name {
		case "softEdge":
			rad := float64(e.Int("rad", 0)) / emuPx
			if rad <= 0 {
				continue
			}
			in := name()
			// the alpha falls off over rad inside the outline
			prims += fmt.Sprintf(`<feGaussianBlur in="SourceAlpha" stdDeviation="%s"/><feComponentTransfer result="%sm"><feFuncA type="linear" slope="2" intercept="-1"/></feComponentTransfer><feComposite in="%s" in2="%sm" operator="in"/>`,
				f(rad/2), in, in, in)
		case "innerShdw":
			c := cx.colorChild(e)
			if !c.OK {
				continue
			}
			dist := float64(e.Int("dist", 0)) / emuPx
			dir := float64(e.Int("dir", 0)) / 60000 * math.Pi / 180
			blur := float64(e.Int("blurRad", 0)) / emuPx
			in := name()
			prims += fmt.Sprintf(`<feComponentTransfer in="SourceAlpha"><feFuncA type="table" tableValues="1 0"/></feComponentTransfer><feGaussianBlur stdDeviation="%s"/><feOffset dx="%s" dy="%s" result="%sa"/>`,
				f(blur/2), f(dist*math.Cos(dir)), f(dist*math.Sin(dir)), in)
			prims += fmt.Sprintf(`<feFlood flood-color="%s" flood-opacity="%s"/><feComposite in2="%sa" operator="in"/><feComposite in2="SourceAlpha" operator="in" result="%ss"/>`,
				c.Hex(), f(c.A), in, in)
			prims += fmt.Sprintf(`<feMerge><feMergeNode in="%s"/><feMergeNode in="%ss"/></feMerge>`, in, in)
		case "blur":
			if rad := float64(e.Int("rad", 0)) / emuPx; rad > 0 {
				prims += fmt.Sprintf(`<feGaussianBlur stdDeviation="%s"/>`, f(rad/2))
			}
		case "reflection":
			refl = e
		}
	}
	for _, e := range kids {
		switch e.Name {
		case "outerShdw", "prstShdw":
			c := cx.colorChild(e)
			if !c.OK {
				continue
			}
			dist := float64(e.Int("dist", 0)) / emuPx
			dir := float64(e.Int("dir", 0)) / 60000 * math.Pi / 180
			blur := float64(e.Int("blurRad", 0)) / emuPx
			if e.Name == "prstShdw" {
				// preset shadows (shdw1..20) are hard shadows; drawn as an outer shadow without blur
				blur = 0
			}
			prims += fmt.Sprintf(`<feDropShadow dx="%s" dy="%s" stdDeviation="%s" flood-color="%s" flood-opacity="%s"/>`,
				f(dist*math.Cos(dir)), f(dist*math.Sin(dir)), f(blur/2), c.Hex(), f(c.A))
		case "glow":
			c := cx.colorChild(e)
			if !c.OK {
				continue
			}
			rad := float64(e.Int("rad", 0)) / emuPx
			prims += fmt.Sprintf(`<feDropShadow dx="0" dy="0" stdDeviation="%s" flood-color="%s" flood-opacity="%s"/>`, f(rad/2), c.Hex(), f(c.A))
		case "fillOverlay":
			r.warnOnce("the fill overlay effect is not shown")
		}
	}
	if prims == "" {
		return "", refl
	}
	id := r.newID("fx")
	fmt.Fprintf(&r.defs, `<filter id="%s" x="-50%%" y="-50%%" width="200%%" height="200%%">%s</filter>`, id, prims)
	return ` filter="url(#` + id + `)"`, refl
}

// dagEffects returns the effects inside an effect graph, in document order.
func dagEffects(n *xmltree.Node) []*xmltree.Node {
	var out []*xmltree.Node
	for _, k := range n.Elems() {
		switch k.Name {
		case "outerShdw", "prstShdw", "glow", "innerShdw", "softEdge", "blur", "reflection":
			out = append(out, k)
		default:
			out = append(out, dagEffects(k)...)
		}
	}
	return out
}

// reflection draws a mirrored, fading copy of markup (the drawing of a shape or picture inside
// box) below the box, as a:reflection describes: start and end opacity at start and end
// positions (fractions of the height), distance, vertical scale and blur.
func (r *slideRenderer) reflection(markup string, box rect, e *xmltree.Node) string {
	if e == nil || markup == "" {
		return ""
	}
	stA, endA := float64(e.Int("stA", 100000))/100000, float64(e.Int("endA", 0))/100000
	stPos, endPos := float64(e.Int("stPos", 0))/100000, float64(e.Int("endPos", 100000))/100000
	dist := float64(e.Int("dist", 0)) / emuPx
	sy := math.Abs(float64(e.Int("sy", -100000)) / 100000)
	if sy == 0 {
		return ""
	}
	bottom := box.y + box.h
	// in the mirrored copy the reflection starts at the bottom of the box (position 0)
	gid, mid := r.newID("rg"), r.newID("rm")
	fmt.Fprintf(&r.defs, `<linearGradient id="%s" gradientUnits="userSpaceOnUse" x1="0" y1="%s" x2="0" y2="%s"><stop offset="%s" stop-color="#fff" stop-opacity="%s"/><stop offset="%s" stop-color="#fff" stop-opacity="%s"/></linearGradient>`,
		gid, f(bottom), f(box.y), f(stPos), f(stA), f(endPos), f(endA))
	fmt.Fprintf(&r.defs, `<mask id="%s" maskUnits="userSpaceOnUse" x="%s" y="%s" width="%s" height="%s"><rect x="%s" y="%s" width="%s" height="%s" fill="url(#%s)"/></mask>`,
		mid, f(box.x-box.w), f(box.y-box.h), f(3*box.w), f(3*box.h), f(box.x-box.w), f(box.y-box.h), f(3*box.w), f(3*box.h), gid)
	blur := ""
	if rad := float64(e.Int("blurRad", 0)) / emuPx; rad > 0 {
		fid := r.newID("rf")
		fmt.Fprintf(&r.defs, `<filter id="%s" x="-20%%" y="-20%%" width="140%%" height="140%%"><feGaussianBlur stdDeviation="%s"/></filter>`, fid, f(rad/2))
		blur = ` filter="url(#` + fid + `)"`
	}
	return fmt.Sprintf(`<g transform="matrix(1 0 0 %s 0 %s)"%s><g mask="url(#%s)">%s</g></g>`,
		f(-sy), f(bottom+dist+bottom*sy), blur, mid, markup)
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
