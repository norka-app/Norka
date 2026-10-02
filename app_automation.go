package main

import (
	"context"
	"errors"
	"log/slog"

	"norka/internal/automation"
	"norka/internal/conf"
	"norka/internal/features"
	"norka/internal/ipc"
	"norka/internal/model"
	"norka/internal/scheme"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const eventAutomationPrompt = "automation:prompt"

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

// SetStartHidden keeps the window in the tray when the CLI launched the app.
func (a *App) SetStartHidden(hidden bool) {
	if a == nil {
		return
	}
	a.startHidden = hidden
	if hidden {
		a.windowVisible.Store(false)
	}
}

// GetAutomationPrompt returns a deep-link prompt that arrived before the UI subscribed.
func (a *App) GetAutomationPrompt() AutomationPrompt {
	if a == nil {
		return AutomationPrompt{}
	}
	a.automationMu.Lock()
	defer a.automationMu.Unlock()
	return a.automationPrompt
}

// DismissAutomationPrompt drops a prompt the user closed without confirming.
func (a *App) DismissAutomationPrompt() {
	if a == nil {
		return
	}
	a.automationMu.Lock()
	a.automationPrompt = AutomationPrompt{}
	a.automationMu.Unlock()
}

// ConfirmAutomation runs a connect or disconnect the user just approved.
// remember stores the tunnel so the next link does not ask again.
func (a *App) ConfirmAutomation(tunnelID int, action string, remember bool) error {
	if a == nil {
		return errors.New("app is not initialized")
	}
	if !a.featureOn(features.Automation) {
		return errors.New("automation is disabled")
	}
	a.automationMu.Lock()
	prompt := a.automationPrompt
	a.automationMu.Unlock()
	if prompt.Kind != "confirm" || prompt.TunnelID != tunnelID || prompt.Action != action {
		return errors.New("nothing to confirm")
	}
	a.DismissAutomationPrompt()
	return a.applyAutomation(action, tunnelID, remember)
}

func (a *App) syncAutomation() {
	if a == nil {
		return
	}
	on := a.featureOn(features.Automation)
	if err := scheme.Sync(on); err != nil {
		slog.Warn("url scheme sync failed", "error", err)
	}
	if !on {
		a.stopAutomationIPC()
		return
	}
	if err := a.startAutomationIPC(); err != nil {
		slog.Error("automation ipc failed", "error", err)
	}
}

func (a *App) startAutomationIPC() error {
	if a.storage == nil {
		return errors.New("config is not ready")
	}
	a.ipcMu.Lock()
	defer a.ipcMu.Unlock()
	if a.ipcCancel != nil {
		return nil
	}
	configPath := a.storage.Path()
	token, err := ipc.EnsureToken(ipc.TokenPath(configPath))
	if err != nil {
		return err
	}
	address, err := ipc.Address(configPath)
	if err != nil {
		return err
	}
	ln, err := ipc.Listen(address)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.ipcCancel = cancel
	go func() {
		if err := ipc.Serve(ctx, ln, token, a.handleAutomation); err != nil {
			slog.Error("automation ipc stopped", "error", err)
		}
	}()
	slog.Info("automation ipc listening", "address", address)
	return nil
}

func (a *App) stopAutomationIPC() {
	if a == nil {
		return
	}
	a.ipcMu.Lock()
	cancel := a.ipcCancel
	a.ipcCancel = nil
	a.ipcMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) handleAutomation(req ipc.Request) ipc.Response {
	locale := a.uiLocaleTag()
	if !a.featureOn(features.Automation) {
		return ipc.Response{
			Code:     ipc.CodeDisabled,
			ExitCode: ipc.ExitDisabled,
			Message:  automation.Message(locale, ipc.CodeDisabled, "", "", nil),
		}
	}
	switch req.Op {
	case ipc.OpStatus, ipc.OpList:
		items, err := a.tunnel.List()
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
		return a.automationTunnel(locale, req.Op, req.Target)
	default:
		return ipc.Response{
			Code:     ipc.CodeBadRequest,
			ExitCode: ipc.ExitUsage,
			Message:  automation.Message(locale, ipc.CodeBadRequest, "", "", nil),
		}
	}
}

func (a *App) automationTunnel(locale, op, target string) ipc.Response {
	cfg, err := a.storage.Load()
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
	updated, err := a.runAutomation(op, tunnel.ID)
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
	a.afterTrayAction()
	return ipc.Response{
		OK:       true,
		Code:     ipc.CodeOK,
		ExitCode: ipc.ExitOK,
		Message:  automation.Message(locale, code, updated.Name, "", nil),
		Tunnels:  tunnelInfos([]model.Tunnel{updated}),
	}
}

func (a *App) runAutomation(op string, id int) (model.Tunnel, error) {
	switch op {
	case ipc.OpConnect:
		return a.tunnel.Start(id, a.tunnelStartLimit())
	case ipc.OpDisconnect:
		return a.tunnel.Stop(id)
	case ipc.OpToggle:
		return a.tunnel.Toggle(id, a.tunnelStartLimit())
	default:
		return model.Tunnel{}, errors.New("unsupported command")
	}
}

func (a *App) applyAutomation(action string, id int, remember bool) error {
	updated, err := a.runAutomation(action, id)
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
		if _, err := a.storage.Update(func(cfg *conf.Config) error {
			cfg.TrustAutomationTunnel(id)
			return nil
		}); err != nil {
			return err
		}
	}
	a.afterTrayAction()
	return nil
}

// HandleDeepLink accepts a norka:// URL forwarded by the single-instance lock
// or present on the first launch. Connect and disconnect ask the first time
// for each tunnel. Open only focuses that tunnel.
func (a *App) HandleDeepLink(raw string) {
	if a == nil {
		return
	}
	locale := a.uiLocaleTag()
	if !a.featureOn(features.Automation) {
		a.showMainWindow()
		a.publishAutomation(AutomationPrompt{Kind: "notice", Code: ipc.CodeDisabled, Detail: automation.Message(locale, ipc.CodeDisabled, "", "", nil)})
		return
	}
	link, err := automation.ParseLink(raw)
	if err != nil {
		a.showMainWindow()
		a.publishAutomation(AutomationPrompt{Kind: "notice", Code: "invalid"})
		return
	}
	if a.storage == nil {
		return
	}
	cfg, err := a.storage.Load()
	if err != nil {
		a.showMainWindow()
		a.publishAutomation(AutomationPrompt{Kind: "notice", Code: ipc.CodeFailed, Detail: err.Error()})
		return
	}
	tunnel, err := automation.Match(cfg.Tunnels, link.Name)
	if err != nil {
		a.showMainWindow()
		var ambiguous *automation.AmbiguousError
		if errors.As(err, &ambiguous) {
			a.publishAutomation(AutomationPrompt{Kind: "notice", Code: ipc.CodeAmbiguous, Name: link.Name, Names: ambiguous.Names()})
			return
		}
		a.publishAutomation(AutomationPrompt{Kind: "notice", Code: ipc.CodeNotFound, Name: link.Name})
		return
	}
	if link.Action == "open" {
		a.DismissAutomationPrompt()
		a.FocusTunnel(tunnel.ID)
		return
	}
	if cfg.AutomationAllows(tunnel.ID) {
		if err := a.applyAutomation(link.Action, tunnel.ID, false); err != nil {
			a.showMainWindow()
			a.publishAutomation(AutomationPrompt{Kind: "notice", Code: ipc.CodeFailed, Name: tunnel.Name, Detail: err.Error()})
		}
		return
	}
	a.showMainWindow()
	a.publishAutomation(AutomationPrompt{
		Kind:     "confirm",
		Code:     link.Action,
		Action:   link.Action,
		TunnelID: tunnel.ID,
		Name:     tunnel.Name,
	})
}

func (a *App) publishAutomation(prompt AutomationPrompt) {
	a.automationMu.Lock()
	a.automationPrompt = prompt
	a.automationMu.Unlock()
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, eventAutomationPrompt, prompt)
	}
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
