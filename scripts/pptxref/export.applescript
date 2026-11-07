-- export.applescript <input.pptx> <output.pptx> <png folder>
-- Opens a deck in PowerPoint, saves a copy and exports one PNG per slide.
on run argv
	set inPath to item 1 of argv
	set outPath to item 2 of argv
	set pngDir to item 3 of argv
	tell application "Microsoft PowerPoint"
		with timeout of 300 seconds
			open (POSIX file inPath)
			set p to active presentation
			save p in (POSIX file outPath) as save as Open XML presentation
			save p in (POSIX file pngDir) as save as PNG
			close p saving no
		end timeout
	end tell
end run
