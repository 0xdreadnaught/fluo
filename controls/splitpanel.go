package controls

import (
	"math"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/theme"
)

// splitDividerThickness is the fixed logical-px thickness of the draggable
// divider bar reserved between the two panes, along the split axis.
// Structural, not themed — mirrors scrollThumbMinH's own structural-constant
// convention in scrollviewer.go.
const splitDividerThickness float32 = 6

// splitDefaultRatio is the fraction of the primary axis given to the First
// pane when a SplitPanel is constructed and SetSplitRatio is never called.
const splitDefaultRatio float32 = 0.5

// splitMinPaneDefault is the default minimum logical-px size enforced for
// EACH pane (see SetMinPaneSize) until overridden.
const splitMinPaneDefault float32 = 20

// SplitPanel is a two-pane container with a single draggable divider between
// First and Second. Its Orientation follows the same convention as
// StackPanel's (see Horizontal/Vertical in stackpanel.go): Horizontal means
// First and Second sit side by side (First left, Second right) separated by
// a VERTICAL divider bar the user drags left/right; Vertical means First
// sits above Second, separated by a HORIZONTAL divider bar dragged up/down.
// In both cases, "Orientation" describes how the PANES are laid out —
// exactly as it does for StackPanel — not which way the divider bar itself
// is drawn (that's always perpendicular to the pane layout, same as a WPF
// GridSplitter's bar runs across the axis it resizes).
//
// SplitRatio is the fraction of the primary axis (width for Horizontal,
// height for Vertical) given to First; the divider's own thickness
// (splitDividerThickness) is reserved separately and never counted toward
// either pane. MinPaneSize is the floor enforced on both panes' lengths.
// Both the ratio-derived split AND the min-pane floor are recomputed fresh
// on every ArrangeContent call (see layout, the single source of truth for
// that math, mirroring ScrollViewer's ArrangeContent doc comment) — so
// resizing the SplitPanel itself always re-derives both panes' pixel sizes
// from the stored ratio rather than preserving stale pixel offsets.
type SplitPanel struct {
	core.Element

	orient Orientation

	first, second core.Widget

	ratio   float32
	minPane float32

	// dragging is true for the duration of a divider-drag pointer capture
	// (Press on the divider through the matching Release); dragGrab is the
	// offset, in logical px along the primary axis, between the pointer and
	// the divider's near edge at the moment the drag began — the same
	// "keep the pointer at its original grab point" math ScrollViewer's
	// dragGrabY/dragGrabX use for thumb dragging (see scrollviewer.go).
	dragging bool
	dragGrab float32

	// hoverPos is the last pointer position (window/absolute space, same
	// space as Bounds()) observed via an uncaptured Move, consulted by
	// Cursor() to decide whether the pointer currently sits over the
	// divider strip. Router delivers Move (via Bubble) before it calls
	// Cursor() (see Router.PointerMove), so Cursor() always sees the
	// position from the very event that triggered this cursor query.
	hoverPos render.Point

	onSplitChanged func(ratio float32)

	// colors is captured from theme.Active() at construction (see
	// NewSplitPanel), matching ScrollViewer's gutter/colors convention:
	// rebuild to re-theme.
	colors theme.ColorTokens
}

// NewSplitPanel returns a SplitPanel with no First/Second pane, a 0.5 split
// ratio, and the default min pane size, styled from theme.Active() at
// construction.
func NewSplitPanel(orientation Orientation) *SplitPanel {
	t := theme.Active()
	return &SplitPanel{
		orient:  orientation,
		ratio:   splitDefaultRatio,
		minPane: splitMinPaneDefault,
		colors:  t.Color,
	}
}

// SetFirst sets (replacing any existing) the leading pane — left for
// Horizontal, top for Vertical — re-parenting it and invalidating measure.
// Any previously set First is detached (its parent cleared), matching the
// Border/ScrollViewer SetChild convention.
func (s *SplitPanel) SetFirst(w core.Widget) *SplitPanel {
	if s.first != nil {
		core.SetParent(s.first, nil)
	}
	s.first = w
	core.SetParent(w, s)
	s.InvalidateMeasure()
	return s
}

// SetSecond sets (replacing any existing) the trailing pane — right for
// Horizontal, bottom for Vertical — re-parenting it and invalidating
// measure, mirroring SetFirst.
func (s *SplitPanel) SetSecond(w core.Widget) *SplitPanel {
	if s.second != nil {
		core.SetParent(s.second, nil)
	}
	s.second = w
	core.SetParent(w, s)
	s.InvalidateMeasure()
	return s
}

// SetSplitRatio sets the fraction of the primary axis given to First,
// clamped to [0,1]. The min-pane floor (SetMinPaneSize) is enforced
// separately, at layout time (see layout), so a ratio that would otherwise
// starve either pane below its minimum is corrected on the very next arrange
// rather than rejected here — the same "clamp where bounds are actually
// known" split of responsibility ScrollViewer's ArrangeContent uses for
// offset clamping. A silent setter: does NOT fire OnSplitChanged, which
// fires only for a user-driven divider drag.
func (s *SplitPanel) SetSplitRatio(r float32) *SplitPanel {
	if r < 0 {
		r = 0
	}
	if r > 1 {
		r = 1
	}
	s.ratio = r
	s.InvalidateArrange()
	return s
}

// SetMinPaneSize sets the minimum logical-px size enforced for EACH pane;
// the divider can never be dragged (or a ratio otherwise arranged) closer
// to either end than this. Layout-relevant: invalidates arrange (not
// measure — a SplitPanel's desired size never depends on MinPaneSize, only
// on its children's own desired sizes plus the divider thickness).
func (s *SplitPanel) SetMinPaneSize(px float32) *SplitPanel {
	if px < 0 {
		px = 0
	}
	s.minPane = px
	s.InvalidateArrange()
	return s
}

// SetOnSplitChanged sets the callback fired with the new ratio whenever the
// user drags the divider — never for a programmatic SetSplitRatio call.
func (s *SplitPanel) SetOnSplitChanged(fn func(ratio float32)) *SplitPanel {
	s.onSplitChanged = fn
	return s
}

// Children returns the panes currently set (First then Second, whichever of
// the two are non-nil), or nil if neither is. Returns a freshly built slice;
// mutating it does not affect the panel.
func (s *SplitPanel) Children() []core.Widget {
	var out []core.Widget
	if s.first != nil {
		out = append(out, s.first)
	}
	if s.second != nil {
		out = append(out, s.second)
	}
	return out
}

// MeasureContent reports a desired size that accommodates both panes plus
// the divider thickness along the primary (split) axis, and the max of the
// two panes' desired sizes on the cross axis — mirroring StackPanel's own
// measureHorizontal/measureVertical (each pane is measured with the cross
// axis bounded by available and the primary axis unbounded, so a pane
// reports its full natural extent along the axis the divider moves).
func (s *SplitPanel) MeasureContent(available render.Size) render.Size {
	if s.orient == Horizontal {
		return s.measureHorizontal(available)
	}
	return s.measureVertical(available)
}

func (s *SplitPanel) measureHorizontal(available render.Size) render.Size {
	infAvail := render.Size{W: float32(math.Inf(1)), H: available.H}

	var w1, h1, w2, h2 float32
	if s.first != nil {
		core.MeasureWidget(s.first, infAvail)
		d := core.DesiredSizeOf(s.first)
		w1, h1 = d.W, d.H
	}
	if s.second != nil {
		core.MeasureWidget(s.second, infAvail)
		d := core.DesiredSizeOf(s.second)
		w2, h2 = d.W, d.H
	}

	maxH := h1
	if h2 > maxH {
		maxH = h2
	}
	return render.Size{W: w1 + w2 + splitDividerThickness, H: maxH}
}

func (s *SplitPanel) measureVertical(available render.Size) render.Size {
	infAvail := render.Size{W: available.W, H: float32(math.Inf(1))}

	var w1, h1, w2, h2 float32
	if s.first != nil {
		core.MeasureWidget(s.first, infAvail)
		d := core.DesiredSizeOf(s.first)
		w1, h1 = d.W, d.H
	}
	if s.second != nil {
		core.MeasureWidget(s.second, infAvail)
		d := core.DesiredSizeOf(s.second)
		w2, h2 = d.W, d.H
	}

	maxW := w1
	if w2 > maxW {
		maxW = w2
	}
	return render.Size{W: maxW, H: h1 + h2 + splitDividerThickness}
}

// clampPaneLen clamps a candidate primary-axis length for First into
// [minPane, available-minPane] — the range that leaves BOTH panes at least
// minPane long. When available itself is too small to fit both minimums
// (available < 2*minPane), the min-pane floor is relaxed rather than forcing
// a negative-length pane: the candidate is clamped to plain [0, available]
// instead, splitting whatever room exists rather than pinning everything to
// one side. Shared by layout (ratio-derived arrange) and dragTo (live drag
// math) — the single clamping rule both paths share, mirroring
// clampScrollOffset's role for ScrollViewer's two scroll axes.
func clampPaneLen(v, available, minPane float32) float32 {
	lo := minPane
	hi := available - minPane
	if hi < lo {
		lo = 0
		hi = available
	}
	if v < lo {
		v = lo
	}
	if v > hi {
		v = hi
	}
	return v
}

// layout computes the three primary-axis-ordered rects — First's sub-rect,
// the divider bar, and Second's sub-rect — from the panel's current Bounds,
// ratio, and MinPaneSize. It is the single source of truth for this split
// math, called fresh by ArrangeContent (to actually arrange the panes),
// Render (to paint the divider bevel at its current position), and
// OnPointer/Cursor (to hit-test and cursor-shape the divider strip) — so all
// four always agree on exactly where the divider currently is, even mid-drag
// before the next arrange pass has run (Bounds() itself doesn't change
// during a drag; only ratio does, on InvalidateArrange).
func (s *SplitPanel) layout() (first, divider, second render.Rect) {
	bounds := s.Bounds()

	if s.orient == Horizontal {
		available := bounds.W - splitDividerThickness
		if available < 0 {
			available = 0
		}
		firstLen := clampPaneLen(s.ratio*available, available, s.minPane)
		secondLen := available - firstLen

		first = render.Rect{X: bounds.X, Y: bounds.Y, W: firstLen, H: bounds.H}
		divider = render.Rect{X: bounds.X + firstLen, Y: bounds.Y, W: splitDividerThickness, H: bounds.H}
		second = render.Rect{X: bounds.X + firstLen + splitDividerThickness, Y: bounds.Y, W: secondLen, H: bounds.H}
		return
	}

	available := bounds.H - splitDividerThickness
	if available < 0 {
		available = 0
	}
	firstLen := clampPaneLen(s.ratio*available, available, s.minPane)
	secondLen := available - firstLen

	first = render.Rect{X: bounds.X, Y: bounds.Y, W: bounds.W, H: firstLen}
	divider = render.Rect{X: bounds.X, Y: bounds.Y + firstLen, W: bounds.W, H: splitDividerThickness}
	second = render.Rect{X: bounds.X, Y: bounds.Y + firstLen + splitDividerThickness, W: bounds.W, H: secondLen}
	return
}

// ArrangeContent arranges First and Second into the sub-rects layout
// computes for the panel's current bounds, ratio, and min pane size. Run on
// every arrange pass (ArrangeWidget always calls ArrangeContent), so
// resizing the SplitPanel re-derives both panes' pixel sizes from the
// stored ratio rather than preserving whatever pixel sizes the previous
// arrange happened to produce.
func (s *SplitPanel) ArrangeContent(bounds render.Rect) {
	first, _, second := s.layout()
	if s.first != nil {
		core.ArrangeWidget(s.first, first)
	}
	if s.second != nil {
		core.ArrangeWidget(s.second, second)
	}
}

// Render paints the divider bar as a raised bevel (drawRaised, the same
// chrome ScrollViewer's thumb uses — see bevel.go), the panel's own chrome
// drawn before its children (RenderWidget calls Render, then recurses into
// Children — see core.RenderWidget's doc comment), sitting in the gap
// between the two panes' arranged sub-rects so it never overlaps either.
func (s *SplitPanel) Render(r render.Renderer) {
	_, divider, _ := s.layout()
	if divider.W <= 0 || divider.H <= 0 {
		return
	}
	drawRaised(r, divider, s.colors.ButtonFace, s.colors)
}

// dividerHitRect is a small naming convenience so OnPointer/Cursor don't
// each recompute first/second rects they don't need.
func (s *SplitPanel) dividerHitRect() render.Rect {
	_, divider, _ := s.layout()
	return divider
}

// Cursor implements input.CursorShaper: the resize cursor for this panel's
// orientation (CursorHResize for Horizontal, CursorVResize for Vertical)
// while a divider drag is in progress, or while the last-observed hover
// position (see hoverPos) sits over the divider strip; CursorArrow
// otherwise. Because Router.PointerMove delivers the triggering Move event
// (which updates hoverPos, see OnPointer) before it calls cursorForPath (see
// router.go), Cursor always reflects the position from the very query that
// asks for it — not a stale one. This is what lets a point over a pane
// report the default (or defer to a CursorShaper further down that pane's
// own subtree, which cursorForPath's leaf-first walk always tries before
// ever reaching this ancestor SplitPanel) instead of the divider's resize
// cursor leaking across the whole panel.
func (s *SplitPanel) Cursor() input.Cursor {
	if !s.dragging && !s.dividerHitRect().Contains(s.hoverPos) {
		return input.CursorArrow
	}
	if s.orient == Horizontal {
		return input.CursorHResize
	}
	return input.CursorVResize
}

// dragTo recomputes ratio from the divider drag's current pointer position,
// keeping the pointer at the same relative grab point within the divider
// (dragGrab) it was at when the drag began — the same math ScrollViewer's
// dragTo/dragToX use for thumb dragging — clamped by clampPaneLen, then
// invalidates arrange and fires OnSplitChanged with the new ratio. A no-op
// (ratio left unchanged, callback not fired) when there's no room to split
// (available <= 0), matching dragTo/dragToX's own no-room no-op.
func (s *SplitPanel) dragTo(pos render.Point) {
	bounds := s.Bounds()

	var available, firstLen float32
	if s.orient == Horizontal {
		available = bounds.W - splitDividerThickness
		if available <= 0 {
			return
		}
		firstLen = pos.X - s.dragGrab - bounds.X
	} else {
		available = bounds.H - splitDividerThickness
		if available <= 0 {
			return
		}
		firstLen = pos.Y - s.dragGrab - bounds.Y
	}

	firstLen = clampPaneLen(firstLen, available, s.minPane)
	s.ratio = firstLen / available
	s.InvalidateArrange()
	if s.onSplitChanged != nil {
		s.onSplitChanged(s.ratio)
	}
}

// OnPointer implements input.PointerHandler:
//
// A Press inside the current divider rect starts a drag: it records the
// grab offset (dragGrab) along the primary axis, captures the pointer, and
// marks the event handled. A Press anywhere else (i.e. over a pane) is left
// unhandled so it bubbles through to that pane's own child content — only
// the divider strip is interactive for the SplitPanel itself.
//
// A Move while this SplitPanel holds the capture updates the split via
// dragTo and is marked handled; otherwise (uncaptured hover) it just
// records hoverPos for Cursor to consult, without marking the event
// handled — a bare hover doesn't consume anything.
//
// A Release while this SplitPanel holds the capture ends the drag.
func (s *SplitPanel) OnPointer(e *input.PointerEvent) {
	switch e.Action {
	case input.Press:
		if s.dividerHitRect().Contains(e.Pos) {
			divider := s.dividerHitRect()
			if s.orient == Horizontal {
				s.dragGrab = e.Pos.X - divider.X
			} else {
				s.dragGrab = e.Pos.Y - divider.Y
			}
			s.dragging = true
			e.Router.Capture(s)
			e.Handled = true
		}
	case input.Move:
		if e.Router.Captured() == s {
			s.dragTo(e.Pos)
			e.Handled = true
		} else {
			s.hoverPos = e.Pos
		}
	case input.Release:
		if e.Router.Captured() == s {
			e.Router.Release()
			s.dragging = false
			e.Handled = true
		}
	}
}
