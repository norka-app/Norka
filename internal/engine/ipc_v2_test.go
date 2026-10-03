package engine

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/norka-app/Norka/internal/biz"
	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/features"
	"github.com/norka-app/Norka/internal/ipc"
	"github.com/norka-app/Norka/internal/model"
	"github.com/norka-app/Norka/internal/netwatch"
	"github.com/norka-app/Norka/internal/notify"
	"github.com/norka-app/Norka/internal/secrets"
	"github.com/norka-app/Norka/internal/wake"
)

func TestV1StatusWithoutHello(t *testing.T) {
	eng, _ := newIPCEngine(t, KindDaemon, true)
	resp := eng.handleAutomation(ipc.Request{Op: ipc.OpStatus})
	if !resp.OK || resp.Code != ipc.CodeOK || resp.Hello != nil || resp.State != nil {
		t.Fatalf("legacy status: %+v", resp)
	}
	numbered := eng.handleAutomation(ipc.Request{V: 1, Op: ipc.OpList})
	if !numbered.OK || numbered.Hello != nil {
		t.Fatalf("v1 list: %+v", numbered)
	}
	if got := eng.handleAutomation(ipc.Request{V: 1, Op: ipc.OpHello}); got.Code != ipc.CodeBadRequest {
		t.Fatalf("v1 hello must stay a bad request: %+v", got)
	}
}

func TestHelloStateAndSecrets(t *testing.T) {
	eng, path := newIPCEngine(t, KindDaemon, true)
	eng.SetVersion("9.9.9")
	if _, err := eng.Acquire(KindDaemon); err != nil {
		t.Fatal(err)
	}
	const secret = "s3cret-hop-should-not-leak"
	saved := eng.handleAutomation(ipc.Request{
		V:  ipc.ProtocolVersion,
		Op: ipc.OpControl,
		Control: &ipc.Control{
			Action: ipc.ActionSave,
			Kind:   ipc.KindJumper,
			Jumper: &model.JumperPayload{
				Name: "bastion", Host: "bastion.example", User: "norka",
				AuthType: "password", Password: secret,
			},
		},
	})
	if !saved.OK || saved.State == nil || len(saved.State.Jumpers) != 1 {
		t.Fatalf("save jumper: %+v", saved)
	}
	raw, err := json.Marshal(saved.State)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), secret) {
		t.Fatalf("state leaked the jumper password: %s", raw)
	}
	if !saved.State.Jumpers[0].HasSecret {
		t.Fatal("jumper should report that a secret is stored")
	}
	if saved.State.Stats == nil {
		t.Fatal("stats feature is on, so the snapshot must include stats")
	}

	hello := eng.handleAutomation(ipc.Request{
		V:      ipc.ProtocolVersion,
		Op:     ipc.OpHello,
		Client: &ipc.ClientInfo{Name: "norka-cli", Version: "test", Protocol: ipc.ProtocolVersion},
	})
	if !hello.OK || hello.Hello == nil {
		t.Fatalf("hello: %+v", hello)
	}
	if hello.Hello.ProtocolVersion != ipc.ProtocolVersion || hello.Hello.MinClientVersion != 1 {
		t.Fatalf("hello versions: %+v", hello.Hello)
	}
	if hello.Hello.Owner != string(KindDaemon) || hello.Hello.PID == 0 || hello.Hello.Version != "9.9.9" {
		t.Fatalf("hello owner: %+v", hello.Hello)
	}

	disk, err := conf.NewStorage(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := disk.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Jumpers) != 1 || strings.Contains(cfg.Jumpers[0].Password, secret) {
		t.Fatalf("config write did not go through storage: %+v", cfg.Jumpers)
	}
}

func TestStateOmitsStatsWhenDisabled(t *testing.T) {
	eng, _ := newIPCEngine(t, KindDaemon, false)
	if _, err := eng.Acquire(KindDaemon); err != nil {
		t.Fatal(err)
	}
	resp := eng.handleAutomation(ipc.Request{V: ipc.ProtocolVersion, Op: ipc.OpState})
	if !resp.OK || resp.State == nil || resp.State.Stats != nil {
		t.Fatalf("stats should be omitted: %+v", resp.State)
	}
}

func TestControlSaveDeleteAndNonOwner(t *testing.T) {
	eng, path := newIPCEngine(t, KindDaemon, true)
	denied := eng.handleAutomation(ipc.Request{
		V:  ipc.ProtocolVersion,
		Op: ipc.OpControl,
		Control: &ipc.Control{
			Action: ipc.ActionSave,
			Kind:   ipc.KindGroup,
			Group:  &model.TunnelGroupPayload{Name: "prod"},
		},
	})
	if denied.OK || !strings.Contains(denied.Message, "engine lock") {
		t.Fatalf("non-owner: %+v", denied)
	}
	cfg, err := conf.NewStorage(path)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := cfg.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Groups) != 0 {
		t.Fatalf("non-owner wrote groups: %+v", loaded.Groups)
	}

	if _, err := eng.Acquire(KindDaemon); err != nil {
		t.Fatal(err)
	}
	group := eng.handleAutomation(ipc.Request{
		V:  ipc.ProtocolVersion,
		Op: ipc.OpControl,
		Control: &ipc.Control{
			Action: ipc.ActionSave,
			Kind:   ipc.KindGroup,
			Group:  &model.TunnelGroupPayload{Name: "prod"},
		},
	})
	if !group.OK || group.State == nil || len(group.State.Groups) != 1 || group.State.Groups[0].Name != "prod" {
		t.Fatalf("save group: %+v", group)
	}
	id := group.State.Groups[0].ID
	deleted := eng.handleAutomation(ipc.Request{
		V:       ipc.ProtocolVersion,
		Op:      ipc.OpControl,
		Control: &ipc.Control{Action: ipc.ActionDelete, Kind: ipc.KindGroup, ID: id},
	})
	if !deleted.OK || deleted.State == nil || len(deleted.State.Groups) != 0 {
		t.Fatalf("delete group: %+v", deleted)
	}
}

func TestControlRestartUsesOwnerRuntime(t *testing.T) {
	rt := &recordingRuntime{}
	eng, _ := newIPCEngineRuntime(t, KindDaemon, true, rt)
	if _, err := eng.Acquire(KindDaemon); err != nil {
		t.Fatal(err)
	}
	resp := eng.handleAutomation(ipc.Request{
		V:  ipc.ProtocolVersion,
		Op: ipc.OpControl,
		Control: &ipc.Control{
			Action: ipc.ActionRestart,
			Kind:   ipc.KindTunnel,
			ID:     7,
		},
	})
	if !resp.OK {
		t.Fatalf("restart: %+v", resp)
	}
	if len(rt.ops) != 2 || rt.ops[0] != "stop" || rt.ops[1] != "start" {
		t.Fatalf("ops %v", rt.ops)
	}
}

func TestControlStartSecretIsNotStoredOrReturned(t *testing.T) {
	rt := &secretRuntime{}
	eng, path := newIPCEngineRuntime(t, KindDaemon, true, rt)
	if _, err := eng.Acquire(KindDaemon); err != nil {
		t.Fatal(err)
	}
	jumper, err := eng.jumper.Create(model.JumperPayload{
		Name: "bastion", Host: "bastion.example", User: "norka", AuthType: "password", Password: "stored-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	tunnel, err := eng.tunnel.Create(model.TunnelPayload{
		Name: "db", Mode: "local", JumperIDs: []int{jumper.ID},
		LocalHost: "127.0.0.1", LocalPort: 15432, RemoteHost: "db.internal", RemotePort: 5432, Status: "stopped",
	})
	if err != nil {
		t.Fatal(err)
	}
	const ephemeral = "ephemeral-one-shot-secret"
	denied := eng.handleAutomation(ipc.Request{
		V:       ipc.ProtocolVersion,
		Op:      ipc.OpControl,
		Control: &ipc.Control{Action: ipc.ActionStart, Kind: ipc.KindTunnel, ID: tunnel.ID},
	})
	if denied.OK || !strings.Contains(denied.Message, "password") {
		t.Fatalf("start without secret: %+v", denied)
	}
	if len(rt.secrets) != 0 {
		t.Fatal("daemon dialed a password tunnel without a secret from the window")
	}
	allowed := eng.handleAutomation(ipc.Request{
		V:  ipc.ProtocolVersion,
		Op: ipc.OpControl,
		Control: &ipc.Control{
			Action:  ipc.ActionStart,
			Kind:    ipc.KindTunnel,
			ID:      tunnel.ID,
			Secrets: []ipc.HopSecret{{JumperID: jumper.ID, Secret: ephemeral}},
		},
	})
	if !allowed.OK {
		t.Fatalf("start with secret: %+v", allowed)
	}
	raw, err := json.Marshal(allowed)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), ephemeral) || strings.Contains(string(raw), "stored-secret") {
		t.Fatalf("response leaked a secret: %s", raw)
	}
	disk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(disk), ephemeral) {
		t.Fatalf("config stored the ephemeral secret: %s", disk)
	}
	if len(rt.secrets) != 1 || rt.secrets[0] != ephemeral {
		t.Fatalf("runtime secrets %+v", rt.secrets)
	}
}

func TestDaemonAnswersWindowWhenAutomationIsOff(t *testing.T) {
	eng, path := newIPCEngine(t, KindDaemon, false)
	storage, err := conf.NewStorage(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Update(func(cfg *conf.Config) error {
		if err := cfg.Features.Set(features.Automation, false); err != nil {
			return err
		}
		return cfg.Features.Set(features.BackgroundMode, true)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.Acquire(KindDaemon); err != nil {
		t.Fatal(err)
	}
	hello := eng.handleAutomation(ipc.Request{V: ipc.ProtocolVersion, Op: ipc.OpHello})
	if !hello.OK || hello.Hello == nil || hello.Hello.Owner != string(KindDaemon) {
		t.Fatalf("hello: %+v", hello)
	}
	legacy := eng.handleAutomation(ipc.Request{Op: ipc.OpStatus})
	if legacy.Code != ipc.CodeDisabled {
		t.Fatalf("legacy status should stay disabled: %+v", legacy)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var snap ipc.Response
	err = eng.handleSubscribe(ctx, ipc.Request{V: ipc.ProtocolVersion, Op: ipc.OpSubscribe}, func(resp ipc.Response) error {
		snap = resp
		cancel()
		return nil
	})
	if snap.State == nil || !snap.OK {
		t.Fatalf("subscribe %v %+v", err, snap)
	}
}

func TestShutdownAndHandover(t *testing.T) {
	t.Run("gui ignores shutdown", func(t *testing.T) {
		eng, path := newIPCEngine(t, KindGUI, true)
		if _, err := eng.Acquire(KindGUI); err != nil {
			t.Fatal(err)
		}
		resp := eng.handleAutomation(ipc.Request{V: ipc.ProtocolVersion, Op: ipc.OpShutdown})
		if resp.Code != ipc.CodeIgnored || resp.OK {
			t.Fatalf("ignored: %+v", resp)
		}
		select {
		case <-eng.stopped():
			t.Fatal("gui shutdown closed the stop signal")
		default:
		}
		if _, err := Acquire(path, KindDaemon, "other"); !errors.Is(err, ErrOwnedByOther) {
			t.Fatalf("lock was released: %v", err)
		}
	})

	t.Run("gui force stops", func(t *testing.T) {
		eng, _ := newIPCEngine(t, KindGUI, true)
		if _, err := eng.Acquire(KindGUI); err != nil {
			t.Fatal(err)
		}
		resp := eng.handleAutomation(ipc.Request{V: ipc.ProtocolVersion, Op: ipc.OpShutdown, Force: true})
		if !resp.OK || resp.After == nil {
			t.Fatalf("force: %+v", resp)
		}
		resp.After()
		select {
		case <-eng.stopped():
		case <-time.After(time.Second):
			t.Fatal("force did not signal stop")
		}
	})

	t.Run("daemon shutdown", func(t *testing.T) {
		eng, _ := newIPCEngine(t, KindDaemon, true)
		if _, err := eng.Acquire(KindDaemon); err != nil {
			t.Fatal(err)
		}
		resp := eng.handleAutomation(ipc.Request{V: ipc.ProtocolVersion, Op: ipc.OpShutdown})
		if !resp.OK || resp.After == nil {
			t.Fatalf("daemon shutdown: %+v", resp)
		}
		resp.After()
		select {
		case <-eng.stopped():
		case <-time.After(time.Second):
			t.Fatal("daemon shutdown did not signal stop")
		}
	})

	t.Run("handover releases the lock", func(t *testing.T) {
		eng, path := newIPCEngine(t, KindDaemon, true)
		if _, err := eng.Acquire(KindDaemon); err != nil {
			t.Fatal(err)
		}
		resp := eng.handleAutomation(ipc.Request{V: ipc.ProtocolVersion, Op: ipc.OpHandover, TimeoutMS: 2000})
		if !resp.OK || !strings.Contains(resp.Message, "released") {
			t.Fatalf("handover: %+v", resp)
		}
		next, err := Acquire(path, KindGUI, "next")
		if err != nil {
			t.Fatalf("caller could not Acquire: %v", err)
		}
		t.Cleanup(func() { _ = next.Release() })
		if resp.After != nil {
			resp.After()
		}
	})

	t.Run("handover timeout keeps the lock", func(t *testing.T) {
		rt := &blockingRuntime{release: make(chan struct{}), entered: make(chan struct{})}
		eng, path := newIPCEngineRuntime(t, KindDaemon, true, rt)
		t.Cleanup(func() { close(rt.release) })
		if _, err := eng.Acquire(KindDaemon); err != nil {
			t.Fatal(err)
		}
		resp := eng.handleAutomation(ipc.Request{V: ipc.ProtocolVersion, Op: ipc.OpHandover, TimeoutMS: 150})
		if resp.Code != ipc.CodeTimeout || resp.OK || !strings.Contains(resp.Message, "engine lock") {
			t.Fatalf("timeout: %+v", resp)
		}
		if _, err := Acquire(path, KindGUI, "next"); !errors.Is(err, ErrOwnedByOther) {
			t.Fatalf("lock released on timeout: %v", err)
		}
	})
}

func TestSubscribeStreamsEvents(t *testing.T) {
	eng, path := newIPCEngine(t, KindDaemon, true)
	eng.SetVersion("dev")
	if _, err := eng.Acquire(KindDaemon); err != nil {
		t.Fatal(err)
	}
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	eng.Start()
	t.Cleanup(eng.Shutdown)

	address, err := ipc.Address(path)
	if err != nil {
		t.Fatal(err)
	}
	token := waitToken(t, path)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	hello := waitIPC(t, ctx, address, token, ipc.Request{
		V:      ipc.ProtocolVersion,
		Op:     ipc.OpHello,
		Client: &ipc.ClientInfo{Name: "test", Version: "t", Protocol: ipc.ProtocolVersion},
	})
	if hello.Hello == nil || hello.Hello.Owner != string(KindDaemon) {
		t.Fatalf("hello over the socket: %+v", hello)
	}

	followCtx, followCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer followCancel()
	stream, err := ipc.Follow(followCtx, address, token, ipc.Request{V: ipc.ProtocolVersion, Op: ipc.OpSubscribe})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stream.Close() })
	snap := nextEvent(t, stream)
	if snap.State == nil {
		t.Fatalf("subscribe must start with a snapshot: %+v", snap)
	}
	_ = eng.LogHandler(nil).Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "daemon ready", 0))
	publishingPoster{emit: eng.publishNotification}.Post(notify.Notice{Title: "Norka", Body: "db dropped", TunnelID: 4})
	saved := eng.handleAutomation(ipc.Request{
		V:  ipc.ProtocolVersion,
		Op: ipc.OpControl,
		Control: &ipc.Control{
			Action: ipc.ActionSave,
			Kind:   ipc.KindGroup,
			Group:  &model.TunnelGroupPayload{Name: "ops"},
		},
	})
	if !saved.OK {
		t.Fatalf("save during subscribe: %+v", saved)
	}
	hop := eng.handleAutomation(ipc.Request{
		V:  ipc.ProtocolVersion,
		Op: ipc.OpControl,
		Control: &ipc.Control{
			Action: ipc.ActionSave,
			Kind:   ipc.KindJumper,
			Jumper: &model.JumperPayload{Name: "edge", Host: "edge.example", User: "norka", AuthType: "ssh_agent"},
		},
	})
	if !hop.OK || hop.State == nil || len(hop.State.Jumpers) != 1 {
		t.Fatalf("jumper: %+v", hop)
	}
	tunnel := eng.handleAutomation(ipc.Request{
		V:  ipc.ProtocolVersion,
		Op: ipc.OpControl,
		Control: &ipc.Control{
			Action: ipc.ActionSave,
			Kind:   ipc.KindTunnel,
			Tunnel: &model.TunnelPayload{
				Name: "db", Mode: "local", JumperIDs: []int{hop.State.Jumpers[0].ID},
				LocalPort: 5432, RemoteHost: "db.internal", RemotePort: 5432, Status: "running",
			},
		},
	})
	if !tunnel.OK || tunnel.State == nil || len(tunnel.State.Tunnels) != 1 {
		t.Fatalf("tunnel: %+v", tunnel)
	}
	stopped := eng.handleAutomation(ipc.Request{
		V:  ipc.ProtocolVersion,
		Op: ipc.OpControl,
		Control: &ipc.Control{
			Action: ipc.ActionStop,
			Kind:   ipc.KindTunnel,
			ID:     tunnel.State.Tunnels[0].ID,
		},
	})
	if !stopped.OK {
		t.Fatalf("stop: %+v", stopped)
	}

	seen := map[string]bool{}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && len(seen) < 4 {
		ev := nextEvent(t, stream)
		if ev.Event == nil {
			continue
		}
		seen[ev.Event.Type] = true
	}
	for _, kind := range []string{ipc.EventStatus, ipc.EventLog, ipc.EventNotification, ipc.EventConfig} {
		if !seen[kind] {
			t.Fatalf("missing %s in %v", kind, seen)
		}
	}
}

func newIPCEngine(t *testing.T, kind Kind, stats bool) (*Engine, string) {
	t.Helper()
	return newIPCEngineRuntime(t, kind, stats, nil)
}

func newIPCEngineRuntime(t *testing.T, kind Kind, stats bool, rt Runtime) (*Engine, string) {
	t.Helper()
	_ = kind
	path := filepath.Join(t.TempDir(), "config.toml")
	storage, err := conf.NewStorage(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Update(func(cfg *conf.Config) error {
		if err := cfg.Features.Set(features.Automation, true); err != nil {
			return err
		}
		if err := cfg.Features.Set(features.WakeReconnect, false); err != nil {
			return err
		}
		return cfg.Features.Set(features.TunnelStats, stats)
	}); err != nil {
		t.Fatal(err)
	}
	eng := New(Options{
		Storage:    storage,
		Vault:      secrets.NewVault(secrets.NewMemoryKeyring(), true),
		Runtime:    rt,
		Watch:      func(context.Context) (<-chan netwatch.Event, error) { return nil, context.Canceled },
		SyncLogin:  func(bool, bool) error { return nil },
		SyncScheme: func(bool) error { return nil },
		Poster:     func(notify.PosterConfig) notify.Poster { return nopPoster{} },
	})
	eng.SetVersion("test")
	t.Cleanup(func() { eng.Shutdown() })
	return eng, path
}

func waitToken(t *testing.T, configPath string) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		token, err := ipc.ReadToken(ipc.TokenPath(configPath))
		if err == nil {
			return token
		}
		last = err
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("token: %v", last)
	return ""
}

func waitIPC(t *testing.T, ctx context.Context, address, token string, req ipc.Request) ipc.Response {
	t.Helper()
	var last error
	for {
		callCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
		resp, err := ipc.Call(callCtx, address, token, req)
		cancel()
		if err == nil {
			return resp
		}
		last = err
		select {
		case <-ctx.Done():
			t.Fatalf("ipc: %v", last)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func nextEvent(t *testing.T, stream *ipc.Follower) ipc.Response {
	t.Helper()
	_ = stream.SetReadDeadline(time.Now().Add(3 * time.Second))
	resp, err := stream.Next()
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	return resp
}

type secretRuntime struct {
	secrets []string
}

func (s *secretRuntime) SetEvents(biz.TunnelEvents) {}

func (s *secretRuntime) List() ([]model.Tunnel, error) { return nil, nil }

func (s *secretRuntime) Start(id int, _ int) (model.Tunnel, error) {
	return model.Tunnel{ID: id, Status: "running"}, nil
}

func (s *secretRuntime) StartWithSecrets(id int, _ int, secrets []biz.DialSecret) (model.Tunnel, error) {
	for _, item := range secrets {
		s.secrets = append(s.secrets, item.Secret)
	}
	return model.Tunnel{ID: id, Name: "db", Status: "running"}, nil
}

func (s *secretRuntime) Stop(id int) (model.Tunnel, error) {
	return model.Tunnel{ID: id, Status: "stopped"}, nil
}

func (s *secretRuntime) Toggle(int, int) (model.Tunnel, error) { return model.Tunnel{}, nil }

func (s *secretRuntime) StartAutoStart(int) error { return nil }

func (s *secretRuntime) Shutdown() {}

func (s *secretRuntime) RecoverAfterWake(context.Context, netwatch.Kind, time.Time, wake.Options) wake.Result {
	return wake.Result{}
}

type recordingRuntime struct {
	ops []string
}

func (r *recordingRuntime) SetEvents(biz.TunnelEvents) {}

func (r *recordingRuntime) List() ([]model.Tunnel, error) { return nil, nil }

func (r *recordingRuntime) Start(id int, _ int) (model.Tunnel, error) {
	r.ops = append(r.ops, "start")
	return model.Tunnel{ID: id, Name: "db", Status: "running"}, nil
}

func (r *recordingRuntime) Stop(id int) (model.Tunnel, error) {
	r.ops = append(r.ops, "stop")
	return model.Tunnel{ID: id, Name: "db", Status: "stopped"}, nil
}

func (r *recordingRuntime) Toggle(int, int) (model.Tunnel, error) { return model.Tunnel{}, nil }

func (r *recordingRuntime) StartAutoStart(int) error { return nil }

func (r *recordingRuntime) Shutdown() {}

func (r *recordingRuntime) RecoverAfterWake(context.Context, netwatch.Kind, time.Time, wake.Options) wake.Result {
	return wake.Result{}
}

type blockingRuntime struct {
	release chan struct{}
	entered chan struct{}
}

func (b *blockingRuntime) SetEvents(biz.TunnelEvents) {}

func (b *blockingRuntime) List() ([]model.Tunnel, error) { return nil, nil }

func (b *blockingRuntime) Start(int, int) (model.Tunnel, error) { return model.Tunnel{}, nil }

func (b *blockingRuntime) Stop(int) (model.Tunnel, error) { return model.Tunnel{}, nil }

func (b *blockingRuntime) Toggle(int, int) (model.Tunnel, error) { return model.Tunnel{}, nil }

func (b *blockingRuntime) StartAutoStart(int) error { return nil }

func (b *blockingRuntime) Shutdown() {
	select {
	case <-b.entered:
	default:
		close(b.entered)
	}
	<-b.release
}

func (b *blockingRuntime) RecoverAfterWake(context.Context, netwatch.Kind, time.Time, wake.Options) wake.Result {
	return wake.Result{}
}
