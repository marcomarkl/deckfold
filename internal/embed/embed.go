// Package embed is the shared conversion: collect references, load and deduplicate assets,
// apply deferred activation as patches to the original bytes, rewrite the CSP, and insert
// the runtime and the asset blocks.
package embed

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"deckfold/internal/adapter"
	"deckfold/internal/assets"
	"deckfold/internal/clean"
	"deckfold/internal/config"
	"deckfold/internal/csp"
	"deckfold/internal/htmlscan"
	"deckfold/internal/inject"
	"deckfold/internal/patch"
	"deckfold/internal/report"
)

// Options control a conversion.
type Options struct {
	Config       config.Config
	Lenient      bool
	AllowOutside []string
	Fetcher      *assets.Fetcher
	Version      string
	Runtime      inject.Runtime
}

// ErrMissing reports unresolved assets without --lenient (exit code 1).
var ErrMissing = errors.New("unresolved assets")

// ErrAlreadyConverted reports an input that already is a deckfold output.
var ErrAlreadyConverted = errors.New("the input already is a deckfold output; to update its runtime use: deckfold --refresh <file>")

// DataURIMin is the size from which existing data: URIs move into asset blocks.
const DataURIMin = 8 << 10

type conv struct {
	in        *adapter.Input
	ad        adapter.Adapter
	opt       Options
	rep       *report.Report
	res       *assets.Resolver
	store     *assets.Store
	patches   []patch.Patch
	seenRes   clean.Seen // duplicate includes
	fontFaces clean.Seen
	embedded  map[string]bool // local paths that were embedded
	frameOrig map[string]bool
	inline    [][]byte // inline scripts of the source (for hash-based CSPs)
	sprites   map[string]bool
	spriteBuf bytes.Buffer
	cssLink   bool // CSS as blob link instead of <style> (CSP without inline styles)
	dropHosts map[string]bool
	slideAt   []int // start offsets of the slides (for locations in messages)
}

// Convert produces the self-contained output.
func Convert(in *adapter.Input, ad adapter.Adapter, opt Options, rep *report.Report) ([]byte, adapter.Meta, error) {
	for _, t := range in.Doc.Tags {
		if t.Name == "meta" && t.Value("name") == "deckfold:version" {
			return nil, adapter.Meta{}, ErrAlreadyConverted
		}
	}
	meta, warns, err := ad.Meta(in)
	if err != nil {
		return nil, meta, err
	}
	rep.Format = ad.Name()
	for _, w := range warns {
		rep.Warn("%s", w)
	}
	res, err := assets.NewResolver(in.Root, opt.AllowOutside)
	if err != nil {
		return nil, meta, err
	}
	c := &conv{in: in, ad: ad, opt: opt, rep: rep, res: res, store: assets.NewStore(), seenRes: clean.Seen{},
		fontFaces: clean.Seen{}, embedded: map[string]bool{}, frameOrig: map[string]bool{}, sprites: map[string]bool{},
		dropHosts: map[string]bool{}}
	if ns, ok := meta.Extra["netSlides"].([]string); ok {
		rep.NetSlides = append(rep.NetSlides, ns...)
	}
	slideClass := "deck-slide"
	if sel := meta.SlideSelector; strings.HasPrefix(sel, ".") && !strings.ContainsAny(sel[1:], " .#[:>") {
		slideClass = sel[1:]
	}
	for _, t := range in.Doc.Tags {
		if t.Name == "section" && strings.Contains(" "+t.Value("class")+" ", " "+slideClass+" ") {
			c.slideAt = append(c.slideAt, t.Start)
		}
	}
	c.cssLink = c.cspBlocksInlineStyle()

	c.walk()

	// assets loaded at runtime (Keynote: JSONP, PDFP, player files)
	shim := map[string]string{}
	extras, err := ad.Collect(in)
	if err != nil {
		return nil, meta, err
	}
	for _, e := range extras {
		full := filepath.Join(in.Root, filepath.FromSlash(e.Path))
		data, err := os.ReadFile(full)
		if err != nil {
			rep.Miss(e.Ref, "loaded at runtime ("+ad.Name()+")", err.Error())
			continue
		}
		h := c.store.Add(data, assets.MimeFor(full, "", data), e.Path)
		if rp, err := filepath.EvalSymlinks(full); err == nil {
			c.embedded[rp] = true
		}
		c.embedded[full] = true
		shim[e.Ref] = h
		if e.Thumb >= 0 && e.Thumb < len(meta.Slides) {
			meta.Slides[e.Thumb].Thumb = h
		}
	}

	if len(rep.Missing) > 0 && !opt.Lenient {
		return nil, meta, ErrMissing
	}
	for _, m := range rep.Missing {
		rep.Warn("not embedded (--lenient): %s (%s)", m.Ref, m.Where)
	}
	if len(rep.Missing) > 0 {
		rep.Missing = nil
	}
	c.reportUnused()

	// runtime block
	d := inject.Defaults{Version: opt.Version, CSSMode: "style", Defaults: opt.Config}
	if c.cssLink {
		d.CSSMode = "link"
	}
	block, err := inject.Block(opt.Version, d, opt.Runtime)
	if err != nil {
		return nil, meta, err
	}
	// rewrite the CSP (after collecting embed origins and inline scripts)
	c.rewriteCSP([]byte(opt.Runtime.JS))

	// runtime right after the charset declaration (otherwise after <head>, <html> or the doctype)
	at := c.headInsertPos()
	c.patches = append(c.patches, patch.Patch{Start: at, End: at, Repl: block, Order: -10})
	if c.spriteBuf.Len() > 0 {
		bs := c.bodyStartPos()
		c.patches = append(c.patches, patch.Patch{Start: bs, End: bs, Repl: c.spriteBuf.Bytes(), Order: -5})
	}

	// metadata and asset blocks before </body>
	deckHash := assets.HashOf(append(append([]byte{}, in.Src...), []byte(strings.Join(c.store.Order, ","))...))
	mb, err := inject.MetaBlock(map[string]any{"deck": meta, "shim": shim, "deckHash": deckHash})
	if err != nil {
		return nil, meta, err
	}
	tail := append(append([]byte("\n"), mb...), inject.AssetBlocks(c.store)...)
	end := len(in.Src)
	if in.Doc.BodyClose >= 0 {
		end = in.Doc.BodyClose
	}
	c.patches = append(c.patches, patch.Patch{Start: end, End: end, Repl: tail, Order: 10})

	out, err := patch.Apply(in.Src, c.patches)
	if err != nil {
		return nil, meta, err
	}
	for _, h := range c.store.Order {
		a := c.store.Get(h)
		rep.Assets = append(rep.Assets, report.Sized{Name: strings.Join(a.Names, ", "), Size: len(a.Data)})
	}
	for h := range opt.Fetcher.Hosts {
		rep.Hosts[h] = true
	}
	return out, meta, nil
}

// ---------- walk over the tags ----------

var executable = map[string]bool{"": true, "text/javascript": true, "application/javascript": true, "module": true,
	"text/ecmascript": true, "application/ecmascript": true, "application/x-javascript": true, "text/jscript": true}

func (c *conv) walk() {
	d := c.in.Doc
	for _, t := range d.Tags {
		switch t.Name {
		case "link":
			c.link(t)
			continue
		case "style":
			c.style(t)
			continue
		case "script":
			c.script(t)
			continue
		case "meta":
			continue
		case "base":
			if t.Value("href") != "" {
				c.rep.Warn("<base href=%q> is ignored; references are resolved relative to the HTML file", t.Value("href"))
			}
			continue
		case "iframe", "frame":
			if c.liveEmbed(t) {
				continue
			}
		}
		c.attrs(t)
	}
}

func (c *conv) where(t *htmlscan.Tag, attr string) string {
	rel, _ := filepath.Rel(c.res.Root, realOr(c.in.HTMLPath))
	s := fmt.Sprintf("%s: <%s %s>", filepath.ToSlash(rel), t.Name, attr)
	if n := c.slideOf(t.Start); n > 0 {
		s += fmt.Sprintf(" (slide %d)", n)
	}
	return s
}

func (c *conv) slideOf(off int) int {
	n := 0
	for i, s := range c.slideAt {
		if s <= off {
			n = i + 1
		}
	}
	return n
}

// liveEmbed treats iframes with an http(s) source as live content (E9).
func (c *conv) liveEmbed(t *htmlscan.Tag) bool {
	src := strings.TrimSpace(t.Value("src"))
	u, err := url.Parse(src)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https" && !strings.HasPrefix(src, "//")) {
		return false
	}
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	origin := u.Scheme + "://" + u.Host
	c.frameOrig[origin] = true
	c.rep.NetSlides = append(c.rep.NetSlides, c.where(t, "src="+src))
	return true
}

// assetAttr reports attributes that reference assets, per element (inventory from step 2 of the plan).
func assetAttr(tag, attr string) bool {
	switch attr {
	case "src":
		return tag == "img" || tag == "video" || tag == "audio" || tag == "source" || tag == "track" || tag == "embed" || tag == "input" || tag == "iframe"
	case "srcset":
		return tag == "img" || tag == "source"
	case "poster":
		return tag == "video"
	case "data":
		return tag == "object"
	case "href", "xlink:href":
		return tag == "image" || tag == "use" || tag == "feimage"
	case "background":
		return tag == "body" || tag == "table" || tag == "td"
	}
	return false
}

func (c *conv) attrs(t *htmlscan.Tag) {
	for _, a := range t.Attrs {
		switch {
		case a.Name == "style" && a.ValStart >= 0:
			if !strings.Contains(strings.ToLower(a.Val), "url(") && !strings.Contains(strings.ToLower(a.Val), "image-set(") {
				continue
			}
			css, changed := c.processCSS(a.Val, c.in.HTMLDir, nil, c.where(t, "style"), 0, false)
			if changed {
				c.replaceAttr(a, "data-df-style", css)
			}
		case assetAttr(t.Name, a.Name) && a.ValStart >= 0:
			val := strings.TrimSpace(a.Val)
			if val == "" {
				continue
			}
			if a.Name == "srcset" {
				if nv, changed := c.srcset(val, c.where(t, a.Name)); changed {
					c.replaceAttr(a, "data-df-srcset", nv)
				}
				continue
			}
			if (t.Name == "use" || t.Name == "image") && (a.Name == "href" || a.Name == "xlink:href") && t.Name == "use" {
				c.useSprite(t, a)
				continue
			}
			if nv, ok := c.ref(val, c.in.HTMLDir, nil, c.where(t, a.Name)); ok && nv != val {
				c.replaceAttr(a, "data-df-"+a.Name, nv)
			}
		}
	}
}

func (c *conv) replaceAttr(a htmlscan.Attr, name, val string) {
	repl := name + `="` + escAttr(val) + `"`
	c.patches = append(c.patches, patch.Patch{Start: a.Start, End: a.End, Repl: []byte(repl)})
}

func escAttr(s string) string {
	return strings.NewReplacer("&", "&amp;", `"`, "&quot;").Replace(s)
}

func (c *conv) srcset(val, where string) (string, bool) {
	parts := strings.Split(val, ",")
	changed := false
	for i, p := range parts {
		f := strings.Fields(strings.TrimSpace(p))
		if len(f) == 0 {
			continue
		}
		if nv, ok := c.ref(f[0], c.in.HTMLDir, nil, where); ok && nv != f[0] {
			f[0] = nv
			changed = true
		}
		parts[i] = strings.Join(f, " ")
	}
	return strings.Join(parts, ", "), changed
}

// useSprite inlines external SVG sprites and points to #id.
func (c *conv) useSprite(t *htmlscan.Tag, a htmlscan.Attr) {
	val := strings.TrimSpace(a.Val)
	if strings.HasPrefix(val, "#") || strings.HasPrefix(strings.ToLower(val), "data:") {
		return
	}
	file, frag := assets.StripRef(val)
	if !assets.IsLocalRef(file) {
		c.rep.Miss(val, c.where(t, a.Name), "external SVG sprites from the network are not supported")
		return
	}
	r, err := c.res.Resolve(file, c.in.HTMLDir)
	if err != nil {
		c.rep.Miss(val, c.where(t, a.Name), err.Error())
		return
	}
	c.warnStage(val, r)
	if !c.sprites[r.Path] {
		data, err := os.ReadFile(r.Path)
		if err != nil {
			c.rep.Miss(val, c.where(t, a.Name), err.Error())
			return
		}
		s := string(data)
		if i := strings.Index(s, "<svg"); i >= 0 {
			s = s[i:]
		}
		c.sprites[r.Path] = true
		c.embedded[r.Path] = true
		c.spriteBuf.WriteString(`<div hidden aria-hidden="true" data-df-sprite="">` + s + `</div>`)
	}
	c.replaceAttr(a, a.Name, frag)
}

// ref resolves a reference and returns the placeholder (or the unchanged value).
// ok is false for an unresolved reference (already recorded in the report).
func (c *conv) ref(val, baseDir string, baseURL *url.URL, where string) (string, bool) {
	v := strings.TrimSpace(val)
	low := strings.ToLower(v)
	switch {
	case v == "" || strings.HasPrefix(v, "#") || strings.HasPrefix(low, "about:") || strings.HasPrefix(low, "blob:") ||
		strings.HasPrefix(low, "javascript:") || strings.HasPrefix(low, "mailto:") || strings.HasPrefix(low, "tel:") ||
		strings.HasPrefix(low, "deckfold:"):
		return val, true
	case strings.HasPrefix(low, "data:"):
		if len(v) < DataURIMin {
			return val, true
		}
		data, mt, ok := assets.ParseDataURI(v)
		if !ok {
			return val, true
		}
		return assets.Placeholder(c.store.Add(data, mt, "data: URI ("+mt+")")), true
	}
	// remote (absolute or relative to a remote base)
	if baseURL != nil || strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") || strings.HasPrefix(v, "//") {
		abs, err := resolveURL(baseURL, v)
		if err != nil {
			c.rep.Miss(v, where, err.Error())
			return val, false
		}
		frag := ""
		if abs.Fragment != "" {
			frag = "#" + abs.Fragment
			abs.Fragment = ""
		}
		data, mt, err := c.opt.Fetcher.Get(abs.String())
		if err != nil {
			c.rep.Miss(abs.String(), where, err.Error())
			return val, false
		}
		return assets.Placeholder(c.store.Add(data, assets.MimeFor(abs.Path, mt, data), abs.String())) + frag, true
	}
	if !assets.IsLocalRef(v) {
		return val, true
	}
	file, suffix := assets.StripRef(v)
	frag := ""
	if i := strings.Index(suffix, "#"); i >= 0 {
		frag = suffix[i:]
	}
	if c.ad.SkipStaticRef(file) {
		return val, true
	}
	r, err := c.res.Resolve(file, baseDir)
	if err != nil {
		c.rep.Miss(v, where, err.Error())
		return val, false
	}
	c.warnStage(v, r)
	data, err := os.ReadFile(r.Path)
	if err != nil {
		c.rep.Miss(v, where, err.Error())
		return val, false
	}
	c.embedded[r.Path] = true
	rel, _ := filepath.Rel(c.res.Root, r.Path)
	return assets.Placeholder(c.store.Add(data, assets.MimeFor(r.Path, "", data), filepath.ToSlash(rel))) + frag, true
}

func (c *conv) warnStage(ref string, r assets.Resolved) {
	if r.Stage <= 1 {
		return
	}
	rel, _ := filepath.Rel(c.res.Root, r.Path)
	why := map[int]string{2: "after URL decoding", 3: "with Unicode normalization", 4: "ignoring upper and lower case"}[r.Stage]
	c.rep.Warn("path corrected (%s): %q -> %s", why, ref, filepath.ToSlash(rel))
}

func resolveURL(base *url.URL, v string) (*url.URL, error) {
	if strings.HasPrefix(v, "//") {
		v = "https:" + v
	}
	u, err := url.Parse(v)
	if err != nil {
		return nil, err
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("scheme %q is not downloaded", u.Scheme)
	}
	return u, nil
}

// ---------- CSS ----------

// processCSS embeds all references and inlines @import. changed reports any change.
func (c *conv) processCSS(css, baseDir string, baseURL *url.URL, where string, depth int, dedupe bool) (string, bool) {
	refs := assets.ScanCSS(css)
	var b strings.Builder
	pos := 0
	changed := false
	for _, r := range refs {
		switch r.Kind {
		case assets.RefImport:
			if r.StmtStart < pos {
				continue
			}
			sub, subDir, subURL, ok := c.loadCSS(r.Value, baseDir, baseURL, where)
			if !ok || depth > 8 {
				continue
			}
			processed, _ := c.processCSS(sub, subDir, subURL, r.Value, depth+1, dedupe)
			b.WriteString(css[pos:r.StmtStart])
			b.WriteString(assets.ImportWrap(processed, r.ImportTail))
			pos = r.StmtEnd
			changed = true
		default:
			if r.Start < pos {
				continue
			}
			nv, ok := c.ref(r.Value, baseDir, baseURL, where)
			if !ok || nv == r.Value {
				continue
			}
			b.WriteString(css[pos:r.Start])
			b.WriteString(nv)
			pos = r.End
			changed = true
		}
	}
	b.WriteString(css[pos:])
	out := b.String()
	if dedupe {
		var n int
		out, n = clean.DedupeFontFaces(out, c.fontFaces)
		if n > 0 {
			c.rep.Warn("%d duplicate @font-face rule(s) merged (%s)", n, where)
			changed = true
		}
	}
	return out, changed
}

// loadCSS loads a stylesheet locally or from the network.
func (c *conv) loadCSS(ref, baseDir string, baseURL *url.URL, where string) (css, dir string, u *url.URL, ok bool) {
	low := strings.ToLower(strings.TrimSpace(ref))
	if baseURL != nil || strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") || strings.HasPrefix(low, "//") {
		abs, err := resolveURL(baseURL, strings.TrimSpace(ref))
		if err != nil {
			c.rep.Miss(ref, where, err.Error())
			return "", "", nil, false
		}
		data, _, err := c.opt.Fetcher.Get(abs.String())
		if err != nil {
			c.rep.Miss(abs.String(), where, err.Error())
			return "", "", nil, false
		}
		c.dropHosts[abs.Scheme+"://"+abs.Host] = true
		return string(data), "", abs, true
	}
	file, _ := assets.StripRef(strings.TrimSpace(ref))
	r, err := c.res.Resolve(file, baseDir)
	if err != nil {
		c.rep.Miss(ref, where, err.Error())
		return "", "", nil, false
	}
	c.warnStage(ref, r)
	data, err := os.ReadFile(r.Path)
	if err != nil {
		c.rep.Miss(ref, where, err.Error())
		return "", "", nil, false
	}
	c.embedded[r.Path] = true
	return string(data), filepath.Dir(r.Path), nil, true
}

// cssBlock creates the inactive replacement for a stylesheet.
func cssBlock(css, media string) []byte {
	// "</" in the CSS would end the script block
	css = strings.ReplaceAll(css, "</", `<\/`)
	attr := ""
	if media != "" {
		attr = ` data-df-media="` + escAttr(media) + `"`
	}
	return []byte(`<script type="text/df-css"` + attr + `>` + css + `</script>`)
}

func (c *conv) link(t *htmlscan.Tag) {
	rel := strings.ToLower(strings.Join(strings.Fields(t.Value("rel")), " "))
	href := strings.TrimSpace(t.Value("href"))
	switch {
	case hasWord(rel, "stylesheet") && !hasWord(rel, "alternate"):
		key := c.refKey(href, c.in.HTMLDir)
		if !c.seenRes.First("css:" + key) {
			c.rep.Warn("duplicate stylesheet include removed: %s", href)
			c.patches = append(c.patches, patch.Patch{Start: t.Start, End: t.End})
			return
		}
		css, dir, u, ok := c.loadCSS(href, c.in.HTMLDir, nil, c.where(t, "href"))
		if !ok {
			return
		}
		processed, _ := c.processCSS(css, dir, u, href, 0, true)
		c.patches = append(c.patches, patch.Patch{Start: t.Start, End: t.End, Repl: cssBlock(processed, t.Value("media"))})
	case hasWord(rel, "preconnect") || hasWord(rel, "dns-prefetch") || hasWord(rel, "prefetch") || hasWord(rel, "preload") || hasWord(rel, "modulepreload") || hasWord(rel, "prerender"):
		// would point nowhere after embedding, or cause network requests
		c.patches = append(c.patches, patch.Patch{Start: t.Start, End: t.End})
	case hasWord(rel, "icon") || hasWord(rel, "apple-touch-icon") || hasWord(rel, "mask-icon"):
		if a, ok := t.Get("href"); ok && a.ValStart >= 0 && href != "" {
			if nv, ok := c.ref(href, c.in.HTMLDir, nil, c.where(t, "href")); ok && nv != href {
				c.replaceAttr(a, "data-df-href", nv)
			}
		}
	}
}

func hasWord(s, w string) bool {
	for _, f := range strings.Fields(s) {
		if f == w {
			return true
		}
	}
	return false
}

// refKey normalizes a reference for detecting duplicate includes.
func (c *conv) refKey(ref, baseDir string) string {
	r := strings.TrimSpace(ref)
	low := strings.ToLower(r)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") || strings.HasPrefix(low, "//") {
		if u, err := resolveURL(nil, r); err == nil {
			u.Fragment = ""
			return u.String()
		}
		return r
	}
	file, _ := assets.StripRef(r)
	if res, err := c.res.Resolve(file, baseDir); err == nil {
		return res.Path
	}
	return path.Clean(file)
}

func (c *conv) style(t *htmlscan.Tag) {
	if t.CloseEnd < 0 {
		return
	}
	css := string(c.in.Doc.Content(t))
	processed, _ := c.processCSS(css, c.in.HTMLDir, nil, c.where(t, ""), 0, true)
	c.patches = append(c.patches, patch.Patch{Start: t.Start, End: t.CloseEnd, Repl: cssBlock(processed, t.Value("media"))})
}

func (c *conv) script(t *htmlscan.Tag) {
	typ := strings.ToLower(strings.TrimSpace(t.Value("type")))
	if i := strings.Index(typ, ";"); i >= 0 {
		typ = strings.TrimSpace(typ[:i])
	}
	if !executable[typ] {
		return // data scripts (e.g. application/json) stay unchanged
	}
	src := strings.TrimSpace(t.Value("src"))
	var b strings.Builder
	b.WriteString("<script")
	if src != "" {
		key := c.refKey(src, c.in.HTMLDir)
		if !c.seenRes.First("js:" + key) {
			c.rep.Warn("duplicate script include removed: %s", src)
			end := t.End
			if t.CloseEnd > 0 {
				end = t.CloseEnd
			}
			c.patches = append(c.patches, patch.Patch{Start: t.Start, End: end})
			return
		}
	} else {
		c.inline = append(c.inline, c.in.Doc.Content(t))
	}
	for _, a := range t.Attrs {
		switch a.Name {
		case "type":
			continue
		case "src":
			nv, ok := c.ref(src, c.in.HTMLDir, nil, c.where(t, "src"))
			if !ok {
				nv = src
			}
			b.WriteString(` data-df-src="` + escAttr(nv) + `"`)
			continue
		case "integrity", "crossorigin":
			continue // does not apply to blob URLs
		}
		b.WriteString(" ")
		b.Write(c.in.Src[a.Start:a.End])
	}
	b.WriteString(` data-df-type="` + escAttr(typ) + `" type="text/df-deferred">`)
	c.patches = append(c.patches, patch.Patch{Start: t.Start, End: t.End, Repl: []byte(b.String())})
}

// ---------- CSP ----------

func (c *conv) cspTag() (*htmlscan.Tag, htmlscan.Attr, bool) {
	for _, t := range c.in.Doc.Tags {
		if t.Name == "meta" && strings.EqualFold(t.Value("http-equiv"), "content-security-policy") {
			if a, ok := t.Get("content"); ok && a.ValStart >= 0 {
				return t, a, true
			}
		}
	}
	return nil, htmlscan.Attr{}, false
}

func (c *conv) cspBlocksInlineStyle() bool {
	_, a, ok := c.cspTag()
	if !ok {
		return false
	}
	p := csp.Parse(a.Val)
	for _, d := range []string{"style-src-elem", "style-src", "default-src"} {
		if v, ok := p.Values[d]; ok {
			for _, s := range v {
				if s == "'unsafe-inline'" {
					return false
				}
			}
			return true
		}
	}
	return false
}

func (c *conv) rewriteCSP(runtimeJS []byte) {
	_, a, ok := c.cspTag()
	if !ok {
		return
	}
	var frames []string
	for o := range c.frameOrig {
		frames = append(frames, o)
	}
	sort.Strings(frames)
	var drop []string
	for h := range c.dropHosts {
		drop = append(drop, h)
	}
	// also drop the embedded Google Fonts hosts (fonts come from fonts.gstatic.com)
	if c.dropHosts["https://fonts.googleapis.com"] {
		drop = append(drop, "fonts.gstatic.com", "fonts.googleapis.com")
	}
	scripts := append([][]byte{}, c.inline...)
	scripts = append(scripts, runtimeJS)
	np := csp.Rewrite(a.Val, csp.Options{FrameOrigins: frames, InlineScripts: scripts, DropHosts: drop})
	c.patches = append(c.patches, patch.Patch{Start: a.ValStart, End: a.ValEnd, Repl: []byte(escAttr(np))})
}

// ---------- positions ----------

func (c *conv) headInsertPos() int {
	// After the charset declaration, so it stays within the first 1024 bytes.
	for _, t := range c.in.Doc.Tags {
		if t.Name == "meta" {
			if _, ok := t.Get("charset"); ok {
				return t.End
			}
			if strings.EqualFold(t.Value("http-equiv"), "content-type") {
				return t.End
			}
		}
		if t.Name == "body" {
			break
		}
	}
	for _, t := range c.in.Doc.Tags {
		if t.Name == "head" {
			return t.End
		}
	}
	for _, t := range c.in.Doc.Tags {
		if t.Name == "html" {
			return t.End
		}
	}
	return c.in.Doc.DoctypeEnd
}

func (c *conv) bodyStartPos() int {
	for _, t := range c.in.Doc.Tags {
		if t.Name == "body" {
			return t.End
		}
	}
	if c.in.Doc.BodyClose >= 0 {
		return c.in.Doc.BodyClose
	}
	return len(c.in.Src)
}

// reportUnused reports, for folder inputs, how many unreferenced files were left out.
func (c *conv) reportUnused() {
	if !c.in.IsDir {
		return
	}
	n, size := 0, int64(0)
	_ = filepath.WalkDir(c.in.Root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		rp, _ := filepath.EvalSymlinks(p)
		if rp == "" {
			rp = p
		}
		if p == c.in.HTMLPath || c.embedded[rp] || c.embedded[p] {
			return nil
		}
		if fi, err := d.Info(); err == nil {
			size += fi.Size()
		}
		n++
		return nil
	})
	if n > 0 {
		c.rep.Warn("%d unreferenced file(s) in the input folder left out (%s)", n, report.HumanSize(int(size)))
	}
}

func realOr(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}
