// Package bundle assembles the runtime sources (without go:embed, so the generator also
// runs when dist/ does not exist yet).
package bundle

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Bundle assembles the sources in srcDir: all *.js in name order inside one IIFE,
// all *.css concatenated.
func Bundle(srcDir string) (js, css string, err error) {
	return BundleNamed(srcDir, "deckfold runtime, generated from runtime/src")
}

// BundleNamed is Bundle with another header comment.
func BundleNamed(srcDir, label string) (js, css string, err error) {
	var jsParts, cssParts []string
	ents, err := os.ReadDir(srcDir)
	if err != nil {
		return "", "", err
	}
	names := make([]string, 0, len(ents))
	for _, e := range ents {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(srcDir, n))
		if err != nil {
			return "", "", err
		}
		switch filepath.Ext(n) {
		case ".js":
			jsParts = append(jsParts, "// --- "+n+" ---\n"+string(b))
		case ".css":
			cssParts = append(cssParts, "/* --- "+n+" --- */\n"+string(b))
		}
	}
	js = "/* " + label + ", do not edit by hand */\n(function(){\"use strict\";\n" + strings.Join(jsParts, "\n") + "\n})();\n"
	css = "/* " + label + " */\n" + strings.Join(cssParts, "\n")
	// Emit ASCII only, so the runtime is correct whatever the charset of the source.
	js, css = asciiJS(js), asciiCSS(css)
	if strings.Contains(strings.ToLower(js), "</script") || strings.Contains(js, "<!--") {
		return "", "", fmt.Errorf("%s contains </script or <!--", srcDir)
	}
	if strings.Contains(strings.ToLower(css), "</style") {
		return "", "", fmt.Errorf("%s contains </style", srcDir)
	}
	return js, css, nil
}

// asciiJS replaces non-ASCII characters with \uXXXX (valid in strings and comments).
func asciiJS(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < 0x80:
			b.WriteRune(r)
		case r > 0xFFFF:
			r -= 0x10000
			fmt.Fprintf(&b, "\\u%04x\\u%04x", 0xD800+(r>>10), 0xDC00+(r&0x3FF))
		default:
			fmt.Fprintf(&b, "\\u%04x", r)
		}
	}
	return b.String()
}

// asciiCSS replaces non-ASCII characters with CSS escapes (\XXXXXX).
func asciiCSS(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 0x80 {
			b.WriteRune(r)
			continue
		}
		fmt.Fprintf(&b, "\\%06x", r)
	}
	return b.String()
}
