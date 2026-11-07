package mtx

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// reader is a bounds-checked byte reader over one CTF stream.
type reader struct {
	b   []byte
	p   int
	err error
}

func (r *reader) u8() int {
	if r.p >= len(r.b) {
		r.err = errEOF
		return 0
	}
	v := r.b[r.p]
	r.p++
	return int(v)
}

func (r *reader) peek() int {
	if r.p >= len(r.b) {
		r.err = errEOF
		return 0
	}
	return int(r.b[r.p])
}

func (r *reader) u16() int { return r.u8()<<8 | r.u8() }

func (r *reader) i16() int { return int(int16(uint16(r.u16()))) }

func (r *reader) bytes(n int) []byte {
	if n < 0 || r.p+n > len(r.b) {
		r.err = errEOF
		return nil
	}
	v := r.b[r.p : r.p+n]
	r.p += n
	return v
}

// u255 reads a 255USHORT (section 6.1.1).
func (r *reader) u255() int {
	switch c := r.u8(); c {
	case 253:
		return r.u16()
	case 255:
		return 253 + r.u8()
	case 254:
		return 2*253 + r.u8()
	default:
		return c
	}
}

// s255 reads a 255SHORT (section 6.1.2).
func (r *reader) s255() int {
	c := r.u8()
	if c == 253 {
		return r.i16()
	}
	sign := 1
	if c == 250 {
		sign = -1
		c = r.u8()
	}
	switch c {
	case 255:
		c = 250 + r.u8()
	case 254:
		c = 2*250 + r.u8()
	}
	return sign * c
}

const (
	hop3Code = 251
	hop4Code = 252

	flagMoreComponents = 0x20
	flagHaveInstr      = 0x100
)

// glyphDecoder walks the glyph stream of the CTF glyf table together with the push data
// (block 2) and the glyph programs (block 3), glyph by glyph (section 6.2.5).
type glyphDecoder struct {
	g, push, code reader
}

// decodeGlyf turns the CTF glyf table into a TrueType glyf table and a long loca table.
func decodeGlyf(glyf, push, code []byte, numGlyphs int) ([]byte, []byte, error) {
	d := &glyphDecoder{g: reader{b: glyf}, push: reader{b: push}, code: reader{b: code}}
	out := make([]byte, 0, len(glyf)*2)
	loca := make([]byte, 4*(numGlyphs+1))
	for i := 0; i < numGlyphs; i++ {
		binary.BigEndian.PutUint32(loca[4*i:], uint32(len(out)))
		var err error
		out, err = d.glyph(out)
		if err != nil {
			return nil, nil, fmt.Errorf("mtx: glyph %d: %w", i, err)
		}
		for len(out)%4 != 0 {
			out = append(out, 0)
		}
	}
	binary.BigEndian.PutUint32(loca[4*numGlyphs:], uint32(len(out)))
	if d.g.p != len(glyf) {
		return nil, nil, fmt.Errorf("mtx: %d bytes of glyph data left over", len(glyf)-d.g.p)
	}
	return out, loca, nil
}

func (d *glyphDecoder) glyph(out []byte) ([]byte, error) {
	r := &d.g
	nc := r.i16()
	if r.err != nil {
		return nil, r.err
	}
	switch {
	case nc == 0:
		// An empty glyph: no outline and no program, length 0 in loca.
		return out, nil
	case nc < 0:
		return d.composite(out)
	}
	var bbox []int
	if nc == 0x7FFF {
		nc = r.i16()
		bbox = []int{r.i16(), r.i16(), r.i16(), r.i16()}
	}
	ends := make([]int, nc)
	last := -1
	for i := range ends {
		v := r.u255()
		if i == 0 {
			last = v
		} else {
			last += v
		}
		ends[i] = last
	}
	n := last + 1
	if r.err != nil || n < 0 || n > 0xFFFF {
		return nil, errors.New("invalid contours")
	}
	flags := r.bytes(n)
	if r.err != nil {
		return nil, r.err
	}
	xs, ys := make([]int, n), make([]int, n)
	x, y := 0, 0
	for i, f := range flags {
		t := triplets[f&0x7F]
		var v uint
		for _, b := range r.bytes(t.bytes - 1) {
			v = v<<8 | uint(b)
		}
		w := uint(8 * (t.bytes - 1))
		dx := int(v>>(w-uint(t.xBits))) & (1<<t.xBits - 1)
		dy := int(v>>(w-uint(t.xBits)-uint(t.yBits))) & (1<<t.yBits - 1)
		dx += t.deltaX
		dy += t.deltaY
		if t.xNeg {
			dx = -dx
		}
		if t.yNeg {
			dy = -dy
		}
		x += dx
		y += dy
		xs[i], ys[i] = x, y
	}
	if r.err != nil {
		return nil, r.err
	}
	if bbox == nil {
		bbox = []int{0, 0, 0, 0}
		for i := range xs {
			if i == 0 || xs[i] < bbox[0] {
				bbox[0] = xs[i]
			}
			if i == 0 || ys[i] < bbox[1] {
				bbox[1] = ys[i]
			}
			if i == 0 || xs[i] > bbox[2] {
				bbox[2] = xs[i]
			}
			if i == 0 || ys[i] > bbox[3] {
				bbox[3] = ys[i]
			}
		}
	}
	prog, err := d.program()
	if err != nil {
		return nil, err
	}
	out = be16(out, nc)
	for _, v := range bbox {
		out = be16(out, v)
	}
	for _, e := range ends {
		out = be16(out, e)
	}
	out = be16(out, len(prog))
	out = append(out, prog...)
	return appendPoints(out, flags, xs, ys), nil
}

func (d *glyphDecoder) composite(out []byte) ([]byte, error) {
	r := &d.g
	out = be16(out, -1)
	out = append(out, r.bytes(8)...) // bounding box
	for {
		start := r.p
		f := r.u16()
		r.u16() // glyph index
		if f&0x1 != 0 {
			r.bytes(4)
		} else {
			r.bytes(2)
		}
		switch {
		case f&0x8 != 0:
			r.bytes(2)
		case f&0x40 != 0:
			r.bytes(4)
		case f&0x80 != 0:
			r.bytes(8)
		}
		if r.err != nil {
			return nil, r.err
		}
		out = append(out, r.b[start:r.p]...)
		if f&flagMoreComponents == 0 {
			if f&flagHaveInstr != 0 {
				prog, err := d.program()
				if err != nil {
					return nil, err
				}
				out = be16(out, len(prog))
				out = append(out, prog...)
			}
			return out, nil
		}
	}
}

// program reads pushCount and codeSize from the glyph stream and returns the glyph program:
// new push instructions for the push data, then the instructions from block 3.
func (d *glyphDecoder) program() ([]byte, error) {
	pushCount := d.g.u255()
	codeSize := d.g.u255()
	if d.g.err != nil {
		return nil, d.g.err
	}
	vals := make([]int, 0, pushCount)
	p := &d.push
	for len(vals) < pushCount {
		switch p.peek() {
		case hop3Code, hop4Code:
			if len(vals) < 2 {
				return nil, errors.New("hop code at the start of the push data")
			}
			a := vals[len(vals)-2]
			hop4 := p.u8() == hop4Code
			vals = append(vals, a, p.s255(), a)
			if hop4 {
				vals = append(vals, p.s255(), a)
			}
		default:
			vals = append(vals, p.s255())
		}
		if p.err != nil {
			return nil, fmt.Errorf("push data: %w", p.err)
		}
	}
	if len(vals) != pushCount {
		return nil, errors.New("hop code beyond the push count")
	}
	prog := pushInstructions(vals)
	prog = append(prog, d.code.bytes(codeSize)...)
	if d.code.err != nil {
		return nil, fmt.Errorf("glyph program: %w", d.code.err)
	}
	return prog, nil
}

// pushInstructions encodes values with PUSHB/PUSHW (up to 8 values) or NPUSHB/NPUSHW, using
// bytes for runs of values in 0..255 and words otherwise.
func pushInstructions(vals []int) []byte {
	var out []byte
	for i := 0; i < len(vals); {
		isByte := vals[i] >= 0 && vals[i] <= 255
		j := i
		for j < len(vals) && j-i < 255 && (vals[j] >= 0 && vals[j] <= 255) == isByte {
			j++
		}
		n := j - i
		switch {
		case isByte && n <= 8:
			out = append(out, byte(0xB0+n-1))
		case isByte:
			out = append(out, 0x40, byte(n))
		case n <= 8:
			out = append(out, byte(0xB8+n-1))
		default:
			out = append(out, 0x41, byte(n))
		}
		for _, v := range vals[i:j] {
			if isByte {
				out = append(out, byte(v))
			} else {
				out = be16(out, v)
			}
		}
		i = j
	}
	return out
}

// appendPoints writes the flags and coordinates of a simple TrueType glyph with short vectors
// and repeated flags.
func appendPoints(out []byte, ctfFlags []byte, xs, ys []int) []byte {
	n := len(xs)
	flags := make([]byte, n)
	var xb, yb []byte
	px, py := 0, 0
	for i := 0; i < n; i++ {
		var f byte
		if ctfFlags[i]&0x80 == 0 {
			f = 0x01 // on curve
		}
		dx, dy := xs[i]-px, ys[i]-py
		px, py = xs[i], ys[i]
		switch {
		case dx == 0:
			f |= 0x10
		case dx > -256 && dx < 256:
			f |= 0x02
			if dx > 0 {
				f |= 0x10
			} else {
				dx = -dx
			}
			xb = append(xb, byte(dx))
		default:
			xb = be16(xb, dx)
		}
		switch {
		case dy == 0:
			f |= 0x20
		case dy > -256 && dy < 256:
			f |= 0x04
			if dy > 0 {
				f |= 0x20
			} else {
				dy = -dy
			}
			yb = append(yb, byte(dy))
		default:
			yb = be16(yb, dy)
		}
		flags[i] = f
	}
	for i := 0; i < n; {
		j := i + 1
		for j < n && flags[j] == flags[i] && j-i <= 255 {
			j++
		}
		if j-i > 2 {
			out = append(out, flags[i]|0x08, byte(j-i-1))
		} else {
			out = append(out, flags[i:j]...)
		}
		i = j
	}
	out = append(out, xb...)
	return append(out, yb...)
}

func be16(b []byte, v int) []byte { return append(b, byte(uint16(v)>>8), byte(v)) }
