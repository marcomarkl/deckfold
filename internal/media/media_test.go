package media

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"testing"

	gwebp "github.com/gen2brain/webp"
	"github.com/tdewolff/font"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
	xwebp "golang.org/x/image/webp"

	"deckfold/internal/pptx/fonts"
)

// TestNoSystemLibrary fails when the WebP package encodes with a libwebp of the machine: the
// output would then depend on the machine. Build and test with -tags nodynamic.
func TestNoSystemLibrary(t *testing.T) {
	if gwebp.Dynamic() == nil {
		t.Fatal("a libwebp installed on this machine is in use; build and test with -tags nodynamic")
	}
	if New().WebPOff() != nil {
		t.Fatal("pictures are not converted in this build")
	}
}

// picture is a test picture that a lossless coder shrinks well: flat areas, a gradient, and
// fully transparent pixels that keep a colour. It has no half-transparent pixels, which would
// keep a PNG from being converted.
func picture(w, h int) *image.NRGBA {
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.NRGBA{uint8(x * 255 / w), uint8((x / 16 * 40) % 256), uint8(y / 8 * 8), 255}
			if x < w/4 && y < h/4 {
				c = color.NRGBA{200, 30, 30, 0} // invisible, with a colour
			}
			m.SetNRGBA(x, y, c)
		}
	}
	return m
}

func pngOf(t *testing.T, m image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, m); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// withChunk puts a chunk right after the header chunk of a PNG file.
func withChunk(data []byte, typ string, body []byte) []byte {
	var c bytes.Buffer
	binary.Write(&c, binary.BigEndian, uint32(len(body)))
	c.WriteString(typ)
	c.Write(body)
	binary.Write(&c, binary.BigEndian, crc32.ChecksumIEEE(append([]byte(typ), body...)))
	out := append([]byte(nil), data[:33]...)
	out = append(out, c.Bytes()...)
	return append(out, data[33:]...)
}

func exifWith(orientation uint16) []byte {
	b := []byte{'M', 'M', 0, 42, 0, 0, 0, 8, 0, 1, 0x01, 0x12, 0, 3, 0, 0, 0, 1}
	b = append(b, byte(orientation>>8), byte(orientation), 0, 0, 0, 0, 0, 0)
	return b
}

// samePixels reads a WebP file with the decoder of golang.org/x/image and compares every sample.
func samePixels(t *testing.T, webp []byte, want *image.NRGBA) {
	t.Helper()
	got, err := xwebp.Decode(bytes.NewReader(webp))
	if err != nil {
		t.Fatalf("the result cannot be read: %v", err)
	}
	n, ok := got.(*image.NRGBA)
	if !ok || !n.Bounds().Eq(want.Bounds()) {
		t.Fatalf("the result is %T %v, want NRGBA %v", got, got.Bounds(), want.Bounds())
	}
	if !bytes.Equal(n.Pix, want.Pix) {
		t.Fatal("the result has other samples than the source")
	}
}

func TestPNG(t *testing.T) {
	src := picture(160, 120)
	data := pngOf(t, src)
	r := New().Optimize(data)
	if !r.Changed || r.Ext != "webp" || r.Mime != "image/webp" || len(r.Data) >= len(data) {
		t.Fatalf("PNG not converted: changed=%v ext=%q %d -> %d bytes", r.Changed, r.Ext, len(data), len(r.Data))
	}
	samePixels(t, r.Data, src) // the invisible colours too
	if chunks, err := parseRIFF(r.Data); err != nil || len(chunks) != 1 || chunks[0].id != "VP8L" {
		t.Errorf("a picture without profile should be a plain lossless WebP file, got %v %v", chunks, err)
	}
}

// TestHalfTransparentPNG: a PNG with pixels between opaque and transparent stays a PNG, because
// Chrome shows such pixels of a WebP file one level off.
func TestHalfTransparentPNG(t *testing.T) {
	m := picture(160, 120)
	m.SetNRGBA(100, 100, color.NRGBA{10, 200, 30, 128})
	data := pngOf(t, m)
	if r := New().Optimize(data); r.Changed || r.Note != "" || !bytes.Equal(r.Data, data) {
		t.Errorf("changed=%v note=%q: a half-transparent PNG must stay as it is, without a warning", r.Changed, r.Note)
	}
	if !halfTransparent(m) || halfTransparent(picture(160, 120)) {
		t.Error("halfTransparent reads the test pictures wrongly")
	}
}

func TestPNGKinds(t *testing.T) {
	pal := image.NewPaletted(image.Rect(0, 0, 200, 100), color.Palette{color.NRGBA{0, 0, 0, 0}, color.NRGBA{250, 10, 10, 255}, color.NRGBA{10, 10, 250, 255}})
	for i := range pal.Pix {
		pal.Pix[i] = uint8((i / 37) % 3)
	}
	gray := image.NewGray(image.Rect(0, 0, 200, 100))
	for i := range gray.Pix {
		gray.Pix[i] = uint8(i / 200 * 2)
	}
	deep := image.NewNRGBA64(image.Rect(0, 0, 64, 64))
	for _, c := range []struct {
		name    string
		img     image.Image
		changed bool
	}{{"palette with transparency", pal, true}, {"grey", gray, true}, {"16 bits per channel", deep, false}} {
		data := pngOf(t, c.img)
		r := New().Optimize(data)
		if r.Changed != c.changed {
			t.Errorf("%s: changed=%v, want %v", c.name, r.Changed, c.changed)
			continue
		}
		if !r.Changed {
			if !bytes.Equal(r.Data, data) {
				t.Errorf("%s: an unchanged result must carry the source", c.name)
			}
			continue
		}
		want, _ := toNRGBA(mustPNG(t, data))
		samePixels(t, r.Data, want)
	}
}

func mustPNG(t *testing.T, data []byte) image.Image {
	t.Helper()
	m, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestPNGColourAndOrientation(t *testing.T) {
	src := picture(160, 120)
	plain := pngOf(t, src)
	profile := bytes.Repeat([]byte("deckfold test profile "), 20)
	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	zw.Write(profile)
	zw.Close()
	iccp := append([]byte("test\x00\x00"), z.Bytes()...)
	gama := []byte{0, 0, 0xb1, 0x8f}

	// a colour profile travels into the WebP file
	r := New().Optimize(withChunk(plain, "iCCP", iccp))
	if !r.Changed {
		t.Fatal("PNG with a colour profile not converted")
	}
	samePixels(t, r.Data, src)
	chunks, err := parseRIFF(r.Data)
	if err != nil || len(chunks) != 3 || chunks[0].id != "VP8X" || chunks[1].id != "ICCP" || chunks[2].id != "VP8L" {
		t.Fatalf("chunks %v, %v; want VP8X, ICCP, VP8L", chunks, err)
	}
	if !bytes.Equal(chunks[1].data, profile) {
		t.Error("the profile in the WebP file is not the one of the PNG")
	}
	if chunks[0].data[0]&(vp8xICC|vp8xAlpha) != vp8xICC|vp8xAlpha {
		t.Errorf("VP8X flags %08b: the profile and alpha bits must be set", chunks[0].data[0])
	}

	// colours that hang on a gamma chunk alone have no place in WebP: the PNG stays
	if r := New().Optimize(withChunk(plain, "gAMA", gama)); r.Changed {
		t.Error("PNG with gAMA and without sRGB or a profile was converted")
	}
	// with an sRGB chunk the gamma chunk is only a fallback
	if r := New().Optimize(withChunk(withChunk(plain, "gAMA", gama), "sRGB", []byte{0})); !r.Changed {
		t.Error("PNG with sRGB and gAMA not converted")
	}
	// an animated PNG stays, and is reported as animated
	if r := New().Optimize(withChunk(plain, "acTL", []byte{0, 0, 0, 2, 0, 0, 0, 0})); r.Changed || !r.Animated {
		t.Errorf("animated PNG: changed=%v animated=%v, want false, true", r.Changed, r.Animated)
	}
	// a turned picture stays; an upright one keeps its EXIF data
	if r := New().Optimize(withChunk(plain, "eXIf", exifWith(6))); r.Changed {
		t.Error("PNG with orientation 6 was converted")
	}
	r = New().Optimize(withChunk(plain, "eXIf", exifWith(1)))
	if !r.Changed {
		t.Fatal("PNG with orientation 1 not converted")
	}
	chunks, _ = parseRIFF(r.Data)
	if n := len(chunks); n != 3 || chunks[n-1].id != "EXIF" || !bytes.Equal(chunks[n-1].data, exifWith(1)) {
		t.Errorf("chunks %v; want the EXIF data of the PNG at the end", chunks)
	}
}

// gifOf builds a GIF of coloured frames on a 240 x 160 canvas; the frames after the first cover
// only a part and have a transparent corner.
func gifOf(t *testing.T, delays []int, loop int, disposal byte) []byte {
	t.Helper()
	pal := color.Palette{color.RGBA{0, 0, 0, 0}, color.RGBA{220, 40, 40, 255}, color.RGBA{40, 180, 60, 255}, color.RGBA{40, 60, 220, 255}}
	g := &gif.GIF{LoopCount: loop, Config: image.Config{Width: 240, Height: 160, ColorModel: pal}}
	for i, d := range delays {
		r := image.Rect(0, 0, 240, 160)
		if i > 0 {
			r = image.Rect(40*i, 20, 120+40*i, 120)
		}
		f := image.NewPaletted(r, pal)
		for j := range f.Pix {
			f.Pix[j] = uint8(1 + (i+j/7)%3)
		}
		if i > 0 {
			f.Pix[0] = 0
		}
		g.Image = append(g.Image, f)
		g.Delay = append(g.Delay, d)
		g.Disposal = append(g.Disposal, disposal)
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, g); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestAnimatedGIF(t *testing.T) {
	for _, disposal := range []byte{gif.DisposalNone, gif.DisposalBackground, gif.DisposalPrevious} {
		for _, loop := range []int{0, -1} {
			data := gifOf(t, []int{10, 20, 30, 5}, loop, disposal)
			r := New().Optimize(data)
			if !r.Changed || !r.Animated || r.Ext != "webp" {
				t.Fatalf("disposal %d loop %d: changed=%v animated=%v", disposal, loop, r.Changed, r.Animated)
			}
			// a second opinion: the animation decoder of libwebp, which is not the reader
			// deckfold checks its result with
			back, err := gwebp.DecodeAll(bytes.NewReader(r.Data))
			if err != nil {
				t.Fatal(err)
			}
			g, _ := gif.DecodeAll(bytes.NewReader(data))
			want, _ := gifCanvases(g)
			var got []animFrame
			for i, im := range back.Image {
				n := image.NewNRGBA(im.Bounds())
				for y := 0; y < n.Bounds().Dy(); y++ {
					for x := 0; x < n.Bounds().Dx(); x++ {
						c := color.NRGBAModel.Convert(im.At(x, y)).(color.NRGBA)
						if c.A == 0 {
							c = color.NRGBA{}
						}
						n.SetNRGBA(x, y, c)
					}
				}
				got = append(got, animFrame{n, back.Delay[i]})
			}
			if !sameAnimation(want, got) {
				t.Errorf("disposal %d: libwebp shows another animation than the GIF (%d and %d states)", disposal, len(want), len(got))
			}
			wantLoop := 0
			if loop == -1 {
				wantLoop = 1
			}
			// the decoder of the WebP package does not report the loop count; it stands in
			// the ANIM chunk, after the four bytes of the background colour
			chunks, _ := parseRIFF(r.Data)
			gotLoop := -1
			for _, c := range chunks {
				if c.id == "ANIM" && len(c.data) >= 6 {
					gotLoop = int(binary.LittleEndian.Uint16(c.data[4:]))
				}
			}
			if gotLoop != wantLoop {
				t.Errorf("loop count %d, want %d", gotLoop, wantLoop)
			}
		}
	}
	// a frame of 10 ms is played slower than written by browsers; such a GIF stays
	if r := New().Optimize(gifOf(t, []int{10, 1, 10}, 0, gif.DisposalNone)); r.Changed || !r.Animated {
		t.Errorf("GIF with a 10 ms frame: changed=%v animated=%v, want false, true", r.Changed, r.Animated)
	}
	// a loop count browsers read in two ways stays as well
	if r := New().Optimize(gifOf(t, []int{10, 10}, 3, gif.DisposalNone)); r.Changed || !r.Animated {
		t.Errorf("GIF that loops three times: changed=%v animated=%v, want false, true", r.Changed, r.Animated)
	}
}

func TestStillGIFAndBMP(t *testing.T) {
	big := image.NewPaletted(image.Rect(0, 0, 300, 200), color.Palette{color.RGBA{255, 255, 255, 255}, color.RGBA{20, 20, 20, 255}})
	for i := range big.Pix {
		big.Pix[i] = uint8((i / 300 / 10) % 2)
	}
	var g bytes.Buffer
	gif.Encode(&g, big, nil)
	r := New().Optimize(g.Bytes())
	if !r.Changed || r.Animated {
		t.Fatalf("still GIF: changed=%v animated=%v, want true, false", r.Changed, r.Animated)
	}
	want, _ := toNRGBA(big)
	samePixels(t, r.Data, want)

	rgb := image.NewRGBA(image.Rect(0, 0, 120, 80))
	for i := 0; i < len(rgb.Pix); i += 4 {
		rgb.Pix[i], rgb.Pix[i+1], rgb.Pix[i+2], rgb.Pix[i+3] = uint8(i/4%120), 90, uint8(i/4/120*3), 255
	}
	var b bytes.Buffer
	if err := bmp.Encode(&b, rgb); err != nil {
		t.Fatal(err)
	}
	r = New().Optimize(b.Bytes())
	if !r.Changed {
		t.Fatal("24-bit BMP not converted")
	}
	want, _ = toNRGBA(rgb)
	samePixels(t, r.Data, want)
	// 32 bits per pixel: readers disagree about the fourth byte
	alpha := append([]byte(nil), b.Bytes()...)
	alpha[28] = 32
	if r := New().Optimize(alpha); r.Changed {
		t.Error("32-bit BMP was converted")
	}
}

func TestTIFF(t *testing.T) {
	src := picture(48, 32)
	var b bytes.Buffer
	if err := tiff.Encode(&b, src, nil); err != nil {
		t.Fatal(err)
	}
	r := New().Optimize(b.Bytes())
	if !r.Changed || r.Note != "" {
		t.Fatalf("TIFF not converted: %+v", r.Note)
	}
	samePixels(t, r.Data, src)
	// a TIFF deckfold cannot read stays, with a warning
	broken := append([]byte(nil), b.Bytes()[:60]...)
	if r := New().Optimize(broken); r.Changed || r.Note != NoteTIFF {
		t.Errorf("unreadable TIFF: changed=%v note=%q", r.Changed, r.Note)
	}
}

func TestOrient(t *testing.T) {
	// a 2 x 3 picture whose pixels are numbered 1..6 row by row, in the red channel
	s := image.NewNRGBA(image.Rect(0, 0, 2, 3))
	for i := 0; i < 6; i++ {
		s.Pix[i*4] = uint8(i + 1)
	}
	want := map[int][]uint8{
		1: {1, 2, 3, 4, 5, 6},
		2: {2, 1, 4, 3, 6, 5}, // mirrored left to right
		3: {6, 5, 4, 3, 2, 1}, // turned by 180 degrees
		4: {5, 6, 3, 4, 1, 2}, // mirrored top to bottom
		5: {1, 3, 5, 2, 4, 6}, // mirrored along the main diagonal
		6: {5, 3, 1, 6, 4, 2}, // turned by 90 degrees clockwise
		7: {6, 4, 2, 5, 3, 1}, // mirrored along the other diagonal
		8: {2, 4, 6, 1, 3, 5}, // turned by 90 degrees counter-clockwise
	}
	for o, w := range want {
		d := orient(s, o)
		if o >= 5 && (d.Bounds().Dx() != 3 || d.Bounds().Dy() != 2) {
			t.Errorf("orientation %d: size %v, want 3 x 2", o, d.Bounds())
			continue
		}
		var got []uint8
		for i := 0; i < 6; i++ {
			got = append(got, d.Pix[i*4])
		}
		if !bytes.Equal(got, w) {
			t.Errorf("orientation %d: %v, want %v", o, got, w)
		}
	}
	if o := exifOrientation(exifWith(6)); o != 6 {
		t.Errorf("orientation read from EXIF data: %d, want 6", o)
	}
}

// woffOf packs a font as WOFF 1.0, each table compressed by itself.
func woffOf(sfnt []byte) []byte {
	flavor, tables, _ := parseSFNT(sfnt)
	var dir, body bytes.Buffer
	off := 44 + 20*len(tables)
	for i, tb := range tables {
		sum := be32(sfnt[12+16*i+4:]) // the checksum of the table, from the directory of the font
		var z bytes.Buffer
		zw := zlib.NewWriter(&z)
		zw.Write(tb.data)
		zw.Close()
		comp := z.Bytes()
		if len(comp) >= len(tb.data) {
			comp = tb.data
		}
		dir.WriteString(tb.tag)
		binary.Write(&dir, binary.BigEndian, []uint32{uint32(off), uint32(len(comp)), uint32(len(tb.data)), sum})
		body.Write(comp)
		for body.Len()%4 != 0 {
			body.WriteByte(0)
		}
		off = 44 + 20*len(tables) + body.Len()
	}
	var out bytes.Buffer
	out.WriteString("wOFF")
	binary.Write(&out, binary.BigEndian, flavor)
	binary.Write(&out, binary.BigEndian, uint32(44+dir.Len()+body.Len()))
	binary.Write(&out, binary.BigEndian, []uint16{uint16(len(tables)), 0})
	binary.Write(&out, binary.BigEndian, uint32(len(sfnt)))
	binary.Write(&out, binary.BigEndian, []uint16{1, 0})
	binary.Write(&out, binary.BigEndian, []uint32{0, 0, 0, 0, 0})
	out.Write(dir.Bytes())
	out.Write(body.Bytes())
	return out.Bytes()
}

func TestFont(t *testing.T) {
	ttf := fonts.BundledFile("Carlito", false, false)
	if len(ttf) == 0 {
		t.Fatal("no bundled font")
	}
	_, want, ok := parseSFNT(ttf)
	if !ok {
		t.Fatal("the bundled font cannot be read")
	}
	for name, data := range map[string][]byte{"TrueType": ttf, "WOFF": woffOf(ttf)} {
		r := New().Optimize(data)
		if !r.Changed || r.Ext != "woff2" || r.Mime != "font/woff2" || len(r.Data) >= len(data) {
			t.Fatalf("%s: not converted: changed=%v %d -> %d bytes", name, r.Changed, len(data), len(r.Data))
		}
		back, err := font.ParseWOFF2(r.Data)
		if err != nil {
			t.Fatalf("%s: the WOFF2 file cannot be read: %v", name, err)
		}
		_, got, ok := parseSFNT(back)
		if !ok || !sameTables(want, got) {
			t.Errorf("%s: the tables of the WOFF2 file are not the ones of the font", name)
		}
	}
	// a font collection is not one font and stays
	if r := New().Optimize(append([]byte("ttcf"), ttf[4:]...)); r.Changed {
		t.Error("a font collection was converted")
	}
}

// TestSameBytesEverywhere pins the results for fixed inputs. The test runs on every platform of
// the CI, so it shows that a result does not depend on the machine; it changes only when the
// WebP or the Brotli package changes.
func TestSameBytesEverywhere(t *testing.T) {
	sum := func(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:8]) }
	pic := New().Optimize(pngOf(t, picture(160, 120)))
	anim := New().Optimize(gifOf(t, []int{10, 20, 30, 5}, 0, gif.DisposalNone))
	fnt := New().Optimize(fonts.BundledFile("Carlito", false, false))
	got := sum(pic.Data) + " " + sum(anim.Data) + " " + sum(fnt.Data)
	const want = "7a6679f0c191a371 cfd834770aaab202 bd33c83e35864ec7"
	if got != want {
		t.Errorf("results %s, want %s", got, want)
	}
	if again := New().Optimize(pngOf(t, picture(160, 120))); !bytes.Equal(again.Data, pic.Data) {
		t.Error("converting the same picture twice gives different bytes")
	}
}

func TestDamagedData(t *testing.T) {
	good := pngOf(t, picture(160, 120))
	huge := append([]byte(nil), good...)
	binary.BigEndian.PutUint32(huge[16:], 60000) // width; the checksum of the header is now wrong, too
	binary.BigEndian.PutUint32(huge[20:], 60000)
	for name, data := range map[string][]byte{
		"placeholder bytes":    []byte("\x89PNG"),
		"cut off":              good[:len(good)/2],
		"claims 60000 x 60000": huge,
		"GIF header only":      []byte("GIF89a"),
		"TIFF header only":     []byte("II*\x00\x08\x00\x00\x00"),
		"font header only":     {0, 1, 0, 0, 0, 200, 0, 0, 0, 0, 0, 0},
		"WOFF header only":     []byte("wOFF\x00\x01\x00\x00"),
		"empty":                nil,
	} {
		r := New().Optimize(data)
		if r.Changed || !bytes.Equal(r.Data, data) {
			t.Errorf("%s: must stay unchanged", name)
		}
		if name != "TIFF header only" && r.Note != "" {
			t.Errorf("%s: unexpected warning %q", name, r.Note)
		}
	}
}

func TestOnceAndStats(t *testing.T) {
	a, b := pngOf(t, picture(160, 120)), pngOf(t, picture(200, 100))
	o := New()
	o.Prepare(3, func(i int) []byte { return [][]byte{a, b, nil}[i] })
	if s := o.Stats(); s.Pictures != 0 {
		t.Errorf("prepared pictures count only once they are used: %+v", s)
	}
	r1, r2 := o.Optimize(a), o.Optimize(a)
	if !r1.Changed || !bytes.Equal(r1.Data, r2.Data) {
		t.Fatal("the same picture must give the same result")
	}
	if direct := New().Optimize(a); !bytes.Equal(direct.Data, r1.Data) {
		t.Error("a prepared result differs from a direct one")
	}
	s := o.Stats()
	if s.Pictures != 1 || s.Before != int64(len(a)) || s.After != int64(len(r1.Data)) {
		t.Errorf("stats %+v, want one picture, %d -> %d bytes", s, len(a), len(r1.Data))
	}
	var none *Optimizer
	if r := none.Optimize(a); r.Changed || !bytes.Equal(r.Data, a) {
		t.Error("without an optimizer every asset stays")
	}
}
