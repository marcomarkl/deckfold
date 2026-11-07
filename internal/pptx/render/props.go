package render

import "deckfold/internal/pptx/xmltree"

// props merges DrawingML property elements (a:pPr, a:rPr, a:bodyPr): attributes override one by
// one, child elements override by group (only one fill, one bullet type, ...).
type props struct {
	attrs map[string]string
	elems map[string]*xmltree.Node // group -> element
}

func newProps() *props { return &props{attrs: map[string]string{}, elems: map[string]*xmltree.Node{}} }

var groups = map[string]string{
	// fills
	"noFill": "fill", "solidFill": "fill", "gradFill": "fill", "blipFill": "fill", "pattFill": "fill", "grpFill": "fill",
	// bullets
	"buNone": "buType", "buAutoNum": "buType", "buChar": "buType", "buBlip": "buType",
	"buClrTx": "buClr", "buClr": "buClr",
	"buSzTx": "buSz", "buSzPct": "buSz", "buSzPts": "buSz",
	"buFontTx": "buFont", "buFont": "buFont",
	// underline
	"uLnTx": "uLn", "uLn": "uLn", "uFillTx": "uFill", "uFill": "uFill",
	// autofit
	"noAutofit": "autofit", "normAutofit": "autofit", "spAutoFit": "autofit",
}

func group(name string) string {
	if g, ok := groups[name]; ok {
		return g
	}
	return name
}

// add merges n over the current values (n has higher priority).
func (p *props) add(n *xmltree.Node) {
	if n == nil {
		return
	}
	for _, a := range n.Attrs {
		if a.Name.Space == "" {
			p.attrs[a.Name.Local] = a.Value
		}
	}
	for _, k := range n.Kids {
		if k.Name == "defRPr" || k.Name == "extLst" {
			continue
		}
		p.elems[group(k.Name)] = k
	}
}

func (p *props) attr(name string) string { return p.attrs[name] }

func (p *props) has(name string) bool { _, ok := p.attrs[name]; return ok }

func (p *props) elem(name string) *xmltree.Node { return p.elems[group(name)] }

func (p *props) int(name string, def int64) int64 {
	v, ok := p.attrs[name]
	if !ok {
		return def
	}
	var i int64
	neg := false
	for j, c := range v {
		if j == 0 && c == '-' {
			neg = true
			continue
		}
		if c < '0' || c > '9' {
			return def
		}
		i = i*10 + int64(c-'0')
	}
	if neg {
		return -i
	}
	return i
}

func (p *props) bool(name string, def bool) bool {
	switch p.attrs[name] {
	case "1", "true", "on":
		return true
	case "0", "false", "off":
		return false
	}
	return def
}
