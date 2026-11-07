// Command bundle builds runtime/dist from runtime/src (run through go generate ./runtime).
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"deckfold/runtime/bundle"
)

func main() {
	// go generate runs in the package folder runtime/
	js, css, err := bundle.Bundle("src")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.MkdirAll("dist", 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for name, data := range map[string]string{"deckfold.js": js, "deckfold.css": css} {
		if err := os.WriteFile(filepath.Join("dist", name), []byte(data), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	fmt.Printf("runtime/dist: %d bytes JS, %d bytes CSS\n", len(js), len(css))
}
