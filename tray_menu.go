package main

import (
	"embed"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/energye/systray"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"norka/internal/features"
	"norka/internal/model"
	"norka/internal/traytext"
)

// Иконки трея по агрегированному статусу (connected / connecting / error / stopped).
//
//go:embed build/tray/*.ico build/tray/*-32.png
var trayStatusIcons embed.FS

const (
	trayRefreshInterval = 3 * time.Second
	// события для фронтенда (App.vue подписывается через EventsOn)
	eventTunnelsChanged = "tunnels:changed"
	eventWindowMode     = "window:mode"
	eventWindowPage     = "window:page"
)

// traySlot — заранее созданный пункт туннеля с подменю. Пункты не пересоздаются:
// у energye/systray на Windows ResetMenu не сбрасывает внутренние списки, поэтому
// меню обновляется через SetTitle / Show / Hide / Check, а порядок сохраняется.
type traySlot struct {
	item, toggle, copyAddr, open *systray.MenuItem
	id                           int
	address, url                 string
	switchPort                   bool // «Подключить вместо «…»»: сначала отключить туннель на этом порту
}

type trayProfileSlot struct {
	item *systray.MenuItem
	id   int
	more bool
}

type trayMenu struct {
	mu           sync.Mutex
	header       *systray.MenuItem
	slots        []*traySlot
	more         *systray.MenuItem
	stopAll      *systray.MenuItem
	retryAll     *systray.MenuItem
	simple       *systray.MenuItem
	advanced     *systray.MenuItem
	profiles     *systray.MenuItem
	profileNone  *systray.MenuItem
	profileSlots []*trayProfileSlot
	signature    string
	iconKey      string
	statusSince  map[int]traySince
	refreshOnce  sync.Once
}

// buildTrayMenu добавляет в меню трея статус, туннели и действия. Вызывается из onReady
// systray в main.go до пунктов «Показать окно» и «Выход».
func (a *App) buildTrayMenu(showWindow func()) {
	m := &a.trayMenu
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statusSince = map[int]traySince{}

	text := traytext.ForLocale(a.uiLocaleTag())
	m.header = systray.AddMenuItem("Norka", "")
	m.header.Disable()
	systray.AddSeparator()

	for i := 0; i < trayMaxTunnelItems; i++ {
		slot := &traySlot{item: systray.AddMenuItem("", "")}
		slot.toggle = slot.item.AddSubMenuItem("", "")
		slot.copyAddr = slot.item.AddSubMenuItem(fmt.Sprintf(text.CopyAddress, "…"), "")
		slot.open = slot.item.AddSubMenuItem(text.OpenBrowser, "")
		slot.toggle.Click(func() { a.trayToggle(slot) })
		slot.copyAddr.Click(func() { a.trayCopyAddress(slot) })
		slot.open.Click(func() { a.trayOpenBrowser(slot) })
		slot.item.Hide()
		m.slots = append(m.slots, slot)
	}
	m.more = systray.AddMenuItem("", text.MoreTooltip)
	m.more.Click(func() { a.trayShowMode(showWindow, "advanced") })
	m.more.Hide()

	systray.AddSeparator()
	m.stopAll = systray.AddMenuItem(text.StopAll, text.StopAllTooltip)
	m.stopAll.Click(func() {
		go a.trayToggleWhere(func(status string) bool {
			return status == "running" || status == "busy" || status == "reconnecting"
		})
	})
	m.retryAll = systray.AddMenuItem(text.RetryFailed, text.RetryFailedTooltip)
	m.retryAll.Click(func() { go a.trayRetryFailed() })
	m.retryAll.Hide()

	systray.AddSeparator()
	m.profiles = systray.AddMenuItem(text.Profiles, text.ProfilesTooltip)
	if !a.featureOn(features.Profiles) {
		m.profiles.Hide()
	}
	m.profileNone = m.profiles.AddSubMenuItem(text.ProfileNone, text.ProfileNoneTooltip)
	m.profileNone.Click(func() { go a.trayActivateProfile(0) })
	for i := 0; i < trayMaxProfileItems; i++ {
		slot := &trayProfileSlot{item: m.profiles.AddSubMenuItem("", "")}
		slot.item.Click(func() { a.trayUseProfileSlot(slot, showWindow) })
		slot.item.Hide()
		m.profileSlots = append(m.profileSlots, slot)
	}

	systray.AddSeparator()
	m.simple = systray.AddMenuItem(text.SimpleMode, text.SimpleModeTooltip)
	m.simple.Click(func() { a.trayShowMode(showWindow, "simple") })
	m.advanced = systray.AddMenuItem(text.AdvancedMode, text.AdvancedModeTooltip)
	m.advanced.Click(func() { a.trayShowMode(showWindow, "advanced") })

	a.startTrayRefresh()
}

// startTrayRefresh обновляет меню и иконку трея по состоянию туннелей.
// Бэкенд не шлёт событий о смене статуса, поэтому раз в trayRefreshInterval сверяемся
// со списком; меню перерисовывается только при изменениях.
func (a *App) startTrayRefresh() {
	a.trayMenu.refreshOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(trayRefreshInterval)
			defer ticker.Stop()
			for {
				a.refreshTrayMenu()
				<-ticker.C
			}
		}()
	})
}

func (a *App) refreshTrayMenu() {
	if a.ensureReady() != nil {
		return
	}
	tunnels, err := a.tunnel.List()
	if err != nil {
		return
	}
	text := traytext.ForLocale(a.uiLocaleTag())
	var profiles []model.Profile
	var activeProfileID int
	profilesOn := false
	if cfg, cfgErr := a.storage.Load(); cfgErr == nil {
		profilesOn = cfg.Features.Enabled(features.Profiles)
		if profilesOn {
			profiles = cfg.Profiles
			activeProfileID = cfg.ActiveProfileID
		}
	}
	m := &a.trayMenu
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.header == nil {
		return // меню трея ещё не создано
	}
	now := time.Now()
	trackTraySince(tunnels, m.statusSince, now)
	state := buildTrayModel(tunnels, m.statusSince, now, text)
	a.applyTrayStaticLabels(m, text)
	profileState := buildTrayProfiles(profiles, activeProfileID, text.MoreProfiles)
	if profileState.ActiveName != "" {
		state.Header = insertTrayProfile(state.Header, profileState.ActiveName)
		state.Tooltip = insertTrayProfile(state.Tooltip, profileState.ActiveName)
	}
	if key := state.iconKey(); key != m.iconKey {
		m.iconKey = key
		setTrayStatusIcon(key)
	}
	sig := state.signature() + "§" + profileState.signature() + "§" + fmt.Sprintf("%t", profilesOn)
	if sig == m.signature {
		return
	}
	m.signature = sig

	m.header.SetTitle(state.Header)
	systray.SetTooltip(state.Tooltip)
	for i, slot := range m.slots {
		if i >= len(state.Items) {
			slot.id = 0
			slot.item.Hide()
			continue
		}
		it := state.Items[i]
		slot.id, slot.address, slot.url, slot.switchPort = it.ID, it.Address, it.URL, it.SwitchPort
		slot.item.SetTitle(it.Title)
		if it.Running {
			slot.item.Check()
		} else {
			slot.item.Uncheck()
		}
		slot.toggle.SetTitle(it.ToggleLabel)
		slot.toggle.SetTooltip(it.ToggleTooltip) // подсказки у пунктов меню показывает только macOS
		slot.copyAddr.SetTitle(fmt.Sprintf(text.CopyAddress, it.Address))
		slot.open.SetTitle(text.OpenBrowser)
		if it.URL != "" {
			slot.open.Enable()
		} else {
			slot.open.Disable()
		}
		slot.item.Show()
	}
	if state.Hidden > 0 {
		m.more.SetTitle(fmt.Sprintf(text.MoreTunnels, state.Hidden))
		m.more.SetTooltip(text.MoreTooltip)
		m.more.Show()
	} else {
		m.more.Hide()
	}
	if state.CanStopAll {
		m.stopAll.Enable()
	} else {
		m.stopAll.Disable()
	}
	if state.CanRetryAll {
		m.retryAll.Show()
	} else {
		m.retryAll.Hide()
	}
	a.paintTrayProfiles(m, profileState, text.NoProfiles, profilesOn)
}

func (a *App) paintTrayProfiles(m *trayMenu, state trayProfileModel, emptyTitle string, visible bool) {
	if m.profiles == nil || m.profileNone == nil || len(m.profileSlots) == 0 {
		return
	}
	if !visible {
		m.profiles.Hide()
		return
	}
	m.profiles.Show()
	if state.ActiveID == 0 {
		m.profileNone.Check()
	} else {
		m.profileNone.Uncheck()
	}
	if state.Empty {
		slot := m.profileSlots[0]
		slot.id, slot.more = 0, false
		slot.item.SetTitle(emptyTitle)
		slot.item.Disable()
		slot.item.Uncheck()
		slot.item.Show()
		for _, extra := range m.profileSlots[1:] {
			extra.id, extra.more = 0, false
			extra.item.Hide()
		}
		return
	}
	for i, slot := range m.profileSlots {
		if i >= len(state.Items) {
			slot.id, slot.more = 0, false
			slot.item.Hide()
			continue
		}
		item := state.Items[i]
		slot.id, slot.more = item.ID, item.More
		slot.item.SetTitle(item.Title)
		slot.item.Enable()
		if item.Active {
			slot.item.Check()
		} else {
			slot.item.Uncheck()
		}
		slot.item.Show()
	}
}

func (a *App) trayUseProfileSlot(slot *trayProfileSlot, showWindow func()) {
	a.trayMenu.mu.Lock()
	id, more := slot.id, slot.more
	a.trayMenu.mu.Unlock()
	if more {
		a.trayShowProfiles(showWindow)
		return
	}
	if id == 0 {
		return
	}
	go a.trayActivateProfile(id)
}

// applyTrayStaticLabels updates menu items whose titles do not depend on a tunnel.
// Caller holds trayMenu.mu.
func (a *App) applyTrayStaticLabels(m *trayMenu, text traytext.Strings) {
	if m.stopAll != nil {
		m.stopAll.SetTitle(text.StopAll)
		m.stopAll.SetTooltip(text.StopAllTooltip)
	}
	if m.retryAll != nil {
		m.retryAll.SetTitle(text.RetryFailed)
		m.retryAll.SetTooltip(text.RetryFailedTooltip)
	}
	if m.simple != nil {
		m.simple.SetTitle(text.SimpleMode)
		m.simple.SetTooltip(text.SimpleModeTooltip)
	}
	if m.advanced != nil {
		m.advanced.SetTitle(text.AdvancedMode)
		m.advanced.SetTooltip(text.AdvancedModeTooltip)
	}
	if m.profiles != nil {
		m.profiles.SetTitle(text.Profiles)
		m.profiles.SetTooltip(text.ProfilesTooltip)
	}
	if m.profileNone != nil {
		m.profileNone.SetTitle(text.ProfileNone)
		m.profileNone.SetTooltip(text.ProfileNoneTooltip)
	}
}

// invalidateTrayMenu заставляет перерисовать меню (например, после смены подписи иконки
// при применении локали, которая сбрасывает подсказку к «Norka»).
func (a *App) invalidateTrayMenu() {
	a.trayMenu.mu.Lock()
	a.trayMenu.signature = ""
	a.trayMenu.mu.Unlock()
	go a.refreshTrayMenu()
}

func setTrayStatusIcon(key string) {
	var name string
	switch runtime.GOOS {
	case "windows":
		name = "build/tray/tray-" + key + ".ico"
	case "darwin":
		return // в строке меню macOS остаётся монохромная template-иконка
	default:
		name = "build/tray/tray-" + key + "-32.png"
	}
	data, err := trayStatusIcons.ReadFile(name)
	if err != nil {
		slog.Warn("tray icon missing", "name", name, "err", err)
		return
	}
	systray.SetIcon(data)
}

// afterTrayAction сразу обновляет трей и просит фронтенд перечитать состояние.
func (a *App) afterTrayAction() {
	a.refreshTrayMenu()
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, eventTunnelsChanged)
	}
}

func (a *App) trayToggle(slot *traySlot) {
	id, switchPort := slot.id, slot.switchPort
	if id == 0 {
		return
	}
	go func() {
		var err error
		if switchPort {
			// пункт «Подключить вместо «…»» — явный выбор: без вопроса, независимо от «Больше не спрашивать»
			err = a.switchTunnel(id)
		} else {
			_, err = a.ToggleTunnel(id)
		}
		if err != nil {
			slog.Error("tray toggle tunnel failed", "tunnel_id", id, "switch_port", switchPort, "err", err)
		}
		a.afterTrayAction()
	}()
}

// switchTunnel отключает туннели, занявшие локальный порт туннеля id, и запускает его (biz.SwitchTo).
func (a *App) switchTunnel(id int) error {
	if err := a.ensureReady(); err != nil {
		return err
	}
	_, err := a.tunnel.SwitchTo(id, a.tunnelStartLimit())
	return err
}

// trayRetryFailed — «Переподключить ошибочные»: повторяет запуск туннелей с ошибкой, кроме тех,
// чей порт держит другой туннель (trayRetryIDs).
func (a *App) trayRetryFailed() {
	if a.ensureReady() != nil {
		return
	}
	tunnels, err := a.tunnel.List()
	if err != nil {
		return
	}
	for _, id := range trayRetryIDs(tunnels) {
		if _, err := a.ToggleTunnel(id); err != nil {
			slog.Error("tray retry tunnel failed", "tunnel_id", id, "err", err)
		}
	}
	a.afterTrayAction()
}

// trayToggleWhere переключает все туннели с подходящим статусом («Отключить все» — активные).
func (a *App) trayToggleWhere(match func(status string) bool) {
	if a.ensureReady() != nil {
		return
	}
	tunnels, err := a.tunnel.List()
	if err != nil {
		return
	}
	for _, t := range tunnels {
		if !match(t.Status) {
			continue
		}
		if _, err := a.ToggleTunnel(t.ID); err != nil {
			slog.Error("tray toggle tunnel failed", "tunnel_id", t.ID, "err", err)
		}
	}
	a.afterTrayAction()
}

func (a *App) trayCopyAddress(slot *traySlot) {
	if a.ctx != nil && slot.address != "" {
		_ = wailsruntime.ClipboardSetText(a.ctx, slot.address)
	}
}

func (a *App) trayOpenBrowser(slot *traySlot) {
	if a.ctx != nil && slot.url != "" {
		wailsruntime.BrowserOpenURL(a.ctx, slot.url)
	}
}

// trayShowMode показывает окно и переключает его в простой или расширенный режим.
func (a *App) trayShowMode(showWindow func(), mode string) {
	if a.ctx == nil {
		return
	}
	showWindow()
	wailsruntime.EventsEmit(a.ctx, eventWindowMode, mode)
}
