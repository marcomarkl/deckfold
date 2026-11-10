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
// picBulletScale is the height of a picture bullet relative to the bullet size.
const picBulletScale = 0.65

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
	para      int     // paragraph index in the text body
	image     string  // picture bullet: asset name (text is empty)
	imgW      float64 // picture bullet width, px (height is size)
	bullet    bool    // the bullet of the line (character or picture)
	wordSpace float64 // extra space per space character (justified lines), px
	rtl       bool    // in a right-to-left paragraph
	kern      bool    // PowerPoint kerns this text (see kerned)
	unit      int     // word or letter number in the paragraph (builds by word or letter)
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
		below     float64 // what the font needs below the baseline; the text ends that far below its last line
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
		bulletImg, bulletAspect := "", 1.0
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
			case "buBlip":
				if name, ok := r.blipAsset(bt.Child("blip"), r.partOfNode(bt)); ok {
					bulletImg = name
					if sz, ok := r.sizes[name]; ok && sz[1] > 0 {
						bulletAspect = sz[0] / sz[1]
					}
					bullet = " " // takes the place of a character bullet in the layout below
				}
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
			kern  bool
		}
		var atoms []atom
		for _, rn := range ip.runs {
			if rn.br {
				atoms = append(atoms, atom{br: true, rp: rn.rp, size: r.runSize(rn.rp, fontScale)})
				continue
			}
			text := applyCaps(rn.text, rn.rp.attr("cap"))
			size := r.runSize(rn.rp, fontScale)
			// a run with shaped or right-to-left text is laid out by the shaper and by the browser,
			// as a whole: its spaces must not become kerned pieces of their own
			plainRun := !fonts.Complex(text) && !fonts.RTL(text)
			for _, piece := range splitBreakable(text) {
				for _, part := range r.splitByFace(piece, rn.rp) {
					kern := plainRun && kerned(rn.rp, part.face, size)
					w := width(part.face, part.text, size, kern) + float64(len([]rune(part.text)))*spacingPx(rn.rp)
					atoms = append(atoms, atom{text: part.text, rp: rn.rp, face: part.face, size: size, width: w, space: strings.TrimSpace(part.text) == "", href: rn.href, kern: kern})
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
			if bulletImg != "" {
				// PowerPoint draws a picture bullet 0.65 times the bullet size high, keeps its
				// aspect ratio, and starts the text right after it when it is wider than the
				// hanging indent (measured against the PowerPoint reference of the bullets deck)
				bw = picBulletScale * bulletSize * bulletAspect
				x = math.Max(lineStartX(true), bx+bw)
				if indent >= 0 {
					x = bx + bw
				}
				cur.segs = append(cur.segs, textSeg{x: bx, face: bulletFace, size: bulletSize, image: bulletImg, imgW: bw, bullet: true})
			} else {
				bulletFace.Use(bullet)
				cur.segs = append(cur.segs, textSeg{x: bx, text: bullet, face: bulletFace, size: bulletSize, color: bulletColor, bullet: true})
				cur.asc = math.Max(cur.asc, bulletFace.Ascent()*bulletSize)
				cur.desc = math.Max(cur.desc, bulletFace.Descent()*bulletSize)
			}
		}
		flush := func() {
			// width without trailing spaces
			end := 0.0
			for _, s := range cur.segs {
				if s.image != "" {
					end = math.Max(end, s.x+s.imgW)
				} else if strings.TrimSpace(s.text) != "" {
					end = math.Max(end, s.x+width(s.face, s.text, s.size, s.kern))
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
		// lastText returns the text piece the next atom follows on the line, for the kerning
		// between the two (nil at the start of a line and after a bullet)
		lastText := func() *textSeg {
			if n := len(cur.segs); n > 0 && !cur.segs[n-1].bullet && cur.segs[n-1].image == "" {
				return &cur.segs[n-1]
			}
			return nil
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
			if wrap && !a.space && x+pairPx(lastText(), a.text, a.face, a.size, a.kern)+a.width > avail+0.5 && len(cur.segs) > 0 && hasNonSpace(cur.segs) {
				flush()
			}
			if a.space && len(cur.segs) == 0 && !firstLine {
				continue // spaces at the start of a wrapped line
			}
			// the pair with what stands before it on the line moves the atom
			x += pairPx(lastText(), a.text, a.face, a.size, a.kern)
			// a single atom wider than the line: break it by characters
			if wrap && !a.space && a.width > avail-x+0.5 && x+a.width > avail {
				rs := []rune(a.text)
				for len(rs) > 0 {
					n := fitRunes(a.face, rs, a.size, avail-x, a.kern)
					if n == 0 {
						if len(cur.segs) > 0 {
							flush()
							continue
						}
						n = 1
					}
					t := string(rs[:n])
					r.addSeg(&cur.segs, &cur.asc, &cur.desc, x, t, a.face, a.size, a.kern, a.rp, cx, a.href)
					x += width(a.face, t, a.size, a.kern)
					rs = rs[n:]
					if len(rs) > 0 {
						flush()
					}
				}
				continue
			}
			r.addSeg(&cur.segs, &cur.asc, &cur.desc, x, a.text, a.face, a.size, a.kern, a.rp, cx, a.href)
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
			if pp.attr("rtl") == "1" {
				// a right-to-left paragraph: the pieces of the line run from right to left
				mirrorLine(l.segs, l.width)
			}
			switch algn {
			case "just", "justLow", "dist", "thaiDist":
				// justified: the space left on the line goes to the spaces between words; the
				// last line of a paragraph stays left aligned, except with "dist"
				if algn == "dist" || algn == "thaiDist" || li < len(lines)-1 {
					justify(l.segs, avail-l.width)
				}
			}
			if shift != 0 {
				for i := range l.segs {
					l.segs[i].x += shift
				}
			}
			// Where PowerPoint puts the baseline (measured against its PDF for 17 fonts, eight
			// line spacings in percent, four in points, a stored reduction and all three anchors:
			// docs/spikes/S4-powerpoint-reference.md, "Where PowerPoint puts the baseline").
			// A single line is 1.2 times the largest font size of the line, whatever the font.
			// The font divides it by its ascender and descender; the part below the baseline
			// stays what it is under every line spacing. The line spacing sets the distance
			// from baseline to baseline. The baseline sits at least three quarters of that
			// distance below the top of its line, and never has more room above it than a
			// single line gives, unless the three quarters ask for it.
			size, hasc, hdesc := 0.0, 0.0, 0.0
			for _, sg := range l.segs {
				if sg.image == "" { // a picture bullet does not make the line higher
					size = math.Max(size, sg.size)
					hasc = math.Max(hasc, sg.face.LineAscent()*sg.size)
					hdesc = math.Max(hdesc, sg.face.LineDescent()*sg.size)
				}
			}
			if hasc+hdesc == 0 {
				hasc, hdesc = l.asc, l.desc // a line without text: the font of the paragraph end
			}
			single := l.asc + l.desc
			if size > 0 {
				single = 1.2 * size
			}
			below := single * hdesc / (hasc + hdesc)
			h := single * lnPct
			if lnPts > 0 {
				h = lnPts
			}
			a := math.Max(0.75*h, math.Min(h, single)-below)
			l.asc, l.desc, l.below = a, h-a, below
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

	// the text ends below its last baseline by what the font needs there, not by the rest of the
	// line spacing: that is what PowerPoint centers and bottom-aligns
	if n := len(lines); n > 0 && lines[n-1].below > 0 {
		lines[n-1].desc = lines[n-1].below
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
		if baselinesOnPoints {
			base = top + math.Round((base-top)/ptPx)*ptPx
		}
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

// baselinesOnPoints: PowerPoint's PDF has every baseline a whole number of points below the top
// of the text (27, 61 and 94 pt for three lines of 28 pt Liberation Sans, where the line
// spacing gives 27.2, 60.8 and 94.4 pt).
const baselinesOnPoints = true

// justify spreads extra px over the spaces between the words of a line (not over leading or
// trailing spaces, and not after the bullet) and moves the segments after each space.
func justify(segs []textSeg, extra float64) {
	if extra <= 0.01 {
		return
	}
	// the spaces that count lie between the first and the last non-space character
	first, last := -1, -1
	var all []rune
	var owner []int
	for i, sg := range segs {
		if sg.bullet {
			continue
		}
		for _, c := range sg.text {
			all = append(all, c)
			owner = append(owner, i)
		}
	}
	for k, c := range all {
		if c != ' ' && c != '\u00a0' {
			if first < 0 {
				first = k
			}
			last = k
		}
	}
	if first < 0 {
		return
	}
	n := 0
	for k := first; k <= last; k++ {
		if all[k] == ' ' {
			n++
		}
	}
	if n == 0 {
		return
	}
	ws := extra / float64(n)
	before := make([]int, len(segs)) // counted spaces before each segment
	cum := 0
	for k := range all {
		i := owner[k]
		if k == 0 || owner[k-1] != i {
			before[i] = cum
		}
		if k > first && k < last && all[k] == ' ' {
			cum++
		}
	}
	for i := range segs {
		if segs[i].bullet {
			continue
		}
		segs[i].x += float64(before[i]) * ws
		segs[i].wordSpace = ws
	}
}

// mirrorLine reverses the order of the pieces of a line within the span they cover, for
// right-to-left paragraphs. The text inside a piece keeps its order; the browser lays it out
// with the bidirectional algorithm (writeText marks right-to-left pieces).
func mirrorLine(segs []textSeg, end float64) {
	if len(segs) == 0 {
		return
	}
	start := segs[0].x
	for _, sg := range segs {
		start = math.Min(start, sg.x)
	}
	for i := range segs {
		w := segs[i].imgW
		if segs[i].image == "" {
			w = width(segs[i].face, segs[i].text, segs[i].size, segs[i].kern)
		}
		segs[i].x = start + end - (segs[i].x + w)
		segs[i].rtl = true
	}
}
