//go:build !darwin && !windows && !linux

package autostart

func IsEnabled() (bool, error) {
	return false, nil
}

func Enable(bool) error {
	return nil
}

func matches(bool) (bool, error) {
	return true, nil
}

func Disable() error {
	return nil
}
