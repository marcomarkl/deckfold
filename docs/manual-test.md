# Manual test with two screens

Some behaviour can only be checked with real hardware: a second screen (projector or monitor), real key presses and a presentation clicker. Use a converted deck with a few slides, ideally one with presenter notes. Mark each item and note what happened.

## Two screens (Chrome or Edge; repeat the marked rows in Safari)

| Check | Expected | Result |
|---|---|---|
| Open the file | The window shows the start screen: title, preview, slide strip, settings, "Start"; no permission prompt | |
| Choose slide 3, reload the window | Slide 3 is still chosen | |
| Click a slide in the strip, or use the arrows beside it | The preview shows it; it is the start slide | |
| Start, press Escape, choose another slide, start again; three times | Every start begins at the chosen slide | |
| Switch "Presenter view" on | A second window opens, without a permission prompt and without a hint about the screens; "Full screen" can always be chosen | |
| Drag the presenter view to the other screen | Nothing changes in either window | |
| Presenter view before the start | It shows the chosen start slide, the slide after it and the notes; the arrow keys there change the start slide on the start screen too | |
| Previews on a slide with builds, in a narrow presenter view | "Current" shows only this slide, "Next" only the next one, each with all builds shown and nothing playing; no step appears on the way | |
| Audience pane on a slide with builds: step 0, last step, B, W, end slide | Step 0 with the builds hidden; empty at the last step; black, white, "End of presentation" | |
| Make the presenter view narrow (about 300 px) and wide again | Narrow: the panes one below the other, the bar one line that scrolls sideways; every control can be reached | |
| Mouse over a slide tile and over a button of the presenter view, on the second screen, then keep the mouse still | The highlight stays while the mouse rests | |
| Note where the presenter view is, then click "Start" (with and without "Full screen") | The presenter view keeps its place and size | |
| Close the presentation window | The presenter view closes too; reloading the presentation window does not close it | |
| Click "Start" with "Full screen" on | The presentation fills its screen at the chosen slide; the presenter view follows | |
| Click "Full screen" in the presenter view | The presenter view fills its screen; the button now reads "Window" and takes it back | |
| Audio on a slide with sound | Audio plays in the presentation without a click | |
| Hide notes, press A+ twice, drag the divider between the panes | The presenter view changes at once; after reloading it, the changes are still there | |
| Time: before Start, after Start, pause, resume, reset (button and T) | 00:00 before the start; then it runs, stands still while paused, and starts again from 00:00 | |
| Type 1:30 as the target, Enter | The field shows 01:30; the time left counts down, turns yellow under two minutes and red after it | |
| Click a slide in the strip of the presenter view during the show; click its arrows | The click jumps there; the arrows only scroll the strip | |
| Switch "Overview" off in the status bar, reload the presenter view | The strip stays hidden | |
| Reload the presenter view during the show | It comes back at the current slide and keeps controlling the presentation | |
| Restart the browser, open the file again | The start screen appears; the settings of the presenter view are kept | |

## Clicker and keys

| Check | Expected | Result |
|---|---|---|
| Page Down / Page Up on the clicker, with the focus in either window | Next and previous step; both windows follow | |
| B or the clicker's blank key | Black screen in the presentation; pressed again, the slide comes back | |
| One Escape in full screen, in the presentation or in the presenter view | The presentation ends: full screen is left, the start screen appears with the last slide chosen, the presenter view stays open and shows the start state | |
| Back on the first slide | Nothing happens; no end slide | |
| Click on the slide; turn the mouse wheel; swipe on the trackpad | The click moves on, in both windows; wheel and swipe do nothing | |
| Enter during the presentation | Nothing happens | |
| One step after the last slide | The black end slide appears and stays on further steps (key, click, presenter view); Escape returns to the start screen | |

## Auto mode

| Check | Expected | Result |
|---|---|---|
| Switch auto mode on the start screen on, start, then cover or minimize the presenter view for a few minutes | The presentation keeps advancing at the set interval | |

## One screen

| Check | Expected | Result |
|---|---|---|
| Switch "Presenter view" on with only the laptop screen, start with "Full screen" | The presentation fills the screen and covers the presenter view; no hint | |

## Safari

| Check | Expected | Result |
|---|---|---|
| Switch "Presenter view" on | No permission prompt, no hint about the screens; "Full screen" can always be chosen | |
| Previews on a slide with builds | Each preview shows its slide with all builds, without playing them | |
| Start with "Full screen" | The presentation fills its screen | |
| Escape in full screen | Safari leaves full screen; a second Escape ends the presentation | |

## Keynote

| Check | Expected | Result |
|---|---|---|
| Presenter view with a Keynote export that has builds and transitions | Both previews show their slide after all its builds without playing them; the tests cannot check this, because the Keynote player draws its effects itself | |
