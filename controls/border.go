package controls

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
)

// Border is a single-child decorator that draws a background, an optional
// stroke, and optional padding around its child.
type Border struct {
	core.Element

	child core.Widget

	background  render.Color
	borderColor render.Color
	borderWidth float32
	radius      float32
	padding     render.Thickness
}

// NewBorder returns an empty Border with no child, no padding, and no
// stroke.
func NewBorder() *Border {
	return &Border{}
}

// SetChild sets (replacing any existing) the single child, re-parenting it
// to this Border and invalidating measure.
func (b *Border) SetChild(w core.Widget) *Border {
	b.child = w
	core.SetParent(w, b)
	b.InvalidateMeasure()
	return b
}

// SetBackground sets the fill color drawn behind the border/child. Purely
// visual: no invalidation needed since the host redraws every frame.
func (b *Border) SetBackground(c render.Color) *Border {
	b.background = c
	return b
}

// SetBorder sets the stroke color and width. A width change affects layout
// (it eats into the child's available space), so it invalidates measure.
func (b *Border) SetBorder(c render.Color, width float32) *Border {
	changed := b.borderWidth != width
	b.borderColor = c
	b.borderWidth = width
	if changed {
		b.InvalidateMeasure()
	}
	return b
}

// SetRadius sets the corner radius used when drawing the background and
// stroke. Purely visual: no invalidation.
func (b *Border) SetRadius(r float32) *Border {
	b.radius = r
	return b
}

// SetPadding sets the space between the border chrome and the child.
// Layout-relevant: invalidates measure.
func (b *Border) SetPadding(t render.Thickness) *Border {
	b.padding = t
	b.InvalidateMeasure()
	return b
}

// sizer is satisfied by any core.Widget (every concrete widget embeds
// core.Element, which exports DesiredSize). core.Widget itself only
// requires the unexported element() method, so childDesiredSize asserts to
// this interface to read back what core.MeasureWidget just computed.
type sizer interface {
	DesiredSize() render.Size
}

// childDesiredSize returns w's desired size as recorded by the last
// MeasureWidget call.
func childDesiredSize(w core.Widget) render.Size {
	if s, ok := w.(sizer); ok {
		return s.DesiredSize()
	}
	return render.Size{}
}

// chrome returns the total inset (padding + borderWidth on all four sides).
func (b *Border) chrome() render.Thickness {
	bw := b.borderWidth
	return render.Thickness{
		Left:   b.padding.Left + bw,
		Top:    b.padding.Top + bw,
		Right:  b.padding.Right + bw,
		Bottom: b.padding.Bottom + bw,
	}
}

// MeasureContent measures the child (if any) with the available space
// reduced by chrome, then adds the chrome back to the child's desired size.
// With no child, the desired size is the chrome alone.
func (b *Border) MeasureContent(available render.Size) render.Size {
	c := b.chrome()

	availW := available.W - c.Left - c.Right
	if availW < 0 {
		availW = 0
	}
	availH := available.H - c.Top - c.Bottom
	if availH < 0 {
		availH = 0
	}

	var childW, childH float32
	if b.child != nil {
		core.MeasureWidget(b.child, render.Size{W: availW, H: availH})
		d := childDesiredSize(b.child)
		childW, childH = d.W, d.H
	}

	return render.Size{
		W: childW + c.Left + c.Right,
		H: childH + c.Top + c.Bottom,
	}
}

// ArrangeContent arranges the child (if any) within bounds inset by chrome.
func (b *Border) ArrangeContent(bounds render.Rect) {
	if b.child == nil {
		return
	}
	c := b.chrome()
	inner := bounds.Inset(c)
	if inner.W < 0 {
		inner.W = 0
	}
	if inner.H < 0 {
		inner.H = 0
	}
	core.ArrangeWidget(b.child, inner)
}

// Render draws the background fill (if visible) and the stroke (if
// visible).
func (b *Border) Render(r render.Renderer) {
	bounds := b.Bounds()
	if b.background.A > 0 {
		r.FillRoundedRect(bounds, b.radius, b.background)
	}
	if b.borderWidth > 0 && b.borderColor.A > 0 {
		r.StrokeRoundedRect(bounds, b.radius, b.borderWidth, b.borderColor)
	}
}

// Children returns the single child in a slice, or nil if there is none.
func (b *Border) Children() []core.Widget {
	if b.child == nil {
		return nil
	}
	return []core.Widget{b.child}
}
