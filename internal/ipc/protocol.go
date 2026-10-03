package ipc

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

const maxMessage = 1 << 20

// ProtocolVersion is the IPC dialect this build speaks.
// MinClientVersion is the oldest numbered dialect still accepted.
// A request that omits v (decoded as 0) is a legacy client and stays accepted,
// so an older norka binary keeps working with a newer app.
const (
	ProtocolVersion  = 1
	MinClientVersion = 1
)

// Operations spoken over the local channel.
const (
	OpConnect    = "connect"
	OpDisconnect = "disconnect"
	OpToggle     = "toggle"
	OpStatus     = "status"
	OpList       = "list"
)

// Result codes. Exit codes are stable and documented in docs/CLI.md.
const (
	CodeOK           = "ok"
	CodeDisabled     = "disabled"
	CodeNotRunning   = "not_running"
	CodeNotFound     = "not_found"
	CodeAmbiguous    = "ambiguous"
	CodeUnauthorized = "unauthorized"
	CodeBadRequest   = "bad_request"
	CodeFailed       = "failed"
	// CodeUpdateApp means the client speaks a newer dialect than this app.
	CodeUpdateApp = "update_app"
	// CodeUpdateCLI means the client is newer than the legacy dialect but older
	// than MinClientVersion.
	CodeUpdateCLI = "update_cli"

	ExitOK         = 0
	ExitDisabled   = 1
	ExitNotRunning = 2
	ExitNotFound   = 3
	ExitAmbiguous  = 4
	ExitUsage      = 5
	ExitIPC        = 6
	ExitFailed     = 7
	// ExitVersion means the client and the app do not speak the same dialect.
	ExitVersion = 8
)

// Sentences norka-cli prints when the dialect does not match.
// The running app returns the same text on the channel.
const (
	MessageUpdateApp = "This norka-cli is newer than Norka. Update Norka."
	MessageUpdateCLI = "This norka-cli is too old for Norka. Update norka-cli."
)

// ErrNotRunning means the local channel is not accepting connections.
var ErrNotRunning = errors.New("norka is not running")

// Request is one command. Token is checked by the server and never logged.
// V is the dialect the client speaks. Omit it (or send 0) for the legacy
// dialect, which this server still accepts.
type Request struct {
	V      int    `json:"v,omitempty"`
	Token  string `json:"token"`
	Op     string `json:"op"`
	Target string `json:"target,omitempty"`
}

// TunnelInfo is the public view of one tunnel. It has no secrets.
type TunnelInfo struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Mode       string `json:"mode"`
	LocalHost  string `json:"localHost,omitempty"`
	LocalPort  int    `json:"localPort,omitempty"`
	RemoteHost string `json:"remoteHost,omitempty"`
	RemotePort int    `json:"remotePort,omitempty"`
	LastError  string `json:"lastError,omitempty"`
}

// Response is one reply. Running is false when this process could not reach Norka.
// V is the dialect this app speaks. A reply without v is an older Norka.
type Response struct {
	V        int          `json:"v,omitempty"`
	OK       bool         `json:"ok"`
	Code     string       `json:"code"`
	Message  string       `json:"message,omitempty"`
	ExitCode int          `json:"exitCode"`
	Running  bool         `json:"running"`
	Tunnels  []TunnelInfo `json:"tunnels,omitempty"`
	Names    []string     `json:"names,omitempty"`
}

// Handler runs one authenticated request.
type Handler func(Request) Response

// Serve accepts connections until ctx is cancelled or ln is closed.
// It refuses any listener that is not a unix socket or a Windows named pipe.
func Serve(ctx context.Context, ln net.Listener, token string, handle Handler) error {
	if ln == nil {
		return errors.New("listener is nil")
	}
	network := ""
	if ln.Addr() != nil {
		network = ln.Addr().Network()
	}
	switch network {
	case "unix", "unixgram", "unixpacket", "pipe":
	default:
		_ = ln.Close()
		return fmt.Errorf("refusing to listen on %s", network)
	}
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go serveConn(conn, token, handle)
	}
}

func serveConn(conn net.Conn, token string, handle Handler) {
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var req Request
	if err := readJSON(conn, &req); err != nil {
		return
	}
	_ = conn.SetDeadline(time.Time{})
	resp := Response{Code: CodeBadRequest, ExitCode: ExitIPC, Message: "bad request"}
	version := GateVersion(req.V)
	switch {
	case !tokenEqual(req.Token, token):
		resp = Response{Code: CodeUnauthorized, ExitCode: ExitIPC, Message: "unauthorized"}
	case version.Reject:
		resp = version.Response
	case handle != nil:
		resp = handle(req)
	}
	resp.V = ProtocolVersion
	resp.Running = true
	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_ = writeJSON(conn, resp)
}

func tokenEqual(got, want string) bool {
	if want == "" {
		return false
	}
	sumGot := sha256.Sum256([]byte(got))
	sumWant := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(sumGot[:], sumWant[:]) == 1
}

func writeJSON(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

func readJSON(r io.Reader, v any) error {
	line, err := bufio.NewReader(io.LimitReader(r, maxMessage)).ReadBytes('\n')
	if err != nil {
		return err
	}
	return json.Unmarshal(line, v)
}
