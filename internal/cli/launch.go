package cli

import (
	"os"
	"os/exec"
)

// LaunchHidden starts this executable in the tray and returns without waiting.
// On Windows the child is created with no console window.
func LaunchHidden() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, HiddenArg)
	cmd.SysProcAttr = hiddenAttr()
	return cmd.Start()
}
