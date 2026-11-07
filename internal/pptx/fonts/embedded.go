package fonts

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"unicode/utf16"

	tfont "github.com/tdewolff/font"

	"deckfold/internal/pptx/fonts/mtx"
)

// Deobfuscate turns a font stored in a presentation (ppt/fonts/*.fntdata) into a plain SFNT file.
// PowerPoint stores embedded fonts as Embedded OpenType (EOT), compressed with MicroType Express
// (spike S7); plain SFNT data is passed through.
func Deobfuscate(data []byte, part string) ([]byte, error) {
	if bytes.HasPrefix(data, []byte{0, 1, 0, 0}) || bytes.HasPrefix(data, []byte("OTTO")) || bytes.HasPrefix(data, []byte("true")) {
		return data, nil
	}
	out, err := parseEOT(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", part, err)
	}
	return out, nil
}

// parseEOT reads the header of the W3C EOT submission (versions 0x00010000, 0x00020001 and
// 0x00020002), removes the XOR obfuscation and decompresses MTX font data.
func parseEOT(b []byte) ([]byte, error) {
	if len(b) < 82 {
		return nil, errors.New("unknown font format")
	}
	le16 := func(p int) int { return int(binary.LittleEndian.Uint16(b[p:])) }
	fontDataSize := int(binary.LittleEndian.Uint32(b[4:]))
	version := binary.LittleEndian.Uint32(b[8:])
	flags := binary.LittleEndian.Uint32(b[12:])
	if le16(34) != 0x504C {
		return nil, errors.New("unknown font format")
	}
	if version != 0x00010000 && version != 0x00020001 && version != 0x00020002 {
		return nil, fmt.Errorf("unsupported EOT version %#x", version)
	}
	p := 82 // after Padding1
	skipName := func(pad bool) bool {
		if pad {
			p += 2
		}
		if p+2 > len(b) {
			return false
		}
		p += 2 + le16(p)
		return p <= len(b)
	}
	// FamilyName, StyleName, VersionName, FullName; Padding2..4 come before the last three.
	ok := skipName(false) && skipName(true) && skipName(true) && skipName(true)
	if ok && version >= 0x00020001 {
		ok = skipName(true) // RootString
	}
	if ok && version == 0x00020002 {
		p += 8                               // RootStringCheckSum, EUDCCodePage
		ok = skipName(true) && p+8 <= len(b) // Signature
		if ok {
			p += 4 // EUDCFlags
			n := int(binary.LittleEndian.Uint32(b[p:]))
			p += 4 + n // EUDCFontData
		}
	}
	if !ok || fontDataSize < 0 || p+fontDataSize > len(b) || p > len(b) {
		return nil, errors.New("EOT header outside the data")
	}
	data := append([]byte{}, b[p:p+fontDataSize]...)
	if flags&0x10000000 != 0 {
		for i := range data {
			data[i] ^= 0x50
		}
	}
	if flags&0x4 == 0 {
		return data, nil
	}
	tables, err := mtx.Decode(data)
	if err != nil {
		return nil, err
	}
	_, cff := tables["CFF "]
	return writeSFNT(tables, cff), nil
}

// EOT wraps a TrueType font as PowerPoint stores embedded fonts: an EOT file (version
// 0x00020002) with MTX-compressed data (mtx.Encode). The deck generator uses it for the public
// test deck with an embedded font.
func EOT(sfnt []byte) ([]byte, error) {
	s, err := tfont.ParseSFNT(sfnt, 0)
	if err != nil {
		return nil, err
	}
	tables := map[string][]byte{}
	for tag, t := range s.Tables {
		tables[tag] = t
	}
	data, err := mtx.Encode(tables)
	if err != nil {
		return nil, err
	}
	name := func(id tfont.NameID) []byte {
		for _, r := range s.Name.Get(id) {
			if v := r.String(); v != "" {
				u := utf16.Encode([]rune(v))
				b := make([]byte, 2*len(u))
				for i, c := range u {
					binary.LittleEndian.PutUint16(b[2*i:], c)
				}
				return b
			}
		}
		return nil
	}
	os2, head := s.Tables["OS/2"], s.Tables["head"]
	if len(os2) < 78 || len(head) < 54 {
		return nil, errors.New("EOT: OS/2 or head missing")
	}
	var h []byte
	u32 := func(v uint32) { h = binary.LittleEndian.AppendUint32(h, v) }
	u16 := func(v uint16) { h = binary.LittleEndian.AppendUint16(h, v) }
	u32(0) // EOTSize, set below
	u32(uint32(len(data)))
	u32(0x00020002)
	u32(0x4)                     // TTEMBED_TTCOMPRESSED
	h = append(h, os2[32:42]...) // PANOSE
	h = append(h, 1)             // DEFAULT_CHARSET
	h = append(h, byte(binary.BigEndian.Uint16(os2[62:])&1))
	u32(uint32(binary.BigEndian.Uint16(os2[4:]))) // weight
	u16(binary.BigEndian.Uint16(os2[8:]))         // fsType
	u16(0x504C)                                   // magic number
	for _, off := range []int{42, 46, 50, 54} {   // Unicode ranges
		u32(binary.BigEndian.Uint32(os2[off:]))
	}
	if len(os2) >= 86 {
		u32(binary.BigEndian.Uint32(os2[78:]))
		u32(binary.BigEndian.Uint32(os2[82:]))
	} else {
		u32(0)
		u32(0)
	}
	u32(binary.BigEndian.Uint32(head[8:])) // checkSumAdjustment
	for i := 0; i < 4; i++ {
		u32(0) // reserved
	}
	for _, id := range []tfont.NameID{1, 2, 5, 4} { // family, style, version, full name
		u16(0) // padding 1 to 4
		n := name(id)
		u16(uint16(len(n)))
		h = append(h, n...)
	}
	u16(0) // padding 5
	u16(0) // RootStringSize
	u32(0) // RootStringCheckSum
	u32(0) // EUDCCodePage
	u16(0) // padding 6
	u16(0) // SignatureSize
	u32(0) // EUDCFlags
	u32(0) // EUDCFontSize
	out := append(h, data...)
	binary.LittleEndian.PutUint32(out, uint32(len(out)))
	return out, nil
}
