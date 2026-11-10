package main

import (
	"fmt"
	"strings"
	"testing"

	"deckfold/internal/pptx/kernprobe"
	"deckfold/internal/pptx/pptxgen"
)

// page writes what pdftotext -bbox would print for the rows of a spec, with word widths from the
// bundled font: kerned(row) says whether the row is set with the kern table pairs, and wobble
// widens every third word by that share of its width (two control words and one kerned word of
// the line), like PowerPoint's own inexactness.
func page(t *testing.T, spec pptxgen.KerningSpec, kerned func(r pptxgen.KerningRow) bool, wobble float64) []byte {
	t.Helper()
	var b strings.Builder
	b.WriteString("<doc>\n  <page width=\"960.000000\" height=\"540.000000\">\n")
	slide := 1
	for _, r := range spec.Rows {
		if r.Slide != slide {
			b.WriteString("  </page>\n  <page width=\"960.000000\" height=\"540.000000\">\n")
			slide = r.Slide
		}
		f, err := kernprobe.Bundled(bundledNames[r.Font], r.Bold)
		if err != nil {
			t.Fatal(err)
		}
		x, y := float64(r.Box[0])*pxToPt, float64(r.Box[1])*pxToPt
		for i, w := range strings.Fields(strings.Join(r.Runs, "")) {
			em := f.Plain(w)
			if kerned(r) {
				em += f.Kern(w)
			}
			width := em * r.SizePt
			if i%3 == 0 {
				width *= 1 + wobble
			}
			// a word in two colors comes out of pdftotext in two pieces
			if len(r.Runs) > 1 && len(w) > 1 && i == 1 {
				fmt.Fprintf(&b, "    <word xMin=\"%f\" yMin=\"%f\" xMax=\"%f\" yMax=\"%f\">%s</word>\n", x, y, x+width/3, y+r.SizePt*1.2, w[:1])
				fmt.Fprintf(&b, "    <word xMin=\"%f\" yMin=\"%f\" xMax=\"%f\" yMax=\"%f\">%s</word>\n", x+width/3, y, x+width, y+r.SizePt*1.2, w[1:])
			} else {
				fmt.Fprintf(&b, "    <word xMin=\"%f\" yMin=\"%f\" xMax=\"%f\" yMax=\"%f\">%s</word>\n", x, y, x+width, y+r.SizePt*1.2, w)
			}
			x += width + f.Plain(" ")*r.SizePt
		}
	}
	b.WriteString("  </page>\n</doc>\n")
	return []byte(b.String())
}

// rowsOf keeps the single-line rows of the public deck in Liberation Sans.
func rowsOf(t *testing.T) pptxgen.KerningSpec {
	t.Helper()
	_, full := pptxgen.KerningWithSpec()
	var spec pptxgen.KerningSpec
	for _, r := range full.Rows {
		if r.Font == "Liberation Sans" && !r.Wrap && r.SpcPt == 0 {
			spec.Rows = append(spec.Rows, r)
		}
	}
	if len(spec.Rows) < 8 {
		t.Fatalf("only %d rows", len(spec.Rows))
	}
	return spec
}

func verdicts(t *testing.T, spec pptxgen.KerningSpec, bbox []byte) map[string]string {
	t.Helper()
	pages, err := parseBBox(bbox)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := measure(spec, pages, fontFlags{}, []string{"LiberationSans", "LiberationSans-Bold"})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, r := range rows {
		if r.Err != "" {
			t.Fatalf("row %s: %s", r.Row.ID, r.Err)
		}
		out[r.Row.ID] = r.Verdict
	}
	return out
}

func TestVerdictFollowsTheWidths(t *testing.T) {
	spec := rowsOf(t)
	// a PowerPoint that kerns from the size in kern on, and whose widths are 0.2 % off the
	// font's numbers for some words, in every row alike
	rule := func(r pptxgen.KerningRow) bool {
		switch r.Kern {
		case "0", "none":
			return false
		case "2000":
			return r.SizePt >= 20
		}
		return r.SizePt >= 12
	}
	got := verdicts(t, spec, page(t, spec, rule, 0.002))
	for _, r := range spec.Rows {
		// Liberation has the same values in both tables
		want := "kerned: kern table = GPOS"
		switch {
		case r.Twin == "":
			want = kernOff
		case !rule(r):
			want = "not kerned"
		}
		if !strings.HasPrefix(got[r.ID], want) {
			t.Errorf("row %s (kern %s at %g pt): %q, want %q", r.ID, r.Kern, r.SizePt, got[r.ID], want)
		}
	}
}

func TestVerdictIsUnclearWhenTheRowWithKernOffIsNotPlain(t *testing.T) {
	spec := rowsOf(t)
	// a PowerPoint that ignores kern="0" and kerns every row: no row is plainly unkerned, and
	// no row can be judged against its twin
	got := verdicts(t, spec, page(t, spec, func(pptxgen.KerningRow) bool { return true }, 0.002))
	for id, v := range got {
		if !strings.HasPrefix(v, "unclear") {
			t.Errorf("row %s: %q, want unclear", id, v)
		}
	}
}

func TestVerdictIsUnclearForKerningOfAnotherSize(t *testing.T) {
	spec := rowsOf(t)
	// half of what the table says: neither unkerned nor the table
	bbox := page(t, spec, func(r pptxgen.KerningRow) bool { return false }, 0)
	pages, err := parseBBox(bbox)
	if err != nil {
		t.Fatal(err)
	}
	full, err := parseBBox(page(t, spec, func(r pptxgen.KerningRow) bool { return r.Twin != "" }, 0))
	if err != nil {
		t.Fatal(err)
	}
	for p := range pages {
		for w := range pages[p].Words {
			a, b := &pages[p].Words[w], full[p].Words[w]
			a.X1 = a.X0 + ((a.X1-a.X0)+(b.X1-b.X0))/2
		}
	}
	rows, err := measure(spec, pages, fontFlags{}, []string{"LiberationSans", "LiberationSans-Bold"})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Row.Twin != "" && r.Verdict != "unclear" {
			t.Errorf("row %s: %q, want unclear", r.Row.ID, r.Verdict)
		}
	}
}

func TestRowWordsReportsOtherText(t *testing.T) {
	spec := rowsOf(t)
	bbox := strings.Replace(string(page(t, spec, func(pptxgen.KerningRow) bool { return false }, 0)), ">noon<", ">moon<", 1)
	pages, err := parseBBox([]byte(bbox))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := measure(spec, pages, fontFlags{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rows[0].Err, `"moon"`) {
		t.Errorf("first row: error %q, want it to name the word the PDF has", rows[0].Err)
	}
}
