package mtx

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"testing"
)

// The bit writer and Huffman encoder of encode.go are the mirror image of the decoder, so the
// tests can build LZCOMP data with copy items, which Encode never writes.

type item struct {
	lit       int // -1 for a copy item
	dup       int // 2, 4 or 6 for a one-byte copy
	dist, len int
}

// pack encodes items as one LZCOMP block that decodes to n bytes.
func packItems(items []item, n int, runLength bool) []byte {
	w := &bitWriter{}
	if runLength {
		w.bit(1)
	} else {
		w.bit(0)
	}
	dist := newAhuff(nil, 1<<distWidth)
	length := newAhuff(nil, 1<<lenWidth)
	w.value(uint(n), 24)
	ranges := 1
	for 1+(1<<(distWidth*ranges))-1 < n {
		ranges++
	}
	dup2 := 256 + (1<<lenWidth)*ranges
	sym := newAhuff(nil, dup2+3)
	for _, it := range items {
		switch {
		case it.dup != 0:
			sym.write(w, dup2+it.dup/2-1)
		case it.lit >= 0:
			sym.write(w, it.lit)
		default:
			d := it.dist - 1
			nr := 1
			for d>>(distWidth*nr) != 0 {
				nr++
			}
			l := it.len - 2
			if it.dist >= max2ByteDist {
				l--
			}
			var groups []int
			for {
				groups = append([]int{l & 3}, groups...)
				l >>= 2
				if l == 0 {
					break
				}
			}
			for i, g := range groups {
				if i < len(groups)-1 {
					g |= 4
				}
				if i == 0 {
					sym.write(w, 256+(nr-1)*8+g)
				} else {
					length.write(w, g)
				}
			}
			for i := nr - 1; i >= 0; i-- {
				dist.write(w, (d>>(distWidth*i))&7)
			}
		}
	}
	return w.flush()
}

func TestUnpackLiteralsAndCopies(t *testing.T) {
	items := []item{{lit: 'a'}, {lit: 'b'}, {lit: 'c'}, {lit: 'd'}, {lit: -1, dist: 1, len: 4}, {dup: 2}, {dup: 4}, {dup: 6}, {lit: 'z'}}
	// "abcd", a copy of the 4 bytes ending 1 byte back, then the one-byte copies from 2, 4
	// and 6 bytes back.
	want := []byte("abcdabcd")
	want = append(want, want[len(want)-2])
	want = append(want, want[len(want)-4])
	want = append(want, want[len(want)-6])
	want = append(want, 'z')
	got, err := unpack(packItems(items, len(want), false), 3)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("unpack = %q, %v; want %q", got, err, want)
	}
}

func TestUnpackLongDistanceAndPreload(t *testing.T) {
	// The data starts with a copy from the preloaded dictionary, whose last bytes are 255
	// four times (InitializeModel); the copy ends 1 byte back, at the end of the dictionary.
	want := []byte{255, 255, 255, 255}
	items := []item{{lit: -1, dist: 1, len: 4}}
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 3000; i++ {
		c := rng.Intn(256)
		want = append(want, byte(c))
		items = append(items, item{lit: c})
	}
	// A copy from 2000 bytes back: beyond 512 the minimum length is 3, so one is added.
	start := len(want) - 2000 - 10 + 1
	want = append(want, want[start:start+10]...)
	items = append(items, item{lit: -1, dist: 2000, len: 10})
	got, err := unpack(packItems(items, len(want), false), 3)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("unpack mismatch: %v (len %d, want %d)", err, len(got), len(want))
	}
}

func TestUnRunLength(t *testing.T) {
	in := []byte{0xAA, 1, 2, 0xAA, 0, 3, 0xAA, 5, 7, 4}
	want := []byte{1, 2, 0xAA, 3, 7, 7, 7, 7, 7, 4}
	got, err := unRunLength(in)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("unRunLength = %v, %v", got, err)
	}
	items := make([]item, len(in))
	for i, b := range in {
		items[i] = item{lit: int(b)}
	}
	got, err = unpack(packItems(items, len(in), true), 3)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("unpack with run length = %v, %v", got, err)
	}
}

func TestTruncatedData(t *testing.T) {
	items := []item{{lit: 1}, {lit: 2}, {lit: 3}}
	data := packItems(items, 300, false) // claims more output than it encodes
	if _, err := unpack(data, 3); err == nil {
		t.Fatal("truncated block decoded without error")
	}
	if _, err := Decode([]byte{3, 0, 0, 0, 0, 0, 5, 0, 0, 1}); err == nil {
		t.Fatal("invalid offsets accepted")
	}
}

func TestCvt(t *testing.T) {
	// Five values stored as differences to the previous one (section 5.2).
	in := []byte{0, 5,
		10,     // +10 -> 10
		248, 2, // +240 -> 250
		239, 5, // -5 -> 245
		240, 1, // -(238+1) -> 6
		238, 0x80, 0, // -32768 -> wraps
	}
	got, err := decodeCvt(in)
	if err != nil {
		t.Fatal(err)
	}
	want := []int16{10, 250, 245, 6, int16(6 - 32768)}
	for i, w := range want {
		if v := int16(binary.BigEndian.Uint16(got[2*i:])); v != w {
			t.Errorf("cvt[%d] = %d, want %d", i, v, w)
		}
	}
}

func TestPushData(t *testing.T) {
	// Glyph stream: pushCount 7, codeSize 2. Push stream: 5, 300 (255, 50), Hop3 with -2
	// (250, 2), 17 and a word.
	d := &glyphDecoder{
		g:    reader{b: []byte{7, 2}},
		push: reader{b: []byte{5, 255, 50, hop3Code, 250, 2, 17, 253, 0x12, 0x34}},
		code: reader{b: []byte{0x2B, 0x2C}},
	}
	prog, err := d.program()
	if err != nil {
		t.Fatal(err)
	}
	// values: 5, 300, 5, -2, 5, 17, 0x1234
	want := []byte{0xB0, 5, 0xB8, 0x01, 0x2C, 0xB0, 5, 0xB8, 0xFF, 0xFE, 0xB1, 5, 17, 0xB8, 0x12, 0x34, 0x2B, 0x2C}
	if !bytes.Equal(prog, want) {
		t.Fatalf("program = % x\nwant      % x", prog, want)
	}
	r := reader{b: []byte{254, 3, 255, 0, 253, 1, 2, 9}}
	for _, w := range []int{509, 253, 258, 9} {
		if v := r.u255(); v != w {
			t.Errorf("255USHORT = %d, want %d", v, w)
		}
	}
}

func TestSimpleGlyph(t *testing.T) {
	// One contour of three points, as in section 5.9: numContours, the end point (2), three flag
	// bytes and the coordinate bytes. Index 1 is (0, +b), index 11 is (+b, 0), and index 104 is
	// (-(257+b1), -(513+b2)) after the WOFF2 form of the same table: here (-306, -556).
	glyf := []byte{0, 1, 2, 1, 11, 0x80 | 104, 0, 100, 49, 43, 0, 0}
	out, loca, err := decodeGlyf(glyf, nil, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if binary.BigEndian.Uint32(loca[4:]) != uint32(len(out)) || len(out)%4 != 0 {
		t.Fatalf("loca %v for %d bytes", loca, len(out))
	}
	// numberOfContours 1, bbox (-206, -556, 100, 0), end point 2, no instructions,
	// flags: on/same/same, on/x short positive/y same, off/x word/y word.
	want := []byte{0, 1, 0xFF, 0x32, 0xFD, 0xD4, 0, 100, 0, 0, 0, 2, 0, 0,
		0x31, 0x33, 0x00, 100, 0xFE, 0xCE, 0xFD, 0xD4}
	if !bytes.Equal(out[:len(want)], want) {
		t.Fatalf("glyph = % x\nwant    % x", out[:len(want)], want)
	}
	for _, b := range out[len(want):] {
		if b != 0 {
			t.Fatalf("padding is not zero: % x", out)
		}
	}
}
