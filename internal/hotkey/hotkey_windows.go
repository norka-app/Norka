//go:build windows

package hotkey

import (
	"fmt"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procRegisterHotKey   = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey = user32.NewProc("UnregisterHotKey")
	procPeekMessageW     = user32.NewProc("PeekMessageW")
)

const (
	modAlt   = 0x0001
	modCtrl  = 0x0002
	modShift = 0x0004
	modWin   = 0x0008
	wmHotkey = 0x0312
	pmRemove = 0x0001
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
	registered := make(chan error, 1)
	stopped := make(chan struct{})
	go func() {
		r, _, callErr := procRegisterHotKey.Call(0, id, uintptr(mods), uintptr(vk))
		if r == 0 {
			if callErr == nil || callErr == syscall.Errno(0) {
				callErr = fmt.Errorf("RegisterHotKey failed")
			}
			registered <- callErr
			return
		}
		registered <- nil
		var message winMsg
		for {
			select {
			case <-stopped:
				_, _, _ = procUnregisterHotKey.Call(0, id)
				return
			default:
			}
			for {
				got, _, _ := procPeekMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0, pmRemove)
				if got == 0 {
					break
				}
				if message.Message == wmHotkey && message.WParam == id {
					callback()
				}
			}
			time.Sleep(30 * time.Millisecond)
		}
	}()
	if err := <-registered; err != nil {
		return nil, fmt.Errorf("%w: %s", ErrRegisterFailed, err.Error())
	}
	var once sync.Once
	return func() {
		once.Do(func() { close(stopped) })
	}, nil
}

func winChord(acc Accelerator) (uint32, uint32, error) {
	var mods uint32
	if acc.Alt {
		mods |= modAlt
	}
	if acc.Ctrl {
		mods |= modCtrl
	}
	if acc.Shift {
		mods |= modShift
	}
	if acc.Meta {
		mods |= modWin
	}
	vk, ok := winVK[acc.Key]
	if !ok {
		return 0, 0, fmt.Errorf("%w: %s", ErrInvalid, acc.Key)
	}
	return mods, vk, nil
}

var winVK = map[string]uint32{
	"space": 0x20, "escape": 0x1B, "tab": 0x09, "enter": 0x0D,
	"0": 0x30, "1": 0x31, "2": 0x32, "3": 0x33, "4": 0x34,
	"5": 0x35, "6": 0x36, "7": 0x37, "8": 0x38, "9": 0x39,
	"a": 0x41, "b": 0x42, "c": 0x43, "d": 0x44, "e": 0x45, "f": 0x46,
	"g": 0x47, "h": 0x48, "i": 0x49, "j": 0x4A, "k": 0x4B, "l": 0x4C,
	"m": 0x4D, "n": 0x4E, "o": 0x4F, "p": 0x50, "q": 0x51, "r": 0x52,
	"s": 0x53, "t": 0x54, "u": 0x55, "v": 0x56, "w": 0x57, "x": 0x58,
	"y": 0x59, "z": 0x5A,
	"f1": 0x70, "f2": 0x71, "f3": 0x72, "f4": 0x73, "f5": 0x74, "f6": 0x75,
	"f7": 0x76, "f8": 0x77, "f9": 0x78, "f10": 0x79, "f11": 0x7A, "f12": 0x7B,
}
