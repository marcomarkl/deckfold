package main

import (
	"regexp"
	"runtime/debug"
	"strconv"
)

// buildVersion names a build without a version from -ldflags by its place in Git, from the
// information go build embeds: a build of a tagged commit is that tag ("0.3.0"); any other build
// is the last tag and the commit ("0.3.0+30a21a3"); "-dirty" marks local changes. Without Git
// information (go run, -buildvcs=false) it is "dev".
func buildVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	var rev string
	var modified bool
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	return versionFrom(bi.Main.Version, rev, modified)
}

var (
	tagVersion = regexp.MustCompile(`^v(\d+\.\d+\.\d+)(\+dirty)?$`)
	// a pseudo-version after the tag vX.Y.Z is vX.Y.(Z+1)-0.<time>-<commit>
	afterTag = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)-0\.\d{14}-[0-9a-f]{12}(\+dirty)?$`)
)

func versionFrom(mainVersion, rev string, modified bool) string {
	dirty := ""
	if modified {
		dirty = "-dirty"
	}
	if m := tagVersion.FindStringSubmatch(mainVersion); m != nil {
		return m[1] + dirty
	}
	short := rev
	if len(short) > 7 {
		short = short[:7]
	}
	if short == "" {
		return "dev"
	}
	if m := afterTag.FindStringSubmatch(mainVersion); m != nil {
		if patch, err := strconv.Atoi(m[3]); err == nil && patch > 0 {
			return m[1] + "." + m[2] + "." + strconv.Itoa(patch-1) + "+" + short + dirty
		}
	}
	return "dev+" + short + dirty
}
