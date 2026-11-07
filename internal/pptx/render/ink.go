package render

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"deckfold/internal/pptx/model"
	"deckfold/internal/pptx/xmltree"
)

// ink draws a p:contentPart (p14) that holds ink: the InkML traces become SVG paths in the
// brush's color, width and transparency, fitted into the part's frame.
func (r *slideRenderer) ink(b *strings.Builder, cp *xmltree.Node, owner ownerKind, part model.Part, t xf) {
	rel, ok := part.Rels[cp.RID("id")]
	if !ok || rel.External {
		return
	}
	doc, err := r.pres.Pkg.XML(rel.Target)
	if err != nil {
		r.warnOnce("ink on slide %d cannot be read: %v", r.slide.Number, err)
		return
	}
	x := cp.Child("xfrm")
	if x == nil {
		return
	}
	box := t.box(x.Child("off"), x.Child("ext"))
	type brushT struct {
		color     string
		width     float64 // px
		opacity   float64
		rectangle bool
	}
	brushes := map[string]brushT{}
	for _, br := range doc.All("brush") {
		bt := brushT{color: "#000000", width: 0, opacity: 1}
		for _, p := range br.Children("brushProperty") {
			v := p.Attr("value")
			switch p.Attr("name") {
			case "color":
				if strings.HasPrefix(v, "#") && len(v) == 7 {
					bt.color = v
				}
			case "width":
				w := parseF(v, 0)
				if p.Attr("units") == "cm" {
					w *= 96 / 2.54 // drawn at its size, whatever the frame (as PowerPoint does)
				}
				bt.width = w
			case "transparency":
				bt.opacity = 1 - parseF(v, 0)/255
			case "tip":
				bt.rectangle = v == "rectangle"
			}
		}
		for _, a := range br.Attrs {
			if a.Name.Local == "id" {
				brushes["#"+a.Value] = bt
			}
		}
	}
	type traceT struct {
		pts   [][2]float64
		brush brushT
	}
	var traces []traceT
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, tr := range doc.All("trace") {
		pts := inkPoints(tr.AllText())
		if len(pts) == 0 {
			continue
		}
		bt, ok := brushes[tr.Attr("brushRef")]
		if !ok {
			bt = brushT{color: "#000000", width: 1.3, opacity: 1}
		}
		for _, p := range pts {
			minX, minY, maxX, maxY = math.Min(minX, p[0]), math.Min(minY, p[1]), math.Max(maxX, p[0]), math.Max(maxY, p[1])
		}
		traces = append(traces, traceT{pts, bt})
	}
	if len(traces) == 0 {
		return
	}
	// the bounding box of the strokes fills the frame, in both directions (as PowerPoint draws
	// it, checked against the PowerPoint reference of the ink deck)
	w, h := math.Max(maxX-minX, 1), math.Max(maxY-minY, 1)
	sx, sy := box.w/w, box.h/h
	ox, oy := box.x, box.y
	end := r.open(b, cp, owner, box, x)
	for _, tr := range traces {
		var d strings.Builder
		for i, p := range tr.pts {
			cmd := "L"
			if i == 0 {
				cmd = "M"
			}
			fmt.Fprintf(&d, "%s%s %s", cmd, f(ox+(p[0]-minX)*sx), f(oy+(p[1]-minY)*sy))
		}
		if len(tr.pts) == 1 {
			d.WriteString("h0.01")
		}
		cap := "round"
		if tr.brush.rectangle {
			cap = "square"
		}
		sw := math.Max(0.5, tr.brush.width)
		op := ""
		if tr.brush.opacity < 1 {
			op = ` stroke-opacity="` + f(tr.brush.opacity) + `"`
		}
		fmt.Fprintf(b, `<path d="%s" fill="none" stroke="%s" stroke-width="%s" stroke-linecap="%s" stroke-linejoin="round"%s/>`, d.String(), tr.brush.color, f(sw), cap, op)
	}
	b.WriteString(end)
}

// inkPoints reads an InkML trace: points separated by commas, the first two channels (X, Y)
// as explicit values (!), first differences (') or second differences ("), which stay in force
// until the next prefix (InkML 1.0, section 3.2.1).
func inkPoints(s string) [][2]float64 {
	var out [][2]float64
	type ch struct {
		mode      byte
		last, vel float64
	}
	chans := []ch{{mode: '!'}, {mode: '!'}}
	for _, p := range strings.Split(s, ",") {
		vals := []float64{}
		modes := []byte{}
		i := 0
		for i < len(p) {
			c := p[i]
			switch {
			case c == ' ' || c == '\t' || c == '\n' || c == '\r':
				i++
				continue
			case c == '!' || c == '\'' || c == '"':
				modes = append(modes, c)
				i++
			default:
				modes = append(modes, 0)
			}
			j := i
			for j < len(p) && (p[j] == '-' && j == i || p[j] == '.' || p[j] >= '0' && p[j] <= '9' || p[j] == 'e' || p[j] == 'E') {
				j++
			}
			if j == i {
				i++
				modes = modes[:len(modes)-1]
				continue
			}
			v, err := strconv.ParseFloat(p[i:j], 64)
			if err != nil {
				modes = modes[:len(modes)-1]
				i = j
				continue
			}
			vals = append(vals, v)
			i = j
		}
		if len(vals) < 2 {
			continue
		}
		var pt [2]float64
		for k := 0; k < 2; k++ {
			c := &chans[k]
			if modes[k] != 0 {
				c.mode = modes[k]
			}
			switch c.mode {
			case '\'':
				c.vel = vals[k]
				c.last += c.vel
			case '"':
				c.vel += vals[k]
				c.last += c.vel
			default:
				c.vel = vals[k] - c.last
				c.last = vals[k]
			}
			pt[k] = c.last
		}
		out = append(out, pt)
	}
	return out
}
