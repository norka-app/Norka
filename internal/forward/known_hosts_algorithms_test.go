package forward

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestKnownHostKeyAlgorithmsPrefersStoredEd25519(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")
	content := "" +
		"10.200.29.191 ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKD7DOe86DhQhY2KDNy2hF9V3IFETuFwAsGWrly6tZc5\n" +
		"10.200.29.175 ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIICe2JGR2qsoepE4MCk6nW36YkRewe09baZQo0H1tTb2\n" +
		"10.200.29.175 ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDY\n" +
		"10.200.29.175 ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBA==\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := knownHostKeyAlgorithms(path, "10.200.29.191", 22)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{ssh.KeyAlgoED25519}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ed25519-only host algorithms = %v, want %v", got, want)
	}

	got, err = knownHostKeyAlgorithms(path, "10.200.29.175", 22)
	if err != nil {
		t.Fatal(err)
	}
	want = []string{
		ssh.KeyAlgoED25519,
		ssh.KeyAlgoECDSA256,
		ssh.KeyAlgoRSASHA512,
		ssh.KeyAlgoRSASHA256,
		ssh.KeyAlgoRSA,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("multi-key host algorithms = %v, want %v", got, want)
	}

	got, err = knownHostKeyAlgorithms(path, "10.200.29.200", 22)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("unknown host algorithms = %v, want none", got)
	}
}

func TestKnownHostKeyAlgorithmsMatchesHashedHost(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")
	line := knownhosts.HashHostname("10.200.29.191") + " ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKD7DOe86DhQhY2KDNy2hF9V3IFETuFwAsGWrly6tZc5\n"
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := knownHostKeyAlgorithms(path, "10.200.29.191", 22)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{ssh.KeyAlgoED25519}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("hashed host algorithms = %v, want %v", got, want)
	}
}

func TestRememberingHostKeyCallbackStoresUnknownAndRejectsChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")
	key := testEd25519PublicKey(t)
	addr := &net.TCPAddr{IP: net.ParseIP("203.0.113.5"), Port: 22}

	cb, err := newRememberingHostKeyCallback(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := cb("example.com:22", addr, key); err != nil {
		t.Fatalf("unknown host: %v", err)
	}

	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(stored), "example.com "+key.Type()+" ") {
		t.Fatalf("known_hosts = %q, want example.com key", stored)
	}

	again, err := newRememberingHostKeyCallback(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := again("example.com:22", addr, key); err != nil {
		t.Fatalf("stored key rejected: %v", err)
	}
	if err := again("example.com:22", addr, testEd25519PublicKey(t)); err == nil {
		t.Fatal("changed host key was accepted")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(stored) {
		t.Fatalf("changed key was written to known_hosts:\n%s", after)
	}
}

func TestRememberingHostKeyCallbackStoresNonStandardPort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")
	key := testEd25519PublicKey(t)
	addr := &net.TCPAddr{IP: net.ParseIP("203.0.113.5"), Port: 2222}

	cb, err := newRememberingHostKeyCallback(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := cb("example.com:2222", addr, key); err != nil {
		t.Fatalf("unknown host: %v", err)
	}
	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(stored), "[example.com]:2222 ") {
		t.Fatalf("known_hosts = %q, want [example.com]:2222", stored)
	}
}

func testEd25519PublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return signer.PublicKey()
}
