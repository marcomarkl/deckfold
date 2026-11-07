-- ppdecks.applescript <animations-base> <animations-out> <transitions-base> <transitions-out>
-- PowerPoint adds the animations and transitions to the base decks itself and saves them, so
-- that the tests also read the timing and transition XML as PowerPoint writes it for effects it
-- created. AppleScript can set neither trigger shapes nor delays.
on run argv
	tell application "Microsoft PowerPoint"
		with timeout of 300 seconds
			open (POSIX file (item 1 of argv))
			set p to active presentation
			set s1 to slide 1 of p
			set ms to main sequence of timeline of s1
			add effect ms for shape "Fade" of s1 fx animation type fade trigger on page click
			add effect ms for shape "Fly" of s1 fx animation type fly trigger on page click
			set e to add effect ms for shape "Exit" of s1 fx animation type fade trigger on page click
			set exit animation of e to true
			set s2 to slide 2 of p
			set ms to main sequence of timeline of s2
			add effect ms for shape "Grow" of s2 fx animation type grow shrink trigger on page click
			add effect ms for shape "Move" of s2 fx animation type right path trigger on page click
			set s3 to slide 3 of p
			set ms to main sequence of timeline of s3
			add effect ms for shape 2 of s3 fx animation type fade level text by first level trigger on page click
			set s4 to slide 4 of p
			set ms to main sequence of timeline of s4
			add effect ms for shape "Click" of s4 fx animation type fade trigger on page click
			add effect ms for shape "With" of s4 fx animation type fade trigger with previous
			add effect ms for shape "After" of s4 fx animation type fade trigger after previous
			save p in (POSIX file (item 2 of argv)) as save as Open XML presentation
			close p saving no

			open (POSIX file (item 3 of argv))
			set p to active presentation
			set fx to {entry effect fade, entry effect push left, entry effect wipe right, entry effect reveal smooth left, entry effect cover left, entry effect cut, entry effect morph by object, entry effect fade}
			repeat with i from 1 to count of fx
				set entry effect of slide show transition of slide (i + 1) of p to item i of fx
			end repeat
			set advance on time of slide show transition of slide 9 of p to true
			set advance time of slide show transition of slide 9 of p to 3
			save p in (POSIX file (item 4 of argv)) as save as Open XML presentation
			close p saving no
		end timeout
	end tell
end run
