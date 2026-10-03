package engine

import (
	"context"
	"errors"
	"log/slog"
	"net"

	"github.com/norka-app/Norka/internal/automation"
	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/features"
	"github.com/norka-app/Norka/internal/ipc"
	"github.com/norka-app/Norka/internal/model"
	"github.com/norka-app/Norka/internal/scheme"
)

// AutomationPrompt is shown by the frontend for a norka:// link.
// Kind is "confirm" or "notice". Code selects the translated sentence.
type AutomationPrompt struct {
	Kind     string   `json:"kind"`
	Code     string   `json:"code"`
	Action   string   `json:"action,omitempty"`
	TunnelID int      `json:"tunnelId,omitempty"`
	Name     string   `json:"name,omitempty"`
	Names    []string `json:"names,omitempty"`
	Detail   string   `json:"detail,omitempty"`
}

func (e *Engine) AutomationPrompt() AutomationPrompt {
	if e == nil {
		return AutomationPrompt{}
	}
	e.automationMu.Lock()
	defer e.automationMu.Unlock()
	return e.automationPrompt
}

// DismissAutomationPrompt drops a prompt the user closed without confirming.
func (e *Engine) DismissAutomationPrompt() {
	if e == nil {
		return
	}
	e.automationMu.Lock()
	e.automationPrompt = AutomationPrompt{}
	e.automationMu.Unlock()
}

// ConfirmAutomation runs a connect or disconnect the user just approved.
// remember stores the tunnel so the next link does not ask again.
func (e *Engine) ConfirmAutomation(tunnelID int, action string, remember bool) error {
	if e == nil {
		return errors.New("app is not initialized")
	}
	if !e.FeatureOn(features.Automation) {
		return errors.New("automation is disabled")
	}
	e.automationMu.Lock()
	prompt := e.automationPrompt
	e.automationMu.Unlock()
	if prompt.Kind != "confirm" || prompt.TunnelID != tunnelID || prompt.Action != action {
		return errors.New("nothing to confirm")
	}
	e.DismissAutomationPrompt()
	return e.applyAutomation(action, tunnelID, remember)
}

func (e *Engine) SyncAutomation() {
	if e == nil {
		return
	}
	on := e.FeatureOn(features.Automation)
	sync := e.syncScheme
	if sync == nil {
		sync = scheme.Sync
	}
	if err := sync(on); err != nil {
		slog.Warn("url scheme sync failed", "error", err)
	}
	// A daemon with background mode on keeps the channel open for the window
	// even when the Automation feature is off. Legacy commands still require it.
	if !e.hosting() || (!on && !e.windowIPC()) {
		e.stopAutomationIPC()
		return
	}
	if err := e.startAutomationIPC(); err != nil {
		slog.Error("automation ipc failed", "error", err)
	}
}

func (e *Engine) startAutomationIPC() error {
	if e.storage == nil {
		return errors.New("config is not ready")
	}
	e.ipcMu.Lock()
	defer e.ipcMu.Unlock()
	if e.ipcCancel != nil {
		return nil
	}
	configPath := e.storage.Path()
	token, err := ipc.EnsureToken(ipc.TokenPath(configPath))
	if err != nil {
		return err
	}
	address, err := ipc.Address(configPath)
	if err != nil {
		return err
	}
	ln, err := e.openListen(address)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.ipcCancel = cancel
	go func() {
		if err := ipc.ServeStream(ctx, ln, token, e.handleAutomation, e.handleSubscribe); err != nil {
			slog.Error("automation ipc stopped", "error", err)
		}
	}()
	slog.Info("automation ipc listening", "address", address)
	return nil
}

func (e *Engine) openListen(address string) (net.Listener, error) {
	if e.listen != nil {
		return e.listen(address)
	}
	return ipc.Listen(address)
}

func (e *Engine) stopAutomationIPC() {
	if e == nil {
		return
	}
	e.ipcMu.Lock()
	cancel := e.ipcCancel
	e.ipcCancel = nil
	e.ipcMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (e *Engine) handleAutomation(req ipc.Request) ipc.Response {
	locale := e.localeTag()
	if req.V >= 2 && ipc.IsV2Op(req.Op) && (e.FeatureOn(features.Automation) || e.windowIPC()) {
		return e.handleV2(req)
	}
	if !e.FeatureOn(features.Automation) {
		return ipc.Response{
			Code:     ipc.CodeDisabled,
			ExitCode: ipc.ExitDisabled,
			Message:  automation.Message(locale, ipc.CodeDisabled, "", "", nil),
		}
	}
	switch req.Op {
	case ipc.OpStatus, ipc.OpList:
		rt := e.active()
		if rt == nil {
			return ipc.Response{Code: ipc.CodeFailed, ExitCode: ipc.ExitFailed, Message: "app is not initialized"}
		}
		items, err := rt.List()
		if err != nil {
			return ipc.Response{Code: ipc.CodeFailed, ExitCode: ipc.ExitFailed, Message: err.Error()}
		}
		views := tunnelInfos(items)
		running := 0
		for _, item := range views {
			if item.Status == "running" || item.Status == "reconnecting" {
				running++
			}
		}
		return ipc.Response{
			OK:       true,
			Code:     ipc.CodeOK,
			ExitCode: ipc.ExitOK,
			Message:  automation.StatusLine(locale, len(views), running),
			Tunnels:  views,
		}
	case ipc.OpConnect, ipc.OpDisconnect, ipc.OpToggle:
		return e.automationTunnel(locale, req.Op, req.Target)
	default:
		return ipc.Response{
			Code:     ipc.CodeBadRequest,
			ExitCode: ipc.ExitUsage,
			Message:  automation.Message(locale, ipc.CodeBadRequest, "", "", nil),
		}
	}
}

func (e *Engine) automationTunnel(locale, op, target string) ipc.Response {
	cfg, err := e.storage.Load()
	if err != nil {
		return ipc.Response{Code: ipc.CodeFailed, ExitCode: ipc.ExitFailed, Message: err.Error()}
	}
	tunnel, err := automation.Match(cfg.Tunnels, target)
	if err != nil {
		var ambiguous *automation.AmbiguousError
		if errors.As(err, &ambiguous) {
			return ipc.Response{
				Code:     ipc.CodeAmbiguous,
				ExitCode: ipc.ExitAmbiguous,
				Message:  automation.Message(locale, ipc.CodeAmbiguous, target, "", ambiguous.Names()),
				Names:    ambiguous.Names(),
			}
		}
		return ipc.Response{
			Code:     ipc.CodeNotFound,
			ExitCode: ipc.ExitNotFound,
			Message:  automation.Message(locale, ipc.CodeNotFound, target, "", nil),
		}
	}
	updated, err := e.runAutomation(op, tunnel.ID)
	if err != nil {
		return ipc.Response{
			Code:     ipc.CodeFailed,
			ExitCode: ipc.ExitFailed,
			Message:  automation.Message(locale, ipc.CodeFailed, tunnel.Name, err.Error(), nil),
		}
	}
	if updated.Status == "error" && op != ipc.OpDisconnect {
		detail := updated.LastError
		return ipc.Response{
			Code:     ipc.CodeFailed,
			ExitCode: ipc.ExitFailed,
			Message:  automation.Message(locale, ipc.CodeFailed, tunnel.Name, detail, nil),
			Tunnels:  tunnelInfos([]model.Tunnel{updated}),
		}
	}
	code := "connected"
	if updated.Status == "stopped" || op == ipc.OpDisconnect {
		code = "disconnected"
	}
	e.tunnelsChanged()
	return ipc.Response{
		OK:       true,
		Code:     ipc.CodeOK,
		ExitCode: ipc.ExitOK,
		Message:  automation.Message(locale, code, updated.Name, "", nil),
		Tunnels:  tunnelInfos([]model.Tunnel{updated}),
	}
}

// SetTunnelRunner sends automation connect, disconnect and toggle to another owner.
// The window sets it while attached to norkad. Nil uses the in-process runtime.
func (e *Engine) SetTunnelRunner(fn func(op string, id int) (model.Tunnel, error)) {
	if e == nil {
		return
	}
	e.ownerMu.Lock()
	e.runner = fn
	e.ownerMu.Unlock()
}

func (e *Engine) runAutomation(op string, id int) (model.Tunnel, error) {
	e.ownerMu.Lock()
	runner := e.runner
	e.ownerMu.Unlock()
	if runner != nil {
		return runner(op, id)
	}
	rt := e.active()
	if rt == nil {
		return model.Tunnel{}, errors.New("app is not initialized")
	}
	switch op {
	case ipc.OpConnect:
		return rt.Start(id, e.TunnelStartLimit())
	case ipc.OpDisconnect:
		return rt.Stop(id)
	case ipc.OpToggle:
		return rt.Toggle(id, e.TunnelStartLimit())
	default:
		return model.Tunnel{}, errors.New("unsupported command")
	}
}

func (e *Engine) applyAutomation(action string, id int, remember bool) error {
	updated, err := e.runAutomation(action, id)
	if err != nil {
		return err
	}
	if updated.Status == "error" && action != "disconnect" {
		if updated.LastError != "" {
			return errors.New(updated.LastError)
		}
		return errors.New("tunnel command failed")
	}
	if remember {
		if _, err := e.storage.Update(func(cfg *conf.Config) error {
			cfg.TrustAutomationTunnel(id)
			return nil
		}); err != nil {
			return err
		}
	}
	e.tunnelsChanged()
	return nil
}

// HandleDeepLink accepts a norka:// URL forwarded by the single-instance lock
// or present on the first launch. Connect and disconnect ask the first time
// for each tunnel. Open only focuses that tunnel.
func (e *Engine) HandleDeepLink(raw string) {
	if e == nil {
		return
	}
	locale := e.localeTag()
	if !e.FeatureOn(features.Automation) {
		e.showWindow()
		e.publishAutomation(AutomationPrompt{Kind: "notice", Code: ipc.CodeDisabled, Detail: automation.Message(locale, ipc.CodeDisabled, "", "", nil)})
		return
	}
	link, err := automation.ParseLink(raw)
	if err != nil {
		e.showWindow()
		e.publishAutomation(AutomationPrompt{Kind: "notice", Code: "invalid"})
		return
	}
	if e.storage == nil {
		return
	}
	cfg, err := e.storage.Load()
	if err != nil {
		e.showWindow()
		e.publishAutomation(AutomationPrompt{Kind: "notice", Code: ipc.CodeFailed, Detail: err.Error()})
		return
	}
	tunnel, err := automation.Match(cfg.Tunnels, link.Name)
	if err != nil {
		e.showWindow()
		var ambiguous *automation.AmbiguousError
		if errors.As(err, &ambiguous) {
			e.publishAutomation(AutomationPrompt{Kind: "notice", Code: ipc.CodeAmbiguous, Name: link.Name, Names: ambiguous.Names()})
			return
		}
		e.publishAutomation(AutomationPrompt{Kind: "notice", Code: ipc.CodeNotFound, Name: link.Name})
		return
	}
	if link.Action == "open" {
		e.DismissAutomationPrompt()
		e.FocusTunnel(tunnel.ID)
		return
	}
	if cfg.AutomationAllows(tunnel.ID) {
		if err := e.applyAutomation(link.Action, tunnel.ID, false); err != nil {
			e.showWindow()
			e.publishAutomation(AutomationPrompt{Kind: "notice", Code: ipc.CodeFailed, Name: tunnel.Name, Detail: err.Error()})
		}
		return
	}
	e.showWindow()
	e.publishAutomation(AutomationPrompt{
		Kind:     "confirm",
		Code:     link.Action,
		Action:   link.Action,
		TunnelID: tunnel.ID,
		Name:     tunnel.Name,
	})
}

func (e *Engine) publishAutomation(prompt AutomationPrompt) {
	e.automationMu.Lock()
	e.automationPrompt = prompt
	e.automationMu.Unlock()
	e.emit(EventAutomationPrompt, prompt)
}

func tunnelInfos(items []model.Tunnel) []ipc.TunnelInfo {
	out := make([]ipc.TunnelInfo, 0, len(items))
	for _, item := range items {
		out = append(out, ipc.TunnelInfo{
			ID:         item.ID,
			Name:       item.Name,
			Status:     item.Status,
			Mode:       item.Mode,
			LocalHost:  item.LocalHost,
			LocalPort:  item.LocalPort,
			RemoteHost: item.RemoteHost,
			RemotePort: item.RemotePort,
			LastError:  item.LastError,
		})
	}
	return out
}
