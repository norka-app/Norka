package hotkey

import (
	"fmt"
	"sync"
)

// threadLoop is the Windows hotkey thread, with the OS calls injected so the
// sequence can be tested without user32. The goroutine locks its OS thread
// before register and never unlocks: RegisterHotKey(hWnd=0) posts WM_HOTKEY
// to that thread's queue, and UnregisterHotKey only works from the same thread.
// stop posts quit to the id captured inside the goroutine and waits until
// unregister has returned, so the same chord can be registered again.
type threadLoop struct {
	lockOSThread func()
	register     func() error
	threadID     func() uint32
	// next blocks until a message arrives. quit ends the loop (WM_QUIT or error).
	next       func() (msg uint32, wparam uintptr, quit bool)
	unregister func()
	postQuit   func(threadID uint32) error
	// onHotkey runs on the loop goroutine and must not call stop.
	onHotkey  func()
	hotkeyMsg uint32
	hotkeyID  uintptr
}

func (l threadLoop) run() (func(), error) {
	if l.lockOSThread == nil || l.register == nil || l.threadID == nil || l.next == nil || l.unregister == nil || l.postQuit == nil {
		return nil, fmt.Errorf("%w: incomplete hotkey loop", ErrInvalid)
	}
	ready := make(chan error, 1)
	done := make(chan struct{})
	var tid uint32
	go func() {
		l.lockOSThread()
		if err := l.register(); err != nil {
			ready <- err
			return
		}
		tid = l.threadID()
		ready <- nil
		for {
			msg, wparam, quit := l.next()
			if quit {
				break
			}
			if msg == l.hotkeyMsg && wparam == l.hotkeyID && l.onHotkey != nil {
				l.onHotkey()
			}
		}
		l.unregister()
		close(done)
	}()
	if err := <-ready; err != nil {
		return nil, err
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			_ = l.postQuit(tid)
			<-done
		})
	}, nil
}
