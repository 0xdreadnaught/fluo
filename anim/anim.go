package anim

import (
	"time"

	"github.com/0xdreadnaught/fluo/timers"
)

// Easing maps a normalized progress t in [0,1] to an eased progress, also in
// [0,1]. Every Easing in this package satisfies f(0)==0 and f(1)==1.
type Easing func(t float32) float32

// Linear returns t unchanged — no easing.
func Linear(t float32) float32 {
	return t
}

// EaseOut is a cubic ease-out curve (1-(1-t)^3): progress starts fast and
// decelerates into the endpoint, front-loading motion so it reads as
// "arriving" rather than "departing". Monotonically increasing over [0,1].
func EaseOut(t float32) float32 {
	inv := 1 - t
	return 1 - inv*inv*inv
}

// EaseInOut is a cubic ease-in-out curve: accelerates out of 0, decelerates
// into 1, symmetric about the midpoint. Monotonically increasing over
// [0,1].
func EaseInOut(t float32) float32 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	f := -2*t + 2
	return 1 - f*f*f/2
}

// tickInterval is the internal cadence at which a Tween re-evaluates its
// progress. It is deliberately fine-grained (well under any realistic UI
// animation duration) so a host that advances its timers.Queue once per
// rendered frame effectively gets one onUpdate per frame; it is an
// implementation detail, not part of Tween's contract.
const tickInterval = 10 * time.Millisecond

// Tween interpolates a normalized progress 0→1 over duration d, driven by
// q.Advance: each internal tick computes elapsed/d (clamped to [0,1]),
// calls onUpdate(ease(t)), and — once t reaches 1 — calls onDone exactly
// once and stops itself (no further onUpdate/onDone calls, even if q keeps
// advancing). onUpdate is called at least once even for a zero (or
// negative) duration, synchronously inside NewTween, with onDone following
// immediately — a degenerate but valid "already done" tween. Not
// goroutine-safe (matches timers.Queue itself).
//
// If the host stalls and then advances q by more than one tickInterval in
// a single Advance call, the underlying timers.Queue catches up by firing
// every due tick within that call (see timers.Queue.Advance), so onUpdate
// (and, at most once, onDone) may be called several times within that one
// Advance — bounded by roughly d/tickInterval, never unbounded. Keep
// onUpdate cheap for this reason.
type Tween struct {
	d        time.Duration
	ease     Easing
	onUpdate func(float32)
	onDone   func()

	timer   *timers.Timer
	elapsed time.Duration
	done    bool
}

// NewTween creates and starts a Tween on q. See the Tween doc comment for
// the update/completion contract.
func NewTween(q *timers.Queue, d time.Duration, ease Easing, onUpdate func(float32), onDone func()) *Tween {
	tw := &Tween{d: d, ease: ease, onUpdate: onUpdate, onDone: onDone}

	if d <= 0 {
		tw.emit(1)
		tw.finish()
		return tw
	}

	period := tickInterval
	if period > d {
		period = d
	}
	tw.timer = q.Every(period, func() {
		tw.elapsed += period
		t := float32(tw.elapsed) / float32(tw.d)
		if t >= 1 {
			t = 1
		}
		tw.emit(t)
		if t >= 1 {
			tw.finish()
		}
	})
	return tw
}

// emit calls onUpdate(ease(t)) if onUpdate is non-nil.
func (tw *Tween) emit(t float32) {
	if tw.onUpdate != nil {
		tw.onUpdate(tw.ease(t))
	}
}

// finish marks the tween done, stops its timer, and calls onDone. Guarded
// by tw.done so it (like Stop) only ever takes effect once.
func (tw *Tween) finish() {
	if tw.done {
		return
	}
	tw.done = true
	if tw.timer != nil {
		tw.timer.Stop()
	}
	if tw.onDone != nil {
		tw.onDone()
	}
}

// Stop halts the tween immediately: no further onUpdate calls, and onDone
// is NOT called (Stop is a cancellation, not a completion). Idempotent —
// calling Stop again (or after the tween has already completed on its own)
// is a no-op.
func (tw *Tween) Stop() {
	if tw.done {
		return
	}
	tw.done = true
	if tw.timer != nil {
		tw.timer.Stop()
	}
}
