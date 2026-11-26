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

Keynote does not write presenter notes into its HTML export, so Keynote decks have no notes panel. PowerPoint, Google Slides and claude.ai decks bring their notes along. The slide counter that the claude.ai player shows after each change is hidden; the presenter view marks the slide in its strip of slides and shows the steps as dots.

### PowerPoint files

deckfold reads the slides, layouts, masters and themes of the file and draws every slide as SVG: shapes and their geometry, fills, outlines and arrowheads, shadows, glow, inner shadows, soft edges, reflections and blur, pictures (also EMF and WMF), tables, charts, SmartArt graphics, ink and text. It lays out the text itself with the metrics of the fonts, including justified text; right-to-left text and complex scripts (Arabic, Hebrew, Indic, Thai) are measured with HarfBuzz and shaped by the browser. Fonts come from the presentation (also when PowerPoint stored them compressed), from the fonts installed on the computer (including the fonts that ship with Microsoft Office) or, when neither has them, from the fonts bundled with deckfold (Carlito for Calibri, Liberation for Arial, Times New Roman and Courier New). The report lists where each font came from. Only subsets of the fonts that may be embedded are stored in the file.

Charts are drawn from the data stored with them: column, bar, line, area, pie, doughnut, scatter, bubble, radar, stock, pie of pie, bar of pie and surface charts. 3D shapes (bevels, extrusions, cameras) and 3D charts are drawn with WebGL (three.js, embedded only in decks that need it); without WebGL, and in the previews of the presenter view, an SVG approximation takes their place.

Animations and transitions are played by deckfold's player: entrance, exit and emphasis effects, motion paths, builds by paragraph, word and letter, effects that start with or after the previous one, and triggers (a click on a shape plays its effects; a click on a video or an audio icon plays or pauses it). Animated GIFs start again when their slide is shown. Morph moves, scales and turns the objects it pairs. The transitions of PowerPoint 2010 and later are drawn too: the 3D ones (cube, flip, gallery, doors and others) with CSS, the ones that break the slide into pieces (glitter, honeycomb, vortex, shred and others) with WebGL, and as a fade where WebGL is missing. Slides with automatic timings advance by themselves in auto mode. A slide hidden in PowerPoint is in the file and hidden from the show; its eye on the start screen shows it again (see Presenting). Its text, pictures and notes are then in the file too, so to pass on a file without such slides, convert it with `--drop-hidden` or `"dropHidden": true` in the configuration file, which leaves them out as deckfold did up to v0.11.0. A PowerPoint file converted before v0.12.0 gets these slides only from a new conversion, not from `--refresh`. A link to another slide works on text and on a shape: to a slide, to the next, previous, first or last slide, to the slide viewed before, or to the end of the show.

What deckfold does not show as PowerPoint does:

- Charts with an automatic layout can differ from PowerPoint by a few pixels in the position of the plot area, and the 3D charts are an approximation.
- 3D shapes and the tile transitions are close to PowerPoint, not identical; Morph does not change the outline of shapes.
- Animations started by a bookmark in a video or audio clip are not played.
- TIFF pictures in CMYK or with 16 bits per channel, and a few TIFF variants deckfold cannot read, stay TIFF, which only Safari shows. The report names them. Other TIFF pictures are converted to WebP.
- Videos in WMV, AVI or ASF are converted to MP4 when ffmpeg is installed (see below). Without ffmpeg they show only their still image, because browsers cannot play them. Media files larger than 1 GB are refused; compress them in PowerPoint first.

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
| `--presenter-elements audience,next,notes` | panes of the presenter view that are shown; the names of earlier versions are still read |
| `--notes-font-size 22` | notes font size in px |
| `--target-minutes 30` | target duration of the talk (0 = none) |
| `--auto` | start in auto mode |
| `--auto-interval 8` | seconds per step when the source has no timings |
| `--loop` | loop in auto mode |
| `--warn-size 250MB` | warning threshold for the output size |
| `--keep-assets` | embed pictures, fonts, video and sound unchanged instead of converting them |
| `--drop-hidden` | leave out the slides hidden in a PowerPoint file (by default they are included and skipped in the show) |
| `--no-cache` | do not keep converted pictures in the folder `.deckfold-cache` beside the output |
| `--lenient` | write the file even if assets cannot be resolved (with a warning) |
| `--generic` | embed an unknown format anyway |
| `--allow-outside folder` | allow files outside the input folder (repeatable); for PowerPoint files, the folder of linked pictures and videos |
| `--proxy url` | proxy for downloads |
| `--refresh file.html` | update the controls of an existing deckfold file; the slides stay unchanged |
| `--content file.json` | write the slide text of an existing deckfold file as JSON, or as Markdown with `file.md` |

Defaults are applied in this order: command line, then configuration file, then built-in defaults. Changes made later under "Settings" in the presentation are stored by the browser per deck and take precedence over the defaults.

Exit codes: 0 success, 1 conversion failed (for example unresolved assets), 2 format not recognized, 3 usage or configuration error.

### The slide text

deckfold reads what is written on the slides and stores it in the file: the text of every slide in the order of the source, the title where the source marks one, list entries with their level, the description of pictures, and the step in which each text shows (not for Keynote exports, which do not say). The search and the outline of the presentation use it. For other programs, `deckfold deck.html --content deck.json` writes it with the titles and notes of the slides, and `--content deck.md` writes Markdown. `docs/content-model.md` describes the fields.

A file from before can get the slide text with `--refresh` when the deck came from claude.ai or Keynote. A PowerPoint presentation has to be converted again.

### What happens during conversion

deckfold reads the export, downloads remote resources (for example Google Fonts) and embeds everything in the file. It removes duplicate includes and duplicate font faces, and it fixes paths that only fail because of URL encoding, Unicode normalization or upper and lower case; every fix shows up as a warning in the report. The CSS and JavaScript of the source stay unchanged. If an asset cannot be resolved, deckfold stops and lists every place that references it.

Files outside the input folder are not embedded, not even through symlinks or absolute paths. When downloading, deckfold refuses addresses in the local or private network. The report lists every host it contacted.

Live content such as embedded web pages stays live. Without a network connection the still image of the source or a placeholder appears; the report names the slides concerned.

Pictures and fonts are converted to smaller formats: PNG, GIF, BMP and TIFF pictures become lossless WebP, and TrueType, OpenType and WOFF fonts become WOFF2. A converted file replaces the original only when it is smaller and reads back as the same picture, sample by sample, or as the same font tables. Colour profiles are kept. JPEG pictures, SVG, video and sound are embedded unchanged, and so is every file that a script of the source loads itself. The report says how many files were converted and what that saved. `--keep-assets`, or `"keepAssets": true` in the configuration file, embeds everything as it is. Converting the pictures of a large deck takes minutes, because deckfold uses the slowest and best setting of the encoder.

So that this time is spent once, deckfold keeps what the encoder wrote in a folder named `.deckfold-cache` beside the output file; without `-o` that is beside the presentation. The next conversion of the same pictures takes them from there and says so in its report. An entry is used only after deckfold has compared it with the picture of the presentation, sample by sample, so a folder that somebody else filled cannot change what the file shows. The folder holds pictures of your slides: leave it out when you pass the output on. You can delete it at any time, and `--no-cache`, or `"cache": false` in the configuration file, switches it off. Fonts are not kept, they convert fast.

Video and sound that not every browser plays are converted when `ffmpeg` and `ffprobe` are installed and on the PATH. deckfold brings no encoder for them. HEVC, AV1, WMV, MPEG-2 and H.264 with 10 bits become H.264 in an MP4 file, at a quality close to the source (`-crf 18`) and with the slowest preset. H.264 in a MOV, MKV or AVI file is packed into an MP4 file without touching the pictures. PCM sound in a WAV file becomes FLAC, which holds the same samples; WMA becomes MP3. What every browser plays already stays as it is: H.264 with AAC in MP4, MP3, AAC, FLAC. Video with HDR, with transparency or interlaced is not converted, and the report names it. Without ffmpeg, video and sound are embedded as they are, and the report says when a file would have been converted. The result for these files depends on the ffmpeg of your machine, so two machines can write different bytes; set `DECKFOLD_NO_FFMPEG=1` to ignore an installed ffmpeg. Converted video and sound are not kept in `.deckfold-cache`.

Large media files that need no conversion are embedded unchanged. Chrome opens a 300 MB file in a little over a second; Safari needs about ten seconds for the same file.

## Presenting

Open the file in a browser. The window it opens in is the presentation. It shows a start screen first: the title, a large preview of the chosen slide as it looks after all its steps, a strip of all slides (a click or the arrows beside it choose the slide to start from; the eye at the right of a slide's number hides it from the show or shows it again), the settings with the "Start" button next to them, and a status line with an info button, a search button (a magnifier) and the number of slides. The start screen fills the window, and the preview and the slides of the strip grow and shrink with it. The settings are:

- **Presenter view:** opens a second window. On the left, large, "Live" shows what the audience sees: the slide at its current step, a black or white screen, the end slide; before the start, the slide the show starts on. On the right, "Preview" shows what the next click brings, the next step of the slide or the next slide, and below it the notes. The window also has a clock, the elapsed and the remaining time, a strip of all slides and the number of slides. The elapsed time runs from "Start", stands still when the show ends and runs on from there when the show starts again; T or the reset button sets it back to zero. Drag the window to the screen where you want to see it. When the show ends, the switch stays on as long as the presenter view is open.
- **Full screen** for the presentation, or a window.
- **Auto mode** with its interval and whether it loops.

A slide hidden with its eye stays in the strip, greyed out, with a crossed eye. The show skips it: next, back, Home, End, the links to the next and previous slide and auto mode go past it. Its number with Enter, a hit of the search or a link to it still shows it, and the show goes on with the next slide that is not hidden. The show always begins on a slide that is not hidden; when the chosen slide is hidden, the status line shows a play sign with the number of the slide it begins on. The last slide that is not hidden cannot be hidden. The number of slides then reads, for example, "7 of 9 slides". The choice is stored in the browser for this presentation; until something is chosen, the slides hidden in PowerPoint are hidden. The presenter view's strip of slides leaves hidden slides out and follows each change at once. The eye is there for PowerPoint, claude.ai and Keynote decks, not for a self-playing Keynote export or an unknown format, where deckfold does not move the player itself.

The magnifier (or `/`, Ctrl+F, Cmd+F) opens the outline: every slide with its title, or with its number and the start of its text when the source marks no title. Typing searches the slide text and shows the slides that hold it; Enter or a click chooses the slide to start from. The presenter view has the same search. There it searches the notes too, and choosing a slide goes there at once, like a click in the strip of slides. Escape, the × beside the text field or a click beside the list closes the search. The presentation window itself has no search, because the audience would see it.

"Start" or Enter begins the presentation in this window; it starts only there, because full screen needs a click in the window that goes full screen. The arrow keys choose the start slide, also from the presenter view, so a clicker works before the start. The info button, at the start of the status line, opens a list of every key as it is set, with the version of deckfold that wrote the file below it. Escape, the × or a click beside the list closes it. The list opens with the mouse only: it has no key of its own.

In the presentation, a click on the slide moves on like the keys for next; links, videos and PowerPoint triggers keep their own click. A link to another slide in a PowerPoint deck or a Keynote export goes there in both windows. The mouse wheel and a swipe on a trackpad do nothing, so that a gesture cannot move the slides by accident. On a touch screen, a tap moves on like a click, a swipe to the left moves on, and a swipe to the right goes back; a swipe counts when it begins on the slide. A swipe up, or the key G, shows a strip of all slides at the bottom of the presentation, with a button that ends the show at its right. A tap or click on a slide in the strip goes to the start of that slide. A slide hidden from the show stands there greyed out and can be chosen too; the show then goes on with the next slide that is not hidden. A swipe down, a tap or click beside the strip, Escape or G closes it. While it is open, the keys for next and back move its mark, and Enter goes to the marked slide. The audience sees the strip, since it is part of the presentation window. Touch is tested with touches made by a script in Chrome. On an iPad with Safari, served from a Mac over the local network, full screen worked and hid the address bar, and the deck ran in portrait and in landscape; every swipe down leaves full screen there, which is a gesture of the system, so the strip is closed with a tap beside it. Swipes and the strip are not confirmed on an iPad yet, and a touch display is not tried. An iPad opens an HTML file from the Files app only as a preview, where the deck does not run; it has to come from a web server.

On a phone or tablet (iOS, iPadOS, Android) the presentation always runs in full screen and has no presenter view, so the start screen shows neither setting there. When full screen is lost on such a device, the next tap or swipe on the slide asks for it again and moves on as usual. On a computer, full screen that is left during the show stays off, the presentation goes on in the window, and F brings it back; "Click for full screen" appears only when full screen was asked for and did not come. Should the player of the source still end up on another slide, deckfold brings it back to its own slide within about a second.

deckfold does not place windows itself and does not check where they are. Put each window where it belongs, for example the presentation on the projector and the presenter view on the laptop; full screen can always be chosen. The presenter view has no full screen of its own; F does nothing there. A window can still be made to fill its screen with the means of the browser or the system. Its buttons for the search and the end of the show show an icon each, with the name as a tooltip.

The heading of "Live" has a dot for each step of the slide, filled up to the one the audience sees, and an arrow for the change to the next slide: every mark after the last filled dot is one click. When the next click changes the slide, the arrow is marked. The top bar has no slide counter. The presenter view has a status bar at the bottom: the number of slides on the left, then hints (for example that the presentation window cannot be reached), and on the right the switches for its three panes (Live, Preview, notes); the time and the strip of slides are always shown. Above the status bar runs the strip of all slides: during the show a click on a slide jumps there, and its arrows only scroll the strip, so that a click never moves the audience on by accident; before the start they choose the start slide. A− and A+ change the size of the notes and are off while the notes are hidden, the target duration sits next to the time (minutes:seconds; "20" becomes 20:00, "1:75" becomes 02:15), and the panes are resized by dragging the dividers between them; the divider above the strip of slides makes the strip and its tiles larger, up to almost half the height of the window. In a narrow window the panes stand one below the other and scroll; the bar at the top stays one line and scrolls sideways. The settings are stored in the browser for each presentation. While steps run by themselves (claude.ai decks), "Live" shows the state the run ends in: deckfold knows that state, not each step on the way, and such a run takes no dot.

Closing the presentation window closes the presenter view too; reloading it does not.

One step after the last slide shows a black end slide, which stays until the presentation is ended: Escape (in either window) or "End show" returns to the start screen with the last slide chosen. Reloading the window keeps the slide chosen or shown last. The presenter view stays open. If the browser cannot switch to full screen, the presentation shows "Click for full screen".

| Key | Action |
|---|---|
| → ↓ Page Down, Space | next |
| ← ↑ Page Up, Backspace | back |
| Home / End | first / last slide that is not hidden |
| digits, then Enter | go to slide n |
| Enter | start (start screen); nothing during the presentation |
| click on the slide | next |
| F, F5 | full screen |
| B or . | black screen |
| W | white screen |
| A | auto mode on/off |
| T | reset the timer |
| / or Ctrl+F, Cmd+F | search and outline (start screen, presenter view) |
| G | slide strip in the presentation: choose a slide, end the show |
| swipe left / right (touch screen) | next / back |
| swipe up / down (touch screen) | show / close the slide strip |
| Esc | end the presentation |

One Escape ends the presentation in full screen too. In Chrome and Edge the key reaches deckfold. Safari takes the key and leaves full screen, and deckfold ends the presentation when full screen is gone. There every other way out of full screen ends it as well, except the key F, which leaves full screen and lets the presentation go on in the window, and except the full screen of a video or an embed, which is not the one of the show; Escape while the slide strip is open ends the presentation in Safari instead of closing the strip. Firefox follows the same rule and is not measured with a real key.

Common presentation clickers send these keys. You can change the bindings under `keys` in the configuration file.

Auto mode uses timings from the source (automatic timings in PowerPoint, Keynote in self-playing mode, automatic steps in claude.ai decks) and the configured interval otherwise. On slides with a playing video or audio clip it waits until the clip ends. The timer runs in the presentation window, so it keeps going when the presenter view is covered or closed. The presentation window hides the mouse cursor after two seconds and asks the browser for a screen wake lock. On a Mac, Chrome and Safari grant it, and in Chrome it kept the screen saver from starting. When the browser does not grant it, for example while the presentation window is minimized, the presenter view says "Screen saver not blocked". No web page can stop a lock that a company policy enforces, and Windows, Linux and Firefox are not measured yet.

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
go test -tags nodynamic ./...
```

The tag `nodynamic` belongs to every build and test. Without it the WebP package loads a libwebp from the system when it finds one, and the output would depend on the machine. A binary built without the tag notices that and leaves pictures unchanged, with a warning. `scripts/release.sh` and the CI set the tag themselves.

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
