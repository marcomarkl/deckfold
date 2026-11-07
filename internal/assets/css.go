package assets

import (
	"net/url"
	"strings"
)

// CSSRefKind distinguishes the kinds of references in CSS.
type CSSRefKind int

const (
	RefURL      CSSRefKind = iota // url(...)
	RefImport                     // @import "..." or @import url(...)
	RefImageSet                   // string inside image-set(...)
)

// CSSRef is a reference in CSS with the offsets of its value (without quotes).
type CSSRef struct {
	Kind       CSSRefKind
	Value      string
	Start, End int // value range
	// For @import: the whole statement including ';' and the conditions after it (media, layer, supports)
	StmtStart, StmtEnd int
	ImportTail         string
}

func urlUnescape(s string) (string, error) { return url.PathUnescape(s) }

// ScanCSS finds url(), @import and image-set() strings.
func ScanCSS(css string) []CSSRef {
	var out []CSSRef
	n := len(css)
	lower := strings.ToLower(css)
	i := 0
	imageSetDepth := 0
	parenDepth := 0
	imageSetStart := -1
	for i < n {
		c := css[i]
		switch {
		case c == '/' && i+1 < n && css[i+1] == '*':
			end := strings.Index(css[i+2:], "*/")
			if end < 0 {
				return out
			}
			i += end + 4
			continue
		case c == '"' || c == '\'':
			j := skipString(css, i)
			if imageSetDepth > 0 && parenDepth == imageSetDepth {
				out = append(out, CSSRef{Kind: RefImageSet, Value: unescapeCSS(css[i+1 : j-1]), Start: i + 1, End: j - 1})
			}
			i = j
			continue
		case c == '(':
			parenDepth++
		case c == ')':
			if imageSetDepth > 0 && parenDepth == imageSetDepth {
				imageSetDepth = 0
				imageSetStart = -1
			}
			parenDepth--
		case c == '@' && strings.HasPrefix(lower[i:], "@import"):
			if r, next, ok := scanImport(css, lower, i); ok {
				out = append(out, r)
				i = next
				continue
			}
		case (c == 'u' || c == 'U') && strings.HasPrefix(lower[i:], "url(") && (i == 0 || !isIdent(css[i-1])):
			if r, next, ok := scanURL(css, i); ok {
				out = append(out, r)
				i = next
				continue
			}
		case strings.HasPrefix(lower[i:], "image-set(") && (i == 0 || !isIdent(css[i-1]) || strings.HasSuffix(lower[:i], "-webkit-")):
			parenDepth++
			imageSetDepth = parenDepth
			imageSetStart = i
			i += len("image-set(")
			continue
		}
		i++
	}
	_ = imageSetStart
	return out
}

func isIdent(c byte) bool {
	return c == '-' || c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func skipString(css string, i int) int {
	q := css[i]
	j := i + 1
	for j < len(css) {
		if css[j] == '\\' {
			j += 2
			continue
		}
		if css[j] == q || css[j] == '\n' {
			return j + 1
		}
		j++
	}
	return len(css)
}

// scanURL reads url(...) starting at position i.
func scanURL(css string, i int) (CSSRef, int, bool) {
	j := i + 4
	n := len(css)
	for j < n && isWS(css[j]) {
		j++
	}
	if j >= n {
		return CSSRef{}, 0, false
	}
	if css[j] == '"' || css[j] == '\'' {
		k := skipString(css, j)
		r := CSSRef{Kind: RefURL, Value: unescapeCSS(css[j+1 : k-1]), Start: j + 1, End: k - 1}
		for k < n && css[k] != ')' {
			k++
		}
		return r, k + 1, true
	}
	k := j
	for k < n && css[k] != ')' {
		k++
	}
	e := k
	for e > j && isWS(css[e-1]) {
		e--
	}
	return CSSRef{Kind: RefURL, Value: unescapeCSS(css[j:e]), Start: j, End: e}, k + 1, true
}

// scanImport reads an @import statement.
func scanImport(css, lower string, i int) (CSSRef, int, bool) {
	n := len(css)
	j := i + len("@import")
	for j < n && isWS(css[j]) {
		j++
	}
	var r CSSRef
	var k int
	switch {
	case j < n && (css[j] == '"' || css[j] == '\''):
		k = skipString(css, j)
		r = CSSRef{Kind: RefImport, Value: unescapeCSS(css[j+1 : k-1]), Start: j + 1, End: k - 1}
	case strings.HasPrefix(lower[j:], "url("):
		u, next, ok := scanURL(css, j)
		if !ok {
			return CSSRef{}, 0, false
		}
		r = u
		r.Kind = RefImport
		k = next
	default:
		return CSSRef{}, 0, false
	}
	semi := strings.IndexByte(css[k:], ';')
	end := n
	tail := strings.TrimSpace(css[k:])
	if semi >= 0 {
		end = k + semi + 1
		tail = strings.TrimSpace(css[k : k+semi])
	}
	r.StmtStart, r.StmtEnd, r.ImportTail = i, end, tail
	return r, end, true
}

func isWS(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' }

// unescapeCSS resolves simple CSS escapes (\" \' \\ and hex escapes).
func unescapeCSS(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		h := 0
		v := 0
		for h < 6 && i+h < len(s) && isHex(s[i+h]) {
			v = v*16 + hexVal(s[i+h])
			h++
		}
		if h > 0 {
			b.WriteRune(rune(v))
			i += h - 1
			if i+1 < len(s) && isWS(s[i+1]) {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	}
	return int(c-'A') + 10
}

// ImportWrap wraps inlined CSS in the conditions of an @import statement.
func ImportWrap(css, tail string) string {
	t := strings.TrimSpace(tail)
	if t == "" {
		return css
	}
	lt := strings.ToLower(t)
	var pre, post string
	if strings.HasPrefix(lt, "layer") {
		// layer or layer(name)
		end := len("layer")
		if strings.HasPrefix(lt, "layer(") {
			end = strings.IndexByte(t, ')') + 1
		}
		name := strings.TrimSuffix(strings.TrimPrefix(t[:end], "layer"), "")
		name = strings.Trim(name, "()")
		pre += "@layer " + name + "{"
		post = "}" + post
		t = strings.TrimSpace(t[end:])
		lt = strings.ToLower(t)
	}
	if strings.HasPrefix(lt, "supports(") {
		end := matchParen(t, len("supports"))
		pre += "@supports " + t[len("supports"):end] + "{"
		post = "}" + post
		t = strings.TrimSpace(t[end:])
	}
	if t != "" {
		pre += "@media " + t + "{"
		post = "}" + post
	}
	return pre + css + post
}

func matchParen(s string, open int) int {
	d := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '(':
			d++
		case ')':
			d--
			if d == 0 {
				return i + 1
			}
		}
	}
	return len(s)
}
