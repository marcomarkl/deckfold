package mtx

import (
	"errors"
	"fmt"
)

// LZCOMP decoder, written from chapter 3 and appendix C of the W3C submission
// "MicroType Express (MTX) Font Format" (5 March 2008).

var errEOF = errors.New("mtx: compressed data ends early")

// bitReader reads bits from the most significant bit of each byte down (BITIO.C).
type bitReader struct {
	data  []byte
	pos   int
	buf   uint
	count uint
	err   error
}

func (r *bitReader) bit() uint {
	if r.count == 0 {
		if r.pos >= len(r.data) {
			r.err = errEOF
			return 0
		}
		r.buf = uint(r.data[r.pos])
		r.pos++
		r.count = 8
	}
	r.count--
	return (r.buf >> r.count) & 1
}

func (r *bitReader) value(n int) uint {
	var v uint
	for i := 0; i < n; i++ {
		v = v<<1 | r.bit()
	}
	return v
}

// ahuff is the adaptive Huffman tree of AHUFF.C. Node 1 is the root; the leaves for the
// symbols 0..range-1 start at index range. Nodes are kept in decreasing order of weight.
type ahuff struct {
	up, left, right, code []int
	weight                []int64
	symbolIndex           []int
	r                     *bitReader
}

func newAhuff(r *bitReader, n int) *ahuff {
	t := &ahuff{
		up: make([]int, 2*n), left: make([]int, 2*n), right: make([]int, 2*n),
		code: make([]int, 2*n), weight: make([]int64, 2*n), symbolIndex: make([]int, n), r: r,
	}
	for i := 2; i < 2*n; i++ {
		t.up[i] = i / 2
		t.weight[i] = 1
	}
	for i := 1; i < n; i++ {
		t.left[i] = 2 * i
		t.right[i] = 2*i + 1
	}
	for i := 0; i < n; i++ {
		t.code[i] = -1
		t.code[n+i] = i
		t.left[n+i] = -1
		t.right[n+i] = -1
		t.symbolIndex[i] = n + i
	}
	t.initWeight(1)
	// Index 0 is not part of the tree; a weight above every node stops the search in update.
	t.weight[0] = 1 << 62
	if n > 256 && n < 512 {
		t.update(t.symbolIndex[256])
		t.update(t.symbolIndex[257])
		for i := 0; i < 12; i++ { // DUP2
			t.update(t.symbolIndex[n-3])
		}
		for i := 0; i < 6; i++ { // DUP4
			t.update(t.symbolIndex[n-2])
		}
	} else {
		for j := 0; j < 2; j++ {
			for i := 0; i < n; i++ {
				t.update(t.symbolIndex[i])
			}
		}
	}
	return t
}

func (t *ahuff) initWeight(a int) int64 {
	if t.code[a] < 0 {
		t.weight[a] = t.initWeight(t.left[a]) + t.initWeight(t.right[a])
	}
	return t.weight[a]
}

func (t *ahuff) swap(a, b int) {
	upa, upb := t.up[a], t.up[b]
	t.left[a], t.left[b] = t.left[b], t.left[a]
	t.right[a], t.right[b] = t.right[b], t.right[a]
	t.code[a], t.code[b] = t.code[b], t.code[a]
	t.weight[a], t.weight[b] = t.weight[b], t.weight[a]
	t.up[a], t.up[b] = upa, upb
	for _, n := range [2]int{a, b} {
		if c := t.code[n]; c < 0 {
			t.up[t.left[n]] = n
			t.up[t.right[n]] = n
		} else {
			t.symbolIndex[c] = n
		}
	}
}

// update adds one to the weight of node a and its parents and keeps the sibling property.
func (t *ahuff) update(a int) {
	for a != 1 {
		w := t.weight[a]
		b := a - 1
		if t.weight[b] == w {
			for t.weight[b] == w {
				b--
			}
			b++
			if b > 1 {
				t.swap(a, b)
				a = b
			}
		}
		t.weight[a] = w + 1
		a = t.up[a]
	}
	t.weight[1]++
}

func (t *ahuff) read() int {
	a := 1
	for {
		if t.r.bit() == 1 {
			a = t.right[a]
		} else {
			a = t.left[a]
		}
		if t.r.err != nil {
			return 0
		}
		if t.code[a] >= 0 {
			break
		}
	}
	s := t.code[a] // update may move the leaf
	t.update(a)
	return s
}

const (
	preloadSize  = 2*32*96 + 4*256
	max2ByteDist = 512
	lenWidth     = 3
	distWidth    = 3
	// maxOutput bounds the 24-bit length field; it is far above any font block.
	maxOutput = 1 << 24
)

// unpack decodes one LZCOMP block. version is the MTX version byte; all versions except 1 start
// with a bit that switches on the run-length decoder.
func unpack(data []byte, version byte) ([]byte, error) {
	r := &bitReader{data: data}
	runLength := false
	if version != 1 {
		runLength = r.bit() == 1
	}
	dist := newAhuff(r, 1<<distWidth)
	length := newAhuff(r, 1<<lenWidth)
	outLen := int(r.value(24))
	if r.err != nil {
		return nil, r.err
	}
	numDistRanges := 1
	for 1+(1<<(distWidth*numDistRanges))-1 < outLen {
		numDistRanges++
	}
	dup2 := 256 + (1<<lenWidth)*numDistRanges
	dup4, dup6 := dup2+1, dup2+2
	sym := newAhuff(r, dup6+1)

	buf := make([]byte, preloadSize+outLen)
	i := 0
	for k := 0; k < 32; k++ {
		for j := 0; j < 96; j++ {
			buf[i], buf[i+1] = byte(k), byte(j)
			i += 2
		}
	}
	for j := 0; i < preloadSize; j++ {
		buf[i], buf[i+1], buf[i+2], buf[i+3] = byte(j), byte(j), byte(j), byte(j)
		i += 4
	}

	pos := preloadSize
	end := preloadSize + outLen
	for pos < end {
		s := sym.read()
		if r.err != nil {
			return nil, r.err
		}
		switch {
		case s < 256:
			buf[pos] = byte(s)
			pos++
		case s == dup2 || s == dup4 || s == dup6:
			buf[pos] = buf[pos-2*(s-dup2+1)]
			pos++
		default:
			ranges := (s-256)/(1<<lenWidth) + 1
			if ranges > numDistRanges {
				return nil, fmt.Errorf("mtx: copy item with %d distance ranges", ranges)
			}
			n := decodeLength(length, s)
			d := 0
			for k := 0; k < ranges; k++ {
				d = d<<distWidth | dist.read()
			}
			d++
			if r.err != nil {
				return nil, r.err
			}
			if d >= max2ByteDist {
				n++
			}
			start := pos - d - n + 1
			if start < 0 || pos+n > end {
				return nil, errors.New("mtx: copy item outside the data")
			}
			for k := 0; k < n; k++ {
				buf[pos] = buf[start+k]
				pos++
			}
		}
	}
	out := buf[preloadSize:]
	if runLength {
		return unRunLength(out)
	}
	return out, nil
}

// decodeLength reads the copy length: two bits from the first symbol and from each following
// symbol of the length tree, until a symbol without the stop bit.
func decodeLength(t *ahuff, symbol int) int {
	const mask = 1 << (lenWidth - 1)
	bits := (symbol - 256) % (1 << lenWidth)
	v := 0
	for {
		v = v<<(lenWidth-1) | bits&^mask
		if bits&mask == 0 || t.r.err != nil {
			break
		}
		bits = t.read()
	}
	return v + 2
}

// unRunLength undoes the run-length stage (RUNLENGTHCOMP_SaveBytes): the first byte is the
// escape byte; escape, 0 is a literal escape byte, escape, n, b is n copies of b.
func unRunLength(in []byte) ([]byte, error) {
	if len(in) == 0 {
		return in, nil
	}
	esc := in[0]
	out := make([]byte, 0, len(in))
	for i := 1; i < len(in); i++ {
		if in[i] != esc {
			out = append(out, in[i])
			continue
		}
		if i+1 >= len(in) {
			return nil, errEOF
		}
		n := int(in[i+1])
		if n == 0 {
			out = append(out, esc)
			i++
			continue
		}
		if i+2 >= len(in) {
			return nil, errEOF
		}
		if len(out)+n > maxOutput {
			return nil, errors.New("mtx: run-length data too large")
		}
		for k := 0; k < n; k++ {
			out = append(out, in[i+2])
		}
		i += 2
	}
	return out, nil
}
