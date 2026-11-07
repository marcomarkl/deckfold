package patch

import (
	"bytes"
	"math/rand"
	"testing"
)

func TestApply(t *testing.T) {
	src := []byte("0123456789")
	out, err := Apply(src, []Patch{{Start: 2, End: 4, Repl: []byte("ab")}, {Start: 0, End: 0, Repl: []byte("<")}, {Start: 10, End: 10, Repl: []byte(">")}, {Start: 6, End: 7}})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "<01ab45789>" {
		t.Fatalf("%s", out)
	}
	if _, err := Apply(src, []Patch{{Start: 1, End: 5}, {Start: 3, End: 6}}); err == nil {
		t.Fatal("overlap not detected")
	}
}

// TestByteFidelity: all bytes outside the patched ranges stay the same.
func TestByteFidelity(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	src := make([]byte, 5000)
	r.Read(src)
	var ps []Patch
	pos := 0
	for pos < len(src)-50 {
		s := pos + r.Intn(40)
		e := s + r.Intn(10)
		ps = append(ps, Patch{Start: s, End: e, Repl: []byte("X")})
		pos = e + 1
	}
	out, err := Apply(src, ps)
	if err != nil {
		t.Fatal(err)
	}
	// segments between the patches must be preserved in order
	o, last := 0, 0
	for _, p := range ps {
		seg := src[last:p.Start]
		if !bytes.Equal(out[o:o+len(seg)], seg) {
			t.Fatalf("segment before %d changed", p.Start)
		}
		o += len(seg) + len(p.Repl)
		last = p.End
	}
	if !bytes.Equal(out[o:], src[last:]) {
		t.Fatal("tail changed")
	}
}
