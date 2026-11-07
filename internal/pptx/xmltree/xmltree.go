// Package xmltree parses an XML part into a small element tree. PresentationML is read by local
// names (prefixes differ between producers, the local names do not collide within a part).
//
// Markup compatibility (mc:AlternateContent) is resolved while parsing: the first mc:Choice whose
// Requires prefixes are all in the supported set wins, otherwise mc:Fallback.
package xmltree

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"
)

const nsMC = "http://schemas.openxmlformats.org/markup-compatibility/2006"

// Node is an element.
type Node struct {
	Space  string // namespace URI
	Name   string // local name
	Attrs  []xml.Attr
	Kids   []*Node
	Text   string // character data directly inside the element
	Parent *Node
}

// Options control parsing.
type Options struct {
	// Supported lists the namespace URIs an mc:Choice may require to be taken.
	Supported map[string]bool
}

// Parse parses data. The depth is limited to protect against hostile input.
func Parse(data []byte, opt Options) (*Node, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	root := &Node{Name: "#document"}
	cur := root
	// prefix -> namespace, per open element, to evaluate mc:Choice Requires="p14 a14"
	type scope struct{ prefixes map[string]string }
	scopes := []scope{{map[string]string{}}}
	depth := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth > 256 {
				return nil, errors.New("xml: nesting too deep")
			}
			sc := scope{map[string]string{}}
			for k, v := range scopes[len(scopes)-1].prefixes {
				sc.prefixes[k] = v
			}
			for _, a := range t.Attr {
				if a.Name.Space == "xmlns" {
					sc.prefixes[a.Name.Local] = a.Value
				}
			}
			scopes = append(scopes, sc)
			n := &Node{Space: t.Name.Space, Name: t.Name.Local, Parent: cur}
			for _, a := range t.Attr {
				if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
					continue
				}
				n.Attrs = append(n.Attrs, a)
			}
			if n.Space == nsMC && n.Name == "Choice" {
				n.Attrs = append(n.Attrs, xml.Attr{Name: xml.Name{Local: "df-ok"}, Value: strconv.FormatBool(requiresOK(n.Attr("Requires"), sc.prefixes, opt.Supported))})
			}
			cur.Kids = append(cur.Kids, n)
			cur = n
		case xml.EndElement:
			depth--
			scopes = scopes[:len(scopes)-1]
			done := cur
			cur = cur.Parent
			if done.Space == nsMC && done.Name == "AlternateContent" {
				resolveAlt(cur, done)
			}
		case xml.CharData:
			if cur != root {
				cur.Text += string(t)
			}
		}
	}
	for _, k := range root.Kids {
		k.Parent = nil
		return k, nil
	}
	return nil, errors.New("xml: no root element")
}

func requiresOK(req string, prefixes map[string]string, supported map[string]bool) bool {
	for _, p := range strings.Fields(req) {
		if !supported[prefixes[p]] {
			return false
		}
	}
	return true
}

// resolveAlt replaces the AlternateContent element alt (last child of parent) by the children
// of the chosen branch.
func resolveAlt(parent, alt *Node) {
	var chosen *Node
	for _, k := range alt.Kids {
		if k.Space == nsMC && k.Name == "Choice" && k.Attr("df-ok") == "true" {
			chosen = k
			break
		}
	}
	if chosen == nil {
		for _, k := range alt.Kids {
			if k.Space == nsMC && k.Name == "Fallback" {
				chosen = k
			}
		}
	}
	kids := parent.Kids[:len(parent.Kids)-1]
	if chosen != nil {
		for _, k := range chosen.Kids {
			k.Parent = parent
			kids = append(kids, k)
		}
	}
	parent.Kids = kids
}

// Attr returns the value of an attribute without namespace, or "".
func (n *Node) Attr(name string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attrs {
		if a.Name.Local == name && a.Name.Space == "" {
			return a.Value
		}
	}
	return ""
}

// NSR is the relationships namespace of attributes such as r:id and r:embed.
const NSR = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"

// RID returns the r:<name> attribute (r:id, r:embed, r:link).
func (n *Node) RID(name string) string { return n.AttrNS(NSR, name) }

// AttrNS returns the value of an attribute in a namespace.
func (n *Node) AttrNS(space, name string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attrs {
		if a.Name.Local == name && a.Name.Space == space {
			return a.Value
		}
	}
	return ""
}

// Has reports whether the attribute is present.
func (n *Node) Has(name string) bool {
	if n == nil {
		return false
	}
	for _, a := range n.Attrs {
		if a.Name.Local == name && a.Name.Space == "" {
			return true
		}
	}
	return false
}

// Int returns an integer attribute or def.
func (n *Node) Int(name string, def int64) int64 {
	v := n.Attr(name)
	if v == "" {
		return def
	}
	i, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil {
		f, err2 := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err2 != nil {
			return def
		}
		return int64(f)
	}
	return i
}

// Bool returns a boolean attribute ("1", "true", "on") or def.
func (n *Node) Bool(name string, def bool) bool {
	switch n.Attr(name) {
	case "1", "true", "on":
		return true
	case "0", "false", "off":
		return false
	}
	return def
}

// Child returns the first child with the local name.
func (n *Node) Child(name string) *Node {
	if n == nil {
		return nil
	}
	for _, k := range n.Kids {
		if k.Name == name {
			return k
		}
	}
	return nil
}

// Children returns all children with the local name.
func (n *Node) Children(name string) []*Node {
	if n == nil {
		return nil
	}
	var out []*Node
	for _, k := range n.Kids {
		if k.Name == name {
			out = append(out, k)
		}
	}
	return out
}

// Path follows a slash-separated list of local names ("cSld/spTree").
func (n *Node) Path(p string) *Node {
	cur := n
	for _, s := range strings.Split(p, "/") {
		cur = cur.Child(s)
		if cur == nil {
			return nil
		}
	}
	return cur
}

// Find returns the first descendant (depth first) with the local name.
func (n *Node) Find(name string) *Node {
	if n == nil {
		return nil
	}
	for _, k := range n.Kids {
		if k.Name == name {
			return k
		}
		if f := k.Find(name); f != nil {
			return f
		}
	}
	return nil
}

// All returns all descendants with the local name.
func (n *Node) All(name string) []*Node {
	var out []*Node
	var walk func(*Node)
	walk = func(x *Node) {
		for _, k := range x.Kids {
			if k.Name == name {
				out = append(out, k)
			}
			walk(k)
		}
	}
	if n != nil {
		walk(n)
	}
	return out
}

// AllText returns the concatenated character data of the element and its descendants.
func (n *Node) AllText() string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var walk func(*Node)
	walk = func(x *Node) {
		b.WriteString(x.Text)
		for _, k := range x.Kids {
			walk(k)
		}
	}
	walk(n)
	return b.String()
}

// Elems returns the child elements, nil for a nil node.
func (n *Node) Elems() []*Node {
	if n == nil {
		return nil
	}
	return n.Kids
}
