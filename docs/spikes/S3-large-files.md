# Spike S3: large Base64 files

Date: 2026-11-07, MacBook Pro M3 Pro, macOS 27.2. Generator `tests/spikes/s3_gen.py`, measurement `tests/spikes/s3.mjs`.

Test files (random data, Base64 inside `<script type="application/octet-stream">`):

- `one.html`: 225 MB of binary data in **one** block, 300 MB of HTML
- `chunks8.html`: 225 MB in 29 blocks of 8 MB, 300 MB of HTML
- `big375.html`: 375 MB in 47 blocks of 8 MB, 500 MB of HTML

| Browser | File | until `load` | read and decode the first block |
|---|---|---|---|
| Chrome | one (300 MB) | 0.9 s | 225 MB: 435 ms in one go (blocking) |
| Chrome | chunks8 (300 MB) | 0.8 s | 8 MB: 17 ms |
| Chrome | big375 (500 MB) | 1.4 s | 8 MB: 17 ms |
| WebKit | one (300 MB) | 10.5 s | 225 MB: 624 ms |
| WebKit | chunks8 (300 MB) | 10.1 s | 8 MB: 22 ms |
| WebKit | big375 (500 MB) | 30.6 s | 8 MB: 21 ms |

The JS heap (CDP `JSHeapUsedSize`) stays at about 1 MB in Chrome: the Base64 text lives in the DOM,
not in the V8 heap, as long as it is not read. Firefox was not measured (see S2).

## Consequences

- **Block size:** large assets are split into parts of at most 6 MB of binary data (8 MB of Base64), each part is
  decoded on its own and the parts are joined as `Blob([part1, part2, ...])`. No single string is larger than 8 MB,
  and no part blocks for more than about 20 ms.
- **Decoding on demand:** assets requested through the shim (for example Keynote slides) are decoded when the
  player asks for them. Assets referenced directly by the HTML are decoded when the page is activated.
- **Acceptance values** (replacing the initial target of "under 5 s"):
  - Chromium: a 300 MB file shows its first slide within 3 s.
  - WebKit/Safari: parsing alone takes about 10 s for 300 MB and about 30 s for 500 MB. There is no hard limit
    for this fallback browser (decision E3); the README mentions the effect, and the 250 MB warning threshold (E8) stays.

## The Base64 text after decoding (2026-11-22)

Question: the loader decodes an asset when it is first needed and keeps its blob URL, but the Base64 text stayed in its block in the DOM, so every asset in use was held twice, as text (about 1.33 times its size) and as Blob. Does emptying the text give memory back?

Measured on a MacBook Pro M3 Pro with 36 GB, macOS 27.2.1, in Chrome 158 and in the WebKit of Playwright (Safari 27.2 engine, not Safari itself, which cannot be driven by script without a setting in its Develop menu). Memory is the footprint (`footprint`, phys_footprint) of all processes of the browser; the median of three runs. Each run opened the file, started it without the presenter view, went through every slide, decoded every remaining asset, waited 10 s after a forced garbage collection (M1), then emptied the text of every asset block and measured 40 s later (M3). Control runs did the same without emptying, because memory also falls by itself after the start. The criterion, set before measuring: in Chrome, with the file of 300 MB, the memory falls by at least half the Base64 text and at least 50 MB more than in the control.

| Browser, file | Base64 text | Fall M1 to M3, emptied | Fall, control | Difference |
|---|---|---|---|---|
| Chrome, `big.html` of the test setup (300 MB, 225 MB of video) | 299 MB | 313 MB | 11 MB | 302 MB |
| Chrome, a deck from the local corpus (65 MB) | 62 MB | 223 MB | 157 MB | 66 MB |
| Chrome, a Keynote export (13 MB) | 12 MB | 39 MB | 23 MB | 16 MB |
| WebKit, the same three files | 299, 62, 12 MB | 0 to 1 MB | 0 to 1 MB | none |

Emptying all blocks took under 1 ms. In Chrome the memory given back was about the size of the text, so the criterion was met. In WebKit nothing changed; whether WebKit keeps the text elsewhere or does not give the memory back was not found out, and Safari is not measured.

Since then the loader decodes each asset once, keeps its Blob, empties the text of its blocks and marks them `data-df-freed`; `url()` and `fresh()` make their URLs from the kept Blob. Measured again with the new and the old loader (the same files, `--refresh` with each), in Chrome, without emptying by hand:

| Chrome | old loader | new loader |
|---|---|---|
| `big.html`, after going through every slide | 782 MB | 448 MB |
| the page process of it | 405 MB | 286 MB |
| the deck from the local corpus, after going through every slide | 568 MB | 499 MB |
| `big.html`, page process on the start screen without forced garbage collection, after 5 / 30 / 60 s | 996 / 742 / 734 MB | 997 / 436 / 424 MB |

The start screen already decodes every asset of these files for its previews, so the peak right after loading is the same with both loaders; Chrome gives the text back by itself within 30 s.
