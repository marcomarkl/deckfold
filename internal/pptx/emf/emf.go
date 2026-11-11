// Package emf converts Enhanced Metafiles (EMF) to SVG. Browsers cannot show EMF, and Office
// stores logos, icons and pictures of OLE objects in it. The converter covers the records that
// such files use: paths with lines and Bézier curves, polygons, rectangles and ellipses, pens and
// brushes, clip paths, transforms, text and embedded bitmaps. Unknown records are counted; when
// too much of a file is unknown, Convert reports it so that the caller can show a placeholder.
package emf

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Result of a conversion.
type Result struct {
	SVG     []byte
	Unknown int // records that were skipped
	Records int
}

// ErrNotEMF marks data that is no EMF file.
var ErrNotEMF = errors.New("not an EMF file")

type xform struct{ a, b, c, d, e, f float64 } // x' = a*x + c*y + e, y' = b*x + d*y + f

func (m xform) apply(x, y float64) (float64, float64) {
	return m.a*x + m.c*y + m.e, m.b*x + m.d*y + m.f
}

func (m xform) mul(n xform) xform { // m then n
	return xform{
		a: m.a*n.a + m.b*n.c, b: m.a*n.b + m.b*n.d,
		c: m.c*n.a + m.d*n.c, d: m.c*n.b + m.d*n.d,
		e: m.e*n.a + m.f*n.c + n.e, f: m.e*n.b + m.f*n.d + n.f,
	}
}

var ident = xform{a: 1, d: 1}

type pen struct {
	style int
	width float64
	col   color.RGBA
	none  bool
}

type brush struct {
	col  color.RGBA
	none bool
	img  string // data URI of a pattern bitmap
}

type font struct {
	height float64
	weight int
	italic bool
	face   string
	esc    float64
}

type dc struct {
	world          xform
	winOrg, winExt [2]float64
	vpOrg, vpExt   [2]float64
	mapMode        int
	pen            pen
	brush          brush
	font           font
	textCol        color.RGBA
	textAlign      int
	polyFill       int
	clip           string // clip path id
	cur            [2]float64
}

type conv struct {
	data      []byte
	bitmap    func(img *image.NRGBA, pngSize int) ([]byte, string) // Options.Bitmap
	out       strings.Builder
	defs      strings.Builder
	objs      map[uint32]any
	st        dc
	stack     []dc
	path      strings.Builder
	inPath    bool
	nid       int
	unknown   int
	records   int
	bounds    [4]float64
	fromFrame bool
}

// Convert converts EMF data.
func Convert(data []byte) (Result, error) { return ConvertWith(data, Options{}) }

// Options configure the conversion of a metafile.
type Options struct {
	// Bitmap, when set, may write a bitmap of the metafile in another format than PNG. It gets
	// the picture and the size of its PNG file and returns the data and the MIME type to use,
	// or nil to keep the PNG.
	Bitmap func(img *image.NRGBA, pngSize int) (data []byte, mime string)
}

// ConvertWith converts an EMF file to SVG with options.
func ConvertWith(data []byte, opt Options) (Result, error) {
	if len(data) < 88 || binary.LittleEndian.Uint32(data) != 1 || string(data[40:44]) != " EMF" {
		return Result{}, ErrNotEMF
	}
	c := &conv{data: data, objs: map[uint32]any{}, bitmap: opt.Bitmap}
	c.st = dc{world: ident, winExt: [2]float64{1, 1}, vpExt: [2]float64{1, 1}, mapMode: 1,
		pen: pen{width: 1, col: color.RGBA{0, 0, 0, 255}}, brush: brush{col: color.RGBA{255, 255, 255, 255}}, polyFill: 1,
		textCol: color.RGBA{0, 0, 0, 255}}
	// header: bounds (device units, the drawn area) and frame (0.01 mm, the picture frame that
	// PowerPoint scales to the shape). The frame converted to device units is the view box.
	for i := 0; i < 4; i++ {
		c.bounds[i] = float64(int32(binary.LittleEndian.Uint32(data[8+4*i:])))
	}
	devX, devY := i32(data, 72), i32(data, 76)
	mmX, mmY := i32(data, 80), i32(data, 84)
	if devX > 0 && devY > 0 && mmX > 0 && mmY > 0 {
		var fr [4]float64
		for i := 0; i < 4; i++ {
			fr[i] = i32(data, 24+4*i) / 100
		}
		sx, sy := devX/mmX, devY/mmY
		if fr[2] > fr[0] && fr[3] > fr[1] {
			c.bounds = [4]float64{fr[0] * sx, fr[1] * sy, fr[2] * sx, fr[3] * sy}
			c.fromFrame = true
		}
	}
	pos := 0
	for pos+8 <= len(data) {
		typ := binary.LittleEndian.Uint32(data[pos:])
		size := int(binary.LittleEndian.Uint32(data[pos+4:]))
		if size < 8 || pos+size > len(data) || size%4 != 0 {
			break
		}
		c.records++
		c.record(typ, data[pos+8:pos+size])
		if typ == 14 { // EOF
			break
		}
		pos += size
	}
	w, h := c.bounds[2]-c.bounds[0], c.bounds[3]-c.bounds[1]
	if !c.fromFrame { // device bounds include their right and bottom edge
		w, h = w+1, h+1
	}
	if w <= 0 || h <= 0 {
		return Result{}, fmt.Errorf("EMF has an empty picture frame")
	}
	var svg strings.Builder
	fmt.Fprintf(&svg, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="%s %s %s %s" preserveAspectRatio="none">`, n(c.bounds[0]), n(c.bounds[1]), n(w), n(h))
	if c.defs.Len() > 0 {
		svg.WriteString("<defs>" + c.defs.String() + "</defs>")
	}
	svg.WriteString(c.out.String())
	svg.WriteString("</svg>")
	return Result{SVG: []byte(svg.String()), Unknown: c.unknown, Records: c.records}, nil
}

func n(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "0"
	}
	s := strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64)
	if s == "-0" {
		return "0"
	}
	return s
}

func i32(b []byte, off int) float64 {
	if off+4 > len(b) {
		return 0
	}
	return float64(int32(binary.LittleEndian.Uint32(b[off:])))
}

func u32(b []byte, off int) uint32 {
	if off+4 > len(b) {
		return 0
	}
	return binary.LittleEndian.Uint32(b[off:])
}

func i16(b []byte, off int) float64 {
	if off+2 > len(b) {
		return 0
	}
	return float64(int16(binary.LittleEndian.Uint16(b[off:])))
}

func f32(b []byte, off int) float64 {
	if off+4 > len(b) {
		return 0
	}
	return float64(math.Float32frombits(binary.LittleEndian.Uint32(b[off:])))
}

func rgb(v uint32) color.RGBA { return color.RGBA{uint8(v), uint8(v >> 8), uint8(v >> 16), 255} }

func hex(c color.RGBA) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

// page maps logical coordinates (after the world transform) to device coordinates.
func (c *conv) page() xform {
	s := c.st
	switch s.mapMode {
	case 7, 8: // MM_ISOTROPIC, MM_ANISOTROPIC
		sx, sy := 1.0, 1.0
		if s.winExt[0] != 0 {
			sx = s.vpExt[0] / s.winExt[0]
		}
		if s.winExt[1] != 0 {
			sy = s.vpExt[1] / s.winExt[1]
		}
		if s.mapMode == 7 {
			m := math.Min(math.Abs(sx), math.Abs(sy))
			sx, sy = math.Copysign(m, sx), math.Copysign(m, sy)
		}
		return xform{a: sx, d: sy, e: s.vpOrg[0] - s.winOrg[0]*sx, f: s.vpOrg[1] - s.winOrg[1]*sy}
	}
	return xform{a: 1, d: 1, e: s.vpOrg[0] - s.winOrg[0], f: s.vpOrg[1] - s.winOrg[1]}
}

func (c *conv) pt(x, y float64) string {
	x, y = c.st.world.mul(c.page()).apply(x, y)
	return n(x) + " " + n(y)
}

// scale returns the length scale of the current transforms (for pen widths).
func (c *conv) scale() float64 {
	m := c.st.world.mul(c.page())
	return math.Sqrt(math.Abs(m.a*m.d - m.b*m.c))
}

func (c *conv) fillAttr() string {
	b := c.st.brush
	rule := ""
	if c.st.polyFill == 2 {
		rule = ` fill-rule="nonzero"`
	} else {
		rule = ` fill-rule="evenodd"`
	}
	if b.none {
		return `fill="none"`
	}
	if b.img != "" {
		c.nid++
		id := fmt.Sprintf("p%d", c.nid)
		fmt.Fprintf(&c.defs, `<pattern id="%s" patternUnits="userSpaceOnUse" width="8" height="8"><image href="%s" width="8" height="8"/></pattern>`, id, b.img)
		return `fill="url(#` + id + `)"` + rule
	}
	s := `fill="` + hex(b.col) + `"` + rule
	return s
}

func (c *conv) strokeAttr() string {
	p := c.st.pen
	if p.none || p.style&0xF == 5 { // PS_NULL
		return ` stroke="none"`
	}
	w := p.width * c.scale()
	if w < 1 {
		w = 1 // cosmetic pens draw one device pixel
	}
	s := ` stroke="` + hex(p.col) + `" stroke-width="` + n(w) + `"`
	switch p.style & 0xF {
	case 1:
		s += ` stroke-dasharray="` + n(3*w) + " " + n(w) + `"`
	case 2:
		s += ` stroke-dasharray="` + n(w) + " " + n(w) + `"`
	}
	switch p.style & 0xF00 {
	case 0x100:
		s += ` stroke-linecap="square"`
	case 0x200:
		s += ` stroke-linecap="butt"`
	default:
		s += ` stroke-linecap="round"`
	}
	switch p.style & 0xF000 {
	case 0x1000:
		s += ` stroke-linejoin="bevel"`
	case 0x2000:
		s += ` stroke-linejoin="miter"`
	default:
		s += ` stroke-linejoin="round"`
	}
	return s
}

func (c *conv) clipAttr() string {
	if c.st.clip == "" {
		return ""
	}
	return ` clip-path="url(#` + c.st.clip + `)"`
}

func (c *conv) draw(d string, fill, stroke bool) {
	if d == "" {
		return
	}
	f := `fill="none"`
	if fill {
		f = c.fillAttr()
	}
	s := ` stroke="none"`
	if stroke {
		s = c.strokeAttr()
	}
	fmt.Fprintf(&c.out, `<path d="%s" %s%s%s/>`, d, f, s, c.clipAttr())
}

// emit adds path data: into the open path bracket, or as a drawn shape.
func (c *conv) emit(d string, closed bool) {
	if c.inPath {
		c.path.WriteString(d)
		return
	}
	c.draw(d, closed, true)
}

func (c *conv) record(typ uint32, b []byte) {
	switch typ {
	case 1, 14, 70, 18, 21, 13, 58, 98, 115, 16, 25, 23: // header, EOF, comment, modes without visible effect
	case 9: // SETWINDOWEXTEX
		c.st.winExt = [2]float64{i32(b, 0), i32(b, 4)}
	case 10: // SETWINDOWORGEX
		c.st.winOrg = [2]float64{i32(b, 0), i32(b, 4)}
	case 11: // SETVIEWPORTEXTEX
		c.st.vpExt = [2]float64{i32(b, 0), i32(b, 4)}
	case 12: // SETVIEWPORTORGEX
		c.st.vpOrg = [2]float64{i32(b, 0), i32(b, 4)}
	case 17: // SETMAPMODE
		c.st.mapMode = int(u32(b, 0))
	case 19: // SETPOLYFILLMODE
		c.st.polyFill = int(u32(b, 0))
	case 22: // SETTEXTALIGN
		c.st.textAlign = int(u32(b, 0))
	case 24: // SETTEXTCOLOR
		c.st.textCol = rgb(u32(b, 0))
	case 33: // SAVEDC
		c.stack = append(c.stack, c.st)
	case 34: // RESTOREDC
		k := int(int32(u32(b, 0)))
		if k < 0 && len(c.stack)+k >= 0 {
			c.st = c.stack[len(c.stack)+k]
			c.stack = c.stack[:len(c.stack)+k]
		}
	case 35: // SETWORLDTRANSFORM
		c.st.world = xform{f32(b, 0), f32(b, 4), f32(b, 8), f32(b, 12), f32(b, 16), f32(b, 20)}
	case 36: // MODIFYWORLDTRANSFORM
		m := xform{f32(b, 0), f32(b, 4), f32(b, 8), f32(b, 12), f32(b, 16), f32(b, 20)}
		switch u32(b, 24) {
		case 1:
			c.st.world = ident
		case 2: // left multiply
			c.st.world = m.mul(c.st.world)
		case 3:
			c.st.world = c.st.world.mul(m)
		case 4:
			c.st.world = m
		}
	case 37: // SELECTOBJECT
		h := u32(b, 0)
		if h&0x80000000 != 0 {
			c.stock(h & 0x7FFFFFFF)
			return
		}
		switch o := c.objs[h].(type) {
		case pen:
			c.st.pen = o
		case brush:
			c.st.brush = o
		case font:
			c.st.font = o
		}
	case 38: // CREATEPEN
		c.objs[u32(b, 0)] = pen{style: int(u32(b, 4)), width: i32(b, 8), col: rgb(u32(b, 16))}
	case 95: // EXTCREATEPEN
		style := int(u32(b, 20))
		w := float64(u32(b, 24))
		bs := u32(b, 28)
		c.objs[u32(b, 0)] = pen{style: style, width: w, col: rgb(u32(b, 32)), none: bs == 1}
	case 39: // CREATEBRUSHINDIRECT
		style := u32(b, 4)
		c.objs[u32(b, 0)] = brush{col: rgb(u32(b, 8)), none: style == 1}
	case 93, 94: // CREATEMONOBRUSH, CREATEDIBPATTERNBRUSHPT
		c.objs[u32(b, 0)] = brush{img: c.dibURI(b, int(u32(b, 8)), int(u32(b, 12)), int(u32(b, 16)), int(u32(b, 20)), -8)}
	case 40: // DELETEOBJECT
		delete(c.objs, u32(b, 0))
	case 82: // EXTCREATEFONTINDIRECTW
		f := font{height: math.Abs(i32(b, 4)), esc: i32(b, 12), weight: int(i32(b, 20)), italic: len(b) > 24 && b[24] != 0}
		if len(b) >= 32+64 {
			u := make([]uint16, 32)
			for i := range u {
				u[i] = binary.LittleEndian.Uint16(b[32+2*i:])
			}
			f.face = strings.TrimRight(string(utf16.Decode(u)), "\x00")
			if i := strings.IndexByte(f.face, 0); i >= 0 {
				f.face = f.face[:i]
			}
		}
		c.objs[u32(b, 0)] = f
	case 27: // MOVETOEX
		c.st.cur = [2]float64{i32(b, 0), i32(b, 4)}
		if c.inPath {
			c.path.WriteString("M" + c.pt(i32(b, 0), i32(b, 4)) + " ")
		}
	case 54: // LINETO
		x, y := i32(b, 0), i32(b, 4)
		d := "L" + c.pt(x, y) + " "
		if !c.inPath {
			d = "M" + c.pt(c.st.cur[0], c.st.cur[1]) + " " + d
		}
		c.emit(d, false)
		c.st.cur = [2]float64{x, y}
	case 59: // BEGINPATH
		c.inPath = true
		c.path.Reset()
	case 60: // ENDPATH
		c.inPath = false
	case 61: // CLOSEFIGURE
		if c.inPath {
			c.path.WriteString("Z ")
		}
	case 62: // FILLPATH
		c.draw(strings.TrimSpace(c.path.String()), true, false)
	case 63: // STROKEANDFILLPATH
		c.draw(strings.TrimSpace(c.path.String()), true, true)
	case 64: // STROKEPATH
		c.draw(strings.TrimSpace(c.path.String()), false, true)
	case 67: // SELECTCLIPPATH
		mode := u32(b, 0)
		if d := strings.TrimSpace(c.path.String()); d != "" && (mode == 1 || mode == 5) {
			c.nid++
			id := fmt.Sprintf("c%d", c.nid)
			rule := "evenodd"
			if c.st.polyFill == 2 {
				rule = "nonzero"
			}
			inner := ""
			if c.st.clip != "" && mode == 1 {
				inner = ` clip-path="url(#` + c.st.clip + `)"`
			}
			fmt.Fprintf(&c.defs, `<clipPath id="%s"%s><path d="%s" clip-rule="%s"/></clipPath>`, id, inner, d, rule)
			c.st.clip = id
		}
		c.path.Reset()
	case 30, 29: // INTERSECTCLIPRECT, EXCLUDECLIPRECT
		if typ == 30 {
			c.nid++
			id := fmt.Sprintf("c%d", c.nid)
			fmt.Fprintf(&c.defs, `<clipPath id="%s"><path d="M%sL%sL%sL%sZ"/></clipPath>`, id, c.pt(i32(b, 0), i32(b, 4)), c.pt(i32(b, 8), i32(b, 4)), c.pt(i32(b, 8), i32(b, 12)), c.pt(i32(b, 0), i32(b, 12)))
			c.st.clip = id
		}
	case 26: // SETMETARGN: reset clipping
		c.st.clip = ""
	case 75: // EXTSELECTCLIPRGN with RGN_COPY and no region resets clipping
		c.st.clip = ""
	case 43: // RECTANGLE
		l, t, r, bt := i32(b, 0), i32(b, 4), i32(b, 8), i32(b, 12)
		c.emit("M"+c.pt(l, t)+"L"+c.pt(r, t)+"L"+c.pt(r, bt)+"L"+c.pt(l, bt)+"Z ", true)
	case 44: // ROUNDRECT
		l, t, r, bt := i32(b, 0), i32(b, 4), i32(b, 8), i32(b, 12)
		c.emit(c.ellipseRect(l, t, r, bt, i32(b, 16)/2, i32(b, 20)/2), true)
	case 42: // ELLIPSE
		l, t, r, bt := i32(b, 0), i32(b, 4), i32(b, 8), i32(b, 12)
		c.emit(c.ellipseRect(l, t, r, bt, (r-l)/2, (bt-t)/2), true)
	case 3, 4, 2, 5, 6: // POLYGON, POLYLINE, POLYBEZIER, POLYBEZIERTO, POLYLINETO (32-bit points)
		c.poly(typ, b, false)
	case 86, 87, 85, 88, 89: // the same with 16-bit points
		c.poly(typ, b, true)
	case 8, 7, 91, 90: // POLYPOLYGON, POLYPOLYLINE (32 and 16 bit)
		c.polyPoly(typ, b)
	case 84: // EXTTEXTOUTW
		c.text(b)
	case 81: // STRETCHDIBITS
		c.stretchDIB(b)
	case 76, 77: // BITBLT, STRETCHBLT
		c.blt(typ, b)
	default:
		c.unknown++
	}
}

func (c *conv) stock(id uint32) {
	switch id {
	case 0: // WHITE_BRUSH
		c.st.brush = brush{col: color.RGBA{255, 255, 255, 255}}
	case 1, 2, 3: // grey brushes
		g := uint8(192 - 64*(id-1))
		c.st.brush = brush{col: color.RGBA{g, g, g, 255}}
	case 4: // BLACK_BRUSH
		c.st.brush = brush{col: color.RGBA{0, 0, 0, 255}}
	case 5: // NULL_BRUSH
		c.st.brush = brush{none: true}
	case 6: // WHITE_PEN
		c.st.pen = pen{width: 1, col: color.RGBA{255, 255, 255, 255}}
	case 7: // BLACK_PEN
		c.st.pen = pen{width: 1, col: color.RGBA{0, 0, 0, 255}}
	case 8: // NULL_PEN
		c.st.pen = pen{none: true}
	}
}

func (c *conv) ellipseRect(l, t, r, b, rx, ry float64) string {
	// four cubic arcs (or a rounded rectangle), transformed point by point
	k := 0.5523
	if rx > (r-l)/2 {
		rx = (r - l) / 2
	}
	if ry > (b-t)/2 {
		ry = (b - t) / 2
	}
	pts := [][2]float64{
		{l + rx, t}, {r - rx, t}, {r - rx + k*rx, t}, {r, t + ry - k*ry}, {r, t + ry},
		{r, b - ry}, {r, b - ry + k*ry}, {r - rx + k*rx, b}, {r - rx, b},
		{l + rx, b}, {l + rx - k*rx, b}, {l, b - ry + k*ry}, {l, b - ry},
		{l, t + ry}, {l, t + ry - k*ry}, {l + rx - k*rx, t}, {l + rx, t},
	}
	p := func(i int) string { return c.pt(pts[i][0], pts[i][1]) }
	return "M" + p(0) + "L" + p(1) + "C" + p(2) + " " + p(3) + " " + p(4) + "L" + p(5) + "C" + p(6) + " " + p(7) + " " + p(8) +
		"L" + p(9) + "C" + p(10) + " " + p(11) + " " + p(12) + "L" + p(13) + "C" + p(14) + " " + p(15) + " " + p(16) + "Z "
}

// points reads count points starting at off.
func points(b []byte, off, count int, short bool) [][2]float64 {
	out := make([][2]float64, 0, count)
	for i := 0; i < count; i++ {
		if short {
			out = append(out, [2]float64{i16(b, off+4*i), i16(b, off+4*i+2)})
		} else {
			out = append(out, [2]float64{i32(b, off+8*i), i32(b, off+8*i+4)})
		}
	}
	return out
}

func (c *conv) poly(typ uint32, b []byte, short bool) {
	count := int(u32(b, 16))
	if count <= 0 || count > 1<<22 {
		return
	}
	ps := points(b, 20, count, short)
	var d strings.Builder
	base := typ
	if short {
		base = map[uint32]uint32{86: 3, 87: 4, 85: 2, 88: 5, 89: 6}[typ]
	}
	switch base {
	case 3, 4: // polygon, polyline
		d.WriteString("M" + c.pt(ps[0][0], ps[0][1]))
		for _, p := range ps[1:] {
			d.WriteString("L" + c.pt(p[0], p[1]))
		}
		if base == 3 {
			d.WriteString("Z")
		}
		c.emit(d.String()+" ", base == 3)
	case 2: // polybezier: start point plus groups of three
		d.WriteString("M" + c.pt(ps[0][0], ps[0][1]))
		for i := 1; i+2 < len(ps); i += 3 {
			d.WriteString("C" + c.pt(ps[i][0], ps[i][1]) + " " + c.pt(ps[i+1][0], ps[i+1][1]) + " " + c.pt(ps[i+2][0], ps[i+2][1]))
		}
		c.emit(d.String()+" ", false)
		c.st.cur = ps[len(ps)-1]
	case 5: // polybezierto: from the current point
		if !c.inPath {
			d.WriteString("M" + c.pt(c.st.cur[0], c.st.cur[1]))
		}
		for i := 0; i+2 < len(ps); i += 3 {
			d.WriteString("C" + c.pt(ps[i][0], ps[i][1]) + " " + c.pt(ps[i+1][0], ps[i+1][1]) + " " + c.pt(ps[i+2][0], ps[i+2][1]))
		}
		c.emit(d.String()+" ", false)
		c.st.cur = ps[len(ps)-1]
	case 6: // polylineto
		if !c.inPath {
			d.WriteString("M" + c.pt(c.st.cur[0], c.st.cur[1]))
		}
		for _, p := range ps {
			d.WriteString("L" + c.pt(p[0], p[1]))
		}
		c.emit(d.String()+" ", false)
		c.st.cur = ps[len(ps)-1]
	}
}

func (c *conv) polyPoly(typ uint32, b []byte) {
	short := typ == 91 || typ == 90
	npolys := int(u32(b, 16))
	total := int(u32(b, 20))
	if npolys <= 0 || npolys > 1<<20 || total <= 0 || total > 1<<22 {
		return
	}
	counts := make([]int, npolys)
	for i := range counts {
		counts[i] = int(u32(b, 24+4*i))
	}
	ps := points(b, 24+4*npolys, total, short)
	closed := typ == 8 || typ == 91
	var d strings.Builder
	k := 0
	for _, cnt := range counts {
		if cnt <= 0 || k+cnt > len(ps) {
			break
		}
		d.WriteString("M" + c.pt(ps[k][0], ps[k][1]))
		for _, p := range ps[k+1 : k+cnt] {
			d.WriteString("L" + c.pt(p[0], p[1]))
		}
		if closed {
			d.WriteString("Z")
		}
		k += cnt
	}
	c.emit(d.String()+" ", closed)
}

func (c *conv) text(b []byte) {
	// EMREXTTEXTOUTW: bounds(16) graphicsMode(4) exScale(4) eyScale(4) then EMRTEXT
	if len(b) < 76 {
		return
	}
	x, y := i32(b, 28), i32(b, 32)
	nch := int(u32(b, 36))
	off := int(u32(b, 40)) - 8
	if nch <= 0 || off < 0 || off+2*nch > len(b) {
		return
	}
	u := make([]uint16, nch)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[off+2*i:])
	}
	txt := string(utf16.Decode(u))
	f := c.st.font
	size := f.height * c.scale()
	if size <= 0 {
		size = 12
	}
	px, py := c.st.world.mul(c.page()).apply(x, y)
	anchor := ""
	switch c.st.textAlign & 6 {
	case 6:
		anchor = ` text-anchor="middle"`
	case 2:
		anchor = ` text-anchor="end"`
	}
	base := ` dominant-baseline="text-before-edge"`
	switch c.st.textAlign & 24 {
	case 24:
		base = ""
	case 8:
		base = ` dominant-baseline="text-after-edge"`
	}
	style := ""
	if f.weight >= 600 {
		style += ` font-weight="bold"`
	}
	if f.italic {
		style += ` font-style="italic"`
	}
	fam := strings.NewReplacer(`"`, "", "<", "", "&", "").Replace(f.face)
	fmt.Fprintf(&c.out, `<text x="%s" y="%s" font-family="%s, sans-serif" font-size="%s" fill="%s"%s%s%s%s xml:space="preserve">%s</text>`,
		n(px), n(py), fam, n(size), hex(c.st.textCol), anchor, base, style, c.clipAttr(), esc(txt))
}

func esc(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}

func (c *conv) stretchDIB(b []byte) {
	// bounds(16) xDest yDest xSrc ySrc cxSrc cySrc offBmiSrc cbBmiSrc offBitsSrc cbBitsSrc usage rop cxDest cyDest
	if len(b) < 72 {
		return
	}
	xd, yd := i32(b, 16), i32(b, 20)
	offBmi, cbBmi, offBits, cbBits := int(u32(b, 40)), int(u32(b, 44)), int(u32(b, 48)), int(u32(b, 52))
	wd, hd := i32(b, 64), i32(b, 68)
	uri := c.dibURI(b, offBmi, cbBmi, offBits, cbBits, -8)
	if uri == "" {
		c.unknown++
		return
	}
	c.image(uri, xd, yd, wd, hd)
}

func (c *conv) blt(typ uint32, b []byte) {
	// bounds(16) xDest yDest cxDest cyDest rop xSrc ySrc xform(24) bkColor usage offBmi cbBmi offBits cbBits [cxSrc cySrc]
	if len(b) < 92 {
		return
	}
	xd, yd, wd, hd := i32(b, 16), i32(b, 20), i32(b, 24), i32(b, 28)
	offBmi, cbBmi, offBits, cbBits := int(u32(b, 76)), int(u32(b, 80)), int(u32(b, 84)), int(u32(b, 88))
	if cbBmi == 0 {
		// a pattern fill of the destination with the current brush
		c.emit("M"+c.pt(xd, yd)+"L"+c.pt(xd+wd, yd)+"L"+c.pt(xd+wd, yd+hd)+"L"+c.pt(xd, yd+hd)+"Z ", true)
		return
	}
	uri := c.dibURI(b, offBmi, cbBmi, offBits, cbBits, -8)
	if uri == "" {
		c.unknown++
		return
	}
	c.image(uri, xd, yd, wd, hd)
}

func (c *conv) image(uri string, x, y, w, h float64) {
	m := c.st.world.mul(c.page())
	fmt.Fprintf(&c.out, `<image href="%s" x="0" y="0" width="1" height="1" preserveAspectRatio="none" transform="matrix(%s %s %s %s %s %s)"%s/>`,
		uri, n(m.a*w), n(m.b*w), n(m.c*h), n(m.d*h), n(m.a*x+m.c*y+m.e), n(m.b*x+m.d*y+m.f), c.clipAttr())
}

// dibURI decodes a device-independent bitmap (offsets relative to the record start, so the
// record body offsets are shifted by base) into a PNG data URI.
func (c *conv) dibURI(b []byte, offBmi, cbBmi, offBits, cbBits, base int) string {
	offBmi += base
	offBits += base
	if offBmi < 0 || cbBmi < 40 || offBmi+cbBmi > len(b) || offBits < 0 || offBits+cbBits > len(b) {
		return ""
	}
	hdr := b[offBmi : offBmi+cbBmi]
	w := int(int32(binary.LittleEndian.Uint32(hdr[4:])))
	h := int(int32(binary.LittleEndian.Uint32(hdr[8:])))
	bpp := int(binary.LittleEndian.Uint16(hdr[14:]))
	comp := binary.LittleEndian.Uint32(hdr[16:])
	clrUsed := int(binary.LittleEndian.Uint32(hdr[32:]))
	topDown := h < 0
	if h < 0 {
		h = -h
	}
	if w <= 0 || h <= 0 || w*h > 1<<26 {
		return ""
	}
	if comp == 4 || comp == 5 { // embedded JPEG or PNG
		mime := "image/jpeg"
		if comp == 5 {
			mime = "image/png"
		}
		return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b[offBits:offBits+cbBits])
	}
	if comp != 0 && comp != 3 {
		return ""
	}
	var pal []color.RGBA
	if bpp <= 8 {
		nc := clrUsed
		if nc == 0 {
			nc = 1 << bpp
		}
		for i := 0; i < nc && 40+4*i+4 <= len(hdr); i++ {
			q := hdr[40+4*i:]
			pal = append(pal, color.RGBA{q[2], q[1], q[0], 255})
		}
	}
	stride := ((w*bpp + 31) / 32) * 4
	bits := b[offBits : offBits+cbBits]
	if stride*h > len(bits) {
		return ""
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	hasAlpha := false
	for y := 0; y < h; y++ {
		row := bits[y*stride:]
		dy := h - 1 - y
		if topDown {
			dy = y
		}
		for x := 0; x < w; x++ {
			var col color.NRGBA
			switch bpp {
			case 32:
				q := row[4*x:]
				col = color.NRGBA{q[2], q[1], q[0], q[3]}
				if q[3] != 0 {
					hasAlpha = true
				}
			case 24:
				q := row[3*x:]
				col = color.NRGBA{q[2], q[1], q[0], 255}
			case 16:
				v := binary.LittleEndian.Uint16(row[2*x:])
				col = color.NRGBA{uint8((v >> 10 & 31) << 3), uint8((v >> 5 & 31) << 3), uint8((v & 31) << 3), 255}
			case 8, 4, 1:
				var idx int
				switch bpp {
				case 8:
					idx = int(row[x])
				case 4:
					idx = int(row[x/2] >> (4 * (1 - x%2)) & 15)
				case 1:
					idx = int(row[x/8] >> (7 - x%8) & 1)
				}
				if idx < len(pal) {
					p := pal[idx]
					col = color.NRGBA{p.R, p.G, p.B, 255}
				}
			default:
				return ""
			}
			img.SetNRGBA(x, dy, col)
		}
	}
	if bpp == 32 && !hasAlpha {
		for i := 3; i < len(img.Pix); i += 4 {
			img.Pix[i] = 255
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return ""
	}
	if c.bitmap != nil {
		if data, mime := c.bitmap(img, buf.Len()); data != nil {
			return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
		}
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}
