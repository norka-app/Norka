package biz

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"norka/internal/conf"
	"norka/internal/model"
)

func TestProfileConfigRoundTripAndLegacyLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	legacy := []byte(`
version = 1
auto_run = false
traffic_monitor_enabled = true

[[tunnels]]
id = 1
name = "db"
mode = "local"
local_host = "127.0.0.1"
local_port = 5432
remote_host = "10.0.0.5"
remote_port = 5432
status = "stopped"
`)
	if err := writeFile(path, legacy); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	storage, err := conf.NewStorage(path)
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	loaded, err := storage.Load()
	if err != nil {
		t.Fatalf("load legacy: %v", err)
	}
	if len(loaded.Tunnels) != 1 || loaded.Tunnels[0].Name != "db" {
		t.Fatalf("legacy tunnels: %+v", loaded.Tunnels)
	}
	if len(loaded.Profiles) != 0 || loaded.ActiveProfileID != 0 || loaded.ProfileStopOthers {
		t.Fatalf("legacy profile defaults: %+v", loaded)
	}
	if !loaded.QuickSearchOn() || loaded.QuickSearchHotkey != "" {
		t.Fatalf("legacy quick search should stay enabled with empty hotkey: %+v", loaded)
	}

	profiles := NewProfileBiz(storage)
	staging, err := profiles.Create(model.ProfilePayload{
		Name:      "staging",
		Emoji:     "🌱",
		Color:     "#0ea5e9",
		TunnelIDs: []int{1, 1, 99},
	})
	if err != nil {
		t.Fatalf("create staging: %v", err)
	}
	home, err := profiles.Create(model.ProfilePayload{
		Name:      "дом",
		Emoji:     "🏠",
		Color:     "#22c55e",
		TunnelIDs: []int{1},
	})
	if err != nil {
		t.Fatalf("create home: %v", err)
	}
	if staging.ID == home.ID {
		t.Fatalf("ids must differ")
	}
	if len(staging.TunnelIDs) != 1 || staging.TunnelIDs[0] != 1 {
		t.Fatalf("unknown and duplicate tunnel ids must be dropped: %+v", staging.TunnelIDs)
	}

	if _, err := profiles.Create(model.ProfilePayload{Name: "Staging", TunnelIDs: []int{1}}); !errors.Is(err, ErrProfileNameExists) {
		t.Fatalf("expected duplicate name, got %v", err)
	}
	if _, err := profiles.Update(staging.ID, model.ProfilePayload{Name: "  prod  ", Color: "red"}); err == nil {
		t.Fatal("expected invalid color to fail")
	}
	renamed, err := profiles.Update(staging.ID, model.ProfilePayload{Name: "prod", Emoji: "🚀", Color: "#ef4444", TunnelIDs: []int{1}})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if renamed.Name != "prod" || renamed.Emoji != "🚀" {
		t.Fatalf("renamed: %+v", renamed)
	}

	if err := profiles.SetStopOthers(true); err != nil {
		t.Fatalf("stop others: %v", err)
	}
	disabled := false
	if _, err := storage.Update(func(cfg *conf.Config) error {
		cfg.QuickSearchEnabled = &disabled
		cfg.QuickSearchHotkey = "ctrl+shift+space"
		cfg.ActiveProfileID = renamed.ID
		return nil
	}); err != nil {
		t.Fatalf("save flags: %v", err)
	}

	again, err := storage.Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(again.Profiles) != 2 || again.Profiles[0].Name != "prod" || again.Profiles[1].Name != "дом" {
		t.Fatalf("profiles: %+v", again.Profiles)
	}
	if !again.ProfileStopOthers || again.ActiveProfileID != renamed.ID {
		t.Fatalf("flags: stop=%v active=%d", again.ProfileStopOthers, again.ActiveProfileID)
	}
	if again.QuickSearchOn() || again.QuickSearchHotkey != "ctrl+shift+space" {
		t.Fatalf("quick search persisted wrong: on=%v hotkey=%q", again.QuickSearchOn(), again.QuickSearchHotkey)
	}

	tunnels := NewTunnelBiz(storage)
	if err := tunnels.Delete(1); err != nil {
		t.Fatalf("delete tunnel: %v", err)
	}
	afterDelete, err := storage.Load()
	if err != nil {
		t.Fatalf("reload after delete: %v", err)
	}
	for _, profile := range afterDelete.Profiles {
		if len(profile.TunnelIDs) != 0 {
			t.Fatalf("tunnel id should leave profiles: %+v", profile)
		}
	}

	if err := profiles.Delete(renamed.ID); err != nil {
		t.Fatalf("delete profile: %v", err)
	}
	cleared, err := storage.Load()
	if err != nil {
		t.Fatalf("reload after profile delete: %v", err)
	}
	if cleared.ActiveProfileID != 0 || len(cleared.Profiles) != 1 {
		t.Fatalf("active profile should clear: %+v", cleared)
	}
}

func TestPlanProfileActivation(t *testing.T) {
	tunnels := []model.Tunnel{
		{ID: 1, Name: "api", LocalHost: "127.0.0.1", LocalPort: 8080, Status: "stopped"},
		{ID: 2, Name: "db", LocalHost: "localhost", LocalPort: 5432, Status: "running"},
		{ID: 3, Name: "redis", LocalHost: "127.0.0.1", LocalPort: 6379, Status: "stopped"},
		{ID: 4, Name: "extra", LocalHost: "127.0.0.1", LocalPort: 9000, Status: "running"},
		{ID: 5, Name: "shadow", LocalHost: "127.0.0.1", LocalPort: 8080, Status: "busy"},
	}
	profile := model.Profile{ID: 7, Name: "staging", TunnelIDs: []int{1, 2, 3, 99}}

	kept := PlanProfileActivation(profile, tunnels, false)
	if !sameIDs(kept.StartIDs, []int{3}) {
		t.Fatalf("start without stopping others: %v", kept.StartIDs)
	}
	if len(kept.Conflicts) != 1 || kept.Conflicts[0].TunnelID != 1 || kept.Conflicts[0].HolderID != 5 || kept.Conflicts[0].InsideProfile {
		t.Fatalf("api is blocked by the outsider on :8080: %+v", kept.Conflicts)
	}
	if len(kept.StopIDs) != 0 {
		t.Fatalf("outsiders must keep running: %v", kept.StopIDs)
	}
	if !sameIDs(kept.AlreadyIDs, []int{2}) {
		t.Fatalf("already running member: %v", kept.AlreadyIDs)
	}

	stopped := PlanProfileActivation(profile, tunnels, true)
	if !sameIDs(stopped.StopIDs, []int{5, 4}) && !sameIDs(stopped.StopIDs, []int{4, 5}) {
		t.Fatalf("outsiders should stop: %v", stopped.StopIDs)
	}
	if !sameIDs(stopped.StartIDs, []int{1, 3}) {
		t.Fatalf("members start after outsiders stop: %v", stopped.StartIDs)
	}

	clash := PlanProfileActivation(model.Profile{TunnelIDs: []int{1}}, tunnels, false)
	if len(clash.StartIDs) != 0 || len(clash.Conflicts) != 1 {
		t.Fatalf("outsider holding the port: %+v", clash)
	}
	if clash.Conflicts[0].HolderID != 5 || clash.Conflicts[0].InsideProfile || clash.Conflicts[0].Port != 8080 {
		t.Fatalf("conflict: %+v", clash.Conflicts[0])
	}

	takenOver := PlanProfileActivation(model.Profile{TunnelIDs: []int{1}}, tunnels, true)
	if !sameIDs(takenOver.StartIDs, []int{1}) || len(takenOver.Conflicts) != 0 {
		t.Fatalf("stop-others frees the port: %+v", takenOver)
	}
	if !containsID(takenOver.StopIDs, 5) || !containsID(takenOver.StopIDs, 4) {
		t.Fatalf("both outsiders stop: %v", takenOver.StopIDs)
	}

	inside := []model.Tunnel{
		{ID: 1, Name: "a", LocalHost: "127.0.0.1", LocalPort: 3000, Status: "stopped"},
		{ID: 2, Name: "b", LocalHost: "127.0.0.1", LocalPort: 3000, Status: "error"},
		{ID: 3, Name: "c", LocalHost: "127.0.0.1", LocalPort: 3001, Status: "reconnecting"},
	}
	shared := PlanProfileActivation(model.Profile{TunnelIDs: []int{1, 2, 3}}, inside, false)
	if !sameIDs(shared.StartIDs, []int{1}) || !sameIDs(shared.AlreadyIDs, []int{3}) {
		t.Fatalf("shared port plan: start=%v already=%v", shared.StartIDs, shared.AlreadyIDs)
	}
	if len(shared.Conflicts) != 1 || !shared.Conflicts[0].InsideProfile || shared.Conflicts[0].HolderName != "a" {
		t.Fatalf("inside conflict: %+v", shared.Conflicts)
	}
}

func TestApplyActivationPlanOrder(t *testing.T) {
	var log []string
	plan := model.ActivationPlan{StopIDs: []int{4, 5}, StartIDs: []int{1, 3}, AlreadyIDs: []int{2}}
	result := ApplyActivationPlan(plan, func(id int) error {
		log = append(log, "stop")
		if id == 5 {
			return errors.New("busy")
		}
		return nil
	}, func(id int) error {
		log = append(log, "start")
		if id == 3 {
			return errors.New("refused")
		}
		return nil
	})
	if len(log) != 4 || log[0] != "stop" || log[1] != "stop" || log[2] != "start" || log[3] != "start" {
		t.Fatalf("stops must finish before starts: %v", log)
	}
	if !sameIDs(result.Stopped, []int{4}) || !sameIDs(result.Started, []int{1}) {
		t.Fatalf("partial result: %+v", result)
	}
	if len(result.Errors) != 2 || result.Errors[0].Action != "stop" || result.Errors[1].Action != "start" {
		t.Fatalf("errors: %+v", result.Errors)
	}
	if !sameIDs(result.AlreadyOn, []int{2}) {
		t.Fatalf("already: %v", result.AlreadyOn)
	}
}

func TestActivateProfilePersistsActiveID(t *testing.T) {
	dir := t.TempDir()
	storage, err := conf.NewStorage(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Update(func(cfg *conf.Config) error {
		cfg.Tunnels = []model.Tunnel{
			{ID: 1, Name: "api", LocalHost: "127.0.0.1", LocalPort: 8080, Status: "stopped"},
			{ID: 2, Name: "other", LocalHost: "127.0.0.1", LocalPort: 9000, Status: "running"},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	profiles := NewProfileBiz(storage)
	created, err := profiles.Create(model.ProfilePayload{Name: "staging", TunnelIDs: []int{1}})
	if err != nil {
		t.Fatal(err)
	}

	var stopped, started []int
	result, err := profiles.Activate(created.ID, func(id int) error {
		stopped = append(stopped, id)
		return nil
	}, func(id int) error {
		started = append(started, id)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(stopped) != 0 || !sameIDs(started, []int{1}) || !sameIDs(result.Started, []int{1}) {
		t.Fatalf("activate without stop-others: stopped=%v started=%v result=%+v", stopped, started, result)
	}
	cfg, err := storage.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveProfileID != created.ID {
		t.Fatalf("active id: %d", cfg.ActiveProfileID)
	}

	if err := profiles.SetStopOthers(true); err != nil {
		t.Fatal(err)
	}
	stopped, started = nil, nil
	if _, err := profiles.Activate(created.ID, func(id int) error {
		stopped = append(stopped, id)
		return nil
	}, func(id int) error {
		started = append(started, id)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !sameIDs(stopped, []int{2}) || !sameIDs(started, []int{1}) {
		t.Fatalf("activate with stop-others: stopped=%v started=%v", stopped, started)
	}

	if err := profiles.ClearActive(); err != nil {
		t.Fatal(err)
	}
	cfg, err = storage.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveProfileID != 0 || !cfg.ProfileStopOthers {
		t.Fatalf("clear keeps the preference: %+v", cfg)
	}
}

func sameIDs(got, want []int) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func containsID(ids []int, want int) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}
