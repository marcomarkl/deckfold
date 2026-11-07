// Package generic is the fallback for unknown formats (--generic, E13): embedding and clean-up,
// plus full screen, screen choice and passed-through keys, but no slide model.
package generic

import "deckfold/internal/adapter"

// Adapter for arbitrary HTML.
type Adapter struct{}

// Name returns the format name.
func (Adapter) Name() string { return "generic" }

// Detect always matches; the adapter is only chosen with --generic.
func (Adapter) Detect(*adapter.Input) bool { return true }

// Collect: no computed paths are known.
func (Adapter) Collect(*adapter.Input) ([]adapter.ExtraAsset, error) { return nil, nil }

// SkipStaticRef: nothing to skip.
func (Adapter) SkipStaticRef(string) bool { return false }

// Meta: no slide model.
func (Adapter) Meta(in *adapter.Input) (adapter.Meta, []string, error) {
	return adapter.Meta{Format: "generic", Title: adapter.Title(in.Doc)}, nil, nil
}
