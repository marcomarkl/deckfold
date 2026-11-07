package render

import (
	"math"
	"strconv"
	"strings"

	"deckfold/internal/pptx/fonts"
	"deckfold/internal/pptx/model"
	"deckfold/internal/pptx/xmltree"
)

// textStyle collects the inherited text properties of one shape, lowest priority first.
type textStyle struct {
	levels  [9][]*xmltree.Node // lvl1pPr .. lvl9pPr sources
	bodyPr  []*xmltree.Node
	fontRef *xmltree.Node // p:style/a:fontRef
}

func (ts *textStyle) addList(lst *xmltree.Node) {
	if lst == nil {
		return
	}
	for i := 0; i < 9; i++ {
		if n := lst.Child("lvl" + strconv.Itoa(i+1) + "pPr"); n != nil {
			ts.levels[i] = append(ts.levels[i], n)
		}
	}
}

// styleFor builds the text style chain of a shape on a slide.
func (r *slideRenderer) styleFor(sp *xmltree.Node, owner ownerKind) *textStyle {
	ts := &textStyle{}
	ts.addList(r.pres.DefaultTextStyle)
	m := r.slide.Layout.Master
	ph := model.Ph(sp)
	var layoutPh, masterPh *xmltree.Node
	switch {
	case ph != nil && owner == ownerSlide:
		layoutPh, masterPh = r.slide.Inherited(ph)
	case ph != nil && owner == ownerLayout:
		masterPh = r.slide.Layout.Inherited(ph)
	}
	if ph != nil {
		typ := model.PhType(ph)
		if layoutPh != nil {
			typ = model.PhType(model.Ph(layoutPh))
		}
		ts.addList(m.TextStyle(typ))
	} else {
		ts.addList(m.Other)
	}
	for _, src := range []*xmltree.Node{masterPh, layoutPh} {
		if src == nil {
			continue
		}
		ts.addList(src.Path("txBody/lstStyle"))
		ts.bodyPr = append(ts.bodyPr, src.Path("txBody/bodyPr"))
	}
	ts.addList(sp.Path("txBody/lstStyle"))
	ts.bodyPr = append(ts.bodyPr, sp.Path("txBody/bodyPr"))
	ts.fontRef = sp.Path("style/fontRef")
	return ts
}

// line output
type textSeg struct {
	x, y      float64 // baseline start, px
	text      string
	face      *fonts.Face
	size      float64 // px
	color     Color
	underline bool
	strike    bool
	spacing   float64 // letter spacing px
	href      string
	para      int // paragraph index in the text body
}

type lineInfo struct {
	segs []textSeg
}

type inRun struct {
	text string
	rp   *props
	br   bool
	href string
}

type inPara struct {
	pp    *props
	lvl   int
	runs  []inRun
	endRP *props
}

const ptPx = 96.0 / 72.0

// layoutText lays out the text body of a shape in the box (px, unrotated) and returns the lines.
func (r *slideRenderer) layoutText(sp *xmltree.Node, ts *textStyle, box rect, cx colorCtx, shapeRels relLookup) []textSeg {
	return r.layoutTextAt(sp, ts, box, cx, shapeRels, 0)
}

// layoutTextAt lays out with a forced font scale when scale > 0. With "shrink text on overflow"
// (a:normAutofit) deckfold uses the scale stored in the file, as PowerPoint does when it opens
// or exports a file: PowerPoint only shrinks the text again while someone edits it. The
// reference images show overflowing text when the file stores no scale.
func (r *slideRenderer) layoutTextAt(sp *xmltree.Node, ts *textStyle, box rect, cx colorCtx, shapeRels relLookup, scale float64) []textSeg {
	body := sp.Child("txBody")
	if body == nil {
		return nil
	}
	bp := newProps()
	for _, b := range ts.bodyPr {
		bp.add(b)
	}
	lIns := float64(bp.int("lIns", 91440)) / emuPx
	tIns := float64(bp.int("tIns", 45720)) / emuPx
	rIns := float64(bp.int("rIns", 91440)) / emuPx
	bIns := float64(bp.int("bIns", 45720)) / emuPx
	inner := rect{box.x + lIns, box.y + tIns, box.w - lIns - rIns, box.h - tIns - bIns}
	wrap := bp.attr("wrap") != "none"
	fontScale, lnReduce := 1.0, 0.0
	if af := bp.elem("normAutofit"); af != nil && af.Name == "normAutofit" {
		if v := af.Int("fontScale", 100000); v > 0 {
			fontScale = float64(v) / 100000
		}
		lnReduce = float64(af.Int("lnSpcReduction", 0)) / 100000
	}
	if scale > 0 {
		fontScale = scale
		lnReduce = math.Min(0.2, (1-scale)*0.5)
	}

	// collect paragraphs with resolved properties
	var paras []inPara
	for _, p := range body.Children("p") {
		ppNode := p.Child("pPr")
		lvl := int(ppNode.Int("lvl", 0))
		if lvl < 0 || lvl > 8 {
			lvl = 0
		}
		pp := newProps()
		rpBase := newProps()
		r.applyFontRef(rpBase, ts.fontRef, cx)
		for _, src := range ts.levels[lvl] {
			pp.add(src)
			rpBase.add(src.Child("defRPr"))
		}
		pp.add(ppNode)
		rpBase.add(ppNode.Child("defRPr"))
		ip := inPara{pp: pp, lvl: lvl}
		for _, k := range p.Kids {
			switch k.Name {
			case "r", "fld":
				rp := cloneProps(rpBase)
				rp.add(k.Child("rPr"))
				txt := k.Child("t").AllText()
				if k.Name == "fld" {
					txt = r.fieldText(k)
				}
				href := ""
				if h := k.Path("rPr/hlinkClick"); h != nil {
					href = shapeRels.href(h)
				}
				ip.runs = append(ip.runs, inRun{text: txt, rp: rp, href: href})
			case "br":
				rp := cloneProps(rpBase)
				rp.add(k.Child("rPr"))
				ip.runs = append(ip.runs, inRun{br: true, rp: rp})
			}
		}
		ip.endRP = cloneProps(rpBase)
		ip.endRP.add(p.Child("endParaRPr"))
		paras = append(paras, ip)
	}
	if len(paras) == 0 {
		return nil
	}

	type pline struct {
		segs      []textSeg
		width     float64
		asc, desc float64
		para      int
	}
	var lines []pline
	var spaceBefore, spaceAfter []float64 // per line
	autoNum := map[int]int{}              // level -> counter
	for pi, ip := range paras {
		pp := ip.pp
		marL := float64(pp.int("marL", 0)) / emuPx
		indent := float64(pp.int("indent", 0)) / emuPx
		algn := pp.attr("algn")
		lnPct, lnPts := 1.0, 0.0
		if ls := pp.elem("lnSpc"); ls != nil {
			if v := ls.Child("spcPct"); v != nil {
				lnPct = float64(v.Int("val", 100000)) / 100000
			} else if v := ls.Child("spcPts"); v != nil {
				lnPts = float64(v.Int("val", 0)) / 100 * ptPx
			}
		}
		lnPct = math.Max(0.1, lnPct-lnReduce)

		// bullet
		bullet, bulletFace, bulletSize, bulletColor := "", (*fonts.Face)(nil), 0.0, Color{}
		firstRP := ip.endRP
		if len(ip.runs) > 0 {
			firstRP = ip.runs[0].rp
		}
		hasText := false
		for _, rn := range ip.runs {
			if rn.text != "" {
				hasText = true
			}
		}
		if bt := pp.elem("buChar"); bt != nil && hasText {
			switch bt.Name {
			case "buChar":
				bullet = bt.Attr("char")
			case "buAutoNum":
				start := int(bt.Int("startAt", 1))
				if _, ok := autoNum[ip.lvl]; !ok {
					autoNum[ip.lvl] = start
				}
				bullet = autoNumber(bt.Attr("type"), autoNum[ip.lvl])
				autoNum[ip.lvl]++
			}
		}
		if bt := pp.elem("buChar"); bt == nil || bt.Name != "buAutoNum" {
			delete(autoNum, ip.lvl)
		}
		for l := range autoNum {
			if l > ip.lvl {
				delete(autoNum, l)
			}
		}
		if bullet != "" {
			size := r.runSize(firstRP, fontScale)
			if s := pp.elem("buSzPct"); s != nil && s.Name == "buSzPct" {
				size *= float64(s.Int("val", 100000)) / 100000
			} else if s != nil && s.Name == "buSzPts" {
				size = float64(s.Int("val", 1800)) / 100 * ptPx
			}
			bulletSize = size
			fam := ""
			if f := pp.elem("buFont"); f != nil && f.Name == "buFont" {
				fam = r.typeface(f.Attr("typeface"))
			}
			if fam == "" {
				fam = r.runFamily(firstRP)
			}
			bulletFace = r.fonts.Face(fam, false, false)
			if !bulletFace.Has([]rune(bullet)[0]) {
				bulletFace = r.fonts.Face("Arial", false, false)
			}
			bulletColor = r.runColor(firstRP, cx)
			if c := pp.elem("buClr"); c != nil && c.Name == "buClr" {
				if cc := cx.colorChild(c); cc.OK {
					bulletColor = cc
				}
			}
		}

		// build atoms: pieces of text that do not break internally
		type atom struct {
			text  string
			rp    *props
			face  *fonts.Face
			size  float64
			width float64
			space bool
			br    bool
			href  string
		}
		var atoms []atom
		for _, rn := range ip.runs {
			if rn.br {
				atoms = append(atoms, atom{br: true, rp: rn.rp, size: r.runSize(rn.rp, fontScale)})
				continue
			}
			text := applyCaps(rn.text, rn.rp.attr("cap"))
			size := r.runSize(rn.rp, fontScale)
			for _, piece := range splitBreakable(text) {
				for _, part := range r.splitByFace(piece, rn.rp) {
					w := measure(part.face, part.text, size) + float64(len([]rune(part.text)))*spacingPx(rn.rp)
					atoms = append(atoms, atom{text: part.text, rp: rn.rp, face: part.face, size: size, width: w, space: strings.TrimSpace(part.text) == "", href: rn.href})
				}
			}
		}

		avail := inner.w
		startLine := len(lines)
		cur := pline{para: pi}
		firstLine := true
		lineStartX := func(first bool) float64 {
			if first && !(bullet != "" && indent < 0) {
				return marL + indent
			}
			return marL
		}
		x := lineStartX(true)
		if bullet != "" {
			bw := measure(bulletFace, bullet, bulletSize)
			bx := marL + indent
			if indent >= 0 {
				x = bx + bw + bulletSize*0.25
			} else if bx+bw > x {
				x = bx + bw + bulletSize*0.25
			}
			bulletFace.Use(bullet)
			cur.segs = append(cur.segs, textSeg{x: bx, text: bullet, face: bulletFace, size: bulletSize, color: bulletColor})
			cur.asc = math.Max(cur.asc, bulletFace.Ascent()*bulletSize)
			cur.desc = math.Max(cur.desc, bulletFace.Descent()*bulletSize)
		}
		flush := func() {
			// width without trailing spaces
			end := 0.0
			for _, s := range cur.segs {
				if strings.TrimSpace(s.text) != "" {
					end = math.Max(end, s.x+measure(s.face, s.text, s.size))
				}
			}
			cur.width = end
			if cur.asc == 0 {
				size := r.runSize(ip.endRP, fontScale)
				f := r.fonts.Face(r.runFamily(ip.endRP), r.runBold(ip.endRP), r.runItalic(ip.endRP))
				cur.asc, cur.desc = f.Ascent()*size, f.Descent()*size
			}
			lines = append(lines, cur)
			cur = pline{para: pi}
			firstLine = false
			x = lineStartX(false)
		}
		for _, a := range atoms {
			if a.br {
				if cur.asc == 0 {
					f := r.fonts.Face(r.runFamily(a.rp), false, false)
					cur.asc, cur.desc = f.Ascent()*a.size, f.Descent()*a.size
				}
				flush()
				continue
			}
			if wrap && !a.space && x+a.width > avail+0.5 && len(cur.segs) > 0 && hasNonSpace(cur.segs) {
				flush()
			}
			if a.space && len(cur.segs) == 0 && !firstLine {
				continue // spaces at the start of a wrapped line
			}
			// a single atom wider than the line: break it by characters
			if wrap && !a.space && a.width > avail-x+0.5 && x+a.width > avail {
				rs := []rune(a.text)
				for len(rs) > 0 {
					n := fitRunes(a.face, rs, a.size, avail-x)
					if n == 0 {
						if len(cur.segs) > 0 {
							flush()
							continue
						}
						n = 1
					}
					t := string(rs[:n])
					r.addSeg(&cur.segs, &cur.asc, &cur.desc, x, t, a.face, a.size, a.rp, cx, a.href)
					x += measure(a.face, t, a.size)
					rs = rs[n:]
					if len(rs) > 0 {
						flush()
					}
				}
				continue
			}
			r.addSeg(&cur.segs, &cur.asc, &cur.desc, x, a.text, a.face, a.size, a.rp, cx, a.href)
			x += a.width
		}
		flush()

		// alignment and line spacing for the lines of this paragraph
		for li := startLine; li < len(lines); li++ {
			l := &lines[li]
			shift := 0.0
			switch algn {
			case "ctr":
				shift = (avail - l.width) / 2
			case "r":
				shift = avail - l.width
			}
			if shift != 0 {
				for i := range l.segs {
					l.segs[i].x += shift
				}
			}
			// PowerPoint's single line spacing is 1.2 times the largest font size of the line,
			// whatever the font; the ascent only places the baseline (measured against the
			// PowerPoint reference images, docs/spikes/S4-powerpoint-reference.md)
			size := 0.0
			for _, sg := range l.segs {
				size = math.Max(size, sg.size)
			}
			h := (l.asc + l.desc) * lnPct
			if size > 0 {
				h = 1.2 * size * lnPct
			}
			a := l.asc * lnPct
			if lnPts > 0 {
				h = lnPts
				a = lnPts * l.asc / (l.asc + l.desc)
			}
			l.asc, l.desc = a, h-a
		}
		natural := 0.0
		if len(lines) > startLine {
			natural = lines[startLine].asc + lines[startLine].desc
		}
		bef, aft := 0.0, 0.0
		if s := pp.elem("spcBef"); s != nil {
			bef = spacing(s, natural, r.runSize(firstRP, fontScale))
		}
		if s := pp.elem("spcAft"); s != nil {
			aft = spacing(s, natural, r.runSize(firstRP, fontScale))
		}
		for li := startLine; li < len(lines); li++ {
			sb, sa := 0.0, 0.0
			if li == startLine && pi > 0 {
				sb = bef // PowerPoint ignores the space before the first paragraph
			}
			if li == len(lines)-1 && pi+1 < len(paras) {
				sa = aft
			}
			spaceBefore = append(spaceBefore, sb)
			spaceAfter = append(spaceAfter, sa)
		}
	}

	// total height and vertical anchor
	total := 0.0
	for i, l := range lines {
		total += spaceBefore[i] + l.asc + l.desc + spaceAfter[i]
	}
	top := inner.y
	switch bp.attr("anchor") {
	case "ctr":
		top = inner.y + (inner.h-total)/2
	case "b":
		top = inner.y + inner.h - total
	}
	var out []textSeg
	y := top
	for i, l := range lines {
		y += spaceBefore[i]
		base := y + l.asc
		for _, s := range l.segs {
			s.x += inner.x
			s.y = base
			s.para = l.para
			out = append(out, s)
		}
		y += l.asc + l.desc + spaceAfter[i]
	}
	return out
}
