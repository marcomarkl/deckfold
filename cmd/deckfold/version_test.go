package main

import "testing"

func TestVersionFromBuildInfo(t *testing.T) {
	rev := "30a21a3343144b88668ab7c0350b866f180042f7"
	for _, c := range []struct {
		main     string
		rev      string
		modified bool
		want     string
	}{
		{"v0.3.0", rev, false, "0.3.0"},
		{"v0.3.0+dirty", rev, true, "0.3.0-dirty"},
		{"v0.3.1-0.20261107191100-30a21a334314", rev, false, "0.3.0+30a21a3"},
		{"v0.3.1-0.20261107191100-30a21a334314+dirty", rev, true, "0.3.0+30a21a3-dirty"},
		{"v0.0.0-20261107191100-30a21a334314", rev, false, "dev+30a21a3"},
		{"(devel)", rev, true, "dev+30a21a3-dirty"},
		{"(devel)", "", false, "dev"},
	} {
		if got := versionFrom(c.main, c.rev, c.modified); got != c.want {
			t.Errorf("versionFrom(%q, %q, %v) = %q, want %q", c.main, c.rev, c.modified, got, c.want)
		}
	}
}
