// Package opc reads an Open Packaging Conventions file (PPTX): the ZIP container, content types
// and relationships. It limits sizes, so that a hostile archive cannot exhaust memory.
package opc

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"

	"deckfold/internal/pptx/xmltree"
)

// Limits for a package.
const (
	MaxPartSize   = 1 << 30 // largest uncompressed part (media)
	MaxXMLSize    = 64 << 20
	MaxTotalSize  = 4 << 30
	MaxRatio      = 400 // compression ratio above which a part counts as a zip bomb (for parts > 1 MB)
	maxPartsCount = 100000
)

// ErrCFB marks an OLE compound file: a legacy .ppt or an encrypted PPTX.
var ErrCFB = errors.New("this is a legacy PowerPoint 97-2003 file (.ppt) or a password-protected presentation. Open it in PowerPoint and save it as .pptx without a password")

// ErrNotZip marks a file that is not a ZIP archive.
var ErrNotZip = errors.New("not a PowerPoint file (no ZIP archive)")

// Package is an opened package.
type Package struct {
	files map[string]*zip.File // by part name without leading slash
	types *contentTypes
	total int64
}

// Rel is a relationship.
type Rel struct {
	ID       string
	Type     string // full type URI
	Target   string // resolved part name for internal targets, raw target for external ones
	External bool
}

// Sniff checks the first bytes of a file: "zip", "cfb" or "".
func Sniff(b []byte) string {
	switch {
	case len(b) >= 4 && bytes.Equal(b[:4], []byte("PK\x03\x04")):
		return "zip"
	case len(b) >= 8 && bytes.Equal(b[:8], []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}):
		return "cfb"
	}
	return ""
}

// Open reads the package from a file.
func Open(p string) (*Package, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	return OpenBytes(data)
}

// OpenBytes reads the package from memory.
func OpenBytes(data []byte) (*Package, error) {
	switch Sniff(data) {
	case "cfb":
		return nil, ErrCFB
	case "zip":
	default:
		return nil, ErrNotZip
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("damaged PowerPoint file: %w", err)
	}
	if len(z.File) > maxPartsCount {
		return nil, fmt.Errorf("damaged PowerPoint file: %d parts", len(z.File))
	}
	pkg := &Package{files: map[string]*zip.File{}}
	for _, f := range z.File {
		name := f.Name
		if strings.Contains(name, "\\") || strings.HasPrefix(name, "/") || hasDotDot(name) {
			return nil, fmt.Errorf("damaged PowerPoint file: invalid part name %q", name)
		}
		if f.UncompressedSize64 > MaxPartSize {
			if strings.HasPrefix(strings.ToLower(name), "ppt/media/") {
				return nil, fmt.Errorf("the media file %s is %.1f GB; deckfold embeds media files up to 1 GB, and browsers cannot open much larger HTML files. Compress the media in PowerPoint (File > Compress Media) and convert again", name, float64(f.UncompressedSize64)/(1<<30))
			}
			return nil, fmt.Errorf("part %s is too large (%d bytes)", name, f.UncompressedSize64)
		}
		if f.UncompressedSize64 > 1<<20 && f.CompressedSize64 > 0 && f.UncompressedSize64/f.CompressedSize64 > MaxRatio {
			return nil, fmt.Errorf("part %s has a suspicious compression ratio", name)
		}
		pkg.total += int64(f.UncompressedSize64)
		pkg.files[strings.ToLower(name)] = f
	}
	if pkg.total > MaxTotalSize {
		return nil, fmt.Errorf("the presentation is too large (%d bytes uncompressed)", pkg.total)
	}
	ct, err := pkg.readXML("[Content_Types].xml")
	if err != nil {
		return nil, fmt.Errorf("damaged PowerPoint file: [Content_Types].xml: %w", err)
	}
	pkg.types = parseTypes(ct)
	return pkg, nil
}

func hasDotDot(name string) bool {
	for _, s := range strings.Split(name, "/") {
		if s == ".." {
			return true
		}
	}
	return false
}

// Has reports whether a part exists.
func (p *Package) Has(name string) bool {
	_, ok := p.files[strings.ToLower(strings.TrimPrefix(name, "/"))]
	return ok
}

// Read returns the bytes of a part.
func (p *Package) Read(name string) ([]byte, error) {
	f, ok := p.files[strings.ToLower(strings.TrimPrefix(name, "/"))]
	if !ok {
		return nil, fmt.Errorf("part %s is missing", name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("part %s: %w", name, err)
	}
	defer rc.Close()
	b, err := io.ReadAll(io.LimitReader(rc, MaxPartSize+1))
	if err != nil {
		return nil, fmt.Errorf("part %s: %w", name, err)
	}
	if int64(len(b)) > MaxPartSize {
		return nil, fmt.Errorf("part %s is too large", name)
	}
	return b, nil
}

// Supported lists the namespaces whose mc:Choice branches deckfold reads (transitions of
// PowerPoint 2010 and later); everything else uses the mc:Fallback branch.
var Supported = map[string]bool{
	"http://schemas.microsoft.com/office/powerpoint/2010/main":    true,
	"http://schemas.microsoft.com/office/powerpoint/2012/main":    true,
	"http://schemas.microsoft.com/office/powerpoint/2015/09/main": true,
}

// XML parses a part as XML.
func (p *Package) XML(name string) (*xmltree.Node, error) { return p.readXML(name) }

func (p *Package) readXML(name string) (*xmltree.Node, error) {
	f, ok := p.files[strings.ToLower(strings.TrimPrefix(name, "/"))]
	if !ok {
		return nil, fmt.Errorf("part %s is missing", name)
	}
	if f.UncompressedSize64 > MaxXMLSize {
		return nil, fmt.Errorf("part %s is too large for XML", name)
	}
	b, err := p.Read(name)
	if err != nil {
		return nil, err
	}
	n, err := xmltree.Parse(b, xmltree.Options{Supported: Supported})
	if err != nil {
		return nil, fmt.Errorf("part %s: %w", name, err)
	}
	return n, nil
}

// ContentType returns the content type of a part.
func (p *Package) ContentType(name string) string {
	name = "/" + strings.TrimPrefix(name, "/")
	if t, ok := p.types.override[strings.ToLower(name)]; ok {
		return t
	}
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(name), "."))
	return p.types.def[ext]
}

// Rels returns the relationships of a part ("" for the package).
func (p *Package) Rels(part string) (map[string]Rel, error) {
	part = strings.TrimPrefix(part, "/")
	dir, file := path.Split(part)
	relsName := dir + "_rels/" + file + ".rels"
	out := map[string]Rel{}
	if !p.Has(relsName) {
		return out, nil
	}
	n, err := p.readXML(relsName)
	if err != nil {
		return nil, err
	}
	for _, r := range n.Children("Relationship") {
		rel := Rel{ID: r.Attr("Id"), Type: r.Attr("Type"), External: r.Attr("TargetMode") == "External"}
		if rel.External {
			rel.Target = r.Attr("Target")
		} else {
			rel.Target = Resolve(dir, r.Attr("Target"))
		}
		out[rel.ID] = rel
	}
	return out, nil
}

// Resolve resolves a relative target against the folder of the source part.
func Resolve(dir, target string) string {
	target = strings.ReplaceAll(target, "\\", "/")
	if strings.HasPrefix(target, "/") {
		return path.Clean(strings.TrimPrefix(target, "/"))
	}
	if i := strings.IndexAny(target, "#?"); i >= 0 {
		target = target[:i]
	}
	j := path.Join(dir, target)
	return strings.TrimPrefix(path.Clean("/"+j), "/")
}

// ByType returns the relationships of one type (matched by the last path segment of the type
// URI, so that strict and transitional URIs both match).
func ByType(rels map[string]Rel, suffix string) []Rel {
	var out []Rel
	for _, r := range rels {
		if strings.HasSuffix(r.Type, "/"+suffix) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

type contentTypes struct {
	def      map[string]string
	override map[string]string
}

func parseTypes(n *xmltree.Node) *contentTypes {
	ct := &contentTypes{def: map[string]string{}, override: map[string]string{}}
	for _, d := range n.Children("Default") {
		ct.def[strings.ToLower(d.Attr("Extension"))] = d.Attr("ContentType")
	}
	for _, o := range n.Children("Override") {
		ct.override[strings.ToLower(o.Attr("PartName"))] = o.Attr("ContentType")
	}
	return ct
}
