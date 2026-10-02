//go:build !windows

package main

// На macOS/Linux нативная геометрия не используется: размер восстанавливается
// через Wails runtime, окно центрируется.
func nativeWindowBounds() (windowRect, bool) { return windowRect{}, false }

func nativeSetWindowBounds(windowRect) bool { return false }

func nativeWorkArea(windowRect) (windowRect, bool) { return windowRect{}, false }
