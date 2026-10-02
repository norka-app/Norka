//go:build !windows

package ipc

import (
	"fmt"
	"os"
)

func fileIsUserOnly(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("automation token file is readable by other users")
	}
	return nil
}

func hardenFile(path string) error {
	return os.Chmod(path, 0o600)
}
