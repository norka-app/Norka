package main

import (
	"errors"

	"github.com/norka-app/Norka/internal/engine"
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
	if a == nil || a.engine == nil {
		return AutomationPrompt{}
	}
	return automationPromptFromEngine(a.engine.AutomationPrompt())
}

// DismissAutomationPrompt drops a prompt the user closed without confirming.
func (a *App) DismissAutomationPrompt() {
	if a == nil || a.engine == nil {
		return
	}
	a.engine.DismissAutomationPrompt()
}

// ConfirmAutomation runs a connect or disconnect the user just approved.
// remember stores the tunnel so the next link does not ask again.
func (a *App) ConfirmAutomation(tunnelID int, action string, remember bool) error {
	if a == nil {
		return errors.New("app is not initialized")
	}
	if a.engine == nil {
		return errors.New("automation is disabled")
	}
	return a.engine.ConfirmAutomation(tunnelID, action, remember)
}

func (a *App) syncAutomation() {
	if a == nil || a.engine == nil {
		return
	}
	a.engine.SyncAutomation()
}

// HandleDeepLink accepts a norka:// URL forwarded by the single-instance lock
// or present on the first launch. Connect and disconnect ask the first time
// for each tunnel. Open only focuses that tunnel.
func (a *App) HandleDeepLink(raw string) {
	if a == nil || a.engine == nil {
		return
	}
	a.engine.HandleDeepLink(raw)
}

func automationPromptFromEngine(p engine.AutomationPrompt) AutomationPrompt {
	return AutomationPrompt{
		Kind:     p.Kind,
		Code:     p.Code,
		Action:   p.Action,
		TunnelID: p.TunnelID,
		Name:     p.Name,
		Names:    p.Names,
		Detail:   p.Detail,
	}
}
