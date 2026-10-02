package netwatch

import (
	"context"
	"time"
)

const (
	// JumpInterval is how often the fallback compares the two clocks.
	JumpInterval = 5 * time.Second
	// JumpGap is the wall-minus-monotonic gap treated as a sleep.
	// A 5s sample that actually covers more than 30s of wall time wins.
	JumpGap = 30 * time.Second
)

// JumpDetected reports a sleep-sized gap between wall time and a monotonic reading.
func JumpDetected(prevWall time.Time, prevMono time.Duration, wall time.Time, mono time.Duration, gap time.Duration) bool {
	if gap <= 0 {
		gap = JumpGap
	}
	wallDelta := wall.Sub(prevWall)
	monoDelta := mono - prevMono
	if monoDelta < 0 {
		monoDelta = 0
	}
	return wallDelta-monoDelta > gap
}

// RunJumpDetector samples clock on each tick. The first observation is the
// baseline. A later sample that jumps emits KindResume.
// ticks is the injectable event source; a nil clock uses the system clocks.
func RunJumpDetector(ctx context.Context, clock DualClock, ticks <-chan struct{}, gap time.Duration, out chan<- Event) {
	if clock == nil {
		clock = systemDual{base: time.Now()}
	}
	if gap <= 0 {
		gap = JumpGap
	}
	prevWall := clock.Wall()
	prevMono := clock.Mono()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ticks:
			if !ok {
				return
			}
			wall := clock.Wall()
			mono := clock.Mono()
			if JumpDetected(prevWall, prevMono, wall, mono, gap) {
				emit(ctx, out, Event{Kind: KindResume})
			}
			prevWall = wall
			prevMono = mono
		}
	}
}

func runSystemJump(ctx context.Context, out chan<- Event) {
	clock := systemDual{base: time.Now()}
	ticker := time.NewTicker(JumpInterval)
	defer ticker.Stop()
	ticks := make(chan struct{})
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				select {
				case ticks <- struct{}{}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	RunJumpDetector(ctx, clock, ticks, JumpGap, out)
}
