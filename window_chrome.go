package main

import (
	"runtime"

	"github.com/norka-app/Norka/internal/features"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Собственная строка заголовка (frontend/src/components/layout/AppTitleBar.vue) — только на Windows:
// там окно без системной рамки (options.App.Frameless). На macOS и Linux остаётся нативный
// заголовок с кнопками оконного менеджера.
const customTitleBar = runtime.GOOS == "windows"

// событие для фронтенда: окно показано (true) или спрятано в трей (false)
const eventWindowVisibility = "window:visibility"

// HasCustomTitleBar сообщает фронтенду, рисовать ли собственную строку заголовка.
func (a *App) HasCustomTitleBar() bool {
	return customTitleBar
}

// startupChrome is fixed before wails.Run. GetWindowChrome returns it so the
// frontend can paint the frameless look only when this process actually
// opened that frame.
var startupChrome ChromePlan

func (a *App) loadStartupChrome() {
	on := false
	if a != nil {
		on = a.featureOn(features.FramelessWindow)
	}
	startupChrome = planWindowChrome(runtime.GOOS, on, micaSupported())
}

// WindowChrome is the frame this process opened with.
type WindowChrome struct {
	Frameless      bool   `json:"frameless"`
	Platform       string `json:"platform"`
	Backdrop       string `json:"backdrop"`
	CustomTitleBar bool   `json:"customTitleBar"`
}

// GetWindowChrome reports the frame chosen at startup. Toggling the flag
// does not change it until the app starts again.
func (a *App) GetWindowChrome() WindowChrome {
	return WindowChrome{
		Frameless:      startupChrome.Frameless,
		Platform:       startupChrome.Platform,
		Backdrop:       startupChrome.Backdrop,
		CustomTitleBar: startupChrome.CustomTitleBar,
	}
}

// CloseMainWindow — кнопка «Закрыть» собственной строки заголовка, ведёт себя как системный крестик:
// прячет окно в трей там, где так делает закрытие окна, иначе завершает приложение.
func (a *App) CloseMainWindow() {
	if a.ctx == nil {
		return
	}
	if a.closeHidesToTray() {
		a.hideMainWindow()
		return
	}
	wailsruntime.Quit(a.ctx)
}

// closeHidesToTray: Windows и Linux прячут окно, пока не выбран «Выход» (beforeClose).
// macOS прячет его через HideWindowOnClose.
func (a *App) closeHidesToTray() bool {
	switch runtime.GOOS {
	case "windows", "linux":
		return !a.allowClose.Load()
	case "darwin":
		return true
	default:
		return false
	}
}

func (a *App) hideMainWindow() {
	a.windowVisible.Store(false)
	wailsruntime.Hide(a.ctx)
	wailsruntime.WindowHide(a.ctx)
	wailsruntime.EventsEmit(a.ctx, eventWindowVisibility, false)
}

// showMainWindow показывает окно из трея / при повторном запуске.
func (a *App) showMainWindow() {
	if a.ctx == nil {
		return
	}
	a.windowVisible.Store(true)
	wailsruntime.Show(a.ctx)
	wailsruntime.WindowShow(a.ctx)
	wailsruntime.WindowUnminimise(a.ctx)
	wailsruntime.EventsEmit(a.ctx, eventWindowVisibility, true)
}
