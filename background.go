package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/norka-app/Norka/internal/attach"
	"github.com/norka-app/Norka/internal/automation"
	"github.com/norka-app/Norka/internal/biz"
	"github.com/norka-app/Norka/internal/features"
	"github.com/norka-app/Norka/internal/hopsecret"
	"github.com/norka-app/Norka/internal/ipc"
	"github.com/norka-app/Norka/internal/loginstart"
	"github.com/norka-app/Norka/internal/model"
	"github.com/norka-app/Norka/internal/notify"
	"github.com/norka-app/Norka/internal/tunnelstats"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	eventBackgroundNotice = "background:notice"
	eventBackgroundOffer  = "background:offer-stop"
	eventBackgroundStatus = "background:status"
)

// BackgroundStatus is the Settings view of norkad. Notice is an i18n key.
type BackgroundStatus struct {
	Attached     bool   `json:"attached"`
	Running      bool   `json:"running"`
	PID          int    `json:"pid"`
	Version      string `json:"version"`
	LoginStart   bool   `json:"loginStart"`
	Notice       string `json:"notice"`
	HostingLocal bool   `json:"hostingLocal"`
}

// backgroundSession is the window's IPC client. It is idle when background mode is off.
type backgroundSession struct {
	mu       sync.Mutex
	attached bool
	address  string
	token    string
	hello    ipc.Hello
	cancel   context.CancelFunc
	child    *attach.Child
	mirror   map[int]ipc.TunnelInfo
	stats    *[]tunnelstats.View
	notice   string
}

func (a *App) backgroundAttached() bool {
	if a == nil {
		return false
	}
	a.bg.mu.Lock()
	defer a.bg.mu.Unlock()
	return a.bg.attached
}

func (a *App) openStartupTarget() {
	if link := automation.LinkFromArgs(os.Args); link != "" {
		a.HandleDeepLink(link)
		return
	}
	if id := notify.ParseFocusArg(os.Args); id > 0 {
		a.FocusTunnel(id)
	}
}

// beginBackground attaches to norkad when the flag is on.
// false means the caller should host tunnels in this process.
func (a *App) beginBackground() bool {
	if a == nil || a.engine == nil {
		return false
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	result := attach.Resolve(ctx, attach.Request{
		Enabled:  true,
		Hello:    a.helloDaemon,
		Spawn:    a.spawnDaemon,
		Kill:     a.killSpawned,
		Attempts: 40,
		Pause:    100 * time.Millisecond,
	})
	if result.Notice != "" {
		a.noteBackground(result.Notice)
	}
	if !result.Attached {
		a.killSpawned()
		return false
	}
	a.forgetSpawned()
	if err := a.followDaemon(ctx); err != nil {
		a.noteBackground(attach.NoticeFailed)
		a.killSpawned()
		return false
	}
	a.engine.SetTunnelRunner(func(op string, id int) (model.Tunnel, error) {
		return a.remoteRunner(op, id)
	})
	return true
}

func (a *App) detachBackground() {
	if a == nil {
		return
	}
	a.bg.mu.Lock()
	a.bg.attached = false
	cancel := a.bg.cancel
	a.bg.cancel = nil
	a.bg.mirror = nil
	a.bg.stats = nil
	a.bg.mu.Unlock()
	if a.engine != nil {
		a.engine.SetTunnelRunner(nil)
	}
	if cancel != nil {
		cancel()
	}
}

func (a *App) noteBackground(key string) {
	if a == nil {
		return
	}
	a.bg.mu.Lock()
	a.bg.notice = key
	a.bg.mu.Unlock()
	if a.ctx == nil || key == "" {
		return
	}
	wailsruntime.EventsEmit(a.ctx, eventBackgroundNotice, map[string]string{"key": key})
}

func (a *App) noticeKey() string {
	if a == nil {
		return ""
	}
	a.bg.mu.Lock()
	defer a.bg.mu.Unlock()
	return a.bg.notice
}

func (a *App) helloDaemon(ctx context.Context) (ipc.Hello, error) {
	if err := a.loadEndpoint(); err != nil {
		return ipc.Hello{}, err
	}
	a.bg.mu.Lock()
	address, token := a.bg.address, a.bg.token
	a.bg.mu.Unlock()
	callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	resp, err := ipc.Call(callCtx, address, token, ipc.Request{
		V:      ipc.ProtocolVersion,
		Op:     ipc.OpHello,
		Client: a.ipcClient(),
	})
	if err != nil {
		return ipc.Hello{}, err
	}
	if !resp.OK || resp.Hello == nil {
		return ipc.Hello{}, errors.New("hello failed")
	}
	a.bg.mu.Lock()
	a.bg.hello = *resp.Hello
	a.bg.mu.Unlock()
	return *resp.Hello, nil
}

func (a *App) loadEndpoint() error {
	if a == nil || a.storage() == nil {
		return errors.New("config is not ready")
	}
	address, err := ipc.Address(a.storage().Path())
	if err != nil {
		return err
	}
	token, err := ipc.ReadToken(ipc.TokenPath(a.storage().Path()))
	if err != nil {
		if os.IsNotExist(err) {
			return ipc.ErrNotRunning
		}
		return err
	}
	a.bg.mu.Lock()
	a.bg.address = address
	a.bg.token = token
	a.bg.mu.Unlock()
	return nil
}

func (a *App) spawnDaemon(ctx context.Context) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	bin := attach.FindNorkad(exe)
	if bin == "" {
		return attach.ErrBinaryMissing
	}
	if a.storage() == nil {
		return errors.New("config is not ready")
	}
	child, err := attach.Spawn(ctx, bin, a.storage().Path())
	if err != nil {
		return err
	}
	a.bg.mu.Lock()
	a.bg.child = child
	a.bg.mu.Unlock()
	return nil
}

func (a *App) killSpawned() {
	if a == nil {
		return
	}
	a.bg.mu.Lock()
	child := a.bg.child
	a.bg.child = nil
	a.bg.mu.Unlock()
	child.Stop()
}

func (a *App) forgetSpawned() {
	if a == nil {
		return
	}
	a.bg.mu.Lock()
	a.bg.child = nil
	a.bg.mu.Unlock()
}

func (a *App) followDaemon(parent context.Context) error {
	if err := a.loadEndpoint(); err != nil {
		return err
	}
	a.bg.mu.Lock()
	address, token := a.bg.address, a.bg.token
	a.bg.mu.Unlock()
	ctx, cancel := context.WithCancel(parent)
	stream, err := ipc.Follow(ctx, address, token, ipc.Request{
		V:      ipc.ProtocolVersion,
		Op:     ipc.OpSubscribe,
		Client: a.ipcClient(),
	})
	if err != nil {
		cancel()
		return err
	}
	_ = stream.SetReadDeadline(time.Now().Add(5 * time.Second))
	first, err := stream.Next()
	if err != nil || first.State == nil || !first.OK {
		_ = stream.Close()
		cancel()
		if err == nil {
			err = errors.New("daemon sent no snapshot")
		}
		return err
	}
	_ = stream.SetReadDeadline(time.Time{})
	a.applySnapshot(*first.State)
	a.bg.mu.Lock()
	a.bg.attached = true
	a.bg.cancel = cancel
	a.bg.mu.Unlock()
	go a.readBackground(ctx, stream)
	a.afterTrayAction()
	return nil
}

func (a *App) readBackground(ctx context.Context, stream *ipc.Follower) {
	defer stream.Close()
	for {
		resp, err := stream.Next()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			a.onBackgroundLost()
			return
		}
		a.applyRemote(resp)
	}
}

func (a *App) onBackgroundLost() {
	a.bg.mu.Lock()
	if !a.bg.attached {
		a.bg.mu.Unlock()
		return
	}
	a.bg.attached = false
	a.bg.mu.Unlock()
	if a.engine != nil {
		a.engine.SetTunnelRunner(nil)
	}
	if !a.featureOn(features.BackgroundMode) {
		return
	}
	a.noteBackground(attach.NoticeFailed)
	a.becomeLocalHost(true)
}

func (a *App) applyRemote(resp ipc.Response) {
	if resp.State != nil {
		a.applySnapshot(*resp.State)
	}
	if resp.Event != nil {
		switch resp.Event.Type {
		case ipc.EventStatus:
			if resp.Event.Tunnel != nil {
				a.bg.mu.Lock()
				if a.bg.mirror == nil {
					a.bg.mirror = map[int]ipc.TunnelInfo{}
				}
				a.bg.mirror[resp.Event.Tunnel.ID] = *resp.Event.Tunnel
				a.bg.mu.Unlock()
			}
		case ipc.EventConfig:
			if resp.Event.State != nil {
				a.applySnapshot(*resp.Event.State)
			}
		}
	}
	a.afterTrayAction()
}

func (a *App) applySnapshot(state ipc.State) {
	mirror := make(map[int]ipc.TunnelInfo, len(state.Tunnels))
	for _, item := range state.Tunnels {
		mirror[item.ID] = item
	}
	a.bg.mu.Lock()
	a.bg.mirror = mirror
	a.bg.stats = state.Stats
	a.bg.mu.Unlock()
}

func (a *App) overlayTunnels(items []model.Tunnel) []model.Tunnel {
	if !a.backgroundAttached() {
		return items
	}
	a.bg.mu.Lock()
	defer a.bg.mu.Unlock()
	if len(a.bg.mirror) == 0 {
		return items
	}
	for i := range items {
		info, ok := a.bg.mirror[items[i].ID]
		if !ok {
			continue
		}
		items[i].Status = info.Status
		items[i].LastError = info.LastError
	}
	return items
}

func (a *App) ipcClient() *ipc.ClientInfo {
	return &ipc.ClientInfo{Name: "norka", Version: appVersion(), Protocol: ipc.ProtocolVersion}
}

func (a *App) ipcCall(ctx context.Context, req ipc.Request) (ipc.Response, error) {
	if err := a.loadEndpoint(); err != nil {
		return ipc.Response{}, err
	}
	a.bg.mu.Lock()
	address, token := a.bg.address, a.bg.token
	a.bg.mu.Unlock()
	req.V = ipc.ProtocolVersion
	if req.Client == nil {
		req.Client = a.ipcClient()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	callCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	resp, err := ipc.Call(callCtx, address, token, req)
	if req.Control != nil {
		for i := range req.Control.Secrets {
			req.Control.Secrets[i].Secret = ""
		}
	}
	if err != nil {
		return resp, err
	}
	if !resp.OK {
		msg := strings.TrimSpace(resp.Message)
		if msg == "" {
			msg = resp.Code
		}
		return resp, errors.New(msg)
	}
	if resp.State != nil {
		a.applySnapshot(*resp.State)
	}
	return resp, nil
}

func (a *App) remoteRunner(op string, id int) (model.Tunnel, error) {
	switch op {
	case ipc.OpConnect:
		return a.remoteStart(id)
	case ipc.OpDisconnect:
		return a.remoteStop(id)
	case ipc.OpToggle:
		return a.remoteToggle(id)
	default:
		return model.Tunnel{}, errors.New("unsupported command")
	}
}

func (a *App) remoteToggle(id int) (model.Tunnel, error) {
	items, err := a.ListTunnels()
	if err != nil {
		return model.Tunnel{}, err
	}
	status := ""
	for _, item := range items {
		if item.ID == id {
			status = item.Status
			break
		}
	}
	switch status {
	case "running", "busy", "reconnecting":
		return a.remoteStop(id)
	default:
		return a.remoteStart(id)
	}
}

func (a *App) remoteStart(id int) (model.Tunnel, error) {
	secrets := a.secretsForTunnel(id)
	defer func() {
		for i := range secrets {
			secrets[i].Secret = ""
		}
	}()
	_, err := a.ipcCall(context.Background(), ipc.Request{
		Op: ipc.OpControl,
		Control: &ipc.Control{
			Action:  ipc.ActionStart,
			Kind:    ipc.KindTunnel,
			ID:      id,
			Secrets: secrets,
		},
	})
	if err != nil {
		return model.Tunnel{}, err
	}
	return a.storedTunnel(id)
}

func (a *App) remoteStop(id int) (model.Tunnel, error) {
	_, err := a.ipcCall(context.Background(), ipc.Request{
		Op:      ipc.OpControl,
		Control: &ipc.Control{Action: ipc.ActionStop, Kind: ipc.KindTunnel, ID: id},
	})
	if err != nil {
		return model.Tunnel{}, err
	}
	return a.storedTunnel(id)
}

func (a *App) remoteSwitch(id int) error {
	items, err := a.ListTunnels()
	if err != nil {
		return err
	}
	target, ok := findTunnel(items, id)
	if !ok {
		return biz.ErrTunnelNotFound
	}
	for _, conflict := range biz.PortConflicts(target, items) {
		if _, err := a.remoteStop(conflict.ID); err != nil {
			return err
		}
	}
	_, err = a.remoteStart(id)
	return err
}

func (a *App) secretsForTunnel(id int) []ipc.HopSecret {
	if a == nil || a.storage() == nil {
		return nil
	}
	cfg, err := a.storage().Load()
	if err != nil {
		return nil
	}
	var tunnel model.Tunnel
	found := false
	for _, item := range cfg.Tunnels {
		if item.ID == id {
			tunnel = item
			found = true
			break
		}
	}
	if !found {
		return nil
	}
	var out []ipc.HopSecret
	for _, jumperID := range tunnel.JumperIDs {
		for _, item := range cfg.Jumpers {
			if item.ID != jumperID {
				continue
			}
			if hopsecret.Reason([]model.Jumper{item}) == "" {
				break
			}
			hop := item
			if a.vault() != nil {
				a.vault().Open(&hop)
			}
			secret := strings.TrimSpace(hop.Password)
			hop.Password = ""
			if secret != "" {
				out = append(out, ipc.HopSecret{JumperID: item.ID, Secret: secret})
			}
			break
		}
	}
	return out
}

func (a *App) storedTunnel(id int) (model.Tunnel, error) {
	items, err := a.ListTunnels()
	if err != nil {
		return model.Tunnel{}, err
	}
	item, ok := findTunnel(items, id)
	if !ok {
		return model.Tunnel{}, biz.ErrTunnelNotFound
	}
	return item, nil
}

func (a *App) saveRemoteTunnel(id int, payload model.TunnelPayload) (model.Tunnel, error) {
	before := a.tunnelIDs()
	ctrl := &ipc.Control{Action: ipc.ActionSave, Kind: ipc.KindTunnel, Tunnel: &payload}
	if id > 0 {
		ctrl.ID = id
	}
	if _, err := a.ipcCall(context.Background(), ipc.Request{Op: ipc.OpControl, Control: ctrl}); err != nil {
		return model.Tunnel{}, err
	}
	return a.pickSavedTunnel(id, before)
}

func (a *App) saveRemoteJumper(id int, payload model.JumperPayload) (model.Jumper, error) {
	before := a.jumperIDs()
	ctrl := &ipc.Control{Action: ipc.ActionSave, Kind: ipc.KindJumper, Jumper: &payload}
	if id > 0 {
		ctrl.ID = id
	}
	if _, err := a.ipcCall(context.Background(), ipc.Request{Op: ipc.OpControl, Control: ctrl}); err != nil {
		return model.Jumper{}, err
	}
	items, err := a.jumper().List()
	if err != nil {
		return model.Jumper{}, err
	}
	if id > 0 {
		for _, item := range items {
			if item.ID == id {
				return item, nil
			}
		}
		return model.Jumper{}, biz.ErrJumperNotFound
	}
	for _, item := range items {
		if _, ok := before[item.ID]; !ok {
			return item, nil
		}
	}
	return model.Jumper{}, errors.New("saved jumper was not found")
}

func (a *App) saveRemoteGroup(id int, payload model.TunnelGroupPayload) (model.TunnelGroup, error) {
	before := a.groupIDs()
	ctrl := &ipc.Control{Action: ipc.ActionSave, Kind: ipc.KindGroup, Group: &payload}
	if id > 0 {
		ctrl.ID = id
	}
	if _, err := a.ipcCall(context.Background(), ipc.Request{Op: ipc.OpControl, Control: ctrl}); err != nil {
		return model.TunnelGroup{}, err
	}
	items, err := a.group().List()
	if err != nil {
		return model.TunnelGroup{}, err
	}
	if id > 0 {
		for _, item := range items {
			if item.ID == id {
				return item, nil
			}
		}
		return model.TunnelGroup{}, biz.ErrGroupNotFound
	}
	for _, item := range items {
		if _, ok := before[item.ID]; !ok {
			return item, nil
		}
	}
	return model.TunnelGroup{}, errors.New("saved group was not found")
}

func (a *App) deleteRemote(kind string, id int) error {
	_, err := a.ipcCall(context.Background(), ipc.Request{
		Op:      ipc.OpControl,
		Control: &ipc.Control{Action: ipc.ActionDelete, Kind: kind, ID: id},
	})
	return err
}

func (a *App) pickSavedTunnel(id int, before map[int]struct{}) (model.Tunnel, error) {
	items, err := a.ListTunnels()
	if err != nil {
		return model.Tunnel{}, err
	}
	if id > 0 {
		item, ok := findTunnel(items, id)
		if !ok {
			return model.Tunnel{}, biz.ErrTunnelNotFound
		}
		return item, nil
	}
	for _, item := range items {
		if _, ok := before[item.ID]; !ok {
			return item, nil
		}
	}
	return model.Tunnel{}, errors.New("saved tunnel was not found")
}

func (a *App) tunnelIDs() map[int]struct{} {
	items, err := a.tunnel().List()
	out := map[int]struct{}{}
	if err != nil {
		return out
	}
	for _, item := range items {
		out[item.ID] = struct{}{}
	}
	return out
}

func (a *App) jumperIDs() map[int]struct{} {
	items, err := a.jumper().List()
	out := map[int]struct{}{}
	if err != nil {
		return out
	}
	for _, item := range items {
		out[item.ID] = struct{}{}
	}
	return out
}

func (a *App) groupIDs() map[int]struct{} {
	items, err := a.group().List()
	out := map[int]struct{}{}
	if err != nil {
		return out
	}
	for _, item := range items {
		out[item.ID] = struct{}{}
	}
	return out
}

func (a *App) handoverDaemon() error {
	_, err := a.ipcCall(context.Background(), ipc.Request{
		Op:        ipc.OpHandover,
		TimeoutMS: int(ipc.DefaultHandoverTimeout / time.Millisecond),
	})
	return err
}

func (a *App) shutdownDaemon() error {
	_, err := a.ipcCall(context.Background(), ipc.Request{Op: ipc.OpShutdown})
	return err
}

func (a *App) releaseLocalHost() {
	if a == nil || a.engine == nil {
		return
	}
	a.engine.SyncWakeWatch(false)
	a.engine.StopAutomation()
	a.engine.ShutdownTunnels()
	a.engine.Release()
}

func (a *App) becomeLocalHost(autostart bool) {
	if a == nil || a.engine == nil {
		return
	}
	a.claimEngine()
	if !a.engineStarted {
		a.engine.Start()
		a.engineStarted = true
	} else {
		a.engine.SyncWakeWatch(a.featureOn(features.WakeReconnect))
		a.syncAutomation()
	}
	if autostart {
		a.engine.StartAutoStart()
	}
	a.afterTrayAction()
}

func (a *App) handOffToDaemon() {
	if a == nil || a.backgroundAttached() {
		return
	}
	a.releaseLocalHost()
	if a.beginBackground() {
		a.emitBackgroundStatus()
		return
	}
	a.becomeLocalHost(true)
	a.emitBackgroundStatus()
}

func (a *App) onBackgroundFlag(id features.ID, enabled bool) {
	if id != features.BackgroundMode {
		return
	}
	if !enabled {
		_ = loginstart.Disable(a.loginSpec())
		a.hideBackgroundQuitItem()
		if a.backgroundAttached() {
			a.emitOfferStop()
		}
		return
	}
	a.installBackgroundQuitItem()
	if !a.backgroundAttached() {
		a.handOffToDaemon()
	}
}

func (a *App) emitOfferStop() {
	if a == nil || a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, eventBackgroundOffer, map[string]bool{"running": true})
}

func (a *App) emitBackgroundStatus() {
	if a == nil || a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, eventBackgroundStatus, map[string]bool{"refresh": true})
}

// GetBackgroundStatus reports norkad for the Settings section.
func (a *App) GetBackgroundStatus() (BackgroundStatus, error) {
	if err := a.ensureReady(); err != nil {
		return BackgroundStatus{}, err
	}
	status := BackgroundStatus{Notice: a.noticeKey()}
	if a.backgroundAttached() {
		a.bg.mu.Lock()
		status.Attached = true
		status.Running = true
		status.PID = a.bg.hello.PID
		status.Version = a.bg.hello.Version
		a.bg.mu.Unlock()
	}
	on, err := loginstart.Enabled(a.loginSpec())
	if err == nil {
		status.LoginStart = on
	}
	status.HostingLocal = a.engineStarted && !status.Attached
	return status, nil
}

// StopBackgroundDaemon asks norkad to release the engine, then this window hosts tunnels.
func (a *App) StopBackgroundDaemon() error {
	if err := a.ensureReady(); err != nil {
		return err
	}
	if a.backgroundAttached() {
		if err := a.handoverDaemon(); err != nil {
			return err
		}
		a.detachBackground()
	}
	a.becomeLocalHost(true)
	a.emitBackgroundStatus()
	return nil
}

// QuitAndStopTunnels stops norkad, when this window is attached, and then quits.
// With the flag off the ordinary quit already stops the tunnels this window hosts.
func (a *App) QuitAndStopTunnels() {
	if a.backgroundAttached() {
		_ = a.shutdownDaemon()
		a.detachBackground()
	}
	a.PrepareForQuit()
	if a != nil && a.ctx != nil {
		wailsruntime.Quit(a.ctx)
		return
	}
	os.Exit(0)
}

// SetBackgroundLoginStart registers or removes the norkad login entry.
func (a *App) SetBackgroundLoginStart(enabled bool) error {
	if err := a.ensureReady(); err != nil {
		return err
	}
	spec := a.loginSpec()
	if !enabled {
		return loginstart.Disable(spec)
	}
	if strings.TrimSpace(spec.Exe) == "" {
		return attach.ErrBinaryMissing
	}
	_, err := loginstart.Enable(spec)
	return err
}

func (a *App) loginSpec() loginstart.Spec {
	exe, _ := os.Executable()
	return loginstart.CurrentSpec(attach.FindNorkad(exe), loginRegistry(), execRun)
}

func execRun(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

func (a *App) stopTunnelID(id int) (model.Tunnel, error) {
	if a.backgroundAttached() {
		return a.remoteStop(id)
	}
	return a.tunnel().Stop(id)
}

func findTunnel(items []model.Tunnel, id int) (model.Tunnel, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return model.Tunnel{}, false
}
