package media

import (
	"bytes"
	"compress/zlib"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"

	gwebp "github.com/gen2brain/webp"
	"golang.org/x/image/bmp"
	xwebp "golang.org/x/image/webp"
)

const mimeWebP = "image/webp"

// toNRGBA copies a decoded picture into 8-bit RGBA without changing any sample. ok is false
// for kinds that would need a conversion with a loss: 16 bits per channel, CMYK, or
// premultiplied alpha that does not divide back exactly.
func toNRGBA(m image.Image) (*image.NRGBA, bool) {
	b := m.Bounds()
	w, h := b.Dx(), b.Dy()
	o := image.NewNRGBA(image.Rect(0, 0, w, h))
	switch s := m.(type) {
	case *image.NRGBA:
		for y := 0; y < h; y++ {
			copy(o.Pix[y*o.Stride:y*o.Stride+w*4], s.Pix[y*s.Stride:y*s.Stride+w*4])
		}
	case *image.RGBA:
		for y := 0; y < h; y++ {
			src := s.Pix[y*s.Stride : y*s.Stride+w*4]
			dst := o.Pix[y*o.Stride:]
			for x := 0; x < w*4; x += 4 {
				a := uint32(src[x+3])
				switch a {
				case 255:
					copy(dst[x:x+4], src[x:x+4])
				case 0:
					if src[x] != 0 || src[x+1] != 0 || src[x+2] != 0 {
						return nil, false
					}
				default:
					// premultiplied: divide, and take the picture only when multiplying
					// gives the stored value again
					for c := 0; c < 3; c++ {
						p := uint32(src[x+c])
						if p > a {
							return nil, false
						}
						v := (p*255 + a/2) / a
						if (v*a+127)/255 != p {
							return nil, false
						}
						dst[x+c] = uint8(v)
					}
					dst[x+3] = uint8(a)
				}
			}
		}
	case *image.Gray:
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				g := s.Pix[y*s.Stride+x]
				i := y*o.Stride + x*4
				o.Pix[i], o.Pix[i+1], o.Pix[i+2], o.Pix[i+3] = g, g, g, 255
			}
		}
	case *image.Paletted:
		pal := make([][4]uint8, 256)
		for i, c := range s.Palette {
			if i >= 256 {
				break
			}
			switch c := c.(type) {
			case color.NRGBA:
				pal[i] = [4]uint8{c.R, c.G, c.B, c.A}
			case color.RGBA:
				if c.A != 255 {
					return nil, false
				}
				pal[i] = [4]uint8{c.R, c.G, c.B, 255}
			default:
				return nil, false
			}
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				p := s.Pix[y*s.Stride+x]
				if int(p) >= len(s.Palette) {
					return nil, false
				}
				copy(o.Pix[y*o.Stride+x*4:], pal[p][:])
			}
		}
	default:
		return nil, false
	}
	return o, true
}

// encodeStill writes img as lossless WebP and reads the result back with another decoder. It
// returns nil when the result does not hold exactly the samples of img, the invisible ones
// included (they show where a picture is used as a texture without premultiplied alpha).
func encodeStill(img *image.NRGBA, icc, exif []byte) []byte {
	var b bytes.Buffer
	if err := gwebp.Encode(&b, img, gwebp.Options{Lossless: true, Quality: 100, Method: 6, Exact: true}); err != nil {
		return nil
	}
	out, err := withMetadata(b.Bytes(), icc, exif)
	if err != nil {
		return nil
	}
	back, err := xwebp.Decode(bytes.NewReader(out))
	if err != nil {
		return nil
	}
	n, ok := back.(*image.NRGBA)
	if !ok || n.Bounds().Dx() != img.Bounds().Dx() || n.Bounds().Dy() != img.Bounds().Dy() {
		return nil
	}
	w := img.Bounds().Dx() * 4
	for y := 0; y < img.Bounds().Dy(); y++ {
		if !bytes.Equal(img.Pix[y*img.Stride:y*img.Stride+w], n.Pix[y*n.Stride:y*n.Stride+w]) {
			return nil
		}
	}
	return out
}

func webpResult(out []byte, before int, always bool) Result {
	if out == nil || (!always && len(out) >= before) {
		return Result{}
	}
	return Result{Data: out, Mime: mimeWebP, Ext: "webp", Changed: true}
}

// pngInfo is what the chunks before the picture data say.
type pngInfo struct {
	w, h, depth  int
	animated     bool // acTL: an animated PNG
	icc          []byte
	srgb, gamma  bool // sRGB chunk; gAMA or cHRM chunk
	cicp         bool
	exif         []byte
	iccUnread    bool
	chunksParsed bool
}

func scanPNG(d []byte) pngInfo {
	var p pngInfo
	for pos := 8; pos+12 <= len(d); {
		n := int(be32(d[pos:]))
		typ := string(d[pos+4 : pos+8])
		if n < 0 || pos+12+n > len(d) {
			return p
		}
		body := d[pos+8 : pos+8+n]
		switch typ {
		case "IHDR":
			if n < 13 {
				return p
			}
			p.w, p.h, p.depth = int(be32(body)), int(be32(body[4:])), int(body[8])
		case "acTL":
			p.animated = true
		case "sRGB":
			p.srgb = true
		case "gAMA", "cHRM":
			p.gamma = true
		case "cICP":
			p.cicp = true
		case "eXIf":
			p.exif = body
		case "iCCP":
			// profile name, a zero byte, the compression method, then the zlib stream
			i := bytes.IndexByte(body, 0)
			if i < 0 || i+2 > len(body) {
				p.iccUnread = true
				break
			}
			zr, err := zlib.NewReader(bytes.NewReader(body[i+2:]))
			if err != nil {
				p.iccUnread = true
				break
			}
			icc, err := io.ReadAll(io.LimitReader(zr, maxICCBytes+1))
			if err != nil || len(icc) == 0 || len(icc) > maxICCBytes {
				p.iccUnread = true
				break
			}
			p.icc = icc
		case "IDAT":
			p.chunksParsed = true
			return p
		}
		pos += 12 + n
	}
	return p
}

// exifOrientation reads the orientation from EXIF data (a TIFF structure); 1 when there is none.
func exifOrientation(t []byte) int {
	tags, ok := tiffTags(t)
	if !ok {
		return 0 // unreadable: the caller keeps the original
	}
	if v, ok := tags[274]; ok {
		return int(v.first)
	}
	return 1
}

func convertPNG(data []byte) Result {
	p := scanPNG(data)
	switch {
	case !p.chunksParsed, p.w <= 0, p.h <= 0, p.w*p.h > maxPixels:
		return Result{}
	case p.animated:
		// image/png reads only the first picture of an animated PNG, so a comparison would not
		// notice the lost animation; browsers show APNG as it is
		return Result{Animated: true}
	case p.depth == 16, p.cicp, p.iccUnread:
		return Result{}
	case p.gamma && !p.srgb && p.icc == nil:
		// The colours depend on gAMA or cHRM alone. WebP has no such field, and a browser
		// that honours it would show the converted picture differently.
		return Result{}
	}
	if p.exif != nil && exifOrientation(p.exif) != 1 {
		return Result{}
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return Result{}
	}
	n, ok := toNRGBA(img)
	if !ok || halfTransparent(n) {
		return Result{}
	}
	return webpResult(encodeStill(n, p.icc, p.exif), len(data), false)
}

// halfTransparent tells whether a picture has pixels that are neither opaque nor fully
// transparent. Such a PNG stays a PNG: Chrome shows the half-transparent pixels of a lossless
// WebP file one level of 255 off from the same samples in a PNG file (measured on 2026-11-11 in
// Chrome with three graphics settings; WebKit shows both the same). The samples in the file
// are equal, the browser rounds differently when it multiplies colour by alpha for the two
// formats.
func halfTransparent(m *image.NRGBA) bool {
	for i := 3; i < len(m.Pix); i += 4 {
		if a := m.Pix[i]; a != 0 && a != 255 {
			return true
		}
	}
	return false
}

func convertBMP(data []byte) Result {
	// With an alpha channel, readers disagree about what a BMP shows; those stay as they are.
	if len(data) < 30 {
		return Result{}
	}
	if hdr := le32(data[14:]); hdr >= 40 && (data[28] == 32 || data[29] != 0) {
		return Result{}
	}
	cfg, err := bmp.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width*cfg.Height > maxPixels {
		return Result{}
	}
	img, err := bmp.Decode(bytes.NewReader(data))
	if err != nil {
		return Result{}
	}
	n, ok := toNRGBA(img)
	if !ok {
		return Result{}
	}
	return webpResult(encodeStill(n, nil, nil), len(data), false)
}

// gifAnimated tells whether a GIF has more than one frame, without decoding the frames fully.
func gifAnimated(data []byte) bool {
	g, err := gif.DecodeAll(bytes.NewReader(data))
	return err == nil && len(g.Image) > 1
}

// gifCanvases puts the frames of a GIF together the way a browser shows them: every state of
// the whole picture, with transparent pixels as 0,0,0,0.
func gifCanvases(g *gif.GIF) ([]animFrame, bool) {
	w, h := g.Config.Width, g.Config.Height
	if w <= 0 || h <= 0 || w*h > maxPixels || len(g.Image) > maxAnimFrames || len(g.Image)*w*h*4 > maxAnimBytes {
		return nil, false
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, w, h))
	var out []animFrame
	for i, f := range g.Image {
		var saved []byte
		disposal := byte(0)
		if i < len(g.Disposal) {
			disposal = g.Disposal[i]
		}
		if disposal == gif.DisposalPrevious {
			saved = append([]byte(nil), canvas.Pix...)
		}
		r := f.Bounds().Intersect(canvas.Bounds())
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				idx := f.Pix[(y-f.Rect.Min.Y)*f.Stride+(x-f.Rect.Min.X)]
				if int(idx) >= len(f.Palette) {
					continue
				}
				cr, cg, cb, ca := f.Palette[idx].RGBA()
				if ca == 0 {
					continue // transparent: the frame below shows through
				}
				if ca != 0xffff {
					return nil, false
				}
				p := canvas.Pix[y*canvas.Stride+x*4:]
				p[0], p[1], p[2], p[3] = uint8(cr>>8), uint8(cg>>8), uint8(cb>>8), 255
			}
		}
		cp := image.NewNRGBA(canvas.Bounds())
		copy(cp.Pix, canvas.Pix)
		ms := 0
		if i < len(g.Delay) {
			ms = g.Delay[i] * 10
		}
		out = append(out, animFrame{cp, ms})
		switch disposal {
		case gif.DisposalBackground:
			clearRect(canvas, r)
		case gif.DisposalPrevious:
			copy(canvas.Pix, saved)
		}
	}
	return out, true
}

// sameAnimation compares two animations as what is seen: runs of the same canvas count as one
// state with the summed time, because an encoder may merge or split equal frames.
func sameAnimation(a, b []animFrame) bool {
	merge := func(in []animFrame) []animFrame {
		var out []animFrame
		for _, f := range in {
			if n := len(out); n > 0 && bytes.Equal(out[n-1].img.Pix, f.img.Pix) {
				out[n-1].ms += f.ms
				continue
			}
			out = append(out, f)
		}
		return out
	}
	a, b = merge(a), merge(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ms != b[i].ms || !a[i].img.Bounds().Eq(b[i].img.Bounds()) || !bytes.Equal(a[i].img.Pix, b[i].img.Pix) {
			return false
		}
	}
	return true
}

func convertGIF(data []byte) Result {
	cfg, err := gif.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxPixels {
		return Result{}
	}
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil || len(g.Image) == 0 {
		return Result{}
	}
	frames, ok := gifCanvases(g)
	if !ok {
		return Result{Animated: len(g.Image) > 1}
	}
	if len(g.Image) == 1 {
		return webpResult(encodeStill(frames[0].img, nil, nil), len(data), false)
	}
	keep := Result{Animated: true}
	// Browsers play a GIF frame of less than 20 ms slower than written; whether they do the
	// same for WebP is not the same in every browser, so such a GIF stays a GIF.
	for _, f := range frames {
		if f.ms < 20 {
			return keep
		}
	}
	// A GIF without a loop block plays once, one with the count 0 plays forever. Other counts
	// are read differently by browsers (n or n+1 runs) and stay as they are.
	var loop int
	switch g.LoopCount {
	case 0:
		loop = 0
	case -1:
		loop = 1
	default:
		return keep
	}
	anim := &gwebp.WEBP{LoopCount: loop}
	for _, f := range frames {
		anim.Image = append(anim.Image, f.img)
		anim.Delay = append(anim.Delay, f.ms)
	}
	var b bytes.Buffer
	if err := gwebp.EncodeAll(&b, anim, gwebp.Options{Lossless: true, Quality: 100, Method: 6, Exact: true}); err != nil {
		return keep
	}
	back, backLoop, err := decodeAnim(b.Bytes())
	if err != nil || backLoop != loop || !sameAnimation(frames, back) || b.Len() >= len(data) {
		return keep
	}
	return Result{Data: b.Bytes(), Mime: mimeWebP, Ext: "webp", Changed: true, Animated: true}
}
