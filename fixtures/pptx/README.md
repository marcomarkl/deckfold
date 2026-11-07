# PowerPoint feature decks

Public test decks for the PPTX converter. Unlike `testdata/`, everything here is committed. The content is neutral English text, and the decks use only the fonts deckfold bundles (Carlito and Liberation).

| Folder | Content | Made by |
|---|---|---|
| `src/` | decks written by the generator | `go run ./scripts/pptxgen` |
| `.` | the same decks after PowerPoint opened and saved them | `scripts/pptxref/pptxref.sh` |
| `ref/<name>/` | one PNG per slide exported by PowerPoint, plus `source.sha256` | `scripts/pptxref/pptxref.sh` |

The tests prefer the PowerPoint-saved copy, because that is the XML PowerPoint itself writes. When a deck has no saved copy yet, they fall back to `src/` and skip the image comparison. The image comparison also checks `source.sha256`: if the saved copy changed after the reference images were made, the test fails instead of comparing against outdated images.

## Making the reference

1. Install the bundled fonts into `~/Library/Fonts` (no admin rights needed).
2. Close all open presentations in PowerPoint and dismiss any start dialogs (sign-in, what's new).
3. Run `scripts/pptxref/pptxref.sh`, or `scripts/pptxref/pptxref.sh basic` for one deck.

If PowerPoint shows a dialog while opening a deck (for example a repair prompt), the script stops after its timeout. A repair prompt means the generator wrote invalid XML; fix the generator instead of saving the repaired deck.

## Decks built in PowerPoint

Two decks have to come from PowerPoint itself, so that the tests also see the animation and transition XML that PowerPoint writes. Build them by hand when they cannot be scripted, save them as `animations.pptx` and `transitions.pptx` in this folder, and keep the content neutral.

`animations.pptx`, one slide per item, each with a title that names it:

- [ ] Entrance: Fade on a rectangle, on click
- [ ] Entrance: Fly In from the bottom, on click
- [ ] Exit: Fade Out, on click
- [ ] Emphasis: Grow/Shrink to 150 %, on click
- [ ] Motion path: Line to the right, on click
- [ ] Paragraph build: a three-bullet text placeholder with Fade, "By paragraph"
- [ ] With Previous and After Previous (0.5 s delay) after a click effect
- [ ] Trigger: an effect started by clicking another shape

`transitions.pptx`, one slide per transition: Fade, Push, Wipe, Reveal, Cover, Cut, Morph, and one slide with "After: 00:03" automatic advance.
