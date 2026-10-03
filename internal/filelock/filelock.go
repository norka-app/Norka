// Package filelock is an exclusive OS lock on a sidecar file.
// Unix uses flock. Windows uses LockFileEx on one byte past the payload,
// so a reader can still read the file without taking the lock.
// The kernel drops the lock when the process exits, including after a crash.
package filelock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	dirPerm  = 0o700
	filePerm = 0o600
	// retryEvery is how often a timed lock retries a busy file.
	retryEvery = 20 * time.Millisecond
)

var (
	// ErrTimeout means the lock was still busy when the deadline passed.
	ErrTimeout = errors.New("timed out waiting for file lock")
	// ErrHeld means the lock is already held and the caller did not wait.
	ErrHeld = errors.New("file lock is already held")
)

// Lock is an exclusive lock held for as long as the file stays open.
type Lock struct {
	f    *os.File
	path string
}

// procHeld makes a second Acquire in this process wait instead of taking
// the OS lock again. On Darwin flock is per process, so that second lock
// would succeed while the first holder still believed it was exclusive.
var (
	procMu   sync.Mutex
	procHeld = map[string]struct{}{}
)

// Acquire exclusively locks path, creating the file and its directory.
// timeout <= 0 tries once and returns ErrHeld when the file is busy.
// A positive timeout waits and returns ErrTimeout if the lock is still busy.
func Acquire(path string, timeout time.Duration) (*Lock, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	if err := waitTurn(abs, timeout, deadline); err != nil {
		return nil, err
	}
	f, err := openLockFile(abs)
	if err != nil {
		unmark(abs)
		return nil, err
	}
	for {
		err := lockExclusive(f)
		if err == nil {
			return &Lock{f: f, path: abs}, nil
		}
		if !errors.Is(err, ErrHeld) {
			_ = f.Close()
			unmark(abs)
			return nil, err
		}
		if timeout <= 0 || !time.Now().Before(deadline) {
			_ = f.Close()
			unmark(abs)
			if timeout <= 0 {
				return nil, ErrHeld
			}
			return nil, fmt.Errorf("%w after %s", ErrTimeout, timeout)
		}
		pause := retryEvery
		if remaining := time.Until(deadline); remaining < pause {
			pause = remaining
		}
		if pause > 0 {
			time.Sleep(pause)
		}
	}
}

// Release unlocks and closes the file. A second call does nothing.
// Closing the file also drops the lock if the process crashes before Release.
func (l *Lock) Release() error {
	if l == nil || l.f == nil {
		return nil
	}
	f := l.f
	path := l.path
	l.f = nil
	unlockErr := unlock(f)
	closeErr := f.Close()
	unmark(path)
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}

func waitTurn(path string, timeout time.Duration, deadline time.Time) error {
	for {
		if mark(path) {
			return nil
		}
		if timeout <= 0 || !time.Now().Before(deadline) {
			if timeout <= 0 {
				return ErrHeld
			}
			return fmt.Errorf("%w after %s", ErrTimeout, timeout)
		}
		pause := retryEvery
		if remaining := time.Until(deadline); remaining < pause {
			pause = remaining
		}
		if pause > 0 {
			time.Sleep(pause)
		}
	}
}

func mark(path string) bool {
	procMu.Lock()
	defer procMu.Unlock()
	if _, ok := procHeld[path]; ok {
		return false
	}
	procHeld[path] = struct{}{}
	return true
}

func unmark(path string) {
	if path == "" {
		return
	}
	procMu.Lock()
	delete(procHeld, path)
	procMu.Unlock()
}

// Rewrite replaces the file contents while the lock is held.
// It does not rename the file: the lock stays on this inode.
func (l *Lock) Rewrite(p []byte) error {
	if l == nil || l.f == nil {
		return errors.New("file lock is released")
	}
	if err := l.f.Truncate(0); err != nil {
		return err
	}
	if _, err := l.f.WriteAt(p, 0); err != nil {
		return err
	}
	return l.f.Sync()
}

func openLockFile(path string) (*os.File, error) {
	path = filepath.Clean(path)
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return nil, fmt.Errorf("create lock directory: %w", err)
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, filePerm)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", path, err)
	}
	return f, nil
}
