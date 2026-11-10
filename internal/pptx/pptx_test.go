package pptx

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"deckfold/internal/htmlscan"
	"deckfold/internal/pptx/deck"
	"deckfold/internal/pptx/pptxgen"
)

// convert writes a generated deck to a temp file and converts it with the bundled fonts.
func convert(t *testing.T, d *pptxgen.Deck) (string, *Result, deck.Data) {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "in.pptx")
	if err := d.Write(src); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "deck")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := Convert(src, out, Options{BundledFontsOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	html, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	doc := htmlscan.Scan(html)
	var data deck.Data
	for _, tg := range doc.Tags {
		if tg.Name == "script" && tg.Value("id") == deck.DataID {
			if err := json.Unmarshal(doc.Content(tg), &data); err != nil {
				t.Fatal(err)
			}
		}
	}
	return string(html), res, data
}

func TestBasicDeck(t *testing.T) {
	html, res, data := convert(t, pptxgen.Basic())
	if res.Slides != 3 || res.Hidden != 1 || len(data.Slides) != 3 {
		t.Fatalf("slides %d, hidden %d, data %d; want 3, 1, 3", res.Slides, res.Hidden, len(data.Slides))
	}
	if strings.Contains(html, "This slide is hidden") {
		t.Error("the hidden slide was rendered")
	}
	if data.Slides[0].Notes != "Notes of the first slide.\nA second paragraph of notes." {
		t.Errorf("notes of slide 1 = %q", data.Slides[0].Notes)
	}
	if data.Slides[1].Label != "Bullets and a picture" {
		t.Errorf("label of slide 2 = %q", data.Slides[1].Label)
	}
	if data.Slides[2].Number != 4 {
		t.Errorf("third visible slide is number %d in the file, want 4", data.Slides[2].Number)
	}
	sections := 0
	for _, tg := range htmlscan.Scan([]byte(html)).Tags {
		if tg.Name == "section" && tg.Value("class") == deck.SlideClass {
			sections++
		}
	}
	if sections != 3 {
		t.Errorf("%d slide sections", sections)
	}
	// the bullet character comes from the master body style
	if !strings.Contains(html, ">•</text>") {
		t.Error("no bullet drawn")
	}
}

func TestShapesDeck(t *testing.T) {
	html, res, _ := convert(t, pptxgen.Shapes())
	for _, w := range res.Warnings {
		if strings.Contains(w, "unknown preset") || strings.Contains(w, "cannot be evaluated") {
			t.Errorf("warning: %s", w)
		}
	}
	for _, want := range []string{"<linearGradient", "<radialGradient", "<pattern", "<feDropShadow", "<marker", "stroke-dasharray", "rotate(30 ", "scale(-1 1)"} {
		if !strings.Contains(html, want) {
			t.Errorf("output has no %s", want)
		}
	}
	// the group halves its child: the ellipse is 200 x 100 px at 760, 520
	if !strings.Contains(html, "M760 570") {
		t.Errorf("grouped ellipse not scaled into the group box")
	}
}

func TestTablesDeck(t *testing.T) {
	html, res, _ := convert(t, pptxgen.Tables())
	for _, w := range res.Warnings {
		if strings.Contains(w, "table") {
			t.Errorf("warning: %s", w)
		}
	}
	for _, want := range []string{">Quarter</text>", ">1850</text>", ">A merged cell over three columns</text>", `fill="#1f3864"`, `fill="#d9e2f3"`} {
		if !strings.Contains(html, want) {
			t.Errorf("table output has no %s", want)
		}
	}
}

// Nothing from a hostile deck ends up executable: markup is escaped, javascript: links are
// dropped, an SVG picture is referenced through <image>, never inlined.
func TestHostileDeck(t *testing.T) {
	html, _, data := convert(t, pptxgen.Hostile())
	body := html[:strings.Index(html, deck.PlayerBegin)]
	for _, bad := range []*regexp.Regexp{
		regexp.MustCompile(`(?i)<script>alert`), regexp.MustCompile(`(?i)javascript:`), regexp.MustCompile(`(?i)<img`),
		regexp.MustCompile(`(?i)<b `), regexp.MustCompile(`(?i)</textarea`),
	} {
		if loc := bad.FindStringIndex(body); loc != nil {
			t.Errorf("output contains %q near %q", bad.String(), body[max(0, loc[0]-40):min(len(body), loc[1]+40)])
		}
	}
	// no element gets an event handler attribute, and only the player and data scripts exist
	scripts := 0
	for _, tg := range htmlscan.Scan([]byte(html)).Tags {
		for _, a := range tg.Attrs {
			if strings.HasPrefix(strings.ToLower(a.Name), "on") {
				t.Errorf("<%s %s=...> in the output", tg.Name, a.Name)
			}
		}
		if tg.Name == "script" {
			scripts++
		}
	}
	if scripts != 2 {
		t.Errorf("%d script elements, want 2 (deck data and player)", scripts)
	}
	if !strings.Contains(body, `href="https://example.com/?a=1&amp;b=2"`) {
		t.Error("the safe link is missing")
	}
	if !regexp.MustCompile(`<image href="assets/[0-9a-f]+\.svg"`).MatchString(body) {
		t.Error("the SVG picture is not referenced as an image file")
	}
	if !strings.Contains(data.Slides[0].Notes, `<script>alert("notes")</script>`) {
		t.Error("notes text lost (it is data, escaped by JSON encoding)")
	}
	if strings.Contains(html, `</script> deck`) {
		t.Error("the title closes a script element")
	}
}

func TestMediaDeck(t *testing.T) {
	html, res, _ := convert(t, pptxgen.Media())
	if !regexp.MustCompile(`<audio xmlns="http://www.w3.org/1999/xhtml" src="assets/[0-9a-f]+\.wav" preload="auto" data-df-pptx-media="auto">`).MatchString(html) {
		t.Error("the audio clip is not marked to start with the slide")
	}
	if strings.Contains(html, ".wmv") {
		t.Error("the WMV file was embedded although browsers cannot play it")
	}
	warned := false
	for _, w := range res.Warnings {
		warned = warned || strings.Contains(w, ".wmv file")
	}
	if !warned {
		t.Errorf("no warning about the WMV video: %v", res.Warnings)
	}
}

// A linked picture outside the presentation folder is refused (E10), a missing one too.
func TestLinkedOutside(t *testing.T) {
	d := pptxgen.New("Linked")
	s := d.Add(pptxgen.LayoutBlank)
	rid := s.Rel("http://schemas.openxmlformats.org/officeDocument/2006/relationships/image", "../../outside.png", "External")
	s.Raw(`<p:pic><p:nvPicPr><p:cNvPr id="5" name="Linked"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr><p:blipFill><a:blip r:link="` + rid + `"/><a:stretch><a:fillRect/></a:stretch></p:blipFill><p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="952500" cy="952500"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>`)
	dir := t.TempDir()
	src := filepath.Join(dir, "in.pptx")
	if err := d.Write(src); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "deck")
	os.Mkdir(out, 0o755)
	_, err := Convert(src, out, Options{BundledFontsOnly: true})
	var me *MissingError
	if !errors.As(err, &me) || len(me.Missing) != 1 {
		t.Fatalf("want one missing linked file, got %v", err)
	}
	if _, err := Convert(src, out, Options{BundledFontsOnly: true, Lenient: true}); err != nil {
		t.Fatalf("with Lenient the deck is written: %v", err)
	}
}

func TestTextDeck(t *testing.T) {
	html, _, _ := convert(t, pptxgen.Text())
	for _, want := range []string{">1.</text>", ">2.</text>", ">3.</text>", ">Slide 5</text>", "rotate(270 ", "text-decoration:underline", "letter-spacing:4px", `fill="#c00000"`} {
		if !strings.Contains(html, want) {
			t.Errorf("text output has no %s", want)
		}
	}
	// "shrink on overflow" uses the scale stored in the file, like PowerPoint: without one the
	// 24 pt text (32 px) keeps its size, with fontScale="62500" it is drawn at 20 px
	sizeOf := func(start string) float64 {
		m := regexp.MustCompile(`font-size="([0-9.]+)"[^>]*>` + start).FindStringSubmatch(html)
		if m == nil {
			t.Fatalf("text %q not found", start)
		}
		var v float64
		fmt.Sscan(m[1], &v)
		return v
	}
	if v := sizeOf("This text box"); v != 32 {
		t.Errorf("text without a stored scale drawn at %v px, want 32", v)
	}
	if v := sizeOf("Stored scale"); v < 19.9 || v > 20.1 {
		t.Errorf("text with fontScale 62.5 %% drawn at %v px, want 20", v)
	}
}

// The timing of the animations deck becomes the expected steps: three clicks on slide 1, two on
// slide 2 (the motion path follows the emphasis automatically), three paragraph steps on slide 3.
func TestAnimationsDeck(t *testing.T) {
	_, res, data := convert(t, pptxgen.Animations())
	steps := []int{}
	for _, s := range data.Slides {
		steps = append(steps, s.Steps)
	}
	if fmt.Sprint(steps) != "[4 3 4 1]" {
		t.Fatalf("steps per slide = %v, want [4 3 4 1]", steps)
	}
	s2 := data.Slides[1].Anim
	if len(s2.Steps[2].Effects) != 2 || s2.Steps[2].Effects[1].Begin != 500 {
		t.Errorf("after-previous effect: %+v", s2.Steps[2].Effects)
	}
	if s2.Transition == nil || s2.Transition.Type != "fade" || s2.Transition.Dur != 750 {
		t.Errorf("transition of slide 2: %+v", s2.Transition)
	}
	s3 := data.Slides[2]
	if s3.Advance == nil || *s3.Advance != 3 || s3.Anim.Transition.Type != "push" {
		t.Errorf("slide 3: advance %v, transition %+v", s3.Advance, s3.Anim.Transition)
	}
	if got := fmt.Sprint(data.Slides[0].Anim.Hidden); !strings.HasSuffix(got, "]") || len(data.Slides[0].Anim.Hidden) != 3 {
		t.Errorf("hidden at start: %v", got)
	}
	for _, w := range res.Warnings {
		t.Logf("warning: %s", w)
	}
}

func TestChartsDeck(t *testing.T) {
	html, res, _ := convert(t, pptxgen.Charts())
	for _, w := range res.Warnings {
		if strings.Contains(w, "chart") {
			t.Errorf("warning: %s", w)
		}
	}
	for _, want := range []string{">Clustered columns</text>", ">North</text>", ">100%</text>", ">2026</text>", "stroke-linejoin=\"round\""} {
		if !strings.Contains(html, want) {
			t.Errorf("chart output has no %s", want)
		}
	}
	// 3 series x 4 categories as bars in each of the three bar charts, plus the legend keys
	if n := strings.Count(html, "<rect x="); n < 36 {
		t.Errorf("only %d rectangles drawn", n)
	}
}

// The text of a table style (white bold header row) ranks above the presentation's default
// text styles, which give black text.
func TestTableStyleText(t *testing.T) {
	d := pptxgen.New("Table style")
	id := "{11111111-2222-3333-4444-555555555555}"
	d.TableStyles = `<a:tblStyle styleId="` + id + `" styleName="Test"><a:wholeTbl><a:tcTxStyle><a:fontRef idx="minor"/><a:schemeClr val="dk1"/></a:tcTxStyle></a:wholeTbl>` +
		`<a:firstRow><a:tcTxStyle b="on"><a:fontRef idx="minor"/><a:schemeClr val="lt1"/></a:tcTxStyle><a:tcStyle><a:fill><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:fill></a:tcStyle></a:firstRow></a:tblStyle>`
	s := d.Add(pptxgen.LayoutBlank)
	cell := func(text string) string {
		return `<a:tc><a:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US"/><a:t>` + text + `</a:t></a:r></a:p></a:txBody><a:tcPr/></a:tc>`
	}
	s.Raw(`<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="4" name="Table"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr><p:xfrm><a:off x="952500" y="952500"/><a:ext cx="5715000" cy="952500"/></p:xfrm>` +
		`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table"><a:tbl><a:tblPr firstRow="1"><a:tableStyleId>` + id + `</a:tableStyleId></a:tblPr>` +
		`<a:tblGrid><a:gridCol w="5715000"/></a:tblGrid><a:tr h="476250">` + cell("Header") + `</a:tr><a:tr h="476250">` + cell("Body") + `</a:tr></a:tbl></a:graphicData></a:graphic></p:graphicFrame>`)
	html, _, _ := convert(t, d)
	header := regexp.MustCompile(`<text [^>]*fill="([^"]+)"[^>]*>Header</text>`).FindStringSubmatch(html)
	body := regexp.MustCompile(`<text [^>]*font-family="([^"]+)"[^>]*fill="([^"]+)"[^>]*>Body</text>`).FindStringSubmatch(html)
	if header == nil || body == nil {
		t.Fatal("table text not found")
	}
	if header[1] != "#ffffff" {
		t.Errorf("header text color %s, want #ffffff from the table style", header[1])
	}
	if body[2] != "#000000" {
		t.Errorf("body text color %s, want #000000", body[2])
	}
	hf := regexp.MustCompile(`<text [^>]*font-family="([^"]+)"[^>]*>Header</text>`).FindStringSubmatch(html)
	if hf == nil || hf[1] == body[1] {
		t.Error("the header row should use the bold face")
	}
}

func TestTriggersDeck(t *testing.T) {
	html, res, data := convert(t, pptxgen.Triggers())
	s1 := data.Slides[0].Anim
	if data.Slides[0].Steps != 1 || len(s1.Triggers) != 1 {
		t.Fatalf("slide 1: %d steps, triggers %+v; want 1 step and one trigger", data.Slides[0].Steps, s1.Triggers)
	}
	tr := s1.Triggers[0]
	if len(tr.Steps) != 2 || tr.Steps[0].Effects[0].Class != "entr" || tr.Steps[1].Effects[0].Class != "exit" {
		t.Errorf("trigger steps: %+v", tr.Steps)
	}
	star := tr.Steps[0].Effects[0].Target
	if fmt.Sprint(s1.Hidden) != "["+star+"]" {
		t.Errorf("hidden at start = %v, want the star (%s)", s1.Hidden, star)
	}
	doc := htmlscan.Scan([]byte(html))
	shape, clickMedia := false, false
	for _, tg := range doc.Tags {
		shape = shape || tg.Name == "g" && tg.Value("data-sid") == tr.Shape
		clickMedia = clickMedia || tg.Name == "audio" && tg.Value("data-df-pptx-media") == "click"
	}
	if !shape {
		t.Errorf("no shape with the trigger id %s", tr.Shape)
	}
	// "play on click" of the audio clip's own icon stays with the player's media handling
	if s2 := data.Slides[1].Anim; len(s2.Triggers) != 0 {
		t.Errorf("slide 2: the media trigger was kept: %+v", s2.Triggers)
	}
	if !clickMedia {
		t.Error("the audio clip is not marked to start on a click")
	}
	if len(res.Warnings) != 0 {
		t.Errorf("warnings: %v", res.Warnings)
	}
}

func TestShapes3DDeck(t *testing.T) {
	html, res, _ := convert(t, pptxgen.Shapes3D())
	doc := htmlscan.Scan([]byte(html))
	var data []map[string]any
	three, lighting := 0, 0
	for _, tg := range doc.Tags {
		if v := tg.Value("data-df-pptx-3d"); v != "" {
			var m map[string]any
			if err := json.Unmarshal([]byte(v), &m); err != nil {
				t.Fatalf("data-df-pptx-3d is not JSON: %v", err)
			}
			data = append(data, m)
		}
		if tg.Name == "script" && tg.Value("id") == "df-pptx-three" {
			three++
		}
		if tg.Name == "fediffuselighting" {
			lighting++
		}
	}
	if len(data) != 11 {
		t.Fatalf("%d shapes with 3D data, want 11", len(data))
	}
	bt, _ := data[0]["bevelT"].(map[string]any)
	if bt["w"] != 8.0 || bt["h"] != 8.0 || bt["prst"] != "circle" || data[0]["contour"] != 0.8 || data[0]["light"] != "threePt" {
		t.Errorf("first bevel: %v", data[0])
	}
	if data[8]["camera"] != "isometricTopUp" || data[8]["depth"] != 120.0 || data[8]["depthColor"] != "#2f5597" {
		t.Errorf("isometric extrusion: %v", data[8])
	}
	if rot, _ := data[10]["rot"].([]any); len(rot) != 3 || rot[0] != 20.0 || rot[1] != 30.0 {
		t.Errorf("explicit camera rotation: %v", data[10]["rot"])
	}
	if three != 1 {
		t.Errorf("%d three.js scripts, want 1", three)
	}
	if lighting < 8 {
		t.Errorf("%d lighting filters for 8 bevels seen from the front", lighting)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("warnings: %v", res.Warnings)
	}
	// a deck without 3D content does not get three.js
	html, _, _ = convert(t, pptxgen.Basic())
	for _, tg := range htmlscan.Scan([]byte(html)).Tags {
		if tg.Name == "script" && tg.Value("id") == "df-pptx-three" {
			t.Error("the basic deck embeds three.js")
		}
	}
}

func TestBulletsDeck(t *testing.T) {
	html, res, _ := convert(t, pptxgen.Bullets())
	doc := htmlscan.Scan([]byte(html))
	// picture bullets are images inside the paragraph groups of the text (which hold no groups)
	type span struct{ from, to int }
	var paras []span
	for _, tg := range doc.Tags {
		if tg.Name == "g" && tg.Value("data-p") != "" {
			paras = append(paras, span{tg.End, tg.End + strings.Index(html[tg.End:], "</g>")})
		}
	}
	bullets := 0
	var heights []string
	for _, tg := range doc.Tags {
		for _, p := range paras {
			if tg.Name == "image" && tg.Start >= p.from && tg.Start < p.to {
				bullets++
				heights = append(heights, tg.Value("height"))
			}
		}
	}
	if bullets != 3 {
		t.Fatalf("%d picture bullets, want 3", bullets)
	}
	if h0, h1 := num(heights[0]), num(heights[1]); h1 < h0*1.45 || h1 > h0*1.55 {
		t.Errorf("bullet heights %v: the second should be 150 percent of the first", heights)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("warnings: %v", res.Warnings)
	}
}

func num(s string) float64 {
	var v float64
	fmt.Sscan(s, &v)
	return v
}

func TestEffectsDeck(t *testing.T) {
	html, res, _ := convert(t, pptxgen.Effects())
	count := map[string]int{}
	for _, tg := range htmlscan.Scan([]byte(html)).Tags {
		count[tg.Name]++
		if tg.Name == "g" && tg.Value("mask") != "" {
			count["reflection"]++
		}
	}
	// inner shadow: one flood; soft edges: three alpha ramps (shape, picture); reflections on a
	// shape and a picture; shadows: preset, effect graph, picture
	if count["feflood"] != 1 || count["reflection"] != 2 || count["fedropshadow"] != 3 {
		t.Errorf("filter primitives: %v", count)
	}
	if count["fegaussianblur"] < 6 {
		t.Errorf("%d blurs; inner shadow, two soft edges, blur and reflections need more", count["fegaussianblur"])
	}
	want := "effect graphs (effectDag) are approximated by the shadows, glows and edges they contain"
	if len(res.Warnings) != 1 || res.Warnings[0] != want {
		t.Errorf("warnings: %v", res.Warnings)
	}
}

// The embedded font (EOT with MTX, as PowerPoint stores it) is used, not a substitute.
func TestFontEmbedDeck(t *testing.T) {
	_, res, _ := convert(t, pptxgen.FontEmbed())
	if fmt.Sprint(res.Fonts) != "[Deckfold Embedded Test: embedded (embedded:Deckfold Embedded Test regular)]" {
		t.Errorf("fonts: %v", res.Fonts)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("warnings: %v", res.Warnings)
	}
}

// The decks of features without occurrences in real presentations convert without warnings
// and produce what the player needs.
func TestGapDecks(t *testing.T) {
	for name, check := range map[string]func(t *testing.T, html string, data deck.Data){
		"text2": func(t *testing.T, html string, data deck.Data) {
			doc := htmlscan.Scan([]byte(html))
			words, letters, rtl, justified := 0, 0, 0, 0
			for _, tg := range doc.Tags {
				words += map[bool]int{true: 1}[tg.Name == "g" && tg.Value("data-w") != ""]
				letters += map[bool]int{true: 1}[tg.Name == "g" && tg.Value("data-c") != ""]
				rtl += map[bool]int{true: 1}[tg.Name == "text" && tg.Value("direction") == "rtl"]
				justified += map[bool]int{true: 1}[tg.Name == "text" && strings.Contains(tg.Value("style"), "word-spacing")]
			}
			if words != 6 || letters != len("Andthisoneletterbyletter") || rtl == 0 || justified == 0 {
				t.Errorf("words %d, letters %d, rtl %d, justified %d", words, letters, rtl, justified)
			}
			e := data.Slides[1].Anim.Steps[1].Effects[0]
			if e.Iterate != "wd" || e.IterDelay != 50 {
				t.Errorf("word build: %+v", e)
			}
		},
		"morph": func(t *testing.T, html string, data deck.Data) {
			if tr := data.Slides[1].Anim.Transition; tr == nil || tr.Type != "morph" || tr.Dur != 2000 {
				t.Errorf("transition %+v", tr)
			}
			keys := map[string]int{}
			for _, tg := range htmlscan.Scan([]byte(html)).Tags {
				if k := tg.Value("data-mk"); k != "" {
					keys[k]++
				}
			}
			pairs := 0
			for _, n := range keys {
				if n == 2 {
					pairs++
				}
			}
			if pairs != 2 {
				t.Errorf("%d objects on both slides, want the circle and the label", pairs)
			}
		},
		"transitions3d": func(t *testing.T, html string, data deck.Data) {
			for i, n := range pptxgen.TransitionNames {
				want := pptxgen.TransitionType(n)
				if tr := data.Slides[i+1].Anim.Transition; tr == nil || tr.Type != want {
					t.Errorf("slide %d: transition %+v, want %s", i+2, tr, want)
				}
			}
		},
		"wmf": func(t *testing.T, html string, data deck.Data) {
			if !regexp.MustCompile(`<image href="assets/[0-9a-f]+\.svg"`).MatchString(html) {
				t.Error("the WMF picture was not converted to SVG")
			}
		},
		"charts2": func(t *testing.T, html string, data deck.Data) {
			doc := htmlscan.Scan([]byte(html))
			circles, charts3d := 0, 0
			for _, tg := range doc.Tags {
				circles += map[bool]int{true: 1}[tg.Name == "circle" && num(tg.Value("r")) > 5] // bubbles, not markers
				charts3d += map[bool]int{true: 1}[tg.Value("data-df-pptx-chart3d") != ""]
			}
			if circles != 4 || charts3d != 5 {
				t.Errorf("%d bubbles (want 4), %d 3D plots (want 5)", circles, charts3d)
			}
		},
		"ink": func(t *testing.T, html string, data deck.Data) {
			strokes := 0
			for _, tg := range htmlscan.Scan([]byte(html)).Tags {
				strokes += map[bool]int{true: 1}[tg.Name == "path" && tg.Value("stroke") == "#E71224"]
			}
			if strokes != 3 {
				t.Errorf("%d ink strokes, want 3", strokes)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			html, res, data := convert(t, pptxgen.Fixtures()[name])
			if len(res.Warnings) != 0 {
				t.Errorf("warnings: %v", res.Warnings)
			}
			check(t, html, data)
		})
	}
}

// A picture fill that a slide placeholder inherits from its layout uses the layout's
// relationships (a regression: it used to look the picture up in the slide's).
func TestInheritedPictureFill(t *testing.T) {
	d := pptxgen.New("Inherited fill")
	s := d.Add(pptxgen.LayoutContent)
	s.Placeholder("title", pptxgen.Para{Text: "Inherited picture fill"})
	s.Placeholder("body", pptxgen.Para{Text: "The body placeholder has the layout's picture fill"})
	raw, err := d.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	// rewrite the package: the layout's body placeholder gets a picture fill with its own
	// relationship id, which the slide does not have
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	png := pptxgen.TestPNG(64, 48)
	for _, f := range zr.File {
		rc, _ := f.Open()
		data, _ := io.ReadAll(rc)
		rc.Close()
		switch f.Name {
		case "ppt/slideLayouts/slideLayout2.xml":
			re := regexp.MustCompile(`(<p:ph[^>]*idx="1"[^>]*/></p:nvPr></p:nvSpPr><p:spPr>)`)
			if !re.Match(data) {
				t.Fatalf("layout body placeholder not found: %s", data)
			}
			data = re.ReplaceAll(data, []byte(`${1}<a:blipFill><a:blip r:embed="rId77"/><a:stretch><a:fillRect/></a:stretch></a:blipFill>`))
		case "ppt/slideLayouts/_rels/slideLayout2.xml.rels":
			data = bytes.Replace(data, []byte("</Relationships>"), []byte(`<Relationship Id="rId77" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/layoutfill.png"/></Relationships>`), 1)
		case "[Content_Types].xml":
			if !bytes.Contains(data, []byte(`Extension="png"`)) {
				data = bytes.Replace(data, []byte(`<Default Extension="xml"`), []byte(`<Default Extension="png" ContentType="image/png"/><Default Extension="xml"`), 1)
			}
		}
		w, _ := zw.Create(f.Name)
		w.Write(data)
	}
	w, _ := zw.Create("ppt/media/layoutfill.png")
	w.Write(png)
	zw.Close()

	dir := t.TempDir()
	src := filepath.Join(dir, "in.pptx")
	os.WriteFile(src, out.Bytes(), 0o644)
	deckDir := filepath.Join(dir, "deck")
	os.Mkdir(deckDir, 0o755)
	if _, err := Convert(src, deckDir, Options{BundledFontsOnly: true}); err != nil {
		t.Fatal(err)
	}
	html, _ := os.ReadFile(filepath.Join(deckDir, "index.html"))
	var pic string
	for _, tg := range htmlscan.Scan(html).Tags {
		if tg.Name == "image" && strings.HasSuffix(tg.Value("href"), ".png") {
			pic = strings.TrimPrefix(tg.Value("href"), "assets/")
		}
	}
	if pic == "" {
		t.Fatal("the inherited picture fill is missing")
	}
	got, err := os.ReadFile(filepath.Join(deckDir, "assets", pic))
	if err != nil || !bytes.Equal(got, png) {
		t.Errorf("the fill shows %s, not the layout's picture", pic)
	}
}

// convertFile converts a PowerPoint file with the bundled fonts.
func convertFile(t *testing.T, src string) (*Result, deck.Data) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "deck")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := Convert(src, out, Options{BundledFontsOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	html, _ := os.ReadFile(filepath.Join(out, "index.html"))
	var data deck.Data
	for _, tg := range htmlscan.Scan(html).Tags {
		if tg.Name == "script" && tg.Value("id") == deck.DataID {
			if err := json.Unmarshal(htmlscan.Scan(html).Content(tg), &data); err != nil {
				t.Fatal(err)
			}
		}
	}
	return res, data
}

// The decks PowerPoint built itself (scripts/pptxref/ppdecks.sh): the timing and transition XML
// as PowerPoint writes it for effects it created.
func TestPowerPointBuiltDecks(t *testing.T) {
	res, data := convertFile(t, filepath.Join("..", "..", "fixtures", "pptx", "powerpoint", "animations.pptx"))
	if len(res.Warnings) != 0 {
		t.Errorf("animations: warnings %v", res.Warnings)
	}
	steps := []int{}
	for _, s := range data.Slides {
		steps = append(steps, s.Steps)
	}
	if fmt.Sprint(steps) != "[4 3 4 2]" {
		t.Errorf("steps per slide = %v, want [4 3 4 2]", steps)
	}
	if e := data.Slides[0].Anim.Steps[3].Effects[0]; e.Class != "exit" {
		t.Errorf("third click on slide 1: %+v, want an exit", e)
	}
	kinds := map[string]bool{}
	for _, st := range data.Slides[1].Anim.Steps {
		for _, e := range st.Effects {
			for _, b := range e.Bhv {
				kinds[b.Type] = true
			}
		}
	}
	if !kinds["scale"] || !kinds["motion"] {
		t.Errorf("slide 2 behaviors %v, want scale and motion", kinds)
	}
	for k := 1; k <= 3; k++ {
		if tg := data.Slides[2].Anim.Steps[k].Effects[0].Target; !strings.HasSuffix(tg, fmt.Sprintf(":p%d", k-1)) {
			t.Errorf("paragraph build step %d targets %s", k, tg)
		}
	}
	st := data.Slides[3].Anim.Steps[1]
	if len(st.Effects) != 3 || st.Effects[1].Begin != 0 || st.Effects[2].Begin <= 0 {
		t.Errorf("click, with previous, after previous: %+v", st.Effects)
	}

	res, data = convertFile(t, filepath.Join("..", "..", "fixtures", "pptx", "powerpoint", "transitions.pptx"))
	if len(res.Warnings) != 0 {
		t.Errorf("transitions: warnings %v", res.Warnings)
	}
	var types []string
	for _, s := range data.Slides[1:] {
		types = append(types, s.Anim.Transition.Type)
	}
	if fmt.Sprint(types) != "[fade push wipe reveal cover cut morph fade]" {
		t.Errorf("transitions %v", types)
	}
	if last := data.Slides[8]; last.Advance == nil || *last.Advance != 3 || last.Anim.Transition.Dur != 2000 {
		t.Errorf("automatic advance %v, duration %v", last.Advance, last.Anim.Transition.Dur)
	}
}

// The content deck: the text blocks the converter stores for the content model are the ones the
// generator put on the slides, with their roles, list levels and the steps in which they show.
// Footer, slide number and the hidden slide are not among them.
func TestContentDeck(t *testing.T) {
	d, spec := pptxgen.ContentWithSpec()
	html, res, data := convert(t, d)
	if data.Version != deck.BlocksVersion || len(data.Slides) != len(spec.Slides) {
		t.Fatalf("version %d, %d slides, want version %d and %d slides", data.Version, len(data.Slides), deck.BlocksVersion, len(spec.Slides))
	}
	for i, want := range spec.Slides {
		got := data.Slides[i]
		if got.Label != want.Title || got.Notes != want.Notes {
			t.Errorf("slide %d: title %q and notes %q, want %q and %q", i+1, got.Label, got.Notes, want.Title, want.Notes)
		}
		if len(got.Blocks) != len(want.Blocks) {
			t.Errorf("slide %d: %d blocks, want %d: %+v", i+1, len(got.Blocks), len(want.Blocks), got.Blocks)
			continue
		}
		for k, w := range want.Blocks {
			g := got.Blocks[k]
			if g.Role != w.Role || g.Text != w.Text || g.Step != w.Step || g.Level != w.Level {
				t.Errorf("slide %d, block %d: %+v, want %+v", i+1, k+1, g, w)
			}
			// links to other slides: a slide by its index among the shown slides, or a direction
			gj, wj := "", ""
			if g.Jump != nil {
				gj = g.Jump.To
				if g.Jump.Slide != nil {
					gj = fmt.Sprint(*g.Jump.Slide)
				}
			}
			if w.Jump != nil {
				wj = w.Jump.To
				if w.Jump.Slide != nil {
					wj = fmt.Sprint(*w.Jump.Slide)
				}
			}
			if gj != wj {
				t.Errorf("slide %d, block %q: jump %q, want %q", i+1, w.Text, gj, wj)
			}
		}
	}
	// the links are in the drawing too: one element per link on text, one on the linked shape
	jumps := map[string]int{}
	for _, tg := range htmlscan.Scan([]byte(html)).Tags {
		if j, ok := tg.Get("data-df-jump"); ok {
			jumps[tg.Name+" "+j.Val]++
		}
	}
	want := map[string]int{"a next": 1, "a prev": 1, "a first": 1, "a last": 1, "a back": 1, "a end": 1, "a 1": 1, "g 2": 1}
	if !reflect.DeepEqual(jumps, want) {
		t.Errorf("links in the drawing: %v, want %v", jumps, want)
	}
	// the link to the hidden slide stays text and is reported; the other links are not
	hidden, other := false, false
	for _, w := range res.Warnings {
		hidden = hidden || strings.Contains(w, "link to a hidden slide")
		other = other || strings.Contains(w, "other than a jump")
	}
	if !hidden || other {
		t.Errorf("warnings: %q", res.Warnings)
	}
}
