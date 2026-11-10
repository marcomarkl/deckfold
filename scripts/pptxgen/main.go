// Command pptxgen writes the public PowerPoint feature decks to fixtures/pptx/src.
//
// Usage: go run ./scripts/pptxgen [-out fixtures/pptx/src] [-big] [-ppbase folder] [-kernlocal folder] [-baselinelocal folder]
//
// The decks use neutral English content and only the fonts deckfold bundles. scripts/pptxref
// opens each of them in PowerPoint, saves it again and exports one reference PNG per slide;
// the tests read the PowerPoint-saved copies in fixtures/pptx.
// -big also writes big.pptx (200 slides) for the performance test; it is not committed.
// -kernlocal writes kerning-local.pptx, the kerning deck for fonts installed on this machine
// (Office, Google and system fonts), with its spec; it stays in a git-ignored folder.
// -baselinelocal writes baseline.pptx, the deck that measures where PowerPoint puts the
// baselines under different line spacings and anchors, with its spec; local as well.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"deckfold/internal/pptx/kernprobe"
	"deckfold/internal/pptx/pptxgen"
)

// kernLocalFamilies are the fonts of the local kerning deck: with a kern table (Calibri, Arial,
// Aptos, Verdana, Gill Sans MT, Helvetica Neue), with an Apple kern table (Helvetica), with
// kerning in GPOS only (Lato, Roboto, Montserrat, DM Sans, Nunito Sans) and without any kerning
// (Century Gothic, Poppins). There is no font with CFF outlines among them: PowerPoint for Mac
// drew STIXGeneral, the one that was at hand, in Calibri (2026-11-10).
var kernLocalFamilies = []string{
	"Calibri", "Arial", "Aptos", "Verdana", "Gill Sans MT", "Helvetica Neue", "Helvetica", "Century Gothic",
	"Lato", "Roboto", "Montserrat", "DM Sans", "Nunito Sans", "Poppins",
}

func main() {
	out := flag.String("out", filepath.Join("fixtures", "pptx", "src"), "output folder")
	big := flag.Bool("big", false, "also write big.pptx with 200 slides")
	ppbase := flag.String("ppbase", "", "only write the base decks for scripts/pptxref/ppdecks.sh into this folder")
	kernlocal := flag.String("kernlocal", "", "only write the kerning deck for fonts installed on this machine into this folder")
	baselinelocal := flag.String("baselinelocal", "", "only write the deck that measures PowerPoint's baselines into this folder")
	flag.Parse()
	if *baselinelocal != "" {
		if err := os.MkdirAll(*baselinelocal, 0o755); err != nil {
			fail(err)
		}
		d, spec := pptxgen.Baselines("Liberation Sans", "Carlito")
		js, _ := json.MarshalIndent(spec, "", "  ")
		if err := os.WriteFile(filepath.Join(*baselinelocal, "baseline.spec.json"), append(js, '\n'), 0o644); err != nil {
			fail(err)
		}
		if err := d.Write(filepath.Join(*baselinelocal, "baseline.pptx")); err != nil {
			fail(err)
		}
		fmt.Println("wrote", filepath.Join(*baselinelocal, "baseline.pptx"))
		return
	}
	if *kernlocal != "" {
		if err := writeKerningLocal(*kernlocal); err != nil {
			fail(err)
		}
		return
	}
	if *ppbase != "" {
		for name, d := range map[string]*pptxgen.Deck{"animations": pptxgen.PPAnimationsBase(), "transitions": pptxgen.PPTransitionsBase()} {
			if err := d.Write(filepath.Join(*ppbase, name+"-base.pptx")); err != nil {
				fail(err)
			}
		}
		return
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fail(err)
	}
	decks := pptxgen.Fixtures()
	if *big {
		decks["big"] = pptxgen.Big(200)
	}
	names := make([]string, 0, len(decks))
	for n := range decks {
		names = append(names, n)
	}
	sort.Strings(names)
	_, spec := pptxgen.AnimationsWithSpec()
	js, _ := json.MarshalIndent(spec, "", "  ")
	if err := os.WriteFile(filepath.Join(*out, "animations.spec.json"), append(js, '\n'), 0o644); err != nil {
		fail(err)
	}
	_, content := pptxgen.ContentWithSpec()
	js, _ = json.MarshalIndent(content, "", "  ")
	if err := os.WriteFile(filepath.Join(*out, "content.spec.json"), append(js, '\n'), 0o644); err != nil {
		fail(err)
	}
	_, kerning := pptxgen.KerningWithSpec()
	js, _ = json.MarshalIndent(kerning, "", "  ")
	if err := os.WriteFile(filepath.Join(*out, "kerning.spec.json"), append(js, '\n'), 0o644); err != nil {
		fail(err)
	}
	for _, n := range names {
		p := filepath.Join(*out, n+".pptx")
		if err := decks[n].Write(p); err != nil {
			fail(err)
		}
		fmt.Println("wrote", p)
	}
}

// writeKerningLocal chooses the words of each font from the tables of its installed files and
// writes the deck and its spec.
func writeKerningLocal(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var fs []pptxgen.KerningFont
	for _, fam := range kernLocalFamilies {
		regular, bold := kernprobe.Find(fam, "Regular"), kernprobe.Find(fam, "Bold")
		if len(regular) == 0 {
			fmt.Println("not installed, left out:", fam)
			continue
		}
		faces := append(append([]*kernprobe.Font{}, regular...), bold...)
		// GPOS through CoreText where it is at hand: go-text misses pairs in some of these fonts
		if kernprobe.CoreTextAvailable() {
			for _, face := range faces {
				if err := face.CoreText(kernprobe.Words); err != nil {
					return err
				}
			}
		} else {
			fmt.Println("no CoreText here: GPOS kerning of", fam, "comes from go-text and may miss pairs")
		}
		f := kerningFont(fam, faces)
		f.Bold = len(bold) > 0
		fs = append(fs, f)
		fmt.Printf("%-15s %d file(s), kern table v%d with %d pairs, GPOS kern %v: %s\n", fam, len(faces), regular[0].KernVersion, regular[0].KernPairs, regular[0].GPOSKern, strings.Join(f.Words, " "))
	}
	d, spec := pptxgen.KerningLocal(fs)
	js, _ := json.MarshalIndent(spec, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "kerning-local.spec.json"), append(js, '\n'), 0o644); err != nil {
		return err
	}
	p := filepath.Join(dir, "kerning-local.pptx")
	fmt.Println("wrote", p)
	return d.Write(p)
}

// kerningFont builds the line of a font: three control words around up to five kerned words,
// and up to two words for which the kern table and GPOS disagree. A font without kerning data
// gets the words of the public deck; all of them are control words there.
func kerningFont(family string, faces []*kernprobe.Font) pptxgen.KerningFont {
	c := kernprobe.Choose(faces, kernprobe.Words, 0.01)
	f := pptxgen.KerningFont{Family: family}
	if len(c.Kerned) == 0 {
		f.Words, f.Control = pptxgen.KerningWords, pptxgen.KerningWords
		return f
	}
	take := func(list []string, n int, skip map[string]bool) []string {
		var out []string
		for _, w := range list {
			if len(out) < n && !skip[w] {
				out = append(out, w)
				skip[w] = true
			}
		}
		return out
	}
	used := map[string]bool{}
	f.Differ = take(c.Differ, 2, used)
	f.Kerned = take(c.Kerned, 5, used)
	f.Control = take(c.Control, 3, used)
	// control words first, in the middle and last; the rest in between
	rest := append(append([]string{}, f.Kerned...), f.Differ...)
	ctl := append([]string{}, f.Control...)
	for len(ctl) < 3 {
		ctl = append(ctl, "")
	}
	half := (len(rest) + 1) / 2
	line := append([]string{ctl[0]}, rest[:half]...)
	line = append(append(append(line, ctl[1]), rest[half:]...), ctl[2])
	for _, w := range line {
		if w != "" {
			f.Words = append(f.Words, w)
		}
	}
	return f
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "pptxgen:", err)
	os.Exit(1)
}
