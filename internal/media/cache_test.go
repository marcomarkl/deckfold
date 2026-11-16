package media

import (
	"bytes"
	"compress/zlib"
	"image"
	"image/gif"
	"os"
	"path/filepath"
	"testing"
)

// with returns an Optimizer that keeps its encoder output in dir.
func with(dir string) *Optimizer {
	o := New()
	o.Cache = NewCache(dir)
	return o
}

// entries lists the files of a cache folder.
func entries(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err == nil && fi.Mode().IsRegular() {
			out = append(out, p)
		}
		return nil
	})
	return out
}

// TestCacheSameBytes: without a cache, with an empty one and with a filled one the result is the
// same, byte for byte, for a still picture, one with a colour profile and an animation.
func TestCacheSameBytes(t *testing.T) {
	plain := pngOf(t, picture(160, 120))
	profile := bytes.Repeat([]byte("deckfold test profile "), 20)
	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	zw.Write(profile)
	zw.Close()
	inputs := map[string][]byte{
		"picture":              plain,
		"picture with profile": withChunk(plain, "iCCP", append([]byte("test\x00\x00"), z.Bytes()...)),
		"animation":            gifOf(t, []int{10, 20, 30, 5}, 0, gif.DisposalNone),
	}
	for name, data := range inputs {
		dir := t.TempDir()
		none := New().Optimize(data)
		cold := with(dir)
		first := cold.Optimize(data)
		warm := with(dir)
		second := warm.Optimize(data)
		if !none.Changed || !bytes.Equal(none.Data, first.Data) || !bytes.Equal(none.Data, second.Data) {
			t.Errorf("%s: results differ: no cache %d bytes, empty cache %d, filled cache %d", name, len(none.Data), len(first.Data), len(second.Data))
		}
		if second.Mime != none.Mime || second.Ext != none.Ext || second.Animated != none.Animated {
			t.Errorf("%s: a result from the cache says %+v, a fresh one %+v", name, second, none)
		}
		if cold.Stats().Cached != 0 || warm.Stats().Cached != 1 {
			t.Errorf("%s: saved encoder runs: %d with the empty cache, %d with the filled one; want 0 and 1", name, cold.Stats().Cached, warm.Stats().Cached)
		}
		if n := len(entries(t, dir)); n != 1 {
			t.Errorf("%s: %d files in the cache, want 1", name, n)
		}
	}
	// The picture with and without the profile has the same samples, so one entry serves both,
	// and the profile comes from the source each time.
	dir := t.TempDir()
	with(dir).Optimize(inputs["picture"])
	o := with(dir)
	r := o.Optimize(inputs["picture with profile"])
	chunks, err := parseRIFF(r.Data)
	if err != nil || len(chunks) != 3 || chunks[1].id != "ICCP" || !bytes.Equal(chunks[1].data, profile) || o.Stats().Cached != 1 {
		t.Errorf("the profile of the source is not in a result from the cache: chunks %v, %v, saved runs %d", chunks, err, o.Stats().Cached)
	}
}

// TestCacheDistrust: an entry that is damaged, holds another picture, carries chunks the encoder
// does not write or is too large is not used. The result is the one without a cache, and a
// usable entry takes its place.
func TestCacheDistrust(t *testing.T) {
	src := picture(160, 120)
	data := pngOf(t, src)
	want := New().Optimize(data)

	other := image.NewNRGBA(src.Bounds())
	copy(other.Pix, src.Pix)
	other.Pix[len(other.Pix)-4] ^= 0x40 // one sample of the last pixel
	otherRaw := New().Optimize(pngOf(t, other)).Data
	if !plainStill(otherRaw) {
		t.Fatal("the other picture is not a plain WebP file")
	}
	withProfile, err := withMetadata(want.Data, []byte("a profile that is not the source's"), nil)
	if err != nil {
		t.Fatal(err)
	}
	bad := map[string][]byte{
		"damaged":            append(append([]byte(nil), want.Data[:len(want.Data)/2]...), make([]byte, len(want.Data)/2)...),
		"not a WebP file":    []byte("<script>alert(1)</script>"),
		"another picture":    otherRaw,
		"a foreign profile":  withProfile,
		"larger than pixels": make([]byte, 160*120*4+4097),
	}
	for name, content := range bad {
		dir := t.TempDir()
		with(dir).Optimize(data)
		files := entries(t, dir)
		if len(files) != 1 {
			t.Fatalf("%s: %d files in the cache, want 1", name, len(files))
		}
		good, _ := os.ReadFile(files[0])
		if err := os.WriteFile(files[0], content, 0o644); err != nil {
			t.Fatal(err)
		}
		o := with(dir)
		r := o.Optimize(data)
		if !bytes.Equal(r.Data, want.Data) {
			t.Errorf("%s: the result is not the one without a cache", name)
		}
		if o.Stats().Cached != 0 {
			t.Errorf("%s: the entry counted as used", name)
		}
		if now, _ := os.ReadFile(files[0]); !bytes.Equal(now, good) {
			t.Errorf("%s: the entry was not replaced by a usable one", name)
		}
	}

	// the same for an animation whose entry is the animation of another GIF
	anim := gifOf(t, []int{10, 20, 30, 5}, 0, gif.DisposalNone)
	wantAnim := New().Optimize(anim)
	slower := New().Optimize(gifOf(t, []int{10, 20, 30, 6}, 0, gif.DisposalNone)).Data
	dir := t.TempDir()
	with(dir).Optimize(anim)
	files := entries(t, dir)
	if len(files) != 1 || !plainAnim(slower) {
		t.Fatalf("animation: %d files in the cache, other animation plain: %v", len(files), plainAnim(slower))
	}
	os.WriteFile(files[0], slower, 0o644)
	o := with(dir)
	if r := o.Optimize(anim); !bytes.Equal(r.Data, wantAnim.Data) || o.Stats().Cached != 0 {
		t.Errorf("animation: an entry with other frame durations was used (saved runs %d)", o.Stats().Cached)
	}
}

// TestCacheKey: an entry is asked for under another name when the encoder or its settings change,
// and the parts of a key cannot run into each other.
func TestCacheKey(t *testing.T) {
	pix := []byte{1, 2, 3, 4}
	base := cacheKey("enc@v1 h1:a", encoderSettings, "still", pix)
	for name, k := range map[string][32]byte{
		"another encoder version": cacheKey("enc@v2 h1:b", encoderSettings, "still", pix),
		"other settings":          cacheKey("enc@v1 h1:a", "lossless quality=100 method=4 exact", "still", pix),
		"another kind":            cacheKey("enc@v1 h1:a", encoderSettings, "animation", pix),
		"other samples":           cacheKey("enc@v1 h1:a", encoderSettings, "still", []byte{1, 2, 3, 5}),
		"the parts split up":      cacheKey("enc@v1 h1:a", encoderSettings, "still", pix[:2], pix[2:]),
	} {
		if k == base {
			t.Errorf("%s: the key is the same", name)
		}
	}
	if id := encoderID(); id == "" || id == "github.com/gen2brain/webp@unknown" {
		t.Errorf("the encoder is not named from the module data of the binary: %q", id)
	}
	a, b := NewCache(t.TempDir()), NewCache(t.TempDir())
	a.encoder = "another"
	img := picture(8, 8)
	if a.stillKey(img) == b.stillKey(img) {
		t.Error("two encoders ask for the same entry")
	}
}

// TestCacheUnwritable: when the folder cannot be made, the conversion gives the same result and
// the cache says why it kept nothing.
func TestCacheUnwritable(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-folder")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	data := pngOf(t, picture(160, 120))
	o := with(filepath.Join(file, cacheDirForTest))
	if r := o.Optimize(data); !bytes.Equal(r.Data, New().Optimize(data).Data) {
		t.Error("the result differs when the cache cannot be written")
	}
	if o.Cache.Err() == nil {
		t.Error("the cache does not report that it could not write")
	}
}

const cacheDirForTest = ".deckfold-cache"

// TestCacheStill: bitmaps that deckfold draws itself (from EMF and WMF pictures) use the cache too.
func TestCacheStill(t *testing.T) {
	img := picture(160, 120)
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255
	}
	png := len(pngOf(t, img))
	want, _ := New().Still(img, png)
	dir := t.TempDir()
	first, _ := with(dir).Still(img, png)
	o := with(dir)
	second, _ := o.Still(img, png)
	if want == nil || !bytes.Equal(want, first) || !bytes.Equal(want, second) || o.Stats().Cached != 1 {
		t.Errorf("Still: %d, %d and %d bytes, saved runs %d", len(want), len(first), len(second), o.Stats().Cached)
	}
}
