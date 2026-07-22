// Package controls provides fluo's built-in widgets.
package controls

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
)

// Fixed is a solid-color fixed-size block, useful as a spacer, a color
// swatch, or a lightweight test widget with a known desired size.
type Fixed struct {
	core.Element

	w, h  float32
	color render.Color
}

// NewFixed returns a Fixed that always measures to (w, h) and paints its
// bounds with c.
func NewFixed(w, h float32, c render.Color) *Fixed {
	return &Fixed{w: w, h: h, color: c}
}

// MeasureContent returns the fixed (w, h) size regardless of the space
// available.
func (f *Fixed) MeasureContent(available render.Size) render.Size {
	return render.Size{W: f.w, H: f.h}
}

// Render fills the widget's bounds with its color; fully transparent
// colors (A == 0) are skipped.
func (f *Fixed) Render(r render.Renderer) {
	if f.color.A == 0 {
		return
	}
	r.FillRect(f.Bounds(), f.color)
}
