package main

import (
	"runtime"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Собственная строка заголовка (frontend/src/components/layout/AppTitleBar.vue) — только на Windows:
// там окно без системной рамки (options.App.Frameless). На macOS остаётся нативный заголовок
// с кнопками окна, на Linux — без изменений.
const customTitleBar = runtime.GOOS == "windows"

// событие для фронтенда: окно показано (true) или спрятано в трей (false)
const eventWindowVisibility = "window:visibility"

// HasCustomTitleBar сообщает фронтенду, рисовать ли собственную строку заголовка.
func (a *App) HasCustomTitleBar() bool {
	return customTitleBar
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

// closeHidesToTray: Windows — пока не выбран «Выход» (beforeClose), macOS — HideWindowOnClose.
func (a *App) closeHidesToTray() bool {
	switch runtime.GOOS {
	case "windows":
		return !a.allowClose.Load()
	case "darwin":
		return true
	default:
		return false
	}
}

func (a *App) hideMainWindow() {
	wailsruntime.Hide(a.ctx)
	wailsruntime.WindowHide(a.ctx)
	wailsruntime.EventsEmit(a.ctx, eventWindowVisibility, false)
}

// showMainWindow показывает окно из трея / при повторном запуске.
func (a *App) showMainWindow() {
	if a.ctx == nil {
		return
	}
	wailsruntime.Show(a.ctx)
	wailsruntime.WindowShow(a.ctx)
	wailsruntime.WindowUnminimise(a.ctx)
	wailsruntime.EventsEmit(a.ctx, eventWindowVisibility, true)
}
