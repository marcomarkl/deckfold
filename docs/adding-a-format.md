# Adding a format

This is the order in which the Keynote adapter was built. Most of its problems only showed up in a real export, in the `srcdoc` previews of the presenter view, or in full screen, so check those places early. The Keynote findings are in `docs/spikes/S1-keynote.md`.

1. **Get a real export.** Create one with the source application and put it into the local corpus under `testdata/<format>/`. Never commit it; `testdata/` is ignored. Descriptions of the format on the web can be out of date (the Keynote export no longer has a `kpf.json`).

2. **Find out how the player loads files.** Search the player code for `location.protocol`, `XMLHttpRequest`, `fetch(`, `createElement("script")` and `Worker(`. Players often load differently on `file:` than on `http(s):` or `about:`. The audience window runs on `file:`, the presenter previews run in `srcdoc` iframes with the protocol `about:`, so both paths must work. Every file the player loads through a computed path has to come out of `Collect` so the shim can map it.

3. **Find out how the player starts and is controlled.** Note the start event (`load`, `DOMContentLoaded` or a `readyState` check), where it listens for keys (`window` or `document`) and whether it reads `key` or `keyCode`, which custom events it dispatches, and whether it calls `focus()` itself. A player that leaves the show on Escape, or that pulls the focus into a preview, needs handling in the runtime.

4. **Write the Go adapter** in `internal/adapter/<format>/` with `Name`, `Detect`, `Collect`, `Meta` and `SkipStaticRef`, and add it to `internal/detect.Choose`. `Meta` returns slides, steps per slide, auto steps or delays, notes and thumbnails.

5. **Write the JS adapter** in `runtime/src/20-model.js` with `ready`, `forward`, `back`, `jump` and `mediaRoot`. Drive the player only through synthetic keys or its public navigation (for example `#n`), and keep the state in the runtime. Run `go generate ./runtime` afterwards.

6. **Test the places where Keynote broke:**
   - solo audience view on `file:`
   - presenter view with both `srcdoc` previews rendering and following the state
   - Escape in full screen (the key must never reach the player)
   - no network requests
   - pixel comparison with the original export

   `tests/e2e/setup.mjs` (corpus discovery), `tests/e2e/helpers.mjs` (`need`) and `tests/e2e/fidelity.spec.mjs` only know `claude` and `keynote` so far; extend them for the new format. Add a golden test in `cmd/deckfold/main_test.go` that checks the player code is embedded byte-identical.

7. **Write down what you found** in `docs/spikes/S<n>-<format>.md`, and add new decisions to `docs/decisions.md`.
