//go:build unix

package filelock

import (
	"errors"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

func lockExclusive(f *os.File) error {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err == nil {
		return nil
	}
	if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) {
		return ErrHeld
	}
	return err
}

func unlock(f *os.File) error {
	err := unix.Flock(int(f.Fd()), unix.LOCK_UN)
	if err == nil || errors.Is(err, io.ErrClosedPipe) {
		return nil
	}
	return err
}
