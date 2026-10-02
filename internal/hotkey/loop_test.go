package hotkey

import (
	"errors"
	"sync"
	"testing"
)

const testHotkeyMsg = uint32(0x0312)

type loopEnv struct {
	mu         sync.Mutex
	taken      bool
	thread     uint32
	order      []string
	quitPosted uint32
	msgs       chan loopMsg
	fired      int
}

type loopMsg struct {
	msg    uint32
	wparam uintptr
	quit   bool
}

func (e *loopEnv) note(step string) {
	e.mu.Lock()
	e.order = append(e.order, step)
	e.mu.Unlock()
}

func (e *loopEnv) loop(id uintptr) threadLoop {
	return threadLoop{
		lockOSThread: func() { e.note("lock") },
		register: func() error {
			e.note("register")
			e.mu.Lock()
			defer e.mu.Unlock()
			if e.taken {
				return errors.New("already registered")
			}
			e.taken = true
			return nil
		},
		threadID: func() uint32 {
			e.note("thread")
			return e.thread
		},
		next: func() (uint32, uintptr, bool) {
			m := <-e.msgs
			return m.msg, m.wparam, m.quit
		},
		unregister: func() {
			e.note("unregister")
			e.mu.Lock()
			e.taken = false
			e.mu.Unlock()
		},
		postQuit: func(threadID uint32) error {
			e.mu.Lock()
			e.quitPosted = threadID
			e.mu.Unlock()
			e.msgs <- loopMsg{quit: true}
			return nil
		},
		onHotkey: func() {
			e.mu.Lock()
			e.fired++
			e.mu.Unlock()
		},
		hotkeyMsg: testHotkeyMsg,
		hotkeyID:  id,
	}
}

func TestThreadLoopHotkeyAndReregister(t *testing.T) {
	env := &loopEnv{thread: 42, msgs: make(chan loopMsg)}
	stop, err := env.loop(7).run()
	if err != nil {
		t.Fatal(err)
	}
	env.msgs <- loopMsg{msg: testHotkeyMsg, wparam: 7}
	env.msgs <- loopMsg{msg: testHotkeyMsg, wparam: 9}
	env.msgs <- loopMsg{msg: 0x0010, wparam: 7}
	stop()
	stop()

	env.mu.Lock()
	fired := env.fired
	quitPosted := env.quitPosted
	taken := env.taken
	order := append([]string(nil), env.order...)
	env.mu.Unlock()

	if fired != 1 {
		t.Fatalf("callback fired %d times", fired)
	}
	if quitPosted != 42 {
		t.Fatalf("quit posted to thread %d", quitPosted)
	}
	if taken {
		t.Fatal("chord still registered after stop")
	}
	if len(order) < 4 || order[0] != "lock" || order[1] != "register" || order[2] != "thread" || order[len(order)-1] != "unregister" {
		t.Fatalf("order: %v", order)
	}

	stop2, err := env.loop(8).run()
	if err != nil {
		t.Fatalf("re-register: %v", err)
	}
	stop2()
}

func TestThreadLoopRejectsDoubleRegister(t *testing.T) {
	env := &loopEnv{thread: 3, msgs: make(chan loopMsg)}
	stop, err := env.loop(1).run()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.loop(2).run(); err == nil || err.Error() != "already registered" {
		t.Fatalf("second register: %v", err)
	}
	stop()
	stop3, err := env.loop(3).run()
	if err != nil {
		t.Fatalf("register after stop: %v", err)
	}
	stop3()
}

func TestThreadLoopRegisterFailure(t *testing.T) {
	unregistered := false
	_, err := threadLoop{
		lockOSThread: func() {},
		register:     func() error { return errors.New("busy") },
		threadID:     func() uint32 { return 1 },
		next:         func() (uint32, uintptr, bool) { return 0, 0, true },
		unregister:   func() { unregistered = true },
		postQuit:     func(uint32) error { return nil },
	}.run()
	if err == nil || err.Error() != "busy" {
		t.Fatalf("got %v", err)
	}
	if unregistered {
		t.Fatal("unregister after a failed register")
	}
}
