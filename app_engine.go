package main

import (
	"github.com/norka-app/Norka/internal/biz"
	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/secrets"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// wailsSink forwards engine events to the Wails window and the tray.
type wailsSink struct {
	app *App
}

func (s wailsSink) Emit(event string, payload any) {
	if s.app == nil || s.app.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(s.app.ctx, event, payload)
}

func (s wailsSink) ShowWindow() {
	if s.app == nil {
		return
	}
	s.app.showMainWindow()
}

func (s wailsSink) TunnelsChanged() {
	if s.app == nil {
		return
	}
	s.app.afterTrayAction()
}

func (a *App) storage() *conf.Storage {
	if a == nil || a.engine == nil {
		return nil
	}
	return a.engine.Storage()
}

func (a *App) tunnel() *biz.TunnelBiz {
	if a == nil || a.engine == nil {
		return nil
	}
	return a.engine.Tunnel()
}

func (a *App) jumper() *biz.JumperBiz {
	if a == nil || a.engine == nil {
		return nil
	}
	return a.engine.Jumper()
}

func (a *App) group() *biz.GroupBiz {
	if a == nil || a.engine == nil {
		return nil
	}
	return a.engine.Group()
}

func (a *App) profile() *biz.ProfileBiz {
	if a == nil || a.engine == nil {
		return nil
	}
	return a.engine.Profile()
}

func (a *App) vault() *secrets.Vault {
	if a == nil || a.engine == nil {
		return nil
	}
	return a.engine.Vault()
}
