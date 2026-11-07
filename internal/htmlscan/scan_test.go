package htmlscan

import "testing"

func TestScanOffsets(t *testing.T) {
	src := []byte(`<!doctype html><html><head><meta charset=utf-8><script type="application/json" id="x">{"a":1}</script>` +
		`<style media='print'>a{color:red}</style></head><body><img src="a b.png" alt=x data-y><p>t</p></body></html>`)
	d := Scan(src)
	var img, script, style *Tag
	for _, tg := range d.Tags {
		switch tg.Name {
		case "img":
			img = tg
		case "script":
			script = tg
		case "style":
			style = tg
		}
	}
	if img == nil || script == nil || style == nil {
		t.Fatal("tags not found")
	}
	a, _ := img.Get("src")
	if string(src[a.ValStart:a.ValEnd]) != "a b.png" || a.Val != "a b.png" {
		t.Fatalf("wrong src offsets: %q", src[a.ValStart:a.ValEnd])
	}
	if string(src[a.Start:a.End]) != `src="a b.png"` {
		t.Fatalf("wrong attribute range: %q", src[a.Start:a.End])
	}
	if v, ok := img.Get("data-y"); !ok || v.ValStart != -1 {
		t.Fatal("attribute without value detected wrongly")
	}
	if string(d.Content(script)) != `{"a":1}` {
		t.Fatalf("script content: %q", d.Content(script))
	}
	if string(src[script.CloseStart:script.CloseEnd]) != "</script>" {
		t.Fatal("wrong script end")
	}
	if style.Value("media") != "print" || string(d.Content(style)) != "a{color:red}" {
		t.Fatal("wrong style")
	}
	if string(src[d.BodyClose:d.BodyClose+7]) != "</body>" || string(src[d.HeadClose:d.HeadClose+7]) != "</head>" {
		t.Fatal("wrong head/body end")
	}
	if d.DoctypeEnd != len("<!doctype html>") {
		t.Fatal("wrong doctype end")
	}
}

func TestEntitiesDecoded(t *testing.T) {
	d := Scan([]byte(`<a title="x &amp; y" href=a&amp;b>`))
	if d.Tags[0].Value("title") != "x & y" || d.Tags[0].Value("href") != "a&b" {
		t.Fatalf("%q %q", d.Tags[0].Value("title"), d.Tags[0].Value("href"))
	}
}
