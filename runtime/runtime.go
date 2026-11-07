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

// PptxJS is the player of decks generated from PowerPoint files.
//
//go:embed dist/pptx-player.js
var PptxJS string

// ThreeJS is the part of three.js that the PowerPoint player uses for 3D (built by scripts/three).
// The deck writer embeds it only in decks that need it.
//
//go:embed dist/three.js
var ThreeJS string
