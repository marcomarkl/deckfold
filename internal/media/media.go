// Package media converts embedded assets into formats that browsers support widely and that
// are smaller, without changing what is shown (decision E17).
//
// A conversion replaces the original only when the result is smaller and reading the result
// back gives the same picture or the same font tables. Everything else stays as it is.
package media

import (
	"crypto/sha256"
	"errors"
	"image"
	"runtime"
	"sort"
	"sync"

	gwebp "github.com/gen2brain/webp"
)

// Result is what became of one asset.
type Result struct {
	Data     []byte // the content to embed: the converted one, or the original
	Mime     string // MIME type of a converted asset, "" when unchanged
	Ext      string // file extension of a converted asset without the dot, "" when unchanged
	Changed  bool
	Animated bool   // the picture has more than one frame (converted or not)
	Note     string // a warning for the report, "" when there is none
}

// Stats sums up what an Optimizer converted, counting every asset once.
type Stats struct {
	Pictures int
	Fonts    int
	Before   int64 // bytes of the converted assets before
	After    int64 // and after
}

type kind int

const (
	kindOther kind = iota
	kindPNG
	kindGIF
	kindBMP
	kindTIFF
	kindSFNT
	kindWOFF
)

// Limits for data from a presentation: a picture or font beyond them stays unchanged.
const (
	maxPixels     = 120 << 20 // pixels of one picture
	maxAnimBytes  = 512 << 20 // all frames of an animation as RGBA
	maxAnimFrames = 4000
	maxFontBytes  = 64 << 20
	maxICCBytes   = 16 << 20
)

type entry struct {
	once   sync.Once
	res    Result // Data is nil when the asset stays unchanged
	kind   kind
	before int
	used   bool
}

// Optimizer converts assets and remembers every result by the content hash of its source, so
// that a picture used on thirty slides is converted once.
type Optimizer struct {
	mu    sync.Mutex
	memo  map[[32]byte]*entry
	extra Stats // pictures converted through Still
	// webpOff is the reason WebP pictures are not written, nil when they are.
	webpOff error
	// Progress, when set, is called from Prepare after every asset (from several goroutines,
	// one call at a time).
	Progress func(done, total int)
}

// ErrSystemLibrary reports a build that would encode with a libwebp installed on the machine.
var ErrSystemLibrary = errors.New("this build uses the libwebp installed on this machine, so pictures stay unchanged; build with -tags nodynamic")

// New creates an Optimizer.
func New() *Optimizer {
	o := &Optimizer{memo: map[[32]byte]*entry{}}
	// Without the build tag nodynamic the WebP package loads a libwebp from the system when it
	// finds one. Its output would depend on the machine, so pictures then stay as they are.
	if gwebp.Dynamic() == nil {
		o.webpOff = ErrSystemLibrary
	}
	return o
}

// WebPOff returns why pictures are not converted, or nil.
func (o *Optimizer) WebPOff() error { return o.webpOff }

// Optimize returns the content to embed for data. The result for unchanged data carries data
// itself.
func (o *Optimizer) Optimize(data []byte) Result {
	if o == nil {
		return Result{Data: data}
	}
	e := o.get(data)
	o.mu.Lock()
	e.used = true
	o.mu.Unlock()
	r := e.res
	if !r.Changed {
		r.Data = data
	}
	return r
}

// Prepare converts n assets on all cores ahead of time; load returns the content of asset i or
// nil. Optimize then finds the results. The output does not depend on the order.
func (o *Optimizer) Prepare(n int, load func(i int) []byte) {
	if o == nil || n == 0 {
		return
	}
	workers := runtime.NumCPU()
	if workers > n {
		workers = n
	}
	var wg sync.WaitGroup
	var pm sync.Mutex
	next, done := 0, 0
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				pm.Lock()
				i := next
				next++
				pm.Unlock()
				if i >= n {
					return
				}
				if data := load(i); data != nil {
					o.get(data)
				}
				pm.Lock()
				done++
				if o.Progress != nil {
					o.Progress(done, n)
				}
				pm.Unlock()
			}
		}()
	}
	wg.Wait()
}

func (o *Optimizer) get(data []byte) *entry {
	h := sha256.Sum256(data)
	o.mu.Lock()
	e := o.memo[h]
	if e == nil {
		e = &entry{}
		o.memo[h] = e
	}
	o.mu.Unlock()
	e.once.Do(func() {
		e.before = len(data)
		e.kind = sniff(data)
		e.res = o.convert(data, e.kind)
	})
	return e
}

// convert never fails: whatever cannot be read or does not get smaller stays unchanged.
func (o *Optimizer) convert(data []byte, k kind) (res Result) {
	defer func() {
		// a decoder that panics on damaged data must not stop the conversion of the deck
		if recover() != nil {
			res = Result{}
		}
	}()
	switch k {
	case kindPNG, kindGIF, kindBMP, kindTIFF:
		if o.webpOff != nil {
			return Result{Animated: k == kindGIF && gifAnimated(data), Note: o.webpOff.Error()}
		}
	}
	switch k {
	case kindPNG:
		return convertPNG(data)
	case kindGIF:
		return convertGIF(data)
	case kindBMP:
		return convertBMP(data)
	case kindTIFF:
		return convertTIFF(data)
	case kindSFNT, kindWOFF:
		return convertFont(data, k)
	}
	return Result{}
}

// Stats returns the totals of the assets that Optimize handed out converted.
func (o *Optimizer) Stats() Stats {
	var s Stats
	if o == nil {
		return s
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	s = o.extra
	for _, e := range o.memo {
		if !e.used || !e.res.Changed {
			continue
		}
		if e.kind == kindSFNT || e.kind == kindWOFF {
			s.Fonts++
		} else {
			s.Pictures++
		}
		s.Before += int64(e.before)
		s.After += int64(len(e.res.Data))
	}
	return s
}

// Notes returns the warnings of the assets that Optimize handed out, each once, sorted.
func (o *Optimizer) Notes() []string {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	seen := map[string]int{}
	for _, e := range o.memo {
		if e.used && e.res.Note != "" {
			seen[e.res.Note]++
		}
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// sniff tells the kind of an asset from its first bytes, never from a file name.
func sniff(d []byte) kind {
	has := func(at int, magic string) bool {
		return len(d) >= at+len(magic) && string(d[at:at+len(magic)]) == magic
	}
	switch {
	case has(0, "\x89PNG\r\n\x1a\n"):
		return kindPNG
	case has(0, "GIF87a"), has(0, "GIF89a"):
		return kindGIF
	case has(0, "II*\x00"), has(0, "MM\x00*"):
		return kindTIFF
	case has(0, "BM") && len(d) >= 30:
		// the size of the info header that follows the 14-byte file header
		switch le32(d[14:]) {
		case 12, 40, 52, 56, 64, 108, 124:
			return kindBMP
		}
	case has(0, "\x00\x01\x00\x00"), has(0, "OTTO"), has(0, "true"):
		return kindSFNT
	case has(0, "wOFF"):
		return kindWOFF
	}
	return kindOther
}

func le32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func be32(b []byte) uint32 {
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

func be16(b []byte) uint16 { return uint16(b[0])<<8 | uint16(b[1]) }

// Still writes a picture that deckfold made itself (a bitmap from an EMF or WMF picture) as
// lossless WebP when that is smaller than its PNG file of pngSize bytes; otherwise it returns
// nil. It is counted in Stats.
func (o *Optimizer) Still(img *image.NRGBA, pngSize int) ([]byte, string) {
	if o == nil || o.webpOff != nil || img.Bounds().Dx()*img.Bounds().Dy() > maxPixels || halfTransparent(img) {
		return nil, ""
	}
	out := encodeStill(img, nil, nil)
	if out == nil || len(out) >= pngSize {
		return nil, ""
	}
	o.mu.Lock()
	o.extra.Pictures++
	o.extra.Before += int64(pngSize)
	o.extra.After += int64(len(out))
	o.mu.Unlock()
	return out, mimeWebP
}
