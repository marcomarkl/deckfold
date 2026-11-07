package render

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"deckfold/internal/pptx/model"
	"deckfold/internal/pptx/xmltree"
)

// Color is a resolved color.
type Color struct {
	R, G, B float64 // 0..1
	A       float64 // 0..1
	OK      bool
}

// Hex returns #rrggbb.
func (c Color) Hex() string {
	return fmt.Sprintf("#%02x%02x%02x", clamp8(c.R), clamp8(c.G), clamp8(c.B))
}

func clamp8(v float64) int {
	return int(math.Round(math.Max(0, math.Min(1, v)) * 255))
}

// ctx carries what color resolution needs: theme, color map and the placeholder color (phClr)
// of a style reference.
type colorCtx struct {
	theme  *model.Theme
	clrMap map[string]string
	ph     *Color
}

var presetColors = map[string]string{
	"black": "000000", "white": "FFFFFF", "red": "FF0000", "green": "008000", "blue": "0000FF",
	"yellow": "FFFF00", "cyan": "00FFFF", "magenta": "FF00FF", "gray": "808080", "grey": "808080",
	"darkGray": "A9A9A9", "lightGray": "D3D3D3", "orange": "FFA500", "purple": "800080",
	"navy": "000080", "teal": "008080", "silver": "C0C0C0", "maroon": "800000", "olive": "808000",
	"lime": "00FF00", "aqua": "00FFFF", "fuchsia": "FF00FF", "brown": "A52A2A", "pink": "FFC0CB",
}

func hexColor(s string) Color {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return Color{}
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return Color{}
	}
	return Color{R: float64(v>>16&255) / 255, G: float64(v>>8&255) / 255, B: float64(v&255) / 255, A: 1, OK: true}
}

// colorOf resolves a color element (srgbClr, schemeClr, sysClr, prstClr, scrgbClr, hslClr)
// including its modifiers.
func (cx colorCtx) colorOf(n *xmltree.Node) Color {
	if n == nil {
		return Color{}
	}
	var c Color
	switch n.Name {
	case "srgbClr":
		c = hexColor(n.Attr("val"))
	case "sysClr":
		c = hexColor(n.Attr("lastClr"))
		if !c.OK {
			if n.Attr("val") == "window" {
				c = hexColor("FFFFFF")
			} else {
				c = hexColor("000000")
			}
		}
	case "prstClr":
		c = hexColor(presetColors[n.Attr("val")])
	case "scrgbClr":
		c = Color{R: linToSRGB(float64(n.Int("r", 0)) / 100000), G: linToSRGB(float64(n.Int("g", 0)) / 100000), B: linToSRGB(float64(n.Int("b", 0)) / 100000), A: 1, OK: true}
	case "hslClr":
		r, g, b := hslToRGB(float64(n.Int("hue", 0))/60000/360, float64(n.Int("sat", 0))/100000, float64(n.Int("lum", 0))/100000)
		c = Color{R: r, G: g, B: b, A: 1, OK: true}
	case "schemeClr":
		v := n.Attr("val")
		if v == "phClr" {
			if cx.ph != nil {
				c = *cx.ph
			} else {
				c = hexColor("000000")
			}
			break
		}
		if m, ok := cx.clrMap[v]; ok {
			v = m
		}
		if cx.theme != nil {
			c = hexColor(cx.theme.Colors[v])
		}
	default:
		return Color{}
	}
	if !c.OK {
		return c
	}
	return applyMods(c, n.Kids)
}

// colorChild resolves the first color element among the children of n.
func (cx colorCtx) colorChild(n *xmltree.Node) Color {
	if n == nil {
		return Color{}
	}
	for _, k := range n.Kids {
		if c := cx.colorOf(k); c.OK {
			return c
		}
	}
	return Color{}
}

func applyMods(c Color, mods []*xmltree.Node) Color {
	for _, m := range mods {
		v := float64(m.Int("val", 0)) / 100000
		switch m.Name {
		case "alpha":
			c.A = v
		case "alphaMod":
			c.A *= v
		case "alphaOff":
			c.A += v
		case "lumMod", "lumOff", "satMod", "satOff", "hueMod", "hueOff", "sat", "lum", "hue":
			h, s, l := rgbToHSL(c.R, c.G, c.B)
			switch m.Name {
			case "lumMod":
				l *= v
			case "lumOff":
				l += v
			case "satMod":
				s *= v
			case "satOff":
				s += v
			case "hueMod":
				h *= v
			case "hueOff":
				h += float64(m.Int("val", 0)) / 60000 / 360
			case "sat":
				s = v
			case "lum":
				l = v
			case "hue":
				h = float64(m.Int("val", 0)) / 60000 / 360
			}
			h = h - math.Floor(h)
			c.R, c.G, c.B = hslToRGB(h, math.Max(0, math.Min(1, s)), math.Max(0, math.Min(1, l)))
		case "tint":
			// PowerPoint applies tint and shade in linear RGB.
			c.R = linToSRGB(sRGBToLin(c.R)*v + (1 - v))
			c.G = linToSRGB(sRGBToLin(c.G)*v + (1 - v))
			c.B = linToSRGB(sRGBToLin(c.B)*v + (1 - v))
		case "shade":
			c.R = linToSRGB(sRGBToLin(c.R) * v)
			c.G = linToSRGB(sRGBToLin(c.G) * v)
			c.B = linToSRGB(sRGBToLin(c.B) * v)
		case "inv":
			c.R, c.G, c.B = 1-c.R, 1-c.G, 1-c.B
		case "gray":
			g := 0.3*c.R + 0.59*c.G + 0.11*c.B
			c.R, c.G, c.B = g, g, g
		case "comp":
			h, s, l := rgbToHSL(c.R, c.G, c.B)
			c.R, c.G, c.B = hslToRGB(math.Mod(h+0.5, 1), s, l)
		}
	}
	c.A = math.Max(0, math.Min(1, c.A))
	return c
}

func sRGBToLin(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func linToSRGB(v float64) float64 {
	v = math.Max(0, math.Min(1, v))
	if v <= 0.0031308 {
		return v * 12.92
	}
	return 1.055*math.Pow(v, 1/2.4) - 0.055
}

func rgbToHSL(r, g, b float64) (h, s, l float64) {
	mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l = (mx + mn) / 2
	if mx == mn {
		return 0, 0, l
	}
	d := mx - mn
	if l > 0.5 {
		s = d / (2 - mx - mn)
	} else {
		s = d / (mx + mn)
	}
	switch mx {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return h / 6, s, l
}

func hslToRGB(h, s, l float64) (r, g, b float64) {
	if s == 0 {
		return l, l, l
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	f := func(t float64) float64 {
		t = t - math.Floor(t)
		switch {
		case t < 1.0/6:
			return p + (q-p)*6*t
		case t < 0.5:
			return q
		case t < 2.0/3:
			return p + (q-p)*(2.0/3-t)*6
		}
		return p
	}
	return f(h + 1.0/3), f(h), f(h - 1.0/3)
}
