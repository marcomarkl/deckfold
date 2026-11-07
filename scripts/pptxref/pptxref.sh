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
# All work happens in tests/.out/pptxref inside the repository (ignored by git). PowerPoint's
# sandbox shows "Grant Access" dialogs for files it has not opened before; confirm them during
# the run.
#
# PowerPoint has to see the bundled fonts (Carlito, Liberation), otherwise it substitutes
# Calibri and Arial. The script copies the ones that are missing into ~/Library/Fonts for the
# run and removes exactly those again at the end, also after an error. PowerPoint reads fonts
# when it starts, so the script quits PowerPoint first (and stops if a presentation is open).
#
# Usage: scripts/pptxref/pptxref.sh [name ...]
set -eu

repo=$(cd "$(dirname "$0")/../.." && pwd)
src="$repo/fixtures/pptx/src"
work="$repo/tests/.out/pptxref"
mkdir -p "$work"

pp_running() { pgrep -qf "Microsoft PowerPoint.app/Contents/MacOS/Microsoft PowerPoint"; }
pp_quit() {
	if pp_running; then
		if [ "$(osascript -e 'tell application "Microsoft PowerPoint" to count presentations')" != "0" ]; then
			echo "pptxref: close the open presentations in PowerPoint first" >&2
			return 1
		fi
		osascript -e 'tell application "Microsoft PowerPoint" to quit'
		while pp_running; do sleep 1; done
	fi
}

# fonts for the run: install the missing bundled fonts, remove them again on exit
fonts="$HOME/Library/Fonts"
installed="$work/installed-fonts.txt"
: > "$installed"
cleanup() {
	pp_quit || true
	if [ -f "$installed" ]; then
		while IFS= read -r f; do rm -f "$fonts/$f"; done < "$installed"
		rm -f "$installed"
	fi
}
trap cleanup EXIT
trap 'exit 130' INT TERM
pp_quit || exit 1
mkdir -p "$fonts"
for gz in "$repo"/internal/pptx/fonts/data/*.ttf.gz; do
	f=$(basename "$gz" .gz)
	if [ ! -e "$fonts/$f" ]; then
		gunzip -c "$gz" > "$fonts/$f"
		echo "$f" >> "$installed"
	fi
done

if [ "$#" -eq 0 ]; then
	set -- $(cd "$src" && ls *.pptx | sed 's/\.pptx$//')
fi

for name in "$@"; do
	# fixed file names per deck, so that PowerPoint sees the same paths on every run
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

# PowerPoint is quit and the fonts are removed by cleanup (trap above).
