package media

import (
	"bytes"
	"image"

	"golang.org/x/image/tiff"
)

// NoteTIFF is the warning for a TIFF picture that could not be converted.
const NoteTIFF = "a TIFF picture stays as it is, because deckfold cannot convert this kind of TIFF; only Safari shows TIFF pictures"

type tiffTag struct {
	first uint32 // the first value, for tags that hold a number
	raw   []byte // the bytes of the value
}

// tiffTags reads the tags of the first image directory of a TIFF structure (a TIFF file, or
// EXIF data). ok is false when the structure cannot be read.
func tiffTags(d []byte) (map[uint16]tiffTag, bool) {
	if len(d) < 8 {
		return nil, false
	}
	var u16 func([]byte) uint16
	var u32 func([]byte) uint32
	switch string(d[0:2]) {
	case "II":
		u16 = func(b []byte) uint16 { return uint16(b[0]) | uint16(b[1])<<8 }
		u32 = le32
	case "MM":
		u16, u32 = be16, be32
	default:
		return nil, false
	}
	if u16(d[2:]) != 42 {
		return nil, false
	}
	off := int(u32(d[4:]))
	if off < 8 || off+2 > len(d) {
		return nil, false
	}
	n := int(u16(d[off:]))
	if off+2+12*n > len(d) {
		return nil, false
	}
	size := map[uint16]int{1: 1, 2: 1, 3: 2, 4: 4, 5: 8, 6: 1, 7: 1, 8: 2, 9: 4, 10: 8, 11: 4, 12: 8}
	tags := map[uint16]tiffTag{}
	for i := 0; i < n; i++ {
		e := d[off+2+12*i:]
		typ, count := u16(e[2:]), int(u32(e[4:]))
		sz, known := size[typ]
		if !known || count < 0 || count > maxICCBytes {
			continue
		}
		val := e[8:12]
		if total := sz * count; total > 4 {
			at := int(u32(e[8:]))
			if at < 0 || at+total > len(d) {
				continue
			}
			val = d[at : at+total]
		} else {
			val = val[:total]
		}
		t := tiffTag{raw: val}
		switch {
		case len(val) == 0:
		case sz == 1:
			t.first = uint32(val[0])
		case sz == 2:
			t.first = uint32(u16(val))
		case sz == 4:
			t.first = u32(val)
		}
		tags[u16(e)] = t
	}
	return tags, true
}

// orient turns and mirrors a picture by the TIFF or EXIF orientation o (1 to 8), so that the
// result needs no orientation any more.
func orient(s *image.NRGBA, o int) *image.NRGBA {
	if o < 2 || o > 8 {
		return s
	}
	ws, hs := s.Bounds().Dx(), s.Bounds().Dy()
	w, h := ws, hs
	if o >= 5 {
		w, h = hs, ws
	}
	d := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sx, sy int
			switch o {
			case 2:
				sx, sy = ws-1-x, y
			case 3:
				sx, sy = ws-1-x, hs-1-y
			case 4:
				sx, sy = x, hs-1-y
			case 5:
				sx, sy = y, x
			case 6:
				sx, sy = y, hs-1-x
			case 7:
				sx, sy = ws-1-y, hs-1-x
			case 8:
				sx, sy = ws-1-y, x
			}
			copy(d.Pix[y*d.Stride+x*4:y*d.Stride+x*4+4], s.Pix[sy*s.Stride+sx*4:sy*s.Stride+sx*4+4])
		}
	}
	return d
}

// convertTIFF always converts a TIFF it can read, also when the result is larger: only Safari
// shows TIFF. "The same picture" here means the pixels of deckfold's decoder, turned as the
// orientation tag says, with the colour profile of the file.
func convertTIFF(data []byte) Result {
	keep := Result{Note: NoteTIFF}
	tags, ok := tiffTags(data)
	if !ok {
		return keep
	}
	cfg, err := tiff.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxPixels {
		return keep
	}
	img, err := tiff.Decode(bytes.NewReader(data))
	if err != nil {
		return keep
	}
	n, ok := toNRGBA(img)
	if !ok {
		return keep
	}
	o := 1
	if t, ok := tags[274]; ok && t.first != 0 {
		o = int(t.first) // some writers store 0, which readers take as "as stored"
	}
	if o < 1 || o > 8 {
		return keep
	}
	var icc []byte
	if t, ok := tags[34675]; ok && len(t.raw) > 0 {
		icc = t.raw
	}
	r := webpResult(encodeStill(orient(n, o), icc, nil), len(data), true)
	if !r.Changed {
		return keep
	}
	return r
}
