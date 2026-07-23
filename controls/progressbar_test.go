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
