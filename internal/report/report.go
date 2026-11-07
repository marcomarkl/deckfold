// Package report collects warnings, errors and figures of a conversion.
package report

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// Missing is an asset that could not be resolved, with the place it was referenced from.
type Missing struct {
	Ref    string // reference as written in the source
	Where  string // location, e.g. "index.html: <img src>"
	Reason string
}

// Sized is an asset with its size, for the size report.
type Sized struct {
	Name string
	Size int
}

// Report holds the result of a conversion.
type Report struct {
	Format     string
	Warnings   []string
	Missing    []Missing
	Hosts      map[string]bool
	NetSlides  []string // slides or places that need a live network connection (E9)
	Assets     []Sized
	OutputSize int
	Output     string
	Summary    string   // one line about the content, e.g. slide counts of a PowerPoint file
	Fonts      []string // fonts and where they came from (PowerPoint files)
}

// New creates an empty report.
func New() *Report { return &Report{Hosts: map[string]bool{}} }

// Warn adds a warning.
func (r *Report) Warn(format string, a ...any) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, a...))
}

// Miss records an asset that could not be resolved.
func (r *Report) Miss(ref, where, reason string) {
	for _, m := range r.Missing {
		if m.Ref == ref && m.Where == where {
			return
		}
	}
	r.Missing = append(r.Missing, Missing{ref, where, reason})
}

// HumanSize formats a byte count.
func HumanSize(n int) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

// PrintMissing prints the list of unresolved assets.
func (r *Report) PrintMissing(w io.Writer) {
	fmt.Fprintf(w, "Unresolved assets (%d):\n", len(r.Missing))
	for _, m := range r.Missing {
		fmt.Fprintf(w, "  - %s\n      referenced in: %s\n      reason: %s\n", m.Ref, m.Where, m.Reason)
	}
}

// Print prints the final report.
func (r *Report) Print(w io.Writer, warnSize int) {
	fmt.Fprintf(w, "Format: %s\n", r.Format)
	if r.Summary != "" {
		fmt.Fprintf(w, "Content: %s\n", r.Summary)
	}
	if r.Output != "" {
		fmt.Fprintf(w, "Output: %s (%s)\n", r.Output, HumanSize(r.OutputSize))
	}
	if len(r.Hosts) > 0 {
		hs := make([]string, 0, len(r.Hosts))
		for h := range r.Hosts {
			hs = append(hs, h)
		}
		sort.Strings(hs)
		fmt.Fprintf(w, "Hosts contacted: %s\n", strings.Join(hs, ", "))
	}
	if len(r.Assets) > 0 {
		a := append([]Sized(nil), r.Assets...)
		sort.Slice(a, func(i, j int) bool { return a[i].Size > a[j].Size })
		if len(a) > 10 {
			a = a[:10]
		}
		fmt.Fprintf(w, "Largest assets:\n")
		for _, s := range a {
			fmt.Fprintf(w, "  %10s  %s\n", HumanSize(s.Size), s.Name)
		}
	}
	if len(r.Fonts) > 0 {
		fmt.Fprintf(w, "Fonts:\n")
		for _, s := range r.Fonts {
			fmt.Fprintf(w, "  %s\n", s)
		}
	}
	if len(r.NetSlides) > 0 {
		fmt.Fprintf(w, "These places need a network connection during the talk (live content; offline they show a still image or a placeholder):\n")
		for _, s := range r.NetSlides {
			fmt.Fprintf(w, "  - %s\n", s)
		}
	}
	if warnSize > 0 && r.OutputSize > warnSize {
		r.Warn("output is %s, above the warning threshold of %s; large files open much more slowly in Safari", HumanSize(r.OutputSize), HumanSize(warnSize))
	}
	if len(r.Warnings) > 0 {
		fmt.Fprintf(w, "Warnings (%d):\n", len(r.Warnings))
		for _, s := range r.Warnings {
			fmt.Fprintf(w, "  - %s\n", s)
		}
	}
	if len(r.Missing) > 0 {
		r.PrintMissing(w)
	}
}
