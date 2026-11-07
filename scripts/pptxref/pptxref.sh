#!/bin/sh
# pptxref: make the PowerPoint reference for the feature decks (macOS, Microsoft PowerPoint).
#
# For every deck in fixtures/pptx/src it
#   1. opens the deck in PowerPoint,
#   2. saves it again as fixtures/pptx/<name>.pptx (the copy the tests read),
#   3. exports one PNG per slide into fixtures/pptx/ref/<name>/,
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
	osascript "$repo/scripts/pptxref/export.applescript" "$run/in.pptx" "$run/out.pptx" "$run/png"
	ref="$repo/fixtures/pptx/ref/$name"
	mkdir -p "$ref"
	rm -f "$ref"/*.png
	i=1
	# PowerPoint names the slides Slide1.png, Slide2.png, ... (localized on some systems).
	for f in $(ls "$run/png" | sort -V); do
		cp "$run/png/$f" "$ref/$i.png"
		i=$((i + 1))
	done
	cp "$run/out.pptx" "$repo/fixtures/pptx/$name.pptx"
	shasum -a 256 "$repo/fixtures/pptx/$name.pptx" | cut -d' ' -f1 > "$ref/source.sha256"
	echo "reference written: $name ($((i - 1)) slides)"
done

# Quit PowerPoint only if no presentation is open.
open_docs=$(osascript -e 'tell application "Microsoft PowerPoint" to count presentations')
if [ "$open_docs" = "0" ]; then
	osascript -e 'tell application "Microsoft PowerPoint" to quit'
fi
