// Package anim reads the timing of a slide (p:timing) and its transition (p:transition) into the
// steps the player plays: step 0 holds what runs when the slide appears, every click group of the
// main sequence adds a step. Effects that follow a click group automatically (after previous) are
// added to that step, delayed by the end of the group before them.
//
// The behaviors stay close to PresentationML (set, anim, animEffect, animMotion, animRot,
// animScale, cmd), so that the player can interpret any preset, not only known preset IDs.
package anim

import (
	"strconv"
	"strings"

	"deckfold/internal/pptx/xmltree"
)

// Slide is the animation data of one slide.
type Slide struct {
	Steps      []Step      `json:"steps"` // steps[0]: effects on entry
	Transition *Transition `json:"transition,omitempty"`
	Advance    *float64    `json:"advance,omitempty"` // seconds until the slide advances by itself
	AdvClick   bool        `json:"advClick"`          // a click advances
	// Hidden lists targets that are invisible when the slide appears (their first effect is an
	// entrance).
	Hidden []string `json:"hidden,omitempty"`
	// Triggers are the interactive sequences: a click on the trigger shape plays its next step.
	Triggers []Trigger `json:"triggers,omitempty"`
}

// Trigger is an interactive sequence of a slide.
type Trigger struct {
	Shape string `json:"shape"` // spid of the shape that starts the sequence when clicked
	Steps []Step `json:"steps"` // one step per click group
}

// Step is one click step.
type Step struct {
	Effects []Effect `json:"effects"`
	Dur     float64  `json:"dur"` // ms until the last behavior ends
}

// Effect is one effect on one target.
type Effect struct {
	Target string     `json:"target"`          // "<spid>" or "<spid>:p<n>" for paragraph n
	Class  string     `json:"class,omitempty"` // entr, exit, emph, path, mediacall
	Preset int        `json:"preset,omitempty"`
	Begin  float64    `json:"begin"` // ms from the start of the step
	Bhv    []Behavior `json:"bhv"`
	// Iterate plays the effect word by word ("wd") or letter by letter ("lt"), each unit
	// IterDelay ms after the one before.
	Iterate   string  `json:"iterate,omitempty"`
	IterDelay float64 `json:"iterDelay,omitempty"`
}

// Behavior is one animation behavior.
type Behavior struct {
	Type       string   `json:"type"` // set, anim, effect, motion, rot, scale, clr, cmd
	Begin      float64  `json:"begin"`
	Dur        float64  `json:"dur"`
	Attr       []string `json:"attr,omitempty"`
	From       string   `json:"from,omitempty"`
	To         string   `json:"to,omitempty"`
	By         string   `json:"by,omitempty"`
	Keys       []Key    `json:"keys,omitempty"`
	Filter     string   `json:"filter,omitempty"`
	Transition string   `json:"transition,omitempty"` // in, out (animEffect)
	Path       string   `json:"path,omitempty"`
	Cmd        string   `json:"cmd,omitempty"`
	Accel      float64  `json:"accel,omitempty"`
	Decel      float64  `json:"decel,omitempty"`
	AutoRev    bool     `json:"autoRev,omitempty"`
	Repeat     float64  `json:"repeat,omitempty"` // repeat count (0 = once, -1 = indefinite)
	Additive   string   `json:"additive,omitempty"`
}

// Key is a keyframe of p:anim.
type Key struct {
	T    float64 `json:"t"` // 0..1
	V    string  `json:"v"`
	Fmla string  `json:"fmla,omitempty"`
}

// Transition is the slide transition.
type Transition struct {
	Type string  `json:"type"` // fade, push, wipe, cover, pull, cut, ... (fallback: fade)
	Dir  string  `json:"dir,omitempty"`
	Dur  float64 `json:"dur"` // ms
	Orig string  `json:"orig,omitempty"`
}

// Report collects what the player cannot show as in PowerPoint.
type Report struct {
	Bookmarks bool     // sequences started by a media bookmark were ignored
	Fallback  []string // transitions played as a fade
	Unknown   []string // behaviors the player plays as appear
}

// Read reads the timing and transition of a slide. paraCount returns the number of paragraphs of
// a shape, for builds with a paragraph range.
func Read(sld *xmltree.Node, rep *Report) Slide {
	out := Slide{Steps: []Step{{}}, AdvClick: true}
	readTransition(sld, &out, rep)
	timing := sld.Child("timing")
	root := timing.Path("tnLst/par/cTn")
	if root == nil {
		return out
	}
	for _, seq := range root.Path("childTnLst").Children("seq") {
		ctn := seq.Child("cTn")
		switch ctn.Attr("nodeType") {
		case "mainSeq":
			readMain(ctn, &out, rep)
		case "interactiveSeq":
			readInteractive(ctn, &out, rep)
		}
	}
	// targets whose first effect is an entrance start hidden; the main sequence comes first,
	// then the interactive sequences
	all := out.Steps
	for _, tr := range out.Triggers {
		all = append(all[:len(all):len(all)], tr.Steps...)
	}
	first := map[string]string{}
	for _, st := range all {
		for _, e := range st.Effects {
			if _, ok := first[e.Target]; !ok {
				first[e.Target] = e.Class
			}
		}
	}
	for _, st := range all {
		for _, e := range st.Effects {
			if first[e.Target] == "entr" {
				out.Hidden = append(out.Hidden, e.Target)
				first[e.Target] = "done"
			}
		}
	}
	return out
}

// readInteractive reads an interactive sequence. A sequence that only plays or pauses the
// medium that starts it ("play on click" of a video) is left to the player's own click handling
// of media.
func readInteractive(seq *xmltree.Node, out *Slide, rep *Report) {
	shape := ""
	for _, c := range seq.Path("stCondLst").Children("cond") {
		switch c.Attr("evt") {
		case "onClick":
			if t := c.Path("tgtEl/spTgt"); t != nil {
				shape = t.Attr("spid")
			}
		case "onMediaBookmark":
			rep.Bookmarks = true
			return
		}
	}
	if shape == "" {
		return
	}
	var steps []Step
	readGroups(seq, &steps, rep)
	ownMedia := true
	for _, st := range steps {
		for _, e := range st.Effects {
			if e.Class != "mediacall" || e.Target != shape {
				ownMedia = false
			}
		}
	}
	if len(steps) == 0 || ownMedia {
		return
	}
	out.Triggers = append(out.Triggers, Trigger{Shape: shape, Steps: steps})
}

func readMain(seq *xmltree.Node, out *Slide, rep *Report) { readGroups(seq, &out.Steps, rep) }

// readGroups adds the click groups of a sequence to steps: a click group starts a new step, an
// automatic group joins the step before it (or the first step when there is none yet).
func readGroups(seq *xmltree.Node, steps *[]Step, rep *Report) {
	step := 0
	for _, grp := range seq.Path("childTnLst").Children("par") {
		gctn := grp.Child("cTn")
		effects := collect(gctn, 0, rep)
		if len(effects) == 0 {
			continue
		}
		click := firstNodeType(gctn) == "clickEffect"
		if click || len(*steps) == 0 {
			*steps = append(*steps, Step{})
			step = len(*steps) - 1
		}
		st := &(*steps)[step]
		offset := 0.0
		if !click {
			offset = st.Dur // an automatic group starts after the group before it
		}
		for _, e := range effects {
			e.Begin += offset
			st.Effects = append(st.Effects, e)
			if end := e.Begin + effectEnd(e); end > st.Dur {
				st.Dur = end
			}
		}
	}
}

func firstNodeType(ctn *xmltree.Node) string {
	for _, k := range ctn.All("cTn") {
		if t := k.Attr("nodeType"); t == "clickEffect" || t == "withEffect" || t == "afterEffect" {
			return t
		}
	}
	return ""
}

func effectEnd(e Effect) float64 {
	end := 0.0
	for _, b := range e.Bhv {
		d := b.Dur
		if b.AutoRev {
			d *= 2
		}
		if b.Repeat > 1 {
			d *= b.Repeat
		}
		if b.Begin+d > end {
			end = b.Begin + d
		}
	}
	return end
}

// delayOf returns the start delay (ms) of a time node, 0 for "indefinite" or a trigger.
func delayOf(ctn *xmltree.Node) float64 {
	for _, c := range ctn.Path("stCondLst").Children("cond") {
		if d := c.Attr("delay"); d != "" && d != "indefinite" {
			v, _ := strconv.ParseFloat(d, 64)
			return v
		}
	}
	return 0
}

// collect walks a group and returns its effects with begin times relative to the group.
func collect(ctn *xmltree.Node, base float64, rep *Report) []Effect {
	var out []Effect
	for _, par := range ctn.Path("childTnLst").Children("par") {
		c := par.Child("cTn")
		at := base + delayOf(c)
		if cls := c.Attr("presetClass"); cls != "" || c.Attr("nodeType") != "" && hasBehaviors(c) {
			e := Effect{Class: cls, Begin: at}
			e.Preset = int(c.Int("presetID", 0))
			it := c.Child("iterate")
			for _, bnode := range c.Path("childTnLst").Kids {
				b, tgt, ok := behavior(bnode, rep)
				if !ok {
					continue
				}
				if e.Target == "" {
					e.Target = tgt
				}
				e.Bhv = append(e.Bhv, b)
			}
			if it != nil && (it.Attr("type") == "wd" || it.Attr("type") == "lt") {
				e.Iterate = it.Attr("type")
				if v := it.Child("tmAbs"); v != nil {
					e.IterDelay = num(v.Attr("val"))
				} else {
					pct := 0.1
					if v := it.Child("tmPct"); v != nil {
						pct = num(v.Attr("val")) / 100000
					}
					e.IterDelay = pct * effectEnd(e)
				}
			}
			if e.Target != "" && len(e.Bhv) > 0 {
				out = append(out, e)
			}
			continue
		}
		out = append(out, collect(c, at, rep)...)
	}
	return out
}

func hasBehaviors(c *xmltree.Node) bool {
	for _, k := range c.Path("childTnLst").Kids {
		switch k.Name {
		case "set", "anim", "animEffect", "animMotion", "animRot", "animScale", "animClr", "cmd", "audio", "video":
			return true
		}
	}
	return false
}

func num(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

func behavior(n *xmltree.Node, rep *Report) (Behavior, string, bool) {
	cb := n.Child("cBhvr")
	if cb == nil {
		return Behavior{}, "", false
	}
	ctn := cb.Child("cTn")
	b := Behavior{Begin: delayOf(ctn)}
	switch d := ctn.Attr("dur"); d {
	case "", "indefinite":
		b.Dur = 1
	default:
		b.Dur = num(d)
	}
	b.Accel = num(ctn.Attr("accel")) / 100000
	b.Decel = num(ctn.Attr("decel")) / 100000
	b.AutoRev = ctn.Bool("autoRev", false)
	if rc := ctn.Attr("repeatCount"); rc != "" {
		if rc == "indefinite" {
			b.Repeat = -1
		} else {
			b.Repeat = num(rc) / 1000
		}
	}
	for _, a := range cb.Path("attrNameLst").Children("attrName") {
		b.Attr = append(b.Attr, strings.TrimSpace(a.AllText()))
	}
	b.Additive = cb.Attr("additive")
	switch n.Name {
	case "set":
		b.Type = "set"
		b.To = value(n.Child("to"))
	case "anim":
		b.Type = "anim"
		b.From, b.To, b.By = n.Attr("from"), n.Attr("to"), n.Attr("by")
		for _, tav := range n.Path("tavLst").Children("tav") {
			k := Key{T: num(tav.Attr("tm")) / 100000, V: value(tav.Child("val")), Fmla: tav.Attr("fmla")}
			if tav.Attr("tm") == "indefinite" {
				k.T = 1
			}
			b.Keys = append(b.Keys, k)
		}
	case "animEffect":
		b.Type = "effect"
		b.Filter = n.Attr("filter")
		b.Transition = n.Attr("transition")
	case "animMotion":
		b.Type = "motion"
		b.Path = n.Attr("path")
		if b.Path == "" {
			fx, fy := n.Child("from"), n.Child("to")
			if by := n.Child("by"); by != nil {
				b.Path = "M 0 0 L " + ftoa(num(by.Attr("x"))/100000) + " " + ftoa(num(by.Attr("y"))/100000)
			} else if fx != nil || fy != nil {
				b.Path = "M " + ftoa(num(fx.Attr("x"))/100000) + " " + ftoa(num(fx.Attr("y"))/100000) + " L " + ftoa(num(fy.Attr("x"))/100000) + " " + ftoa(num(fy.Attr("y"))/100000)
			}
		}
	case "animRot":
		b.Type = "rot"
		b.By, b.From, b.To = ftoa(num(n.Attr("by"))/60000), n.Attr("from"), n.Attr("to")
	case "animScale":
		b.Type = "scale"
		if by := n.Child("by"); by != nil {
			b.By = ftoa(num(by.Attr("x"))/100000) + " " + ftoa(num(by.Attr("y"))/100000)
		}
		if to := n.Child("to"); to != nil {
			b.To = ftoa(num(to.Attr("x"))/100000) + " " + ftoa(num(to.Attr("y"))/100000)
		}
	case "animClr":
		b.Type = "clr"
		rep.Unknown = appendOnce(rep.Unknown, "color change")
	case "cmd":
		b.Type = "cmd"
		b.Cmd = n.Attr("cmd")
	default:
		return Behavior{}, "", false
	}
	return b, target(cb.Child("tgtEl")), true
}

func value(v *xmltree.Node) string {
	if v == nil {
		return ""
	}
	for _, k := range v.Kids {
		switch k.Name {
		case "strVal", "fltVal", "intVal", "boolVal":
			return k.Attr("val")
		}
	}
	return ""
}

func target(t *xmltree.Node) string {
	sp := t.Child("spTgt")
	if sp == nil {
		return ""
	}
	id := sp.Attr("spid")
	if pr := sp.Path("txEl/pRg"); pr != nil {
		return id + ":p" + pr.Attr("st")
	}
	return id
}

func ftoa(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func appendOnce(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}

// transitions the player draws itself: in 2D with transforms and clip paths, in 3D with CSS
// transforms, and as tiles with three.js (falling back to a fade without WebGL); everything
// else is played as a fade. Transitions of PowerPoint 2010 and later (p14, p15) are known by
// their local names, a p15:prstTrans by its preset.
var knownTransitions = map[string]bool{
	"morph": true, "fade": true, "push": true, "wipe": true, "cover": true, "pull": true, "cut": true, "split": true,
	"circle": true, "zoom": true, "dissolve": true, "strips": true, "blinds": true, "checker": true, "comb": true,
	"randomBar": true, "wheel": true, "wheelReverse": true, "wedge": true, "diamond": true, "plus": true, "newsflash": true,
	"reveal": true, "flash": true, "flythrough": true, "warp": true,
	// 3D
	"prism": true, "flip": true, "gallery": true, "conveyor": true, "doors": true, "window": true, "ferris": true,
	"pan": true, "switch": true,
	// tiles (three.js)
	"glitter": true, "honeycomb": true, "vortex": true, "shred": true, "ripple": true, "fracture": true, "crush": true,
	"curtains": true, "wind": true, "prestige": true, "peelOff": true, "pageCurlDouble": true, "pageCurlSingle": true,
	"airplane": true, "origami": true,
}

// WebGLTransitions are the transitions the player draws with three.js.
var WebGLTransitions = map[string]bool{
	"glitter": true, "honeycomb": true, "vortex": true, "shred": true, "ripple": true, "fracture": true, "crush": true,
	"curtains": true, "wind": true, "prestige": true, "airplane": true, "origami": true,
}

func readTransition(sld *xmltree.Node, out *Slide, rep *Report) {
	t := sld.Child("transition")
	if t == nil {
		return
	}
	out.AdvClick = t.Bool("advClick", true)
	if v := t.Attr("advTm"); v != "" {
		s := num(v) / 1000
		out.Advance = &s
	}
	dur := 0.0
	switch t.Attr("spd") {
	case "slow":
		dur = 1000
	case "med":
		dur = 750
	default:
		dur = 500
	}
	for _, a := range t.Attrs {
		if a.Name.Local == "dur" { // p14:dur
			dur = num(a.Value)
		}
	}
	for _, k := range t.Kids {
		if k.Name == "sndAc" || k.Name == "extLst" {
			continue
		}
		name := k.Name
		if name == "prstTrans" && k.Attr("prst") != "" {
			name = k.Attr("prst")
		}
		tr := &Transition{Type: name, Dur: dur, Dir: k.Attr("dir")}
		if k.Attr("invX") == "1" || k.Attr("isInverted") == "1" {
			tr.Dir += "-inv"
		}
		if k.Name == "fade" && k.Bool("thruBlk", false) {
			tr.Orig = "fadeThroughBlack"
		}
		if !knownTransitions[name] {
			tr.Orig, tr.Type = name, "fade"
			rep.Fallback = appendOnce(rep.Fallback, name)
		}
		if k.Name == "cut" {
			tr.Dur = 0
		}
		out.Transition = tr
		break
	}
}

// FirstStep returns the first step of the click order in which target (a shape id, or
// "<spid>:p<n>" for a paragraph) shows: 0 when it is there as the slide appears, -1 when only a
// trigger shows it or nothing does.
func (s Slide) FirstStep(target string) int {
	hidden := false
	for _, h := range s.Hidden {
		if h == target {
			hidden = true
			break
		}
	}
	if !hidden {
		return 0
	}
	for n, st := range s.Steps {
		for _, e := range st.Effects {
			if e.Target == target && e.Class == "entr" {
				return n
			}
		}
	}
	return -1
}
