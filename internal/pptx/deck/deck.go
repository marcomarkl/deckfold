// Package deck writes the HTML deck deckfold generates from a presentation: index.html with one
// <section class="df-pptx-slide"> per slide, the deck data (#df-pptx-deck), the player and an
// assets folder. The regular pipeline (detect, adapter "pptx", embed) then turns that folder
// into the self-contained output.
package deck

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"deckfold/internal/inject"
	"deckfold/internal/pptx/anim"
	"deckfold/internal/pptx/render"
)

// Marker comments around the player; --refresh replaces the player code between them
// (internal/inject.ReplacePlayer).
const (
	PlayerBegin = inject.PlayerBegin
	PlayerEnd   = inject.PlayerEnd
	// PlayerID is the id of the player script.
	PlayerID = "df-pptx-player"
	// DataID is the id of the deck data block.
	DataID = "df-pptx-deck"
	// SlideClass is the class of a slide section.
	SlideClass = "df-pptx-slide"
)

// Data is the deck description the player and the Go adapter read.
type Data struct {
	Version    int         `json:"version"`
	Title      string      `json:"title"`
	Width      float64     `json:"width"`
	Height     float64     `json:"height"`
	Loop       bool        `json:"loop,omitempty"`
	UseTimings bool        `json:"useTimings"`
	Slides     []SlideData `json:"slides"`
}

// SlideData describes one slide.
type SlideData struct {
	Number  int         `json:"number"` // position in the PowerPoint file
	Label   string      `json:"label,omitempty"`
	Notes   string      `json:"notes,omitempty"`
	Steps   int         `json:"steps"`
	Auto    []bool      `json:"auto,omitempty"`
	Delay   []float64   `json:"delay,omitempty"`
	Advance *float64    `json:"advance,omitempty"` // automatic advance after seconds (advTm)
	Shapes  []string    `json:"shapes,omitempty"`
	Anim    *anim.Slide `json:"anim,omitempty"`
}

// Fonts carries the embedded font files: CSS alias -> file name in assets.
type Font struct {
	Alias string
	File  string
}

// Write writes the deck into dir (which must exist and be empty). threeJS is the three.js
// bundle; it is embedded only when the deck has 3D content (res.WebGL).
func Write(dir string, res *render.Result, data Data, fonts []Font, playerJS, threeJS string) error {
	assets := filepath.Join(dir, "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		return err
	}
	names := make([]string, 0, len(res.Assets))
	for n := range res.Assets {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if strings.ContainsAny(n, `/\`) || strings.HasPrefix(n, ".") {
			return fmt.Errorf("invalid asset name %q", n)
		}
		if res.Assets[n] == nil {
			continue // already written by the renderer (render.Options.AssetDir)
		}
		if err := os.WriteFile(filepath.Join(assets, n), res.Assets[n], 0o644); err != nil {
			return err
		}
	}
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	b.WriteString("<meta name=\"deckfold:source\" content=\"pptx\">\n")
	b.WriteString("<title>" + htmlEsc(data.Title) + "</title>\n")
	if len(fonts) > 0 {
		b.WriteString("<style>\n")
		for _, f := range fonts {
			fmt.Fprintf(&b, "@font-face{font-family:%q;src:url(\"assets/%s\") format(\"truetype\");font-display:block}\n", f.Alias, f.File)
		}
		b.WriteString("</style>\n")
	}
	b.WriteString("</head>\n<body>\n")
	fmt.Fprintf(&b, "<div id=\"df-pptx-stage\" style=\"width:%spx;height:%spx\">\n", num(res.Width), num(res.Height))
	for i, s := range res.Slides {
		fmt.Fprintf(&b, "<section class=\"%s\" data-index=\"%d\" style=\"position:relative;width:%spx;height:%spx;overflow:hidden\">%s</section>\n",
			SlideClass, i, num(res.Width), num(res.Height), s.SVG)
	}
	b.WriteString("</div>\n")
	js, err := json.Marshal(data)
	if err != nil {
		return err
	}
	// JSON inside <script>: "</" must not appear literally
	safe := strings.ReplaceAll(string(js), "</", `<\/`)
	safe = strings.ReplaceAll(safe, "<!--", `<\!--`)
	fmt.Fprintf(&b, "<script type=\"application/json\" id=\"%s\">%s</script>\n", DataID, safe)
	if res.WebGL && threeJS != "" {
		b.WriteString(inject.ThreeBegin + "<script id=\"df-pptx-three\">" + threeJS + "</script>" + inject.ThreeEnd + "\n")
	}
	b.WriteString(PlayerBegin + PlayerScript(playerJS) + PlayerEnd + "\n")
	b.WriteString("</body>\n</html>\n")
	return os.WriteFile(filepath.Join(dir, "index.html"), []byte(b.String()), 0o644)
}

// PlayerScript returns the script element of the player.
func PlayerScript(js string) string {
	return "<script id=\"" + PlayerID + "\">" + js + "</script>"
}

func num(v float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", v), "0"), ".")
}

func htmlEsc(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}
