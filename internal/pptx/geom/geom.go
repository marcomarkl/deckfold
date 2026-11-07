// Package geom evaluates DrawingML shape geometry: the preset shapes defined in ECMA-376
// (presetShapeDefinitions.xml, embedded unchanged, see NOTICE) and custom geometry. It returns
// SVG path data and the text rectangle of a shape.
package geom

import (
	_ "embed"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"deckfold/internal/pptx/xmltree"
)

//go:embed presetShapeDefinitions.xml
var presetXML []byte

// Def is a geometry definition (preset or custom).
type Def struct {
	av   []gd
	gds  []gd
	path []*xmltree.Node
	rect *xmltree.Node
}

type gd struct{ name, fmla string }

var (
	once    sync.Once
	presets map[string]*Def
)

func load() {
	presets = map[string]*Def{}
	root, err := xmltree.Parse(presetXML, xmltree.Options{})
	if err != nil {
		panic("presetShapeDefinitions.xml: " + err.Error())
	}
	for _, s := range root.Kids {
		presets[s.Name] = parseDef(s)
	}
}

func parseDef(n *xmltree.Node) *Def {
	d := &Def{}
	for _, g := range n.Child("avLst").Children("gd") {
		d.av = append(d.av, gd{g.Attr("name"), g.Attr("fmla")})
	}
	for _, g := range n.Child("gdLst").Children("gd") {
		d.gds = append(d.gds, gd{g.Attr("name"), g.Attr("fmla")})
	}
	d.path = n.Child("pathLst").Children("path")
	d.rect = n.Child("rect")
	return d
}

// Preset returns a preset definition by name.
func Preset(name string) (*Def, bool) {
	once.Do(load)
	d, ok := presets[name]
	return d, ok
}

// Count returns the number of preset shapes.
func Count() int { once.Do(load); return len(presets) }

// Custom parses an a:custGeom element.
func Custom(n *xmltree.Node) *Def { return parseDef(n) }

// Result is an evaluated geometry in the coordinates of the shape box.
type Result struct {
	Fill   string // path data of the filled sub-paths
	Stroke string // path data of the stroked sub-paths
	// Text rectangle relative to the box origin.
	TL, TT, TR, TB float64
}

// Eval evaluates the definition for a box (x, y, w, h in px); adj overrides adjust values
// (from a:avLst of the shape).
func (d *Def) Eval(x, y, w, h float64, adj *xmltree.Node) (Result, error) {
	// Guides work in EMU-like units; using px directly is fine because all formulas scale.
	v := builtins(w, h)
	for _, g := range d.av {
		v[g.name] = 0
		if err := eval(v, g.name, g.fmla); err != nil {
			return Result{}, err
		}
	}
	for _, g := range adj.Children("gd") {
		if err := eval(v, g.Attr("name"), g.Attr("fmla")); err != nil {
			return Result{}, err
		}
	}
	for _, g := range d.gds {
		if err := eval(v, g.name, g.fmla); err != nil {
			return Result{}, err
		}
	}
	var fill, stroke strings.Builder
	for _, p := range d.path {
		pw, ph := float64(p.Int("w", 0)), float64(p.Int("h", 0))
		sx, sy := 1.0, 1.0
		if pw > 0 {
			sx = w / pw
		}
		if ph > 0 {
			sy = h / ph
		}
		data, err := pathData(p, v, x, y, sx, sy)
		if err != nil {
			return Result{}, err
		}
		if p.Attr("fill") != "none" {
			fill.WriteString(data)
		}
		if p.Bool("stroke", true) {
			stroke.WriteString(data)
		}
	}
	res := Result{Fill: strings.TrimSpace(fill.String()), Stroke: strings.TrimSpace(stroke.String()), TR: w, TB: h}
	if d.rect != nil {
		var err error
		get := func(name string) float64 {
			if err != nil {
				return 0
			}
			var f float64
			f, err = arg(v, name)
			return f
		}
		res.TL, res.TT, res.TR, res.TB = get(d.rect.Attr("l")), get(d.rect.Attr("t")), get(d.rect.Attr("r")), get(d.rect.Attr("b"))
		if err != nil {
			return Result{}, err
		}
	}
	return res, nil
}

func builtins(w, h float64) map[string]float64 {
	ss, ls := math.Min(w, h), math.Max(w, h)
	v := map[string]float64{
		"w": w, "h": h, "l": 0, "t": 0, "r": w, "b": h, "hc": w / 2, "vc": h / 2,
		"ss": ss, "ls": ls,
		"cd2": 10800000, "cd4": 5400000, "cd8": 2700000, "3cd4": 16200000, "3cd8": 8100000, "5cd8": 13500000, "7cd8": 18900000,
	}
	for _, n := range []int{2, 3, 4, 5, 6, 8, 10, 12, 32} {
		v["wd"+strconv.Itoa(n)] = w / float64(n)
		v["hd"+strconv.Itoa(n)] = h / float64(n)
	}
	for _, n := range []int{2, 4, 6, 8, 16, 32} {
		v["ssd"+strconv.Itoa(n)] = ss / float64(n)
	}
	return v
}

func arg(v map[string]float64, s string) (float64, error) {
	if x, ok := v[s]; ok {
		return x, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("unknown guide %q", s)
	}
	return f, nil
}

const angUnit = 60000.0 // 60000ths of a degree

func rad(a float64) float64 { return a / angUnit * math.Pi / 180 }

func eval(v map[string]float64, name, fmla string) error {
	parts := strings.Fields(fmla)
	if len(parts) == 0 {
		return fmt.Errorf("empty formula for %s", name)
	}
	a := make([]float64, len(parts)-1)
	for i, p := range parts[1:] {
		x, err := arg(v, p)
		if err != nil {
			return err
		}
		a[i] = x
	}
	need := map[string]int{"*/": 3, "+-": 3, "+/": 3, "?:": 3, "abs": 1, "at2": 2, "cat2": 3, "cos": 2, "max": 2, "min": 2,
		"mod": 3, "pin": 3, "sat2": 3, "sin": 2, "sqrt": 1, "tan": 2, "val": 1}
	n, ok := need[parts[0]]
	if !ok {
		return fmt.Errorf("unknown formula %q", parts[0])
	}
	if len(a) < n {
		return fmt.Errorf("formula %q needs %d arguments", fmla, n)
	}
	var r float64
	switch parts[0] {
	case "*/":
		if a[2] == 0 {
			r = 0
		} else {
			r = a[0] * a[1] / a[2]
		}
	case "+-":
		r = a[0] + a[1] - a[2]
	case "+/":
		if a[2] == 0 {
			r = 0
		} else {
			r = (a[0] + a[1]) / a[2]
		}
	case "?:":
		if a[0] > 0 {
			r = a[1]
		} else {
			r = a[2]
		}
	case "abs":
		r = math.Abs(a[0])
	case "at2":
		r = math.Atan2(a[1], a[0]) * 180 / math.Pi * angUnit
	case "cat2":
		r = a[0] * math.Cos(math.Atan2(a[2], a[1]))
	case "cos":
		r = a[0] * math.Cos(rad(a[1]))
	case "max":
		r = math.Max(a[0], a[1])
	case "min":
		r = math.Min(a[0], a[1])
	case "mod":
		r = math.Sqrt(a[0]*a[0] + a[1]*a[1] + a[2]*a[2])
	case "pin":
		switch {
		case a[1] < a[0]:
			r = a[0]
		case a[1] > a[2]:
			r = a[2]
		default:
			r = a[1]
		}
	case "sat2":
		r = a[0] * math.Sin(math.Atan2(a[2], a[1]))
	case "sin":
		r = a[0] * math.Sin(rad(a[1]))
	case "sqrt":
		r = math.Sqrt(math.Max(0, a[0]))
	case "tan":
		r = a[0] * math.Tan(rad(a[1]))
	case "val":
		r = a[0]
	}
	if math.IsNaN(r) || math.IsInf(r, 0) {
		r = 0
	}
	v[name] = r
	return nil
}

func pathData(p *xmltree.Node, v map[string]float64, ox, oy, sx, sy float64) (string, error) {
	var b strings.Builder
	var cx, cy float64 // current point, path units
	pt := func(n *xmltree.Node) (float64, float64, error) {
		x, err := arg(v, n.Attr("x"))
		if err != nil {
			return 0, 0, err
		}
		y, err := arg(v, n.Attr("y"))
		return x, y, err
	}
	out := func(x, y float64) string { return num(ox+x*sx) + " " + num(oy+y*sy) }
	for _, c := range p.Kids {
		switch c.Name {
		case "moveTo", "lnTo":
			x, y, err := pt(c.Child("pt"))
			if err != nil {
				return "", err
			}
			cmd := "M"
			if c.Name == "lnTo" {
				cmd = "L"
			}
			b.WriteString(cmd + out(x, y) + " ")
			cx, cy = x, y
		case "close":
			b.WriteString("Z ")
		case "quadBezTo", "cubicBezTo":
			pts := c.Children("pt")
			cmd := "Q"
			if c.Name == "cubicBezTo" {
				cmd = "C"
			}
			b.WriteString(cmd)
			for i, q := range pts {
				x, y, err := pt(q)
				if err != nil {
					return "", err
				}
				if i > 0 {
					b.WriteString(" ")
				}
				b.WriteString(out(x, y))
				cx, cy = x, y
			}
			b.WriteString(" ")
		case "arcTo":
			wr, err := arg(v, c.Attr("wR"))
			if err != nil {
				return "", err
			}
			hr, err := arg(v, c.Attr("hR"))
			if err != nil {
				return "", err
			}
			st, err := arg(v, c.Attr("stAng"))
			if err != nil {
				return "", err
			}
			sw, err := arg(v, c.Attr("swAng"))
			if err != nil {
				return "", err
			}
			nx, ny := arcTo(&b, cx, cy, wr, hr, st, sw, ox, oy, sx, sy)
			cx, cy = nx, ny
		}
	}
	return b.String(), nil
}

// ellipseParam converts a visual angle on an ellipse to its parametric angle.
func ellipseParam(wr, hr, ang float64) float64 {
	a := rad(ang)
	return math.Atan2(wr*math.Sin(a), hr*math.Cos(a))
}

// arcTo appends an elliptical arc starting at the current point (cx, cy) and returns the end point.
func arcTo(b *strings.Builder, cx, cy, wr, hr, st, sw, ox, oy, sx, sy float64) (float64, float64) {
	if wr == 0 || hr == 0 {
		return cx, cy
	}
	t1 := ellipseParam(wr, hr, st)
	ctrX, ctrY := cx-wr*math.Cos(t1), cy-hr*math.Sin(t1)
	// split sweeps of 180 degrees or more so that every SVG arc is unambiguous
	steps := int(math.Ceil(math.Abs(sw) / (179 * angUnit)))
	if steps < 1 {
		steps = 1
	}
	var ex, ey float64
	for i := 1; i <= steps; i++ {
		ang := st + sw*float64(i)/float64(steps)
		t := ellipseParam(wr, hr, ang)
		ex, ey = ctrX+wr*math.Cos(t), ctrY+hr*math.Sin(t)
		sweep := "1"
		if sw < 0 {
			sweep = "0"
		}
		fmt.Fprintf(b, "A%s %s 0 0 %s %s %s ", num(wr*math.Abs(sx)), num(hr*math.Abs(sy)), sweep, num(ox+ex*sx), num(oy+ey*sy))
	}
	return ex, ey
}

func num(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "0"
	}
	s := strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64)
	if s == "-0" {
		return "0"
	}
	return s
}
