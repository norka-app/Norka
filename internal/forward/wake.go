package forward

import (
	"fmt"
	"time"

	"github.com/norka-app/Norka/internal/wake"

	"golang.org/x/crypto/ssh"
)

const (
	waitElapsed = iota
	waitStopped
	waitWoke
)

// Phase reports whether this runtime should be probed after sleep.
// A tunnel the user stopped, or one that is still dialing for the first time,
// is not reconnecting.
func (f *LocalForward) Phase() string {
	if f == nil {
		return wake.PhaseStopped
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.started || f.stopping {
		return wake.PhaseStopped
	}
	if f.client != nil {
		return wake.PhaseConnected
	}
	if f.listener != nil {
		return wake.PhaseReconnecting
	}
	return wake.PhaseStarting
}

// Probe sends one keepalive and returns whether the session answered in time.
// It does not close the client; ReconnectNow does that after arming an immediate retry.
func (f *LocalForward) Probe(timeout time.Duration) (bool, error) {
	if f == nil || f.isStopping() {
		return false, nil
	}
	client := f.snapshotClient()
	if client == nil {
		return false, nil
	}
	if timeout <= 0 {
		timeout = wake.ProbeTimeout
	}
	if err := probeKeepalive(client, timeout); err != nil {
		return false, err
	}
	return true, nil
}

// ReconnectNow drops a dead session or interrupts backoff.
// It returns false when the user has already stopped the tunnel.
func (f *LocalForward) ReconnectNow() bool {
	if f == nil || f.isStopping() || !f.isStarted() {
		return false
	}
	f.armImmediate()
	if client := f.snapshotClient(); client != nil {
		_ = client.Close()
	}
	return !f.isStopping()
}

func (f *LocalForward) isStarted() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.started
}

func (f *LocalForward) snapshotClient() *ssh.Client {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.client
}

func (f *LocalForward) armImmediate() {
	f.immediate.Store(true)
	f.quietCycle.Store(true)
	f.signalNudge()
}

func (f *LocalForward) consumeImmediate() bool {
	return f.immediate.Swap(false)
}

func (f *LocalForward) nudgeSignal() chan struct{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.nudgeCh
}

func (f *LocalForward) signalNudge() {
	ch := f.nudgeSignal()
	if ch == nil {
		return
	}
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (f *LocalForward) drainNudge() {
	ch := f.nudgeSignal()
	if ch == nil {
		return
	}
	select {
	case <-ch:
	default:
	}
}

func (f *LocalForward) waitOrWake(wait time.Duration, stop <-chan struct{}) int {
	nudge := f.nudgeSignal()
	if wait <= 0 {
		select {
		case <-stop:
			return waitStopped
		case <-nudge:
			return waitWoke
		default:
			return waitElapsed
		}
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-stop:
		return waitStopped
	case <-nudge:
		return waitWoke
	case <-timer.C:
		return waitElapsed
	}
}

func probeKeepalive(client *ssh.Client, timeout time.Duration) error {
	if client == nil {
		return fmt.Errorf("ssh client is nil")
	}
	errCh := make(chan error, 1)
	go func() {
		_, err := TestJumperLatency(client)
		errCh <- err
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-errCh:
		return err
	case <-timer.C:
		return fmt.Errorf("keepalive timeout after %s", timeout)
	}
}
