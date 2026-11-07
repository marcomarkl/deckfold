// Command pptxgen writes the public PowerPoint feature decks to fixtures/pptx/src.
//
// Usage: go run ./scripts/pptxgen [-out fixtures/pptx/src] [-big] [-ppbase folder]
//
// The decks use neutral English content and only the fonts deckfold bundles. scripts/pptxref
// opens each of them in PowerPoint, saves it again and exports one reference PNG per slide;
// the tests read the PowerPoint-saved copies in fixtures/pptx.
// -big also writes big.pptx (200 slides) for the performance test; it is not committed.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"deckfold/internal/pptx/pptxgen"
)

func main() {
	out := flag.String("out", filepath.Join("fixtures", "pptx", "src"), "output folder")
	big := flag.Bool("big", false, "also write big.pptx with 200 slides")
	ppbase := flag.String("ppbase", "", "only write the base decks for scripts/pptxref/ppdecks.sh into this folder")
	flag.Parse()
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
	for _, n := range names {
		p := filepath.Join(*out, n+".pptx")
		if err := decks[n].Write(p); err != nil {
			fail(err)
		}
		fmt.Println("wrote", p)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "pptxgen:", err)
	os.Exit(1)
}
