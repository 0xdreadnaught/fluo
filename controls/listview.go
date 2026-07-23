package controls

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
	"github.com/0xdreadnaught/fluo/theme"
)

// listViewDefaultW and listViewDefaultH are ListView's default desired size
// (v0: fixed, like ScrollViewer wrapping a natural-sized child — but a
// ListView's "content" is virtualized and can be arbitrarily long, so unlike
// ScrollViewer it cannot measure to a real content size; {160, 240} is the
// structural default a caller overrides via SetWidth/SetHeight).
const (
	listViewDefaultW float32 = 160
	listViewDefaultH float32 = 240
)

// ListChangeKind enumerates the granular list-mutation kinds a ListView
// reacts to. It mirrors bind.ChangeKind exactly — bind.ChangeKind is a type
// alias for this type (see bind/list.go) — so *bind.List[T] values satisfy
// ListItems below without controls importing bind.
type ListChangeKind uint8

const (
	ListChangeAdd ListChangeKind = iota
	ListChangeRemove
	ListChangeReplace
	ListChangeReset
)

// ListChange mirrors bind.Change exactly (bind.Change is a type alias for
// this type). It, and ListChangeKind above, are declared here rather than in
// bind because ListItems.OnChange must name this payload type, and bind
// already imports controls (for bind.Items and, from Task 3, ListSelected);
// controls importing bind back would be a cycle. Declaring the granular
// change vocabulary in controls and having bind re-export it as type
// aliases keeps a single canonical definition while breaking the cycle — the
// one deliberately inverted seam in an otherwise bind-depends-on-controls
// codebase. Document prominently; do not "fix" by having controls import
// bind.
type ListChange struct {
	Kind  ListChangeKind
	Index int // -1 for a full reset
}

// ListItems is the minimal observable string source a ListView virtualizes:
// a read-only indexed sequence (Len/At) plus a granular change-notification
// channel. *bind.List[string] satisfies this interface structurally (see
// ListChange's doc comment for why this is an interface rather than a
// concrete *bind.List[string] parameter).
type ListItems interface {
	Len() int
	At(i int) string
	OnChange(f func(ListChange)) (cancel func())
}

// ListView virtualizes a single-column, uniform-row-height list of strings:
// only the rows intersecting the current viewport are realized, into a
// pool of reused TextBlocks (see ArrangeContent). v0: string rows only
// (custom row factories arrive later); no selection or keyboard handling
// (Task 3).
type ListView struct {
	core.Element
	virtualizer

	face  *text.Face
	items ListItems

	pool []*TextBlock

	cancel func()
}

// NewListView returns a ListView rendering items (v0: plain strings) with
// face, styled from theme.Active() at construction (rebuild to re-theme).
// It subscribes to items.OnChange (the granular channel) so that any list
// mutation invalidates measure+arrange; v0 does not use the Change payload
// for incremental updates (a full re-layout recomputes the visible range
// and re-texts the pool from scratch) — per-row incrementalism is a later
// phase. Callers MUST call Dispose() when done with the ListView (e.g. from
// a rebuild's cancel path) to release this subscription — see Dispose.
func NewListView(face *text.Face, items ListItems) *ListView {
	t := theme.Active()

	l := &ListView{face: face, items: items}
	l.gutter = t.Metric.ScrollGutter
	l.thumbColor = t.Color.ScrollThumb
	l.thumbRadius = t.Metric.ControlCornerRadius
	l.rowH = defaultRowHeight(face, t)
	l.count = func() int {
		if l.items == nil {
			return 0
		}
		return l.items.Len()
	}

	l.cancel = items.OnChange(func(ListChange) { l.InvalidateMeasure() })
	return l
}

// defaultRowHeight is face.LineHeight()+2*PaddingS, nil-face-safe (a nil
// face contributes zero line height, matching TextBlock's own nil-face
// convention).
func defaultRowHeight(face *text.Face, t *theme.Theme) float32 {
	var lineH float32
	if face != nil {
		lineH = face.LineHeight()
	}
	return lineH + 2*t.Metric.PaddingS
}

// RowHeight returns the current row height (the default set at
// construction, or a later SetRowHeight override).
func (l *ListView) RowHeight() float32 {
	return l.rowH
}

// SetRowHeight overrides the row height. Purely an arrange-time concern
// (which rows are visible and where they land, not ListView's own {160,240}
// desired size), so it invalidates arrange only.
func (l *ListView) SetRowHeight(h float32) *ListView {
	l.rowH = h
	l.InvalidateArrange()
	return l
}

// Dispose releases l's subscription to items' granular change channel.
// ListView is fluo's FIRST disposable control: every other v0 control holds
// no external resource and needs no explicit teardown, but a ListView's
// items subscription otherwise outlives the ListView itself (the list holds
// the callback, not the other way around) — so callers MUST call Dispose()
// when a ListView is removed from the tree (e.g. a gallery rebuild's cancel
// path, alongside any bind cancels) to stop it reacting to further list
// mutations. Safe to call more than once (the underlying cancel is
// idempotent); calling it leaves the ListView otherwise usable, just no
// longer reactive to the list.
func (l *ListView) Dispose() {
	if l.cancel != nil {
		l.cancel()
	}
}

// Children returns the CURRENT realized row pool (for hit-testing/render),
// a copy so mutating it does not affect the ListView.
func (l *ListView) Children() []core.Widget {
	if len(l.pool) == 0 {
		return nil
	}
	out := make([]core.Widget, len(l.pool))
	for i, tb := range l.pool {
		out[i] = tb
	}
	return out
}

// MeasureContent reports ListView's fixed {160,240} default desired size
// (see listViewDefaultW/H), never exceeding available — a ListView never
// asks its parent for more room than it was offered, matching
// ScrollViewer's own convention, even though (unlike ScrollViewer) its
// desired size does not depend on its (virtualized, unboundedly long)
// content at all.
func (l *ListView) MeasureContent(available render.Size) render.Size {
	w := listViewDefaultW
	if w > available.W {
		w = available.W
	}
	h := listViewDefaultH
	if h > available.H {
		h = available.H
	}
	return render.Size{W: w, H: h}
}

// ArrangeContent is the single source of truth for offset clamping (via
// virtualizer.layout) and for row realization: it computes the viewport
// (bounds minus the thumb gutter on the right), clamps the scroll offset,
// determines the visible row range, resizes the TextBlock pool to exactly
// that many rows (see ensurePool — existing pool entries are re-texted in
// place, never reallocated), and arranges each pool entry at its row's
// position, offset by the current scroll.
func (l *ListView) ArrangeContent(bounds render.Rect) {
	viewport := bounds.Inset(render.Thickness{Right: l.gutter})
	if viewport.W < 0 {
		viewport.W = 0
	}
	if viewport.H < 0 {
		viewport.H = 0
	}

	l.layout(viewport)

	first, last := l.visibleRange()
	n := last - first
	l.shrinkPool(n)

	for i := 0; i < n; i++ {
		idx := first + i
		text := l.items.At(idx)

		var tb *TextBlock
		if i < len(l.pool) {
			// Reuse: re-text ONLY when the value actually changed. This
			// isn't just an optimization — TextBlock.SetText invalidates
			// measure and climbs to its parent (l), and ListView's own
			// desired size never depends on its rows' content, so an
			// unconditional SetText here would leave l spuriously
			// measure-dirty after every arrange pass purely from our own
			// re-texting (see the sibling growth branch below for why a
			// brand-new pool entry avoids this by construction).
			tb = l.pool[i]
			if tb.Text() != text {
				tb.SetText(text)
			}
		} else {
			// Grow: construct WITH the correct text directly (never via ""
			// then SetText) so populating a freshly grown pool slot never
			// fires TextBlock's own invalidate-parent hook — SetParent
			// below runs after construction, so even if it did fire, there
			// would be no parent yet to climb into.
			tb = NewTextBlock(l.face, text)
			core.SetParent(tb, l)
			l.pool = append(l.pool, tb)
		}

		rowRect := render.Rect{
			X: viewport.X,
			Y: viewport.Y + float32(idx)*l.rowH - l.offset,
			W: viewport.W,
			H: l.rowH,
		}
		core.MeasureWidget(tb, render.Size{W: viewport.W, H: l.rowH})
		core.ArrangeWidget(tb, rowRect)
	}
}

// shrinkPool detaches (core.SetParent(tb, nil)) and drops any pool entries
// beyond the first n, keeping the pool size exactly equal to the current
// visible-row count (per the brief's pool invariant). Growth and reuse are
// handled inline in ArrangeContent, where the target text for each slot is
// already known.
func (l *ListView) shrinkPool(n int) {
	if len(l.pool) > n {
		for _, tb := range l.pool[n:] {
			core.SetParent(tb, nil)
		}
		l.pool = l.pool[:n]
	}
}

// ClipRect implements core.ClipProvider, clipping realized rows to
// ListView's own full bounds (thumb gutter included, so the thumb itself —
// drawn in RenderOverlay, which runs after the clip is popped — is never
// cropped), matching ScrollViewer.ClipRect.
func (l *ListView) ClipRect() (render.Rect, bool) {
	return l.Bounds(), true
}

// RenderOverlay implements core.OverlayRenderer, drawing the thumb above the
// clipped rows when there is content to scroll to, matching
// ScrollViewer.RenderOverlay.
func (l *ListView) RenderOverlay(r render.Renderer) {
	rect, ok := l.thumbRect()
	if !ok {
		return
	}
	r.FillRoundedRect(rect, l.thumbRadius, l.thumbColor)
}

// OnPointer implements input.PointerHandler, matching ScrollViewer.OnPointer
// exactly: Wheel scrolls by scrollWheelStep logical px per notch and is
// always handled; a Press inside the current thumb rect starts a drag and
// is handled, while a Press elsewhere is left unhandled so it bubbles to
// realized rows (row click-to-select arrives in Task 3); Move/Release are
// only acted on while this ListView holds the capture.
func (l *ListView) OnPointer(e *input.PointerEvent) {
	switch e.Action {
	case input.Wheel:
		l.scrollBy(-e.Delta.Y * scrollWheelStep)
		l.InvalidateArrange()
		e.Handled = true
	case input.Press:
		if rect, ok := l.thumbRect(); ok && rect.Contains(e.Pos) {
			l.dragGrabY = e.Pos.Y - rect.Y
			e.Router.Capture(l)
			e.Handled = true
		}
	case input.Move:
		if e.Router.Captured() == l {
			l.dragTo(e.Pos.Y)
			l.InvalidateArrange()
			e.Handled = true
		}
	case input.Release:
		if e.Router.Captured() == l {
			e.Router.Release()
			e.Handled = true
		}
	}
}
