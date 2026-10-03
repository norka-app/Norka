package biz

import (
	"path/filepath"
	"testing"

	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/model"
)

func TestPortConflicts(t *testing.T) {
	all := []model.Tunnel{
		{ID: 1, Name: "a", LocalHost: "127.0.0.1", LocalPort: 3000, Status: "running"},
		{ID: 2, Name: "b", LocalHost: "localhost", LocalPort: 3000, Status: "stopped"},
		{ID: 3, Name: "c", LocalHost: "", LocalPort: 3000, Status: "busy"},
		{ID: 4, Name: "d", LocalHost: "127.0.0.2", LocalPort: 3000, Status: "running"},
		{ID: 5, Name: "e", LocalHost: "127.0.0.1", LocalPort: 3001, Status: "reconnecting"},
		{ID: 6, Name: "f", LocalHost: "127.0.0.1", LocalPort: 3000, Status: "error"},
	}
	ids := func(items []model.Tunnel) []int {
		var out []int
		for _, it := range items {
			out = append(out, it.ID)
		}
		return out
	}
	cases := []struct {
		id   int
		want []int
	}{
		{id: 2, want: []int{1, 3}}, // localhost == 127.0.0.1 == empty host; error/stopped do not hold the port
		{id: 1, want: []int{3}},
		{id: 4, want: nil}, // other bind address
		{id: 5, want: nil},
	}
	for _, c := range cases {
		target, _ := findTunnelByID(all, c.id)
		got := ids(PortConflicts(target, all))
		if len(got) != len(c.want) {
			t.Errorf("PortConflicts(%d) = %v, want %v", c.id, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("PortConflicts(%d) = %v, want %v", c.id, got, c.want)
				break
			}
		}
	}
	if got := PortConflicts(model.Tunnel{ID: 9}, all); got != nil {
		t.Errorf("no port: %v", got)
	}
}

func newSwitchTestBiz(t *testing.T, tunnels []model.Tunnel) *TunnelBiz {
	t.Helper()
	storage, err := conf.NewStorage(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	if _, err := storage.Update(func(cfg *conf.Config) error {
		cfg.Tunnels = tunnels
		return nil
	}); err != nil {
		t.Fatalf("seed tunnels: %v", err)
	}
	return NewTunnelBiz(storage)
}

func statusesByID(t *testing.T, b *TunnelBiz) map[int]model.Tunnel {
	t.Helper()
	items, err := b.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	out := make(map[int]model.Tunnel, len(items))
	for _, it := range items {
		out[it.ID] = it
	}
	return out
}

func TestSwitchToStopsConflictsThenStarts(t *testing.T) {
	// jumper 99 does not exist: the start fails fast with "jumper not found" (no network)
	tunnel := func(id int, host string, status string) model.Tunnel {
		return model.Tunnel{ID: id, Name: "t", Mode: "local", JumperIDs: []int{99}, LocalHost: host, LocalPort: 3000, RemoteHost: "10.0.0.1", RemotePort: 80, Status: status}
	}
	b := newSwitchTestBiz(t, []model.Tunnel{
		tunnel(1, "127.0.0.1", "running"),
		tunnel(2, "localhost", "stopped"),
		tunnel(3, "127.0.0.2", "running"),
	})

	got, err := b.SwitchTo(2, 0)
	if err != nil {
		t.Fatalf("switch: %v", err)
	}
	if got.ID != 2 || got.Status != "error" || got.LastError != "jumper not found" {
		t.Fatalf("switched tunnel = %+v, want start attempted (error: jumper not found)", got)
	}
	state := statusesByID(t, b)
	if state[1].Status != "stopped" {
		t.Errorf("conflicting tunnel status = %q, want stopped", state[1].Status)
	}
	if state[3].Status != "running" {
		t.Errorf("tunnel on another address status = %q, want running", state[3].Status)
	}
}

func TestSwitchToLeavesActiveTargetAlone(t *testing.T) {
	b := newSwitchTestBiz(t, []model.Tunnel{
		{ID: 1, Name: "a", Mode: "local", LocalPort: 3000, Status: "running"},
		{ID: 2, Name: "b", Mode: "local", LocalPort: 3000, Status: "running"},
	})
	got, err := b.SwitchTo(2, 0)
	if err != nil {
		t.Fatalf("switch: %v", err)
	}
	if got.Status != "running" {
		t.Fatalf("target = %q, want running", got.Status)
	}
	if state := statusesByID(t, b); state[1].Status != "running" {
		t.Errorf("other tunnel = %q, want untouched", state[1].Status)
	}
}
