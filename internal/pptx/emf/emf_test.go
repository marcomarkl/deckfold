package emf

import (
	"encoding/binary"
	"strings"
	"testing"
)

// build writes an EMF file from records (type, body).
func build(recs ...[]byte) []byte {
	hdr := make([]byte, 88)
	binary.LittleEndian.PutUint32(hdr[0:], 1)
	binary.LittleEndian.PutUint32(hdr[4:], 88)
	for i, v := range []int32{0, 0, 99, 99, 0, 0, 1000, 1000} { // bounds, frame (0.01 mm)
		binary.LittleEndian.PutUint32(hdr[8+4*i:], uint32(v))
	}
	copy(hdr[40:], " EMF")
	for i, v := range []int32{100, 100, 10, 10} { // device px, mm: 10 px per mm
		binary.LittleEndian.PutUint32(hdr[72+4*i:], uint32(v))
	}
	out := hdr
	for _, r := range append(recs, rec(14, make([]byte, 12))) {
		out = append(out, r...)
	}
	return out
}

func rec(typ uint32, body []byte) []byte {
	b := make([]byte, 8+len(body))
	binary.LittleEndian.PutUint32(b, typ)
	binary.LittleEndian.PutUint32(b[4:], uint32(len(b)))
	copy(b[8:], body)
	return b
}

func ints(v ...int32) []byte {
	b := make([]byte, 4*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint32(b[4*i:], uint32(x))
	}
	return b
}

func TestPathWithBrush(t *testing.T) {
	data := build(
		rec(39, ints(1, 0, 0x0000FF)), // CREATEBRUSHINDIRECT: solid red (COLORREF 0x00BBGGRR)
		rec(37, ints(1)),              // SELECTOBJECT
		rec(59, nil),                  // BEGINPATH
		rec(27, ints(10, 10)),         // MOVETOEX
		rec(54, ints(90, 10)),         // LINETO
		rec(54, ints(50, 90)),
		rec(61, nil), // CLOSEFIGURE
		rec(60, nil), // ENDPATH
		rec(62, ints(0, 0, 99, 99)),
		rec(9999, nil), // unknown
	)
	r, err := Convert(data)
	if err != nil {
		t.Fatal(err)
	}
	svg := string(r.SVG)
	for _, want := range []string{`viewBox="0 0 100 100"`, `d="M10 10 L90 10 L50 90 Z"`, `fill="#ff0000"`} {
		if !strings.Contains(svg, want) {
			t.Errorf("SVG has no %s:\n%s", want, svg)
		}
	}
	if r.Unknown != 1 {
		t.Errorf("unknown records = %d, want 1", r.Unknown)
	}
}

func TestNotEMF(t *testing.T) {
	if _, err := Convert([]byte("nope")); err != ErrNotEMF {
		t.Errorf("err = %v", err)
	}
}
