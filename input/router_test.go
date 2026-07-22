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
	focusable bool
	cursor    input.Cursor
	router    *input.Router // captured on press when capturing==true
	capturing bool

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
func (p *probe) OnFocusChanged(f bool) { p.events = append(p.events, fmt.Sprintf("focus:%v", f)) }

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
