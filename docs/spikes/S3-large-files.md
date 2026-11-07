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
