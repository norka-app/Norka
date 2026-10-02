package biz

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"norka/internal/conf"
	"norka/internal/forward"
	"norka/internal/model"
	"norka/internal/secrets"
)

var (
	ErrTunnelNotFound       = errors.New("tunnel not found")
	ErrFreePlanRunningLimit = errors.New("free plan running tunnel limit exceeded")
)

// FreePlanRunningLimit is the max concurrent running tunnels for non-Pro users.
const FreePlanRunningLimit = 3

// statusReconnecting marks a tunnel whose SSH connection dropped while the
// runtime is still alive and re-dialing with backoff (forward.reconnectWithBackoff).
// It ends with "running" on success or "error" when reconnect gives up.
const statusReconnecting = "reconnecting"

// TunnelEvents receives tunnel lifecycle changes that are worth telling the user.
// Implementations must ignore calls that the user triggered themselves; TunnelBiz
// only reports drops, reconnects, give-ups and start results.
type TunnelEvents interface {
	Dropped(id int, name string)
	Reconnected(id int, name string)
	GaveUp(id int, name string)
	ConnectFailed(id int, name string)
	Connected(id int, name string)
}

type TunnelBiz struct {
	storage  *conf.Storage
	secrets  *secrets.Vault
	events   TunnelEvents
	mu       sync.Mutex
	runs     map[int]*forward.LocalForward
	starting map[int]context.CancelFunc
}

func NewTunnelBiz(storage *conf.Storage) *TunnelBiz {
	return &TunnelBiz{
		storage: storage,
		runs:    make(map[int]*forward.LocalForward),
	}
}

// SetSecrets hydrates jumper passwords from the OS keychain before dialing.
func (b *TunnelBiz) SetSecrets(vault *secrets.Vault) {
	if b == nil {
		return
	}
	b.secrets = vault
}

// SetEvents attaches OS notification fan-out. Nil disables it.
func (b *TunnelBiz) SetEvents(events TunnelEvents) {
	if b == nil {
		return
	}
	b.events = events
}

func (b *TunnelBiz) List() ([]model.Tunnel, error) {
	cfg, err := b.storage.Load()
	if err != nil {
		return nil, err
	}

	items := append([]model.Tunnel{}, cfg.Tunnels...)
	b.attachRuntimeLatencies(items)
	b.normalizeStaleReconnecting(items)
	return items, nil
}

// normalizeStaleReconnecting reports a persisted "reconnecting" status as "error"
// when no runtime is alive for the tunnel (e.g. the app exited mid-reconnect),
// so the UI never shows an endless reconnect.
func (b *TunnelBiz) normalizeStaleReconnecting(items []model.Tunnel) {
	for i := range items {
		if items[i].Status == statusReconnecting && !b.isRunning(items[i].ID) {
			items[i].Status = "error"
		}
	}
}

func (b *TunnelBiz) Create(payload model.TunnelPayload) (model.Tunnel, error) {
	payload = normalizeTunnelPayload(payload)

	var created model.Tunnel
	_, err := b.storage.Update(func(cfg *conf.Config) error {
		if err := validateTunnelPayload(payload); err != nil {
			return err
		}
		if _, err := collectJumpers(cfg.Jumpers, payload.JumperIDs); err != nil {
			return err
		}
		if err := validateGroupID(cfg.Groups, payload.GroupID); err != nil {
			return err
		}

		created = model.Tunnel{
			ID:          nextTunnelID(cfg.Tunnels),
			Name:        payload.Name,
			GroupID:     payload.GroupID,
			Mode:        payload.Mode,
			JumperIDs:   append([]int{}, payload.JumperIDs...),
			LocalHost:   payload.LocalHost,
			LocalPort:   payload.LocalPort,
			RemoteHost:  payload.RemoteHost,
			RemotePort:  payload.RemotePort,
			AutoStart:   payload.AutoStart,
			Status:      payload.Status,
			LastError:   "",
			Description: payload.Description,
		}
		cfg.Tunnels = append(cfg.Tunnels, created)
		return nil
	})
	if err != nil {
		return model.Tunnel{}, err
	}

	return created, nil
}

func (b *TunnelBiz) Update(id int, payload model.TunnelPayload) (model.Tunnel, error) {
	if id <= 0 {
		return model.Tunnel{}, fmt.Errorf("invalid tunnel id")
	}
	if b.isRunning(id) {
		return model.Tunnel{}, fmt.Errorf("tunnel is running, stop it before editing")
	}

	payload = normalizeTunnelPayload(payload)

	var updated model.Tunnel
	_, err := b.storage.Update(func(cfg *conf.Config) error {
		if err := validateTunnelPayload(payload); err != nil {
			return err
		}
		if _, err := collectJumpers(cfg.Jumpers, payload.JumperIDs); err != nil {
			return err
		}
		if err := validateGroupID(cfg.Groups, payload.GroupID); err != nil {
			return err
		}

		idx := -1
		for i := range cfg.Tunnels {
			if cfg.Tunnels[i].ID == id {
				idx = i
				break
			}
		}
		if idx == -1 {
			return ErrTunnelNotFound
		}

		updated = model.Tunnel{
			ID:          id,
			Name:        payload.Name,
			GroupID:     payload.GroupID,
			Mode:        payload.Mode,
			JumperIDs:   append([]int{}, payload.JumperIDs...),
			LocalHost:   payload.LocalHost,
			LocalPort:   payload.LocalPort,
			RemoteHost:  payload.RemoteHost,
			RemotePort:  payload.RemotePort,
			AutoStart:   payload.AutoStart,
			Status:      payload.Status,
			LastError:   cfg.Tunnels[idx].LastError,
			Description: payload.Description,
		}
		cfg.Tunnels[idx] = updated
		return nil
	})
	if err != nil {
		return model.Tunnel{}, err
	}

	return updated, nil
}

func (b *TunnelBiz) MoveToGroup(id int, groupID int) (model.Tunnel, error) {
	if id <= 0 {
		return model.Tunnel{}, fmt.Errorf("invalid tunnel id")
	}
	if groupID < 0 {
		groupID = 0
	}

	var updated model.Tunnel
	_, err := b.storage.Update(func(cfg *conf.Config) error {
		if err := validateGroupID(cfg.Groups, groupID); err != nil {
			return err
		}

		idx := -1
		for i := range cfg.Tunnels {
			if cfg.Tunnels[i].ID == id {
				idx = i
				break
			}
		}
		if idx == -1 {
			return ErrTunnelNotFound
		}

		cfg.Tunnels[idx].GroupID = groupID
		updated = cfg.Tunnels[idx]
		return nil
	})
	if err != nil {
		return model.Tunnel{}, err
	}

	return updated, nil
}

func (b *TunnelBiz) Delete(id int) error {
	if id <= 0 {
		return fmt.Errorf("invalid tunnel id")
	}
	if err := b.stopRuntime(id); err != nil {
		return err
	}

	_, err := b.storage.Update(func(cfg *conf.Config) error {
		idx := -1
		for i := range cfg.Tunnels {
			if cfg.Tunnels[i].ID == id {
				idx = i
				break
			}
		}
		if idx == -1 {
			return ErrTunnelNotFound
		}

		cfg.Tunnels = append(cfg.Tunnels[:idx], cfg.Tunnels[idx+1:]...)
		detachTunnelFromProfiles(cfg, id)
		return nil
	})
	return err
}

func (b *TunnelBiz) RunningCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.runs)
}

// Toggle starts or stops a tunnel. maxRunning <= 0 means unlimited (Pro);
// otherwise starting is blocked when RunningCount() >= maxRunning.
func (b *TunnelBiz) Toggle(id int, maxRunning int) (model.Tunnel, error) {
	tunnel, err := b.tunnelByID(id)
	if err != nil {
		return model.Tunnel{}, err
	}

	if b.isRunning(id) || tunnel.Status == "running" {
		slog.Info("tunnel toggle stop", "tunnel_id", tunnel.ID, "name", tunnel.Name)
		return b.stopTunnel(id)
	}

	if b.cancelStart(id) {
		slog.Info("tunnel toggle cancel start", "tunnel_id", tunnel.ID, "name", tunnel.Name)
		_ = b.stopRuntime(id)
		return b.updateStatus(id, "stopped", "")
	}

	return b.startTunnel(tunnel, maxRunning)
}

// Start connects a tunnel. A tunnel that is already running is returned as it is.
func (b *TunnelBiz) Start(id int, maxRunning int) (model.Tunnel, error) {
	tunnel, err := b.tunnelByID(id)
	if err != nil {
		return model.Tunnel{}, err
	}
	if b.isRunning(id) || b.isStarting(id) {
		return tunnel, nil
	}
	return b.startTunnel(tunnel, maxRunning)
}

func (b *TunnelBiz) tunnelByID(id int) (model.Tunnel, error) {
	if id <= 0 {
		return model.Tunnel{}, fmt.Errorf("invalid tunnel id")
	}
	cfg, err := b.storage.Load()
	if err != nil {
		return model.Tunnel{}, err
	}
	tunnel, ok := findTunnelByID(cfg.Tunnels, id)
	if !ok {
		return model.Tunnel{}, ErrTunnelNotFound
	}
	return tunnel, nil
}

func (b *TunnelBiz) stopTunnel(id int) (model.Tunnel, error) {
	if err := b.stopRuntime(id); err != nil {
		return model.Tunnel{}, err
	}
	return b.updateStatus(id, "stopped", "")
}

func (b *TunnelBiz) startTunnel(tunnel model.Tunnel, maxRunning int) (model.Tunnel, error) {
	id := tunnel.ID
	if maxRunning > 0 && b.RunningCount() >= maxRunning {
		return model.Tunnel{}, fmt.Errorf("%w: limit %d", ErrFreePlanRunningLimit, maxRunning)
	}
	cfg, err := b.storage.Load()
	if err != nil {
		return model.Tunnel{}, err
	}

	jumpers, err := collectJumpers(cfg.Jumpers, tunnel.JumperIDs)
	if err != nil {
		b.emitConnectFailed(tunnel.ID, tunnel.Name)
		updated, statusErr := b.updateStatus(id, "error", "jumper not found")
		if statusErr != nil {
			return model.Tunnel{}, ErrJumperNotFound
		}
		return updated, nil
	}
	if tunnel.Mode != "local" && tunnel.Mode != "remote" && tunnel.Mode != "dynamic" {
		msg := fmt.Sprintf("mode %s is not supported yet, only local, remote and dynamic forward are implemented", tunnel.Mode)
		b.emitConnectFailed(tunnel.ID, tunnel.Name)
		updated, statusErr := b.updateStatus(id, "error", msg)
		if statusErr != nil {
			return model.Tunnel{}, errors.New(msg)
		}
		return updated, nil
	}

	if err := b.startRuntime(tunnel, b.hydrateJumpers(jumpers)); err != nil {
		if errors.Is(err, context.Canceled) {
			_ = b.stopRuntime(id)
			return b.updateStatus(id, "stopped", "")
		}
		b.emitConnectFailed(tunnel.ID, tunnel.Name)
		updated, statusErr := b.updateStatus(id, "error", errReason(err))
		if statusErr != nil {
			return model.Tunnel{}, fmt.Errorf("start tunnel failed: %v (persist status failed: %v)", err, statusErr)
		}
		return updated, nil
	}

	slog.Info("tunnel toggle start", "tunnel_id", tunnel.ID, "name", tunnel.Name)
	b.emitConnected(tunnel.ID, tunnel.Name)
	updated, err := b.updateStatus(id, "running", "")
	if err != nil {
		_ = b.stopRuntime(id)
		return model.Tunnel{}, err
	}
	return updated, nil
}

func (b *TunnelBiz) TestConnection(payload model.TunnelPayload, inlineJumper *model.JumperPayload) (time.Duration, error) {
	payload = normalizeTunnelPayload(payload)
	if payload.Status == "" {
		payload.Status = "stopped"
	}
	allowEmptyJumpers := inlineJumper != nil
	if err := validateTunnelPayloadWithOption(payload, !allowEmptyJumpers); err != nil {
		return 0, err
	}

	chain := make([]model.Jumper, 0, len(payload.JumperIDs)+1)
	var inline model.Jumper
	hasInline := false
	if inlineJumper != nil {
		jumperPayload := normalizeJumperPayload(*inlineJumper)
		if err := validateJumperPayload(jumperPayload, false); err != nil {
			return 0, fmt.Errorf("jumper: %w", err)
		}
		inline = model.Jumper{
			Name:                   jumperPayload.Name,
			Host:                   jumperPayload.Host,
			Port:                   jumperPayload.Port,
			User:                   jumperPayload.User,
			AuthType:               jumperPayload.AuthType,
			KeyPath:                jumperPayload.KeyPath,
			AgentSocketPath:        jumperPayload.AgentSocketPath,
			Password:               jumperPayload.Password,
			BypassHostVerification: jumperPayload.BypassHostVerification,
			KeepAliveIntervalMs:    jumperPayload.KeepAliveIntervalMs,
			TimeoutMs:              jumperPayload.TimeoutMs,
			HostKeyAlgorithms:      jumperPayload.HostKeyAlgorithms,
			Notes:                  jumperPayload.Notes,
		}
		hasInline = true
	}
	if len(payload.JumperIDs) > 0 {
		cfg, err := b.storage.Load()
		if err != nil {
			return 0, err
		}
		jumpers, err := collectJumpers(cfg.Jumpers, payload.JumperIDs)
		if err != nil {
			return 0, err
		}
		chain = append(chain, b.hydrateJumpers(jumpers)...)
	}
	if hasInline {
		chain = append(chain, inline)
	}

	t := model.Tunnel{
		Name:       payload.Name,
		Mode:       payload.Mode,
		LocalHost:  payload.LocalHost,
		LocalPort:  payload.LocalPort,
		RemoteHost: payload.RemoteHost,
		RemotePort: payload.RemotePort,
	}
	return forward.TestTunnelConnection(t, chain)
}

func (b *TunnelBiz) attachRuntimeLatencies(items []model.Tunnel) {
	if len(items) == 0 {
		return
	}

	b.mu.Lock()
	runs := make(map[int]*forward.LocalForward, len(b.runs))
	for id, run := range b.runs {
		runs[id] = run
	}
	b.mu.Unlock()

	for i := range items {
		if items[i].Status != "running" {
			items[i].LatencyMs = 0
			continue
		}
		run, ok := runs[items[i].ID]
		if !ok || run == nil {
			items[i].LatencyMs = 0
			continue
		}
		latency, hasLatency := run.LastLatency()
		if !hasLatency || latency <= 0 {
			items[i].LatencyMs = 0
			continue
		}
		items[i].LatencyMs = latency.Milliseconds()
	}
}

func (b *TunnelBiz) TrafficSnapshot() (up, down uint64) {
	b.mu.Lock()
	runs := make([]*forward.LocalForward, 0, len(b.runs))
	for _, run := range b.runs {
		if run != nil {
			runs = append(runs, run)
		}
	}
	b.mu.Unlock()

	for _, run := range runs {
		runUp, runDown := run.Traffic()
		up += runUp
		down += runDown
	}
	return up, down
}

// StartAutoStart starts tunnels marked autoStart. maxRunning <= 0 means unlimited
// (Pro); otherwise only the first maxRunning auto-start tunnels are started.
func (b *TunnelBiz) StartAutoStart(maxRunning int) error {
	cfg, err := b.storage.Load()
	if err != nil {
		return err
	}

	autoStartTunnels := make([]model.Tunnel, 0, len(cfg.Tunnels))
	for _, t := range cfg.Tunnels {
		if !t.AutoStart {
			continue
		}
		autoStartTunnels = append(autoStartTunnels, t)
	}

	if maxRunning > 0 && len(autoStartTunnels) > maxRunning {
		autoStartTunnels = autoStartTunnels[:maxRunning]
	}

	for _, t := range autoStartTunnels {
		_, _ = b.updateStatus(t.ID, "busy", "")
	}

	var wg sync.WaitGroup
	for _, tunnel := range autoStartTunnels {
		t := tunnel
		wg.Add(1)
		go func() {
			defer wg.Done()
			if t.Mode != "local" && t.Mode != "remote" && t.Mode != "dynamic" {
				b.emitConnectFailed(t.ID, t.Name)
				_, _ = b.updateStatus(t.ID, "error", fmt.Sprintf("mode %s is not supported yet, only local, remote and dynamic forward are implemented", t.Mode))
				return
			}

			jumpers, err := collectJumpers(cfg.Jumpers, t.JumperIDs)
			if err != nil {
				b.emitConnectFailed(t.ID, t.Name)
				_, _ = b.updateStatus(t.ID, "error", "jumper not found")
				return
			}
			if err := b.startRuntime(t, b.hydrateJumpers(jumpers)); err != nil {
				if errors.Is(err, context.Canceled) {
					_, _ = b.updateStatus(t.ID, "stopped", "")
					return
				}
				b.emitConnectFailed(t.ID, t.Name)
				_, _ = b.updateStatus(t.ID, "error", errReason(err))
				return
			}
			b.emitConnected(t.ID, t.Name)
			_, _ = b.updateStatus(t.ID, "running", "")
		}()
	}
	wg.Wait()
	return nil
}

func (b *TunnelBiz) Shutdown() {
	b.mu.Lock()
	ids := make([]int, 0, len(b.runs))
	for id := range b.runs {
		ids = append(ids, id)
	}
	b.mu.Unlock()

	for _, id := range ids {
		_ = b.stopRuntime(id)
		_, _ = b.updateStatus(id, "stopped", "")
	}
}

func (b *TunnelBiz) startRuntime(t model.Tunnel, jumpers []model.Jumper) error {
	ctx, cancel := context.WithCancel(context.Background())
	b.mu.Lock()
	if _, ok := b.runs[t.ID]; ok {
		b.mu.Unlock()
		cancel()
		return nil
	}
	if b.starting == nil {
		b.starting = make(map[int]context.CancelFunc)
	}
	if prev, ok := b.starting[t.ID]; ok {
		b.mu.Unlock()
		prev()
		cancel()
		return context.Canceled
	}
	b.starting[t.ID] = cancel
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		if current := b.starting[t.ID]; current != nil {
			delete(b.starting, t.ID)
		}
		b.mu.Unlock()
	}()

	if err := ctx.Err(); err != nil {
		return err
	}

	run := forward.NewLocalForward(t, jumpers)
	run.SetStartContext(ctx)
	if err := run.Start(); err != nil {
		slog.Error("tunnel runtime start failed", "tunnel_id", t.ID, "name", t.Name, "err", err)
		return err
	}

	if err := ctx.Err(); err != nil {
		_ = run.Stop()
		return err
	}

	b.mu.Lock()
	if _, ok := b.runs[t.ID]; ok {
		b.mu.Unlock()
		_ = run.Stop()
		return nil
	}
	b.runs[t.ID] = run
	b.mu.Unlock()
	if err := ctx.Err(); err != nil {
		_ = b.stopRuntime(t.ID)
		return err
	}
	slog.Info("tunnel runtime started", "tunnel_id", t.ID, "name", t.Name)

	go b.watchRuntime(t.ID, run)
	return nil
}

func (b *TunnelBiz) watchRuntime(id int, run *forward.LocalForward) {
	done := run.Done()
	if done == nil {
		return
	}

	name := b.tunnelName(id)
	events := run.Events()
	reconnecting := false
	lastDisconnectErr := ""
	for {
		select {
		case <-done:
			b.mu.Lock()
			active, ok := b.runs[id]
			if !ok || active != run {
				b.mu.Unlock()
				return
			}
			delete(b.runs, id)
			b.mu.Unlock()

			if run.Err() != nil {
				slog.Warn("tunnel runtime exited with error", "tunnel_id", id, "err", run.Err())
				if b.events != nil {
					b.events.GaveUp(id, name)
				}
				_, _ = b.updateStatus(id, "error", errReason(run.Err()))
			} else {
				slog.Info("tunnel runtime exited", "tunnel_id", id)
				if reconnecting {
					// do not leave the tunnel stuck in "reconnecting" once the runtime is gone
					_, _ = b.updateStatus(id, "error", lastDisconnectErr)
				}
			}
			return
		case evt, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			b.mu.Lock()
			active, stillRunning := b.runs[id]
			b.mu.Unlock()
			if !stillRunning || active != run {
				continue
			}
			switch evt.Type {
			case forward.RuntimeEventDisconnected:
				// LocalForward emits Disconnected right before it starts re-dialing with backoff.
				slog.Warn("tunnel runtime disconnected, reconnecting", "tunnel_id", id, "err", evt.Err)
				reconnecting = true
				lastDisconnectErr = errReason(evt.Err)
				if b.events != nil && !evt.Quiet {
					b.events.Dropped(id, name)
				}
				_, _ = b.updateStatus(id, statusReconnecting, lastDisconnectErr)
			case forward.RuntimeEventReconnected:
				slog.Info("tunnel runtime reconnected", "tunnel_id", id)
				reconnecting = false
				lastDisconnectErr = ""
				if b.events != nil && !evt.Quiet {
					b.events.Reconnected(id, name)
				}
				_, _ = b.updateStatus(id, "running", "")
			}
		}
	}
}

func (b *TunnelBiz) stopRuntime(id int) error {
	b.mu.Lock()
	run, ok := b.runs[id]
	if ok {
		delete(b.runs, id)
	}
	b.mu.Unlock()

	if !ok {
		return nil
	}
	return run.Stop()
}

func (b *TunnelBiz) cancelStart(id int) bool {
	b.mu.Lock()
	cancel, ok := b.starting[id]
	b.mu.Unlock()
	if !ok || cancel == nil {
		return false
	}
	cancel()
	return true
}

func (b *TunnelBiz) isRunning(id int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, ok := b.runs[id]
	return ok
}

func (b *TunnelBiz) updateStatus(id int, status, lastError string) (model.Tunnel, error) {
	var updated model.Tunnel
	_, err := b.storage.Update(func(cfg *conf.Config) error {
		idx := -1
		for i := range cfg.Tunnels {
			if cfg.Tunnels[i].ID == id {
				idx = i
				break
			}
		}
		if idx == -1 {
			return ErrTunnelNotFound
		}
		cfg.Tunnels[idx].Status = status
		cfg.Tunnels[idx].LastError = strings.TrimSpace(lastError)
		updated = cfg.Tunnels[idx]
		return nil
	})
	if err != nil {
		return model.Tunnel{}, err
	}
	if updated.LastError != "" {
		slog.Info("tunnel status updated", "tunnel_id", updated.ID, "name", updated.Name, "status", updated.Status, "error", updated.LastError)
	} else {
		slog.Info("tunnel status updated", "tunnel_id", updated.ID, "name", updated.Name, "status", updated.Status)
	}
	return updated, nil
}

func errReason(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
}

func normalizeTunnelPayload(payload model.TunnelPayload) model.TunnelPayload {
	payload.Name = strings.TrimSpace(payload.Name)
	payload.Mode = strings.TrimSpace(payload.Mode)
	payload.LocalHost = strings.TrimSpace(payload.LocalHost)
	payload.RemoteHost = strings.TrimSpace(payload.RemoteHost)
	payload.Description = strings.TrimSpace(payload.Description)
	payload.Status = strings.TrimSpace(payload.Status)
	payload.JumperIDs = normalizeJumperIDs(payload.JumperIDs)
	if payload.GroupID < 0 {
		payload.GroupID = 0
	}

	if payload.Mode == "" {
		payload.Mode = "local"
	}
	if payload.Status == "" {
		payload.Status = "stopped"
	}
	if payload.LocalHost == "" {
		payload.LocalHost = "127.0.0.1"
	}

	return payload
}

func validateTunnelPayload(payload model.TunnelPayload) error {
	return validateTunnelPayloadWithOption(payload, true)
}

func validateTunnelPayloadWithOption(payload model.TunnelPayload, requireJumpers bool) error {
	if payload.Name == "" {
		return fmt.Errorf("name is required")
	}
	if requireJumpers && len(payload.JumperIDs) == 0 {
		return fmt.Errorf("jumperIds is required")
	}
	if payload.LocalHost == "" {
		return fmt.Errorf("localHost is required")
	}
	if payload.LocalPort < 1 || payload.LocalPort > 65535 {
		return fmt.Errorf("localPort must be between 1 and 65535")
	}
	switch payload.Mode {
	case "local", "remote", "dynamic":
	default:
		return fmt.Errorf("unsupported mode: %s", payload.Mode)
	}
	if payload.Mode != "dynamic" {
		if payload.RemoteHost == "" {
			return fmt.Errorf("remoteHost is required for non-dynamic mode")
		}
		if payload.RemotePort < 1 || payload.RemotePort > 65535 {
			return fmt.Errorf("remotePort must be between 1 and 65535")
		}
	}
	switch payload.Status {
	case "running", "stopped", "error":
	default:
		return fmt.Errorf("unsupported status: %s", payload.Status)
	}
	return nil
}

func (b *TunnelBiz) hydrateJumpers(jumpers []model.Jumper) []model.Jumper {
	if b == nil || b.secrets == nil {
		return jumpers
	}
	for i := range jumpers {
		b.secrets.Open(&jumpers[i])
	}
	return jumpers
}

func (b *TunnelBiz) tunnelName(id int) string {
	if b == nil || b.storage == nil {
		return ""
	}
	cfg, err := b.storage.Load()
	if err != nil {
		return ""
	}
	tunnel, ok := findTunnelByID(cfg.Tunnels, id)
	if !ok {
		return ""
	}
	return tunnel.Name
}

func (b *TunnelBiz) emitConnectFailed(id int, name string) {
	if b != nil && b.events != nil {
		b.events.ConnectFailed(id, name)
	}
}

func (b *TunnelBiz) emitConnected(id int, name string) {
	if b != nil && b.events != nil {
		b.events.Connected(id, name)
	}
}

func collectJumpers(items []model.Jumper, ids []int) ([]model.Jumper, error) {
	if len(ids) == 0 {
		return nil, ErrJumperNotFound
	}
	collected := make([]model.Jumper, 0, len(ids))
	for _, id := range ids {
		item, ok := findJumperByID(items, id)
		if !ok {
			return nil, ErrJumperNotFound
		}
		collected = append(collected, item)
	}
	return collected, nil
}

func normalizeJumperIDs(ids []int) []int {
	out := make([]int, 0, len(ids))
	seen := make(map[int]struct{}, len(ids))
	appendID := func(id int) {
		if id <= 0 {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	for _, id := range ids {
		appendID(id)
	}
	return out
}

func findJumperByID(items []model.Jumper, id int) (model.Jumper, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return model.Jumper{}, false
}

func findTunnelByID(items []model.Tunnel, id int) (model.Tunnel, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return model.Tunnel{}, false
}

func nextTunnelID(items []model.Tunnel) int {
	next := 1
	for _, item := range items {
		if item.ID >= next {
			next = item.ID + 1
		}
	}
	return next
}
