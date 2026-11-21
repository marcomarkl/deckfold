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

// Element lists of v0.9.1 and before (E20): a former default becomes today's default, "current"
// becomes "audience", "time" and "overview" are dropped, and the share of the audience pane is taken
// and dropped; an unknown name still fails.
func TestOldElements(t *testing.T) {
	for in, want := range map[string][]string{
		`["current","audience","next","notes","time","overview"]`: Elements,
		`["current","next","notes","time","overview"]`:            Elements,
		`["current","notes","time"]`:                              {"audience", "notes"},
		`["audience","current","overview"]`:                       {"audience"},
		`["next"]`:                                                {"next"},
		`[]`:                                                      {},
	} {
		c, err := Merge([]byte(`{"presenter":{"elements":`+in+`,"layout":{"main":0.7,"audience":0.4}}}`), Default())
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if !sameList(c.Presenter.Elements, want) {
			t.Errorf("%s: elements %q, want %q", in, c.Presenter.Elements, want)
		}
		if c.Presenter.Layout != (Layout{Main: 0.7, Side: 0.45}) {
			t.Errorf("%s: layout %+v", in, c.Presenter.Layout)
		}
	}
	if _, err := Merge([]byte(`{"presenter":{"elements":["current","slides"]}}`), Default()); err == nil {
		t.Error("an unknown element passed")
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
