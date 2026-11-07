package assets

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"golang.org/x/text/unicode/norm"
)

func write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveStages(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "img", "café é.png"), "x")
	write(t, filepath.Join(root, "img", "Logo.PNG"), "y")
	r, err := NewResolver(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		ref   string
		stage int
	}{
		{"img/café é.png", 1},
		{"img/caf%C3%A9%20%C3%A9.png", 2},
		{norm.NFD.String("img/café é.png"), 3},
		{"img/logo.png", 4},
		{"./img/Logo.PNG?v=2#frag", 1},
	}
	for _, c := range cases {
		res, err := r.Resolve(c.ref, root)
		if err != nil {
			t.Fatalf("%q: %v", c.ref, err)
		}
		// macOS normalizes Unicode in the file system; stage 3 can show up as 1 there.
		if res.Stage != c.stage && !(c.stage == 3 && res.Stage == 1) {
			t.Errorf("%q: stage %d, expected %d", c.ref, res.Stage, c.stage)
		}
	}
	if _, err := r.Resolve("img/missing.png", root); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing file: %v", err)
	}
}

func TestResolveBoundary(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "deck")
	write(t, filepath.Join(root, "index.html"), "x")
	write(t, filepath.Join(base, "secret.txt"), "key")
	r, _ := NewResolver(root, nil)
	for _, ref := range []string{"../secret.txt", filepath.Join(base, "secret.txt"), "file://" + filepath.Join(base, "secret.txt")} {
		if _, err := r.Resolve(ref, root); !errors.Is(err, ErrOutside) {
			t.Errorf("%q: expected ErrOutside, got %v", ref, err)
		}
	}
	if runtime.GOOS != "windows" {
		if err := os.Symlink(filepath.Join(base, "secret.txt"), filepath.Join(root, "link.txt")); err != nil {
			t.Fatal(err)
		}
		if _, err := r.Resolve("link.txt", root); !errors.Is(err, ErrOutside) {
			t.Errorf("symlink pointing outside: %v", err)
		}
	}
	// allowed with --allow-outside
	r2, _ := NewResolver(root, []string{base})
	if _, err := r2.Resolve("../secret.txt", root); err != nil {
		t.Errorf("--allow-outside: %v", err)
	}
}

func TestBlockedIPs(t *testing.T) {
	for _, s := range []string{"127.0.0.1", "10.1.2.3", "192.168.0.1", "172.16.5.5", "169.254.169.254", "::1", "fe80::1", "0.0.0.0", "100.64.1.1", "fd00::1"} {
		if !isBlockedIP(net.ParseIP(s)) {
			t.Errorf("%s should be blocked", s)
		}
	}
	for _, s := range []string{"8.8.8.8", "142.250.1.1", "2a00:1450:4001::1"} {
		if isBlockedIP(net.ParseIP(s)) {
			t.Errorf("%s should not be blocked", s)
		}
	}
}

func TestFetchRefusesPrivate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("x")) }))
	defer srv.Close()
	f, _ := NewFetcher("", 5*time.Second, 1<<20)
	if _, _, err := f.Get(srv.URL + "/a.css"); !errors.Is(err, ErrPrivateAddr) {
		t.Fatalf("request to %s: expected ErrPrivateAddr, got %v", srv.URL, err)
	}
	f.AllowPrivate = true
	f.cache = map[string]fetched{}
	if d, _, err := f.Get(srv.URL + "/a.css"); err != nil || string(d) != "x" {
		t.Fatalf("with AllowPrivate: %v", err)
	}
}

func TestFetchSizeLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(make([]byte, 2048)) }))
	defer srv.Close()
	f, _ := NewFetcher("", 5*time.Second, 1024)
	f.AllowPrivate = true
	if _, _, err := f.Get(srv.URL); err == nil {
		t.Fatal("size limit not enforced")
	}
}

func TestScanCSS(t *testing.T) {
	css := `@import url("a.css") screen; @import 'b.css';
/* url(no.png) */ .x{background:url( c.png ) , URL('d.png#f')} .y{background-image:image-set("e.png" 1x, url(f.png) 2x)}`
	refs := ScanCSS(css)
	var got []string
	for _, r := range refs {
		got = append(got, r.Value)
		if css[r.Start:r.End] == "" {
			t.Errorf("empty range for %s", r.Value)
		}
	}
	want := []string{"a.css", "b.css", "c.png", "d.png#f", "e.png", "f.png"}
	if len(got) != len(want) {
		t.Fatalf("%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%v", got)
		}
	}
	if refs[0].ImportTail != "screen" || ImportWrap("X", "screen") != "@media screen{X}" {
		t.Fatalf("import condition: %q", refs[0].ImportTail)
	}
}

func TestDataURI(t *testing.T) {
	d, m, ok := ParseDataURI("data:image/svg+xml;base64,PHN2Zy8+")
	if !ok || m != "image/svg+xml" || string(d) != "<svg/>" {
		t.Fatal("base64")
	}
	d, _, ok = ParseDataURI("data:,a%20b")
	if !ok || string(d) != "a b" {
		t.Fatal("percent")
	}
}

func TestStoreDedupe(t *testing.T) {
	s := NewStore()
	a := s.Add([]byte("same"), "text/plain", "a")
	b := s.Add([]byte("same"), "text/plain", "b")
	if a != b || s.Len() != 1 || len(s.Get(a).Names) != 2 {
		t.Fatal("deduplication by content hash")
	}
}
