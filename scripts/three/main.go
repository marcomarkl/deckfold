// Command three builds runtime/dist/three.js, the part of three.js that the PowerPoint player
// uses for 3D shapes, particle transitions and 3D charts.
//
// Usage (from the repository root): cd scripts/three && go run .
//
// It downloads the pinned npm package into tests/.out/three, checks its integrity, and bundles
// entry.js with esbuild into a minified classic script (IIFE) that sets window.__dfThree and
// contains only ASCII characters. The deck writer embeds the result only in decks that need it.
// This command has its own Go module, so esbuild is not a dependency of deckfold.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

const (
	version   = "0.186.1"
	tarball   = "https://registry.npmjs.org/three/-/three-" + version + ".tgz"
	integrity = "sha512-blFeqb49wRCSGUGj7gtpfnSGHy2lwDk94RhUmS1c/hTby70kvChbWpkJ4Pm1390LqzzvTmzgXKHPEafJwCb8jA=="
)

func main() {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	check(err)
	work := filepath.Join(root, "tests", ".out", "three", version)
	pkg := filepath.Join(work, "node_modules", "three")
	if _, err := os.Stat(filepath.Join(pkg, "package.json")); err != nil {
		check(fetch(pkg))
	}
	license, err := os.ReadFile(filepath.Join(pkg, "LICENSE"))
	check(err)
	copyright := ""
	for _, l := range strings.Split(string(license), "\n") {
		if strings.HasPrefix(l, "Copyright") {
			copyright = strings.TrimSpace(l)
		}
	}
	if copyright == "" {
		check(fmt.Errorf("no copyright line in the three.js license"))
	}
	res := api.Build(api.BuildOptions{
		EntryPoints:      []string{"entry.js"},
		Bundle:           true,
		Format:           api.FormatIIFE,
		GlobalName:       "__dfThree",
		Platform:         api.PlatformBrowser,
		Target:           api.ES2018,
		MinifyWhitespace: true, MinifyIdentifiers: true, MinifySyntax: true,
		Charset:       api.CharsetASCII,
		LegalComments: api.LegalCommentsNone,
		NodePaths:     []string{filepath.Join(work, "node_modules")},
		Banner: map[string]string{"js": "/* three.js " + version + " (parts used by the deckfold PowerPoint player), MIT License, " +
			strings.ReplaceAll(copyright, "©", "(c)") + ". Built by scripts/three. */"},
		Write: false,
	})
	for _, m := range res.Errors {
		fmt.Fprintln(os.Stderr, m.Text)
	}
	if len(res.Errors) > 0 || len(res.OutputFiles) != 1 {
		os.Exit(1)
	}
	out := res.OutputFiles[0].Contents
	for _, c := range out {
		if c > 127 {
			check(fmt.Errorf("output contains non-ASCII bytes"))
		}
	}
	dst := filepath.Join(root, "runtime", "dist", "three.js")
	check(os.WriteFile(dst, out, 0o644))
	fmt.Printf("runtime/dist/three.js: %d bytes (three.js %s)\n", len(out), version)
}

// fetch downloads the npm tarball, checks it against the integrity value of the registry and
// unpacks it.
func fetch(dir string) error {
	resp, err := http.Get(tarball)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("%s: %s", tarball, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	sum := sha512.Sum512(data)
	if got := "sha512-" + base64.StdEncoding.EncodeToString(sum[:]); got != integrity {
		return fmt.Errorf("three %s: integrity %s, want %s", version, got, integrity)
	}
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(strings.TrimPrefix(h.Name, "package/"))
		if h.Typeflag != tar.TypeReg || strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			continue
		}
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		f, err := os.Create(p)
		if err != nil {
			return err
		}
		if _, err := io.CopyN(f, tr, h.Size); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "three:", err)
		os.Exit(1)
	}
}
