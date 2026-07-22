package controls

import (
	"math"
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
)

// TestCrossWidgetNestingRegression builds a Grid inside a StackPanel inside a
// DockPanel — three different container kinds nested together — and checks
// that measure/arrange survive an Inf-available-height pass without
// panicking, that every level resolves to a finite desired size, and that
// invalidation from a deeply-nested leaf still bubbles all the way to the
// root.
func TestCrossWidgetNestingRegression(t *testing.T) {
	leaf := NewFixed(40, 20, render.RGB(1, 2, 3))

	grid := NewGrid().Cols(Star(1))
	grid.Add(leaf, 0, 0)

	stack := NewStackPanel(Vertical).Add(grid)

	dock := NewDockPanel() // lastFills defaults to true
	dock.Add(stack, DockLeft)

	// Finite width, +Inf available height.
	core.MeasureWidget(dock, render.Size{W: 400, H: float32(math.Inf(1))})
	core.ArrangeWidget(dock, render.Rect{X: 0, Y: 0, W: 400, H: 300})

	for name, w := range map[string]core.Widget{
		"dock":  dock,
		"stack": stack,
		"grid":  grid,
		"leaf":  leaf,
	} {
		d := core.DesiredSizeOf(w)
		if math.IsInf(float64(d.W), 0) || math.IsInf(float64(d.H), 0) {
			t.Fatalf("%s desired size is not finite: %v", name, d)
		}
	}

	if got := core.DesiredSizeOf(stack).H; math.IsInf(float64(got), 0) {
		t.Fatalf("stack desired H is Inf: %v", got)
	}

	if dock.NeedsLayout() {
		t.Fatal("dock should be clean after measure+arrange")
	}

	leaf.InvalidateMeasure()

	if !dock.NeedsLayout() {
		t.Fatal("leaf invalidation should bubble up through grid and stack to the dock root")
	}
}
