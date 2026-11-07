package assets

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

// Asset is embedded content, deduplicated by content hash.
type Asset struct {
	Hash  string
	Mime  string
	Data  []byte
	Size  int      // len(Data), kept after Release
	Names []string // origin(s), for the report
}

// Store holds all assets of one output.
type Store struct {
	byHash map[string]*Asset
	Order  []string
}

// NewStore creates an empty store.
func NewStore() *Store { return &Store{byHash: map[string]*Asset{}} }

// HashOf returns the short hash of some content.
func HashOf(data []byte) string {
	s := sha256.Sum256(data)
	return hex.EncodeToString(s[:])[:20]
}

// Add stores content and returns its hash.
func (s *Store) Add(data []byte, mimeType, name string) string {
	h := HashOf(data)
	if a, ok := s.byHash[h]; ok {
		for _, n := range a.Names {
			if n == name {
				return h
			}
		}
		a.Names = append(a.Names, name)
		return h
	}
	s.byHash[h] = &Asset{Hash: h, Mime: mimeType, Data: data, Size: len(data), Names: []string{name}}
	s.Order = append(s.Order, h)
	return h
}

// Get returns an asset.
func (s *Store) Get(h string) *Asset { return s.byHash[h] }

// Release drops the content of all assets (after they were encoded into the output), so that
// large media is not held twice. Size and names stay for the report.
func (s *Store) Release() {
	for _, a := range s.byHash {
		a.Data = nil
	}
}

// Len returns the number of assets.
func (s *Store) Len() int { return len(s.Order) }

// Placeholder is the token the loader replaces with a blob URL at runtime.
func Placeholder(h string) string { return "deckfold:" + h }

// MimeFor determines the MIME type from file name, server header and content.
func MimeFor(name, given string, data []byte) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".woff2":
		return "font/woff2"
	case ".woff":
		return "font/woff"
	case ".ttf":
		return "font/ttf"
	case ".otf":
		return "font/otf"
	case ".svg":
		return "image/svg+xml"
	case ".js", ".jsonp", ".pdfp":
		return "text/javascript"
	case ".json":
		return "application/json"
	case ".css":
		return "text/css"
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	case ".mp3":
		return "audio/mpeg"
	case ".m4a":
		return "audio/mp4"
	case ".wav":
		return "audio/wav"
	case ".pdf":
		return "application/pdf"
	case ".vtt":
		return "text/vtt"
	}
	if given != "" && given != "application/octet-stream" {
		return given
	}
	if t := mime.TypeByExtension(ext); t != "" {
		if i := strings.Index(t, ";"); i >= 0 {
			t = t[:i]
		}
		return t
	}
	if len(data) > 0 {
		t := http.DetectContentType(data)
		if i := strings.Index(t, ";"); i >= 0 {
			t = t[:i]
		}
		return t
	}
	return "application/octet-stream"
}

// ParseDataURI splits a data: URI. ok is false for malformed input.
func ParseDataURI(u string) (data []byte, mimeType string, ok bool) {
	if !strings.HasPrefix(strings.ToLower(u), "data:") {
		return nil, "", false
	}
	comma := strings.Index(u, ",")
	if comma < 0 {
		return nil, "", false
	}
	meta := u[5:comma]
	payload := u[comma+1:]
	isB64 := false
	parts := strings.Split(meta, ";")
	mimeType = parts[0]
	for _, p := range parts[1:] {
		if strings.EqualFold(p, "base64") {
			isB64 = true
		}
	}
	if mimeType == "" {
		mimeType = "text/plain"
	}
	if isB64 {
		clean := strings.Map(func(r rune) rune {
			if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
				return -1
			}
			return r
		}, payload)
		d, err := base64.StdEncoding.DecodeString(clean)
		if err != nil {
			d, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(clean, "="))
			if err != nil {
				return nil, "", false
			}
		}
		return d, mimeType, true
	}
	dec, err := urlUnescape(payload)
	if err != nil {
		return nil, "", false
	}
	return []byte(dec), mimeType, true
}
