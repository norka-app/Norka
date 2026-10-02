//go:build windows

package main

import (
	"os"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Класс главного окна Wails v2 на Windows (options.Windows.WindowClassName по умолчанию).
const wailsWindowClass = "wailsWindow"

const (
	monitorDefaultToNull = 0
	swpNoZOrder          = 0x0004
	swpNoActivate        = 0x0010
)

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")
	procGetClassNameW            = user32.NewProc("GetClassNameW")
	procGetWindowRect            = user32.NewProc("GetWindowRect")
	procIsWindow                 = user32.NewProc("IsWindow")
	procIsIconic                 = user32.NewProc("IsIconic")
	procSetWindowPos             = user32.NewProc("SetWindowPos")
	procMonitorFromRect          = user32.NewProc("MonitorFromRect")
	procGetMonitorInfoW          = user32.NewProc("GetMonitorInfoW")

	mainWindowMu   sync.Mutex
	mainWindowHWND uintptr
	foundHWND      uintptr
	enumWindowsCB  = syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		var pid uint32
		procGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid != uint32(os.Getpid()) {
			return 1
		}
		var buf [64]uint16
		n, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if windows.UTF16ToString(buf[:n]) == wailsWindowClass {
			foundHWND = hwnd
			return 0
		}
		return 1
	})
)

type rect32 struct {
	Left, Top, Right, Bottom int32
}

type monitorInfo struct {
	CbSize    uint32
	RcMonitor rect32
	RcWork    rect32
	DwFlags   uint32
}

func toWindowRect(r rect32) windowRect {
	return windowRect{X: int(r.Left), Y: int(r.Top), W: int(r.Right - r.Left), H: int(r.Bottom - r.Top)}
}

func mainWindow() uintptr {
	mainWindowMu.Lock()
	defer mainWindowMu.Unlock()
	if mainWindowHWND != 0 {
		if ok, _, _ := procIsWindow.Call(mainWindowHWND); ok == 0 {
			mainWindowHWND = 0
		}
	}
	if mainWindowHWND == 0 {
		foundHWND = 0
		procEnumWindows.Call(enumWindowsCB, 0)
		mainWindowHWND = foundHWND
	}
	return mainWindowHWND
}

// nativeWindowBounds — внешний прямоугольник окна в физических пикселях (абсолютные координаты).
func nativeWindowBounds() (windowRect, bool) {
	hwnd := mainWindow()
	if hwnd == 0 {
		return windowRect{}, false
	}
	if iconic, _, _ := procIsIconic.Call(hwnd); iconic != 0 {
		return windowRect{}, false // у свёрнутого окна координаты -32000
	}
	var r rect32
	if ok, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r))); ok == 0 {
		return windowRect{}, false
	}
	return toWindowRect(r), true
}

func nativeSetWindowBounds(r windowRect) bool {
	hwnd := mainWindow()
	if hwnd == 0 {
		return false
	}
	ok, _, _ := procSetWindowPos.Call(hwnd, 0, uintptr(r.X), uintptr(r.Y), uintptr(r.W), uintptr(r.H), swpNoZOrder|swpNoActivate)
	return ok != 0
}

// nativeWorkArea — рабочая область (без панели задач) монитора, на который попадает r.
// onScreen=false, если r не пересекается ни с одним монитором.
func nativeWorkArea(r windowRect) (windowRect, bool) {
	rc := rect32{Left: int32(r.X), Top: int32(r.Y), Right: int32(r.X + r.W), Bottom: int32(r.Y + r.H)}
	monitor, _, _ := procMonitorFromRect.Call(uintptr(unsafe.Pointer(&rc)), monitorDefaultToNull)
	if monitor == 0 {
		return windowRect{}, false
	}
	info := monitorInfo{CbSize: uint32(unsafe.Sizeof(monitorInfo{}))}
	if ok, _, _ := procGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info))); ok == 0 {
		return windowRect{}, false
	}
	return toWindowRect(info.RcWork), true
}
