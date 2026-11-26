# S11: Scrolling the slide strip with a trackpad

Measured on 2026-11-26. The maintainer had seen the slide strip of the start screen move smoothly when a swipe on the trackpad took it to later slides and jerk when a swipe took it back to earlier ones, in Safari and in Chrome. The question was where that comes from. Nothing in deckfold was changed on the strip, because the measurement did not show the fault.

## How it was measured

The public deck `fixtures/pptx/src/transitions3d.pptx` (38 slides) was converted with the runtime of `main` at that day (commit `dfb1e2f`, tiles 160 px wide, `scroll-snap-type: x proximity`, `scroll-snap-align: start`). A script added after the runtime block of a copy logged, in memory only: the `scrollLeft` of the strip in every animation frame, every `wheel` event with `deltaX`, `deltaY` and `deltaMode`, `scroll` and `scrollend`, the moment a tile got its picture (a `MutationObserver` on the `.df-thumb` elements, since `fillTile` is not reachable from outside the runtime) and long tasks (`PerformanceObserver`, Chrome only). A button showed and copied the log. In Chrome driven by Playwright, scrolling the strip with `mouse.wheel` gave the same frame times with this log and with one that recorded the frames only (median 16.7 ms, 95th percentile 17.4 to 17.7 ms), so the log does not slow the strip down.

The maintainer swiped by hand on a MacBook trackpad, three times to later slides and three times back, in Safari 27.2 and in Chrome 158 on macOS 27.2.

## Results

- The fault did not show in either browser this time (the maintainer's words).
- Safari: 60 frames a second, 16.7 ms apart in both directions; one gap of 68 ms in 693 frames; no long tasks. The wheel events come every 8 ms and fall off smoothly after the finger is lifted.
- Chrome: 120 frames a second, 8.3 ms apart, no gap over 16.5 ms, no long tasks. In six frames the strip moved 120 to 220 px at once where the frames around moved 50 to 100 px (for example from 971 to 1195), three times while going to later slides and three times while going back, so in both directions alike. Where these steps come from is not established; they do not explain a fault in one direction.
- A tile getting its picture fell together with no gap and no jump, in either browser.
- In Chrome driven by Playwright, synthetic wheel events (`mouse.wheel`, 40 px every 16 ms) moved the strip against the direction of the scroll in about a third of the frames that moved, in both directions. A real trackpad in the maintainer's Chrome did not show that.

## What this does not show

The fault itself, which did not appear. A strip with the larger tiles of decision E25 (seven tiles across the window), which came after this measurement. Other trackpads, a mouse with a wheel, Firefox. When the fault is seen again, the copy in `tests/.out/strip/strip-log.html` (or a new one built the same way) gives the log of that case; the summary at its top groups wheel events less than 400 ms apart into one gesture, so swipes in quick succession count as one, and the raw data below it are what to read.
