package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
)

func TestCanvasAbsolute(t *testing.T) {
	a := NewFixed(30, 30, render.RGB(1, 2, 3))
	c := NewCanvas().Add(a, 15, 25)
	core.MeasureWidget(c, render.Size{W: 200, H: 200})
	if got := c.DesiredSize(); got != (render.Size{}) {
		t.Fatalf("canvas desires 0, got %v", got)
	}
	core.ArrangeWidget(c, render.Rect{X: 100, Y: 100, W: 200, H: 200})
	if got := a.Bounds(); got != (render.Rect{X: 115, Y: 125, W: 30, H: 30}) {
		t.Fatalf("a=%v", got)
	}
}
