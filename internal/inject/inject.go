// Package inject builds the runtime block (loader, controls, defaults) and the asset blocks,
// and swaps the runtime block of an existing output on --refresh.
package inject

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"deckfold/internal/assets"
	"deckfold/internal/config"
)

// Markers delimit the replaceable runtime block.
const (
	BeginMarker = "<!--deckfold:runtime:begin-->"
	EndMarker   = "<!--deckfold:runtime:end-->"
	// PartBytes is the binary size per asset block (S3: at most 8 MB of Base64 per block).
	PartBytes = 6 << 20
)

// Runtime holds the parts of the control layer.
type Runtime struct {
	JS, CSS string
}

// Defaults are stored as JSON in the runtime block and can be replaced with --refresh.
type Defaults struct {
	Version  string        `json:"version"`
	CSSMode  string        `json:"cssMode"` // "style" or "link" (CSP without inline styles)
	Defaults config.Config `json:"defaults"`
}

// Block builds the runtime block.
func Block(version string, d Defaults, rt Runtime) ([]byte, error) {
	if strings.Contains(strings.ToLower(rt.JS), "</script") {
		return nil, errors.New("runtime JS contains </script")
	}
	if strings.Contains(strings.ToLower(rt.CSS), "</style") {
		return nil, errors.New("runtime CSS contains </style")
	}
	cfg, err := jsonForScript(d)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.WriteString(BeginMarker)
	fmt.Fprintf(&b, `<meta name="deckfold:version" content="%s">`, version)
	b.WriteString(`<script type="application/json" id="df-config">`)
	b.Write(cfg)
	b.WriteString(`</script><style id="df-style">`)
	b.WriteString(rt.CSS)
	b.WriteString(`</style><script id="df-runtime">`)
	b.WriteString(rt.JS)
	b.WriteString(`</script>`)
	b.WriteString(EndMarker)
	return b.Bytes(), nil
}

// jsonForScript encodes JSON so it is safe inside <script>.
func jsonForScript(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(true) // escapes < > & as \u003c etc.
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(b.Bytes()), nil
}

// MetaBlock builds the metadata block (deck, shim table), outside the runtime markers.
func MetaBlock(meta any) ([]byte, error) {
	j, err := jsonForScript(meta)
	if err != nil {
		return nil, err
	}
	return append(append([]byte(`<script type="application/json" id="df-meta">`), j...), []byte(`</script>`)...), nil
}

// AssetBlocks encodes all assets as Base64 blocks (at most PartBytes binary per block).
func AssetBlocks(s *assets.Store) []byte {
	var b bytes.Buffer
	for _, h := range s.Order {
		a := s.Get(h)
		parts := (len(a.Data) + PartBytes - 1) / PartBytes
		if parts == 0 {
			parts = 1
		}
		for i := 0; i < parts; i++ {
			lo, hi := i*PartBytes, (i+1)*PartBytes
			if hi > len(a.Data) {
				hi = len(a.Data)
			}
			if i == 0 {
				fmt.Fprintf(&b, `<script type="application/octet-stream" id="df-a-%s-0" data-mime="%s" data-parts="%d">`, h, htmlAttr(a.Mime), parts)
			} else {
				fmt.Fprintf(&b, `<script type="application/octet-stream" id="df-a-%s-%d">`, h, i)
			}
			b.WriteString(base64.StdEncoding.EncodeToString(a.Data[lo:hi]))
			b.WriteString("</script>\n")
		}
	}
	return b.Bytes()
}

func htmlAttr(s string) string {
	r := strings.NewReplacer("&", "&amp;", `"`, "&quot;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

// ErrNoBlock reports a file without a deckfold runtime block.
var ErrNoBlock = errors.New("not a deckfold output: runtime markers missing")

// Find returns the range of the runtime block.
func Find(src []byte) (int, int, error) {
	i := bytes.Index(src, []byte(BeginMarker))
	j := bytes.Index(src, []byte(EndMarker))
	if i < 0 || j < i {
		return 0, 0, ErrNoBlock
	}
	return i, j + len(EndMarker), nil
}

// ReadDefaults reads the defaults from an existing runtime block.
func ReadDefaults(src []byte) (Defaults, error) {
	var d Defaults
	i, j, err := Find(src)
	if err != nil {
		return d, err
	}
	blk := src[i:j]
	const open = `<script type="application/json" id="df-config">`
	k := bytes.Index(blk, []byte(open))
	if k < 0 {
		return d, errors.New("df-config missing from the runtime block")
	}
	rest := blk[k+len(open):]
	e := bytes.Index(rest, []byte("</script>"))
	if e < 0 {
		return d, errors.New("df-config is not terminated")
	}
	if err := json.Unmarshal(rest[:e], &d); err != nil {
		return d, fmt.Errorf("cannot read df-config: %w", err)
	}
	return d, nil
}

// Replace swaps the runtime block.
func Replace(src, block []byte) ([]byte, error) {
	i, j, err := Find(src)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(src)-(j-i)+len(block))
	out = append(out, src[:i]...)
	out = append(out, block...)
	out = append(out, src[j:]...)
	return out, nil
}

// Markers around the player of a deck generated from PowerPoint (see internal/pptx/deck).
const (
	PlayerBegin = "<!--deckfold:pptx-player:begin-->"
	PlayerEnd   = "<!--deckfold:pptx-player:end-->"
)

// ReplacePlayer swaps the code of the PowerPoint player in an output. It keeps the script
// element as embed rewrote it (deferred type) and only replaces its content. found is false
// for outputs that are not PowerPoint decks.
func ReplacePlayer(src []byte, js string) (out []byte, found bool, err error) {
	i := bytes.LastIndex(src, []byte(PlayerBegin))
	if i < 0 {
		return src, false, nil
	}
	j := bytes.Index(src[i:], []byte(PlayerEnd))
	if j < 0 {
		return nil, true, errors.New("the PowerPoint player block is not terminated")
	}
	blk := src[i : i+j]
	open := bytes.Index(blk, []byte("<script"))
	if open < 0 {
		return nil, true, errors.New("the PowerPoint player block has no script")
	}
	gt := bytes.IndexByte(blk[open:], '>')
	end := bytes.LastIndex(blk, []byte("</script>"))
	if gt < 0 || end < open+gt {
		return nil, true, errors.New("the PowerPoint player script is malformed")
	}
	if strings.Contains(strings.ToLower(js), "</script") {
		return nil, true, errors.New("player code contains </script")
	}
	start := i + open + gt + 1
	stop := i + end
	out = make([]byte, 0, len(src)-(stop-start)+len(js))
	out = append(out, src[:start]...)
	out = append(out, js...)
	out = append(out, src[stop:]...)
	return out, true, nil
}
