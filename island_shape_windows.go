//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// Сглаживание капсулы делает прозрачный WebView и CSS, а не SetWindowRgn:
// однобитная область окна даёт лесенку. Здесь только снимаем старую маску,
// запрещаем DWM скруглять внешние углы и включаем изменение размера полного окна.
var (
	procGetWindowLongPtr = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtr = user32.NewProc("SetWindowLongPtrW")
	procSetWindowRgn     = user32.NewProc("SetWindowRgn")
	dwmapi               = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmSetAttribute  = dwmapi.NewProc("DwmSetWindowAttribute")
)

const (
	wsThickFrame   = uintptr(0x00040000)
	wsMaximizeBox  = uintptr(0x00010000)
	swpNoMove      = 0x0002
	swpNoSize      = 0x0001
	swpFrameChange = 0x0020

	// DWMWA_WINDOW_CORNER_PREFERENCE = 33, DWMWCP_DONOTROUND = 1.
	dwmwaWindowCornerPreference = 33
	dwmwcpDoNotRound            = 1
)

func clearIslandRegion() {
	hwnd := mainWindow()
	if hwnd == 0 {
		return
	}
	procSetWindowRgn.Call(hwnd, 0, 1)
	setSquareWindowCorners(hwnd)
}

func setAdvancedChrome(enabled bool) {
	hwnd := mainWindow()
	if hwnd == 0 {
		return
	}
	// GWLP_STYLE = -16.
	style, _, _ := procGetWindowLongPtr.Call(hwnd, ^uintptr(15))
	if enabled {
		style |= wsThickFrame | wsMaximizeBox
	} else {
		style &^= wsThickFrame | wsMaximizeBox
	}
	procSetWindowLongPtr.Call(hwnd, ^uintptr(15), style)
	procSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoZOrder|swpNoActivate|swpFrameChange)
	setSquareWindowCorners(hwnd)
}

func setSquareWindowCorners(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	pref := int32(dwmwcpDoNotRound)
	procDwmSetAttribute.Call(hwnd, dwmwaWindowCornerPreference, uintptr(unsafe.Pointer(&pref)), unsafe.Sizeof(pref))
}
