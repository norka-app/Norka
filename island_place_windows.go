//go:build windows

package main

import (
	"math"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

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
	applyIslandShapes(hwnd, []IslandShape{{
		X: 0, Y: 0, W: width, H: height, Radius: radius,
	}}, scale)
	return true
}

// placeWorkAreaHostNative растягивает окно на рабочую область и возвращает её размер в DIP.
func (a *App) placeWorkAreaHostNative() (int, int, bool) {
	hwnd := mainWindow()
	if hwnd == 0 || a == nil || a.ctx == nil {
		return 0, 0, false
	}
	scale := windowDPIScale(hwnd)
	if scale <= 0 {
		scale = 1
	}
	current, ok := nativeWindowBounds()
	if !ok {
		return 0, 0, false
	}
	work, onScreen := nativeWorkArea(current)
	if !onScreen {
		return 0, 0, false
	}
	dipW := physicalToDIP(work.W, scale)
	dipH := physicalToDIP(work.H, scale)
	if dipW < 1 {
		dipW = 1
	}
	if dipH < 1 {
		dipH = 1
	}
	wailsruntime.WindowSetSize(a.ctx, dipW, dipH)
	if !nativeSetWindowBounds(work) {
		return 0, 0, false
	}
	a.islandScale = scale
	return dipW, dipH, true
}
