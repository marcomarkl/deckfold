// Package render turns the slides of a presentation into SVG. Every shape becomes a <g> element
// with its geometry, fill, outline and text, in the z-order of the slide; text is laid out by
// deckfold (see text.go) and drawn as SVG text with the embedded fonts.
package render

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"deckfold/internal/pptx/anim"
	"deckfold/internal/pptx/emf"
	"deckfold/internal/pptx/fonts"
	"deckfold/internal/pptx/model"
	"deckfold/internal/pptx/opc"
	"deckfold/internal/pptx/xmltree"
)

type xmlAttr = xml.Attr
type xmlName = xml.Name

const emuPx = 9525.0

type rect struct{ x, y, w, h float64 }

type ownerKind int

const (
	ownerSlide ownerKind = iota
	ownerLayout
	ownerMaster
)

// Options configure rendering.
type Options struct {
	Fonts *fonts.Resolver
	Warn  func(format string, a ...any)
	// Linked loads a file the presentation links to (TargetMode="External"): a path relative to
	// the presentation, a file: URL or an http(s) URL. Without it, linked files are reported.
	Linked func(target string) (data []byte, ext string, err error)
	// Missing is called for a linked file that cannot be loaded (where: e.g. "slide 3").
	Missing func(target, where string, err error)
	// AssetDir, when set, receives every asset file as soon as it is known; Result.Assets then
	// lists the names with nil content, so media is not held in memory until the end.
	AssetDir string
}

// Slide is one rendered slide.
type Slide struct {
	Number int    // position in the file, 1-based
	Label  string // title text
	Notes  string
	SVG    string // the slide content (an <svg> element)
	// Shapes lists the shape ids on the slide in drawing order (data-sid values).
	Shapes []string
	// Blocks is the text of the slide for the content model, in drawing order.
	Blocks []Block
	// Anim holds the steps, effects and the transition of the slide.
	Anim anim.Slide
}

// Result is the rendered presentation.
type Result struct {
	Width, Height float64 // px
	Slides        []Slide
	Assets        map[string][]byte // file name -> content (nil when written to Options.AssetDir)
	Faces         []FaceUse
	AnimReport    anim.Report
	// WebGL is set when a slide has content the player draws with three.js (3D shapes).
	WebGL bool
}

// FaceUse maps a CSS family alias to a face.
type FaceUse struct {
	Alias string
	Face  *fonts.Face
}

// Renderer renders a presentation.
type Renderer struct {
	pres    *model.Presentation
	opt     Options
	fonts   *fonts.Resolver
	assets  map[string][]byte
	aliases map[*fonts.Face]string
	faces   []FaceUse
	warned  map[string]bool
	sizes   map[string][2]float64 // pixel size of image assets
	dir     string                // Options.AssetDir
	err     error                 // first error writing an asset
	webGL   bool                  // a slide has content the player draws with three.js
}

// Render renders the visible slides.
func Render(pres *model.Presentation, opt Options) (*Result, error) {
	if opt.Fonts == nil {
		opt.Fonts = fonts.NewResolver(nil)
	}
	if opt.Warn == nil {
		opt.Warn = func(string, ...any) {}
	}
	if opt.Missing == nil {
		opt.Missing = func(target, where string, err error) {
			opt.Warn("%s: linked file %s cannot be loaded: %v", where, target, err)
		}
	}
	rd := &Renderer{pres: pres, opt: opt, fonts: opt.Fonts, assets: map[string][]byte{}, aliases: map[*fonts.Face]string{}, warned: map[string]bool{},
		sizes: map[string][2]float64{}, dir: opt.AssetDir}
	if rd.dir != "" {
		if err := os.MkdirAll(rd.dir, 0o755); err != nil {
			return nil, err
		}
	}
	res := &Result{Width: float64(pres.Width) / emuPx, Height: float64(pres.Height) / emuPx, Assets: rd.assets}
	for _, s := range pres.Slides {
		if s.Hidden {
			continue
		}
		// the number a slide number field shows is the place of the slide in the deck: PowerPoint
		// counts the hidden slides too
		sr := &slideRenderer{Renderer: rd, slide: s, slideNum: pres.FirstSlideNum + s.Number - 1, idp: fmt.Sprintf("s%d", s.Number)}
		out, err := sr.render()
		if err != nil {
			return nil, fmt.Errorf("slide %d: %w", s.Number, err)
		}
		out.Anim = anim.Read(s.Root, &res.AnimReport)
		if t := out.Anim.Transition; t != nil && anim.WebGLTransitions[t.Type] {
			rd.webGL = true // tile transitions are drawn with three.js
		}
		res.Slides = append(res.Slides, out)
	}
	res.Faces = rd.faces
	res.WebGL = rd.webGL
	if rd.err != nil {
		return nil, rd.err
	}
	return res, nil
}

// warnOnce reports a warning once per message.
func (rd *Renderer) warnOnce(format string, a ...any) {
	m := fmt.Sprintf(format, a...)
	if rd.warned[m] {
		return
	}
	rd.warned[m] = true
	rd.opt.Warn("%s", m)
}

func (rd *Renderer) alias(f *fonts.Face) string {
	if a, ok := rd.aliases[f]; ok {
		return a
	}
	a := "dff" + strconv.Itoa(len(rd.faces)+1)
	rd.aliases[f] = a
	rd.faces = append(rd.faces, FaceUse{Alias: a, Face: f})
	return a
}

// asset stores data and returns its file name (content addressed, so names never come from
// the archive).
func (rd *Renderer) asset(data []byte, ext string) string {
	sum := sha256.Sum256(data)
	ext = strings.ToLower(strings.Trim(ext, "."))
	if ext == "" || len(ext) > 5 || strings.ContainsAny(ext, "/\\") {
		ext = "bin"
	}
	name := hex.EncodeToString(sum[:10]) + "." + ext
	if _, ok := rd.assets[name]; ok {
		return name
	}
	if w, h := imageSize(data); w > 0 {
		rd.sizes[name] = [2]float64{w, h}
	}
	if rd.dir == "" {
		rd.assets[name] = data
		return name
	}
	if err := os.WriteFile(filepath.Join(rd.dir, name), data, 0o644); err != nil && rd.err == nil {
		rd.err = err
	}
	rd.assets[name] = nil
	return name
}

type slideRenderer struct {
	*Renderer
	slide    *model.Slide
	slideNum int
	idp      string // id prefix for defs
	defs     strings.Builder
	nid      int
	sids     []string
	label    string
	nodePart map[*xmltree.Node]model.Part // part an inherited property node belongs to
	split    map[string]string            // shape id -> "wd" or "lt" for word and letter builds
	curSplit string                       // split mode of the text being written
	curChart *xmltree.Node                // chart space of the chart being drawn
	blocks   []Block                      // text of the slide for the content model
	groups   []string                     // data-sid of the open groups of the slide
	frame    string                       // data-sid of the slide's table or SmartArt frame being drawn
}

func (r *slideRenderer) newID(kind string) string {
	r.nid++
	return fmt.Sprintf("%s-%s%d", r.idp, kind, r.nid)
}

func (r *slideRenderer) render() (Slide, error) {
	W, H := float64(r.pres.Width)/emuPx, float64(r.pres.Height)/emuPx
	// shapes with builds by word or letter get their text in units the player can animate
	r.split = map[string]string{}
	for _, c := range r.slide.Root.Child("timing").All("cTn") {
		if it := c.Child("iterate"); it != nil && (it.Attr("type") == "wd" || it.Attr("type") == "lt") {
			if t := c.Find("spTgt"); t != nil {
				r.split[t.Attr("spid")] = it.Attr("type")
			}
		}
	}
	var body strings.Builder
	r.background(&body, W, H)
	layout, master := r.slide.Layout, r.slide.Layout.Master
	// showMasterSp on the slide hides the shapes of layout and master, on the layout those of the master
	if r.slide.Root.Bool("showMasterSp", true) {
		if layout.Root.Bool("showMasterSp", true) {
			r.tree(&body, master.Root.Path("cSld/spTree"), ownerMaster, master.Part, identity())
		}
		r.tree(&body, layout.Root.Path("cSld/spTree"), ownerLayout, layout.Part, identity())
	}
	r.tree(&body, r.slide.Root.Path("cSld/spTree"), ownerSlide, r.slide.Part, identity())
	var svg strings.Builder
	// font-kerning: deckfold places every character itself (kernedText) and measures text that
	// PowerPoint does not kern without kerning, so the browser must not kern on its own. A font
	// that is embedded whole would otherwise be kerned with its GPOS.
	fmt.Fprintf(&svg, `<svg class="df-pptx-svg" xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" style="font-kerning:none">`, f(W), f(H), f(W), f(H))
	if r.defs.Len() > 0 {
		svg.WriteString("<defs>" + r.defs.String() + "</defs>")
	}
	svg.WriteString(body.String())
	svg.WriteString("</svg>")
	return Slide{Number: r.slide.Number, Label: r.label, Notes: r.slide.Notes, SVG: svg.String(), Shapes: r.sids, Blocks: r.blocks}, nil
}

// xf maps child coordinates (EMU) of a group to slide px.
type xf struct{ sx, sy, tx, ty float64 }

func identity() xf { return xf{1 / emuPx, 1 / emuPx, 0, 0} }

func (t xf) box(off, ext *xmltree.Node) rect {
	x, y := float64(off.Int("x", 0)), float64(off.Int("y", 0))
	w, h := float64(ext.Int("cx", 0)), float64(ext.Int("cy", 0))
	return rect{x*t.sx + t.tx, y*t.sy + t.ty, w * t.sx, h * t.sy}
}

type relLookup struct {
	part model.Part
	rd   *Renderer
}

// jumpScheme marks a link to another slide in the href of a text run: jumpScheme + the target as
// jumpTarget returns it. A link to the web never starts with it (see SafeURL).
const jumpScheme = "df-jump:"

// showJumps maps the jump of ppaction://hlinkshowjump to the target in the content model.
var showJumps = map[string]string{"nextslide": "next", "previousslide": "prev", "firstslide": "first", "lastslide": "last",
	"lastslideviewed": "back", "endshow": "end"}

// jumpTarget returns the target of a link to another slide: "next", "prev", "first", "last",
// "back" (the slide viewed before), "end" (end the show), or the index of the slide among the
// shown slides. It returns "" for every other link, and for a link to a hidden slide, which is
// not part of the deck deckfold writes.
func (rl relLookup) jumpTarget(h *xmltree.Node) string {
	action := h.Attr("action")
	if j, ok := strings.CutPrefix(action, "ppaction://hlinkshowjump?jump="); ok {
		return showJumps[j]
	}
	if action != "ppaction://hlinksldjump" {
		return ""
	}
	rel, ok := rl.part.Rels[h.RID("id")]
	if !ok || rel.External {
		return ""
	}
	shown := 0
	for _, s := range rl.rd.pres.Slides {
		if s.Name == rel.Target {
			if s.Hidden {
				rl.rd.warnOnce("a link to a hidden slide is shown as text without an action")
				return ""
			}
			return strconv.Itoa(shown)
		}
		if !s.Hidden {
			shown++
		}
	}
	return ""
}

// href returns the target of a link on text: a web address, or jumpScheme and the target of a
// link to another slide.
func (rl relLookup) href(h *xmltree.Node) string {
	if strings.HasPrefix(h.Attr("action"), "ppaction://") {
		if j := rl.jumpTarget(h); j != "" {
			return jumpScheme + j
		}
		if a := h.Attr("action"); a != "ppaction://hlinksldjump" {
			rl.rd.warnOnce("links with an action other than a jump to a slide are shown as text without an action")
		}
	}
	id := h.RID("id")
	if id == "" {
		return ""
	}
	rel, ok := rl.part.Rels[id]
	if !ok || !rel.External {
		return ""
	}
	return SafeURL(rel.Target)
}

// SafeURL returns the URL when its scheme is http, https or mailto, otherwise "".
func SafeURL(u string) string {
	t := strings.TrimSpace(u)
	l := strings.ToLower(t)
	if strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") || strings.HasPrefix(l, "mailto:") {
		return t
	}
	return ""
}

func (r *slideRenderer) tree(b *strings.Builder, tree *xmltree.Node, owner ownerKind, part model.Part, t xf) {
	if tree == nil {
		return
	}
	for _, k := range tree.Kids {
		switch k.Name {
		case "sp", "cxnSp":
			if owner != ownerSlide && model.Ph(k) != nil {
				continue // layout and master placeholders are templates, not content
			}
			r.shape(b, k, owner, part, t)
		case "pic":
			if owner != ownerSlide && model.Ph(k) != nil {
				continue
			}
			r.picture(b, k, owner, part, t)
		case "grpSp":
			r.group(b, k, owner, part, t)
		case "graphicFrame":
			if owner != ownerSlide && model.Ph(k) != nil {
				continue
			}
			r.graphicFrame(b, k, owner, part, t)
		case "contentPart":
			r.ink(b, k, owner, part, t)
		}
	}
}

// xfrmOf returns the xfrm of a shape, inherited from the layout or master placeholder.
func (r *slideRenderer) xfrmOf(sp *xmltree.Node, owner ownerKind) *xmltree.Node {
	for _, sppr := range []string{"spPr", "grpSpPr"} {
		if x := sp.Path(sppr + "/xfrm"); x != nil && x.Child("off") != nil {
			return x
		}
	}
	if x := sp.Child("xfrm"); x != nil { // graphicFrame
		return x
	}
	ph := model.Ph(sp)
	if ph == nil {
		return nil
	}
	var chain []*xmltree.Node
	switch owner {
	case ownerSlide:
		l, m := r.slide.Inherited(ph)
		chain = []*xmltree.Node{l, m}
	case ownerLayout:
		chain = []*xmltree.Node{r.slide.Layout.Inherited(ph)}
	}
	for _, c := range chain {
		if c == nil {
			continue
		}
		if x := c.Path("spPr/xfrm"); x != nil && x.Child("off") != nil {
			return x
		}
	}
	return nil
}

// inheritedSpPr returns the spPr elements of the placeholder chain, highest priority first.
func (r *slideRenderer) inheritedSpPr(sp *xmltree.Node, owner ownerKind) []*xmltree.Node {
	out := []*xmltree.Node{sp.Child("spPr")}
	ph := model.Ph(sp)
	if ph == nil {
		return out
	}
	// remember the part of each inherited node: picture fills refer to that part's relationships
	if r.nodePart == nil {
		r.nodePart = map[*xmltree.Node]model.Part{}
	}
	switch owner {
	case ownerSlide:
		l, m := r.slide.Inherited(ph)
		out = append(out, l.Child("spPr"), m.Child("spPr"))
		r.nodePart[l.Child("spPr")] = r.slide.Layout.Part
		r.nodePart[m.Child("spPr")] = r.slide.Layout.Master.Part
	case ownerLayout:
		m := r.slide.Layout.Inherited(ph).Child("spPr")
		out = append(out, m)
		r.nodePart[m] = r.slide.Layout.Master.Part
	}
	return out
}

// partOfNode returns the part (slide, layout or master) a node belongs to, for relationships of
// inherited properties such as picture bullets.
func (r *slideRenderer) partOfNode(n *xmltree.Node) model.Part {
	top := n
	for top != nil && top.Parent != nil {
		top = top.Parent
	}
	switch top {
	case r.slide.Layout.Root:
		return r.slide.Layout.Part
	case r.slide.Layout.Master.Root:
		return r.slide.Layout.Master.Part
	}
	return r.slide.Part
}

func (r *slideRenderer) colorCtx() colorCtx {
	return colorCtx{theme: r.slide.Layout.Master.Theme, clrMap: r.slide.ClrMap}
}

func (r *slideRenderer) shapeID(sp *xmltree.Node, owner ownerKind) string {
	id := ""
	for _, nv := range []string{"nvSpPr", "nvPicPr", "nvGrpSpPr", "nvGraphicFramePr", "nvCxnSpPr", "nvContentPartPr"} {
		if c := sp.Path(nv + "/cNvPr"); c != nil {
			id = c.Attr("id")
			break
		}
	}
	prefix, ok := map[ownerKind]string{ownerSlide: "", ownerLayout: "l", ownerMaster: "m"}[owner]
	if !ok {
		prefix = "x" // shapes inside SmartArt drawings and OLE frames
	}
	return prefix + id
}

// shapeJump returns the target of a link to another slide on the shape itself (a click anywhere
// on it follows the link), "" when there is none. Only shapes of the slide have such links.
func (r *slideRenderer) shapeJump(sp *xmltree.Node, owner ownerKind) string {
	if owner != ownerSlide {
		return ""
	}
	for _, nv := range []string{"nvSpPr", "nvPicPr", "nvGrpSpPr", "nvGraphicFramePr", "nvCxnSpPr"} {
		if h := sp.Path(nv + "/cNvPr/hlinkClick"); h != nil {
			return relLookup{r.slide.Part, r.Renderer}.jumpTarget(h)
		}
	}
	return ""
}

// open starts the group of a shape and returns the markup that closes it. The outer group
// carries the shape id and no transform, so that the player can animate it with CSS transforms;
// a rotation sits on an inner group.
func (r *slideRenderer) open(b *strings.Builder, sp *xmltree.Node, owner ownerKind, box rect, x *xmltree.Node) string {
	sid := r.shapeID(sp, owner)
	if owner == ownerSlide {
		r.sids = append(r.sids, sid)
	}
	morph := ""
	if owner == ownerSlide {
		// for the morph transition: a key that matches the same object on the next slide (a
		// hash, so that shape names do not end up in the page) and the unrotated box
		rot := 0.0
		if x != nil {
			rot = float64(x.Int("rot", 0)) / 60000
		}
		morph = fmt.Sprintf(` data-mk="%s" data-box="%s %s %s %s %s"`, morphKey(sp), f(box.x), f(box.y), f(box.w), f(box.h), f(rot))
	}
	if j := r.shapeJump(sp, owner); j != "" {
		morph += ` data-df-jump="` + esc(j) + `"`
	}
	fmt.Fprintf(b, `<g data-sid="%s"%s>`, esc(sid), morph)
	if x != nil {
		if rot := float64(x.Int("rot", 0)) / 60000; rot != 0 {
			fmt.Fprintf(b, `<g transform="rotate(%s %s %s)">`, f(rot), f(box.x+box.w/2), f(box.y+box.h/2))
			return "</g></g>"
		}
	}
	return "</g>"
}

func (r *slideRenderer) shape(b *strings.Builder, sp *xmltree.Node, owner ownerKind, part model.Part, t xf) {
	x := r.xfrmOf(sp, owner)
	if x == nil {
		return
	}
	box := t.box(x.Child("off"), x.Child("ext"))
	cx := r.colorCtx()
	end := r.open(b, sp, owner, box, x)

	sppr := r.inheritedSpPr(sp, owner)
	geo := firstChild(sppr, "prstGeom", "custGeom")
	flipH, flipV := x.Bool("flipH", false), x.Bool("flipV", false)
	path, textRect := r.geometry(geo, box)
	if path != "" {
		fill := r.fillAttr(sppr, sp.Path("style/fillRef"), cx, part, box)
		stroke := r.strokeAttr(sppr, sp.Path("style/lnRef"), cx)
		ft := flipTransform(box, flipH, flipV)
		if fill != `fill="none"` || stroke != "" {
			td := r.shape3D(sppr, sp.Path("style/effectRef"), cx)
			fx, refl := r.effects(sppr, sp.Path("style/effectRef"), cx, td)
			ends := ""
			if stroke != "" {
				ends = r.lineEnds(sppr, sp.Path("style/lnRef"), cx)
			}
			data := ""
			if td != nil {
				b.WriteString(td.behind(path, fill, ft))
				data = td.attr()
				r.webGL = true
			}
			fmt.Fprintf(b, `<path d="%s" %s%s%s%s%s%s/>`, path, fill, stroke, ends, fx, ft, data)
			b.WriteString(r.reflection(fmt.Sprintf(`<path d="%s" %s%s%s%s/>`, path, fill, stroke, fx, ft), box, refl))
		}
	}
	if model.Ph(sp) != nil && owner == ownerSlide {
		if t := model.PhType(model.Ph(sp)); (t == "title" || t == "ctrTitle") && r.label == "" {
			r.label = strings.TrimSpace(plainText(sp.Child("txBody")))
		}
	}
	if sp.Child("txBody") != nil {
		ts := r.styleFor(sp, owner)
		if owner == ownerSlide {
			ph := ""
			if n := model.Ph(sp); n != nil {
				ph = model.PhType(n)
			}
			r.collectText(sp.Child("txBody"), ts, r.shapeID(sp, owner), ph, true, false, relLookup{part, r.Renderer}, r.shapeJump(sp, owner))
		} else if r.frame != "" {
			r.collectText(sp.Child("txBody"), ts, r.frame, "", false, false, relLookup{part, r.Renderer}, "")
		}
		bp := newProps()
		for _, n := range ts.bodyPr {
			bp.add(n)
		}
		rot := 0.0
		switch bp.attr("vert") {
		case "vert", "eaVert", "wordArtVertRtl", "mongolianVert":
			rot = 90
		case "vert270":
			rot = 270
		case "wordArtVert":
			r.warnOnce("stacked vertical text (WordArt) is shown horizontally")
		}
		if rot != 0 {
			// lay the text out in the turned box, then turn it into place
			c := rect{textRect.x + textRect.w/2, textRect.y + textRect.h/2, 0, 0}
			turned := rect{c.x - textRect.h/2, c.y - textRect.w/2, textRect.h, textRect.w}
			fmt.Fprintf(b, `<g transform="rotate(%s %s %s)">`, f(rot), f(c.x), f(c.y))
			r.curSplit = r.splitOf(sp, owner)
			r.writeText(b, r.layoutText(sp, ts, turned, cx, relLookup{part, r.Renderer}))
			r.curSplit = ""
			b.WriteString("</g>")
		} else {
			r.curSplit = r.splitOf(sp, owner)
			r.writeText(b, r.layoutText(sp, ts, textRect, cx, relLookup{part, r.Renderer}))
			r.curSplit = ""
		}
	}
	b.WriteString(end)
}

func firstChild(nodes []*xmltree.Node, names ...string) *xmltree.Node {
	for _, n := range nodes {
		for _, nm := range names {
			if c := n.Child(nm); c != nil {
				return c
			}
		}
	}
	return nil
}

func flipTransform(box rect, h, v bool) string {
	if !h && !v {
		return ""
	}
	sx, sy := 1.0, 1.0
	if h {
		sx = -1
	}
	if v {
		sy = -1
	}
	cx, cy := box.x+box.w/2, box.y+box.h/2
	return fmt.Sprintf(` transform="translate(%s %s) scale(%s %s) translate(%s %s)"`, f(cx), f(cy), f(sx), f(sy), f(-cx), f(-cy))
}

func plainText(body *xmltree.Node) string {
	var parts []string
	for _, p := range body.Children("p") {
		var s strings.Builder
		for _, k := range p.Kids {
			switch k.Name {
			case "r", "fld":
				s.WriteString(k.Child("t").AllText())
			case "br":
				s.WriteString(" ")
			}
		}
		parts = append(parts, s.String())
	}
	return strings.Join(parts, " ")
}

// splitOf returns the build unit ("wd", "lt" or "") of a slide shape's text.
func (r *slideRenderer) splitOf(sp *xmltree.Node, owner ownerKind) string {
	if owner != ownerSlide {
		return ""
	}
	return r.split[r.shapeID(sp, owner)]
}

// splitSegs cuts text segments into words ("wd", a word keeps the spaces after it) or letters
// ("lt") and numbers the units per paragraph, so that builds by word or letter can animate them
// one after the other. A word that runs over several segments keeps one number.
func splitSegs(segs []textSeg, mode string) []textSeg {
	var out []textSeg
	unit, para := -1, -1
	prevSpace := true
	for _, s := range segs {
		if s.para != para {
			para, unit, prevSpace = s.para, -1, true
		}
		if s.image != "" || s.bullet || s.rtl || s.text == "" {
			s.unit = -1
			out = append(out, s)
			continue
		}
		rs := []rune(s.text)
		start := 0
		flush := func(end int) {
			if end <= start {
				return
			}
			piece := s
			piece.text = string(rs[start:end])
			// where the piece starts: the width of what stands before it in the segment, and the
			// pair between the two when the text is kerned
			before := s
			before.text = string(rs[:start])
			piece.x = s.x + width(s.face, before.text, s.size, s.kern) + float64(start)*s.spacing + pairPx(&before, piece.text, s.face, s.size, s.kern)
			piece.unit = unit
			out = append(out, piece)
			start = end
		}
		for i, c := range rs {
			space := c == ' ' || c == '\u00a0'
			switch {
			case mode == "lt" && !space:
				flush(i)
				unit++
			case mode == "wd" && !space && prevSpace:
				flush(i)
				unit++
			}
			prevSpace = space
		}
		flush(len(rs))
	}
	return out
}

// kernedText returns the escaped text of a segment. In kerned text every character that forms a
// pair with the one before it sits in a tspan whose dx moves it, and the text after it, by the
// kerning. The positions are deckfold's own in every browser that way: the embedded subsets
// carry no kerning tables, and a browser would take GPOS where PowerPoint takes the kern table.
func kernedText(s textSeg) string {
	if !s.kern {
		return esc(s.text)
	}
	var b strings.Builder
	rs := []rune(s.text)
	start := 0
	for i := 1; i < len(rs); i++ {
		if dx := f(s.face.Kern(rs[i-1], rs[i]) * s.size); dx != "0" {
			b.WriteString(esc(string(rs[start:i])) + `<tspan dx="` + dx + `">` + esc(string(rs[i])) + `</tspan>`)
			start = i + 1
		}
	}
	b.WriteString(esc(string(rs[start:])))
	return b.String()
}

// writeText draws text segments, one group per paragraph (data-p), so that paragraph builds can
// animate them one by one. In shapes with builds by word or letter, each unit gets its own
// group (data-w or data-c) inside the paragraph group.
func (r *slideRenderer) writeText(b *strings.Builder, segs []textSeg) {
	cur := -1
	attr := map[string]string{"wd": "data-w", "lt": "data-c"}[r.curSplit]
	unit := -1
	if attr != "" {
		segs = splitSegs(segs, r.curSplit)
	}
	for _, s := range segs {
		if attr != "" && (s.para != cur || s.unit != unit) && unit >= 0 {
			b.WriteString("</g>")
			unit = -1
		}
		if s.para != cur {
			if cur >= 0 {
				b.WriteString("</g>")
			}
			cur = s.para
			fmt.Fprintf(b, `<g data-p="%d">`, cur)
		}
		if attr != "" && s.unit >= 0 && s.unit != unit {
			unit = s.unit
			fmt.Fprintf(b, `<g %s="%d">`, attr, unit)
		}
		if s.image != "" {
			// picture bullet: its bottom sits just above the baseline
			h := picBulletScale * s.size
			fmt.Fprintf(b, `<image href="assets/%s" x="%s" y="%s" width="%s" height="%s" preserveAspectRatio="none"/>`,
				s.image, f(s.x), f(s.y-h-0.07*s.size), f(s.imgW), f(h))
			continue
		}
		if s.color.OK && s.color.A == 0 {
			continue
		}
		alias := r.alias(s.face)
		style := ""
		if s.face.Bold && !fontIsBold(s.face) {
			style += "font-weight:bold;"
		}
		deco := ""
		if s.underline {
			deco = "underline"
		}
		if s.strike {
			deco = strings.TrimSpace(deco + " line-through")
		}
		if deco != "" {
			style += "text-decoration:" + deco + ";"
		}
		if s.spacing != 0 {
			style += "letter-spacing:" + f(s.spacing) + "px;"
		}
		if s.wordSpace != 0 {
			style += "word-spacing:" + f(s.wordSpace) + "px;"
		}
		if s.rtl || fonts.RTL(s.text) || fonts.Complex(s.text) {
			// shaped text is measured with the kerning of the shaper, so the browser kerns it
			// too; everywhere else the slide switches the browser's own kerning off
			style += "font-kerning:auto;"
		}
		op := ""
		if s.color.A < 1 {
			op = ` fill-opacity="` + f(s.color.A) + `"`
		}
		if style != "" {
			style = ` style="` + style + `"`
		}
		dir := ""
		if s.rtl || fonts.RTL(s.text) {
			// right-to-left: the browser orders and shapes the text; it ends at x, its left edge
			dir = ` direction="rtl" text-anchor="end"`
		}
		txt := fmt.Sprintf(`<text x="%s" y="%s" font-family="%s" font-size="%s" fill="%s"%s%s%s xml:space="preserve">%s</text>`,
			f(s.x), f(s.y), alias, f(s.size), s.color.Hex(), op, style, dir, kernedText(s))
		if j, ok := strings.CutPrefix(s.href, jumpScheme); ok {
			// a link to another slide: the control layer follows it (data-df-jump), no address
			txt = `<a data-df-jump="` + esc(j) + `">` + txt + `</a>`
		} else if s.href != "" {
			txt = `<a href="` + esc(s.href) + `" target="_blank" rel="noopener">` + txt + `</a>`
		}
		b.WriteString(txt)
	}
	if unit >= 0 {
		b.WriteString("</g>")
	}
	if cur >= 0 {
		b.WriteString("</g>")
	}
}

// fontIsBold reports whether the face file itself is bold (otherwise the browser emboldens).
func fontIsBold(f *fonts.Face) bool { return f.Bold }

func (r *slideRenderer) picture(b *strings.Builder, pic *xmltree.Node, owner ownerKind, part model.Part, t xf) {
	x := r.xfrmOf(pic, owner)
	if x == nil {
		return
	}
	box := t.box(x.Child("off"), x.Child("ext"))
	blip := pic.Path("blipFill/blip")
	name, ok := r.blipAsset(blip, part)
	end := r.open(b, pic, owner, box, x)
	if owner == ownerSlide {
		if descr := strings.TrimSpace(pic.Path("nvPicPr/cNvPr").Attr("descr")); descr != "" {
			r.blocks = append(r.blocks, Block{Shape: r.shapeID(pic, owner), Para: -1, Groups: append([]string(nil), r.groups...), Role: "alt", Text: descr})
		}
	}
	sppr := []*xmltree.Node{pic.Child("spPr")}
	geo := firstChild(sppr, "prstGeom", "custGeom")
	path, _ := r.geometry(geo, box)
	if ok {
		clip := r.newID("c")
		fmt.Fprintf(&r.defs, `<clipPath id="%s"><path d="%s"/></clipPath>`, clip, path)
		img := r.imageRect(pic.Child("blipFill"), box)
		ft := flipTransform(box, x.Bool("flipH", false), x.Bool("flipV", false))
		op := ""
		if a := blip.Child("alphaModFix"); a != nil {
			op = ` opacity="` + f(float64(a.Int("amt", 100000))/100000) + `"`
		}
		// The player restarts GIF animations when the slide is shown, as PowerPoint does.
		gif := ""
		if strings.HasSuffix(strings.ToLower(name), ".gif") {
			gif = ` data-df-pptx-gif=""`
		}
		// effects (shadow, bevel) sit on an outer group: a filter on the clipped group itself
		// would be clipped away
		cx := r.colorCtx()
		ref := pic.Path("style/effectRef")
		td := r.shape3D(sppr, ref, cx)
		fx, refl := r.effects(sppr, ref, cx, td)
		data := ""
		if td != nil {
			b.WriteString(td.behind(path, `fill="#808080"`, ""))
			data = td.attr()
			r.webGL = true
		}
		if fx != "" || data != "" {
			fmt.Fprintf(b, `<g%s%s>`, fx, data)
		}
		pic := fmt.Sprintf(`<g clip-path="url(#%s)"%s><image href="assets/%s" x="%s" y="%s" width="%s" height="%s" preserveAspectRatio="none"%s%s/></g>`,
			clip, op, name, f(img.x), f(img.y), f(img.w), f(img.h), ft, gif)
		b.WriteString(pic)
		if fx != "" || data != "" {
			b.WriteString("</g>")
		}
		b.WriteString(r.reflection(pic, box, refl))
	}
	stroke := r.strokeAttr(sppr, pic.Path("style/lnRef"), r.colorCtx())
	if stroke != "" && path != "" {
		fmt.Fprintf(b, `<path d="%s" fill="none"%s/>`, path, stroke)
	}
	r.media(b, pic, part, box, name)
	b.WriteString(end)
}

// media adds the video or audio of a picture shape (a:videoFile, a:audioFile, p14:media). The
// picture itself stays as the poster frame or the audio icon.
func (r *slideRenderer) media(b *strings.Builder, pic *xmltree.Node, part model.Part, box rect, poster string) {
	nv := pic.Path("nvPicPr/nvPr")
	vf, af := nv.Child("videoFile"), nv.Child("audioFile")
	if vf == nil && af == nil {
		return
	}
	kind, el := "video", vf
	if vf == nil {
		kind, el = "audio", af
	}
	sid := r.shapeID(pic, ownerSlide)
	// p14:media holds the embedded file; a:videoFile may only link it
	var rel opc.Rel
	found := false
	for _, m := range nv.All("media") {
		if rr, ok := part.Rels[m.RID("embed")]; ok {
			rel, found = rr, true
			break
		}
	}
	if !found {
		if rr, ok := part.Rels[el.RID("link")]; ok {
			rel, found = rr, true
		}
	}
	if !found {
		return
	}
	mode := r.mediaTrigger(sid)
	if rel.External {
		if u := SafeURL(rel.Target); u != "" && kind == "video" {
			// online video (YouTube, Stream): stays live, with the poster underneath (E9)
			fmt.Fprintf(b, `<foreignObject x="%s" y="%s" width="%s" height="%s"><iframe xmlns="http://www.w3.org/1999/xhtml" src="%s" style="border:0;width:100%%;height:100%%" allow="autoplay; fullscreen"></iframe></foreignObject>`,
				f(box.x), f(box.y), f(box.w), f(box.h), esc(u))
			return
		}
		data, ext, err := r.linked(rel.Target)
		if err != nil {
			return
		}
		r.mediaElement(b, kind, r.asset(data, ext), ext, poster, box, mode)
		return
	}
	data, err := r.pres.Pkg.Read(rel.Target)
	if err != nil {
		r.warnOnce("media file %s cannot be read: %v", rel.Target, err)
		return
	}
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(rel.Target), "."))
	r.mediaElement(b, kind, r.asset(data, ext), ext, poster, box, mode)
}

// playable lists media formats browsers play.
var playable = map[string]bool{"mp4": true, "m4v": true, "mov": true, "webm": true, "mp3": true, "m4a": true, "wav": true, "aac": true, "ogg": true, "oga": true}

func (r *slideRenderer) mediaElement(b *strings.Builder, kind, name, ext, poster string, box rect, mode string) {
	if !playable[ext] {
		r.warnOnce("the %s on slide %d is a .%s file, which browsers cannot play; only its still image is shown. Convert it to MP4 (video) or MP3 (audio)", kind, r.slide.Number, ext)
		return
	}
	if kind == "audio" {
		fmt.Fprintf(b, `<foreignObject x="%s" y="%s" width="1" height="1"><audio xmlns="http://www.w3.org/1999/xhtml" src="assets/%s" preload="auto" data-df-pptx-media="%s"></audio></foreignObject>`,
			f(box.x), f(box.y), name, mode)
		return
	}
	ps := ""
	if poster != "" {
		ps = ` poster="assets/` + poster + `"`
	}
	fmt.Fprintf(b, `<foreignObject x="%s" y="%s" width="%s" height="%s"><video xmlns="http://www.w3.org/1999/xhtml" src="assets/%s"%s preload="auto" playsinline="" data-df-pptx-media="%s" style="display:block;width:100%%;height:100%%;object-fit:fill"></video></foreignObject>`,
		f(box.x), f(box.y), f(box.w), f(box.h), name, ps, mode)
}

// mediaTrigger reads from the slide timing whether a medium starts by itself ("auto") or on a
// click ("click").
func (r *slideRenderer) mediaTrigger(sid string) string {
	timing := r.slide.Root.Child("timing")
	for _, cmd := range timing.All("cmd") {
		if !strings.HasPrefix(cmd.Attr("cmd"), "playFrom") {
			continue
		}
		if t := cmd.Find("spTgt"); t == nil || t.Attr("spid") != sid {
			continue
		}
		for p := cmd.Parent; p != nil; p = p.Parent {
			if p.Name == "cTn" || p.Child("cTn") != nil {
				c := p
				if p.Name != "cTn" {
					c = p.Child("cTn")
				}
				switch c.Attr("nodeType") {
				case "clickEffect":
					return "click"
				case "withEffect", "afterEffect":
					return "auto"
				case "interactiveSeq":
					return "click"
				}
			}
		}
	}
	return "click"
}

// imageRect returns where the whole image lies so that its srcRect part fills the box.
func (r *slideRenderer) imageRect(bf *xmltree.Node, box rect) rect {
	sr := bf.Child("srcRect")
	l, t, rr, bb := float64(sr.Int("l", 0))/100000, float64(sr.Int("t", 0))/100000, float64(sr.Int("r", 0))/100000, float64(sr.Int("b", 0))/100000
	fw, fh := 1-l-rr, 1-t-bb
	if fw <= 0.001 {
		fw = 1
	}
	if fh <= 0.001 {
		fh = 1
	}
	w, h := box.w/fw, box.h/fh
	img := rect{box.x - l*w, box.y - t*h, w, h}
	if fr := bf.Path("stretch/fillRect"); fr != nil {
		// fillRect insets (rare): shrink the target box
		fl, ft, fr2, fb := float64(fr.Int("l", 0))/100000, float64(fr.Int("t", 0))/100000, float64(fr.Int("r", 0))/100000, float64(fr.Int("b", 0))/100000
		if fl != 0 || ft != 0 || fr2 != 0 || fb != 0 {
			tb := rect{box.x + fl*box.w, box.y + ft*box.h, box.w * (1 - fl - fr2), box.h * (1 - ft - fb)}
			w, h = tb.w/fw, tb.h/fh
			img = rect{tb.x - l*w, tb.y - t*h, w, h}
		}
	}
	return img
}

// blipAsset stores the image of an a:blip and returns its asset name.
func (r *slideRenderer) blipAsset(blip *xmltree.Node, part model.Part) (string, bool) {
	if blip == nil {
		return "", false
	}
	// an SVG image (PowerPoint 2016+) is stored next to a PNG fallback
	id := blip.RID("embed")
	for _, ext := range blip.All("svgBlip") {
		if sid := ext.RID("embed"); sid != "" {
			id = sid
		}
	}
	if id == "" {
		if lid := blip.RID("link"); lid != "" {
			if rel, ok := part.Rels[lid]; ok && rel.External {
				if data, ext, err := r.linked(rel.Target); err == nil {
					return r.asset(data, ext), true
				}
			}
		}
		return "", false
	}
	rel, ok := part.Rels[id]
	if !ok || rel.External {
		return "", false
	}
	data, err := r.pres.Pkg.Read(rel.Target)
	if err != nil {
		r.warnOnce("picture %s cannot be read: %v", rel.Target, err)
		return "", false
	}
	format := pictureFormat(rel.Target, r.pres.Pkg.ContentType(rel.Target), data)
	switch format {
	case "":
		r.warnOnce("a picture on slide %d is in a format deckfold does not know (%s) and is left out", r.slide.Number, r.pres.Pkg.ContentType(rel.Target))
		return "", false
	case "emf":
		res, err := emf.Convert(data)
		if err != nil {
			r.warnOnce("an EMF picture on slide %d cannot be read (%v) and is left out", r.slide.Number, err)
			return "", false
		}
		if res.Unknown*2 > res.Records {
			r.warnOnce("an EMF picture on slide %d uses drawing commands deckfold does not convert; it may be incomplete", r.slide.Number)
		}
		return r.asset(res.SVG, "svg"), true
	case "wmf":
		res, err := emf.ConvertWMF(data)
		if err != nil {
			r.warnOnce("a WMF picture on slide %d cannot be read (%v) and is left out", r.slide.Number, err)
			return "", false
		}
		if res.Unknown*2 > res.Records {
			r.warnOnce("a WMF picture on slide %d uses drawing commands deckfold does not convert; it may be incomplete", r.slide.Number)
		}
		return r.asset(res.SVG, "svg"), true
	}
	return r.asset(data, format), true
}

// pictureTypes maps the content types of pictures to the format names used for assets.
var pictureTypes = map[string]string{
	"image/png": "png", "image/jpeg": "jpg", "image/jpg": "jpg", "image/gif": "gif", "image/svg+xml": "svg", "image/webp": "webp",
	"image/bmp": "bmp", "image/x-ms-bmp": "bmp", "image/tiff": "tiff", "image/x-emf": "emf", "image/emf": "emf", "image/x-wmf": "wmf", "image/wmf": "wmf",
}

// pictureFormat names the format of a picture part: by the extension of its file where that is
// the one of a picture, otherwise by its content type, otherwise by its first bytes. PowerPoint
// for Mac stores some pictures as .bin files and names their format only in [Content_Types].xml
// (seen with SVG and EMF pictures). Returns "" for a part that is no picture deckfold knows.
func pictureFormat(target, contentType string, data []byte) string {
	switch ext := strings.ToLower(strings.TrimPrefix(path.Ext(target), ".")); ext {
	case "jpg", "jpeg", "png", "gif", "svg", "webp", "bmp", "emf", "wmf", "tif", "tiff":
		return ext
	}
	if f := pictureTypes[strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))]; f != "" {
		return f
	}
	has := func(at int, magic string) bool {
		return len(data) >= at+len(magic) && string(data[at:at+len(magic)]) == magic
	}
	switch {
	case has(0, "\x89PNG\r\n\x1a\n"):
		return "png"
	case has(0, "\xff\xd8\xff"):
		return "jpg"
	case has(0, "GIF8"):
		return "gif"
	case has(0, "RIFF") && has(8, "WEBP"):
		return "webp"
	case has(0, "\x01\x00\x00\x00") && has(40, " EMF"):
		return "emf"
	case has(0, "\xd7\xcd\xc6\x9a"):
		return "wmf"
	}
	head := data
	if len(head) > 2048 {
		head = head[:2048]
	}
	if text := strings.TrimLeft(strings.TrimPrefix(string(head), "\xef\xbb\xbf"), " \t\r\n"); strings.HasPrefix(text, "<") && strings.Contains(text, "<svg") {
		return "svg"
	}
	return ""
}

func (r *slideRenderer) group(b *strings.Builder, g *xmltree.Node, owner ownerKind, part model.Part, t xf) {
	x := g.Path("grpSpPr/xfrm")
	if x == nil {
		r.tree(b, g, owner, part, t)
		return
	}
	box := t.box(x.Child("off"), x.Child("ext"))
	off, ext, choff, chext := x.Child("off"), x.Child("ext"), x.Child("chOff"), x.Child("chExt")
	cw, ch := float64(chext.Int("cx", ext.Int("cx", 1))), float64(chext.Int("cy", ext.Int("cy", 1)))
	sx, sy := 1.0, 1.0
	if cw > 0 {
		sx = float64(ext.Int("cx", 0)) / cw
	}
	if ch > 0 {
		sy = float64(ext.Int("cy", 0)) / ch
	}
	// child EMU -> parent EMU: (c - chOff) * s + off ; then parent transform
	ox, oy := float64(off.Int("x", 0)), float64(off.Int("y", 0))
	cox, coy := float64(choff.Int("x", 0)), float64(choff.Int("y", 0))
	nt := xf{
		sx: t.sx * sx, sy: t.sy * sy,
		tx: t.tx + t.sx*(ox-cox*sx), ty: t.ty + t.sy*(oy-coy*sy),
	}
	end := r.open(b, g, owner, box, x)
	if owner == ownerSlide {
		r.groups = append(r.groups, r.shapeID(g, owner))
	}
	if ft := flipTransform(box, x.Bool("flipH", false), x.Bool("flipV", false)); ft != "" {
		b.WriteString("<g" + ft + ">")
		r.tree(b, g, owner, part, nt)
		b.WriteString("</g>")
	} else {
		r.tree(b, g, owner, part, nt)
	}
	if owner == ownerSlide {
		r.groups = r.groups[:len(r.groups)-1]
	}
	b.WriteString(end)
}

func (r *slideRenderer) graphicFrame(b *strings.Builder, gf *xmltree.Node, owner ownerKind, part model.Part, t xf) {
	x := r.xfrmOf(gf, owner)
	if x == nil {
		return
	}
	box := t.box(x.Child("off"), x.Child("ext"))
	gd := gf.Path("graphic/graphicData")
	uri := gd.Attr("uri")
	switch {
	case strings.HasSuffix(uri, "/table"):
		_ = r.open(b, gf, owner, box, nil)
		if owner == ownerSlide {
			r.frame = r.shapeID(gf, owner)
		}
		r.table(b, gd.Child("tbl"), owner, part, box)
		r.frame = ""
		b.WriteString("</g>")
		return
	case strings.HasSuffix(uri, "/ole"):
		// OLE object: PowerPoint stores a picture of it (mc:Fallback), drawn in the frame
		if pic := gd.Find("pic"); pic != nil {
			_ = r.open(b, gf, owner, box, nil)
			r.picture(b, pic, ownerKind(-1), part, t)
			b.WriteString("</g>")
			return
		}
	case strings.HasSuffix(uri, "/diagram"):
		if r.smartArt(b, gf, gd, owner, part, t, box) {
			return
		}
	case strings.HasSuffix(uri, "/chart"):
		var cb strings.Builder
		if r.chart(&cb, gf, gd, part, box) {
			end := r.open(b, gf, owner, box, nil)
			b.WriteString(cb.String())
			b.WriteString(end)
			return
		}
	}
	_ = r.open(b, gf, owner, box, nil)
	kind := "object"
	switch {
	case strings.HasSuffix(uri, "/chart"):
		kind = "chart"
	case strings.HasSuffix(uri, "/diagram"):
		kind = "SmartArt graphic without a stored drawing"
	}
	r.warnOnce("%s on slide %d is shown as a placeholder (not supported yet)", kind, r.slide.Number)
	fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" fill="#f2f2f2" stroke="#bfbfbf"/>`, f(box.x), f(box.y), f(box.w), f(box.h))
	b.WriteString("</g>")
}

// smartArt draws a SmartArt graphic from the drawing PowerPoint stores with it
// (ppt/diagrams/drawingN.xml). Without that drawing it returns false.
func (r *slideRenderer) smartArt(b *strings.Builder, gf, gd *xmltree.Node, owner ownerKind, part model.Part, t xf, box rect) bool {
	ids := gd.Child("relIds")
	dm, ok := part.Rels[ids.RID("dm")]
	if !ok || dm.External {
		return false
	}
	dmXML, err := r.pres.Pkg.XML(dm.Target)
	if err != nil {
		return false
	}
	ext := dmXML.Find("dataModelExt")
	if ext == nil {
		return false
	}
	dr, ok := part.Rels[ext.Attr("relId")]
	if !ok || dr.External {
		return false
	}
	drawing, err := r.pres.Pkg.XML(dr.Target)
	if err != nil {
		return false
	}
	rels, err := r.pres.Pkg.Rels(dr.Target)
	if err != nil {
		return false
	}
	tree := drawing.Child("spTree")
	if tree == nil {
		return false
	}
	// shapes in the drawing are positioned relative to the frame
	off := gf.Path("xfrm/off")
	nt := xf{sx: t.sx, sy: t.sy, tx: t.tx + t.sx*float64(off.Int("x", 0)), ty: t.ty + t.sy*float64(off.Int("y", 0))}
	_ = r.open(b, gf, owner, box, nil)
	if owner == ownerSlide {
		r.frame = r.shapeID(gf, owner)
	}
	r.tree(b, tree, ownerKind(-1), model.Part{Name: dr.Target, Root: drawing, Rels: rels}, nt)
	r.frame = ""
	b.WriteString("</g>")
	return true
}

func (r *slideRenderer) background(b *strings.Builder, w, h float64) {
	cx := r.colorCtx()
	for _, part := range []model.Part{r.slide.Part, r.slide.Layout.Part, r.slide.Layout.Master.Part} {
		bg := part.Root.Path("cSld/bg")
		if bg == nil {
			continue
		}
		box := rect{0, 0, w, h}
		if pr := bg.Child("bgPr"); pr != nil {
			fill := r.fillAttr([]*xmltree.Node{pr}, nil, cx, part, box)
			fmt.Fprintf(b, `<rect class="df-pptx-bg" width="%s" height="%s" %s/>`, f(w), f(h), fill)
			return
		}
		if ref := bg.Child("bgRef"); ref != nil {
			fill := r.styleFill(ref, cx, part, box, true)
			fmt.Fprintf(b, `<rect class="df-pptx-bg" width="%s" height="%s" %s/>`, f(w), f(h), fill)
			return
		}
	}
	fmt.Fprintf(b, `<rect class="df-pptx-bg" width="%s" height="%s" fill="#ffffff"/>`, f(w), f(h))
}

// f formats a number for SVG.
func f(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "0"
	}
	s := strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64)
	if s == "-0" {
		return "0"
	}
	return s
}

func esc(s string) string {
	var b strings.Builder
	for _, c := range s {
		switch c {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '\'':
			b.WriteString("&#39;")
		default:
			if c < 0x20 && c != '\t' && c != '\n' {
				continue
			}
			b.WriteRune(c)
		}
	}
	return b.String()
}

// AddAsset stores an extra file (for example a font subset) in dir (or in memory when dir is
// empty) and returns its name.
func (res *Result) AddAsset(data []byte, ext, dir string) (string, error) {
	rd := &Renderer{assets: res.Assets, sizes: map[string][2]float64{}, dir: dir}
	name := rd.asset(data, ext)
	return name, rd.err
}

// linked loads a linked file through Options.Linked and reports a failure as missing.
func (r *slideRenderer) linked(target string) ([]byte, string, error) {
	if r.opt.Linked == nil {
		err := fmt.Errorf("linked files are not loaded")
		r.opt.Missing(target, fmt.Sprintf("slide %d", r.slide.Number), err)
		return nil, "", err
	}
	data, ext, err := r.opt.Linked(target)
	if err != nil {
		r.opt.Missing(target, fmt.Sprintf("slide %d", r.slide.Number), err)
		return nil, "", err
	}
	return data, ext, nil
}

// morphKey returns the key the morph transition matches objects by: a name that starts with
// "!!" (PowerPoint's way to pair objects explicitly), otherwise the kind of object, its
// geometry and its text, otherwise its name.
func morphKey(sp *xmltree.Node) string {
	name := ""
	for _, nv := range []string{"nvSpPr", "nvPicPr", "nvGrpSpPr", "nvGraphicFramePr", "nvCxnSpPr", "nvContentPartPr"} {
		if c := sp.Path(nv + "/cNvPr"); c != nil {
			name = c.Attr("name")
			break
		}
	}
	key := "n:" + name
	if !strings.HasPrefix(name, "!!") {
		if txt := strings.TrimSpace(plainText(sp.Child("txBody"))); txt != "" {
			key = "t:" + sp.Name + ":" + sp.Path("spPr/prstGeom").Attr("prst") + ":" + txt
		}
	}
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:6])
}
