package biz

import (
	"errors"
	"strings"
	"testing"

	"github.com/norka-app/Norka/internal/localbind"
	"github.com/norka-app/Norka/internal/model"
)

func busyLocalListen() error {
	return localbind.AnnotateListen("127.0.0.1:3000", errors.New(
		"listen tcp 127.0.0.1:3000: bind: Only one usage of each socket address (protocol/network address/port) is normally permitted.",
	))
}

func TestExternalBindReasonHintsWhenPortIsNotNorka(t *testing.T) {
	tunnel := model.Tunnel{
		ID: 1, Mode: "local", LocalHost: "127.0.0.1", LocalPort: 3000, RemotePort: 8080,
	}
	got := externalBindReason(busyLocalListen(), tunnel, []model.Tunnel{tunnel})
	port, suggested, remote, raw, ok := localbind.Parse(got)
	if !ok || port != 3000 || remote != 8080 || suggested < 1 || suggested == 3000 {
		t.Fatalf("hint = %q", got)
	}
	if !strings.Contains(raw, "Only one usage of each socket address") {
		t.Fatalf("raw OS error dropped: %q", raw)
	}
}

func TestExternalBindReasonKeepsNorkaConflict(t *testing.T) {
	tunnel := model.Tunnel{
		ID: 1, Mode: "local", LocalHost: "127.0.0.1", LocalPort: 3000, RemotePort: 80,
	}
	siblings := []model.Tunnel{
		tunnel,
		{ID: 2, Name: "api", Status: "running", Mode: "local", LocalHost: "127.0.0.1", LocalPort: 3000},
	}
	got := externalBindReason(busyLocalListen(), tunnel, siblings)
	if strings.Contains(got, localbind.Prefix) {
		t.Fatalf("Norka port conflict was rewritten: %s", got)
	}
	if !strings.Contains(got, "Only one usage of each socket address") {
		t.Fatalf("raw error = %q", got)
	}
}

func TestExternalBindReasonLeavesOtherFailures(t *testing.T) {
	tunnel := model.Tunnel{ID: 1, Mode: "local", LocalHost: "127.0.0.1", LocalPort: 3000, RemotePort: 80}
	err := errors.New("ssh: handshake failed")
	if got := externalBindReason(err, tunnel, nil); got != err.Error() {
		t.Fatalf("other error = %q", got)
	}

	for _, mode := range []string{"remote", "dynamic"} {
		item := tunnel
		item.Mode = mode
		got := externalBindReason(busyLocalListen(), item, nil)
		if strings.Contains(got, localbind.Prefix) {
			t.Fatalf("mode %s was rewritten: %s", mode, got)
		}
	}
}
