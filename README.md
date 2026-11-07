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

deckfold reads the slides, layouts, masters and themes of the file and draws every slide as SVG: shapes and their geometry, fills, outlines and arrowheads, shadows and glow, pictures (also EMF), tables, charts (column, bar, line, area, pie, doughnut), SmartArt graphics and text. It lays out the text itself with the metrics of the fonts. Fonts come from the presentation, from the fonts installed on the computer (including the fonts that ship with Microsoft Office) or, when neither has them, from the fonts bundled with deckfold (Carlito for Calibri, Liberation for Arial, Times New Roman and Courier New). The report lists where each font came from. Only subsets of the fonts that may be embedded are stored in the file.

Animations and transitions are played by deckfold's player: entrance, exit and emphasis effects, motion paths, builds by paragraph, effects that start with or after the previous one, and the transitions fade, push, wipe, cover, reveal and cut. Slides with automatic timings advance by themselves in auto mode. Hidden slides are left out.

What deckfold does not show as PowerPoint does:

- Morph, 3D and the other dynamic transitions are played as a fade.
- Animations by word or by letter are played by paragraph; animations started by clicking a shape (triggers) are not played.
- Fonts embedded in the presentation are stored compressed (MicroType Express), which deckfold cannot read yet; it uses the installed fonts or a bundled substitute instead and reports it.
- WMF pictures, scatter, radar, bubble and stock charts and 3D effects are left out or shown as a placeholder, with a warning.
- Right-to-left text and complex scripts are laid out without shaping, picture bullets are not shown.
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
| `--presenter-elements current,next,notes,time,overview` | visible parts of the presenter view |
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

Open the file in a browser. The start screen offers "Present", "Audience view only" and "Settings".

"Present" opens the audience window. The window you started from becomes the presenter view with the current slide, a preview of the next step, notes, a clock, the elapsed time, an optional remaining time, and a slide overview for jumping.

In Chrome and Edge, deckfold asks for access to your screens the first time. You then assign the audience window and the presenter view to a screen each; the choice is remembered. Safari and Firefox cannot place windows themselves: drag the audience window to the projector and click "Full screen" there. Firefox is not covered by the automated tests yet.

If the browser cannot switch the audience window to full screen without a click, the window shows "Click for full screen". If the window is blocked or closed, or if the presenter view is reloaded, the presenter view offers to reopen it at the same slide.

| Key | Action |
|---|---|
| → ↓ Page Down, Space, Enter | next |
| ← ↑ Page Up, Backspace | back |
| Home / End | first / last slide |
| digits, then Enter | go to slide n |
| F, F5 | full screen |
| B or . | black screen |
| W | white screen |
| A | auto mode on/off |
| T | reset the timer |
| Esc | outside full screen: end the presentation |

Common presentation clickers send these keys. You can change the bindings under `keys` in the configuration file.

Auto mode uses timings from the source (automatic timings in PowerPoint, Keynote in self-playing mode, automatic steps in claude.ai decks) and the configured interval otherwise. On slides with a playing video or audio clip it waits until the clip ends. The timer runs in the audience window, so it keeps going when the presenter view is covered or closed. The audience window keeps the screen awake and hides the mouse cursor after two seconds.

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

The PowerPoint tests use public feature decks in `fixtures/pptx/`, written by `go run ./scripts/pptxgen`; `fixtures/pptx/README.md` explains how the PowerPoint reference images are made.

The golden tests and most browser tests need sample decks in `testdata/`, which is not part of the repository: put claude.ai decks in `testdata/claude/*.html` and Keynote exports in `testdata/keynote/<name>/`. Without them those tests are skipped.

## License

MIT, see [LICENSE](LICENSE).
