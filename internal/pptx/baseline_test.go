package pptx

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"deckfold/internal/htmlscan"
	"deckfold/internal/pptx/pptxgen"
)

// powerPointBaselines has, for every box of the baseline deck (pptxgen.Baselines), where
// PowerPoint put the three baselines: points below the top of the box, read from the PDF that
// PowerPoint 16.114.4 for Mac exported on 2026-11-10 (pdftocairo -svg gives the baseline of
// every glyph). PowerPoint puts a baseline a whole number of points below the top of the text
// and the PDF has positions on a grid of 0.24 pt, hence values such as 26.88 for 27.
var powerPointBaselines = []struct {
	font, id string
	pt       [3]float64
}{
	{"Liberation Sans", "pct70", [3]float64{18, 41.04, 65.04}},
	{"Liberation Sans", "pct80", [3]float64{20.88, 47.04, 73.92}},
	{"Liberation Sans", "pct90", [3]float64{24, 54, 84}},
	{"Liberation Sans", "pct100", [3]float64{26.88, 60.96, 94.08}},
	{"Liberation Sans", "pct110", [3]float64{28.08, 65.04, 102}},
	{"Liberation Sans", "pct120", [3]float64{30, 71.04, 110.88}},
	{"Liberation Sans", "pct150", [3]float64{37.92, 88.08, 138.96}},
	{"Liberation Sans", "pct200", [3]float64{49.92, 118.08, 185.04}},
	{"Liberation Sans", "pts20", [3]float64{14.88, 35.04, 54.96}},
	{"Liberation Sans", "pts28", [3]float64{22.08, 49.92, 78}},
	{"Liberation Sans", "pts40", [3]float64{30, 70.08, 109.92}},
	{"Liberation Sans", "pts50", [3]float64{37.92, 88.08, 138}},
	{"Liberation Sans", "reduce10", [3]float64{24, 54, 84}},
	{"Liberation Sans", "reduce20", [3]float64{20.88, 47.04, 73.92}},
	{"Liberation Sans", "reduce20-scale625", [3]float64{11.04, 24.96, 40.08}},
	{"Liberation Sans", "reduce10-pct90", [3]float64{20.88, 47.04, 73.92}},
	{"Liberation Sans", "pct90-18pt", [3]float64{14.88, 35.04, 54}},
	{"Liberation Sans", "pct150-18pt", [3]float64{24, 56.88, 89.04}},
	{"Liberation Sans", "pct100-ctr", [3]float64{201.6, 235.68, 268.56}},
	{"Liberation Sans", "pct100-b", [3]float64{376.08, 410.16, 443.28}},
	{"Liberation Sans", "pct90-ctr", [3]float64{203.52, 233.52, 263.52}},
	{"Liberation Sans", "pct150-b", [3]float64{342.96, 393.12, 444}},
	{"Carlito", "pct70", [3]float64{18, 41.04, 65.04}},
	{"Carlito", "pct80", [3]float64{19.92, 47.04, 73.92}},
	{"Carlito", "pct90", [3]float64{23.04, 53.04, 83.04}},
	{"Carlito", "pct100", [3]float64{25.92, 60, 92.88}},
	{"Carlito", "pct110", [3]float64{28.08, 65.04, 102}},
	{"Carlito", "pct120", [3]float64{30, 71.04, 110.88}},
	{"Carlito", "pct150", [3]float64{37.92, 88.08, 138.96}},
	{"Carlito", "pct200", [3]float64{49.92, 118.08, 185.04}},
	{"Carlito", "pts20", [3]float64{14.88, 35.04, 54.96}},
	{"Carlito", "pts28", [3]float64{20.88, 48.96, 77.04}},
	{"Carlito", "pts40", [3]float64{30, 70.08, 109.92}},
	{"Carlito", "pts50", [3]float64{37.92, 88.08, 138}},
	{"Carlito", "reduce10", [3]float64{23.04, 53.04, 83.04}},
	{"Carlito", "reduce20", [3]float64{19.92, 47.04, 73.92}},
	{"Carlito", "reduce20-scale625", [3]float64{11.04, 24.96, 40.08}},
	{"Carlito", "reduce10-pct90", [3]float64{19.92, 47.04, 73.92}},
	{"Carlito", "pct90-18pt", [3]float64{14.88, 34.08, 54}},
	{"Carlito", "pct150-18pt", [3]float64{24, 56.88, 89.04}},
	{"Carlito", "pct100-ctr", [3]float64{200.64, 234.48, 267.6}},
	{"Carlito", "pct100-b", [3]float64{375.12, 409.2, 442.08}},
	{"Carlito", "pct90-ctr", [3]float64{202.56, 232.56, 262.56}},
	{"Carlito", "pct150-b", [3]float64{342, 391.92, 443.04}},
}

// Where the renderer puts the baselines against where PowerPoint puts them: every line
// spacing in percent and in points, a stored reduction of the line spacing, a stored font scale
// and the three vertical anchors, in a font whose metrics leave room in a line (Liberation Sans)
// and in one that fills it (Carlito). The numbers are PowerPoint's, so the renderer is not held
// against itself.
func TestBaselinesAgainstPowerPoint(t *testing.T) {
	d, spec := pptxgen.Baselines("Liberation Sans", "Carlito")
	if len(spec) != len(powerPointBaselines) {
		t.Fatalf("the deck has %d boxes, the table %d", len(spec), len(powerPointBaselines))
	}
	page, _, _ := convert(t, d)
	// the baselines of every box, by slide and the left edge of the box
	got := map[[2]int][]float64{}
	doc := htmlscan.Scan([]byte(page))
	slide, key := 0, [2]int{}
	for _, tg := range doc.Tags {
		switch {
		case tg.Name == "svg" && strings.Contains(tg.Value("class"), "df-pptx-svg"):
			slide++
		case tg.Name == "g" && tg.Value("data-box") != "":
			x, _ := strconv.ParseFloat(strings.Fields(tg.Value("data-box"))[0], 64)
			key = [2]int{slide, int(math.Round(x))}
		case tg.Name == "text":
			y, _ := strconv.ParseFloat(tg.Value("y"), 64)
			if ys := got[key]; len(ys) == 0 || ys[len(ys)-1] != y {
				got[key] = append(got[key], y)
			}
		}
	}
	for i, b := range spec {
		want := powerPointBaselines[i]
		if want.font != b.Font || want.id != b.ID {
			t.Fatalf("box %d is %s %s, the table has %s %s", i, b.Font, b.ID, want.font, want.id)
		}
		ys := got[[2]int{b.Slide, b.Box[0]}]
		if len(ys) != 3 {
			t.Errorf("%s %s: %d baselines, want 3", b.Font, b.ID, len(ys))
			continue
		}
		for j, y := range ys {
			pt := (y - float64(b.Box[1])) * 72 / 96
			if math.Abs(pt-want.pt[j]) > 0.25 {
				t.Errorf("%s %s, line %d: baseline %.2f pt below the top of the box, in PowerPoint %.2f pt", b.Font, b.ID, j+1, pt, want.pt[j])
			}
		}
	}
}
