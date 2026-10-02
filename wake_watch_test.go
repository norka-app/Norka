package main

import "testing"

func TestWakeWatchStaysOffWhenFlagDisabled(t *testing.T) {
	app := &App{}
	app.syncWakeWatch(false)
	if app.wakeCancel != nil {
		t.Fatal("disabled flag started the watcher")
	}
}

func TestWakeWatchStartsAndStopsWithFlag(t *testing.T) {
	app := &App{}
	app.syncWakeWatch(true)
	if app.wakeCancel == nil {
		t.Fatal("enabled flag did not start the watcher")
	}
	app.syncWakeWatch(true)
	if app.wakeCancel == nil {
		t.Fatal("second enable dropped the watcher")
	}
	app.syncWakeWatch(false)
	if app.wakeCancel != nil {
		t.Fatal("turning the flag off left the watcher running")
	}
}
