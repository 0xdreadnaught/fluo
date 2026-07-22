package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
)

// testWidgetRecorder records the available size it receives when measured.
type testWidgetRecorder struct {
	core.Element
	availableReceived render.Size
}

func (t *testWidgetRecorder) MeasureContent(available render.Size) render.Size {
	t.availableReceived = available
	return render.Size{W: 10, H: 10}
}

func TestDockLayout(t *testing.T) {
	top := NewFixed(0, 30, render.RGB(1, 2, 3))   // full-width bar (W stretches)
	left := NewFixed(80, 0, render.RGB(1, 2, 3))  // sidebar
	fill := NewFixed(10, 10, render.RGB(1, 2, 3)) // content
	d := NewDockPanel().Add(top, DockTop).Add(left, DockLeft).Add(fill, DockLeft)
	core.MeasureWidget(d, render.Size{W: 400, H: 300})
	core.ArrangeWidget(d, render.Rect{X: 0, Y: 0, W: 400, H: 300})
	if got := top.Bounds(); got != (render.Rect{X: 0, Y: 0, W: 400, H: 30}) {
		t.Fatalf("top=%v", got)
	}
	if got := left.Bounds(); got != (render.Rect{X: 0, Y: 30, W: 80, H: 270}) {
		t.Fatalf("left=%v", got)
	}
	if got := fill.Bounds(); got != (render.Rect{X: 80, Y: 30, W: 320, H: 270}) {
		t.Fatalf("fill=%v", got)
	}
}

func TestDockNoFill(t *testing.T) {
	a := NewFixed(50, 50, render.RGB(1, 2, 3))
	a.SetAlign(core.Start, core.Start)
	d := NewDockPanel().SetLastChildFill(false).Add(a, DockLeft)
	core.MeasureWidget(d, render.Size{W: 400, H: 300})
	core.ArrangeWidget(d, render.Rect{X: 0, Y: 0, W: 400, H: 300})
	if got := a.Bounds(); got != (render.Rect{X: 0, Y: 0, W: 50, H: 50}) {
		t.Fatalf("a=%v", got)
	}
}

func TestDockDesired(t *testing.T) {
	d := NewDockPanel().SetLastChildFill(false).
		Add(NewFixed(80, 20, render.RGB(1, 2, 3)), DockLeft).
		Add(NewFixed(60, 40, render.RGB(1, 2, 3)), DockTop)
	core.MeasureWidget(d, render.Size{W: 400, H: 300})
	if got := d.DesiredSize(); got != (render.Size{W: 140, H: 60}) {
		t.Fatalf("desired=%v", got)
	}
}

func TestDockMeasureNarrowsAvailable(t *testing.T) {
	// First child consumes 100px width on the left; second child should see narrowed available.
	first := NewFixed(100, 0, render.RGB(1, 2, 3))
	second := &testWidgetRecorder{}
	d := NewDockPanel().Add(first, DockLeft).Add(second, DockLeft)
	core.MeasureWidget(d, render.Size{W: 400, H: 300})
	// Second child should see available.W = 400 - 100 = 300
	if got := second.availableReceived.W; got != 300 {
		t.Fatalf("second child available.W=%v, want 300", got)
	}
}
