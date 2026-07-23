package controls

import (
	"testing"
	"time"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/timers"
)

// newTestTooltip returns a ToolTipArea wrapping a small Fixed child, hosted
// as the sole content of a fresh OverlayHost already laid out at {200,200}
// — the setup every test below shares. No Router is wired: these tests
// drive OnPointer directly (matching the brief's "Enter/Leave only" scope),
// not through a real hit-tested pointer flow.
func newTestTooltip(t *testing.T) (*ToolTipArea, *OverlayHost) {
	t.Helper()
	child := NewFixed(40, 20, render.RGB(10, 20, 30))
	ta := NewToolTipArea(child, nil, "hint")

	host := NewOverlayHost()
	host.SetContent(ta)
	layoutOverlay(host, 200, 200)

	return ta, host
}

func TestToolTipEnterAdvancePastDelayShows(t *testing.T) {
	start := time.Now()
	q := timers.NewQueue(start)

	ta, host := newTestTooltip(t)
	ta.SetTimers(q)

	ta.OnPointer(&input.PointerEvent{Action: input.Enter})
	if host.PopupCount() != 0 {
		t.Fatalf("PopupCount right after Enter = %d, want 0 (not yet due)", host.PopupCount())
	}

	q.Advance(start.Add(tooltipDelay))
	if host.PopupCount() != 1 {
		t.Fatalf("PopupCount after Advance(tooltipDelay) = %d, want 1", host.PopupCount())
	}
}

func TestToolTipLeaveBeforeDelayNeverShows(t *testing.T) {
	start := time.Now()
	q := timers.NewQueue(start)

	ta, host := newTestTooltip(t)
	ta.SetTimers(q)

	ta.OnPointer(&input.PointerEvent{Action: input.Enter})
	ta.OnPointer(&input.PointerEvent{Action: input.Leave})

	q.Advance(start.Add(2 * tooltipDelay))
	if host.PopupCount() != 0 {
		t.Fatalf("PopupCount after Leave-before-delay then Advance = %d, want 0 (must never show)", host.PopupCount())
	}
}

func TestToolTipLeaveAfterShowHides(t *testing.T) {
	start := time.Now()
	q := timers.NewQueue(start)

	ta, host := newTestTooltip(t)
	ta.SetTimers(q)

	ta.OnPointer(&input.PointerEvent{Action: input.Enter})
	q.Advance(start.Add(tooltipDelay))
	if host.PopupCount() != 1 {
		t.Fatalf("PopupCount after show = %d, want 1", host.PopupCount())
	}

	ta.OnPointer(&input.PointerEvent{Action: input.Leave})
	if host.PopupCount() != 0 {
		t.Fatalf("PopupCount after Leave = %d, want 0 (hidden)", host.PopupCount())
	}
}

func TestToolTipNoTimersShowsImmediately(t *testing.T) {
	ta, host := newTestTooltip(t) // no SetTimers call: nil queue.

	ta.OnPointer(&input.PointerEvent{Action: input.Enter})
	if host.PopupCount() != 1 {
		t.Fatalf("PopupCount after Enter with no timers wired = %d, want 1 (immediate)", host.PopupCount())
	}
}

func TestToolTipNeverMarksHandled(t *testing.T) {
	ta, _ := newTestTooltip(t)

	e := &input.PointerEvent{Action: input.Enter}
	ta.OnPointer(e)
	if e.Handled {
		t.Fatal("Enter set Handled = true, want false (must not swallow hover events)")
	}

	e2 := &input.PointerEvent{Action: input.Leave}
	ta.OnPointer(e2)
	if e2.Handled {
		t.Fatal("Leave set Handled = true, want false")
	}
}

func TestToolTipWrapperTransparentToChildLayout(t *testing.T) {
	child := NewFixed(40, 20, render.RGB(1, 2, 3))
	ta := NewToolTipArea(child, nil, "hint")

	core.MeasureWidget(ta, render.Size{W: 200, H: 200})
	core.ArrangeWidget(ta, render.Rect{X: 10, Y: 5, W: 200, H: 200})

	got, want := core.BoundsOf(child), core.BoundsOf(ta)
	if got != want {
		t.Fatalf("child bounds = %+v, want %+v (wrapper transparent to child layout)", got, want)
	}
}
