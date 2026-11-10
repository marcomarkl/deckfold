package main

import (
	"fmt"
	"html"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"deckfold/internal/pptx/kernprobe"
	"deckfold/internal/pptx/pptxgen"
)

const pxToPt = 0.75 // the spec has px at 96 dpi, the PDF has points

type pdfWord struct {
	Text           string
	X0, Y0, X1, Y1 float64
}

type pdfPage struct {
	W, H  float64
	Words []pdfWord
}

var (
	pageRe = regexp.MustCompile(`<page width="([\d.]+)" height="([\d.]+)">`)
	wordRe = regexp.MustCompile(`<word xMin="([-\d.]+)" yMin="([-\d.]+)" xMax="([-\d.]+)" yMax="([-\d.]+)">(.*?)</word>`)
)

// parseBBox reads the output of pdftotext -bbox.
func parseBBox(b []byte) ([]pdfPage, error) {
	var pages []pdfPage
	for _, line := range strings.Split(string(b), "\n") {
		if m := pageRe.FindStringSubmatch(line); m != nil {
			w, _ := strconv.ParseFloat(m[1], 64)
			h, _ := strconv.ParseFloat(m[2], 64)
			pages = append(pages, pdfPage{W: w, H: h})
			continue
		}
		if m := wordRe.FindStringSubmatch(line); m != nil && len(pages) > 0 {
			var v [4]float64
			for i := range v {
				v[i], _ = strconv.ParseFloat(m[i+1], 64)
			}
			p := &pages[len(pages)-1]
			p.Words = append(p.Words, pdfWord{html.UnescapeString(m[5]), v[0], v[1], v[2], v[3]})
		}
	}
	if len(pages) == 0 {
		return nil, fmt.Errorf("no pages in the output of pdftotext")
	}
	return pages, nil
}

// wordResult is one word of a row: where PowerPoint put it and what the font says.
type wordResult struct {
	Text  string
	Kind  string  // "kerned", "control", "differ" or ""
	Line  int     // line of the paragraph, from 0
	X     float64 // pt
	Width float64 // pt, in the PDF

	chars            int
	plainEm          float64 // advance widths
	kernEm, gposEm   float64 // kerning inside the word
	kernRun, gposRun float64 // the part of it that lies across a run boundary
	// model widths in pt and the PDF's distance from them, relative to the plain width
	Plain, Kern, GPOS    float64
	RPlain, RKern, RGPOS float64
	RKernRun, RGPOSRun   float64 // against the models without the pairs across runs
	// Delta is the width in this row minus the width in its twin with kern="0", in pt: the
	// kerning PowerPoint applied to the word (only for a row with a twin).
	Delta float64
}

type gapResult struct {
	Between           string  // last and first letter around the space
	PDF               float64 // pt
	Plain, Kern, GPOS float64 // models in pt
}

type rowResult struct {
	Row       pptxgen.KerningRow
	Face      *kernprobe.Font
	FaceNote  string
	Drawn     bool // the PDF lists the PostScript name of the face
	Size      float64
	SizeFrom  string
	Scatter   float64 // of all control words in this font family, against the plain model
	Own       float64 // of the control words of this row alone
	Twin      int     // index of the row with kern="0" this row is compared with, or -1
	Noise     float64 // pt: how far control words differ between a row and its twin, in this family
	ShareKern string  // the kerning applied, as a share of what the kern table says
	ShareGPOS string  // and of what GPOS says
	Words     []wordResult
	Gaps      []gapResult
	Lines     int
	LineStart map[string][]string // first word of every line: in the PDF and by each model
	Verdict   string
	Err       string
}

var bundledNames = map[string]string{"Liberation Sans": "LiberationSans", "Liberation Serif": "LiberationSerif", "Liberation Mono": "LiberationMono", "Carlito": "Carlito"}

// candidates returns the font files a row can have been drawn with.
func candidates(family string, bold bool, overrides fontFlags) ([]*kernprobe.Font, error) {
	style := "Regular"
	if bold {
		style = "Bold"
	}
	if src, ok := overrides[family]; ok {
		if name, ok := strings.CutPrefix(src, "bundled:"); ok {
			f, err := kernprobe.Bundled(name, bold)
			return []*kernprobe.Font{f}, err
		}
		f, err := kernprobe.Open(src, 0)
		return []*kernprobe.Font{f}, err
	}
	if name, ok := bundledNames[family]; ok {
		f, err := kernprobe.Bundled(name, bold)
		return []*kernprobe.Font{f}, err
	}
	fs := kernprobe.Find(family, style)
	if len(fs) == 0 {
		return nil, fmt.Errorf("%s %s is not installed", family, style)
	}
	return fs, nil
}

// rowWords finds the words of a row on its page: everything between the top of its box and the
// top of the next box on the slide, in reading order, matched against the text of the spec.
func rowWords(row pptxgen.KerningRow, rows []pptxgen.KerningRow, pages []pdfPage) ([]wordResult, int, error) {
	if row.Slide < 1 || row.Slide > len(pages) {
		return nil, 0, fmt.Errorf("the PDF has no page %d", row.Slide)
	}
	page := pages[row.Slide-1]
	top := float64(row.Box[1]) * pxToPt
	bottom := page.H
	for _, o := range rows {
		if y := float64(o.Box[1]) * pxToPt; o.Slide == row.Slide && y > top && y < bottom {
			bottom = y
		}
	}
	var ws []pdfWord
	for _, w := range page.Words {
		if c := (w.Y0 + w.Y1) / 2; c >= top && c < bottom {
			ws = append(ws, w)
		}
	}
	sort.SliceStable(ws, func(i, j int) bool { return ws[i].Y0 < ws[j].Y0 })
	line := make([]int, len(ws))
	lines := 0
	for i := range ws {
		if i > 0 && ws[i].Y0-ws[i-1].Y0 > 0.5*(ws[i].Y1-ws[i].Y0) {
			lines++
		}
		line[i] = lines
	}
	idx := make([]int, len(ws))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		if line[idx[a]] != line[idx[b]] {
			return line[idx[a]] < line[idx[b]]
		}
		return ws[idx[a]].X0 < ws[idx[b]].X0
	})
	want := strings.Fields(strings.Join(row.Runs, ""))
	var out []wordResult
	acc, first := "", -1
	for _, i := range idx {
		if len(out) == len(want) {
			return nil, 0, fmt.Errorf("more text than the spec has: %q", ws[i].Text)
		}
		// pdftotext can cut a word where the color changes; put the pieces together again
		if first < 0 {
			first = i
		}
		acc += ws[i].Text
		w := want[len(out)]
		if acc == w {
			out = append(out, wordResult{Text: w, Line: line[first], X: ws[first].X0, Width: ws[i].X1 - ws[first].X0, chars: len([]rune(w))})
			acc, first = "", -1
		} else if !strings.HasPrefix(w, acc) {
			return nil, 0, fmt.Errorf("the PDF has %q where the spec has %q", acc, w)
		}
	}
	if len(out) != len(want) {
		return nil, 0, fmt.Errorf("%d of %d words found", len(out), len(want))
	}
	return out, lines + 1, nil
}

// fill computes what face says about the words of a row (in em) and the kinds of the words.
func fill(r *rowResult, face *kernprobe.Font) error {
	text := strings.Join(r.Row.Runs, "")
	kind := map[string]string{}
	for _, w := range r.Row.Control {
		kind[w] = "control"
	}
	for _, w := range r.Row.Kerned {
		kind[w] = "kerned"
	}
	for _, w := range r.Row.Differ {
		kind[w] = "differ"
	}
	// positions in the text where a run ends and the next begins
	cut := map[int]bool{}
	n := 0
	for _, run := range r.Row.Runs[:len(r.Row.Runs)-1] {
		n += len([]rune(run))
		cut[n] = true
	}
	rs := []rune(text)
	// texts CoreText is asked about: the words, the pairs across runs, the letters around spaces
	ask := map[string]bool{}
	pos := 0
	type span struct{ from, to int }
	var spans []span
	for i := range r.Words {
		for pos < len(rs) && rs[pos] == ' ' {
			pos++
		}
		spans = append(spans, span{pos, pos + r.Words[i].chars})
		ask[r.Words[i].Text] = true
		for p := pos + 1; p < pos+r.Words[i].chars; p++ {
			if cut[p] {
				ask[string(rs[p-1:p+1])] = true
			}
		}
		if i > 0 {
			ask[string([]rune{rs[spans[i-1].to-1], ' ', rs[pos]})] = true
		}
		pos += r.Words[i].chars
	}
	if face.Path != "" && face.GPOSKern && kernprobe.CoreTextAvailable() {
		var texts []string
		for t := range ask {
			texts = append(texts, t)
		}
		sort.Strings(texts)
		if err := face.CoreText(texts); err != nil {
			return err
		}
	}
	for i := range r.Words {
		w := &r.Words[i]
		if !face.Has(w.Text) {
			return fmt.Errorf("%s has no glyph for a letter of %q", face.PostScript, w.Text)
		}
		w.Kind = kind[w.Text]
		w.plainEm, w.kernEm, w.gposEm = face.Plain(w.Text), face.Kern(w.Text), face.GPOS(w.Text)
		w.kernRun, w.gposRun = 0, 0
		for p := spans[i].from + 1; p < spans[i].to; p++ {
			if cut[p] {
				w.kernRun += face.Pair(rs[p-1], rs[p])
				w.gposRun += face.GPOS(string(rs[p-1 : p+1]))
			}
		}
	}
	return nil
}

// fit computes the font size from the control words and the distance of every word from the
// models. It returns the scatter of the control words of this row.
func fit(r *rowResult, face *kernprobe.Font) float64 {
	spc := r.Row.SpcPt
	sumW, sumEm := 0.0, 0.0
	for _, w := range r.Words {
		if w.Kind == "control" {
			sumW += w.Width - spc*float64(w.chars-1)
			sumEm += w.plainEm
		}
	}
	r.Size, r.SizeFrom = r.Row.SizePt, "nominal"
	if r.Row.FontScale > 0 {
		r.Size *= float64(r.Row.FontScale) / 100
	}
	if sumEm > 0 {
		r.Size, r.SizeFrom = sumW/sumEm, "control words"
	}
	scatter := 0.0
	for i := range r.Words {
		w := &r.Words[i]
		w.Plain = w.plainEm*r.Size + spc*float64(w.chars-1)
		w.Kern, w.GPOS = w.Plain+w.kernEm*r.Size, w.Plain+w.gposEm*r.Size
		w.RPlain, w.RKern, w.RGPOS = (w.Width-w.Plain)/w.Plain, (w.Width-w.Kern)/w.Plain, (w.Width-w.GPOS)/w.Plain
		w.RKernRun = (w.Width - (w.Kern - w.kernRun*r.Size)) / w.Plain
		w.RGPOSRun = (w.Width - (w.GPOS - w.gposRun*r.Size)) / w.Plain
		if w.Kind == "control" {
			scatter = math.Max(scatter, math.Abs(w.RPlain))
		}
	}
	// the gaps between the words of a line
	r.Gaps = nil
	space := face.Plain(" ")
	for i := 1; i < len(r.Words); i++ {
		a, b := r.Words[i-1], r.Words[i]
		if a.Line != b.Line {
			continue
		}
		ar, br := []rune(a.Text), []rune(b.Text)
		last, first := ar[len(ar)-1], br[0]
		plain := space*r.Size + 2*spc
		r.Gaps = append(r.Gaps, gapResult{Between: string([]rune{last, ' ', first}), PDF: b.X - (a.X + a.Width), Plain: plain,
			Kern: plain + (face.Pair(last, ' ')+face.Pair(' ', first))*r.Size,
			GPOS: plain + face.GPOS(string([]rune{last, ' ', first}))*r.Size})
	}
	return scatter
}

func kernedWords(r *rowResult) []wordResult {
	var out []wordResult
	for _, w := range r.Words {
		if w.Kind == "kerned" || w.Kind == "differ" {
			out = append(out, w)
		}
	}
	return out
}

const kernOff = "kern off: plain"

// baseline judges a row with kern="0", the twin other rows are compared with: it has to be
// unkerned itself (PowerPoint could ignore the attribute). The widths of its kerned words are
// summed, because PowerPoint rounds the width of a word by up to a percent at small sizes, in
// both directions. The row is plain when that sum is at least five times nearer to the sum of
// the advance widths than to the sum with the kerning of a table. A table whose kerning is
// less than 1 % of the width of these words cannot be told from no kerning and is left out.
func baseline(r *rowResult) string {
	kerned := kernedWords(r)
	if len(kerned) == 0 {
		return "no kerned words: nothing to tell"
	}
	var pdf, plain, kern, gpos float64
	for _, w := range kerned {
		pdf, plain, kern, gpos = pdf+w.Width, plain+w.Plain, kern+w.Kern, gpos+w.GPOS
	}
	nearest := math.Inf(1)
	for _, model := range []float64{kern, gpos} {
		if math.Abs(model-plain) < 0.01*plain {
			continue
		}
		nearest = math.Min(nearest, math.Abs(pdf-model)/math.Max(math.Abs(pdf-plain), 1e-9))
	}
	switch {
	case math.IsInf(nearest, 1):
		return "no kerned words: nothing to tell"
	case nearest < 5:
		return fmt.Sprintf("unclear: with kern off only %.1f times nearer to plain than to a kerned width", nearest)
	case nearest > 999:
		return kernOff
	}
	return fmt.Sprintf("%s (%.0f times nearer than to a kerned width)", kernOff, nearest)
}

// minNoise is the smallest noise a verdict works with, in pt: the PDF has its positions to a
// few thousandths of a point.
const minNoise = 0.01

// paired judges a row against its twin. Delta, the difference of a word's width between the
// two, is the kerning PowerPoint applied. The row is not kerned when no kerned word differs
// from its twin by more than three times the noise (the largest difference of a control word
// between a row and its twin in this font family). It is kerned by a table when every kerned
// word differs by what that table says, within three times the noise or 2 % of the value,
// whichever is larger.
func paired(r *rowResult, twin *rowResult) string {
	kerned := kernedWords(r)
	if len(kerned) == 0 {
		worst := 0.0
		for _, w := range r.Words {
			worst = math.Max(worst, math.Abs(w.Delta))
		}
		return fmt.Sprintf("no kerning data in the font; no word differs from the twin by more than %.3f pt", worst)
	}
	tol := 3 * math.Max(r.Noise, minNoise)
	type model struct {
		name string
		pt   func(w wordResult) float64
	}
	acrossRuns := len(r.Row.Runs) > 1
	var models []model
	add := func(name string, on bool, pt func(w wordResult) float64) {
		if !on {
			return
		}
		for _, w := range kerned {
			if pt(w) != 0 {
				models = append(models, model{name, pt})
				return
			}
		}
	}
	add("kern table", true, func(w wordResult) float64 { return w.kernEm * r.Size })
	add("GPOS", true, func(w wordResult) float64 { return w.gposEm * r.Size })
	add("kern table but not across runs", acrossRuns, func(w wordResult) float64 { return (w.kernEm - w.kernRun) * r.Size })
	add("GPOS but not across runs", acrossRuns, func(w wordResult) float64 { return (w.gposEm - w.gposRun) * r.Size })
	if len(models) == 0 {
		// a face without pairs for these words (the bold of a family whose regular has them)
		worst := 0.0
		for _, w := range r.Words {
			worst = math.Max(worst, math.Abs(w.Delta))
		}
		return fmt.Sprintf("no kerning data for these words in the font; no word differs from the twin by more than %.3f pt", worst)
	}
	if !strings.HasPrefix(twin.Verdict, kernOff) {
		return "unclear: its twin with kern off is not plainly unkerned"
	}
	share := func(m model) float64 {
		d, v := 0.0, 0.0
		for _, w := range kerned {
			d += w.Delta
			v += m.pt(w)
		}
		return d / v
	}
	unkerned := true
	for _, w := range kerned {
		if math.Abs(w.Delta) > tol {
			unkerned = false
		}
	}
	var fits []string
	for _, m := range models {
		sh := fmt.Sprintf("%.1f %%", 100*share(m))
		switch m.name {
		case "kern table":
			r.ShareKern = sh
		case "GPOS":
			r.ShareGPOS = sh
		}
		ok := true
		for _, w := range kerned {
			if v := m.pt(w); math.Abs(w.Delta-v) > math.Max(tol, 0.02*math.Abs(v)) {
				ok = false
			}
		}
		if ok {
			fits = append(fits, m.name)
		}
	}
	switch {
	case unkerned && len(fits) == 0:
		return "not kerned"
	case !unkerned && len(fits) > 0:
		return "kerned: " + strings.Join(fits, " = ")
	}
	return "unclear"
}

// lineStarts returns the first word of every line: as the PDF has them, and as a greedy line
// break with the widths of each model would put them into the box.
func lineStarts(r *rowResult, face *kernprobe.Font) map[string][]string {
	out := map[string][]string{}
	for i, w := range r.Words {
		if i == 0 || w.Line != r.Words[i-1].Line {
			out["PDF"] = append(out["PDF"], w.Text)
		}
	}
	avail := float64(r.Row.Box[2]) * pxToPt
	space := face.Plain(" ")
	for name, m := range map[string]struct {
		word func(w wordResult) float64
		gap  func(a, b rune) float64
	}{
		"plain":      {func(w wordResult) float64 { return w.Plain }, func(a, b rune) float64 { return space * r.Size }},
		"kern table": {func(w wordResult) float64 { return w.Kern }, func(a, b rune) float64 { return (space + face.Pair(a, ' ') + face.Pair(' ', b)) * r.Size }},
		"GPOS":       {func(w wordResult) float64 { return w.GPOS }, func(a, b rune) float64 { return (space + face.GPOS(string([]rune{a, ' ', b}))) * r.Size }},
	} {
		x := 0.0
		for i, w := range r.Words {
			width := m.word(w)
			if i > 0 {
				ar := []rune(r.Words[i-1].Text)
				g := m.gap(ar[len(ar)-1], []rune(w.Text)[0])
				if x+g+width <= avail+0.5 {
					x += g + width
					continue
				}
			}
			out[name] = append(out[name], w.Text)
			x = width
		}
	}
	return out
}

// measure matches the rows of the spec with the words of the PDF and judges every row.
func measure(spec pptxgen.KerningSpec, pages []pdfPage, overrides fontFlags, drawn []string) ([]rowResult, error) {
	inPDF := map[string]bool{}
	for _, n := range drawn {
		inPDF[n] = true
	}
	rows := make([]rowResult, len(spec.Rows))
	type key struct {
		font string
		bold bool
	}
	byFont := map[key][]int{}
	var order []key
	for i, row := range spec.Rows {
		rows[i].Row = row
		ws, lines, err := rowWords(row, spec.Rows, pages)
		if err != nil {
			rows[i].Err = err.Error()
			continue
		}
		rows[i].Words, rows[i].Lines = ws, lines
		k := key{row.Font, row.Bold}
		if _, ok := byFont[k]; !ok {
			order = append(order, k)
		}
		byFont[k] = append(byFont[k], i)
	}
	for _, k := range order {
		cands, err := candidates(k.font, k.bold, overrides)
		if err != nil {
			for _, i := range byFont[k] {
				rows[i].Err = err.Error()
			}
			continue
		}
		// the file whose advance widths fit the control words best is the one PowerPoint drew
		best, bestScatter, note := -1, math.Inf(1), ""
		var scatters []string
		for c, face := range cands {
			worst, failed := 0.0, error(nil)
			for _, i := range byFont[k] {
				if failed = fill(&rows[i], face); failed != nil {
					break
				}
				worst = math.Max(worst, fit(&rows[i], face))
			}
			if failed != nil {
				scatters = append(scatters, face.Path+": "+failed.Error())
				continue
			}
			scatters = append(scatters, fmt.Sprintf("%s: %.3f %%", face.Path, 100*worst))
			if worst < bestScatter {
				best, bestScatter = c, worst
			}
		}
		if best < 0 {
			for _, i := range byFont[k] {
				rows[i].Err = strings.Join(scatters, "; ")
			}
			continue
		}
		if len(cands) > 1 {
			note = fmt.Sprintf("%d files, scatter of the control words: %s", len(cands), strings.Join(scatters, "; "))
		}
		face := cands[best]
		for _, i := range byFont[k] {
			if err := fill(&rows[i], face); err != nil {
				rows[i].Err = err.Error()
				continue
			}
			rows[i].Own = fit(&rows[i], face)
			rows[i].Face, rows[i].FaceNote, rows[i].Drawn = face, note, inPDF[face.PostScript]
		}
	}
	// The scatter a verdict works with is the largest distance of a control word from the plain
	// model in the whole family, regular and bold, all sizes: a row has three control words at
	// most, too few to know how far PowerPoint's widths lie from the font's numbers.
	pooled := map[string]float64{}
	for _, r := range rows {
		if r.Err == "" {
			pooled[r.Row.Font] = math.Max(pooled[r.Row.Font], r.Own)
		}
	}
	// twins: the difference of every word to the row with kern="0", and the noise of that
	// difference from the control words
	index := map[[2]string]int{}
	for i, r := range rows {
		index[[2]string{r.Row.Font, r.Row.ID}] = i
	}
	noise := map[string]float64{}
	for i := range rows {
		r := &rows[i]
		r.Twin = -1
		if r.Err != "" || r.Row.Twin == "" {
			continue
		}
		j, ok := index[[2]string{r.Row.Font, r.Row.Twin}]
		if !ok || rows[j].Err != "" || len(rows[j].Words) != len(r.Words) {
			r.Err = "its twin " + r.Row.Twin + " was not measured"
			continue
		}
		r.Twin = j
		for k := range r.Words {
			r.Words[k].Delta = r.Words[k].Width - rows[j].Words[k].Width
			if r.Words[k].Kind == "control" {
				noise[r.Row.Font] = math.Max(noise[r.Row.Font], math.Abs(r.Words[k].Delta))
			}
		}
	}
	// baselines first, then the rows that are compared with them
	for pass := 0; pass < 2; pass++ {
		for i := range rows {
			r := &rows[i]
			if r.Err != "" || (r.Twin >= 0) != (pass == 1) {
				continue
			}
			r.Scatter, r.Noise = pooled[r.Row.Font], noise[r.Row.Font]
			if r.Twin >= 0 {
				r.Verdict = paired(r, &rows[r.Twin])
			} else {
				r.Verdict = baseline(r)
			}
			if !r.Drawn {
				r.Verdict = "font not in the PDF (" + r.Face.PostScript + "); " + r.Verdict
			}
			if r.Lines > 1 {
				r.LineStart = lineStarts(r, r.Face)
			}
		}
	}
	return rows, nil
}

func pct(v float64) string { return fmt.Sprintf("%+.2f", 100*v) }

// wordTable lists every word with its models.
func wordTable(rows []rowResult) string {
	var b strings.Builder
	b.WriteString("slide\tfont\tstyle\trow\tword\tkind\tline\tx pt\twidth pt\tplain pt\tkern table pt\tGPOS pt\tvs plain %\tvs kern table %\tvs GPOS %\tminus twin pt\tkern table says pt\tGPOS says pt\n")
	for _, r := range rows {
		for _, w := range r.Words {
			delta := ""
			if r.Twin >= 0 {
				delta = fmt.Sprintf("%+.3f", w.Delta)
			}
			fmt.Fprintf(&b, "%d\t%s\t%s\t%s\t%s\t%s\t%d\t%.3f\t%.3f\t%.3f\t%.3f\t%.3f\t%s\t%s\t%s\t%s\t%+.3f\t%+.3f\n", r.Row.Slide, r.Row.Font, style(r.Row.Bold), r.Row.ID,
				w.Text, w.Kind, w.Line, w.X, w.Width, w.Plain, w.Kern, w.GPOS, pct(w.RPlain), pct(w.RKern), pct(w.RGPOS), delta, w.kernEm*r.Size, w.gposEm*r.Size)
		}
	}
	return b.String()
}

func style(bold bool) string {
	if bold {
		return "Bold"
	}
	return "Regular"
}

// rowTable has one line per row: the size found, the scatter, how far the kerned words are from
// each model at most, what the gaps with a pair say, and the verdict.
func rowTable(rows []rowResult) string {
	var b strings.Builder
	b.WriteString("slide\tfont\trow\tkern\tsize pt\tscatter % (row / family)\tkerned words: worst vs plain %\tvs kern table %\tvs GPOS %\tgaps with a pair\tnoise pt\tapplied of kern table\tapplied of GPOS\tverdict\n")
	notes := map[string]bool{}
	for _, r := range rows {
		if r.Err != "" {
			fmt.Fprintf(&b, "%d\t%s\t%s\t%s\t\t\t\t\t\t\t\t\t\tnot measured: %s\n", r.Row.Slide, r.Row.Font, r.Row.ID, r.Row.Kern, r.Err)
			continue
		}
		worst := func(v func(w wordResult) float64, has func(w wordResult) float64) string {
			m, any := 0.0, false
			for _, w := range r.Words {
				if w.Kind == "kerned" || w.Kind == "differ" {
					if has != nil && has(w) != 0 {
						any = true
					}
					if a := math.Abs(v(w)); a > math.Abs(m) {
						m = v(w)
					}
				}
			}
			if has != nil && !any {
				return "-"
			}
			return pct(m)
		}
		noise, dash := "-", func(v string) string {
			if v == "" {
				return "-"
			}
			return v
		}
		if r.Twin >= 0 {
			noise = fmt.Sprintf("%.3f", r.Noise)
		}
		fmt.Fprintf(&b, "%d\t%s\t%s\t%s\t%.2f of %.4g\t%.2f / %.2f\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.Row.Slide, r.Row.Font, r.Row.ID, r.Row.Kern, r.Size, r.Row.SizePt, 100*r.Own, 100*r.Scatter,
			worst(func(w wordResult) float64 { return w.RPlain }, nil),
			worst(func(w wordResult) float64 { return w.RKern }, func(w wordResult) float64 { return w.kernEm }),
			worst(func(w wordResult) float64 { return w.RGPOS }, func(w wordResult) float64 { return w.gposEm }),
			gapNote(r), noise, dash(r.ShareKern), dash(r.ShareGPOS), r.Verdict)
		if r.LineStart != nil {
			for _, m := range []string{"PDF", "plain", "kern table", "GPOS"} {
				fmt.Fprintf(&b, "\t\t\t\tlines start (%s): %s\n", m, strings.Join(r.LineStart[m], " "))
			}
		}
		if r.FaceNote != "" && !notes[r.FaceNote] {
			notes[r.FaceNote] = true
		}
	}
	var ns []string
	for n := range notes {
		ns = append(ns, n)
	}
	sort.Strings(ns)
	for _, n := range ns {
		b.WriteString("note: " + n + "\n")
	}
	return b.String()
}

// gapNote says, for the gaps whose letters have a pair with the space, which model they are
// nearer to.
func gapNote(r rowResult) string {
	n, kern, gpos, plain := 0, 0, 0, 0
	for _, g := range r.Gaps {
		dk, dg := math.Abs(g.Kern-g.Plain), math.Abs(g.GPOS-g.Plain)
		if dk < 0.02 && dg < 0.02 {
			continue
		}
		n++
		best, name := math.Abs(g.PDF-g.Plain), "plain"
		if dk >= 0.02 && math.Abs(g.PDF-g.Kern) < best {
			best, name = math.Abs(g.PDF-g.Kern), "kern"
		}
		if dg >= 0.02 && math.Abs(g.PDF-g.GPOS) < best {
			name = "gpos"
		}
		switch name {
		case "kern":
			kern++
		case "gpos":
			gpos++
		default:
			plain++
		}
	}
	if n == 0 {
		return "-"
	}
	return fmt.Sprintf("%d: plain %d, kern table %d, GPOS %d", n, plain, kern, gpos)
}
