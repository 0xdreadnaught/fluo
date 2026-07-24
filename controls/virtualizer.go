package controls

import (
	"math"

	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
	"github.com/0xdreadnaught/fluo/theme"
)

// virtualizer owns the uniform-row scroll/viewport math shared by any
// control that realizes only the visible slice of a long, fixed-row-height
// list (ListView v0; DataGrid's body later). It knows row height, item
// count, and vertical scroll offset (clamped at arrange time exactly like
// ScrollViewer's own rawOffset/offset split — see scrollviewer.go's doc
// comment on ArrangeContent, which is the single source of truth for
// clamping there; layout below plays the identical role here) plus the
// viewport rect as of the last arrange pass, and it reuses ScrollViewer's
// own thumb-geometry/wheel-step constants (scrollThumbMinH, scrollWheelStep
// in scrollviewer.go) so the two controls feel identical to scroll.
//
// It is embedded BY VALUE in its owning widget, not a core.Widget itself: it
// has no element(), MeasureContent/ArrangeContent, or Render of its own —
// the owner (ListView) drives it explicitly from its own ArrangeContent (via
// layout) and OnPointer (via the wheel/drag helpers below), and promotes its
// fields/methods through Go's anonymous-embedding rules. Its methods take
// pointer receivers; embedding by value still allows this because a
// *ListView's virtualizer field is addressable.
type virtualizer struct {
	rowH  float32
	count func() int

	// rawOffset is the last value requested via wheel/drag, before clamping.
	// offset is the clamped value as of the last layout call — layout is the
	// single source of truth for clamping.
	rawOffset float32
	offset    float32

	// viewport is the owner's content rect (already inset by the thumb
	// gutter) as of the last layout call; used for both visible-range math
	// and thumb geometry.
	viewport render.Rect

	// dragGrabY is the y-offset (logical px) between the pointer and the
	// thumb's top edge at drag start, exactly like ScrollViewer.dragGrabY.
	dragGrabY float32

	// gutter is captured from theme.Active() at construction, matching
	// ScrollViewer's own capture convention. Thumb/track colors are NOT
	// captured here: the owning control (ListView, DataGrid) already holds
	// its own `colors theme.ColorTokens` field and passes it directly to
	// drawScrollThumb from its own RenderOverlay.
	gutter float32
}

// totalHeight returns rowH * count(), the full (unvirtualized) content
// height. A nil count or a negative/zero rowH both report zero.
func (v *virtualizer) totalHeight() float32 {
	if v.count == nil || v.rowH <= 0 {
		return 0
	}
	n := v.count()
	if n <= 0 {
		return 0
	}
	return float32(n) * v.rowH
}

// layout is the single source of truth for offset clamping (mirroring
// ScrollViewer.ArrangeContent): it stores viewport, clamps rawOffset into
// [0, max(0, totalHeight-viewport.H)], and stores the clamped result (read
// back via offset).
func (v *virtualizer) layout(viewport render.Rect) {
	v.viewport = viewport

	maxOffset := v.totalHeight() - viewport.H
	if maxOffset < 0 {
		maxOffset = 0
	}
	offset := v.rawOffset
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	v.offset = offset
}

// visibleRange returns the half-open row-index range [first, last) that
// intersects the viewport at the last-clamped offset: first is the row
// containing the viewport's top edge, last is one past the row containing
// the viewport's bottom edge, so a row that is only partially visible at
// either edge is still included. Both bounds are clamped into [0, count()];
// an empty list, a non-positive rowH, or a non-positive viewport height all
// report an empty range (0, 0).
func (v *virtualizer) visibleRange() (first, last int) {
	if v.count == nil || v.rowH <= 0 || v.viewport.H <= 0 {
		return 0, 0
	}
	n := v.count()
	if n <= 0 {
		return 0, 0
	}

	first = int(math.Floor(float64(v.offset / v.rowH)))
	last = int(math.Ceil(float64((v.offset + v.viewport.H) / v.rowH)))

	if first < 0 {
		first = 0
	}
	if first > n {
		first = n
	}
	if last > n {
		last = n
	}
	if last < first {
		last = first
	}
	return first, last
}

// thumbGeometry returns the thumb's track (the right gutter strip) and its
// height, independent of the current scroll offset, or ok==false when there
// is no content or the content fits entirely within the viewport (nothing to
// scroll). Identical math to ScrollViewer.thumbGeometry, against
// totalHeight instead of a single child's desired height.
func (v *virtualizer) thumbGeometry() (track render.Rect, thumbH float32, ok bool) {
	total := v.totalHeight()
	if v.viewport.H <= 0 || total <= v.viewport.H {
		return render.Rect{}, 0, false
	}
	track = render.Rect{
		X: v.viewport.Right(),
		Y: v.viewport.Y,
		W: v.gutter,
		H: v.viewport.H,
	}
	thumbH = track.H * track.H / total
	if thumbH < scrollThumbMinH {
		thumbH = scrollThumbMinH
	}
	if thumbH > track.H {
		thumbH = track.H
	}
	return track, thumbH, true
}

// thumbRect returns the thumb's current on-screen rect, or ok==false when
// there is nothing to scroll (see thumbGeometry). Identical math to
// ScrollViewer.thumbRect.
func (v *virtualizer) thumbRect() (render.Rect, bool) {
	track, thumbH, ok := v.thumbGeometry()
	if !ok {
		return render.Rect{}, false
	}
	total := v.totalHeight()
	maxOffset := total - v.viewport.H
	thumbY := track.Y
	if maxOffset > 0 {
		thumbY = track.Y + (v.offset/maxOffset)*(track.H-thumbH)
	}
	return render.Rect{X: track.X, Y: thumbY, W: track.W, H: thumbH}, true
}

// scrollBy requests a relative change to the vertical offset, clamped on the
// next layout call like ScrollViewer.ScrollBy. Callers (ListView.OnPointer)
// must still invalidate arrange themselves — virtualizer has no Element to
// invalidate.
func (v *virtualizer) scrollBy(dy float32) {
	v.rawOffset += dy
}

// dragTo recomputes rawOffset from a drag's current pointer y-position,
// identical math to ScrollViewer.dragTo. A no-op when there is nothing to
// scroll (thumbGeometry not ok, or content already fits the viewport).
func (v *virtualizer) dragTo(posY float32) {
	track, thumbH, ok := v.thumbGeometry()
	if !ok {
		return
	}
	total := v.totalHeight()
	maxOffset := total - v.viewport.H
	if maxOffset <= 0 {
		return
	}
	span := track.H - thumbH
	thumbY := posY - v.dragGrabY
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
	v.rawOffset = frac * maxOffset
}

// initVirtualizer captures the theme-derived fields every virtualizer-owning
// control sets identically at construction (ListView, DataGrid — both
// embed a virtualizer by value and call this from their own New* func):
// the thumb gutter (ScrollViewer's own convention, per the type doc comment
// above) and the default row height (defaultRowHeight(face, t), in
// listview.go). v.count is deliberately left for the caller to set
// afterward — that closure differs per owner (it reads items.Len() for
// ListView, g.rowCount for DataGrid) and has no theme-derived default to
// share.
func initVirtualizer(v *virtualizer, face *text.Face, t *theme.Theme) {
	v.gutter = t.Metric.ScrollGutter
	v.rowH = defaultRowHeight(face, t)
}
