# Design decisions

Code comments and the spike notes refer to these decisions by their IDs: E1 to E16 for the control layer (E1 to E13 from the first version), P1 to P18 for the PowerPoint converter.

| ID | Decision | Choice | Reason |
|---|---|---|---|
| E1 | Input formats | Keynote HTML export and claude.ai decks; PowerPoint and Google Slides go through PPTX, then Keynote. **Revised by P1:** PowerPoint files are read directly | current PowerPoint versions have no HTML export |
| E2 | Google Slides | PPTX download, opened and exported in Keynote. **Revised by P1:** the PPTX download goes to deckfold directly | Google Slides exports no HTML; no adapter of its own needed |
| E3 | Target browsers (revised by E14) | the same presenting in every browser; Chrome and Edge add Keyboard Lock (one Escape in full screen) | no browser reports reliably which screen a window is on, so deckfold does not check it in any browser (E14) |
| E4 | Where the presenter view is configured | defaults from CLI flags or a JSON file, changeable at runtime and stored per deck in the browser | fixed starting point, adjustable on site |
| E5 | Presenter view elements | current slide and next step, notes, time, slide overview; shown or hidden, notes size, target duration and pane sizes are changed while the view runs | matches Keynote and PowerPoint, plus an overview for jumping |
| E6 | Notes for Keynote decks | none | the Keynote HTML export contains no presenter notes |
| E7 | Auto mode timing | timings from the source first, otherwise a global interval; waits for playing media; optional loop | keeps the intent of the source and never cuts off a video |
| E8 | Media size | embedded unchanged, decoded on demand, warning above a threshold | no quality loss, no dependency on a transcoder |
| E9 | Live web content | stays live; offline it shows the still image of the source or a placeholder; reported during conversion | keeps interactivity and still works offline |
| E10 | Asset cannot be resolved | abort with a list of locations; `--lenient` writes the file anyway | the default output is guaranteed to be self-contained |
| E11 | Extent of the clean-up | safe changes only; CSS and JS content stays unchanged | animations must not break |
| E12 | Code signing | binaries are unsigned; the README describes how to allow them | no cost, can be added later |
| E13 | Unknown format | abort; `--generic` embeds it without presenter view, overview and auto mode | the presenter view never goes missing silently |
| E14 | Presenting (v0.4.0) | the window the file is opened in is the presentation, with a start screen (preview, slide strip, settings); the presenter view is a second window the user switches on and drags to a screen; deckfold places no window and does not check where the windows are, so full screen can always be chosen; the previews show whole slides after all their steps; the source player runs only after "Start", and the end reloads the window back to the start screen; one Escape ends the presentation (Keyboard Lock in Chrome and Edge) | without the window management permission Chrome puts every popup on the opener's screen, Safari and Firefox never place windows. Without the permission no browser reports reliably which screen a window is on (Wayland reports no positions; macOS with separate Spaces shows a dragged window on the screen under the mouse, while browsers report the screen with the larger part of it). A draft of v0.4.0 asked for the permission in Chrome and Edge and blocked full screen while both windows were on the same screen; in the maintainer's test this did not work reliably, so it was removed and placing the windows is left to the user, see `docs/spikes/S2-file-protocol.md` |
| E15 | Input and the player's state (v0.4.0) | a click on the slide is "next" through deckfold; the wheel and swipes are taken before the source player and dropped; after every move and once a second the show window compares the slide the player shows with its own state and jumps back when they differ (at most twice for the same state); a preview reaches each state within one task, so no step in between is painted | the claude.ai player moves by itself on a click, on the wheel and on a swipe, the Keynote player on a swipe; a player that moved past deckfold left the presentation and the presenter view on different slides, and back no longer reached the first slide. Previews pressed the steps one after another, and every step was painted |
| E16 | What is written on the slides (v0.5.0) | every output of a claude.ai, Keynote or PowerPoint deck holds a content model next to the deck metadata: for each slide its text blocks with a role, the step in which each shows where the source tells it (PowerPoint and claude.ai; the Keynote export does not, so its blocks have none), and links to other slides; a title only where the source marks one (the title placeholder of PowerPoint, the label of claude.ai; Keynote marks none), and no role is guessed from size or position. A search and an outline on the start screen and in the presenter view use it, in one overlay; the notes are searched only in the presenter view, and choosing an entry goes to the start of its slide like a slide tile does. A link to another slide goes through deckfold's state. `--content` writes the model as JSON or Markdown; `--refresh` adds it to older claude.ai and Keynote outputs. The source players stay, and the source HTML is not changed for it | until then deckfold knew how a slide runs (steps, timings, notes) and not what is on it. The source players stay because they are what keeps the slides looking like the original. A guessed title would be wrong on some slides without anyone noticing, so a slide without a marked title shows its number and the start of its text. The start screen may already be on the projector, which is why it does not show notes. A link the player followed by itself would leave the presentation and the presenter view on different slides (see E15) |

## PowerPoint converter

| ID | Decision | Choice | Reason |
|---|---|---|---|
| P1 | Scope | PowerPoint files (.pptx, .ppsx, .pptm) are converted directly; deckfold renders the slides and plays them with its own player; the control layer stays the same | a PPTX file has no player of its own, and the Keynote route needs a Mac with Keynote and loses the notes |
| P2 | Approach | native Go: slides as SVG, a timing model for animations; the Keynote route stays possible | the only way that needs no installation, keeps the MIT license and runs on all platforms |
| P3 | Scope of the first version (revised by P9) | static content plus the animation core; 3D transitions and Morph are played as a fade | covers most real decks |
| P4 | Text layout | deckfold lays out text itself with the font metrics | text is the largest risk for fidelity, and the result is the same in every browser |
| P5 | Fonts | cascade: fonts embedded in the presentation, then installed fonts, then the bundled OFL fonts; fsType is honoured; subsets are embedded | line breaks match, font licenses are respected |
| P6 | Charts | own SVG drawing for the common chart types, from the data cache in the file | chart slides stay presentable |
| P7 | EMF and WMF | own converter for the common records | logos and older pictures stay visible |
| P8 | Test decks | generated public feature decks with PowerPoint reference images, compared by SSIM | fidelity can be measured and regressions show up |
| P9 | Scope after the first release | every known gap of the converter is closed, also features no real deck used so far | decks from other sources use them |
| P10 | Order | features by how often they occur in real decks: embedded fonts, GIFs, triggers, 3D, picture bullets, effects; then the rest | the most visible gains first |
| P11 | Morph and 3D transitions | objects are paired by "!!" names, otherwise by kind and text, and interpolated in position, size, rotation, color and opacity; geometric 3D transitions with CSS 3D | visible motion without morphing outlines |
| P12 | 3D shape effects | drawn with WebGL (three.js) from the shape's outline | close to PowerPoint |
| P13 | Without WebGL | an SVG approximation (lighting filter for bevels, offset copy for extrusions) is the base drawing; previews keep it | the overview and machines without WebGL stay usable |
| P14 | Tile transitions | WebGL, with the slides drawn into textures; a fade without WebGL | the same graphics basis as P12 |
| P15 | PowerPoint references | made in one announced run per group of decks | few rounds of access dialogs for the maintainer |
| P16 | Keynote and claude.ai decks for the tests | Keynote exports stay local (they contain Apple's player); a neutral claude.ai deck comes from the maintainer | no third-party player code in the repository |
| P17 | WebGL library | three.js, bundled into a classic script with only the parts the player uses, embedded only in decks that need it | a proven library; decks without 3D stay small |

Defaults that were set without a separate decision:

- For Keynote and claude.ai, the source player stays unchanged and deckfold adds a control layer with one adapter per format, because the animations could not be rebuilt faithfully. PowerPoint files have no source player; there deckfold's own player runs under the same control layer (P1).
- PowerPoint: `.ppt` and password-protected files are refused with a hint (exit code 2); hidden slides are skipped; SmartArt is drawn from the drawing PowerPoint stores with it, otherwise a placeholder; equations, ink, OLE objects and 3D models show the fallback picture stored in the file; shadows and glow become SVG filters, 3D bevels are left out and reported; media stays unchanged (E8), WMV, AVI and ASF show only their still image; animations by word or letter are played by paragraph; triggers (interactive sequences) are not played; `advTm` is the source timing for auto mode (E7).
- deckfold is written in Go: one static binary per platform.
- Assets are embedded as Base64 blocks and turned into blob URLs at runtime; fonts load through rewritten `@font-face` rules, and the start waits for `document.fonts.ready`. One-shot assets (scripts, XHR, fetch, worker code) are released after use; assets referenced by the DOM stay.
- Remote resources (for example Google Fonts) are downloaded during conversion.
- The CSP of the source is extended for `blob:` and live embeds.
- Default key bindings follow common presentation clickers and can be changed in the configuration.
- A start screen with a click, because full screen and window placement need a user gesture.
- Audio plays only in the audience window.
- The notes panel is hidden when a deck has no notes (follows from E6).
- The UI language follows the browser language (German or English).
- The output is `<name>.standalone.html` or the `-o` path; the warning threshold is 250 MB.
| P18 | Text blocks and links to other slides | the renderer collects the text of a slide while it draws it: the paragraphs of the slide's own shapes, table cells and SmartArt shapes, with the role from the placeholder type and the bullet, and the description of pictures; not the text of layout and master, the placeholders for date, footer and slide number, or chart labels. The step of a block comes from the animations: the latest first appearance of its paragraph, its shape and the groups around it. A link to another slide (`ppaction://hlinksldjump`, `ppaction://hlinkshowjump`) is drawn with `data-df-jump` on the text or the shape and followed by the control layer; a link to a hidden slide stays text and is reported | the renderer already knows every paragraph, its placeholder and its bullet, so the content model costs no second pass over the file. Layout and master text would repeat on every slide. Until now such links were drawn as text without an action |
