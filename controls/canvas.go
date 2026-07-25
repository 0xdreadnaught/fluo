package controls

import (
	"math"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
)

// canvasItem pairs a child widget with the (x, y) offset at which it is
// placed, relative to the Canvas's own bounds.
type canvasItem struct {
	child core.Widget
	x, y  float32
}

// Canvas is a container widget that positions children at fixed (x, y)
// offsets rather than flowing or stretching them. Each child is measured
// with unbounded space and arranged at its own desired size (no stretch on
// a canvas, matching WPF Canvas semantics) — a child that wants Stretch
// alignment simply gets its desired size, not the Canvas's full extent.
//
// Canvas desires zero size: it is only useful given an explicit size or a
// filled slot; it does not size to its children.
type Canvas struct {
	core.Element

	items []canvasItem
}

// NewCanvas returns a new, empty Canvas.
func NewCanvas() *Canvas {
	return &Canvas{}
}

// Add places w at the given (x, y) offset within this Canvas, re-parenting
// it and invalidating measure.
func (c *Canvas) Add(w core.Widget, x, y float32) *Canvas {
	c.items = append(c.items, canvasItem{child: w, x: x, y: y})
	core.SetParent(w, c)
	c.InvalidateMeasure()
	return c
}

// Children returns the child widgets in the order they were added. Returns
// a copy; mutating it does not affect the panel.
func (c *Canvas) Children() []core.Widget {
	children := make([]core.Widget, len(c.items))
	for i, item := range c.items {
		children[i] = item.child
	}
	return children
}

// MeasureContent measures every child with unbounded (+Inf, +Inf) available
// space — a canvas imposes no constraint on its children — and always
// desires zero itself (WPF semantics: a Canvas's own extent is driven by its
// parent, not by its children's positions/sizes).
func (c *Canvas) MeasureContent(available render.Size) render.Size {
	inf := render.Size{W: float32(math.Inf(1)), H: float32(math.Inf(1))}
	for _, item := range c.items {
		core.MeasureWidget(item.child, inf)
	}
	return render.Size{}
}

// ArrangeContent places each child at bounds.{X,Y} offset by its (x, y),
// using its own desired size as the slot; ArrangeWidget's own margin inset
// then nets the child's final bounds down to childDesired-margins (per
// axis, floored at zero) — the same pattern StackPanel/DockPanel use, since
// margins are only reachable through ArrangeWidget, not from this package.
// Every child is always arranged, even hidden ones — ArrangeWidget zeroes a
// hidden child's bounds itself.
func (c *Canvas) ArrangeContent(bounds render.Rect) {
	for _, item := range c.items {
		desired := core.DesiredSizeOf(item.child)
		slot := render.Rect{
			X: bounds.X + item.x,
			Y: bounds.Y + item.y,
			W: desired.W,
			H: desired.H,
		}
		core.ArrangeWidget(item.child, slot)
	}
}
