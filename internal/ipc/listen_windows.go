//go:build windows

package ipc

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/Microsoft/go-winio"
)

// Address is the per-user named pipe. The name includes the user SID so two
// accounts on one machine do not share a pipe. TCP is never used.
func Address(configPath string) (string, error) {
	if strings.TrimSpace(configPath) == "" {
		return "", fmt.Errorf("config path is empty")
	}
	return endpoint("")
}

func endpoint(string) (string, error) {
	sid, err := currentUserSID()
	if err != nil {
		return "", err
	}
	return `\\.\pipe\norka-` + sid, nil
}

// Listen opens a named pipe whose security descriptor grants access only to
// the current user.
func Listen(address string) (net.Listener, error) {
	if !strings.HasPrefix(address, `\\.\pipe\`) {
		return nil, fmt.Errorf("invalid pipe address")
	}
	sid, err := currentUserSID()
	if err != nil {
		return nil, err
	}
	return winio.ListenPipe(address, &winio.PipeConfig{
		SecurityDescriptor: "D:P(A;;GA;;;" + sid + ")",
		InputBufferSize:    1 << 16,
		OutputBufferSize:   1 << 16,
	})
}

func dial(ctx context.Context, address string) (net.Conn, error) {
	return winio.DialPipeContext(ctx, address)
}
