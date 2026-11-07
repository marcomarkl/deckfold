# S7: Fonts embedded in PowerPoint files

Checked on 2026-11-07 with 31 PowerPoint files from a private collection (not part of the repository) and the fonts installed with PowerPoint 16.114.4.

## Format of `.fntdata`

PowerPoint lists embedded fonts in `p:embeddedFontLst` of `ppt/presentation.xml`, one part per style (`ppt/fonts/fontN.fntdata`). In every file of the collection that embeds fonts, these parts are Embedded OpenType (EOT) files with MicroType Express (MTX) compression. `tdewolff/font` reads EOT, but not the compressed variant; it reports "EOT compression not supported".

There is no GUID obfuscation as in Word's `.odttf` files.

## Consequence

The embedded stage of the font cascade (P5) needs an MTX decoder: the LZCOMP decompressor plus the Compact Table Format that MTX uses for `glyf`, `loca`, `cvt` and `hdmx`. Until then deckfold reports each embedded font it cannot read and continues with the next stage, installed fonts and then the bundled ones. In the collection, most embedded families were also installed on the machine, so the output still used the right fonts there.

## Aptos and the Office fonts

- PowerPoint for Mac ships its fonts in `/Applications/Microsoft PowerPoint.app/Contents/Resources/DFonts`, among them Aptos (the Office default since 2023) in all weights.
- Aptos has `fsType` 0x0008 (editable embedding), so deckfold may embed a subset of it.
- The Office font cache in `~/Library/Group Containers/UBF8T346G9.Office/FontCache` (cloud fonts) cannot be read by a command-line program without an explicit macOS privacy permission. The font scanner skips folders it cannot read.
- Without Office (Linux, Windows without Office) there is no metric-compatible substitute for Aptos; deckfold uses Carlito and warns that line breaks can differ from PowerPoint.
