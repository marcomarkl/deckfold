# PowerPoint feature decks

Public test decks for the PPTX converter. Unlike `testdata/`, everything here is committed. The content is neutral English text, and the decks use only the fonts deckfold bundles (Carlito and Liberation).

| Folder | Content | Made by |
|---|---|---|
| `src/` | decks written by the generator | `go run ./scripts/pptxgen` |
| `.` | the same decks after PowerPoint opened and saved them | `scripts/pptxref/pptxref.sh` |
| `ref/<name>/` | one PNG per shown slide (PowerPoint's PDF export, rendered with PDFKit), plus `source.sha256` | `scripts/pptxref/pptxref.sh` |
| `powerpoint/` | the animations and transitions decks, with effects and transitions added by PowerPoint itself | `scripts/pptxref/ppdecks.sh` |

The tests prefer the PowerPoint-saved copy, because that is the XML PowerPoint itself writes. When a deck has no saved copy yet, they fall back to `src/` and skip the image comparison. The image comparison also checks `source.sha256`: if the saved copy changed after the reference images were made, the test fails instead of comparing against outdated images.

## The content deck

`content.pptx` is the deck for the content model (`docs/content-model.md`): titles and a subtitle, a list with two levels, a picture with a description, a table, a group, builds, a shape only a trigger shows, a footer and a slide number, a hidden slide, and links to other slides on text and on a shape. The generator writes down next to it what a reader of the deck would list (`src/content.spec.json`); the tests compare that with what the converter reads and with what the player shows in every step. `src/content.md` is the Markdown `--content` writes for the deck; it was read by hand and is compared as a whole.

## Making the reference

1. Nothing to install: the script copies the bundled fonts it needs into `~/Library/Fonts` for the run and removes them again at the end. It quits PowerPoint before, because PowerPoint reads fonts when it starts.
2. Close all open presentations in PowerPoint and dismiss any start dialogs (sign-in, what's new).
   PowerPoint's sandbox shows a "Grant Access" dialog for files it has not opened before. Granting the work folder in that dialog did not stop it in a test, so expect to confirm the dialogs during a run.
3. Run `scripts/pptxref/pptxref.sh` (it works in `tests/.out/pptxref`, which git ignores) with the names of the decks whose source in `fixtures/pptx/src` changed, for example `scripts/pptxref/pptxref.sh basic`; `git status fixtures/pptx/src` shows them. Without arguments the script runs over every deck, which costs a round of access dialogs per deck and rewrites the metadata of every saved copy; avoid it.

PowerPoint writes new metadata every time it saves, so a run over an unchanged deck changes its saved copy, `source.sha256` and sometimes a reference image, although nothing in the content changed. Discard such changes with `git checkout -- fixtures/pptx/<name>.pptx fixtures/pptx/ref/<name>`.

If PowerPoint shows a dialog while opening a deck (for example a repair prompt), the script stops after its timeout. A repair prompt means the generator wrote invalid XML; fix the generator instead of saving the repaired deck.

## Decks built in PowerPoint

`fixtures/pptx/powerpoint/animations.pptx` and `transitions.pptx` let the tests read the timing and transition XML as PowerPoint writes it for effects it created itself. `scripts/pptxref/ppdecks.sh` writes two base decks with `go run ./scripts/pptxgen -ppbase`, has PowerPoint add the effects and transitions through AppleScript (`ppdecks.applescript`) and saves the result here. The same notes about access dialogs apply as for `pptxref.sh`.

- `animations.pptx`: Fade, Fly In and a Fade exit on click; Grow/Shrink and a motion path to the right; a three-paragraph build with Fade; a click effect followed by With Previous and After Previous.
- `transitions.pptx`: Fade, Push, Wipe, Reveal, Cover, Cut and Morph, and a slide that advances after 3 seconds.

PowerPoint's scripting dictionary sets neither trigger shapes nor delays, so triggers and delays are covered by the generated decks only (`triggers.pptx`, `animations.pptx` in `src/`).

## Transitions as PowerPoint writes them

PowerPoint reports a deck as damaged when a PowerPoint 2010 transition is not in the exact form PowerPoint itself writes: for example `p14:flip dir="l"` with an explicit direction, `p15:prstTrans prst="..."` for the newer ones, and no `p14:dur` next to them. The generator (`pptxgen.TransitionNames`) writes that form; the form came from transitions PowerPoint set itself through AppleScript.
