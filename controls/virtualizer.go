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
// Horizontal scrolling (control-variants Task 4) generalizes the SAME
// pattern to the X axis: rawOffsetX/offsetX mirror rawOffset/offset exactly,
// clamped by layout against a host-provided contentWidth (there is no
// virtualizer-owned notion of "total content width" the way totalHeight()
// derives height from rowH*count() — DataGrid's is sum(colWidths), ListView's
// is the widest row's measured text — so the owner passes it into layout
// explicitly every pass, the same way it already computes and owns viewport
// itself). thumbGeometryX/thumbRectX/dragToX/scrollByX are the X-axis
// counterparts of thumbGeometry/thumbRect/dragTo/scrollBy, sharing
// scrollDragAxis (declared in scrollviewer.go, same package) so a captured
// Move/Release knows which axis a drag is tracking — mirroring
// ScrollViewer's own drag field exactly.
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
	// single source of truth for clamping, and folds the clamped result back
	// into rawOffset so the pair only ever disagrees for requests made since
	// that call. rawOffsetX/offsetX are the exact X-axis counterparts.
	rawOffset, offset   float32
	rawOffsetX, offsetX float32

	// viewport is the owner's content rect (already inset by both the
	// vertical AND horizontal thumb gutters, whichever the owner's
	// ArrangeContent decided to reserve) as of the last layout call; used for
	// both visible-range/thumb-geometry math on either axis. contentW is the
	// host-provided content width from that same layout call (see the type
	// doc comment) — stored so thumbGeometryX/dragToX, called later from
	// RenderOverlay/OnPointer, don't need the owner to re-derive it.
	viewport render.Rect
	contentW float32

	// dragGrabY is the y-offset (logical px) between the pointer and the
	// thumb's top edge at drag start, exactly like ScrollViewer.dragGrabY.
	// dragGrabX is its X-axis counterpart; drag records which axis a capture
	// is currently tracking (scrollDragAxis, declared in scrollviewer.go),
	// exactly like ScrollViewer.drag.
	dragGrabY float32
	dragGrabX float32
	drag      scrollDragAxis

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

// layout is the single source of truth for offset clamping on BOTH axes
// (mirroring ScrollViewer.ArrangeContent): it stores viewport and
// contentWidth (see the type doc comment for why the latter is host-provided
// rather than virtualizer-derived), clamps rawOffset into
// [0, max(0, totalHeight-viewport.H)] and rawOffsetX into
// [0, max(0, contentWidth-viewport.W)] — via the same clampScrollOffset
// ScrollViewer itself uses, so the two controls clamp identically — and
// stores the clamped results (read back via offset/offsetX), folding each
// back into its raw counterpart so the unbounded accumulators can't drift
// past the end stop.
func (v *virtualizer) layout(viewport render.Rect, contentWidth float32) {
	v.viewport = viewport
	v.contentW = contentWidth

	v.offset = clampScrollOffset(v.rawOffset, v.totalHeight(), viewport.H)
	v.offsetX = clampScrollOffset(v.rawOffsetX, contentWidth, viewport.W)

	// Fold the clamp back into the raw accumulators, exactly as
	// ScrollViewer.ArrangeContent does (see its comment): scrollBy/scrollByX
	// add to them without bound, so an overshoot at an end stop would
	// otherwise have to be burned off notch by notch before the clamped
	// offset moved again.
	v.rawOffset = v.offset
	v.rawOffsetX = v.offsetX
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

// rowTop returns the y-coordinate of row idx's top edge in the owner's
// coordinate space (viewport-relative, minus the current vertical scroll
// offset) — the single source of truth for where a uniform row is drawn, so
// ListView/DataGrid's arrange and render can't drift apart from each other or
// from rowIndexAt's inverse.
//
// The idx*rowH term is accumulated in float64 and only then narrowed to
// float32: a float32 idx can no longer represent consecutive integers past
// 2^24, so at very large counts float32(idx)*rowH quantizes — adjacent rows
// round to the same top and overlap, and a click hit-tests to the wrong row.
// float64 stays exact well past any realistic row count. At idx < 2^24 this is
// bit-identical to the old float32(idx)*rowH (both are the one correctly
// rounded float32 of the real product idx*rowH — float64(idx)*float64(rowH) is
// itself exact there, ≤ 48 significant bits), so nothing changes — no visible
// shift, no golden churn — at normal counts.
func (v *virtualizer) rowTop(idx int) float32 {
	return v.viewport.Y + float32(float64(idx)*float64(v.rowH)) - v.offset
}

// rowIndexAt returns the row index whose band contains y (same coordinate
// space as rowTop) — the exact inverse of rowTop, floor((y - viewport.Y +
// offset) / rowH) computed entirely in float64 so it stays consistent with
// rowTop past idx 2^24 rather than quantizing on its own. A non-positive rowH
// reports 0. The result is unclamped (it can be negative, or >= count for a y
// outside the realized band); callers range-check it themselves, exactly as
// the prior inline hit-test math did.
func (v *virtualizer) rowIndexAt(y float32) int {
	if v.rowH <= 0 {
		return 0
	}
	return int(math.Floor((float64(y) - float64(v.viewport.Y) + float64(v.offset)) / float64(v.rowH)))
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
// next layout call like ScrollViewer.ScrollBy, and reports whether that
// clamped offset will actually move. Callers (ListView.OnPointer) must still
// invalidate arrange themselves — virtualizer has no Element to invalidate.
//
// The "did it move" answer has to come back now rather than after the next
// layout pass, because the wheel handler that asks has to decide right here
// whether to mark the event Handled: a notch that lands on the offset we are
// already at scrolls nothing, and consuming it would stop input.Bubble at
// this control instead of letting an enclosing scroller have it. So the
// clamp is predicted with the same clampScrollOffset layout will apply,
// against the same totalHeight and viewport, and rawOffset is left untouched
// when the answer is no — which also keeps a stalled wheel from piling up a
// raw offset far past the end.
func (v *virtualizer) scrollBy(dy float32) bool {
	if clampScrollOffset(v.rawOffset+dy, v.totalHeight(), v.viewport.H) == v.offset {
		return false
	}
	v.rawOffset += dy
	return true
}

// scrollByX requests a relative change to the horizontal offset, clamped on
// the next layout call, mirroring scrollBy on the X axis (against contentW
// rather than totalHeight).
func (v *virtualizer) scrollByX(dx float32) bool {
	if clampScrollOffset(v.rawOffsetX+dx, v.contentW, v.viewport.W) == v.offsetX {
		return false
	}
	v.rawOffsetX += dx
	return true
}

// thumbGeometryX returns the horizontal thumb's track (the bottom gutter
// strip) and its width, or ok==false when there is no content or contentW
// fits entirely within the viewport (nothing to scroll horizontally).
// Identical math to ScrollViewer.thumbGeometryX, except the "does this
// overflow" comparison is against v.viewport.W — the SAME final width value
// the owner's ArrangeContent already decided the horizontal gutter against
// (see ListView/DataGrid ArrangeContent's own hGutter decision, which
// computes contentWidth vs. that identical pre-Bottom-inset viewport.W) — so
// this can never disagree with whether that gutter was actually reserved.
func (v *virtualizer) thumbGeometryX() (track render.Rect, thumbW float32, ok bool) {
	if v.viewport.W <= 0 || v.contentW <= v.viewport.W {
		return render.Rect{}, 0, false
	}
	track = render.Rect{
		X: v.viewport.X,
		Y: v.viewport.Bottom(),
		W: v.viewport.W,
		H: v.gutter,
	}
	thumbW = track.W * track.W / v.contentW
	if thumbW < scrollThumbMinH {
		thumbW = scrollThumbMinH
	}
	if thumbW > track.W {
		thumbW = track.W
	}
	return track, thumbW, true
}

// thumbRectX returns the horizontal thumb's current on-screen rect, or
// ok==false when there is nothing to scroll (see thumbGeometryX). Identical
// math to ScrollViewer.thumbRectX.
func (v *virtualizer) thumbRectX() (render.Rect, bool) {
	track, thumbW, ok := v.thumbGeometryX()
	if !ok {
		return render.Rect{}, false
	}
	maxOffset := v.contentW - v.viewport.W
	thumbX := track.X
	if maxOffset > 0 {
		thumbX = track.X + (v.offsetX/maxOffset)*(track.W-thumbW)
	}
	return render.Rect{X: thumbX, Y: track.Y, W: thumbW, H: track.H}, true
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

// dragToX recomputes rawOffsetX from a drag's current pointer x-position,
// mirroring dragTo on the X axis (dragGrabX in place of dragGrabY). A no-op
// when there is nothing to scroll horizontally.
func (v *virtualizer) dragToX(posX float32) {
	track, thumbW, ok := v.thumbGeometryX()
	if !ok {
		return
	}
	maxOffset := v.contentW - v.viewport.W
	if maxOffset <= 0 {
		return
	}
	span := track.W - thumbW
	thumbX := posX - v.dragGrabX
	if thumbX < track.X {
		thumbX = track.X
	}
	if thumbX > track.X+span {
		thumbX = track.X + span
	}
	var frac float32
	if span > 0 {
		frac = (thumbX - track.X) / span
	}
	v.rawOffsetX = frac * maxOffset
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
