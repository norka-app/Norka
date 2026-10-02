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

func legacyApp() *autostart.App {
	return &autostart.App{
		Name:        legacyAppName,
		DisplayName: "SSHurik",
		Exec:        []string{"sshurik"},
	}
}

func migrateLegacyAutostart() {
	legacy := legacyApp()
	if !legacy.IsEnabled() {
		return
	}
	current := newApp()
	if !current.IsEnabled() {
		_ = current.Enable()
	}
	_ = legacy.Disable()
}

func IsEnabled() (bool, error) {
	migrateLegacyAutostart()
	return newApp().IsEnabled(), nil
}

func Enable() error {
	migrateLegacyAutostart()
	return newApp().Enable()
}

func Disable() error {
	if err := newApp().Disable(); err != nil {
		return err
	}
	legacy := legacyApp()
	if !legacy.IsEnabled() {
		return nil
	}
	return legacy.Disable()
}
