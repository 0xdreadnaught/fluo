package core

import (
	"math"
	"slices"
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
	ArrangeWidget(s, render.Rect{X: 5, Y: 5, W: 200, H: 200})
	if got, want := BoundsOf(s), s.Bounds(); got != want {
		t.Fatalf("BoundsOf=%v want %v", got, want)
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

func TestSetParentRejectsDoubleParent(t *testing.T) {
	parentA := &stub{}
	parentB := &stub{}
	child := &stub{}

	SetParent(child, parentA)

	// Re-setting the same parent is a no-op, not a panic.
	SetParent(child, parentA)

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when re-parenting an already-parented widget")
		}
	}()
	SetParent(child, parentB)
}

func TestParentOf(t *testing.T) {
	parent := &stub{}
	child := &stub{}
	SetParent(child, parent)
	if got := ParentOf(child); got != Widget(parent) {
		t.Fatalf("ParentOf(child) = %v, want parent", got)
	}
	if got := ParentOf(parent); got != nil {
		t.Fatalf("ParentOf(parent) = %v, want nil (root has no parent)", got)
	}
}

// recordRenderer is a minimal render.Renderer stub for exercising
// RenderWidget's draw order. PushClip/PopClip append "push"/"pop" markers
// to the shared ops slice (the same slice widgets append their own markers
// to, from Render/RenderOverlay); every other method is a no-op since the
// clip/overlay hook tests only care about ordering, not pixels.
type recordRenderer struct {
	ops *[]string
}

func (r *recordRenderer) Begin(fbWidth, fbHeight int, scale float32) {}
func (r *recordRenderer) End()                                       {}
func (r *recordRenderer) FillRect(rect render.Rect, c render.Color)  {}
func (r *recordRenderer) FillRoundedRect(rect render.Rect, radius float32, c render.Color) {
}
func (r *recordRenderer) StrokeRoundedRect(rect render.Rect, radius, width float32, c render.Color) {
}
func (r *recordRenderer) DrawShadow(rect render.Rect, radius, blur float32, c render.Color) {}
func (r *recordRenderer) CreateTexture(w, h int, rgba []byte) render.TextureID {
	return render.NoTexture
}
func (r *recordRenderer) UpdateTexture(id render.TextureID, x, y, w, h int, rgba []byte) {}
func (r *recordRenderer) DeleteTexture(id render.TextureID)                              {}
func (r *recordRenderer) DrawQuad(dst, src render.Rect, tex render.TextureID, tint render.Color) {
}
func (r *recordRenderer) DrawSDFQuads(quads []render.GlyphQuad, tex render.TextureID, c render.Color) {
}
func (r *recordRenderer) PushClip(rect render.Rect) { *r.ops = append(*r.ops, "push") }
func (r *recordRenderer) PopClip()                  { *r.ops = append(*r.ops, "pop") }

// clipStub is a Widget that always implements ClipProvider: its Render
// appends name to the shared ops slice, and ClipRect reports (clip, hasClip)
// so a test can opt in or out of actually clipping.
type clipStub struct {
	Element
	ops      *[]string
	name     string
	hasClip  bool
	clip     render.Rect
	children []Widget
}

func (s *clipStub) MeasureContent(render.Size) render.Size { return render.Size{} }
func (s *clipStub) ArrangeContent(render.Rect)             {}
func (s *clipStub) Render(render.Renderer)                 { *s.ops = append(*s.ops, s.name) }
func (s *clipStub) Children() []Widget                     { return s.children }
func (s *clipStub) ClipRect() (render.Rect, bool)          { return s.clip, s.hasClip }

// overlayStub is a Widget that always implements OverlayRenderer: its
// Render and RenderOverlay each append a marker to the shared ops slice.
type overlayStub struct {
	Element
	ops      *[]string
	name     string
	children []Widget
}

func (s *overlayStub) MeasureContent(render.Size) render.Size { return render.Size{} }
func (s *overlayStub) ArrangeContent(render.Rect)             {}
func (s *overlayStub) Render(render.Renderer)                 { *s.ops = append(*s.ops, s.name) }
func (s *overlayStub) Children() []Widget                     { return s.children }
func (s *overlayStub) RenderOverlay(render.Renderer)          { *s.ops = append(*s.ops, s.name+"-overlay") }

func TestRenderWidgetClipsChildren(t *testing.T) {
	var ops []string
	child := &clipStub{ops: &ops, name: "child"}
	parent := &clipStub{
		ops: &ops, name: "self",
		hasClip:  true,
		clip:     render.Rect{X: 0, Y: 0, W: 10, H: 10},
		children: []Widget{child},
	}

	RenderWidget(parent, &recordRenderer{ops: &ops})

	want := []string{"self", "push", "child", "pop"}
	if !slices.Equal(ops, want) {
		t.Fatalf("ops = %v, want %v", ops, want)
	}
}

func TestRenderWidgetClipsChildrenNoClipWhenNotApplied(t *testing.T) {
	var ops []string
	child := &clipStub{ops: &ops, name: "child"}
	parent := &clipStub{ops: &ops, name: "self", hasClip: false, children: []Widget{child}}

	RenderWidget(parent, &recordRenderer{ops: &ops})

	want := []string{"self", "child"}
	if !slices.Equal(ops, want) {
		t.Fatalf("ops = %v, want %v (no push/pop when ClipRect ok==false)", ops, want)
	}
}

func TestRenderWidgetOverlayAfterChildren(t *testing.T) {
	var ops []string
	child := &overlayStub{ops: &ops, name: "child"}
	parent := &overlayStub{ops: &ops, name: "self", children: []Widget{child}}

	RenderWidget(parent, &recordRenderer{ops: &ops})

	want := []string{"self", "child", "child-overlay", "self-overlay"}
	if !slices.Equal(ops, want) {
		t.Fatalf("ops = %v, want %v", ops, want)
	}
}

func TestIsVisible(t *testing.T) {
	s := &stub{contentW: 50, contentH: 20}
	if !IsVisible(s) {
		t.Fatal("widget should be visible by default")
	}
	s.SetVisible(false)
	if IsVisible(s) {
		t.Fatal("widget should be hidden after SetVisible(false)")
	}
}
