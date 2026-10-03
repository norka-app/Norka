//go:build windows

package loginstart

import (
	"errors"

	"golang.org/x/sys/windows/registry"
)

// WindowsRegistry writes HKCU. It is not a service and does not need an administrator.
type WindowsRegistry struct{}

func (WindowsRegistry) SetString(key, name, value string) error {
	opened, err := registry.OpenKey(registry.CURRENT_USER, key, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer opened.Close()
	return opened.SetStringValue(name, value)
}

func (WindowsRegistry) GetString(key, name string) (string, bool, error) {
	opened, err := registry.OpenKey(registry.CURRENT_USER, key, registry.QUERY_VALUE)
	if err != nil {
		return "", false, err
	}
	defer opened.Close()
	value, _, err := opened.GetStringValue(name)
	if errors.Is(err, registry.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (WindowsRegistry) DeleteValue(key, name string) error {
	opened, err := registry.OpenKey(registry.CURRENT_USER, key, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer opened.Close()
	err = opened.DeleteValue(name)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	return err
}
