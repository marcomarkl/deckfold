package bundle

import (
	"os"
	"testing"
)

// TestDistUpToDate: the checked-in bundle matches the sources (otherwise run go generate ./runtime).
func TestDistUpToDate(t *testing.T) {
	js, css, err := Bundle("../src")
	if err != nil {
		t.Fatal(err)
	}
	djs, err1 := os.ReadFile("../dist/deckfold.js")
	dcss, err2 := os.ReadFile("../dist/deckfold.css")
	if err1 != nil || err2 != nil {
		t.Fatal("runtime/dist missing: run go generate ./runtime")
	}
	if string(djs) != js || string(dcss) != css {
		t.Fatal("runtime/dist is out of date: run go generate ./runtime")
	}
	pjs, _, err := BundleNamed("../pptx-player", "deckfold PowerPoint player, generated from runtime/pptx-player")
	if err != nil {
		t.Fatal(err)
	}
	dp, err := os.ReadFile("../dist/pptx-player.js")
	if err != nil || string(dp) != pjs {
		t.Fatal("runtime/dist/pptx-player.js is out of date: run go generate ./runtime")
	}
	for _, c := range js + css + pjs {
		if c > 127 {
			t.Fatal("bundle contains non-ASCII characters")
		}
	}
}
