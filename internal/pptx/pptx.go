// Package pptx converts a PowerPoint file (.pptx, .ppsx, .pptm) into an HTML deck with
// deckfold's own player. The deck is written into a folder that the regular pipeline then
// embeds into one self-contained file.
package pptx

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"deckfold/internal/assets"
	"deckfold/internal/pptx/deck"
	"deckfold/internal/pptx/fonts"
	"deckfold/internal/pptx/model"
	"deckfold/internal/pptx/opc"
	"deckfold/internal/pptx/render"
	"deckfold/internal/report"
	dfruntime "deckfold/runtime"
)

// Extensions handled by this package.
var Extensions = map[string]bool{".pptx": true, ".ppsx": true, ".pptm": true, ".potx": false}

// ErrUnsupported marks an input deckfold cannot read (legacy .ppt, encrypted file); exit code 2.
var ErrUnsupported = errors.New("unsupported PowerPoint file")

// IsPowerPoint reports whether a path should go through this package: by extension, or by
// content for the legacy .ppt format.
func IsPowerPoint(path string, head []byte) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if Extensions[ext] || ext == ".ppt" || ext == ".pps" {
		return true
	}
	return false
}

// Options configure the conversion.
type Options struct {
	// BundledFontsOnly ignores installed fonts (reproducible output for tests).
	BundledFontsOnly bool
	// FontDirs overrides the font folders to scan.
	FontDirs []string
	// AllowOutside lists folders outside the presentation's folder that linked files may come
	// from (--allow-outside).
	AllowOutside []string
	// Fetcher downloads linked pictures with an http(s) URL; nil reports them as missing.
	Fetcher *assets.Fetcher
	// Lenient writes the deck even when linked files are missing (--lenient, E10).
	Lenient bool
}

// MissingError lists linked files that could not be loaded (exit code 1 unless --lenient).
type MissingError struct{ Missing []report.Missing }

func (e *MissingError) Error() string {
	return fmt.Sprintf("%d linked files of the presentation cannot be loaded", len(e.Missing))
}

// Result reports what the conversion did.
type Result struct {
	Slides   int
	Hidden   int
	Warnings []string
	Fonts    []string // one line per font: family, source
}

// Convert reads the presentation at src and writes the deck into dir.
func Convert(src, dir string, opt Options) (*Result, error) {
	pkg, err := opc.Open(src)
	if err != nil {
		if errors.Is(err, opc.ErrCFB) || errors.Is(err, opc.ErrNotZip) {
			return nil, fmt.Errorf("%w: %v", ErrUnsupported, err)
		}
		return nil, err
	}
	defer pkg.Close()
	pres, err := model.Read(pkg)
	if err != nil {
		if errors.Is(err, model.ErrStructure) {
			return nil, fmt.Errorf("%w: %v", ErrUnsupported, err)
		}
		return nil, err
	}
	res := &Result{}
	warn := func(format string, a ...any) { res.Warnings = append(res.Warnings, fmt.Sprintf(format, a...)) }

	embedded := map[string][]byte{}
	for _, ef := range pres.EmbeddedFonts {
		data, err := pkg.Read(ef.Part)
		if err != nil {
			warn("embedded font %s cannot be read: %v", ef.Typeface, err)
			continue
		}
		plain, err := fonts.Deobfuscate(data, ef.Part)
		if err != nil {
			warn("embedded font %s cannot be read: %v", ef.Typeface, err)
			continue
		}
		embedded[ef.Typeface+"|"+ef.Style] = plain
	}
	var missing []report.Missing
	linked, err := linker(src, opt)
	if err != nil {
		return nil, err
	}
	fr := fonts.NewResolver(embedded)
	fr.BundledOnly = opt.BundledFontsOnly
	if opt.FontDirs != nil {
		fr.Dirs = opt.FontDirs
	}
	out, err := render.Render(pres, render.Options{Fonts: fr, Warn: warn, Linked: linked, AssetDir: filepath.Join(dir, "assets"),
		Missing: func(target, where string, err error) {
			missing = append(missing, report.Missing{Ref: target, Where: filepath.Base(src) + ": " + where, Reason: err.Error()})
		}})
	if err != nil {
		return nil, err
	}
	if len(missing) > 0 {
		if !opt.Lenient {
			return nil, &MissingError{Missing: missing}
		}
		for _, m := range missing {
			warn("%s: linked file %s is missing (%s); left out", m.Where, m.Ref, m.Reason)
		}
	}
	res.Warnings = append(res.Warnings, fr.Warnings...)

	var dfonts []deck.Font
	for _, fu := range out.Faces {
		data, _, err := fu.Face.Subset()
		if err != nil {
			warn("font %s cannot be subset (%v); it is left out", fu.Face.Family, err)
			continue
		}
		name, err := out.AddAsset(data, "ttf", filepath.Join(dir, "assets"))
		if err != nil {
			return nil, err
		}
		dfonts = append(dfonts, deck.Font{Alias: fu.Alias, File: name})
		line := fmt.Sprintf("%s: %s (%s)", fu.Face.Family, fu.Face.Source, filepath.Base(fu.Face.File))
		if fu.Face.Subst {
			line += ", substitute"
			warn("font %s is not available; %s is used instead, so line breaks can differ from PowerPoint", fu.Face.Family, filepath.Base(fu.Face.File))
		}
		res.Fonts = appendUnique(res.Fonts, line)
	}

	data := deck.Data{Version: 1, Title: pres.Title, Width: out.Width, Height: out.Height, Loop: pres.Loop, UseTimings: pres.UseTimings}
	for _, s := range out.Slides {
		a := s.Anim
		sd := deck.SlideData{Number: s.Number, Label: s.Label, Notes: s.Notes, Steps: len(a.Steps), Shapes: s.Shapes, Anim: &a, Advance: a.Advance}
		if sd.Steps < 1 {
			sd.Steps = 1
		}
		data.Slides = append(data.Slides, sd)
	}
	ar := out.AnimReport
	if ar.Triggers {
		warn("animations started by clicking a shape (triggers) are not played")
	}
	if ar.Iterate {
		warn("animations by word or by letter are played by paragraph")
	}
	for _, t := range ar.Fallback {
		warn("the %s transition is played as a fade", t)
	}
	for _, u := range ar.Unknown {
		warn("the %s animation is not played", u)
	}
	if data.Title == "" {
		data.Title = strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	}
	for _, s := range pres.Slides {
		if s.Hidden {
			res.Hidden++
		}
	}
	res.Slides = len(out.Slides)
	if res.Slides == 0 {
		return nil, fmt.Errorf("the presentation has no visible slides")
	}
	if err := deck.Write(dir, out, data, dfonts, dfruntime.PptxJS); err != nil {
		return nil, err
	}
	return res, nil
}

func appendUnique(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}

// linker returns the loader for linked files: local paths go through assets.Resolver with the
// folder of the presentation as the boundary, http(s) URLs through the fetcher.
func linker(src string, opt Options) (func(string) ([]byte, string, error), error) {
	dir, err := filepath.Abs(filepath.Dir(src))
	if err != nil {
		return nil, err
	}
	res, err := assets.NewResolver(dir, opt.AllowOutside)
	if err != nil {
		return nil, err
	}
	return func(target string) ([]byte, string, error) {
		t := strings.TrimSpace(target)
		low := strings.ToLower(t)
		switch {
		case strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://"):
			if opt.Fetcher == nil {
				return nil, "", fmt.Errorf("downloads are not enabled")
			}
			data, ct, err := opt.Fetcher.Get(t)
			if err != nil {
				return nil, "", err
			}
			ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(strings.SplitN(t, "?", 2)[0])), ".")
			if ext == "" {
				if i := strings.LastIndex(ct, "/"); i >= 0 {
					ext = strings.SplitN(ct[i+1:], ";", 2)[0]
				}
			}
			return data, ext, nil
		case strings.HasPrefix(low, "file:"):
			u, err := url.Parse(t)
			if err != nil {
				return nil, "", err
			}
			p := u.Path
			if len(p) > 2 && p[0] == '/' && p[2] == ':' { // file:///C:/...
				p = p[1:]
			}
			t = p
		}
		r, err := res.Resolve(t, dir)
		if err != nil {
			return nil, "", err
		}
		data, err := os.ReadFile(r.Path)
		if err != nil {
			return nil, "", err
		}
		return data, strings.TrimPrefix(strings.ToLower(filepath.Ext(r.Path)), "."), nil
	}, nil
}
