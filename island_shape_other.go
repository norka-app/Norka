//go:build !windows

package main

func islandShapeSupported() bool { return false }

func applyIslandShapes(_ uintptr, _ []IslandShape, _ float64) {}

func applyCurrentIslandShapes([]IslandShape, float64) {}

func clearIslandRegion() {}
