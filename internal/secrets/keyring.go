package secrets

import (
	"errors"
	"log/slog"

	"github.com/zalando/go-keyring"
)

// ServiceName is the OS keychain service for jumper passwords and key passphrases.
const ServiceName = "norka"

// ErrNotFound is returned when a keychain item does not exist.
var ErrNotFound = errors.New("secret not found in keyring")

// Keyring stores a secret under a service and account name.
type Keyring interface {
	Set(service, user, secret string) error
	Get(service, user string) (string, error)
	Delete(service, user string) error
}

type systemKeyring struct{}

func (systemKeyring) Set(service, user, secret string) error {
	return keyring.Set(service, user, secret)
}

func (systemKeyring) Get(service, user string) (string, error) {
	secret, err := keyring.Get(service, user)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	return secret, err
}

func (systemKeyring) Delete(service, user string) error {
	err := keyring.Delete(service, user)
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

// Probe checks that the keychain can round-trip a throwaway value.
func Probe(ring Keyring) error {
	if ring == nil {
		return errors.New("keyring is nil")
	}
	const user = "norka-keychain-probe"
	const secret = "ok"
	if err := ring.Set(ServiceName, user, secret); err != nil {
		return err
	}
	got, err := ring.Get(ServiceName, user)
	delErr := ring.Delete(ServiceName, user)
	if err != nil {
		return err
	}
	if got != secret {
		return errors.New("keychain probe mismatch")
	}
	if delErr != nil && !errors.Is(delErr, ErrNotFound) {
		slog.Warn("keychain probe cleanup failed", "error", delErr)
	}
	return nil
}
