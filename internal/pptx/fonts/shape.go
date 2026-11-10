package fonts

import (
	"unicode"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"
)

// Complex reports whether text needs shaping: right-to-left scripts and scripts whose glyphs
// change with their neighbors (Arabic, Hebrew, Indic, Thai and others). Latin, Greek and
// Cyrillic text is measured from the advance widths as before.
func Complex(text string) bool {
	for _, r := range text {
		if r < 0x0590 {
			continue
		}
		switch {
		case r >= 0x0590 && r <= 0x08FF, // Hebrew, Arabic, Syriac, Thaana, NKo, Samaritan
			r >= 0x0900 && r <= 0x0DFF,                             // Indic scripts, Sinhala
			r >= 0x0E00 && r <= 0x0FFF,                             // Thai, Lao, Tibetan
			r >= 0x1000 && r <= 0x109F,                             // Myanmar
			r >= 0x1780 && r <= 0x17FF,                             // Khmer
			r >= 0xFB1D && r <= 0xFDFF, r >= 0xFE70 && r <= 0xFEFF: // presentation forms
			return true
		}
	}
	return false
}

// RTL reports whether text starts with a right-to-left letter (strong direction).
func RTL(text string) bool {
	for _, r := range text {
		switch {
		case r >= 0x0590 && r <= 0x08FF, r >= 0xFB1D && r <= 0xFDFF, r >= 0xFE70 && r <= 0xFEFF:
			return true
		case unicode.IsLetter(r):
			return false
		}
	}
	return false
}

// ShapedWidth returns the width of text in em, shaped with HarfBuzz (go-text/typesetting). A
// face that shapes text is embedded whole, so that the browser finds the same substitutions
// and positions (subsets drop the GSUB and GPOS tables). ok is false when the font cannot be
// shaped; the caller then measures advance widths.
func (f *Face) ShapedWidth(text string) (float64, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.shaper() == nil {
		return 0, false
	}
	f.noSubset = true
	rs := []rune(text)
	dir := di.DirectionLTR
	if RTL(text) {
		dir = di.DirectionRTL
	}
	script := language.Common
	for _, r := range rs {
		if s := language.LookupScript(r); s != language.Common && s != language.Inherited {
			script = s
			break
		}
	}
	var sh shaping.HarfbuzzShaper
	out := sh.Shape(shaping.Input{Text: rs, RunStart: 0, RunEnd: len(rs), Direction: dir, Face: f.shapeFace,
		Size: fixed.I(1000), Script: script})
	return float64(out.Advance) / 64 / 1000, true
}
