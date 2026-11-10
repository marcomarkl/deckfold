# Spike S1: Keynote 15.4 HTML export

Date: 2026-11-07. Keynote 15.4 (Keynote Creator Studio), macOS 27.2.

## What was done

- A PowerPoint file (34 slides, transitions only, no builds) was opened in Keynote and exported with
  AppleScript (`export d to POSIX file ... as HTML`). The export has 354 files and 8.9 MB.
- Keynote crashed on the following AppleScript `close d saving no`
  (`EXC_BREAKPOINT` in `NSApplicationUncaughtExceptionHandler` from `_NSAppleEventManagerGenericHandler`).
  The export was already complete at that point. Do not close documents through AppleScript.
- An export into `/private/tmp/...` failed with error `-609` (Keynote is sandboxed). Export into folders below `~/Documents`.

## Not done yet

- A native Keynote test deck with builds, Magic Move, video, audio and self-playing timing. It has to be made by hand.
- A Google Slides deck downloaded as PPTX.

Builds, media (video and audio), soundtrack and self-playing mode are therefore not verified against a real export.

## Findings

**Layout** (differs from older descriptions on the web: there is no `kpf.json`):

```
index.html                         loads assets/player/main.js only
assets/header.json  / header.jsonp header: slideList, slideCount, slideWidth/Height (720x405),
                                   showMode, autoplayTransitionDelay, autoplayBuildDelay,
                                   loopSlideshow, soundtrack, creator "Apple Keynote 15.4",
                                   majorVersion 1, minorVersion 2, fonts
assets/<SLIDE-UUID>/<UUID>.json / .jsonp   events (events[].effects[], Core Animation layers)
assets/<SLIDE-UUID>/assets/<UUID>.pdf / .pdfp   slide content as PDF (textures)
assets/<SLIDE-UUID>/thumbnail.jpeg         thumbnail
assets/player/main.js (2.4 MB)    player including pdf.js and embedded CMaps
assets/player/pdfjs/pdf.worker.min.js
assets/player/pdfjs/web/cmaps/*.bcmap (168 files, 1.6 MB)
assets/player/*.png                UI images
```

**How the player loads files** (this decides what the shim has to cover):

- On `file:` (checked with `"file:"===window.location.protocol`) the player loads `header.jsonp`,
  `<UUID>.jsonp` and `<UUID>.pdfp` through a **dynamic `<script>`** with `setAttribute("src", ...)`
  and `document.head.appendChild`. The callbacks are the globals `local_header`, `local_slide` and
  `local_pdf`. A `.pdfp` file holds the PDF as Base64 inside JSONP.
- On `http(s):` it loads `.json` with `XMLHttpRequest` and the PDF through pdf.js (`fetch`).
- CMaps: on `file:` pdf.js calls `get_binary_bcmap(name)`; that function and all CMaps are
  embedded in `main.js` as Base64. The folder `pdfjs/web/cmaps/` is not read on `file:`.
- `GlobalWorkerOptions.workerSrc = "./assets/player/pdfjs/pdf.worker.min.js"`: pdf.js tries a worker
  and otherwise falls back to a "fake worker" loaded through a script tag.
- The player starts on the window `load` event (`window.addEventListener("load", ...)`) and uses WebGL (with a fallback).

**Control:**

- The show controller is a module-local variable (`UC`); there is **no global API**.
- Keys: `document.addEventListener("keydown")`, evaluated through `charCode||keyCode`.
  Digits followed by Enter (13) jump to a slide (`jumpToSlide`), Home/End go to the first/last slide;
  other codes trigger next build, next slide, previous build and previous slide.
  Synthetic events therefore need a `keyCode`.
- On `file:` the player does **not** write a hash into the URL.
- The player dispatches `ShowController:SlideIndexDidChangeEvent` (with `detail.slideIndex`) on `document`.

**Events:** every slide of this export has exactly one event (`effects[0].type = "transition"`).
One event is one click; builds show up as additional events (still to be confirmed with a native deck).

**Notes:** neither `header.json` nor the slide JSON files contain presenter notes. This confirms decision E6.

**Self-playing mode:** `header.showMode`, `autoplayTransitionDelay` (5 s) and `autoplayBuildDelay` (2 s)
are in the header. That gives decision E7 a source for Keynote timings.

**Version:** `header.creator` ("Apple Keynote 15.4") and `majorVersion`/`minorVersion` (1.2) are used for version detection.

## Links, builds and the text for screen readers (2026-11-10)

Measured on an export of the content deck (`fixtures/pptx/src/content.pptx`, opened in Keynote 15.4 and exported as HTML): 5 slides, because the slide hidden in PowerPoint is not exported.

**What Keynote takes over from the PowerPoint file:** links to other slides on text and on a shape, builds by paragraph and by object. A shape that only a trigger shows becomes an ordinary build (Keynote has no triggers).

**Links** are in `events[].hyperlinks` of the slide JSON. Each has a `url`, a `targetRectangle` (x, y, width, height in slide units; the stage of this export is 960 x 540) and, for most targets, `events` with the drawn transition to the target.

| Link in PowerPoint | `url` |
|---|---|
| next, previous, first, last slide | `?slide=next`, `?slide=previous`, `?slide=first`, `?slide=last` |
| last slide viewed | `?action=retreat` |
| end show | `?action=exitpresentation` |
| a slide | `?slideid=<id of the slide in slideList>` |

The link to the hidden slide has a `?slideid=` that is not in `slideList`. A link on text has exactly the rectangle of its entry in `accessibility` (8 of 8); the link on the shape has the rectangle of the shape, and no entry has that rectangle.

**Where the slide is in the window:** the elements `#stageArea`, `#stage` and `#hyperlinkPlane` have the bounding box of the slide. In a window of 1400 x 820 the slide is 960 x 540 at (220, 140), unscaled; in a window of 900 x 700 it is 900 x 506 with `matrix(0.9375, ...)` on `#stage`. A click is mapped to slide units with the bounding box of `#stage`.

**Builds:** a slide with three paragraph builds has four events (`buildIn` three times, then `transition`), a slide with two object builds has three. An event is one click, and the event of the transition is the last step.

**The text for screen readers is not the visible state.** `events[].accessibility` lists texts with a rectangle each:

- On the slide with object builds, all six texts are in every event, also the two objects that appear on the first and second click.
- On the slide with paragraph builds, the paragraphs are added event by event, but the third event lists none of the list's paragraphs, although three are visible then.
- The last event lists everything on both slides.

What is visible is in the layer tree of each event: every texture has `initialState.hidden`, and there the built objects and paragraphs change from hidden to shown in the right event. deckfold does not read steps from it: assigning a text to a layer would go through rectangles and would be ambiguous where objects overlap. The blocks of a Keynote deck have no step.

**What the list holds:** the name of a picture ("Gradient"), not its description; the footer text; for the slide number field one entry with the object replacement character (U+FFFC) as its text; no text of the table.

**Opening the file from a script:** three attempts opened two documents. The first `open` through AppleScript did not return within two minutes. Keynote showed a dialog that the file could not be imported, and while it was open the next `open` did not return either. After the dialog was closed, two documents named "content" appeared (from the waiting requests). The file was in a git-ignored folder of the project. Why one attempt failed is not known. A dialog in Keynote blocks Apple events to it, as in PowerPoint.

## Consequences for the implementation

- Keynote detection: `index.html` + `assets/header.json(p)` + `assets/player/main.js` (not `kpf.json`).
- The whole `assets/` folder is embedded except `Acknowledgements.pdf`.
  The audience window runs on `file:` and uses JSONP/PDFP. The previews in the presenter view are
  `srcdoc` iframes (see S2); there the protocol is `about:`, and the player loads `.json`, `.pdf` and the
  CMaps through XHR/fetch. Without these files the previews would not run (about 3.5 MB more for the test deck).
- The shim covers dynamic `<script src>` (JSONP), XHR, `fetch`, `Worker`, image and media `src`, `<style>`
  elements inserted at runtime (webpack style-loader for the player images), and replays a missed `load`.
- In the preview role, `new URL(path, "about:srcdoc")` is redirected to the base URL of the output file,
  because pdf.js resolves relative PDF paths against `window.location`.
- The player calls `focus()` itself; in the preview role `focus()` is disabled, otherwise an iframe
  would take the keyboard focus.
- Control through synthetic keys with `keyCode`: forward 39 (build), back 219 (`[`, build),
  jump to a slide with digits followed by Enter. Escape and Q (leave the show) never reach the player.
- Self-playing mode: `showMode` 1 = self-playing, 2 = links only.
- In a window the stage is shown at 720x405 and only scales up in full screen; the original behaves the same.
