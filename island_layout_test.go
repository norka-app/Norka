package main

import "testing"

func TestCapsuleOriginCentersBelowTopGap(t *testing.T) {
	work := dipRect{X: 10, Y: 40, W: 1920, H: 1040}
	got := capsuleOrigin(work, 360, 36)
	if got.X != 10+(1920-360)/2 || got.Y != 40+islandTopGapDIP || got.W != 360 || got.H != 36 {
		t.Fatalf("capsule = %+v", got)
	}
	odd := capsuleOrigin(dipRect{X: 0, Y: 0, W: 100, H: 80}, 31, 36)
	if odd.X != (100-31)/2 || odd.Y != islandTopGapDIP {
		t.Fatalf("odd capsule = %+v", odd)
	}
}

func TestLayoutIslandStageAutoPlacesPanelUnderCapsule(t *testing.T) {
	got := layoutIslandStage(1600, 900, 360, 36, 18, -1, -1, 0, 0)
	if got.IslandX != (1600-360)/2 || got.IslandY != islandTopGapDIP || got.IslandR != 18 {
		t.Fatalf("island = %+v", got)
	}
	if got.PanelW != 1024 || got.PanelH != 768 {
		t.Fatalf("default panel = %+v", got)
	}
	if got.PanelX != (1600-1024)/2 {
		t.Fatalf("panel x = %d", got.PanelX)
	}
	if got.PanelY != got.IslandY+got.IslandH+islandPanelGap {
		t.Fatalf("panel y = %d", got.PanelY)
	}
}

func TestLayoutIslandStageClampsPanelInsideHost(t *testing.T) {
	got := layoutIslandStage(800, 600, 280, 36, 18, 900, 900, 1024, 768)
	if got.PanelW != 800 || got.PanelH != 600 {
		t.Fatalf("clamped size = %+v", got)
	}
	if got.PanelX != 0 || got.PanelY != 0 {
		t.Fatalf("clamped origin = %+v", got)
	}
	if got.IslandW != 280 || got.IslandX != (800-280)/2 {
		t.Fatalf("island = %+v", got)
	}
}

func TestLayoutIslandStageKeepsExplicitPanelBelowCapsule(t *testing.T) {
	got := layoutIslandStage(1600, 1000, 96, 36, 18, 40, 0, 400, 300)
	floor := islandTopGapDIP + 36 + islandPanelMinY
	if got.PanelY < floor {
		t.Fatalf("panel overlaps capsule: %+v", got)
	}
	if got.PanelX != 40 || got.PanelW != 400 || got.PanelH != 300 {
		t.Fatalf("panel = %+v", got)
	}
}
