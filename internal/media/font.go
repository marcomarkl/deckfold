package media

import (
	"bytes"
	"sort"

	"github.com/andybalholm/brotli"
	"github.com/tdewolff/font"
)

const mimeWOFF2 = "font/woff2"

type sfntTable struct {
	tag  string
	data []byte
}

// parseSFNT reads the table directory of a TrueType or OpenType font. Collections (ttcf) are
// not fonts of one face and are not read.
func parseSFNT(d []byte) (flavor uint32, tables []sfntTable, ok bool) {
	if len(d) < 12 || len(d) > maxFontBytes {
		return 0, nil, false
	}
	flavor = be32(d)
	if flavor != 0x00010000 && flavor != 0x4f54544f && flavor != 0x74727565 { // 1.0, OTTO, true
		return 0, nil, false
	}
	n := int(be16(d[4:]))
	if n == 0 || 12+16*n > len(d) {
		return 0, nil, false
	}
	seen := map[string]bool{}
	for i := 0; i < n; i++ {
		e := d[12+16*i:]
		tag := string(e[0:4])
		off, length := int(be32(e[8:])), int(be32(e[12:]))
		if off < 0 || length < 0 || off+length > len(d) || seen[tag] {
			return 0, nil, false
		}
		seen[tag] = true
		tables = append(tables, sfntTable{tag, d[off : off+length]})
	}
	return flavor, tables, true
}

// The tags WOFF2 writes as one number, in the order of the specification (section 4.1).
var woff2Tags = []string{
	"cmap", "head", "hhea", "hmtx", "maxp", "name", "OS/2", "post", "cvt ", "fpgm", "glyf", "loca", "prep", "CFF ", "VORG", "EBDT",
	"EBLC", "gasp", "hdmx", "kern", "LTSH", "PCLT", "VDMX", "vhea", "vmtx", "BASE", "GDEF", "GPOS", "GSUB", "EBSC", "JSTF", "MATH",
	"CBDT", "CBLC", "COLR", "CPAL", "SVG ", "sbix", "acnt", "avar", "bdat", "bloc", "bsln", "cvar", "fdsc", "feat", "fmtx", "fvar",
	"gvar", "hsty", "just", "lcar", "mort", "morx", "opbd", "prop", "trak", "Zapf", "Silf", "Glat", "Gloc", "Feat", "Sill",
}

func putBase128(b *bytes.Buffer, v uint32) {
	var tmp [5]byte
	i := len(tmp) - 1
	tmp[i] = byte(v & 0x7f)
	for v >>= 7; v > 0; v >>= 7 {
		i--
		tmp[i] = byte(v&0x7f) | 0x80
	}
	b.Write(tmp[i:])
}

func putBE32(b *bytes.Buffer, v uint32) {
	b.Write([]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)})
}

func putBE16(b *bytes.Buffer, v uint16) { b.Write([]byte{byte(v >> 8), byte(v)}) }

// writeWOFF2 packs the tables of a font as WOFF2 (W3C Recommendation "WOFF File Format 2.0").
// No table is transformed: glyf and loca are written with the transformation version 3, which
// section 4.1 defines as the null transform for these two, every other table with version 0.
// So a reader gets the tables back byte for byte, with one exception the format demands:
// bit 11 of the flags in the head table is set (section 5), which marks a font that went
// through such a packing. A digital signature (DSIG) is left out: it signs the original file
// and is void for any repacked one, and WOFF2 readers refuse a file that still has it.
func writeWOFF2(flavor uint32, tables []sfntTable) []byte {
	ts := make([]sfntTable, 0, len(tables))
	var loca *sfntTable
	for i, t := range tables {
		switch {
		case t.tag == "loca":
			loca = &tables[i]
			continue
		case t.tag == "DSIG":
			continue
		case t.tag == "head" && len(t.data) >= 18:
			d := append([]byte(nil), t.data...)
			d[16] |= 0x08
			t.data = d
		}
		ts = append(ts, t)
	}
	sort.SliceStable(ts, func(i, j int) bool { return ts[i].tag < ts[j].tag })
	if loca != nil {
		// loca follows glyf directly, as the format asks for transformed fonts
		at := len(ts)
		for i, t := range ts {
			if t.tag == "glyf" {
				at = i + 1
			}
		}
		ts = append(ts[:at], append([]sfntTable{*loca}, ts[at:]...)...)
	}
	var dir, raw bytes.Buffer
	sfntSize := 12 + 16*len(ts)
	for _, t := range ts {
		idx := 63
		for i, k := range woff2Tags {
			if k == t.tag {
				idx = i
				break
			}
		}
		flags := byte(idx)
		if t.tag == "glyf" || t.tag == "loca" {
			flags |= 3 << 6
		}
		dir.WriteByte(flags)
		if idx == 63 {
			dir.WriteString(t.tag)
		}
		putBase128(&dir, uint32(len(t.data)))
		raw.Write(t.data)
		sfntSize += (len(t.data) + 3) &^ 3
	}
	var comp bytes.Buffer
	bw := brotli.NewWriterOptions(&comp, brotli.WriterOptions{Quality: 11, LGWin: 24})
	if _, err := bw.Write(raw.Bytes()); err != nil {
		return nil
	}
	if err := bw.Close(); err != nil {
		return nil
	}
	total := 48 + dir.Len() + comp.Len()
	total = (total + 3) &^ 3
	var out bytes.Buffer
	out.WriteString("wOF2")
	putBE32(&out, flavor)
	putBE32(&out, uint32(total))
	putBE16(&out, uint16(len(ts)))
	putBE16(&out, 0)
	putBE32(&out, uint32(sfntSize))
	putBE32(&out, uint32(comp.Len()))
	putBE16(&out, 1) // version of the font file, 1.0
	putBE16(&out, 0)
	for i := 0; i < 5; i++ { // no metadata, no private data
		putBE32(&out, 0)
	}
	out.Write(dir.Bytes())
	out.Write(comp.Bytes())
	for out.Len() < total {
		out.WriteByte(0)
	}
	return out.Bytes()
}

// sameTables compares two fonts table by table. Two things in the head table are left out: the
// checksum adjustment, which covers the whole file and is written anew by every tool, and
// bit 11 of the flags, which WOFF2 sets. A DSIG table of the first font is not expected in
// the second.
func sameTables(a, b []sfntTable) bool {
	m := map[string][]byte{}
	for _, t := range a {
		if t.tag != "DSIG" {
			m[t.tag] = t.data
		}
	}
	if len(m) != len(b) {
		return false
	}
	for _, t := range b {
		x, ok := m[t.tag]
		if !ok || len(x) != len(t.data) {
			return false
		}
		if t.tag == "head" && len(x) >= 18 {
			if !bytes.Equal(x[:8], t.data[:8]) || !bytes.Equal(x[12:16], t.data[12:16]) || x[16]|0x08 != t.data[16]|0x08 || !bytes.Equal(x[17:], t.data[17:]) {
				return false
			}
			continue
		}
		if !bytes.Equal(x, t.data) {
			return false
		}
	}
	return true
}

func convertFont(data []byte, k kind) Result {
	if len(data) > maxFontBytes {
		return Result{}
	}
	sfnt := data
	if k == kindWOFF {
		// the header names the size of the unpacked font
		if len(data) < 44 || be32(data[16:]) > maxFontBytes {
			return Result{}
		}
		var err error
		if sfnt, err = font.ParseWOFF(data); err != nil {
			return Result{}
		}
	}
	flavor, tables, ok := parseSFNT(sfnt)
	if !ok {
		return Result{}
	}
	out := writeWOFF2(flavor, tables)
	if out == nil || len(out) >= len(data) {
		return Result{}
	}
	// read the result back with a WOFF2 reader that is not this writer
	back, err := font.ParseWOFF2(out)
	if err != nil {
		return Result{}
	}
	_, got, ok := parseSFNT(back)
	if !ok || !sameTables(tables, got) {
		return Result{}
	}
	return Result{Data: out, Mime: mimeWOFF2, Ext: "woff2", Changed: true}
}
