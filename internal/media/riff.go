package media

import (
	"bytes"
	"errors"
	"image"

	xwebp "golang.org/x/image/webp"
)

// The WebP container: a RIFF file of chunks. A lossless picture from the encoder is one VP8L
// chunk; a colour profile, EXIF data or an animation need the extended form, which starts with
// a VP8X chunk.

type chunk struct {
	id   string
	data []byte
}

var errWebP = errors.New("not a WebP file deckfold can read")

func parseRIFF(b []byte) ([]chunk, error) {
	if len(b) < 12 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WEBP" {
		return nil, errWebP
	}
	end := 8 + int(le32(b[4:]))
	if end > len(b) || end < 12 {
		return nil, errWebP
	}
	return parseChunks(b[12:end])
}

func parseChunks(b []byte) ([]chunk, error) {
	var out []chunk
	for len(b) > 0 {
		if len(b) < 8 {
			return nil, errWebP
		}
		n := int(le32(b[4:]))
		if n < 0 || 8+n > len(b) {
			return nil, errWebP
		}
		out = append(out, chunk{string(b[0:4]), b[8 : 8+n]})
		n += n & 1 // chunks are padded to an even size
		if 8+n > len(b) {
			break
		}
		b = b[8+n:]
	}
	return out, nil
}

func putLE32(b *bytes.Buffer, v uint32) {
	b.Write([]byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)})
}

func putLE24(b *bytes.Buffer, v uint32) { b.Write([]byte{byte(v), byte(v >> 8), byte(v >> 16)}) }

func writeChunks(b *bytes.Buffer, chunks []chunk) {
	for _, c := range chunks {
		b.WriteString(c.id)
		putLE32(b, uint32(len(c.data)))
		b.Write(c.data)
		if len(c.data)&1 == 1 {
			b.WriteByte(0)
		}
	}
}

func buildRIFF(chunks []chunk) []byte {
	var body bytes.Buffer
	body.WriteString("WEBP")
	writeChunks(&body, chunks)
	var out bytes.Buffer
	out.WriteString("RIFF")
	putLE32(&out, uint32(body.Len()))
	out.Write(body.Bytes())
	return out.Bytes()
}

// Flags of the VP8X chunk.
const (
	vp8xAnim  = 0x02
	vp8xEXIF  = 0x08
	vp8xAlpha = 0x10
	vp8xICC   = 0x20
)

func vp8x(flags byte, w, h int) chunk {
	var b bytes.Buffer
	b.Write([]byte{flags, 0, 0, 0})
	putLE24(&b, uint32(w-1))
	putLE24(&b, uint32(h-1))
	return chunk{"VP8X", b.Bytes()}
}

// withMetadata adds a colour profile and EXIF data to a WebP file from the encoder. The chunk
// order is the one the format prescribes: VP8X, ICCP, ANIM, picture data, EXIF.
func withMetadata(webp []byte, icc, exif []byte) ([]byte, error) {
	if len(icc) == 0 && len(exif) == 0 {
		return webp, nil
	}
	chunks, err := parseRIFF(webp)
	if err != nil || len(chunks) == 0 {
		return nil, errWebP
	}
	var flags byte
	var w, h int
	var rest []chunk
	switch chunks[0].id {
	case "VP8L":
		d := chunks[0].data
		if len(d) < 5 || d[0] != 0x2f {
			return nil, errWebP
		}
		bits := le32(d[1:])
		w, h = int(bits&0x3fff)+1, int(bits>>14&0x3fff)+1
		if bits>>28&1 == 1 {
			flags |= vp8xAlpha
		}
		rest = chunks
	case "VP8X":
		d := chunks[0].data
		if len(d) < 10 {
			return nil, errWebP
		}
		flags = d[0]
		w, h = (int(d[4])|int(d[5])<<8|int(d[6])<<16)+1, (int(d[7])|int(d[8])<<8|int(d[9])<<16)+1
		for _, c := range chunks[1:] {
			if c.id != "ICCP" && c.id != "EXIF" {
				rest = append(rest, c)
			}
		}
	default:
		return nil, errWebP
	}
	out := make([]chunk, 0, len(rest)+3)
	if len(icc) > 0 {
		flags |= vp8xICC
	}
	if len(exif) > 0 {
		flags |= vp8xEXIF
	}
	out = append(out, vp8x(flags, w, h))
	if len(icc) > 0 {
		out = append(out, chunk{"ICCP", icc})
	}
	out = append(out, rest...)
	if len(exif) > 0 {
		out = append(out, chunk{"EXIF", exif})
	}
	return buildRIFF(out), nil
}

// animFrame is one state of an animation: the whole canvas and how long it shows.
type animFrame struct {
	img *image.NRGBA
	ms  int
}

// decodeAnim reads an animated WebP file with a decoder that is not the encoder's: the frames
// are taken from the ANMF chunks, decoded one by one and put together by the rules of the
// format. It returns the canvases, their durations and the loop count (0 = forever).
//
// It only takes what a lossless animation made from a GIF needs: every pixel of a frame is
// opaque or fully transparent. Anything else is an error, and the caller keeps the original.
func decodeAnim(webp []byte) ([]animFrame, int, error) {
	chunks, err := parseRIFF(webp)
	if err != nil || len(chunks) == 0 || chunks[0].id != "VP8X" || len(chunks[0].data) < 10 {
		return nil, 0, errWebP
	}
	d := chunks[0].data
	if d[0]&vp8xAnim == 0 {
		return nil, 0, errWebP
	}
	cw, ch := (int(d[4])|int(d[5])<<8|int(d[6])<<16)+1, (int(d[7])|int(d[8])<<8|int(d[9])<<16)+1
	if cw*ch > maxPixels {
		return nil, 0, errWebP
	}
	loop := -1
	canvas := image.NewNRGBA(image.Rect(0, 0, cw, ch))
	var frames []animFrame
	var prev image.Rectangle
	prevDispose := false
	for _, c := range chunks[1:] {
		switch c.id {
		case "ANIM":
			if len(c.data) < 6 {
				return nil, 0, errWebP
			}
			loop = int(c.data[4]) | int(c.data[5])<<8
		case "ANMF":
			f := c.data
			if len(f) < 16 {
				return nil, 0, errWebP
			}
			x, y := 2*(int(f[0])|int(f[1])<<8|int(f[2])<<16), 2*(int(f[3])|int(f[4])<<8|int(f[5])<<16)
			fw, fh := (int(f[6])|int(f[7])<<8|int(f[8])<<16)+1, (int(f[9])|int(f[10])<<8|int(f[11])<<16)+1
			ms := int(f[12]) | int(f[13])<<8 | int(f[14])<<16
			noBlend, dispose := f[15]&0x02 != 0, f[15]&0x01 != 0
			r := image.Rect(x, y, x+fw, y+fh)
			if !r.In(canvas.Bounds()) {
				return nil, 0, errWebP
			}
			sub, err := parseChunks(f[16:])
			if err != nil || len(sub) != 1 || sub[0].id != "VP8L" {
				return nil, 0, errWebP
			}
			im, err := xwebp.Decode(bytes.NewReader(buildRIFF(sub)))
			if err != nil {
				return nil, 0, err
			}
			fr, ok := im.(*image.NRGBA)
			if !ok || fr.Bounds().Dx() != fw || fr.Bounds().Dy() != fh {
				return nil, 0, errWebP
			}
			if prevDispose {
				clearRect(canvas, prev)
			}
			for yy := 0; yy < fh; yy++ {
				src := fr.Pix[yy*fr.Stride : yy*fr.Stride+fw*4]
				dst := canvas.Pix[(y+yy)*canvas.Stride+x*4:]
				for xx := 0; xx < fw*4; xx += 4 {
					switch a := src[xx+3]; {
					case a == 255:
						copy(dst[xx:xx+4], src[xx:xx+4])
					case a == 0:
						// blended, the canvas shows through; not blended, the pixel
						// becomes transparent
						if noBlend {
							dst[xx], dst[xx+1], dst[xx+2], dst[xx+3] = 0, 0, 0, 0
						}
					default:
						return nil, 0, errWebP
					}
				}
			}
			prev, prevDispose = r, dispose
			cp := image.NewNRGBA(canvas.Bounds())
			copy(cp.Pix, canvas.Pix)
			frames = append(frames, animFrame{cp, ms})
			if len(frames) > maxAnimFrames || len(frames)*cw*ch*4 > maxAnimBytes {
				return nil, 0, errWebP
			}
		}
	}
	if loop < 0 || len(frames) == 0 {
		return nil, 0, errWebP
	}
	return frames, loop, nil
}

func clearRect(m *image.NRGBA, r image.Rectangle) {
	r = r.Intersect(m.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		row := m.Pix[y*m.Stride+r.Min.X*4 : y*m.Stride+r.Max.X*4]
		for i := range row {
			row[i] = 0
		}
	}
}
