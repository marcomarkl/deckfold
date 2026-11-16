package media

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"image"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
)

// encoderSettings names how the WebP encoder is called (the gwebp.Options in image.go). It is
// part of every cache key: change it together with those options.
const encoderSettings = "lossless quality=100 method=6 exact"

// Cache keeps what the WebP encoder wrote, between runs. Encoding at the slowest setting takes
// seconds for one picture; everything around it is fast. So the cache holds the bare output of
// the encoder, under a key made of the encoder, its settings and the samples that went in, and
// a result from the cache goes the same way as a fresh one: deckfold adds the colour profile and
// the EXIF data of the source itself, reads the file back with the other decoder and compares
// every sample. An entry that is damaged, belongs to another picture or carries chunks the
// encoder does not write is not used and gets replaced.
//
// What this check shows is that an entry holds the same picture, not that it holds the very
// bytes the encoder would write. An output is byte for byte the one of a run without the cache
// when deckfold wrote the entries itself.
type Cache struct {
	dir     string
	encoder string

	mu   sync.Mutex
	hits int
	err  error // the first failure to write; the cache is off from then on
}

// NewCache returns a cache in dir. The folder is made when the first entry is written.
func NewCache(dir string) *Cache {
	return &Cache{dir: dir, encoder: encoderID()}
}

// Dir returns the folder of the cache.
func (c *Cache) Dir() string {
	if c == nil {
		return ""
	}
	return c.dir
}

// Hits returns how many encoder runs the cache saved.
func (c *Cache) Hits() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hits
}

// Err returns why the cache stopped writing, or nil.
func (c *Cache) Err() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *Cache) hit() {
	c.mu.Lock()
	c.hits++
	c.mu.Unlock()
}

// encoderID names the WebP package this program was built with, from the module data in the
// binary, so that an entry of another version of the encoder is never asked for.
func encoderID() string {
	const mod = "github.com/gen2brain/webp"
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, d := range bi.Deps {
			if d.Path == mod {
				if d.Replace != nil {
					d = d.Replace
				}
				return d.Path + "@" + d.Version + " " + d.Sum
			}
		}
	}
	return mod + "@unknown"
}

// cacheKey is the name of an entry: the encoder, its settings, what kind of picture it is and
// the data that went into the encoder. Every part is written with its length, so that no two
// lists of parts give the same bytes.
func cacheKey(encoder, settings, kind string, parts ...[]byte) [32]byte {
	h := sha256.New()
	put := func(b []byte) {
		var n [8]byte
		binary.LittleEndian.PutUint64(n[:], uint64(len(b)))
		h.Write(n[:])
		h.Write(b)
	}
	put([]byte(encoder))
	put([]byte(settings))
	put([]byte(kind))
	for _, p := range parts {
		put(p)
	}
	var k [32]byte
	h.Sum(k[:0])
	return k
}

func dims(vals ...int) []byte {
	b := make([]byte, 0, 8*len(vals))
	for _, v := range vals {
		b = binary.LittleEndian.AppendUint64(b, uint64(v))
	}
	return b
}

// pixels returns the samples of img row by row, without the padding a sub-image has.
func pixels(img *image.NRGBA) []byte {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if img.Stride == w*4 && len(img.Pix) == w*h*4 {
		return img.Pix
	}
	out := make([]byte, 0, w*h*4)
	for y := 0; y < h; y++ {
		out = append(out, img.Pix[y*img.Stride:y*img.Stride+w*4]...)
	}
	return out
}

func (c *Cache) stillKey(img *image.NRGBA) [32]byte {
	return cacheKey(c.encoder, encoderSettings, "still", dims(img.Bounds().Dx(), img.Bounds().Dy()), pixels(img))
}

func (c *Cache) animKey(frames []animFrame, loop int) [32]byte {
	parts := [][]byte{dims(loop, len(frames))}
	for _, f := range frames {
		parts = append(parts, dims(f.ms, f.img.Bounds().Dx(), f.img.Bounds().Dy()), pixels(f.img))
	}
	return cacheKey(c.encoder, encoderSettings, "animation", parts...)
}

func (c *Cache) path(key [32]byte) string {
	name := hex.EncodeToString(key[:])
	return filepath.Join(c.dir, name[:2], name[2:]+".webp")
}

// load returns the entry for key, or nil. An entry of more than max bytes is not read: no
// encoder output is larger than the samples it was made of.
func (c *Cache) load(key [32]byte, max int) []byte {
	if c == nil {
		return nil
	}
	p := c.path(key)
	fi, err := os.Lstat(p)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() == 0 || fi.Size() > int64(max) {
		return nil
	}
	data, err := os.ReadFile(p)
	if err != nil || len(data) > max {
		return nil
	}
	return data
}

// store writes an entry through a temporary file, so that a second run never reads a half
// written one. A failure switches the cache off for the rest of the run and is kept for Err.
func (c *Cache) store(key [32]byte, data []byte) {
	if c == nil {
		return
	}
	c.mu.Lock()
	off := c.err != nil
	c.mu.Unlock()
	if off {
		return
	}
	p := c.path(key)
	err := os.MkdirAll(filepath.Dir(p), 0o755)
	if err == nil {
		var tmp *os.File
		if tmp, err = os.CreateTemp(filepath.Dir(p), ".tmp-*"); err == nil {
			_, err = tmp.Write(data)
			if cerr := tmp.Close(); err == nil {
				err = cerr
			}
			if err == nil {
				err = os.Rename(tmp.Name(), p)
			}
			if err != nil {
				os.Remove(tmp.Name())
			}
		}
	}
	if err != nil {
		c.mu.Lock()
		if c.err == nil {
			c.err = err
		}
		c.mu.Unlock()
	}
}

// plainStill tells whether b is a WebP file as the encoder writes a still picture: the picture
// data and nothing else. A colour profile or EXIF data in an entry would not come from deckfold.
func plainStill(b []byte) bool {
	chunks, err := parseRIFF(b)
	return err == nil && len(chunks) == 1 && chunks[0].id == "VP8L"
}

// plainAnim tells whether b is a WebP file as the encoder writes an animation: the header, the
// animation settings and the frames.
func plainAnim(b []byte) bool {
	chunks, err := parseRIFF(b)
	if err != nil || len(chunks) < 3 || chunks[0].id != "VP8X" || chunks[1].id != "ANIM" {
		return false
	}
	for _, c := range chunks[2:] {
		if c.id != "ANMF" {
			return false
		}
	}
	return true
}
