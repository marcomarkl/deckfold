// Package patch applies replacements to byte ranges of an original.
// All bytes outside the ranges stay unchanged.
package patch

import (
	"fmt"
	"io"
	"sort"
)

// Patch replaces src[Start:End] with Repl. Start==End is an insertion.
type Patch struct {
	Start, End int
	Repl       []byte
	// Write, when set, writes the replacement itself (after Repl) instead of holding it in
	// memory; WriteTo uses it, Apply cannot.
	Write func(w io.Writer) error
	// Order sorts insertions at the same position (lower first).
	Order int
}

// Apply applies the patches. Overlapping ranges are an error.
func Apply(src []byte, ps []Patch) ([]byte, error) {
	for _, p := range ps {
		if p.Write != nil {
			return nil, fmt.Errorf("patch: a streamed patch needs WriteTo")
		}
	}
	s := sorted(ps)
	size := len(src)
	for _, p := range s {
		size += len(p.Repl) - (p.End - p.Start)
	}
	out := make([]byte, 0, size)
	pos := 0
	for _, p := range s {
		if p.Start < pos || p.End < p.Start || p.End > len(src) {
			return nil, fmt.Errorf("patch: overlapping or invalid range %d-%d (position %d)", p.Start, p.End, pos)
		}
		out = append(out, src[pos:p.Start]...)
		out = append(out, p.Repl...)
		pos = p.End
	}
	out = append(out, src[pos:]...)
	return out, nil
}

// WriteTo writes src with the patches applied to w, without building the result in memory.
func WriteTo(w io.Writer, src []byte, ps []Patch) (int64, error) {
	s := sorted(ps)
	var n int64
	pos := 0
	write := func(b []byte) error {
		k, err := w.Write(b)
		n += int64(k)
		return err
	}
	cw := &countWriter{w: w}
	for _, p := range s {
		if p.Start < pos || p.End < p.Start || p.End > len(src) {
			return n, fmt.Errorf("patch: overlapping or invalid range %d-%d (position %d)", p.Start, p.End, pos)
		}
		if err := write(src[pos:p.Start]); err != nil {
			return n, err
		}
		if err := write(p.Repl); err != nil {
			return n, err
		}
		if p.Write != nil {
			cw.n = 0
			if err := p.Write(cw); err != nil {
				return n + cw.n, err
			}
			n += cw.n
		}
		pos = p.End
	}
	err := write(src[pos:])
	return n, err
}

type countWriter struct {
	w io.Writer
	n int64
}

func (c *countWriter) Write(b []byte) (int, error) {
	k, err := c.w.Write(b)
	c.n += int64(k)
	return k, err
}

// sorted returns the patches in the order they apply.
func sorted(ps []Patch) []Patch {
	s := make([]Patch, len(ps))
	copy(s, ps)
	sort.SliceStable(s, func(i, j int) bool {
		if s[i].Start != s[j].Start {
			return s[i].Start < s[j].Start
		}
		// insertions (empty range) before replacements at the same position
		ei, ej := s[i].Start == s[i].End, s[j].Start == s[j].End
		if ei != ej {
			return ei
		}
		return s[i].Order < s[j].Order
	})
	return s
}
