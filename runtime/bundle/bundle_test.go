package bundle

import (
	"os"
	"regexp"
	"strings"
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

// TestThreeWrapped checks runtime/dist/three.js, which scripts/three builds from the pinned npm
// package: a classic script (no import or export statements) that sets __dfThree, in ASCII,
// with the license notice, and exporting every name that scripts/three/entry.js lists.
func TestThreeWrapped(t *testing.T) {
	js, err := os.ReadFile("../dist/three.js")
	if err != nil {
		t.Fatal("runtime/dist/three.js missing: run cd scripts/three && go run .")
	}
	s := string(js)
	if !strings.HasPrefix(s, "/* three.js ") || !strings.Contains(s[:300], "MIT License") || !strings.Contains(s[:300], "three.js authors") {
		t.Error("the license notice is missing at the top")
	}
	if !strings.Contains(s, "\nvar __dfThree=(()=>{") || !strings.HasSuffix(strings.TrimSpace(s), "})();") {
		t.Error("not wrapped as a classic script that sets __dfThree")
	}
	if regexp.MustCompile(`(^|[;}\n])\s*(import|export)\s*[{*"']`).MatchString(s) {
		t.Error("contains module import or export statements")
	}
	if strings.Contains(strings.ToLower(s), "</script") {
		t.Error("contains </script")
	}
	for _, c := range js {
		if c > 127 {
			t.Fatal("contains non-ASCII characters")
		}
	}
	entry, err := os.ReadFile("../../scripts/three/entry.js")
	if err != nil {
		t.Fatal(err)
	}
	body := string(entry)[strings.Index(string(entry), "export {")+len("export {"):]
	body = body[:strings.Index(body, "}")]
	for _, name := range strings.FieldsFunc(body, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' }) {
		if !strings.Contains(s, name+":()=>") {
			t.Errorf("%s is not exported: run cd scripts/three && go run .", name)
		}
	}
}
