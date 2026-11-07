# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Rules

- This is a public repository. Everything that can be read publicly is written in English: README, CLAUDE.md, `docs/`, code comments, test names, CLI output and error messages, commit messages, and texts on GitHub. Write these texts following the `anthropic-skills:vermenschlichen` skill (load it first; apply its rules to English). Conversations with the maintainer stay in German. `internal/repocheck` fails on German umlauts, sharp s and the German opening quotation mark (U+201E) in tracked files outside the German UI strings in `runtime/src/00-core.js`; it does not catch German without these characters.
- The browser UI stays bilingual: German and English strings live side by side in the `TEXT` table in `runtime/src/00-core.js`, and the language follows the browser.
- `testdata/` holds private sample decks and is never committed. Do not put file names, slide content or notes from it into code, tests, docs or commits. Tests read expected values (slide counts, notes) from the deck metadata.

## What deckfold is

A Go CLI that turns presentations into one self-contained HTML file that runs offline: PowerPoint files (also Google Slides via its PPTX download) and the HTML exports of Keynote and claude.ai. For HTML exports the source player stays unchanged; PowerPoint files are rendered by deckfold into an HTML deck with its own player. deckfold embeds every asset and adds its own control layer (runtime in JS): start screen, presenter view, auto mode, screen choice, keyboard and clicker control. Design decisions E1 to E13 and P1 to P8 (PowerPoint) are listed in `docs/decisions.md`.

## Commands

```bash
go generate ./runtime          # bundle runtime/src into runtime/dist (after EVERY change in runtime/src or runtime/pptx-player)
go build ./...
go vet ./...
go test ./...                  # includes a test that runtime/dist matches runtime/src
go test -run TestGoldenKeynote -v ./cmd/deckfold
DECKFOLD_OFFLINE=1 go test ./...   # skips the golden test that downloads remote fonts
go run ./cmd/deckfold <deck.html|export-folder|deck.pptx> -o /tmp/x.html --force
go run ./scripts/pptxgen          # rewrite the public PowerPoint feature decks in fixtures/pptx/src
DECKFOLD_BUNDLED_FONTS=1 go run ./cmd/deckfold deck.pptx -o /tmp/x.html --force   # ignore installed fonts
scripts/release.sh 0.1.0       # six binaries (darwin/windows/linux x amd64/arm64, CGO_ENABLED=0) into dist/
```

Browser tests (Playwright, folder `tests/`). `e2e/setup.mjs` builds the binary and writes all test outputs to `tests/.out/` (including a file of about 300 MB for the performance test, created only on the first run) plus `corpus.json` with the corpus entries it found:

```bash
cd tests && npm install && npx playwright install chromium webkit
npx playwright test
npx playwright test e2e/claude.spec.mjs -g "recovery" --project=chrome
```

Projects: `chrome` (channel `chrome`, autoplay without a gesture allowed) and `webkit` (only `webkit.spec.mjs`, the fallback browser). Firefox is not configured because the Playwright Firefox does not start on macOS 27.2. Tests pick buttons by position (`.df-start-buttons button:nth-child(n)`) because the UI language follows the browser. `helpers.mjs` has `need(kind)` to skip without a corpus entry and `expected(page, ops)` to compute expected states with the runtime's own model.

CI (`.github/workflows/ci.yml`) runs on every push to main, every pull request and every `v*` tag: `go vet`, `go test` and the smoke test on Linux, Windows and macOS (x64 and arm64), on Linux also as an unprivileged user. The `fidelity` job runs the PowerPoint browser tests (`tests/e2e/pptx*.spec.mjs`, `perf.spec.mjs`) in Chrome on Linux. Run the smoke test locally with `go run ./scripts/smoke -bin <binary>`. A `v*` tag also builds the six binaries with `scripts/release.sh` and publishes them as a pre-release. To release, add `docs/release-notes/<tag>.md` first (the job stops without it), then push the tag. The two-screen checks that CI cannot do are in `docs/manual-test.md`.

Every output contains the runtime bundle, and the bundle contains marker strings such as `text/df-css`, `text/df-deferred`, `data-df-` and `deckfold:`. When a test asserts on generated HTML, count tags or attributes with `internal/htmlscan` (see `cmd/deckfold/main_test.go`), never substring occurrences in the whole file. This holds for every HTML file that contains the runtime bundle or the PowerPoint player, including the intermediate deck from `internal/pptx/deck`: the player's own comments contain `df-pptx-slide`, and escaped slide text can look like an attribute. Check text only inside the element it belongs to.

Local corpus layout (not in the repository): `testdata/claude/*.html` and `testdata/keynote/<export>/index.html`. The public PowerPoint decks are in `fixtures/pptx/` and are committed.

## Architecture

**Pipeline (Go):** `cmd/deckfold/main.go` (flags, precedence CLI > `--config` > defaults, `--refresh`, exit codes 0/1/2/3) → `internal/detect` (load the input, choose the adapter) → `internal/embed.Convert` (shared conversion) → output.

- **Fidelity:** HTML is never re-serialized. `internal/htmlscan` reads tags with byte offsets; every change is an `internal/patch` patch on the original bytes. Inline scripts and the deck JSON stay byte-identical (the golden tests check this).
- **Deferred activation:** `embed` makes everything inactive that the browser would load or run right away: asset attributes become `data-df-<attr>` with the placeholder `deckfold:<hash>`, stylesheets become `<script type="text/df-css">`, executable scripts get `type="text/df-deferred"` (data scripts such as `application/json` stay). The runtime loader activates them in the original order.
- **Output layout:** the runtime block sits right after `<meta charset>` (which must stay within the first 1024 bytes), between `<!--deckfold:runtime:begin/end-->`; only this block is swapped by `--refresh` (`internal/inject`). Before `</body>`: `#df-meta` (deck metadata, shim table, deckHash) and the asset blocks `<script type="application/octet-stream" id="df-a-<hash>-<n>">` (Base64, at most 6 MB of binary data per block, deduplicated by content hash in `internal/assets.Store`).
- **CSP** (`internal/csp`): extended (`blob:`, embed origins), embedded Google hosts removed. If the source uses `'unsafe-inline'` without hashes or nonces, NO hashes may be added (that would block the source scripts). The loader decodes with `atob`, never with `fetch` or a worker (the claude.ai CSP forbids both).
- **Boundaries:** `assets.Resolver` resolves only inside the input folder (symlinks checked, `--allow-outside` as the exception), in stages: exact, URL decoding, NFC/NFD, case-insensitive (every fix is reported as a warning). `assets.Fetcher` refuses private, loopback and link-local targets.

**PowerPoint** (`internal/pptx`): `main.go` sends `.pptx`/`.ppsx`/`.pptm` (and ZIP or OLE files without an HTML extension) through `pptx.Convert` into a temporary folder before `detect.Load`; from there the regular pipeline runs with the `pptx` adapter.
- `opc` (ZIP with size and path limits, relationships), `xmltree` (element tree by local names; `mc:AlternateContent` is resolved while parsing, only the PowerPoint 2010+ namespaces count as supported), `model` (slides, layouts, masters, themes, placeholder inheritance slide -> layout by idx -> master by type), `geom` (the unchanged Ecma `presetShapeDefinitions.xml` with a formula evaluator), `fonts` (cascade embedded -> installed -> bundled Carlito/Liberation, fsType, subsets renamed because of the Reserved Font Names), `render` (one SVG per slide; every slide shape is `<g data-sid>`, paragraphs are `<g data-p>`; text layout is deckfold's own), `emf`, `anim` (timing -> steps), `deck` (writes `index.html`, `#df-pptx-deck`, the player between `<!--deckfold:pptx-player:begin/end-->`).
- The player (`runtime/pptx-player`, bundled into `runtime/dist/pptx-player.js`) belongs to deckfold, so the JS adapter drives it through `window.__dfPptx` (`goTo`, `next`, `prev`, `showAll`, `state`); this is the exception to the rule below about synthetic keys. `goTo` jumps without animation, only `next` animates; previews never animate or play media. `--refresh` also replaces the player code.
- Text from the presentation is data: everything is escaped, links are limited to http, https and mailto, SVG pictures are referenced through `<image>`, asset names are content hashes. `TestHostileDeck` checks this.
- Tests: `internal/pptx/pptx_test.go` (structure of the feature decks), `tests/e2e/pptx.spec.mjs` (browser; expected animation states come from `fixtures/pptx/src/animations.spec.json`), `tests/e2e/pptx-fidelity.spec.mjs` (SSIM against PowerPoint images in `fixtures/pptx/ref/`, threshold 0.95, exceptions with a reason in the file). After changing a feature deck, run `go run ./scripts/pptxgen` and then `scripts/pptxref/pptxref.sh` with only the names of the changed decks (never without arguments over all decks: PowerPoint rewrites the metadata of every deck it saves, and every deck costs access dialogs); otherwise the hash check in the fidelity test fails. Convert test decks with `DECKFOLD_BUNDLED_FONTS=1` so that the output does not depend on the machine.

**Adapters** (`internal/adapter/{claude,keynote,pptx,generic}`, Go side): `Detect`, `Collect` (files loaded at runtime through computed paths, which end up in the shim table), `Meta` (slides, steps, auto flags, delays, notes, thumbnails). Each format also has a JS adapter in `runtime/src/20-model.js`. To add a format, follow `docs/adding-a-format.md`.
- claude.ai: detected by `meta[name^="hz:"]` + `#deck-motion`; notes come from `data-speaker-notes`; the player advances auto steps itself. Strict structure check; an unknown structure is an error.
- Keynote 15.4 (export format 1.2): detected by `assets/header.json(p)` + `assets/player/main.js`. The whole `assets/` folder is embedded, because the audience window on `file:` uses JSONP/PDFP while the srcdoc previews load `.json`/`.pdf`/CMaps through XHR/fetch. No notes panel (Keynote exports no notes). Details and key codes: `docs/spikes/S1-keynote.md`.

**Runtime** (`runtime/src/*.js`, bundled in name order into one IIFE; the bundler emits ASCII only). Roles (`00-core.js`): `start` (the window the file was opened in: start screen, then presenter view; never runs the source scripts), `audience` (popup with `window.name = "df-audience:<token>"`, or solo through `sessionStorage df-solo`; owns the state and the auto timer), `preview` (srcdoc iframes in the presenter view; get their assets through `parent.__deckfold.assets`).
- `10-loader.js`: placeholders to blob URLs, activate styles and attributes, shim for `setAttribute`, `src` setters, XHR, fetch, Worker and `<style>` inserted at runtime, start the deferred scripts and replay a missed `DOMContentLoaded`/`load`.
- `20-model.js`: state model (slide, step). The runtime keeps the state itself and drives source players only through synthetic keys (with `keyCode`) or `#n`; never read internal player variables. The PowerPoint player is deckfold's own and is driven through its API (see above). A negative entry in `delay` means "no source timing for this step".
- `30-keys.js`: capture listeners on `window` are registered as soon as the runtime loads (before any source script); bound keys never reach the player, Escape included. Channel: `postMessage` with sender and token checks (no BroadcastChannel: messages do not arrive in WebKit).
- `40-audience.js` / `50-presenter.js`: the audience window is the authority for the state; the presenter view sends commands, buffers them until it reconnects, and answers `hello` with `init`. Heartbeat, recovery through `sessionStorage`, wake lock, full-screen fallback, offline placeholder for live embeds.
- Settings: defaults from `#df-config`, differences in `localStorage` under `deckfold:<deckHash>` (Chrome shares `localStorage` across all `file://` pages).

Background measurements on `file://` behaviour (Window Management API, blob URLs in popups and iframes) and on large files: `docs/spikes/S2-file-protocol.md`, `docs/spikes/S3-large-files.md`.

## PowerPoint automation

`scripts/pptxref/pptxref.sh` drives PowerPoint through AppleScript to make the reference images (see `fixtures/pptx/README.md` and `docs/spikes/S4-powerpoint-reference.md`). PowerPoint's sandbox shows modal "Grant Access" dialogs for files it has not opened before, and the maintainer has to confirm them; a folder grant does not stop them, and macOS blocks writing into PowerPoint's container. Tell the maintainer before every run, keep the fixed work folder `~/Documents/deckfold-pptxref` with fixed file names, and do not query PowerPoint through AppleScript just to check its state (that starts it again). PowerPoint does not answer Apple events while a start dialog is open; close it first. Work only in folders below `~/Documents` and quit PowerPoint afterwards when no presentation is open.

## Keynote automation

Never close a Keynote document through AppleScript (Keynote crashes). Export only into folders below `~/Documents` (Keynote is sandboxed). A native Keynote test deck with builds, video, audio and self-playing mode is still missing.

If you launched Keynote (or any other app or long-running process such as a Playwright browser or a local server) during a task, stop it before you report the task as done. For Keynote, first check that no document is open (`osascript -e 'tell application "Keynote" to get name of every document'`), then quit it with `tell application "Keynote" to quit`. Use the name of the installed app; on some Macs it is "Keynote Creator Studio". Never use `close` on a document (see above).
