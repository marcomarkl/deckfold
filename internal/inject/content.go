package inject

import (
	"encoding/base64"
	"fmt"
	"strconv"

	"deckfold/internal/htmlscan"
)

// Assets reads the assets embedded in a finished deckfold file. It reads the file only: nothing
// here opens another file, whatever name a reference has.
type Assets struct {
	doc    *htmlscan.Doc
	blocks map[string]*htmlscan.Tag // id -> asset block
}

// ReadAssets indexes the asset blocks of a scanned output.
func ReadAssets(d *htmlscan.Doc) *Assets {
	a := &Assets{doc: d, blocks: map[string]*htmlscan.Tag{}}
	for _, t := range d.Tags {
		if t.Name == "script" && t.Value("type") == "application/octet-stream" {
			if id := t.Value("id"); len(id) > 5 && id[:5] == "df-a-" {
				if _, dup := a.blocks[id]; !dup {
					a.blocks[id] = t
				}
			}
		}
	}
	return a
}

// Get returns the asset with the given content hash, decoded from its blocks.
func (a *Assets) Get(hash string) ([]byte, error) {
	first, ok := a.blocks["df-a-"+hash+"-0"]
	if !ok {
		return nil, fmt.Errorf("asset %s is not in the file", hash)
	}
	parts := 1
	if n, err := strconv.Atoi(first.Value("data-parts")); err == nil && n > 1 {
		parts = n
	}
	var out []byte
	for i := 0; i < parts; i++ {
		t, ok := a.blocks["df-a-"+hash+"-"+strconv.Itoa(i)]
		if !ok {
			return nil, fmt.Errorf("asset %s: part %d of %d is missing", hash, i+1, parts)
		}
		p, err := base64.StdEncoding.DecodeString(string(a.doc.Content(t)))
		if err != nil {
			return nil, fmt.Errorf("asset %s: part %d cannot be decoded: %w", hash, i+1, err)
		}
		out = append(out, p...)
	}
	return out, nil
}

// SetContent puts the content block into a scanned output: in place of the block it has, or
// right after the metadata block when it has none. It returns the new file.
func SetContent(d *htmlscan.Doc, block []byte) ([]byte, error) {
	var meta, old *htmlscan.Tag
	for _, t := range d.Tags {
		if t.Name != "script" || t.Value("type") != "application/json" || t.CloseEnd < 0 {
			continue
		}
		switch t.Value("id") {
		case "df-meta":
			if meta == nil {
				meta = t
			}
		case ContentID:
			if old == nil {
				old = t
			}
		}
	}
	src := d.Src
	switch {
	case old != nil:
		out := make([]byte, 0, len(src)+len(block))
		out = append(out, src[:old.Start]...)
		out = append(out, block...)
		return append(out, src[old.CloseEnd:]...), nil
	case meta != nil:
		out := make([]byte, 0, len(src)+len(block)+1)
		out = append(out, src[:meta.CloseEnd]...)
		out = append(out, '\n')
		out = append(out, block...)
		return append(out, src[meta.CloseEnd:]...), nil
	}
	return nil, fmt.Errorf("the file has no deck metadata block")
}

// HasContent tells whether a scanned output holds a content block.
func HasContent(d *htmlscan.Doc) bool {
	for _, t := range d.Tags {
		if t.Name == "script" && t.Value("id") == ContentID && t.Value("type") == "application/json" {
			return true
		}
	}
	return false
}
