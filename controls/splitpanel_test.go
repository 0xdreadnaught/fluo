package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
)

// layoutSplitPanel measures then arranges s at the given bounds (origin at
// x,y), mirroring layoutScrollViewer's role in scrollviewer_test.go.
func layoutSplitPanel(s *SplitPanel, x, y, w, h float32) {
	core.MeasureWidget(s, render.Size{W: w, H: h})
	core.ArrangeWidget(s, render.Rect{X: x, Y: y, W: w, H: h})
}

func TestSplitPanelHorizontalDefaultRatioEvenSplit(t *testing.T) {
	first := NewFixed(10, 10, render.RGB(1, 2, 3))
	second := NewFixed(10, 10, render.RGB(4, 5, 6))
	s := NewSplitPanel(Horizontal).SetFirst(first).SetSecond(second)
	layoutSplitPanel(s, 0, 0, 100, 50)

	// available = 100 - divider(6) = 94; half each = 47.
	wantFirst := render.Rect{X: 0, Y: 0, W: 47, H: 50}
	wantSecond := render.Rect{X: 47 + splitDividerThickness, Y: 0, W: 47, H: 50}

	if got := core.BoundsOf(first); got != wantFirst {
		t.Fatalf("first bounds=%v, want %v", got, wantFirst)
	}
	if got := core.BoundsOf(second); got != wantSecond {
		t.Fatalf("second bounds=%v, want %v", got, wantSecond)
	}
}

func TestSplitPanelVerticalDefaultRatioEvenSplit(t *testing.T) {
	first := NewFixed(10, 10, render.RGB(1, 2, 3))
	second := NewFixed(10, 10, render.RGB(4, 5, 6))
	s := NewSplitPanel(Vertical).SetFirst(first).SetSecond(second)
	layoutSplitPanel(s, 0, 0, 50, 100)

	// available = 100 - divider(6) = 94; half each = 47.
	wantFirst := render.Rect{X: 0, Y: 0, W: 50, H: 47}
	wantSecond := render.Rect{X: 0, Y: 47 + splitDividerThickness, W: 50, H: 47}

	if got := core.BoundsOf(first); got != wantFirst {
		t.Fatalf("first bounds=%v, want %v", got, wantFirst)
	}
	if got := core.BoundsOf(second); got != wantSecond {
		t.Fatalf("second bounds=%v, want %v", got, wantSecond)
	}
}

func TestSplitPanelSetSplitRatioResplits(t *testing.T) {
	first := NewFixed(10, 10, render.RGB(1, 2, 3))
	second := NewFixed(10, 10, render.RGB(4, 5, 6))
	s := NewSplitPanel(Horizontal).SetFirst(first).SetSecond(second)
	s.SetSplitRatio(0.25)
	layoutSplitPanel(s, 0, 0, 100, 50)

	// available = 94; first = 0.25*94 = 23.5.
	want := float32(23.5)
	if got := core.BoundsOf(first).W; got != want {
		t.Fatalf("first width=%v, want %v", got, want)
	}
}

func TestSplitPanelSetSplitRatioClampsOutOfRange(t *testing.T) {
	s := NewSplitPanel(Horizontal)

	s.SetSplitRatio(-1)
	if s.ratio != 0 {
		t.Fatalf("ratio=%v, want 0 (clamped)", s.ratio)
	}

	s.SetSplitRatio(5)
	if s.ratio != 1 {
		t.Fatalf("ratio=%v, want 1 (clamped)", s.ratio)
	}
}

func TestSplitPanelSetSplitRatioDoesNotFireOnSplitChanged(t *testing.T) {
	s := NewSplitPanel(Horizontal)
	fired := false
	s.SetOnSplitChanged(func(r float32) { fired = true })

	s.SetSplitRatio(0.75)
	if fired {
		t.Fatal("SetSplitRatio must not fire OnSplitChanged")
	}
}

// TestSplitPanelMinPaneSizeClampsRatio proves a ratio that would otherwise
// starve First below MinPaneSize is corrected at arrange time (see layout),
// even though SetSplitRatio itself stored the raw, unclamped-by-pixels
// ratio.
func TestSplitPanelMinPaneSizeClampsRatio(t *testing.T) {
	first := NewFixed(10, 10, render.RGB(1, 2, 3))
	second := NewFixed(10, 10, render.RGB(4, 5, 6))
	s := NewSplitPanel(Horizontal).SetFirst(first).SetSecond(second)
	s.SetMinPaneSize(30)
	s.SetSplitRatio(0.01) // would give First ~1px without the floor
	layoutSplitPanel(s, 0, 0, 100, 50)

	if got := core.BoundsOf(first).W; got != 30 {
		t.Fatalf("first width=%v, want 30 (min pane floor)", got)
	}
	// available(94) - first(30) = 64 for Second.
	if got := core.BoundsOf(second).W; got != 64 {
		t.Fatalf("second width=%v, want 64", got)
	}
}

// TestSplitPanelMinPaneSizeClampsBothEnds mirrors the above pinning the
// ratio near 1 instead, proving the floor protects Second too.
func TestSplitPanelMinPaneSizeClampsBothEnds(t *testing.T) {
	first := NewFixed(10, 10, render.RGB(1, 2, 3))
	second := NewFixed(10, 10, render.RGB(4, 5, 6))
	s := NewSplitPanel(Horizontal).SetFirst(first).SetSecond(second)
	s.SetMinPaneSize(30)
	s.SetSplitRatio(0.99)
	layoutSplitPanel(s, 0, 0, 100, 50)

	// available(94) - minPane(30) = 64 is First's ceiling.
	if got := core.BoundsOf(first).W; got != 64 {
		t.Fatalf("first width=%v, want 64 (clamped below the ceiling)", got)
	}
	if got := core.BoundsOf(second).W; got != 30 {
		t.Fatalf("second width=%v, want 30 (min pane floor)", got)
	}
}

func TestSplitPanelDividerDragChangesRatioAndFiresCallback(t *testing.T) {
	first := NewFixed(10, 10, render.RGB(1, 2, 3))
	second := NewFixed(10, 10, render.RGB(4, 5, 6))
	s := NewSplitPanel(Horizontal).SetFirst(first).SetSecond(second)
	layoutSplitPanel(s, 0, 0, 100, 50)

	var lastRatio float32
	fired := 0
	s.SetOnSplitChanged(func(r float32) {
		fired++
		lastRatio = r
	})

	_, divider, _ := s.layout()
	r := input.NewRouter()

	pressPos := render.Point{X: divider.X + divider.W/2, Y: 25}
	press := &input.PointerEvent{Action: input.Press, Pos: pressPos, Router: r}
	s.OnPointer(press)
	if !press.Handled {
		t.Fatal("press on divider not marked Handled")
	}
	if r.Captured() != s {
		t.Fatal("press on divider did not capture the SplitPanel")
	}

	// Drag the divider well to the right.
	move := &input.PointerEvent{Action: input.Move, Pos: render.Point{X: 80, Y: 25}, Router: r}
	s.OnPointer(move)
	if !move.Handled {
		t.Fatal("move while captured not marked Handled")
	}
	if fired == 0 {
		t.Fatal("OnSplitChanged not fired during drag")
	}
	if lastRatio <= 0.5 {
		t.Fatalf("ratio after drag right=%v, want > 0.5", lastRatio)
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

func TestSplitPanelPressOnPaneNotHandled(t *testing.T) {
	first := NewFixed(10, 10, render.RGB(1, 2, 3))
	second := NewFixed(10, 10, render.RGB(4, 5, 6))
	s := NewSplitPanel(Horizontal).SetFirst(first).SetSecond(second)
	layoutSplitPanel(s, 0, 0, 100, 50)

	r := input.NewRouter()
	press := &input.PointerEvent{Action: input.Press, Pos: render.Point{X: 5, Y: 5}, Router: r}
	s.OnPointer(press)
	if press.Handled {
		t.Fatal("press over a pane should not be handled by the SplitPanel itself")
	}
	if r.Captured() != nil {
		t.Fatal("press over a pane should not capture the pointer")
	}
}

func TestSplitPanelCursorOverDividerAndPane(t *testing.T) {
	s := NewSplitPanel(Horizontal)
	layoutSplitPanel(s, 0, 0, 100, 50)
	_, divider, _ := s.layout()

	r := input.NewRouter()

	// Hover over the divider.
	moveDivider := &input.PointerEvent{Action: input.Move, Pos: render.Point{X: divider.X + 1, Y: 25}, Router: r}
	s.OnPointer(moveDivider)
	if got := s.Cursor(); got != input.CursorHResize {
		t.Fatalf("cursor over divider=%v, want CursorHResize", got)
	}

	// Hover over a pane.
	movePane := &input.PointerEvent{Action: input.Move, Pos: render.Point{X: 5, Y: 25}, Router: r}
	s.OnPointer(movePane)
	if got := s.Cursor(); got != input.CursorArrow {
		t.Fatalf("cursor over pane=%v, want CursorArrow", got)
	}
}

func TestSplitPanelCursorVerticalOrientation(t *testing.T) {
	s := NewSplitPanel(Vertical)
	layoutSplitPanel(s, 0, 0, 50, 100)
	_, divider, _ := s.layout()

	r := input.NewRouter()
	move := &input.PointerEvent{Action: input.Move, Pos: render.Point{X: 25, Y: divider.Y + 1}, Router: r}
	s.OnPointer(move)
	if got := s.Cursor(); got != input.CursorVResize {
		t.Fatalf("cursor over divider (vertical)=%v, want CursorVResize", got)
	}
}

func TestSplitPanelChildren(t *testing.T) {
	s := NewSplitPanel(Horizontal)
	if got := s.Children(); got != nil {
		t.Fatalf("Children with no panes=%v, want nil", got)
	}

	first := NewFixed(10, 10, render.RGB(1, 2, 3))
	second := NewFixed(10, 10, render.RGB(4, 5, 6))
	s.SetFirst(first).SetSecond(second)

	got := s.Children()
	if len(got) != 2 || got[0] != core.Widget(first) || got[1] != core.Widget(second) {
		t.Fatalf("Children=%v, want [first, second]", got)
	}
}
