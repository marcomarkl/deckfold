package kernprobe

import (
	"bytes"
	_ "embed"
	"fmt"
	"math"
	"os/exec"
	"runtime"
	"strings"
)

//go:embed ctkern.swift
var ctScript string

// CoreTextAvailable reports whether CoreText can be asked: on macOS with the swift command.
func CoreTextAvailable() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	_, err := exec.LookPath("swift")
	return err == nil
}

// CoreText asks CoreText for the kerning it applies to each text and keeps the answers: GPOS
// returns them from then on for these texts. go-text/typesetting, which GPOS uses otherwise,
// misses pairs in some fonts (in Montserrat 2.x it finds "AV" but not "VA" or "To"; CoreText and
// the font's own table have them), so the measurement of installed fonts goes through CoreText.
// The font must come from a file (Open or Find).
func (f *Font) CoreText(texts []string) error {
	if f.Path == "" {
		return fmt.Errorf("CoreText needs the font as a file")
	}
	if len(texts) == 0 {
		return nil
	}
	cmd := exec.Command("swift", append([]string{"-", f.Path, f.PostScript}, texts...)...)
	cmd.Stdin = strings.NewReader(ctScript)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("CoreText for %s: %v: %s", f.PostScript, err, strings.TrimSpace(stderr.String()))
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != len(texts)+1 || lines[0] != f.PostScript {
		return fmt.Errorf("CoreText for %s: unexpected answer (%d lines, name %q)", f.PostScript, len(lines), lines[0])
	}
	if f.ct == nil {
		f.ct = map[string]float64{}
	}
	for i, t := range texts {
		var plain, kern float64
		if _, err := fmt.Sscanf(lines[i+1], "%f %f", &plain, &kern); err != nil {
			return fmt.Errorf("CoreText for %s: %q: %v", f.PostScript, lines[i+1], err)
		}
		// the same face and the same advance widths, or the kerning says nothing about this font
		if want := f.Plain(t); math.Abs(plain-want) > 0.002 {
			return fmt.Errorf("CoreText for %s: %q is %.4f em wide there and %.4f em here", f.PostScript, t, plain, want)
		}
		f.ct[t] = kern
	}
	return nil
}
