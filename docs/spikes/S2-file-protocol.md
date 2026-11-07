# Spike S2: Chromium and WebKit on file://

Date: 2026-11-07. Playwright 1.65.0 with Chromium (bundled), Google Chrome (channel `chrome`) and WebKit 27.2.
Measurement script: `tests/spikes/s2.mjs`; the test pages were an opener `a.html` and a popup `b.html`.

## Results (measured automatically)

| Check | Chromium / Chrome | WebKit (close to Safari) |
|---|---|---|
| `isSecureContext` on `file://` | yes | yes |
| `getScreenDetails` available | yes | no |
| `permissions.query({name:"window-management"})` | `prompt` | TypeError |
| `permissions.query({name:"window-placement"})` | TypeError | TypeError |
| `new Worker("file.js")` | SecurityError | error |
| `new Worker(blobURL)` | works | works |
| `<script src=blobURL>` | works | works |
| `srcdoc` iframe loads a blob URL of its parent | yes | yes |
| blob iframe loads a blob URL of its parent | **no** | yes |
| blob iframe accesses `parent` | SecurityError | yes |
| `window.open` from a click | window opens | window opens |
| popup accesses `opener.document` | SecurityError | SecurityError |
| popup uses a blob URL of the opener | no access | no access |
| `postMessage` popup to opener | works | works |
| `BroadcastChannel` popup to opener | works | **message does not arrive** |
| `localStorage` on `file://` | works | works |

Firefox: the Playwright Firefox build (1572) does not start on macOS 27.2 ("Could not find profile folder"),
not even outside the shell sandbox. Firefox is not installed on the test machine either. **Open.**

## Consequences

- **Window messaging:** `postMessage` through `opener` only, with sender and token checks. No BroadcastChannel (WebKit).
- **Each window decodes its assets itself.** The opener cannot hand its blob URLs to the popup (no DOM access).
- **Preview iframes use `srcdoc`.** They inherit the origin and can use the parent's blob URLs;
  blob iframes cannot do that in Chromium. This makes the planned fallback with transferables unnecessary.
- **Workers:** the shim maps `new Worker(path)` to `new Worker(blobURL)` (for pdf.js in the Keynote player).
- **Permission:** only `window-management` is queried; `window-placement` throws a TypeError in current browsers and is caught.
- **Feature detection** through `'getScreenDetails' in window`; WebKit takes the fallback path (decision E3).

## Not testable automatically (manual test with two screens, open)

- Does the window management permission for `file://` persist across browser restarts?
- Do `requestFullscreen({screen})` followed by `window.open` work with a **single** click (fullscreen companion window)?
- Can the popup enter full screen without its own gesture?
- May the popup play audio without a gesture (autoplay policy)?

The runtime handles each of these failing (full-screen fallback, "Open audience window" button).
The manual check is part of the verification step "manual test with two screens".
