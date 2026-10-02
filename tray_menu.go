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

type trayMenu struct {
	mu          sync.Mutex
	header      *systray.MenuItem
	slots       []*traySlot
	more        *systray.MenuItem
	stopAll     *systray.MenuItem
	retryAll    *systray.MenuItem
	signature   string
	iconKey     string
	statusSince map[int]traySince
	refreshOnce sync.Once
}

// buildTrayMenu добавляет в меню трея статус, туннели и действия. Вызывается из onReady
// systray в main.go до пунктов «Показать окно» и «Выход».
func (a *App) buildTrayMenu(showWindow func()) {
	m := &a.trayMenu
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statusSince = map[int]traySince{}

	m.header = systray.AddMenuItem("Norka", "")
	m.header.Disable()
	systray.AddSeparator()

	for i := 0; i < trayMaxTunnelItems; i++ {
		slot := &traySlot{item: systray.AddMenuItem("", "")}
		slot.toggle = slot.item.AddSubMenuItem("", "")
		slot.copyAddr = slot.item.AddSubMenuItem("Скопировать адрес", "")
		slot.open = slot.item.AddSubMenuItem("Открыть в браузере", "")
		slot.toggle.Click(func() { a.trayToggle(slot) })
		slot.copyAddr.Click(func() { a.trayCopyAddress(slot) })
		slot.open.Click(func() { a.trayOpenBrowser(slot) })
		slot.item.Hide()
		m.slots = append(m.slots, slot)
	}
	m.more = systray.AddMenuItem("", "Открыть список туннелей")
	m.more.Click(func() { a.trayShowMode(showWindow, "advanced") })
	m.more.Hide()

	systray.AddSeparator()
	m.stopAll = systray.AddMenuItem("Отключить все", "Остановить все активные туннели")
	m.stopAll.Click(func() {
		go a.trayToggleWhere(func(status string) bool {
			return status == "running" || status == "busy" || status == "reconnecting"
		})
	})
	m.retryAll = systray.AddMenuItem("Переподключить ошибочные", "Повторить запуск туннелей с ошибкой")
	m.retryAll.Click(func() { go a.trayRetryFailed() })
	m.retryAll.Hide()

	systray.AddSeparator()
	simple := systray.AddMenuItem("Простой режим", "Компактное окно на один туннель")
	simple.Click(func() { a.trayShowMode(showWindow, "simple") })
	advanced := systray.AddMenuItem("Расширенный режим", "Полное окно со списком туннелей")
	advanced.Click(func() { a.trayShowMode(showWindow, "advanced") })

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
	m := &a.trayMenu
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.header == nil {
		return // меню трея ещё не создано
	}
	now := time.Now()
	trackTraySince(tunnels, m.statusSince, now)
	state := buildTrayModel(tunnels, m.statusSince, now)
	if key := state.iconKey(); key != m.iconKey {
		m.iconKey = key
		setTrayStatusIcon(key)
	}
	sig := state.signature()
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
		slot.copyAddr.SetTitle("Скопировать адрес (" + it.Address + ")")
		if it.URL != "" {
			slot.open.Enable()
		} else {
			slot.open.Disable()
		}
		slot.item.Show()
	}
	if state.Hidden > 0 {
		m.more.SetTitle(fmt.Sprintf("Ещё туннелей: %d…", state.Hidden))
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
