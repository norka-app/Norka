package main

import (
	"fmt"
	"strings"
	"time"

	"norka/internal/biz"
	"norka/internal/model"
)

// Пункты меню трея строятся из живого состояния туннелей (internal/biz → List()).
// Агрегированный статус совпадает с иконкой Norka в боковой панели
// (frontend/src/utils/norka-status.js): connecting > partial > error > connected > stopped,
// ошибки старше trayStaleErrorAfter для иконки не считаются.
const (
	trayMaxTunnelItems  = 10
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
	Status      string // connected | connecting | partial | error | stopped
	Header      string
	Tooltip     string
	Items       []trayTunnelItem
	Hidden      int // туннели, не поместившиеся в меню
	CanStopAll  bool
	CanRetryAll bool
}

func (m trayModel) iconKey() string {
	if m.Status == "partial" {
		return "error"
	}
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

// trackTrayErrorSince запоминает, когда туннель перешёл в 'error'.
func trackTrayErrorSince(tunnels []model.Tunnel, since map[int]time.Time, now time.Time) {
	inError := make(map[int]bool, len(tunnels))
	for _, t := range tunnels {
		if t.Status != "error" {
			continue
		}
		inError[t.ID] = true
		if _, ok := since[t.ID]; !ok {
			since[t.ID] = now
		}
	}
	for id := range since {
		if !inError[id] {
			delete(since, id)
		}
	}
}

func buildTrayModel(tunnels []model.Tunnel, errorSince map[int]time.Time, now time.Time) trayModel {
	var running, busy, failed, stale int
	m := trayModel{}
	for _, t := range tunnels {
		switch t.Status {
		case "running":
			running++
			m.CanStopAll = true
		case "busy", "reconnecting":
			busy++
			m.CanStopAll = true
		case "error":
			if since, ok := errorSince[t.ID]; ok && now.Sub(since) > trayStaleErrorAfter {
				stale++
			} else {
				failed++
			}
		}
	}
	m.CanRetryAll = len(trayRetryIDs(tunnels)) > 0
	switch {
	case busy > 0:
		m.Status = "connecting"
	case failed > 0 && running > 0:
		m.Status = "partial"
	case failed > 0:
		m.Status = "error"
	case running > 0:
		m.Status = "connected"
	default:
		m.Status = "stopped"
	}

	switch {
	case len(tunnels) == 0:
		m.Header = "Norka · нет туннелей"
	default:
		m.Header = fmt.Sprintf("Norka · %d из %d подключено", running, len(tunnels))
	}
	m.Tooltip = m.Header
	switch m.Status {
	case "connecting":
		m.Tooltip += " · подключение…"
	case "partial", "error":
		m.Tooltip += fmt.Sprintf(" · ошибок: %d", failed)
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
			item.ToggleLabel = "Отключить"
			if t.Mode == "local" || t.Mode == "" {
				item.URL = "http://" + addr
			}
		case "reconnecting":
			item.Title += " — переподключение…"
			item.ToggleLabel = "Отключить"
		case "busy":
			item.Title += " — подключение…"
			item.ToggleLabel = "Отменить подключение"
		case "error":
			item.Title += " — ошибка"
			item.ToggleLabel = "Повторить"
			setTraySwitchPort(&item, t, tunnels)
		default:
			item.ToggleLabel = "Подключить"
			setTraySwitchPort(&item, t, tunnels)
		}
		m.Items = append(m.Items, item)
	}
	return m
}

// setTraySwitchPort меняет «Подключить» / «Повторить» на «Подключить вместо «…»», если локальный
// порт туннеля держит другой туннель: запуск всё равно не смог бы занять порт.
func setTraySwitchPort(item *trayTunnelItem, t model.Tunnel, tunnels []model.Tunnel) {
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
	item.ToggleLabel = fmt.Sprintf("Подключить вместо «%s»", joined)
	if len(conflicts) == 1 {
		item.ToggleTooltip = fmt.Sprintf("Порт %d занят: «%s» будет отключён", t.LocalPort, joined)
	} else {
		item.ToggleTooltip = fmt.Sprintf("Порт %d занят: «%s» будут отключены", t.LocalPort, joined)
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
