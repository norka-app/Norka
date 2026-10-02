package traytext

import (
	"embed"
	"encoding/json"
	"sync"
)

//go:embed tray.json
var trayFS embed.FS

// Strings holds systray menu / icon labels for one locale (see tray.json).
type Strings struct {
	ShowMainTitle   string `json:"showMainTitle"`
	ShowMainTooltip string `json:"showMainTooltip"`
	QuitTitle       string `json:"quitTitle"`
	QuitTooltip     string `json:"quitTooltip"`
	IconTooltip     string `json:"iconTooltip"`
	AppTitle        string `json:"appTitle"`
}

var (
	trayOnce     sync.Once
	trayByLocale map[string]Strings
)

func load() {
	data, err := trayFS.ReadFile("tray.json")
	if err != nil {
		trayByLocale = map[string]Strings{"ru": fallbackRussian()}
		return
	}
	var m map[string]Strings
	if err := json.Unmarshal(data, &m); err != nil || len(m) == 0 {
		trayByLocale = map[string]Strings{
			"ru": fallbackRussian(),
		}
		return
	}
	trayByLocale = m
}

func fallbackRussian() Strings {
	return Strings{
		ShowMainTitle:   "Показать окно",
		ShowMainTooltip: "Вывести окно приложения на передний план",
		QuitTitle:       "Выход",
		QuitTooltip:     "Закрыть приложение",
		IconTooltip:     "Norka",
		AppTitle:        "Norka",
	}
}

// ForLocale returns the Russian tray labels. The interface has one language.
func ForLocale(locale string) Strings {
	_ = locale
	trayOnce.Do(load)
	if s, ok := trayByLocale["ru"]; ok && s.ShowMainTitle != "" {
		return s
	}
	return fallbackRussian()
}
