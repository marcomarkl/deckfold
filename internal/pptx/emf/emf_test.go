package emf

import (
	"encoding/binary"
	"image"
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

// TestBitmapHook: a bitmap in a metafile goes into the SVG as PNG, unless Options.Bitmap
// returns another form of it.
func TestBitmapHook(t *testing.T) {
	const w, h = 4, 2
	body := make([]byte, 72)
	copy(body[16:], ints(10, 10, 0, 0, w, h, 80, 40, 120, 24, 0, 0x00CC0020, 40, 20))
	bmi := make([]byte, 40)
	binary.LittleEndian.PutUint32(bmi[0:], 40)
	binary.LittleEndian.PutUint32(bmi[4:], w)
	binary.LittleEndian.PutUint32(bmi[8:], h)
	binary.LittleEndian.PutUint16(bmi[12:], 1)
	binary.LittleEndian.PutUint16(bmi[14:], 24)
	bits := make([]byte, 24)
	for i := range bits {
		bits[i] = byte(40 * i)
	}
	file := build(rec(81, append(append(body, bmi...), bits...)))

	res, err := Convert(file)
	if err != nil || !strings.Contains(string(res.SVG), `href="data:image/png;base64,`) {
		t.Fatalf("without a hook the bitmap must be a PNG: %v %s", err, res.SVG)
	}
	var got image.Rectangle
	var pngSize int
	res, err = ConvertWith(file, Options{Bitmap: func(img *image.NRGBA, size int) ([]byte, string) {
		got, pngSize = img.Bounds(), size
		return []byte("X"), "image/webp"
	}})
	if err != nil || !strings.Contains(string(res.SVG), `href="data:image/webp;base64,WA=="`) || strings.Contains(string(res.SVG), "image/png") {
		t.Errorf("the hook's data is not in the SVG: %v %s", err, res.SVG)
	}
	if got != image.Rect(0, 0, w, h) || pngSize == 0 {
		t.Errorf("the hook got a picture of %v and a PNG size of %d", got, pngSize)
	}
	res, _ = ConvertWith(file, Options{Bitmap: func(*image.NRGBA, int) ([]byte, string) { return nil, "" }})
	if !strings.Contains(string(res.SVG), `href="data:image/png;base64,`) {
		t.Errorf("a hook that returns nothing must leave the PNG: %s", res.SVG)
	}
}
