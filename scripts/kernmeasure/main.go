// Command kernmeasure finds out from a PDF that PowerPoint exported whether PowerPoint kerned
// the text of a kerning deck, and with which table of the font.
//
// Usage: go run ./scripts/kernmeasure -spec <deck>.spec.json -pdf <deck>.pdf
//
//	[-saved <deck saved by PowerPoint>.pptx] [-words <words.json>] [-out tests/.out/kerning]
//	[-font "Typeface=bundled:LiberationSerif"] [-font "Typeface=/path/to/font.ttf"]
//
// go run ./scripts/kernmeasure -substitutes compares instead, for the text of the public decks,
// the widths of the fonts PowerPoint kerns (Calibri, Arial, Helvetica, Times New Roman, where
// they are installed) with the widths of the bundled fonts deckfold draws in their place.
//
// The spec comes from go run ./scripts/pptxgen (fixtures/pptx/src/kerning.spec.json, or the
// spec of the local deck from -kernlocal). The PDF is the one scripts/pptxref/pptxref.sh leaves
// in tests/.out/pptxref. The word boxes of the PDF are read with pdftotext (poppler).
//
// For every word it compares the width in the PDF with three widths computed from the font:
// the advance widths alone, with the pairs of the kern table, and with the kerning of GPOS.
// PowerPoint's widths lie up to half a percent from the font's own numbers even without
// kerning, more for words with capitals than for the control words of the spec. So a row is
// not judged by its own widths but against its twin, the same line with kern="0": the
// difference of a word's width between the two is the kerning PowerPoint applied, and it is
// compared with what each table says (functions baseline and paired in measure.go).
//
// What it measures is the PDF export of PowerPoint for Mac, the source of all reference images.
// It cannot say whether the slide show sets text the same way.
package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"deckfold/internal/pptx/pptxgen"
)

type fontFlags map[string]string

func (f fontFlags) String() string { return fmt.Sprint(map[string]string(f)) }
func (f fontFlags) Set(v string) error {
	name, src, ok := strings.Cut(v, "=")
	if !ok {
		return errors.New(`want "Typeface=bundled:Name" or "Typeface=/path/to/font"`)
	}
	f[name] = src
	return nil
}

func main() {
	specPath := flag.String("spec", "", "spec of the kerning deck (JSON)")
	pdfPath := flag.String("pdf", "", "PDF that PowerPoint exported from the deck")
	saved := flag.String("saved", "", "the deck as PowerPoint saved it: report the kern attributes it kept")
	wordsOut := flag.String("words", "", "write PowerPoint's word positions to this JSON file (needs -saved)")
	out := flag.String("out", filepath.Join("tests", ".out", "kerning"), "folder for the tables")
	subs := flag.Bool("substitutes", false, "compare the bundled substitutes with the installed originals instead")
	overrides := fontFlags{}
	flag.Var(overrides, "font", "font file for a typeface of the spec (repeatable)")
	flag.Parse()
	if *subs {
		if err := substitutes(filepath.Join("fixtures", "pptx", "src"), *out); err != nil {
			fmt.Fprintln(os.Stderr, "kernmeasure:", err)
			os.Exit(1)
		}
		return
	}
	if *specPath == "" || *pdfPath == "" {
		flag.Usage()
		os.Exit(3)
	}
	if err := run(*specPath, *pdfPath, *saved, *wordsOut, *out, overrides); err != nil {
		fmt.Fprintln(os.Stderr, "kernmeasure:", err)
		os.Exit(1)
	}
}

func run(specPath, pdfPath, saved, wordsOut, out string, overrides fontFlags) error {
	var spec pptxgen.KerningSpec
	js, err := os.ReadFile(specPath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(js, &spec); err != nil {
		return fmt.Errorf("%s: %w", specPath, err)
	}
	bbox, err := exec.Command("pdftotext", "-bbox", pdfPath, "-").Output()
	if err != nil {
		return fmt.Errorf("pdftotext (poppler) on %s: %w", pdfPath, err)
	}
	pages, err := parseBBox(bbox)
	if err != nil {
		return err
	}
	drawn := pdfFonts(pdfPath)
	rows, err := measure(spec, pages, overrides, drawn)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	name := strings.TrimSuffix(filepath.Base(pdfPath), filepath.Ext(pdfPath))
	if err := os.WriteFile(filepath.Join(out, name+"-words.tsv"), []byte(wordTable(rows)), 0o644); err != nil {
		return err
	}
	summary := rowTable(rows)
	if err := os.WriteFile(filepath.Join(out, name+"-rows.tsv"), []byte(summary), 0o644); err != nil {
		return err
	}
	fmt.Print(summary)
	fmt.Println("\nfonts in the PDF:", strings.Join(drawn, ", "))
	if saved != "" {
		report, err := savedKern(saved)
		if err != nil {
			return err
		}
		fmt.Print("\nkern in the copy PowerPoint saved:\n" + report)
	}
	if wordsOut != "" {
		if saved == "" {
			return errors.New("-words needs -saved, for the hash of the copy the positions belong to")
		}
		return writeWords(wordsOut, saved, rows)
	}
	return nil
}

// pdfFonts returns the PostScript names of the fonts in the PDF (without the subset prefix).
func pdfFonts(pdf string) []string {
	out, err := exec.Command("pdffonts", pdf).Output()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var names []string
	for i, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if i < 2 || len(f) == 0 {
			continue
		}
		n := f[0]
		if _, rest, ok := strings.Cut(n, "+"); ok {
			n = rest
		}
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names
}

// savedKern lists the kern attributes of every text box of a deck, and of its text styles.
func savedKern(path string) (string, error) {
	z, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer z.Close()
	var b strings.Builder
	var names []string
	files := map[string]*zip.File{}
	for _, f := range z.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") || strings.HasPrefix(f.Name, "ppt/slideMasters/") || f.Name == "ppt/presentation.xml" {
			names = append(names, f.Name)
			files[f.Name] = f
		}
	}
	sort.Slice(names, func(i, j int) bool {
		if len(names[i]) != len(names[j]) {
			return len(names[i]) < len(names[j])
		}
		return names[i] < names[j]
	})
	for _, n := range names {
		r, err := files[n].Open()
		if err != nil {
			return "", err
		}
		shapes, err := kernAttrs(r)
		r.Close()
		if err != nil {
			return "", fmt.Errorf("%s: %w", n, err)
		}
		for _, s := range shapes {
			fmt.Fprintf(&b, "  %-32s %-28s %s\n", n, s.name, s.kern)
		}
	}
	return b.String(), nil
}

type shapeKern struct{ name, kern string }

// kernAttrs reads one XML part: for each shape (and for the part outside shapes) the kern values
// on run properties and on the default run properties of list and text styles.
func kernAttrs(r io.Reader) ([]shapeKern, error) {
	dec := xml.NewDecoder(r)
	var out []shapeKern
	cur := "(styles)"
	vals := map[string][]string{}
	order := []string{cur}
	add := func(v string) {
		if _, ok := vals[cur]; !ok && cur != "(styles)" {
			order = append(order, cur)
		}
		vals[cur] = append(vals[cur], v)
	}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch se.Name.Local {
		case "cNvPr":
			for _, a := range se.Attr {
				if a.Name.Local == "name" {
					cur = a.Value
				}
			}
		case "rPr", "defRPr", "endParaRPr":
			v := "-"
			for _, a := range se.Attr {
				if a.Name.Local == "kern" {
					v = a.Value
				}
			}
			add(se.Name.Local + "=" + v)
		}
	}
	for _, n := range order {
		counts := map[string]int{}
		var keys []string
		for _, v := range vals[n] {
			if counts[v] == 0 {
				keys = append(keys, v)
			}
			counts[v]++
		}
		var parts []string
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s (%d)", k, counts[k]))
		}
		if len(parts) > 0 {
			out = append(out, shapeKern{n, strings.Join(parts, ", ")})
		}
	}
	return out, nil
}

// wordsFile is what the tests of the converter compare its own word positions with.
type wordsFile struct {
	Source string     `json:"source"` // SHA-256 of the deck PowerPoint saved, as in source.sha256
	Note   string     `json:"note"`
	Rows   []wordsRow `json:"rows"`
}

type wordsRow struct {
	ID    string      `json:"id"`
	Slide int         `json:"slide"`
	Font  string      `json:"font"`
	Words []wordPlace `json:"words"`
}

type wordPlace struct {
	Text  string  `json:"text"`
	Line  int     `json:"line"`  // line of the paragraph, from 0
	X     float64 `json:"x"`     // left edge in pt from the left edge of the slide
	Width float64 `json:"width"` // in pt
}

func writeWords(path, saved string, rows []rowResult) error {
	data, err := os.ReadFile(saved)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	wf := wordsFile{Source: hex.EncodeToString(sum[:]),
		Note: "Word positions in the PDF PowerPoint exported from the kerning deck, written by scripts/kernmeasure. Regenerate after scripts/pptxref/pptxref.sh kerning."}
	for _, r := range rows {
		wr := wordsRow{ID: r.Row.ID, Slide: r.Row.Slide, Font: r.Row.Font}
		for _, w := range r.Words {
			wr.Words = append(wr.Words, wordPlace{w.Text, w.Line, round(w.X, 3), round(w.Width, 3)})
		}
		wf.Rows = append(wf.Rows, wr)
	}
	js, err := json.MarshalIndent(wf, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(js, '\n'), 0o644)
}

func round(v float64, digits int) float64 {
	p := 1.0
	for i := 0; i < digits; i++ {
		p *= 10
	}
	if v < 0 {
		return float64(int64(v*p-0.5)) / p
	}
	return float64(int64(v*p+0.5)) / p
}
