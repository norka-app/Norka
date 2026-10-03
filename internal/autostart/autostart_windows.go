//go:build windows

package autostart

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

func openRunKey(access uint32) (registry.Key, error) {
	return registry.OpenKey(registry.CURRENT_USER, runKeyPath, access)
}

func IsEnabled() (bool, error) {
	key, err := openRunKey(registry.QUERY_VALUE)
	if err != nil {
		return false, err
	}
	defer key.Close()

	_, _, err = key.GetStringValue(appName)
	if err == registry.ErrNotExist {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func Enable(hidden bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	key, err := openRunKey(registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetStringValue(appName, windowsRunValue(exe, hidden))
}

func matches(hidden bool) (bool, error) {
	key, err := openRunKey(registry.QUERY_VALUE)
	if err != nil {
		return false, err
	}
	defer key.Close()
	got, _, err := key.GetStringValue(appName)
	if err == registry.ErrNotExist {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	exe, err := os.Executable()
	if err != nil {
		return false, err
	}
	return got == windowsRunValue(exe, hidden), nil
}

func Disable() error {
	key, err := openRunKey(registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	err = key.DeleteValue(appName)
	if err == registry.ErrNotExist {
		return nil
	}
	return err
}
