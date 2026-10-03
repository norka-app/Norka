package forward

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/norka-app/Norka/internal/model"
	"github.com/norka-app/Norka/internal/netwatch"
	"github.com/norka-app/Norka/internal/wake"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

var ErrUnsupportedMode = errors.New("only local, remote and dynamic modes are supported")

const (
	initReconnectWait = 500 * time.Millisecond
	maxReconnectWait  = 1 * time.Minute
	reconnectTimeout  = wake.GiveUpWindow
)

var (
	sshAgentMu   sync.Mutex
	sshAgentInst agent.ExtendedAgent
	sshAgentSock string
	sshAgentConn net.Conn
)

type RuntimeEventType string

const (
	RuntimeEventDisconnected RuntimeEventType = "disconnected"
	RuntimeEventReconnected  RuntimeEventType = "reconnected"
)

type RuntimeEvent struct {
	Type RuntimeEventType
	Err  error
	// Quiet suppresses the per-tunnel notification. A wake reconnect posts one
	// summary instead of one notice per tunnel.
	Quiet bool
}

type LocalForward struct {
	tunnel  model.Tunnel
	jumpers []model.Jumper

	mu          sync.Mutex
	started     bool
	stopping    bool
	runErr      error
	client      *ssh.Client
	clientClose func()
	listener    net.Listener
	done        chan struct{}
	events      chan RuntimeEvent
	keepStop    chan struct{}
	nudgeCh     chan struct{}
	immediate   atomic.Bool
	quietCycle  atomic.Bool
	lastLatency time.Duration
	bytesUp     atomic.Uint64
	bytesDown   atomic.Uint64
	stopOnce    sync.Once
	wg          sync.WaitGroup
	startCtx    context.Context
}

func NewLocalForward(tunnel model.Tunnel, jumpers []model.Jumper) *LocalForward {
	return &LocalForward{
		tunnel:  tunnel,
		jumpers: append([]model.Jumper{}, jumpers...),
	}
}

// SetStartContext cancels the initial dial when ctx is cancelled. The running
// tunnel does not keep this context after Start returns.
func (f *LocalForward) SetStartContext(ctx context.Context) {
	f.startCtx = ctx
}

func (f *LocalForward) Start() error {
	ctx := f.startCtx
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	mode := normalizeForwardMode(f.tunnel.Mode)
	if mode != "local" && mode != "remote" && mode != "dynamic" {
		return ErrUnsupportedMode
	}

	f.mu.Lock()
	if f.started {
		f.mu.Unlock()
		return nil
	}
	f.started = true
	f.runErr = nil
	f.done = make(chan struct{})
	f.events = make(chan RuntimeEvent, 8)
	f.keepStop = make(chan struct{})
	f.nudgeCh = make(chan struct{}, 1)
	f.mu.Unlock()

	slog.Info(
		"tunnel forward start",
		"tunnel_id", f.tunnel.ID,
		"name", f.tunnel.Name,
		"jumper_hops", len(f.jumpers),
		"keepalive_interval_ms", f.lastJumper().KeepAliveIntervalMs,
		"timeout_ms", f.lastJumper().TimeoutMs,
	)

	client, closeChain, err := dialSSHChainContext(ctx, f.jumpers)
	if err != nil {
		f.setRunErr(err)
		slog.Error("tunnel initial dial failed", "tunnel_id", f.tunnel.ID, "name", f.tunnel.Name, "err", err)
		return err
	}
	if err := ctx.Err(); err != nil {
		closeChain()
		return err
	}
	if mode == "local" {
		probeTimeout := dialTimeoutFromJumper(f.lastJumper())
		if err := probeRemoteDial(client, f.tunnel.RemoteHost, f.tunnel.RemotePort, probeTimeout); err != nil {
			closeChain()
			f.setRunErr(err)
			slog.Error(
				"tunnel initial remote probe failed",
				"tunnel_id",
				f.tunnel.ID, "name", f.tunnel.Name, "err", err)
			return err
		}
	} else if mode == "dynamic" {
		if err := probeDynamicForwardCapability(client); err != nil {
			closeChain()
			f.setRunErr(err)
			slog.Error("tunnel dynamic probe failed", "tunnel_id", f.tunnel.ID, "name", f.tunnel.Name, "err", err)
			return err
		}
	}

	var ln net.Listener
	if mode == "remote" {
		ln, err = f.bindRemoteListener(client)
		if err != nil {
			closeChain()
			f.setRunErr(err)
			slog.Error("tunnel remote listen failed", "tunnel_id", f.tunnel.ID, "name", f.tunnel.Name, "err", err)
			return err
		}
	} else {
		localHost := strings.TrimSpace(f.tunnel.LocalHost)
		if localHost == "" {
			localHost = "127.0.0.1"
		}
		localAddr := net.JoinHostPort(localHost, strconv.Itoa(f.tunnel.LocalPort))
		ln, err = net.Listen("tcp", localAddr)
		if err != nil {
			closeChain()
			runErr := fmt.Errorf("listen %s failed: %w", localAddr, err)
			f.setRunErr(runErr)
			slog.Error("tunnel listen failed", "tunnel_id", f.tunnel.ID, "name", f.tunnel.Name, "addr", localAddr, "err", runErr)
			return runErr
		}
	}

	f.mu.Lock()
	f.client = client
	f.clientClose = closeChain
	f.listener = ln
	f.lastLatency = 0
	done := f.done
	f.mu.Unlock()

	if latency, latencyErr := TestJumperLatency(client); latencyErr == nil {
		f.setLastLatency(latency)
	}

	if mode == "remote" {
		go f.serveRemote(done)
	} else {
		go f.serveLocal(done)
	}
	go f.monitorClientLifecycle(client)
	return nil
}

func (f *LocalForward) Stop() error {
	f.stopOnce.Do(func() {
		slog.Info("tunnel forward stop requested", "tunnel_id", f.tunnel.ID, "name", f.tunnel.Name)
		f.mu.Lock()
		f.stopping = true
		ln := f.listener
		c := f.client
		closeClient := f.clientClose
		done := f.done
		keepStop := f.keepStop
		f.listener = nil
		f.client = nil
		f.clientClose = nil
		f.mu.Unlock()

		if keepStop != nil {
			close(keepStop)
		}
		if closeClient != nil {
			closeClient()
		} else if c != nil {
			_ = c.Close()
		}
		if ln != nil {
			_ = ln.Close()
		}
		if done != nil {
			<-done
		}
		f.wg.Wait()
	})
	return nil
}

func (f *LocalForward) Done() <-chan struct{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.done
}

func (f *LocalForward) Events() <-chan RuntimeEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.events
}

func (f *LocalForward) Err() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.runErr
}

func (f *LocalForward) LastLatency() (time.Duration, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lastLatency <= 0 {
		return 0, false
	}
	return f.lastLatency, true
}

func (f *LocalForward) Traffic() (up, down uint64) {
	return f.bytesUp.Load(), f.bytesDown.Load()
}

func (f *LocalForward) serveLocal(done chan struct{}) {
	defer close(done)

	for {
		f.mu.Lock()
		ln := f.listener
		f.mu.Unlock()
		if ln == nil {
			return
		}

		conn, err := ln.Accept()
		if err != nil {
			if f.isStopping() {
				return
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				continue
			}
			if f.Err() == nil {
				f.setRunErr(fmt.Errorf("accept failed: %w", err))
			}
			return
		}

		f.wg.Add(1)
		go func(localConn net.Conn) {
			defer f.wg.Done()
			f.handleConn(localConn)
		}(conn)
	}
}

func (f *LocalForward) serveRemote(done chan struct{}) {
	defer close(done)

	for {
		if f.isStopping() {
			return
		}

		f.mu.Lock()
		ln := f.listener
		f.mu.Unlock()
		if ln == nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		conn, err := ln.Accept()
		if err != nil {
			if f.isStopping() {
				return
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				continue
			}
			time.Sleep(200 * time.Millisecond)
			continue
		}

		f.wg.Add(1)
		go func(remoteConn net.Conn) {
			defer f.wg.Done()
			f.handleConn(remoteConn)
		}(conn)
	}
}

func (f *LocalForward) handleConn(localConn net.Conn) {
	f.mu.Lock()
	client := f.client
	f.mu.Unlock()

	if client == nil {
		_ = localConn.Close()
		return
	}

	if normalizeForwardMode(f.tunnel.Mode) == "dynamic" {
		f.handleDynamicConn(localConn, client)
		return
	}
	if normalizeForwardMode(f.tunnel.Mode) == "remote" {
		f.handleRemoteConn(localConn)
		return
	}
	f.handleLocalConn(localConn, client)
}

func (f *LocalForward) handleLocalConn(localConn net.Conn, client *ssh.Client) {
	remoteAddr := net.JoinHostPort(strings.TrimSpace(f.tunnel.RemoteHost), strconv.Itoa(f.tunnel.RemotePort))
	remoteConn, err := client.Dial("tcp", remoteAddr)
	if err != nil {
		_ = localConn.Close()
		return
	}

	f.bridge(localConn, remoteConn)
}

func (f *LocalForward) handleDynamicConn(localConn net.Conn, client *ssh.Client) {
	targetAddr, err := readSOCKS5ConnectTarget(localConn)
	if err != nil {
		_ = localConn.Close()
		return
	}

	remoteConn, err := client.Dial("tcp", targetAddr)
	if err != nil {
		_ = writeSOCKS5Reply(localConn, socksReplyGeneralFailure)
		_ = localConn.Close()
		return
	}

	if err := writeSOCKS5Reply(localConn, socksReplySucceeded); err != nil {
		_ = remoteConn.Close()
		_ = localConn.Close()
		return
	}

	f.bridge(localConn, remoteConn)
}

func (f *LocalForward) handleRemoteConn(remoteConn net.Conn) {
	localHost := strings.TrimSpace(f.tunnel.LocalHost)
	if localHost == "" {
		localHost = "127.0.0.1"
	}
	localAddr := net.JoinHostPort(localHost, strconv.Itoa(f.tunnel.LocalPort))
	localConn, err := net.Dial("tcp", localAddr)
	if err != nil {
		_ = remoteConn.Close()
		return
	}
	f.bridge(localConn, remoteConn)
}

func (f *LocalForward) monitorClientLifecycle(client *ssh.Client) {
	defer f.closeEvents()

	for {
		disconnectErr := f.waitClientLoss(client)
		if disconnectErr == nil || f.isStopping() {
			return
		}

		slog.Warn("tunnel connection lost", "tunnel_id", f.tunnel.ID, "name", f.tunnel.Name, "err", disconnectErr)
		f.emitEvent(RuntimeEvent{
			Type:  RuntimeEventDisconnected,
			Err:   disconnectErr,
			Quiet: f.quietCycle.Load(),
		})
		f.setClient(nil, nil)

		reconnectedClient, reconnectClose, reconnectErr := f.reconnectWithBackoff()
		if reconnectErr != nil {
			f.setRunErr(fmt.Errorf("%v: %w", disconnectErr, reconnectErr))
			slog.Error("tunnel reconnect failed", "tunnel_id", f.tunnel.ID, "name", f.tunnel.Name, "err", reconnectErr)
			f.closeListener()
			return
		}
		if reconnectedClient == nil {
			return
		}
		f.emitEvent(RuntimeEvent{
			Type:  RuntimeEventReconnected,
			Quiet: f.quietCycle.Swap(false),
		})
		slog.Info("tunnel reconnected", "tunnel_id", f.tunnel.ID, "name", f.tunnel.Name)
		f.setClient(reconnectedClient, reconnectClose)
		client = reconnectedClient
	}
}

func (f *LocalForward) waitClientLoss(client *ssh.Client) error {
	if client == nil {
		return nil
	}
	stop := f.stopSignal()
	if stop == nil {
		return nil
	}

	lost := make(chan error, 1)
	clientDone := make(chan struct{})
	var once sync.Once

	report := func(err error) {
		if err == nil {
			err = errors.New("ssh connection closed")
		}
		once.Do(func() {
			select {
			case lost <- err:
			default:
			}
		})
	}

	go func() {
		err := client.Wait()
		if err != nil {
			report(fmt.Errorf("ssh connection closed: %w", err))
			return
		}
		report(nil)
	}()

	interval := keepAliveInterval(f.lastJumper())
	if interval > 0 {
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			if !f.runKeepAliveProbe(client, stop, clientDone, interval, report) {
				return
			}
			for {
				select {
				case <-clientDone:
					return
				case <-stop:
					return
				case <-ticker.C:
					if !f.runKeepAliveProbe(client, stop, clientDone, interval, report) {
						return
					}
				}
			}
		}()
	}

	select {
	case err := <-lost:
		close(clientDone)
		return err
	case <-stop:
		close(clientDone)
		return nil
	}
}

func (f *LocalForward) reconnectWithBackoff() (*ssh.Client, func(), error) {
	stop := f.stopSignal()
	if stop == nil {
		return nil, nil, nil
	}

	budget := reconnectTimeout
	wait := initReconnectWait
	if f.consumeImmediate() {
		f.drainNudge()
		wait = 0
		budget = reconnectTimeout
	}
	var lastErr error
	attempt := 0

	for {
		if f.consumeImmediate() {
			f.drainNudge()
			wait = 0
			budget = reconnectTimeout
		}
		if budget <= 0 {
			break
		}

		scheduled := minDuration(wait, budget)
		started := time.Now()
		switch f.waitOrWake(scheduled, stop) {
		case waitStopped:
			return nil, nil, nil
		case waitWoke:
			f.immediate.Store(false)
			f.drainNudge()
			wait = 0
			budget = reconnectTimeout
		default:
			elapsed := time.Since(started)
			budget = wake.ChargeAwake(budget, reconnectTimeout, elapsed, scheduled, netwatch.JumpGap)
			if elapsed > scheduled+netwatch.JumpGap {
				wait = 0
			}
			if budget <= 0 {
				break
			}
		}
		attempt++
		slog.Info("tunnel reconnect attempt", "tunnel_id", f.tunnel.ID, "name", f.tunnel.Name, "attempt", attempt, "wait", wait.String())

		client, closeChain, err := dialSSHChain(f.jumpers)
		if err == nil {
			if normalizeForwardMode(f.tunnel.Mode) == "remote" {
				ln, listenErr := f.bindRemoteListener(client)
				if listenErr != nil {
					closeChain()
					lastErr = listenErr
					slog.Warn("tunnel remote listen rebind failed", "tunnel_id", f.tunnel.ID, "name", f.tunnel.Name, "attempt", attempt, "err", listenErr)
					wait = nextReconnectWait(wait)
					continue
				}
				f.replaceListener(ln)
			}
			slog.Info("tunnel reconnect succeeded", "tunnel_id", f.tunnel.ID, "name", f.tunnel.Name, "attempt", attempt)
			if f.immediate.Load() {
				// The network changed again while this dial was in flight.
				closeChain()
				f.drainNudge()
				wait = 0
				budget = reconnectTimeout
				continue
			}
			return client, closeChain, nil
		}

		lastErr = err
		slog.Warn("tunnel reconnect failed", "tunnel_id", f.tunnel.ID, "name", f.tunnel.Name, "attempt", attempt, "err", err)
		wait = nextReconnectWait(wait)
	}

	if lastErr == nil {
		lastErr = errors.New("reconnect timeout")
	}
	return nil, nil, fmt.Errorf("reconnect timeout after %s: %w", reconnectTimeout, lastErr)
}

func nextReconnectWait(current time.Duration) time.Duration {
	if current <= 0 {
		return initReconnectWait
	}
	next := current * 2
	if next > maxReconnectWait {
		return maxReconnectWait
	}
	return next
}

func minDuration(a, b time.Duration) time.Duration {
	if a <= b {
		return a
	}
	return b
}

func keepAliveInterval(jumper model.Jumper) time.Duration {
	interval := time.Duration(jumper.KeepAliveIntervalMs) * time.Millisecond
	if interval > 0 {
		return interval
	}
	return 0
}

func (f *LocalForward) lastJumper() model.Jumper {
	if len(f.jumpers) == 0 {
		return model.Jumper{}
	}
	return f.jumpers[len(f.jumpers)-1]
}

func keepAliveRequestTimeout(interval time.Duration) time.Duration {
	if interval <= 0 {
		return 5 * time.Second
	}
	timeout := interval / 2
	if timeout < 5*time.Second {
		timeout = 5 * time.Second
	}
	if timeout > 10*time.Second {
		timeout = 10 * time.Second
	}
	return timeout
}

func (f *LocalForward) runKeepAliveProbe(
	client *ssh.Client,
	stop <-chan struct{},
	clientDone <-chan struct{},
	interval time.Duration,
	report func(err error),
) bool {
	timeout := keepAliveRequestTimeout(interval)
	type keepAliveResult struct {
		latency time.Duration
		err     error
	}
	result := make(chan keepAliveResult, 1)

	go func() {
		latency, err := TestJumperLatency(client)
		result <- keepAliveResult{latency: latency, err: err}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-clientDone:
		return false
	case <-stop:
		return false
	case probe := <-result:
		if probe.err == nil {
			f.setLastLatency(probe.latency)
			slog.Debug("tunnel keepalive probe ok", "tunnel_id", f.tunnel.ID, "name", f.tunnel.Name)
			return true
		}
		if f.isStopping() {
			return false
		}
		report(fmt.Errorf("keepalive failed: %w", probe.err))
		slog.Warn("tunnel keepalive failed", "tunnel_id", f.tunnel.ID, "name", f.tunnel.Name, "err", probe.err)
		_ = client.Close()
		return false
	case <-timer.C:
		if f.isStopping() {
			return false
		}
		timeoutErr := fmt.Errorf("keepalive timeout after %s", timeout)
		report(timeoutErr)
		slog.Warn("tunnel keepalive timeout", "tunnel_id", f.tunnel.ID, "name", f.tunnel.Name, "timeout", timeout.String())
		_ = client.Close()
		return false
	}
}

func (f *LocalForward) stopSignal() <-chan struct{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.keepStop
}

func (f *LocalForward) setClient(client *ssh.Client, closeFn func()) {
	f.mu.Lock()
	oldClose := f.clientClose
	defer f.mu.Unlock()
	f.client = client
	f.clientClose = closeFn
	f.lastLatency = 0
	if oldClose != nil {
		go oldClose()
	}
}

func (f *LocalForward) setLastLatency(latency time.Duration) {
	if latency <= 0 {
		return
	}
	f.mu.Lock()
	f.lastLatency = latency
	f.mu.Unlock()
}

func (f *LocalForward) replaceListener(listener net.Listener) {
	f.mu.Lock()
	old := f.listener
	f.listener = listener
	f.mu.Unlock()
	if old != nil && old != listener {
		_ = old.Close()
	}
}

func (f *LocalForward) closeListener() {
	f.mu.Lock()
	ln := f.listener
	f.listener = nil
	f.mu.Unlock()
	if ln != nil {
		_ = ln.Close()
	}
}

func (f *LocalForward) emitEvent(event RuntimeEvent) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.events == nil {
		return
	}
	select {
	case f.events <- event:
	default:
	}
}

func (f *LocalForward) closeEvents() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.events == nil {
		return
	}
	close(f.events)
	f.events = nil
}

func (f *LocalForward) isStopping() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stopping
}

func (f *LocalForward) setRunErr(err error) {
	if err == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.runErr == nil {
		f.runErr = err
	}
}

func normalizeForwardMode(raw string) string {
	mode := strings.TrimSpace(raw)
	if mode == "" {
		return "local"
	}
	return mode
}

func (f *LocalForward) bindRemoteListener(client *ssh.Client) (net.Listener, error) {
	host := strings.TrimSpace(f.tunnel.RemoteHost)
	if host == "" {
		host = "127.0.0.1"
	}
	addr := net.JoinHostPort(host, strconv.Itoa(f.tunnel.RemotePort))
	ln, err := client.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("remote listen %s failed: %w", addr, err)
	}
	return ln, nil
}

type countingConnReader struct {
	net.Conn
	counter *atomic.Uint64
}

func (c *countingConnReader) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 {
		c.counter.Add(uint64(n))
	}
	return n, err
}

func (f *LocalForward) bridge(local, remote net.Conn) {
	defer local.Close()
	defer remote.Close()

	done := make(chan struct{}, 2)

	go func() {
		_, _ = io.Copy(remote, &countingConnReader{Conn: local, counter: &f.bytesUp})
		done <- struct{}{}
	}()

	go func() {
		_, _ = io.Copy(local, &countingConnReader{Conn: remote, counter: &f.bytesDown})
		done <- struct{}{}
	}()

	<-done
}

// Bridge exposes connection bridging for integration tests.
func (f *LocalForward) Bridge(local, remote net.Conn) {
	f.bridge(local, remote)
}

func dialSSH(jumper model.Jumper) (*ssh.Client, error) {
	return dialSSHContext(context.Background(), jumper)
}

func dialSSHContext(ctx context.Context, jumper model.Jumper) (*ssh.Client, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	conf, err := makeSSHClientConfig(jumper)
	if err != nil {
		return nil, err
	}

	host := strings.TrimSpace(jumper.Host)
	if host == "" {
		return nil, fmt.Errorf("jumper host is required")
	}
	port := jumper.Port
	if port <= 0 {
		port = 22
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	timeout := conf.Timeout
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, fmt.Errorf("ssh dial %s failed: %w", addr, err)
	}

	type handshakeResult struct {
		client *ssh.Client
		err    error
	}
	done := make(chan handshakeResult, 1)
	go func() {
		cconn, chans, reqs, err := ssh.NewClientConn(conn, addr, conf)
		if err != nil {
			_ = conn.Close()
			done <- handshakeResult{err: err}
			return
		}
		done <- handshakeResult{client: ssh.NewClient(cconn, chans, reqs)}
	}()

	select {
	case <-ctx.Done():
		_ = conn.Close()
		return nil, ctx.Err()
	case res := <-done:
		if err := ctx.Err(); err != nil {
			if res.client != nil {
				_ = res.client.Close()
			}
			return nil, err
		}
		if res.err != nil {
			if errors.Is(res.err, context.Canceled) || errors.Is(res.err, context.DeadlineExceeded) {
				return nil, res.err
			}
			return nil, fmt.Errorf("ssh dial %s failed: %w", addr, res.err)
		}
		return res.client, nil
	}
}

func dialThroughContext(ctx context.Context, client *ssh.Client, addr string) (net.Conn, error) {
	type dialResult struct {
		conn net.Conn
		err  error
	}
	done := make(chan dialResult, 1)
	go func() {
		conn, err := client.Dial("tcp", addr)
		done <- dialResult{conn: conn, err: err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-done:
		if err := ctx.Err(); err != nil {
			if res.conn != nil {
				_ = res.conn.Close()
			}
			return nil, err
		}
		return res.conn, res.err
	}
}

// ExecuteRemoteCommand runs one shell command on the last hop of an SSH chain.
func ExecuteRemoteCommand(jumpers []model.Jumper, command string) (string, string, error) {
	if strings.TrimSpace(command) == "" {
		return "", "", fmt.Errorf("command is required")
	}

	client, closeChain, err := dialSSHChain(jumpers)
	if err != nil {
		return "", "", err
	}
	defer closeChain()

	session, err := client.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("create ssh session failed: %w", err)
	}
	defer session.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(command); err != nil {
		return stdout.String(), stderr.String(), err
	}
	return stdout.String(), stderr.String(), nil
}

func dialSSHChain(jumpers []model.Jumper) (*ssh.Client, func(), error) {
	return dialSSHChainContext(context.Background(), jumpers)
}

func dialSSHChainContext(ctx context.Context, jumpers []model.Jumper) (*ssh.Client, func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if len(jumpers) == 0 {
		return nil, nil, fmt.Errorf("at least one jumper is required")
	}

	clients := make([]*ssh.Client, 0, len(jumpers))
	first, err := dialSSHContext(ctx, jumpers[0])
	if err != nil {
		return nil, nil, err
	}
	clients = append(clients, first)
	current := first

	closeOnce := sync.Once{}
	closeAll := func() {
		closeOnce.Do(func() {
			for i := len(clients) - 1; i >= 0; i-- {
				_ = clients[i].Close()
			}
		})
	}

	for i := 1; i < len(jumpers); i++ {
		next := jumpers[i]
		conf, err := makeSSHClientConfig(next)
		if err != nil {
			closeAll()
			return nil, nil, err
		}

		host := strings.TrimSpace(next.Host)
		if host == "" {
			closeAll()
			return nil, nil, fmt.Errorf("jumper[%d] host is required", i)
		}
		port := next.Port
		if port <= 0 {
			port = 22
		}
		addr := net.JoinHostPort(host, strconv.Itoa(port))

		conn, err := dialThroughContext(ctx, current, addr)
		if err != nil {
			closeAll()
			return nil, nil, fmt.Errorf("ssh dial %s via hop %d failed: %w", addr, i, err)
		}
		if err := ctx.Err(); err != nil {
			_ = conn.Close()
			closeAll()
			return nil, nil, err
		}

		cconn, chans, reqs, err := ssh.NewClientConn(conn, addr, conf)
		if err != nil {
			_ = conn.Close()
			closeAll()
			return nil, nil, fmt.Errorf("ssh handshake %s via hop %d failed: %w", addr, i, err)
		}

		client := ssh.NewClient(cconn, chans, reqs)
		clients = append(clients, client)
		current = client
	}

	return current, closeAll, nil
}

func makeSSHClientConfig(jumper model.Jumper) (*ssh.ClientConfig, error) {
	user := strings.TrimSpace(jumper.User)
	if user == "" {
		return nil, fmt.Errorf("jumper user is required")
	}

	auth, err := makeAuthMethod(jumper)
	if err != nil {
		return nil, err
	}

	cb, err := makeHostKeyCallback(jumper.BypassHostVerification)
	if err != nil {
		return nil, err
	}

	timeout := time.Duration(jumper.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: cb,
		Timeout:         timeout,
	}

	// Поддержка HostKeyAlgorithms
	if strings.TrimSpace(jumper.HostKeyAlgorithms) != "" {
		config.HostKeyAlgorithms = parseHostKeyAlgorithms(jumper.HostKeyAlgorithms)
	} else if !jumper.BypassHostVerification {
		// Go asks for ECDSA and RSA before ed25519. OpenSSH asks for the key
		// already stored in known_hosts. Offering only those algorithms avoids
		// a false "knownhosts: key mismatch" when the server also has other keys.
		if algorithms := preferredKnownHostAlgorithms(jumper.Host, jumper.Port); len(algorithms) > 0 {
			config.HostKeyAlgorithms = algorithms
		}
	}

	return config, nil
}

func makeAuthMethod(jumper model.Jumper) (ssh.AuthMethod, error) {
	switch strings.TrimSpace(jumper.AuthType) {
	case "password":
		if jumper.Password == "" {
			return nil, fmt.Errorf("password auth requires password")
		}
		return ssh.Password(jumper.Password), nil
	case "ssh_key":
		signer, err := loadPrivateSigner(jumper.KeyPath, jumper.Password)
		if err != nil {
			return nil, err
		}
		return ssh.PublicKeys(ensureSignerSupportsLegacyRSA(signer)), nil
	case "ssh_agent":
		return ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
			return getSSHAgentSigners(jumper)
		}), nil
	default:
		return nil, fmt.Errorf("unsupported authType: %s", jumper.AuthType)
	}
}

func getSSHAgentSigners(jumper model.Jumper) ([]ssh.Signer, error) {
	a, sock, err := getSSHAgent(jumper.AgentSocketPath)
	if err != nil {
		return nil, err
	}

	signers, err := a.Signers()
	if err != nil {
		// Agent may have restarted while the app keeps running.
		resetSSHAgent()
		var retryErr error
		a, sock, retryErr = getSSHAgent(jumper.AgentSocketPath)
		if retryErr != nil {
			return nil, retryErr
		}
		signers, err = a.Signers()
		if err != nil {
			return nil, fmt.Errorf("load identities from SSH agent failed (%s): %w", sock, err)
		}
	}
	if len(signers) == 0 {
		return nil, fmt.Errorf("ssh agent has no identities; selected socket=%s; run ssh-add first", sock)
	}
	for i := range signers {
		signers[i] = ensureSignerSupportsLegacyRSA(signers[i])
	}
	return signers, nil
}

func ensureSignerSupportsLegacyRSA(signer ssh.Signer) ssh.Signer {
	if signer == nil {
		return nil
	}
	pub := signer.PublicKey()
	if pub == nil || pub.Type() != ssh.KeyAlgoRSA {
		return signer
	}
	algorithmSigner, ok := signer.(ssh.AlgorithmSigner)
	if !ok {
		return signer
	}
	compatibleSigner, err := ssh.NewSignerWithAlgorithms(algorithmSigner, []string{
		ssh.KeyAlgoRSASHA512,
		ssh.KeyAlgoRSASHA256,
		ssh.KeyAlgoRSA,
	})
	if err != nil {
		return signer
	}
	return compatibleSigner
}

func userKnownHostsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir failed: %w", err)
	}
	return filepath.Join(home, ".ssh", "known_hosts"), nil
}

var knownHostsMu sync.Mutex

func makeHostKeyCallback(bypass bool) (ssh.HostKeyCallback, error) {
	if bypass {
		return ssh.InsecureIgnoreHostKey(), nil
	}

	knownHostsPath, err := userKnownHostsPath()
	if err != nil {
		return nil, err
	}
	return newRememberingHostKeyCallback(knownHostsPath)
}

// newRememberingHostKeyCallback проверяет ~/.ssh/known_hosts.
// Незнакомый хост записывается туда при первом подключении — то же, что ответ yes
// в команде ssh. Уже сохранённый ключ, который внезапно сменился, по-прежнему отклоняется.
func newRememberingHostKeyCallback(path string) (ssh.HostKeyCallback, error) {
	if err := ensureKnownHostsFile(path); err != nil {
		return nil, err
	}
	cb, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("known_hosts load failed (%s): %w", path, err)
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := cb(hostname, remote, key)
		if err == nil || !isUnknownHostKey(err) {
			return err
		}
		if err := appendKnownHost(path, hostname, key); err != nil {
			return fmt.Errorf("save host key to known_hosts failed (%s): %w", path, err)
		}
		slog.Info("saved new ssh host key", "host", knownhosts.Normalize(hostname), "type", key.Type())
		return nil
	}, nil
}

func isUnknownHostKey(err error) bool {
	var keyErr *knownhosts.KeyError
	return errors.As(err, &keyErr) && len(keyErr.Want) == 0
}

func ensureKnownHostsFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create .ssh dir failed: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create known_hosts failed (%s): %w", path, err)
	}
	return f.Close()
}

func appendKnownHost(path, hostname string, key ssh.PublicKey) error {
	line := knownhosts.Line([]string{hostname}, key)
	knownHostsMu.Lock()
	defer knownHostsMu.Unlock()

	f, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() > 0 {
		buf := make([]byte, 1)
		if _, err := f.ReadAt(buf, info.Size()-1); err != nil {
			return err
		}
		if buf[0] != '\n' {
			if _, err := f.WriteString("\n"); err != nil {
				return err
			}
		}
	}
	_, err = f.WriteString(line + "\n")
	return err
}

func loadPrivateSigner(keyPath, passphrase string) (ssh.Signer, error) {
	resolvedPath, err := resolveKeyPath(keyPath)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("read key file failed (%s): %w", resolvedPath, err)
	}

	if passphrase != "" {
		signer, passErr := ssh.ParsePrivateKeyWithPassphrase(raw, []byte(passphrase))
		if passErr == nil {
			return signer, nil
		}
		signer, keyErr := ssh.ParsePrivateKey(raw)
		if keyErr == nil {
			return signer, nil
		}
		return nil, fmt.Errorf("parse key failed: %v / %v", passErr, keyErr)
	}

	signer, err := ssh.ParsePrivateKey(raw)
	if err != nil {
		return nil, fmt.Errorf("parse key failed: %w", err)
	}
	return signer, nil
}

func resolveKeyPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("ssh_key auth requires keyPath")
	}

	if strings.HasPrefix(trimmed, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir failed: %w", err)
		}
		trimmed = filepath.Join(home, strings.TrimPrefix(trimmed, "~/"))
	}

	if filepath.IsAbs(trimmed) {
		return filepath.Clean(trimmed), nil
	}

	if _, err := os.Stat(trimmed); err == nil {
		return filepath.Clean(trimmed), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Clean(trimmed), nil
	}
	fromSSHDir := filepath.Join(home, ".ssh", trimmed)
	if _, err := os.Stat(fromSSHDir); err == nil {
		return filepath.Clean(fromSSHDir), nil
	}

	return filepath.Clean(trimmed), nil
}

func getSSHAgent(preferredSocketPath string) (agent.ExtendedAgent, string, error) {
	sshAgentMu.Lock()
	defer sshAgentMu.Unlock()

	candidates := agentSocketCandidates(preferredSocketPath)
	if len(candidates) == 0 {
		return nil, "", fmt.Errorf("no SSH agent socket found; set agent socket path, SSH_AUTH_SOCK, or NORKA_SSH_AUTH_SOCK")
	}

	if sshAgentInst != nil {
		if signers, err := sshAgentInst.Signers(); err == nil && len(signers) > 0 && socketInCandidates(sshAgentSock, candidates) {
			return sshAgentInst, sshAgentSock, nil
		}
		resetSSHAgentLocked()
	}

	var failures []string
	for _, sock := range candidates {
		conn, err := dialSSHAgentSocket(sock)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", sock, err))
			continue
		}

		inst := agent.NewClient(conn)
		signers, err := inst.Signers()
		if err != nil {
			_ = conn.Close()
			failures = append(failures, fmt.Sprintf("%s: %v", sock, err))
			continue
		}
		if len(signers) == 0 {
			_ = conn.Close()
			failures = append(failures, fmt.Sprintf("%s: no identities", sock))
			continue
		}

		sshAgentInst = inst
		sshAgentSock = sock
		sshAgentConn = conn
		return sshAgentInst, sshAgentSock, nil
	}

	return nil, "", fmt.Errorf("ssh agent has no usable identities; tried: %s", strings.Join(failures, "; "))
}

func agentSocketCandidates(preferredSocketPath string) []string {
	seen := map[string]struct{}{}
	var candidates []string
	add := func(sock string) {
		normalized := normalizeAgentSocketPath(sock)
		if normalized == "" {
			return
		}
		if _, exists := seen[normalized]; exists {
			return
		}
		seen[normalized] = struct{}{}
		candidates = append(candidates, normalized)
	}

	add(preferredSocketPath)
	add(os.Getenv("NORKA_SSH_AUTH_SOCK"))
	add(os.Getenv("SSH_AUTH_SOCK"))

	for _, s := range defaultAgentSocketCandidates() {
		add(s)
	}

	if runtime.GOOS != "windows" {
		home, err := os.UserHomeDir()
		if err == nil && strings.TrimSpace(home) != "" {
			add(filepath.Join(home, ".ssh", "ssh-agent.sock"))
		}
	}

	return candidates
}

func socketInCandidates(sock string, candidates []string) bool {
	normalizedSock := normalizeAgentSocketPath(sock)
	for _, candidate := range candidates {
		if candidate == normalizedSock {
			return true
		}
	}
	return false
}

func normalizeAgentSocketPath(sock string) string {
	s := strings.TrimSpace(sock)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return s
		}
		return filepath.Join(home, strings.TrimPrefix(s, "~/"))
	}
	return s
}

func resetSSHAgent() {
	sshAgentMu.Lock()
	defer sshAgentMu.Unlock()
	resetSSHAgentLocked()
}

func resetSSHAgentLocked() {
	if sshAgentConn != nil {
		_ = sshAgentConn.Close()
	}
	sshAgentInst = nil
	sshAgentSock = ""
	sshAgentConn = nil
}

func splitAlgorithms(s string) []string {
	return strings.Split(s, ",")
}

func preferredKnownHostAlgorithms(host string, port int) []string {
	path, err := userKnownHostsPath()
	if err != nil {
		return nil
	}
	algorithms, err := knownHostKeyAlgorithms(path, host, port)
	if err != nil {
		return nil
	}
	return algorithms
}

func knownHostKeyAlgorithms(path, host string, port int) ([]string, error) {
	if port <= 0 {
		port = 22
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	found := map[string]struct{}{}
	for _, raw := range bytes.Split(data, []byte("\n")) {
		line := strings.Trim(string(raw), " \t\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		marker, pattern, keyType, ok := parseKnownHostLine(line)
		if !ok || marker == "@revoked" || marker == "@cert-authority" {
			continue
		}
		if !knownHostPatternMatches(pattern, host, port) {
			continue
		}
		found[keyType] = struct{}{}
	}
	return algorithmsForKnownKeyTypes(found), nil
}

func parseKnownHostLine(line string) (marker, pattern, keyType string, ok bool) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return "", "", "", false
	}
	index := 0
	if strings.HasPrefix(fields[0], "@") {
		marker = fields[0]
		index = 1
	}
	if len(fields) < index+3 {
		return "", "", "", false
	}
	return marker, fields[index], fields[index+1], true
}

func knownHostPatternMatches(pattern, host string, port int) bool {
	matched := false
	for _, part := range strings.Split(pattern, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		negate := false
		if strings.HasPrefix(part, "!") {
			negate = true
			part = part[1:]
		}
		if part == "" || !knownHostTokenMatches(part, host, port) {
			continue
		}
		if negate {
			return false
		}
		matched = true
	}
	return matched
}

func knownHostTokenMatches(token, host string, port int) bool {
	if strings.HasPrefix(token, "|") {
		return hashedKnownHostMatches(token, host, port)
	}
	patternHost, patternPort := splitKnownHostToken(token)
	if patternPort != strconv.Itoa(port) {
		return false
	}
	return wildcardMatch([]byte(patternHost), []byte(host))
}

func splitKnownHostToken(token string) (host, port string) {
	if strings.HasPrefix(token, "[") {
		parsedHost, parsedPort, err := net.SplitHostPort(token)
		if err == nil {
			return parsedHost, parsedPort
		}
	}
	parsedHost, parsedPort, err := net.SplitHostPort(token)
	if err == nil {
		return parsedHost, parsedPort
	}
	return token, "22"
}

func hashedKnownHostMatches(token, host string, port int) bool {
	hashType, salt, sum, err := decodeKnownHostHash(token)
	if err != nil || hashType != "1" {
		return false
	}
	target := knownhosts.Normalize(net.JoinHostPort(host, strconv.Itoa(port)))
	mac := hmac.New(sha1.New, salt)
	mac.Write([]byte(target))
	return hmac.Equal(mac.Sum(nil), sum)
}

func decodeKnownHostHash(encoded string) (hashType string, salt, sum []byte, err error) {
	parts := strings.Split(encoded, "|")
	if len(parts) != 4 || parts[0] != "" {
		return "", nil, nil, errors.New("invalid hashed known_hosts token")
	}
	salt, err = base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return "", nil, nil, err
	}
	sum, err = base64.StdEncoding.DecodeString(parts[3])
	if err != nil {
		return "", nil, nil, err
	}
	return parts[1], salt, sum, nil
}

func wildcardMatch(pattern, value []byte) bool {
	for {
		if len(pattern) == 0 {
			return len(value) == 0
		}
		if len(value) == 0 {
			return false
		}
		if pattern[0] == '*' {
			if len(pattern) == 1 {
				return true
			}
			for index := range value {
				if wildcardMatch(pattern[1:], value[index:]) {
					return true
				}
			}
			return false
		}
		if pattern[0] == '?' || pattern[0] == value[0] {
			pattern = pattern[1:]
			value = value[1:]
			continue
		}
		return false
	}
}

func algorithmsForKnownKeyTypes(types map[string]struct{}) []string {
	if len(types) == 0 {
		return nil
	}
	var algorithms []string
	seen := map[string]struct{}{}
	add := func(items ...string) {
		for _, item := range items {
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			algorithms = append(algorithms, item)
		}
	}
	if _, ok := types[ssh.KeyAlgoED25519]; ok {
		add(ssh.KeyAlgoED25519)
	}
	if _, ok := types[ssh.KeyAlgoECDSA256]; ok {
		add(ssh.KeyAlgoECDSA256)
	}
	if _, ok := types[ssh.KeyAlgoECDSA384]; ok {
		add(ssh.KeyAlgoECDSA384)
	}
	if _, ok := types[ssh.KeyAlgoECDSA521]; ok {
		add(ssh.KeyAlgoECDSA521)
	}
	if _, ok := types[ssh.KeyAlgoRSA]; ok {
		add(ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSA)
	} else {
		if _, ok := types[ssh.KeyAlgoRSASHA512]; ok {
			add(ssh.KeyAlgoRSASHA512)
		}
		if _, ok := types[ssh.KeyAlgoRSASHA256]; ok {
			add(ssh.KeyAlgoRSASHA256)
		}
	}
	return algorithms
}

// parseHostKeyAlgorithms разбирает строку HostKeyAlgorithms в синтаксисе OpenSSH:
// - "+ssh-rsa": добавить ssh-rsa к алгоритмам по умолчанию
// - "ssh-rsa": использовать только ssh-rsa (заменить алгоритмы по умолчанию)
// - "-ssh-rsa": убрать ssh-rsa из алгоритмов по умолчанию
// - список через запятую: использовать его как есть
func parseHostKeyAlgorithms(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	// Проверяем синтаксис в стиле OpenSSH
	if strings.HasPrefix(s, "+") {
		// "+ssh-rsa": добавить к алгоритмам по умолчанию
		algorithm := strings.TrimPrefix(s, "+")
		return append(getDefaultHostKeyAlgorithms(), algorithm)
	} else if strings.HasPrefix(s, "-") {
		// "-ssh-rsa": убрать из алгоритмов по умолчанию
		algorithm := strings.TrimPrefix(s, "-")
		return removeFromDefaultHostKeyAlgorithms(algorithm)
	} else {
		// Обычный список через запятую
		return splitAlgorithms(s)
	}
}

// getDefaultHostKeyAlgorithms возвращает алгоритмы ключа хоста, которые Go SSH поддерживает по умолчанию
func getDefaultHostKeyAlgorithms() []string {
	// Алгоритмы ключа хоста, которые Go SSH поддерживает по умолчанию
	return []string{
		"ssh-rsa",
		"rsa-sha2-256",
		"rsa-sha2-512",
		"ecdsa-sha2-nistp256",
		"ecdsa-sha2-nistp384",
		"ecdsa-sha2-nistp521",
		"ssh-ed25519",
	}
}

// removeFromDefaultHostKeyAlgorithms убирает указанный алгоритм из списка по умолчанию
func removeFromDefaultHostKeyAlgorithms(algorithm string) []string {
	defaultAlgos := getDefaultHostKeyAlgorithms()
	var result []string
	for _, algo := range defaultAlgos {
		if algo != algorithm {
			result = append(result, algo)
		}
	}
	return result
}
