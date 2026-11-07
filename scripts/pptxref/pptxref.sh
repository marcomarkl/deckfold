#!/bin/sh
# pptxref: make the PowerPoint reference for the feature decks (macOS, Microsoft PowerPoint).
#
# For every deck in fixtures/pptx/src it
#   1. opens the deck in PowerPoint,
#   2. saves it again as fixtures/pptx/<name>.pptx (the copy the tests read),
#   3. exports it as PDF and renders one PNG per shown slide into fixtures/pptx/ref/<name>/
#      (rasterize.swift, PDFKit; PowerPoint's own PNG export writes no files),
#   4. writes fixtures/pptx/ref/<name>/source.sha256 with the hash of the saved copy.
#
# PowerPoint is sandboxed, so all work happens in ~/Documents/deckfold-pptxref and the results
# are copied back. Install the bundled fonts first (Carlito and Liberation, see THIRD_PARTY_LICENSES.md)
# into ~/Library/Fonts, otherwise PowerPoint substitutes them.
#
# Usage: scripts/pptxref/pptxref.sh [name ...]
set -eu

repo=$(cd "$(dirname "$0")/../.." && pwd)
src="$repo/fixtures/pptx/src"
work="$HOME/Documents/deckfold-pptxref"
mkdir -p "$work"

if [ "$#" -eq 0 ]; then
	set -- $(cd "$src" && ls *.pptx | sed 's/\.pptx$//')
fi

for name in "$@"; do
	run="$work/$name-$(date +%s)"
	mkdir -p "$run/png"
	cp "$src/$name.pptx" "$run/in.pptx"
	osascript "$repo/scripts/pptxref/export.applescript" "$run/in.pptx" "$run/out.pptx" "$run/deck.pdf"
	# slide size in CSS pixels (EMU / 9525)
	size=$(unzip -p "$run/out.pptx" ppt/presentation.xml | grep -o '<p:sldSz[^>]*>' | head -1)
	cx=$(echo "$size" | sed -E 's/.*cx="([0-9]+)".*/\1/')
	cy=$(echo "$size" | sed -E 's/.*cy="([0-9]+)".*/\1/')
	swift "$repo/scripts/pptxref/rasterize.swift" "$run/deck.pdf" "$run/png" $((cx / 9525)) $((cy / 9525))
	ref="$repo/fixtures/pptx/ref/$name"
	mkdir -p "$ref"
	rm -f "$ref"/*.png
	cp "$run/png"/*.png "$ref/"
	i=$(ls "$run/png" | wc -l | tr -d ' ')
	cp "$run/out.pptx" "$repo/fixtures/pptx/$name.pptx"
	shasum -a 256 "$repo/fixtures/pptx/$name.pptx" | cut -d' ' -f1 > "$ref/source.sha256"
	echo "reference written: $name ($i slides)"
done

# Quit PowerPoint only if no presentation is open.
open_docs=$(osascript -e 'tell application "Microsoft PowerPoint" to count presentations')
if [ "$open_docs" = "0" ]; then
	osascript -e 'tell application "Microsoft PowerPoint" to quit'
fi
