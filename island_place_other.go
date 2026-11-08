//go:build !windows

package main

func (a *App) placeCapsuleNative(_, _, _ int) bool { return false }

func (a *App) placeWorkAreaHostNative() (int, int, bool) { return 0, 0, false }
