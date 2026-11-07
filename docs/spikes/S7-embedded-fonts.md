# S7: Fonts embedded in PowerPoint files

Checked on 2026-11-07 with 31 PowerPoint files (46 for the MTX decoder) from a private collection (not part of the repository) and the fonts installed with PowerPoint 16.114.4.

## Format of `.fntdata`

PowerPoint lists embedded fonts in `p:embeddedFontLst` of `ppt/presentation.xml`, one part per style (`ppt/fonts/fontN.fntdata`). In every file of the collection that embeds fonts, these parts are Embedded OpenType (EOT) files with MicroType Express (MTX) compression. `tdewolff/font` reads EOT, but not the compressed variant; it reports "EOT compression not supported". All 94 font files seen use flags 0x4 (compressed), some also 0x1 (subset); none is XOR-obfuscated.

There is no GUID obfuscation as in Word's `.odttf` files.

## MTX decoder

`internal/pptx/fonts/mtx` decodes MTX, written from the W3C submission of 5 March 2008 (https://www.w3.org/Submission/MTX/). The submission is complete enough for a decoder: chapter 3 and the sample code in appendix C cover LZCOMP with its three adaptive Huffman trees, the preloaded dictionary and the run-length stage, and chapters 5 to 7 cover the Compact Table Format. Monotype licenses the format and the sample code royalty-free (see `THIRD_PARTY_LICENSES.md`). The open decoders `libeot` and `mtx-decompressor` are MPL-2.0 and were not used; sfntly has only an encoder.

What the submission leaves open, and what the real files showed (checked on 2026-11-07):

- Where the prose and the code of the submission disagree (the byte ranges of the `cvt` encoding, the stop bit of the copy length), the code is right.
- An empty glyph is a single `numContours` of 0 in the glyph stream, without push count or code size.
- The table of section 5.11 is the same as the triplet table of WOFF2, which comes from MTX: all 128 rows give the same values as the WOFF2 formula.
- Some embedded fonts are OpenType with CFF outlines. MTX then only compresses the tables; there is no `glyf`.
- One font had an empty `prep` table at the end of the file, which `tdewolff/font` rejects; the decoder leaves empty tables out.

The decoder rebuilds `glyf` with new push instructions for the separated push data, rebuilds `loca` in the long format and decodes `cvt`. It leaves out `hdmx` and `VDMX`, which only cache hinted metrics, and `DSIG`, whose signature no longer matches.

Results with 46 PowerPoint files from a private collection (42 of them embed fonts, 94 different font files):

- All 94 decode, and every glyph of the result parses.
- 38 of them are installed on the test machine in the same version (same name ID 5). For all 38, every glyph reachable through `cmap` matches the installed copy: outline points, on-curve flags, contour ends, bounding box, advance width, and the glyph program (push values and the instructions after them). The other fonts were installed in a different version (for example Arial from Windows and from macOS) or not at all.
- After conversion, Chrome loads the embedded fonts in all 38 converted decks that use them, without OTS errors. The warnings about unreadable embedded fonts are gone.

There is no MTX encoder to make public test files with. The unit tests in `internal/pptx/fonts/mtx` build LZCOMP data with a small encoder in the test file; a local test (`TestEmbeddedFontsInCorpus`) decodes the fonts of the decks in `testdata/pptx`.

## Aptos and the Office fonts

- PowerPoint for Mac ships its fonts in `/Applications/Microsoft PowerPoint.app/Contents/Resources/DFonts`, among them Aptos (the Office default since 2023) in all weights.
- Aptos has `fsType` 0x0008 (editable embedding), so deckfold may embed a subset of it.
- The Office font cache in `~/Library/Group Containers/UBF8T346G9.Office/FontCache` (cloud fonts) cannot be read by a command-line program without an explicit macOS privacy permission. The font scanner skips folders it cannot read.
- Without Office (Linux, Windows without Office) there is no metric-compatible substitute for Aptos; deckfold uses Carlito and warns that line breaks can differ from PowerPoint.
