package pptx

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	// the overflowing 24 pt text (32 px) is drawn smaller
	re := regexp.MustCompile(`font-size="([0-9.]+)"[^>]*>This text box`)
	m := re.FindStringSubmatch(html)
	if m == nil {
		t.Fatal("shrinking text not found")
	}
	var size float64
	fmt.Sscan(m[1], &size)
	if size >= 32 {
		t.Errorf("shrinking text drawn at %v px, want less than 32", size)
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
