//go:build windows

package main

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// execRun запускает внешнюю команду без окна консоли.
// Автозапуск на Windows пишет HKCU и эту функцию не зовёт.
// Флаги те же, что у перезапуска после обновления: окно cmd не появляется.
func execRun(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
	return cmd.Run()
}
