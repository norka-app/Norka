package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/norka-app/Norka/internal/biz"
	"github.com/norka-app/Norka/internal/model"
	"github.com/norka-app/Norka/internal/traytext"
)

// Пункты меню трея строятся из живого состояния туннелей (internal/biz → List()).
// Агрегированный статус совпадает с иконкой Norka в боковой панели
// (frontend/src/utils/norka-status.js): глаза одного цвета и показывают последнее событие —
// ошибка подключения → error, успешное подключение или остановка туннеля → connected
// (если что-то работает); без свежего события — connecting > error > connected > stopped.
// Ошибки старше trayStaleErrorAfter для иконки не считаются.
const (
	trayMaxTunnelItems  = 10
	trayMaxProfileItems = 12
	trayStaleErrorAfter = 5 * time.Minute
)

type trayTunnelItem struct {
	ID          int
	Title       string
	Running     bool // галочка у пункта
	ToggleLabel string
	// ToggleTooltip и SwitchPort — порт туннеля держит другой работающий/подключающийся туннель:
	// пункт «Подключить вместо «…»» сначала отключает тот туннель (biz.SwitchTo)
	ToggleTooltip string
	SwitchPort    bool
	Address       string // localhost:port — для «Скопировать адрес»
	URL           string // http://localhost:port — только для Local Forward в статусе running
}

type trayModel struct {
	Status      string // connected | connecting | error | stopped
	Header      string
	Tooltip     string
	Items       []trayTunnelItem
	Hidden      int // туннели, не поместившиеся в меню
	CanStopAll  bool
	CanRetryAll bool
}

func (m trayModel) iconKey() string {
	return m.Status
}

// signature меняется только когда меню нужно перерисовать.
func (m trayModel) signature() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s|%s|%d|%t|%t", m.Status, m.Header, m.Hidden, m.CanStopAll, m.CanRetryAll)
	for _, it := range m.Items {
		fmt.Fprintf(&b, "|%d:%s:%t:%s:%s:%t:%s:%s", it.ID, it.Title, it.Running, it.ToggleLabel, it.ToggleTooltip, it.SwitchPort, it.Address, it.URL)
	}
	return b.String()
}

func trayDisplayHost(host string) string {
	switch strings.TrimSpace(host) {
	case "", "127.0.0.1", "0.0.0.0", "::1", "localhost":
		return "localhost"
	default:
		return strings.TrimSpace(host)
	}
}

// traySince — текущий статус туннеля и момент, когда трей увидел переход в него.
type traySince struct {
	Status string
	At     time.Time
}

// trackTraySince запоминает, когда каждый туннель перешёл в свой текущий статус: по этому
// времени считаются устаревшие ошибки и выбирается последнее событие для иконки.
func trackTraySince(tunnels []model.Tunnel, since map[int]traySince, now time.Time) {
	seen := make(map[int]bool, len(tunnels))
	for _, t := range tunnels {
		seen[t.ID] = true
		if prev, ok := since[t.ID]; !ok || prev.Status != t.Status {
			since[t.ID] = traySince{Status: t.Status, At: now}
		}
	}
	for id := range since {
		if !seen[id] {
			delete(since, id)
		}
	}
}

func buildTrayModel(tunnels []model.Tunnel, since map[int]traySince, now time.Time, text traytext.Strings) trayModel {
	var running, busy, failed int
	// последнее событие: время и было ли среди одновременных событий ошибка
	var lastAt time.Time
	var hasLast, lastError bool
	note := func(t model.Tunnel, isError bool) {
		s, ok := since[t.ID]
		if !ok || s.Status != t.Status {
			return
		}
		switch {
		case !hasLast || s.At.After(lastAt):
			lastAt, hasLast, lastError = s.At, true, isError
		case s.At.Equal(lastAt):
			lastError = lastError || isError
		}
	}
	m := trayModel{}
	for _, t := range tunnels {
		switch t.Status {
		case "running":
			running++
			m.CanStopAll = true
			note(t, false)
		case "busy", "reconnecting":
			busy++
			m.CanStopAll = true
		case "error":
			if s, ok := since[t.ID]; ok && s.Status == "error" && now.Sub(s.At) > trayStaleErrorAfter {
				continue // устаревшая ошибка иконку не красит
			}
			failed++
			note(t, true)
		case "stopped":
			note(t, false)
		}
	}
	m.CanRetryAll = len(trayRetryIDs(tunnels)) > 0
	switch {
	case busy > 0:
		m.Status = "connecting"
	case failed > 0 && running > 0:
		// глаза не бывают разными: ошибка — последнее событие (или событий нет) → error,
		// успешное подключение или остановка туннеля после ошибки → connected
		if !hasLast || lastError {
			m.Status = "error"
		} else {
			m.Status = "connected"
		}
	case failed > 0:
		m.Status = "error"
	case running > 0:
		m.Status = "connected"
	default:
		m.Status = "stopped"
	}

	switch {
	case len(tunnels) == 0:
		m.Header = text.HeaderNone
	default:
		m.Header = fmt.Sprintf(text.HeaderCount, running, len(tunnels))
	}
	m.Tooltip = m.Header
	switch {
	case m.Status == "connecting":
		m.Tooltip += text.TooltipConnecting
	case failed > 0:
		m.Tooltip += fmt.Sprintf(text.TooltipErrors, failed)
	}

	for i, t := range tunnels {
		if i >= trayMaxTunnelItems {
			m.Hidden = len(tunnels) - trayMaxTunnelItems
			break
		}
		addr := fmt.Sprintf("%s:%d", trayDisplayHost(t.LocalHost), t.LocalPort)
		item := trayTunnelItem{
			ID:      t.ID,
			Title:   fmt.Sprintf("%s · %d", t.Name, t.LocalPort),
			Running: t.Status == "running",
			Address: addr,
		}
		switch t.Status {
		case "running":
			item.ToggleLabel = text.ToggleDisconnect
			if t.Mode == "local" || t.Mode == "" {
				item.URL = "http://" + addr
			}
		case "reconnecting":
			item.Title += text.StatusReconnecting
			item.ToggleLabel = text.ToggleDisconnect
		case "busy":
			item.Title += text.StatusConnecting
			item.ToggleLabel = text.ToggleCancel
		case "error":
			item.Title += text.StatusError
			item.ToggleLabel = text.ToggleRetry
			setTraySwitchPort(&item, t, tunnels, text)
		default:
			item.ToggleLabel = text.ToggleConnect
			setTraySwitchPort(&item, t, tunnels, text)
		}
		m.Items = append(m.Items, item)
	}
	return m
}

// setTraySwitchPort меняет «Подключить» / «Повторить» на «Подключить вместо «…»», если локальный
// порт туннеля держит другой туннель: запуск всё равно не смог бы занять порт.
func setTraySwitchPort(item *trayTunnelItem, t model.Tunnel, tunnels []model.Tunnel, text traytext.Strings) {
	conflicts := biz.PortConflicts(t, tunnels)
	if len(conflicts) == 0 {
		return
	}
	names := make([]string, len(conflicts))
	for i, c := range conflicts {
		names[i] = c.Name
	}
	joined := strings.Join(names, "», «")
	item.SwitchPort = true
	item.ToggleLabel = fmt.Sprintf(text.ToggleInstead, joined)
	if len(conflicts) == 1 {
		item.ToggleTooltip = fmt.Sprintf(text.PortBusyOne, t.LocalPort, joined)
	} else {
		item.ToggleTooltip = fmt.Sprintf(text.PortBusyMany, t.LocalPort, joined)
	}
}

// trayRetryIDs — туннели для «Переподключить ошибочные». Туннель, чей порт держит другой
// работающий или подключающийся туннель, пропускаем: чужой туннель отключается только явным
// «Подключить вместо «…»». Из нескольких ошибочных туннелей на одном порту запускается первый.
func trayRetryIDs(tunnels []model.Tunnel) []int {
	planned := append([]model.Tunnel(nil), tunnels...)
	var ids []int
	for i, t := range planned {
		if t.Status != "error" || len(biz.PortConflicts(t, planned)) > 0 {
			continue
		}
		ids = append(ids, t.ID)
		planned[i].Status = "busy" // будет запущен — его порт занят для следующих
	}
	return ids
}

type trayProfileItem struct {
	ID     int
	Title  string
	Active bool
	More   bool
}

type trayProfileModel struct {
	ActiveID   int
	ActiveName string
	Items      []trayProfileItem
	Empty      bool
}

func (m trayProfileModel) signature() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d|%s|%t", m.ActiveID, m.ActiveName, m.Empty)
	for _, it := range m.Items {
		fmt.Fprintf(&b, "|%d:%s:%t:%t", it.ID, it.Title, it.Active, it.More)
	}
	return b.String()
}

// buildTrayProfiles puts the active profile first so it stays visible, then
// the rest in config order. The last slot becomes «Ещё профилей», when the
// list does not fit.
func buildTrayProfiles(profiles []model.Profile, activeID int, moreFormat string) trayProfileModel {
	modelOut := trayProfileModel{ActiveID: activeID}
	if len(profiles) == 0 {
		modelOut.Empty = true
		return modelOut
	}
	ordered := make([]model.Profile, 0, len(profiles))
	var active []model.Profile
	for _, profile := range profiles {
		if profile.ID == activeID {
			modelOut.ActiveName = profileTrayTitle(profile)
			active = append(active, profile)
			continue
		}
		ordered = append(ordered, profile)
	}
	ordered = append(active, ordered...)

	limit := len(ordered)
	hidden := 0
	if len(ordered) > trayMaxProfileItems {
		limit = trayMaxProfileItems - 1
		hidden = len(ordered) - limit
	}
	for i := 0; i < limit; i++ {
		profile := ordered[i]
		modelOut.Items = append(modelOut.Items, trayProfileItem{
			ID:     profile.ID,
			Title:  profileTrayTitle(profile),
			Active: profile.ID == activeID,
		})
	}
	if hidden > 0 {
		modelOut.Items = append(modelOut.Items, trayProfileItem{
			Title: fmt.Sprintf(moreFormat, hidden),
			More:  true,
		})
	}
	return modelOut
}

func profileTrayTitle(profile model.Profile) string {
	name := strings.TrimSpace(profile.Name)
	emoji := strings.TrimSpace(profile.Emoji)
	if emoji == "" {
		return name
	}
	return emoji + " " + name
}

func insertTrayProfile(header, name string) string {
	const prefix = "Norka · "
	if strings.HasPrefix(header, prefix) {
		return prefix + name + " · " + strings.TrimPrefix(header, prefix)
	}
	return header
}
