package main

func (a *App) syncWakeWatch(enabled bool) {
	if a == nil || a.engine == nil {
		return
	}
	a.engine.SyncWakeWatch(enabled)
}
