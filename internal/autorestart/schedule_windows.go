//go:build windows

package autorestart

import (
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
	"github.com/norka-app/Norka/internal/update"
)

func scheduleRelaunch(exe string, args []string) error {
	// Ждём именно завершения этого процесса, а не фиксированные 2 секунды через
	// cmd. Иначе timeout.exe и start открывают свои консоли: у отсоединённого
	// cmd нет окна, и каждый консольный потомок получает новое.
	argv := make([]string, 0, len(args)+2)
	argv = append(argv, update.AfterUpdateWaitArgs(os.Getpid())...)
	argv = append(argv, args...)
	cmd := exec.Command(exe, argv...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP,
	}
	return cmd.Start()
}
