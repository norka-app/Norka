package biz

import (
	"path/filepath"
	"testing"

	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/model"
)

func TestStartAutoStartSkipsFilteredTunnel(t *testing.T) {
	dir := t.TempDir()
	storage, err := conf.NewStorage(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	jumpers := NewJumperBiz(storage)
	jumper, err := jumpers.Create(model.JumperPayload{
		Name:     "pw",
		Host:     "127.0.0.1",
		Port:     1,
		User:     "root",
		AuthType: "password",
		Password: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	tunnels := NewTunnelBiz(storage)
	created, err := tunnels.Create(model.TunnelPayload{
		Name:       "db",
		Mode:       "local",
		JumperIDs:  []int{jumper.ID},
		LocalHost:  "127.0.0.1",
		LocalPort:  18080,
		RemoteHost: "10.0.0.8",
		RemotePort: 5432,
		AutoStart:  true,
		Status:     "stopped",
	})
	if err != nil {
		t.Fatal(err)
	}
	tunnels.SetAutoStartSkip(func(model.Tunnel, []model.Jumper) string {
		return "password auth needs the GUI keychain prompt"
	})
	if err := tunnels.StartAutoStart(0); err != nil {
		t.Fatal(err)
	}
	cfg, err := storage.Load()
	if err != nil {
		t.Fatal(err)
	}
	got, ok := findTunnelByID(cfg.Tunnels, created.ID)
	if !ok {
		t.Fatal("tunnel missing")
	}
	if got.Status != "stopped" || got.LastError != "" {
		t.Fatalf("skipped tunnel was dialed: status %s error %q", got.Status, got.LastError)
	}
}
