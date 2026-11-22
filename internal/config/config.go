// Package config holds the defaults for presenter view, auto mode and key bindings, and loads and
// validates the JSON configuration.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Presenter describes the presenter view (E4, E5, E20).
type Presenter struct {
	Elements      []string `json:"elements"`
	NotesFontSize int      `json:"notesFontSize"`
	// TargetSeconds is the target duration of the talk (0 = none), entered as minutes:seconds.
	TargetSeconds int `json:"targetSeconds"`
	// TargetMinutes is the setting of older versions; Upgrade turns it into TargetSeconds.
	TargetMinutes int    `json:"targetMinutes,omitempty"`
	Layout        Layout `json:"layout"`
}

// Layout is the size of the presenter view panes, changed by dragging the dividers: Main is the
// share of the left column ("Live", what the audience sees) in the width, Side the share of
// "Preview" (the next click) in the side column, Strip the width of a tile of the slide overview
// in CSS pixels (MinStrip to MaxStrip; the runtime also keeps the overview within 45 % of the
// window height). Files of v0.4 to v0.9.1 also have "audience", the share of a pane that is gone
// since E20; Merge takes it and drops it.
type Layout struct {
	Main  float64 `json:"main"`
	Side  float64 `json:"side"`
	Strip int     `json:"strip"`
}

// The width of a tile of the slide overview in the presenter view, in CSS pixels.
const (
	MinStrip = 96
	MaxStrip = 480
)

// Auto describes auto mode (E7).
type Auto struct {
	Enabled         bool    `json:"enabled"`
	IntervalSeconds float64 `json:"intervalSeconds"`
	Loop            bool    `json:"loop"`
}

// Config is the full configuration.
type Config struct {
	Presenter Presenter           `json:"presenter"`
	Auto      Auto                `json:"auto"`
	Keys      map[string][]string `json:"keys"`
	WarnSize  string              `json:"warnSize"`
	// KeepAssets embeds pictures and fonts unchanged instead of converting them (E17). It
	// steers the conversion only and is not written into the output, where the rest of the
	// configuration becomes the defaults of the presentation.
	KeepAssets bool `json:"-"`
	// NoCache switches off the folder beside the output that keeps what the picture encoder
	// wrote (configuration key "cache", false). Like KeepAssets it steers the conversion only.
	NoCache bool `json:"-"`
	// DropHidden leaves out the slides hidden in a PowerPoint file (configuration key
	// "dropHidden"), as deckfold did before E21. Like KeepAssets it steers the conversion only.
	DropHidden bool `json:"-"`
}

// Elements are the presenter view panes that can be hidden (E20): what the audience sees, the
// next click, the notes. Time and the slide overview are always shown.
var Elements = []string{"audience", "next", "notes"}

// formerElements are default element lists of earlier versions (the audience pane came with v0.4,
// E20 came after v0.9.1).
var formerElements = [][]string{
	{"current", "next", "notes", "time", "overview"},
	{"current", "audience", "next", "notes", "time", "overview"},
}

// CleanElements turns an element list of an earlier version into one of today: a default list of
// an earlier version (which the file never chose) gives way to the current default; otherwise
// "current" (the pane on the left, now "audience") becomes "audience", "time" and "overview" (now
// always shown) are dropped, and a name named twice counts once. Other names stay, so that
// Validate reports them. runtime/src/30-keys.js does the same for lists stored in the browser.
func CleanElements(els []string) []string {
	if els == nil {
		return nil
	}
	for _, l := range formerElements {
		if sameList(l, els) {
			return append([]string(nil), Elements...)
		}
	}
	out := []string{}
	for _, e := range els {
		switch e {
		case "current":
			e = "audience"
		case "time", "overview":
			continue
		}
		if !in(out, e) {
			out = append(out, e)
		}
	}
	return out
}

// Actions are the actions keys can be bound to.
var Actions = []string{"next", "prev", "first", "last", "fullscreen", "black", "white", "auto", "timerReset", "end", "find"}

// retiredActions were bound to keys by older versions; Merge accepts and drops them, so that
// older configuration files still load (diagnostics showed the screen detection of v0.4 drafts).
var retiredActions = []string{"diagnostics"}

// MaxTargetSeconds is the longest target duration: 600 minutes.
const MaxTargetSeconds = 600 * 60

// Upgrade turns settings of older versions into their current form: targetMinutes becomes
// targetSeconds.
func (p *Presenter) Upgrade() {
	if p.TargetMinutes != 0 && p.TargetSeconds == 0 {
		p.TargetSeconds = p.TargetMinutes * 60
	}
	p.TargetMinutes = 0
}

// Default returns the built-in defaults.
func Default() Config {
	return Config{
		Presenter: Presenter{Elements: append([]string(nil), Elements...), NotesFontSize: 22, Layout: Layout{Main: 0.6, Side: 0.45, Strip: 132}},
		Auto:      Auto{Enabled: false, IntervalSeconds: 8, Loop: false},
		Keys: map[string][]string{
			"next":       {"ArrowRight", "ArrowDown", "PageDown", " "},
			"prev":       {"ArrowLeft", "ArrowUp", "PageUp", "Backspace"},
			"first":      {"Home"},
			"last":       {"End"},
			"fullscreen": {"f", "F", "F5"},
			"black":      {"b", "B", "."},
			"white":      {"w", "W"},
			"auto":       {"a", "A"},
			"timerReset": {"t", "T"},
			"end":        {"Escape"},
			// search and outline; Ctrl+F and Cmd+F open it too and are not configurable
			"find": {"/"},
		},
		WarnSize: "250MB",
	}
}

// Load reads a JSON file and applies it on top of base. Unknown keys are an error.
func Load(path string, base Config) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return base, err
	}
	return Merge(data, base)
}

// Merge applies JSON data on top of base.
func Merge(data []byte, base Config) (Config, error) {
	// Take keys one by one so only what is given overrides.
	c := base
	c.Keys = copyKeys(base.Keys)
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var in struct {
		Presenter *struct {
			Elements      *[]string `json:"elements"`
			NotesFontSize *int      `json:"notesFontSize"`
			TargetMinutes *int      `json:"targetMinutes"`
			TargetSeconds *int      `json:"targetSeconds"`
			Layout        *struct {
				Main     *float64 `json:"main"`
				Side     *float64 `json:"side"`
				Strip    *int     `json:"strip"`
				Audience *float64 `json:"audience"` // v0.4 to v0.9.1; taken and dropped (E20)
			} `json:"layout"`
		} `json:"presenter"`
		Auto *struct {
			Enabled         *bool    `json:"enabled"`
			IntervalSeconds *float64 `json:"intervalSeconds"`
			Loop            *bool    `json:"loop"`
		} `json:"auto"`
		Keys map[string][]string `json:"keys"`
		// screens.rememberChoice belonged to the screen choice of v0.3 and earlier; it is accepted
		// and ignored, so that older configuration files still load.
		Screens *struct {
			RememberChoice *bool `json:"rememberChoice"`
		} `json:"screens"`
		WarnSize   *string `json:"warnSize"`
		KeepAssets *bool   `json:"keepAssets"`
		Cache      *bool   `json:"cache"`
		DropHidden *bool   `json:"dropHidden"`
		Schema     string  `json:"$schema"`
	}
	if err := dec.Decode(&in); err != nil {
		return base, fmt.Errorf("configuration: %w", err)
	}
	if p := in.Presenter; p != nil {
		if p.Elements != nil {
			c.Presenter.Elements = CleanElements(*p.Elements)
		}
		if p.NotesFontSize != nil {
			c.Presenter.NotesFontSize = *p.NotesFontSize
		}
		if p.TargetMinutes != nil && p.TargetSeconds == nil {
			c.Presenter.TargetSeconds = *p.TargetMinutes * 60
		}
		if p.TargetSeconds != nil {
			c.Presenter.TargetSeconds = *p.TargetSeconds
		}
		if l := p.Layout; l != nil {
			if l.Main != nil {
				c.Presenter.Layout.Main = *l.Main
			}
			if l.Side != nil {
				c.Presenter.Layout.Side = *l.Side
			}
			if l.Strip != nil {
				c.Presenter.Layout.Strip = *l.Strip
			}
		}
	}
	if a := in.Auto; a != nil {
		if a.Enabled != nil {
			c.Auto.Enabled = *a.Enabled
		}
		if a.IntervalSeconds != nil {
			c.Auto.IntervalSeconds = *a.IntervalSeconds
		}
		if a.Loop != nil {
			c.Auto.Loop = *a.Loop
		}
	}
	for k, v := range in.Keys {
		if contains(retiredActions, k) {
			continue
		}
		c.Keys[k] = v
	}
	if in.WarnSize != nil {
		c.WarnSize = *in.WarnSize
	}
	if in.KeepAssets != nil {
		c.KeepAssets = *in.KeepAssets
	}
	if in.Cache != nil {
		c.NoCache = !*in.Cache
	}
	if in.DropHidden != nil {
		c.DropHidden = *in.DropHidden
	}
	return c, c.Validate()
}

func copyKeys(m map[string][]string) map[string][]string {
	out := make(map[string][]string, len(m))
	for k, v := range m {
		out[k] = append([]string(nil), v...)
	}
	return out
}

// Validate checks value ranges and names.
func (c Config) Validate() error {
	for _, e := range c.Presenter.Elements {
		if !in(Elements, e) {
			return fmt.Errorf("presenter.elements: unknown element %q (allowed: %s)", e, strings.Join(Elements, ", "))
		}
	}
	if c.Presenter.NotesFontSize < 8 || c.Presenter.NotesFontSize > 96 {
		return fmt.Errorf("presenter.notesFontSize: %d is not between 8 and 96", c.Presenter.NotesFontSize)
	}
	if c.Presenter.TargetSeconds < 0 || c.Presenter.TargetSeconds > MaxTargetSeconds {
		return fmt.Errorf("presenter.targetSeconds: %d is not between 0 and %d", c.Presenter.TargetSeconds, MaxTargetSeconds)
	}
	for _, v := range []float64{c.Presenter.Layout.Main, c.Presenter.Layout.Side} {
		if v < 0.2 || v > 0.8 {
			return fmt.Errorf("presenter.layout: %g is not between 0.2 and 0.8", v)
		}
	}
	if s := c.Presenter.Layout.Strip; s < MinStrip || s > MaxStrip {
		return fmt.Errorf("presenter.layout.strip: %d is not between %d and %d", s, MinStrip, MaxStrip)
	}
	if c.Auto.IntervalSeconds < 1 || c.Auto.IntervalSeconds > 3600 {
		return fmt.Errorf("auto.intervalSeconds: %g is not between 1 and 3600", c.Auto.IntervalSeconds)
	}
	for k := range c.Keys {
		if !in(Actions, k) {
			return fmt.Errorf("keys: unknown action %q (allowed: %s)", k, strings.Join(Actions, ", "))
		}
	}
	if _, err := ParseSize(c.WarnSize); err != nil {
		return fmt.Errorf("warnSize: %w", err)
	}
	return nil
}

// KnownAction reports whether keys can be bound to the action.
func KnownAction(a string) bool { return in(Actions, a) }

// formerKeys are default key lists of earlier versions. A file made then still carries them; on
// --refresh an action whose keys are exactly such a list gets the current default (up to v0.4
// drafts, Enter moved on as well; now Enter only starts the presentation).
var formerKeys = map[string][][]string{
	"next": {{"ArrowRight", "ArrowDown", "PageDown", " ", "Enter"}},
}

// FormerDefaultKeys tells whether keys is a default list an earlier version gave to action.
func FormerDefaultKeys(action string, keys []string) bool {
	for _, l := range formerKeys[action] {
		if sameList(l, keys) {
			return true
		}
	}
	return false
}

func sameList(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func in(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// ParseSize reads sizes such as "250MB", "1.5GB", "800KB" or plain bytes.
func ParseSize(s string) (int, error) {
	t := strings.ToUpper(strings.TrimSpace(s))
	mult := 1.0
	for _, u := range []struct {
		suf string
		m   float64
	}{{"GB", 1 << 30}, {"MB", 1 << 20}, {"KB", 1 << 10}, {"B", 1}} {
		if strings.HasSuffix(t, u.suf) {
			mult = u.m
			t = strings.TrimSpace(strings.TrimSuffix(t, u.suf))
			break
		}
	}
	v, err := strconv.ParseFloat(t, 64)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	return int(v * mult), nil
}

// Example is the sample configuration printed by --print-config.
func Example() string {
	// keepAssets, cache and dropHidden are read from a configuration file but not stored in an
	// output, so Config leaves them out of its JSON form; the sample shows them.
	b, _ := json.MarshalIndent(struct {
		Config
		KeepAssets bool `json:"keepAssets"`
		Cache      bool `json:"cache"`
		DropHidden bool `json:"dropHidden"`
	}{Config: Default(), Cache: true}, "", "  ")
	return string(b) + "\n"
}

func contains(list []string, s string) bool { return in(list, s) }
