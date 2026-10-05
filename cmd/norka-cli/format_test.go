package main

import (
	"strings"
	"testing"

	"github.com/norka-app/Norka/internal/ipc"
	"github.com/norka-app/Norka/internal/localbind"
)

func TestFailedTextExplainsForeignLocalBind(t *testing.T) {
	raw := "listen 127.0.0.1:3000 failed: listen tcp 127.0.0.1:3000: bind: Only one usage of each socket address (protocol/network address/port) is normally permitted."
	resp := ipc.Response{
		Message: raw,
		Tunnels: []ipc.TunnelInfo{{
			Name:      "web",
			LastError: localbind.Format(3000, 13000, 8080, raw),
		}},
	}
	got := failedText(command{Target: "web"}, resp)
	if !strings.Contains(got, `The command failed for "web":`) ||
		!strings.Contains(got, "another program") ||
		!strings.Contains(got, "for example to 13000") ||
		!strings.Contains(got, raw) ||
		strings.Contains(got, localbind.Prefix) {
		t.Fatalf("failed text = %q", got)
	}
}

func TestFailedTextKeepsUnrelatedError(t *testing.T) {
	resp := ipc.Response{Tunnels: []ipc.TunnelInfo{{Name: "web", LastError: "ssh: handshake failed"}}}
	got := failedText(command{Target: "web"}, resp)
	if got != `The command failed for "web": ssh: handshake failed` {
		t.Fatalf("failed text = %q", got)
	}
}
