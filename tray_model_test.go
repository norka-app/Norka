package main

import (
	"testing"
	"time"

	"norka/internal/model"
)

func TestBuildTrayModel(t *testing.T) {
	now := time.Date(2026, 10, 1, 20, 0, 0, 0, time.UTC)
	tunnels := []model.Tunnel{
		{ID: 1, Name: "dev", Mode: "local", LocalHost: "127.0.0.1", LocalPort: 3000, Status: "running"},
		{ID: 2, Name: "db", Mode: "local", LocalPort: 5432, Status: "error"},
		{ID: 3, Name: "socks", Mode: "dynamic", LocalPort: 1080, Status: "stopped"},
	}
	// без истории переходов (запуск приложения): ошибка важнее подключения, глаза не бывают разными
	m := buildTrayModel(tunnels, map[int]traySince{}, now)
	if m.Status != "error" || m.iconKey() != "error" {
		t.Fatalf("status = %q / icon %q, want error / error", m.Status, m.iconKey())
	}
	if m.Tooltip != "Norka · 1 из 3 подключено · ошибок: 1" {
		t.Fatalf("tooltip = %q", m.Tooltip)
	}
	if m.Header != "Norka · 1 из 3 подключено" {
		t.Fatalf("header = %q", m.Header)
	}
	if !m.CanStopAll || !m.CanRetryAll {
		t.Fatalf("CanStopAll=%t CanRetryAll=%t, want both true", m.CanStopAll, m.CanRetryAll)
	}
	want := []trayTunnelItem{
		{ID: 1, Title: "dev · 3000", Running: true, ToggleLabel: "Отключить", Address: "localhost:3000", URL: "http://localhost:3000"},
		{ID: 2, Title: "db · 5432 — ошибка", ToggleLabel: "Повторить", Address: "localhost:5432"},
		{ID: 3, Title: "socks · 1080", ToggleLabel: "Подключить", Address: "localhost:1080"},
	}
	for i, it := range m.Items {
		if it != want[i] {
			t.Errorf("item %d = %+v, want %+v", i, it, want[i])
		}
	}

	// ошибка старше 5 минут не красит иконку
	stale := map[int]traySince{2: {Status: "error", At: now.Add(-6 * time.Minute)}}
	if got := buildTrayModel(tunnels, stale, now).Status; got != "connected" {
		t.Errorf("stale error status = %q, want connected", got)
	}

	busy := append([]model.Tunnel{}, tunnels...)
	busy[2].Status = "reconnecting"
	if got := buildTrayModel(busy, nil, now).Status; got != "connecting" {
		t.Errorf("reconnecting status = %q, want connecting", got)
	}

	if got := buildTrayModel(nil, nil, now); got.Status != "stopped" || got.Header != "Norka · нет туннелей" || got.CanStopAll {
		t.Errorf("empty model = %+v", got)
	}

	many := make([]model.Tunnel, trayMaxTunnelItems+3)
	for i := range many {
		many[i] = model.Tunnel{ID: i + 1, Name: "t", LocalPort: 1000 + i, Status: "stopped"}
	}
	if got := buildTrayModel(many, nil, now); len(got.Items) != trayMaxTunnelItems || got.Hidden != 3 {
		t.Errorf("overflow: items=%d hidden=%d", len(got.Items), got.Hidden)
	}
}

func TestBuildTrayModelPortConflict(t *testing.T) {
	now := time.Date(2026, 10, 1, 20, 0, 0, 0, time.UTC)
	tunnels := []model.Tunnel{
		{ID: 1, Name: "user2@10.200.29.191", Mode: "local", LocalHost: "127.0.0.1", LocalPort: 3000, Status: "running"},
		{ID: 2, Name: "127.0.0.1-3000", Mode: "local", LocalHost: "localhost", LocalPort: 3000, Status: "stopped"},
		{ID: 3, Name: "web", Mode: "local", LocalPort: 3000, Status: "error"},
		{ID: 4, Name: "db", Mode: "local", LocalPort: 5432, Status: "error"},
		{ID: 5, Name: "other-ip", Mode: "local", LocalHost: "127.0.0.2", LocalPort: 3000, Status: "stopped"},
	}
	m := buildTrayModel(tunnels, nil, now)
	want := []trayTunnelItem{
		{ID: 1, Title: "user2@10.200.29.191 · 3000", Running: true, ToggleLabel: "Отключить", Address: "localhost:3000", URL: "http://localhost:3000"},
		{ID: 2, Title: "127.0.0.1-3000 · 3000", ToggleLabel: "Подключить вместо «user2@10.200.29.191»",
			ToggleTooltip: "Порт 3000 занят: «user2@10.200.29.191» будет отключён", SwitchPort: true, Address: "localhost:3000"},
		{ID: 3, Title: "web · 3000 — ошибка", ToggleLabel: "Подключить вместо «user2@10.200.29.191»",
			ToggleTooltip: "Порт 3000 занят: «user2@10.200.29.191» будет отключён", SwitchPort: true, Address: "localhost:3000"},
		{ID: 4, Title: "db · 5432 — ошибка", ToggleLabel: "Повторить", Address: "localhost:5432"},
		{ID: 5, Title: "other-ip · 3000", ToggleLabel: "Подключить", Address: "127.0.0.2:3000"},
	}
	for i, it := range m.Items {
		if it != want[i] {
			t.Errorf("item %d = %+v, want %+v", i, it, want[i])
		}
	}

	// подключающийся туннель тоже держит порт; несколько — перечислены через запятую
	both := []model.Tunnel{
		{ID: 1, Name: "a", LocalPort: 8080, Status: "busy"},
		{ID: 2, Name: "b", LocalPort: 8080, Status: "reconnecting"},
		{ID: 3, Name: "c", LocalPort: 8080, Status: "stopped"},
	}
	if it := buildTrayModel(both, nil, now).Items[2]; it.ToggleLabel != "Подключить вместо «a», «b»" ||
		it.ToggleTooltip != "Порт 8080 занят: «a», «b» будут отключены" || !it.SwitchPort {
		t.Errorf("two conflicts item = %+v", it)
	}
}

func TestTrayRetryIDsSkipsPortConflicts(t *testing.T) {
	now := time.Date(2026, 10, 1, 20, 0, 0, 0, time.UTC)
	tunnels := []model.Tunnel{
		{ID: 1, Name: "dev", LocalPort: 3000, Status: "running"},
		{ID: 2, Name: "web", LocalPort: 3000, Status: "error"},  // порт держит dev — пропуск
		{ID: 3, Name: "db", LocalPort: 5432, Status: "error"},   // повторить
		{ID: 4, Name: "db-2", LocalPort: 5432, Status: "error"}, // тот же порт, что у db, — только первый
		{ID: 5, Name: "socks", LocalPort: 1080, Status: "stopped"},
	}
	got := trayRetryIDs(tunnels)
	if len(got) != 1 || got[0] != 3 {
		t.Fatalf("trayRetryIDs = %v, want [3]", got)
	}
	if !buildTrayModel(tunnels, nil, now).CanRetryAll {
		t.Error("CanRetryAll = false, want true (db can be retried)")
	}

	// все ошибочные туннели упираются в занятый порт — пункт «Переподключить ошибочные» скрыт
	blocked := tunnels[:2]
	if ids := trayRetryIDs(blocked); len(ids) != 0 {
		t.Errorf("blocked trayRetryIDs = %v, want none", ids)
	}
	if m := buildTrayModel(blocked, nil, now); m.CanRetryAll {
		t.Error("CanRetryAll = true for a tunnel whose port is held")
	}
}

func TestTrackTraySince(t *testing.T) {
	now := time.Now()
	since := map[int]traySince{9: {Status: "error", At: now}, 2: {Status: "running", At: now.Add(-time.Hour)}}
	trackTraySince([]model.Tunnel{{ID: 1, Status: "error"}, {ID: 2, Status: "running"}}, since, now)
	if s, ok := since[1]; !ok || s.Status != "error" || !s.At.Equal(now) {
		t.Errorf("error tunnel tracked as %+v", s)
	}
	if s := since[2]; !s.At.Equal(now.Add(-time.Hour)) {
		t.Error("unchanged status must keep its time")
	}
	if _, ok := since[9]; ok {
		t.Error("vanished tunnel not cleared")
	}
}

// Глаза показывают последнее событие и никогда не бывают разными.
func TestTrayStatusLatestEvent(t *testing.T) {
	t0 := time.Date(2026, 10, 2, 13, 0, 0, 0, time.UTC)
	since := map[int]traySince{}
	step := func(at time.Time, tunnels ...model.Tunnel) string {
		trackTraySince(tunnels, since, at)
		return buildTrayModel(tunnels, since, at).Status
	}
	db := model.Tunnel{ID: 1, Name: "db", LocalPort: 5432}
	web := model.Tunnel{ID: 2, Name: "web", LocalPort: 8080}
	with := func(t model.Tunnel, status string) model.Tunnel { t.Status = status; return t }

	if got := step(t0, with(db, "running"), with(web, "stopped")); got != "connected" {
		t.Fatalf("start = %q, want connected", got)
	}
	if got := step(t0.Add(time.Second), with(db, "running"), with(web, "busy")); got != "connecting" {
		t.Fatalf("connecting = %q", got)
	}
	// подключение web упало, db работает → красные
	if got := step(t0.Add(2*time.Second), with(db, "running"), with(web, "error")); got != "error" {
		t.Fatalf("failed connect = %q, want error", got)
	}
	// ошибочный web остановлен → снова зелёные
	if got := step(t0.Add(3*time.Second), with(db, "running"), with(web, "stopped")); got != "connected" {
		t.Fatalf("after stopping errored = %q, want connected", got)
	}
	// снова ошибка web, затем db переподключился успешно → зелёные при оставшейся ошибке
	step(t0.Add(4*time.Second), with(db, "running"), with(web, "error"))
	step(t0.Add(5*time.Second), with(db, "reconnecting"), with(web, "error"))
	if got := step(t0.Add(6*time.Second), with(db, "running"), with(web, "error")); got != "connected" {
		t.Fatalf("successful connect after error = %q, want connected", got)
	}
	// db остановлен, работающих нет, ошибка web осталась → красные
	if got := step(t0.Add(7*time.Second), with(db, "stopped"), with(web, "error")); got != "error" {
		t.Fatalf("only errors left = %q, want error", got)
	}
	// всё остановлено → закрыты
	if got := step(t0.Add(8*time.Second), with(db, "stopped"), with(web, "stopped")); got != "stopped" {
		t.Fatalf("all stopped = %q, want stopped", got)
	}
}
