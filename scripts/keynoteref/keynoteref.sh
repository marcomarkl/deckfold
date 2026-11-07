#!/bin/sh
# keynoteref: export generated feature decks with Keynote as HTML, for the local Keynote browser
# tests (macOS, Keynote). The exports contain Apple's Keynote player, so they stay out of the
# repository: they go to tests/.out/keynote/<name>/, where tests/e2e/setup.mjs finds them when
# neither fixtures/keynote nor testdata/keynote has an export.
#
# Keynote is driven through AppleScript. A Keynote document is never closed through AppleScript
# (Keynote crashes); at the end Keynote is quit, but only when every open document is one this
# script opened.
#
# Keynote 15.4 on macOS 27.2 does not open files in tests/.out through AppleScript: open returns
# nothing and no access dialog appears (checked on 2026-11-07). Then export by hand: open
# fixtures/pptx/<name>.pptx in Keynote, File > Export To > HTML, into tests/.out/keynote/<name>.
#
# Usage: scripts/keynoteref/keynoteref.sh [name ...]   (default: basic animations)
set -eu

repo=$(cd "$(dirname "$0")/../.." && pwd)
out="$repo/tests/.out/keynote"
mkdir -p "$out"
app=$(ls -d /Applications/Keynote*.app 2>/dev/null | head -1)
if [ -z "$app" ]; then
	echo "keynoteref: Keynote is not installed" >&2
	exit 1
fi
name_of_app=$(basename "$app" .app)
[ "$#" -eq 0 ] && set -- basic animations

opened=""
for name in "$@"; do
	src="$repo/fixtures/pptx/$name.pptx"
	[ -f "$src" ] || src="$repo/fixtures/pptx/src/$name.pptx"
	in="$out/$name.pptx"
	cp "$src" "$in"
	rm -rf "$out/$name"
	# the application name has to be literal, so that AppleScript knows Keynote's terms ("HTML")
	osascript - "$in" "$out/$name" <<APPLESCRIPT || { echo "keynoteref: Keynote could not open $name; export it by hand (see the comment at the top)" >&2; exit 1; }
on run argv
	set inPath to item 1 of argv
	set outPath to item 2 of argv
	tell application "$name_of_app"
		with timeout of 300 seconds
			set d to open (POSIX file inPath)
			if d is missing value then error "Keynote did not open the file"
			export d to (POSIX file outPath) as HTML
		end timeout
	end tell
end run
APPLESCRIPT
	if [ ! -f "$out/$name/index.html" ]; then
		echo "keynoteref: Keynote wrote no HTML export for $name" >&2
		exit 1
	fi
	opened="$opened $name $name.pptx"
	echo "exported: $name -> tests/.out/keynote/$name"
done

# quit Keynote, but only when the open documents are the ones opened here
docs=$(osascript -e "tell application \"$name_of_app\" to get name of every document" 2>/dev/null || true)
others=""
for d in $(echo "$docs" | tr ',' '\n' | sed 's/^ *//'); do
	case " $opened " in *" $d "*) ;; *) others="$others $d" ;; esac
done
if [ -z "$others" ]; then
	osascript -e "tell application \"$name_of_app\" to quit saving no"
else
	echo "keynoteref: other documents are open in Keynote ($others); Keynote stays open" >&2
fi
