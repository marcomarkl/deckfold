package render

import (
	"bytes"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"deckfold/internal/pptx/geom"
	"deckfold/internal/pptx/xmltree"
)

// geometry returns the SVG path of a shape and its text rectangle. Without geometry the shape
// is a rectangle.
func (r *slideRenderer) geometry(g *xmltree.Node, box rect) (string, rect) {
	rectPath := "M" + f(box.x) + " " + f(box.y) + "H" + f(box.x+box.w) + "V" + f(box.y+box.h) + "H" + f(box.x) + "Z"
	if g == nil {
		return rectPath, box
	}
	var def *geom.Def
	switch g.Name {
	case "prstGeom":
		name := g.Attr("prst")
		if name == "rect" || name == "" {
			return rectPath, box
		}
		d, ok := geom.Preset(name)
		if !ok {
			r.warnOnce("unknown preset shape %q is drawn as a rectangle", name)
			return rectPath, box
		}
		def = d
	case "custGeom":
		def = geom.Custom(g)
	default:
		return rectPath, box
	}
	res, err := def.Eval(box.x, box.y, box.w, box.h, g.Child("avLst"))
	if err != nil {
		r.warnOnce("shape geometry cannot be evaluated (%v); drawn as a rectangle", err)
		return rectPath, box
	}
	path := res.Fill
	if path == "" {
		path = res.Stroke
	}
	tr := rect{box.x + res.TL, box.y + res.TT, res.TR - res.TL, res.TB - res.TT}
	if tr.w <= 0 || tr.h <= 0 {
		tr = box
	}
	return path, tr
}

// imageSize returns the pixel size of an image, or 0, 0.
func imageSize(data []byte) (float64, float64) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}
	return float64(cfg.Width), float64(cfg.Height)
}
