//go:build darwin

package autostart

import (
	"os"
	"path/filepath"
	"slices"

	"github.com/emersion/go-autostart"
)

const appDisplayName = "Norka"

func newApp(hidden bool) *autostart.App {
	execPath, _ := os.Executable()
	return &autostart.App{
		Name:        appName,
		DisplayName: appDisplayName,
		Exec:        darwinProgramArguments(execPath, hidden),
	}
}

func launchAgentPath() string {
	return filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents", appName+".plist")
}

func IsEnabled() (bool, error) {
	return newApp(false).IsEnabled(), nil
}

func Enable(hidden bool) error {
	return newApp(hidden).Enable()
}

func matches(hidden bool) (bool, error) {
	data, err := os.ReadFile(launchAgentPath())
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	exe, err := os.Executable()
	if err != nil {
		return false, err
	}
	got, ok := plistProgramArgs(string(data))
	if !ok {
		return false, nil
	}
	return slices.Equal(got, darwinProgramArguments(exe, hidden)), nil
}

func Disable() error {
	return newApp(false).Disable()
}
