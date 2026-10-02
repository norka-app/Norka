//go:build !windows

package ipc

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

var listenMu sync.Mutex

// Address is the unix socket for this config file: the runtime dir when
// XDG_RUNTIME_DIR is set, otherwise next to config.toml.
func Address(configPath string) (string, error) {
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		return "", fmt.Errorf("config path is empty")
	}
	return endpoint(filepath.Dir(configPath))
}

func endpoint(configDir string) (string, error) {
	if dir := strings.TrimSpace(os.Getenv("XDG_RUNTIME_DIR")); dir != "" {
		return filepath.Join(dir, "norka", "norka.sock"), nil
	}
	if strings.TrimSpace(configDir) == "" {
		return "", fmt.Errorf("config directory is empty")
	}
	return filepath.Join(configDir, "norka.sock"), nil
}

// Listen creates a user-only unix socket. An existing live socket is left
// alone. A stale socket file is removed. TCP is never used.
func Listen(address string) (net.Listener, error) {
	if address == "" || strings.Contains(address, "://") {
		return nil, fmt.Errorf("invalid ipc address")
	}
	if err := os.MkdirAll(filepath.Dir(address), 0o700); err != nil {
		return nil, err
	}
	if info, err := os.Lstat(address); err == nil {
		mode := info.Mode()
		if mode&os.ModeSocket == 0 && mode&os.ModeSymlink == 0 {
			return nil, fmt.Errorf("ipc path exists and is not a socket")
		}
		if conn, dialErr := net.DialTimeout("unix", address, 150*time.Millisecond); dialErr == nil {
			_ = conn.Close()
			return nil, fmt.Errorf("automation ipc is already listening")
		}
		if err := os.Remove(address); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	listenMu.Lock()
	defer listenMu.Unlock()
	old := syscall.Umask(0o077)
	ln, err := net.Listen("unix", address)
	syscall.Umask(old)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(address, 0o600); err != nil {
		_ = ln.Close()
		_ = os.Remove(address)
		return nil, err
	}
	return &unixListener{Listener: ln, path: address}, nil
}

type unixListener struct {
	net.Listener
	path string
}

func (l *unixListener) Close() error {
	err := l.Listener.Close()
	if l.path != "" {
		_ = os.Remove(l.path)
	}
	return err
}

func dial(ctx context.Context, address string) (net.Conn, error) {
	var dialer net.Dialer
	return dialer.DialContext(ctx, "unix", address)
}
