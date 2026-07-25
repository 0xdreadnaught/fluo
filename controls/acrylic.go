package controls

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/theme"
)

// AcrylicSurface is a single-child decorator that draws a backdrop-blurred,
// tinted "acrylic"/mica material behind its child, then renders the child on
// top. It is Border-like: SetChild, SetPadding, and SetRadius all behave the
// same way (measure/arrange delegate to the child, inset by padding), except
// the background is painted via render.Renderer.DrawBackdropBlur instead of
// a flat FillRoundedRect.
type AcrylicSurface struct {
	core.Element

	child core.Widget

	tint    render.Color
	radius  float32
	padding render.Thickness
}

// NewAcrylicSurface returns an empty AcrylicSurface with no child, no
// padding, and its tint from theme.Active().Color.AcrylicTint at
// construction time (rebuild to re-theme, matching every other themed
// control).
func NewAcrylicSurface() *AcrylicSurface {
	return &AcrylicSurface{tint: theme.Active().Color.AcrylicTint}
}

// SetChild sets (replacing any existing) the single child, re-parenting it
// to this AcrylicSurface and invalidating measure. Any previously set child
// is detached (its parent cleared) so its future invalidations stop
// climbing into this AcrylicSurface.
func (a *AcrylicSurface) SetChild(w core.Widget) *AcrylicSurface {
	if a.child != nil {
		core.SetParent(a.child, nil)
	}
	a.child = w
	core.SetParent(w, a)
	a.InvalidateMeasure()
	return a
}

// SetTint overrides the tint composited over the blurred backdrop (default:
// theme.Active().Color.AcrylicTint as of construction). Purely visual: no
// invalidation needed since the host redraws every frame.
func (a *AcrylicSurface) SetTint(c render.Color) *AcrylicSurface {
	a.tint = c
	return a
}

// SetRadius sets the corner radius used when drawing the acrylic surface.
// Purely visual: no invalidation.
func (a *AcrylicSurface) SetRadius(r float32) *AcrylicSurface {
	a.radius = r
	return a
}

// SetPadding sets the space between the acrylic surface's edge and the
// child. Layout-relevant: invalidates measure.
func (a *AcrylicSurface) SetPadding(t render.Thickness) *AcrylicSurface {
	a.padding = t
	a.InvalidateMeasure()
	return a
}

// MeasureContent measures the child (if any) with the available space
// reduced by padding, then adds the padding back to the child's desired
// size. With no child, the desired size is the padding alone.
func (a *AcrylicSurface) MeasureContent(available render.Size) render.Size {
	availW := available.W - a.padding.Left - a.padding.Right
	if availW < 0 {
		availW = 0
	}
	availH := available.H - a.padding.Top - a.padding.Bottom
	if availH < 0 {
		availH = 0
	}

	var childW, childH float32
	if a.child != nil {
		core.MeasureWidget(a.child, render.Size{W: availW, H: availH})
		d := core.DesiredSizeOf(a.child)
		childW, childH = d.W, d.H
	}

	return render.Size{
		W: childW + a.padding.Left + a.padding.Right,
		H: childH + a.padding.Top + a.padding.Bottom,
	}
}

// ArrangeContent arranges the child (if any) within bounds inset by padding.
func (a *AcrylicSurface) ArrangeContent(bounds render.Rect) {
	if a.child == nil {
		return
	}
	inner := bounds.Inset(a.padding)
	if inner.W < 0 {
		inner.W = 0
	}
	if inner.H < 0 {
		inner.H = 0
	}
	core.ArrangeWidget(a.child, inner)
}

// Render draws the backdrop-blur acrylic background. core.RenderWidget calls
// this BEFORE recursing into Children, so the child is always painted on top
// of the (already blurred+tinted) surface.
func (a *AcrylicSurface) Render(r render.Renderer) {
	r.DrawBackdropBlur(a.Bounds(), a.radius, a.tint)
}

// Children returns the single child in a slice, or nil if there is none.
// Returns a copy; mutating it does not affect the panel.
func (a *AcrylicSurface) Children() []core.Widget {
	if a.child == nil {
		return nil
	}
	return []core.Widget{a.child}
}
