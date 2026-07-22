package controls

import (
	"math"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/theme"
)

// scrollThumbMinH is the minimum height a drawn thumb is ever shrunk to,
// regardless of how large the content-to-viewport ratio is. Structural, not
// themed.
const scrollThumbMinH float32 = 24

// scrollWheelStep is the number of logical px scrolled per wheel notch.
const scrollWheelStep float32 = 48

// ScrollViewer scrolls a single child vertically (v0: no horizontal bar; a
// horizontal scrollbar/axis is a later phase). It clips its child to its own
// bounds, draws an overlay thumb on the right when the child is taller than
// the viewport, and responds to mouse wheel and thumb-drag input.
type ScrollViewer struct {
	core.Element

	child core.Widget

	// rawOffset is the last value requested via ScrollTo/ScrollBy, before
	// clamping. offset is the clamped value as of the last ArrangeContent
	// call — ArrangeContent is the single source of truth for clamping (see
	// its doc comment).
	rawOffset float32
	offset    float32

	// viewport and childH are the viewport rect and the child's desired
	// content height as of the last ArrangeContent call. Both are needed to
	// compute the thumb geometry and to drive drag math from OnPointer.
	viewport render.Rect
	childH   float32

	// dragGrabY is the y-offset (in logical px) between the pointer position
	// and the thumb's top edge at the moment a thumb drag began, so the thumb
	// tracks the pointer at a fixed grab point rather than snapping its top
	// edge to the cursor.
	dragGrabY float32

	// gutter, thumbColor, and thumbRadius are captured from theme.Active()
	// at construction (see NewScrollViewer); structural constants (thumb
	// min height, wheel step) are not themed.
	gutter      float32
	thumbColor  render.Color
	thumbRadius float32
}

// NewScrollViewer returns an empty ScrollViewer with no child and offset 0,
// styled from theme.Active() at construction; rebuild to re-theme.
func NewScrollViewer() *ScrollViewer {
	t := theme.Active()
	return &ScrollViewer{
		gutter:      t.Metric.ScrollGutter,
		thumbColor:  t.Color.ScrollThumb,
		thumbRadius: t.Metric.ControlCornerRadius,
	}
}

// SetChild sets (replacing any existing) the single scrolled child,
// re-parenting it to this ScrollViewer and invalidating measure. Any
// previously set child is detached (its parent cleared), matching the
// Border convention: its future invalidations stop climbing into this
// ScrollViewer.
func (s *ScrollViewer) SetChild(w core.Widget) *ScrollViewer {
	if s.child != nil {
		core.SetParent(s.child, nil)
	}
	s.child = w
	core.SetParent(w, s)
	s.InvalidateMeasure()
	return s
}

// OffsetY returns the current vertical scroll offset, clamped to
// [0, max(0, childH-viewportH)] as of the last arrange pass.
func (s *ScrollViewer) OffsetY() float32 {
	return s.offset
}

// ScrollTo requests a new vertical offset. The value is stored raw and
// clamped on the next arrange pass (ArrangeContent is the single source of
// truth for clamping), so OffsetY may not reflect y until layout runs again.
func (s *ScrollViewer) ScrollTo(y float32) *ScrollViewer {
	s.rawOffset = y
	s.InvalidateArrange()
	return s
}

// ScrollBy requests a relative change to the vertical offset, clamped on the
// next arrange pass like ScrollTo.
func (s *ScrollViewer) ScrollBy(dy float32) {
	s.rawOffset += dy
	s.InvalidateArrange()
}

// Children returns the single scrolled child in a slice, or nil if there is
// none. Returns a copy; mutating it does not affect the viewer.
func (s *ScrollViewer) Children() []core.Widget {
	if s.child == nil {
		return nil
	}
	return []core.Widget{s.child}
}

// MeasureContent measures the child (if any) with the available width
// reduced by the thumb gutter and unbounded height (so the child reports its
// full natural content height), then reports the min of (child size + gutter
// on the width axis) and the available size per axis — a ScrollViewer never
// asks its parent for more room than it was offered, even if its content is
// taller/wider.
func (s *ScrollViewer) MeasureContent(available render.Size) render.Size {
	childAvailW := available.W - s.gutter
	if childAvailW < 0 {
		childAvailW = 0
	}
	childAvail := render.Size{W: childAvailW, H: float32(math.Inf(1))}

	var childW, childH float32
	if s.child != nil {
		core.MeasureWidget(s.child, childAvail)
		d := core.DesiredSizeOf(s.child)
		childW, childH = d.W, d.H
	}

	desiredW := childW + s.gutter
	if desiredW > available.W {
		desiredW = available.W
	}
	desiredH := childH
	if desiredH > available.H {
		desiredH = available.H
	}
	return render.Size{W: desiredW, H: desiredH}
}

// ArrangeContent is the single source of truth for offset clamping: it
// computes the viewport (bounds minus the thumb gutter on the right),
// clamps rawOffset into [0, max(0, childH-viewportH)], stores the clamped
// result (read back via OffsetY), and arranges the child at
// {viewport.X, viewport.Y-offset, viewport.W, childDesiredH} so the clip
// (see ClipRect) crops whatever scrolls above/below the viewport.
func (s *ScrollViewer) ArrangeContent(bounds render.Rect) {
	viewport := bounds.Inset(render.Thickness{Right: s.gutter})
	if viewport.W < 0 {
		viewport.W = 0
	}

	var childH float32
	if s.child != nil {
		childH = core.DesiredSizeOf(s.child).H
	}

	maxOffset := childH - viewport.H
	if maxOffset < 0 {
		maxOffset = 0
	}
	offset := s.rawOffset
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}

	s.viewport = viewport
	s.childH = childH
	s.offset = offset

	if s.child != nil {
		core.ArrangeWidget(s.child, render.Rect{
			X: viewport.X,
			Y: viewport.Y - offset,
			W: viewport.W,
			H: childH,
		})
	}
}

// ClipRect implements core.ClipProvider, clipping the child to the
// ScrollViewer's own full bounds (thumb gutter included, so the thumb itself
// — drawn in RenderOverlay, which runs after the clip is popped — is never
// cropped).
func (s *ScrollViewer) ClipRect() (render.Rect, bool) {
	return s.Bounds(), true
}

// thumbGeometry returns the thumb's track (the right gutter strip) and its
// height, independent of the current scroll offset, or ok==false when there
// is no child or the child fits entirely within the viewport (nothing to
// scroll, so no thumb is drawn/hit-testable).
func (s *ScrollViewer) thumbGeometry() (track render.Rect, thumbH float32, ok bool) {
	if s.viewport.H <= 0 || s.childH <= s.viewport.H {
		return render.Rect{}, 0, false
	}
	track = render.Rect{
		X: s.viewport.Right(),
		Y: s.viewport.Y,
		W: s.gutter,
		H: s.viewport.H,
	}
	thumbH = track.H * track.H / s.childH
	if thumbH < scrollThumbMinH {
		thumbH = scrollThumbMinH
	}
	if thumbH > track.H {
		thumbH = track.H
	}
	return track, thumbH, true
}

// thumbRect returns the thumb's current on-screen rect (track position plus
// the vertical offset proportional to the current scroll offset), or
// ok==false when there is nothing to scroll (see thumbGeometry).
func (s *ScrollViewer) thumbRect() (render.Rect, bool) {
	track, thumbH, ok := s.thumbGeometry()
	if !ok {
		return render.Rect{}, false
	}
	maxOffset := s.childH - s.viewport.H
	thumbY := track.Y
	if maxOffset > 0 {
		thumbY = track.Y + (s.offset/maxOffset)*(track.H-thumbH)
	}
	return render.Rect{X: track.X, Y: thumbY, W: track.W, H: thumbH}, true
}

// RenderOverlay implements core.OverlayRenderer, drawing the thumb (a
// rounded-rect fill in the theme's ScrollThumb color, captured at
// construction) above the clipped child when there is content to scroll to.
func (s *ScrollViewer) RenderOverlay(r render.Renderer) {
	rect, ok := s.thumbRect()
	if !ok {
		return
	}
	r.FillRoundedRect(rect, s.thumbRadius, s.thumbColor)
}

// dragTo recomputes rawOffset from a drag's current pointer y-position,
// keeping the pointer at the same relative grab point within the thumb
// (dragGrabY) that it was at when the drag began, and invalidates arrange so
// the next layout pass clamps and applies it.
func (s *ScrollViewer) dragTo(posY float32) {
	track, thumbH, ok := s.thumbGeometry()
	if !ok {
		return
	}
	maxOffset := s.childH - s.viewport.H
	if maxOffset <= 0 {
		return
	}
	span := track.H - thumbH
	thumbY := posY - s.dragGrabY
	if thumbY < track.Y {
		thumbY = track.Y
	}
	if thumbY > track.Y+span {
		thumbY = track.Y + span
	}
	var frac float32
	if span > 0 {
		frac = (thumbY - track.Y) / span
	}
	s.rawOffset = frac * maxOffset
	s.InvalidateArrange()
}

// OnPointer implements input.PointerHandler: Wheel scrolls by
// scrollWheelStep logical px per notch and is always handled; a Press inside
// the current thumb rect starts a drag (capturing the pointer via
// e.Router.Capture and recording the grab offset) and is handled, while a
// Press elsewhere is left unhandled so it bubbles through to the scrolled
// content; Move/Release are only acted on while this ScrollViewer holds the
// capture (drag in progress).
func (s *ScrollViewer) OnPointer(e *input.PointerEvent) {
	switch e.Action {
	case input.Wheel:
		s.ScrollBy(-e.Delta.Y * scrollWheelStep)
		e.Handled = true
	case input.Press:
		if rect, ok := s.thumbRect(); ok && rect.Contains(e.Pos) {
			s.dragGrabY = e.Pos.Y - rect.Y
			e.Router.Capture(s)
			e.Handled = true
		}
	case input.Move:
		if e.Router.Captured() == s {
			s.dragTo(e.Pos.Y)
			e.Handled = true
		}
	case input.Release:
		if e.Router.Captured() == s {
			e.Router.Release()
			e.Handled = true
		}
	}
}
