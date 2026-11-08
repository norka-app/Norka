//go:build windows

package main

import "math"

var procGetDpiForWindow = user32.NewProc("GetDpiForWindow")

// placeCapsuleNative ставит капсулу в рабочей области текущего монитора.
// Вызывающий уже держит window.mu и выставил размер в DIP.
func (a *App) placeCapsuleNative(width, height, radius int) bool {
	hwnd := mainWindow()
	if hwnd == 0 || a == nil || a.ctx == nil {
		return false
	}
	scale := windowDPIScale(hwnd)
	if scale <= 0 {
		scale = 1
	}
	a.islandScale = scale
	current, ok := nativeWindowBounds()
	if !ok {
		current = windowRect{W: 1, H: 1}
	}
	work, onScreen := nativeWorkArea(current)
	if !onScreen {
		return false
	}
	physW := int(math.Round(float64(width) * scale))
	physH := int(math.Round(float64(height) * scale))
	if physW < 1 {
		physW = 1
	}
	if physH < 1 {
		physH = 1
	}
	origin := capsuleOrigin(dipRect{
		X: physicalToDIP(work.X, scale),
		Y: physicalToDIP(work.Y, scale),
		W: physicalToDIP(work.W, scale),
		H: physicalToDIP(work.H, scale),
	}, width, height)
	target := windowRect{
		X: int(math.Round(float64(origin.X) * scale)),
		Y: int(math.Round(float64(origin.Y) * scale)),
		W: physW,
		H: physH,
	}
	if !nativeSetWindowBounds(target) {
		return false
	}
	// Маску не ставим: скругление рисует CSS поверх прозрачного WebView.
	procSetWindowRgn.Call(hwnd, 0, 1)
	setSquareWindowCorners(hwnd)
	_ = radius
	return true
}

func windowDPIScale(hwnd uintptr) float64 {
	if hwnd == 0 {
		return 1
	}
	dpi, _, _ := procGetDpiForWindow.Call(hwnd)
	if dpi == 0 {
		return 1
	}
	return float64(dpi) / 96
}
