package pptx

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"deckfold/internal/htmlscan"
	"deckfold/internal/pptx/fonts"
	"deckfold/internal/pptx/pptxgen"
)

// svgWord is a word as the SVG of a slide places it: the x of its first letter and the width up
// to the end of its last letter, in px, and the line of its paragraph.
type svgWord struct {
	text     string
	line     int
	x, width float64
}

var (
	letterSpacingRe = regexp.MustCompile(`letter-spacing:([-\d.]+)px`)
	tspanDxRe       = regexp.MustCompile(`^<tspan[^>]*\bdx="([-\d.]+)"`)
)

// kerningRowsInSVG reads the words of every text box of the converted kerning deck, keyed by
// slide and the left and top of the box. It reads them the way a browser draws them: every
// letter advances by the width the font gives it at the font size of its text element, plus the
// letter spacing of the element, and a tspan with dx moves the letters from there on.
func kerningRowsInSVG(t *testing.T, page []byte, face func(slide, x, y int) *fonts.Face) map[[3]int][]svgWord {
	t.Helper()
	type letter struct {
		r       rune
		x, adv  float64
		y       float64
		spacing float64
	}
	letters := map[[3]int][]letter{}
	doc := htmlscan.Scan(page)
	slide, key := 0, [3]int{}
	for _, tg := range doc.Tags {
		switch {
		case tg.Name == "svg" && strings.Contains(tg.Value("class"), "df-pptx-svg"):
			slide++
		case tg.Name == "g" && tg.Value("data-box") != "":
			f := strings.Fields(tg.Value("data-box"))
			x, _ := strconv.ParseFloat(f[0], 64)
			y, _ := strconv.ParseFloat(f[1], 64)
			key = [3]int{slide, int(math.Round(x)), int(math.Round(y))}
		case tg.Name == "text":
			fc := face(key[0], key[1], key[2])
			if fc == nil {
				continue
			}
			x, _ := strconv.ParseFloat(tg.Value("x"), 64)
			y, _ := strconv.ParseFloat(tg.Value("y"), 64)
			size, _ := strconv.ParseFloat(tg.Value("font-size"), 64)
			spacing := 0.0
			if m := letterSpacingRe.FindStringSubmatch(tg.Value("style")); m != nil {
				spacing, _ = strconv.ParseFloat(m[1], 64)
			}
			end := strings.Index(string(doc.Src[tg.End:]), "</text>")
			if end < 0 {
				t.Fatalf("text element at %d is not closed", tg.Start)
			}
			body := string(doc.Src[tg.End : tg.End+end])
			for body != "" {
				if body[0] == '<' {
					close := strings.IndexByte(body, '>')
					if m := tspanDxRe.FindStringSubmatch(body[:close+1]); m != nil {
						dx, _ := strconv.ParseFloat(m[1], 64)
						x += dx
					}
					body = body[close+1:]
					continue
				}
				next := strings.IndexByte(body, '<')
				if next < 0 {
					next = len(body)
				}
				for _, r := range html.UnescapeString(body[:next]) {
					adv := fc.Advance(r) * size
					letters[key] = append(letters[key], letter{r, x, adv, y, spacing})
					x += adv + spacing
				}
				body = body[next:]
			}
		}
	}
	out := map[[3]int][]svgWord{}
	for k, ls := range letters {
		sort.SliceStable(ls, func(i, j int) bool {
			if ls[i].y != ls[j].y {
				return ls[i].y < ls[j].y
			}
			return ls[i].x < ls[j].x
		})
		line, start := 0, -1
		flush := func(i int) {
			if start >= 0 {
				var s strings.Builder
				for _, l := range ls[start:i] {
					s.WriteRune(l.r)
				}
				last := ls[i-1]
				out[k] = append(out[k], svgWord{s.String(), line, ls[start].x, last.x + last.adv - ls[start].x})
				start = -1
			}
		}
		for i, l := range ls {
			if i > 0 && l.y != ls[i-1].y {
				flush(i)
				line++
			}
			if l.r == ' ' {
				flush(i)
			} else if start < 0 {
				start = i
			}
		}
		flush(len(ls))
	}
	return out
}

// The kerning deck against PowerPoint: every word of every row has to stand where PowerPoint
// put it and be as wide (fixtures/pptx/ref/kerning/words.json, read from PowerPoint's PDF by
// scripts/kernmeasure). The expectation is PowerPoint's own numbers, and what is compared with
// them is read from the SVG as a browser draws it, so the renderer is not held against itself.
//
// PowerPoint's widths lie up to 0.4 % from the font's numbers at 28 pt and up to 0.9 % at 10 to
// 12 pt, kerning or not; kerning changes the kerned words of the deck by 2 to 11 %.
func TestKerningDeckAgainstPowerPoint(t *testing.T) {
	root := filepath.Join("..", "..", "fixtures", "pptx")
	saved := filepath.Join(root, "kerning.pptx")
	data, err := os.ReadFile(saved)
	if err != nil {
		t.Fatal(err)
	}
	var words struct {
		Source string `json:"source"`
		Rows   []struct {
			ID    string `json:"id"`
			Slide int    `json:"slide"`
			Font  string `json:"font"`
			Words []struct {
				Text  string  `json:"text"`
				Line  int     `json:"line"`
				X     float64 `json:"x"`
				Width float64 `json:"width"`
			} `json:"words"`
		} `json:"rows"`
	}
	js, err := os.ReadFile(filepath.Join(root, "ref", "kerning", "words.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(js, &words); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != words.Source {
		t.Fatalf("words.json belongs to another kerning.pptx; write it again:\n  go run ./scripts/kernmeasure -spec fixtures/pptx/src/kerning.spec.json -pdf tests/.out/pptxref/kerning.pdf -saved fixtures/pptx/kerning.pptx -words fixtures/pptx/ref/kerning/words.json")
	}
	_, spec := pptxgen.KerningWithSpec()
	if len(spec.Rows) != len(words.Rows) {
		t.Fatalf("the spec has %d rows, words.json %d", len(spec.Rows), len(words.Rows))
	}

	out := filepath.Join(t.TempDir(), "deck")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Convert(saved, out, Options{BundledFontsOnly: true}); err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	res := fonts.NewResolver(nil)
	res.BundledOnly = true
	rowAt := map[[3]int]pptxgen.KerningRow{}
	for _, r := range spec.Rows {
		rowAt[[3]int{r.Slide, r.Box[0], r.Box[1]}] = r
	}
	got := kerningRowsInSVG(t, page, func(slide, x, y int) *fonts.Face {
		r, ok := rowAt[[3]int{slide, x, y}]
		if !ok {
			return nil
		}
		return res.Face(r.Font, r.Bold, false)
	})

	const pt = 96.0 / 72 // words.json has points
	byID := map[string][]svgWord{}
	for i, r := range spec.Rows {
		ws := got[[3]int{r.Slide, r.Box[0], r.Box[1]}]
		byID[r.Font+"/"+r.ID] = ws
		want := words.Rows[i]
		if want.ID != r.ID || want.Font != r.Font {
			t.Fatalf("row %d: words.json has %s %s, the spec %s %s", i, want.Font, want.ID, r.Font, r.ID)
		}
		if len(ws) != len(want.Words) {
			t.Errorf("%s %s: %d words in the SVG, %d in PowerPoint", r.Font, r.ID, len(ws), len(want.Words))
			continue
		}
		if r.FontScale > 0 {
			// PowerPoint drew this box at 11.0 pt where the stored scale gives 11.2 pt, which is
			// not about kerning; the row is only compared with its twin, below
			continue
		}
		var bad []string
		left := ws[0].x
		for j, w := range ws {
			p := want.Words[j]
			switch {
			case w.text != p.Text:
				bad = append(bad, fmt.Sprintf("word %d is %q, PowerPoint has %q", j, w.text, p.Text))
			case w.line != p.Line:
				bad = append(bad, fmt.Sprintf("%q is on line %d, in PowerPoint on line %d", w.text, w.line, p.Line))
			case math.Abs(w.width-p.Width*pt) > 0.012*p.Width*pt+0.1:
				bad = append(bad, fmt.Sprintf("%q is %.2f px wide, in PowerPoint %.2f px (%+.1f %%)", w.text, w.width, p.Width*pt, 100*(w.width/(p.Width*pt)-1)))
			case math.Abs(w.x-p.X*pt) > 0.006*(p.X*pt-left)+1:
				bad = append(bad, fmt.Sprintf("%q starts at %.1f px, in PowerPoint at %.1f px", w.text, w.x, p.X*pt))
			}
		}
		if len(bad) > 0 {
			t.Errorf("%s %s (kern %s at %g pt): %s", r.Font, r.ID, r.Kern, r.SizePt, strings.Join(bad, "; "))
		}
	}
	// the box with a stored font scale of 40 %: 11.2 pt is below kern="1200", so the row has to
	// equal its twin with kern="0", as it does in PowerPoint
	for _, r := range spec.Rows {
		if r.FontScale == 0 || r.Twin == "" {
			continue
		}
		a, b := byID[r.Font+"/"+r.ID], byID[r.Font+"/"+r.Twin]
		if len(a) != len(b) {
			t.Errorf("%s %s: %d words, its twin %d", r.Font, r.ID, len(a), len(b))
			continue
		}
		for j := range a {
			if math.Abs(a[j].width-b[j].width) > 0.01 || math.Abs(a[j].x-b[j].x) > 0.01 {
				t.Errorf("%s %s: %q differs from the twin with kern off (%.2f px wide at %.2f, twin %.2f at %.2f)", r.Font, r.ID, a[j].text, a[j].width, a[j].x, b[j].width, b[j].x)
			}
		}
	}
}

// Builds by word and by letter cut the text into pieces with their own positions. A kerned
// line has to keep the place of every word when it is cut, and the pieces must not overlap or
// drift apart.
func TestKernedTextKeepsItsPlacesInBuilds(t *testing.T) {
	const text = "AVATAR Toy Year PAVE"
	d := pptxgen.New("Kerned builds")
	s := d.Add(pptxgen.LayoutBlank)
	box := func(name string, y float64) int {
		return s.TextBox(name, pptxgen.Box{X: 100, Y: y, W: 1000, H: 60}, "",
			pptxgen.Para{Text: text, RPr: `sz="2800" kern="1200"`, RKids: `<a:latin typeface="Liberation Sans"/>`})
	}
	box("Whole", 100)
	w, l := box("By word", 250), box("By letter", 400)
	tl := &pptxgen.Timeline{}
	tl.Add(pptxgen.Group{Subs: []pptxgen.Sub{{Effects: []pptxgen.Eff{{Spid: w, Para: -1, Class: "entr", Preset: 10, NodeType: "clickEffect", Iterate: "wd", Bhv: pptxgen.FadeIn}}}}})
	tl.Add(pptxgen.Group{Subs: []pptxgen.Sub{{Effects: []pptxgen.Eff{{Spid: l, Para: -1, Class: "entr", Preset: 10, NodeType: "clickEffect", Iterate: "lt", Bhv: pptxgen.FadeIn}}}}})
	s.Timing = tl.XML()
	page, _, _ := convert(t, d)

	res := fonts.NewResolver(nil)
	res.BundledOnly = true
	face := res.Face("Liberation Sans", false, false)
	rows := kerningRowsInSVG(t, []byte(page), func(slide, x, y int) *fonts.Face { return face })
	whole, byWord, byLetter := rows[[3]int{1, 100, 100}], rows[[3]int{1, 100, 250}], rows[[3]int{1, 100, 400}]
	if len(whole) != 4 {
		t.Fatalf("%d words in the line that is not cut: %+v", len(whole), whole)
	}
	// the line is kerned at all: AVATAR is narrower than its advance widths
	plain := 0.0
	for _, r := range "AVATAR" {
		plain += face.Advance(r) * 28 * 96 / 72
	}
	if whole[0].width > plain-5 {
		t.Fatalf("AVATAR is %.2f px wide, its advance widths give %.2f px: the line is not kerned", whole[0].width, plain)
	}
	groups := 0
	for _, tg := range htmlscan.Scan([]byte(page)).Tags {
		if _, ok := tg.Get("data-c"); ok && tg.Name == "g" {
			groups++
		}
	}
	if want := len(strings.ReplaceAll(text, " ", "")); groups != want {
		t.Errorf("%d groups of a letter, want one per letter (%d)", groups, want)
	}
	for name, cut := range map[string][]svgWord{"by word": byWord, "by letter": byLetter} {
		if len(cut) != len(whole) {
			t.Errorf("%s: %d words, want %d", name, len(cut), len(whole))
			continue
		}
		for i, w := range whole {
			// positions are written with two decimals, per piece
			if c := cut[i]; c.text != w.text || math.Abs(c.x-w.x) > 0.02 || math.Abs(c.width-w.width) > 0.03 {
				t.Errorf("%s: %q at %.2f px, %.2f px wide; in the line that is not cut at %.2f px, %.2f px wide", name, c.text, c.x, c.width, w.x, w.width)
			}
		}
	}
}

// Shaped text is left to the shaper and the browser. The Hebrew line of the text2 deck is in
// Arial, which deckfold draws in Liberation Sans and kerns: the line must stay one piece (its
// spaces once became kerned pieces of their own, and the line fell apart), and it is the one
// place where the browser's own kerning stays on.
func TestShapedRunIsNotCutByKerning(t *testing.T) {
	page, _, _ := convert(t, pptxgen.Text2())
	doc := htmlscan.Scan([]byte(page))
	slides, rtl := 0, 0
	for _, tg := range doc.Tags {
		switch {
		case tg.Name == "svg" && strings.Contains(tg.Value("class"), "df-pptx-svg"):
			slides++
			if !strings.Contains(tg.Value("style"), "font-kerning:none") {
				t.Errorf("slide %d: the svg element does not switch the browser's kerning off (style %q)", slides, tg.Value("style"))
			}
		case tg.Name == "text" && tg.Value("direction") == "rtl":
			rtl++
			end := strings.Index(string(doc.Src[tg.End:]), "</text>")
			if words := len(strings.Fields(html.UnescapeString(string(doc.Src[tg.End : tg.End+end])))); words != 4 {
				t.Errorf("the right-to-left line is cut: a piece of %d words, want the 4 words in one", words)
			}
			if !strings.Contains(tg.Value("style"), "font-kerning:auto") {
				t.Errorf("the right-to-left line does not keep the browser's kerning (style %q)", tg.Value("style"))
			}
		}
	}
	if slides == 0 || rtl != 1 {
		t.Errorf("%d slides, %d right-to-left pieces; want one piece", slides, rtl)
	}
}
