//go:build windows

package hotkey

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"
)

var (
	user32                 = syscall.NewLazyDLL("user32.dll")
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procRegisterHotKey     = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey   = user32.NewProc("UnregisterHotKey")
	procGetMessageW        = user32.NewProc("GetMessageW")
	procPostThreadMessageW = user32.NewProc("PostThreadMessageW")
	procGetCurrentThreadId = kernel32.NewProc("GetCurrentThreadId")
)

const (
	wmHotkey = 0x0312
	wmQuit   = 0x0012
)

type winMsg struct {
	HWND    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	PtX     int32
	PtY     int32
}

var hotkeySeq atomic.Int32

func listen(acc Accelerator, callback func()) (func(), error) {
	mods, vk, err := winChord(acc)
	if err != nil {
		return nil, err
	}
	id := uintptr(hotkeySeq.Add(1))
	stop, err := threadLoop{
		// Lock for the goroutine's whole life. Exiting without UnlockOSThread
		// terminates the OS thread, which is what we want: the queue belonged
		// to this registration only.
		lockOSThread: runtime.LockOSThread,
		register: func() error {
			r, _, callErr := procRegisterHotKey.Call(0, id, uintptr(mods), uintptr(vk))
			if r == 0 {
				if callErr == nil || callErr == syscall.Errno(0) {
					callErr = fmt.Errorf("RegisterHotKey failed")
				}
				return callErr
			}
			return nil
		},
		threadID: func() uint32 {
			r, _, _ := procGetCurrentThreadId.Call()
			return uint32(r)
		},
		next: func() (uint32, uintptr, bool) {
			var message winMsg
			// hWnd=0 reads this thread's window messages and its thread queue,
			// which is where RegisterHotKey(0, …) posts WM_HOTKEY.
			ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
			if int32(ret) <= 0 {
				return 0, 0, true
			}
			return message.Message, message.WParam, false
		},
		unregister: func() {
			_, _, _ = procUnregisterHotKey.Call(0, id)
		},
		postQuit: func(threadID uint32) error {
			r, _, callErr := procPostThreadMessageW.Call(uintptr(threadID), wmQuit, 0, 0)
			if r == 0 {
				if callErr == nil || callErr == syscall.Errno(0) {
					return fmt.Errorf("PostThreadMessageW failed")
				}
				return callErr
			}
			return nil
		},
		onHotkey:  callback,
		hotkeyMsg: wmHotkey,
		hotkeyID:  id,
	}.run()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrRegisterFailed, err.Error())
	}
	return stop, nil
}
