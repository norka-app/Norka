// Package localbind recognizes a local listen that failed because the port is
// already taken, and a one-line hint the UI can translate.
//
// The hint is only for a process that is not another Norka tunnel. A conflict
// with a running Norka tunnel stays the raw listen error so the existing
// "connect instead" flow is unchanged.
package localbind

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

// Prefix marks a stored last error whose first line is machine-readable and
// whose following lines are the original OS error.
const Prefix = "norka:local-port-external"

// Error is a local listen failure caused by an address already in use.
// Remote forwards are not wrapped with it.
type Error struct {
	Err error
}

func (e *Error) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// AnnotateListen wraps a failed local net.Listen. Address-in-use becomes *Error
// so callers can tell it apart from other listen failures. The text still
// starts with "listen <addr> failed:" and includes the OS error.
func AnnotateListen(addr string, err error) error {
	if err == nil {
		return nil
	}
	wrapped := fmt.Errorf("listen %s failed: %w", addr, err)
	if !IsAddrInUse(err) {
		return wrapped
	}
	return &Error{Err: wrapped}
}

// IsAddrInUse reports whether err is a local bind failure for a busy port.
// A remote-forward listen ("remote listen …") is not included: that port is
// on the SSH server, not on this computer.
func IsAddrInUse(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "remote listen") {
		return false
	}
	if errors.Is(err, syscall.EADDRINUSE) {
		return true
	}
	return strings.Contains(msg, "address already in use") ||
		strings.Contains(msg, "only one usage of each socket address") ||
		strings.Contains(msg, "только одно использование адреса сокета") ||
		strings.Contains(msg, "eaddrinuse") ||
		strings.Contains(msg, "wsaeaddrinuse")
}

// ExamplePort is a concrete alternate local port (port+10000, or 13000 when
// that would not fit). It is an example, not a claim that the port was probed.
func ExamplePort(port int) int {
	if port >= 1 && port <= 55535 {
		return port + 10000
	}
	if port != 13000 {
		return 13000
	}
	return 13001
}

// SuggestPort returns a local port that accepted a listen on host just now,
// preferring ExamplePort. If none of the next few ports is free, it returns
// the example anyway. Callers must describe that number as an example.
func SuggestPort(host string, port int) int {
	candidate := ExamplePort(port)
	host = strings.TrimSpace(host)
	if host == "" {
		host = "127.0.0.1"
	}
	for i := 0; i < 32; i++ {
		next := candidate + i
		if next < 1 || next > 65535 || next == port {
			continue
		}
		ln, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(next)))
		if err != nil {
			continue
		}
		_ = ln.Close()
		return next
	}
	return candidate
}

// Format stores the hint parameters and the original OS error.
func Format(port, suggested, remote int, raw string) string {
	return fmt.Sprintf("%s port=%d suggested=%d remote=%d\n%s", Prefix, port, suggested, remote, strings.TrimSpace(raw))
}

var markerPattern = regexp.MustCompile(`(?s)^norka:local-port-external port=(\d+) suggested=(\d+) remote=(\d+)\r?\n(.*)$`)

// Parse splits a stored last error. raw is the original OS text.
func Parse(message string) (port, suggested, remote int, raw string, ok bool) {
	match := markerPattern.FindStringSubmatch(strings.TrimSpace(message))
	if match == nil {
		return 0, 0, 0, "", false
	}
	port, errPort := strconv.Atoi(match[1])
	suggested, errSuggested := strconv.Atoi(match[2])
	remote, errRemote := strconv.Atoi(match[3])
	if errPort != nil || errSuggested != nil || errRemote != nil {
		return 0, 0, 0, "", false
	}
	if port < 1 || port > 65535 || suggested < 1 || suggested > 65535 || remote < 0 || remote > 65535 {
		return 0, 0, 0, "", false
	}
	return port, suggested, remote, strings.TrimSpace(match[4]), true
}

// Present is the English CLI line plus the original OS error.
func Present(detail string) string {
	port, suggested, remote, raw, ok := Parse(detail)
	if !ok {
		return detail
	}
	hint := fmt.Sprintf("Local port %d is already used by another program (often Docker or another service), not by a Norka tunnel. Change the tunnel's local port, for example to %d, and open http://localhost:%d — the remote port can stay %d.", port, suggested, suggested, remote)
	if raw == "" {
		return hint
	}
	return hint + "\n" + raw
}
