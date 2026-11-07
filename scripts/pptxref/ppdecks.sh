#!/bin/sh
# ppdecks: let PowerPoint build the animations and transitions decks in fixtures/pptx/powerpoint
# itself (macOS, Microsoft PowerPoint). The base decks come from scripts/pptxgen -ppbase; the
# work folder is tests/.out/ppdecks. PowerPoint shows "Grant Access" dialogs for files it has not
# opened before; confirm them. PowerPoint is quit at the end when no presentation is open.
set -eu
repo=$(cd "$(dirname "$0")/../.." && pwd)
work="$repo/tests/.out/ppdecks"
mkdir -p "$work" "$repo/fixtures/pptx/powerpoint"
(cd "$repo" && go run ./scripts/pptxgen -ppbase "$work")
rm -f "$work/animations-out.pptx" "$work/transitions-out.pptx"
osascript "$repo/scripts/pptxref/ppdecks.applescript" "$work/animations-base.pptx" "$work/animations-out.pptx" "$work/transitions-base.pptx" "$work/transitions-out.pptx"
for n in animations transitions; do
	if [ ! -s "$work/$n-out.pptx" ]; then
		echo "ppdecks: PowerPoint did not save $n" >&2
		exit 1
	fi
	cp "$work/$n-out.pptx" "$repo/fixtures/pptx/powerpoint/$n.pptx"
	echo "written: fixtures/pptx/powerpoint/$n.pptx"
done
if [ "$(osascript -e 'tell application "Microsoft PowerPoint" to count presentations')" = "0" ]; then
	osascript -e 'tell application "Microsoft PowerPoint" to quit'
fi
