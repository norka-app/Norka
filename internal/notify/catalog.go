package notify

import (
	"embed"
	"encoding/json"
	"sync"
)

//go:embed catalog.json
var catalogFS embed.FS

// Catalog holds OS notification copy for one locale.
type Catalog struct {
	Locale                string `json:"locale"`
	Title                 string `json:"title"`
	TunnelOne             string `json:"tunnelOne"`
	TunnelFew             string `json:"tunnelFew"`
	TunnelMany            string `json:"tunnelMany"`
	DroppedOne            string `json:"droppedOne"`
	DroppedSingular       string `json:"droppedSingular"`
	DroppedPlural         string `json:"droppedPlural"`
	ReconnectedOne        string `json:"reconnectedOne"`
	ReconnectedSingular   string `json:"reconnectedSingular"`
	ReconnectedPlural     string `json:"reconnectedPlural"`
	GaveUpOne             string `json:"gaveUpOne"`
	GaveUpSingular        string `json:"gaveUpSingular"`
	GaveUpPlural          string `json:"gaveUpPlural"`
	ConnectFailedOne      string `json:"connectFailedOne"`
	ConnectFailedSingular string `json:"connectFailedSingular"`
	ConnectFailedPlural   string `json:"connectFailedPlural"`
	ConnectedOne          string `json:"connectedOne"`
	ConnectedSingular     string `json:"connectedSingular"`
	ConnectedPlural       string `json:"connectedPlural"`
}

var (
	catalogOnce sync.Once
	catalogs    map[string]Catalog
)

func loadCatalogs() {
	catalogs = map[string]Catalog{}
	data, err := catalogFS.ReadFile("catalog.json")
	if err != nil {
		catalogs["ru"] = fallbackRussian()
		return
	}
	if err := json.Unmarshal(data, &catalogs); err != nil || len(catalogs) == 0 {
		catalogs = map[string]Catalog{"ru": fallbackRussian()}
	}
}

func fallbackRussian() Catalog {
	return Catalog{
		Locale:                "ru",
		Title:                 "Norka",
		TunnelOne:             "туннель",
		TunnelFew:             "туннеля",
		TunnelMany:            "туннелей",
		DroppedOne:            "Туннель «%s» отключился",
		DroppedSingular:       "%d %s отключился",
		DroppedPlural:         "%d %s отключились",
		ReconnectedOne:        "Туннель «%s» переподключён",
		ReconnectedSingular:   "%d %s переподключён",
		ReconnectedPlural:     "%d %s переподключены",
		GaveUpOne:             "Не удалось восстановить туннель «%s»",
		GaveUpSingular:        "Не удалось восстановить %d %s",
		GaveUpPlural:          "Не удалось восстановить %d %s",
		ConnectFailedOne:      "Не удалось подключить туннель «%s»",
		ConnectFailedSingular: "Не удалось подключить %d %s",
		ConnectFailedPlural:   "Не удалось подключить %d %s",
		ConnectedOne:          "Туннель «%s» подключён",
		ConnectedSingular:     "%d %s подключён",
		ConnectedPlural:       "%d %s подключены",
	}
}

// CatalogFor returns notification copy. Unknown locales fall back to Russian.
func CatalogFor(locale string) Catalog {
	catalogOnce.Do(loadCatalogs)
	if cat, ok := catalogs[locale]; ok && cat.Title != "" {
		return cat
	}
	if cat, ok := catalogs["ru"]; ok && cat.Title != "" {
		return cat
	}
	return fallbackRussian()
}
