//go:build !windows

package main

import "syscall"

func hiddenAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}
