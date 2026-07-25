package controls

import (
	"testing"
	"time"

	"github.com/0xdreadnaught/fluo/anim"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/timers"
)

var animT0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func approxColor(a, b render.Color, eps float32) bool {
	d := func(x, y float32) float32 {
		v := x - y
		if v < 0 {
			v = -v
		}
		return v
	}
	return d(a.R, b.R) <= eps && d(a.G, b.G) <= eps && d(a.B, b.B) <= eps && d(a.A, b.A) <= eps
}

func TestColorAnimSetTargetNoQueueIsInstant(t *testing.T) {
	from := render.RGBA(255, 0, 0, 255)
	to := render.RGBA(0, 255, 0, 255)

	a := newColorAnim(from)
	a.SetTarget(nil, to)

	if a.Current() != to {
		t.Fatalf("Current() after nil-queue SetTarget = %v, want %v (instant jump)", a.Current(), to)
	}
}

func TestColorAnimCrossFadesOverTween(t *testing.T) {
	from := render.RGBA(255, 0, 0, 255)
	to := render.RGBA(0, 255, 0, 255)

	q := timers.NewQueue(animT0)
	a := newColorAnim(from)
	a.SetTarget(q, to)

	// Nothing has ticked yet: still showing the starting color.
	if a.Current() != from {
		t.Fatalf("Current() before any Advance = %v, want %v (unchanged)", a.Current(), from)
	}

	q.Advance(animT0.Add(colorAnimDuration / 2))
	want := lerpColor(from, to, anim.EaseOut(0.5))
	if got := a.Current(); !approxColor(got, want, 0.02) {
		t.Fatalf("Current() at half duration = %v, want ~%v", got, want)
	}

	q.Advance(animT0.Add(colorAnimDuration))
	if got := a.Current(); got != to {
		t.Fatalf("Current() at full duration = %v, want %v", got, to)
	}
}

func TestColorAnimSameTargetIsNoOp(t *testing.T) {
	from := render.RGBA(255, 0, 0, 255)
	to := render.RGBA(0, 255, 0, 255)

	q := timers.NewQueue(animT0)
	a := newColorAnim(from)
	a.SetTarget(q, to)
	q.Advance(animT0.Add(colorAnimDuration / 2))
	mid := a.Current()

	// Re-targeting to the SAME color (as Render would do every frame while
	// the state hasn't changed) must not restart the tween from mid-flight.
	a.SetTarget(q, to)
	if got := a.Current(); got != mid {
		t.Fatalf("Current() right after a same-target SetTarget = %v, want unchanged %v", got, mid)
	}

	q.Advance(animT0.Add(colorAnimDuration))
	if got := a.Current(); got != to {
		t.Fatalf("Current() after completing = %v, want %v", got, to)
	}
}

func TestColorAnimRedirectMidFlightStartsFromCurrent(t *testing.T) {
	rest := render.RGBA(200, 200, 200, 255)
	hover := render.RGBA(100, 100, 255, 255)

	q := timers.NewQueue(animT0)
	a := newColorAnim(rest)
	a.SetTarget(q, hover)
	q.Advance(animT0.Add(colorAnimDuration / 2))
	partway := a.Current()

	// Redirect back toward rest mid-flight: must continue from the CURRENT
	// interpolated color, not pop back to hover or to the original rest.
	a.SetTarget(q, rest)
	if got := a.Current(); got != partway {
		t.Fatalf("Current() immediately after redirect = %v, want unchanged %v", got, partway)
	}
	q.Advance(animT0.Add(colorAnimDuration/2 + colorAnimDuration))
	if got := a.Current(); got != rest {
		t.Fatalf("Current() after redirected tween completes = %v, want %v", got, rest)
	}
}

// TestButtonAnimatedFillDefaultOffIsPassthrough pins the CRITICAL default-off
// contract: a Button that never calls SetAnimated (every existing Button,
// every existing golden) must have animatedFill return its input unchanged,
// so Render's painted fill is byte-identical to before this feature existed.
func TestButtonAnimatedFillDefaultOffIsPassthrough(t *testing.T) {
	b := NewButton(nil, "X")
	rest := b.colors.ControlFill
	hover := b.colors.ControlFillHover

	if got := b.animatedFill(rest); got != rest {
		t.Fatalf("animatedFill(rest) on a non-animated button = %v, want %v unchanged", got, rest)
	}
	// Even a differing color must pass straight through: animated defaults
	// false regardless of what fill stateColors resolves to.
	if got := b.animatedFill(hover); got != hover {
		t.Fatalf("animatedFill(hover) on a non-animated button = %v, want %v unchanged", got, hover)
	}
}

// TestButtonAnimatedNoTimersIsPassthrough pins the documented "animated but
// no queue wired" fallback: SetAnimated(true) alone, with SetTimers never
// called, must still be instant/pass-through.
func TestButtonAnimatedNoTimersIsPassthrough(t *testing.T) {
	b := NewButton(nil, "X").SetAnimated(true)
	hover := b.colors.ControlFillHover

	if got := b.animatedFill(hover); got != hover {
		t.Fatalf("animatedFill(hover) with SetAnimated(true) but no SetTimers = %v, want %v unchanged", got, hover)
	}
}

// TestButtonAnimatedCrossFadesFillTransition drives a Button through a
// rest->hover state change with SetAnimated(true) and a wired timers.Queue,
// and checks the fill actually cross-fades (mid-tween value between rest
// and hover) rather than snapping.
func TestButtonAnimatedCrossFadesFillTransition(t *testing.T) {
	b := NewButton(nil, "X").SetAnimated(true)
	q := timers.NewQueue(animT0)
	b.SetTimers(q)

	rest := b.colors.ControlFill
	hover := b.colors.ControlFillHover

	// First animated frame at rest: no tween in flight yet, seeded exactly
	// at rest (no fade-in-from-nothing pop).
	if got := b.animatedFill(rest); got != rest {
		t.Fatalf("first animatedFill(rest) = %v, want %v (seeded, no animation)", got, rest)
	}

	// State changes to hover: still showing rest until the queue advances.
	if got := b.animatedFill(hover); got != rest {
		t.Fatalf("animatedFill(hover) immediately on state change = %v, want still %v (rest)", got, rest)
	}

	q.Advance(animT0.Add(colorAnimDuration / 2))
	want := lerpColor(rest, hover, anim.EaseOut(0.5))
	if got := b.animatedFill(hover); !approxColor(got, want, 0.02) {
		t.Fatalf("animatedFill(hover) at half duration = %v, want ~%v", got, want)
	}

	q.Advance(animT0.Add(colorAnimDuration))
	if got := b.animatedFill(hover); got != hover {
		t.Fatalf("animatedFill(hover) at full duration = %v, want %v", got, hover)
	}
}
