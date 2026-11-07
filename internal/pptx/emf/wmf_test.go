package emf

import (
	"strings"
	"testing"

	"deckfold/internal/pptx/pptxgen"
)

func TestWMF(t *testing.T) {
	res, err := ConvertWMF(pptxgen.TestWMF())
	if err != nil {
		t.Fatal(err)
	}
	svg := string(res.SVG)
	if res.Unknown != 0 {
		t.Errorf("%d unknown records", res.Unknown)
	}
	for _, want := range []string{`viewBox="0 0 400 300"`, `fill="#dc2828"`, `stroke="#2850dc"`, `fill="#28a03c"`, `>WMF</text>`, `font-weight="bold"`} {
		if !strings.Contains(svg, want) {
			t.Errorf("SVG lacks %s:\n%s", want, svg)
		}
	}
	if strings.Count(svg, "<path") != 5 {
		t.Errorf("%d paths, want rectangle, ellipse, triangle, arc and line", strings.Count(svg, "<path"))
	}
	if _, err := ConvertWMF([]byte("not a metafile at all")); err == nil {
		t.Error("garbage accepted")
	}
}
