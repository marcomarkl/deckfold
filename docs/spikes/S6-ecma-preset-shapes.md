# S6: License of the preset shape definitions

Checked on 2026-11-07.

PowerPoint stores most shapes only by name (`<a:prstGeom prst="roundRect">`). The geometry of the about 190 presets is defined in `presetShapeDefinitions.xml`, which ECMA-376 Part 1 ships as an electronic annex (`OfficeOpenXML-DrawingMLGeometries.zip` inside the download of the 5th edition, December 2016).

## What the license allows

Ecma publishes its standards under the default Ecma copyright notice ([Ecma text copyright policy, version 3](https://ecma-international.org/wp-content/uploads/Ecma_text_copyright_policy_version_3.pdf)). It allows copies and four kinds of derivative works, among them:

> works by making use of this specification in standard conformant products by implementing (e.g. by copy and paste wholly or partly) the functionality therein.

The condition is that the copyright notice and the license text stay with the copy or derivative work, and that the content itself is not modified.

Other open source projects ship the same file: LibreOffice (`oox/source/drawingml/customshapes/presetShapeDefinitions.xml`, MPL 2.0) and Apache POI (Apache License 2.0).

## Consequence for deckfold

- deckfold embeds the file unchanged with `go:embed` and parses it on first use, instead of turning it into generated Go code. That is simpler, and the file stays exactly as Ecma published it.
- The full Ecma copyright notice goes into a `NOTICE` file next to the XML and into `THIRD_PARTY_LICENSES.md`, which is published with every release.
- The Ecma name is not used to promote deckfold.
