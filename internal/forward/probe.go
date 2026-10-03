package forward

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/norka-app/Norka/internal/model"

	"golang.org/x/crypto/ssh"
)

// ErrPasswordNotProbed means a hop uses password authentication.
// ProbeChain returns it before the password is sent.
var ErrPasswordNotProbed = errors.New("password authentication is not probed")

// ChainProbe is a key or agent login plus an optional forwarded TCP dial.
// Latency is one keepalive round trip and stays zero when that request fails.
type ChainProbe struct {
	Latency       time.Duration
	KeepaliveErr  error
	TargetChecked bool
	TargetErr     error
}

// ProbeChain logs in with public-key or agent authentication. When targetHost
// is set, it dials that address through the session. A password hop is rejected
// before any credential is sent.
func ProbeChain(ctx context.Context, jumpers []model.Jumper, targetHost string, targetPort int) (ChainProbe, error) {
	if err := ctx.Err(); err != nil {
		return ChainProbe{}, err
	}
	for i, hop := range jumpers {
		if strings.TrimSpace(hop.AuthType) == "password" {
			return ChainProbe{}, fmt.Errorf("hop %d: %w", i+1, ErrPasswordNotProbed)
		}
	}
	client, closeChain, err := dialSSHChainContext(ctx, jumpers)
	if err != nil {
		return ChainProbe{}, err
	}
	defer closeChain()

	var result ChainProbe
	start := time.Now()
	if _, _, kerr := client.SendRequest("keepalive@openssh.com", true, nil); kerr != nil {
		result.KeepaliveErr = kerr
	} else {
		result.Latency = time.Since(start)
	}

	host := strings.TrimSpace(targetHost)
	if host == "" || targetPort <= 0 {
		return result, nil
	}
	result.TargetChecked = true
	if err := ctx.Err(); err != nil {
		result.TargetErr = err
		return result, nil
	}
	timeout := dialTimeoutFromJumpers(jumpers)
	if timeout > 4*time.Second {
		timeout = 4 * time.Second
	}
	result.TargetErr = probeRemoteDial(client, host, targetPort, timeout)
	return result, nil
}

// ResolveKeyPath expands a configured private-key path. It does not read the file.
func ResolveKeyPath(path string) (string, error) {
	return resolveKeyPath(path)
}

// TestJumperLatency measures pure SSH channel round-trip latency via keepalive.
func TestJumperLatency(client *ssh.Client) (time.Duration, error) {
	if client == nil {
		return 0, fmt.Errorf("ssh client is nil")
	}

	start := time.Now()
	_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
	if err != nil {
		return 0, err
	}
	return time.Since(start), nil
}

// TestJumperConnection verifies SSH handshake/auth against the jumper.
func TestJumperConnection(jumper model.Jumper) error {
	client, err := dialSSH(jumper)
	if err != nil {
		return err
	}
	return client.Close()
}

// TestTunnelConnection verifies tunnel prerequisites and target reachability.
// Currently it supports "local", "remote" and "dynamic" modes only.
func TestTunnelConnection(tunnel model.Tunnel, jumpers []model.Jumper) (time.Duration, error) {
	mode := strings.TrimSpace(tunnel.Mode)
	if mode == "" {
		mode = "local"
	}
	if mode != "local" && mode != "remote" && mode != "dynamic" {
		return 0, fmt.Errorf("mode %s test is not supported yet", mode)
	}

	if mode == "local" || mode == "dynamic" {
		localHost := strings.TrimSpace(tunnel.LocalHost)
		if localHost == "" {
			localHost = "127.0.0.1"
		}
		localAddr := net.JoinHostPort(localHost, strconv.Itoa(tunnel.LocalPort))
		ln, err := net.Listen("tcp", localAddr)
		if err != nil {
			return 0, fmt.Errorf("local listen %s failed: %w", localAddr, err)
		}
		_ = ln.Close()
	}

	client, closeChain, err := dialSSHChain(jumpers)
	if err != nil {
		return 0, err
	}
	defer closeChain()

	latency, err := TestJumperLatency(client)
	if err != nil {
		return 0, fmt.Errorf("measure ssh latency failed: %w", err)
	}

	if mode == "dynamic" {
		if err := probeDynamicForwardCapability(client); err != nil {
			return 0, err
		}
		return latency, nil
	}
	if mode == "remote" {
		if err := probeRemoteListen(client, tunnel.RemoteHost, tunnel.RemotePort); err != nil {
			return 0, err
		}
		return latency, nil
	}

	timeout := dialTimeoutFromJumpers(jumpers)
	if err := probeRemoteDial(client, tunnel.RemoteHost, tunnel.RemotePort, timeout); err != nil {
		return 0, err
	}
	return latency, nil
}

func dialTimeoutFromJumpers(jumpers []model.Jumper) time.Duration {
	if len(jumpers) == 0 {
		return 5 * time.Second
	}
	return dialTimeoutFromJumper(jumpers[len(jumpers)-1])
}

func dialTimeoutFromJumper(jumper model.Jumper) time.Duration {
	timeout := time.Duration(jumper.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		return 5 * time.Second
	}
	return timeout
}

func probeRemoteDial(client *ssh.Client, remoteHost string, remotePort int, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	addr := net.JoinHostPort(strings.TrimSpace(remoteHost), strconv.Itoa(remotePort))

	type dialResult struct {
		conn net.Conn
		err  error
	}

	ch := make(chan dialResult, 1)
	go func() {
		conn, err := client.Dial("tcp", addr)
		select {
		case ch <- dialResult{conn: conn, err: err}:
		default:
			if conn != nil {
				_ = conn.Close()
			}
		}
	}()

	select {
	case res := <-ch:
		if res.err != nil {
			return fmt.Errorf("remote dial %s failed: %w", addr, res.err)
		}
		return res.conn.Close()
	case <-time.After(timeout):
		return fmt.Errorf("remote dial %s timed out after %s", addr, timeout)
	}
}

func probeRemoteListen(client *ssh.Client, remoteHost string, remotePort int) error {
	host := strings.TrimSpace(remoteHost)
	if host == "" {
		host = "127.0.0.1"
	}
	addr := net.JoinHostPort(host, strconv.Itoa(remotePort))
	ln, err := client.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("remote listen %s failed: %w", addr, err)
	}
	return ln.Close()
}

func probeDynamicForwardCapability(client *ssh.Client) error {
	// Use a closed local target to detect "forwarding prohibited" without requiring a real endpoint.
	probeAddr := "127.0.0.1:1"
	remoteConn, err := client.Dial("tcp", probeAddr)
	if err == nil {
		return remoteConn.Close()
	}
	if isPortForwardDenied(err) {
		return fmt.Errorf("dynamic forward is not allowed by ssh server: %w", err)
	}
	return nil
}

func isPortForwardDenied(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "administratively prohibited") ||
		strings.Contains(msg, "forwarding disabled") ||
		strings.Contains(msg, "port forwarding disabled")
}
