package netwatch

import "time"

// Clock is the time source the debouncer waits on. Tests supply a fake.
type Clock interface {
	Now() time.Time
	NewTimer(d time.Duration) Timer
}

// Timer is the piece of a clock the debouncer selects on.
type Timer interface {
	C() <-chan time.Time
	Stop() bool
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func (systemClock) NewTimer(d time.Duration) Timer {
	return goTimer{timer: time.NewTimer(d)}
}

type goTimer struct {
	timer *time.Timer
}

func (t goTimer) C() <-chan time.Time { return t.timer.C }

func (t goTimer) Stop() bool { return t.timer.Stop() }

// DualClock separates wall time from a monotonic reading.
// Wall time jumps across sleep; the monotonic reading does not have to.
type DualClock interface {
	Wall() time.Time
	Mono() time.Duration
}

type systemDual struct {
	base time.Time
}

func (c systemDual) Wall() time.Time { return time.Now().Round(0) }

func (c systemDual) Mono() time.Duration { return time.Since(c.base) }
