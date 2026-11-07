// Package csp parses and extends a Content-Security-Policy from a meta tag.
package csp

import (
	"crypto/sha256"
	"encoding/base64"
	"sort"
	"strings"
)

// Policy is an ordered list of directives.
type Policy struct {
	Names  []string
	Values map[string][]string
}

// Parse splits a CSP string.
func Parse(s string) *Policy {
	p := &Policy{Values: map[string][]string{}}
	for _, d := range strings.Split(s, ";") {
		f := strings.Fields(d)
		if len(f) == 0 {
			continue
		}
		name := strings.ToLower(f[0])
		if _, ok := p.Values[name]; ok {
			continue // duplicate directive: browsers use the first one
		}
		p.Names = append(p.Names, name)
		p.Values[name] = f[1:]
	}
	return p
}

// String joins the policy back together.
func (p *Policy) String() string {
	var parts []string
	for _, n := range p.Names {
		v := p.Values[n]
		if len(v) == 0 {
			parts = append(parts, n)
		} else {
			parts = append(parts, n+" "+strings.Join(v, " "))
		}
	}
	return strings.Join(parts, "; ")
}

// Has reports whether the directive exists.
func (p *Policy) Has(name string) bool { _, ok := p.Values[name]; return ok }

// effective returns the sources in effect for a directive (falling back to default-src).
func (p *Policy) effective(name string) ([]string, bool) {
	if v, ok := p.Values[name]; ok {
		return v, true
	}
	switch name {
	case "frame-src", "worker-src":
		if v, ok := p.Values["child-src"]; ok {
			return v, true
		}
	case "script-src-elem":
		if v, ok := p.Values["script-src"]; ok {
			return v, true
		}
	case "style-src-elem", "style-src-attr":
		if v, ok := p.Values["style-src"]; ok {
			return v, true
		}
	}
	if v, ok := p.Values["default-src"]; ok {
		return v, true
	}
	return nil, false
}

// Allow adds sources to a directive. If the directive does not apply (no own entry, no
// default-src), nothing is needed. Otherwise it is materialized from the list in effect.
func (p *Policy) Allow(name string, sources ...string) {
	cur, ok := p.effective(name)
	if !ok {
		return
	}
	vals := append([]string(nil), cur...)
	// 'none' cannot be combined with other sources
	if len(vals) == 1 && vals[0] == "'none'" {
		vals = nil
	}
	for _, s := range sources {
		if !contains(vals, s) {
			vals = append(vals, s)
		}
	}
	if _, own := p.Values[name]; !own {
		p.Names = append(p.Names, name)
	}
	p.Values[name] = vals
}

// Remove drops the sources matching a condition from a directive.
func (p *Policy) Remove(name string, drop func(string) bool) {
	v, ok := p.Values[name]
	if !ok {
		return
	}
	var keep []string
	for _, s := range v {
		if !drop(s) {
			keep = append(keep, s)
		}
	}
	if len(keep) == 0 {
		keep = []string{"'none'"}
	}
	p.Values[name] = keep
}

// UsesHashOrNonce reports whether script-src uses hashes or nonces.
func (p *Policy) UsesHashOrNonce() bool {
	v, _ := p.effective("script-src")
	for _, s := range v {
		ls := strings.ToLower(s)
		if strings.HasPrefix(ls, "'sha") || strings.HasPrefix(ls, "'nonce-") {
			return true
		}
	}
	return false
}

// AllowsUnsafeInline reports whether script-src allows 'unsafe-inline'.
func (p *Policy) AllowsUnsafeInline() bool {
	v, ok := p.effective("script-src")
	if !ok {
		return true
	}
	return contains(v, "'unsafe-inline'")
}

// ScriptHash returns the CSP hash of an inline script.
func ScriptHash(code []byte) string {
	s := sha256.Sum256(code)
	return "'sha256-" + base64.StdEncoding.EncodeToString(s[:]) + "'"
}

func contains(v []string, s string) bool {
	for _, x := range v {
		if x == s {
			return true
		}
	}
	return false
}

// Options describes the extensions deckfold needs.
type Options struct {
	FrameOrigins  []string // origins of live embeds
	InlineScripts [][]byte // all inline scripts (source + deckfold), only relevant for hash/nonce policies
	DropHosts     []string // external hosts that are now embedded (e.g. Google Fonts)
}

// Rewrite extends the policy for the self-contained output.
func Rewrite(s string, o Options) string {
	p := Parse(s)
	for _, d := range []string{"img-src", "media-src", "font-src", "style-src", "style-src-elem"} {
		if p.Has(d) || (d != "style-src-elem" && p.Has("default-src")) {
			p.Allow(d, "blob:", "data:")
		}
	}
	// blob scripts (JSONP, worker code) and srcdoc iframes for the previews
	if p.Has("script-src") || p.Has("default-src") {
		p.Allow("script-src", "blob:")
	}
	if p.Has("worker-src") || p.Has("child-src") || p.Has("script-src") || p.Has("default-src") {
		p.Allow("worker-src", "blob:")
	}
	frames := append([]string{"about:", "blob:"}, o.FrameOrigins...)
	sort.Strings(frames[2:])
	if p.Has("frame-src") || p.Has("child-src") || p.Has("default-src") {
		p.Allow("frame-src", frames...)
	}
	// remove hosts that are now embedded
	for _, d := range p.Names {
		p.Remove(d, func(src string) bool {
			for _, h := range o.DropHosts {
				if strings.Contains(src, h) {
					return true
				}
			}
			return false
		})
	}
	// Add hashes only if the source already uses hashes or nonces. Otherwise a hash would
	// disable 'unsafe-inline' and block the source scripts.
	if p.UsesHashOrNonce() {
		var hs []string
		for _, c := range o.InlineScripts {
			hs = append(hs, ScriptHash(c))
		}
		p.Allow("script-src", hs...)
	}
	return p.String()
}
