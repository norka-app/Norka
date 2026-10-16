//go:build !windows

package main

import "os/exec"

func execRun(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}
