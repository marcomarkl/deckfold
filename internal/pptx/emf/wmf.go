package emf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image/color"
	"math"
	"strings"

	"golang.org/x/text/encoding/charmap"
)

// ErrNotWMF marks data that is no WMF file.
var ErrNotWMF = errors.New("not a WMF file")

// ConvertWMF converts a Windows Metafile (MS-WMF) to SVG, with the same drawing state and
// output as Convert: paths, polygons, rectangles, ellipses and arcs, pens, brushes, fonts and
// text, bitmaps (DIB), window and viewport, saved states and clip rectangles. The picture frame
// is the bounding box of the placeable header, otherwise the window.
func ConvertWMF(data []byte) (Result, error) {
	off := 0
	var box [4]float64
	hasBox := false
	if len(data) >= 22 && binary.LittleEndian.Uint32(data) == 0x9AC6CDD7 {
		for i := 0; i < 4; i++ {
			box[i] = i16(data, 6+2*i)
		}
		hasBox = box[2] != box[0] && box[3] != box[1]
		off = 22
	}
	if len(data) < off+18 {
		return Result{}, ErrNotWMF
	}
	if t := binary.LittleEndian.Uint16(data[off:]); t != 1 && t != 2 || binary.LittleEndian.Uint16(data[off+2:]) != 9 {
		return Result{}, ErrNotWMF
	}
	c := &conv{data: data, objs: map[uint32]any{}}
	c.st = dc{world: ident, winExt: [2]float64{1, 1}, vpExt: [2]float64{1, 1}, mapMode: 1,
		pen: pen{width: 1, col: color.RGBA{0, 0, 0, 255}}, brush: brush{col: color.RGBA{255, 255, 255, 255}}, polyFill: 1,
		textCol: color.RGBA{0, 0, 0, 255}}
	w := &wmf{conv: c}
	pos := off + 18
	for pos+6 <= len(data) {
		size := int(binary.LittleEndian.Uint32(data[pos:])) * 2
		fn := binary.LittleEndian.Uint16(data[pos+4:])
		if size < 6 || pos+size > len(data) {
			break
		}
		c.records++
		if fn == 0 { // META_EOF
			break
		}
		w.record(fn, data[pos+6:pos+size])
		pos += size
	}
	switch {
	case hasBox:
		// logical coordinates of the placeable header; the drawing uses the window, which maps to it
		c.bounds = [4]float64{math.Min(box[0], box[2]), math.Min(box[1], box[3]), math.Max(box[0], box[2]), math.Max(box[1], box[3])}
	case w.winSet:
		c.bounds = [4]float64{w.win[0], w.win[1], w.win[0] + w.win[2], w.win[1] + w.win[3]}
	default:
		return Result{}, fmt.Errorf("WMF has no picture frame")
	}
	bw, bh := c.bounds[2]-c.bounds[0], c.bounds[3]-c.bounds[1]
	if bw <= 0 || bh <= 0 {
		return Result{}, fmt.Errorf("WMF has an empty picture frame")
	}
	var svg strings.Builder
	fmt.Fprintf(&svg, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="%s %s %s %s" preserveAspectRatio="none">`, n(c.bounds[0]), n(c.bounds[1]), n(bw), n(bh))
	if c.defs.Len() > 0 {
		svg.WriteString("<defs>" + c.defs.String() + "</defs>")
	}
	svg.WriteString(c.out.String())
	svg.WriteString("</svg>")
	return Result{SVG: []byte(svg.String()), Unknown: c.unknown, Records: c.records}, nil
}

type wmf struct {
	*conv
	win    [4]float64 // window origin and extent
	winSet bool
}

// u16w reads an unsigned 16-bit value; WMF parameters are 16-bit words.
func u16w(b []byte, off int) int {
	if off < 0 || off+2 > len(b) {
		return 0
	}
	return int(binary.LittleEndian.Uint16(b[off:]))
}

func colorref(b []byte, off int) color.RGBA {
	if off+4 > len(b) {
		return color.RGBA{0, 0, 0, 255}
	}
	return color.RGBA{b[off], b[off+1], b[off+2], 255}
}

// add puts an object into the lowest free slot of the object table.
func (w *wmf) add(o any) {
	for i := uint32(0); ; i++ {
		if _, ok := w.objs[i]; !ok {
			w.objs[i] = o
			return
		}
	}
}

func (w *wmf) record(fn uint16, b []byte) {
	c := w.conv
	switch fn {
	// The SVG keeps logical coordinates: the view box is the window (or the placeable header's
	// box, which is in logical units), so window, viewport and mapping mode need no transform.
	case 0x020B: // SETWINDOWORG
		w.win[0], w.win[1] = i16(b, 2), i16(b, 0)
	case 0x020C: // SETWINDOWEXT
		w.win[2], w.win[3], w.winSet = i16(b, 2), i16(b, 0), true
	case 0x020D, 0x020E, 0x0103: // SETVIEWPORTORG, SETVIEWPORTEXT, SETMAPMODE
	case 0x0106: // SETPOLYFILLMODE
		c.st.polyFill = u16w(b, 0)
	case 0x012E: // SETTEXTALIGN
		c.st.textAlign = u16w(b, 0)
	case 0x0209: // SETTEXTCOLOR
		c.st.textCol = colorref(b, 0)
	case 0x001E: // SAVEDC
		c.stack = append(c.stack, c.st)
	case 0x0127: // RESTOREDC
		k := int(int16(u16w(b, 0)))
		if k < 0 && -k <= len(c.stack) {
			c.st = c.stack[len(c.stack)+k]
			c.stack = c.stack[:len(c.stack)+k]
		}
	case 0x02FA: // CREATEPENINDIRECT
		p := pen{style: u16w(b, 0), width: math.Abs(i16(b, 2)), col: colorref(b, 6)}
		p.none = p.style&0xF == 5
		w.add(p)
	case 0x02FC: // CREATEBRUSHINDIRECT
		br := brush{col: colorref(b, 2)}
		br.none = u16w(b, 0) == 1 // BS_NULL
		w.add(br)
	case 0x02FB: // CREATEFONTINDIRECT
		f := font{height: math.Abs(i16(b, 0)), weight: u16w(b, 8), italic: len(b) > 10 && b[10] != 0, esc: i16(b, 4) / 10}
		if len(b) > 18 {
			name := b[18:]
			if i := strings.IndexByte(string(name), 0); i >= 0 {
				name = name[:i]
			}
			if s, err := charmap.Windows1252.NewDecoder().Bytes(name); err == nil {
				f.face = string(s)
			}
		}
		w.add(f)
	case 0x00F7, 0x0142, 0x01F9, 0x06FF: // CREATEPALETTE, DIBCREATEPATTERNBRUSH, CREATEPATTERNBRUSH, CREATEREGION
		if fn == 0x0142 {
			w.add(brush{col: color.RGBA{128, 128, 128, 255}})
		} else {
			w.add(nil)
		}
	case 0x012D: // SELECTOBJECT
		switch o := c.objs[uint32(u16w(b, 0))].(type) {
		case pen:
			c.st.pen = o
		case brush:
			c.st.brush = o
		case font:
			c.st.font = o
		}
	case 0x01F0: // DELETEOBJECT
		delete(c.objs, uint32(u16w(b, 0)))
	case 0x0214: // MOVETO
		c.st.cur = [2]float64{i16(b, 2), i16(b, 0)}
	case 0x0213: // LINETO
		x, y := i16(b, 2), i16(b, 0)
		c.draw("M"+c.pt(c.st.cur[0], c.st.cur[1])+"L"+c.pt(x, y)+" ", false, true)
		c.st.cur = [2]float64{x, y}
	case 0x041B: // RECTANGLE
		l, t, r, bt := i16(b, 6), i16(b, 4), i16(b, 2), i16(b, 0)
		c.draw("M"+c.pt(l, t)+"L"+c.pt(r, t)+"L"+c.pt(r, bt)+"L"+c.pt(l, bt)+"Z ", true, true)
	case 0x061C: // ROUNDRECT
		c.draw(c.ellipseRect(i16(b, 10), i16(b, 8), i16(b, 6), i16(b, 4), i16(b, 2)/2, i16(b, 0)/2), true, true)
	case 0x0418: // ELLIPSE
		l, t, r, bt := i16(b, 6), i16(b, 4), i16(b, 2), i16(b, 0)
		c.draw(c.ellipseRect(l, t, r, bt, (r-l)/2, (bt-t)/2), true, true)
	case 0x0817, 0x081A, 0x0830: // ARC, PIE, CHORD
		w.arc(fn, b)
	case 0x0324, 0x0325: // POLYGON, POLYLINE
		k := u16w(b, 0)
		if 2+4*k > len(b) || k < 2 {
			return
		}
		var d strings.Builder
		for i := 0; i < k; i++ {
			if i == 0 {
				d.WriteString("M")
			} else {
				d.WriteString("L")
			}
			d.WriteString(c.pt(i16(b, 2+4*i), i16(b, 4+4*i)))
		}
		if fn == 0x0324 {
			d.WriteString("Z")
		}
		c.draw(d.String()+" ", fn == 0x0324, true)
	case 0x0538: // POLYPOLYGON
		np := u16w(b, 0)
		p := 2 + 2*np
		var d strings.Builder
		for i := 0; i < np; i++ {
			k := u16w(b, 2+2*i)
			if p+4*k > len(b) {
				return
			}
			for j := 0; j < k; j++ {
				if j == 0 {
					d.WriteString("M")
				} else {
					d.WriteString("L")
				}
				d.WriteString(c.pt(i16(b, p+4*j), i16(b, p+2+4*j)))
			}
			d.WriteString("Z ")
			p += 4 * k
		}
		c.draw(d.String(), true, true)
	case 0x0521: // TEXTOUT
		k := u16w(b, 0)
		if 2+k > len(b) {
			return
		}
		s := 2 + k + k%2
		w.text(i16(b, s+2), i16(b, s), b[2:2+k])
	case 0x0A32: // EXTTEXTOUT
		y, x, k, opts := i16(b, 0), i16(b, 2), u16w(b, 4), u16w(b, 6)
		s := 8
		if opts&0x6 != 0 {
			s += 8
		}
		if s+k > len(b) {
			return
		}
		w.text(x, y, b[s:s+k])
	case 0x0416: // INTERSECTCLIPRECT
		l, t, r, bt := i16(b, 6), i16(b, 4), i16(b, 2), i16(b, 0)
		c.nid++
		id := fmt.Sprintf("c%d", c.nid)
		fmt.Fprintf(&c.defs, `<clipPath id="%s"><path d="M%sL%sL%sL%sZ"/></clipPath>`, id, c.pt(l, t), c.pt(r, t), c.pt(r, bt), c.pt(l, bt))
		c.st.clip = id
	case 0x0F43: // STRETCHDIB: rop(4) usage(2) srcH srcW ySrc xSrc destH destW yDest xDest, DIB
		w.dib(b, 22, i16(b, 20), i16(b, 18), i16(b, 16), i16(b, 14))
	case 0x0B41: // DIBSTRETCHBLT: rop(4) srcH srcW ySrc xSrc destH destW yDest xDest, DIB
		w.dib(b, 20, i16(b, 18), i16(b, 16), i16(b, 14), i16(b, 12))
	case 0x0940: // DIBBITBLT: rop(4) ySrc xSrc height width yDest xDest, DIB
		w.dib(b, 16, i16(b, 14), i16(b, 12), i16(b, 10), i16(b, 8))
	case 0x0102, 0x0104, 0x0107, 0x0201, 0x0626, 0x0231, 0x0234, 0x0035, 0x0037, 0x0211, 0x0105: // modes and escapes without visible effect
	default:
		c.unknown++
	}
}

// text draws ANSI text at a logical point with the current font and alignment.
func (w *wmf) text(x, y float64, raw []byte) {
	s, err := charmap.Windows1252.NewDecoder().Bytes(raw)
	if err != nil {
		return
	}
	// reuse the EMF text drawing: build an EMREXTTEXTOUTW body
	u := []rune(string(s))
	b := make([]byte, 76+2*len(u))
	binary.LittleEndian.PutUint32(b[28:], uint32(int32(x)))
	binary.LittleEndian.PutUint32(b[32:], uint32(int32(y)))
	binary.LittleEndian.PutUint32(b[36:], uint32(len(u)))
	binary.LittleEndian.PutUint32(b[40:], 76+8)
	for i, r := range u {
		if r > 0xFFFF {
			r = '?'
		}
		binary.LittleEndian.PutUint16(b[76+2*i:], uint16(r))
	}
	w.conv.text(b)
}

// arc draws an elliptical arc, pie or chord in the rectangle, from the start to the end radial
// (counterclockwise, as GDI draws).
func (w *wmf) arc(fn uint16, b []byte) {
	c := w.conv
	ye, xe, ys, xs := i16(b, 0), i16(b, 2), i16(b, 4), i16(b, 6)
	bt, r, t, l := i16(b, 8), i16(b, 10), i16(b, 12), i16(b, 14)
	cx, cy, rx, ry := (l+r)/2, (t+bt)/2, math.Abs(r-l)/2, math.Abs(bt-t)/2
	if rx == 0 || ry == 0 {
		return
	}
	a0 := math.Atan2((ys-cy)/ry, (xs-cx)/rx)
	a1 := math.Atan2((ye-cy)/ry, (xe-cx)/rx)
	// counterclockwise on the screen is decreasing angle in y-down coordinates
	for a1 >= a0 {
		a1 -= 2 * math.Pi
	}
	var d strings.Builder
	steps := int(math.Ceil((a0 - a1) / (math.Pi / 16)))
	for i := 0; i <= steps; i++ {
		a := a0 + (a1-a0)*float64(i)/float64(steps)
		p := c.pt(cx+rx*math.Cos(a), cy+ry*math.Sin(a))
		if i == 0 {
			if fn == 0x081A {
				d.WriteString("M" + c.pt(cx, cy) + "L" + p)
			} else {
				d.WriteString("M" + p)
			}
		} else {
			d.WriteString("L" + p)
		}
	}
	if fn != 0x0817 {
		d.WriteString("Z")
	}
	c.draw(d.String()+" ", fn != 0x0817, true)
}

// dib draws the device-independent bitmap that starts at off in b into the logical rectangle.
func (w *wmf) dib(b []byte, off int, h, wd, y, x float64) {
	if off+40 > len(b) {
		w.unknown++
		return
	}
	hs := int(binary.LittleEndian.Uint32(b[off:]))
	bits := int(binary.LittleEndian.Uint16(b[off+14:]))
	comp := binary.LittleEndian.Uint32(b[off+16:])
	used := int(binary.LittleEndian.Uint32(b[off+32:]))
	pal := used
	if pal == 0 && bits <= 8 {
		pal = 1 << bits
	}
	cb := hs + 4*pal
	if comp == 3 && hs == 40 {
		cb += 12 // BI_BITFIELDS masks
	}
	uri := w.dibURI(b, off, cb, off+cb, len(b)-off-cb, 0)
	if uri == "" {
		w.unknown++
		return
	}
	w.image(uri, x, y, wd, h)
}
