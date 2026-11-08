//go:build !windows

package main

func (a *App) placeCapsuleNative(_, _, _ int) bool { return false }
