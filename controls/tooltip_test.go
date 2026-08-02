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

// TestToolTipRealRouterMoveShowsLeaveHidesPressPassesThrough is the Phase 5
// final-fix integration test for Important #1 (non-modal tooltips), driven
// entirely through a REAL input.Router (no direct OnPointer calls, unlike
// every other test in this file): a ToolTipArea-wrapped widget lives beside
// an unrelated probe under a real OverlayHost. Moving onto the wrapped
// widget shows the tip immediately (no timers.Queue wired); moving away
// hides it — Router's own ordinary (uncaptured) hover-diffing delivering
// Enter/Leave is what makes both of these work now that ShowPopupNonModal
// engages no capture at all. And, the key regression: with the tip open, a
// press on the unrelated probe reaches it directly — it is NOT swallowed by
// a light-dismiss capture, because there is none to swallow it (a modally
// captured tooltip, by contrast, WOULD eat that first click).
func TestToolTipRealRouterMoveShowsLeaveHidesPressPassesThrough(t *testing.T) {
	child := NewFixed(60, 20, render.RGB(1, 2, 3))
	ta := NewToolTipArea(child, nil, "hint") // no SetTimers: immediate show/hide

	probe := &ovProbe{}
	probe.SetWidth(60)
	probe.SetHeight(20)

	canvas := NewCanvas().Add(ta, 0, 0).Add(probe, 200, 0)

	host := NewOverlayHost()
	r := input.NewRouter()
	host.SetRouter(r)
	host.SetContent(canvas)
	r.SetRoot(host)
	layoutOverlay(host, 400, 200)

	taBounds := core.BoundsOf(ta)
	inside := render.Point{X: taBounds.X + 5, Y: taBounds.Y + 5}
	neutral := render.Point{X: 150, Y: 5} // outside both ta and probe entirely
	probeBounds := core.BoundsOf(probe)
	onProbe := render.Point{X: probeBounds.X + 5, Y: probeBounds.Y + 5}

	r.PointerMove(inside, 0) // Enter -> immediate show
	if got := host.PopupCount(); got != 1 {
		t.Fatalf("PopupCount after Move onto the tooltipped widget = %v, want 1", got)
	}

	r.PointerMove(neutral, 0) // Leave (ta) -> hides; doesn't touch probe
	if got := host.PopupCount(); got != 0 {
		t.Fatalf("PopupCount after Move away = %v, want 0 (Leave closed it)", got)
	}

	r.PointerMove(inside, 0) // re-show for the press check below
	if got := host.PopupCount(); got != 1 {
		t.Fatalf("PopupCount after re-showing = %v, want 1", got)
	}

	// probe has received no events yet (moves above never touched it): a
	// press on it now is the whole regression this test targets.
	r.PointerButton(input.ButtonLeft, true, onProbe, 0)
	if got := probe.events; len(got) != 1 || got[0] != "press" {
		t.Fatalf("probe.events = %v, want [press] (not swallowed by the open, non-modal tip)", got)
	}
	if got := host.PopupCount(); got != 1 {
		t.Fatalf("PopupCount after the press elsewhere = %v, want 1 (press doesn't light-dismiss a non-modal popup)", got)
	}
}

// TestToolTipPendingTimerCancelledWhenModalOpens is FIX #21 scenario (b): a
// tip whose dwell timer is still pending when a modal popup opens (e.g. from a
// keystroke, while the pointer dwells over the tooltip) must NOT fire — before
// the fix its timer kept running because the modal capture silences the hover
// diffing that would otherwise Leave the ToolTipArea, so the tip popped over
// the modal at stale bounds once the delay elapsed.
func TestToolTipPendingTimerCancelledWhenModalOpens(t *testing.T) {
	start := time.Now()
	q := timers.NewQueue(start)

	ta, host := newTestTooltip(t)
	ta.SetTimers(q)

	ta.OnPointer(&input.PointerEvent{Action: input.Enter}) // arms the dwell timer
	if ta.pendingTimer == nil {
		t.Fatal("pendingTimer nil right after Enter, want an armed timer")
	}

	// A modal popup opens while the timer is still pending.
	host.ShowPopup(NewFixed(30, 15, render.RGB(1, 2, 3)), render.Rect{X: 10, Y: 10}, nil)

	if ta.pendingTimer != nil {
		t.Fatal("pendingTimer still armed after a modal opened, want it stopped")
	}

	// Advancing past the delay must NOT show the tip: only the modal remains.
	q.Advance(start.Add(2 * tooltipDelay))
	if got := host.PopupCount(); got != 1 {
		t.Fatalf("PopupCount after Advance = %d, want 1 (the modal only; the tip must never fire over it)", got)
	}
}

// TestToolTipShowingTipHiddenWhenModalOpens is FIX #21 scenario (a): a tip
// that is already showing when a modal popup opens must be hidden, not left
// floating beside the modal for its whole lifetime.
func TestToolTipShowingTipHiddenWhenModalOpens(t *testing.T) {
	ta, host := newTestTooltip(t) // no timers: Enter shows immediately

	ta.OnPointer(&input.PointerEvent{Action: input.Enter})
	if got := host.PopupCount(); got != 1 {
		t.Fatalf("PopupCount after Enter = %d, want 1 (tip showing)", got)
	}

	host.ShowPopup(NewFixed(30, 15, render.RGB(1, 2, 3)), render.Rect{X: 10, Y: 10}, nil)

	// The tip is gone; only the modal remains.
	if got := host.PopupCount(); got != 1 {
		t.Fatalf("PopupCount after the modal opened = %d, want 1 (tip hidden, modal only)", got)
	}
	if ta.open {
		t.Fatal("ta.open still true after a modal opened, want the tip hidden")
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
