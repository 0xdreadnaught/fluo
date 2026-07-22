package core

import "github.com/0xdreadnaught/fluo/render"

// Auto is the sentinel meaning "size to content" for Width/Height. Note that
// the zero value of a float32 (0) also means auto — see the note on Element
// below — so Auto exists purely for readability at call sites.
const Auto float32 = -1

// Alignment controls how a widget is positioned (and possibly sized) within
// the slot its parent allocates to it along one axis.
type Alignment uint8

const (
	Stretch Alignment = iota
	Start
	Center
	End
)

// Element is the embeddable base of every widget. Its ZERO VALUE is a valid
// widget state: visible, auto-sized, Stretch-aligned, dirty (needs layout).
//
// Zero-value notes:
//   - width/height: only a value > 0 counts as an explicit size. Both the
//     zero value (0) and Auto (-1) mean "size to content".
//   - maxW/maxH: <= 0 means "no maximum" (treated as +Inf). minW/minH have no
//     such special-casing; their zero value (0) is simply "no minimum".
//   - hidden: zero value false, so a fresh Element is visible.
//   - halign/valign: zero value Stretch (iota 0).
//   - measureClean/arrangeClean are the dirty flags stored INVERTED so that
//     the zero value (false) means "dirty" — a fresh Element always reports
//     NeedsLayout() == true without any constructor.
type Element struct {
	margin render.Thickness

	width, height          float32
	minW, minH, maxW, maxH float32

	halign, valign Alignment

	hidden bool

	desired render.Size
	bounds  render.Rect

	measureClean bool
	arrangeClean bool

	parent Widget
}

// element implements the unexported half of Widget; embedding Element
// promotes this method so external packages can satisfy Widget.
func (e *Element) element() *Element { return e }

// Default Widget content behavior: a bare Element embed is a valid, empty,
// leaf widget.
func (e *Element) MeasureContent(available render.Size) render.Size { return render.Size{} }
func (e *Element) ArrangeContent(bounds render.Rect)                {}
func (e *Element) Render(r render.Renderer)                         {}
func (e *Element) Children() []Widget                               { return nil }

// SetMargin sets the outer margin. Layout-relevant: invalidates measure.
func (e *Element) SetMargin(t render.Thickness) {
	e.margin = t
	e.InvalidateMeasure()
}

// SetWidth fixes the width axis when w > 0; w <= 0 means auto (size to content).
func (e *Element) SetWidth(w float32) {
	e.width = w
	e.InvalidateMeasure()
}

// SetHeight fixes the height axis when h > 0; h <= 0 means auto.
func (e *Element) SetHeight(h float32) {
	e.height = h
	e.InvalidateMeasure()
}

// SetMinSize sets the minimum content size per axis.
func (e *Element) SetMinSize(w, h float32) {
	e.minW = w
	e.minH = h
	e.InvalidateMeasure()
}

// SetMaxSize sets the maximum content size per axis; <= 0 means unbounded.
func (e *Element) SetMaxSize(w, h float32) {
	e.maxW = w
	e.maxH = h
	e.InvalidateMeasure()
}

// SetAlign sets horizontal/vertical alignment. Purely an arrange-time
// concern, so it only invalidates arrange.
func (e *Element) SetAlign(h, v Alignment) {
	e.halign = h
	e.valign = v
	e.InvalidateArrange()
}

// SetVisible toggles visibility. Either direction changes what the parent
// sees during its own measure pass (a hidden widget measures to zero), so
// both transitions invalidate measure (which bubbles to the parent chain).
func (e *Element) SetVisible(v bool) {
	e.hidden = !v
	e.InvalidateMeasure()
}

// Visible reports whether the widget is visible.
func (e *Element) Visible() bool { return !e.hidden }

// DesiredSize returns the size computed by the last MeasureWidget call.
func (e *Element) DesiredSize() render.Size { return e.desired }

// Bounds returns the absolute rect computed by the last ArrangeWidget call.
func (e *Element) Bounds() render.Rect { return e.bounds }

// InvalidateMeasure marks this element (and, transitively, its ancestors) as
// needing both re-measure and re-arrange. The walk up the parent chain stops
// as soon as it reaches an ancestor that was already measure-dirty, since
// that ancestor's own ancestors must already be dirty too.
func (e *Element) InvalidateMeasure() {
	e.measureClean = false
	e.arrangeClean = false
	p := e.parent
	for p != nil {
		pe := p.element()
		wasDirty := !pe.measureClean
		pe.measureClean = false
		pe.arrangeClean = false
		if wasDirty {
			break
		}
		p = pe.parent
	}
}

// InvalidateArrange marks this element (and its ancestors) as needing
// re-arrange only. Stops climbing once it reaches an already arrange-dirty
// ancestor.
func (e *Element) InvalidateArrange() {
	e.arrangeClean = false
	p := e.parent
	for p != nil {
		pe := p.element()
		wasDirty := !pe.arrangeClean
		pe.arrangeClean = false
		if wasDirty {
			break
		}
		p = pe.parent
	}
}

// NeedsLayout reports whether this element requires a measure and/or
// arrange pass.
func (e *Element) NeedsLayout() bool {
	return !e.measureClean || !e.arrangeClean
}
