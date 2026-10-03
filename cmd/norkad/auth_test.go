package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/norka-app/Norka/internal/model"
)

func TestSkipReason(t *testing.T) {
	plain, encrypted := writeKeys(t)
	tunnel := model.Tunnel{Name: "db"}
	cases := []struct {
		name   string
		hop    model.Jumper
		reason string
	}{
		{name: "agent", hop: model.Jumper{AuthType: "ssh_agent"}},
		{name: "plain key", hop: model.Jumper{AuthType: "ssh_key", KeyPath: plain}},
		{name: "key with passphrase already stored", hop: model.Jumper{AuthType: "ssh_key", KeyPath: encrypted, Password: "secret"}},
		{name: "password", hop: model.Jumper{AuthType: "password", Password: "secret"}, reason: reasonPasswordAuth},
		{name: "password in keychain", hop: model.Jumper{AuthType: "password", SecretRef: "jumper-1"}, reason: reasonPasswordAuth},
		{name: "encrypted key", hop: model.Jumper{AuthType: "ssh_key", KeyPath: encrypted}, reason: reasonKeyPassphrase},
		{name: "passphrase only in keychain", hop: model.Jumper{AuthType: "ssh_key", KeyPath: plain, SecretRef: "jumper-2"}, reason: reasonKeyPassphrase},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := skipReason(tunnel, []model.Jumper{tc.hop})
			if got != tc.reason {
				t.Fatalf("reason %q, want %q", got, tc.reason)
			}
		})
	}
}

func writeKeys(t *testing.T) (plain, encrypted string) {
	t.Helper()
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	plain = filepath.Join(dir, "plain")
	encrypted = filepath.Join(dir, "encrypted")
	runKeygen(t, "-t", "ed25519", "-f", plain, "-N", "", "-q")
	runKeygen(t, "-t", "ed25519", "-f", encrypted, "-N", "secret", "-q")
	if _, err := os.Stat(plain); err != nil {
		t.Fatal(err)
	}
	return plain, encrypted
}

func runKeygen(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("ssh-keygen", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen %v: %v\n%s", args, err, out)
	}
}
