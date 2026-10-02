//go:build darwin

package autostart

import (
	"os"

	"github.com/emersion/go-autostart"
)

const appDisplayName = "Norka"

func newApp() *autostart.App {
	execPath, _ := os.Executable()
	if execPath == "" {
		execPath = "norka"
	}
	return &autostart.App{
		Name:        appName,
		DisplayName: appDisplayName,
		Exec:        []string{execPath},
	}
}

func IsEnabled() (bool, error) {
	return newApp().IsEnabled(), nil
}

func Enable() error {
	return newApp().Enable()
}

func Disable() error {
	return newApp().Disable()
}
