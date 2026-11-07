package fonts

import (
	"encoding/binary"
	"sort"
	"unicode/utf16"

	tfont "github.com/tdewolff/font"
)

// writeSFNT writes an SFNT file from its tables. Unlike tdewolff/font's writer it does not stamp
// the current time into the head table, so the output is the same on every run.
func writeSFNT(tables map[string][]byte, cff bool) []byte {
	tags := make([]string, 0, len(tables))
	for t := range tables {
		if len(t) == 4 {
			tags = append(tags, t)
		}
	}
	sort.Strings(tags)
	n := len(tags)
	es := 0
	for 1<<(es+1) <= n {
		es++
	}
	sr := (1 << es) * 16
	head := make([]byte, 12+16*n)
	if cff {
		copy(head, "OTTO")
	} else {
		binary.BigEndian.PutUint32(head, 0x00010000)
	}
	binary.BigEndian.PutUint16(head[4:], uint16(n))
	binary.BigEndian.PutUint16(head[6:], uint16(sr))
	binary.BigEndian.PutUint16(head[8:], uint16(es))
	binary.BigEndian.PutUint16(head[10:], uint16(n*16-sr))
	out := head
	headOff := -1
	for i, tag := range tags {
		data := tables[tag]
		if tag == "head" && len(data) >= 12 {
			data = append([]byte{}, data...)
			binary.BigEndian.PutUint32(data[8:], 0) // checksumAdjustment, set below
		}
		off := len(out)
		if tag == "head" {
			headOff = off
		}
		rec := head[12+16*i:]
		copy(rec, tag)
		binary.BigEndian.PutUint32(rec[4:], checksum(data))
		binary.BigEndian.PutUint32(rec[8:], uint32(off))
		binary.BigEndian.PutUint32(rec[12:], uint32(len(data)))
		out = append(out, data...)
		for len(out)%4 != 0 {
			out = append(out, 0)
		}
		head = out[:12+16*n]
	}
	if headOff >= 0 && headOff+12 <= len(out) {
		binary.BigEndian.PutUint32(out[headOff+8:], 0xB1B0AFBA-checksum(out))
	}
	return out
}

func checksum(b []byte) uint32 {
	var s uint32
	for i := 0; i < len(b); i += 4 {
		var w [4]byte
		copy(w[:], b[i:min(i+4, len(b))])
		s += binary.BigEndian.Uint32(w[:])
	}
	return s
}

// nameTable builds a name table that gives the subset its own family name. Copyright (0),
// license (13) and license URL (14) of the original font are kept.
func nameTable(family string, orig *tfont.SFNT) []byte {
	type rec struct {
		id  uint16
		val string
	}
	recs := []rec{
		{1, family}, {2, "Regular"}, {3, family}, {4, family}, {6, family},
	}
	if orig != nil && orig.Name != nil {
		for _, id := range []tfont.NameID{0, 13, 14} {
			for _, r := range orig.Name.Get(id) {
				if s := r.String(); s != "" {
					recs = append(recs, rec{uint16(id), s})
					break
				}
			}
		}
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].id < recs[j].id })
	var strs []byte
	hdr := make([]byte, 6+12*len(recs))
	binary.BigEndian.PutUint16(hdr[0:], 0)
	binary.BigEndian.PutUint16(hdr[2:], uint16(len(recs)))
	binary.BigEndian.PutUint16(hdr[4:], uint16(len(hdr)))
	for i, r := range recs {
		u := utf16.Encode([]rune(r.val))
		b := make([]byte, 2*len(u))
		for j, c := range u {
			binary.BigEndian.PutUint16(b[2*j:], c)
		}
		p := hdr[6+12*i:]
		binary.BigEndian.PutUint16(p[0:], 3)      // Windows
		binary.BigEndian.PutUint16(p[2:], 1)      // Unicode BMP
		binary.BigEndian.PutUint16(p[4:], 0x0409) // en-US
		binary.BigEndian.PutUint16(p[6:], r.id)
		binary.BigEndian.PutUint16(p[8:], uint16(len(b)))
		binary.BigEndian.PutUint16(p[10:], uint16(len(strs)))
		strs = append(strs, b...)
	}
	return append(hdr, strs...)
}

// cmapTable writes a cmap table for a subset from the characters it maps: format 4 for the
// Basic Multilingual Plane (platforms 0/3 and 3/1) and, when needed, format 12 for the rest
// (platforms 0/4 and 3/10). The subset's own cmap is not used: tdewolff/font rebuilds it from
// a reverse lookup that reports wrong characters for some glyphs (for example the u with
// diaeresis for the cedilla of Liberation Serif), so a composite glyph's component could take
// over a character.
func cmapTable(m map[rune]uint16) []byte {
	runes := make([]rune, 0, len(m))
	for r := range m {
		runes = append(runes, r)
	}
	sort.Slice(runes, func(i, j int) bool { return runes[i] < runes[j] })
	type seg struct {
		start, end rune
		glyph      uint16
	}
	// runs of characters whose glyph ids also follow each other
	var segs []seg
	for _, r := range runes {
		g := m[r]
		if n := len(segs); n > 0 && segs[n-1].end == r-1 && uint32(segs[n-1].glyph)+uint32(r-segs[n-1].start) == uint32(g) {
			segs[n-1].end = r
			continue
		}
		segs = append(segs, seg{r, r, g})
	}
	var bmp []seg
	for _, s := range segs {
		if s.start <= 0xFFFF {
			if s.end > 0xFFFF {
				s.end = 0xFFFF
			}
			if s.start == 0xFFFF {
				continue
			}
			if s.end == 0xFFFF {
				s.end = 0xFFFE
			}
			bmp = append(bmp, s)
		}
	}
	// format 4 with idDelta for every segment, plus the closing 0xFFFF segment
	n := len(bmp) + 1
	es := 0
	for 1<<(es+1) <= n {
		es++
	}
	f4 := make([]byte, 16+8*n)
	be := binary.BigEndian
	be.PutUint16(f4[0:], 4)
	be.PutUint16(f4[2:], uint16(len(f4)))
	be.PutUint16(f4[6:], uint16(2*n))
	be.PutUint16(f4[8:], uint16(2*(1<<es)))
	be.PutUint16(f4[10:], uint16(es))
	be.PutUint16(f4[12:], uint16(2*n-2*(1<<es)))
	for i := 0; i < n; i++ {
		start, end, delta := uint16(0xFFFF), uint16(0xFFFF), uint16(1)
		if i < len(bmp) {
			start, end = uint16(bmp[i].start), uint16(bmp[i].end)
			delta = bmp[i].glyph - start
		}
		be.PutUint16(f4[14+2*i:], end)
		be.PutUint16(f4[16+2*n+2*i:], start)
		be.PutUint16(f4[16+4*n+2*i:], delta)
		// idRangeOffset stays 0
	}
	var f12 []byte
	if len(runes) > 0 && runes[len(runes)-1] > 0xFFFF {
		f12 = make([]byte, 16+12*len(segs))
		be.PutUint16(f12[0:], 12)
		be.PutUint32(f12[4:], uint32(len(f12)))
		be.PutUint32(f12[12:], uint32(len(segs)))
		for i, s := range segs {
			be.PutUint32(f12[16+12*i:], uint32(s.start))
			be.PutUint32(f12[20+12*i:], uint32(s.end))
			be.PutUint32(f12[24+12*i:], uint32(s.glyph))
		}
	}
	type rec struct {
		platform, encoding uint16
		table              []byte
	}
	recs := []rec{{0, 3, f4}, {3, 1, f4}}
	if f12 != nil {
		recs = []rec{{0, 3, f4}, {0, 4, f12}, {3, 1, f4}, {3, 10, f12}}
	}
	out := make([]byte, 4+8*len(recs))
	be.PutUint16(out[2:], uint16(len(recs)))
	offs := map[*byte]uint32{}
	for i, r := range recs {
		off, ok := offs[&r.table[0]]
		if !ok {
			off = uint32(len(out))
			offs[&r.table[0]] = off
			out = append(out, r.table...)
		}
		be.PutUint16(out[4+8*i:], r.platform)
		be.PutUint16(out[6+8*i:], r.encoding)
		be.PutUint32(out[8+8*i:], off)
	}
	return out
}
