package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
)

func TestProgressBarDefaultValue(t *testing.T) {
	p := NewProgressBar()
	if p.Value() != 0 {
		t.Fatalf("NewProgressBar().Value() = %v, want 0", p.Value())
	}
}

func TestProgressBarMeasuresToFixedDesiredSize(t *testing.T) {
	p := NewProgressBar()
	core.MeasureWidget(p, render.Size{W: 1000, H: 1000})
	d := core.DesiredSizeOf(p)
	if d.W != 160 || d.H != 8 {
		t.Fatalf("DesiredSize() = %v, want {160 8}", d)
	}
}

func TestProgressBarSetValueClamps(t *testing.T) {
	p := NewProgressBar()

	p.SetValue(-0.5)
	if p.Value() != 0 {
		t.Fatalf("SetValue(-0.5) = %v, want 0", p.Value())
	}

	p.SetValue(1.5)
	if p.Value() != 1 {
		t.Fatalf("SetValue(1.5) = %v, want 1", p.Value())
	}

	p.SetValue(0.3)
	if p.Value() != 0.3 {
		t.Fatalf("SetValue(0.3) = %v, want 0.3", p.Value())
	}
}

// layoutProgressBar measures then arranges w at the given absolute rect —
// same pattern as layoutSlider (slider_test.go).
func layoutProgressBar(w core.Widget, bounds render.Rect) {
	core.MeasureWidget(w, render.Size{W: bounds.W, H: bounds.H})
	core.ArrangeWidget(w, bounds)
}

// TestProgressBarDefaultOrientationAndSolid pins the opt-in invariant: a
// bare NewProgressBar() is Horizontal and chunked (solid == false),
// matching every existing test/golden in this file.
func TestProgressBarDefaultOrientationAndSolid(t *testing.T) {
	p := NewProgressBar()
	if p.orientation != Horizontal {
		t.Fatalf("NewProgressBar().orientation = %v, want Horizontal", p.orientation)
	}
	if p.solid {
		t.Fatal("NewProgressBar().solid = true, want false (chunked default)")
	}
}

// TestProgressBarVerticalMeasuresToSwappedDesiredSize proves Vertical's
// MeasureContent swaps the fixed {160, 8} Horizontal desired size to a
// tall-narrow {8, 160}.
func TestProgressBarVerticalMeasuresToSwappedDesiredSize(t *testing.T) {
	p := NewProgressBar().SetOrientation(Vertical)
	core.MeasureWidget(p, render.Size{W: 1000, H: 1000})
	d := core.DesiredSizeOf(p)
	if d.W != 8 || d.H != 160 {
		t.Fatalf("DesiredSize() = %v, want {8 160}", d)
	}
}

// TestProgressBarVerticalChunkedFillsBottomUp proves Vertical's default
// chunked fill grows bottom-to-top: every chunk's bottom edge falls at or
// above the well's own bottom edge, and the bottom-most chunk sits flush
// against it (the fill is anchored at the bottom, not the top).
func TestProgressBarVerticalChunkedFillsBottomUp(t *testing.T) {
	p := NewProgressBar().SetOrientation(Vertical).SetValue(0.5)
	layoutProgressBar(p, render.Rect{X: 0, Y: 0, W: 8, H: 160})

	rr := &recordRenderer{}
	p.Render(rr)

	inner := p.Bounds().Inset(render.Uniform(2))

	var maxBottom float32 = -1
	count := 0
	for _, f := range rr.fills {
		if f.color != p.colors.Highlight {
			continue
		}
		count++
		if bottom := f.rect.Y + f.rect.H; bottom > inner.Y+inner.H+0.001 {
			t.Fatalf("chunk %+v overflows the well's bottom edge %v", f.rect, inner.Y+inner.H)
		} else if bottom > maxBottom {
			maxBottom = bottom
		}
	}
	if count == 0 {
		t.Fatal("Render emitted no Highlight chunk")
	}
	if maxBottom != inner.Y+inner.H {
		t.Fatalf("bottom-most chunk's bottom edge = %v, want %v flush with the well's bottom edge (fill anchored at the bottom)", maxBottom, inner.Y+inner.H)
	}
}

// TestProgressBarSolidVsChunkedFillLength proves SetSolid(true) replaces
// the default gapped, chunk-quantized fill with exactly one Highlight rect
// spanning the value proportion precisely (no chunk quantization, no
// gaps).
func TestProgressBarSolidVsChunkedFillLength(t *testing.T) {
	chunked := NewProgressBar().SetValue(0.5)
	layoutProgressBar(chunked, render.Rect{X: 0, Y: 0, W: 160, H: 8})
	rrC := &recordRenderer{}
	chunked.Render(rrC)

	solid := NewProgressBar().SetSolid(true).SetValue(0.5)
	layoutProgressBar(solid, render.Rect{X: 0, Y: 0, W: 160, H: 8})
	rrS := &recordRenderer{}
	solid.Render(rrS)

	inner := chunked.Bounds().Inset(render.Uniform(2))
	wantRight := inner.X + inner.W*0.5

	chunkCount := 0
	for _, f := range rrC.fills {
		if f.color == chunked.colors.Highlight {
			chunkCount++
		}
	}
	if chunkCount < 2 {
		t.Fatalf("chunked fill emitted %d Highlight blocks, want >= 2 (gapped chunks)", chunkCount)
	}

	solidCount := 0
	var solidRect filledRect
	for _, f := range rrS.fills {
		if f.color == solid.colors.Highlight {
			solidCount++
			solidRect = f
		}
	}
	if solidCount != 1 {
		t.Fatalf("solid fill emitted %d Highlight fills, want exactly 1 (no chunk gaps)", solidCount)
	}
	if solidRect.rect.X != inner.X {
		t.Fatalf("solid fill X = %v, want %v (starts at the well's inset left edge)", solidRect.rect.X, inner.X)
	}
	if solidRight := solidRect.rect.X + solidRect.rect.W; solidRight != wantRight {
		t.Fatalf("solid fill right edge = %v, want %v (exact value proportion, unlike chunk quantization)", solidRight, wantRight)
	}
}

// TestProgressBarNotFocusable proves ProgressBar implements none of
// input.Focusable/PointerHandler/KeyHandler: it must never become a
// candidate for router focus (Tab/Shift+Tab traversal) and must never
// receive pointer or key events at all — there is no "ignore input" branch
// to test because ProgressBar has no OnPointer/OnKey methods in the first
// place.
func TestProgressBarNotFocusable(t *testing.T) {
	p := NewProgressBar()
	if _, ok := core.Widget(p).(input.Focusable); ok {
		t.Fatal("ProgressBar implements input.Focusable, want it not to (NOT focusable per spec)")
	}
	if _, ok := core.Widget(p).(input.PointerHandler); ok {
		t.Fatal("ProgressBar implements input.PointerHandler, want it not to (no input per spec)")
	}
	if _, ok := core.Widget(p).(input.KeyHandler); ok {
		t.Fatal("ProgressBar implements input.KeyHandler, want it not to (no input per spec)")
	}

	r := input.NewRouter()
	r.SetRoot(p)
	r.FocusNext()
	if r.Focused() != nil {
		t.Fatalf("Focused() after FocusNext() over a tree containing only a ProgressBar = %v, want nil", r.Focused())
	}
}
