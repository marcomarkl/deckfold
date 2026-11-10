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
# It waits until the system lists the fonts before it starts PowerPoint, and it checks the fonts
# in every exported PDF: a deck drawn with a replaced font gets no reference (on 2026-11-10 a
# run wrote reference images in Calibri without any sign of it).
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

# PowerPoint has to find the fonts when it starts: wait until the system lists them
if [ -s "$installed" ]; then
	n=0
	until swift -e 'import AppKit; let have = Set(NSFontManager.shared.availableFontFamilies); exit(["Carlito", "Liberation Sans", "Liberation Serif", "Liberation Mono"].allSatisfy { have.contains($0) } ? 0 : 1)' 2>/dev/null; do
		n=$((n + 1))
		if [ "$n" -ge 15 ]; then
			echo "pptxref: the bundled fonts did not show up in the system's font list; no reference written" >&2
			exit 1
		fi
		sleep 2
	done
fi

# drawn_with_other_fonts: PowerPoint replaces a font it cannot find without saying so (Carlito by
# Calibri, for example), and the reference would then show another font than deckfold's
# rendering. The PDF names the fonts it was drawn with (/FontName /ABCDEF+Carlito-Regular). When
# the deck names a bundled font that the PDF does not have, while the PDF has a font the deck
# does not name, the bundled font was replaced: prints the fonts that took its place.
drawn_with_other_fonts() {
	deck_fonts=$(unzip -p "$1" 'ppt/presentation.xml' 'ppt/theme/*' 'ppt/slideMasters/*' 'ppt/slideLayouts/*' 'ppt/slides/*' 2>/dev/null |
		grep -o 'typeface="[^"]*"' | sed -e 's/^typeface="//' -e 's/"$//' -e 's/ //g' | sort -u)
	pdf_fonts=$(LC_ALL=C strings -a "$2" | grep -o '/[A-Z]\{6\}+[A-Za-z][A-Za-z0-9]*' | sed -e 's/^.*+//' -e 's/PSMT$//' -e 's/MT$//' | sort -u)
	for b in Carlito LiberationSans LiberationSerif LiberationMono; do
		if echo "$deck_fonts" | grep -qx "$b" && ! echo "$pdf_fonts" | grep -qx "$b"; then
			for p in $pdf_fonts; do
				echo "$deck_fonts" | grep -qix "$p" || echo "$p"
			done
		fi
	done | sort -u | tr '\n' ' '
}

if [ "$#" -eq 0 ]; then
	set -- $(cd "$src" && ls *.pptx | sed 's/\.pptx$//')
fi

failed=""
for name in "$@"; do
	# fixed file names per deck, so that PowerPoint sees the same paths on every run
	in="$work/$name-in.pptx" out="$work/$name-out.pptx" pdf="$work/$name.pdf" png="$work/$name-png"
	rm -rf "$png"
	mkdir -p "$png"
	cp "$src/$name.pptx" "$in"
	rm -f "$out" "$pdf"
	# a deck PowerPoint reports as damaged (repair prompt) is not saved; go on with the next one
	if ! osascript "$repo/scripts/pptxref/export.applescript" "$in" "$out" "$pdf" || [ ! -s "$out" ] || [ ! -s "$pdf" ]; then
		echo "pptxref: PowerPoint did not save $name (was it reported as damaged?); no reference written" >&2
		failed="$failed $name"
		continue
	fi
	other=$(drawn_with_other_fonts "$in" "$pdf")
	if [ -n "$other" ]; then
		echo "pptxref: PowerPoint drew $name with ${other}instead of the bundled fonts the deck names; no reference written" >&2
		failed="$failed $name"
		continue
	fi
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

if [ -n "$failed" ]; then
	echo "pptxref: no reference for:$failed" >&2
	exit 1
fi
# PowerPoint is quit and the fonts are removed by cleanup (trap above).
