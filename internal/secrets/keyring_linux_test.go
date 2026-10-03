//go:build linux

package secrets

import (
	"testing"

	"github.com/norka-app/Norka/internal/model"
)

// TestLinuxKeychainFallsBackWithoutSecretService checks the real Secret Service
// provider. When it cannot store a probe secret, passwords stay in the config
// struct instead of the keychain.
func TestLinuxKeychainFallsBackWithoutSecretService(t *testing.T) {
	if err := Probe(systemKeyring{}); err == nil {
		t.Skip("Secret Service accepted a probe on this machine")
	}

	vault := OpenSystem()
	if vault.Available() {
		t.Fatal("keychain reported available after a failed probe")
	}

	next := &model.Jumper{Name: "jump", AuthType: "password"}
	forget := vault.Seal(nil, next, "s3cret")
	if forget != "" {
		t.Fatalf("forget ref = %q", forget)
	}
	if next.Password != "s3cret" || next.SecretRef != "" {
		t.Fatalf("plaintext fallback = password %q ref %q", next.Password, next.SecretRef)
	}
}
