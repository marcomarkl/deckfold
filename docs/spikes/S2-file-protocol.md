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

## Two screens in headed Chrome (2026-11-08)

Reported from a manual test with v0.3.0 and Chrome 156 on two screens: after "Start" the presenter view went into full screen, but no audience window opened ("Looking for the audience window"); "Reopen" opened it on the presenter's screen.

Reproduction with Playwright and Google Chrome 156.0.8078.75 in headed mode on the same machine (an external 5K screen and the built-in display). Playwright starts Chrome with `--disable-popup-blocking`, so the script removed that switch (`ignoreDefaultArgs`) and granted the window management permission through CDP (`Browser.grantPermissions` with `windowManagement`), because Playwright's own permission list does not know it. Clicks came from Playwright input events.

| Case | `window.open` | Audience window | Presenter view |
|---|---|---|---|
| (a) `requestFullscreen({screen})`, then `window.open` with `fullscreen` in the features | window | on the built-in display, at the chosen position, not in full screen | full screen |
| (b) without full screen for the presenter view | window | the same | window |
| (c) as (b), without `fullscreen` in the features | window | the same | window |

- The reproduction did not show the reported failure: in all three cases the audience window opened on the chosen screen. What differed in the manual test (the browser profile, the permission granted through the prompt in the same session) is not known.
- `fullscreen` in the window features neither blocks the popup nor opens it in full screen in Chrome 156; the feature was an origin trial (Chrome 119 to 122). deckfold no longer passes it.
- `navigator.keyboard.lock(["Escape"])` resolves on `file://` in element full screen. With it, one real Escape ended the presentation in the maintainer's test.
- A second manual test with the presenter view kept as a window still put the audience window on the laptop, at "Start" and at "Reopen", although deckfold passed the coordinates of the external screen. A reproduction with the opener on the laptop placed the window correctly. Note for reproductions: `--window-position` and `--window-size` apply to every new window of Chrome and override the position of a popup; move the opener through CDP (`Browser.setWindowBounds`) instead. Clicks must come from input events, because Chrome ignores the position of a popup opened without a user gesture.

## Reproducing window and mouse behaviour (2026-11-09)

Measured with Playwright and Google Chrome in headed mode on the built-in display only, while looking for a presenter view that moved after "Start" and a hover highlight that disappeared on a second screen:

- `requestFullscreen` from a Playwright click set `document.fullscreenElement`, but the window stayed in the state "normal" at its size (900 x 600). With CDP `Browser.setWindowBounds` and `windowState: "fullscreen"` Chrome reported "fullscreen" at the size of the screen (1728 x 1084). Whether that matches the native macOS full screen a real click starts, with its own Space, is not known. Windows were placed with `Browser.getWindowForTarget` and `Browser.setWindowBounds`.
- Playwright's mouse events do not move the real mouse pointer, so hover behaviour that depends on the pointer and the screen it is on cannot be reproduced this way.
- For a measurement in which the maintainer works the windows, `addInitScript` loads the logging into every window (the CSP of the deck does not apply to it) and `exposeBinding` collects the log in the test script across reloads.

A fault that does not show up under these conditions is not ruled out.

## Why deckfold does not place windows (2026-11-08)

A research pass over the W3C Window Management specification, the Chrome documentation and the Chromium tests found:

- Without the `window-management` permission, Chrome opens a popup and moves it with `moveTo` only on the opener's screen, and clamps the coordinates to that screen. With the permission, `left`/`top` count from the origin of all screens, but the window always stays inside its target screen.
- The permission prompt appears only during a user gesture. Whether and how Chrome stores the permission for `file://` pages was not documented in any source found.
- `requestFullscreen({screen})` and the "fullscreen companion window" (full screen on one screen plus a popup on another in one click) need the screen object from `getScreenDetails()`, so the same permission.
- Full screen without a gesture ("Automatic Fullscreen") needs an enterprise policy.
- Safari and Firefox have no Window Management API.

So placement can only work on some machines and depends on a state the page cannot see or fix. Since v0.4.0 (decision E14) the window the file is opened in is the presentation, the presenter view is a popup on the same screen, and the user drags the windows where they belong.

## Moving the presenter view by hand (2026-11-08)

In the maintainer's test on macOS (Chrome 156, "Displays have separate Spaces" on), the hint that both windows are on the same screen stayed after the presenter view was dragged to the other screen, until the window was resized. Resizing also made the window jump or shrink; deckfold does not resize windows. A diagnostics panel (Shift+D, removed again with the check) showed the values both windows reported. It then showed what happened (2026-11-09): the presenter view had been dragged to the external screen (x 0 to 2560), but its frame (x 2043, width 1200) still reached 683 px onto the laptop screen (x 2560 to 4288), and the screen Chrome reported was the laptop.

## Which screen a browser reports for a window (research, 2026-11-09)

Sources: vendor documentation, Chromium, WebKit and Firefox source at `main`, the W3C specification. Everything below is documented or in the source; what is not is listed at the end.

- **Specification:** "Operating systems and user agents are assumed to define a canonical screen for a given Window, for example the screen with the largest area intersecting the window." `ScreenDetails.currentScreen` corresponds to `window.screen` (W3C Window Management, https://w3c.github.io/window-management/, §2.10 and §3.3).
- **macOS:** `NSWindow.screen` is "the screen where most of the window is on" (https://developer.apple.com/documentation/appkit/nswindow/screen); Chrome (`ui/display/mac/screen_mac.mm`) and Safari (`Source/WebKit/UIProcess/mac/WebViewImpl.mm`) take `window.screen` from it. With "Displays have separate Spaces", windows "may not visibly span displays", and "A window will get assigned to the display containing the mouse if the window is moved by the user"; it is clipped at the edge of that display (AppKit release notes for 10.9, https://developer.apple.com/library/archive/releasenotes/AppKit/RN-AppKitOlderNotes/index.html). So the screen a user sees and the screen the browser reports can differ, and no web API tells which display shows the window. That explains the measurement above.
- **Windows:** `MonitorFromWindow` returns "the display monitor that has the largest area of intersection with the window" (https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-monitorfromwindow); Chrome uses it (`ui/display/win/screen_win.cc`) and updates on every `WM_MOVING`/`WM_MOVE` (`ui/views/win/hwnd_message_handler.cc`). Firefox picks the screen with the greatest intersection itself (`widget/ScreenManager.cpp`). A window may span monitors.
- **Linux:** X11 behaves like Windows (largest overlap). Under Wayland a client does not know its position: Chrome assumes windows at (0, 0) (commit b439ff33, `ui/ozone/platform/wayland`), and Firefox reports `availLeft`/`availTop` as 0 (https://bugzilla.mozilla.org/show_bug.cgi?id=1795066).
- **Permission:** `getScreenDetails()`, `currentScreen` and `currentscreenchange` need the `window-management` permission; `screen.isExtended` and the `change` event of `window.screen` do not, but Firefox and Safari have neither (MDN browser-compat-data, `api/Screen.json`).
- **Other tools:** Google Slides requires Chrome and the screen permission for presenter view on several screens (https://support.google.com/docs/answer/1696787); reveal.js opens its speaker view as a plain window and leaves the placement to the user (`plugin/notes/plugin.js`). No documented permission-free method to compare the screens of two windows was found.

First decision (E14, maintainer, 2026-11-09): Chrome and Edge use the permission and compare `currentScreen` of both windows, which blocks full screen while they share a screen; without the API (Safari, Firefox) or without the permission, deckfold does not check where the windows are, as in reveal.js.

Revised the same day after the maintainer's next test, in which this check did not work reliably: deckfold no longer asks for the permission and checks in no browser where the windows are, as in reveal.js. Full screen can always be chosen, and placing the windows is left to the user. The diagnostics panel (Shift+D) went with the check.

Not documented: when macOS moves a clipped window fully onto one display; whether `NSWindow.screen` uses the Space assignment or the geometry first; how Chrome and Firefox treat `file://` as a secure context for these interfaces; Chrome's `screenX` under Wayland in a test.

## Full screen of the presenter view after a drag to another screen (2026-11-22)

Since decision E23 (2026-11-25) the presenter view has no full screen of its own, so this fault can no longer be met there. The section stays as the record of what was measured; the same fault of Chrome would hold for any window that is dragged to another screen and then asks for full screen.

Reported by the maintainer (Chrome 158, macOS 27.2, external 5K screen at x 0 to 2560 and the laptop screen at x 2560 to 4288, "Displays have separate Spaces" on): after the presenter view was dragged from the laptop screen to the 5K screen, the first click on its full-screen button made the window jump and stay a window; a second click worked. Before the start, during the show and after it. Safari, with the same file and the same moves, went to full screen at the first click.

A log in both windows (position, `window.screen`, full-screen events, keys, clicks with their screen position, every 100 ms) showed, in the maintainer's Chrome:

- During the drag Chrome reported one new position only. Afterwards the window lay fully on the 5K screen, but Chrome reported it at x 2006 to 3206 and `window.screen` named the laptop screen; the click on the full-screen button was reported at x 3175, on the laptop screen as well.
- `requestFullscreen()` resolved and `fullscreenchange` fired; about 90 ms later full screen ended, and only then `window.screen` named the 5K screen.
- After a further drag on the 5K screen Chrome reported every position (11 in about a second), and full screen worked at the first click.
- Windows that Chrome knew to lie inside one screen went to full screen at the first click every time (16 of 16 dragged by the maintainer, 45 of 45 moved by script).

So after a drag to another screen Chrome can hold a wrong position and screen for its own window, and macOS ends full screen when it is asked for on that basis. Chrome asks macOS for full screen with `-[NSWindow toggleFullScreen:]` and takes `window.screen` from `NSWindow.screen` (Chromium 158, `native_widget_ns_window_fullscreen_controller.mm`, `ui/display/mac/screen_mac.mm`). Keyboard Lock is not involved: on macOS it only sets a flag in the view.

A page cannot correct this, measured in Chrome 158:

- `window.moveTo` keeps the window inside the screen Chrome believes it is on: 5 of 5 moves to the other screen ended at the edge of the old one. A first fix that moved the presenter view onto "its" screen before full screen took it back to the screen it came from.
- `requestFullscreen()` uses up the click; a second call without a new click was refused 0 ms to 6 s after a failure.
- With the window management permission, `requestFullscreen({ screen })` on the screen of the click worked for windows moved by script, but the click itself is reported in the same wrong coordinates, so it would name the old screen; the permission also asks once for every local file. The maintainer chose not to use it.

deckfold therefore leaves full screen as it is; the README names the fault and what helps. How it was measured: Chrome driven by Playwright reports full screen to the page but keeps the window a normal window, and a window dragged by hand cannot be reproduced by script (macOS drops synthetic mouse events without the Accessibility permission, and a window moved by script never got a wrong position). The measurements by script ran in the installed Chrome, started with its own profile in `tests/.out` and `--remote-debugging-port`, with clicks as raw CDP `Input.dispatchMouseEvent` events; once a permission is set through CDP, that Chrome refuses every plain `requestFullscreen()`.

## The highlight under a resting mouse (2026-11-11)

Seen twice with the presenter view on a second screen: the highlight of a slide tile or a button went away while the mouse rested on it. Measured on the maintainer's Mac (macOS 27.2, Chrome 157, a 5K display and the built-in one), with real mouse movements.

- A page logged its own events in a presenter view opened by a script. When the highlight went, the page had received `mouseout` to nothing and `mouseleave` on the document, at a place in the middle of the window where the pointer had rested for eleven seconds. With every timer of the page stopped, the pointer did not come back. So the updates the presenter view makes twice a second are not the cause.
- A page without deckfold, opened as a popup in the maintainer's own Chrome and dragged to the second screen, logged focus as well. While its window was active, the highlight stayed for the 48 s the pointer rested. While it was not, every entry of the pointer was followed by a leave within a tenth of a second, in the middle of the window.

Chrome on macOS shows hover only in the active window. That is the platform's convention for inactive windows (WebKit bug 187545 names it, and says Chrome and Firefox follow it), and a page cannot change it. The runtime sets the focus only on elements of its own window (`grep "\.focus("`), never on another window, so deckfold does not take the active state from the presenter view. What does is a click anywhere else: in the presentation window, for example on "Start", or in another app.

What this does not show: the 48 s were measured on the page without deckfold, not in the presenter view itself. The script-driven window emulates focus, so it could not say whether its window was active.
