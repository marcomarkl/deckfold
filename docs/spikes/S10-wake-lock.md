# S10: Screen wake lock

Measured and researched on 2026-11-21. The question was whether the presentation window keeps the screen saver, display sleep and the automatic screen lock away while a deck is shown, in which browsers and on which systems. Measurement script: `tests/spikes/s10.mjs`.

## What deckfold does

Before decision E19, `wake()` in `runtime/src/40-audience.js` called `navigator.wakeLock.request("screen")` once the presentation had started, and again whenever the page became visible. It did not look at the result, so a refused request went unnoticed. The first measurements below were taken with that code.

Since E19 (see "Safari wants a click" and "What changed" below):

- The request is made in the click or key on "Start" itself, before full screen is requested (`begin()` in `runtime/src/35-start.js`). The presentation takes it over.
- The presentation holds the lock. It asks again on every key, click and command while it holds none, when the page becomes visible, when the browser releases the lock, and every 10 s after a refusal. A request that is still open is never joined by a second one.
- `info().wake` says `pending` (a request open for less than 3 s), `held`, `off` or `none` (no API). The presenter view shows "Screen saver not blocked" in its status line while the presentation reports `off` or `none`; the audience sees nothing.
- The start screen and the presenter view request no lock. The lock does not depend on full screen.

## Measured on a Mac

macOS 27.2.1, Google Chrome 158.0.8127.36 (channel `chrome`) and the WebKit of Playwright 1.65.0, both headed. The deck was `fixtures/pptx/basic.pptx`, converted with `DECKFOLD_BUNDLED_FONTS=1` and opened over `file://`. The settings of the machine: screen saver after 600 s (`defaults -currentHost read com.apple.screensaver idleTime`), lock 900 s after the screen saver starts or the display turns off (`sysadminctl -screenLock status`), display sleep after 30 minutes (`pmset -g`).

### Is the lock granted (`s10.mjs grant`)

| | Chrome | Playwright WebKit |
|---|---|---|
| `isSecureContext` on `file://` | true | true |
| `navigator.wakeLock` | present | present |
| `request("screen")` | resolves | never settled; granted since the request is made in the click on "Start" |
| power assertion while presenting | `NoDisplaySleepAssertion named: "Blink Wake Lock"`, `PreventUserIdleDisplaySleep` goes from 0 to 1 | none; since E19 `PreventUserIdleDisplaySleep named: "Screen Wake Lock"` |
| after the browser is closed | assertion gone | |

The Chrome assertion stayed when the window was switched to full screen through CDP. `pmset` lists the assertions of every process, and at times other Chrome processes on the machine held their own "Blink Wake Lock"; the assertion of the measured browser is the one whose process started with it and whose assertion ended when it closed. Safari was measured by hand later (below); the Playwright WebKit result alone said nothing about it.

Minimizing the window through CDP (`Browser.setWindowBounds`, `minimized`) left `document.visibilityState` at `visible` and the lock in place. Playwright keeps its pages visible, so covered and minimized windows were measured by hand (below).

### Does the screen saver start while the lock is held (`s10.mjs idle`)

Nobody touched the machine during the run; the HID idle time (`ioreg -c IOHIDSystem`, `HIDIdleTime`) rose without a reset. The script looked for the screen saver process (`ScreenSaverEngine` or `legacyScreenSaver`) and for `CGSSessionScreenIsLocked = Yes` in `ioreg -n Root -d1`.

| Phase | Idle time | Display assertions | Screen saver | `CGSSessionScreenIsLocked` |
|---|---|---|---|---|
| A: presentation in full screen, lock granted, page `visible` | 48 s to 699 s | only the "Blink Wake Lock" of the measured Chrome (after the first 33 s) | did not start | no |
| B: browser closed | 699 s to 851 s | none | did not start | no |
| B | 861 s | none | `legacyScreenSaver` running | yes |

With the lock held, the screen saver did not start, although the idle time passed its delay of 600 s by 99 s. Phase B is the control: once no display assertion was left, the screen saver started on its own, between 152 and 162 s after the browser was closed. The assertion of the Claude desktop app (`NoIdleSleepAssertion`, system sleep only) was there in both phases, so it does not keep the screen saver away.

What the run does not show:

- Why the screen saver started 2.5 minutes after the release and not at once.
- Whether `CGSSessionScreenIsLocked = Yes` means that a password was needed. The key came with the screen saver, although the lock delay is 900 s.
- Whether the display turns off after 30 minutes while the lock is held, and whether the lock that follows 900 s after the screen saver would come; the run was shorter than both.
- A lock enforced by an MDM profile.
- Safari, Firefox, Windows, Linux and ChromeOS.

## Safari wants a click

Safari 27.2 on the same Mac, by hand on 2026-11-21: the maintainer opened the files in Safari and clicked, and `pmset -g assertions` was read after each step. Nothing of this ran in Playwright.

A build that asked after the start, as the code before E19 did (already with the new request after a refusal), showed no display assertion at all when `basic.html` was started in Safari. A test page (`tests/.out/wakelock/gesture.html`, not committed) made the request at different times:

| Request | Result |
|---|---|
| inside a click | granted; `PreventUserIdleDisplaySleep named: "Screen Wake Lock"` by Safari |
| 1.5 s after a click | granted |
| 0.5 s after the page loaded, without a click | `NotAllowedError: Permission was denied` |
| 10 s after a click | `NotAllowedError: Permission was denied` |
| in one click: `requestFullscreen()`, then the wake lock | wake lock refused, full screen on |
| in one click: the wake lock, then `requestFullscreen()` | both granted |

Safari grants the lock only shortly after a click or key, and `requestFullscreen()` uses that click up (the Fullscreen API consumes the user activation). That build asked after `setTimeout(0)` and after the source scripts had been activated, and the start screen had requested full screen in the same click; both explain the refusal. The cases that worked fit too: the request inside a click and 1.5 s after it had an activation left.

With the code of E19, `basic.html` in Safari with the presenter view and full screen on: the assertion "Screen Wake Lock" appeared, and the presenter view showed no hint. Earlier, in a build that asked in the click but after `requestFullscreen()`, Safari refused and the presenter view showed the hint.

What follows from this: once Safari has released the lock, for example while the window was hidden, a request without a click is refused, so the 10 s rhythm cannot win it back there; the next key or click in the presentation window can. Whether Safari releases the lock for a minimized or covered window was not measured. Keys pressed in the presenter view reach the presentation as commands, not as a click or key of its own, so they give Safari no activation there.

## Covered and minimized windows in Chrome

Google Chrome 158, by hand on 2026-11-21, with the code of E19, the presenter view on the same screen and full screen off:

- The presenter view laid fully over the presentation window: the "Blink Wake Lock" assertion stayed, with the same ID, for at least 17 s after the window was covered (read 31 s after it was made).
- The presentation window minimized to the Dock: the assertion was gone, and the presenter view showed "Screen saver not blocked".
- The window brought back from the Dock without a click: a new assertion, and the hint went away.

A covered presentation window therefore keeps the lock in Chrome, and the presenter view needs no lock of its own for that case. A minimized one loses it while it is minimized; the presenter view says so.

## What changed (decision E19)

- The request in the click on "Start", before full screen, and the request on keys and clicks: from the Safari measurements.
- The new request after a release and every 10 s after a refusal: no browser was seen releasing a lock while the page stayed visible; it costs a few lines and covers it.
- The hint in the presenter view: the presenter sees when the browser does not hold the lock, for example in a browser without the API or with a minimized presentation window.
- No lock in the presenter view: a covered presentation window keeps its lock in Chrome.
- The browser tests (`tests/e2e/presenting.spec.mjs`, "wake lock") use a stand-in for `navigator.wakeLock` and check when deckfold asks, in which event, before full screen, and what the presenter view shows. What a lock does to the system only the measurements here show.

## What the sources say

A search of the specifications, the browser sources, the vendor documentation and the bug trackers on 2026-11-21. It shows which system call each browser makes. Whether that call keeps the screen saver and the lock away is documented for none of macOS and Windows, and no source reports a measurement.

The specification ([Screen Wake Lock API](https://www.w3.org/TR/screen-wake-lock/), a working draft) limits the API to secure contexts and releases every lock when the document becomes `hidden`; a request from a hidden document is refused with `NotAllowedError`. Which window states a browser reports as `hidden` it leaves open. Whether `file://` is a secure context is up to the browser: [Secure Contexts](https://w3c.github.io/webappsec-secure-contexts/) says it SHOULD be and allows browsers to leave it out. Chrome and WebKit treat it as secure on this Mac (table above, and S2).

Chromium (Chrome, Edge and the other browsers built on it) turns the type `screen` into `kPreventDisplaySleep` ([wake_lock_type.cc](https://chromium.googlesource.com/chromium/src/+/main/third_party/blink/renderer/modules/wake_lock/wake_lock_type.cc)). Per system, on the `main` branch of 2026-11-21:

| System | Call | Screen saver and lock |
|---|---|---|
| macOS | IOKit assertion `kIOPMAssertionTypeNoDisplaySleep` ([power_save_blocker_mac.cc](https://chromium.googlesource.com/chromium/src/+/main/services/device/wake_lock/power_save_blocker/power_save_blocker_mac.cc)) | No documentation from Apple found. A [forum thread](https://developer.apple.com/forums/thread/26776) reports that such an assertion stopped holding off the screen saver with OS X 10.11. On this Mac it did hold it off (above). |
| Windows | `PowerSetRequest` with `PowerRequestDisplayRequired`, before Windows 11 also `PowerRequestSystemRequired` ([power_save_blocker_win.cc](https://chromium.googlesource.com/chromium/src/+/main/services/device/wake_lock/power_save_blocker/power_save_blocker_win.cc)) | Not documented. The policy [Interactive logon: Machine inactivity limit](https://learn.microsoft.com/en-us/previous-versions/windows/it-pro/windows-10/security/threat-protection/security-policy-settings/interactive-logon-machine-inactivity-limit) locks by starting the screen saver; whether a power request stops it is not stated. For the older `SetThreadExecutionState`, which Chromium no longer uses, [Microsoft's reference](https://learn.microsoft.com/windows/win32/api/winbase/nf-winbase-setthreadexecutionstate) says it "does not stop the screen saver", while [Raymond Chen](https://devblogs.microsoft.com/oldnewthing/20090820-00/?p=17043) wrote the opposite in 2009. |
| Linux | `org.gnome.SessionManager.Inhibit` with the flag that keeps the session from going idle, otherwise `org.freedesktop.ScreenSaver.Inhibit`; under Wayland also `zwp_idle_inhibitor_v1` (from a code comment); with `--headless` nothing ([power_save_blocker_linux.cc](https://chromium.googlesource.com/chromium/src/+/main/services/device/wake_lock/power_save_blocker/power_save_blocker_linux.cc)) | The [idle inhibition draft](https://specifications.freedesktop.org/idle-inhibit/latest) names the screen saver as one of the things an inhibit stops, the lock it does not name. Whether GNOME, KDE and others follow it was not checked. |
| ChromeOS | `PowerPolicyController::AddScreenWakeLock`, passed on to powerd ([power_save_blocker_ash.cc](https://chromium.googlesource.com/chromium/src/+/main/services/device/wake_lock/power_save_blocker/power_save_blocker_ash.cc), [power_policy_controller.cc](https://chromium.googlesource.com/chromium/src/+/main/chromeos/dbus/power/power_policy_controller.cc)) | A policy can turn screen wake locks into system wake locks or ignore them. What powerd does with them was not checked. |

Firefox has the API since [bug 1589554](https://bugzilla.mozilla.org/show_bug.cgi?id=1589554) (version 122, Nightly only). For release builds the sources disagree: a comment in [bug 1874849](https://bugzilla.mozilla.org/show_bug.cgi?id=1874849) points to the notes of Firefox 124, [MDN](https://developer.mozilla.org/en-US/docs/Mozilla/Firefox/Releases/126) lists it under 126. On Linux Firefox tries the freedesktop interfaces, the XDG portal, GNOME, XScreenSaver and Wayland idle-inhibit in turn ([WakeLockListener.cpp](https://searchfox.org/mozilla-central/source/widget/gtk/WakeLockListener.cpp), [bug 1587360](https://bugzilla.mozilla.org/show_bug.cgi?id=1587360)). Under XFCE the inhibition stopped after a long run and the screen saver came back; [bug 1862159](https://bugzilla.mozilla.org/show_bug.cgi?id=1862159) has the fix in Firefox 144. What Firefox calls on macOS and Windows was not found.

Safari has the API since 16.4 ([WebKit blog](https://webkit.org/blog/13966/webkit-features-in-safari-16-4/), [WebKit bug 247573](https://bugs.webkit.org/show_bug.cgi?id=247573)). Which macOS call it makes, and whether it needs a user gesture, was not found in the sources; the measurements above show both ("Safari wants a click").

## Open

- Safari on macOS: whether the lock it grants holds off the screen saver (the idle run was made with Chrome only), and what it does with a minimized or covered window.
- Display sleep and the lock after the screen saver delay, in a run longer than 30 minutes.
- Windows with and without the inactivity policy, Linux under GNOME and KDE, Firefox on all systems.
