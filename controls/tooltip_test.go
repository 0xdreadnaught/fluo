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

// TestToolTipNestedInPopupStaysOpen is the regression test for the
// diffPopupHover reentrancy bug: a ToolTipArea-wrapped row lives inside an
// already-open (combo-like) popup, and — with NO timers.Queue wired — its
// Enter handler calls ShowPopup SYNCHRONOUSLY, mid-delivery, from inside
// OverlayHost.OnPointer's own hover-diff loop (diffPopupHover). Before the
// snapshot-and-generation-guard fix, the outer diffPopupHover call's
// unconditional final write clobbered the nested ShowPopup's own popupHover
// reset with a stale path rooted at the now-no-longer-topmost popup, and the
// very next Move then delivered a phantom Leave to the ToolTipArea — closing
// the tooltip it had just opened.
func TestToolTipNestedInPopupStaysOpen(t *testing.T) {
	row := NewToolTipArea(NewFixed(60, 20, render.RGB(1, 2, 3)), nil, "hint")
	// No SetTimers call: Enter shows the tip immediately and synchronously —
	// the reentrant call this test targets.

	host := NewOverlayHost()
	r := input.NewRouter()
	host.SetRouter(r)
	host.SetContent(NewFixed(300, 300, render.RGB(4, 5, 6)))
	r.SetRoot(host) // BEFORE ShowPopup: SetRoot itself resets any capture

	// A combo-like popup containing the tooltip-wrapped row, opened
	// directly — this test isn't about ComboBox specifically, it just needs
	// SOME already-open popup whose content is a ToolTipArea.
	anchor := render.Rect{X: 20, Y: 20, W: 10, H: 10}
	host.ShowPopup(row, anchor, nil)
	layoutOverlay(host, 300, 300)

	rowBounds := core.BoundsOf(row)
	pos := render.Point{X: rowBounds.X + 5, Y: rowBounds.Y + 5}

	r.PointerMove(pos, 0) // Enter -> ToolTipArea shows its tip synchronously

	if got := host.PopupCount(); got != 2 {
		t.Fatalf("PopupCount after Enter opened the nested tooltip = %d, want 2 (combo-like popup + tooltip)", got)
	}

	// Move 1px within the SAME row: must not phantom-close the tooltip.
	r.PointerMove(render.Point{X: pos.X + 1, Y: pos.Y}, 0)

	if got := host.PopupCount(); got != 2 {
		t.Fatalf("PopupCount after a second Move within the same row = %d, want 2 (tooltip must stay open, no phantom close)", got)
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
