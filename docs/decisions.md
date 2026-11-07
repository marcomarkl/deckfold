# Design decisions

Code comments and the spike notes refer to these decisions by their IDs (E1 to E13).

| ID | Decision | Choice | Reason |
|---|---|---|---|
| E1 | Input formats | Keynote HTML export and claude.ai decks; PowerPoint and Google Slides go through PPTX, then Keynote | current PowerPoint versions have no HTML export |
| E2 | Google Slides | PPTX download, opened and exported in Keynote | Google Slides exports no HTML; no adapter of its own needed |
| E3 | Target browsers | full support in Chromium (Chrome, Edge); Firefox and Safari without screen placement | only Chromium has the Window Management API |
| E4 | Where the presenter view is configured | defaults from CLI flags or a JSON file, changeable at runtime and stored per deck in the browser | fixed starting point, adjustable on site |
| E5 | Presenter view elements | current slide and next step, notes, time, slide overview | matches Keynote and PowerPoint, plus an overview for jumping |
| E6 | Notes for Keynote decks | none | the Keynote HTML export contains no presenter notes |
| E7 | Auto mode timing | timings from the source first, otherwise a global interval; waits for playing media; optional loop | keeps the intent of the source and never cuts off a video |
| E8 | Media size | embedded unchanged, decoded on demand, warning above a threshold | no quality loss, no dependency on a transcoder |
| E9 | Live web content | stays live; offline it shows the still image of the source or a placeholder; reported during conversion | keeps interactivity and still works offline |
| E10 | Asset cannot be resolved | abort with a list of locations; `--lenient` writes the file anyway | the default output is guaranteed to be self-contained |
| E11 | Extent of the clean-up | safe changes only; CSS and JS content stays unchanged | animations must not break |
| E12 | Code signing | binaries are unsigned; the README describes how to allow them | no cost, can be added later |
| E13 | Unknown format | abort; `--generic` embeds it without presenter view, overview and auto mode | the presenter view never goes missing silently |

Defaults that were set without a separate decision:

- The source player stays unchanged and deckfold adds a control layer with one adapter per format, because the animations could not be rebuilt faithfully.
- deckfold is written in Go: one static binary per platform.
- Assets are embedded as Base64 blocks and turned into blob URLs at runtime; fonts load through rewritten `@font-face` rules, and the start waits for `document.fonts.ready`.
- Remote resources (for example Google Fonts) are downloaded during conversion.
- The CSP of the source is extended for `blob:` and live embeds.
- Default key bindings follow common presentation clickers and can be changed in the configuration.
- A start screen with a click, because full screen and window placement need a user gesture.
- Audio plays only in the audience window.
- The notes panel is hidden when a deck has no notes (follows from E6).
- The UI language follows the browser language (German or English).
- The output is `<name>.standalone.html` or the `-o` path; the warning threshold is 250 MB.
