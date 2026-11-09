# deckfold

deckfold turns a presentation into a single HTML file that works without a folder and without a network connection. It reads PowerPoint files directly, and the HTML exports of Keynote and claude.ai. Fonts, images, audio and video are stored inside the file. For HTML exports, transitions and animations come unchanged from the player of the source; PowerPoint files are rendered by deckfold and played by its own player. On top of that, deckfold adds its own controls: full screen, a presenter view, auto mode, a free choice of screens, and control by keyboard or presentation clicker.

deckfold is a command-line tool and only converts. You present in a browser. There is one binary per platform; it needs no installation and no admin rights.

## Supported sources

| Source | How |
|---|---|
| PowerPoint | pass the `.pptx` file (also `.ppsx` and `.pptm`) to deckfold. Older `.ppt` files and password-protected files have to be saved as `.pptx` without a password first |
| Google Slides | File > Download > Microsoft PowerPoint (.pptx), then pass the file to deckfold. YouTube or Drive videos embedded in Slides do not survive the PPTX export; insert them as video files first |
| Keynote | File > Export To > HTML, then pass the exported folder to deckfold |
| claude.ai | download the slide deck as HTML and pass the file to deckfold |

deckfold does not recognize other HTML files and stops with exit code 2. With `--generic` the file is embedded and cleaned up anyway; it then gets full screen, screen choice and keyboard control, but no presenter view, no overview and no auto mode.

Keynote does not write presenter notes into its HTML export, so Keynote decks have no notes panel. PowerPoint, Google Slides and claude.ai decks bring their notes along.

### PowerPoint files

deckfold reads the slides, layouts, masters and themes of the file and draws every slide as SVG: shapes and their geometry, fills, outlines and arrowheads, shadows, glow, inner shadows, soft edges, reflections and blur, pictures (also EMF and WMF), tables, charts, SmartArt graphics, ink and text. It lays out the text itself with the metrics of the fonts, including justified text; right-to-left text and complex scripts (Arabic, Hebrew, Indic, Thai) are measured with HarfBuzz and shaped by the browser. Fonts come from the presentation (also when PowerPoint stored them compressed), from the fonts installed on the computer (including the fonts that ship with Microsoft Office) or, when neither has them, from the fonts bundled with deckfold (Carlito for Calibri, Liberation for Arial, Times New Roman and Courier New). The report lists where each font came from. Only subsets of the fonts that may be embedded are stored in the file.

Charts are drawn from the data stored with them: column, bar, line, area, pie, doughnut, scatter, bubble, radar, stock, pie of pie, bar of pie and surface charts. 3D shapes (bevels, extrusions, cameras) and 3D charts are drawn with WebGL (three.js, embedded only in decks that need it); without WebGL, and in the previews of the presenter view, an SVG approximation takes their place.

Animations and transitions are played by deckfold's player: entrance, exit and emphasis effects, motion paths, builds by paragraph, word and letter, effects that start with or after the previous one, and triggers (a click on a shape plays its effects; a click on a video or an audio icon plays or pauses it). Animated GIFs start again when their slide is shown. Morph moves, scales and turns the objects it pairs. The transitions of PowerPoint 2010 and later are drawn too: the 3D ones (cube, flip, gallery, doors and others) with CSS, the ones that break the slide into pieces (glitter, honeycomb, vortex, shred and others) with WebGL, and as a fade where WebGL is missing. Slides with automatic timings advance by themselves in auto mode. Hidden slides are left out.

What deckfold does not show as PowerPoint does:

- Charts with an automatic layout can differ from PowerPoint by a few pixels in the position of the plot area, and the 3D charts are an approximation.
- 3D shapes and the tile transitions are close to PowerPoint, not identical; Morph does not change the outline of shapes.
- Animations started by a bookmark in a video or audio clip are not played.
- Videos in WMV, AVI or ASF show only their still image, because browsers cannot play them. Media files larger than 1 GB are refused; compress them in PowerPoint first.

## Usage

```bash
deckfold MyTalk
```

```bash
deckfold "My deck.html" -o talk.html
```

```bash
deckfold Quarterly.pptx
```

Without `-o` the result is `<name>.standalone.html` next to the input. deckfold never overwrites its input, and it overwrites an existing output only with `--force`.

| Option | Meaning |
|---|---|
| `-o file.html` | output file |
| `--force` | overwrite an existing output |
| `--config file.json` | defaults from a configuration file (example: `deckfold --print-config`) |
| `--presenter-elements current,audience,next,notes,time,overview` | visible parts of the presenter view |
| `--notes-font-size 22` | notes font size in px |
| `--target-minutes 30` | target duration of the talk (0 = none) |
| `--auto` | start in auto mode |
| `--auto-interval 8` | seconds per step when the source has no timings |
| `--loop` | loop in auto mode |
| `--warn-size 250MB` | warning threshold for the output size |
| `--lenient` | write the file even if assets cannot be resolved (with a warning) |
| `--generic` | embed an unknown format anyway |
| `--allow-outside folder` | allow files outside the input folder (repeatable); for PowerPoint files, the folder of linked pictures and videos |
| `--proxy url` | proxy for downloads |
| `--refresh file.html` | update only the controls of an existing deckfold file |

Defaults are applied in this order: command line, then configuration file, then built-in defaults. Changes made later under "Settings" in the presentation are stored by the browser per deck and take precedence over the defaults.

Exit codes: 0 success, 1 conversion failed (for example unresolved assets), 2 format not recognized, 3 usage or configuration error.

### What happens during conversion

deckfold reads the export, downloads remote resources (for example Google Fonts) and embeds everything in the file. It removes duplicate includes and duplicate font faces, and it fixes paths that only fail because of URL encoding, Unicode normalization or upper and lower case; every fix shows up as a warning in the report. The CSS and JavaScript of the source stay unchanged. If an asset cannot be resolved, deckfold stops and lists every place that references it.

Files outside the input folder are not embedded, not even through symlinks or absolute paths. When downloading, deckfold refuses addresses in the local or private network. The report lists every host it contacted.

Live content such as embedded web pages stays live. Without a network connection the still image of the source or a placeholder appears; the report names the slides concerned.

Large media files are embedded unchanged. Chrome opens a 300 MB file in a little over a second; Safari needs about ten seconds for the same file.

## Presenting

Open the file in a browser. The window it opens in is the presentation. It shows a start screen first: the title, a large preview of the chosen slide as it looks after all its steps, a strip of all slides (a click or the arrows beside it choose the slide to start from), the settings, and the "Start" button next to them:

- **Presenter view:** opens a second window with previews of the current and the next slide (each as it looks after all its steps), notes, a clock, the elapsed and the remaining time and a strip of all slides. Below the current slide, the audience pane shows what the audience sees whenever that is not the whole slide: a step before the last one, a black or white screen, the end slide. Drag the window to the screen where you want to see it.
- **Full screen** for the presentation, or a window.
- **Auto mode** with its interval and whether it loops.

"Start" or Enter begins the presentation in this window; it starts only there, because full screen needs a click in the window that goes full screen. The arrow keys choose the start slide, also from the presenter view, so a clicker works before the start. Below the settings, the start screen lists every key as it is set.

In the presentation, a click on the slide moves on like the keys for next; links, videos and PowerPoint triggers keep their own click. The mouse wheel and swipes on a trackpad or touch screen do nothing, so that a gesture cannot move the slides by accident. Should the player of the source still end up on another slide, deckfold brings it back to its own slide within about a second.

deckfold does not place windows itself and does not check where they are. Put each window where it belongs, for example the presentation on the projector and the presenter view on the laptop; full screen can always be chosen. The presenter view goes into full screen with F or its "Full screen" button, which then reads "Window" and takes it back.

The presenter view has a status bar at the bottom: hints on the left (for example that the presentation window cannot be reached), and on the right the switches for its panes (current slide, audience, next slide, notes, time, slide strip). Above the status bar runs the strip of all slides: during the show a click on a slide jumps there, and its arrows only scroll the strip, so that a click never moves the audience on by accident; before the start they choose the start slide. A− and A+ change the size of the notes, the target duration sits next to the time (minutes:seconds; "20" becomes 20:00, "1:75" becomes 02:15), and the panes are resized by dragging the dividers between them. In a narrow window the panes stand one below the other and scroll; the bar at the top stays one line and scrolls sideways. The settings are stored in the browser for each presentation. While steps run by themselves (claude.ai decks), the audience pane is empty if the run ends with the whole slide shown: deckfold knows the state the run ends in, not each step on the way.

Closing the presentation window closes the presenter view too; reloading it does not.

One step after the last slide shows a black end slide, which stays until the presentation is ended: Escape (in either window) or "End show" returns to the start screen with the last slide chosen. Reloading the window keeps the slide chosen or shown last. The presenter view stays open. If the browser cannot switch to full screen, the presentation shows "Click for full screen".

| Key | Action |
|---|---|
| → ↓ Page Down, Space | next |
| ← ↑ Page Up, Backspace | back |
| Home / End | first / last slide |
| digits, then Enter | go to slide n |
| Enter | start (start screen); nothing during the presentation |
| click on the slide | next |
| F, F5 | full screen |
| B or . | black screen |
| W | white screen |
| A | auto mode on/off |
| T | reset the timer |
| Esc | end the presentation |

In Chrome and Edge, one Escape ends the presentation in full screen too. Safari and Firefox leave full screen on the first Escape; a second one ends the presentation.

Common presentation clickers send these keys. You can change the bindings under `keys` in the configuration file.

Auto mode uses timings from the source (automatic timings in PowerPoint, Keynote in self-playing mode, automatic steps in claude.ai decks) and the configured interval otherwise. On slides with a playing video or audio clip it waits until the clip ends. The timer runs in the presentation window, so it keeps going when the presenter view is covered or closed. The presentation window keeps the screen awake and hides the mouse cursor after two seconds.

## Download and first start

Binaries for macOS, Windows and Linux (x64 and Arm) are on the [releases page](https://github.com/marcomarkl/deckfold/releases), with checksums in `SHA256SUMS`. The current version is a pre-release; its release notes list what has been tested so far.

The binaries are not signed, so the operating systems ask before the first start:

- macOS: System Settings > Privacy & Security > "Open Anyway", or in a terminal `xattr -d com.apple.quarantine deckfold`.
- Windows: in the SmartScreen dialog choose "More info", then "Run anyway". You can also drop a folder onto `deckfold.exe`; the window then waits for Enter at the end.
- Linux: `chmod +x deckfold`.

Downloads honour `HTTPS_PROXY` and `NO_PROXY` or the `--proxy` option. deckfold does not use the system proxy settings of Windows and macOS (PAC files). Certificates come from the system certificate store, including company certificates.

## Development

You need Go (tested with 1.27) and, for the browser tests, Node.js.

```bash
go generate ./runtime
```

```bash
go test ./...
```

```bash
cd tests && npm install && npx playwright install chromium webkit && npx playwright test
```

```bash
scripts/release.sh 0.1.0
```

The browser part lives in `runtime/src/` (controls) and `runtime/pptx-player/` (the player of PowerPoint decks) and is bundled into `runtime/dist/` with `go generate ./runtime`; a test checks that the checked-in bundle is up to date. The format adapters are in `internal/adapter/`, the shared conversion is in `internal/embed/`, the PowerPoint renderer is in `internal/pptx/`. The design decisions are listed in `docs/decisions.md`, and background notes on the formats and browsers are in `docs/spikes/`.

The PowerPoint tests use public feature decks in `fixtures/pptx/`, written by `go run ./scripts/pptxgen` and saved again by PowerPoint; each slide is compared with an image exported by PowerPoint. `fixtures/pptx/README.md` explains how the reference images are made.

The golden tests and most browser tests need sample decks in `testdata/`, which is not part of the repository: put claude.ai decks in `testdata/claude/*.html` and Keynote exports in `testdata/keynote/<name>/`. Without them those tests are skipped.

## License

MIT, see [LICENSE](LICENSE).
