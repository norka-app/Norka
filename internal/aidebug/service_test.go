package aidebug

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"norka/internal/model"
)

func TestMatchRulesPublicKeyOfferedButRejected(t *testing.T) {
	rawError := "ssh handshake failed: ssh: unable to authenticate, attempted methods [none publickey], no supported methods remain"
	debugOutput := `
debug1: Offering public key: norka-test-key.pem explicit
debug1: Authentications that can continue: publickey
debug1: No more authentication methods to try.
Permission denied (publickey).
`

	rules := matchRules(rawError, debugOutput)
	if !containsRule(rules, "Public key was offered but rejected by remote server") {
		t.Fatalf("expected remote public key rejection rule, got %#v", rules)
	}
	if containsRule(rules, "Key file is invalid or unreadable") {
		t.Fatalf("did not expect local key file rule, got %#v", rules)
	}
}

func TestResolveKeyPathFallsBackToSSHDir(t *testing.T) {
	home := t.TempDir()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	keyPath := filepath.Join(sshDir, "norka-test-key.pem")
	if err := os.WriteFile(keyPath, []byte("dummy"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	resolved, err := resolveKeyPath("norka-test-key.pem")
	if err != nil {
		t.Fatalf("resolveKeyPath() error = %v", err)
	}
	if resolved != keyPath {
		t.Fatalf("resolveKeyPath() = %q, want %q", resolved, keyPath)
	}
}

func TestBuildSSHConfigResolvesIdentityFile(t *testing.T) {
	home := t.TempDir()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	keyPath := filepath.Join(sshDir, "norka-test-key.pem")
	if err := os.WriteFile(keyPath, []byte("dummy"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	cfgPath, cleanup, checks := buildSSHConfig([]model.Jumper{{
		Host:     "example.com",
		User:     "root",
		AuthType: "ssh_key",
		KeyPath:  "norka-test-key.pem",
	}})
	t.Cleanup(func() {
		if cleanup != nil {
			cleanup()
		}
	})
	if len(checks) == 0 || checks[0].Status != "ok" {
		t.Fatalf("buildSSHConfig() checks = %#v, want ok", checks)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "IdentityFile "+keyPath) {
		t.Fatalf("config does not contain resolved identity file path:\n%s", string(data))
	}
}

func TestNormalizeLLMConfidenceAcceptsNumbers(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "high decimal", value: 0.9, want: "high"},
		{name: "medium decimal", value: 0.5, want: "medium"},
		{name: "low decimal", value: 0.2, want: "low"},
		{name: "high percent", value: 85.0, want: "high"},
		{name: "string number", value: "0.75", want: "high"},
		{name: "localized high", value: "высокая", want: "high"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeLLMConfidence(tt.value); got != tt.want {
				t.Fatalf("normalizeLLMConfidence(%v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestCallLLMDoesNotContactBackend(t *testing.T) {
	service := NewService("", "")
	_, err := service.callLLM(
		context.Background(),
		"en",
		"permission denied",
		nil,
		nil,
		sshClient{},
		"",
		DiagnosticInput{TargetType: "tunnel_test"},
	)
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("callLLM() error = %v, want disabled", err)
	}
}

func containsRule(rules []string, want string) bool {
	for _, rule := range rules {
		if rule == want {
			return true
		}
	}
	return false
}
