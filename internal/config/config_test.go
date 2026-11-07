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
