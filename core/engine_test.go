package core

import (
	"math"
	"testing"

	"github.com/0xdreadnaught/fluo/render"
)

// stub: fixed content size, records last arranged bounds.
type stub struct {
	Element
	contentW, contentH float32
	lastContent        render.Rect
}

func (s *stub) MeasureContent(render.Size) render.Size {
	return render.Size{W: s.contentW, H: s.contentH}
}
func (s *stub) ArrangeContent(b render.Rect) { s.lastContent = b }

func TestMeasureMarginAndDesired(t *testing.T) {
	s := &stub{contentW: 50, contentH: 20}
	s.SetMargin(render.Thickness{Left: 5, Top: 10, Right: 15, Bottom: 20})
	MeasureWidget(s, render.Size{W: 200, H: 200})
	if got := s.DesiredSize(); got != (render.Size{W: 70, H: 50}) {
		t.Fatalf("desired=%v", got)
	}
}

func TestMeasureExplicitSizeWins(t *testing.T) {
	s := &stub{contentW: 50, contentH: 20}
	s.SetWidth(80)
	MeasureWidget(s, render.Size{W: 200, H: 200})
	if got := s.DesiredSize(); got != (render.Size{W: 80, H: 20}) {
		t.Fatalf("desired=%v", got)
	}
}

func TestMeasureClampMinMax(t *testing.T) {
	s := &stub{contentW: 50, contentH: 20}
	s.SetMinSize(60, 0)
	s.SetMaxSize(0, 10) // W unbounded, H capped
	MeasureWidget(s, render.Size{W: 200, H: 200})
	if got := s.DesiredSize(); got != (render.Size{W: 60, H: 10}) {
		t.Fatalf("desired=%v", got)
	}
}

func TestMeasureInfAvailable(t *testing.T) {
	inf := float32(math.Inf(1))
	s := &stub{contentW: 50, contentH: 20}
	s.SetMargin(render.Uniform(4))
	MeasureWidget(s, render.Size{W: inf, H: inf})
	if got := s.DesiredSize(); got != (render.Size{W: 58, H: 28}) {
		t.Fatalf("desired=%v", got)
	}
}

func TestArrangeStretchFillsSlot(t *testing.T) {
	s := &stub{contentW: 50, contentH: 20}
	s.SetMargin(render.Uniform(10))
	MeasureWidget(s, render.Size{W: 200, H: 100})
	ArrangeWidget(s, render.Rect{X: 0, Y: 0, W: 200, H: 100})
	if got := s.Bounds(); got != (render.Rect{X: 10, Y: 10, W: 180, H: 80}) {
		t.Fatalf("bounds=%v", got)
	}
	if s.lastContent != s.Bounds() {
		t.Fatalf("ArrangeContent got %v", s.lastContent)
	}
}

func TestArrangeAlignments(t *testing.T) {
	for _, tc := range []struct {
		h, v Alignment
		want render.Rect
	}{
		{Start, Start, render.Rect{X: 0, Y: 0, W: 50, H: 20}},
		{Center, Center, render.Rect{X: 75, Y: 40, W: 50, H: 20}},
		{End, End, render.Rect{X: 150, Y: 80, W: 50, H: 20}},
	} {
		s := &stub{contentW: 50, contentH: 20}
		s.SetAlign(tc.h, tc.v)
		MeasureWidget(s, render.Size{W: 200, H: 100})
		ArrangeWidget(s, render.Rect{X: 0, Y: 0, W: 200, H: 100})
		if got := s.Bounds(); got != tc.want {
			t.Errorf("align %v/%v: bounds=%v want %v", tc.h, tc.v, got, tc.want)
		}
	}
}

func TestArrangeExplicitSizeCentersUnderStretch(t *testing.T) {
	s := &stub{contentW: 50, contentH: 20}
	s.SetWidth(60) // halign stays Stretch
	MeasureWidget(s, render.Size{W: 200, H: 100})
	ArrangeWidget(s, render.Rect{X: 0, Y: 0, W: 200, H: 100})
	b := s.Bounds()
	if b.W != 60 || b.X != 70 {
		t.Fatalf("bounds=%v (want W=60 X=70)", b)
	}
	if b.H != 100 || b.Y != 0 {
		t.Fatalf("bounds=%v (H should stretch)", b)
	}
}

func TestHiddenMeasuresZeroAndSkipsRender(t *testing.T) {
	s := &stub{contentW: 50, contentH: 20}
	s.SetVisible(false)
	MeasureWidget(s, render.Size{W: 200, H: 100})
	if got := s.DesiredSize(); got != (render.Size{}) {
		t.Fatalf("desired=%v", got)
	}
}

func TestDesiredSizeOfMatchesDesiredSize(t *testing.T) {
	s := &stub{contentW: 50, contentH: 20}
	MeasureWidget(s, render.Size{W: 200, H: 200})
	if got, want := DesiredSizeOf(s), s.DesiredSize(); got != want {
		t.Fatalf("DesiredSizeOf=%v want %v", got, want)
	}
}

func TestInvalidationBubbles(t *testing.T) {
	parent := &stub{contentW: 100, contentH: 100}
	child := &stub{contentW: 10, contentH: 10}
	SetParent(child, parent)
	MeasureWidget(parent, render.Size{W: 200, H: 200})
	MeasureWidget(child, render.Size{W: 200, H: 200})
	ArrangeWidget(parent, render.Rect{X: 0, Y: 0, W: 200, H: 200})
	ArrangeWidget(child, render.Rect{X: 0, Y: 0, W: 10, H: 10})
	if parent.NeedsLayout() || child.NeedsLayout() {
		t.Fatal("should be clean after layout")
	}
	child.InvalidateMeasure()
	if !child.NeedsLayout() || !parent.NeedsLayout() {
		t.Fatal("invalidation must bubble to parent")
	}
}
