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
