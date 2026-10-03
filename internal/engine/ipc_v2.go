package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/norka-app/Norka/internal/automation"
	"github.com/norka-app/Norka/internal/biz"
	"github.com/norka-app/Norka/internal/features"
	"github.com/norka-app/Norka/internal/hopsecret"
	"github.com/norka-app/Norka/internal/ipc"
	"github.com/norka-app/Norka/internal/model"
	"github.com/norka-app/Norka/internal/tunnelstats"
)

func (e *Engine) handleV2(req ipc.Request) ipc.Response {
	switch req.Op {
	case ipc.OpHello:
		return e.handleHello(req)
	case ipc.OpState:
		return e.handleState()
	case ipc.OpControl:
		return e.handleControl(req)
	case ipc.OpShutdown:
		return e.handleShutdown(req)
	case ipc.OpHandover:
		return e.handleHandover(req)
	default:
		return ipc.Response{Code: ipc.CodeBadRequest, ExitCode: ipc.ExitUsage, Message: "bad request"}
	}
}

func (e *Engine) handleHello(req ipc.Request) ipc.Response {
	if req.Client != nil {
		slog.Info("ipc hello", "client", req.Client.Name, "version", req.Client.Version, "protocol", req.Client.Protocol)
	}
	meta := e.ownerMeta()
	version := strings.TrimSpace(meta.Version)
	if version == "" {
		e.ownerMu.Lock()
		version = e.version
		e.ownerMu.Unlock()
	}
	return ipc.Response{
		OK:       true,
		Code:     ipc.CodeOK,
		ExitCode: ipc.ExitOK,
		Hello: &ipc.Hello{
			ProtocolVersion:  ipc.ProtocolVersion,
			MinClientVersion: ipc.MinClientVersion,
			Owner:            string(meta.Kind),
			PID:              meta.PID,
			Version:          version,
		},
	}
}

func (e *Engine) handleState() ipc.Response {
	st, err := e.snapshot()
	if err != nil {
		return ipcFailed(err)
	}
	return ipc.Response{OK: true, Code: ipc.CodeOK, ExitCode: ipc.ExitOK, State: &st}
}

func (e *Engine) handleSubscribe(ctx context.Context, req ipc.Request, send func(ipc.Response) error) error {
	if !e.FeatureOn(features.Automation) && !e.windowIPC() {
		return send(ipc.Response{
			Code:     ipc.CodeDisabled,
			ExitCode: ipc.ExitDisabled,
			Message:  "automation is disabled",
		})
	}
	_ = req
	sub := e.addSub()
	defer e.removeSub(sub)
	st, err := e.snapshot()
	if err != nil {
		return send(ipcFailed(err))
	}
	if err := send(ipc.Response{OK: true, Code: ipc.CodeOK, ExitCode: ipc.ExitOK, State: &st}); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev := <-sub.ch:
			if err := send(ipc.Response{OK: true, Code: ipc.CodeOK, ExitCode: ipc.ExitOK, Event: &ev}); err != nil {
				return err
			}
		}
	}
}

func (e *Engine) handleControl(req ipc.Request) ipc.Response {
	if !e.ownsEngine() {
		return notOwnerResponse()
	}
	if req.Control == nil {
		return ipc.Response{Code: ipc.CodeBadRequest, ExitCode: ipc.ExitUsage, Message: "control is required"}
	}
	switch req.Control.Action {
	case ipc.ActionStart, ipc.ActionStop, ipc.ActionRestart:
		return e.controlTunnelRun(req)
	case ipc.ActionSave:
		return e.controlSave(req.Control)
	case ipc.ActionDelete:
		return e.controlDelete(req.Control)
	default:
		return ipc.Response{Code: ipc.CodeBadRequest, ExitCode: ipc.ExitUsage, Message: "unsupported control action"}
	}
}

func (e *Engine) controlTunnelRun(req ipc.Request) ipc.Response {
	ctrl := req.Control
	if ctrl.Kind != "" && ctrl.Kind != ipc.KindTunnel {
		return ipc.Response{Code: ipc.CodeBadRequest, ExitCode: ipc.ExitUsage, Message: "start, stop and restart apply to a tunnel"}
	}
	rt := e.active()
	if rt == nil {
		return ipcFailed(errors.New("app is not initialized"))
	}
	id := ctrl.ID
	if id <= 0 {
		target := strings.TrimSpace(req.Target)
		if target == "" {
			return ipc.Response{Code: ipc.CodeBadRequest, ExitCode: ipc.ExitUsage, Message: "tunnel id is required"}
		}
		found, resp, ok := e.matchTunnel(target)
		if !ok {
			return resp
		}
		id = found.ID
	}
	var (
		tunnel model.Tunnel
		err    error
	)
	switch ctrl.Action {
	case ipc.ActionStart:
		tunnel, err = e.startOwned(rt, id, ctrl.Secrets)
	case ipc.ActionStop:
		tunnel, err = rt.Stop(id)
	case ipc.ActionRestart:
		if _, err = rt.Stop(id); err == nil {
			tunnel, err = e.startOwned(rt, id, ctrl.Secrets)
		}
	}
	wipeHopSecrets(ctrl.Secrets)
	if err != nil {
		return ipcTunnelErr(err)
	}
	if tunnel.Status == "error" && ctrl.Action != ipc.ActionStop {
		detail := tunnel.LastError
		if detail == "" {
			detail = "tunnel command failed"
		}
		return ipc.Response{
			Code:     ipc.CodeFailed,
			ExitCode: ipc.ExitFailed,
			Message:  detail,
			Tunnels:  tunnelInfos([]model.Tunnel{tunnel}),
		}
	}
	return ipc.Response{
		OK:       true,
		Code:     ipc.CodeOK,
		ExitCode: ipc.ExitOK,
		Tunnels:  tunnelInfos([]model.Tunnel{tunnel}),
	}
}

func (e *Engine) controlSave(ctrl *ipc.Control) ipc.Response {
	var err error
	switch ctrl.Kind {
	case ipc.KindTunnel:
		if ctrl.Tunnel == nil {
			return ipc.Response{Code: ipc.CodeBadRequest, ExitCode: ipc.ExitUsage, Message: "tunnel payload is required"}
		}
		if ctrl.ID > 0 {
			_, err = e.tunnel.Update(ctrl.ID, *ctrl.Tunnel)
		} else {
			_, err = e.tunnel.Create(*ctrl.Tunnel)
		}
	case ipc.KindJumper:
		if ctrl.Jumper == nil {
			return ipc.Response{Code: ipc.CodeBadRequest, ExitCode: ipc.ExitUsage, Message: "jumper payload is required"}
		}
		if ctrl.ID > 0 {
			_, err = e.jumper.Update(ctrl.ID, *ctrl.Jumper)
		} else {
			_, err = e.jumper.Create(*ctrl.Jumper)
		}
	case ipc.KindGroup:
		if ctrl.Group == nil {
			return ipc.Response{Code: ipc.CodeBadRequest, ExitCode: ipc.ExitUsage, Message: "group payload is required"}
		}
		if ctrl.ID > 0 {
			_, err = e.group.Update(ctrl.ID, *ctrl.Group)
		} else {
			_, err = e.group.Create(*ctrl.Group)
		}
	default:
		return ipc.Response{Code: ipc.CodeBadRequest, ExitCode: ipc.ExitUsage, Message: "control kind must be tunnel, jumper or group"}
	}
	if err != nil {
		return ipcTunnelErr(err)
	}
	return e.finishConfigChange()
}

func (e *Engine) controlDelete(ctrl *ipc.Control) ipc.Response {
	if ctrl.ID <= 0 {
		return ipc.Response{Code: ipc.CodeBadRequest, ExitCode: ipc.ExitUsage, Message: "id is required"}
	}
	var err error
	switch ctrl.Kind {
	case ipc.KindTunnel:
		err = e.tunnel.Delete(ctrl.ID)
	case ipc.KindJumper:
		err = e.jumper.Delete(ctrl.ID)
	case ipc.KindGroup:
		err = e.group.Delete(ctrl.ID)
	default:
		return ipc.Response{Code: ipc.CodeBadRequest, ExitCode: ipc.ExitUsage, Message: "control kind must be tunnel, jumper or group"}
	}
	if err != nil {
		return ipcTunnelErr(err)
	}
	return e.finishConfigChange()
}

func (e *Engine) finishConfigChange() ipc.Response {
	e.publishConfig()
	st, err := e.snapshot()
	if err != nil {
		return ipcFailed(err)
	}
	return ipc.Response{OK: true, Code: ipc.CodeOK, ExitCode: ipc.ExitOK, State: &st}
}

func (e *Engine) handleShutdown(req ipc.Request) ipc.Response {
	kind := e.ownerKind()
	if kind == "" {
		return notOwnerResponse()
	}
	if kind == KindGUI && !req.Force {
		return ipc.Response{
			Code:     ipc.CodeIgnored,
			ExitCode: ipc.ExitFailed,
			Message:  "the window ignored shutdown; pass force to stop it",
		}
	}
	e.ShutdownTunnels()
	return ipc.Response{
		OK:       true,
		Code:     ipc.CodeOK,
		ExitCode: ipc.ExitOK,
		Message:  "stopping",
		After:    e.signalStop,
	}
}

func (e *Engine) handleHandover(req ipc.Request) ipc.Response {
	if !e.ownsEngine() {
		return notOwnerResponse()
	}
	timeout := time.Duration(req.TimeoutMS) * time.Millisecond
	if err := e.stopTunnelsWithin(timeout); err != nil {
		return ipc.Response{
			Code:     ipc.CodeTimeout,
			ExitCode: ipc.ExitFailed,
			Message:  err.Error(),
		}
	}
	e.Release()
	return ipc.Response{
		OK:       true,
		Code:     ipc.CodeOK,
		ExitCode: ipc.ExitOK,
		Message:  "engine lock released",
		After:    e.signalStop,
	}
}

func (e *Engine) stopTunnelsWithin(timeout time.Duration) error {
	if timeout <= 0 {
		timeout = ipc.DefaultHandoverTimeout
	}
	done := make(chan struct{})
	go func() {
		e.ShutdownTunnels()
		close(done)
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-timer.C:
		return fmt.Errorf("handover timed out after %s; the owner still holds the engine lock", timeout)
	}
}

func (e *Engine) snapshot() (ipc.State, error) {
	if e == nil || e.tunnel == nil || e.jumper == nil || e.group == nil {
		return ipc.State{}, errors.New("app is not initialized")
	}
	tunnels, err := e.tunnel.List()
	if err != nil {
		return ipc.State{}, err
	}
	jumpers, err := e.jumper.List()
	if err != nil {
		return ipc.State{}, err
	}
	groups, err := e.group.List()
	if err != nil {
		return ipc.State{}, err
	}
	st := ipc.State{
		Tunnels: tunnelInfos(tunnels),
		Jumpers: jumperInfos(jumpers),
		Groups:  groupInfos(groups),
	}
	if e.FeatureOn(features.TunnelStats) {
		views := e.tunnel.Stats()
		if views == nil {
			views = []tunnelstats.View{}
		}
		st.Stats = &views
	}
	return st, nil
}

func (e *Engine) matchTunnel(target string) (model.Tunnel, ipc.Response, bool) {
	if e == nil || e.storage == nil {
		return model.Tunnel{}, ipcFailed(errors.New("app is not initialized")), false
	}
	cfg, err := e.storage.Load()
	if err != nil {
		return model.Tunnel{}, ipcFailed(err), false
	}
	tunnel, err := automation.Match(cfg.Tunnels, target)
	if err != nil {
		var ambiguous *automation.AmbiguousError
		if errors.As(err, &ambiguous) {
			return model.Tunnel{}, ipc.Response{
				Code:     ipc.CodeAmbiguous,
				ExitCode: ipc.ExitAmbiguous,
				Message:  "name matches more than one tunnel",
				Names:    ambiguous.Names(),
			}, false
		}
		return model.Tunnel{}, ipc.Response{
			Code:     ipc.CodeNotFound,
			ExitCode: ipc.ExitNotFound,
			Message:  "tunnel not found",
		}, false
	}
	return tunnel, ipc.Response{}, true
}

func (e *Engine) ownsEngine() bool {
	if e == nil {
		return false
	}
	e.ownerMu.Lock()
	defer e.ownerMu.Unlock()
	return e.owner != nil
}

func (e *Engine) ownerKind() Kind {
	if e == nil {
		return ""
	}
	e.ownerMu.Lock()
	defer e.ownerMu.Unlock()
	if e.owner != nil {
		return e.owner.Meta().Kind
	}
	if e.hostWithoutLock {
		return KindGUI
	}
	return ""
}

func (e *Engine) ownerMeta() OwnerMeta {
	if e == nil {
		return OwnerMeta{}
	}
	e.ownerMu.Lock()
	owner := e.owner
	e.ownerMu.Unlock()
	if owner != nil {
		return owner.Meta()
	}
	if e.storage == nil {
		return OwnerMeta{}
	}
	meta, err := ReadOwner(e.storage.Path())
	if err != nil {
		return OwnerMeta{}
	}
	return meta
}

func jumperInfos(items []model.Jumper) []ipc.JumperInfo {
	out := make([]ipc.JumperInfo, 0, len(items))
	for _, item := range items {
		out = append(out, ipc.JumperInfo{
			ID:                     item.ID,
			Name:                   item.Name,
			Host:                   item.Host,
			Port:                   item.Port,
			User:                   item.User,
			AuthType:               item.AuthType,
			KeyPath:                item.KeyPath,
			AgentSocketPath:        item.AgentSocketPath,
			HasSecret:              item.HasSecret || item.Password != "" || strings.TrimSpace(item.SecretRef) != "",
			BypassHostVerification: item.BypassHostVerification,
			KeepAliveIntervalMs:    item.KeepAliveIntervalMs,
			TimeoutMs:              item.TimeoutMs,
			HostKeyAlgorithms:      item.HostKeyAlgorithms,
			Notes:                  item.Notes,
		})
	}
	return out
}

func groupInfos(items []model.TunnelGroup) []ipc.GroupInfo {
	out := make([]ipc.GroupInfo, 0, len(items))
	for _, item := range items {
		out = append(out, ipc.GroupInfo{ID: item.ID, Name: item.Name})
	}
	return out
}

// windowIPC is the channel the GUI uses while norkad holds the engine.
// Automation may be off; legacy commands still refuse that case.
func (e *Engine) windowIPC() bool {
	return e != nil && e.ownerKind() == KindDaemon && e.FeatureOn(features.BackgroundMode)
}

type secretStarter interface {
	StartWithSecrets(id int, maxRunning int, secrets []biz.DialSecret) (model.Tunnel, error)
}

func (e *Engine) startOwned(rt Runtime, id int, secrets []ipc.HopSecret) (model.Tunnel, error) {
	defer wipeHopSecrets(secrets)
	reason, found := e.secretBlock(id, secrets)
	if found && reason != "" {
		return model.Tunnel{}, errors.New(reason)
	}
	if starter, ok := rt.(secretStarter); ok && len(secrets) > 0 {
		return starter.StartWithSecrets(id, e.TunnelStartLimit(), dialSecrets(secrets))
	}
	if rt == nil {
		return model.Tunnel{}, errors.New("app is not initialized")
	}
	return rt.Start(id, e.TunnelStartLimit())
}

func (e *Engine) secretBlock(id int, secrets []ipc.HopSecret) (string, bool) {
	if e == nil || e.tunnel == nil || e.jumper == nil || id <= 0 {
		return "", false
	}
	tunnels, err := e.tunnel.List()
	if err != nil {
		return "", false
	}
	var tunnel model.Tunnel
	found := false
	for _, item := range tunnels {
		if item.ID == id {
			tunnel = item
			found = true
			break
		}
	}
	if !found {
		return "", false
	}
	jumpers, err := e.jumper.List()
	if err != nil {
		return "", false
	}
	index := map[int]model.Jumper{}
	for _, item := range jumpers {
		index[item.ID] = item
	}
	hops := make([]model.Jumper, 0, len(tunnel.JumperIDs))
	for _, jumperID := range tunnel.JumperIDs {
		hop, ok := index[jumperID]
		if !ok {
			continue
		}
		hops = append(hops, hop)
	}
	covered := map[int]bool{}
	for _, item := range secrets {
		if strings.TrimSpace(item.Secret) != "" {
			covered[item.JumperID] = true
		}
	}
	return hopsecret.Uncovered(hops, func(jumperID int) bool { return covered[jumperID] }), true
}

func dialSecrets(items []ipc.HopSecret) []biz.DialSecret {
	out := make([]biz.DialSecret, 0, len(items))
	for _, item := range items {
		out = append(out, biz.DialSecret{JumperID: item.JumperID, Secret: item.Secret})
	}
	return out
}

func wipeHopSecrets(items []ipc.HopSecret) {
	for i := range items {
		items[i].Secret = ""
	}
}

func notOwnerResponse() ipc.Response {
	return ipc.Response{
		Code:     ipc.CodeFailed,
		ExitCode: ipc.ExitFailed,
		Message:  "this process does not hold the engine lock",
	}
}

func ipcFailed(err error) ipc.Response {
	if err == nil {
		err = errors.New("command failed")
	}
	return ipc.Response{Code: ipc.CodeFailed, ExitCode: ipc.ExitFailed, Message: err.Error()}
}

func ipcTunnelErr(err error) ipc.Response {
	if errors.Is(err, biz.ErrTunnelNotFound) || errors.Is(err, biz.ErrJumperNotFound) || errors.Is(err, biz.ErrGroupNotFound) {
		return ipc.Response{Code: ipc.CodeNotFound, ExitCode: ipc.ExitNotFound, Message: err.Error()}
	}
	return ipcFailed(err)
}
