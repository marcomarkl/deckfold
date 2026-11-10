package inject

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"deckfold/internal/assets"
	"deckfold/internal/htmlscan"
)

// The asset blocks hold each asset in parts of at most PartBytes, which decode to the data.
func TestAssetBlocks(t *testing.T) {
	s := assets.NewStore()
	s.Add(nil, "application/octet-stream", "empty")
	s.Add([]byte("small asset"), "text/plain", "small")
	big := bytes.Repeat([]byte{0, 1, 2, 3, 4, 5, 6}, (PartBytes*2+123)/7)
	h := s.Add(big, "video/mp4", "big")
	got := AssetBlocks(s)
	if n := bytes.Count(got, []byte("<script ")); n != 1+1+3 {
		t.Errorf("%d script blocks, want 5", n)
	}
	var data []byte
	for i := 0; i < 3; i++ {
		open := []byte(fmt.Sprintf(`id="df-a-%s-%d"`, h, i))
		k := bytes.Index(got, open)
		if k < 0 {
			t.Fatalf("part %d missing", i)
		}
		k += bytes.IndexByte(got[k:], '>') + 1
		e := k + bytes.Index(got[k:], []byte("</script>"))
		part, err := base64.StdEncoding.DecodeString(string(got[k:e]))
		if err != nil || len(part) > PartBytes {
			t.Fatalf("part %d: %d bytes, %v", i, len(part), err)
		}
		data = append(data, part...)
	}
	if !bytes.Equal(data, big) {
		t.Error("the parts do not decode to the data")
	}
}

// Text in the content block is data: markup in it cannot end the script element or open a tag,
// and the block reads back as the same JSON.
func TestContentBlock(t *testing.T) {
	text := `</script><img src=x onerror=alert(1)> & "quoted"`
	in := map[string]any{"version": 1, "slides": []any{map[string]any{"blocks": []any{map[string]any{"role": "text", "text": text, "step": 0}}}}}
	b, err := ContentBlock(in)
	if err != nil {
		t.Fatal(err)
	}
	d := htmlscan.Scan(b)
	if len(d.Tags) != 1 || d.Tags[0].Name != "script" || d.Tags[0].Value("id") != ContentID || d.Tags[0].Value("type") != "application/json" {
		t.Fatalf("the block is not one JSON script element: %d tags", len(d.Tags))
	}
	var out struct {
		Slides []struct {
			Blocks []struct{ Text string }
		}
	}
	if err := json.Unmarshal(d.Content(d.Tags[0]), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.Slides[0].Blocks[0].Text; got != text {
		t.Errorf("text read back as %q", got)
	}
}
