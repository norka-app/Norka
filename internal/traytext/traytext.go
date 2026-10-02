package traytext

import (
	"embed"
	"encoding/json"
	"reflect"
	"strings"
	"sync"

	"norka/internal/uilocale"
)

//go:embed tray.json
var trayFS embed.FS

// Strings holds user-visible tray, tooltip, and native-dialog labels for one locale.
type Strings struct {
	ShowMainTitle       string `json:"showMainTitle"`
	ShowMainTooltip     string `json:"showMainTooltip"`
	QuitTitle           string `json:"quitTitle"`
	QuitTooltip         string `json:"quitTooltip"`
	IconTooltip         string `json:"iconTooltip"`
	AppTitle            string `json:"appTitle"`
	CopyAddress         string `json:"copyAddress"`
	OpenBrowser         string `json:"openBrowser"`
	MoreTunnels         string `json:"moreTunnels"`
	MoreTooltip         string `json:"moreTooltip"`
	StopAll             string `json:"stopAll"`
	StopAllTooltip      string `json:"stopAllTooltip"`
	RetryFailed         string `json:"retryFailed"`
	RetryFailedTooltip  string `json:"retryFailedTooltip"`
	SimpleMode          string `json:"simpleMode"`
	SimpleModeTooltip   string `json:"simpleModeTooltip"`
	AdvancedMode        string `json:"advancedMode"`
	AdvancedModeTooltip string `json:"advancedModeTooltip"`
	Profiles            string `json:"profiles"`
	ProfilesTooltip     string `json:"profilesTooltip"`
	ProfileNone         string `json:"profileNone"`
	ProfileNoneTooltip  string `json:"profileNoneTooltip"`
	NoProfiles          string `json:"noProfiles"`
	MoreProfiles        string `json:"moreProfiles"`
	HeaderNone          string `json:"headerNone"`
	HeaderCount         string `json:"headerCount"`
	TooltipConnecting   string `json:"tooltipConnecting"`
	TooltipErrors       string `json:"tooltipErrors"`
	ToggleDisconnect    string `json:"toggleDisconnect"`
	StatusReconnecting  string `json:"statusReconnecting"`
	StatusConnecting    string `json:"statusConnecting"`
	ToggleCancel        string `json:"toggleCancel"`
	StatusError         string `json:"statusError"`
	ToggleRetry         string `json:"toggleRetry"`
	ToggleConnect       string `json:"toggleConnect"`
	ToggleInstead       string `json:"toggleInstead"`
	PortBusyOne         string `json:"portBusyOne"`
	PortBusyMany        string `json:"portBusyMany"`
	SelectConfigDir     string `json:"selectConfigDir"`
	ExportConfigTitle   string `json:"exportConfigTitle"`
	ImportConfigTitle   string `json:"importConfigTitle"`
	TomlFilter          string `json:"tomlFilter"`
}

var (
	trayOnce     sync.Once
	trayByLocale map[string]Strings
)

func load() {
	data, err := trayFS.ReadFile("tray.json")
	if err != nil {
		trayByLocale = map[string]Strings{
			"ru": fallbackRussian(),
			"en": fallbackEnglish(),
		}
		return
	}
	var m map[string]Strings
	if err := json.Unmarshal(data, &m); err != nil || len(m) == 0 {
		trayByLocale = map[string]Strings{
			"ru": fallbackRussian(),
			"en": fallbackEnglish(),
		}
		return
	}
	trayByLocale = m
}

func fallbackRussian() Strings {
	return Strings{
		ShowMainTitle:       "Показать окно",
		ShowMainTooltip:     "Вывести окно приложения на передний план",
		QuitTitle:           "Выход",
		QuitTooltip:         "Закрыть приложение",
		IconTooltip:         "Norka",
		AppTitle:            "Norka",
		CopyAddress:         "Скопировать адрес (%s)",
		OpenBrowser:         "Открыть в браузере",
		MoreTunnels:         "Ещё туннелей: %d…",
		MoreTooltip:         "Открыть список туннелей",
		StopAll:             "Отключить все",
		StopAllTooltip:      "Остановить все активные туннели",
		RetryFailed:         "Переподключить ошибочные",
		RetryFailedTooltip:  "Повторить запуск туннелей с ошибкой",
		SimpleMode:          "Простой режим",
		SimpleModeTooltip:   "Компактное окно на один туннель",
		AdvancedMode:        "Расширенный режим",
		AdvancedModeTooltip: "Полное окно со списком туннелей",
		Profiles:            "Профили",
		ProfilesTooltip:     "Подключить набор туннелей",
		ProfileNone:         "Не выбран",
		ProfileNoneTooltip:  "Снять активный профиль, туннели не останавливать",
		NoProfiles:          "Нет профилей",
		MoreProfiles:        "Ещё профилей: %d…",
		HeaderNone:          "Norka · нет туннелей",
		HeaderCount:         "Norka · %d из %d подключено",
		TooltipConnecting:   " · подключение…",
		TooltipErrors:       " · ошибок: %d",
		ToggleDisconnect:    "Отключить",
		StatusReconnecting:  " — переподключение…",
		StatusConnecting:    " — подключение…",
		ToggleCancel:        "Отменить подключение",
		StatusError:         " — ошибка",
		ToggleRetry:         "Повторить",
		ToggleConnect:       "Подключить",
		ToggleInstead:       "Подключить вместо «%s»",
		PortBusyOne:         "Порт %d занят: «%s» будет отключён",
		PortBusyMany:        "Порт %d занят: «%s» будут отключены",
		SelectConfigDir:     "Выберите каталог конфигурации",
		ExportConfigTitle:   "Экспорт конфигурации",
		ImportConfigTitle:   "Импорт конфигурации",
		TomlFilter:          "Конфигурация TOML (*.toml)",
	}
}

func fallbackEnglish() Strings {
	return Strings{
		ShowMainTitle:       "Show window",
		ShowMainTooltip:     "Bring the window to the front",
		QuitTitle:           "Quit",
		QuitTooltip:         "Quit the application",
		IconTooltip:         "Norka",
		AppTitle:            "Norka",
		CopyAddress:         "Copy address (%s)",
		OpenBrowser:         "Open in browser",
		MoreTunnels:         "%d more tunnels…",
		MoreTooltip:         "Open the tunnel list",
		StopAll:             "Disconnect all",
		StopAllTooltip:      "Stop every active tunnel",
		RetryFailed:         "Retry failed",
		RetryFailedTooltip:  "Start tunnels that failed",
		SimpleMode:          "Simple mode",
		SimpleModeTooltip:   "Compact window for one tunnel",
		AdvancedMode:        "Advanced mode",
		AdvancedModeTooltip: "Full window with the tunnel list",
		Profiles:            "Profiles",
		ProfilesTooltip:     "Connect a set of tunnels",
		ProfileNone:         "None",
		ProfileNoneTooltip:  "Clear the active profile without disconnecting tunnels",
		NoProfiles:          "No profiles",
		MoreProfiles:        "%d more profiles…",
		HeaderNone:          "Norka · no tunnels",
		HeaderCount:         "Norka · %d of %d connected",
		TooltipConnecting:   " · connecting…",
		TooltipErrors:       " · errors: %d",
		ToggleDisconnect:    "Disconnect",
		StatusReconnecting:  " — reconnecting…",
		StatusConnecting:    " — connecting…",
		ToggleCancel:        "Cancel connection",
		StatusError:         " — error",
		ToggleRetry:         "Retry",
		ToggleConnect:       "Connect",
		ToggleInstead:       "Connect instead of “%s”",
		PortBusyOne:         "Port %d is in use: “%s” will be disconnected",
		PortBusyMany:        "Port %d is in use: “%s” will be disconnected",
		SelectConfigDir:     "Select configuration folder",
		ExportConfigTitle:   "Export configuration",
		ImportConfigTitle:   "Import configuration",
		TomlFilter:          "TOML config (*.toml)",
	}
}

// ForLocale returns tray and dialog labels for ru or en.
func ForLocale(locale string) Strings {
	trayOnce.Do(load)
	tag := uilocale.Normalize(locale)
	if s, ok := trayByLocale[tag]; ok && s.ShowMainTitle != "" {
		return s
	}
	if tag == uilocale.PrefRU {
		return fallbackRussian()
	}
	return fallbackEnglish()
}

// Complete reports whether every string field is non-empty.
func (s Strings) Complete() bool {
	v := reflect.ValueOf(s)
	for i := 0; i < v.NumField(); i++ {
		if strings.TrimSpace(v.Field(i).String()) == "" {
			return false
		}
	}
	return true
}
