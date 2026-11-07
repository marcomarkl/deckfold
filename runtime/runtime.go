// Package dfruntime embeds the control layer (loader, adapters, presenter view, auto mode)
// that goes into every output. Sources are in src/, the checked-in bundle in dist/.
// Rebuild with: go generate ./runtime
package dfruntime

//go:generate go run ../scripts/bundle

import _ "embed"

//go:embed dist/deckfold.js
var JS string

//go:embed dist/deckfold.css
var CSS string
