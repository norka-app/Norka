//go:build !windows

package cli

import "syscall"

func hiddenAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}
