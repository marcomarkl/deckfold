# Adding a format

This is the order in which the Keynote adapter was built. Most of its problems only showed up in a real export, in the `srcdoc` previews of the presenter view, or in full screen, so check those places early. The Keynote findings are in `docs/spikes/S1-keynote.md`.

1. **Get a real export.** Create one with the source application and put it into the local corpus under `testdata/<format>/`. Never commit it; `testdata/` is ignored. Descriptions of the format on the web can be out of date (the Keynote export no longer has a `kpf.json`).

2. **Find out how the player loads files.** Search the player code for `location.protocol`, `XMLHttpRequest`, `fetch(`, `createElement("script")` and `Worker(`. Players often load differently on `file:` than on `http(s):` or `about:`. The presentation runs on `file:`, the previews run in `srcdoc` iframes with the protocol `about:`, so both paths must work. Every file the player loads through a computed path has to come out of `Collect` so the shim can map it.

3. **Find out how the player starts and is controlled.** Note the start event (`load`, `DOMContentLoaded` or a `readyState` check), where it listens for keys (`window` or `document`) and whether it reads `key` or `keyCode`, which custom events it dispatches, and whether it calls `focus()` itself. A player that leaves the show on Escape, or that pulls the focus into a preview, needs handling in the runtime.

4. **Write the Go adapter** in `internal/adapter/<format>/` with `Name`, `Detect`, `Collect`, `Meta`, `Content` and `SkipStaticRef`, and add it to `internal/detect.Choose` and `ByName`. `Meta` returns slides, steps per slide, auto steps or delays, notes and thumbnails. `Content` returns the text of every slide for the content model (`docs/content-model.md`): blocks in the order of the source, each with a role the source states and the step in which it shows. It reads through the `ContentSource` it is given and never from the file system itself, because `--refresh` calls it on a finished deckfold file, where the files of the export are embedded assets.

5. **Write the JS adapter** in `runtime/src/20-model.js` with `ready`, `forward`, `back`, `jump`, `mediaRoot` and `shown` (the slide the player shows, or `null` when it cannot tell; the show window brings a player that went elsewhere back). If the player moves by itself on a click anywhere, set `guardClicks`; if it handles clicks on some elements itself (media, triggers), give it `interactive(target)`; if the slides can link to other slides, give it `link(target, event, state)`, which returns the target of the link under a click, so that the show window follows it through its own state (the Keynote adapter finds the link by the place of the click, because the player draws on a canvas). Drive the player only through synthetic keys or its public navigation (for example `#n`), and keep the state in the runtime. Run `go generate ./runtime` afterwards.

6. **Test the places where Keynote broke:**
   - the start screen on `file:` (preview, slide strip) and the presentation after "Start"
   - the presenter view window with both `srcdoc` previews rendering and following the state
   - Escape in full screen (the key must never reach the player)
   - no network requests
   - pixel comparison with the original export

   `tests/e2e/setup.mjs` (corpus discovery), `tests/e2e/helpers.mjs` (`need`) and `tests/e2e/fidelity.spec.mjs` only know `claude` and `keynote` so far; extend them for the new format. Add a golden test in `cmd/deckfold/main_test.go` that checks the player code is embedded byte-identical.

7. **Write down what you found** in `docs/spikes/S<n>-<format>.md`, and add new decisions to `docs/decisions.md`.

## Formats without a player of their own

PowerPoint files have no player, so deckfold renders them itself (`internal/pptx`). The same approach fits any format that is a document rather than an HTML export:

1. Convert the input into an HTML deck in a temporary folder: one element per slide, the deck data as JSON, deckfold's own player, the assets as files. Branch in `cmd/deckfold/main.go` before `detect.Load`, by extension and file signature.
2. Mark the deck (`<meta name="deckfold:source" content="...">`) and write a Go adapter that detects the mark and reads slides, steps, notes and timings from the deck data. Set `Meta.SlideSelector` so that the presenter overview and the asset report find the slides.
3. Give the player a small public API and drive it from the JS adapter. `jump` has to restore any step without animation (previews, back, recovery), only `forward` animates.
4. Treat everything from the source file as untrusted: escape text and attributes, allow only harmless link schemes, never inline SVG from the file, name assets by their content hash.
5. Keep the player between marker comments, so that `--refresh` can replace it (`internal/inject.ReplacePlayer`).
