//go:build windows

package scheme

import (
	"errors"
	"os"

	"golang.org/x/sys/windows/registry"
)

const protocolKey = `Software\Classes\norka`

// Sync writes HKCU\Software\Classes\norka while automation is on and removes
// that key when the flag is turned off.
func Sync(enabled bool) error {
	if enabled {
		return register()
	}
	return unregister()
}

func register() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	command, err := OpenCommand(exe)
	if err != nil {
		return err
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, protocolKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	if err := key.SetStringValue("", "URL:Norka"); err != nil {
		_ = key.Close()
		return err
	}
	if err := key.SetStringValue("URL Protocol", ""); err != nil {
		_ = key.Close()
		return err
	}
	if err := key.Close(); err != nil {
		return err
	}
	cmdKey, _, err := registry.CreateKey(registry.CURRENT_USER, protocolKey+`\shell\open\command`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer cmdKey.Close()
	return cmdKey.SetStringValue("", command)
}

func unregister() error {
	keys := []string{
		protocolKey + `\shell\open\command`,
		protocolKey + `\shell\open`,
		protocolKey + `\shell`,
		protocolKey,
	}
	var first error
	for _, path := range keys {
		err := registry.DeleteKey(registry.CURRENT_USER, path)
		if err != nil && !errors.Is(err, registry.ErrNotExist) && first == nil {
			first = err
		}
	}
	return first
}
