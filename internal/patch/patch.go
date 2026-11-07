// Package patch applies replacements to byte ranges of an original.
// All bytes outside the ranges stay unchanged.
package patch

import (
	"fmt"
	"sort"
)

// Patch replaces src[Start:End] with Repl. Start==End is an insertion.
type Patch struct {
	Start, End int
	Repl       []byte
	// Order sorts insertions at the same position (lower first).
	Order int
}

// Apply applies the patches. Overlapping ranges are an error.
func Apply(src []byte, ps []Patch) ([]byte, error) {
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
