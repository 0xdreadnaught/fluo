package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
)

// TestHitPathScaleIndependence is the Phase 8 Task 6 high-DPI audit's TDD
// invariant pin: core.MeasureWidget/core.ArrangeWidget (layout) and
// input.HitPath (hit-testing) take only LOGICAL px — render.Size,
// render.Rect, render.Point — and none of their signatures has any notion
// of a device-pixel scale. The renderer's scale (fbW/winW — see
// render/gl.Renderer.Begin's scale param, and app.Surface.Frame, which
// computes `scale = fbW/winW` and passes it ONLY to r.Begin) is consumed
// exclusively at draw time; it never reaches layout or hit-testing.
//
// This test lays out a small Canvas tree (two Fixed leaves) once, against
// a fixed logical size, and hit-tests three points. It then walks several
// framebuffer sizes a hi-DPI window might report for that SAME logical
// size — 1x, 1.5x, 2x, 3x — re-deriving scale exactly as
// app.Surface.Frame does (`scale := fbW / winW`), and asserts each derived
// scale changes nothing about the already-computed bounds or hit results.
// Since MeasureWidget, ArrangeWidget, and HitPath never accept a scale
// argument at all, this holds trivially by construction — which is
// exactly the invariant worth pinning: nothing in this codebase has a way
// to leak a device-pixel scale into logical layout or hit-testing.
func TestHitPathScaleIndependence(t *testing.T) {
	winW, winH := float32(200), float32(150)

	a := NewFixed(40, 30, render.RGB(1, 2, 3))
	b := NewFixed(50, 20, render.RGB(4, 5, 6))
	root := NewCanvas().Add(a, 10, 10).Add(b, 80, 60)

	logical := render.Size{W: winW, H: winH}
	core.MeasureWidget(root, logical)
	core.ArrangeWidget(root, render.Rect{X: 0, Y: 0, W: winW, H: winH})

	wantBoundsA := render.Rect{X: 10, Y: 10, W: 40, H: 30}
	wantBoundsB := render.Rect{X: 80, Y: 60, W: 50, H: 20}
	if got := core.BoundsOf(a); got != wantBoundsA {
		t.Fatalf("a bounds = %v, want %v", got, wantBoundsA)
	}
	if got := core.BoundsOf(b); got != wantBoundsB {
		t.Fatalf("b bounds = %v, want %v", got, wantBoundsB)
	}

	ptA := render.Point{X: 20, Y: 20}     // inside a's bounds
	ptB := render.Point{X: 100, Y: 70}    // inside b's bounds
	ptNone := render.Point{X: 250, Y: 20} // outside root's own bounds entirely (root stretches to fill 0..200 x 0..150)

	leafOf := func(p render.Point) core.Widget {
		path := input.HitPath(root, p)
		if len(path) == 0 {
			return nil
		}
		return path[len(path)-1]
	}

	if got := leafOf(ptA); got != core.Widget(a) {
		t.Fatalf("hit at ptA = %v, want a", got)
	}
	if got := leafOf(ptB); got != core.Widget(b) {
		t.Fatalf("hit at ptB = %v, want b", got)
	}
	if got := leafOf(ptNone); got != nil {
		t.Fatalf("hit at ptNone = %v, want nil", got)
	}

	// Simulate the framebuffer sizes a hi-DPI window would report for this
	// same logical (winW, winH) window, re-deriving scale exactly as
	// app.Surface.Frame does. Layout above ran once, before any of these
	// scales existed; HitPath below takes no scale argument to smuggle one
	// into — so every one of these iterations must reproduce the identical
	// bounds and hit results asserted above.
	for _, fbScale := range []float32{1, 1.5, 2, 3} {
		fbW, fbH := winW*fbScale, winH*fbScale

		var scale float32 = 1
		if winW != 0 { // mirrors app.Surface.Frame's own guard
			scale = fbW / winW
		}
		if scale != fbScale {
			t.Fatalf("sanity: derived scale %v != %v", scale, fbScale)
		}
		_ = fbH // only fbW/winW feeds scale; fbH is unused by that formula, same as Frame

		if got := core.BoundsOf(a); got != wantBoundsA {
			t.Fatalf("scale=%v: a bounds changed to %v", fbScale, got)
		}
		if got := core.BoundsOf(b); got != wantBoundsB {
			t.Fatalf("scale=%v: b bounds changed to %v", fbScale, got)
		}
		if got := leafOf(ptA); got != core.Widget(a) {
			t.Fatalf("scale=%v: hit at ptA = %v, want a", fbScale, got)
		}
		if got := leafOf(ptB); got != core.Widget(b) {
			t.Fatalf("scale=%v: hit at ptB = %v, want b", fbScale, got)
		}
		if got := leafOf(ptNone); got != nil {
			t.Fatalf("scale=%v: hit at ptNone = %v, want nil", fbScale, got)
		}
	}
}
