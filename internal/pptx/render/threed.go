package render

import (
	"encoding/json"
	"fmt"
	"math"

	"deckfold/internal/pptx/xmltree"
)

// threeD holds the 3D properties of a shape (a:scene3d and a:sp3d), in px and degrees. The
// renderer draws an SVG approximation (a lighting filter for the bevel, a band for the contour,
// an offset copy for a visible extrusion); the player draws the shape with three.js from the
// same data when WebGL is available (data-df-pptx-3d).
type threeD struct {
	BevelT       *bevel    `json:"bevelT,omitempty"`
	BevelB       *bevel    `json:"bevelB,omitempty"`
	Depth        float64   `json:"depth,omitempty"`
	DepthColor   string    `json:"depthColor,omitempty"`
	Contour      float64   `json:"contour,omitempty"`
	ContourColor string    `json:"contourColor,omitempty"`
	Material     string    `json:"material"`
	Camera       string    `json:"camera"`
	Rot          []float64 `json:"rot,omitempty"` // camera rotation: latitude, longitude, revolution
	Fov          float64   `json:"fov,omitempty"`
	Light        string    `json:"light"`
	LightDir     string    `json:"lightDir"`
	LightRev     float64   `json:"lightRev,omitempty"`
}

type bevel struct {
	W    float64 `json:"w"`
	H    float64 `json:"h"`
	Prst string  `json:"prst"`
}

// frontal reports whether the camera looks straight at the slide, so that depth is not visible.
func (t *threeD) frontal() bool {
	if t.Camera != "orthographicFront" && t.Camera != "perspectiveFront" {
		return false
	}
	for _, v := range t.Rot {
		if math.Mod(math.Abs(v), 360) > 0.01 {
			return false
		}
	}
	return true
}

// shape3D reads the 3D properties of a shape: a:sp3d and a:scene3d from the spPr chain, otherwise
// from the theme's effect style that the shape style references. nil when the shape is flat.
func (r *slideRenderer) shape3D(sppr []*xmltree.Node, ref *xmltree.Node, cx colorCtx) *threeD {
	var sp3d, scene *xmltree.Node
	for _, n := range sppr {
		if sp3d == nil {
			sp3d = n.Child("sp3d")
		}
		if scene == nil {
			scene = n.Child("scene3d")
		}
	}
	if ref != nil && (sp3d == nil || scene == nil) {
		idx := int(ref.Int("idx", 0))
		th := r.slide.Layout.Master.Theme
		if idx >= 1 && idx <= len(th.EffectStyles) {
			es := th.EffectStyles[idx-1]
			ph := cx.colorChild(ref)
			cx.ph = &ph
			if sp3d == nil {
				sp3d = es.Child("sp3d")
			}
			if scene == nil {
				scene = es.Child("scene3d")
			}
		}
	}
	t := &threeD{Material: "warmMatte", Camera: "orthographicFront", Light: "threePt", LightDir: "t"}
	if cam := scene.Child("camera"); cam != nil {
		if p := cam.Attr("prst"); p != "" {
			t.Camera = p
		}
		if rot := cam.Child("rot"); rot != nil {
			t.Rot = []float64{float64(rot.Int("lat", 0)) / 60000, float64(rot.Int("lon", 0)) / 60000, float64(rot.Int("rev", 0)) / 60000}
		}
		if fov := cam.Attr("fov"); fov != "" {
			t.Fov = float64(cam.Int("fov", 0)) / 60000
		}
	}
	if lr := scene.Child("lightRig"); lr != nil {
		if v := lr.Attr("rig"); v != "" {
			t.Light = v
		}
		if v := lr.Attr("dir"); v != "" {
			t.LightDir = v
		}
		if rot := lr.Child("rot"); rot != nil {
			t.LightRev = float64(rot.Int("rev", 0)) / 60000
		}
	}
	if sp3d != nil {
		readBevel := func(n *xmltree.Node) *bevel {
			if n == nil {
				return nil
			}
			b := &bevel{W: float64(n.Int("w", 76200)) / emuPx, H: float64(n.Int("h", 76200)) / emuPx, Prst: n.Attr("prst")}
			if b.Prst == "" {
				b.Prst = "circle"
			}
			if b.W <= 0 || b.H <= 0 {
				return nil
			}
			return b
		}
		t.BevelT, t.BevelB = readBevel(sp3d.Child("bevelT")), readBevel(sp3d.Child("bevelB"))
		t.Depth = float64(sp3d.Int("extrusionH", 0)) / emuPx
		t.Contour = float64(sp3d.Int("contourW", 0)) / emuPx
		if m := sp3d.Attr("prstMaterial"); m != "" {
			t.Material = m
		}
		if c := cx.colorChild(sp3d.Child("extrusionClr")); c.OK {
			t.DepthColor = c.Hex()
		}
		if c := cx.colorChild(sp3d.Child("contourClr")); c.OK {
			t.ContourColor = c.Hex()
		}
	}
	if t.BevelT == nil && t.BevelB == nil && t.Depth == 0 && t.Contour == 0 && t.frontal() {
		return nil
	}
	return t
}

// attr returns the data attribute the player reads.
func (t *threeD) attr() string {
	js, _ := json.Marshal(t)
	return ` data-df-pptx-3d="` + esc(string(js)) + `"`
}

// lightAngles returns the direction of the key light in SVG filter terms: the azimuth (degrees
// clockwise from the x axis, y pointing down) and the elevation.
func (t *threeD) lightAngles() (az, el float64) {
	az = map[string]float64{"t": 270, "tl": 225, "tr": 315, "l": 180, "r": 0, "b": 90, "bl": 135, "br": 45}[t.LightDir]
	if _, ok := map[string]bool{"t": true, "tl": true, "tr": true, "l": true, "r": true, "b": true, "bl": true, "br": true}[t.LightDir]; !ok {
		az = 270
	}
	az = math.Mod(az+t.LightRev, 360)
	switch t.Light {
	case "harsh", "contrasting", "sunrise", "sunset":
		el = 35
	case "soft", "balanced", "brightRoom", "glow":
		el = 55
	default:
		el = 45
	}
	return az, el
}

// bevelFilter returns SVG filter primitives that light the edge of the shape like a bevel seen
// from the front: the blurred alpha channel is the height field. Diffuse light is scaled so that
// flat areas keep their color; the specular highlight is shifted so that flat areas get none.
func (t *threeD) bevelFilter() string {
	b := t.BevelT
	if b == nil {
		return ""
	}
	az, el := t.lightAngles()
	spec, exp := 0.35, 20.0
	switch t.Material {
	case "matte", "flat", "powder", "dkEdge":
		spec = 0
	case "plastic", "clear", "translucentPowder":
		spec, exp = 0.7, 25
	case "metal", "softmetal":
		spec, exp = 1, 12
	}
	if t.Light == "flat" {
		return ""
	}
	sigma := math.Max(0.5, b.W/2)
	if b.Prst == "angle" || b.Prst == "coolSlant" || b.Prst == "slope" || b.Prst == "hardEdge" {
		sigma = math.Max(0.5, b.W/3) // harder edge
	}
	scale := b.H
	if b.Prst == "relaxedInset" || b.Prst == "softRound" {
		scale = b.H * 0.6
	}
	if b.Prst == "divot" || b.Prst == "riblet" || b.Prst == "artDeco" || b.Prst == "cross" {
		scale = -b.H * 0.5 // an inward profile: the light falls on the other side
	}
	flat := math.Sin(el * math.Pi / 180)
	light := fmt.Sprintf(`<feDistantLight azimuth="%s" elevation="%s"/>`, f(az), f(el))
	s := fmt.Sprintf(`<feGaussianBlur in="SourceAlpha" stdDeviation="%s" result="bvA"/>`, f(sigma))
	s += fmt.Sprintf(`<feDiffuseLighting in="bvA" surfaceScale="%s" diffuseConstant="1" lighting-color="#fff" result="bvD">%s</feDiffuseLighting>`, f(scale), light)
	// ambient share of the light, so that faces turned away from the light are not black
	const amb = 0.35
	s += fmt.Sprintf(`<feComponentTransfer in="bvD" result="bvD2"><feFuncR type="linear" slope="%[1]s" intercept="%[2]s"/><feFuncG type="linear" slope="%[1]s" intercept="%[2]s"/><feFuncB type="linear" slope="%[1]s" intercept="%[2]s"/></feComponentTransfer>`, f((1-amb)/flat), f(amb))
	s += `<feComposite in="SourceGraphic" in2="bvD2" operator="arithmetic" k1="1" result="bvM"/>`
	if spec > 0 {
		// specular level of a flat area: N.H with H halfway between the light and the viewer
		half := (90 - el) / 2 * math.Pi / 180
		base := spec * math.Pow(math.Cos(half), exp)
		s += fmt.Sprintf(`<feSpecularLighting in="bvA" surfaceScale="%s" specularConstant="%s" specularExponent="%s" lighting-color="#fff" result="bvS">%s</feSpecularLighting>`, f(scale), f(spec), f(exp), light)
		s += fmt.Sprintf(`<feComponentTransfer in="bvS" result="bvS2"><feFuncR type="linear" intercept="%[1]s"/><feFuncG type="linear" intercept="%[1]s"/><feFuncB type="linear" intercept="%[1]s"/><feFuncA type="linear" intercept="%[1]s"/></feComponentTransfer>`, f(-base))
		s += `<feComposite in="bvS2" in2="SourceAlpha" operator="in" result="bvS3"/>`
		s += `<feComposite in="bvM" in2="bvS3" operator="arithmetic" k2="1" k3="1" result="bvM"/>`
	}
	return s + `<feComposite in="bvM" in2="SourceGraphic" operator="in" result="bevel"/>`
}

// behind returns SVG drawn behind the shape: the contour band, and for a camera that is not
// frontal the side of the extrusion as an offset copy.
func (t *threeD) behind(path, fill, ft string) string {
	s := ""
	if t.Depth > 0 && !t.frontal() {
		lat, lon := 20.0, -20.0
		if len(t.Rot) == 3 {
			lat, lon = t.Rot[0], t.Rot[1]
		}
		dx := -t.Depth * math.Sin(lon*math.Pi/180)
		dy := t.Depth * math.Sin(lat*math.Pi/180)
		side := fill
		if t.DepthColor != "" {
			side = `fill="` + t.DepthColor + `"`
		}
		s += fmt.Sprintf(`<g transform="translate(%s %s)" data-df-pptx-3d-base=""><path d="%s" %s%s/><path d="%s" fill="#000" fill-opacity="0.3"%s/></g>`, f(dx), f(dy), path, side, ft, path, ft)
	}
	if t.Contour > 0 {
		c := t.ContourColor
		if c == "" {
			c = "#000000"
		}
		s += fmt.Sprintf(`<path d="%s" fill="none" stroke="%s" stroke-width="%s" stroke-linejoin="round"%s/>`, path, c, f(2*t.Contour), ft)
	}
	return s
}
