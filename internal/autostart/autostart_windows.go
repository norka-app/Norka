//go:build windows

package autostart

import (
	"os"
	"strconv"

	"golang.org/x/sys/windows/registry"
)

const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

func openRunKey(access uint32) (registry.Key, error) {
	return registry.OpenKey(registry.CURRENT_USER, runKeyPath, access)
}

func IsEnabled() (bool, error) {
	migrateLegacyRunValue()

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

// migrateLegacyRunValue moves a previous "sshurik" Run entry to "norka"
// and points it at the current executable.
func migrateLegacyRunValue() {
	key, err := openRunKey(registry.QUERY_VALUE | registry.SET_VALUE)
	if err != nil {
		return
	}
	defer key.Close()

	_, _, err = key.GetStringValue(legacyAppName)
	if err != nil {
		return
	}
	if _, _, currentErr := key.GetStringValue(appName); currentErr == registry.ErrNotExist {
		exe, exeErr := os.Executable()
		if exeErr != nil || exe == "" {
			return
		}
		if err := key.SetStringValue(appName, strconv.Quote(exe)); err != nil {
			return
		}
	}
	_ = key.DeleteValue(legacyAppName)
}

func Enable() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	key, err := openRunKey(registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if err := key.SetStringValue(appName, strconv.Quote(exe)); err != nil {
		return err
	}
	err = key.DeleteValue(legacyAppName)
	if err == registry.ErrNotExist {
		return nil
	}
	return err
}

func Disable() error {
	key, err := openRunKey(registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	err = key.DeleteValue(appName)
	if err != nil && err != registry.ErrNotExist {
		return err
	}
	err = key.DeleteValue(legacyAppName)
	if err == registry.ErrNotExist {
		return nil
	}
	return err
}
