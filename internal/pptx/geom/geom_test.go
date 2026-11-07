package geom

import (
	"strings"
	"testing"
)

// Every preset shape of ECMA-376 evaluates without an error and produces a path.
func TestAllPresets(t *testing.T) {
	if n := Count(); n < 180 {
		t.Fatalf("only %d preset shapes loaded", n)
	}
	for name := range presets {
		d, _ := Preset(name)
		res, err := d.Eval(10, 20, 200, 100, nil)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if res.Fill == "" && res.Stroke == "" {
			t.Errorf("%s: empty path", name)
		}
		if strings.Contains(res.Fill+res.Stroke, "NaN") {
			t.Errorf("%s: NaN in path", name)
		}
	}
}

func TestEllipseAndRoundRect(t *testing.T) {
	d, _ := Preset("ellipse")
	res, err := d.Eval(0, 0, 200, 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	// starts at the left middle and draws arcs with radii 100 x 50
	if !strings.HasPrefix(res.Fill, "M0 50") || !strings.Contains(res.Fill, "A100 50") {
		t.Errorf("ellipse path = %q", res.Fill)
	}
	// text rectangle of an ellipse is inset (inscribed rectangle)
	if res.TL < 29 || res.TL > 29.6 {
		t.Errorf("ellipse text inset = %v, want 100*(1-cos 45deg) = 29.3", res.TL)
	}
	d, _ = Preset("roundRect")
	res, _ = d.Eval(0, 0, 200, 100, nil)
	// default radius: 16667 / 100000 of the short side
	if !strings.HasPrefix(res.Fill, "M0 16.67") {
		t.Errorf("roundRect path = %q", res.Fill)
	}
}
