# S4: Reference images from PowerPoint

Checked on 2026-11-07 with Microsoft PowerPoint 16.114.4 on macOS 27.2.

## The workflow that works

`scripts/pptxref/pptxref.sh` opens each feature deck in PowerPoint through AppleScript, saves it again (`save as Open XML presentation`), exports it as PDF (`save as PDF`) and renders one PNG per page with PDFKit (`rasterize.swift`), at the slide size in CSS pixels. All nine feature decks went through it in about 100 seconds. PowerPoint opened every generated deck without a repair prompt.

Findings on the way:

- **Start dialogs block Apple events.** In a first attempt PowerPoint did not answer `make new presentation` within two minutes (`-1712`), probably because a start dialog was open. In a later session it answered at once.
- **`save as PNG` writes nothing.** It reports success for a file path and for a folder path, but no image appears anywhere in the sandbox or in `~/Documents`. `save as PDF` works, so the PNGs are made from the PDF.
- **The sandbox asks for new files.** PowerPoint shows a "Grant Access" dialog for files it has not opened before. A first version of the script used a new folder per run and made PowerPoint ask for every deck. The script now uses one folder with fixed file names, but granting that folder in the dialog did not stop the dialogs for new decks in a test on 2026-11-07. The script cannot work inside PowerPoint's own container, which would avoid the dialogs: macOS refuses other programs write access to it. So the dialogs have to be confirmed during a run.
- **Hidden slides are left out of the PDF**, so there is one page per shown slide, the same count as in the deckfold output.
- **Fonts have to be installed.** Without Carlito and Liberation in `~/Library/Fonts`, PowerPoint draws the decks with Calibri and Arial (metric-compatible, but different glyphs). `pptxref.sh` therefore copies the missing bundled fonts there for the run and removes them again at the end.
- **The images show the editing view**: objects with entrance or exit animations are visible. The browser test therefore compares the player's static mode (`__dfPptx.showAll`).

## What the comparison found

The first run of `tests/e2e/pptx-fidelity.spec.mjs` led to three changes in the renderer:

- **Shrink on overflow uses the stored scale.** With `a:normAutofit` and no `fontScale`, PowerPoint draws the text at its full size and lets it overflow; it shrinks text only while someone edits it. deckfold had recomputed the scale and drew the text smaller. It now uses the stored values only.
- **Line pitch is 1.2 times the font size.** PowerPoint's single line spacing does not follow the font's ascent and descent: with Carlito (ascent plus descent 1.22 em) every line came out about 0.7 px too far down at 28 pt.
- **Chart defaults.** For chart elements without `c:spPr`, PowerPoint draws black gridlines and axes with tick marks, smoothed lines with automatic marker symbols, an 18 pt title, axis steps after Excel's rule (maximum plus 5 %, then a step of 1, 2 or 5 times a power of ten) and 10 % steps on 100 % axes. A doughnut without `c:holeSize` has no hole.

SSIM after these changes, per slide:

| Deck | Slides |
|---|---|
| animations | 0.999, 0.999, 0.988, 0.999 |
| basic | 0.980, 0.969, 0.997 |
| charts | 0.677, 0.918 (exceptions, see below) |
| hostile | 0.978 |
| media | 0.988, 0.985 |
| shapes | 0.993, 0.966 |
| tables | 0.979 |
| text | 0.979 |

The threshold stays at 0.95. The two chart slides are exceptions in the test: values, axis scale, legend and colors match, but PowerPoint lays out the plot area of an automatic layout a few pixels differently, and SSIM in 8 x 8 windows reacts strongly to many one-pixel lines that are offset.

## Second and third run: the decks for the closed gaps

Twelve more feature decks went through the script on 2026-11-07, in two announced runs: `gifs`, `triggers`, `shapes3d`, `bullets`, `effects` and `fontembed` (a deck with an embedded OFL font), then `wmf`, `text2`, `morph`, `transitions3d`, `charts2` and `ink`.

Findings:

- **PowerPoint reported `transitions3d` as damaged.** Saving the repaired deck gave an empty file. The cause was the form of the PowerPoint 2010 and 2013 transitions: PowerPoint accepts only the form it writes itself, with explicit directions (`p14:flip dir="l"`, `p15:wind invX="1"`), `p15:prstTrans prst="..."` for the newer presets and no `p14:dur` beside them. The form came from transitions that PowerPoint set itself through AppleScript; the generator writes it now and PowerPoint opens the deck without a prompt.
- **PowerPoint dropped the ink.** It keeps an InkML part only when the `p14:contentPart` points to it with the `customXml` relationship type. PowerPoint stretches the strokes to the frame of the part in both directions and draws them at the brush width in centimeters, whatever the frame's size.
- **Picture bullets** are 0.65 times the font size high and do not count for the line pitch or the first baseline (0.916 before, 0.959 after).
- **3D shapes:** the isometric camera presets are a rotation of 45 degrees about z and 54.74 degrees about x, the shape turns about its front face, and a fill light from the opposite side keeps the dark faces from going black. Slide 3 of `shapes3d` went from 0.905 to 0.961 with these changes.
- **Charts with an automatic layout and chart text without `c:txPr`** differ in the font and in the plot area. The data, axes and colors match; the slides of `charts2` are exceptions in the test.
- **The baseline of Liberation Serif** (the embedded font in `fontembed`) is 1 to 3 px lower in PowerPoint than in deckfold; the glyphs match.
- PowerPoint built `fixtures/pptx/powerpoint/animations.pptx` and `transitions.pptx` itself (`scripts/pptxref/ppdecks.sh`). Its scripting dictionary sets neither trigger shapes nor delays.

SSIM per slide after all three runs (Chrome on macOS, measured on 2026-11-07):

| Deck | Slides |
|---|---|
| bullets | 0.959 |
| charts2 | 0.74 to 0.90 (exceptions) |
| effects | 0.981, 0.985 |
| fontembed | 0.918 (exception) |
| gifs | 0.996, 0.990 |
| ink | 1.000 |
| morph | 0.999, 0.999 |
| shapes3d | 0.953, 0.966, 0.961 |
| text2 | 0.958, 0.988 |
| transitions3d | 0.983 to 0.999 (38 slides) |
| triggers | 0.998, 0.999 |
| wmf | 0.978 |

The decks of the first run kept their values. The images show the editing view, so they cover the static drawing only; the transitions and Morph are checked by the browser tests in `tests/e2e/pptx.spec.mjs`.

## A reference drawn with another font (2026-11-10)

The first reference for the content deck compared at SSIM 0.954 on its list slide and failed on the Windows runner at 0.948. Every line of text was 1 to 4 px shorter in PowerPoint's image than in deckfold's. The cause was the reference: the PDF of that run names Calibri where the PDFs of every earlier run name Carlito (`pdffonts`). PowerPoint had not found the bundled fonts, replaced them without a word, and kerned Calibri. Why it did not find them in that run is not known; a second run minutes later found them.

`pptxref.sh` now waits until the system lists the bundled fonts before it starts PowerPoint, and it reads the font names from every exported PDF. When the deck names a bundled font that the PDF does not have, while the PDF has a font the deck does not name, it writes no reference for that deck. With the right font the list slide compares at 0.972.

Two things this showed about text:

- PowerPoint does not kern Carlito. Carlito has its kerning in GPOS only (no `kern` table), and deckfold's unkerned text matches PowerPoint's to SSIM 0.997 on a text slide. Kerning deckfold's text with the GPOS pairs made 19 of 87 slides worse and was taken back.
- PowerPoint does kern Calibri (the shorter lines of the wrong reference). deckfold measures and draws every font without kerning, so a deck in Calibri, converted on a machine that has Calibri, has lines up to 0.7 % longer than PowerPoint's. The section "How PowerPoint kerns" below has the measurement.

**The Windows runner measures lower.** The 85 slides that the Windows runner (Chrome, CI run 42807437229) and the Mac both compared on 2026-11-10 are 0.001 lower on the runner on average, and list slides up to 0.009 lower (slide 2 of `basic`: 0.969 on the Mac, 0.960 on the runner). A reference that passes on the Mac within 0.01 of the threshold can fail in CI, as the 0.954 above did.

## How PowerPoint kerns (2026-11-10)

Measured with PowerPoint 16.114.4 for Mac on macOS 27.2, on its PDF export, which is also where the reference images come from. Whether the slide show sets text the same way is not measured.

**The rule.** PowerPoint kerns a run when its font size is at least the value of its `kern` attribute, and it takes the pairs from the legacy `kern` table of the font. It does not read the kerning in GPOS.

**How it was measured.** Two decks from `go run ./scripts/pptxgen`: the public `kerning` deck in Liberation Sans, Liberation Serif and Carlito (60 rows), and a local one with 14 installed fonts that cannot go into the repository (`-kernlocal`, 84 rows). Every row is one line of the same words in its own text box. Most words have letter pairs the font kerns, three have none. Each row that could be kerned has a twin, the same line with `kern="0"`. `scripts/kernmeasure` reads the word boxes of the PDF (`pdftotext -bbox`) and compares the width of every word with its width in the twin: the difference is the kerning PowerPoint applied. It then holds that difference against what the font's `kern` table and its GPOS say.

The twins are there because a first run without them could not be judged. PowerPoint's word widths lie up to 0.4 % from the sum of the advance widths at 28 pt and up to 0.9 % at 10 to 12 pt, also without any kerning, and more for words with capitals than for the lowercase control words. A rule built on the scatter of the control words called a third of the rows unclear. Between a row and its twin the control words differ by 0.011 pt at most.

**What `kern` does** (Liberation Sans and Serif, which have a `kern` table):

| Row | Kerning applied, as a share of the `kern` table |
|---|---|
| `kern="1200"` at 28 pt | 99.8 % and 100.0 % |
| `kern="1200"` at 12 pt | 100.4 % and 99.8 % |
| `kern="1200"` at 10 pt | 0.0 % |
| `kern="2000"` at 28 pt | 99.8 % and 100.0 % |
| `kern="2000"` at 18 pt | 0.0 % |
| no `kern` on the run or in any style it inherits | 0.0 % |
| `kern="1200"` only in the list style of the shape | 99.8 % and 100.0 % |
| bold | 100.6 % and 99.9 % |
| with letter spacing of 3 pt | 99.5 % and 100.1 % |
| the first pair of every word across a run boundary | 99.9 % and 100.1 % |
| a paragraph of three lines | 100.1 % and 100.0 % |
| 28 pt in a box with a stored `fontScale` of 40 % | 0.0 % |

- `kern` is a threshold and it includes its own value. Without the attribute PowerPoint does not kern.
- Pairs with the space are applied like the others, and so are pairs whose letters are in different runs.
- Lines break by the kerned widths: the paragraph in Liberation Serif breaks before other words than its twin with `kern="0"`.
- The size PowerPoint compares is the size it draws. The box with `fontScale="40000"` was drawn at 11.0 pt (10.95 to 11.01 pt from the word widths, where 40 % of 28 pt is 11.2 pt) and was not kerned with `kern="1200"`.
- PowerPoint kept every `kern` value when it saved the deck and added none.

**Which table.** The same pair of rows, `kern="0"` and `kern="1200"` at 28 pt, for every font:

| Font | Kerning data | PowerPoint |
|---|---|---|
| Liberation Sans, Liberation Serif | `kern` table and GPOS, same values | kerned, 99.8 to 100.0 % |
| Calibri | `kern` table and GPOS, same values | kerned, 99.9 % |
| Arial | `kern` table and GPOS, same values | kerned, 99.9 % |
| Aptos | `kern` table and GPOS, different for some pairs | kerned, 100.1 % of the `kern` table (GPOS would be 104.9 %) |
| Verdana | `kern` table, GPOS without kerning | kerned, 99.6 % |
| Gill Sans MT, Helvetica Neue | `kern` table only | kerned, 99.8 % and 100.0 % |
| Helvetica | `kern` table in Apple's format (version 1) | kerned, 99.9 % |
| Carlito, Lato, Roboto, Montserrat, DM Sans, Nunito Sans | GPOS only | not kerned, 0.0 % |
| Century Gothic, Poppins | none | rows equal their twins |

In Aptos the word "Keyway" is 0.666 pt narrower with kerning at 28 pt. Its `kern` table says 0.669 pt, its GPOS 0.109 pt. At 10 pt with `kern="1200"` no font was kerned. The bold faces behave like the regular ones; Verdana Bold has no pairs for the words of its row and says nothing.

**What this explains.** The three things known before fit the rule. PowerPoint draws Carlito without kerning, and deckfold's unkerned Carlito matches it (SSIM 0.997): Carlito has no `kern` table. The pair "Te" in the `fontembed` deck is not kerned: the embedded subset has no `kern` table. The reference PowerPoint drew in Calibri by mistake had shorter lines: Calibri has a `kern` table, and the styles of every feature deck have `kern="1200"`.

**Substitutes.** deckfold draws Carlito for Calibri and Liberation for Arial, Helvetica and Times New Roman when the original is not installed. Their kerning is the original's: over all 9025 pairs of printable ASCII characters, Carlito's GPOS gives what Calibri's `kern` table gives (611 kerned pairs), and the `kern` tables of Liberation Sans and Liberation Serif give what those of Arial and Helvetica (96 pairs each) and Times New Roman (104 pairs) give, with no pair further apart than 0.001 em. For the 87 Latin paragraphs of the feature decks, a substitute without kerning is 0.2 to 0.5 % wider than PowerPoint's line on average and up to 5.5 % for a line of kerned words; with its own kerning it has the same width, for Helvetica within 0.02 % (`go run ./scripts/kernmeasure -substitutes`).

That holds for the regular faces, and for bold, italic and bold italic of Arial and Times New Roman. Two cases differ. Carlito Bold kerns 51 of the 9025 pairs that Calibri Bold leaves alone (most with an apostrophe or a hyphen, a few between letters such as "Lo" and "oV"); the other 609 pairs agree. Helvetica Oblique and Bold Oblique have other pairs than Liberation Sans Italic (72 and 59 differ). deckfold kerns Carlito Bold for Calibri Bold all the same (the maintainer's choice: without kerning all 609 pairs would be off, with it 51 rare ones are tighter than in PowerPoint). Slanted text in Helvetica stays unkerned when Helvetica is missing.

**Not measured.**

- Fonts with CFF outlines, which cannot have a `kern` table. PowerPoint drew STIXGeneral, the one at hand, in Calibri.
- PowerPoint for Windows.
- Hebrew, Arabic and other shaped text.

**Two things about the libraries.** go-text/typesetting, which deckfold uses to measure shaped text, misses GPOS pairs in some fonts: in Montserrat it kerns "AV" but not "VA" or "To", and it misses pairs in Calibri. CoreText and the fonts' own tables have them, so the measurement asks CoreText for GPOS (for the bundled fonts a test holds go-text against it). tdewolff/font, which deckfold reads fonts with, refuses Helvetica because of its Apple `kern` table; deckfold therefore never draws Helvetica itself and takes Liberation Sans.

**What deckfold does with it** (decision P19). The renderer kerns by this rule: `kern` at the size it draws, pairs from the `kern` table of the face (`fonts.Face.Kern`), across runs and with the space, and it breaks lines by the kerned widths. In the SVG every kerned pair is a `tspan` with `dx`, and the slide sets `font-kerning:none`. Two tests hold the result against PowerPoint's word positions (`fixtures/pptx/ref/kerning/words.json`): one reads the SVG (`internal/pptx/kerning_test.go`), one asks the browser where it draws every letter (`tests/e2e/pptx.spec.mjs`). Both fail without kerning for exactly the 16 rows PowerPoint kerns.

The browser's own kerning had to go: Chrome kerns a font that is embedded whole with its GPOS (a line of 28 pt Liberation Sans came out 395.6 px wide instead of the 421.2 px deckfold had measured), which deckfold does to fonts that also shape Hebrew or Arabic text. WebKit did not kern that line.

No other feature deck changes: 20 of the 22 have the same slides as before, and `text2` differs only by `font-kerning:auto` on its Hebrew line. In the 288 slides of the local corpus, 14 change, with 28 kerned pieces of text among 4445, and no slide gets other line breaks.

**What the image comparison said.** With kerning, slide 1 of the `kerning` deck (Liberation Sans) went from SSIM 0.749 to 0.828 and reached 0.962 when deckfold's image was moved down by 2 px. Slide 2 (Liberation Serif) was at 0.791 and reached 0.949 moved down by 3 px; slide 3 (Carlito, nothing kerned) was at 0.943 and reached 0.977 moved up by 1 px. No slide gained from a horizontal shift. The rest was the place of the baseline, which the next section is about.

## Where PowerPoint puts the baseline (2026-11-10)

deckfold put the first baseline of a line one ascent below its top (the ascent of the font times the line spacing). PowerPoint does not: its baselines were 2 px lower for Liberation Sans, 3 px lower for Liberation Serif and 1 px higher for Carlito, and several pixels off under a line spacing of 150 %.

**How it was measured.** `pdftocairo -svg` gives the baseline of every glyph of a PDF. Three PDFs of PowerPoint 16.114.4 for Mac were read: the two kerning decks (17 fonts, at 28 pt and 10 pt, three of them also at 12 and 18 pt, in boxes without insets) and a deck made for this, `go run ./scripts/pptxgen -baselinelocal tests/.out/baseline` with `pptxref.sh --pdf`: 44 boxes of three lines in Liberation Sans and Carlito, with line spacings of 70 to 200 %, of 20 to 50 pt, a stored reduction of the line spacing, a stored font scale and the three vertical anchors.

**The rule.**

- A single line is 1.2 times the font size, whatever the font (known since 2026-11-07).
- The font divides that line by the ascender and the descender of its `hhea` table: at 28 pt a line is 33.6 pt, of which Liberation Sans has 27.2 pt above the baseline and 6.4 pt below, Carlito 26.2 pt and 7.4 pt. The part below the baseline stays the same under every line spacing.
- The line spacing is the distance from one baseline to the next: the single line times the percentage, or the points. A stored reduction (`lnSpcReduction`) is taken off the percentage, and a stored font scale shrinks the font size first.
- The baseline lies at least three quarters of that distance below the top of its line. Otherwise it leaves the font its part below, and never more room above than a single line has. So a line spacing below 100 % takes the height off the top of the line, and above about 108 % the place of the first baseline no longer depends on the font: 28 pt at 150 % has it 38 pt down in both fonts.
- The text ends below its last baseline by the font's part, not by the rest of the line spacing. Centered and bottom-aligned text is placed with that height.
- Every baseline is a whole number of points below the top of the text: three lines of 28 pt Liberation Sans stand at 27, 61 and 94 pt, where the line spacing gives 27.2, 60.8 and 94.4 pt. Whether that is PowerPoint's layout or its PDF export cannot be told from a PDF.

**How well it fits.** Of the 57 combinations of font, style and size in the kerning decks, 56 have the first baseline where the rule puts it with the `hhea` metrics; with the `OS/2` win metrics 47, with the typo metrics 22. Helvetica, Nunito Sans and Poppins tell the tables apart. The one that does not fit is Century Gothic Regular at 28 pt, which is 1 pt higher than its bold with the same metrics. All 132 baselines of the baseline deck are within 0.12 pt of the rule (the PDF has positions on a grid of 0.24 pt). Across the 169 lines of the feature decks whose PDFs were at hand, the mean distance of deckfold's baseline from PowerPoint's went from 1.01 pt to 0.07 pt.

`internal/pptx/baseline_test.go` holds the renderer against the 132 measured baselines; with the rule from before, 90 of them fail.

**What does not fit.** The embedded font of the `fontembed` deck: PowerPoint puts its baselines 1 pt higher than the same metrics give for an installed font. That deck stays an exception in the image comparison.

**The image comparison after both changes.** On the Mac the `kerning` deck is at 0.958, 0.962, 0.985, 0.946, 0.951 and 0.975. Moving the image no longer helps: the ends of lines differ by a pixel because PowerPoint's word widths are not exactly the font's, and a baseline on a whole point is on a third of a pixel.

Chrome on the CI runners is lower for this deck: 0.943, 0.945, 0.976, 0.935, 0.939 and 0.977 on Linux, 0.944, 0.945, 0.971, 0.943, 0.948 and 0.974 on Windows. That is not the kerning. Slide 5 has the paragraphs of slide 4 without a kerned line and loses as much, and slide 3, in Carlito with nothing kerned, loses up to 0.014. The other 90 slides of the feature decks differ from the Mac by 0.0006 on average and by 0.007 at most. The slides of this deck are full of text, down to 10 pt, and SSIM follows how a browser draws letters more closely there. The deck therefore has a threshold of its own in the test, 0.92, and no exceptions. That still catches the deck drawn another way: with the kerning switched off, slides 1, 2 and 4 are at 0.828, 0.853 and 0.828, and with kerning but the baselines of before slide 1 was at 0.828 as well. Of the 88 slides of the other decks, 11 gain and 49 lose, none by more than 0.009 (`text` slide 1 from 0.979 to 0.970, `basic` slide 1 from 0.980 to 0.998). Without the whole points, 13 gain and 26 lose, none by more than 0.005, and the `kerning` deck is at 0.952 to 0.981. Both are differences of a fraction of a pixel, which the browser and the PDF renderer draw differently. The whole points stay because they are what the PDFs have (`baselinesOnPoints` in `internal/pptx/render/text.go`).

## Still open

- A comparison of the 3D shapes with WebGL in a browser with a GPU: the tests draw WebGL with SwiftShader.
