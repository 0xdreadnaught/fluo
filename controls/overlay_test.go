package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
)

// ovProbe is a minimal leaf widget for OverlayHost tests: it records every
// Press it receives (so light-dismiss tests can assert content did or
// didn't see one) and can optionally accept keyboard focus (so the
// Detach-on-close test has something router.Focus can point at). Explicit
// size comes from core.Element's SetWidth/SetHeight (as in the input
// package's own probe pattern), not a MeasureContent override.
type ovProbe struct {
	core.Element

	events    []string
	focusable bool
}

func (p *ovProbe) OnPointer(e *input.PointerEvent) {
	switch e.Action {
	case input.Press:
		p.events = append(p.events, "press")
	case input.Move:
		p.events = append(p.events, "move")
	}
}

func (p *ovProbe) AcceptsFocus() bool { return p.focusable }

// layoutOverlay measures then arranges h at bounds {0,0,w,h}, the pattern
// every test below shares.
func layoutOverlay(h *OverlayHost, w, hgt float32) {
	core.MeasureWidget(h, render.Size{W: w, H: hgt})
	core.ArrangeWidget(h, render.Rect{X: 0, Y: 0, W: w, H: hgt})
}

func TestOverlayPlacementBelowAnchor(t *testing.T) {
	host := NewOverlayHost()
	host.SetContent(NewFixed(400, 300, render.RGB(1, 2, 3)))

	popup := NewFixed(60, 30, render.RGB(4, 5, 6))
	anchor := render.Rect{X: 100, Y: 50, W: 40, H: 20}
	host.ShowPopup(popup, anchor, nil)

	layoutOverlay(host, 400, 300)

	want := render.Rect{X: 100, Y: 70, W: 60, H: 30} // Y = anchor.Bottom()
	if got := core.BoundsOf(popup); got != want {
		t.Fatalf("popup bounds = %+v, want %+v", got, want)
	}
}

func TestOverlayPlacementFlipsAboveNearBottomEdge(t *testing.T) {
	host := NewOverlayHost()
	host.SetContent(NewFixed(400, 300, render.RGB(1, 2, 3)))

	popup := NewFixed(60, 30, render.RGB(4, 5, 6))
	// anchor.Bottom() = 295; 295+30 = 325 > bounds.Bottom() (300) -> flips.
	anchor := render.Rect{X: 100, Y: 290, W: 40, H: 5}
	host.ShowPopup(popup, anchor, nil)

	layoutOverlay(host, 400, 300)

	want := render.Rect{X: 100, Y: 260, W: 60, H: 30} // Y = anchor.Y - popupH
	if got := core.BoundsOf(popup); got != want {
		t.Fatalf("popup bounds = %+v, want %+v", got, want)
	}
}

func TestOverlayPlacementFlipClampsToHostTop(t *testing.T) {
	host := NewOverlayHost()
	host.SetContent(NewFixed(400, 50, render.RGB(1, 2, 3)))

	popup := NewFixed(60, 30, render.RGB(4, 5, 6))
	// anchor.Bottom() = 45; 45+30 = 75 > bounds.Bottom() (50) -> flips.
	// Flipped y = anchor.Y - popupH = 10 - 30 = -20, clamped to bounds.Y (0).
	anchor := render.Rect{X: 100, Y: 10, W: 40, H: 35}
	host.ShowPopup(popup, anchor, nil)

	layoutOverlay(host, 400, 50)

	if got := core.BoundsOf(popup).Y; got != 0 {
		t.Fatalf("popup Y = %v, want 0 (clamped after flip)", got)
	}
}

func TestOverlayPlacementHorizontalClamp(t *testing.T) {
	host := NewOverlayHost()
	host.SetContent(NewFixed(400, 300, render.RGB(1, 2, 3)))

	popup := NewFixed(60, 30, render.RGB(4, 5, 6))
	// anchor.X = 380; bounds.Right()-popupW = 340 -> clamps down to 340.
	anchor := render.Rect{X: 380, Y: 50, W: 10, H: 10}
	host.ShowPopup(popup, anchor, nil)

	layoutOverlay(host, 400, 300)

	if got := core.BoundsOf(popup).X; got != 340 {
		t.Fatalf("popup X = %v, want 340 (clamped to bounds.Right()-popupW)", got)
	}
}

func TestOverlayPlacementHorizontalClampNegative(t *testing.T) {
	host := NewOverlayHost()
	host.SetContent(NewFixed(400, 300, render.RGB(1, 2, 3)))

	popup := NewFixed(60, 30, render.RGB(4, 5, 6))
	anchor := render.Rect{X: -20, Y: 50, W: 10, H: 10}
	host.ShowPopup(popup, anchor, nil)

	layoutOverlay(host, 400, 300)

	if got := core.BoundsOf(popup).X; got != 0 {
		t.Fatalf("popup X = %v, want 0 (clamped to bounds.X)", got)
	}
}

// TestOverlayLightDismissSwallowsOutsidePress is the primary light-dismiss
// regression: content is a single probe covering the ENTIRE host (so every
// non-popup pixel is "under" it), a real input.Router drives the press, and
// the press lands well outside the popup. The only way this can close the
// popup AND leave the probe's event log empty is the capture-based
// interception documented on OverlayHost.OnPointer — proving light dismiss
// doesn't let the press fall through to content first.
func TestOverlayLightDismissSwallowsOutsidePress(t *testing.T) {
	probe := &ovProbe{}
	probe.SetWidth(400)
	probe.SetHeight(300)

	host := NewOverlayHost()
	r := input.NewRouter()
	host.SetRouter(r)
	host.SetContent(probe)
	r.SetRoot(host) // BEFORE ShowPopup: SetRoot itself resets any capture

	popup := NewFixed(60, 30, render.RGB(4, 5, 6))
	anchor := render.Rect{X: 300, Y: 50, W: 10, H: 10}
	host.ShowPopup(popup, anchor, nil)

	layoutOverlay(host, 400, 300)

	if got := host.PopupCount(); got != 1 {
		t.Fatalf("PopupCount before press = %v, want 1", got)
	}

	// Popup bounds are {300,60,60,30} (below-anchor, no clamp needed); (10,10)
	// is well outside that and well inside the probe's full-host coverage.
	r.PointerButton(input.ButtonLeft, true, render.Point{X: 10, Y: 10}, 0)

	if got := host.PopupCount(); got != 0 {
		t.Fatalf("PopupCount after outside press = %v, want 0 (dismissed)", got)
	}
	if got := probe.events; len(got) != 0 {
		t.Fatalf("probe.events = %v, want none (press must be swallowed, not delivered to content)", got)
	}
}

// TestOverlayLightDismissKeepsOpenOnInsidePress mirrors the above but
// presses inside the topmost popup's bounds: the popup must stay open.
func TestOverlayLightDismissKeepsOpenOnInsidePress(t *testing.T) {
	probe := &ovProbe{}
	probe.SetWidth(400)
	probe.SetHeight(300)

	host := NewOverlayHost()
	r := input.NewRouter()
	host.SetRouter(r)
	host.SetContent(probe)
	r.SetRoot(host) // BEFORE ShowPopup: SetRoot itself resets any capture

	popup := NewFixed(60, 30, render.RGB(4, 5, 6))
	anchor := render.Rect{X: 300, Y: 50, W: 10, H: 10}
	host.ShowPopup(popup, anchor, nil)

	layoutOverlay(host, 400, 300)

	// Popup bounds are {300,60,60,30}; (310,65) is inside them.
	r.PointerButton(input.ButtonLeft, true, render.Point{X: 310, Y: 65}, 0)

	if got := host.PopupCount(); got != 1 {
		t.Fatalf("PopupCount after inside press = %v, want 1 (still open)", got)
	}
}

// TestPopupContentReceivesPress proves the forwarding half of OnPointer: a
// press inside the topmost popup's bounds reaches a probe INSIDE the popup
// (via input.HitPath + input.Bubble), while a probe under the host's own
// content — still covering the entire host — sees nothing, and the popup
// stays open (forwarding, not dismissal).
func TestPopupContentReceivesPress(t *testing.T) {
	hostProbe := &ovProbe{}
	hostProbe.SetWidth(400)
	hostProbe.SetHeight(300)

	popupProbe := &ovProbe{}
	popupProbe.SetWidth(60)
	popupProbe.SetHeight(30)

	host := NewOverlayHost()
	r := input.NewRouter()
	host.SetRouter(r)
	host.SetContent(hostProbe)
	r.SetRoot(host) // BEFORE ShowPopup: SetRoot itself resets any capture

	anchor := render.Rect{X: 300, Y: 50, W: 10, H: 10}
	host.ShowPopup(popupProbe, anchor, nil)
	layoutOverlay(host, 400, 300)

	// Popup bounds are {300,60,60,30}; (310,65) is inside them.
	r.PointerButton(input.ButtonLeft, true, render.Point{X: 310, Y: 65}, 0)

	if got := popupProbe.events; len(got) != 1 || got[0] != "press" {
		t.Fatalf("popupProbe.events = %v, want [press] (forwarded into the popup's subtree)", got)
	}
	if got := hostProbe.events; len(got) != 0 {
		t.Fatalf("hostProbe.events = %v, want none (content stays inert while a popup is open)", got)
	}
	if got := host.PopupCount(); got != 1 {
		t.Fatalf("PopupCount after inside press = %v, want 1 (forwarding, not dismissal)", got)
	}
}

// TestPopupContentMoveForwarded proves forwarding isn't Press-only: a Move
// inside the topmost popup's bounds also reaches the popup's own probe.
func TestPopupContentMoveForwarded(t *testing.T) {
	popupProbe := &ovProbe{}
	popupProbe.SetWidth(60)
	popupProbe.SetHeight(30)

	host := NewOverlayHost()
	r := input.NewRouter()
	host.SetRouter(r)
	host.SetContent(NewFixed(400, 300, render.RGB(1, 2, 3)))
	r.SetRoot(host)

	anchor := render.Rect{X: 300, Y: 50, W: 10, H: 10}
	host.ShowPopup(popupProbe, anchor, nil)
	layoutOverlay(host, 400, 300)

	// Popup bounds are {300,60,60,30}; (310,65) is inside them.
	r.PointerMove(render.Point{X: 310, Y: 65}, 0)

	if got := popupProbe.events; len(got) != 1 || got[0] != "move" {
		t.Fatalf("popupProbe.events = %v, want [move] (forwarded into the popup's subtree)", got)
	}
}

func TestOverlayClosePopupIdempotentAndDismissOnce(t *testing.T) {
	host := NewOverlayHost()
	popup := NewFixed(60, 30, render.RGB(4, 5, 6))

	dismissCount := 0
	host.ShowPopup(popup, render.Rect{X: 10, Y: 10, W: 10, H: 10}, func() { dismissCount++ })

	if got := host.PopupCount(); got != 1 {
		t.Fatalf("PopupCount after ShowPopup = %v, want 1", got)
	}

	host.ClosePopup(popup)
	if got := host.PopupCount(); got != 0 {
		t.Fatalf("PopupCount after ClosePopup = %v, want 0", got)
	}
	if dismissCount != 1 {
		t.Fatalf("dismissCount after first ClosePopup = %v, want 1", dismissCount)
	}

	// Repeat calls (already closed) are no-ops: no further onDismiss fires.
	host.ClosePopup(popup)
	host.ClosePopup(popup)
	if dismissCount != 1 {
		t.Fatalf("dismissCount after repeat ClosePopup calls = %v, want still 1 (idempotent)", dismissCount)
	}
}

func TestOverlayCloseTopPopupClosesOnlyTopmost(t *testing.T) {
	host := NewOverlayHost()
	first := NewFixed(60, 30, render.RGB(1, 0, 0))
	second := NewFixed(40, 20, render.RGB(0, 1, 0))

	host.ShowPopup(first, render.Rect{X: 10, Y: 10, W: 10, H: 10}, nil)
	host.ShowPopup(second, render.Rect{X: 20, Y: 20, W: 10, H: 10}, nil)

	if got := host.PopupCount(); got != 2 {
		t.Fatalf("PopupCount after two ShowPopup calls = %v, want 2", got)
	}

	host.CloseTopPopup()

	if got := host.PopupCount(); got != 1 {
		t.Fatalf("PopupCount after CloseTopPopup = %v, want 1", got)
	}
	if got := core.ParentOf(second); got != nil {
		t.Fatalf("second (topmost) ParentOf = %v, want nil (detached)", got)
	}
	if got := core.ParentOf(first); got != core.Widget(host) {
		t.Fatalf("first (still open) ParentOf = %v, want host (untouched)", got)
	}
}

func TestOverlayClosePopupDetachesRouterState(t *testing.T) {
	focusable := &ovProbe{focusable: true}
	focusable.SetWidth(50)
	focusable.SetHeight(20)

	host := NewOverlayHost()
	r := input.NewRouter()
	host.SetRouter(r)
	r.SetRoot(host)

	host.ShowPopup(focusable, render.Rect{X: 10, Y: 10, W: 10, H: 10}, nil)
	layoutOverlay(host, 400, 300)

	r.Focus(focusable)
	if got := r.Focused(); got != core.Widget(focusable) {
		t.Fatalf("Focused() = %v, want focusable", got)
	}

	host.ClosePopup(focusable)

	if got := r.Focused(); got != nil {
		t.Fatalf("Focused() after ClosePopup = %v, want nil (router.Detach cleared it)", got)
	}
}

func TestOverlayHostForFindsThroughNestedPanels(t *testing.T) {
	leaf := NewFixed(10, 10, render.RGB(1, 2, 3))
	grid := NewGrid().Cols(Star(1))
	grid.Add(leaf, 0, 0)
	stack := NewStackPanel(Vertical).Add(grid)

	host := NewOverlayHost()
	host.SetContent(stack)

	if got := OverlayHostFor(leaf); got != host {
		t.Fatalf("OverlayHostFor(leaf) = %v, want host (through Grid+StackPanel nesting)", got)
	}
	if got := OverlayHostFor(stack); got != host {
		t.Fatalf("OverlayHostFor(stack) = %v, want host", got)
	}
	if got := OverlayHostFor(host); got != host {
		t.Fatalf("OverlayHostFor(host) = %v, want itself", got)
	}
}

func TestOverlayHostForNilWhenAbsent(t *testing.T) {
	orphan := NewFixed(10, 10, render.RGB(1, 2, 3))
	if got := OverlayHostFor(orphan); got != nil {
		t.Fatalf("OverlayHostFor(orphan) = %v, want nil (never attached to a host)", got)
	}
}
