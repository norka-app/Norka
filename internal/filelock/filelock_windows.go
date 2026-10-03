//go:build windows

package filelock

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// payloadLockOffset is a single byte past anything this process writes.
// LockFileEx denies reads of the locked range. flock on Unix does not.
// Keeping the lock byte past the payload lets another process read the
// file (engine.lock JSON) without taking the lock.
const payloadLockOffset uint64 = 1 << 32

func lockExclusive(f *os.File) error {
	ol := overlappedAt(payloadLockOffset)
	err := windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		1,
		0,
		&ol,
	)
	if err == nil {
		return nil
	}
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return ErrHeld
	}
	return err
}

func unlock(f *os.File) error {
	ol := overlappedAt(payloadLockOffset)
	err := windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &ol)
	if err == nil || errors.Is(err, windows.ERROR_NOT_LOCKED) {
		return nil
	}
	return err
}

func overlappedAt(offset uint64) windows.Overlapped {
	return windows.Overlapped{
		Offset:     uint32(offset),
		OffsetHigh: uint32(offset >> 32),
	}
}
