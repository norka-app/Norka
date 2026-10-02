//go:build !windows && !darwin

package update

import (
	"context"
	"errors"
)

func installPrepared(context.Context, string) error {
	return errors.New("install is not supported on this os")
}

func scheduleRelaunch() error {
	return errors.New("relaunch is not supported on this os")
}

func openLocalFile(string) error {
	return errors.New("open is not supported on this os")
}
