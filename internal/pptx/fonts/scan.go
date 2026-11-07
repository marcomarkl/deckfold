package fonts

import (
	"encoding/binary"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

// scanFonts walks the font folders and returns the faces by lower-case family name. It reads
// only the table directory, the name table and the OS/2 table of each file, skips folders it
// cannot read and handles font collections (.ttc).
func scanFonts(dirs []string) map[string][]installedFont {
	out := map[string][]installedFont{}
	seen := map[string]bool{}
	for _, dir := range dirs {
		_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				if seen[p] {
					return filepath.SkipDir
				}
				seen[p] = true
				return nil
			}
			ext := strings.ToLower(filepath.Ext(p))
			if ext != ".ttf" && ext != ".otf" && ext != ".ttc" && ext != ".otc" {
				return nil
			}
			f, err := os.Open(p)
			if err != nil {
				return nil
			}
			defer f.Close()
			for _, face := range readFaces(f) {
				face.path = p
				for _, fam := range face.families {
					k := strings.ToLower(fam)
					out[k] = append(out[k], face.installedFont)
				}
			}
			return nil
		})
	}
	return out
}

type scannedFace struct {
	installedFont
	families []string
}

func readFaces(r io.ReadSeeker) []scannedFace {
	head := make([]byte, 12)
	if _, err := io.ReadFull(r, head); err != nil {
		return nil
	}
	if string(head[:4]) != "ttcf" {
		if f, ok := readFace(r, 0, 0); ok {
			return []scannedFace{f}
		}
		return nil
	}
	n := int(binary.BigEndian.Uint32(head[8:]))
	if n <= 0 || n > 256 {
		return nil
	}
	offs := make([]byte, 4*n)
	if _, err := io.ReadFull(r, offs); err != nil {
		return nil
	}
	var out []scannedFace
	for i := 0; i < n; i++ {
		if f, ok := readFace(r, int64(binary.BigEndian.Uint32(offs[4*i:])), i); ok {
			out = append(out, f)
		}
	}
	return out
}

func readFace(r io.ReadSeeker, off int64, index int) (scannedFace, bool) {
	var face scannedFace
	face.index = index
	if _, err := r.Seek(off, io.SeekStart); err != nil {
		return face, false
	}
	hdr := make([]byte, 12)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return face, false
	}
	num := int(binary.BigEndian.Uint16(hdr[4:]))
	if num <= 0 || num > 512 {
		return face, false
	}
	dir := make([]byte, 16*num)
	if _, err := io.ReadFull(r, dir); err != nil {
		return face, false
	}
	var nameOff, nameLen, os2Off, os2Len uint32
	for i := 0; i < num; i++ {
		e := dir[16*i:]
		switch string(e[:4]) {
		case "name":
			nameOff, nameLen = binary.BigEndian.Uint32(e[8:]), binary.BigEndian.Uint32(e[12:])
		case "OS/2":
			os2Off, os2Len = binary.BigEndian.Uint32(e[8:]), binary.BigEndian.Uint32(e[12:])
		}
	}
	if nameOff == 0 || nameLen < 6 || nameLen > 1<<20 {
		return face, false
	}
	name := make([]byte, nameLen)
	if _, err := r.Seek(int64(nameOff), io.SeekStart); err != nil {
		return face, false
	}
	if _, err := io.ReadFull(r, name); err != nil {
		return face, false
	}
	face.families, face.bold, face.italic = parseNames(name)
	if os2Off != 0 && os2Len >= 64 {
		os2 := make([]byte, 64)
		if _, err := r.Seek(int64(os2Off), io.SeekStart); err == nil {
			if _, err := io.ReadFull(r, os2); err == nil {
				weight := binary.BigEndian.Uint16(os2[4:])
				sel := binary.BigEndian.Uint16(os2[62:])
				face.bold = weight >= 600 || sel&0x20 != 0
				face.italic = sel&0x01 != 0
			}
		}
	}
	return face, len(face.families) > 0
}

// parseNames returns the family names (IDs 1 and 16, English) and the style from the subfamily.
func parseNames(b []byte) (families []string, bold, italic bool) {
	count := int(binary.BigEndian.Uint16(b[2:]))
	storage := int(binary.BigEndian.Uint16(b[4:]))
	set := map[string]bool{}
	for i := 0; i < count && 6+12*i+12 <= len(b); i++ {
		rec := b[6+12*i:]
		platform, lang, id := binary.BigEndian.Uint16(rec), binary.BigEndian.Uint16(rec[4:]), binary.BigEndian.Uint16(rec[6:])
		length, off := int(binary.BigEndian.Uint16(rec[8:])), int(binary.BigEndian.Uint16(rec[10:]))
		if id != 1 && id != 2 && id != 16 && id != 17 {
			continue
		}
		start := storage + off
		if start < 0 || start+length > len(b) {
			continue
		}
		raw := b[start : start+length]
		var s string
		switch {
		case platform == 3 || platform == 0:
			if platform == 3 && lang&0xFF != 0x09 {
				continue // English names only
			}
			u := make([]uint16, len(raw)/2)
			for j := range u {
				u[j] = binary.BigEndian.Uint16(raw[2*j:])
			}
			s = string(utf16.Decode(u))
		case platform == 1 && lang == 0:
			s = string(raw)
		default:
			continue
		}
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		switch id {
		case 1, 16:
			if !set[s] {
				set[s] = true
				families = append(families, s)
			}
		case 2, 17:
			l := strings.ToLower(s)
			bold = bold || strings.Contains(l, "bold") || strings.Contains(l, "black") || strings.Contains(l, "heavy")
			italic = italic || strings.Contains(l, "italic") || strings.Contains(l, "oblique")
		}
	}
	return families, bold, italic
}
