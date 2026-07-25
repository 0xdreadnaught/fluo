package anim

import (
	"testing"
	"time"

	"github.com/0xdreadnaught/fluo/timers"
)

var t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// approxEqual reports whether a and b are within eps of each other.
func approxEqual(a, b, eps float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= eps
}

func TestLinearEndpointsAndMidpoint(t *testing.T) {
	if Linear(0) != 0 {
		t.Fatalf("Linear(0) = %v, want 0", Linear(0))
	}
	if Linear(1) != 1 {
		t.Fatalf("Linear(1) = %v, want 1", Linear(1))
	}
	if Linear(0.5) != 0.5 {
		t.Fatalf("Linear(0.5) = %v, want 0.5", Linear(0.5))
	}
}

func TestEaseOutEndpoints(t *testing.T) {
	if EaseOut(0) != 0 {
		t.Fatalf("EaseOut(0) = %v, want 0", EaseOut(0))
	}
	if EaseOut(1) != 1 {
		t.Fatalf("EaseOut(1) = %v, want 1", EaseOut(1))
	}
}

func TestEaseOutHalfwayIsAheadOfLinear(t *testing.T) {
	// Cubic ease-out front-loads progress: at the midpoint it must already
	// be further along than a linear tween (the whole point of "ease out").
	if v := EaseOut(0.5); v <= 0.5 {
		t.Fatalf("EaseOut(0.5) = %v, want > 0.5", v)
	}
}

func TestEaseOutMonotonic(t *testing.T) {
	prev := float32(-1)
	for i := 0; i <= 20; i++ {
		v := EaseOut(float32(i) / 20)
		if v < prev {
			t.Fatalf("EaseOut not monotonic: at step %d value %v < previous %v", i, v, prev)
		}
		prev = v
	}
}

func TestEaseInOutEndpointsAndMonotonic(t *testing.T) {
	if EaseInOut(0) != 0 {
		t.Fatalf("EaseInOut(0) = %v, want 0", EaseInOut(0))
	}
	if EaseInOut(1) != 1 {
		t.Fatalf("EaseInOut(1) = %v, want 1", EaseInOut(1))
	}
	prev := float32(-1)
	for i := 0; i <= 20; i++ {
		v := EaseInOut(float32(i) / 20)
		if v < prev {
			t.Fatalf("EaseInOut not monotonic: at step %d value %v < previous %v", i, v, prev)
		}
		prev = v
	}
}

func TestTweenAdvanceHalfwayMatchesEase(t *testing.T) {
	q := timers.NewQueue(t0)
	var got float32
	updates := 0
	done := false
	NewTween(q, 100*time.Millisecond, EaseOut, func(v float32) {
		got = v
		updates++
	}, func() { done = true })

	q.Advance(t0.Add(50 * time.Millisecond))

	if updates == 0 {
		t.Fatal("onUpdate never called after advancing halfway")
	}
	if want := EaseOut(0.5); !approxEqual(got, want, 0.02) {
		t.Fatalf("onUpdate value at halfway = %v, want ~%v", got, want)
	}
	if done {
		t.Fatal("onDone fired at halfway, want not yet")
	}
}

func TestTweenCompletesExactlyOnceAtFullDuration(t *testing.T) {
	q := timers.NewQueue(t0)
	var last float32
	doneCount := 0
	NewTween(q, 100*time.Millisecond, Linear, func(v float32) {
		last = v
	}, func() { doneCount++ })

	q.Advance(t0.Add(100 * time.Millisecond))
	if last != 1 {
		t.Fatalf("onUpdate final value = %v, want 1", last)
	}
	if doneCount != 1 {
		t.Fatalf("onDone called %d times, want 1", doneCount)
	}

	// Further advances must not refire onUpdate/onDone: the tween has
	// stopped itself.
	q.Advance(t0.Add(500 * time.Millisecond))
	if doneCount != 1 {
		t.Fatalf("onDone called %d times after extra advance, want still 1", doneCount)
	}
}

func TestTweenOvershootClampsToOne(t *testing.T) {
	q := timers.NewQueue(t0)
	var last float32
	NewTween(q, 100*time.Millisecond, Linear, func(v float32) { last = v }, nil)

	// Jump straight past the full duration in one Advance.
	q.Advance(t0.Add(10 * time.Second))
	if last != 1 {
		t.Fatalf("onUpdate final value after overshoot = %v, want 1 (clamped)", last)
	}
}

func TestTweenStopIsIdempotentAndSuppressesOnDone(t *testing.T) {
	q := timers.NewQueue(t0)
	updates := 0
	doneCount := 0
	tw := NewTween(q, 100*time.Millisecond, Linear, func(v float32) { updates++ }, func() { doneCount++ })

	q.Advance(t0.Add(30 * time.Millisecond))
	if updates == 0 {
		t.Fatal("expected at least one onUpdate before Stop")
	}
	seenBeforeStop := updates

	tw.Stop()
	tw.Stop() // idempotent: must not panic or double-act

	q.Advance(t0.Add(1000 * time.Millisecond))
	if updates != seenBeforeStop {
		t.Fatalf("onUpdate fired after Stop: %d calls before, %d after", seenBeforeStop, updates)
	}
	if doneCount != 0 {
		t.Fatalf("onDone fired after Stop, want never (Stop halts with no onDone)")
	}
	if q.Len() != 0 {
		t.Fatalf("queue still holds the stopped tween's timer: len=%d", q.Len())
	}
}

func TestTweenZeroDurationCompletesImmediately(t *testing.T) {
	q := timers.NewQueue(t0)
	var got float32
	doneCount := 0
	NewTween(q, 0, Linear, func(v float32) { got = v }, func() { doneCount++ })

	if got != 1 {
		t.Fatalf("onUpdate value for zero-duration tween = %v, want 1 (immediate)", got)
	}
	if doneCount != 1 {
		t.Fatalf("onDone called %d times for zero-duration tween, want 1", doneCount)
	}
	if q.Len() != 0 {
		t.Fatalf("zero-duration tween left a timer in the queue: len=%d", q.Len())
	}
}
