//go:build windows

package update

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
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

func scheduleRelaunch() error {
	exe, err := currentExecutable()
	if err != nil {
		return err
	}
	scriptPath := filepath.Join(os.TempDir(), fmt.Sprintf("norka-relaunch-%d.cmd", os.Getpid()))
	body := windowsRelaunchScript(os.Getpid(), exe)
	if err := os.WriteFile(scriptPath, []byte(body), 0o600); err != nil {
		return err
	}
	cmd := exec.Command("cmd", "/C", scriptPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000008 | 0x00000200, // DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP
	}
	return cmd.Start()
}

func openLocalFile(path string) error {
	return exec.Command("cmd", "/C", "start", "", path).Start()
}
