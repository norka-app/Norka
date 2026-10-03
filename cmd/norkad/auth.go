package main

import (
	"github.com/norka-app/Norka/internal/hopsecret"
	"github.com/norka-app/Norka/internal/model"
)

const (
	reasonPasswordAuth  = hopsecret.ReasonPassword
	reasonKeyPassphrase = hopsecret.ReasonPassphrase
)

// skipReason reports why norkad must not dial this tunnel on its own.
// An empty string means keys or ssh-agent can start without asking the GUI.
// An explicit start from the window can still pass the secret over IPC.
// Secrets are the values stored in config: a keychain reference is not opened,
// because that read can block on the GUI unlock prompt.
func skipReason(_ model.Tunnel, jumpers []model.Jumper) string {
	return hopsecret.Reason(jumpers)
}
