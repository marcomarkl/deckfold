//go:build windows

package main

import (
	"bufio"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// setupConsole switches console output to UTF-8 so non-ASCII characters print correctly.
func setupConsole() {
	_ = windows.SetConsoleOutputCP(65001)
}

// pauseIfOwnConsole waits for Enter when deckfold owns its console (for example after a
// file was dropped onto the .exe), so the report stays readable.
func pauseIfOwnConsole() {
	var pids [4]uint32
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetConsoleProcessList")
	if proc.Find() != nil {
		return
	}
	n, _, _ := proc.Call(uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids)))
	if n != 1 {
		return
	}
	fmt.Print("\nPress Enter to close ...")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}
