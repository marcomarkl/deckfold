package pptxgen

import (
	"fmt"
	"strings"
)

// Timeline builds a p:timing element the way PowerPoint writes it: a main sequence of click
// groups, each with sub-groups that start after a delay, each holding effects.
type Timeline struct {
	id     int
	groups []string
	builds []string
}

// Group is a click group (or, when Auto is set, a group that starts after the previous one).
type Group struct {
	Auto bool
	Subs []Sub
}

// Sub is a sub-group: effects that start together, Delay ms after the start of the group.
type Sub struct {
	Delay   int
	Effects []Eff
}

// Eff is one effect. Bhv builds the behaviors with a target string from Target.
type Eff struct {
	Spid     int
	Para     int // paragraph index for a paragraph build, -1 for the whole shape
	Class    string
	Preset   int
	Subtype  int
	NodeType string // clickEffect, withEffect, afterEffect
	Delay    int
	Bhv      func(tl *Timeline, tgt string) string
}

// Target returns the p:tgtEl of an effect.
func (e Eff) target() string {
	if e.Para >= 0 {
		return fmt.Sprintf(`<p:tgtEl><p:spTgt spid="%d"><p:txEl><p:pRg st="%d" end="%d"/></p:txEl></p:spTgt></p:tgtEl>`, e.Spid, e.Para, e.Para)
	}
	return fmt.Sprintf(`<p:tgtEl><p:spTgt spid="%d"/></p:tgtEl>`, e.Spid)
}

func (tl *Timeline) next() int { tl.id++; return tl.id }

// Add appends a group.
func (tl *Timeline) Add(g Group) {
	var b strings.Builder
	cond := `<p:cond delay="indefinite"/>`
	if g.Auto {
		cond = `<p:cond delay="indefinite"/><p:cond evt="onBegin" delay="0"><p:tn val="2"/></p:cond>`
	}
	fmt.Fprintf(&b, `<p:par><p:cTn id="%d" fill="hold"><p:stCondLst>%s</p:stCondLst><p:childTnLst>`, tl.next(), cond)
	for _, s := range g.Subs {
		fmt.Fprintf(&b, `<p:par><p:cTn id="%d" fill="hold"><p:stCondLst><p:cond delay="%d"/></p:stCondLst><p:childTnLst>`, tl.next(), s.Delay)
		for _, e := range s.Effects {
			fmt.Fprintf(&b, `<p:par><p:cTn id="%d" presetID="%d" presetClass="%s" presetSubtype="%d" fill="hold" grpId="0" nodeType="%s"><p:stCondLst><p:cond delay="%d"/></p:stCondLst><p:childTnLst>%s</p:childTnLst></p:cTn></p:par>`,
				tl.next(), e.Preset, e.Class, e.Subtype, e.NodeType, e.Delay, e.Bhv(tl, e.target()))
			if e.Para >= 0 {
				b := fmt.Sprintf(`<p:bldP spid="%d" grpId="0" build="p"/>`, e.Spid)
				if !contains(tl.builds, b) {
					tl.builds = append(tl.builds, b)
				}
			}
		}
		b.WriteString(`</p:childTnLst></p:cTn></p:par>`)
	}
	b.WriteString(`</p:childTnLst></p:cTn></p:par>`)
	tl.groups = append(tl.groups, b.String())
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// XML returns the p:timing element.
func (tl *Timeline) XML() string {
	bld := ""
	if len(tl.builds) > 0 {
		bld = `<p:bldLst>` + strings.Join(tl.builds, "") + `</p:bldLst>`
	}
	return `<p:timing><p:tnLst><p:par><p:cTn id="1" dur="indefinite" restart="never" nodeType="tmRoot"><p:childTnLst>` +
		`<p:seq concurrent="1" nextAc="seek"><p:cTn id="2" dur="indefinite" nodeType="mainSeq"><p:childTnLst>` + strings.Join(tl.groups, "") +
		`</p:childTnLst></p:cTn><p:prevCondLst><p:cond evt="onPrev" delay="0"><p:tgtEl><p:sldTgt/></p:tgtEl></p:cond></p:prevCondLst>` +
		`<p:nextCondLst><p:cond evt="onNext" delay="0"><p:tgtEl><p:sldTgt/></p:tgtEl></p:cond></p:nextCondLst></p:seq>` +
		`</p:childTnLst></p:cTn></p:par></p:tnLst>` + bld + `</p:timing>`
}

// behavior helpers (the XML PowerPoint writes for the common presets)

func ctn(tl *Timeline, dur, delay int, extra string) string {
	d := ""
	if delay > 0 {
		d = fmt.Sprintf(`<p:stCondLst><p:cond delay="%d"/></p:stCondLst>`, delay)
	}
	return fmt.Sprintf(`<p:cTn id="%d" dur="%d" fill="hold"%s>%s</p:cTn>`, tl.next(), dur, extra, d)
}

// SetVisible: <p:set> style.visibility to visible (or hidden) after delay ms.
func SetVisible(vis string, delay int) func(*Timeline, string) string {
	return func(tl *Timeline, tgt string) string {
		return `<p:set><p:cBhvr>` + ctn(tl, 1, delay, "") + tgt + `<p:attrNameLst><p:attrName>style.visibility</p:attrName></p:attrNameLst></p:cBhvr><p:to><p:strVal val="` + vis + `"/></p:to></p:set>`
	}
}

// Filter: <p:animEffect> with a filter (fade, wipe(down), ...), in or out.
func Filter(transition, filter string, dur int) func(*Timeline, string) string {
	return func(tl *Timeline, tgt string) string {
		return `<p:animEffect transition="` + transition + `" filter="` + filter + `"><p:cBhvr>` + ctn(tl, dur, 0, "") + tgt + `</p:cBhvr></p:animEffect>`
	}
}

// Prop: <p:anim> of one property with two keyframes (values may be formulas such as #ppt_y).
func Prop(attr, from, to string, dur int) func(*Timeline, string) string {
	return func(tl *Timeline, tgt string) string {
		return `<p:anim calcmode="lin" valueType="num"><p:cBhvr additive="base">` + ctn(tl, dur, 0, "") + tgt + `<p:attrNameLst><p:attrName>` + attr + `</p:attrName></p:attrNameLst></p:cBhvr>` +
			`<p:tavLst><p:tav tm="0"><p:val><p:strVal val="` + from + `"/></p:val></p:tav><p:tav tm="100000"><p:val><p:strVal val="` + to + `"/></p:val></p:tav></p:tavLst></p:anim>`
	}
}

// Scale: <p:animScale> by a factor (percent).
func Scale(pct, dur int) func(*Timeline, string) string {
	return func(tl *Timeline, tgt string) string {
		return `<p:animScale><p:cBhvr>` + ctn(tl, dur, 0, ` autoRev="0"`) + tgt + `</p:cBhvr><p:by x="` + fmt.Sprint(pct*1000) + `" y="` + fmt.Sprint(pct*1000) + `"/></p:animScale>`
	}
}

// Motion: <p:animMotion> along a path in fractions of the slide.
func Motion(path string, dur int) func(*Timeline, string) string {
	return func(tl *Timeline, tgt string) string {
		return `<p:animMotion origin="layout" path="` + path + `" pathEditMode="relative" ptsTypes=""><p:cBhvr>` + ctn(tl, dur, 0, "") + tgt + `<p:attrNameLst><p:attrName>ppt_x</p:attrName><p:attrName>ppt_y</p:attrName></p:attrNameLst></p:cBhvr></p:animMotion>`
	}
}

// All combines behavior builders.
func All(bs ...func(*Timeline, string) string) func(*Timeline, string) string {
	return func(tl *Timeline, tgt string) string {
		var s string
		for _, b := range bs {
			s += b(tl, tgt)
		}
		return s
	}
}

// Entrance and exit presets as PowerPoint writes them.
var (
	FadeIn  = All(SetVisible("visible", 0), Filter("in", "fade", 500))
	FadeOut = All(Filter("out", "fade", 500), SetVisible("hidden", 499))
	FlyIn   = All(SetVisible("visible", 0), Prop("ppt_x", "#ppt_x", "#ppt_x", 500), Prop("ppt_y", "1+#ppt_h/2", "#ppt_y", 500))
	WipeIn  = All(SetVisible("visible", 0), Filter("in", "wipe(down)", 500))
)
