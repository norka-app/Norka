package secrets

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"

	"norka/internal/model"
)

// Vault moves jumper passwords between config structs and the OS keychain.
type Vault struct {
	ring      Keyring
	available bool
}

// NewVault builds a vault. available is false when the OS keychain cannot be used.
func NewVault(ring Keyring, available bool) *Vault {
	return &Vault{ring: ring, available: available}
}

// OpenSystem probes the OS keychain. A failed probe keeps passwords in config.toml.
func OpenSystem() *Vault {
	ring := systemKeyring{}
	if err := Probe(ring); err != nil {
		slog.Warn("os keychain unavailable; jumper passwords remain in config.toml", "error", err)
		return &Vault{ring: ring, available: false}
	}
	slog.Info("os keychain available for jumper secrets")
	return &Vault{ring: ring, available: true}
}

// Available reports whether new secrets are written to the keychain.
func (v *Vault) Available() bool {
	return v != nil && v.available
}

// Open fills Password from the keychain when the struct only has a reference.
// A plaintext Password already on the struct is left as-is.
func (v *Vault) Open(jumper *model.Jumper) {
	if v == nil || jumper == nil || v.ring == nil {
		return
	}
	if jumper.Password != "" || strings.TrimSpace(jumper.SecretRef) == "" {
		return
	}
	secret, err := v.ring.Get(ServiceName, jumper.SecretRef)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			slog.Warn("keychain read failed", "jumper_id", jumper.ID, "error", err)
		}
		return
	}
	jumper.Password = secret
}

// Seal stores plaintext in the keychain when it is available and clears it from next.
// An empty plaintext keeps the existing secret. The returned reference should be
// deleted after the config save succeeds.
func (v *Vault) Seal(existing *model.Jumper, next *model.Jumper, plaintext string) string {
	if next == nil {
		return ""
	}
	if v == nil || !v.available || v.ring == nil {
		return sealPlain(existing, next, plaintext)
	}
	if next.AuthType == "ssh_agent" {
		next.Password = ""
		next.SecretRef = ""
		return secretRef(existing)
	}
	if plaintext != "" {
		ref := secretRef(existing)
		if ref == "" {
			ref = newSecretRef()
		}
		if err := v.ring.Set(ServiceName, ref, plaintext); err != nil {
			slog.Warn("keychain write failed; storing jumper secret in config", "error", err)
			next.Password = plaintext
			next.SecretRef = ""
			return secretRef(existing)
		}
		next.Password = ""
		next.SecretRef = ref
		return ""
	}
	if existing != nil {
		next.Password = existing.Password
		next.SecretRef = existing.SecretRef
		return ""
	}
	next.Password = ""
	next.SecretRef = ""
	return ""
}

func sealPlain(existing *model.Jumper, next *model.Jumper, plaintext string) string {
	if next.AuthType == "ssh_agent" {
		next.Password = ""
		next.SecretRef = ""
		return secretRef(existing)
	}
	if plaintext != "" {
		next.Password = plaintext
		next.SecretRef = ""
		return secretRef(existing)
	}
	if existing != nil {
		next.Password = existing.Password
		next.SecretRef = existing.SecretRef
		return ""
	}
	next.Password = ""
	next.SecretRef = ""
	return ""
}

// Forget removes a keychain item. A missing item is not an error.
func (v *Vault) Forget(ref string) {
	if v == nil || v.ring == nil {
		return
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return
	}
	if err := v.ring.Delete(ServiceName, ref); err != nil && !errors.Is(err, ErrNotFound) {
		slog.Warn("keychain delete failed", "error", err)
	}
}

// Redact clears Password and sets HasSecret for API responses.
func Redact(jumper *model.Jumper) {
	if jumper == nil {
		return
	}
	jumper.HasSecret = jumper.Password != "" || strings.TrimSpace(jumper.SecretRef) != ""
	jumper.Password = ""
}

func secretRef(jumper *model.Jumper) string {
	if jumper == nil {
		return ""
	}
	return strings.TrimSpace(jumper.SecretRef)
}

func newSecretRef() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return hex.EncodeToString([]byte("norka-fallback-ref!!"))
	}
	return "jumper-" + hex.EncodeToString(buf)
}
