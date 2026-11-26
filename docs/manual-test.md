# Manual test with two screens

Some behaviour can only be checked with real hardware: a second screen (projector or monitor), real key presses and a presentation clicker. Use a converted deck with a few slides, ideally one with presenter notes. Mark each item and note what happened.

## Two screens (Chrome or Edge; repeat the marked rows in Safari)

| Check | Expected | Result |
|---|---|---|
| Open the file | The window shows the start screen: the title in the middle, preview, slide strip, settings, "Start", and below the settings a last row with the button for the keys, "Search" and the number of slides; nothing has to be scrolled in a window of 1280 x 800; no permission prompt | |
| Click the button with the keyboard icon; press the arrow keys and Enter; then Escape. Open it again and close it with the ×, then with a click beside the list | The list of keys opens over the start screen, with the version of deckfold below it. While it is open the arrows do not choose a slide and Enter does not start. Each of the three ways closes it | |
| Make the window lower, down to about 500 pixels | The preview shrinks first and keeps the shape of the slide; below that the start screen scrolls, and the last row can still be reached | |
| Choose slide 3, reload the window | Slide 3 is still chosen | |
| Click a slide in the strip, or use the arrows beside it | The preview shows it; it is the start slide | |
| Start, press Escape, choose another slide, start again; three times | Every start begins at the chosen slide | |
| Switch "Presenter view" on | A second window opens, without a permission prompt and without a hint about the screens; "Full screen" can always be chosen | |
| Drag the presenter view to the other screen | Nothing changes in either window | |
| Presenter view before the start | "Live" shows the chosen start slide at its first step, "Preview" what the first click brings, and the notes; the arrow keys there change the start slide on the start screen too | |
| Previews on a slide with builds, in a narrow presenter view | "Live" shows the step the audience sees, "Preview" the step after it, then at the last step the next slide; nothing plays, and no step appears on the way | |
| Start on a slide without builds; then a slide with builds: step 0, last step, B, W, end slide | "Live" shows the first slide at once; then step 0 with the builds hidden, the whole slide at the last step, black, white, "End of presentation" | |
| With the clicker, go through a PowerPoint slide with builds, a claude.ai slide with steps that run by themselves and a Keynote slide with builds | The dots of "Live" fill one per click; steps that run by themselves take no dot; at the last dot the arrow is marked, and the next click changes the slide | |
| Make the presenter view narrow (about 300 px) and wide again | Narrow: the panes one below the other, the bar one line that scrolls sideways; every control can be reached | |
| Click once into the presenter view on the second screen, then put the mouse on a slide tile and on a button and keep it still | The highlight stays while the mouse rests. Chrome on macOS shows it only in the active window: after a click in the presentation window it is gone, also under the mouse, until the presenter view is clicked again | |
| Note where the presenter view is, then click "Start" (with and without "Full screen") | The presenter view keeps its place and size | |
| Close the presentation window | The presenter view closes too; reloading the presentation window does not close it | |
| End the show with the presenter view open | On the start screen the switch of the presenter view is on at once, not a second later | |
| Click "Start" with "Full screen" on | The presentation fills its screen at the chosen slide; the presenter view follows | |
| Look for a full-screen button in the presenter view; press F there | There is none, and F changes neither window | |
| Look at the presenter view on the laptop while the projector shows the slides | The icons for search and end show are clear without their names; "Live" and "Preview" read as meant | |
| Audio on a slide with sound | Audio plays in the presentation without a click | |
| With ffmpeg installed: convert a deck with a video that browsers do not play (HEVC from an iPhone, WMV) and a WAV sound; play both in Chrome, Safari and Firefox | The report names the converted video and sound. The video plays with its sound, in the right colours and at the right speed; the sound plays | |
| Hide notes, press A+ twice, drag the dividers between the panes and the one above the strip of slides | While the notes are hidden, A− and A+ are grey; the presenter view changes at once, the tiles of the strip grow with it; after reloading it, the changes are still there | |
| On the start screen, with the presenter view on the second screen, click the eye of a slide, then start and click through | The tile is greyed out with a crossed eye, the presenter view's strip leaves the slide out at once and its count reads "n of m slides"; the clicker goes past the slide, also on the projector, and the eye is easy to hit on the laptop | |
| Time: before Start, after Start, pause, resume, reset (button and T) | 00:00 before the start; then it runs, stands still while paused, and starts again from 00:00 | |
| Type 1:30 as the target, Enter | The field shows 01:30; the time left counts down, turns yellow under two minutes and red after it | |
| Click a slide in the strip of the presenter view during the show; click its arrows | The click jumps there; the arrows only scroll the strip | |
| Reload the presenter view during the show | It comes back at the current slide and keeps controlling the presentation | |
| Restart the browser, open the file again | The start screen appears; the settings of the presenter view are kept | |
| The magnifier on the start screen, or `/`; type a word from a slide; Enter | The outline lists every slide; typing leaves the slides with the word; Enter chooses the slide as the start slide. A word that is only in the notes finds nothing here | |
| Open the search on the start screen and in the presenter view; click the × beside the text field | The search closes, and the keys move the slides again | |
| During the show: the magnifier in the presenter view, type a word from the notes, Enter | The slide is found with "Notes" next to the place; Enter shows the slide to the audience at its first step; the presentation window never shows the search | |
| During the show: open the search in the presenter view, press the arrow keys and Escape | The arrows move in the list and the audience stays on its slide; Escape closes the search and the presentation goes on | |
| PowerPoint deck, and Keynote export, with a link to another slide: click the link in the presentation, in a window and in full screen | Both windows show the linked slide | |

## Clicker and keys

| Check | Expected | Result |
|---|---|---|
| Page Down / Page Up on the clicker, with the focus in either window | Next and previous step; both windows follow | |
| B or the clicker's blank key | Black screen in the presentation; pressed again, the slide comes back | |
| One Escape, with the presentation in full screen, in the presentation or in the presenter view | The presentation ends: full screen is left, the start screen appears with the last slide chosen, the presenter view stays open and shows the start state | |
| Escape in the presenter view before the start | Nothing happens; the start screen does not change | |
| Back on the first slide | Nothing happens; no end slide | |
| Click on the slide; turn the mouse wheel; swipe on the trackpad | The click moves on, in both windows; wheel and swipe do nothing | |
| G during the presentation; the arrow keys; Enter; G or Escape | A strip of all slides appears at the bottom with the button that ends the show; the arrows move its mark, not the slides; Enter goes to the marked slide; G or Escape closes it, and Escape does not end the show then | |
| On a tablet or touch display (iPad with Safari, Android with Chrome): swipe left and right on the slide, then tap | The swipe to the left moves on, the one to the right goes back, the tap moves on once; the page neither scrolls nor zooms nor goes back in the history | |
| On a tablet or touch display: swipe up on the slide, tap a slide in the strip; swipe up again and tap the button at its right | The strip appears; the tap goes to that slide and closes the strip; the button ends the show | |
| On a tablet or touch display: pull down on the slide; swipe in from the left and from the bottom edge | The page does not reload; what the system does at the edges is noted here | iPad, Safari, 2026-11-25: in full screen every swipe down leaves full screen, wherever it begins. The edges are not noted yet |
| On a tablet: the start screen | Neither "Presenter view" nor "Full screen" is offered; "Start" begins in full screen | |
| On a tablet, during the show: leave full screen (on an iPad with a swipe down), then tap the slide; leave it again and swipe left | Nothing covers the slide after full screen is left; the tap and the swipe move on and bring full screen back | |
| On a tablet: "Full screen" on the start screen, then "Start"; turn the tablet | Full screen without the address bar; the slide fits in portrait and in landscape | iPad, Safari, 2026-11-25: full screen hides the address bar; portrait and landscape work (a claude.ai deck) |
| On an iPad: open the file from the Files app | Noted here: the Files app shows only a preview, in which the deck does not run; serve the file from a web server instead | iPad, 2026-11-25: only the preview is offered |
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
| One Escape in full screen | Safari leaves full screen and the presentation ends: the start screen appears with the last slide chosen | |
| F in full screen, then an arrow key, then F | Full screen is left and the presentation goes on in the window, with nothing over the slide; the arrow moves on; F brings full screen back | |
| G in full screen, then Escape | Safari leaves full screen and the presentation ends (in Chrome, Escape only closes the strip) | |
| With the presenter view open and the show in full screen: a click and a key in the presenter view | The presentation stays in full screen and goes on | |
| A video or an embed in its own full screen, then its button for leaving it or Escape | The presentation goes on, in full screen or in the window | |
| A click on a link of a slide in full screen (it opens a new tab), then back to the tab of the show | Not measured yet: note whether the show is still on its slide or back on the start screen | |

## Keynote

| Check | Expected | Result |
|---|---|---|
| Presenter view with a Keynote export that has builds and transitions | Both previews show their slide after all its builds without playing them; the tests cannot check this, because the Keynote player draws its effects itself | |
