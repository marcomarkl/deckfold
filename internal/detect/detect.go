// Package detect loads the input and chooses the matching adapter.
package detect

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"deckfold/internal/adapter"
	"deckfold/internal/adapter/claude"
	"deckfold/internal/adapter/generic"
	"deckfold/internal/adapter/keynote"
	"deckfold/internal/htmlscan"
)

// ErrUnknown reports an unrecognized format (exit code 2).
var ErrUnknown = errors.New("format not recognized: neither a Keynote HTML export nor a claude.ai deck. With --generic the file is embedded anyway, then without presenter view, overview and auto mode")

// Load reads an input: a folder (with index.html) or an HTML file.
func Load(arg string) (*adapter.Input, error) {
	fi, err := os.Stat(arg)
	if err != nil {
		return nil, err
	}
	in := &adapter.Input{Arg: arg}
	if fi.IsDir() {
		in.IsDir = true
		in.Root = arg
		in.HTMLPath = filepath.Join(arg, "index.html")
		if _, err := os.Stat(in.HTMLPath); err != nil {
			return nil, fmt.Errorf("%s contains no index.html", arg)
		}
	} else {
		in.HTMLPath = arg
		in.Root = filepath.Dir(arg)
	}
	in.Root, _ = filepath.Abs(in.Root)
	in.HTMLPath, _ = filepath.Abs(in.HTMLPath)
	in.HTMLDir = filepath.Dir(in.HTMLPath)
	in.Src, err = os.ReadFile(in.HTMLPath)
	if err != nil {
		return nil, err
	}
	in.Doc = htmlscan.Scan(in.Src)
	return in, nil
}

// Choose picks the adapter. With useGeneric, the generic adapter applies when no other matches.
func Choose(in *adapter.Input, useGeneric bool) (adapter.Adapter, error) {
	for _, a := range []adapter.Adapter{keynote.Adapter{}, claude.Adapter{}} {
		if a.Detect(in) {
			return a, nil
		}
	}
	if useGeneric {
		return generic.Adapter{}, nil
	}
	return nil, ErrUnknown
}
