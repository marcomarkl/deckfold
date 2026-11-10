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
- PowerPoint does kern Calibri (the shorter lines of the wrong reference). deckfold measures and draws every font without kerning, so a deck in Calibri, converted on a machine that has Calibri, has lines up to 0.7 % longer than PowerPoint's. That is not measured against a reference yet.

**The Windows runner measures lower.** The 85 slides that the Windows runner (Chrome, CI run 42807437229) and the Mac both compared on 2026-11-10 are 0.001 lower on the runner on average, and list slides up to 0.009 lower (slide 2 of `basic`: 0.969 on the Mac, 0.960 on the runner). A reference that passes on the Mac within 0.01 of the threshold can fail in CI, as the 0.954 above did.

## Still open

- A comparison of the 3D shapes with WebGL in a browser with a GPU: the tests draw WebGL with SwiftShader.
