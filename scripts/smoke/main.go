// Command smoke runs a deckfold binary against a small fixture and checks the result.
// It is the minimal test the CI workflow runs on every platform before a release.
//
// Usage: go run ./scripts/smoke -bin <path to deckfold> [-as-user <name>]
//
// -as-user runs the binary through "sudo -u <name>" (Linux only), to show that it works
// without admin rights.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"deckfold/internal/htmlscan"
)

var fixture = []string{"deck.html", "style.css", "app.js", "pixel.png"}

func main() {
	bin := flag.String("bin", "", "path to the deckfold binary")
	asUser := flag.String("as-user", "", "run the binary as this user through sudo (Linux only)")
	flag.Parse()
	if *bin == "" {
		fail("missing -bin")
	}
	if *asUser != "" && runtime.GOOS != "linux" {
		fail("-as-user is only supported on Linux")
	}
	binPath, err := filepath.Abs(*bin)
	if err != nil {
		fail("bin: %v", err)
	}

	// Copy the fixture into a fresh directory that a guest user can read and write.
	work, err := os.MkdirTemp("", "deckfold-smoke-")
	if err != nil {
		fail("temp dir: %v", err)
	}
	defer os.RemoveAll(work)
	if err := os.Chmod(work, 0o777); err != nil {
		fail("chmod: %v", err)
	}
	src := fixtureDir()
	for _, name := range fixture {
		data, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			fail("fixture: %v", err)
		}
		if err := os.WriteFile(filepath.Join(work, name), data, 0o644); err != nil {
			fail("fixture: %v", err)
		}
	}

	out, err := run(*asUser, binPath, "--version")
	if err != nil || !strings.HasPrefix(out, "deckfold ") {
		fail("--version: %v\n%s", err, out)
	}
	fmt.Print("version: ", out)

	result := filepath.Join(work, "out.html")
	if out, err := run(*asUser, binPath, "--generic", filepath.Join(work, "deck.html"), "-o", result); err != nil {
		fail("conversion: %v\n%s", err, out)
	}
	data, err := os.ReadFile(result)
	if err != nil {
		fail("output: %v", err)
	}
	check(data)
	fmt.Printf("smoke test passed (%d bytes%s)\n", len(data), userNote(*asUser))
}

// run starts the binary with Enter on stdin and a 60 s limit, so a pause for a key press
// (Windows console detection) can never block the run.
func run(asUser, bin string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	name, full := bin, args
	if asUser != "" {
		name, full = "sudo", append([]string{"-n", "-u", asUser, bin}, args...)
	}
	cmd := exec.CommandContext(ctx, name, full...)
	cmd.Stdin = strings.NewReader("\n")
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	if ctx.Err() != nil {
		return buf.String(), fmt.Errorf("timed out after 60 s")
	}
	return buf.String(), err
}

var extRef = regexp.MustCompile(`(?i)(?:src|href|poster|data|srcset)\s*=\s*["']?\s*(?:https?:)?//`)

// check verifies the converted file with the same scanner deckfold uses.
func check(data []byte) {
	d := htmlscan.Scan(data)
	version, deferred, blocks := false, 0, 0
	for _, t := range d.Tags {
		switch {
		case t.Name == "meta" && t.Value("name") == "deckfold:version":
			version = true
		case t.Name == "script" && t.Value("type") == "text/df-deferred":
			deferred++
		case t.Name == "script" && t.Value("type") == "application/octet-stream":
			blocks++
		}
		if t.Name != "a" && extRef.Match(data[t.Start:t.End]) {
			fail("external reference in the output: %s", data[t.Start:t.End])
		}
	}
	if !version {
		fail(`meta name="deckfold:version" missing`)
	}
	if deferred != 1 {
		fail("expected 1 deferred script after removing the duplicate, found %d", deferred)
	}
	if blocks < 1 {
		fail("no asset blocks in the output")
	}
}

// fixtureDir is the folder of this program's source (scripts/smoke).
func fixtureDir() string {
	if _, err := os.Stat(filepath.Join("scripts", "smoke", "deck.html")); err == nil {
		return filepath.Join("scripts", "smoke")
	}
	if _, err := os.Stat("deck.html"); err == nil {
		return "."
	}
	fail("run this from the repository root (scripts/smoke/deck.html not found)")
	return ""
}

func userNote(u string) string {
	if u == "" {
		return ""
	}
	return ", run as user " + u
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "smoke test failed: "+format+"\n", a...)
	os.Exit(1)
}
