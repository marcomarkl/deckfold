package config

import "testing"

func TestMergeAndValidate(t *testing.T) {
	c, err := Merge([]byte(`{"presenter":{"notesFontSize":30},"auto":{"loop":true},"keys":{"next":["n"]}}`), Default())
	if err != nil {
		t.Fatal(err)
	}
	if c.Presenter.NotesFontSize != 30 || !c.Auto.Loop || c.Auto.IntervalSeconds != 8 || c.Keys["next"][0] != "n" || len(c.Keys["prev"]) == 0 {
		t.Fatalf("%+v", c)
	}
	bad := []string{
		`{"presenter":{"elements":["foo"]}}`,
		`{"presenter":{"notesFontSize":2}}`,
		`{"auto":{"intervalSeconds":0}}`,
		`{"keys":{"jump":["j"]}}`,
		`{"warnSize":"lots"}`,
		`{"presenter":{"layout":{"main":0.95}}}`,
		`{"unknown":1}`,
	}
	for _, b := range bad {
		if _, err := Merge([]byte(b), Default()); err == nil {
			t.Errorf("%s: expected an error", b)
		}
	}
}

func TestParseSize(t *testing.T) {
	for in, want := range map[string]int{"250MB": 250 << 20, "1.5GB": 3 << 29, "800 KB": 800 << 10, "12": 12} {
		if got, err := ParseSize(in); err != nil || got != want {
			t.Errorf("%s: %d %v", in, got, err)
		}
	}
}

func TestDefaultValid(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestLayoutAndOldScreens(t *testing.T) {
	c, err := Merge([]byte(`{"presenter":{"layout":{"side":0.3}},"screens":{"rememberChoice":false},"keys":{"diagnostics":["D"]}}`), Default())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Keys["diagnostics"]; ok {
		t.Error("the retired diagnostics key was kept")
	}
	if c.Presenter.Layout.Side != 0.3 || c.Presenter.Layout.Main != 0.6 {
		t.Fatalf("layout %+v", c.Presenter.Layout)
	}
}

// targetMinutes of older configuration files becomes targetSeconds; targetSeconds wins when both are given.
func TestTargetMinutesBecomeSeconds(t *testing.T) {
	c, err := Merge([]byte(`{"presenter":{"targetMinutes":20}}`), Default())
	if err != nil {
		t.Fatal(err)
	}
	if c.Presenter.TargetSeconds != 1200 || c.Presenter.TargetMinutes != 0 {
		t.Errorf("targetSeconds %d, targetMinutes %d", c.Presenter.TargetSeconds, c.Presenter.TargetMinutes)
	}
	c, err = Merge([]byte(`{"presenter":{"targetMinutes":20,"targetSeconds":95}}`), Default())
	if err != nil || c.Presenter.TargetSeconds != 95 {
		t.Errorf("targetSeconds %d, %v", c.Presenter.TargetSeconds, err)
	}
	if _, err := Merge([]byte(`{"presenter":{"targetSeconds":36001}}`), Default()); err == nil {
		t.Error("a target over 600 minutes was accepted")
	}
	p := Presenter{TargetMinutes: 5}
	p.Upgrade()
	if p.TargetSeconds != 300 || p.TargetMinutes != 0 {
		t.Errorf("upgrade %+v", p)
	}
}
