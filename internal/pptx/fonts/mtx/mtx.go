// Package mtx decompresses MicroType Express fonts, the compression PowerPoint uses for fonts
// embedded in a presentation (EOT with the TTEMBED_TTCOMPRESSED flag).
//
// It is written from the W3C member submission "MicroType Express (MTX) Font Format" of
// 5 March 2008 (https://www.w3.org/Submission/MTX/), which Monotype licenses royalty-free
// (see THIRD_PARTY_LICENSES.md). An MTX font is three LZCOMP-compressed blocks: the font tables
// in Compact Table Format (CTF), the push data of the glyph programs, and the rest of the glyph
// programs. Decode turns them back into the tables of a TrueType font.
//
// The hdmx and VDMX tables are left out: they only cache hinted widths and heights for a
// rasterizer, browsers do not need them, and the fonts in a presentation are subset anyway.
package mtx

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Decode decompresses an MTX font and returns its tables, keyed by tag, ready to be written as
// an SFNT file. The loca table is rebuilt, glyf and cvt are decoded, and head is changed to the
// long loca format.
func Decode(data []byte) (map[string][]byte, error) {
	if len(data) < 10 {
		return nil, errors.New("mtx: header too short")
	}
	version := data[0]
	off2 := int(data[4])<<16 | int(data[5])<<8 | int(data[6])
	off3 := int(data[7])<<16 | int(data[8])<<8 | int(data[9])
	if off2 < 10 || off3 < off2 || off3 > len(data) {
		return nil, fmt.Errorf("mtx: invalid block offsets %d, %d for %d bytes", off2, off3, len(data))
	}
	var blocks [3][]byte
	for i, b := range [3][]byte{data[10:off2], data[off2:off3], data[off3:]} {
		out, err := unpack(b, version)
		if err != nil {
			return nil, fmt.Errorf("mtx: block %d: %w", i+1, err)
		}
		blocks[i] = out
	}
	return decodeCTF(blocks[0], blocks[1], blocks[2])
}

// decodeCTF rebuilds the font tables from the three decompressed blocks (chapters 5 to 7).
func decodeCTF(rest, push, code []byte) (map[string][]byte, error) {
	if len(rest) < 12 {
		return nil, errors.New("mtx: font tables too short")
	}
	n := int(binary.BigEndian.Uint16(rest[4:]))
	if 12+16*n > len(rest) {
		return nil, errors.New("mtx: table directory outside the data")
	}
	tables := map[string][]byte{}
	for i := 0; i < n; i++ {
		rec := rest[12+16*i:]
		tag := string(rec[:4])
		off := int(binary.BigEndian.Uint32(rec[8:]))
		length := int(binary.BigEndian.Uint32(rec[12:]))
		if off < 0 || length < 0 || off+length > len(rest) || off+length < off {
			return nil, fmt.Errorf("mtx: table %q outside the data", tag)
		}
		tables[tag] = rest[off : off+length]
	}
	head, maxp := tables["head"], tables["maxp"]
	if len(head) < 54 || len(maxp) < 6 {
		return nil, errors.New("mtx: head or maxp missing")
	}
	out := map[string][]byte{}
	for tag, t := range tables {
		switch tag {
		case "loca", "glyf", "cvt ", "hdmx", "VDMX", "DSIG":
		default:
			if len(t) == 0 {
				continue // an empty table (seen: prep) adds nothing and trips some parsers
			}
			out[tag] = t
		}
	}
	if cvt, ok := tables["cvt "]; ok {
		t, err := decodeCvt(cvt)
		if err != nil {
			return nil, err
		}
		out["cvt "] = t
	}
	if glyf, ok := tables["glyf"]; ok {
		numGlyphs := int(binary.BigEndian.Uint16(maxp[4:]))
		g, loca, err := decodeGlyf(glyf, push, code, numGlyphs)
		if err != nil {
			return nil, err
		}
		out["glyf"], out["loca"] = g, loca
		h := append([]byte{}, head...)
		binary.BigEndian.PutUint16(h[50:], 1) // indexToLocFormat: long offsets
		out["head"] = h
	}
	return out, nil
}

// decodeCvt undoes section 5.2: a count followed by values stored as differences to the
// previous value, each in one to three bytes.
func decodeCvt(b []byte) ([]byte, error) {
	if len(b) < 2 {
		return nil, errors.New("mtx: cvt too short")
	}
	n := int(binary.BigEndian.Uint16(b))
	out := make([]byte, 2*n)
	p := 2
	var last uint16
	for i := 0; i < n; i++ {
		if p >= len(b) {
			return nil, errors.New("mtx: cvt ends early")
		}
		code := b[p]
		p++
		var v int
		switch {
		case code < 238:
			v = int(code)
		case code == 238:
			if p+2 > len(b) {
				return nil, errors.New("mtx: cvt ends early")
			}
			v = int(int16(binary.BigEndian.Uint16(b[p:])))
			p += 2
		default:
			if p >= len(b) {
				return nil, errors.New("mtx: cvt ends early")
			}
			if code >= 248 {
				v = int(b[p]) + (int(code)-247)*238
			} else {
				v = -(int(b[p]) + (int(code)-239)*238)
			}
			p++
		}
		last += uint16(v)
		binary.BigEndian.PutUint16(out[2*i:], last)
	}
	return out, nil
}
