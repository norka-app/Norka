package localbind

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"
)

func TestAnnotateListenMarksBusyPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	addr := ln.Addr().String()

	_, listenErr := net.Listen("tcp", addr)
	if listenErr == nil {
		t.Fatal("second listen succeeded")
	}
	if !IsAddrInUse(listenErr) {
		t.Fatalf("IsAddrInUse(%q) = false", listenErr)
	}
	annotated := AnnotateListen(addr, listenErr)
	var marked *Error
	if !errors.As(annotated, &marked) {
		t.Fatalf("annotated type %T, want *Error", annotated)
	}
	text := annotated.Error()
	if !strings.Contains(text, "listen "+addr+" failed:") || !IsAddrInUse(annotated) {
		t.Fatalf("annotated = %q", text)
	}
}

func TestIsAddrInUseStrings(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"listen tcp 127.0.0.1:3000: bind: address already in use", true},
		{"listen tcp 127.0.0.1:3000: bind: Only one usage of each socket address (protocol/network address/port) is normally permitted.", true},
		{"listen tcp 127.0.0.1:3000: bind: Обычно разрешается только одно использование адреса сокета (протокол/сетевой адрес/порт).", true},
		{"bind: WSAEADDRINUSE", true},
		{"remote listen 0.0.0.0:80 failed: bind: address already in use", false},
		{"dial tcp 10.0.0.1:22: connect: connection refused", false},
		{"", false},
	}
	for _, c := range cases {
		got := IsAddrInUse(errors.New(c.msg))
		if got != c.want {
			t.Errorf("IsAddrInUse(%q) = %v, want %v", c.msg, got, c.want)
		}
	}
	if IsAddrInUse(nil) {
		t.Fatal("nil error was in use")
	}
}

func TestExamplePort(t *testing.T) {
	if got := ExamplePort(3000); got != 13000 {
		t.Fatalf("3000 -> %d, want 13000", got)
	}
	if got := ExamplePort(60000); got != 13000 {
		t.Fatalf("60000 -> %d, want 13000", got)
	}
	if got := ExamplePort(13000); got != 23000 {
		t.Fatalf("13000 -> %d, want 23000", got)
	}
}

func TestSuggestPortSkipsBusyExample(t *testing.T) {
	var occupied net.Listener
	base := 0
	for candidate := 25000; candidate < 27000; candidate += 40 {
		ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(candidate+10000)))
		if err != nil {
			continue
		}
		occupied = ln
		base = candidate
		break
	}
	if occupied == nil {
		t.Fatal("no free port pair")
	}
	defer occupied.Close()

	got := SuggestPort("127.0.0.1", base)
	if got == base+10000 {
		t.Fatalf("suggested the busy example port %d", got)
	}
	if got < 1 || got > 65535 || got == base {
		t.Fatalf("suggested %d", got)
	}
}

func TestFormatParsePresent(t *testing.T) {
	raw := "listen 127.0.0.1:3000 failed: listen tcp 127.0.0.1:3000: bind: Only one usage of each socket address (protocol/network address/port) is normally permitted."
	stored := Format(3000, 13000, 8080, raw)
	port, suggested, remote, gotRaw, ok := Parse(stored)
	if !ok || port != 3000 || suggested != 13000 || remote != 8080 || gotRaw != raw {
		t.Fatalf("parse %#v -> %d %d %d %q %v", stored, port, suggested, remote, gotRaw, ok)
	}
	shown := Present(stored)
	if !strings.Contains(shown, "Local port 3000 is already used by another program") ||
		!strings.Contains(shown, "for example to 13000") ||
		!strings.Contains(shown, "http://localhost:13000") ||
		!strings.Contains(shown, "remote port can stay 8080") ||
		!strings.Contains(shown, raw) ||
		!strings.HasPrefix(shown, "Local port") {
		t.Fatalf("present = %q", shown)
	}
	if strings.Contains(shown, Prefix) {
		t.Fatalf("present leaked the marker: %q", shown)
	}
	if Present("dial tcp: connection refused") != "dial tcp: connection refused" {
		t.Fatal("unrelated error was rewritten")
	}
}
