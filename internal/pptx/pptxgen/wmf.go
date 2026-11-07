package pptxgen

import "encoding/binary"

// TestWMF returns a Windows Metafile of 400 x 300 logical units with a placeable header: a red
// rectangle, a blue outlined ellipse, a green triangle, a black arc, a line and the text "WMF".
func TestWMF() []byte {
	var recs []byte
	rec := func(fn uint16, params ...int) {
		b := make([]byte, 6+2*len(params))
		binary.LittleEndian.PutUint32(b, uint32(3+len(params)))
		binary.LittleEndian.PutUint16(b[4:], fn)
		for i, p := range params {
			binary.LittleEndian.PutUint16(b[6+2*i:], uint16(int16(p)))
		}
		recs = append(recs, b...)
	}
	color := func(r, g, b int) (int, int) { return r | g<<8, b }
	rec(0x020B, 0, 0)     // SETWINDOWORG y x
	rec(0x020C, 300, 400) // SETWINDOWEXT y x
	lo, hi := color(220, 40, 40)
	rec(0x02FC, 0, lo, hi, 0) // CREATEBRUSHINDIRECT solid -> object 0
	lo, hi = color(40, 80, 220)
	rec(0x02FA, 0, 6, 0, lo, hi) // CREATEPENINDIRECT solid, width 6 -> object 1
	lo, hi = color(40, 160, 60)
	rec(0x02FC, 0, lo, hi, 0)                           // green brush -> object 2
	rec(0x012D, 0)                                      // SELECTOBJECT red brush
	rec(0x012D, 1)                                      // SELECTOBJECT blue pen
	rec(0x041B, 140, 180, 20, 20)                       // RECTANGLE bottom right top left
	rec(0x0418, 140, 380, 20, 220)                      // ELLIPSE
	rec(0x012D, 2)                                      // green brush
	rec(0x0324, 3, 20, 280, 100, 170, 180, 280)         // POLYGON
	rec(0x0817, 230, 240, 230, 380, 290, 380, 170, 240) // ARC: end, start, rect
	rec(0x0214, 290, 20)                                // MOVETO y x
	rec(0x0213, 290, 200)                               // LINETO
	// CREATEFONTINDIRECT: height -40, weight 700, face "Arial"
	face := []byte("Arial\x00")
	font := []int{-40, 0, 0, 0, 700, 0, 0, 0}
	for i := 0; i < len(face); i += 2 {
		v := int(face[i])
		if i+1 < len(face) {
			v |= int(face[i+1]) << 8
		}
		font = append(font, v)
	}
	rec(0x02FB, font...) // -> object 3
	rec(0x012D, 3)
	lo, hi = color(0, 0, 0)
	rec(0x0209, lo, hi)                       // SETTEXTCOLOR
	rec(0x012E, 24)                           // SETTEXTALIGN TA_BASELINE
	rec(0x0521, 3, 'W'|'M'<<8, 'F', 280, 250) // TEXTOUT "WMF" at x 250, y 280 (y first after the string)
	rec(0, 0)                                 // EOF (no parameters; the extra word is padding)

	head := make([]byte, 22+18)
	binary.LittleEndian.PutUint32(head, 0x9AC6CDD7)
	binary.LittleEndian.PutUint16(head[10:], 400) // right
	binary.LittleEndian.PutUint16(head[12:], 300) // bottom
	binary.LittleEndian.PutUint16(head[14:], 96)  // units per inch
	var sum uint16
	for i := 0; i < 20; i += 2 {
		sum ^= binary.LittleEndian.Uint16(head[i:])
	}
	binary.LittleEndian.PutUint16(head[20:], sum)
	m := head[22:]
	binary.LittleEndian.PutUint16(m, 1)
	binary.LittleEndian.PutUint16(m[2:], 9)
	binary.LittleEndian.PutUint16(m[4:], 0x0300)
	binary.LittleEndian.PutUint32(m[6:], uint32((18+len(recs))/2))
	binary.LittleEndian.PutUint16(m[10:], 4)
	binary.LittleEndian.PutUint32(m[12:], 16)
	return append(head, recs...)
}
