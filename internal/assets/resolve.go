package assets

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// Resolver resolves local references inside a root folder.
type Resolver struct {
	Root         string   // root folder of the input (absolute, symlinks resolved)
	AllowOutside []string // additional allowed folders (--allow-outside)
}

// ErrOutside reports a path outside the root folder.
var ErrOutside = errors.New("is outside the input folder (allow it with --allow-outside)")

// ErrNotFound reports a missing file.
var ErrNotFound = errors.New("file not found")

// NewResolver creates a resolver.
func NewResolver(root string, allow []string) (*Resolver, error) {
	r, err := realAbs(root)
	if err != nil {
		return nil, err
	}
	res := &Resolver{Root: r}
	for _, a := range allow {
		ra, err := realAbs(a)
		if err != nil {
			return nil, fmt.Errorf("--allow-outside %s: %w", a, err)
		}
		res.AllowOutside = append(res.AllowOutside, ra)
	}
	return res, nil
}

func realAbs(p string) (string, error) {
	a, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(a)
}

// Resolved is a resolved local reference.
type Resolved struct {
	Path  string // absolute real path
	Stage int    // 1 exact, 2 URL decoding, 3 Unicode normalization, 4 case-insensitive
}

// StripRef removes fragment and query from a reference.
func StripRef(ref string) (string, string) {
	suffix := ""
	if i := strings.IndexAny(ref, "?#"); i >= 0 {
		suffix = ref[i:]
		ref = ref[:i]
	}
	return ref, suffix
}

// IsLocalRef reports whether a reference points to a local file (no scheme, no protocol-relative host).
func IsLocalRef(ref string) bool {
	r := strings.TrimSpace(ref)
	if r == "" || strings.HasPrefix(r, "#") || strings.HasPrefix(r, "//") {
		return false
	}
	if u, err := url.Parse(r); err == nil && u.Scheme != "" {
		// Windows drive letters such as C:\ count as local absolute paths
		if len(u.Scheme) == 1 {
			return true
		}
		return false
	}
	return true
}

// Resolve resolves ref relative to baseDir. Stages: exact, URL decoding, NFC/NFD, case-insensitive.
func (r *Resolver) Resolve(ref, baseDir string) (Resolved, error) {
	p, _ := StripRef(strings.TrimSpace(ref))
	if strings.HasPrefix(strings.ToLower(p), "file:") {
		return Resolved{}, ErrOutside
	}
	if filepath.IsAbs(p) || strings.HasPrefix(p, "/") || (len(p) > 1 && p[1] == ':') {
		// Absolute paths are only allowed through --allow-outside
		cand := filepath.Clean(filepath.FromSlash(p))
		if rp, ok := r.tryStages(cand); ok {
			if r.allowed(rp.Path, true) {
				return rp, nil
			}
		}
		return Resolved{}, ErrOutside
	}
	cand := filepath.Join(baseDir, filepath.FromSlash(p))
	res, ok := r.tryStages(cand)
	if !ok {
		// Stage 2: URL decoding
		if dec, err := url.PathUnescape(p); err == nil && dec != p {
			if res2, ok2 := r.tryStages(filepath.Join(baseDir, filepath.FromSlash(dec))); ok2 {
				res, ok = res2, true
				if res.Stage < 2 {
					res.Stage = 2
				}
			}
		}
	}
	if !ok {
		return Resolved{}, ErrNotFound
	}
	if !r.allowed(res.Path, false) {
		return Resolved{}, ErrOutside
	}
	return res, nil
}

func (r *Resolver) allowed(p string, absolute bool) bool {
	if !absolute && within(p, r.Root) {
		return true
	}
	for _, a := range r.AllowOutside {
		if within(p, a) {
			return true
		}
	}
	return false
}

func within(p, dir string) bool {
	rel, err := filepath.Rel(dir, p)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// tryStages checks the exact path, Unicode-normalized variants and a case-insensitive match.
func (r *Resolver) tryStages(p string) (Resolved, bool) {
	if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
		if rp, err := filepath.EvalSymlinks(p); err == nil {
			// Exact also means the spelling matches on case-insensitive file systems.
			if exactCase(p) {
				return Resolved{Path: rp, Stage: 1}, true
			}
			// Case-insensitive file system: use the spelling on disk, so the same file
			// referenced with different spellings is recognized as one file.
			if q, ok := caseInsensitive(p); ok {
				if cq, err := filepath.EvalSymlinks(q); err == nil {
					return Resolved{Path: cq, Stage: 4}, true
				}
			}
			return Resolved{Path: rp, Stage: 4}, true
		}
	}
	for _, f := range []norm.Form{norm.NFC, norm.NFD} {
		q := f.String(p)
		if q == p {
			continue
		}
		if fi, err := os.Stat(q); err == nil && !fi.IsDir() {
			if rp, err := filepath.EvalSymlinks(q); err == nil {
				return Resolved{Path: rp, Stage: 3}, true
			}
		}
	}
	if q, ok := caseInsensitive(p); ok {
		if rp, err := filepath.EvalSymlinks(q); err == nil {
			return Resolved{Path: rp, Stage: 4}, true
		}
	}
	return Resolved{}, false
}

// exactCase checks whether the last path component is spelled exactly as in its directory.
func exactCase(p string) bool {
	dir, base := filepath.Split(filepath.Clean(p))
	if base == "" || dir == "" {
		return true
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return true
	}
	for _, e := range ents {
		if e.Name() == base || norm.NFC.String(e.Name()) == norm.NFC.String(base) {
			return true
		}
	}
	return false
}

// caseInsensitive walks the path component by component, ignoring case and Unicode form.
func caseInsensitive(p string) (string, bool) {
	p = filepath.Clean(p)
	vol := filepath.VolumeName(p)
	rest := strings.TrimPrefix(p[len(vol):], string(filepath.Separator))
	cur := vol + string(filepath.Separator)
	if !filepath.IsAbs(p) {
		cur = "."
		rest = p
	}
	for _, part := range strings.Split(rest, string(filepath.Separator)) {
		if part == "" {
			continue
		}
		if part == "." || part == ".." {
			cur = filepath.Join(cur, part)
			continue
		}
		ents, err := os.ReadDir(cur)
		if err != nil {
			return "", false
		}
		found := ""
		want := strings.ToLower(norm.NFC.String(part))
		for _, e := range ents {
			if strings.ToLower(norm.NFC.String(e.Name())) == want {
				found = e.Name()
				break
			}
		}
		if found == "" {
			return "", false
		}
		cur = filepath.Join(cur, found)
	}
	if fi, err := os.Stat(cur); err == nil && !fi.IsDir() {
		return cur, true
	}
	return "", false
}
