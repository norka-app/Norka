//go:build !darwin && !windows

package autostart

func IsEnabled() (bool, error) {
	return false, nil
}

func Enable() error {
	return nil
}

func Disable() error {
	return nil
}
