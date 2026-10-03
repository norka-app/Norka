//go:build windows

package attach

import "syscall"

func detachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow: true,
		// DETACHED_PROCESS | CREATE_NO_WINDOW. The child survives the window
		// and does not flash a console.
		CreationFlags: 0x00000008 | 0x08000000,
	}
}
