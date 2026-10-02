//go:build windows

package update

import (
	"time"

	"golang.org/x/sys/windows"
)

// WaitForPIDExit blocks until pid exits or afterUpdateWaitTimeout elapses.
// A missing, inaccessible, or already exited process returns immediately.
func WaitForPIDExit(pid int) {
	waitForPIDExit(pid, afterUpdateWaitTimeout)
}

func waitForPIDExit(pid int, timeout time.Duration) {
	if pid <= 0 || uint64(pid) > uint64(^uint32(0)) {
		return
	}
	ms := timeout.Milliseconds()
	if ms <= 0 {
		return
	}
	if ms > int64(^uint32(0)) {
		ms = int64(^uint32(0))
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return
	}
	defer windows.CloseHandle(handle)
	_, _ = windows.WaitForSingleObject(handle, uint32(ms))
}
