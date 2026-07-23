package controls

import (
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
)

// titleBarFace loads a real face for layout-accurate TitleBar tests (nil
// faces are fine for pure state-machine tests, but layout/drag-region tests
// want a non-degenerate title so bounds aren't all zero).
func titleBarFace(t *testing.T) *text.Face {
	t.Helper()
	f, err := text.Load(goregular.TTF)
	if err != nil {
		t.Fatalf("text.Load: %v", err)
	}
	return text.NewFace(f, 14)
}

// layoutTitleBar measures then arranges tb at the given absolute rect, the
// pattern every test below shares.
func layoutTitleBar(tb *TitleBar, bounds render.Rect) {
	core.MeasureWidget(tb, render.Size{W: bounds.W, H: bounds.H})
	core.ArrangeWidget(tb, bounds)
}

func TestTitleBarSetTitle(t *testing.T) {
	tb := NewTitleBar(titleBarFace(t), "fluo")
	if got := tb.title.Text(); got != "fluo" {
		t.Fatalf("title = %q, want %q", got, "fluo")
	}

	tb.SetTitle("renamed")
	if got := tb.title.Text(); got != "renamed" {
		t.Fatalf("title after SetTitle = %q, want %q", got, "renamed")
	}
}

// TestTitleBarLayoutButtonsRightAligned proves the three caption buttons sit
// flush against the bar's right edge (close rightmost, then maximize, then
// minimize, each abutting the next with no gap) and the title occupies the
// remaining space at the bar's left edge.
func TestTitleBarLayoutButtonsRightAligned(t *testing.T) {
	tb := NewTitleBar(titleBarFace(t), "fluo")
	layoutTitleBar(tb, render.Rect{X: 0, Y: 0, W: 300, H: 32})

	closeB := core.BoundsOf(tb.close)
	maxB := core.BoundsOf(tb.max)
	minB := core.BoundsOf(tb.min)
	titleB := core.BoundsOf(tb.title)

	if closeB.Right() != 300 {
		t.Fatalf("close right edge = %v, want 300 (flush with the bar's right edge)", closeB.Right())
	}
	if maxB.Right() != closeB.X {
		t.Fatalf("max right edge = %v, want to abut close's left edge %v", maxB.Right(), closeB.X)
	}
	if minB.Right() != maxB.X {
		t.Fatalf("min right edge = %v, want to abut max's left edge %v", minB.Right(), maxB.X)
	}
	if titleB.X <= 0 || titleB.X >= minB.X {
		t.Fatalf("title X = %v, want strictly between 0 and min's left edge %v", titleB.X, minB.X)
	}
}

// TestTitleBarDragRegion covers the three cases the brief calls out: over
// the title (draggable), over a caption button (not draggable), and outside
// the bar entirely (not draggable).
func TestTitleBarDragRegion(t *testing.T) {
	tb := NewTitleBar(titleBarFace(t), "fluo")
	layoutTitleBar(tb, render.Rect{X: 0, Y: 0, W: 300, H: 32})

	overTitle := render.Point{X: 10, Y: 16}
	if !tb.DragRegion(overTitle) {
		t.Fatal("DragRegion over the title area = false, want true")
	}

	closeB := core.BoundsOf(tb.close)
	overClose := render.Point{X: closeB.X + closeB.W/2, Y: closeB.Y + closeB.H/2}
	if tb.DragRegion(overClose) {
		t.Fatal("DragRegion over the close button = true, want false")
	}

	outside := render.Point{X: 10, Y: 100}
	if tb.DragRegion(outside) {
		t.Fatal("DragRegion outside the bar = true, want false")
	}
}

// TestTitleBarCaptionCallbacksFireViaRouter drives each caption button
// through a real input.Router (press+release at its center), proving
// OnMinimize/OnMaximize/OnClose fire for the RIGHT button and only that one.
func TestTitleBarCaptionCallbacksFireViaRouter(t *testing.T) {
	tb := NewTitleBar(titleBarFace(t), "fluo")

	var minimized, maximized, closed bool
	tb.OnMinimize(func() { minimized = true })
	tb.OnMaximize(func() { maximized = true })
	tb.OnClose(func() { closed = true })

	r := input.NewRouter()
	r.SetRoot(tb)
	layoutTitleBar(tb, render.Rect{X: 0, Y: 0, W: 300, H: 32})

	click := func(w core.Widget) {
		b := core.BoundsOf(w)
		p := render.Point{X: b.X + b.W/2, Y: b.Y + b.H/2}
		r.PointerButton(input.ButtonLeft, true, p, 0)
		r.PointerButton(input.ButtonLeft, false, p, 0)
	}

	click(tb.min)
	if !minimized || maximized || closed {
		t.Fatalf("after clicking minimize: minimized=%v maximized=%v closed=%v, want only minimized", minimized, maximized, closed)
	}

	click(tb.max)
	if !maximized || closed {
		t.Fatalf("after clicking maximize: maximized=%v closed=%v, want maximized true, closed false", maximized, closed)
	}

	click(tb.close)
	if !closed {
		t.Fatal("OnClose did not fire from a click on the close button")
	}
}

// TestTitleBarNilCallbacksAreSilentNoop proves clicking a caption button
// before its callback is set does not panic.
func TestTitleBarNilCallbacksAreSilentNoop(t *testing.T) {
	tb := NewTitleBar(titleBarFace(t), "fluo")

	r := input.NewRouter()
	r.SetRoot(tb)
	layoutTitleBar(tb, render.Rect{X: 0, Y: 0, W: 300, H: 32})

	b := core.BoundsOf(tb.close)
	p := render.Point{X: b.X + b.W/2, Y: b.Y + b.H/2}
	r.PointerButton(input.ButtonLeft, true, p, 0)
	r.PointerButton(input.ButtonLeft, false, p, 0) // must not panic with no OnClose set
}
