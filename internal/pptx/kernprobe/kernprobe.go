// Package kernprobe reads what a font file says about kerning: the advance widths, the pairs of
// the legacy kern table and the kerning its GPOS table adds. The reference tools use it to find
// out which of the two PowerPoint applies (scripts/kernmeasure) and to choose words that can
// tell them apart (the kerning feature deck). The converter does not use it.
package kernprobe

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf16"

	"github.com/go-text/typesetting/di"
	gtfont "github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"

	"deckfold/internal/pptx/fonts"
)

// Font is one face of a font file.
type Font struct {
	Path       string // the file ("" for a bundled font)
	Index      int    // face in a font collection
	Family     string // name ID 1
	Style      string // name ID 2, for example "Regular" or "Bold"
	PostScript string // name ID 6, the name a PDF shows

	KernVersion int  // version of the kern table: 0 (Microsoft), 1 (Apple), -1 without a table
	KernPairs   int  // horizontal pairs in the format 0 subtables of the kern table
	GPOSKern    bool // GPOS has a kern feature

	face  *gtfont.Face
	pairs map[[2]gtfont.GID]int16
	ct    map[string]float64 // answers of CoreText, see CoreText
}

// Parse reads face index of a font file (index 0 for a plain font). Glyphs and advance widths
// come from go-text/typesetting, which also reads fonts that tdewolff/font refuses (Helvetica
// with its Apple kern table); the kern table is read here, and a test holds it against
// tdewolff/font, the library the converter uses.
func Parse(data []byte, index int) (*Font, error) {
	offs := faceOffsets(data)
	if index < 0 || index >= len(offs) {
		return nil, fmt.Errorf("no face %d", index)
	}
	f := &Font{Index: index, KernVersion: -1}
	if len(offs) > 1 || bytes.HasPrefix(data, []byte("ttcf")) {
		faces, err := gtfont.ParseTTC(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		if index >= len(faces) {
			return nil, fmt.Errorf("no face %d", index)
		}
		f.face = faces[index]
	} else {
		face, err := gtfont.ParseTTF(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		f.face = face
	}
	off := offs[index]
	f.Family, f.Style, f.PostScript = names(table(data, off, "name"))
	if k := table(data, off, "kern"); len(k) >= 4 {
		f.KernVersion = int(binary.BigEndian.Uint16(k))
		if f.KernVersion == 1 {
			f.pairs = appleKern(k)
		} else {
			f.pairs = kernTable(k)
		}
		f.KernPairs = len(f.pairs)
	}
	f.GPOSKern = hasFeature(table(data, off, "GPOS"), "kern")
	return f, nil
}

// Open reads face index of the font file at path.
func Open(path string, index int) (*Font, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := Parse(data, index)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	f.Path = path
	return f, nil
}

// Bundled reads one of the fonts deckfold bundles ("Carlito", "LiberationSans",
// "LiberationSerif", "LiberationMono").
func Bundled(name string, bold bool) (*Font, error) {
	return Parse(fonts.BundledFile(name, bold, false), 0)
}

// Has reports whether the font has a glyph for every character of text.
func (f *Font) Has(text string) bool {
	for _, r := range text {
		if _, ok := f.face.NominalGlyph(r); !ok {
			return false
		}
	}
	return true
}

func (f *Font) glyph(r rune) gtfont.GID {
	g, _ := f.face.NominalGlyph(r)
	return g
}

// Plain returns the width of text in em from the advance widths alone.
func (f *Font) Plain(text string) float64 {
	w := 0.0
	for _, r := range text {
		w += float64(f.face.HorizontalAdvance(f.glyph(r)))
	}
	return w / float64(f.face.Upem())
}

// Pair returns what the kern table says for two characters, in em.
func (f *Font) Pair(a, b rune) float64 {
	return float64(f.pairs[[2]gtfont.GID{f.glyph(a), f.glyph(b)}]) / float64(f.face.Upem())
}

// Kern returns the sum of the kern table pairs inside text, in em (0 without a kern table).
func (f *Font) Kern(text string) float64 {
	if len(f.pairs) == 0 {
		return 0
	}
	sum, prev := 0.0, rune(-1)
	for _, r := range text {
		if prev >= 0 {
			sum += f.Pair(prev, r)
		}
		prev = r
	}
	return sum
}

// GPOS returns what the kern feature of GPOS adds to the width of text, in em: the shaped width
// with the feature minus the shaped width without it. It is 0 for a font whose GPOS has no kern
// feature; there a shaper would fall back to the kern table, which Kern already reports. For
// texts CoreText was asked about, it returns CoreText's answer.
func (f *Font) GPOS(text string) float64 {
	if !f.GPOSKern || text == "" {
		return 0
	}
	if k, ok := f.ct[text]; ok {
		return k
	}
	return f.shaped(text, 1) - f.shaped(text, 0)
}

func (f *Font) shaped(text string, kern uint32) float64 {
	rs := []rune(text)
	var sh shaping.HarfbuzzShaper
	out := sh.Shape(shaping.Input{Text: rs, RunStart: 0, RunEnd: len(rs), Direction: di.DirectionLTR, Face: f.face,
		Size: fixed.I(1000), Script: language.Latin, Language: language.NewLanguage("en"),
		FontFeatures: []shaping.FontFeature{
			{Tag: ot.MustNewTag("kern"), Value: kern},
			// no ligatures: a ligature glyph has its own width and would hide the pairs
			{Tag: ot.MustNewTag("liga"), Value: 0}, {Tag: ot.MustNewTag("clig"), Value: 0},
			{Tag: ot.MustNewTag("calt"), Value: 0}, {Tag: ot.MustNewTag("dlig"), Value: 0},
		}})
	return float64(out.Advance) / 64 / 1000
}

// names returns the family (ID 1), the style (ID 2) and the PostScript name (ID 6) of a name
// table, preferring the Windows English records.
func names(b []byte) (family, style, ps string) {
	if len(b) < 6 {
		return
	}
	n, strOff := int(binary.BigEndian.Uint16(b[2:])), int(binary.BigEndian.Uint16(b[4:]))
	got := map[int]int{} // name ID -> rank of the record taken
	set := func(id, rank int, s string) {
		if s == "" || got[id] >= rank {
			return
		}
		got[id] = rank
		switch id {
		case 1:
			family = s
		case 2:
			style = s
		case 6:
			ps = s
		}
	}
	for i := 0; i < n; i++ {
		rec := b[6+12*i:]
		if len(rec) < 12 {
			break
		}
		plat, lang, id := binary.BigEndian.Uint16(rec), binary.BigEndian.Uint16(rec[4:]), int(binary.BigEndian.Uint16(rec[6:]))
		l, o := int(binary.BigEndian.Uint16(rec[8:])), strOff+int(binary.BigEndian.Uint16(rec[10:]))
		if id != 1 && id != 2 && id != 6 || o+l > len(b) {
			continue
		}
		raw := b[o : o+l]
		switch {
		case plat == 3 || plat == 0:
			u := make([]uint16, len(raw)/2)
			for j := range u {
				u[j] = binary.BigEndian.Uint16(raw[2*j:])
			}
			rank := 2
			if plat == 3 && lang == 0x409 {
				rank = 3
			}
			set(id, rank, string(utf16.Decode(u)))
		case plat == 1:
			set(id, 1, string(raw))
		}
	}
	return
}

// kernTable reads the horizontal format 0 subtables of a kern table of version 0.
func kernTable(b []byte) map[[2]gtfont.GID]int16 {
	out := map[[2]gtfont.GID]int16{}
	n, p := int(binary.BigEndian.Uint16(b[2:])), 4
	for i := 0; i < n && p+6 <= len(b); i++ {
		length, coverage := int(binary.BigEndian.Uint16(b[p+2:])), binary.BigEndian.Uint16(b[p+4:])
		// bit 0 horizontal, bit 1 minimum values, bit 2 cross-stream; the high byte is the format
		if coverage>>8 == 0 && coverage&0x7 == 1 && p+14 <= len(b) {
			readPairs(out, b, p+14, int(binary.BigEndian.Uint16(b[p+6:])))
		}
		if length == 0 {
			break
		}
		p += length
	}
	return out
}

// appleKern reads the horizontal format 0 subtables of an Apple kern table (version 1.0).
func appleKern(b []byte) map[[2]gtfont.GID]int16 {
	out := map[[2]gtfont.GID]int16{}
	if len(b) < 8 {
		return out
	}
	n, p := int(binary.BigEndian.Uint32(b[4:])), 8
	for i := 0; i < n && p+8 <= len(b); i++ {
		length, coverage := int(binary.BigEndian.Uint32(b[p:])), binary.BigEndian.Uint16(b[p+4:])
		// 0x8000 vertical, 0x4000 cross-stream, 0x2000 variation; the low byte is the format
		if coverage&0xE000 == 0 && coverage&0xFF == 0 && p+16 <= len(b) {
			readPairs(out, b, p+16, int(binary.BigEndian.Uint16(b[p+8:])))
		}
		if length == 0 {
			break
		}
		p += length
	}
	return out
}

// readPairs reads n records of left glyph, right glyph and value starting at q. A pair listed
// in more than one subtable adds up, as in tdewolff/font.
func readPairs(out map[[2]gtfont.GID]int16, b []byte, q, n int) {
	for j := 0; j < n && q+6 <= len(b); j, q = j+1, q+6 {
		k := [2]gtfont.GID{gtfont.GID(binary.BigEndian.Uint16(b[q:])), gtfont.GID(binary.BigEndian.Uint16(b[q+2:]))}
		out[k] += int16(binary.BigEndian.Uint16(b[q+4:]))
	}
}

// hasFeature reports whether a GPOS or GSUB table lists the feature tag.
func hasFeature(b []byte, tag string) bool {
	if len(b) < 8 {
		return false
	}
	list := int(binary.BigEndian.Uint16(b[6:]))
	if list+2 > len(b) {
		return false
	}
	n := int(binary.BigEndian.Uint16(b[list:]))
	for i := 0; i < n; i++ {
		p := list + 2 + 6*i
		if p+4 > len(b) {
			break
		}
		if string(b[p:p+4]) == tag {
			return true
		}
	}
	return false
}

// Find returns every installed face of a family with the given style ("Regular" or "Bold"),
// from the font folders deckfold itself searches. A family can be installed more than once
// (macOS and Microsoft Office both bring Arial), so it returns all of them, sorted by path.
func Find(family, style string) []*Font {
	var out []*Font
	for _, dir := range fonts.FontDirs() {
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			switch strings.ToLower(filepath.Ext(path)) {
			case ".ttf", ".otf", ".ttc":
			default:
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			for i, off := range faceOffsets(data) {
				fam, st, _ := names(table(data, off, "name"))
				if !strings.EqualFold(fam, family) || !strings.EqualFold(st, style) {
					continue
				}
				f, err := Parse(data, i)
				if err != nil {
					continue
				}
				f.Path = path
				out = append(out, f)
			}
			return nil
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// faceOffsets returns the offset of the table directory of every face in a font file.
func faceOffsets(data []byte) []int {
	if len(data) < 12 {
		return nil
	}
	if !bytes.HasPrefix(data, []byte("ttcf")) {
		return []int{0}
	}
	n := int(binary.BigEndian.Uint32(data[8:]))
	var out []int
	for i := 0; i < n && 16+4*i <= len(data); i++ {
		out = append(out, int(binary.BigEndian.Uint32(data[12+4*i:])))
	}
	return out
}

// table returns a table of the face whose table directory starts at off.
func table(data []byte, off int, tag string) []byte {
	if off+12 > len(data) {
		return nil
	}
	n := int(binary.BigEndian.Uint16(data[off+4:]))
	for i := 0; i < n; i++ {
		rec := off + 12 + 16*i
		if rec+16 > len(data) {
			break
		}
		if string(data[rec:rec+4]) == tag {
			o, l := int(binary.BigEndian.Uint32(data[rec+8:])), int(binary.BigEndian.Uint32(data[rec+12:]))
			if o+l <= len(data) {
				return data[o : o+l]
			}
		}
	}
	return nil
}

// Words are the candidates Choose picks from: plain English words, the first half with letter
// pairs that fonts usually kern (T, V, W, Y and A next to round or diagonal letters), the second
// half with stems only, which fonts usually leave alone.
var Words = []string{
	"AVATAR", "Avocado", "Tomato", "Table", "Text", "Type", "Toy", "Water", "Wave", "Voyage", "Year", "Yoyo",
	"Vault", "Away", "Today", "Taxes", "Layout", "Tower", "Vote", "Wolf", "PAVE", "WAVY", "Tea", "Party",
	"Keyway", "Royalty", "Typeface", "Tornado", "Yacht", "Vacuum", "Weave", "Aviary", "Valve", "Yawn",
	"Avenue", "Teapot", "Tutor", "Fast", "Raw", "Ivory",
	"noon", "minimum", "hidden", "common", "mind", "union", "dune", "mood", "bend", "hound", "dim", "inn",
	"nine", "million", "dominion", "human", "medium", "idiom", "demon", "blend", "hummed", "inbound",
}

// Choice is what Choose found among the candidate words.
type Choice struct {
	Kerned  []string // kerning of at least the wanted share of the width, in every font that kerns
	Control []string // no kerning at all, in any table of any font
	Differ  []string // the kern table and GPOS disagree in at least one font that has both
}

// Share returns the kerning of text as a share of its plain width, from the table a font has:
// the kern table, or GPOS for a font without one.
func (f *Font) Share(text string) float64 {
	w := f.Plain(text)
	if w == 0 {
		return 0
	}
	k := f.Kern(text)
	if f.KernVersion < 0 {
		k = f.GPOS(text)
	}
	return k / w
}

// Choose sorts candidate words by what the fonts say about them. A word counts as kerned when
// its kerning is at least min of its width (as a share, for example 0.01) in every font that has
// kerning data; fonts without any are left out of that rule. Words a font has no glyphs for are
// skipped.
func Choose(fs []*Font, candidates []string, min float64) Choice {
	var c Choice
	for _, w := range candidates {
		kerned, control, differ, usable := true, true, false, true
		for _, f := range fs {
			if !f.Has(w) {
				usable = false
				break
			}
			k, g := f.Kern(w), f.GPOS(w)
			if k != 0 || g != 0 {
				control = false
			}
			if f.KernVersion >= 0 || f.GPOSKern {
				if s := f.Share(w); s > -min && s < min {
					kerned = false
				}
			}
			if f.KernVersion >= 0 && f.GPOSKern {
				if d := (k - g) / f.Plain(w); d > 0.005 || d < -0.005 {
					differ = true
				}
			}
		}
		if !usable {
			continue
		}
		if kerned && !control {
			c.Kerned = append(c.Kerned, w)
		}
		if control {
			c.Control = append(c.Control, w)
		}
		if differ {
			c.Differ = append(c.Differ, w)
		}
	}
	return c
}
