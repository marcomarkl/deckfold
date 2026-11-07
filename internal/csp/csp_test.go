package csp

import (
	"strings"
	"testing"
)

const claude = "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline' https://fonts.googleapis.com; font-src data: https://fonts.gstatic.com; img-src data: blob:; media-src data: blob:; frame-src about:; child-src about:; worker-src 'none'; connect-src 'none'"

func TestUnsafeInlineGetsNoHash(t *testing.T) {
	out := Rewrite(claude, Options{InlineScripts: [][]byte{[]byte("x()")}, DropHosts: []string{"fonts.googleapis.com", "fonts.gstatic.com"}, FrameOrigins: []string{"https://example.com"}})
	p := Parse(out)
	if strings.Contains(out, "'sha256-") {
		t.Fatalf("a hash would disable 'unsafe-inline': %s", out)
	}
	if !contains(p.Values["script-src"], "'unsafe-inline'") || !contains(p.Values["script-src"], "blob:") {
		t.Fatalf("script-src: %v", p.Values["script-src"])
	}
	if strings.Contains(out, "fonts.g") {
		t.Fatalf("Google hosts not removed: %s", out)
	}
	if !contains(p.Values["font-src"], "blob:") || !contains(p.Values["style-src"], "blob:") {
		t.Fatalf("blob: missing: %s", out)
	}
	if !contains(p.Values["frame-src"], "https://example.com") || contains(p.Values["frame-src"], "*") {
		t.Fatalf("frame-src: %v", p.Values["frame-src"])
	}
	if !contains(p.Values["worker-src"], "blob:") || contains(p.Values["worker-src"], "'none'") {
		t.Fatalf("worker-src: %v", p.Values["worker-src"])
	}
	if !contains(p.Values["connect-src"], "'none'") {
		t.Fatal("connect-src must not be opened")
	}
}

func TestHashCSPGetsHashes(t *testing.T) {
	in := "script-src 'sha256-abc='; img-src 'self'"
	out := Rewrite(in, Options{InlineScripts: [][]byte{[]byte("x()"), []byte("y()")}})
	if strings.Count(out, "'sha256-") != 3 {
		t.Fatalf("hashes missing: %s", out)
	}
	if !strings.Contains(out, ScriptHash([]byte("x()"))) {
		t.Fatal("hash of the inline script missing")
	}
}

func TestDefaultSrcMaterialized(t *testing.T) {
	out := Rewrite("default-src 'self'", Options{})
	p := Parse(out)
	if !contains(p.Values["img-src"], "'self'") || !contains(p.Values["img-src"], "blob:") {
		t.Fatalf("%s", out)
	}
}
