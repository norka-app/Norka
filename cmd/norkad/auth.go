package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/norka-app/Norka/internal/model"
)

const (
	reasonPasswordAuth  = "password auth needs the GUI keychain prompt; norkad only uses keys and ssh-agent"
	reasonKeyPassphrase = "key passphrase needs the GUI keychain prompt; norkad will not block on it"
)

// skipReason reports why norkad must not dial this tunnel.
// An empty string means keys or ssh-agent can start without asking the GUI.
// Secrets are the values stored in config: a keychain reference is not opened,
// because that read can block on the GUI unlock prompt.
func skipReason(_ model.Tunnel, jumpers []model.Jumper) string {
	for _, hop := range jumpers {
		auth := strings.TrimSpace(hop.AuthType)
		if auth == "password" {
			return reasonPasswordAuth
		}
		if strings.TrimSpace(hop.Password) == "" && strings.TrimSpace(hop.SecretRef) != "" {
			return reasonKeyPassphrase
		}
		if auth == "ssh_key" && strings.TrimSpace(hop.Password) == "" && keyNeedsPassphrase(hop.KeyPath) {
			return reasonKeyPassphrase
		}
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
