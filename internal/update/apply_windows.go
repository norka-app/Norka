//go:build windows

package update

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
)

func installPrepared(_ context.Context, src string) error {
	exe, err := currentExecutable()
	if err != nil {
		return err
	}
	if !dirWritable(filepath.Dir(exe)) {
		return errNotWritable
	}
	return swapFile(exe, src)
}

// scheduleRelaunch starts the replaced executable directly. A detached cmd
// script used to poll with tasklist, find and ping, and each of those console
// programs opened its own visible window. The new process waits with
// OpenProcess/WaitForSingleObject and never creates a console.
func scheduleRelaunch() error {
	exe, err := currentExecutable()
	if err != nil {
		return err
	}
	return relaunchCommand(exe, os.Getpid()).Start()
}

func relaunchCommand(exe string, pid int) *exec.Cmd {
	cmd := exec.Command(exe, AfterUpdateWaitArgs(pid)...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP,
	}
	return cmd
}

func openLocalFile(path string) error {
	return shellOpen(path)
}

// shellOpen asks the shell to open a file or folder. ShellExecute does not
// start cmd.exe, so no console window appears.
func shellOpen(path string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL)
}
