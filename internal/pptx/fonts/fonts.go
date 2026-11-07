// Package fonts finds the font files for the typefaces of a presentation and subsets them for
// embedding. The cascade is: fonts embedded in the presentation, then installed fonts, then the
// OFL fonts bundled with deckfold (Carlito, Liberation). The embedding permissions in the OS/2
// table (fsType) are honoured.
package fonts

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	tfont "github.com/tdewolff/font"
)

//go:embed data/*.ttf.gz
var bundled embed.FS

// Source says where a face came from.
type Source string

const (
	Embedded  Source = "embedded"
	Installed Source = "installed"
	Bundled   Source = "bundled"
)

// fsType bits (OS/2 table).
const (
	fsRestricted = 0x0002
	fsNoSubset   = 0x0100
	fsBitmapOnly = 0x0200
)

// Face is a loaded font face.
type Face struct {
	Family   string // family as requested by the presentation
	File     string // file or bundle name, for the report
	Source   Source
	Bold     bool
	Italic   bool
	Subst    bool // a substitute for the requested family
	data     []byte
	sfnt     *tfont.SFNT
	upem     float64
	ascent   float64 // in em
	descent  float64 // in em (positive)
	noSubset bool
	mu       sync.Mutex
	used     map[rune]bool
}

// Ascent returns the ascent in em (OS/2 winAscent, as PowerPoint uses it for line height).
func (f *Face) Ascent() float64 { return f.ascent }

// Descent returns the descent in em, positive.
func (f *Face) Descent() float64 { return f.descent }

// Advance returns the advance width of a rune in em.
func (f *Face) Advance(r rune) float64 {
	g := f.sfnt.GlyphIndex(r)
	return float64(f.sfnt.GlyphAdvance(g)) / f.upem
}

// Has reports whether the face has a glyph for the rune.
func (f *Face) Has(r rune) bool { return f.sfnt.GlyphIndex(r) != 0 }

// Use records runes the output needs from this face.
func (f *Face) Use(s string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.used == nil {
		f.used = map[rune]bool{}
	}
	for _, r := range s {
		f.used[r] = true
	}
}

// Subset returns the font file to embed (a subset with a neutral family name, or the whole
// file when the font forbids subsetting) and the family name the CSS must use.
func (f *Face) Subset() (data []byte, family string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.noSubset {
		sum := sha256.Sum256(f.data)
		return f.data, "df-" + hex.EncodeToString(sum[:5]), nil
	}
	runes := make([]rune, 0, len(f.used))
	for r := range f.used {
		runes = append(runes, r)
	}
	sort.Slice(runes, func(i, j int) bool { return runes[i] < runes[j] })
	ids := []uint16{0}
	seen := map[uint16]bool{0: true}
	for _, r := range runes {
		if g := f.sfnt.GlyphIndex(r); !seen[g] {
			seen[g] = true
			ids = append(ids, g)
		}
	}
	sub, err := f.sfnt.Subset(ids, tfont.SubsetOptions{Tables: tfont.KeepMinTables})
	if err != nil {
		return nil, "", err
	}
	if g, ok := sub.Tables["glyf"]; ok && len(g) == 0 {
		// only empty outlines (spaces): browsers reject a zero-length glyf table, so keep one
		// visible glyph
		if dot := f.sfnt.GlyphIndex('.'); dot != 0 && !seen[dot] {
			ids = append(ids, dot)
			if sub, err = f.sfnt.Subset(ids, tfont.SubsetOptions{Tables: tfont.KeepMinTables}); err != nil {
				return nil, "", err
			}
		}
	}
	sum := sha256.Sum256(append(append([]byte{}, f.data[:min(len(f.data), 4096)]...), []byte(string(runes))...))
	family = "df-" + hex.EncodeToString(sum[:5])
	sub.Tables["name"] = nameTable(family, f.sfnt)
	delete(sub.Tables, "DSIG")
	return writeSFNT(sub.Tables, sub.IsCFF), family, nil
}

// Resolver finds faces. It is safe for use by one conversion.
type Resolver struct {
	embedded  map[string][]byte // key: lower(family)+"|"+style
	scanOnce  sync.Once
	installed map[string][]installedFont // lower(family) -> files
	cache     map[string]*Face
	// BundledOnly skips installed fonts (tests use it for reproducible output).
	BundledOnly bool
	Dirs        []string // font folders to scan (default: platform folders)
	Warnings    []string
}

type installedFont struct {
	path   string
	index  int
	bold   bool
	italic bool
}

// NewResolver returns a resolver. embedded maps "family|style" (style: regular, bold, italic,
// boldItalic) to font files from the presentation.
func NewResolver(embedded map[string][]byte) *Resolver {
	m := map[string][]byte{}
	for k, v := range embedded {
		m[strings.ToLower(k)] = v
	}
	return &Resolver{embedded: m, cache: map[string]*Face{}}
}

func styleName(bold, italic bool) string {
	switch {
	case bold && italic:
		return "boldItalic"
	case bold:
		return "bold"
	case italic:
		return "italic"
	}
	return "regular"
}

// Face returns the face for a family and style. It never fails: when nothing else is found,
// it returns a bundled substitute.
func (r *Resolver) Face(family string, bold, italic bool) *Face {
	key := strings.ToLower(family) + "|" + styleName(bold, italic)
	if f, ok := r.cache[key]; ok {
		return f
	}
	f := r.find(family, bold, italic)
	r.cache[key] = f
	return f
}

func (r *Resolver) find(family string, bold, italic bool) *Face {
	style := styleName(bold, italic)
	if data, ok := r.embedded[strings.ToLower(family)+"|"+style]; ok {
		if f, err := load(data, 0, family, "embedded:"+family+" "+style, Embedded, bold, italic); err == nil && f.allowed() {
			return f
		} else if err != nil {
			r.warn("embedded font %s (%s) cannot be read: %v", family, style, err)
		}
	}
	if !r.BundledOnly {
		r.scanOnce.Do(r.scan)
		if cands := r.installed[strings.ToLower(family)]; len(cands) > 0 {
			best := cands[0]
			for _, c := range cands {
				if c.bold == bold && c.italic == italic {
					best = c
					break
				}
			}
			if data, err := os.ReadFile(best.path); err == nil {
				if f, err := load(data, best.index, family, best.path, Installed, bold, italic); err == nil {
					if f.allowed() {
						return f
					}
					r.warn("font %s may not be embedded (fsType 0x%04x); a substitute is used", family, f.sfnt.OS2.FsType)
				}
			}
		}
	}
	name, exact := bundledFor(family)
	data := bundledData(name, bold, italic)
	f, err := load(data, 0, family, name, Bundled, bold, italic)
	if err != nil {
		panic(fmt.Sprintf("bundled font %s: %v", name, err))
	}
	f.Subst = !exact
	return f
}

func (r *Resolver) warn(format string, a ...any) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, a...))
}

// allowed applies fsType: restricted and bitmap-only fonts may not be embedded.
func (f *Face) allowed() bool {
	if f.sfnt.OS2 == nil {
		return true
	}
	t := f.sfnt.OS2.FsType
	if t&fsBitmapOnly != 0 {
		return false
	}
	// Restricted (bit 1) without any of the permissive bits.
	if t&0x000E == fsRestricted {
		return false
	}
	f.noSubset = t&fsNoSubset != 0
	return true
}

func load(data []byte, index int, family, file string, src Source, bold, italic bool) (*Face, error) {
	if !bytes.HasPrefix(data, []byte{0, 1, 0, 0}) && !bytes.HasPrefix(data, []byte("OTTO")) && !bytes.HasPrefix(data, []byte("true")) && !bytes.HasPrefix(data, []byte("ttcf")) {
		plain, err := tfont.ToSFNT(data) // WOFF, WOFF2, EOT
		if err != nil {
			return nil, err
		}
		data = plain
	}
	sf, err := tfont.ParseSFNT(data, index)
	if err != nil {
		return nil, err
	}
	if sf.Head == nil || sf.Head.UnitsPerEm == 0 {
		return nil, fmt.Errorf("no head table")
	}
	if bytes.HasPrefix(data, []byte("ttcf")) {
		// font collection: keep the plain SFNT bytes of the chosen face
		data = writeSFNT(sf.Tables, sf.IsCFF)
	}
	f := &Face{Family: family, File: file, Source: src, Bold: bold, Italic: italic, data: data, sfnt: sf, upem: float64(sf.Head.UnitsPerEm)}
	if sf.OS2 != nil && sf.OS2.UsWinAscent > 0 {
		f.ascent = float64(sf.OS2.UsWinAscent) / f.upem
		f.descent = float64(sf.OS2.UsWinDescent) / f.upem
	} else if sf.Hhea != nil {
		f.ascent = float64(sf.Hhea.Ascender) / f.upem
		f.descent = -float64(sf.Hhea.Descender) / f.upem
	}
	if f.ascent <= 0 {
		f.ascent, f.descent = 0.9, 0.25
	}
	return f, nil
}

// substitutes maps common Office families to the bundled metric-compatible ones.
var substitutes = map[string]string{
	"calibri":          "Carlito",
	"calibri light":    "Carlito",
	"carlito":          "Carlito",
	"arial":            "LiberationSans",
	"helvetica":        "LiberationSans",
	"helvetica neue":   "LiberationSans",
	"arimo":            "LiberationSans",
	"liberation sans":  "LiberationSans",
	"times new roman":  "LiberationSerif",
	"times":            "LiberationSerif",
	"tinos":            "LiberationSerif",
	"liberation serif": "LiberationSerif",
	"courier new":      "LiberationMono",
	"courier":          "LiberationMono",
	"cousine":          "LiberationMono",
	"liberation mono":  "LiberationMono",
	"consolas":         "LiberationMono",
	"cambria":          "LiberationSerif",
	"georgia":          "LiberationSerif",
	"aptos":            "Carlito",
	"segoe ui":         "LiberationSans",
	"verdana":          "LiberationSans",
	"tahoma":           "LiberationSans",
}

// metricExact lists the families whose bundled substitute has the same metrics.
var metricExact = map[string]bool{
	"calibri": true, "carlito": true, "arial": true, "arimo": true, "liberation sans": true,
	"times new roman": true, "tinos": true, "liberation serif": true, "courier new": true,
	"cousine": true, "liberation mono": true,
}

func bundledFor(family string) (name string, exact bool) {
	k := strings.ToLower(strings.TrimSpace(family))
	if n, ok := substitutes[k]; ok {
		return n, metricExact[k] || strings.EqualFold(strings.ReplaceAll(n, "Liberation", "Liberation "), family) || strings.EqualFold(n, family)
	}
	if strings.Contains(k, "mono") || strings.Contains(k, "code") {
		return "LiberationMono", false
	}
	if strings.Contains(k, "serif") && !strings.Contains(k, "sans") {
		return "LiberationSerif", false
	}
	return "LiberationSans", false
}

var (
	bundleMu    sync.Mutex
	bundleCache = map[string][]byte{}
)

func bundledData(name string, bold, italic bool) []byte {
	suffix := map[string]string{"regular": "Regular", "bold": "Bold", "italic": "Italic", "boldItalic": "BoldItalic"}[styleName(bold, italic)]
	file := "data/" + name + "-" + suffix + ".ttf.gz"
	bundleMu.Lock()
	defer bundleMu.Unlock()
	if b, ok := bundleCache[file]; ok {
		return b
	}
	gz, err := bundled.ReadFile(file)
	if err != nil {
		panic(err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		panic(err)
	}
	b, err := io.ReadAll(zr)
	if err != nil {
		panic(err)
	}
	bundleCache[file] = b
	return b
}

// FontDirs returns the font folders of the platform, including the private font folders of
// Microsoft Office.
func FontDirs() []string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/System/Library/Fonts", "/Library/Fonts", filepath.Join(home, "Library/Fonts"),
			"/Applications/Microsoft PowerPoint.app/Contents/Resources/DFonts",
			filepath.Join(home, "Library/Group Containers/UBF8T346G9.Office/FontCache"),
		}
	case "windows":
		win := os.Getenv("WINDIR")
		if win == "" {
			win = `C:\Windows`
		}
		local := os.Getenv("LOCALAPPDATA")
		return []string{filepath.Join(win, "Fonts"), filepath.Join(local, `Microsoft\Windows\Fonts`), filepath.Join(local, `Microsoft\FontCache\4\CloudFonts`)}
	default:
		return []string{"/usr/share/fonts", "/usr/local/share/fonts", filepath.Join(home, ".local/share/fonts"), filepath.Join(home, ".fonts")}
	}
}

func (r *Resolver) scan() {
	dirs := r.Dirs
	if dirs == nil {
		dirs = FontDirs()
	}
	r.installed = scanFonts(dirs)
	for _, list := range r.installed {
		sort.Slice(list, func(i, j int) bool {
			if list[i].path != list[j].path {
				return list[i].path < list[j].path
			}
			return list[i].index < list[j].index
		})
	}
}
