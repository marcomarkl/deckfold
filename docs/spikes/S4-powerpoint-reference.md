# S4: Reference images from PowerPoint

Checked on 2026-11-07 with Microsoft PowerPoint 16.114.4 on macOS 27.2.

## The workflow that works

`scripts/pptxref/pptxref.sh` opens each feature deck in PowerPoint through AppleScript, saves it again (`save as Open XML presentation`), exports it as PDF (`save as PDF`) and renders one PNG per page with PDFKit (`rasterize.swift`), at the slide size in CSS pixels. All nine feature decks went through it in about 100 seconds. PowerPoint opened every generated deck without a repair prompt.

Findings on the way:

- **Start dialogs block Apple events.** In a first attempt PowerPoint did not answer `make new presentation` within two minutes (`-1712`), probably because a start dialog was open. In a later session it answered at once.
- **`save as PNG` writes nothing.** It reports success for a file path and for a folder path, but no image appears anywhere in the sandbox or in `~/Documents`. `save as PDF` works, so the PNGs are made from the PDF.
- **The sandbox asks for new files.** PowerPoint shows a "Grant Access" dialog for files it has not opened before. A first version of the script used a new folder per run and made PowerPoint ask for every deck. The script now uses one folder with fixed file names, but granting that folder in the dialog did not stop the dialogs for new decks in a test on 2026-11-07. The script cannot work inside PowerPoint's own container, which would avoid the dialogs: macOS refuses other programs write access to it. So the dialogs have to be confirmed during a run.
- **Hidden slides are left out of the PDF**, so there is one page per shown slide, the same count as in the deckfold output.
- **Fonts have to be installed.** Without Carlito and Liberation in `~/Library/Fonts`, PowerPoint draws the decks with Calibri and Arial (metric-compatible, but different glyphs). The bundled fonts are now installed there for the reference.
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

## Still open

- The two decks that should come from PowerPoint itself (animations and transitions as PowerPoint writes them). The scripting dictionary has `add effect` and `slide show transition`, so they can probably be built by script.
- A comparison in Chromium on Linux; the CI job runs the same test there.
