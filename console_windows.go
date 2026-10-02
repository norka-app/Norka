//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

var procAttachConsole = windows.NewLazySystemDLL("kernel32.dll").NewProc("AttachConsole")

// attachParentConsole connects a windowed Windows build to the console that
// launched it, so `norka status` can print from cmd and PowerShell.
// It never calls AllocConsole, which would open a new window.
func attachParentConsole() {
	const attachParentProcess = ^uintptr(0)
	r, _, _ := procAttachConsole.Call(attachParentProcess)
	if r == 0 {
		return
	}
	_ = windows.SetConsoleOutputCP(65001)
	if out, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE); err == nil {
		os.Stdout = os.NewFile(uintptr(out), "stdout")
	}
	if errh, err := windows.GetStdHandle(windows.STD_ERROR_HANDLE); err == nil {
		os.Stderr = os.NewFile(uintptr(errh), "stderr")
	}
	if in, err := windows.GetStdHandle(windows.STD_INPUT_HANDLE); err == nil {
		os.Stdin = os.NewFile(uintptr(in), "stdin")
	}
}
