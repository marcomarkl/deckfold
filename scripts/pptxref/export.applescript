-- export.applescript <input.pptx> <output.pptx> <output.pdf>
-- Opens a deck in PowerPoint, saves a copy and exports it as PDF (one page per shown slide).
-- PowerPoint 16.114 reports success for "save as PNG" but writes no files, so the PNGs are made
-- from the PDF by rasterize.swift.
on run argv
	set inPath to item 1 of argv
	set outPath to item 2 of argv
	set pdfPath to item 3 of argv
	tell application "Microsoft PowerPoint"
		with timeout of 300 seconds
			open (POSIX file inPath)
			set p to active presentation
			save p in (POSIX file outPath) as save as Open XML presentation
			save p in (POSIX file pdfPath) as save as PDF
			close p saving no
		end timeout
	end tell
end run
