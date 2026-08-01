package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
)

// clickProbe is a minimal leaf widget wrapping a ClickBehavior directly (no
// Button chrome), for isolated state-machine tests. Explicit size comes from
// core.Element's SetWidth/SetHeight, matching the probe pattern used
// throughout controls/overlay_test.go and input/router_test.go.
type clickProbe struct {
	core.Element
	click ClickBehavior
}

func (p *clickProbe) OnPointer(e *input.PointerEvent) {
	p.click.HandlePointer(e, p)
}

// layoutProbe measures then arranges p at the given absolute rect — the
// probe's own Bounds() (read by ClickBehavior via core.BoundsOf) come from
// this, not from its (irrelevant, zero) MeasureContent.
func layoutProbe(p *clickProbe, bounds render.Rect) {
	core.MeasureWidget(p, render.Size{W: bounds.W, H: bounds.H})
	core.ArrangeWidget(p, bounds)
}

func TestClickBehaviorPressCaptureReleaseInsideFires(t *testing.T) {
	p := &clickProbe{}
	p.SetWidth(40)
	p.SetHeight(20)

	clicks := 0
	p.click.OnClick = func() { clicks++ }

	r := input.NewRouter()
	r.SetRoot(p)
	layoutProbe(p, render.Rect{X: 0, Y: 0, W: 40, H: 20})

	inside := render.Point{X: 10, Y: 10}
	r.PointerButton(input.ButtonLeft, true, inside, 0)
	if !p.click.Pressed() {
		t.Fatal("Pressed() = false after Press, want true")
	}
	if got := r.Captured(); got != core.Widget(p) {
		t.Fatalf("Captured() after Press = %v, want probe", got)
	}

	r.PointerButton(input.ButtonLeft, false, inside, 0)
	if p.click.Pressed() {
		t.Fatal("Pressed() = true after Release, want false")
	}
	if got := r.Captured(); got != nil {
		t.Fatalf("Captured() after Release = %v, want nil", got)
	}
	if clicks != 1 {
		t.Fatalf("clicks = %d, want 1 (release inside must fire)", clicks)
	}
}

func TestClickBehaviorReleaseOutsideDoesNotFire(t *testing.T) {
	p := &clickProbe{}
	p.SetWidth(40)
	p.SetHeight(20)

	clicks := 0
	p.click.OnClick = func() { clicks++ }

	r := input.NewRouter()
	r.SetRoot(p)
	layoutProbe(p, render.Rect{X: 0, Y: 0, W: 40, H: 20})

	r.PointerButton(input.ButtonLeft, true, render.Point{X: 10, Y: 10}, 0)
	// Still captured, so this Release is delivered directly to p regardless
	// of position — well outside its {0,0,40,20} bounds.
	r.PointerButton(input.ButtonLeft, false, render.Point{X: 500, Y: 500}, 0)

	if p.click.Pressed() {
		t.Fatal("Pressed() = true after outside Release, want false")
	}
	if got := r.Captured(); got != nil {
		t.Fatalf("Captured() after outside Release = %v, want nil (still released)", got)
	}
	if clicks != 0 {
		t.Fatalf("clicks = %d, want 0 (release outside must not fire)", clicks)
	}
}

// A Release that arrives at a ClickBehavior which never took a capture must
// leave whoever DOES hold the capture alone. input.Router.Release pops the
// top of the capture stack blind, so an unguarded release here would drop
// someone else's grab — in practice an OverlayHost's modal capture, since
// that host is exactly what forwards pointer events into a popup subtree
// while holding one (see OverlayHost.OnPointer), and an owner can perfectly
// well skip HandlePointer on the Press and reach it on the Release.
func TestClickBehaviorReleaseWithoutCaptureLeavesOthersCapture(t *testing.T) {
	holder := &clickProbe{}
	p := &clickProbe{}
	p.SetWidth(40)
	p.SetHeight(20)

	clicks := 0
	p.click.OnClick = func() { clicks++ }

	r := input.NewRouter()
	r.SetRoot(p)
	layoutProbe(p, render.Rect{X: 0, Y: 0, W: 40, H: 20})

	// holder grabs the pointer the way OverlayHost does for a modal popup.
	r.Capture(holder)

	// A Release forwarded into p's subtree with no matching Press: p is not
	// pressed and holds no capture of its own.
	e := &input.PointerEvent{Action: input.Release, Pos: render.Point{X: 10, Y: 10}, Router: r}
	input.Bubble([]core.Widget{p}, e)

	if got := r.Captured(); got != core.Widget(holder) {
		t.Fatalf("Captured() after unpressed Release = %v, want holder (capture must not be popped)", got)
	}
	if clicks != 0 {
		t.Fatalf("clicks = %d, want 0 (no Press preceded this Release)", clicks)
	}
}

// The guard must not break legitimate nesting: a widget that captures while
// someone else already holds the grab still pops only its own entry, leaving
// the outer capture restored.
func TestClickBehaviorReleasePopsOnlyItsOwnNestedCapture(t *testing.T) {
	holder := &clickProbe{}
	p := &clickProbe{}
	p.SetWidth(40)
	p.SetHeight(20)

	clicks := 0
	p.click.OnClick = func() { clicks++ }

	r := input.NewRouter()
	r.SetRoot(p)
	layoutProbe(p, render.Rect{X: 0, Y: 0, W: 40, H: 20})

	r.Capture(holder)

	inside := render.Point{X: 10, Y: 10}
	input.Bubble([]core.Widget{p}, &input.PointerEvent{Action: input.Press, Pos: inside, Router: r})
	if got := r.Captured(); got != core.Widget(p) {
		t.Fatalf("Captured() after Press = %v, want probe (nested over holder)", got)
	}

	input.Bubble([]core.Widget{p}, &input.PointerEvent{Action: input.Release, Pos: inside, Router: r})
	if got := r.Captured(); got != core.Widget(holder) {
		t.Fatalf("Captured() after Release = %v, want holder (outer capture restored)", got)
	}
	if clicks != 1 {
		t.Fatalf("clicks = %d, want 1 (press then release inside still fires)", clicks)
	}
}

func TestClickBehaviorHoverTracking(t *testing.T) {
	p := &clickProbe{}
	p.SetWidth(40)
	p.SetHeight(20)

	r := input.NewRouter()
	r.SetRoot(p)
	layoutProbe(p, render.Rect{X: 0, Y: 0, W: 40, H: 20})

	if p.click.Hover() {
		t.Fatal("Hover() = true before any Move, want false")
	}

	r.PointerMove(render.Point{X: 10, Y: 10}, 0) // enters
	if !p.click.Hover() {
		t.Fatal("Hover() = false after Move inside, want true")
	}

	r.PointerMove(render.Point{X: 500, Y: 500}, 0) // leaves
	if p.click.Hover() {
		t.Fatal("Hover() = true after Move outside, want false")
	}
}

func TestClickBehaviorActivateFiresOnClickDirectly(t *testing.T) {
	var c ClickBehavior
	clicks := 0
	c.OnClick = func() { clicks++ }

	c.Activate()
	c.Activate()

	if clicks != 2 {
		t.Fatalf("clicks = %d, want 2 (Activate bypasses press/release entirely)", clicks)
	}
}

func TestClickBehaviorActivateNilOnClickNoop(t *testing.T) {
	var c ClickBehavior
	c.Activate() // must not panic
}
