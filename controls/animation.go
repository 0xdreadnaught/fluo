package controls

import (
	"time"

	"github.com/0xdreadnaught/fluo/anim"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/timers"
)

// colorAnimDuration is the normative cross-fade duration for animated
// control fill transitions (hover/press and back), per the Phase 8 Task 2
// spec: ~120ms, EaseOut.
const colorAnimDuration = 120 * time.Millisecond

// colorAnim smoothly interpolates a render.Color toward successive target
// colors via an anim.Tween (EaseOut, colorAnimDuration) driven by a
// timers.Queue, lerping each of R/G/B/A independently. It is the shared
// building block controls embed to cross-fade state-driven fills (e.g.
// Button's hover/press chrome) instead of snapping — see Button.animatedFill
// for the wiring.
//
// colorAnim has no "unanimated" mode of its own: a control that wants the
// v0 instant/snap behavior simply never calls SetTarget with a queue (see
// Button.animatedFill's animated+timers gate) rather than asking colorAnim
// to behave differently. Passing a nil queue to SetTarget IS supported,
// though, as the one exception: it jumps straight to the target with no
// tween at all — the fallback for "animated requested but no timers.Queue
// wired yet" (SetAnimated(true) with no SetTimers call).
type colorAnim struct {
	current render.Color // the color currently being displayed
	from    render.Color // the color the in-flight tween started from
	to      render.Color // the color the in-flight (or just-settled) tween targets
	tween   *anim.Tween
}

// newColorAnim returns a colorAnim already settled at (and displaying) c —
// no animation plays until the first SetTarget call requests a different
// color.
func newColorAnim(c render.Color) *colorAnim {
	return &colorAnim{current: c, from: c, to: c}
}

// Current returns the color currently displayed: the live interpolated
// value while a tween is in flight, or the settled color otherwise.
func (a *colorAnim) Current() render.Color {
	return a.current
}

// SetTarget starts (or redirects) a cross-fade from the CURRENTLY DISPLAYED
// color toward target, driven by q over colorAnimDuration with EaseOut. A
// nil q skips the tween entirely and jumps straight to target — see the
// colorAnim doc comment.
//
// Calling SetTarget again with the same color already being approached (or
// already settled at — a.to covers both) is a deliberate no-op: a control's
// Render typically calls SetTarget every frame with whatever color the
// current state resolves to, and restarting the tween from a.current every
// single one of those frames would make it chase the target asymptotically
// forever (current always a little closer, never quite arriving, onDone
// never firing) instead of a clean, finite cross-fade. Only an ACTUAL
// change in target (a real state transition, e.g. rest->hover) redirects
// the tween — always from whatever color is currently on screen, so a
// rapid hover-in/hover-out never pops.
func (a *colorAnim) SetTarget(q *timers.Queue, target render.Color) {
	if target == a.to {
		return
	}
	if a.tween != nil {
		a.tween.Stop()
		a.tween = nil
	}
	if q == nil {
		a.current = target
		a.from = target
		a.to = target
		return
	}

	a.from = a.current
	a.to = target
	a.tween = anim.NewTween(q, colorAnimDuration, anim.EaseOut, func(v float32) {
		// v==1 lands exactly at completion (EaseOut(1)==1 exactly); assign
		// a.to directly rather than through lerpColor's arithmetic so the
		// settled color is bit-exact (no float rounding drift) — this is
		// what lets Button's animated fill match its un-animated golden
		// once a tween finishes.
		if v >= 1 {
			a.current = a.to
			return
		}
		a.current = lerpColor(a.from, a.to, v)
	}, func() {
		a.tween = nil
	})
}

// lerpColor linearly interpolates each channel of from toward to by t
// (typically, but not required to be, in [0,1] — callers clamp upstream).
func lerpColor(from, to render.Color, t float32) render.Color {
	return render.Color{
		R: from.R + (to.R-from.R)*t,
		G: from.G + (to.G-from.G)*t,
		B: from.B + (to.B-from.B)*t,
		A: from.A + (to.A-from.A)*t,
	}
}
