package fonts

import (
	"bytes"
	"strings"

	"github.com/go-text/typesetting/di"
	gtfont "github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"
)

// PowerPoint kerns with the pairs of the legacy kern table of a font and does not read the
// kerning in GPOS (measured on 2026-11-10, docs/spikes/S4-powerpoint-reference.md, "How
// PowerPoint kerns"). A face therefore kerns with its kern table, when it has one.
//
// A bundled font that stands in for another family is a different matter: PowerPoint drew the
// original. For the pairs below the substitute has the kerning of the original's kern table,
// pair by pair over the printable ASCII characters (go run ./scripts/kernmeasure -substitutes):
// Liberation Sans and Liberation Serif in their own kern table, Carlito in its GPOS. Carlito
// itself has no kern table, so a deck that names Carlito is not kerned, as in PowerPoint.
//
// Carlito Bold is the one case that is not exact: it has the 609 pairs of Calibri Bold and 51
// more that Calibri Bold leaves alone (most with an apostrophe or a hyphen). It kerns anyway,
// the maintainer's choice on 2026-11-10: without kerning all 609 would be off.
type kernSource int

const (
	kernNone  kernSource = iota
	kernTable            // the kern table of the face
	kernGPOS             // the kern feature of the face's GPOS (Carlito for Calibri)
)

// kernedSubstitutes lists, by the family a deck names, where the kerning of its bundled
// substitute comes from and for which styles. Left out because they differ too much: the oblique
// faces of Helvetica (72 and 59 of some 95 pairs). Every other substituted family stays without
// kerning: nothing says its substitute kerns like the original.
var kernedSubstitutes = map[string]struct {
	source kernSource
	styles string
}{
	"calibri":         {kernGPOS, "regular bold italic boldItalic"},
	"arial":           {kernTable, "regular bold italic boldItalic"},
	"helvetica":       {kernTable, "regular bold"},
	"times new roman": {kernTable, "regular bold italic boldItalic"},
}

// setKern decides where the kerning of a face comes from. bundledName is the bundled font the
// face was loaded from, or "" for an embedded or installed font.
func (f *Face) setKern(bundledName string) {
	own := kernNone
	if f.sfnt.Kern != nil {
		own = kernTable
	}
	if bundledName == "" || strings.EqualFold(bundledName, f.Family) || strings.EqualFold(strings.ReplaceAll(bundledName, "Liberation", "Liberation "), f.Family) {
		f.kern = own
		return
	}
	f.kern = kernNone
	if s, ok := kernedSubstitutes[strings.ToLower(strings.TrimSpace(f.Family))]; ok {
		for _, st := range strings.Fields(s.styles) {
			if st == styleName(f.Bold, f.Italic) {
				f.kern = s.source
			}
		}
	}
}

// Kerns reports whether the face has kerning that PowerPoint applies.
func (f *Face) Kerns() bool { return f.kern != kernNone }

// Kern returns the kerning between two characters in em, as PowerPoint applies it when a run
// is kerned (0 for most pairs, and always 0 for a face that does not kern).
func (f *Face) Kern(a, b rune) float64 {
	switch f.kern {
	case kernTable:
		return float64(f.sfnt.Kerning(f.sfnt.GlyphIndex(a), f.sfnt.GlyphIndex(b))) / f.upem
	case kernGPOS:
		return f.gposPair(a, b)
	}
	return 0
}

// shaper returns the face for go-text/typesetting, loaded on first use.
func (f *Face) shaper() *gtfont.Face {
	if f.shapeFace == nil && !f.shapeFailed {
		gf, err := gtfont.ParseTTF(bytes.NewReader(f.data))
		if err != nil {
			f.shapeFailed = true
		} else {
			f.shapeFace = gf
		}
	}
	return f.shapeFace
}

// gposPair returns what the kern feature of GPOS does to the pair, in em: the width of the two
// characters shaped with the feature minus their width without it.
func (f *Face) gposPair(a, b rune) float64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := [2]rune{a, b}
	if v, ok := f.gpos[key]; ok {
		return v
	}
	v := 0.0
	if face := f.shaper(); face != nil {
		script := language.Latin
		if s := language.LookupScript(a); s != language.Common && s != language.Inherited {
			script = s
		}
		width := func(kern uint32) float64 {
			var sh shaping.HarfbuzzShaper
			out := sh.Shape(shaping.Input{Text: key[:], RunStart: 0, RunEnd: 2, Direction: di.DirectionLTR, Face: face,
				Size: fixed.I(1000), Script: script, Language: language.NewLanguage("en"),
				FontFeatures: []shaping.FontFeature{
					{Tag: ot.MustNewTag("kern"), Value: kern},
					// no ligatures: the text is drawn letter by letter
					{Tag: ot.MustNewTag("liga"), Value: 0}, {Tag: ot.MustNewTag("clig"), Value: 0},
					{Tag: ot.MustNewTag("calt"), Value: 0}, {Tag: ot.MustNewTag("dlig"), Value: 0},
				}})
			return float64(out.Advance) / 64 / 1000
		}
		v = width(1) - width(0)
	}
	if f.gpos == nil {
		f.gpos = map[[2]rune]float64{}
	}
	f.gpos[key] = v
	return v
}
