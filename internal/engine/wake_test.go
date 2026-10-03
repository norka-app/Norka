package engine

import "testing"

func TestWakeWatchStaysOffWhenFlagDisabled(t *testing.T) {
	eng := &Engine{}
	eng.SyncWakeWatch(false)
	if eng.wakeCancel != nil {
		t.Fatal("disabled flag started the watcher")
	}
}

func TestWakeWatchStartsAndStopsWithFlag(t *testing.T) {
	eng := &Engine{}
	eng.HostWithoutLock()
	t.Cleanup(func() { eng.SyncWakeWatch(false) })
	eng.SyncWakeWatch(true)
	if eng.wakeCancel == nil {
		t.Fatal("enabled flag did not start the watcher")
	}
	eng.SyncWakeWatch(true)
	if eng.wakeCancel == nil {
		t.Fatal("second enable dropped the watcher")
	}
	eng.SyncWakeWatch(false)
	if eng.wakeCancel != nil {
		t.Fatal("turning the flag off left the watcher running")
	}
}
