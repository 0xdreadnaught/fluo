package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
)

func vstack(gap float32, kids ...core.Widget) *StackPanel {
	return NewStackPanel(Vertical).SetGap(gap).Add(kids...)
}

func TestVStackMeasure(t *testing.T) {
	sp := vstack(0,
		NewFixed(50, 20, render.RGB(1, 2, 3)),
		NewFixed(80, 30, render.RGB(1, 2, 3)),
	)
	core.MeasureWidget(sp, render.Size{W: 200, H: 200})
	if got := sp.DesiredSize(); got != (render.Size{W: 80, H: 50}) {
		t.Fatalf("desired=%v", got)
	}
}

func TestVStackGap(t *testing.T) {
	sp := vstack(6,
		NewFixed(10, 10, render.RGB(1, 2, 3)),
		NewFixed(10, 10, render.RGB(1, 2, 3)),
		NewFixed(10, 10, render.RGB(1, 2, 3)),
	)
	core.MeasureWidget(sp, render.Size{W: 200, H: 200})
	if got := sp.DesiredSize().H; got != 42 {
		t.Fatalf("H=%v (3*10+2*6)", got)
	} // 42
}

func TestVStackArrangePositionsAndStretch(t *testing.T) {
	a := NewFixed(50, 20, render.RGB(1, 2, 3)) // Fixed content 50 wide, but panel slot is 120: Stretch default
	b := NewFixed(80, 30, render.RGB(1, 2, 3))
	b.SetAlign(core.Start, core.Start)
	sp := vstack(0, a, b)
	core.MeasureWidget(sp, render.Size{W: 120, H: 300})
	core.ArrangeWidget(sp, render.Rect{X: 10, Y: 10, W: 120, H: 300})
	if got := a.Bounds(); got != (render.Rect{X: 10, Y: 10, W: 120, H: 20}) {
		t.Fatalf("a=%v (stretch cross-axis)", got)
	}
	if got := b.Bounds(); got != (render.Rect{X: 10, Y: 30, W: 80, H: 30}) {
		t.Fatalf("b=%v (start-aligned)", got)
	}
}

func TestHStackMeasureAndArrange(t *testing.T) {
	sp := NewStackPanel(Horizontal).Add(
		NewFixed(50, 20, render.RGB(1, 2, 3)),
		NewFixed(30, 40, render.RGB(1, 2, 3)),
	)
	core.MeasureWidget(sp, render.Size{W: 200, H: 200})
	if got := sp.DesiredSize(); got != (render.Size{W: 80, H: 40}) {
		t.Fatalf("desired=%v", got)
	}
	core.ArrangeWidget(sp, render.Rect{X: 0, Y: 0, W: 200, H: 60})
	kids := sp.Children()
	if got := kids[1].(*Fixed).Bounds().X; got != 50 {
		t.Fatalf("second child X=%v", got)
	}
}

func TestStackSkipsHiddenChildren(t *testing.T) {
	hid := NewFixed(50, 50, render.RGB(1, 2, 3))
	hid.SetVisible(false)
	sp := vstack(10, NewFixed(10, 10, render.RGB(1, 2, 3)), hid, NewFixed(10, 10, render.RGB(1, 2, 3)))
	core.MeasureWidget(sp, render.Size{W: 200, H: 200})
	// hidden child contributes 0 size AND no gap: 10+10 + one 10 gap = 30
	if got := sp.DesiredSize().H; got != 30 {
		t.Fatalf("H=%v", got)
	}
}

func TestStackVisibleZeroExtentChildKeepsGap(t *testing.T) {
	// Visible zero-extent child should keep its gaps but contribute no height.
	zeroExtent := NewFixed(10, 0, render.RGB(1, 2, 3))
	sp := vstack(10, NewFixed(10, 10, render.RGB(1, 2, 3)), zeroExtent, NewFixed(10, 10, render.RGB(1, 2, 3)))
	core.MeasureWidget(sp, render.Size{W: 200, H: 200})
	// Expected: 10 + gap(10) + 0 + gap(10) + 10 = 40
	if got := sp.DesiredSize().H; got != 40 {
		t.Fatalf("H=%v, want 40", got)
	}

	// Arrange and verify the middle child's Bounds() is updated (not stale).
	core.ArrangeWidget(sp, render.Rect{X: 0, Y: 0, W: 100, H: 100})
	kids := sp.Children()
	if got := kids[1].(*Fixed).Bounds(); got != (render.Rect{X: 0, Y: 20, W: 100, H: 0}) {
		t.Fatalf("middle child bounds=%v, want {X:0 Y:20 W:100 H:0}", got)
	}
}
