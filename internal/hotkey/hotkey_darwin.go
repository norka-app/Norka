//go:build darwin && cgo

package hotkey

/*
#cgo darwin LDFLAGS: -framework Carbon -framework Cocoa
#cgo darwin CFLAGS: -Wno-deprecated-declarations

#include <Carbon/Carbon.h>

extern void goHotkeyCallback(int id);

static OSStatus norkaHotkeyHandler(EventHandlerCallRef next, EventRef event, void *userData) {
	(void)next;
	(void)userData;
	EventHotKeyID hid;
	GetEventParameter(event, kEventParamDirectObject, typeEventHotKeyID, NULL, sizeof(hid), NULL, &hid);
	goHotkeyCallback((int)hid.id);
	return noErr;
}

static void norkaInstallHotkeyHandler(void) {
	EventTypeSpec spec;
	spec.eventClass = kEventClassKeyboard;
	spec.eventKind = kEventHotKeyPressed;
	InstallApplicationEventHandler(NewEventHandlerUPP(norkaHotkeyHandler), 1, &spec, NULL, NULL);
}

static void *norkaRegisterHotkey(int id, int keyCode, int mods, int *statusOut) {
	EventHotKeyID hid;
	hid.signature = 'nork';
	hid.id = (unsigned int)id;
	EventHotKeyRef ref = NULL;
	OSStatus status = RegisterEventHotKey((unsigned int)keyCode, (unsigned int)mods, hid, GetApplicationEventTarget(), 0, &ref);
	if (statusOut != NULL) {
		*statusOut = (int)status;
	}
	if (status != noErr) {
		return NULL;
	}
	return (void *)ref;
}

static void norkaUnregisterHotkey(void *ref) {
	if (ref != NULL) {
		UnregisterEventHotKey((EventHotKeyRef)ref);
	}
}
*/
import "C"

import (
	"fmt"
	"sync"
	"sync/atomic"
	"unsafe"
)

const (
	darwinCmd     = 1 << 8
	darwinShift   = 1 << 9
	darwinOption  = 1 << 11
	darwinControl = 1 << 12
)

var (
	darwinMu        sync.Mutex
	darwinNext      atomic.Int32
	darwinHandlers  = map[int]func(){}
	darwinRefs      = map[int]unsafe.Pointer{}
	darwinInstalled bool
)

func listen(acc Accelerator, callback func()) (func(), error) {
	keyCode, ok := darwinKeyCode[acc.Key]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrInvalid, acc.Key)
	}
	mods := 0
	if acc.Meta {
		mods |= darwinCmd
	}
	if acc.Shift {
		mods |= darwinShift
	}
	if acc.Alt {
		mods |= darwinOption
	}
	if acc.Ctrl {
		mods |= darwinControl
	}
	id := int(darwinNext.Add(1))

	darwinMu.Lock()
	defer darwinMu.Unlock()
	if !darwinInstalled {
		C.norkaInstallHotkeyHandler()
		darwinInstalled = true
	}
	var status C.int
	ref := C.norkaRegisterHotkey(C.int(id), C.int(keyCode), C.int(mods), &status)
	if status != 0 || ref == nil {
		return nil, fmt.Errorf("%w: carbon status %d", ErrRegisterFailed, int(status))
	}
	darwinHandlers[id] = callback
	darwinRefs[id] = ref

	var once sync.Once
	return func() {
		once.Do(func() {
			darwinMu.Lock()
			defer darwinMu.Unlock()
			if stored, ok := darwinRefs[id]; ok {
				C.norkaUnregisterHotkey(stored)
				delete(darwinRefs, id)
			}
			delete(darwinHandlers, id)
		})
	}, nil
}

func dispatchDarwinHotkey(id int) {
	darwinMu.Lock()
	callback := darwinHandlers[id]
	darwinMu.Unlock()
	if callback != nil {
		callback()
	}
}

// Carbon virtual key codes (Events.h / HIToolbox).
var darwinKeyCode = map[string]int{
	"a": 0, "s": 1, "d": 2, "f": 3, "h": 4, "g": 5, "z": 6, "x": 7, "c": 8, "v": 9,
	"b": 11, "q": 12, "w": 13, "e": 14, "r": 15, "y": 16, "t": 17,
	"1": 18, "2": 19, "3": 20, "4": 21, "6": 22, "5": 23, "9": 25, "7": 26, "8": 28, "0": 29,
	"o": 31, "u": 32, "i": 34, "p": 35, "l": 37, "j": 38, "k": 40, "n": 45, "m": 46,
	"space": 49, "enter": 36, "tab": 48, "escape": 53,
	"f1": 122, "f2": 120, "f3": 99, "f4": 118, "f5": 96, "f6": 97,
	"f7": 98, "f8": 100, "f9": 101, "f10": 109, "f11": 103, "f12": 111,
}
