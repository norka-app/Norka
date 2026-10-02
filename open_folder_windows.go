//go:build windows

package main

import "golang.org/x/sys/windows"

func openFolder(path string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	// ShellExecute, not "cmd /C start" or explorer.exe: a console child of a
	// windowless parent would open its own visible window.
	return windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL)
}
