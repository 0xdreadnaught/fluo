package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
)

// ovProbe is a minimal leaf widget for OverlayHost tests: it records every
// Press/Move it receives (so light-dismiss and forwarding tests can assert
// content or popup internals did or didn't see one), can optionally accept
// keyboard focus (so the Detach-on-close test has something router.Focus can
// point at), and can optionally capture-on-press + release-on-release (so
// TestPopupInternalCaptureRestoresHost can simulate a ScrollViewer-thumb-like
// drag started from a forwarded event inside an open popup). Explicit size
// comes from core.Element's SetWidth/SetHeight (as in the input package's
// own probe pattern), not a MeasureContent override.
type ovProbe struct {
	core.Element

	events    []string
	focusable bool
	capturing bool
}

func (p *ovProbe) OnPointer(e *input.PointerEvent) {
	switch e.Action {
	case input.Press:
		p.events = append(p.events, "press")
		if p.capturing {
			e.Router.Capture(p)
		}
	case input.Release:
		p.events = append(p.events, "release")
		if p.capturing {
			e.Router.Release()
		}
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

// TestPopupInternalCaptureRestoresHost proves the router's NESTED capture
// stack does its job for OverlayHost specifically: a popup-internal widget
// (standing in for a ScrollViewer thumb or similar drag) captures on press
// while the event is being forwarded under the host's own modal capture,
// then releases on release. Captured() must end up back on the host — not
// nil — so light dismiss is still armed for whatever happens next.
func TestPopupInternalCaptureRestoresHost(t *testing.T) {
	popupProbe := &ovProbe{capturing: true}
	popupProbe.SetWidth(60)
	popupProbe.SetHeight(30)

	host := NewOverlayHost()
	r := input.NewRouter()
	host.SetRouter(r)
	host.SetContent(NewFixed(400, 300, render.RGB(1, 2, 3)))
	r.SetRoot(host) // BEFORE ShowPopup: SetRoot itself resets any capture

	anchor := render.Rect{X: 300, Y: 50, W: 10, H: 10}
	host.ShowPopup(popupProbe, anchor, nil)
	layoutOverlay(host, 400, 300)

	if got := r.Captured(); got != core.Widget(host) {
		t.Fatalf("Captured() after ShowPopup = %v, want host", got)
	}

	// Popup bounds are {300,60,60,30}; (310,65) is inside them.
	pos := render.Point{X: 310, Y: 65}
	r.PointerButton(input.ButtonLeft, true, pos, 0) // forwarded press -> popupProbe captures (nests over host)

	if got := r.Captured(); got != core.Widget(popupProbe) {
		t.Fatalf("Captured() mid-drag = %v, want popupProbe (nested over host)", got)
	}

	r.PointerButton(input.ButtonLeft, false, pos, 0) // popupProbe's own release -> pops back to host

	if got := r.Captured(); got != core.Widget(host) {
		t.Fatalf("Captured() after inner release = %v, want host (restored, light dismiss still armed)", got)
	}
	if got := popupProbe.events; len(got) != 2 || got[0] != "press" || got[1] != "release" {
		t.Fatalf("popupProbe.events = %v, want [press release]", got)
	}
}

// TestMultiPopupCloseRestoresPointerFlow is the overlay-level regression for
// the multi-popup capture leak: opening a SECOND popup while the host
// already holds the capture (from the first) must not push a second,
// redundant entry onto the router's capture stack — otherwise closing both
// popups would leave one stale "host" entry behind, Captured() would still
// report the host with zero popups open, OnPointer's len(h.popups)==0
// early-return would swallow every future press, and pointer input would be
// permanently dead. After closing both popups, a press at the content
// probe's position must actually reach it, and Captured() must be nil.
func TestMultiPopupCloseRestoresPointerFlow(t *testing.T) {
	probe := &ovProbe{}
	probe.SetWidth(400)
	probe.SetHeight(300)

	host := NewOverlayHost()
	r := input.NewRouter()
	host.SetRouter(r)
	host.SetContent(probe)
	r.SetRoot(host) // BEFORE ShowPopup: SetRoot itself resets any capture

	first := NewFixed(60, 30, render.RGB(1, 0, 0))
	second := NewFixed(40, 20, render.RGB(0, 1, 0))
	host.ShowPopup(first, render.Rect{X: 10, Y: 10, W: 10, H: 10}, nil)
	host.ShowPopup(second, render.Rect{X: 200, Y: 10, W: 10, H: 10}, nil)
	layoutOverlay(host, 400, 300)

	if got := r.Captured(); got != core.Widget(host) {
		t.Fatalf("Captured() with two popups open = %v, want host", got)
	}

	host.CloseTopPopup() // closes second; first still open, host still captured
	if got := host.PopupCount(); got != 1 {
		t.Fatalf("PopupCount after closing one of two = %v, want 1", got)
	}
	host.CloseTopPopup() // closes first; stack now empty -> capture released

	if got := host.PopupCount(); got != 0 {
		t.Fatalf("PopupCount after closing both = %v, want 0", got)
	}
	if got := r.Captured(); got != nil {
		t.Fatalf("Captured() after closing both popups = %v, want nil (fully released)", got)
	}

	// With capture released, an ordinary press must reach content normally.
	r.PointerButton(input.ButtonLeft, true, render.Point{X: 10, Y: 10}, 0)

	if got := probe.events; len(got) != 1 || got[0] != "press" {
		t.Fatalf("probe.events = %v, want [press] (pointer flow restored, not permanently swallowed)", got)
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

// hoverProbe is a minimal leaf widget that records every Enter/Leave/Move it
// receives. Unlike ovProbe above (which only tracks Press/Release/Move, and
// is shared by tests that must NOT see extra Enter/Leave noise — e.g.
// TestPopupContentMoveForwarded's exact-one-event assertion), hoverProbe
// exists specifically to prove OverlayHost's hover synthesis (diffPopupHover,
// invoked from OnPointer on a forwarded or swallowed Move) delivers real
// Enter/Leave pairs to popup-internal widgets from live pointer movement —
// not just Press/Release, which is all that reached them before that fix.
type hoverProbe struct {
	core.Element
	events []string
}

func (p *hoverProbe) OnPointer(e *input.PointerEvent) {
	switch e.Action {
	case input.Enter:
		p.events = append(p.events, "enter")
	case input.Leave:
		p.events = append(p.events, "leave")
	case input.Move:
		p.events = append(p.events, "move")
	}
}

// TestPopupHoverEnterLeaveSynthesis is the OverlayHost-level regression for
// the row-hover fix: two hoverProbes stacked vertically inside an open
// popup, driven by REAL router.PointerMove calls (not direct OnPointer
// calls) while the host holds its modal capture. Moving onto rowA must
// synthesize an Enter (then forward the Move); moving from rowA to rowB must
// synthesize a Leave on rowA and an Enter on rowB; moving outside the popup
// entirely (still captured — the popup itself stays open, this isn't a
// light-dismiss press) must synthesize a Leave on rowB, with nothing left
// hovered.
func TestPopupHoverEnterLeaveSynthesis(t *testing.T) {
	rowA := &hoverProbe{}
	rowA.SetWidth(60)
	rowA.SetHeight(20)
	rowB := &hoverProbe{}
	rowB.SetWidth(60)
	rowB.SetHeight(20)

	stack := NewStackPanel(Vertical).Add(rowA, rowB)

	host := NewOverlayHost()
	r := input.NewRouter()
	host.SetRouter(r)
	host.SetContent(NewFixed(400, 300, render.RGB(1, 2, 3)))
	r.SetRoot(host) // BEFORE ShowPopup: SetRoot itself resets any capture

	anchor := render.Rect{X: 100, Y: 50, W: 10, H: 10}
	host.ShowPopup(stack, anchor, nil)
	layoutOverlay(host, 400, 300)

	// Popup (the stack, desired 60x40) is placed below-anchor at
	// {100,60,60,40}: rowA at {100,60,60,20}, rowB at {100,80,60,20}.
	posA := render.Point{X: 110, Y: 65}    // inside rowA
	posB := render.Point{X: 110, Y: 90}    // inside rowB
	posOutside := render.Point{X: 5, Y: 5} // well outside the popup entirely

	r.PointerMove(posA, 0)
	if got := rowA.events; len(got) != 2 || got[0] != "enter" || got[1] != "move" {
		t.Fatalf("rowA.events after move onto A = %v, want [enter move]", got)
	}
	if got := rowB.events; len(got) != 0 {
		t.Fatalf("rowB.events after move onto A = %v, want none", got)
	}

	r.PointerMove(posB, 0)
	if got := rowA.events; len(got) != 3 || got[2] != "leave" {
		t.Fatalf("rowA.events after move to B = %v, want [enter move leave]", got)
	}
	if got := rowB.events; len(got) != 2 || got[0] != "enter" || got[1] != "move" {
		t.Fatalf("rowB.events after move to B = %v, want [enter move]", got)
	}

	r.PointerMove(posOutside, 0)
	if got := rowB.events; len(got) != 3 || got[2] != "leave" {
		t.Fatalf("rowB.events after move outside popup = %v, want [enter move leave]", got)
	}
	if host.PopupCount() != 1 {
		t.Fatalf("PopupCount after moving outside (no press) = %d, want 1 (popup stays open)", host.PopupCount())
	}
}
