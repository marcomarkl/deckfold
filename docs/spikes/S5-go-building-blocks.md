# S5: Go building blocks for the PPTX renderer

Checked on 2026-11-07 with Go 1.27.3.

## Text shaping and line breaking

`github.com/go-text/typesetting` v0.3.5 is dual-licensed under the Unlicense and BSD-3-Clause. It contains:

- `harfbuzz` and `shaping`: a Go port of HarfBuzz for glyph shaping (kerning, ligatures, complex scripts)
- `bidi`: the Unicode bidirectional algorithm, needed for right-to-left text
- `segmenter`: Unicode line, word and grapheme segmentation (UAX #14, UAX #29)
- `font`: an OpenType reader with access to metrics and the `OS/2` table
- `fontscan`: discovery of installed system fonts on macOS, Windows and Linux

It would cover shaping and line breaking for every script. The first version does not use it yet: deckfold measures text with the advance widths of the font (without kerning, which matches the subsets, see below) and breaks lines at spaces, hyphens, slashes and between CJK characters. Right-to-left text and complex scripts (Arabic, Indic) are not shaped yet; `go-text/typesetting` is the candidate for that.

## Font subsetting

`go-text/typesetting` cannot write fonts. `github.com/tdewolff/font` (MIT, maintained, last push October 2026) has `SFNT.Subset`, which handles TrueType (`glyf`/`loca`) and CFF outlines; only CFF2 is not supported. It renumbers the glyphs and by default keeps only the tables a renderer needs, so `GSUB` and `GPOS` are dropped.

Consequences:

- Without `GPOS` the browser draws the subset without kerning. The layout must measure text the same way, or place text in smaller units (words) so that the difference cannot add up over a line. The first version measures without kerning; whether that matches PowerPoint closely enough is measured once the PowerPoint reference images exist (S4).
- CFF2 fonts (variable `.otf`) are embedded whole.
- The `name` table of every subset gets a new family name (see below).

## Reserved Font Names

Both bundled fallback families carry Reserved Font Names:

| Family | License | Reserved Font Name |
|---|---|---|
| Carlito (metric-compatible with Calibri) | OFL 1.1 | "Carlito" |
| Liberation Sans, Serif, Mono 2.1.5 (metric-compatible with Arial, Times New Roman, Courier New) | OFL 1.1 | "Liberation", plus "Arimo", "Tinos", "Cousine" for the Google-digitized data |

The OFL FAQ treats a font embedded in a document as unmodified, also as a subset (entry 1.12), but it treats a subset web font as a Modified Version that must not use the Reserved Font Name (entry 2.6). A deckfold output is a document that loads its fonts through `@font-face`, so it sits between the two cases. deckfold takes the stricter reading: every subset gets a neutral family name in its `name` table (for example `df-<hash>`). The CSS refers to that name anyway, so nothing visible changes.

## Binary size

The fallback fonts add to every binary:

| Fonts | Files | Uncompressed |
|---|---|---|
| Liberation 2.1.5 (Sans, Serif, Mono, four styles each) | 12 | 4.4 MB |
| Carlito (four styles) | 4 | 2.7 MB |

That is about 7.1 MB per binary. Stored gzip-compressed in the binary and unpacked on first use, it should come to roughly half; the exact number is measured when the fonts are added.

## Formulas of the preset geometries

The guide formulas in `presetShapeDefinitions.xml` use 17 operators (`*/`, `+-`, `+/`, `?:`, `abs`, `at2`, `cat2`, `cos`, `max`, `min`, `mod`, `pin`, `sat2`, `sin`, `sqrt`, `tan`, `val`) on integer arguments with angles in 60000ths of a degree. deckfold evaluates them with its own small interpreter instead of a library.
