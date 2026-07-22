package core

import (
	"math"

	"github.com/0xdreadnaught/fluo/render"
)

// Widget is implemented by embedding Element (which supplies element()) and
// defining content behavior. External packages CAN implement it: embedding
// core.Element promotes the unexported element() method, so any struct that
// embeds Element and overrides zero or more of the content methods
// satisfies this interface.
type Widget interface {
	element() *Element
	// MeasureContent returns the content's desired size given the space
	// available to it (already inset by margins/explicit size/min-max).
	// available may carry +Inf components; implementations must be
	// Inf-safe and must return a finite size.
	MeasureContent(available render.Size) render.Size
	// ArrangeContent positions/arranges children within the widget's own
	// absolute bounds (already computed by the engine).
	ArrangeContent(bounds render.Rect)
	// Render draws the widget itself only; children are drawn separately
	// by RenderWidget.
	Render(r render.Renderer)
	// Children returns this widget's children, or nil for leaves.
	Children() []Widget
}

// SetParent records parent as child's layout parent. Container widgets call
// this from their Add (or equivalent) methods. A nil parent detaches child:
// its future invalidations no longer climb into any ancestor.
//
// Fail-fast double-parenting guard: if parent is non-nil and child already
// has a different non-nil parent, SetParent panics rather than silently
// re-homing the child (which would leave the old parent's child slice
// pointing at a widget that no longer reports it as an ancestor). Detach
// first with SetParent(child, nil) before re-adding elsewhere. Re-setting
// the same parent is a no-op; setting nil is always allowed.
func SetParent(child, parent Widget) {
	e := child.element()
	if parent != nil && e.parent != nil && e.parent != parent {
		panic("core: widget already has a parent; detach it (SetParent(w, nil)) before re-adding")
	}
	e.parent = parent
}

// DesiredSizeOf returns w's desired size as computed by the last MeasureWidget
// call. It is the way parents (and external custom panels) read a child's
// measurement back — Widget itself deliberately exposes no getters.
func DesiredSizeOf(w Widget) render.Size {
	return w.element().desired
}

// BoundsOf returns w's arranged bounds in absolute window space, as computed
// by the last ArrangeWidget call. Valid after ArrangeWidget has run; like
// DesiredSizeOf, it exists so parents (and external custom panels) can read
// a child's layout result back without Widget itself exposing getters.
func BoundsOf(w Widget) render.Rect {
	return w.element().bounds
}

// IsVisible reports whether w participates in layout and rendering.
// Parents use it to decide gap/extent contribution for hidden children.
func IsVisible(w Widget) bool {
	return !w.element().hidden
}

// clampAxis clamps v into [min, max]. max <= 0 means "no maximum" (+Inf).
// Note: min/max are always finite (set via SetMinSize/SetMaxSize), and v may
// be +Inf; Inf compared against a finite max simply clamps down to that max,
// and Inf minus a finite value stays Inf — no Inf-Inf subtraction ever
// occurs here, so no NaN can arise.
func clampAxis(v, min, max float32) float32 {
	if max <= 0 {
		max = float32(math.Inf(1))
	}
	if v < min {
		v = min
	}
	if v > max {
		v = max
	}
	return v
}

// insetSize subtracts a margin from a size, clamping each axis to >= 0.
func insetSize(s render.Size, m render.Thickness) render.Size {
	w := s.W - m.Left - m.Right
	if w < 0 {
		w = 0
	}
	h := s.H - m.Top - m.Bottom
	if h < 0 {
		h = 0
	}
	return render.Size{W: w, H: h}
}

// insetRect insets a rect by a margin, clamping W/H to >= 0.
func insetRect(r render.Rect, m render.Thickness) render.Rect {
	ir := r.Inset(m)
	if ir.W < 0 {
		ir.W = 0
	}
	if ir.H < 0 {
		ir.H = 0
	}
	return ir
}

// MeasureWidget is the only entry point that runs a widget's measure pass.
// It takes the Widget interface (not *Element) so that a concrete widget's
// overridden MeasureContent is the one invoked, and reads/writes layout
// state through w.element().
func MeasureWidget(w Widget, available render.Size) {
	e := w.element()

	if e.hidden {
		e.desired = render.Size{}
		e.measureClean = true
		return
	}

	// 2: inner = available minus margins.
	inner := insetSize(available, e.margin)

	// 3: explicit size wins on the space offered to content.
	if e.width > 0 {
		inner.W = e.width
	}
	if e.height > 0 {
		inner.H = e.height
	}

	// 4: clamp to [min, max].
	inner.W = clampAxis(inner.W, e.minW, e.maxW)
	inner.H = clampAxis(inner.H, e.minH, e.maxH)

	// 5: ask the widget for its content's desired size.
	content := w.MeasureContent(inner)

	// 6: explicit size wins again on the resulting content size, then
	// re-clamp to [min, max].
	if e.width > 0 {
		content.W = e.width
	}
	if e.height > 0 {
		content.H = e.height
	}
	content.W = clampAxis(content.W, e.minW, e.maxW)
	content.H = clampAxis(content.H, e.minH, e.maxH)

	// 7: desired = content + margins.
	e.desired = render.Size{
		W: content.W + e.margin.Left + e.margin.Right,
		H: content.H + e.margin.Top + e.margin.Bottom,
	}
	e.measureClean = true
}

// ArrangeWidget is the only entry point that runs a widget's arrange pass.
// It takes the Widget interface for the same dispatch reason as
// MeasureWidget.
func ArrangeWidget(w Widget, final render.Rect) {
	e := w.element()

	if e.hidden {
		e.bounds = render.Rect{}
		e.arrangeClean = true
		return
	}

	// 2: slot = final inset by margins.
	slot := insetRect(final, e.margin)

	// 3: contentDesired per axis = max(0, desired - margins), capped at slot.
	cdw := e.desired.W - e.margin.Left - e.margin.Right
	if cdw < 0 {
		cdw = 0
	}
	if cdw > slot.W {
		cdw = slot.W
	}
	cdh := e.desired.H - e.margin.Top - e.margin.Bottom
	if cdh < 0 {
		cdh = 0
	}
	if cdh > slot.H {
		cdh = slot.H
	}

	// 4: per-axis size/position.
	var x, y, w2, h2 float32

	if e.halign == Stretch && !(e.width > 0) {
		w2 = slot.W
		x = slot.X
	} else {
		w2 = cdw
		switch e.halign {
		case Start:
			x = slot.X
		case End:
			x = slot.X + slot.W - w2
		default: // Center, or Stretch with an explicit width
			x = slot.X + (slot.W-w2)/2
		}
	}

	if e.valign == Stretch && !(e.height > 0) {
		h2 = slot.H
		y = slot.Y
	} else {
		h2 = cdh
		switch e.valign {
		case Start:
			y = slot.Y
		case End:
			y = slot.Y + slot.H - h2
		default: // Center, or Stretch with an explicit height
			y = slot.Y + (slot.H-h2)/2
		}
	}

	// 5: bounds is absolute; arrange content within it, then clear dirty.
	e.bounds = render.Rect{X: x, Y: y, W: w2, H: h2}
	w.ArrangeContent(e.bounds)
	e.arrangeClean = true
}

// RenderWidget draws w and, in order, its children. Hidden widgets (and
// their entire subtree) are skipped.
func RenderWidget(w Widget, r render.Renderer) {
	e := w.element()
	if e.hidden {
		return
	}
	w.Render(r)
	for _, c := range w.Children() {
		RenderWidget(c, r)
	}
}
