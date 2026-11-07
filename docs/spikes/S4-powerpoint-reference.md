# S4: Reference images from PowerPoint

Checked on 2026-11-07 with Microsoft PowerPoint 16.114.4 on macOS 27.2. The spike is not finished; this note records what is known.

## What the scripting dictionary offers

The AppleScript dictionary of PowerPoint (`sdef`) has everything the reference workflow needs:

- `save <presentation> in <file> as save as PNG` (one image per slide), `save as PDF` and `save as Open XML presentation`
- `add effect` on the main sequence of a slide's timeline, with the effect types of the animation gallery (`animation type fade`, `animation type fly`, ...), and `slide show transition` with `entry effect`, `advance on time` and `advance time`

So the two decks that have to come from PowerPoint itself (see `fixtures/pptx/README.md`) can probably be built by script instead of by hand. That is not tested yet.

## What failed

A first script (`make new presentation`, then shapes, an effect, a transition, and the three `save` calls) ended after two minutes with `AppleEvent timed out (-1712)` on its first command. PowerPoint was running but did not answer. The likely cause is a dialog at start (sign-in, what's new or the template gallery), which blocks Apple events until someone closes it. It could not be checked, because screen access to PowerPoint was not granted in that session.

## Still open

- Whether `save as PNG` writes one file per slide, under which names and at which resolution.
- Which state the exported images show. Expected: the editing view, with every object visible, including those with entrance or exit animations. The browser tests rely on this and compare the player's static mode (`__dfPptx.showAll`) with the images.
- The SSIM threshold. The plan starts from 0.95 and calibrates it with one simple slide, measured in Chromium on macOS and on Linux.

`scripts/pptxref/pptxref.sh` is written for this workflow but has not run yet. Until it has, the tests use the generator output in `fixtures/pptx/src` and skip the image comparison.
