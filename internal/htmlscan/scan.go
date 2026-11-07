// Package htmlscan reads HTML only to find tags, attributes and raw-text content
// together with their byte offsets. It never serializes: changes are applied as
// patches to the original bytes (see package patch).
package htmlscan

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
)

// Attr is an attribute of a start tag with byte offsets into the source.
type Attr struct {
	Name     string // lower case
	Val      string // decoded value (entities resolved)
	RawVal   string // value as written in the source (without quotes)
	Start    int    // start of the name
	End      int    // end of the whole attribute (including value and quotes)
	ValStart int    // start of the value without quotes, -1 if there is no value
	ValEnd   int
	Quote    byte // '"', '\'' or 0
}

// Tag is a start tag or a self-closing tag.
type Tag struct {
	Name        string
	Start, End  int // range of the start tag
	Attrs       []Attr
	SelfClosing bool
	// For raw-text elements (script, style, textarea, title, ...):
	ContentStart, ContentEnd int // -1 if no content was recorded
	CloseStart, CloseEnd     int // range of the end tag, -1 if there is none
	Index                    int // running number of all start tags
}

// Get returns an attribute and whether it exists.
func (t *Tag) Get(name string) (Attr, bool) {
	for _, a := range t.Attrs {
		if a.Name == name {
			return a, true
		}
	}
	return Attr{}, false
}

// Value returns the value of an attribute or "".
func (t *Tag) Value(name string) string {
	a, _ := t.Get(name)
	return a.Val
}

// Doc is the result of a scan.
type Doc struct {
	Src  []byte
	Tags []*Tag
	// Position (start) of the end tags </head> and </body>, -1 if absent.
	HeadClose, BodyClose int
	// End of the doctype, 0 if there is none.
	DoctypeEnd int
}

var rawTextTags = map[string]bool{"script": true, "style": true, "textarea": true, "title": true, "xmp": true, "iframe": true, "noembed": true, "noframes": true, "plaintext": true}

// Scan splits src into tags with offsets.
func Scan(src []byte) *Doc {
	d := &Doc{Src: src, HeadClose: -1, BodyClose: -1}
	z := html.NewTokenizer(bytes.NewReader(src))
	z.SetMaxBuf(0)
	off := 0
	var open *Tag // raw-text element whose content is being read
	idx := 0
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		raw := z.Raw()
		start, end := off, off+len(raw)
		off = end
		switch tt {
		case html.StartTagToken, html.SelfClosingTagToken:
			name, _ := z.TagName()
			t := &Tag{Name: strings.ToLower(string(name)), Start: start, End: end, SelfClosing: tt == html.SelfClosingTagToken,
				ContentStart: -1, ContentEnd: -1, CloseStart: -1, CloseEnd: -1, Index: idx}
			idx++
			t.Attrs = parseAttrs(raw, start)
			d.Tags = append(d.Tags, t)
			if rawTextTags[t.Name] && tt == html.StartTagToken {
				open = t
				t.ContentStart, t.ContentEnd = end, end
			} else {
				open = nil
			}
		case html.TextToken:
			if open != nil {
				if open.ContentStart == open.ContentEnd {
					open.ContentStart = start
				}
				open.ContentEnd = end
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			ln := strings.ToLower(string(name))
			if open != nil && ln == open.Name {
				open.CloseStart, open.CloseEnd = start, end
			} else if open == nil {
				switch ln {
				case "head":
					if d.HeadClose < 0 {
						d.HeadClose = start
					}
				case "body":
					d.BodyClose = start
				}
			}
			open = nil
		case html.DoctypeToken:
			d.DoctypeEnd = end
			open = nil
		default:
			// comments: no raw-text context
			if tt != html.CommentToken {
				open = nil
			}
		}
	}
	return d
}

// Content returns the raw-text content of a tag.
func (d *Doc) Content(t *Tag) []byte {
	if t.ContentStart < 0 {
		return nil
	}
	return d.Src[t.ContentStart:t.ContentEnd]
}

// parseAttrs reads the attributes from the raw start tag.
// base is the offset of the tag in the document.
func parseAttrs(raw []byte, base int) []Attr {
	var out []Attr
	i := 1 // after '<'
	n := len(raw)
	for i < n && !isSpace(raw[i]) && raw[i] != '>' && raw[i] != '/' {
		i++
	}
	for i < n {
		for i < n && (isSpace(raw[i]) || raw[i] == '/') {
			i++
		}
		if i >= n || raw[i] == '>' {
			break
		}
		ns := i
		// An attribute name may start with '=' (HTML spec); otherwise read up to a separator.
		if raw[i] == '=' {
			i++
		}
		for i < n && !isSpace(raw[i]) && raw[i] != '=' && raw[i] != '>' && !(raw[i] == '/' && i+1 < n && raw[i+1] == '>') {
			i++
		}
		a := Attr{Name: strings.ToLower(string(raw[ns:i])), Start: base + ns, ValStart: -1, ValEnd: -1}
		j := i
		for j < n && isSpace(raw[j]) {
			j++
		}
		if j < n && raw[j] == '=' {
			j++
			for j < n && isSpace(raw[j]) {
				j++
			}
			if j < n && (raw[j] == '"' || raw[j] == '\'') {
				q := raw[j]
				vs := j + 1
				k := vs
				for k < n && raw[k] != q {
					k++
				}
				a.Quote = q
				a.ValStart, a.ValEnd = base+vs, base+k
				a.RawVal = string(raw[vs:k])
				if k < n {
					k++
				}
				i = k
			} else {
				vs := j
				k := vs
				for k < n && !isSpace(raw[k]) && raw[k] != '>' {
					k++
				}
				a.ValStart, a.ValEnd = base+vs, base+k
				a.RawVal = string(raw[vs:k])
				i = k
			}
			a.Val = html.UnescapeString(a.RawVal)
		}
		a.End = base + i
		if a.Name != "" {
			out = append(out, a)
		}
	}
	return out
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' }
