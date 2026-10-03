package main

import (
	"errors"
	"os/exec"
	"strings"

	"github.com/norka-app/Norka/internal/cli"
)

func startHidden(exe string) error {
	exe = strings.TrimSpace(exe)
	if exe == "" {
		return errors.New("executable path is empty")
	}
	cmd := exec.Command(exe, cli.HiddenArg)
	cmd.SysProcAttr = hiddenAttr()
	return cmd.Start()
}
