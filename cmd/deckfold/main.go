// Command deckfold turns a presentation (PowerPoint file, Keynote HTML export, claude.ai deck)
// into a single self-contained HTML file with full screen, presenter view, auto mode and screen
// choice.
//
// Exit codes: 0 success, 1 conversion failed (e.g. unresolved assets),
// 2 format not recognized, 3 usage or configuration error.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"deckfold/internal/assets"
	"deckfold/internal/config"
	"deckfold/internal/detect"
	"deckfold/internal/embed"
	"deckfold/internal/inject"
	"deckfold/internal/pptx"
	"deckfold/internal/pptx/opc"
	"deckfold/internal/report"
	dfruntime "deckfold/runtime"
)

// Version is set with -ldflags for releases.
var Version = "0.1.0-dev"

type multi []string

func (m *multi) String() string     { return strings.Join(*m, ",") }
func (m *multi) Set(v string) error { *m = append(*m, v); return nil }

// valueFlags take a value; needed to allow positional arguments before flags.
var valueFlags = map[string]bool{"o": true, "config": true, "warn-size": true, "auto-interval": true,
	"presenter-elements": true, "notes-font-size": true, "target-minutes": true, "allow-outside": true, "proxy": true}

func main() {
	setupConsole()
	code := run(os.Args[1:], os.Stdout, os.Stderr)
	pauseIfOwnConsole()
	os.Exit(code)
}

// reorder moves flags in front of the positional arguments.
func reorder(args []string) []string {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			flags = append(flags, a)
			name := strings.TrimLeft(a, "-")
			if !strings.Contains(name, "=") && valueFlags[name] && i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		pos = append(pos, a)
	}
	return append(flags, pos...)
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("deckfold", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("o", "", "output file (default: <name>.standalone.html next to the input)")
	force := fs.Bool("force", false, "overwrite an existing output file")
	cfgPath := fs.String("config", "", "JSON configuration (example: deckfold --print-config)")
	lenient := fs.Bool("lenient", false, "write the file even if assets cannot be resolved (with a warning)")
	generic := fs.Bool("generic", false, "embed an unknown format anyway (no presenter view, overview or auto mode)")
	warnSize := fs.String("warn-size", "", "warning threshold for the output size, e.g. 250MB")
	auto := fs.Bool("auto", false, "start in auto mode")
	autoInterval := fs.Float64("auto-interval", 0, "auto mode interval in seconds (when the source has no timings)")
	loop := fs.Bool("loop", false, "loop in auto mode")
	elements := fs.String("presenter-elements", "", "presenter view elements: current,next,notes,time,overview")
	notesSize := fs.Int("notes-font-size", 0, "notes font size in px")
	target := fs.Int("target-minutes", -1, "target duration of the talk in minutes (0 = none)")
	refresh := fs.Bool("refresh", false, "update the runtime of an existing deckfold output")
	var allow multi
	fs.Var(&allow, "allow-outside", "allow a folder outside the input folder (repeatable)")
	proxy := fs.String("proxy", "", "proxy for downloads (otherwise HTTPS_PROXY/NO_PROXY)")
	printCfg := fs.Bool("print-config", false, "print a sample configuration")
	showVer := fs.Bool("version", false, "print the version")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "deckfold %s: turn an HTML presentation into one self-contained file\n\n", Version)
		fmt.Fprintf(stderr, "Usage:\n  deckfold <folder|file.html|file.pptx> [options]\n  deckfold --refresh <output.html> [options]\n\nOptions:\n")
		fs.PrintDefaults()
		fmt.Fprintf(stderr, "\nExit codes: 0 success, 1 conversion failed, 2 format not recognized, 3 usage or configuration error\n")
	}
	if err := fs.Parse(reorder(args)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 3
	}
	if *showVer {
		fmt.Fprintln(stdout, "deckfold", Version)
		return 0
	}
	if *printCfg {
		fmt.Fprint(stdout, config.Example())
		return 0
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return 3
	}
	input := fs.Arg(0)
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })

	// Configuration: built-in defaults (or, with --refresh, the defaults stored in the file),
	// then the configuration file, then the flags.
	base := config.Default()
	var refreshSrc []byte
	if *refresh {
		b, err := os.ReadFile(input)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 3
		}
		d, err := inject.ReadDefaults(b)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		refreshSrc = b
		base = mergeKnown(base, d.Defaults)
	}
	cfg := base
	if *cfgPath != "" {
		c, err := config.Load(*cfgPath, base)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 3
		}
		cfg = c
	}
	if set["warn-size"] {
		cfg.WarnSize = *warnSize
	}
	if set["auto"] {
		cfg.Auto.Enabled = *auto
	}
	if set["auto-interval"] {
		cfg.Auto.IntervalSeconds = *autoInterval
	}
	if set["loop"] {
		cfg.Auto.Loop = *loop
	}
	if set["presenter-elements"] {
		cfg.Presenter.Elements = splitList(*elements)
	}
	if set["notes-font-size"] {
		cfg.Presenter.NotesFontSize = *notesSize
	}
	if set["target-minutes"] {
		cfg.Presenter.TargetMinutes = *target
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 3
	}
	warnBytes, _ := config.ParseSize(cfg.WarnSize)
	rt := inject.Runtime{JS: dfruntime.JS, CSS: dfruntime.CSS}

	if *refresh {
		return doRefresh(input, *out, *force, refreshSrc, cfg, rt, stdout, stderr)
	}

	fetcher, err := assets.NewFetcher(*proxy, 20*time.Second, 300<<20)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 3
	}
	rep := report.New()
	loadPath, srcPath, srcIsDir := input, input, false
	if fi, err := os.Stat(input); err == nil {
		srcIsDir = fi.IsDir()
	}
	if !srcIsDir && isPowerPoint(input) {
		// PowerPoint: render the deck into a temporary folder, then embed it like any export.
		tmp, err := os.MkdirTemp("", "deckfold-pptx-")
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		defer os.RemoveAll(tmp)
		res, err := pptx.Convert(input, tmp, pptx.Options{BundledFontsOnly: os.Getenv("DECKFOLD_BUNDLED_FONTS") == "1",
			AllowOutside: allow, Fetcher: fetcher, Lenient: *lenient})
		if err != nil {
			var me *pptx.MissingError
			if errors.As(err, &me) {
				rep.Missing = me.Missing
				rep.PrintMissing(stderr)
				fmt.Fprintln(stderr, "aborted: the file would not be self-contained. With --lenient it is written anyway (with a warning).")
				return 1
			}
			fmt.Fprintln(stderr, "error:", err)
			if errors.Is(err, pptx.ErrUnsupported) {
				return 2
			}
			return 1
		}
		rep.Summary = plural(res.Slides, "slide")
		if res.Hidden > 0 {
			rep.Summary += ", " + plural(res.Hidden, "hidden slide") + " skipped"
		}
		rep.Fonts = res.Fonts
		for _, w := range res.Warnings {
			rep.Warn("%s", w)
		}
		loadPath = tmp
	}
	in, err := detect.Load(loadPath)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 3
	}
	ad, err := detect.Choose(in, *generic)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}
	outPath := *out
	if outPath == "" {
		outPath = defaultOutput(srcPath, srcIsDir)
	}
	if same(outPath, srcPath) || same(outPath, in.HTMLPath) {
		fmt.Fprintln(stderr, "error: the output would overwrite the input; choose another name with -o")
		return 3
	}
	if _, err := os.Stat(outPath); err == nil && !*force {
		fmt.Fprintf(stderr, "error: %s already exists (overwrite with --force)\n", outPath)
		return 3
	}
	var size int64
	err = writeFileStream(outPath, func(w io.Writer) error {
		var err error
		size, _, err = embed.ConvertTo(w, in, ad, embed.Options{Config: cfg, Lenient: *lenient, AllowOutside: allow,
			Fetcher: fetcher, Version: Version, Runtime: rt}, rep)
		return err
	})
	if err != nil {
		switch {
		case errors.Is(err, embed.ErrMissing):
			rep.PrintMissing(stderr)
			fmt.Fprintln(stderr, "aborted: the file would not be self-contained. With --lenient it is written anyway (with a warning).")
			return 1
		case errors.Is(err, embed.ErrAlreadyConverted):
			fmt.Fprintln(stderr, "error:", err)
			return 3
		}
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	rep.Output, rep.OutputSize = outPath, int(size)
	rep.Print(stdout, warnBytes)
	return 0
}

// mergeKnown takes valid defaults from an existing output (missing fields keep the built-in default).
func mergeKnown(def, old config.Config) config.Config {
	c := old
	if len(c.Presenter.Elements) == 0 && c.Presenter.NotesFontSize == 0 {
		return def
	}
	if c.Keys == nil {
		c.Keys = def.Keys
	}
	if c.WarnSize == "" {
		c.WarnSize = def.WarnSize
	}
	if c.Validate() != nil {
		return def
	}
	return c
}

func doRefresh(input, out string, force bool, src []byte, cfg config.Config, rt inject.Runtime, stdout, stderr io.Writer) int {
	block, err := inject.Block(Version, inject.Defaults{Version: Version, CSSMode: cssModeOf(src), Defaults: cfg}, rt)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	res, err := inject.Replace(src, block)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	// decks generated from PowerPoint also get the current player
	res, _, err = inject.ReplacePlayer(res, dfruntime.PptxJS)
	if err == nil {
		res, _, err = inject.ReplaceThree(res, dfruntime.ThreeJS)
	}
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	target := input
	if out != "" {
		target = out
		if _, err := os.Stat(target); err == nil && !force {
			fmt.Fprintf(stderr, "error: %s already exists (overwrite with --force)\n", target)
			return 3
		}
	}
	if err := writeFile(target, res); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	fmt.Fprintf(stdout, "runtime updated to %s: %s (%s)\n", Version, target, report.HumanSize(len(res)))
	return 0
}

func cssModeOf(src []byte) string {
	if d, err := inject.ReadDefaults(src); err == nil && d.CSSMode != "" {
		return d.CSSMode
	}
	return "style"
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func defaultOutput(arg string, isDir bool) string {
	clean := filepath.Clean(arg)
	if isDir {
		return clean + ".standalone.html"
	}
	ext := filepath.Ext(clean)
	return strings.TrimSuffix(clean, ext) + ".standalone.html"
}

func same(a, b string) bool {
	aa, err1 := filepath.Abs(a)
	bb, err2 := filepath.Abs(b)
	if err1 != nil || err2 != nil {
		return false
	}
	if aa == bb {
		return true
	}
	fa, err1 := os.Stat(aa)
	fb, err2 := os.Stat(bb)
	return err1 == nil && err2 == nil && os.SameFile(fa, fb)
}

// writeFile writes atomically through a temporary file in the target folder.
// writeFileStream writes a file atomically: write fills a temporary file next to p (through a
// buffer), which then replaces p. On an error nothing is left behind.
func writeFileStream(p string, write func(w io.Writer) error) error {
	dir := filepath.Dir(p)
	tmp, err := os.CreateTemp(dir, ".deckfold-*.tmp")
	if err != nil {
		return err
	}
	bw := bufio.NewWriterSize(tmp, 1<<20)
	if err := write(bw); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := bw.Flush(); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	_ = os.Chmod(tmp.Name(), 0o644)
	return os.Rename(tmp.Name(), p)
}

func writeFile(p string, data []byte) error {
	dir := filepath.Dir(p)
	tmp, err := os.CreateTemp(dir, ".deckfold-*.tmp")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	_ = os.Chmod(tmp.Name(), 0o644)
	return os.Rename(tmp.Name(), p)
}

// isPowerPoint reports whether the input is a PowerPoint file: by extension, or by content for
// files without one of the PowerPoint extensions (a ZIP with a presentation, an OLE file).
func isPowerPoint(p string) bool {
	if pptx.IsPowerPoint(p, nil) {
		return true
	}
	ext := strings.ToLower(filepath.Ext(p))
	if ext == ".html" || ext == ".htm" || ext == ".xhtml" {
		return false
	}
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, 8)
	n, _ := io.ReadFull(f, head)
	switch opc.Sniff(head[:n]) {
	case "zip", "cfb":
		return true
	}
	return false
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
