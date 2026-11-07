package mtx

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
)

// Encode compresses the tables of a TrueType font into MTX. It is a simple encoder for test
// data (the public feature deck with an embedded font and the round trip test): every point is
// written with the 16-bit triplets of section 5.11, glyph programs go unchanged into block 3
// (no separate push data), and LZCOMP writes only literal bytes. The result is valid MTX, but
// larger than what an optimizing encoder writes. hdmx, VDMX and DSIG are left out.
func Encode(tables map[string][]byte) ([]byte, error) {
	head, maxp, loca, glyf := tables["head"], tables["maxp"], tables["loca"], tables["glyf"]
	if len(head) < 54 || len(maxp) < 6 || glyf == nil || loca == nil {
		return nil, errors.New("mtx: not a TrueType font")
	}
	numGlyphs := int(binary.BigEndian.Uint16(maxp[4:]))
	long := binary.BigEndian.Uint16(head[50:]) == 1
	offset := func(i int) int {
		if long {
			return int(binary.BigEndian.Uint32(loca[4*i:]))
		}
		return 2 * int(binary.BigEndian.Uint16(loca[2*i:]))
	}
	if (long && len(loca) < 4*(numGlyphs+1)) || (!long && len(loca) < 2*(numGlyphs+1)) {
		return nil, errors.New("mtx: loca too short")
	}
	var g, code []byte
	for i := 0; i < numGlyphs; i++ {
		a, b := offset(i), offset(i+1)
		if a > b || b > len(glyf) {
			return nil, fmt.Errorf("mtx: glyph %d outside glyf", i)
		}
		var err error
		g, code, err = encodeGlyph(g, code, glyf[a:b])
		if err != nil {
			return nil, fmt.Errorf("mtx: glyph %d: %w", i, err)
		}
	}
	ctf := map[string][]byte{}
	for tag, t := range tables {
		switch tag {
		case "hdmx", "VDMX", "DSIG":
		case "glyf":
			ctf[tag] = g
		case "loca":
			ctf[tag] = nil // present in the directory with offset and length 0 (section 5.1)
		case "cvt ":
			ctf[tag] = encodeCvt(t)
		default:
			ctf[tag] = t
		}
	}
	blocks := [3][]byte{pack(directory(ctf)), pack(nil), pack(code)}
	out := []byte{3, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	put24 := func(b []byte, v int) { b[0], b[1], b[2] = byte(v>>16), byte(v>>8), byte(v) }
	put24(out[1:], len(blocks[0])+len(blocks[1])+len(blocks[2])) // copy limit: the whole data
	put24(out[4:], 10+len(blocks[0]))
	put24(out[7:], 10+len(blocks[0])+len(blocks[1]))
	for _, b := range blocks {
		out = append(out, b...)
	}
	return out, nil
}

// directory writes tables as an SFNT file; a nil table gets offset and length 0.
func directory(tables map[string][]byte) []byte {
	tags := make([]string, 0, len(tables))
	for t := range tables {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	n := len(tags)
	out := make([]byte, 12+16*n)
	binary.BigEndian.PutUint32(out, 0x00010000)
	binary.BigEndian.PutUint16(out[4:], uint16(n))
	for i, tag := range tags {
		rec := out[12+16*i:]
		copy(rec, tag)
		if tables[tag] == nil {
			continue
		}
		binary.BigEndian.PutUint32(rec[8:], uint32(len(out)))
		binary.BigEndian.PutUint32(rec[12:], uint32(len(tables[tag])))
		out = append(out, tables[tag]...)
		for len(out)%4 != 0 {
			out = append(out, 0)
		}
	}
	return out
}

func put255(b []byte, v int) []byte {
	switch {
	case v < 253:
		return append(b, byte(v))
	case v < 253+256:
		return append(b, 255, byte(v-253))
	default:
		return append(b, 253, byte(v>>8), byte(v))
	}
}

// encodeGlyph appends one TrueType glyph in CTF form to g and its program to code.
func encodeGlyph(g, code, t []byte) ([]byte, []byte, error) {
	if len(t) == 0 {
		return append(g, 0, 0), code, nil
	}
	if len(t) < 10 {
		return nil, nil, errors.New("glyph too short")
	}
	nc := int(int16(binary.BigEndian.Uint16(t)))
	if nc < 0 {
		// composite: header and components as in TrueType, then the program sizes
		p := 10
		flags := 0
		for {
			if p+4 > len(t) {
				return nil, nil, errors.New("composite glyph too short")
			}
			flags = int(binary.BigEndian.Uint16(t[p:]))
			p += 4
			if flags&0x1 != 0 {
				p += 4
			} else {
				p += 2
			}
			switch {
			case flags&0x8 != 0:
				p += 2
			case flags&0x40 != 0:
				p += 4
			case flags&0x80 != 0:
				p += 8
			}
			if flags&flagMoreComponents == 0 {
				break
			}
		}
		if p > len(t) {
			return nil, nil, errors.New("composite glyph too short")
		}
		g = append(g, 0xFF, 0xFF)
		g = append(g, t[2:p]...)
		if flags&flagHaveInstr != 0 {
			if p+2 > len(t) {
				return nil, nil, errors.New("composite program missing")
			}
			n := int(binary.BigEndian.Uint16(t[p:]))
			if p+2+n > len(t) {
				return nil, nil, errors.New("composite program too short")
			}
			g = put255(put255(g, 0), n)
			code = append(code, t[p+2:p+2+n]...)
		}
		return g, code, nil
	}
	// simple glyph: the bounding box is written explicitly (numContours 0x7FFF)
	p := 10
	ends := make([]int, nc)
	for i := range ends {
		if p+2 > len(t) {
			return nil, nil, errors.New("simple glyph too short")
		}
		ends[i] = int(binary.BigEndian.Uint16(t[p:]))
		p += 2
	}
	if p+2 > len(t) {
		return nil, nil, errors.New("simple glyph too short")
	}
	ni := int(binary.BigEndian.Uint16(t[p:]))
	p += 2
	if p+ni > len(t) {
		return nil, nil, errors.New("program too short")
	}
	prog := t[p : p+ni]
	p += ni
	n := 0
	if nc > 0 {
		n = ends[nc-1] + 1
	}
	flags := make([]byte, 0, n)
	for len(flags) < n {
		if p >= len(t) {
			return nil, nil, errors.New("flags too short")
		}
		f := t[p]
		p++
		flags = append(flags, f)
		if f&0x08 != 0 {
			if p >= len(t) {
				return nil, nil, errors.New("flags too short")
			}
			for k := 0; k < int(t[p]) && len(flags) < n; k++ {
				flags = append(flags, f)
			}
			p++
		}
	}
	coord := func(short, same byte) ([]int, error) {
		out := make([]int, n)
		for i, f := range flags {
			switch {
			case f&short != 0:
				if p >= len(t) {
					return nil, errors.New("coordinates too short")
				}
				v := int(t[p])
				p++
				if f&same == 0 {
					v = -v
				}
				out[i] = v
			case f&same == 0:
				if p+2 > len(t) {
					return nil, errors.New("coordinates too short")
				}
				out[i] = int(int16(binary.BigEndian.Uint16(t[p:])))
				p += 2
			}
		}
		return out, nil
	}
	dx, err := coord(0x02, 0x10)
	if err != nil {
		return nil, nil, err
	}
	dy, err := coord(0x04, 0x20)
	if err != nil {
		return nil, nil, err
	}
	g = append(g, 0x7F, 0xFF, t[0], t[1])
	g = append(g, t[2:10]...) // bounding box
	// the end point of the first contour, then the number of points of each further contour
	for i, e := range ends {
		if i == 0 {
			g = put255(g, e)
		} else {
			g = put255(g, e-ends[i-1])
		}
	}
	var xy []byte
	for i, f := range flags {
		// triplets 124..127: 16 bits for dx and for dy, with the signs in the index
		idx := byte(124)
		if dx[i] >= 0 {
			idx |= 1
		}
		if dy[i] >= 0 {
			idx |= 2
		}
		if f&0x01 == 0 {
			idx |= 0x80 // off curve
		}
		g = append(g, idx)
		ax, ay := abs(dx[i]), abs(dy[i])
		xy = append(xy, byte(ax>>8), byte(ax), byte(ay>>8), byte(ay))
	}
	g = append(g, xy...)
	g = put255(put255(g, 0), len(prog))
	return g, append(code, prog...), nil
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// encodeCvt writes the cvt table of section 5.2, every difference as a word when it does not
// fit into one byte.
func encodeCvt(t []byte) []byte {
	n := len(t) / 2
	out := []byte{byte(n >> 8), byte(n)}
	var last uint16
	for i := 0; i < n; i++ {
		v := binary.BigEndian.Uint16(t[2*i:])
		d := int16(v - last)
		last = v
		if d >= 0 && d < 238 {
			out = append(out, byte(d))
		} else {
			out = append(out, 238, byte(uint16(d)>>8), byte(d))
		}
	}
	return out
}

// pack writes data as an LZCOMP block of literal bytes, without run-length coding.
func pack(data []byte) []byte {
	w := &bitWriter{}
	w.bit(0) // no run-length stage
	newAhuff(nil, 1<<distWidth)
	newAhuff(nil, 1<<lenWidth)
	w.value(uint(len(data)), 24)
	ranges := 1
	for 1+(1<<(distWidth*ranges))-1 < len(data) {
		ranges++
	}
	sym := newAhuff(nil, 256+(1<<lenWidth)*ranges+3)
	for _, b := range data {
		sym.write(w, int(b))
	}
	return w.flush()
}

// bitWriter writes bits from the most significant bit of each byte down (BITIO.C).
type bitWriter struct {
	out   []byte
	buf   byte
	count int
}

func (w *bitWriter) bit(b uint) {
	w.buf = w.buf<<1 | byte(b&1)
	if w.count++; w.count == 8 {
		w.out = append(w.out, w.buf)
		w.buf, w.count = 0, 0
	}
}

func (w *bitWriter) value(v uint, n int) {
	for i := n - 1; i >= 0; i-- {
		w.bit(v >> uint(i))
	}
}

func (w *bitWriter) flush() []byte {
	for w.count != 0 {
		w.bit(0)
	}
	return w.out
}

// write encodes a symbol with the adaptive Huffman tree (AHUFF_WriteSymbol).
func (t *ahuff) write(w *bitWriter, sym int) {
	a := t.symbolIndex[sym]
	var path []uint
	for n := a; n != 1; n = t.up[n] {
		if t.right[t.up[n]] == n {
			path = append(path, 1)
		} else {
			path = append(path, 0)
		}
	}
	for i := len(path) - 1; i >= 0; i-- {
		w.bit(path[i])
	}
	t.update(a)
}
