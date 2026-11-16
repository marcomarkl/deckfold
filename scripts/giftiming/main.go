// Command giftiming writes the page for the browser test that compares how fast an animated GIF
// and its WebP file play (tests/e2e/gif-timing.spec.mjs).
//
// For each frame duration it makes a GIF of three frames, red, green and blue in the middle of a
// grey gradient, converts it the way deckfold does (internal/media) and puts both pictures into
// one HTML page as data URIs, so that a canvas may read their pixels on file://. It stops when a
// GIF is not converted, because the test would then compare a GIF with itself.
//
//	go run ./scripts/giftiming -out tests/.out/giftiming
package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"strings"

	"deckfold/internal/media"
)

// Frame durations in hundredths of a second, the unit of the GIF format. A GIF with a frame
// under 20 ms is not converted (decision E17), so 2 is the shortest.
var delays = []int{50, 20, 10, 5, 2}

const w, h = 320, 200

func picture(delay int) ([]byte, error) {
	pal := color.Palette{color.RGBA{220, 40, 40, 255}, color.RGBA{40, 180, 60, 255}, color.RGBA{40, 60, 220, 255}}
	for i := 0; i < 200; i++ {
		v := uint8(40 + i)
		pal = append(pal, color.RGBA{v, v, v, 255})
	}
	g := &gif.GIF{LoopCount: 0, Config: image.Config{Width: w, Height: h, ColorModel: pal}}
	for f := 0; f < 3; f++ {
		img := image.NewPaletted(image.Rect(0, 0, w, h), pal)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				c := uint8(3 + (x+y)%200)
				if x >= w/2-50 && x < w/2+50 && y >= h/2-50 && y < h/2+50 {
					c = uint8(f)
				}
				img.Pix[y*img.Stride+x] = c
			}
		}
		g.Image = append(g.Image, img)
		g.Delay = append(g.Delay, delay)
		g.Disposal = append(g.Disposal, gif.DisposalNone)
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, g); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func run(out string) error {
	opt := media.New()
	if err := opt.WebPOff(); err != nil {
		return err
	}
	var page strings.Builder
	page.WriteString("<!doctype html><meta charset=\"utf-8\"><title>GIF and WebP timing</title>\n")
	for _, d := range delays {
		data, err := picture(d)
		if err != nil {
			return err
		}
		res := opt.Optimize(data)
		if !res.Changed || !res.Animated || res.Mime != "image/webp" {
			return fmt.Errorf("the GIF with frames of %d ms was not converted (%d bytes, note %q)", d*10, len(data), res.Note)
		}
		ms := d * 10
		fmt.Fprintf(&page, "<img data-kind=\"gif\" data-ms=\"%d\" src=\"data:image/gif;base64,%s\">\n", ms, base64.StdEncoding.EncodeToString(data))
		fmt.Fprintf(&page, "<img data-kind=\"webp\" data-ms=\"%d\" src=\"data:image/webp;base64,%s\">\n", ms, base64.StdEncoding.EncodeToString(res.Data))
		fmt.Printf("%d ms: GIF %d bytes, WebP %d bytes\n", ms, len(data), len(res.Data))
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "index.html"), []byte(page.String()), 0o644)
}

func main() {
	out := flag.String("out", "", "folder for index.html")
	flag.Parse()
	if *out == "" {
		fmt.Fprintln(os.Stderr, "usage: giftiming -out <folder>")
		os.Exit(3)
	}
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
