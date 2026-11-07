// Package repocheck holds repository-wide checks.
package repocheck

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// germanRunes are characters that only show up in German text: the umlauts, sharp s and the
// German opening quotation mark (U+201E). Written as escapes so this file passes its own check.
const germanRunes = "\u00e4\u00f6\u00fc\u00c4\u00d6\u00dc\u00df\u201e"

// allowed lists files that may contain German text: the German UI strings of the runtime.
var allowed = map[string]bool{"runtime/src/00-core.js": true}

// TestPublicTextsAreEnglish fails on German characters in tracked files. Everything in the
// repository is public and written in English (see CLAUDE.md). German text without these
// characters is not detected.
func TestPublicTextsAreEnglish(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("git ls-files not available: %v", err)
	}
	var problems []string
	for _, name := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
		if name == "" || allowed[name] {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || bytes.IndexByte(data, 0) >= 0 {
			continue // deleted in the working tree, or binary
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.ContainsAny(line, germanRunes) {
				problems = append(problems, fmt.Sprintf("%s:%d: %s", name, i+1, strings.TrimSpace(line)))
			}
		}
	}
	if len(problems) > 0 {
		t.Errorf("German text in public files (write it in English):\n%s", strings.Join(problems, "\n"))
	}
}
