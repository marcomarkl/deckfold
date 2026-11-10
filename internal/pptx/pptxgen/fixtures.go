package pptxgen

import (
	"bytes"
	"fmt"

	"deckfold/internal/pptx/fonts"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"strings"
)

// Fixtures returns the public feature decks by name.
func Fixtures() map[string]*Deck {
	return map[string]*Deck{
		"basic":         Basic(),
		"shapes":        Shapes(),
		"tables":        Tables(),
		"hostile":       Hostile(),
		"media":         Media(),
		"text":          Text(),
		"animations":    Animations(),
		"charts":        Charts(),
		"gifs":          GIFs(),
		"triggers":      Triggers(),
		"shapes3d":      Shapes3D(),
		"bullets":       Bullets(),
		"effects":       Effects(),
		"fontembed":     FontEmbed(),
		"text2":         Text2(),
		"morph":         Morph(),
		"transitions3d": Transitions3D(),
		"wmf":           WMF(),
		"charts2":       Charts2(),
		"ink":           InkDeck(),
		"content":       Content(),
	}
}

// Basic is the deck for the first end-to-end slice: placeholders that inherit from the layout,
// body text with bullets, a picture, notes and a hidden slide.
func Basic() *Deck {
	d := New("Basic feature deck")

	s := d.Add(LayoutTitle)
	s.Placeholder("ctrTitle", Para{Text: "Basic feature deck"})
	s.Placeholder("subTitle", Para{Text: "Placeholders, text, a picture and notes"})
	s.Notes = "Notes of the first slide.\nA second paragraph of notes."

	s = d.Add(LayoutContent)
	s.Placeholder("title", Para{Text: "Bullets and a picture"})
	s.Placeholder("body",
		Para{Text: "The first bullet sits on level one"},
		Para{Text: "A second bullet with a longer text that has to wrap onto a second line inside the placeholder"},
		Para{Text: "A bullet on level two", Level: 1},
		Para{Text: "Back on level one"})
	s.Notes = "Notes of the second slide."

	s = d.Add(LayoutContent)
	s.Hidden = true
	s.Placeholder("title", Para{Text: "This slide is hidden"})

	s = d.Add(LayoutBlank)
	s.Picture(d, "Gradient picture", Box{X: 720, Y: 160, W: 400, H: 300}, "png", "image/png", TestPNG(400, 300), "", "")
	s.TextBox("Caption", Box{X: 120, Y: 160, W: 520, H: 60}, "",
		Para{Text: "A text box next to a picture", RPr: `sz="3200" b="1"`})
	s.TextBox("Plain text", Box{X: 120, Y: 260, W: 520, H: 200}, `<a:bodyPr wrap="square" rtlCol="0"/>`,
		Para{Text: "Plain body text in a text box. It wraps at the right edge of the box and keeps the default size of 18 points."})
	s.Notes = "Notes of the last slide."
	return d
}

// Big returns a deck with n similar slides for the performance test.
func Big(n int) *Deck {
	d := New("Big deck")
	img := TestPNG(640, 360)
	for i := 0; i < n; i++ {
		s := d.Add(LayoutContent)
		s.Placeholder("title", Para{Text: fmt.Sprintf("Slide %d", i+1)})
		s.Placeholder("body", Para{Text: "First point"}, Para{Text: "Second point"}, Para{Text: "Third point", Level: 1})
		s.Picture(d, "Picture", Box{X: 820, Y: 300, W: 320, H: 180}, "png", "image/png", img, "", "")
		s.Notes = fmt.Sprintf("Notes for slide %d.", i+1)
	}
	return d
}

// TestPNG returns a deterministic picture: a diagonal gradient with a dark frame and a circle.
func TestPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	cx, cy, r := w/2, h/2, min(w, h)/4
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{uint8(40 + 180*x/w), uint8(90 + 120*y/h), 200, 255}
			if x < 6 || y < 6 || x >= w-6 || y >= h-6 {
				c = color.RGBA{20, 30, 60, 255}
			}
			if dx, dy := x-cx, y-cy; dx*dx+dy*dy <= r*r {
				c = color.RGBA{250, 200, 40, 255}
			}
			img.Set(x, y, c)
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}

// TestGIF returns an animated GIF of w x h pixels that shows red, green and blue for half a
// second each and loops.
func TestGIF(w, h int) []byte {
	pal := color.Palette{color.RGBA{220, 40, 40, 255}, color.RGBA{40, 180, 60, 255}, color.RGBA{40, 80, 220, 255}}
	g := &gif.GIF{}
	for i := range pal {
		im := image.NewPaletted(image.Rect(0, 0, w, h), pal)
		for p := range im.Pix {
			im.Pix[p] = uint8(i)
		}
		g.Image = append(g.Image, im)
		g.Delay = append(g.Delay, 50)
	}
	var b bytes.Buffer
	_ = gif.EncodeAll(&b, g)
	return b.Bytes()
}

// GIFs has an animated GIF, which starts again from its first frame whenever its slide is
// shown, and a second slide to leave it for.
func GIFs() *Deck {
	d := New("Animated GIF feature deck")
	s := d.Add(LayoutBlank)
	s.TextBox("Caption", Box{X: 120, Y: 120, W: 1040, H: 80}, "",
		Para{Text: "An animated GIF: red, green, blue", RPr: `sz="3200" b="1"`})
	s.Picture(d, "Animated GIF", Box{X: 490, Y: 260, W: 300, H: 300}, "gif", "image/gif", TestGIF(60, 60), "", "")
	s = d.Add(LayoutContent)
	s.Placeholder("title", Para{Text: "A slide without a GIF"})
	s.Placeholder("body", Para{Text: "Going back shows the GIF from its first frame again"})
	return d
}

// Triggers has an interactive sequence: each click on a button shape plays the next click group
// (show a star, hide it again), and an audio clip that plays and pauses when its own icon is
// clicked, which the player handles as a medium.
func Triggers() *Deck {
	d := New("Trigger feature deck")
	s := d.Add(LayoutBlank)
	s.TextBox("Title", Box{X: 60, Y: 40, W: 1100, H: 60}, "", Para{Text: "Click the button to show or hide the star", RPr: `sz="3200"`})
	btn := s.Shape("Button", "roundRect", Box{X: 120, Y: 300, W: 300, H: 120}, solid("4472C4"), "", Para{Text: "Button", RPr: `sz="2800"`})
	star := s.Shape("Star", "star5", Box{X: 700, Y: 220, W: 300, H: 300}, solid("FFC000"), line(12700, "7F6000", ""))
	tl := &Timeline{id: 2}
	tl.Trigger(btn,
		Group{Subs: []Sub{{Effects: []Eff{{Spid: star, Para: -1, Class: "entr", Preset: 10, NodeType: "clickEffect", Bhv: FadeIn}}}}},
		Group{Subs: []Sub{{Effects: []Eff{{Spid: star, Para: -1, Class: "exit", Preset: 10, NodeType: "clickEffect", Bhv: FadeOut}}}}})
	s.Timing = tl.XML()
	s.Notes = "The star is hidden until the button is clicked."

	s = d.Add(LayoutBlank)
	s.TextBox("Title", Box{X: 60, Y: 40, W: 1100, H: 60}, "", Para{Text: "Click the icon to play or pause the tone", RPr: `sz="3200"`})
	tone := s.MediaShape(d, "audio", "Tone", Box{X: 600, Y: 300, W: 80, H: 80}, "wav", "audio/wav", WAV(2), TestPNG(80, 80))
	tl = &Timeline{id: 2}
	tl.Trigger(tone, Group{Subs: []Sub{{Effects: []Eff{{Spid: tone, Para: -1, Class: "mediacall", Preset: 2, NodeType: "clickEffect",
		Bhv: func(tl *Timeline, tgt string) string {
			return `<p:cmd type="call" cmd="togglePause"><p:cBhvr>` + ctn(tl, 1, 0, "") + tgt + `</p:cBhvr></p:cmd>`
		}}}}}})
	s.Timing = tl.XML()
	return d
}

// WMF has a Windows Metafile picture (TestWMF).
func WMF() *Deck {
	d := New("WMF feature deck")
	s := d.Add(LayoutBlank)
	s.TextBox("Title", Box{X: 60, Y: 30, W: 1160, H: 60}, "", Para{Text: "A Windows Metafile picture", RPr: `sz="3200"`})
	s.Picture(d, "Metafile", Box{X: 320, Y: 150, W: 640, H: 480}, "wmf", "image/x-wmf", TestWMF(), "", "")
	return d
}

// TransitionNames lists the transitions of the transitions3d deck, in slide order (slide 1 has
// none): the 2D transitions of PresentationML, the 3D ones of PowerPoint 2010 (p14) and the
// presets of PowerPoint 2013 (p15).
var TransitionNames = []string{
	"strips", "blinds", "checker", "comb", "randomBar", "wheel", "wedge", "diamond", "plus", "newsflash",
	// the p14 and p15 elements exactly as PowerPoint 16 writes them (set with AppleScript)
	`p14:prism`, `p14:flip dir="l"`, `p14:gallery dir="l"`, `p14:conveyor dir="l"`, `p14:doors dir="vert"`,
	`p14:window dir="vert"`, `p14:ferris dir="l"`, `p14:pan dir="u"`, `p14:switch dir="l"`, `p14:flythrough`,
	`p14:warp dir="in"`, `p14:flash`, `p14:reveal`, `p14:glitter dir="r"`, `p14:honeycomb`, `p14:vortex`,
	`p14:shred`, `p14:ripple`,
	`p15:fracture`, `p15:crush`, `p15:curtains`, `p15:wind invX="1"`, `p15:prestige`, `p15:peelOff`,
	`p15:pageCurlDouble`, `p15:airplane invX="1"`, `p15:origami invX="1"`,
}

// TransitionType returns the local name of an entry of TransitionNames.
func TransitionType(n string) string {
	n = n[strings.Index(n, ":")+1:]
	if i := strings.IndexByte(n, ' '); i >= 0 {
		n = n[:i]
	}
	return n
}

// transitionXML returns the p:transition, wrapped for p14 and p15 transitions as PowerPoint 16
// writes it (no p14:dur; the speed is spd).
func transitionXML(name string) string {
	switch {
	case strings.HasPrefix(name, "p14:"):
		return `<mc:AlternateContent xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"><mc:Choice xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main" Requires="p14">` +
			`<p:transition spd="slow"><` + name + `/></p:transition>` +
			`</mc:Choice><mc:Fallback><p:transition spd="slow"><p:fade/></p:transition></mc:Fallback></mc:AlternateContent>`
	case strings.HasPrefix(name, "p15:"):
		rest := strings.TrimPrefix(name, "p15:")
		attrs := ""
		if i := strings.IndexByte(rest, ' '); i >= 0 {
			rest, attrs = rest[:i], rest[i:]
		}
		return `<mc:AlternateContent xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"><mc:Choice xmlns:p15="http://schemas.microsoft.com/office/powerpoint/2012/main" Requires="p15">` +
			`<p:transition spd="slow"><p15:prstTrans prst="` + rest + `"` + attrs + `/></p:transition>` +
			`</mc:Choice><mc:Fallback><p:transition spd="slow"><p:fade/></p:transition></mc:Fallback></mc:AlternateContent>`
	}
	return fmt.Sprintf(`<p:transition spd="slow"><p:%s/></p:transition>`, name)
}

// Transitions3D has one slide per transition of TransitionNames, each with its name as the
// title and a colored panel, so that the browser test can see the slides change.
func Transitions3D() *Deck {
	d := New("Transition feature deck")
	colors := []string{"4472C4", "ED7D31", "70AD47", "FFC000", "5B9BD5", "A5A5A5"}
	add := func(i int, title, tr string) {
		s := d.Add(LayoutBlank)
		s.Transition = tr
		s.Shape("Panel", "rect", Box{X: 0, Y: 0, W: 1280, H: 720}, solid(colors[i%len(colors)]), "")
		s.TextBox("Title", Box{X: 80, Y: 300, W: 1120, H: 120}, "", Para{Text: title, RPr: `sz="6000" b="1"`})
	}
	add(0, "Transitions", "")
	for i, n := range TransitionNames {
		add(i+1, TransitionType(n), transitionXML(n))
	}
	return d
}

// MorphTransition returns the transition element PowerPoint writes for Morph (with a fade as
// the fallback for older readers).
func MorphTransition(ms int) string {
	return `<mc:AlternateContent xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"><mc:Choice xmlns:p159="http://schemas.microsoft.com/office/powerpoint/2015/09/main" xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main" Requires="p159">` +
		fmt.Sprintf(`<p:transition spd="slow" p14:dur="%d"><p159:morph option="byObject"/></p:transition>`, ms) +
		`</mc:Choice><mc:Fallback><p:transition spd="slow"><p:fade/></p:transition></mc:Fallback></mc:AlternateContent>`
}

// Morph has two slides joined by the Morph transition: a circle paired by its "!!" name moves,
// grows and changes color, a shape paired by its text moves, a star disappears and
// a new shape appears.
func Morph() *Deck {
	d := New("Morph feature deck")
	s := d.Add(LayoutBlank)
	s.Shape("!!Ball", "ellipse", Box{X: 120, Y: 260, W: 160, H: 160}, solid("4472C4"), "")
	s.Shape("Label", "rect", Box{X: 420, Y: 120, W: 400, H: 80}, solid("A5A5A5"), "", Para{Text: "Moving label", RPr: `sz="2800"`})
	s.Shape("Star", "star5", Box{X: 900, Y: 400, W: 200, H: 200}, solid("FFC000"), "")
	s = d.Add(LayoutBlank)
	s.Transition = MorphTransition(2000)
	s.Shape("!!Ball", "ellipse", Box{X: 820, Y: 180, W: 320, H: 320}, solid("ED7D31"), "")
	s.Shape("Label", "rect", Box{X: 140, Y: 520, W: 400, H: 80}, solid("A5A5A5"), "", Para{Text: "Moving label", RPr: `sz="2800"`})
	s.Shape("New", "triangle", Box{X: 200, Y: 160, W: 220, H: 200}, solid("70AD47"), "")
	return d
}

// Text2 covers justified text, a right-to-left paragraph (Hebrew, with Arial for the complex
// script) and builds by word and by letter.
func Text2() *Deck {
	d := New("Text feature deck 2")
	s := d.Add(LayoutBlank)
	s.TextBox("Title", Box{X: 60, Y: 30, W: 1160, H: 60}, "", Para{Text: "Justified and right-to-left text", RPr: `sz="3200"`})
	s.TextBox("Justified", Box{X: 60, Y: 120, W: 560, H: 300}, `<a:bodyPr wrap="square" rtlCol="0"><a:spAutoFit/></a:bodyPr>`,
		Para{Text: "Justified text spreads the space left on each line over the gaps between its words, so that both edges of the paragraph are straight. The last line of the paragraph stays aligned to the left.", PPr: `algn="just"`, RPr: `sz="2000"`})
	he := "\u05d6\u05d4\u05d5 \u05d8\u05e7\u05e1\u05d8 \u05de\u05d9\u05de\u05d9\u05df \u05dc\u05e9\u05de\u05d0\u05dc"
	s.TextBox("Right to left", Box{X: 680, Y: 120, W: 540, H: 120}, `<a:bodyPr wrap="square" rtlCol="1"><a:spAutoFit/></a:bodyPr>`,
		Para{Text: he, PPr: `algn="r" rtl="1"`, RPr: `sz="2800"`, RKids: `<a:latin typeface="Arial"/><a:cs typeface="Arial"/>`})

	s = d.Add(LayoutBlank)
	s.TextBox("Title", Box{X: 60, Y: 30, W: 1160, H: 60}, "", Para{Text: "Builds by word and by letter", RPr: `sz="3200"`})
	w := s.TextBox("By word", Box{X: 100, Y: 180, W: 1080, H: 80}, "", Para{Text: "This sentence appears word by word", RPr: `sz="3600"`})
	l := s.TextBox("By letter", Box{X: 100, Y: 360, W: 1080, H: 80}, "", Para{Text: "And this one letter by letter", RPr: `sz="3600"`})
	tl := &Timeline{id: 2}
	tl.Add(Group{Subs: []Sub{{Effects: []Eff{{Spid: w, Para: -1, Class: "entr", Preset: 10, NodeType: "clickEffect", Iterate: "wd", Bhv: FadeIn}}}}})
	tl.Add(Group{Subs: []Sub{{Effects: []Eff{{Spid: l, Para: -1, Class: "entr", Preset: 10, NodeType: "clickEffect", Iterate: "lt", Bhv: FadeIn}}}}})
	s.Timing = tl.XML()
	return d
}

// FontEmbed has a font embedded as PowerPoint embeds fonts (EOT with MicroType Express): a
// subset of Liberation Serif renamed to "Deckfold Embedded Test". The name is not one deckfold
// or PowerPoint can find elsewhere, and a fallback would be a sans serif font, so a wrong
// result shows.
func FontEmbed() *Deck {
	const family = "Deckfold Embedded Test"
	text := []string{"Text in an embedded font", "The quick brown fox jumps over the lazy dog.", "Umlauts and accents: \u00c4\u00d6\u00dc \u00e4\u00f6\u00fc \u00e9 \u00e8 \u00e7 \u00f1, digits 0123456789"}
	var chars string
	for _, t := range text {
		chars += t
	}
	sfnt, err := fonts.BundledSubset("LiberationSerif", family, chars)
	if err != nil {
		panic(err)
	}
	eot, err := fonts.EOT(sfnt)
	if err != nil {
		panic(err)
	}
	d := New("Embedded font feature deck")
	d.Fonts = []Font{{Typeface: family, Regular: eot}}
	s := d.Add(LayoutBlank)
	for i, t := range text {
		size := "4000"
		if i > 0 {
			size = "2800"
		}
		s.TextBox(fmt.Sprintf("Line %d", i+1), Box{X: 100, Y: 140 + float64(i)*140, W: 1080, H: 100}, "",
			Para{Text: t, RPr: `sz="` + size + `"`, RKids: `<a:latin typeface="` + family + `"/>`})
	}
	return d
}

// Effects covers the shape effects beyond outer shadow and glow: inner shadow, soft edges,
// reflection, blur, a preset shadow, an effect graph, and the same effects on a picture.
func Effects() *Deck {
	d := New("Effects feature deck")
	s := d.Add(LayoutBlank)
	s.TextBox("Title", Box{X: 60, Y: 30, W: 1160, H: 60}, "", Para{Text: "Inner shadow, soft edges, reflection, blur", RPr: `sz="3200"`})
	fx := func(e string) string { return `<a:ln><a:noFill/></a:ln><a:effectLst>` + e + `</a:effectLst>` }
	s.Shape("Inner shadow", "roundRect", Box{X: 80, Y: 160, W: 250, H: 180}, solid("4472C4"),
		fx(`<a:innerShdw blurRad="114300" dist="38100" dir="2700000"><a:prstClr val="black"><a:alpha val="60000"/></a:prstClr></a:innerShdw>`))
	s.Shape("Soft edges", "ellipse", Box{X: 380, Y: 160, W: 250, H: 180}, solid("ED7D31"), fx(`<a:softEdge rad="190500"/>`))
	s.Shape("Reflection", "rect", Box{X: 680, Y: 160, W: 250, H: 180}, solid("70AD47"),
		fx(`<a:reflection blurRad="6350" stA="52000" endA="300" endPos="35000" dir="5400000" sy="-100000" algn="bl" rotWithShape="0"/>`))
	s.Shape("Blur", "star5", Box{X: 980, Y: 160, W: 220, H: 200}, solid("FFC000"), fx(`<a:blur rad="63500"/>`))
	s.Shape("Preset shadow", "rect", Box{X: 80, Y: 460, W: 250, H: 160}, solid("A5A5A5"),
		fx(`<a:prstShdw prst="shdw17" dist="38100" dir="2700000"><a:srgbClr val="404040"/></a:prstShdw>`))
	s.Shape("Effect graph", "rect", Box{X: 380, Y: 460, W: 250, H: 160}, solid("5B9BD5"),
		`<a:ln><a:noFill/></a:ln><a:effectDag><a:cont type="sib"><a:outerShdw blurRad="76200" dist="38100" dir="2700000"><a:srgbClr val="000000"><a:alpha val="40000"/></a:srgbClr></a:outerShdw></a:cont></a:effectDag>`)

	s = d.Add(LayoutBlank)
	s.TextBox("Title", Box{X: 60, Y: 30, W: 1160, H: 60}, "", Para{Text: "Effects on pictures", RPr: `sz="3200"`})
	id := s.Picture(d, "Picture with reflection", Box{X: 140, Y: 160, W: 400, H: 225}, "png", "image/png", TestPNG(400, 225), "", "")
	id2 := s.Picture(d, "Picture with soft edges and shadow", Box{X: 700, Y: 160, W: 400, H: 225}, "png", "image/png", TestPNG(400, 225), "", "")
	addPicFx := func(id int, e string) {
		for i, sh := range s.Shapes {
			tag := fmt.Sprintf(`<p:cNvPr id="%d" `, id)
			if strings.Contains(sh, tag) {
				s.Shapes[i] = strings.Replace(sh, `</a:prstGeom></p:spPr>`, `</a:prstGeom><a:effectLst>`+e+`</a:effectLst></p:spPr>`, 1)
			}
		}
	}
	addPicFx(id, `<a:reflection blurRad="12700" stA="38000" endPos="28000" dist="5000" dir="5400000" sy="-100000" algn="bl" rotWithShape="0"/>`)
	addPicFx(id2, `<a:outerShdw blurRad="190500" dist="76200" dir="2700000" algn="tl" rotWithShape="0"><a:prstClr val="black"><a:alpha val="50000"/></a:prstClr></a:outerShdw><a:softEdge rad="63500"/>`)
	return d
}

// Bullets has picture bullets (a:buBlip): a square and a wide picture, at the text size and at
// 150 percent.
func Bullets() *Deck {
	d := New("Picture bullet feature deck")
	s := d.Add(LayoutContent)
	s.Placeholder("title", Para{Text: "Picture bullets"})
	sq := s.Rel(relImage, "../media/"+d.Media("png", "image/png", TestPNG(48, 48)), "")
	wide := s.Rel(relImage, "../media/"+d.Media("png", "image/png", TestPNG(96, 48)), "")
	blip := func(rid, size string) string {
		return `<a:buSzPct val="` + size + `"/><a:buBlip><a:blip r:embed="` + rid + `"/></a:buBlip>`
	}
	s.Placeholder("body",
		Para{Text: "A square picture as the bullet", PKids: blip(sq, "100000")},
		Para{Text: "The same picture at 150 percent", PKids: blip(sq, "150000")},
		Para{Text: "A wide picture on level two", Level: 1, PKids: blip(wide, "100000")},
		Para{Text: "Back to a character bullet"})
	return d
}

// threeD returns a:scene3d and a:sp3d for a shape.
func threeD(camera, rig, dir, sp3d string) string {
	return `<a:scene3d><a:camera prst="` + camera + `"/><a:lightRig rig="` + rig + `" dir="` + dir + `"/></a:scene3d>` + sp3d
}

// Shapes3D covers 3D shape effects: bevels with the light rigs and contours seen in real decks,
// other bevel profiles and materials, and extrusions seen through turned and perspective cameras.
func Shapes3D() *Deck {
	d := New("3D shape feature deck")
	title := func(s *Slide, t string) {
		s.TextBox("Title", Box{X: 60, Y: 30, W: 1160, H: 60}, "", Para{Text: t, RPr: `sz="3200"`})
	}
	s := d.Add(LayoutBlank)
	title(s, "Bevels seen from the front")
	for i, v := range []struct{ name, prst, color, rig string }{
		{"Bevel three point", "roundRect", "4472C4", "threePt"},
		{"Bevel contrasting", "ellipse", "ED7D31", "contrasting"},
		{"Bevel two point", "rect", "70AD47", "twoPt"},
	} {
		sp3d := `<a:sp3d contourW="7620"><a:bevelT/><a:contourClr><a:srgbClr val="FFFFFF"/></a:contourClr></a:sp3d>`
		s.Shape(v.name, v.prst, Box{X: 100 + float64(i)*380, Y: 200, W: 300, H: 200}, solid(v.color), `<a:ln><a:noFill/></a:ln>`+threeD("orthographicFront", v.rig, "t", sp3d),
			Para{Text: v.rig, RPr: `sz="2000"`})
	}
	s.Shape("Thick contour", "rect", Box{X: 100, Y: 480, W: 1060, H: 120}, solid("5B9BD5"),
		`<a:ln><a:noFill/></a:ln>`+threeD("orthographicFront", "threePt", "t", `<a:sp3d contourW="38100"><a:bevelT w="101600" h="25400"/><a:contourClr><a:srgbClr val="1F3864"/></a:contourClr></a:sp3d>`))

	s = d.Add(LayoutBlank)
	title(s, "Bevel profiles and materials")
	for i, v := range []struct{ prst, material string }{
		{"angle", "plastic"}, {"softRound", "warmMatte"}, {"coolSlant", "metal"}, {"divot", "matte"},
	} {
		sp3d := `<a:sp3d prstMaterial="` + v.material + `"><a:bevelT w="152400" h="76200" prst="` + v.prst + `"/></a:sp3d>`
		s.Shape(v.prst, "rect", Box{X: 80 + float64(i)*290, Y: 220, W: 240, H: 240}, solid("A5A5A5"), `<a:ln><a:noFill/></a:ln>`+threeD("orthographicFront", "balanced", "tl", sp3d),
			Para{Text: v.prst + ", " + v.material, RPr: `sz="1600"`})
	}

	s = d.Add(LayoutBlank)
	title(s, "Extrusions seen at an angle")
	s.Shape("Isometric cube", "rect", Box{X: 140, Y: 240, W: 240, H: 240}, solid("4472C4"),
		`<a:ln><a:noFill/></a:ln>`+threeD("isometricTopUp", "threePt", "t", `<a:sp3d extrusionH="1143000"><a:extrusionClr><a:srgbClr val="2F5597"/></a:extrusionClr></a:sp3d>`))
	s.Shape("Perspective star", "star5", Box{X: 520, Y: 220, W: 280, H: 280}, solid("FFC000"),
		`<a:ln><a:noFill/></a:ln>`+threeD("perspectiveContrastingRightFacing", "threePt", "t", `<a:sp3d extrusionH="457200" prstMaterial="plastic"><a:bevelT w="38100" h="38100"/></a:sp3d>`))
	s.Shape("Turned ring", "donut", Box{X: 900, Y: 220, W: 280, H: 280}, solid("70AD47"),
		`<a:ln><a:noFill/></a:ln><a:scene3d><a:camera prst="orthographicFront"><a:rot lat="1200000" lon="1800000" rev="0"/></a:camera><a:lightRig rig="threePt" dir="t"/></a:scene3d><a:sp3d extrusionH="228600"><a:bevelT/></a:sp3d>`)
	return d
}

func solid(hex string) string { return `<a:solidFill><a:srgbClr val="` + hex + `"/></a:solidFill>` }

func line(w int, hex, extra string) string {
	return fmt.Sprintf(`<a:ln w="%d"><a:solidFill><a:srgbClr val="%s"/></a:solidFill>%s</a:ln>`, w, hex, extra)
}

// Shapes covers preset geometries, fills, outlines, arrowheads, effects, rotation, flips and groups.
func Shapes() *Deck {
	d := New("Shapes feature deck")
	s := d.Add(LayoutBlank)
	presets := []string{"rect", "roundRect", "ellipse", "triangle", "rtTriangle", "diamond", "pentagon", "hexagon",
		"star5", "star12", "rightArrow", "chevron", "cloud", "heart", "lightningBolt", "donut", "can", "cube", "wedgeRectCallout", "flowChartDecision"}
	for i, p := range presets {
		x, y := 60+float64(i%5)*240, 40+float64(i/5)*170
		s.Shape(p, p, Box{X: x, Y: y, W: 180, H: 120}, solid("4472C4"), line(19050, "1F3864", ""), Para{Text: p, RPr: `sz="1200"`})
	}
	s.Notes = "Twenty preset shapes with text."

	s = d.Add(LayoutBlank)
	grad := `<a:gradFill rotWithShape="1"><a:gsLst><a:gs pos="0"><a:srgbClr val="FFC000"/></a:gs><a:gs pos="100000"><a:srgbClr val="C00000"/></a:gs></a:gsLst><a:lin ang="2700000" scaled="0"/></a:gradFill>`
	radial := `<a:gradFill><a:gsLst><a:gs pos="0"><a:srgbClr val="FFFFFF"/></a:gs><a:gs pos="100000"><a:srgbClr val="2E75B6"/></a:gs></a:gsLst><a:path path="circle"><a:fillToRect l="50000" t="50000" r="50000" b="50000"/></a:path></a:gradFill>`
	patt := `<a:pattFill prst="dkDnDiag"><a:fgClr><a:srgbClr val="70AD47"/></a:fgClr><a:bgClr><a:srgbClr val="FFFFFF"/></a:bgClr></a:pattFill>`
	theme := `<a:solidFill><a:schemeClr val="accent2"><a:lumMod val="75000"/></a:schemeClr></a:solidFill>`
	alpha := `<a:solidFill><a:srgbClr val="7030A0"><a:alpha val="50000"/></a:srgbClr></a:solidFill>`
	for i, fl := range []string{grad, radial, patt, theme, alpha} {
		s.Shape(fmt.Sprintf("Fill %d", i+1), "roundRect", Box{X: 60 + float64(i)*240, Y: 60, W: 200, H: 140}, fl, line(12700, "404040", ""))
	}
	dashes := []string{"solid", "dash", "dot", "lgDashDot", "sysDash"}
	for i, ds := range dashes {
		y := 260 + float64(i)*40
		s.Raw(fmt.Sprintf(`<p:cxnSp><p:nvCxnSpPr><p:cNvPr id="%d" name="Line %d"/><p:cNvCxnSpPr/><p:nvPr/></p:nvCxnSpPr><p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="0"/></a:xfrm><a:prstGeom prst="line"><a:avLst/></a:prstGeom>%s</p:spPr></p:cxnSp>`,
			s.ID(), i+1, Px(60), Px(y), Px(500), line(28575, "C00000", `<a:prstDash val="`+ds+`"/><a:headEnd type="none"/><a:tailEnd type="triangle"/>`)))
	}
	shadow := `<a:effectLst><a:outerShdw blurRad="76200" dist="38100" dir="2700000" algn="tl" rotWithShape="0"><a:srgbClr val="000000"><a:alpha val="40000"/></a:srgbClr></a:outerShdw></a:effectLst>`
	s.Raw(fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="Shadowed"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm rot="1800000"><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom>%s%s</p:spPr></p:sp>`,
		s.ID(), Px(700), Px(300), Px(200), Px(120), solid("ED7D31"), shadow))
	s.Raw(fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="Flipped arrow"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm flipH="1"><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rightArrow"><a:avLst/></a:prstGeom>%s</p:spPr></p:sp>`,
		s.ID(), Px(980), Px(300), Px(200), Px(120), solid("5B9BD5")))
	// a group scaled to half of its child extent
	g1, g2 := s.ID(), s.ID()
	s.Raw(fmt.Sprintf(`<p:grpSp><p:nvGrpSpPr><p:cNvPr id="%d" name="Group"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/><a:chOff x="0" y="0"/><a:chExt cx="%d" cy="%d"/></a:xfrm></p:grpSpPr>`+
		`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="Grouped ellipse"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="ellipse"><a:avLst/></a:prstGeom>%s</p:spPr></p:sp></p:grpSp>`,
		g1, Px(760), Px(520), Px(200), Px(100), Px(400), Px(200), g2, Px(400), Px(200), solid("A5A5A5")))
	s.Notes = "Fills, dashed lines with arrowheads, a shadow, a flipped arrow and a scaled group."
	return d
}

// Tables covers a table with a header row, banded rows, cell fills, merged cells and borders.
func Tables() *Deck {
	d := New("Tables feature deck")
	s := d.Add(LayoutContent)
	s.Placeholder("title", Para{Text: "A table with a header row"})
	cell := func(text, extra string) string {
		return `<a:tc><a:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US" sz="1600" dirty="0"/><a:t>` + Esc(text) + `</a:t></a:r></a:p></a:txBody><a:tcPr>` + extra + `</a:tcPr></a:tc>`
	}
	border := `<a:lnB w="12700"><a:solidFill><a:srgbClr val="404040"/></a:solidFill></a:lnB>`
	rows := ""
	data := [][]string{{"Quarter", "Units", "Share"}, {"First", "1200", "21 %"}, {"Second", "1850", "33 %"}, {"Third", "1400", "25 %"}, {"Fourth", "1150", "21 %"}}
	for ri, r := range data {
		rows += fmt.Sprintf(`<a:tr h="%d">`, Px(40))
		for _, c := range r {
			fill := ""
			if ri == 0 {
				fill = solid("1F3864")
			} else if ri%2 == 0 {
				fill = solid("D9E2F3")
			}
			rows += cell(c, border+fill)
		}
		rows += `</a:tr>`
	}
	rows += fmt.Sprintf(`<a:tr h="%d">`, Px(40)) + strings.Replace(cell("A merged cell over three columns", border), "<a:tc>", `<a:tc gridSpan="3">`, 1) +
		`<a:tc hMerge="1"><a:txBody><a:bodyPr/><a:lstStyle/><a:p><a:endParaRPr lang="en-US"/></a:p></a:txBody><a:tcPr/></a:tc>` +
		`<a:tc hMerge="1"><a:txBody><a:bodyPr/><a:lstStyle/><a:p><a:endParaRPr lang="en-US"/></a:p></a:txBody><a:tcPr/></a:tc></a:tr>`
	s.Raw(fmt.Sprintf(`<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="%d" name="Table"/><p:cNvGraphicFramePr><a:graphicFrameLocks noGrp="1"/></p:cNvGraphicFramePr><p:nvPr/></p:nvGraphicFramePr>`+
		`<p:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></p:xfrm><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table"><a:tbl><a:tblPr firstRow="1" bandRow="1"/>`+
		`<a:tblGrid><a:gridCol w="%d"/><a:gridCol w="%d"/><a:gridCol w="%d"/></a:tblGrid>%s</a:tbl></a:graphicData></a:graphic></p:graphicFrame>`,
		s.ID(), Px(120), Px(180), Px(900), Px(240), Px(400), Px(250), Px(250), rows))
	s.Notes = "Header row, banded rows, a merged cell."
	return d
}

// Hostile contains markup in text, notes and names, a javascript: link and an SVG picture with a
// script. The output must contain none of it in executable form.
func Hostile() *Deck {
	d := New("<script>alert(1)</script> deck")
	s := d.Add(LayoutContent)
	s.Placeholder("title", Para{Text: `</text><script>alert("title")</script>`})
	s.Placeholder("body", Para{Text: `<img src=x onerror=alert(1)> & "quotes" 'apostrophes'`})
	rid := s.Rel(relHyperlink, "javascript:alert(document.domain)", "External")
	safe := s.Rel(relHyperlink, "https://example.com/?a=1&b=2", "External")
	s.Raw(fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="&lt;b onmouseover=alert(1)&gt;"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>`+
		`<p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US"><a:hlinkClick r:id="%s"/></a:rPr><a:t>A javascript link</a:t></a:r></a:p><a:p><a:r><a:rPr lang="en-US"><a:hlinkClick r:id="%s"/></a:rPr><a:t>A safe link</a:t></a:r></a:p></p:txBody></p:sp>`,
		s.ID(), Px(100), Px(500), Px(600), Px(100), rid, safe))
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><script>alert("svg")</script><rect width="100" height="100" fill="red" onload="alert(2)"/></svg>`)
	svgName := d.Media("svg", "image/svg+xml", svg)
	pngName := d.Media("png", "image/png", TestPNG(100, 100))
	pr := s.Rel(relImage, "../media/"+pngName, "")
	sr := s.Rel(relImage, "../media/"+svgName, "")
	s.Raw(fmt.Sprintf(`<p:pic><p:nvPicPr><p:cNvPr id="%d" name="SVG picture"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr><p:blipFill><a:blip r:embed="%s"><a:extLst><a:ext uri="{96DAC541-7B7A-43D3-8B79-37D633B846F1}"><asvg:svgBlip xmlns:asvg="http://schemas.microsoft.com/office/drawing/2016/SVG/main" r:embed="%s"/></a:ext></a:extLst></a:blip><a:stretch><a:fillRect/></a:stretch></p:blipFill>`+
		`<p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>`,
		s.ID(), pr, sr, Px(900), Px(450), Px(200), Px(200)))
	s.Notes = `Notes with <script>alert("notes")</script> and </textarea><b>markup</b>.`
	return d
}

// Media has an audio clip that starts with the slide and a Windows Media video, which browsers
// cannot play (only its poster is shown, with a warning).
func Media() *Deck {
	d := New("Media feature deck")
	s := d.Add(LayoutContent)
	s.Placeholder("title", Para{Text: "Audio that starts with the slide"})
	id := s.MediaShape(d, "audio", "Tone", Box{X: 600, Y: 300, W: 80, H: 80}, "wav", "audio/wav", WAV(2), TestPNG(80, 80))
	s.Timing = AutoPlay(id, 2000)
	s.Notes = "A two second tone starts automatically."
	s = d.Add(LayoutContent)
	s.Placeholder("title", Para{Text: "A video in a format browsers cannot play"})
	s.MediaShape(d, "video", "Old video", Box{X: 340, Y: 200, W: 600, H: 340}, "wmv", "video/x-ms-wmv", []byte("not a real video"), TestPNG(600, 340))
	return d
}

// Text covers alignment, numbering, line spacing, run formatting, shrink on overflow, vertical
// text and a slide number field.
func Text() *Deck {
	d := New("Text feature deck")
	d.FirstSlideNum = 5
	s := d.Add(LayoutBlank)
	s.TextBox("Aligned", Box{X: 60, Y: 40, W: 500, H: 160}, `<a:bodyPr wrap="square"/>`,
		Para{Text: "Left aligned"}, Para{Text: "Centered", PPr: `algn="ctr"`}, Para{Text: "Right aligned", PPr: `algn="r"`})
	s.TextBox("Numbered", Box{X: 60, Y: 220, W: 500, H: 200}, `<a:bodyPr wrap="square"/>`,
		Para{Text: "First item", PPr: `marL="342900" indent="-342900"`, PKids: `<a:buFont typeface="+mj-lt"/><a:buAutoNum type="arabicPeriod"/>`},
		Para{Text: "Second item", PPr: `marL="342900" indent="-342900"`, PKids: `<a:buFont typeface="+mj-lt"/><a:buAutoNum type="arabicPeriod"/>`},
		Para{Text: "Third item", PPr: `marL="342900" indent="-342900"`, PKids: `<a:buFont typeface="+mj-lt"/><a:buAutoNum type="arabicPeriod"/>`})
	s.TextBox("Runs", Box{X: 640, Y: 40, W: 580, H: 160}, `<a:bodyPr wrap="square"/>`,
		Para{Text: "Bold red text", RPr: `b="1"`, RKids: `<a:solidFill><a:srgbClr val="C00000"/></a:solidFill>`},
		Para{Text: "Italic underlined", RPr: `i="1" u="sng"`},
		Para{Text: "Wide character spacing", RPr: `spc="300"`})
	long := "This text box shrinks its text on overflow. It holds more text than fits at the declared size of twenty-four points, so the text has to become smaller to stay inside the box."
	s.TextBox("Shrinking", Box{X: 640, Y: 240, W: 420, H: 120}, `<a:bodyPr wrap="square"><a:normAutofit/></a:bodyPr>`, Para{Text: long, RPr: `sz="2400"`})
	s.TextBox("Shrunk", Box{X: 640, Y: 520, W: 420, H: 90}, `<a:bodyPr wrap="square"><a:normAutofit fontScale="62500" lnSpcReduction="20000"/></a:bodyPr>`, Para{Text: "Stored scale: this text is drawn at 62.5 percent of its 24 point size.", RPr: `sz="2400"`})
	s.TextBox("Vertical", Box{X: 1120, Y: 240, W: 80, H: 400}, `<a:bodyPr vert="vert270" wrap="square" anchor="ctr"/>`, Para{Text: "Vertical text", RPr: `sz="2800"`, PPr: `algn="ctr"`})
	s.Raw(fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="Slide number"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>`+
		`<p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US"/><a:t>Slide </a:t></a:r><a:fld id="{B6F15528-21DE-4FAA-801E-634DDDAF4B2B}" type="slidenum"><a:rPr lang="en-US"/><a:t>99</a:t></a:fld></a:p></p:txBody></p:sp>`,
		s.ID(), Px(60), Px(620), Px(300), Px(40)))
	s.Raw(fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="Line spacing"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>`+
		`<p:txBody><a:bodyPr wrap="square"/><a:lstStyle/><a:p><a:pPr><a:lnSpc><a:spcPct val="150000"/></a:lnSpc></a:pPr><a:r><a:rPr lang="en-US"/><a:t>One and a half line spacing over two lines of text in this box</a:t></a:r></a:p></p:txBody></p:sp>`,
		s.ID(), Px(60), Px(440), Px(500), Px(140)))
	s.Notes = "Text features."
	return d
}

// AnimSpec is the expected state of the animated shapes after each step of a slide, read by the
// browser test (fixtures/pptx/src/animations.spec.json).
type AnimSpec struct {
	Slide int                 `json:"slide"` // index among the visible slides
	Steps []map[string]string `json:"steps"` // per step: target ("<spid>" or "<spid>:p<n>") -> "visible", "hidden", "moved", "scaled"
}

// ids replaces shape names in the expected states by the targets the player uses.
func ids(steps []map[string]string, names map[string]int) []map[string]string {
	out := make([]map[string]string, len(steps))
	for i, st := range steps {
		out[i] = map[string]string{}
		for k, v := range st {
			name, para := k, ""
			if j := strings.Index(k, ":"); j >= 0 {
				name, para = k[:j], k[j:]
			}
			out[i][fmt.Sprint(names[name])+para] = v
		}
	}
	return out
}

// Animations covers the animation core: entrance (fade, fly in, wipe), exit, emphasis, a motion
// path, a paragraph build, with and after previous, and transitions with automatic advance.
func Animations() *Deck {
	d, _ := AnimationsWithSpec()
	return d
}

// AnimationsWithSpec returns the deck and the expected states.
func AnimationsWithSpec() (*Deck, []AnimSpec) {
	d := New("Animations feature deck")
	var spec []AnimSpec
	box := func(s *Slide, name string, x float64, hex string) int {
		return s.Shape(name, "rect", Box{X: x, Y: 300, W: 160, H: 120}, solid(hex), "", Para{Text: name, RPr: `sz="1400"`})
	}

	// slide 1: entrance effects, each on its own click
	s := d.Add(LayoutBlank)
	s.TextBox("Title", Box{X: 60, Y: 40, W: 1000, H: 60}, "", Para{Text: "Entrance effects", RPr: `sz="3200"`})
	a, b, c := box(s, "Fade", 100, "4472C4"), box(s, "Fly", 400, "ED7D31"), box(s, "Wipe", 700, "70AD47")
	tl := &Timeline{}
	tl.Add(Group{Subs: []Sub{{Effects: []Eff{{Spid: a, Para: -1, Class: "entr", Preset: 10, NodeType: "clickEffect", Bhv: FadeIn}}}}})
	tl.Add(Group{Subs: []Sub{{Effects: []Eff{{Spid: b, Para: -1, Class: "entr", Preset: 2, Subtype: 4, NodeType: "clickEffect", Bhv: FlyIn}}}}})
	tl.Add(Group{Subs: []Sub{{Effects: []Eff{{Spid: c, Para: -1, Class: "entr", Preset: 22, Subtype: 4, NodeType: "clickEffect", Bhv: WipeIn}}}}})
	s.Timing = tl.XML()
	spec = append(spec, AnimSpec{Slide: 0, Steps: ids([]map[string]string{
		{"Fade": "hidden", "Fly": "hidden", "Wipe": "hidden"},
		{"Fade": "visible", "Fly": "hidden", "Wipe": "hidden"},
		{"Fade": "visible", "Fly": "visible", "Wipe": "hidden"},
		{"Fade": "visible", "Fly": "visible", "Wipe": "visible"},
	}, map[string]int{"Fade": a, "Fly": b, "Wipe": c})})

	// slide 2: exit, emphasis, motion path; with and after previous in one click
	s = d.Add(LayoutBlank)
	s.Transition = `<p:transition spd="med"><p:fade/></p:transition>`
	s.TextBox("Title", Box{X: 60, Y: 40, W: 1000, H: 60}, "", Para{Text: "Exit, emphasis, motion", RPr: `sz="3200"`})
	ex, gr, mv := box(s, "Exit", 100, "A5A5A5"), box(s, "Grow", 400, "FFC000"), box(s, "Move", 700, "5B9BD5")
	tl = &Timeline{}
	tl.Add(Group{Subs: []Sub{{Effects: []Eff{{Spid: ex, Para: -1, Class: "exit", Preset: 10, NodeType: "clickEffect", Bhv: FadeOut}}}}})
	tl.Add(Group{Subs: []Sub{
		{Effects: []Eff{{Spid: gr, Para: -1, Class: "emph", Preset: 6, NodeType: "clickEffect", Bhv: Scale(150, 500)}}},
		{Delay: 500, Effects: []Eff{{Spid: mv, Para: -1, Class: "path", Preset: 42, NodeType: "afterEffect", Bhv: Motion("M 0 0 L 0.2 0 E", 1000)}}},
	}})
	s.Timing = tl.XML()
	spec = append(spec, AnimSpec{Slide: 1, Steps: ids([]map[string]string{
		{"Exit": "visible", "Grow": "visible", "Move": "visible"},
		{"Exit": "hidden", "Grow": "visible", "Move": "visible"},
		{"Exit": "hidden", "Grow": "scaled", "Move": "moved"},
	}, map[string]int{"Exit": ex, "Grow": gr, "Move": mv})})

	// slide 3: a paragraph build; the slide advances by itself after 3 s
	s = d.Add(LayoutContent)
	s.Transition = `<p:transition spd="fast" advTm="3000"><p:push dir="u"/></p:transition>`
	s.Placeholder("title", Para{Text: "A paragraph build"})
	body := s.ID()
	s.Raw(fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="Points"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph idx="1"/></p:nvPr></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/>%s</p:txBody></p:sp>`,
		body, paras([]Para{{Text: "First point"}, {Text: "Second point"}, {Text: "Third point"}})))
	tl = &Timeline{}
	for i := 0; i < 3; i++ {
		tl.Add(Group{Subs: []Sub{{Effects: []Eff{{Spid: body, Para: i, Class: "entr", Preset: 10, NodeType: "clickEffect", Bhv: FadeIn}}}}})
	}
	s.Timing = tl.XML()
	spec = append(spec, AnimSpec{Slide: 2, Steps: ids([]map[string]string{
		{"Points:p0": "hidden", "Points:p1": "hidden", "Points:p2": "hidden"},
		{"Points:p0": "visible", "Points:p1": "hidden", "Points:p2": "hidden"},
		{"Points:p0": "visible", "Points:p1": "visible", "Points:p2": "hidden"},
		{"Points:p0": "visible", "Points:p1": "visible", "Points:p2": "visible"},
	}, map[string]int{"Points": body})})

	s = d.Add(LayoutBlank)
	s.Transition = `<p:transition spd="med"><p:wipe dir="r"/></p:transition>`
	s.TextBox("End", Box{X: 60, Y: 300, W: 1000, H: 60}, "", Para{Text: "The end", RPr: `sz="4000"`, PPr: `algn="ctr"`})
	return d, spec
}

// Charts covers the chart types of the first version: clustered column, stacked bar, line, area,
// pie and doughnut.
func Charts() *Deck {
	d := New("Charts feature deck")
	cats := []string{"North", "East", "South", "West"}
	ser := []ChartSeries{{"2024", []float64{12, 19, 8, 15}}, {"2025", []float64{14, 17, 11, 18}}, {"2026", []float64{16, 21, 9, 20}}}
	s := d.Add(LayoutBlank)
	s.Chart(d, "Columns", Box{X: 40, Y: 40, W: 600, H: 320}, "barChart", `<c:barDir val="col"/><c:grouping val="clustered"/>`, "Clustered columns", cats, ser)
	s.Chart(d, "Stacked bars", Box{X: 660, Y: 40, W: 580, H: 320}, "barChart", `<c:barDir val="bar"/><c:grouping val="stacked"/><c:overlap val="100"/>`, "Stacked bars", cats, ser)
	s.Chart(d, "Lines", Box{X: 40, Y: 380, W: 600, H: 320}, "lineChart", `<c:grouping val="standard"/>`, "Lines", cats, ser)
	s.Chart(d, "Percent", Box{X: 660, Y: 380, W: 580, H: 320}, "barChart", `<c:barDir val="col"/><c:grouping val="percentStacked"/><c:overlap val="100"/>`, "100 % columns", cats, ser)
	s = d.Add(LayoutBlank)
	s.Chart(d, "Area", Box{X: 40, Y: 40, W: 600, H: 320}, "areaChart", `<c:grouping val="standard"/>`, "Area", cats, ser[:1])
	s.Chart(d, "Pie", Box{X: 660, Y: 40, W: 580, H: 320}, "pieChart", "", "Pie", cats, ser[:1])
	s.Chart(d, "Doughnut", Box{X: 40, Y: 380, W: 600, H: 320}, "doughnutChart", "", "Doughnut", cats, ser[:1])
	return d
}
