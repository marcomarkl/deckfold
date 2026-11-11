# S8: Converting pictures and fonts

Measured on 2026-11-11 for decision E17. The question was which formats make the embedded assets smaller without changing what a browser shows, and whether deckfold can write them without CGO and without anything installed.

## What the numbers were taken on

The local corpus: ten PowerPoint decks and one Keynote export, with 1596 media files, 809 of them different. Together 1.86 GB: 1532 MB video, 212 MB PNG, 50.5 MB JPEG, 23 MB TIFF, the rest small. The claude.ai deck of the corpus has only WOFF2 fonts and one SVG, so it shows nothing about pictures in HTML exports.

The browser comparisons ran in Chrome (channel `chrome`, SwiftShader, as in the browser tests) and in Playwright's WebKit on macOS 27.2. Firefox does not start there, so nothing below is measured in Firefox.

## Browser support

From the caniuse data of 2026-11-11 (share of users with full support):

| Format | Full support | Limits |
|---|---|---|
| WebP | 97.24 % | Safari from 14, on every macOS from 16.0 |
| AVIF | 96.69 % | Safari from 16.1, Firefox from 93 |
| JPEG XL | 53.99 % | not in Edge and Samsung Internet, partial in Safari |
| APNG | 97.39 % | |
| WOFF2 | 97.4 % | |
| H.264 in MP4 | 97.57 % | Firefox uses the codecs of the operating system |
| HEVC | 18.06 % | partial in Chrome, Edge and Firefox |
| AV1 | 81.05 % | Safari only on devices with a hardware decoder |
| FLAC | 97.37 % | |
| MP3 | 97.57 % | |

TIFF is shown by Safari only (MDN, image file type guide).

## PNG

Lossless WebP against lossless AVIF, with the command-line tools, on all 364 PNG files of the corpus (212.3 MB):

| Target | Result | Saved | Files that did not get smaller |
|---|---|---|---|
| WebP (`cwebp -z 9`) | 111.5 MB | 47.5 % | 0 |
| AVIF (`avifenc --lossless`) | 175.1 MB | 17.5 % | 175 |

Encoders that build without CGO, on a sample of 41 of these files (27.2 MB):

| Encoder | Saved | Time | Not smaller than the PNG |
|---|---|---|---|
| libwebp translated to Go (`gen2brain/webp`), method 6 | 46.7 % | 253 s | 0 |
| the same, method 4 | 45.5 % | 16.9 s | 0 |
| `deepteams/webp`, method 6 | 43.8 % | 16.0 s | 0 |
| `HugoSmits86/nativewebp` | 37.2 % | 8.1 s | 4 |

deckfold uses the first, because the result counts and the time does not. It is libwebp compiled to WebAssembly and translated to Go source, so the binary needs no runtime for it.

Without its `exact` option libwebp changes the colour of fully transparent pixels: in 12 of the 41 files the samples under alpha 0 differed from the PNG. With `exact` all 41 files have every sample of the PNG, and the files are 0.10 % larger. deckfold encodes with `exact`.

### What browsers show

Each pair is a PNG and the WebP file deckfold makes of it, 240 x 160 pixels, compared by screenshot.

The counter-check first: a PNG with a Display P3 profile against a WebP file of the same pixels without the profile differs in about 37 170 of 38 400 pixels, by up to 91 levels, in both browsers. So the comparison does see colour management.

| Pair | WebKit | Chrome |
|---|---|---|
| PNG with profile, WebP with the same profile | equal | differs in half-transparent pixels |
| PNG without colour data | equal | differs in half-transparent pixels |
| PNG with sRGB and gAMA chunk | equal | same count of differing pixels |
| PNG with EXIF data, upright | equal | same count of differing pixels |
| PNG with pHYs chunk (144 dpi) | equal, same size | same count of differing pixels, same size |

In Chrome the half-transparent pixels differ. The test picture has 4661 pixels with alpha 128; 4110 of them are one level of 255 off in the WebP file, and with the P3 profile up to three levels. No pixel outside that area differs; the differing pixels were located for the first two pairs. The samples in the two files are the same, so the difference comes from how Chrome multiplies colour by alpha for the two formats. For the picture without a profile, three graphics settings (SwiftShader, SwiftShader with a forced sRGB profile, no GPU) gave the same count; with the profile, the setting without GPU showed no difference. For this reason a PNG with half-transparent pixels stays a PNG.

A PNG whose colours depend on a gAMA chunk alone (no sRGB chunk, no profile) differs from an untagged WebP file in every pixel, by up to 73 levels with gamma 1.0, in both browsers. WebP has no field for it, so such a PNG stays as well.

### What stays a PNG in the corpus

| Reason | Files | Size |
|---|---|---|
| half-transparent pixels | 123 | 34.8 MB |
| colours from gAMA or cHRM alone | 14 | 10.6 MB |
| 16 bits per channel | 1 | 1.3 MB |

No PNG of the corpus is animated, and none has an EXIF orientation other than upright (38 carry EXIF data).

## GIF

All four GIF files of the corpus were converted, two of them animated: 463 105 bytes became 175 264 (62.2 % smaller).

An animated GIF becomes an animated lossless WebP file. deckfold checks the result with its own reader: it takes the frames from the ANMF chunks, decodes each with `golang.org/x/image/webp`, puts them together by the rules of the format and compares every state and its duration with the GIF. Runs of equal frames count as one state, because the encoder merges them.

Two kinds of GIF stay as they are. Browsers play a frame of less than 20 ms slower than written, and whether they do that for WebP in the same way is not measured. A loop count other than "forever" or "once" is read as n or n + 1 runs depending on the browser.

In the browser the converted animation plays in step with the GIF. A GIF of three frames of 500 ms and its WebP file, side by side on one page and sampled for 6.5 seconds, changed colour at the same moments in Chrome and in WebKit, with every state between 497 and 503 ms long. The same GIF without a loop block stopped on its last frame in both formats and both browsers. Shorter frames were not measured. The browser tests check in Chrome and WebKit that the animated WebP picture of the `gifs` deck starts again when its slide is entered.

## TIFF

13 of the 25 TIFF files were converted (1.32 MB became 0.60 MB). The other twelve are ones the decoder of `golang.org/x/image/tiff` does not read: six in CMYK, three with four or five samples per pixel that it rejects for RGB, three it cannot decompress. They stay TIFF, and the report says so.

The decoder ignores the orientation tag and the colour profile tag; deckfold reads both itself, turns the picture and puts the profile into the WebP file. For TIFF "the same picture" means the pixels of this decoder: only Safari shows the original, so there is nothing to compare against in the other browsers.

## Fonts

`tdewolff/font`, which deckfold already uses, has a WOFF2 writer. Its own reader could not read back 19 of 20 fonts it wrote (the 16 bundled ones and four system fonts). deckfold therefore has a writer of its own.

It transforms no table. The W3C Recommendation "WOFF File Format 2.0" defines this in section 4.1: transformation version 0 is the null transform for every table except `glyf` and `loca`, and version 3 is the null transform for these two. So a reader gets the tables back byte for byte. Two things differ from the source, and both are required: bit 11 of the flags in the `head` table is set (section 5), and a digital signature (`DSIG`) is left out, because it signs the original file.

| Fonts | Before | After | Saved |
|---|---|---|---|
| the 16 bundled fonts | 7.09 MB | 2.86 MB | 59.6 % |
| five system fonts (Arial, Georgia, Tahoma, Times New Roman, Verdana) | 2.85 MB | 1.29 MB | 54.7 % |

The transformation of `glyf` that WOFF2 offers would save more. It is not used, because the result could then not be compared table by table.

## JPEG

JPEG pictures are not converted: a smaller file in another format means encoding again, which changes pixels. Packing the same coefficients with better entropy coding would be lossless. `jpegtran` saves 4.9 % on the 199 JPEG files of the corpus with optimized Huffman tables (50.5 MB become 48.0 MB) and 8.8 % as progressive JPEG (46.0 MB). No Go package that does this without CGO was found, so it is not built.

## Whole decks

Nine of the ten PowerPoint decks convert (the tenth has a video of more than 1 GB, which deckfold refuses). With `--keep-assets` every output is byte for byte the output of the code before E17.

| | Before | After |
|---|---|---|
| converted assets (235 pictures, 42 fonts) | 177.0 MB | 93.6 MB (47.1 % smaller) |
| the nine output files | 980.1 MB | 869.1 MB (11.3 % smaller) |
| time for all nine | a few seconds | 638 s |

The output files shrink much less than the converted assets, because video and JPEG make up most of them and stay unchanged.

## The build tag

Without the build tag `nodynamic`, `gen2brain/webp` loads a libwebp from the system when it finds one. On the Mac the measurements ran on it found none, although Homebrew has one installed; on a Linux machine with `libwebp.so` it would. `media.New` asks the package and converts no picture when a system library is active.

## Video and sound, not built yet

Measured for the next step, on 12-second cuts of the three HEVC videos of the corpus, re-encoded to H.264 with the native x264 (preset "slow"):

| x264 setting | VMAF against the source | Size against the source |
|---|---|---|
| CRF 18 | 96.6 to 97.9 | 44 to 62 % |
| CRF 23 | 95.0 to 95.8 | 22 to 27 % |
| CRF 28 | 90.0 to 93.3 | 11 to 14 % |

Two of the three sources have 10 bits per channel. An encoder inside the binary would be an ffmpeg build as WebAssembly; none has been run yet, so its speed and memory use are unknown.
