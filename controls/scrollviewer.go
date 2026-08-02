package controls

import (
	"math"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/theme"
)

// scrollThumbMinH is the minimum height/width a drawn thumb is ever shrunk
// to, regardless of how large the content-to-viewport ratio is on either
// axis. Structural, not themed.
const scrollThumbMinH float32 = 24

// scrollWheelStep is the number of logical px scrolled per wheel notch.
const scrollWheelStep float32 = 48

// scrollDragAxis records which axis a thumb-drag capture is currently
// tracking, so a captured Move/Release (which carries no information about
// which thumb was originally pressed) knows whether to update the
// vertical or horizontal offset. Set on Press when a drag begins, reset to
// scrollDragNone on Release.
type scrollDragAxis uint8

const (
	scrollDragNone scrollDragAxis = iota
	scrollDragVertical
	scrollDragHorizontal
)

// ScrollViewer scrolls a single child on both axes: vertically (the
// original v0 behavior) and horizontally (added alongside it — see the
// field/method doc comments below for the X-axis counterparts of every
// Y-axis member). It clips its child to its own bounds, draws overlay
// thumbs — vertical on the right, horizontal along the bottom — when the
// child overflows the corresponding axis, and responds to mouse wheel and
// thumb-drag input on either axis.
//
// The two axes are NOT symmetric, by design, to keep a ScrollViewer whose
// content only overflows vertically byte-identical to the original
// single-axis implementation:
//   - The vertical thumb's gutter (on the right) is reserved
//     UNCONDITIONALLY — regardless of whether the content is actually
//     taller than the viewport — exactly as before horizontal scrolling
//     existed (see MeasureContent/ArrangeContent). This is existing,
//     load-bearing behavior: TestScrollViewerThemeMetrics asserts the
//     child is always measured with width reduced by the gutter, even for
//     non-overflowing content.
//   - The horizontal thumb's gutter (on the bottom) is reserved only when
//     the content's natural width actually exceeds the ScrollViewer's own
//     OUTER bounds width (not the vertical-gutter-reduced inner viewport
//     width) — see ArrangeContent's doc comment for why the comparison
//     deliberately uses the unreduced bounds width.
type ScrollViewer struct {
	core.Element

	child core.Widget

	// rawOffset/rawOffsetX are the last values requested via
	// ScrollTo/ScrollBy (vertical) and ScrollToX/ScrollByX (horizontal),
	// before clamping. offset/offsetX are the clamped values as of the
	// last ArrangeContent call — ArrangeContent is the single source of
	// truth for clamping on both axes (see its doc comment), and folds each
	// clamped result back into its raw counterpart, so the pair only ever
	// disagrees for requests made since that pass.
	rawOffset, offset   float32
	rawOffsetX, offsetX float32

	// viewport is the viewport rect as of the last ArrangeContent call —
	// bounds inset by the always-on vertical gutter (right) and the
	// conditional horizontal gutter (bottom), covering both axes' thumb
	// geometry and drag math. childW/childH are the child's desired
	// content size as of that same call.
	viewport render.Rect
	childW   float32
	childH   float32

	// dragGrabY/dragGrabX are the offset (in logical px) between the
	// pointer position and the relevant thumb's near edge at the moment a
	// drag began, so the thumb tracks the pointer at a fixed grab point
	// rather than snapping its edge to the cursor. drag records which
	// axis dragGrabY/dragGrabX applies to for the duration of a capture.
	dragGrabY float32
	dragGrabX float32
	drag      scrollDragAxis

	// gutter is captured from theme.Active() at construction (see
	// NewScrollViewer); structural constants (thumb min height, wheel
	// step) are not themed. colors is the full classic token set, needed
	// by RenderOverlay's drawScrollThumb calls (raised thumb bevel +
	// track) on both axes.
	gutter float32
	colors theme.ColorTokens
}

// NewScrollViewer returns an empty ScrollViewer with no child and offset 0,
// styled from theme.Active() at construction; rebuild to re-theme.
func NewScrollViewer() *ScrollViewer {
	t := theme.Active()
	return &ScrollViewer{
		gutter: t.Metric.ScrollGutter,
		colors: t.Color,
	}
}

// SetChild sets (replacing any existing) the single scrolled child,
// re-parenting it to this ScrollViewer and invalidating measure. Any
// previously set child is detached (its parent cleared), matching the
// Border convention: its future invalidations stop climbing into this
// ScrollViewer.
//
// SetChild(nil) clears the child outright rather than panicking: Children,
// MeasureContent, ArrangeContent and thumbGeometry all already handle a nil
// child, so an empty ScrollViewer is a supported state.
func (s *ScrollViewer) SetChild(w core.Widget) *ScrollViewer {
	if s.child != nil {
		core.SetParent(s.child, nil)
	}
	s.child = w
	if w != nil {
		core.SetParent(w, s)
	}
	s.InvalidateMeasure()
	return s
}

// OffsetY returns the current vertical scroll offset, clamped to
// [0, max(0, childH-viewportH)] as of the last arrange pass.
func (s *ScrollViewer) OffsetY() float32 {
	return s.offset
}

// OffsetX returns the current horizontal scroll offset, clamped to
// [0, max(0, childW-viewportW)] as of the last arrange pass.
func (s *ScrollViewer) OffsetX() float32 {
	return s.offsetX
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

// ScrollToX requests a new horizontal offset, clamped on the next arrange
// pass like ScrollTo.
func (s *ScrollViewer) ScrollToX(x float32) *ScrollViewer {
	s.rawOffsetX = x
	s.InvalidateArrange()
	return s
}

// ScrollByX requests a relative change to the horizontal offset, clamped on
// the next arrange pass like ScrollBy.
func (s *ScrollViewer) ScrollByX(dx float32) {
	s.rawOffsetX += dx
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
// reduced by the (always-on) vertical thumb gutter and unbounded height (so
// the child reports its full natural content height), then reports the min
// of (child size + gutter on the width axis) and the available size per
// axis — a ScrollViewer never asks its parent for more room than it was
// offered, even if its content is taller/wider.
//
// Unchanged from the pre-horizontal-scroll implementation: the width
// offered to the child stays bounded (not unbounded like height), so
// content that adapts its own reported size to the available budget (e.g.
// wrapping text) still wraps to that budget rather than reporting some
// larger "natural" width. Only content whose own MeasureContent ignores
// available width (Fixed, or a StackPanel whose children do) can report a
// desired width exceeding the viewport — enabling horizontal scroll for it
// via ArrangeContent's full-width arrange (see its doc comment). This
// mirrors height's own pre-existing asymmetry (childAvailH was already
// unconditionally infinite) and keeps TestScrollViewerThemeMetrics — which
// asserts this exact bounded width is what the child is measured with — and
// every other consumer of the existing measure contract byte-identical.
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

// ArrangeContent is the single source of truth for offset clamping on both
// axes. It:
//  1. Reserves the vertical thumb's gutter on the right, UNCONDITIONALLY —
//     exactly as before horizontal scrolling existed (see the type doc
//     comment's asymmetry note).
//  2. Decides whether to reserve the horizontal thumb's gutter on the
//     bottom by comparing the child's natural width against bounds.W — the
//     ScrollViewer's own OUTER width, NOT the vertical-gutter-reduced inner
//     viewport width computed in step 1. This mirrors how the vertical
//     overflow decision has always effectively compared against bounds.H
//     (which nothing else ever reduces). Comparing against the reduced
//     inner viewport instead would make ANY existing vertical-only
//     ScrollViewer whose content's natural width happens to equal
//     bounds.W — as the pre-existing scroll.png golden's fixture does,
//     Fixed(120,...) children inside a 120-wide viewer — newly grow a
//     horizontal thumb, breaking a golden that predates this feature and
//     was never meant to scroll horizontally. The tradeoff: content whose
//     natural width sits strictly between (bounds.W - vgutter) and
//     bounds.W is arranged slightly wider than the inner viewport with no
//     thumb ever shown for that sliver — a narrow, harmless edge case (see
//     ArrangeWidget below).
//  3. Clamps rawOffset/rawOffsetX into [0, max(0, childH/W-viewportH/W)],
//     stores the clamped results (read back via OffsetY/OffsetX) and folds
//     them back into rawOffset/rawOffsetX so the unbounded accumulators
//     can't drift past the end stop.
//  4. Arranges the child at {viewport.X-offsetX, viewport.Y-offset,
//     arrangeW, childH} — arrangeW is at least viewport.W (preserving the
//     original Stretch-to-fill behavior for non-overflowing content) but
//     extends to the child's full desired width when that exceeds
//     viewport.W, letting it overflow horizontally exactly as childH
//     already lets it overflow vertically. The clip (see ClipRect) crops
//     whatever scrolls past the viewport on either axis.
func (s *ScrollViewer) ArrangeContent(bounds render.Rect) {
	var childW, childH float32
	if s.child != nil {
		d := core.DesiredSizeOf(s.child)
		childW, childH = d.W, d.H
	}

	viewport := bounds.Inset(render.Thickness{Right: s.gutter})
	if viewport.W < 0 {
		viewport.W = 0
	}

	hGutter := float32(0)
	if childW > bounds.W {
		hGutter = s.gutter
	}
	viewport = viewport.Inset(render.Thickness{Bottom: hGutter})
	if viewport.H < 0 {
		viewport.H = 0
	}

	offset := clampScrollOffset(s.rawOffset, childH, viewport.H)
	offsetX := clampScrollOffset(s.rawOffsetX, childW, viewport.W)

	s.viewport = viewport
	s.childW = childW
	s.childH = childH
	s.offset = offset
	s.offsetX = offsetX

	// Fold the clamp back into the raw accumulators. ScrollBy/ScrollByX add
	// to them without bound, so holding a scroll at an end stop otherwise
	// leaves rawOffset far past the max while offset sits pinned at it — and
	// every subsequent step back has to first burn off all that accumulated
	// slack before the clamped offset moves at all, a dead zone as many
	// notches deep as the overshoot. The raw value's only job is to carry a
	// request from between two arrange passes; once this pass has resolved
	// it, the clamped result IS the current request.
	s.rawOffset = offset
	s.rawOffsetX = offsetX

	if s.child != nil {
		arrangeW := viewport.W
		if childW > arrangeW {
			arrangeW = childW
		}
		core.ArrangeWidget(s.child, render.Rect{
			X: viewport.X - offsetX,
			Y: viewport.Y - offset,
			W: arrangeW,
			H: childH,
		})
	}
}

// clampScrollOffset clamps a raw (pre-clamp) scroll offset into
// [0, max(0, contentLen-viewportLen)] — the single clamping rule both axes'
// offset/offsetX share (see ArrangeContent).
//
// A NaN raw offset clamps to 0 rather than passing through: every ordinary
// comparison against NaN is false, so a plain lo/hi pair of ifs would let it
// straight out the other side, and a NaN offset poisons everything derived
// from it downstream (virtualizer.visibleRange floors it into an int row
// index and the list collapses to zero rows for good). Treating it as the
// low bound keeps a single bad float from wedging the control.
func clampScrollOffset(raw, contentLen, viewportLen float32) float32 {
	maxOffset := contentLen - viewportLen
	if maxOffset < 0 {
		maxOffset = 0
	}
	if raw != raw { // NaN
		return 0
	}
	if raw < 0 {
		return 0
	}
	if raw > maxOffset {
		return maxOffset
	}
	return raw
}

// scrollThumbLength returns the drawn thumb's length along the scroll axis
// — trackLen*trackLen/contentLen, the standard track-proportional thumb
// sizing — clamped to [scrollThumbMinH, trackLen]. Shared by the vertical
// and horizontal thumb geometry (thumbGeometry/thumbGeometryX).
func scrollThumbLength(trackLen, contentLen float32) float32 {
	length := trackLen * trackLen / contentLen
	if length < scrollThumbMinH {
		length = scrollThumbMinH
	}
	if length > trackLen {
		length = trackLen
	}
	return length
}

// scrollThumbPos returns the thumb's current on-track position (the
// coordinate of its near edge, in the same space as trackStart) given the
// current offset proportional to maxOffset, or trackStart unchanged when
// there is no room to scroll (maxOffset <= 0). Shared by
// thumbRect/thumbRectX.
func scrollThumbPos(trackStart, trackLen, thumbLen, offset, maxOffset float32) float32 {
	if maxOffset <= 0 {
		return trackStart
	}
	return trackStart + (offset/maxOffset)*(trackLen-thumbLen)
}

// scrollDragOffset converts a drag's current pointer position
// (posAlongAxis) into a new raw offset, keeping the pointer at the same
// relative grab point within the thumb (posAlongAxis-grabOffset) that it
// was at when the drag began, clamped so the thumb's near edge stays within
// [trackStart, trackStart+trackLen-thumbLen]. Shared by dragTo/dragToX.
// Returns 0 when there is no room to scroll (maxOffset <= 0); callers must
// guard that case themselves if they want to skip the call entirely (both
// dragTo and dragToX do, matching the pre-existing dragTo behavior of
// leaving rawOffset untouched rather than resetting it to 0).
func scrollDragOffset(trackStart, trackLen, thumbLen, posAlongAxis, grabOffset, maxOffset float32) float32 {
	if maxOffset <= 0 {
		return 0
	}
	span := trackLen - thumbLen
	thumbPos := posAlongAxis - grabOffset
	if thumbPos < trackStart {
		thumbPos = trackStart
	}
	if thumbPos > trackStart+span {
		thumbPos = trackStart + span
	}
	var frac float32
	if span > 0 {
		frac = (thumbPos - trackStart) / span
	}
	return frac * maxOffset
}

// ClipRect implements core.ClipProvider, clipping the child to the
// ScrollViewer's own full bounds (both thumb gutters included, so the
// thumbs themselves — drawn in RenderOverlay, which runs after the clip is
// popped — are never cropped). Covers both axes unchanged: the full bounds
// were always the clip rect, on both X and Y, even before horizontal
// scrolling existed.
func (s *ScrollViewer) ClipRect() (render.Rect, bool) {
	return s.Bounds(), true
}

// thumbGeometry returns the vertical thumb's track (the right gutter
// strip) and its height, independent of the current scroll offset, or
// ok==false when there is no child or the child fits entirely within the
// viewport (nothing to scroll, so no thumb is drawn/hit-testable).
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
	return track, scrollThumbLength(track.H, s.childH), true
}

// thumbGeometryX returns the horizontal thumb's track (the bottom gutter
// strip) and its width, independent of the current scroll offset, or
// ok==false when there is no child or the child's natural width doesn't
// exceed the ScrollViewer's own bounds width (see ArrangeContent's doc
// comment for why this compares against bounds.W rather than the reduced
// viewport.W — the same comparison ArrangeContent uses to decide whether to
// reserve the bottom gutter at all, so the thumb only ever appears exactly
// when that gutter exists to draw it in).
func (s *ScrollViewer) thumbGeometryX() (track render.Rect, thumbW float32, ok bool) {
	bounds := s.Bounds()
	if s.viewport.W <= 0 || s.childW <= bounds.W {
		return render.Rect{}, 0, false
	}
	track = render.Rect{
		X: s.viewport.X,
		Y: s.viewport.Bottom(),
		W: s.viewport.W,
		H: s.gutter,
	}
	return track, scrollThumbLength(track.W, s.childW), true
}

// thumbRect returns the vertical thumb's current on-screen rect (track
// position plus the offset proportional to the current scroll offset), or
// ok==false when there is nothing to scroll (see thumbGeometry).
func (s *ScrollViewer) thumbRect() (render.Rect, bool) {
	track, thumbH, ok := s.thumbGeometry()
	if !ok {
		return render.Rect{}, false
	}
	maxOffset := s.childH - s.viewport.H
	thumbY := scrollThumbPos(track.Y, track.H, thumbH, s.offset, maxOffset)
	return render.Rect{X: track.X, Y: thumbY, W: track.W, H: thumbH}, true
}

// thumbRectX returns the horizontal thumb's current on-screen rect (track
// position plus the offset proportional to the current scroll offset), or
// ok==false when there is nothing to scroll (see thumbGeometryX).
func (s *ScrollViewer) thumbRectX() (render.Rect, bool) {
	track, thumbW, ok := s.thumbGeometryX()
	if !ok {
		return render.Rect{}, false
	}
	maxOffset := s.childW - s.viewport.W
	thumbX := scrollThumbPos(track.X, track.W, thumbW, s.offsetX, maxOffset)
	return render.Rect{X: thumbX, Y: track.Y, W: thumbW, H: track.H}, true
}

// RenderOverlay implements core.OverlayRenderer, drawing the classic
// scrollbar track+thumb (drawScrollThumb — a flat ButtonFace track with a
// raised ButtonFace thumb) above the clipped child for each axis that has
// content to scroll to: vertical along the right edge, horizontal along the
// bottom edge. When both are shown, thumbGeometry/thumbGeometryX's tracks
// (sized from s.viewport, which is inset on both the reserved right and
// bottom gutters) naturally stop short of each other, leaving the
// bottom-right corner square and undrawn by either track.
func (s *ScrollViewer) RenderOverlay(r render.Renderer) {
	if track, _, ok := s.thumbGeometry(); ok {
		thumb, _ := s.thumbRect()
		drawScrollThumb(r, track, thumb, s.colors)
	}
	if track, _, ok := s.thumbGeometryX(); ok {
		thumb, _ := s.thumbRectX()
		drawScrollThumb(r, track, thumb, s.colors)
	}
}

// dragTo recomputes rawOffset from a vertical drag's current pointer
// y-position, keeping the pointer at the same relative grab point within
// the thumb (dragGrabY) that it was at when the drag began, and invalidates
// arrange so the next layout pass clamps and applies it. A no-op when there
// is no room to scroll vertically.
func (s *ScrollViewer) dragTo(posY float32) {
	track, thumbH, ok := s.thumbGeometry()
	if !ok {
		return
	}
	maxOffset := s.childH - s.viewport.H
	if maxOffset <= 0 {
		return
	}
	s.rawOffset = scrollDragOffset(track.Y, track.H, thumbH, posY, s.dragGrabY, maxOffset)
	s.InvalidateArrange()
}

// dragToX recomputes rawOffsetX from a horizontal drag's current pointer
// x-position, mirroring dragTo on the X axis (dragGrabX in place of
// dragGrabY). A no-op when there is no room to scroll horizontally.
func (s *ScrollViewer) dragToX(posX float32) {
	track, thumbW, ok := s.thumbGeometryX()
	if !ok {
		return
	}
	maxOffset := s.childW - s.viewport.W
	if maxOffset <= 0 {
		return
	}
	s.rawOffsetX = scrollDragOffset(track.X, track.W, thumbW, posX, s.dragGrabX, maxOffset)
	s.InvalidateArrange()
}

// canScrollY reports whether there is vertical content to scroll to (an
// alias for thumbGeometry's ok result, without the track/thumb geometry),
// used by OnPointer to route a plain wheel notch.
func (s *ScrollViewer) canScrollY() bool {
	_, _, ok := s.thumbGeometry()
	return ok
}

// wheelBy applies a wheel notch to the vertical offset and reports whether
// the CLAMPED offset actually moved — i.e. whether this ScrollViewer really
// consumed the notch. It predicts the clamp (the same clampScrollOffset
// ArrangeContent will apply on the next pass, against the same childH and
// viewport.H) rather than waiting for that pass, because OnPointer has to
// decide Handled now: a notch that would land on the offset the viewer is
// already at scrolls nothing, and marking it Handled would stop input.Bubble
// dead at this viewer instead of letting an outer scroller take it. Leaves
// rawOffset untouched (and skips the invalidate) in that case, so a stalled
// wheel can't accumulate a raw offset far past the end either.
func (s *ScrollViewer) wheelBy(dy float32) bool {
	if clampScrollOffset(s.rawOffset+dy, s.childH, s.viewport.H) == s.offset {
		return false
	}
	s.ScrollBy(dy)
	return true
}

// wheelByX is wheelBy's X-axis counterpart, clamping against childW and
// viewport.W.
func (s *ScrollViewer) wheelByX(dx float32) bool {
	if clampScrollOffset(s.rawOffsetX+dx, s.childW, s.viewport.W) == s.offsetX {
		return false
	}
	s.ScrollByX(dx)
	return true
}

// OnPointer implements input.PointerHandler:
//
// Wheel scrolls vertically by scrollWheelStep logical px per notch by
// default (matching the pre-horizontal-scroll behavior exactly when there
// is vertical content to scroll to), Shift+Wheel scrolls horizontally
// instead, and a plain Wheel scrolls horizontally too when there is no
// vertical content to scroll to but there IS horizontal content (so a
// purely horizontally-overflowing ScrollViewer is still wheel-scrollable
// without requiring Shift). A wheel notch is handled only when it actually
// moved the clamped offset (see wheelBy): a viewer whose content already
// fits, or that is already pinned at the end stop the notch pushes toward,
// leaves the event unhandled so input.Bubble carries it on out to an
// enclosing scroller instead of swallowing it into a dead zone.
//
// A Press inside the current vertical thumb rect starts a vertical drag,
// checked first (matching the original single-axis priority); otherwise a
// Press inside the current horizontal thumb rect starts a horizontal drag.
// Either capture records which axis it's tracking (s.drag) so a subsequent
// Move/Release — only acted on while this ScrollViewer holds the capture —
// know which offset to update. A Press matching neither thumb is left
// unhandled so it bubbles through to the scrolled content.
func (s *ScrollViewer) OnPointer(e *input.PointerEvent) {
	switch e.Action {
	case input.Wheel:
		delta := -e.Delta.Y * scrollWheelStep
		var moved bool
		switch {
		case e.Mods&input.ModShift != 0:
			moved = s.wheelByX(delta)
		case s.canScrollY():
			moved = s.wheelBy(delta)
		default:
			moved = s.wheelByX(delta)
		}
		if moved {
			e.Handled = true
		}
	case input.Press:
		if rect, ok := s.thumbRect(); ok && rect.Contains(e.Pos) {
			s.dragGrabY = e.Pos.Y - rect.Y
			s.drag = scrollDragVertical
			e.Router.Capture(s)
			e.Handled = true
		} else if rect, ok := s.thumbRectX(); ok && rect.Contains(e.Pos) {
			s.dragGrabX = e.Pos.X - rect.X
			s.drag = scrollDragHorizontal
			e.Router.Capture(s)
			e.Handled = true
		}
	case input.Move:
		if e.Router.Captured() == s {
			if s.drag == scrollDragHorizontal {
				s.dragToX(e.Pos.X)
			} else {
				s.dragTo(e.Pos.Y)
			}
			e.Handled = true
		}
	case input.Release:
		if e.Router.Captured() == s {
			e.Router.Release()
			s.drag = scrollDragNone
			e.Handled = true
		}
	}
}
