// Command windowlog writes a copy of a deckfold file with a log of both windows in it
// (windowlog.js): the window position and the screen the browser reports, full screen, focus,
// keys, clicks and the messages between the windows, on one clock. It is the first measurement for
// a fault in how the windows behave that only the maintainer can produce, such as a drag to
// another screen; the maintainer opens the copy, shows the fault and copies the log with a click.
//
// The script goes right after the runtime block. A browser runs it only where the file allows
// inline scripts without a hash or nonce, so the command stops when the Content-Security-Policy
// of the file would block it. It never writes over its input.
//
//	go run ./scripts/windowlog tests/.out/deck.html tests/.out/deck-log.html
package main

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"deckfold/internal/htmlscan"
	"deckfold/internal/inject"
)

//go:embed windowlog.js
var logScript []byte

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: go run ./scripts/windowlog <deck.html> <copy.html>")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(in, out string) error {
	a, err := filepath.Abs(in)
	if err != nil {
		return err
	}
	b, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	if a == b {
		return errors.New("the copy must not replace the input")
	}
	src, err := os.ReadFile(in)
	if err != nil {
		return err
	}
	res, err := addLog(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, res, 0o644); err != nil {
		return err
	}
	fmt.Printf("window log added: %s\n", out)
	return nil
}

// addLog returns src with the log script right after the runtime block.
func addLog(src []byte) ([]byte, error) {
	if err := allowsInline(src); err != nil {
		return nil, err
	}
	_, end, err := inject.Find(src)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.Write(src[:end])
	buf.WriteString("<script>")
	buf.Write(logScript)
	buf.WriteString("</script>")
	buf.Write(src[end:])
	return buf.Bytes(), nil
}

// allowsInline reports an error when a Content-Security-Policy in a meta element of src would block
// an inline script: script-src (or default-src without it) without 'unsafe-inline', or with a
// hash or nonce, which makes browsers ignore 'unsafe-inline'.
func allowsInline(src []byte) error {
	d := htmlscan.Scan(src)
	for _, t := range d.Tags {
		if t.Name != "meta" || !strings.EqualFold(t.Value("http-equiv"), "content-security-policy") {
			continue
		}
		dirs := map[string][]string{}
		for _, part := range strings.Split(t.Value("content"), ";") {
			f := strings.Fields(part)
			if len(f) > 0 {
				dirs[strings.ToLower(f[0])] = f[1:]
			}
		}
		srcs, ok := dirs["script-src"]
		if !ok {
			srcs, ok = dirs["default-src"]
		}
		if !ok {
			continue
		}
		inline := false
		for _, s := range srcs {
			l := strings.ToLower(s)
			if l == "'unsafe-inline'" {
				inline = true
			}
			if strings.HasPrefix(l, "'sha") || strings.HasPrefix(l, "'nonce-") {
				return fmt.Errorf("the Content-Security-Policy of the file lists %s, so the browser would block the log script", s)
			}
		}
		if !inline {
			return errors.New("the Content-Security-Policy of the file does not allow inline scripts, so the browser would block the log script")
		}
	}
	return nil
}
