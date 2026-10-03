// Package hopsecret reports hops norkad must not dial on its own.
// A password or a key passphrase can block on the GUI keychain prompt.
// The window may pass that secret over IPC for one explicit start.
// The value is not written to config and is not logged.
package hopsecret

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/norka-app/Norka/internal/model"
)

const (
	// ReasonPassword is password authentication.
	ReasonPassword = "password auth needs the GUI keychain prompt; norkad only uses keys and ssh-agent"
	// ReasonPassphrase is an encrypted key or a keychain reference without the secret in memory.
	ReasonPassphrase = "key passphrase needs the GUI keychain prompt; norkad will not block on it"
)

// Reason returns why the chain must not be dialed without a secret from the window.
// An empty string means keys or ssh-agent can start without asking.
// Secrets already stored in config are not opened: a keychain read can block.
func Reason(jumpers []model.Jumper) string {
	return Uncovered(jumpers, nil)
}

// Uncovered is Reason, skipping hops covered reports as already supplied.
// covered may be nil.
func Uncovered(jumpers []model.Jumper, covered func(jumperID int) bool) string {
	for _, hop := range jumpers {
		why := hopReason(hop)
		if why == "" {
			continue
		}
		if covered != nil && covered(hop.ID) {
			continue
		}
		return why
	}
	return ""
}

func hopReason(hop model.Jumper) string {
	auth := strings.TrimSpace(hop.AuthType)
	if auth == "password" {
		return ReasonPassword
	}
	if strings.TrimSpace(hop.Password) == "" && strings.TrimSpace(hop.SecretRef) != "" {
		return ReasonPassphrase
	}
	if auth == "ssh_key" && strings.TrimSpace(hop.Password) == "" && keyNeedsPassphrase(hop.KeyPath) {
		return ReasonPassphrase
	}
	return ""
}

func keyNeedsPassphrase(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return false
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	_, err = ssh.ParsePrivateKey(raw)
	var missing *ssh.PassphraseMissingError
	return errors.As(err, &missing)
}
