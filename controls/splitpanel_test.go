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

// TestSplitPanelDragAtMinPaneWallFiresOnce pins dragTo's change gate. Once
// clampPaneLen has pinned the divider against a min-pane ceiling, every
// further Move in that direction produces the identical ratio — dragTo used
// to fire OnSplitChanged for each of them anyway, reporting a stream of
// "changes" that changed nothing (a listener persisting the layout would
// write on every mouse move against the wall). One fire for the move that
// actually reached the wall, none for the ones that stay there.
func TestSplitPanelDragAtMinPaneWallFiresOnce(t *testing.T) {
	first := NewFixed(10, 10, render.RGB(1, 2, 3))
	second := NewFixed(10, 10, render.RGB(4, 5, 6))
	s := NewSplitPanel(Horizontal).SetFirst(first).SetSecond(second)
	layoutSplitPanel(s, 0, 0, 100, 50)

	fired := 0
	s.SetOnSplitChanged(func(float32) { fired++ })

	_, divider, _ := s.layout()
	r := input.NewRouter()
	s.OnPointer(&input.PointerEvent{
		Action: input.Press,
		Pos:    render.Point{X: divider.X + divider.W/2, Y: 25},
		Router: r,
	})

	// available = 100 - divider(6) = 94, minPane = 20, so First clamps at 20
	// and the ratio pins at 20/94. Every X at or left of that wall lands on
	// the same clamped result.
	drag := func(x float32) {
		s.OnPointer(&input.PointerEvent{Action: input.Move, Pos: render.Point{X: x, Y: 25}, Router: r})
	}

	drag(0) // reaches the wall: a real change, fires once
	if fired != 1 {
		t.Fatalf("fired after reaching the min-pane wall = %d, want 1", fired)
	}
	wall := s.ratio

	drag(-5)
	drag(-40)
	drag(0)
	if fired != 1 {
		t.Fatalf("fired after further drags at the wall = %d, want 1 (no re-fire without a real change)", fired)
	}
	if s.ratio != wall {
		t.Fatalf("ratio while pinned = %v, want %v (unchanged)", s.ratio, wall)
	}

	// Dragging back off the wall is a real change again.
	drag(60)
	if fired != 2 {
		t.Fatalf("fired after dragging back off the wall = %d, want 2", fired)
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

// startDividerDrag presses at the middle of s's divider and returns the
// router holding the capture, so a test can then move the pointer wherever
// it likes. Fails the test if the press didn't take the capture.
func startDividerDrag(t *testing.T, s *SplitPanel) *input.Router {
	t.Helper()
	_, divider, _ := s.layout()
	r := input.NewRouter()
	press := &input.PointerEvent{
		Action: input.Press,
		Pos:    render.Point{X: divider.X + divider.W/2, Y: divider.Y + divider.H/2},
		Router: r,
	}
	s.OnPointer(press)
	if r.Captured() != s {
		t.Fatal("press on the divider did not capture the SplitPanel")
	}
	return r
}

// dragDividerTo moves an already-started drag to pos and re-runs layout, so
// the panes' arranged bounds reflect the new ratio.
func dragDividerTo(t *testing.T, s *SplitPanel, r *input.Router, pos render.Point, x, y, w, h float32) {
	t.Helper()
	move := &input.PointerEvent{Action: input.Move, Pos: pos, Router: r}
	s.OnPointer(move)
	if !move.Handled {
		t.Fatal("move while captured not marked Handled")
	}
	layoutSplitPanel(s, x, y, w, h)
}

// wantPaneLen fails unless got is want to within a hundredth of a pixel. A
// drag stores its result as a RATIO, which layout multiplies back out, so a
// clamped pane length only survives that float32 round trip approximately
// (64 comes back as 63.999996) — exact equality would be testing the
// arithmetic, not the clamp.
func wantPaneLen(t *testing.T, label string, got, want float32) {
	t.Helper()
	if d := got - want; d > 0.01 || d < -0.01 {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
}

// TestSplitPanelDragClampsAtMinPaneFirstSide covers dragTo's own
// clampPaneLen call at the low bound. The existing drag test only drags to
// a position well inside the legal range, so a dragTo that dropped its
// clamp entirely would still pass it — here the pointer is dragged past the
// panel's left edge, and First must stop at MinPaneSize instead of
// collapsing (or going negative).
func TestSplitPanelDragClampsAtMinPaneFirstSide(t *testing.T) {
	first := NewFixed(10, 10, render.RGB(1, 2, 3))
	second := NewFixed(10, 10, render.RGB(4, 5, 6))
	s := NewSplitPanel(Horizontal).SetFirst(first).SetSecond(second)
	s.SetMinPaneSize(30)
	layoutSplitPanel(s, 0, 0, 100, 50)

	r := startDividerDrag(t, s)
	// Far outside the panel, on the First side.
	dragDividerTo(t, s, r, render.Point{X: -500, Y: 25}, 0, 0, 100, 50)

	wantPaneLen(t, "first width after dragging past the left edge", core.BoundsOf(first).W, 30)
	// available(94) - first(30) = 64.
	wantPaneLen(t, "second width", core.BoundsOf(second).W, 64)
	// The stored ratio itself must be the clamped one — layout() re-clamps
	// on the way to the arranged bounds, so only the ratio shows whether
	// dragTo did its own clamping.
	wantPaneLen(t, "ratio", s.ratio, 30.0/94.0)
}

// TestSplitPanelDragClampsAtMinPaneSecondSide is the same at the high
// bound: dragged past the panel's right edge, Second must keep MinPaneSize
// rather than being squeezed to nothing.
func TestSplitPanelDragClampsAtMinPaneSecondSide(t *testing.T) {
	first := NewFixed(10, 10, render.RGB(1, 2, 3))
	second := NewFixed(10, 10, render.RGB(4, 5, 6))
	s := NewSplitPanel(Horizontal).SetFirst(first).SetSecond(second)
	s.SetMinPaneSize(30)
	layoutSplitPanel(s, 0, 0, 100, 50)

	r := startDividerDrag(t, s)
	dragDividerTo(t, s, r, render.Point{X: 5000, Y: 25}, 0, 0, 100, 50)

	// available(94) - minPane(30) = 64 is First's ceiling.
	wantPaneLen(t, "first width after dragging past the right edge", core.BoundsOf(first).W, 64)
	wantPaneLen(t, "second width", core.BoundsOf(second).W, 30)
	wantPaneLen(t, "ratio", s.ratio, 64.0/94.0)
}

// TestSplitPanelDragDegenerateAvailableBelowTwoMinPanes covers the case
// clampPaneLen relaxes its floor for: a panel too small to give BOTH panes
// their minimum. The floor must not be applied from both ends at once
// (which would put First above its own ceiling and hand Second a negative
// length) — dragging either way just splits whatever room exists.
func TestSplitPanelDragDegenerateAvailableBelowTwoMinPanes(t *testing.T) {
	first := NewFixed(10, 10, render.RGB(1, 2, 3))
	second := NewFixed(10, 10, render.RGB(4, 5, 6))
	s := NewSplitPanel(Horizontal).SetFirst(first).SetSecond(second)
	s.SetMinPaneSize(30) // 2*30 = 60 > available (40-6 = 34)
	layoutSplitPanel(s, 0, 0, 40, 50)

	r := startDividerDrag(t, s)

	dragDividerTo(t, s, r, render.Point{X: -500, Y: 25}, 0, 0, 40, 50)
	wantPaneLen(t, "first width dragged fully left", core.BoundsOf(first).W, 0)
	wantPaneLen(t, "second width (all of the available room)", core.BoundsOf(second).W, 34)
	wantPaneLen(t, "ratio dragged fully left", s.ratio, 0)

	dragDividerTo(t, s, r, render.Point{X: 5000, Y: 25}, 0, 0, 40, 50)
	wantPaneLen(t, "first width dragged fully right", core.BoundsOf(first).W, 34)
	wantPaneLen(t, "second width (never negative)", core.BoundsOf(second).W, 0)
	wantPaneLen(t, "ratio dragged fully right", s.ratio, 1)
}

// TestSplitPanelSetPaneNilClearsSlot pins the nil guard both pane setters
// were missing: each already detached the OUTGOING pane behind a nil check,
// then handed the INCOMING one straight to core.SetParent, which dereferences
// its child argument — so clearing a pane panicked, even though every read
// path (Children, MeasureContent, ArrangeContent, layout) has always handled
// an empty slot.
func TestSplitPanelSetPaneNilClearsSlot(t *testing.T) {
	first := NewFixed(10, 10, render.RGB(1, 2, 3))
	second := NewFixed(10, 10, render.RGB(4, 5, 6))
	s := NewSplitPanel(Horizontal).SetFirst(first).SetSecond(second)

	s.SetFirst(nil)
	if got := s.Children(); len(got) != 1 || got[0] != core.Widget(second) {
		t.Fatalf("Children() after SetFirst(nil) = %v, want [second]", got)
	}
	if p := core.ParentOf(first); p != nil {
		t.Fatalf("detached first's parent = %v, want nil", p)
	}

	s.SetSecond(nil)
	if got := s.Children(); len(got) != 0 {
		t.Fatalf("Children() after SetSecond(nil) = %v, want empty", got)
	}

	// An empty panel still lays out and renders: the divider is all that's
	// left, and nothing downstream should trip over the missing panes.
	layoutSplitPanel(s, 0, 0, 100, 50)

	// Setting a real pane back afterward works normally.
	s.SetFirst(first)
	if got := s.Children(); len(got) != 1 || got[0] != core.Widget(first) {
		t.Fatalf("Children() after re-setting first = %v, want [first]", got)
	}
}
