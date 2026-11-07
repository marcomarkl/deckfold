# Manual test with two screens

Some behaviour can only be checked with real hardware: a second screen (projector or monitor) and a presentation clicker. Use a converted deck with a few slides, ideally one with presenter notes. Mark each item and note what happened.

## Chrome or Edge with two screens

| Check | Expected | Result |
|---|---|---|
| Click "Present" for the first time | The browser asks for permission to manage windows on all screens | |
| Screen dialog | Both screens are listed; the audience window is preselected on the screen that is not the current one | |
| Click "Start" with "Presenter view in full screen" on | One click puts the presenter view into full screen on its screen and opens the audience window on the other screen | |
| Audience window | It is in full screen without another click; if not, it shows "Click for full screen" and one click there is enough | |
| Audio on a slide with sound | Audio plays in the audience window without a click | |
| Restart the browser, open the file again, click "Present" | No new permission prompt; the screen choice is remembered | |

## Clicker and keys

| Check | Expected | Result |
|---|---|---|
| Page Down / Page Up on the clicker | Next and previous step in both windows | |
| B or the clicker's blank key | Black screen in the audience window; pressed again, the slide comes back | |
| Escape in full screen | The browser leaves full screen; the presentation keeps running | |
| Escape outside full screen | The presentation ends and the audience window closes | |

## Auto mode

| Check | Expected | Result |
|---|---|---|
| Start auto mode (A), then cover or minimize the presenter view for a few minutes | The audience window keeps advancing at the set interval | |

## Screens changing

| Check | Expected | Result |
|---|---|---|
| Unplug the second screen during the talk, then plug it back in | The presenter view reports the change and offers to choose the screens again | |

## Safari

| Check | Expected | Result |
|---|---|---|
| Click "Present" | The audience window opens; the presenter view explains how to move it | |
| Drag the audience window to the second screen, click "Full screen" | The audience window fills the second screen | |
