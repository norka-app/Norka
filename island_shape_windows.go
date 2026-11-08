//go:build windows

package main

import (
	"math"

	"golang.org/x/sys/windows"
)

var (
	gdi32                  = windows.NewLazySystemDLL("gdi32.dll")
	procCreateRoundRectRgn = gdi32.NewProc("CreateRoundRectRgn")
	procCreateRectRgn      = gdi32.NewProc("CreateRectRgn")
	procCombineRgn         = gdi32.NewProc("CombineRgn")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procSetWindowRgn       = user32.NewProc("SetWindowRgn")
	procGetDpiForWindow    = user32.NewProc("GetDpiForWindow")
)

const rgnOr = 2

func islandShapeSupported() bool { return true }

func applyCurrentIslandShapes(shapes []IslandShape, scale float64) {
	applyIslandShapes(mainWindow(), shapes, scale)
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

func roundedRegion(x, y, w, h, radius int) uintptr {
	if w < 1 || h < 1 {
		return 0
	}
	if radius <= 0 {
		handle, _, _ := procCreateRectRgn.Call(uintptr(x), uintptr(y), uintptr(x+w), uintptr(y+h))
		return handle
	}
	ellipse := radius * 2
	if ellipse > w {
		ellipse = w
	}
	if ellipse > h {
		ellipse = h
	}
	// +1: правый и нижний край CreateRoundRectRgn не включаются.
	handle, _, _ := procCreateRoundRectRgn.Call(
		uintptr(x), uintptr(y), uintptr(x+w+1), uintptr(y+h+1), uintptr(ellipse), uintptr(ellipse),
	)
	return handle
}

func deleteObject(handle uintptr) {
	if handle != 0 {
		procDeleteObject.Call(handle)
	}
}

// applyIslandShapes вырезает окно по сумме скруглённых прямоугольников.
// Координаты фигур — DIP; scale переводит их в физические пиксели окна.
// SetWindowRgn забирает регион себе, исходники после CombineRgn удаляем.
func applyIslandShapes(hwnd uintptr, shapes []IslandShape, scale float64) {
	if hwnd == 0 {
		return
	}
	if scale <= 0 {
		scale = 1
	}
	var combined uintptr
	for _, shape := range shapes {
		if shape.W < 1 || shape.H < 1 {
			continue
		}
		x := int(math.Round(float64(shape.X) * scale))
		y := int(math.Round(float64(shape.Y) * scale))
		w := int(math.Round(float64(shape.W) * scale))
		h := int(math.Round(float64(shape.H) * scale))
		r := int(math.Round(float64(shape.Radius) * scale))
		part := roundedRegion(x, y, w, h, r)
		if part == 0 {
			continue
		}
		if combined == 0 {
			combined = part
			continue
		}
		next, _, _ := procCreateRectRgn.Call(0, 0, 1, 1)
		if next == 0 {
			deleteObject(part)
			continue
		}
		procCombineRgn.Call(next, combined, part, rgnOr)
		deleteObject(combined)
		deleteObject(part)
		combined = next
	}
	if combined == 0 {
		procSetWindowRgn.Call(hwnd, 0, 1)
		return
	}
	procSetWindowRgn.Call(hwnd, combined, 1)
}

func clearIslandRegion() {
	hwnd := mainWindow()
	if hwnd == 0 {
		return
	}
	procSetWindowRgn.Call(hwnd, 0, 1)
}
