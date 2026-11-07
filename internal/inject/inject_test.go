package inject

import (
	"bytes"
	"testing"

	"deckfold/internal/assets"
)

// AssetBlocksSize must be exact: the embed step allocates the output with it once.
func TestAssetBlocksSize(t *testing.T) {
	s := assets.NewStore()
	s.Add(nil, "application/octet-stream", "empty")
	s.Add([]byte("small asset"), "text/plain", "small")
	big := bytes.Repeat([]byte{0, 1, 2, 3, 4, 5, 6}, (PartBytes*2+123)/7)
	s.Add(big, "video/mp4", "big")
	got := AssetBlocks(s)
	if len(got) != AssetBlocksSize(s) {
		t.Fatalf("AssetBlocksSize = %d, blocks are %d bytes", AssetBlocksSize(s), len(got))
	}
	if cap(got) != len(got) {
		t.Errorf("blocks were reallocated: cap %d, len %d", cap(got), len(got))
	}
	if n := bytes.Count(got, []byte("<script ")); n != 1+1+3 {
		t.Errorf("%d script blocks, want 5", n)
	}
}
