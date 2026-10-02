package biz

import (
	"fmt"
	"log/slog"
	"strings"

	"norka/internal/model"
)

// holdsLocalPort reports whether a tunnel in this status listens (or is about to listen)
// on its local port.
func holdsLocalPort(status string) bool {
	switch status {
	case "running", "busy", statusReconnecting:
		return true
	}
	return false
}

func normalizeBindHost(host string) string {
	switch h := strings.ToLower(strings.TrimSpace(host)); h {
	case "", "localhost", "::1", "[::1]":
		return "127.0.0.1"
	default:
		return h
	}
}

// PortConflicts returns the other tunnels that are running or connecting on t's local
// host:port. Only one of them can listen there. Same rule as the frontend
// (frontend/src/utils/port-conflicts.js).
func PortConflicts(t model.Tunnel, all []model.Tunnel) []model.Tunnel {
	if t.LocalPort <= 0 {
		return nil
	}
	host := normalizeBindHost(t.LocalHost)
	var conflicts []model.Tunnel
	for _, other := range all {
		if other.ID == t.ID || !holdsLocalPort(other.Status) {
			continue
		}
		if other.LocalPort == t.LocalPort && normalizeBindHost(other.LocalHost) == host {
			conflicts = append(conflicts, other)
		}
	}
	return conflicts
}

// Stop stops a tunnel, or cancels a start in progress, and marks it stopped.
// A tunnel that is not running and not connecting is returned as it is.
func (b *TunnelBiz) Stop(id int) (model.Tunnel, error) {
	tunnel, err := b.tunnelByID(id)
	if err != nil {
		return model.Tunnel{}, err
	}
	if !b.isRunning(id) && !b.isStarting(id) && tunnel.Status != "running" && tunnel.Status != "busy" && tunnel.Status != statusReconnecting {
		return tunnel, nil
	}
	if b.cancelStart(id) {
		_ = b.stopRuntime(id)
		return b.updateStatus(id, "stopped", "")
	}
	if err := b.stopRuntime(id); err != nil {
		return model.Tunnel{}, err
	}
	return b.updateStatus(id, "stopped", "")
}

// SwitchTo starts tunnel id after stopping the tunnels that hold its local port
// (PortConflicts). A tunnel that is already running or connecting is left as is.
// maxRunning is passed to Toggle.
func (b *TunnelBiz) SwitchTo(id int, maxRunning int) (model.Tunnel, error) {
	items, err := b.List()
	if err != nil {
		return model.Tunnel{}, err
	}
	target, ok := findTunnelByID(items, id)
	if !ok {
		return model.Tunnel{}, ErrTunnelNotFound
	}
	if holdsLocalPort(target.Status) || b.isRunning(id) || b.isStarting(id) {
		return target, nil
	}
	for _, other := range PortConflicts(target, items) {
		slog.Info("tunnel port switch stop", "tunnel_id", other.ID, "name", other.Name, "for_tunnel_id", id)
		if _, err := b.Stop(other.ID); err != nil {
			return model.Tunnel{}, fmt.Errorf("stop tunnel %s: %w", other.Name, err)
		}
	}
	return b.Toggle(id, maxRunning)
}

func (b *TunnelBiz) isStarting(id int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, ok := b.starting[id]
	return ok
}
