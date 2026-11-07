package clean

import (
	"strings"
	"testing"
)

func TestDedupeFontFaces(t *testing.T) {
	seen := Seen{}
	a := "@font-face{font-family:X;src:url(deckfold:1)}\n.a{}"
	b := "@font-face {font-family:X; src:url(deckfold:1)}\n@font-face{font-family:Y;src:url(deckfold:2)}"
	out1, n1 := DedupeFontFaces(a, seen)
	out2, n2 := DedupeFontFaces(strings.ReplaceAll(b, "{font-family:X; ", "{font-family:X;"), seen)
	if n1 != 0 || out1 != a {
		t.Fatal("the first block must stay")
	}
	if n2 != 1 || strings.Contains(out2, "font-family:X") || !strings.Contains(out2, "font-family:Y") {
		t.Fatalf("duplicate not removed: %q (%d)", out2, n2)
	}
}
