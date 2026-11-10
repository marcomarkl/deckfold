package render

import (
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"deckfold/internal/pptx/fonts"
	"deckfold/internal/pptx/xmltree"
)

func cloneProps(p *props) *props {
	c := newProps()
	for k, v := range p.attrs {
		c.attrs[k] = v
	}
	for k, v := range p.elems {
		c.elems[k] = v
	}
	return c
}

// applyFontRef sets the defaults a shape style gives its text (font and color).
func (r *slideRenderer) applyFontRef(rp *props, ref *xmltree.Node, cx colorCtx) {
	if ref == nil {
		return
	}
	switch ref.Attr("idx") {
	case "major":
		rp.attrs["df-latin"] = "+mj-lt"
	case "minor":
		rp.attrs["df-latin"] = "+mn-lt"
	}
	for _, k := range ref.Kids {
		if c := cx.colorOf(k); c.OK {
			rp.elems["fill"] = &xmltree.Node{Name: "solidFill", Kids: []*xmltree.Node{k}}
			break
		}
	}
}

// typeface resolves theme font references (+mj-lt, +mn-lt, ...).
func (r *slideRenderer) typeface(tf string) string {
	th := r.slide.Layout.Master.Theme
	switch tf {
	case "+mj-lt":
		return th.MajorLatin
	case "+mn-lt":
		return th.MinorLatin
	case "+mj-ea":
		return th.MajorEA
	case "+mn-ea":
		return th.MinorEA
	case "+mj-cs":
		return th.MajorCS
	case "+mn-cs":
		return th.MinorCS
	}
	return tf
}

func (r *slideRenderer) runFamily(rp *props) string {
	tf := ""
	if l := rp.elem("latin"); l != nil {
		tf = l.Attr("typeface")
	}
	if tf == "" {
		tf = rp.attr("df-latin")
	}
	tf = r.typeface(tf)
	if tf == "" {
		tf = r.slide.Layout.Master.Theme.MinorLatin
	}
	if tf == "" {
		tf = "Calibri"
	}
	return tf
}

func (r *slideRenderer) runEA(rp *props) string {
	if l := rp.elem("ea"); l != nil {
		return r.typeface(l.Attr("typeface"))
	}
	return ""
}

func (r *slideRenderer) runCS(rp *props) string {
	if l := rp.elem("cs"); l != nil {
		return r.typeface(l.Attr("typeface"))
	}
	return ""
}

func (r *slideRenderer) runBold(rp *props) bool   { return rp.bool("b", false) }
func (r *slideRenderer) runItalic(rp *props) bool { return rp.bool("i", false) }

// runSize returns the font size in px.
func (r *slideRenderer) runSize(rp *props, scale float64) float64 {
	sz := float64(rp.int("sz", 1800)) / 100
	if sz <= 0 {
		sz = 18
	}
	size := sz * ptPx * scale
	if b := rp.int("baseline", 0); b != 0 {
		size *= 0.67
	}
	return size
}

func (r *slideRenderer) runColor(rp *props, cx colorCtx) Color {
	if f := rp.elem("solidFill"); f != nil && f.Name == "solidFill" {
		if c := cx.colorChild(f); c.OK {
			return c
		}
	}
	if f := rp.elem("gradFill"); f != nil && f.Name == "gradFill" {
		// text with a gradient: the first stop as a solid color
		if gs := f.Path("gsLst/gs"); gs != nil {
			if c := cx.colorChild(gs); c.OK {
				return c
			}
		}
	}
	if f := rp.elem("noFill"); f != nil && f.Name == "noFill" {
		return Color{A: 0, OK: true}
	}
	c := cx.colorOf(&xmltree.Node{Name: "schemeClr", Attrs: attrs("val", "tx1")})
	if !c.OK {
		c = hexColor("000000")
	}
	return c
}

func spacingPx(rp *props) float64 {
	return float64(rp.int("spc", 0)) / 100 * ptPx
}

// kerned reports whether PowerPoint kerns a run drawn in face at size px. The run attribute
// kern is the smallest font size from which on it does, in hundredths of a point; without the
// attribute, and with kern="0", it does not. PowerPoint compares the size it draws, so a size
// that a stored font scale has shrunk below the value is not kerned. The pairs come from the
// kern table of the font, never from GPOS (fonts.Face.Kern). Measured against PowerPoint with
// the kerning deck: docs/spikes/S4-powerpoint-reference.md, "How PowerPoint kerns".
func kerned(rp *props, face *fonts.Face, size float64) bool {
	k := rp.int("kern", 0)
	return k > 0 && face.Kerns() && size/ptPx >= float64(k)/100-1e-6
}

type facePart struct {
	text string
	face *fonts.Face
}

// splitByFace splits text into parts by the face that has the glyphs: latin font, then the
// east asian and complex script fonts of the run, then a bundled sans face.
func (r *slideRenderer) splitByFace(text string, rp *props) []facePart {
	bold, italic := r.runBold(rp), r.runItalic(rp)
	primary := r.fonts.Face(r.runFamily(rp), bold, italic)
	var cands []*fonts.Face
	pick := func(c rune) *fonts.Face {
		if unicode.IsSpace(c) || primary.Has(c) {
			return primary
		}
		if cands == nil {
			for _, fam := range []string{r.runEA(rp), r.runCS(rp), "Arial"} {
				if fam != "" {
					cands = append(cands, r.fonts.Face(fam, bold, italic))
				}
			}
		}
		for _, f := range cands {
			if f.Has(c) {
				return f
			}
		}
		return primary
	}
	var out []facePart
	for _, c := range text {
		f := pick(c)
		if n := len(out); n > 0 && out[n-1].face == f {
			out[n-1].text += string(c)
		} else {
			out = append(out, facePart{string(c), f})
		}
	}
	return out
}

func (r *slideRenderer) addSeg(segs *[]textSeg, asc, desc *float64, x float64, text string, face *fonts.Face, size float64, kern bool, rp *props, cx colorCtx, href string) {
	if text == "" {
		return
	}
	face.Use(text)
	u := rp.attr("u")
	seg := textSeg{
		x: x, text: text, face: face, size: size, color: r.runColor(rp, cx),
		underline: u != "" && u != "none", strike: rp.attr("strike") != "" && rp.attr("strike") != "noStrike",
		spacing: spacingPx(rp), href: href, kern: kern,
	}
	// continue the previous piece when it ends exactly here and looks the same
	if n := len(*segs); n > 0 {
		last := &(*segs)[n-1]
		end := last.x + width(last.face, last.text, last.size, last.kern) + float64(len([]rune(last.text)))*last.spacing + pairPx(last, text, face, size, kern)
		if last.face == seg.face && last.size == seg.size && last.color == seg.color && last.underline == seg.underline &&
			last.strike == seg.strike && last.spacing == seg.spacing && last.href == seg.href && last.kern == seg.kern && math.Abs(end-x) < 0.01 {
			last.text += text
			*asc = math.Max(*asc, face.Ascent()*size)
			*desc = math.Max(*desc, face.Descent()*size)
			return
		}
	}
	*segs = append(*segs, seg)
	*asc = math.Max(*asc, face.Ascent()*size)
	*desc = math.Max(*desc, face.Descent()*size)
}

func measure(f *fonts.Face, text string, size float64) float64 {
	if fonts.Complex(text) {
		if w, ok := f.ShapedWidth(text); ok {
			return w * size
		}
	}
	w := 0.0
	for _, c := range text {
		w += f.Advance(c)
	}
	return w * size
}

// width returns the width of text in px: measure, plus the kerning between its characters when
// the text is kerned. Shaped text has its kerning from the shaper already.
func width(f *fonts.Face, text string, size float64, kern bool) float64 {
	w := measure(f, text, size)
	if !kern || fonts.Complex(text) {
		return w
	}
	prev := rune(-1)
	for _, c := range text {
		if prev >= 0 {
			w += f.Kern(prev, c) * size
		}
		prev = c
	}
	return w
}

// pairPx returns the kerning in px between the end of a piece of text on a line and the text
// that follows it. PowerPoint kerns across the boundary of two runs and with a space like
// inside a word; a pair exists only within one font at one size, and when both sides are
// kerned.
func pairPx(before *textSeg, after string, face *fonts.Face, size float64, kern bool) float64 {
	if before == nil || !before.kern || !kern || before.face != face || before.size != size || before.text == "" || after == "" {
		return 0
	}
	last, _ := utf8.DecodeLastRuneInString(before.text)
	first, _ := utf8.DecodeRuneInString(after)
	return face.Kern(last, first) * size
}

func fitRunes(f *fonts.Face, rs []rune, size, avail float64, kern bool) int {
	w := 0.0
	for i, c := range rs {
		w += f.Advance(c) * size
		if kern && i > 0 {
			w += f.Kern(rs[i-1], c) * size
		}
		if w > avail+0.5 {
			return i
		}
	}
	return len(rs)
}

func hasNonSpace(segs []textSeg) bool {
	for _, s := range segs {
		if strings.TrimSpace(s.text) != "" {
			return true
		}
	}
	return false
}

// splitBreakable splits text at break opportunities: runs of spaces are their own pieces, and
// a hyphen or slash ends a piece.
func splitBreakable(s string) []string {
	var out []string
	var b strings.Builder
	inSpace := false
	flush := func() {
		if b.Len() > 0 {
			out = append(out, b.String())
			b.Reset()
		}
	}
	for _, c := range s {
		sp := c == ' ' || c == '\t'
		if sp != inSpace {
			flush()
			inSpace = sp
		}
		b.WriteRune(c)
		if !sp && (c == '-' || c == '/' || c == '–' || c == '—' || isCJK(c)) {
			flush()
		}
	}
	flush()
	return out
}

func isCJK(c rune) bool {
	return unicode.Is(unicode.Han, c) || unicode.Is(unicode.Hiragana, c) || unicode.Is(unicode.Katakana, c) || unicode.Is(unicode.Hangul, c)
}

func applyCaps(s, cap string) string {
	if cap == "all" || cap == "small" {
		return strings.ToUpper(s)
	}
	return s
}

// spacing returns spcBef/spcAft in px: spcPts in hundredths of a point, spcPct in thousandths of
// a percent of the line.
func spacing(n *xmltree.Node, natural, size float64) float64 {
	if v := n.Child("spcPts"); v != nil {
		return float64(v.Int("val", 0)) / 100 * ptPx
	}
	if v := n.Child("spcPct"); v != nil {
		base := natural
		if base == 0 {
			base = size * 1.2
		}
		return float64(v.Int("val", 0)) / 100000 * base
	}
	return 0
}

// autoNumber formats an automatic bullet number.
func autoNumber(typ string, n int) string {
	num := strconv.Itoa(n)
	switch {
	case strings.HasPrefix(typ, "alphaLc"):
		num = alpha(n, false)
	case strings.HasPrefix(typ, "alphaUc"):
		num = alpha(n, true)
	case strings.HasPrefix(typ, "romanLc"):
		num = strings.ToLower(roman(n))
	case strings.HasPrefix(typ, "romanUc"):
		num = roman(n)
	}
	switch {
	case strings.HasSuffix(typ, "ParenBoth"):
		return "(" + num + ")"
	case strings.HasSuffix(typ, "ParenR"):
		return num + ")"
	case strings.HasSuffix(typ, "Plain"):
		return num
	case strings.HasSuffix(typ, "Minus"):
		return "- " + num + " -"
	}
	return num + "."
}

func alpha(n int, upper bool) string {
	if n < 1 {
		n = 1
	}
	base := 'a'
	if upper {
		base = 'A'
	}
	c := string(rune(base) + rune((n-1)%26))
	return strings.Repeat(c, (n-1)/26+1)
}

func roman(n int) string {
	if n <= 0 || n >= 4000 {
		return strconv.Itoa(n)
	}
	vals := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	syms := []string{"M", "CM", "D", "CD", "C", "XC", "L", "XL", "X", "IX", "V", "IV", "I"}
	var b strings.Builder
	for i, v := range vals {
		for n >= v {
			b.WriteString(syms[i])
			n -= v
		}
	}
	return b.String()
}

// fieldText returns the text of a field: slide numbers are computed, everything else keeps the
// stored text.
func (r *slideRenderer) fieldText(fld *xmltree.Node) string {
	if fld.Attr("type") == "slidenum" && r.slideNum > 0 {
		return strconv.Itoa(r.slideNum)
	}
	return fld.Child("t").AllText()
}

func attrs(kv ...string) []xmlAttr {
	var out []xmlAttr
	for i := 0; i+1 < len(kv); i += 2 {
		out = append(out, xmlAttr{Name: xmlName{Local: kv[i]}, Value: kv[i+1]})
	}
	return out
}
