// Package config holds the defaults for presenter view, auto mode, key bindings and
// screen choice, and loads and validates the JSON configuration.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Presenter describes the presenter view (E4, E5).
type Presenter struct {
	Elements      []string `json:"elements"`
	NotesFontSize int      `json:"notesFontSize"`
	TargetMinutes int      `json:"targetMinutes"`
}

// Auto describes auto mode (E7).
type Auto struct {
	Enabled         bool    `json:"enabled"`
	IntervalSeconds float64 `json:"intervalSeconds"`
	Loop            bool    `json:"loop"`
}

// Screens describes the screen choice.
type Screens struct {
	RememberChoice bool `json:"rememberChoice"`
}

// Config is the full configuration.
type Config struct {
	Presenter Presenter           `json:"presenter"`
	Auto      Auto                `json:"auto"`
	Keys      map[string][]string `json:"keys"`
	Screens   Screens             `json:"screens"`
	WarnSize  string              `json:"warnSize"`
}

// Elements are the allowed presenter view elements.
var Elements = []string{"current", "next", "notes", "time", "overview"}

// Actions are the actions keys can be bound to.
var Actions = []string{"next", "prev", "first", "last", "fullscreen", "black", "white", "auto", "timerReset", "end"}

// Default returns the built-in defaults.
func Default() Config {
	return Config{
		Presenter: Presenter{Elements: append([]string(nil), Elements...), NotesFontSize: 22, TargetMinutes: 0},
		Auto:      Auto{Enabled: false, IntervalSeconds: 8, Loop: false},
		Keys: map[string][]string{
			"next":       {"ArrowRight", "ArrowDown", "PageDown", " ", "Enter"},
			"prev":       {"ArrowLeft", "ArrowUp", "PageUp", "Backspace"},
			"first":      {"Home"},
			"last":       {"End"},
			"fullscreen": {"f", "F", "F5"},
			"black":      {"b", "B", "."},
			"white":      {"w", "W"},
			"auto":       {"a", "A"},
			"timerReset": {"t", "T"},
			"end":        {"Escape"},
		},
		Screens:  Screens{RememberChoice: true},
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
		} `json:"presenter"`
		Auto *struct {
			Enabled         *bool    `json:"enabled"`
			IntervalSeconds *float64 `json:"intervalSeconds"`
			Loop            *bool    `json:"loop"`
		} `json:"auto"`
		Keys    map[string][]string `json:"keys"`
		Screens *struct {
			RememberChoice *bool `json:"rememberChoice"`
		} `json:"screens"`
		WarnSize *string `json:"warnSize"`
		Schema   string  `json:"$schema"`
	}
	if err := dec.Decode(&in); err != nil {
		return base, fmt.Errorf("configuration: %w", err)
	}
	if p := in.Presenter; p != nil {
		if p.Elements != nil {
			c.Presenter.Elements = *p.Elements
		}
		if p.NotesFontSize != nil {
			c.Presenter.NotesFontSize = *p.NotesFontSize
		}
		if p.TargetMinutes != nil {
			c.Presenter.TargetMinutes = *p.TargetMinutes
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
		c.Keys[k] = v
	}
	if s := in.Screens; s != nil && s.RememberChoice != nil {
		c.Screens.RememberChoice = *s.RememberChoice
	}
	if in.WarnSize != nil {
		c.WarnSize = *in.WarnSize
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
	if c.Presenter.TargetMinutes < 0 || c.Presenter.TargetMinutes > 600 {
		return fmt.Errorf("presenter.targetMinutes: %d is not between 0 and 600", c.Presenter.TargetMinutes)
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
	b, _ := json.MarshalIndent(Default(), "", "  ")
	return string(b) + "\n"
}
