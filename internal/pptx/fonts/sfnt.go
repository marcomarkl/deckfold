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
