// Package clean holds the safe clean-ups (E11): detecting duplicate includes and merging
// identical @font-face rules. CSS and JS content is otherwise left unchanged.
package clean

import (
	"strings"
)

// Seen remembers resources already included (key: resolved URL or path, or content hash).
type Seen map[string]bool

// First returns true the first time a key is seen.
func (s Seen) First(key string) bool {
	if s[key] {
		return false
	}
	s[key] = true
	return true
}

// DedupeFontFaces removes @font-face blocks whose text (after embedding) has already been
// seen. seen spans all stylesheets of a document. Returns the CSS and the number of removed blocks.
func DedupeFontFaces(css string, seen Seen) (string, int) {
	lower := strings.ToLower(css)
	var b strings.Builder
	pos, removed := 0, 0
	for {
		i := strings.Index(lower[pos:], "@font-face")
		if i < 0 {
			break
		}
		i += pos
		open := strings.IndexByte(css[i:], '{')
		if open < 0 {
			break
		}
		end := matchBrace(css, i+open)
		if end < 0 {
			break
		}
		block := css[i:end]
		key := normalize(block)
		b.WriteString(css[pos:i])
		if seen.First(key) {
			b.WriteString(block)
		} else {
			removed++
		}
		pos = end
	}
	b.WriteString(css[pos:])
	return b.String(), removed
}

func matchBrace(s string, open int) int {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '"', '\'':
			q := s[i]
			for i++; i < len(s) && s[i] != q; i++ {
				if s[i] == '\\' {
					i++
				}
			}
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return -1
}

// normalize removes whitespace differences for comparing @font-face blocks.
func normalize(s string) string {
	return strings.Join(strings.Fields(s), "")
}
