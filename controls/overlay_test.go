package controls

import (
	"math"
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
	case input.Wheel:
		p.events = append(p.events, "wheel")
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

// TestNonModalPopupDoesNotEngageCapture is the Phase 5 final-fix regression
// for Important #1 (non-modal popups): ShowPopupNonModal must never engage
// the host's router capture — with only a non-modal popup open, Captured()
// stays nil, and an ordinary press elsewhere reaches content normally (not
// swallowed by light-dismiss, because there is no light-dismiss to speak
// of without a capture).
func TestNonModalPopupDoesNotEngageCapture(t *testing.T) {
	probe := &ovProbe{}
	probe.SetWidth(400)
	probe.SetHeight(300)

	host := NewOverlayHost()
	r := input.NewRouter()
	host.SetRouter(r)
	host.SetContent(probe)
	r.SetRoot(host)

	popup := NewFixed(60, 30, render.RGB(4, 5, 6))
	anchor := render.Rect{X: 300, Y: 50, W: 10, H: 10}
	host.ShowPopupNonModal(popup, anchor, nil)
	layoutOverlay(host, 400, 300)

	if got := r.Captured(); got != nil {
		t.Fatalf("Captured() with only a non-modal popup open = %v, want nil", got)
	}

	// A press well outside the popup (but inside the probe's full-host
	// coverage) must reach the probe normally — no light-dismiss swallow.
	r.PointerButton(input.ButtonLeft, true, render.Point{X: 10, Y: 10}, 0)

	if got := probe.events; len(got) != 1 || got[0] != "press" {
		t.Fatalf("probe.events = %v, want [press] (non-modal popup must not swallow it)", got)
	}
	if got := host.PopupCount(); got != 1 {
		t.Fatalf("PopupCount after outside press = %v, want 1 (non-modal popup is never light-dismissed)", got)
	}
}

// TestMixedModalNonModalCaptureGovernedByModal proves the two popup kinds
// coexist correctly on the same stack: the modal capture stays engaged as
// long as ANY modal popup remains — closing the non-modal one first leaves
// it untouched, and only closing the modal one actually releases it.
func TestMixedModalNonModalCaptureGovernedByModal(t *testing.T) {
	host := NewOverlayHost()
	r := input.NewRouter()
	host.SetRouter(r)
	host.SetContent(NewFixed(400, 300, render.RGB(1, 2, 3)))
	r.SetRoot(host)

	modalPopup := NewFixed(60, 30, render.RGB(4, 5, 6))
	nonModalPopup := NewFixed(40, 20, render.RGB(7, 8, 9))
	host.ShowPopup(modalPopup, render.Rect{X: 10, Y: 10, W: 10, H: 10}, nil)
	host.ShowPopupNonModal(nonModalPopup, render.Rect{X: 200, Y: 10, W: 10, H: 10}, nil)
	layoutOverlay(host, 400, 300)

	if got := r.Captured(); got != core.Widget(host) {
		t.Fatalf("Captured() with modal+non-modal open = %v, want host", got)
	}

	host.CloseTopPopup() // closes nonModalPopup (topmost); modalPopup still open
	if got := host.PopupCount(); got != 1 {
		t.Fatalf("PopupCount after closing the non-modal one = %v, want 1", got)
	}
	if got := r.Captured(); got != core.Widget(host) {
		t.Fatalf("Captured() after closing the non-modal popup = %v, want host (modal still open)", got)
	}

	host.CloseTopPopup() // closes modalPopup; no modal popup left
	if got := host.PopupCount(); got != 0 {
		t.Fatalf("PopupCount after closing both = %v, want 0", got)
	}
	if got := r.Captured(); got != nil {
		t.Fatalf("Captured() after closing the modal popup = %v, want nil (fully released)", got)
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

// TestOverlayChainOutsidePressClosesAllPopups is the overlay-level regression
// for Phase 8 Task 1's (a) semantics: with a TWO-level popup stack (neither
// popup overlapping the other), a single press that lands inside NEITHER
// popup (but well inside content's full-host coverage) must close BOTH,
// topmost first (each firing its own onDismiss, in that order), and swallow
// the press — content must never see it. This replaces the old
// topmost-only CloseTopPopup swallow.
func TestOverlayChainOutsidePressClosesAllPopups(t *testing.T) {
	content := &ovProbe{}
	content.SetWidth(400)
	content.SetHeight(300)

	host := NewOverlayHost()
	r := input.NewRouter()
	host.SetRouter(r)
	host.SetContent(content)
	r.SetRoot(host) // BEFORE ShowPopup: SetRoot itself resets any capture

	var dismissed []string
	first := NewFixed(60, 30, render.RGB(1, 0, 0))
	second := NewFixed(40, 20, render.RGB(0, 1, 0))
	host.ShowPopup(first, render.Rect{X: 10, Y: 10, W: 10, H: 10}, func() { dismissed = append(dismissed, "first") })
	host.ShowPopup(second, render.Rect{X: 200, Y: 10, W: 10, H: 10}, func() { dismissed = append(dismissed, "second") })
	layoutOverlay(host, 400, 300)

	// first bounds {10,20,60,30}; second bounds {200,20,40,20}; (350,150) is
	// outside both, well inside content's full-host coverage.
	outside := render.Point{X: 350, Y: 150}
	r.PointerButton(input.ButtonLeft, true, outside, 0)

	if got := host.PopupCount(); got != 0 {
		t.Fatalf("PopupCount after outside press = %d, want 0 (whole chain closed in one press)", got)
	}
	want := []string{"second", "first"} // topmost first
	if len(dismissed) != len(want) || dismissed[0] != want[0] || dismissed[1] != want[1] {
		t.Fatalf("dismissed = %v, want %v", dismissed, want)
	}
	if got := content.events; len(got) != 0 {
		t.Fatalf("content.events = %v, want none (press swallowed, not delivered to content)", got)
	}
}

// TestOverlayChainPressInLowerPopupClosesAboveAndDelivers is the
// overlay-level regression for Phase 8 Task 1's (b) semantics: with a
// TWO-level popup stack, a press that lands inside the LOWER (non-topmost)
// popup's bounds only must close every popup above it (here, just the
// topmost one — firing its onDismiss) and then forward the press into the
// lower popup's own subtree via input.HitPath + input.Bubble, exactly as
// single-popup forwarding always worked.
func TestOverlayChainPressInLowerPopupClosesAboveAndDelivers(t *testing.T) {
	lowerProbe := &ovProbe{}
	lowerProbe.SetWidth(60)
	lowerProbe.SetHeight(30)
	upperProbe := &ovProbe{}
	upperProbe.SetWidth(40)
	upperProbe.SetHeight(20)

	host := NewOverlayHost()
	r := input.NewRouter()
	host.SetRouter(r)
	host.SetContent(NewFixed(400, 300, render.RGB(1, 2, 3)))
	r.SetRoot(host) // BEFORE ShowPopup: SetRoot itself resets any capture

	upperDismissed := false
	host.ShowPopup(lowerProbe, render.Rect{X: 10, Y: 10, W: 10, H: 10}, nil)
	host.ShowPopup(upperProbe, render.Rect{X: 200, Y: 10, W: 10, H: 10}, func() { upperDismissed = true })
	layoutOverlay(host, 400, 300)

	// lowerProbe bounds {10,20,60,30}; press inside it, well outside upperProbe.
	pos := render.Point{X: 30, Y: 30}
	r.PointerButton(input.ButtonLeft, true, pos, 0)

	if !upperDismissed {
		t.Fatal("upper popup's onDismiss did not fire — want it closed (chain-aware: popups above the pressed level close first)")
	}
	if got := host.PopupCount(); got != 1 {
		t.Fatalf("PopupCount after press in the lower popup = %d, want 1 (only the lower popup remains)", got)
	}
	if got := lowerProbe.events; len(got) != 1 || got[0] != "press" {
		t.Fatalf("lowerProbe.events = %v, want [press] (forwarded into the containing popup)", got)
	}
}

// TestOverlayChainWheelClosesAbove pins the "close-above generalizes to
// every pointer action, not just Press/Move" claim: with a TWO-level popup
// stack, a Wheel event landing inside the LOWER (non-topmost) popup's
// bounds closes the upper popup first (its onDismiss fires) and forwards
// the Wheel into the lower popup's own subtree — no panic, no special-case
// needed, since OnPointer's popupAt + close-above walk never branches on
// e.Action (only the diffPopupHover call, gated on Move, does).
func TestOverlayChainWheelClosesAbove(t *testing.T) {
	lowerProbe := &ovProbe{}
	lowerProbe.SetWidth(60)
	lowerProbe.SetHeight(30)
	upperProbe := &ovProbe{}
	upperProbe.SetWidth(40)
	upperProbe.SetHeight(20)

	host := NewOverlayHost()
	r := input.NewRouter()
	host.SetRouter(r)
	host.SetContent(NewFixed(400, 300, render.RGB(1, 2, 3)))
	r.SetRoot(host) // BEFORE ShowPopup: SetRoot itself resets any capture

	upperDismissed := false
	host.ShowPopup(lowerProbe, render.Rect{X: 10, Y: 10, W: 10, H: 10}, nil)
	host.ShowPopup(upperProbe, render.Rect{X: 200, Y: 10, W: 10, H: 10}, func() { upperDismissed = true })
	layoutOverlay(host, 400, 300)

	// lowerProbe bounds {10,20,60,30}; wheel inside it, well outside upperProbe.
	pos := render.Point{X: 30, Y: 30}
	r.PointerWheel(render.Point{X: 0, Y: -1}, pos, 0)

	if !upperDismissed {
		t.Fatal("upper popup's onDismiss did not fire on a Wheel landing in the lower popup — want it closed (chain-aware close-above generalizes to every action)")
	}
	if got := host.PopupCount(); got != 1 {
		t.Fatalf("PopupCount after wheel in the lower popup = %d, want 1 (only the lower popup remains)", got)
	}
	if got := lowerProbe.events; len(got) != 1 || got[0] != "wheel" {
		t.Fatalf("lowerProbe.events = %v, want [wheel] (forwarded into the containing popup)", got)
	}
}

// TestOverlayChainHoverMoveBetweenLevelsAutoCloses is the overlay-level
// regression for Phase 8 Task 1's (c) semantics: with a TWO-level popup
// stack, hovering the topmost popup behaves exactly as single-popup hover
// always did (Enter then Move, forwarded), but moving from there to a
// position inside the LOWER popup only — a DIFFERENT containing popup —
// auto-closes the (now stale) upper popup first, then synthesizes a fresh
// Enter+Move against the lower popup's own hover path. No Leave reaches
// upperProbe: it's detached/closed silently, matching ClosePopup's
// Detach-on-close convention (see popupHover's own doc comment), not
// delivered a Leave on its way out.
func TestOverlayChainHoverMoveBetweenLevelsAutoCloses(t *testing.T) {
	lowerProbe := &hoverProbe{}
	lowerProbe.SetWidth(60)
	lowerProbe.SetHeight(30)
	upperProbe := &hoverProbe{}
	upperProbe.SetWidth(40)
	upperProbe.SetHeight(20)

	host := NewOverlayHost()
	r := input.NewRouter()
	host.SetRouter(r)
	host.SetContent(NewFixed(400, 300, render.RGB(1, 2, 3)))
	r.SetRoot(host) // BEFORE ShowPopup: SetRoot itself resets any capture

	host.ShowPopup(lowerProbe, render.Rect{X: 10, Y: 10, W: 10, H: 10}, nil)
	host.ShowPopup(upperProbe, render.Rect{X: 200, Y: 10, W: 10, H: 10}, nil)
	layoutOverlay(host, 400, 300)

	// upperProbe bounds {200,20,40,20}; hover into it first — unaffected by
	// the chain-aware change (it's already the topmost/containing popup).
	posUpper := render.Point{X: 210, Y: 25}
	r.PointerMove(posUpper, 0)
	if got := upperProbe.events; len(got) != 2 || got[0] != "enter" || got[1] != "move" {
		t.Fatalf("upperProbe.events after hovering it = %v, want [enter move]", got)
	}
	if got := host.PopupCount(); got != 2 {
		t.Fatalf("PopupCount after hovering the topmost popup = %d, want 2 (unaffected)", got)
	}

	// lowerProbe bounds {10,20,60,30}; moving there finds a DIFFERENT
	// (lower) containing popup.
	posLower := render.Point{X: 30, Y: 30}
	r.PointerMove(posLower, 0)

	if got := host.PopupCount(); got != 1 {
		t.Fatalf("PopupCount after moving into the lower popup = %d, want 1 (upper auto-closed)", got)
	}
	if got := lowerProbe.events; len(got) != 2 || got[0] != "enter" || got[1] != "move" {
		t.Fatalf("lowerProbe.events after the cross-level Move = %v, want [enter move]", got)
	}
}

// ovKeyProbe is a minimal widget for OverlayHost.OnKey delegation tests: it
// counts every OnKey delivery (so a test can assert a key reached it exactly
// once — never zero, never twice) and optionally exposes a single child (so
// a test can build a focused-widget-under-content shape without pulling in
// a whole real container type).
type ovKeyProbe struct {
	core.Element

	child core.Widget
	keys  int
}

func (p *ovKeyProbe) OnKey(e *input.KeyEvent) {
	p.keys++
}

func (p *ovKeyProbe) Children() []core.Widget {
	if p.child == nil {
		return nil
	}
	return []core.Widget{p.child}
}

// TestOverlayDelegatesUnfocusedKeyToContent is the regression test for
// OverlayHost.OnKey: with NO widget in the tree focused,
// input.Router.dispatchKey would otherwise deliver a key event to the bare
// router root alone (see dispatchKey's doc comment) — which, for a tree
// rooted at OverlayHost, is the host itself, never content. Without OnKey's
// delegation, a window-level accelerator hosted on content (fluo-gallery's
// T-key theme toggle, for instance) would never fire from a pristine,
// nothing-yet-focused launch. Confirms delegation happens, and exactly once.
func TestOverlayDelegatesUnfocusedKeyToContent(t *testing.T) {
	host := NewOverlayHost()
	content := &ovKeyProbe{}
	host.SetContent(content)

	r := input.NewRouter()
	host.SetRouter(r)
	r.SetRoot(host)

	r.KeyDown(input.KeyT, 0, 0)

	if content.keys != 1 {
		t.Fatalf("content.keys = %d, want exactly 1 (unfocused delegation)", content.keys)
	}
}

// TestOverlayDoesNotDoubleDeliverFocusedKey is the companion regression: with
// some widget under content (not content itself) focused,
// input.Router.dispatchKey already bubbles the key event up the focused
// widget's own ancestor chain — content sits on that chain (it is the
// focused child's parent) and receives the event that way, before dispatchKey
// ever reaches the host. OverlayHost.OnKey's delegation must recognize this
// (via e.Router.Focused() != nil) and skip re-delivering to content, or
// content would see the very same key event twice.
func TestOverlayDoesNotDoubleDeliverFocusedKey(t *testing.T) {
	host := NewOverlayHost()
	content := &ovKeyProbe{}
	child := &ovProbe{focusable: true}
	content.child = child
	core.SetParent(child, content)
	host.SetContent(content)

	r := input.NewRouter()
	host.SetRouter(r)
	r.SetRoot(host)
	r.Focus(child)

	r.KeyDown(input.KeyT, 0, 0)

	if content.keys != 1 {
		t.Fatalf("content.keys = %d, want exactly 1 (bubbled once via the focused chain, not double-delivered by OnKey's delegation)", content.keys)
	}
}

// TestClampFRejectsNaN is clampF's half of the NaN guard both shared clamp
// helpers now carry (see TestClampScrollOffsetRejectsNaN): v < lo and
// v > hi are both false for NaN, so an unguarded clampF would hand a NaN
// straight back to the caller and place a popup at a nonsensical position.
// It must resolve to lo instead, and leave every ordinary value alone.
func TestClampFRejectsNaN(t *testing.T) {
	if got := clampF(float32(math.NaN()), 10, 100); got != 10 {
		t.Fatalf("clampF(NaN, 10, 100) = %v, want 10 (the low bound)", got)
	}
	// Inverted range: lo still wins for NaN, exactly as it does for any
	// other value once hi collapses onto lo.
	if got := clampF(float32(math.NaN()), 10, 5); got != 10 {
		t.Fatalf("clampF(NaN, 10, 5) = %v, want 10", got)
	}

	cases := []struct{ v, lo, hi, want float32 }{
		{50, 10, 100, 50},   // in range: unchanged
		{-5, 10, 100, 10},   // below the floor
		{500, 10, 100, 100}, // past the ceiling
		{50, 10, 5, 10},     // inverted range: lo wins
	}
	for _, c := range cases {
		if got := clampF(c.v, c.lo, c.hi); got != c.want {
			t.Errorf("clampF(%v, %v, %v) = %v, want %v", c.v, c.lo, c.hi, got, c.want)
		}
	}
}

// TestOverlaySetContentNilClearsContent pins the nil guard SetContent was
// missing: it detached the OUTGOING content behind a nil check, then handed
// the INCOMING one straight to core.SetParent, which dereferences its child
// argument — so clearing the content panicked, even though MeasureContent,
// ArrangeContent, Children and OnKey have always tolerated a host with none.
func TestOverlaySetContentNilClearsContent(t *testing.T) {
	content := NewFixed(40, 20, render.RGB(1, 2, 3))
	host := NewOverlayHost()
	host.SetContent(content)

	host.SetContent(nil)

	if got := host.Children(); len(got) != 0 {
		t.Fatalf("Children() after SetContent(nil) = %v, want empty", got)
	}
	if p := core.ParentOf(content); p != nil {
		t.Fatalf("detached content's parent = %v, want nil", p)
	}

	// A content-less host still measures, arranges and renders popups.
	popup := NewFixed(30, 15, render.RGB(4, 5, 6))
	host.ShowPopup(popup, render.Rect{X: 5, Y: 5}, nil)
	layoutOverlay(host, 200, 100)
	if got := host.Children(); len(got) != 1 || got[0] != core.Widget(popup) {
		t.Fatalf("Children() with no content and one popup = %v, want [popup]", got)
	}

	// Setting real content back afterward works normally.
	host.SetContent(content)
	if got := core.ParentOf(content); got != core.Widget(host) {
		t.Fatalf("re-set content's parent = %v, want host", got)
	}
}

// TestClosingStackedPopupRefocusesSurvivor is the FIX #20 regression: with two
// modal dialogs open, closing the top one (which strips focus from its own
// scrim via router.Detach) must hand focus to the dialog now beneath it, so
// Escape still reaches that dialog. Before the fix, focus went nil and stayed
// nil, so OverlayHost.OnKey delegated the next Escape to CONTENT and the
// surviving button-less dialog — whose only close path IS Escape — was stuck.
func TestClosingStackedPopupRefocusesSurvivor(t *testing.T) {
	host, r := newTestDialogHost(t)
	face := buttonFace(t)

	// Both button-less: Escape is the only way to close either one.
	var lower []DialogResult
	ShowDialog(host, face, DialogSpec{
		Title: "Lower", Body: "beneath",
		OnResult: func(res DialogResult) { lower = append(lower, res) },
	})
	var upper []DialogResult
	ShowDialog(host, face, DialogSpec{
		Title: "Upper", Body: "on top",
		OnResult: func(res DialogResult) { upper = append(upper, res) },
	})

	if got := host.PopupCount(); got != 2 {
		t.Fatalf("PopupCount = %d, want 2", got)
	}
	lowerScrim := host.popups[0].w
	upperScrim := host.popups[1].w
	if r.Focused() != upperScrim {
		t.Fatalf("Focused() = %v, want the upper dialog's scrim after it opened", r.Focused())
	}

	// Close the top dialog.
	r.KeyDown(input.KeyEscape, 0, 0)

	if len(upper) != 1 || upper[0] != DialogDismissed {
		t.Fatalf("upper results = %v, want [DialogDismissed]", upper)
	}
	if got := host.PopupCount(); got != 1 {
		t.Fatalf("PopupCount after first Esc = %d, want 1 (lower survives)", got)
	}
	if r.Focused() != lowerScrim {
		t.Fatalf("Focused() after closing the top dialog = %v, want the lower dialog's scrim (Esc must still reach it)", r.Focused())
	}

	// Escape must now close the survivor.
	r.KeyDown(input.KeyEscape, 0, 0)
	if len(lower) != 1 || lower[0] != DialogDismissed {
		t.Fatalf("lower results after second Esc = %v, want [DialogDismissed]", lower)
	}
	if got := host.PopupCount(); got != 0 {
		t.Fatalf("PopupCount after second Esc = %d, want 0", got)
	}
}
