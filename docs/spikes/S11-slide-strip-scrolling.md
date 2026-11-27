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

## White at the edges while the window is resized (2026-11-27)

The maintainer saw white flash inside the window, at its right and bottom edge, while dragging a corner of the start screen in Safari 27.2 on macOS 27.2, after the layout of decision E25 had made the preview and the tiles follow the window size. The question was whether deckfold draws something late or in the wrong place.

How it was measured: copies of a claude.ai deck with a script after the runtime block that kept, in memory, for every animation frame the window size, the size of the preview's stage against the scaled iframe in it, and the width of the first drawn tile against the picture in it. The preview is fitted in a `ResizeObserver` of deckfold (`fit()` in `32-ui.js`), which runs after the animation frame and before the frame is painted, so a second `ResizeObserver` created later measured the preview as it is painted. The maintainer dragged the window by hand, larger and smaller, for about six seconds in each copy.

- Copy A, the runtime as it is: 306 frames with a new window size. After deckfold had fitted it, the preview was never more than 0.9 px off its stage (0 of 307 fittings above 1.5 px); at the start of a frame, before the fitting, it was up to 64 px off. No tile was more than 0.5 px off its picture. White flashed at the right and bottom edge.
- Copy B, the same with `color-scheme: dark` on the root element of the start screen: 319 frames, the same results, and white flashed the same way.
- Page C, a dark page without deckfold (`html` and `body` in #15161a, one line of text): white flashed the same way.
- Page D, the deck with the runtime from before E25 (a column of 850 px, a preview of at most 42 % of the window height, tiles of 160 px): white flashed the same way.

So the white is not drawn by deckfold and not caused by E25: Safari shows it for any page, apparently on the area a growing window gains before the page has painted it. A page does not see that moment, so nothing in deckfold changes it. Chrome, Firefox and other macOS versions were not tried; nor was a smaller window or a slower drag.

A related sight from the same day, without a measurement of its cause: after the window had been dragged several times and a show had been started in full screen and ended, Safari showed a picture of its own window, toolbar included, over the real one, with nothing in it reacting. It stayed over a reload, a new tab and `about:blank` in the same window; Safari and the macOS accessibility interface reported one window. Quitting Safari removed it. A page cannot draw Safari's toolbar, so the picture was Safari's, left from a change of the window; whether deckfold's full screen or the reload at the end of a show set it off is not known.
