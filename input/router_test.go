package input_test

import (
	"fmt"
	"testing"

	"github.com/0xdreadnaught/fluo/controls"
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
)

// probe: records events; configurable handled/focusable/cursor. Given
// verbatim by the task brief (fields + AcceptsFocus/Cursor/OnPointer/
// OnFocusChanged) so Task 4's keyboard/focus tests can extend this same file
// without rewriting it.
type probe struct {
	core.Element
	name      string
	events    []string // e.g. "press", "enter", "leave", "wheel", "key:9", "focus:true"
	handlePtr bool     // mark pointer events handled
	handleKey bool     // mark key events handled (OnKey's Handled, mirrors handlePtr)
	focusable bool
	cursor    input.Cursor
	router    *input.Router // set by the test; used by capturing and reclaim
	capturing bool

	// lastTarget records e.Target from the most recent OnPointer call, so
	// capture-delivery tests (TestReleaseUnderCapture, TestWheelUnderCapture)
	// can assert Target == the captured widget even when the event's
	// position falls outside that widget's bounds.
	lastTarget core.Widget

	// reclaim: if true, OnFocusChanged(false) reentrantly calls
	// router.Focus(p) to try to reclaim focus for itself (regression probe
	// for TestFocusReentrancyIgnored — the router's guard must ignore this).
	reclaim bool

	// child: NOT part of the brief's probe listing. core.Element alone is
	// always a leaf (Children() returns nil), but TestBubbleStopsAtHandled
	// and TestBubbleReachesParent need an actual parent-probe/child-probe
	// pair on the SAME hit-test path (HitPath only bubbles real ancestors,
	// never siblings — see hittest.go). This single-child forwarding is the
	// same override idiom every container in controls/ already uses
	// (Canvas, Border); it changes nothing about the fields/methods the
	// brief specifies, it only adds the minimum needed to nest one probe
	// inside another.
	child core.Widget
}

func (p *probe) AcceptsFocus() bool   { return p.focusable }
func (p *probe) Cursor() input.Cursor { return p.cursor }
func (p *probe) OnPointer(e *input.PointerEvent) {
	p.lastTarget = e.Target
	switch e.Action {
	case input.Press:
		p.events = append(p.events, "press")
		if p.capturing {
			e.Router.Capture(p)
		}
	case input.Release:
		p.events = append(p.events, "release")
	case input.Move:
		p.events = append(p.events, "move")
	case input.Enter:
		p.events = append(p.events, "enter")
	case input.Leave:
		p.events = append(p.events, "leave")
	case input.Wheel:
		p.events = append(p.events, "wheel")
	}
	if p.handlePtr {
		e.Handled = true
	}
}
func (p *probe) OnFocusChanged(f bool) {
	p.events = append(p.events, fmt.Sprintf("focus:%v", f))
	if !f && p.reclaim {
		p.router.Focus(p)
	}
}

// OnKey records "key" for every key event delivered to this probe, marking
// it Handled when handleKey is set (mirrors handlePtr's role for OnPointer).
func (p *probe) OnKey(e *input.KeyEvent) {
	p.events = append(p.events, "key")
	if p.handleKey {
		e.Handled = true
	}
}

// setChild makes p a single-child pass-through container: its child is
// measured with whatever space p is given, then arranged to fill p's own
// bounds exactly (so a point inside p is also inside its child, for
// nesting-dependent hit-test/bubble tests).
func (p *probe) setChild(w core.Widget) *probe {
	p.child = w
	core.SetParent(w, p)
	p.InvalidateMeasure()
	return p
}

func (p *probe) Children() []core.Widget {
	if p.child == nil {
		return nil
	}
	return []core.Widget{p.child}
}

func (p *probe) MeasureContent(available render.Size) render.Size {
	if p.child != nil {
		core.MeasureWidget(p.child, available)
	}
	return render.Size{}
}

func (p *probe) ArrangeContent(bounds render.Rect) {
	if p.child != nil {
		core.ArrangeWidget(p.child, bounds)
	}
}

func TestBubbleStopsAtHandled(t *testing.T) {
	leaf := &probe{name: "leaf", handlePtr: true}
	parent := (&probe{name: "parent"}).setChild(leaf)
	parent.SetWidth(50)
	parent.SetHeight(50)
	root := controls.NewCanvas().Add(parent, 0, 0)
	layout(root, 100, 100)

	r := input.NewRouter()
	r.SetRoot(root)
	r.PointerButton(input.ButtonLeft, true, render.Point{X: 10, Y: 10}, 0)

	if got := leaf.events; len(got) != 1 || got[0] != "press" {
		t.Fatalf("leaf.events = %v, want [press]", got)
	}
	if got := parent.events; len(got) != 0 {
		t.Fatalf("parent.events = %v, want none (leaf handled the press)", got)
	}
}

func TestBubbleReachesParent(t *testing.T) {
	leaf := &probe{name: "leaf"} // handlePtr=false
	parent := (&probe{name: "parent"}).setChild(leaf)
	parent.SetWidth(50)
	parent.SetHeight(50)
	root := controls.NewCanvas().Add(parent, 0, 0)
	layout(root, 100, 100)

	r := input.NewRouter()
	r.SetRoot(root)
	r.PointerButton(input.ButtonLeft, true, render.Point{X: 10, Y: 10}, 0)

	// Both fire; leaf-first is inherent to the leaf->root dispatch order
	// (already proven directly by TestBubbleStopsAtHandled: if parent ran
	// before leaf, leaf's later Handled=true couldn't have stopped parent
	// from receiving the event, but it does).
	if got := leaf.events; len(got) != 1 || got[0] != "press" {
		t.Fatalf("leaf.events = %v, want [press]", got)
	}
	if got := parent.events; len(got) != 1 || got[0] != "press" {
		t.Fatalf("parent.events = %v, want [press]", got)
	}
}

func TestHoverEnterLeave(t *testing.T) {
	a := &probe{name: "a"}
	a.SetWidth(50)
	a.SetHeight(50)
	b := &probe{name: "b"}
	b.SetWidth(50)
	b.SetHeight(50)
	root := controls.NewCanvas().Add(a, 0, 0).Add(b, 60, 0)
	layout(root, 200, 100)

	r := input.NewRouter()
	r.SetRoot(root)

	// PointerMove both diffs hover (direct Enter/Leave) AND bubbles a plain
	// Move down the same path (spec point 1/5: Move bubbles like Press/
	// Release/Wheel; only Enter/Leave themselves are direct-only), so each
	// move adds an "enter"/"leave" AND a "move" to whichever widgets are on
	// the resulting path.
	r.PointerMove(render.Point{X: 10, Y: 10}, 0) // over A
	if got := a.events; len(got) != 2 || got[0] != "enter" || got[1] != "move" {
		t.Fatalf("a.events after move-over-A = %v, want [enter move]", got)
	}
	if got := b.events; len(got) != 0 {
		t.Fatalf("b.events after move-over-A = %v, want none", got)
	}

	r.PointerMove(render.Point{X: 70, Y: 10}, 0) // over B
	if got := a.events; len(got) != 3 || got[2] != "leave" {
		t.Fatalf("a.events after move-over-B = %v, want [enter move leave]", got)
	}
	if got := b.events; len(got) != 2 || got[0] != "enter" || got[1] != "move" {
		t.Fatalf("b.events after move-over-B = %v, want [enter move]", got)
	}

	r.PointerMove(render.Point{X: 80, Y: 10}, 0) // still over B
	if got := a.events; len(got) != 3 {
		t.Fatalf("a.events after move-within-B = %v, want unchanged [enter move leave]", got)
	}
	if got := b.events; len(got) != 3 || got[2] != "move" {
		t.Fatalf("b.events after move-within-B = %v, want [enter move move] (no re-enter)", got)
	}
}

func TestCursorFromPath(t *testing.T) {
	p := &probe{name: "hand", cursor: input.CursorHand}
	p.SetWidth(50)
	p.SetHeight(50)
	root := controls.NewCanvas().Add(p, 0, 0)
	layout(root, 100, 100)

	r := input.NewRouter()
	r.SetRoot(root)

	if c := r.PointerMove(render.Point{X: 10, Y: 10}, 0); c != input.CursorHand {
		t.Fatalf("cursor over probe = %v, want CursorHand", c)
	}
	if c := r.PointerMove(render.Point{X: 90, Y: 90}, 0); c != input.CursorArrow {
		t.Fatalf("cursor over empty canvas = %v, want CursorArrow", c)
	}
}

func TestCaptureRoutesAll(t *testing.T) {
	a := &probe{name: "a", capturing: true}
	a.SetWidth(50)
	a.SetHeight(50)
	b := &probe{name: "b"}
	b.SetWidth(50)
	b.SetHeight(50)
	root := controls.NewCanvas().Add(a, 0, 0).Add(b, 60, 0)
	layout(root, 200, 100)

	r := input.NewRouter()
	r.SetRoot(root)

	r.PointerButton(input.ButtonLeft, true, render.Point{X: 10, Y: 10}, 0) // press on A -> captures
	if r.Captured() != core.Widget(a) {
		t.Fatalf("Captured() = %v, want a", r.Captured())
	}

	// Move to a point OUTSIDE a's bounds (in fact inside b's bounds): a
	// still gets "move" (capture bypasses hit-testing), and hover is
	// suppressed everywhere (b gets no "enter", a gets no "enter"/"leave").
	r.PointerMove(render.Point{X: 70, Y: 10}, 0)
	if got := a.events; len(got) != 2 || got[1] != "move" {
		t.Fatalf("a.events after captured move = %v, want [press move]", got)
	}
	if got := b.events; len(got) != 0 {
		t.Fatalf("b.events during capture = %v, want none (hover suppressed)", got)
	}

	r.Release()
	if r.Captured() != nil {
		t.Fatalf("Captured() after Release() = %v, want nil", r.Captured())
	}

	// Hover resumes: moving within b now produces a normal enter (plus the
	// Move itself, which bubbles independently of the hover diff).
	r.PointerMove(render.Point{X: 70, Y: 10}, 0)
	if got := b.events; len(got) != 2 || got[0] != "enter" || got[1] != "move" {
		t.Fatalf("b.events after release+move = %v, want [enter move]", got)
	}
}

func TestReleaseUnderCapture(t *testing.T) {
	a := &probe{name: "a", capturing: true}
	a.SetWidth(50)
	a.SetHeight(50)
	b := &probe{name: "b"}
	b.SetWidth(50)
	b.SetHeight(50)
	root := controls.NewCanvas().Add(a, 0, 0).Add(b, 60, 0)
	layout(root, 200, 100)

	r := input.NewRouter()
	r.SetRoot(root)

	r.PointerButton(input.ButtonLeft, true, render.Point{X: 10, Y: 10}, 0) // press on a -> captures
	if r.Captured() != core.Widget(a) {
		t.Fatalf("Captured() = %v, want a", r.Captured())
	}

	// Release at a point OUTSIDE a's bounds (in fact inside b's): still goes
	// only to the captured widget (a), with Target == a, and b sees nothing.
	r.PointerButton(input.ButtonLeft, false, render.Point{X: 70, Y: 10}, 0)

	if got := a.events; len(got) != 2 || got[1] != "release" {
		t.Fatalf("a.events = %v, want [press release]", got)
	}
	if a.lastTarget != core.Widget(a) {
		t.Fatalf("release Target = %v, want a", a.lastTarget)
	}
	if got := b.events; len(got) != 0 {
		t.Fatalf("b.events = %v, want none (release routed to captured widget only)", got)
	}
}

func TestWheelUnderCapture(t *testing.T) {
	a := &probe{name: "a", capturing: true}
	a.SetWidth(50)
	a.SetHeight(50)
	b := &probe{name: "b"}
	b.SetWidth(50)
	b.SetHeight(50)
	root := controls.NewCanvas().Add(a, 0, 0).Add(b, 60, 0)
	layout(root, 200, 100)

	r := input.NewRouter()
	r.SetRoot(root)

	r.PointerButton(input.ButtonLeft, true, render.Point{X: 10, Y: 10}, 0) // press on a -> captures
	if r.Captured() != core.Widget(a) {
		t.Fatalf("Captured() = %v, want a", r.Captured())
	}

	// Wheel at a point OUTSIDE a's bounds (in fact inside b's): still goes
	// only to the captured widget (a), with Target == a, and b sees nothing.
	r.PointerWheel(render.Point{X: 0, Y: 1}, render.Point{X: 70, Y: 10}, 0)

	if got := a.events; len(got) != 2 || got[1] != "wheel" {
		t.Fatalf("a.events = %v, want [press wheel]", got)
	}
	if a.lastTarget != core.Widget(a) {
		t.Fatalf("wheel Target = %v, want a", a.lastTarget)
	}
	if got := b.events; len(got) != 0 {
		t.Fatalf("b.events = %v, want none (wheel routed to captured widget only)", got)
	}
}

// TestNestedCaptureRestores is the primary regression for Capture/Release's
// nesting contract: capturing while another widget already holds the grab
// pushes over it, and Release pops back to that previous captor rather than
// clearing capture outright. This is what lets a popup-internal drag (e.g.
// a ScrollViewer thumb) release cleanly back into an OverlayHost's own modal
// capture instead of dropping it.
// TestCaptureTopIdempotent is the regression for the multi-popup capture
// leak: Capture(a) called twice in a row (a already IS the current top)
// must not push a second stack entry — otherwise a single Release would
// leave a's own capture still active, wedging dispatch. A genuinely
// different widget still nests normally (covered by
// TestNestedCaptureRestores).
func TestCaptureTopIdempotent(t *testing.T) {
	a := &probe{name: "a"}

	r := input.NewRouter()

	r.Capture(a)
	r.Capture(a) // re-asserting the same top must be a no-op, not a second push

	r.Release()
	if got := r.Captured(); got != nil {
		t.Fatalf("Captured() after one Release() following two Capture(a) calls = %v, want nil (second Capture(a) must not have pushed)", got)
	}
}

func TestNestedCaptureRestores(t *testing.T) {
	a := &probe{name: "a"}
	b := &probe{name: "b"}

	r := input.NewRouter()

	r.Capture(a)
	if r.Captured() != core.Widget(a) {
		t.Fatalf("Captured() after Capture(a) = %v, want a", r.Captured())
	}

	r.Capture(b)
	if r.Captured() != core.Widget(b) {
		t.Fatalf("Captured() after Capture(b) = %v, want b (nested over a)", r.Captured())
	}

	r.Release()
	if r.Captured() != core.Widget(a) {
		t.Fatalf("Captured() after inner Release() = %v, want a (restored, not nil)", r.Captured())
	}

	r.Release()
	if r.Captured() != nil {
		t.Fatalf("Captured() after outer Release() = %v, want nil", r.Captured())
	}
}

// TestDetachClearsCaptureStack proves Detach filters the ENTIRE capture
// stack, not just the top: an outer capture (host) survives the detach of
// an unrelated inner one, and becomes the active capture again once the
// inner entry is removed — exactly as if the inner widget's own Release had
// run, which matters because a torn-down popup can't be trusted to run its
// own cleanup.
func TestDetachClearsCaptureStack(t *testing.T) {
	host := &probe{name: "host"}
	inner := &probe{name: "inner"}

	r := input.NewRouter()
	r.Capture(host)
	r.Capture(inner)

	if r.Captured() != core.Widget(inner) {
		t.Fatalf("Captured() before Detach = %v, want inner", r.Captured())
	}

	r.Detach(inner)

	if r.Captured() != core.Widget(host) {
		t.Fatalf("Captured() after Detach(inner) = %v, want host (surviving outer capture)", r.Captured())
	}
}

func TestWheelBubbles(t *testing.T) {
	leaf := &probe{name: "leaf"}
	leaf.SetWidth(50)
	leaf.SetHeight(50)
	root := controls.NewCanvas().Add(leaf, 0, 0)
	layout(root, 100, 100)

	r := input.NewRouter()
	r.SetRoot(root)
	r.PointerWheel(render.Point{X: 0, Y: 1}, render.Point{X: 10, Y: 10}, 0)

	if got := leaf.events; len(got) != 1 || got[0] != "wheel" {
		t.Fatalf("leaf.events = %v, want [wheel]", got)
	}
}

func TestClickFocusRetainedOnNonFocusable(t *testing.T) {
	p := &probe{name: "p", focusable: true}
	p.SetWidth(50)
	p.SetHeight(50)
	root := controls.NewCanvas().Add(p, 0, 0)
	layout(root, 100, 100)

	r := input.NewRouter()
	r.SetRoot(root)

	r.PointerButton(input.ButtonLeft, true, render.Point{X: 10, Y: 10}, 0) // press on p
	if r.Focused() != core.Widget(p) {
		t.Fatalf("Focused() = %v, want p", r.Focused())
	}
	if got := p.events; len(got) != 2 || got[0] != "focus:true" || got[1] != "press" {
		t.Fatalf("p.events = %v, want [focus:true press]", got)
	}

	// Pressing on empty canvas (no Focusable on the hit path) RETAINS focus —
	// it neither clears it nor fires OnFocusChanged on p.
	r.PointerButton(input.ButtonLeft, true, render.Point{X: 90, Y: 90}, 0) // press on empty canvas
	if r.Focused() != core.Widget(p) {
		t.Fatalf("Focused() after empty-space click = %v, want p (retained)", r.Focused())
	}
	if got := p.events; len(got) != 2 {
		t.Fatalf("p.events after empty-space click = %v, want unchanged [focus:true press] (no focus:false)", got)
	}

	// The programmatic path, Focus(nil), still clears focus explicitly.
	r.Focus(nil)
	if r.Focused() != nil {
		t.Fatalf("Focused() after Focus(nil) = %v, want nil", r.Focused())
	}
	if got := p.events; len(got) != 3 || got[2] != "focus:false" {
		t.Fatalf("p.events after Focus(nil) = %v, want [focus:true press focus:false]", got)
	}
}

// TestSetRootResetsState is a regression test for SetRoot's state-hygiene
// contract: swapping the root widget tree must not leave hover, capture, or
// focus pointing at widgets from the tree that's being replaced. Without the
// reset, a widget from the old tree could keep the pointer grab or keyboard
// focus indefinitely (nothing in the new tree can ever release/blur it), and
// a stale hover path would fire a spurious Leave for it on the very next
// PointerMove over the new tree.
func TestSetRootResetsState(t *testing.T) {
	a := &probe{name: "a", focusable: true, capturing: true}
	a.SetWidth(50)
	a.SetHeight(50)
	rootA := controls.NewCanvas().Add(a, 0, 0)
	layout(rootA, 100, 100)

	rootB := controls.NewCanvas() // a disjoint tree with nothing at (10,10)
	layout(rootB, 100, 100)

	r := input.NewRouter()
	r.SetRoot(rootA)

	r.PointerMove(render.Point{X: 10, Y: 10}, 0)                           // hover over a: enter+move
	r.PointerButton(input.ButtonLeft, true, render.Point{X: 10, Y: 10}, 0) // focus a, and (capturing) capture a
	if r.Focused() != core.Widget(a) {
		t.Fatalf("Focused() = %v, want a", r.Focused())
	}
	if r.Captured() != core.Widget(a) {
		t.Fatalf("Captured() = %v, want a", r.Captured())
	}

	before := len(a.events)

	r.SetRoot(rootB)
	if r.Focused() != nil {
		t.Fatalf("Focused() after SetRoot(treeB) = %v, want nil", r.Focused())
	}
	if r.Captured() != nil {
		t.Fatalf("Captured() after SetRoot(treeB) = %v, want nil", r.Captured())
	}
	// Focus is cleared via the normal Focus(nil) path, so a still sees its
	// OnFocusChanged(false) notification.
	if got := a.events; len(got) != before+1 || got[len(got)-1] != "focus:false" {
		t.Fatalf("a.events after SetRoot(treeB) = %v, want exactly one more [focus:false] appended", got)
	}

	afterReset := len(a.events)

	// A PointerMove over treeB at the same coordinates a used to occupy must
	// NOT fire a Leave on a: hover was reset outright, not diffed against the
	// (now unreachable) tree-A path.
	r.PointerMove(render.Point{X: 10, Y: 10}, 0)
	for _, e := range a.events[afterReset:] {
		if e == "leave" {
			t.Fatalf("a.events after post-SetRoot move = %v, want no \"leave\" for tree-A widgets", a.events)
		}
	}
}

func TestKeyGoesToFocused(t *testing.T) {
	a := &probe{name: "a", focusable: true}
	a.SetWidth(50)
	a.SetHeight(50)
	b := &probe{name: "b", focusable: true}
	b.SetWidth(50)
	b.SetHeight(50)
	root := controls.NewCanvas().Add(a, 0, 0).Add(b, 60, 0)
	layout(root, 200, 100)

	r := input.NewRouter()
	r.SetRoot(root)

	r.PointerButton(input.ButtonLeft, true, render.Point{X: 10, Y: 10}, 0) // focus a via click
	r.KeyDown(input.KeyEnter, 0, 0)

	if got := a.events; len(got) != 3 || got[2] != "key" {
		t.Fatalf("a.events = %v, want [focus:true press key]", got)
	}
	if got := b.events; len(got) != 0 {
		t.Fatalf("b.events = %v, want none (b is not focused)", got)
	}
}

func TestTabCycles(t *testing.T) {
	a := &probe{name: "a", focusable: true}
	a.SetWidth(50)
	a.SetHeight(50)
	b := &probe{name: "b", focusable: true}
	b.SetWidth(50)
	b.SetHeight(50)
	c := &probe{name: "c", focusable: true}
	c.SetWidth(50)
	c.SetHeight(50)
	root := controls.NewCanvas().Add(a, 0, 0).Add(b, 60, 0).Add(c, 120, 0)
	layout(root, 200, 100)

	r := input.NewRouter()
	r.SetRoot(root)

	r.PointerButton(input.ButtonLeft, true, render.Point{X: 10, Y: 10}, 0) // click a
	if r.Focused() != core.Widget(a) {
		t.Fatalf("Focused() = %v, want a", r.Focused())
	}

	r.KeyDown(input.KeyTab, 0, 0)
	if r.Focused() != core.Widget(b) {
		t.Fatalf("Focused() after Tab = %v, want b", r.Focused())
	}

	r.KeyDown(input.KeyTab, 0, 0)
	if r.Focused() != core.Widget(c) {
		t.Fatalf("Focused() after Tab = %v, want c", r.Focused())
	}

	r.KeyDown(input.KeyTab, 0, 0) // wraps
	if r.Focused() != core.Widget(a) {
		t.Fatalf("Focused() after wrapping Tab = %v, want a", r.Focused())
	}

	r.KeyDown(input.KeyTab, 0, input.ModShift) // shift+tab steps backward
	if r.Focused() != core.Widget(c) {
		t.Fatalf("Focused() after Shift+Tab = %v, want c", r.Focused())
	}
}

func TestKeyBubbles(t *testing.T) {
	leaf := &probe{name: "leaf", focusable: true}                      // accepts focus, doesn't handle keys
	parent := (&probe{name: "parent", handleKey: true}).setChild(leaf) // handles keys, doesn't accept focus
	parent.SetWidth(50)
	parent.SetHeight(50)
	root := controls.NewCanvas().Add(parent, 0, 0)
	layout(root, 100, 100)

	r := input.NewRouter()
	r.SetRoot(root)

	r.PointerButton(input.ButtonLeft, true, render.Point{X: 10, Y: 10}, 0) // focuses leaf (only focusable on path)
	if r.Focused() != core.Widget(leaf) {
		t.Fatalf("Focused() = %v, want leaf", r.Focused())
	}

	r.KeyDown(input.KeyEnter, 0, 0)

	if got := leaf.events; len(got) != 3 || got[2] != "key" {
		t.Fatalf("leaf.events = %v, want [focus:true press key]", got)
	}
	if got := parent.events; len(got) != 2 || got[0] != "press" || got[1] != "key" {
		t.Fatalf("parent.events = %v, want [press key] (leaf didn't handle, so it bubbled to parent)", got)
	}
}

func TestFocusReentrancyIgnored(t *testing.T) {
	a := &probe{name: "a", focusable: true, reclaim: true}
	a.SetWidth(50)
	a.SetHeight(50)
	b := &probe{name: "b", focusable: true}
	b.SetWidth(50)
	b.SetHeight(50)
	root := controls.NewCanvas().Add(a, 0, 0).Add(b, 60, 0)
	layout(root, 200, 100)

	r := input.NewRouter()
	r.SetRoot(root)
	a.router = r

	r.Focus(a)
	if r.Focused() != core.Widget(a) {
		t.Fatalf("Focused() = %v, want a", r.Focused())
	}

	// Focusing b blurs a. a's OnFocusChanged(false) reentrantly calls
	// r.Focus(a), trying to reclaim focus for itself from inside the
	// callback. The router's reentrancy guard must ignore that inner call —
	// without it, this would either stack-overflow (two widgets fighting
	// over focus) or leave focus back on a instead of the widget the caller
	// actually asked for.
	r.Focus(b)
	if r.Focused() != core.Widget(b) {
		t.Fatalf("Focused() after reentrant reclaim attempt = %v, want b (reentrant Focus must be ignored)", r.Focused())
	}
}

// Focus left behind in a subtree that has since been hidden must not keep
// receiving keystrokes: a TabControl switching tabs, an Expander collapsing,
// or a plain SetVisible(false) all hide a container without telling the
// router, and the focused widget inside is then invisible but still typed
// into. Note the ancestor walk is what matters here — leaf itself is never
// hidden, only the container above it.
func TestKeyNotDeliveredIntoHiddenSubtree(t *testing.T) {
	leaf := &probe{name: "leaf", focusable: true, handleKey: true}
	leaf.SetWidth(50)
	leaf.SetHeight(50)
	sub := (&probe{name: "sub"}).setChild(leaf)
	sub.SetWidth(50)
	sub.SetHeight(50)
	root := (&probe{name: "root"}).setChild(sub)
	layout(root, 100, 100)

	r := input.NewRouter()
	r.SetRoot(root)
	r.Focus(leaf)

	// Normal visible-focus dispatch is untouched: the key reaches leaf.
	before := len(leaf.events)
	if consumed := r.KeyDown(input.KeyA, 'a', 0); !consumed {
		t.Fatal("KeyDown with visible focus = false, want true (leaf handles it)")
	}
	if got := leaf.events[before:]; len(got) != 1 || got[0] != "key" {
		t.Fatalf("leaf.events after visible KeyDown = %v, want exactly [key]", got)
	}

	// Hide the ANCESTOR, the way setTabContentVisible does for a deselected
	// tab's content. leaf's own visible flag stays true.
	sub.SetVisible(false)
	if !core.IsVisible(leaf) {
		t.Fatal("leaf is hidden itself, want visible (the test must exercise the ancestor walk)")
	}

	before = len(leaf.events)
	rootBefore := len(root.events)
	if consumed := r.KeyDown(input.KeyA, 'a', 0); consumed {
		t.Fatal("KeyDown into hidden subtree = true, want false (nothing focused took it)")
	}

	if got := leaf.events[before:]; len(got) != 1 || got[0] != "focus:false" {
		t.Fatalf("leaf.events after hidden KeyDown = %v, want exactly [focus:false] (no key, blur fired)", got)
	}
	if r.Focused() != nil {
		t.Fatalf("Focused() after hidden KeyDown = %v, want nil", r.Focused())
	}
	// With focus cleared, the key goes to the root, exactly as it would have
	// had nothing been focused in the first place.
	if got := root.events[rootBefore:]; len(got) != 1 || got[0] != "key" {
		t.Fatalf("root.events after hidden KeyDown = %v, want exactly [key]", got)
	}
}

// Re-showing the subtree does not resurrect the cleared focus (it was cleared
// for real, not merely suppressed), and focusing back into the now-visible
// subtree delivers keys again.
func TestKeyDeliveryResumesAfterSubtreeReshown(t *testing.T) {
	leaf := &probe{name: "leaf", focusable: true, handleKey: true}
	leaf.SetWidth(50)
	leaf.SetHeight(50)
	sub := (&probe{name: "sub"}).setChild(leaf)
	sub.SetWidth(50)
	sub.SetHeight(50)
	root := (&probe{name: "root"}).setChild(sub)
	layout(root, 100, 100)

	r := input.NewRouter()
	r.SetRoot(root)
	r.Focus(leaf)

	sub.SetVisible(false)
	r.KeyDown(input.KeyA, 'a', 0) // clears focus

	sub.SetVisible(true)
	if r.Focused() != nil {
		t.Fatalf("Focused() after re-showing = %v, want nil (focus was cleared, not suspended)", r.Focused())
	}

	r.Focus(leaf)
	before := len(leaf.events)
	if consumed := r.KeyDown(input.KeyA, 'a', 0); !consumed {
		t.Fatal("KeyDown after re-focusing a visible leaf = false, want true")
	}
	if got := leaf.events[before:]; len(got) != 1 || got[0] != "key" {
		t.Fatalf("leaf.events after re-focus KeyDown = %v, want exactly [key]", got)
	}
}

// TestDetachClearsSubtreeState is the primary regression test for Detach:
// hover, capture, and focus all point at a leaf buried inside the subtree
// being detached, and Detach must clear all three. Focus is the one that
// fires a notification on the way out (OnFocusChanged(false), same as
// Focus(nil)); capture and hover are cleared silently — proven here by
// checking that no further events land on leaf beyond that one "focus:false".
func TestDetachClearsSubtreeState(t *testing.T) {
	leaf := &probe{name: "leaf", focusable: true, capturing: true}
	leaf.SetWidth(50)
	leaf.SetHeight(50)
	sub := (&probe{name: "sub"}).setChild(leaf)
	sub.SetWidth(50)
	sub.SetHeight(50)
	root := controls.NewCanvas().Add(sub, 0, 0)
	layout(root, 100, 100)

	r := input.NewRouter()
	r.SetRoot(root)

	r.PointerMove(render.Point{X: 10, Y: 10}, 0)                           // hover leaf
	r.PointerButton(input.ButtonLeft, true, render.Point{X: 10, Y: 10}, 0) // focus + capture leaf

	if r.Focused() != core.Widget(leaf) {
		t.Fatalf("Focused() = %v, want leaf", r.Focused())
	}
	if r.Captured() != core.Widget(leaf) {
		t.Fatalf("Captured() = %v, want leaf", r.Captured())
	}

	before := len(leaf.events)

	r.Detach(sub) // detach the subtree root, not leaf itself

	if r.Focused() != nil {
		t.Fatalf("Focused() after Detach = %v, want nil", r.Focused())
	}
	if r.Captured() != nil {
		t.Fatalf("Captured() after Detach = %v, want nil", r.Captured())
	}
	if got := leaf.events[before:]; len(got) != 1 || got[0] != "focus:false" {
		t.Fatalf("leaf.events after Detach = %v, want exactly [focus:false]", got)
	}

	afterDetach := len(leaf.events)

	// Hover was cleared silently (no "leave" fired by Detach itself), and
	// leaf is no longer tracked as hovered at all, so a move elsewhere must
	// not produce a spurious "leave" for it.
	r.PointerMove(render.Point{X: 90, Y: 90}, 0)
	for _, e := range leaf.events[afterDetach:] {
		if e == "leave" {
			t.Fatalf("leaf.events after post-Detach move = %v, want no \"leave\"", leaf.events)
		}
	}
}

// TestDetachUnrelatedUntouched is a regression test for Detach's subtree
// scoping: state (focus/capture/hover) that points at a widget OUTSIDE the
// detached subtree must survive untouched — no events, no clearing.
func TestDetachUnrelatedUntouched(t *testing.T) {
	other := &probe{name: "other", focusable: true, capturing: true}
	other.SetWidth(50)
	other.SetHeight(50)
	leaf := &probe{name: "leaf"}
	leaf.SetWidth(50)
	leaf.SetHeight(50)
	sub := (&probe{name: "sub"}).setChild(leaf)
	sub.SetWidth(50)
	sub.SetHeight(50)
	root := controls.NewCanvas().Add(other, 0, 0).Add(sub, 60, 0)
	layout(root, 200, 100)

	r := input.NewRouter()
	r.SetRoot(root)

	r.PointerMove(render.Point{X: 10, Y: 10}, 0)                           // hover other
	r.PointerButton(input.ButtonLeft, true, render.Point{X: 10, Y: 10}, 0) // focus + capture other

	if r.Focused() != core.Widget(other) {
		t.Fatalf("Focused() = %v, want other", r.Focused())
	}
	if r.Captured() != core.Widget(other) {
		t.Fatalf("Captured() = %v, want other", r.Captured())
	}

	before := len(other.events)

	r.Detach(sub) // unrelated subtree; other is a sibling, not inside it

	if r.Focused() != core.Widget(other) {
		t.Fatalf("Focused() after Detach(sub) = %v, want other (untouched)", r.Focused())
	}
	if r.Captured() != core.Widget(other) {
		t.Fatalf("Captured() after Detach(sub) = %v, want other (untouched)", r.Captured())
	}
	if got := other.events[before:]; len(got) != 0 {
		t.Fatalf("other.events after Detach(sub) = %v, want none", got)
	}
}

// TestDetachNilIsNoop is a one-liner regression for Detach's nil guard: a
// nil w (e.g. a caller passing a not-yet-set popup reference) must return
// immediately rather than panicking on the nil core.Widget interface value,
// and must leave all router state untouched.
func TestDetachNilIsNoop(t *testing.T) {
	other := &probe{name: "other", focusable: true}
	other.SetWidth(50)
	other.SetHeight(50)
	layout(other, 50, 50)

	r := input.NewRouter()
	r.SetRoot(other)
	r.Focus(other)

	r.Detach(nil) // must not panic

	if r.Focused() != core.Widget(other) {
		t.Fatalf("Focused() after Detach(nil) = %v, want other (untouched)", r.Focused())
	}
}

// fakeClipboard is a minimal input.Clipboard for TestClipboardAccessors: a
// single in-memory string, no host/OS involvement.
type fakeClipboard struct{ text string }

func (f *fakeClipboard) Get() string  { return f.text }
func (f *fakeClipboard) Set(s string) { f.text = s }

func TestClipboardAccessors(t *testing.T) {
	r := input.NewRouter()
	if r.Clipboard() != nil {
		t.Fatalf("Clipboard() on fresh router = %v, want nil (headless default)", r.Clipboard())
	}

	fc := &fakeClipboard{text: "hello"}
	r.SetClipboard(fc)

	if r.Clipboard() != input.Clipboard(fc) {
		t.Fatalf("Clipboard() = %v, want fc", r.Clipboard())
	}
	if got := r.Clipboard().Get(); got != "hello" {
		t.Fatalf("Clipboard().Get() = %q, want %q", got, "hello")
	}

	r.Clipboard().Set("world")
	if fc.text != "world" {
		t.Fatalf("fc.text after Clipboard().Set = %q, want %q (roundtrip through the router)", fc.text, "world")
	}
}

// TestNilRootSafe is a regression test for a panic found in the live app
// host: glfw callbacks are wired (and can already fire, e.g. an OS-buffered
// event replayed on the first PollEvents) before the frame callback ever
// gets a chance to call router.SetRoot(root). Every dispatch entry point
// must tolerate a Router with no root (and no capture) — and a bare
// HitPath(nil, ...) call, which any host could make directly — without
// touching the nil core.Widget interface (calling any method on it, even
// indirectly via core.IsVisible/BoundsOf, panics).
func TestNilRootSafe(t *testing.T) {
	if p := input.HitPath(nil, render.Point{X: 5, Y: 5}); p != nil {
		t.Fatalf("HitPath(nil, ...) = %v, want nil", p)
	}

	r := input.NewRouter() // no SetRoot call

	if cur := r.PointerMove(render.Point{X: 1, Y: 1}, 0); cur != input.CursorArrow {
		t.Fatalf("PointerMove with no root: cursor = %v, want CursorArrow", cur)
	}
	r.PointerButton(input.ButtonLeft, true, render.Point{X: 1, Y: 1}, 0)
	r.PointerButton(input.ButtonLeft, false, render.Point{X: 1, Y: 1}, 0)
	r.PointerWheel(render.Point{X: 0, Y: 1}, render.Point{X: 1, Y: 1}, 0)
	r.KeyDown(input.KeyEnter, 0, 0)
	r.KeyDown(input.KeyTab, 0, 0) // unhandled Tab reaches FocusNext's rootless no-op
	r.KeyUp(input.KeyEnter, 0)
	// Reaching here without a panic is the assertion; nothing above has an
	// observable effect on a rootless router beyond the cursor check.
}

// TestPointerButtonConsumed covers PointerButton's consumed return across
// its three dispatch branches: a press that lands on a widget that marks
// itself handled, a press over empty space (hits nothing, so nothing can
// handle it), and a press delivered under an active capture (always
// consumed — the pointer was already claimed exclusively by a fluo widget).
func TestPointerButtonConsumed(t *testing.T) {
	handling := &probe{name: "handling", handlePtr: true}
	handling.SetWidth(50)
	handling.SetHeight(50)
	root := controls.NewCanvas().Add(handling, 0, 0)
	layout(root, 100, 100)

	r := input.NewRouter()
	r.SetRoot(root)

	if consumed := r.PointerButton(input.ButtonLeft, true, render.Point{X: 10, Y: 10}, 0); !consumed {
		t.Fatalf("PointerButton over a handling widget: consumed = false, want true")
	}
	if consumed := r.PointerButton(input.ButtonLeft, true, render.Point{X: 90, Y: 90}, 0); consumed {
		t.Fatalf("PointerButton over empty space: consumed = true, want false")
	}
}

// TestPointerButtonConsumedNotHandled covers the case a press's hit-path
// reaches a PointerHandler that chooses not to mark the event handled
// (probe with handlePtr: false): consumed must track e.Handled, not mere
// hit-path presence, so this is false even though the pointer is over an
// interactive widget.
func TestPointerButtonConsumedNotHandled(t *testing.T) {
	notHandling := &probe{name: "notHandling"} // handlePtr defaults to false
	notHandling.SetWidth(50)
	notHandling.SetHeight(50)
	root := controls.NewCanvas().Add(notHandling, 0, 0)
	layout(root, 100, 100)

	r := input.NewRouter()
	r.SetRoot(root)

	if consumed := r.PointerButton(input.ButtonLeft, true, render.Point{X: 10, Y: 10}, 0); consumed {
		t.Fatalf("PointerButton over a non-handling widget: consumed = true, want false")
	}
}

// TestPointerButtonConsumedUnderCapture covers the captured-delivery branch:
// consumed is unconditionally true while a capture is active, since the
// pointer was already claimed exclusively by a fluo widget regardless of
// whether that widget marks this particular delivery handled.
func TestPointerButtonConsumedUnderCapture(t *testing.T) {
	a := &probe{name: "a", capturing: true} // captures on press, doesn't set handlePtr
	a.SetWidth(50)
	a.SetHeight(50)
	root := controls.NewCanvas().Add(a, 0, 0)
	layout(root, 100, 100)

	r := input.NewRouter()
	r.SetRoot(root)

	r.PointerButton(input.ButtonLeft, true, render.Point{X: 10, Y: 10}, 0) // captures
	if r.Captured() != core.Widget(a) {
		t.Fatalf("Captured() = %v, want a", r.Captured())
	}
	if consumed := r.PointerButton(input.ButtonLeft, false, render.Point{X: 10, Y: 10}, 0); !consumed {
		t.Fatalf("PointerButton delivered under capture: consumed = false, want true")
	}
}

// TestKeyDownConsumed covers KeyDown's consumed return: true when the
// currently focused widget's focus-anchored dispatch (the focused widget
// itself, or an ancestor it bubbles to) sets e.Handled, false when nothing
// is focused at all — even if the rootless/root-only fallback in
// dispatchKey happens to mark the event handled, since no fluo widget holds
// focus in that case.
func TestKeyDownConsumed(t *testing.T) {
	a := &probe{name: "a", focusable: true, handleKey: true}
	a.SetWidth(50)
	a.SetHeight(50)
	root := controls.NewCanvas().Add(a, 0, 0)
	layout(root, 100, 100)

	r := input.NewRouter()
	r.SetRoot(root)

	r.PointerButton(input.ButtonLeft, true, render.Point{X: 10, Y: 10}, 0) // focuses a
	if r.Focused() != core.Widget(a) {
		t.Fatalf("Focused() = %v, want a", r.Focused())
	}
	if consumed := r.KeyDown(input.KeyEnter, 0, 0); !consumed {
		t.Fatalf("KeyDown to a focused, handling widget: consumed = false, want true")
	}

	r.Focus(nil)
	if consumed := r.KeyDown(input.KeyEnter, 0, 0); consumed {
		t.Fatalf("KeyDown with nothing focused: consumed = true, want false")
	}
}

// TestKeyDownConsumedTabBookkeeping covers Tab/Shift+Tab: the router's own
// focus-cycling bookkeeping (KeyDown's tail, run when nothing upstream
// handles KeyTab) marks the event Handled for its own purposes, but must NOT
// count as a fluo widget having taken the key — it is router-internal
// navigation, not delivery to any widget.
func TestKeyDownConsumedTabBookkeeping(t *testing.T) {
	a := &probe{name: "a", focusable: true}
	a.SetWidth(50)
	a.SetHeight(50)
	b := &probe{name: "b", focusable: true}
	b.SetWidth(50)
	b.SetHeight(50)
	root := controls.NewCanvas().Add(a, 0, 0).Add(b, 60, 0)
	layout(root, 200, 100)

	r := input.NewRouter()
	r.SetRoot(root)

	r.PointerButton(input.ButtonLeft, true, render.Point{X: 10, Y: 10}, 0) // focus a
	if consumed := r.KeyDown(input.KeyTab, 0, 0); consumed {
		t.Fatalf("KeyDown(Tab) resolved via focus-cycling bookkeeping: consumed = true, want false")
	}
	if r.Focused() != core.Widget(b) {
		t.Fatalf("Focused() after Tab = %v, want b", r.Focused())
	}
}

// TestWantCaptureKeyboard covers WantCaptureKeyboard's focus-tracking:
// false with nothing focused, true once a widget is, false again once focus
// is cleared.
func TestWantCaptureKeyboard(t *testing.T) {
	a := &probe{name: "a", focusable: true}
	a.SetWidth(50)
	a.SetHeight(50)
	root := controls.NewCanvas().Add(a, 0, 0)
	layout(root, 100, 100)

	r := input.NewRouter()
	r.SetRoot(root)

	if r.WantCaptureKeyboard() {
		t.Fatalf("WantCaptureKeyboard() with nothing focused = true, want false")
	}

	r.Focus(a)
	if !r.WantCaptureKeyboard() {
		t.Fatalf("WantCaptureKeyboard() with a focused = false, want true")
	}

	r.Focus(nil)
	if r.WantCaptureKeyboard() {
		t.Fatalf("WantCaptureKeyboard() after clearing focus = true, want false")
	}
}

// TestWantCapturePointer covers WantCapturePointer across hover and capture:
// false with the pointer over empty space, true once hover reaches a
// PointerHandler, and true while a capture is active even with the pointer
// positioned outside the captor's own bounds (capture bypasses hit-testing
// entirely — see Capture's doc comment).
func TestWantCapturePointer(t *testing.T) {
	a := &probe{name: "a", capturing: true}
	a.SetWidth(50)
	a.SetHeight(50)
	root := controls.NewCanvas().Add(a, 0, 0)
	layout(root, 100, 100)

	r := input.NewRouter()
	r.SetRoot(root)

	if r.WantCapturePointer() {
		t.Fatalf("WantCapturePointer() before any move = true, want false")
	}

	r.PointerMove(render.Point{X: 90, Y: 90}, 0) // empty space
	if r.WantCapturePointer() {
		t.Fatalf("WantCapturePointer() over empty space = true, want false")
	}

	r.PointerMove(render.Point{X: 10, Y: 10}, 0) // over a
	if !r.WantCapturePointer() {
		t.Fatalf("WantCapturePointer() over a = false, want true")
	}

	r.PointerButton(input.ButtonLeft, true, render.Point{X: 10, Y: 10}, 0) // a captures
	if r.Captured() != core.Widget(a) {
		t.Fatalf("Captured() = %v, want a", r.Captured())
	}
	if !r.WantCapturePointer() {
		t.Fatalf("WantCapturePointer() while captured = false, want true")
	}
}

// --- FocusedCaretRect ---

// caretProbe is a minimal input.CaretRector: reports a fixed rect while ok is
// true, false otherwise — kept separate from probe (used pervasively by
// every other test in this file) so those tests don't need to know anything
// about caret rects.
type caretProbe struct {
	core.Element
	rect render.Rect
	ok   bool
}

func (c *caretProbe) AcceptsFocus() bool                   { return true }
func (c *caretProbe) CaretScreenRect() (render.Rect, bool) { return c.rect, c.ok }

func TestFocusedCaretRectNothingFocused(t *testing.T) {
	root := controls.NewCanvas()
	r := input.NewRouter()
	r.SetRoot(root)

	if _, ok := r.FocusedCaretRect(); ok {
		t.Fatal("FocusedCaretRect() ok = true with nothing focused, want false")
	}
}

// TestFocusedCaretRectFocusedNonCaretProvider covers a focused widget that
// simply doesn't implement CaretRector (e.g. a Button) — FocusedCaretRect
// must report false rather than panicking on the failed type assertion.
func TestFocusedCaretRectFocusedNonCaretProvider(t *testing.T) {
	a := &probe{name: "a", focusable: true}
	a.SetWidth(50)
	a.SetHeight(50)
	root := controls.NewCanvas().Add(a, 0, 0)
	layout(root, 100, 100)

	r := input.NewRouter()
	r.SetRoot(root)
	r.Focus(a)

	if _, ok := r.FocusedCaretRect(); ok {
		t.Fatal("FocusedCaretRect() ok = true for a focused non-CaretRector widget, want false")
	}
}

func TestFocusedCaretRectFakeProvider(t *testing.T) {
	c := &caretProbe{rect: render.Rect{X: 5, Y: 6, W: 2, H: 14}, ok: true}
	root := controls.NewCanvas().Add(c, 0, 0)
	layout(root, 100, 100)

	r := input.NewRouter()
	r.SetRoot(root)
	r.Focus(c)

	got, ok := r.FocusedCaretRect()
	if !ok {
		t.Fatal("FocusedCaretRect() ok = false with a focused CaretRector, want true")
	}
	if got != c.rect {
		t.Fatalf("FocusedCaretRect() = %v, want %v", got, c.rect)
	}
}

// TestFocusedCaretRectFakeProviderReportsFalse covers a focused CaretRector
// that itself has nothing to report right now (mirrors TextBox.
// CaretScreenRect's own "false" branches, e.g. transiently unfocused from
// its own point of view) — FocusedCaretRect must pass that false straight
// through rather than treating "implements CaretRector" alone as enough.
func TestFocusedCaretRectFakeProviderReportsFalse(t *testing.T) {
	c := &caretProbe{ok: false}
	root := controls.NewCanvas().Add(c, 0, 0)
	layout(root, 100, 100)

	r := input.NewRouter()
	r.SetRoot(root)
	r.Focus(c)

	if _, ok := r.FocusedCaretRect(); ok {
		t.Fatal("FocusedCaretRect() ok = true when the CaretRector itself reports false, want false")
	}
}

// TestFocusedCaretRectRealTextBox is the end-to-end wiring check with the
// real control CaretRector is meant for: FocusedCaretRect must return
// exactly what the focused TextBox's own CaretScreenRect reports.
func TestFocusedCaretRectRealTextBox(t *testing.T) {
	tb := controls.NewTextBox(nil)
	tb.SetText("hi")
	tb.SetWidth(200)
	tb.SetHeight(30)
	root := controls.NewCanvas().Add(tb, 0, 0)
	layout(root, 300, 100)

	r := input.NewRouter()
	r.SetRoot(root)
	r.Focus(tb)

	got, ok := r.FocusedCaretRect()
	if !ok {
		t.Fatal("FocusedCaretRect() ok = false with a focused TextBox, want true")
	}
	want, _ := tb.CaretScreenRect()
	if got != want {
		t.Fatalf("FocusedCaretRect() = %v, want %v (TextBox.CaretScreenRect())", got, want)
	}
}

// clickProbe records the ClickCount and Time of every pointer event it
// receives, split by action, so the multi-click tests below assert on what
// the Router actually stamped rather than on state the Router keeps
// privately. It marks presses handled but never captures, so every press goes
// through the ordinary hit-test path.
type clickProbe struct {
	core.Element
	pressCounts   []int
	pressTimes    []float64
	releaseCounts []int
}

func (p *clickProbe) OnPointer(e *input.PointerEvent) {
	switch e.Action {
	case input.Press:
		p.pressCounts = append(p.pressCounts, e.ClickCount)
		p.pressTimes = append(p.pressTimes, e.Time)
	case input.Release:
		p.releaseCounts = append(p.releaseCounts, e.ClickCount)
	}
	e.Handled = true
}

// newClickProbeRouter builds a 50x50 clickProbe at the origin of a laid-out
// root, wired to a router whose time source reads *clock — so a test drives
// multi-click detection by assigning to clock rather than by sleeping.
func newClickProbeRouter(clock *float64) (*clickProbe, *input.Router) {
	p := &clickProbe{}
	p.SetWidth(50)
	p.SetHeight(50)
	root := controls.NewCanvas().Add(p, 0, 0)
	layout(root, 100, 100)

	r := input.NewRouter()
	r.SetRoot(root)
	r.SetTimeSource(func() float64 { return *clock })
	return p, r
}

// TestClickCountRuns walks one router through every way a click run can
// continue or break: three quick presses at the same spot climb 1, 2, 3, a
// fourth keeps climbing (the Router counts rather than wrapping — widgets
// decide what "beyond 3" means), then a press after too long a gap, a press
// too far away, and a press with a different button each restart at 1.
func TestClickCountRuns(t *testing.T) {
	clock := 0.0
	p, r := newClickProbeRouter(&clock)

	at := render.Point{X: 10, Y: 10}
	moved := render.Point{X: at.X + input.MultiClickDistance + 1, Y: at.Y}
	press := func(b input.Button, dt float64, pt render.Point) {
		clock += dt
		r.PointerButton(b, true, pt, 0)
	}

	press(input.ButtonLeft, 0, at)      // 1: first press ever
	press(input.ButtonLeft, 0.1, at)    // 2: well inside the interval
	press(input.ButtonLeft, 0.1, at)    // 3
	press(input.ButtonLeft, 0.1, at)    // 4: keeps counting past a triple
	press(input.ButtonLeft, 1.0, at)    // 1: gap exceeds MultiClickInterval
	press(input.ButtonLeft, 0.1, at)    // 2: quick again, a new run resumes
	press(input.ButtonLeft, 0.1, moved) // 1: beyond MultiClickDistance
	press(input.ButtonRight, 0.1, moved)

	want := []int{1, 2, 3, 4, 1, 2, 1, 1}
	if len(p.pressCounts) != len(want) {
		t.Fatalf("saw %d presses (%v), want %d", len(p.pressCounts), p.pressCounts, len(want))
	}
	for i, w := range want {
		if p.pressCounts[i] != w {
			t.Fatalf("press %d: ClickCount = %d, want %d (full run: %v)", i, p.pressCounts[i], w, p.pressCounts)
		}
	}
}

// TestClickCountAtThresholdEdges pins the boundaries themselves: a press
// exactly MultiClickInterval later, or exactly MultiClickDistance away, still
// continues the run (both comparisons are inclusive), while a hair past
// either one does not.
func TestClickCountAtThresholdEdges(t *testing.T) {
	for _, tc := range []struct {
		name string
		dt   float64
		dx   float32
		want int
	}{
		{"exactly at the time limit", input.MultiClickInterval, 0, 2},
		{"just past the time limit", input.MultiClickInterval + 0.01, 0, 1},
		{"exactly at the distance limit", 0.05, input.MultiClickDistance, 2},
		{"just past the distance limit", 0.05, input.MultiClickDistance + 0.01, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clock := 0.0
			p, r := newClickProbeRouter(&clock)

			first := render.Point{X: 10, Y: 10}
			r.PointerButton(input.ButtonLeft, true, first, 0)
			clock += tc.dt
			r.PointerButton(input.ButtonLeft, true, render.Point{X: first.X + tc.dx, Y: first.Y}, 0)

			if len(p.pressCounts) != 2 {
				t.Fatalf("saw %d presses (%v), want 2", len(p.pressCounts), p.pressCounts)
			}
			if p.pressCounts[1] != tc.want {
				t.Fatalf("second press ClickCount = %d, want %d", p.pressCounts[1], tc.want)
			}
		})
	}
}

// TestClickCountVerticalDistanceBreaksRun covers the y half of the slop box.
// The x half is exercised above; both matter because the test is per-axis
// (a rectangle) rather than radial.
func TestClickCountVerticalDistanceBreaksRun(t *testing.T) {
	clock := 0.0
	p, r := newClickProbeRouter(&clock)

	r.PointerButton(input.ButtonLeft, true, render.Point{X: 10, Y: 10}, 0)
	clock += 0.05
	r.PointerButton(input.ButtonLeft, true, render.Point{X: 10, Y: 10 + input.MultiClickDistance + 1}, 0)

	if p.pressCounts[1] != 1 {
		t.Fatalf("press moved vertically out of the slop box: ClickCount = %d, want 1", p.pressCounts[1])
	}
}

// TestClickCountWithoutTimeSource is the regression lock on the additive
// promise: a router the host never gave a clock behaves exactly as it always
// did — every press is standalone no matter how many land on the same pixel,
// since without a clock they would all share timestamp 0 and so look
// infinitely fast.
func TestClickCountWithoutTimeSource(t *testing.T) {
	p := &clickProbe{}
	p.SetWidth(50)
	p.SetHeight(50)
	root := controls.NewCanvas().Add(p, 0, 0)
	layout(root, 100, 100)

	r := input.NewRouter()
	r.SetRoot(root) // deliberately no SetTimeSource

	at := render.Point{X: 10, Y: 10}
	for i := 0; i < 3; i++ {
		r.PointerButton(input.ButtonLeft, true, at, 0)
	}
	for i, got := range p.pressCounts {
		if got != 1 {
			t.Fatalf("press %d on a clockless router: ClickCount = %d, want 1 (full run: %v)", i, got, p.pressCounts)
		}
		if p.pressTimes[i] != 0 {
			t.Fatalf("press %d on a clockless router: Time = %v, want 0", i, p.pressTimes[i])
		}
	}

	// Installing a clock afterwards starts counting from scratch rather than
	// chaining onto presses dispatched before it existed.
	clock := 0.0
	r.SetTimeSource(func() float64 { return clock })
	r.PointerButton(input.ButtonLeft, true, at, 0)
	if got := p.pressCounts[len(p.pressCounts)-1]; got != 1 {
		t.Fatalf("first press after installing a clock: ClickCount = %d, want 1", got)
	}
}

// TestClickCountOnlyOnPress checks the field stays 0 for everything that is
// not a press, and that the release every real double-click interleaves does
// not itself break the run.
func TestClickCountOnlyOnPress(t *testing.T) {
	clock := 0.0
	p, r := newClickProbeRouter(&clock)

	at := render.Point{X: 10, Y: 10}
	r.PointerButton(input.ButtonLeft, true, at, 0)
	clock += 0.05
	r.PointerButton(input.ButtonLeft, false, at, 0)
	clock += 0.05
	r.PointerButton(input.ButtonLeft, true, at, 0)

	if len(p.pressCounts) != 2 || p.pressCounts[1] != 2 {
		t.Fatalf("press counts = %v, want [1 2] (an intervening release must not break the run)", p.pressCounts)
	}
	for i, got := range p.releaseCounts {
		if got != 0 {
			t.Fatalf("release %d: ClickCount = %d, want 0", i, got)
		}
	}
}

// timeProbe records e.Time for the Move/Press/Wheel events it receives,
// ignoring the Enter that arrives alongside the first move (updateHover's
// derived notifications carry no timestamp by design).
type timeProbe struct {
	core.Element
	times []float64
}

func (p *timeProbe) OnPointer(e *input.PointerEvent) {
	switch e.Action {
	case input.Move, input.Press, input.Wheel:
		p.times = append(p.times, e.Time)
	}
}

// TestPointerEventTimeStamped proves Time reaches widgets from the installed
// clock on all three real dispatch entry points, not only on presses.
func TestPointerEventTimeStamped(t *testing.T) {
	const clock = 12.5

	p := &timeProbe{}
	p.SetWidth(50)
	p.SetHeight(50)
	root := controls.NewCanvas().Add(p, 0, 0)
	layout(root, 100, 100)

	r := input.NewRouter()
	r.SetRoot(root)
	r.SetTimeSource(func() float64 { return clock })

	at := render.Point{X: 10, Y: 10}
	r.PointerMove(at, 0)
	r.PointerButton(input.ButtonLeft, true, at, 0)
	r.PointerWheel(render.Point{Y: 1}, at, 0)

	if len(p.times) != 3 {
		t.Fatalf("saw %d timestamped events (%v), want 3", len(p.times), p.times)
	}
	for i, got := range p.times {
		if got != clock {
			t.Fatalf("event %d: Time = %v, want %v", i, got, clock)
		}
	}
}
