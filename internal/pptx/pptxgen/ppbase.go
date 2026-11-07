package pptxgen

// Base decks for the decks that PowerPoint builds itself (scripts/pptxref/ppdecks.sh): shapes
// with names and no timing; PowerPoint adds the animations and transitions through AppleScript
// and saves the result in fixtures/pptx/powerpoint.

// PPAnimationsBase has the shapes for PowerPoint's own animations: entrance fade, fly in and an
// exit fade (slide 1), grow and a motion path (slide 2), a paragraph build (slide 3), and a
// click effect followed by effects with and after the previous one (slide 4).
func PPAnimationsBase() *Deck {
	d := New("Animations built by PowerPoint")
	box := func(s *Slide, name string, x float64, color string) {
		s.Shape(name, "rect", Box{X: x, Y: 300, W: 240, H: 160}, solid(color), "", Para{Text: name, RPr: `sz="2400"`})
	}
	s := d.Add(LayoutBlank)
	s.TextBox("Title", Box{X: 60, Y: 40, W: 1000, H: 60}, "", Para{Text: "Entrance and exit", RPr: `sz="3200"`})
	box(s, "Fade", 100, "4472C4")
	box(s, "Fly", 500, "ED7D31")
	box(s, "Exit", 900, "A5A5A5")
	s = d.Add(LayoutBlank)
	s.TextBox("Title", Box{X: 60, Y: 40, W: 1000, H: 60}, "", Para{Text: "Emphasis and motion", RPr: `sz="3200"`})
	box(s, "Grow", 200, "FFC000")
	box(s, "Move", 600, "5B9BD5")
	s = d.Add(LayoutContent)
	s.Placeholder("title", Para{Text: "Paragraphs"})
	s.Placeholder("body", Para{Text: "First paragraph"}, Para{Text: "Second paragraph"}, Para{Text: "Third paragraph"})
	s = d.Add(LayoutBlank)
	s.TextBox("Title", Box{X: 60, Y: 40, W: 1000, H: 60}, "", Para{Text: "With and after previous", RPr: `sz="3200"`})
	box(s, "Click", 100, "4472C4")
	box(s, "With", 500, "70AD47")
	box(s, "After", 900, "ED7D31")
	return d
}

// PPTransitionsBase has one slide per transition that PowerPoint sets (fade, push, wipe,
// reveal, cover, cut, morph and an automatic advance after 3 seconds).
func PPTransitionsBase() *Deck {
	d := New("Transitions set by PowerPoint")
	names := []string{"Start", "Fade", "Push", "Wipe", "Reveal", "Cover", "Cut", "Morph", "Automatic advance"}
	colors := []string{"4472C4", "ED7D31", "70AD47", "FFC000", "5B9BD5", "A5A5A5"}
	for i, n := range names {
		s := d.Add(LayoutBlank)
		s.Shape("Panel", "rect", Box{X: 0, Y: 0, W: 1280, H: 720}, solid(colors[i%len(colors)]), "")
		s.TextBox("Title", Box{X: 80, Y: 300, W: 1120, H: 120}, "", Para{Text: n, RPr: `sz="6000" b="1"`})
	}
	return d
}
