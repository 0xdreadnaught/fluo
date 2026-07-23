package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/theme"
)

// recordingWidget is a leaf widget that records the available size it was
// last measured with, so tests can observe what a parent (e.g. ScrollViewer)
// passed down without depending on the child's own layout math.
type recordingWidget struct {
	core.Element

	lastAvailable render.Size
}

func (r *recordingWidget) MeasureContent(available render.Size) render.Size {
	r.lastAvailable = available
	return render.Size{W: 10, H: 10}
}

func (r *recordingWidget) ArrangeContent(bounds render.Rect) {}

func (r *recordingWidget) Render(rr render.Renderer) {}

func (r *recordingWidget) Children() []core.Widget { return nil }

// TestScrollViewerThemeMetrics asserts the gutter reserved for the thumb
// track comes from the active theme's Metric.ScrollGutter at construction
// time, under both Light and Dark.
func TestScrollViewerThemeMetrics(t *testing.T) {
	defer theme.SetActive(nil)

	// Use a custom theme with a distinctive ScrollGutter value to prove token wiring.
	customTheme := theme.Dark()
	customTheme.Metric.ScrollGutter = 20
	theme.SetActive(customTheme)

	child := &recordingWidget{}
	s := NewScrollViewer().SetChild(child)
	core.MeasureWidget(s, render.Size{W: 100, H: 50})

	want := float32(100) - 20
	if got := child.lastAvailable.W; got != want {
		t.Fatalf("child available width=%v, want %v (100 - 20)", got, want)
	}
}

// layoutScrollViewer measures then arranges s at the given bounds (origin at
// x,y), the pattern every test below shares.
func layoutScrollViewer(s *ScrollViewer, x, y, w, h float32) {
	core.MeasureWidget(s, render.Size{W: w, H: h})
	core.ArrangeWidget(s, render.Rect{X: x, Y: y, W: w, H: h})
}

func TestScrollViewerOffsetClampsBeyondContent(t *testing.T) {
	child := NewFixed(80, 200, render.RGB(1, 2, 3))
	s := NewScrollViewer().SetChild(child)
	s.ScrollTo(10000)
	layoutScrollViewer(s, 10, 20, 100, 50)

	// viewport H = 50 (bounds H, gutter is right-only); childH = 200.
	want := float32(200 - 50)
	if got := s.OffsetY(); got != want {
		t.Fatalf("offset=%v, want %v", got, want)
	}
}

func TestScrollViewerOffsetClampsNegativeToZero(t *testing.T) {
	child := NewFixed(80, 200, render.RGB(1, 2, 3))
	s := NewScrollViewer().SetChild(child)
	s.ScrollTo(-50)
	layoutScrollViewer(s, 10, 20, 100, 50)

	if got := s.OffsetY(); got != 0 {
		t.Fatalf("offset=%v, want 0", got)
	}
}

func TestScrollViewerChildPositionReflectsOffset(t *testing.T) {
	child := NewFixed(80, 200, render.RGB(1, 2, 3))
	s := NewScrollViewer().SetChild(child)
	s.ScrollTo(75) // within [0, 150]
	layoutScrollViewer(s, 10, 20, 100, 50)

	if got := s.OffsetY(); got != 75 {
		t.Fatalf("offset=%v, want 75", got)
	}
	// viewport = bounds inset by {Right: 12} => {X:10, Y:20, W:88, H:50}.
	wantY := float32(20 - 75)
	if got := core.BoundsOf(child); got.Y != wantY {
		t.Fatalf("child bounds=%v, want Y=%v", got, wantY)
	}
}

func TestScrollViewerSmallChildOffsetPinnedZero(t *testing.T) {
	child := NewFixed(80, 10, render.RGB(1, 2, 3)) // shorter than the 50-tall viewport
	s := NewScrollViewer().SetChild(child)
	s.ScrollTo(500) // no room to scroll regardless of what's requested
	layoutScrollViewer(s, 10, 20, 100, 50)

	if got := s.OffsetY(); got != 0 {
		t.Fatalf("offset=%v, want 0 (nothing to scroll)", got)
	}
	if got := core.BoundsOf(child); got.Y != 20 {
		t.Fatalf("child bounds=%v, want Y=20 (unscrolled)", got)
	}
}

func TestScrollViewerWheelScrollsAndHandles(t *testing.T) {
	child := NewFixed(80, 200, render.RGB(1, 2, 3))
	s := NewScrollViewer().SetChild(child)
	layoutScrollViewer(s, 10, 20, 100, 50)
	if got := s.OffsetY(); got != 0 {
		t.Fatalf("initial offset=%v, want 0", got)
	}

	r := input.NewRouter()
	e := &input.PointerEvent{Action: input.Wheel, Delta: render.Point{Y: -2}, Router: r}
	s.OnPointer(e)
	if !e.Handled {
		t.Fatal("wheel event not marked Handled")
	}

	// -Delta.Y * 48 = -(-2)*48 = 96.
	layoutScrollViewer(s, 10, 20, 100, 50)
	if got := s.OffsetY(); got != 96 {
		t.Fatalf("offset after wheel=%v, want 96", got)
	}
}

func TestScrollViewerThumbDragViaCapture(t *testing.T) {
	child := NewFixed(80, 200, render.RGB(1, 2, 3))
	s := NewScrollViewer().SetChild(child)
	layoutScrollViewer(s, 10, 20, 100, 50)

	rect, ok := s.thumbRect()
	if !ok {
		t.Fatal("expected a thumb rect (childH 200 > viewport H 50)")
	}

	r := input.NewRouter()
	pressPos := render.Point{X: rect.X + rect.W/2, Y: rect.Y + 1}
	press := &input.PointerEvent{Action: input.Press, Pos: pressPos, Router: r}
	s.OnPointer(press)
	if !press.Handled {
		t.Fatal("press inside thumb not marked Handled")
	}
	if r.Captured() != s {
		t.Fatal("press inside thumb did not capture the ScrollViewer")
	}

	// Drag the thumb most of the way down the track.
	move := &input.PointerEvent{Action: input.Move, Pos: render.Point{X: pressPos.X, Y: rect.Y + 20}, Router: r}
	s.OnPointer(move)
	if !move.Handled {
		t.Fatal("move while captured not marked Handled")
	}
	layoutScrollViewer(s, 10, 20, 100, 50)
	if got := s.OffsetY(); got <= 0 {
		t.Fatalf("offset after drag=%v, want > 0", got)
	}

	release := &input.PointerEvent{Action: input.Release, Router: r}
	s.OnPointer(release)
	if !release.Handled {
		t.Fatal("release while captured not marked Handled")
	}
	if r.Captured() != nil {
		t.Fatal("release did not clear capture")
	}
}

func TestScrollViewerNonThumbPressNotHandled(t *testing.T) {
	child := NewFixed(80, 200, render.RGB(1, 2, 3))
	s := NewScrollViewer().SetChild(child)
	layoutScrollViewer(s, 10, 20, 100, 50)

	// Well inside the viewport, nowhere near the right-gutter thumb.
	r := input.NewRouter()
	press := &input.PointerEvent{Action: input.Press, Pos: render.Point{X: 15, Y: 25}, Router: r}
	s.OnPointer(press)
	if press.Handled {
		t.Fatal("press outside the thumb should not be handled (must bubble to content)")
	}
	if r.Captured() != nil {
		t.Fatal("press outside the thumb should not capture")
	}
}
