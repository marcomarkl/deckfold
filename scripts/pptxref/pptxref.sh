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
# are copied back. PowerPoint's sandbox shows "Grant Access" dialogs for files it has not opened
# before; confirm them during the run. Install the bundled fonts first (Carlito and Liberation, see THIRD_PARTY_LICENSES.md)
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
	# fixed file names in one folder: PowerPoint asks for file access ("Grant Access") for every
	# path it has not seen; granting the folder once covers every later run
	in="$work/$name-in.pptx" out="$work/$name-out.pptx" pdf="$work/$name.pdf" png="$work/$name-png"
	rm -rf "$png"
	mkdir -p "$png"
	cp "$src/$name.pptx" "$in"
	osascript "$repo/scripts/pptxref/export.applescript" "$in" "$out" "$pdf"
	# slide size in CSS pixels (EMU / 9525)
	size=$(unzip -p "$out" ppt/presentation.xml | grep -o '<p:sldSz[^>]*>' | head -1)
	cx=$(echo "$size" | sed -E 's/.*cx="([0-9]+)".*/\1/')
	cy=$(echo "$size" | sed -E 's/.*cy="([0-9]+)".*/\1/')
	swift "$repo/scripts/pptxref/rasterize.swift" "$pdf" "$png" $((cx / 9525)) $((cy / 9525))
	ref="$repo/fixtures/pptx/ref/$name"
	mkdir -p "$ref"
	rm -f "$ref"/*.png
	cp "$png"/*.png "$ref/"
	i=$(ls "$png" | wc -l | tr -d ' ')
	cp "$out" "$repo/fixtures/pptx/$name.pptx"
	shasum -a 256 "$repo/fixtures/pptx/$name.pptx" | cut -d' ' -f1 > "$ref/source.sha256"
	echo "reference written: $name ($i slides)"
done

# Quit PowerPoint only if no presentation is open.
open_docs=$(osascript -e 'tell application "Microsoft PowerPoint" to count presentations')
if [ "$open_docs" = "0" ]; then
	osascript -e 'tell application "Microsoft PowerPoint" to quit'
fi
