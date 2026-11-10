# Content model

Every deckfold output of a claude.ai, Keynote or PowerPoint deck carries a description of what is written on its slides. The runtime uses it for the search and the outline, and `deckfold deck.html --content out.json` (or `out.md`) writes it to a file for other programs. Decks embedded with `--generic` have no content model.

The model is a JSON block in the output file, next to the deck metadata:

```html
<script type="application/json" id="df-content">{"version":1,"slides":[…]}</script>
```

## Schema

```json
{
  "version": 1,
  "slides": [
    {
      "title": "Quarterly figures",
      "blocks": [
        { "role": "title", "text": "Quarterly figures", "step": 0 },
        { "role": "item", "text": "Revenue is up", "step": 1 },
        { "role": "item", "text": "in every region", "step": 2, "level": 1 },
        { "role": "text", "text": "Back to the overview", "step": 0, "jump": { "slide": 1 } }
      ]
    }
  ]
}
```

`slides` has one entry for every slide of the deck metadata (`deck.slides` in `#df-meta`), in the same order. Slides that are hidden in a PowerPoint file are in neither list. Presenter notes stay in the deck metadata (`deck.slides[].notes`); `--content` puts both together.

| Field | Meaning |
|---|---|
| `title` | The title of the slide. It is only there when the source marks one: the title placeholder of a PowerPoint slide, the label of a claude.ai slide. Keynote exports mark no title. |
| `blocks` | The text on the slide in the order of the source. A slide without text has an empty list. |
| `role` | `title`, `subtitle`, `text`, `item` (a list entry) or `alt` (the description of a picture). |
| `text` | The text with white space collapsed. A line break inside a block is a space. |
| `step` | The first step of the slide in which the block shows; 0 means from the start. When a block sits inside something that appears later, the later step counts. `-1` means the block shows in no step of the click order: a PowerPoint shape that only a trigger shows, or one that stays hidden. The field is missing when the source does not tell the step, which is the case for every block of a Keynote export. |
| `level` | The list level of an `item`, left out for the first level. |
| `jump` | A link to another slide: `{"slide": n}` with an index into `slides`, or `{"to": "next"}` with `next` and `prev` (the next and the previous slide, not step), `first`, `last`, `back` (the slide shown before) or `end` (end the presentation). PowerPoint decks and Keynote exports have them. A link to a hidden or skipped slide is not a link here, because such slides are not in the deck. |

There are no positions in the model, and no links to the web.

## What `--content` writes

`deckfold deck.html --content deck.json` writes the model together with what the deck metadata knows: `format` (`claude`, `keynote` or `pptx`), the `title` of the deck, its `width` and `height`, and for every slide its `notes` next to `title` and `blocks`.

`--content deck.md` writes Markdown: the deck title as the heading, then every slide with its title or, without one, as "Slide n", its text as paragraphs, list entries as a list with their levels, picture descriptions as "Image: …" and the notes as a quote. The steps and the links are only in the JSON. Text from the slides is escaped, so it cannot add a heading, a link or HTML to the Markdown.

## Where roles come from

A role comes from the structure of the source. deckfold does not guess one from the size or the place of a text.

- **PowerPoint:** the placeholder type gives `title` (title and centered title) and `subtitle`; a paragraph with a bullet or a number is an `item` with its `level`; every other paragraph is `text`. The description of a picture (its alt text) is `alt`. Blocks are the paragraphs of the shapes on the slide itself, shapes in groups and table cells included. Text from the layout and the master, the placeholders for date, footer and slide number, and the labels of charts are not in the model.
- **claude.ai:** a list entry (`li`) is an `item`, every other text is `text`, the alt text of a picture is `alt`. A block is the text of the innermost block element; bold or colored parts of a sentence belong to the block around them. The slide title is only in `title`, because the export does not mark the element that shows it.
- **Keynote:** every text is `text`. The export draws the slides as pictures; the only text in it is what it lists for screen readers, and that marks neither titles nor lists. The order is the one of that list, which is not always top to bottom. The list has what Keynote puts into it: the name of a picture instead of its description, no text of tables, and the footer. It does not say in which step a text shows: an object that a build shows later is listed from the start, and a step in between can leave out paragraphs that are visible. So Keynote blocks have no `step`. A link is on the block that has exactly the rectangle of the link; a link on a shape has no such block and is followed in the presentation only.

## Versions

`version` is 1. A field added later does not change the version. A field whose meaning changes does. The runtime treats a version it does not know like a file without a model: there is no search and no outline. `--content` stops with an error then and says that the file was made by a newer deckfold.

## Older files

`deckfold deck.html --refresh` adds the model to an output made before the model existed, when the deck came from claude.ai or Keynote: the file still holds everything the model is read from. A PowerPoint output from before has to be converted again from the presentation. If the slide text of a file cannot be read, `--refresh` still updates the controls and says so.
