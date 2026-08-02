package controls

import (
	"math"

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
// (custom row factories arrive later).
//
// Selection (Task 3): single index, -1 meaning none. selected is set
// programmatically (silent, SetSelectedIndex) or by the user (row click,
// Up/Down/Home/End while focused — both funneled through selectUser, which
// fires OnChanged only on an actual change, matching ComboBox.selectUser/
// Slider.setValue's "notify only on real change" convention). Both paths
// auto-scroll the resulting selection into view via scrollIntoView — see
// its doc comment for why this matters for bind.ListSelected's model-push
// direction too, not just the user-driven ones.
type ListView struct {
	core.Element
	virtualizer

	face  *text.Face
	items ListItems

	pool []*TextBlock

	// visibleFirst is the item index of pool[0] as of the last ArrangeContent
	// pass (i.e. the `first` returned by virtualizer.visibleRange), kept so
	// Render can locate the selected row's on-screen rect (if it is
	// currently realized) without recomputing the visible range itself.
	visibleFirst int

	selected  int // -1 == none
	focused   bool
	onChanged func(int)

	colors  theme.ColorTokens
	metrics theme.MetricTokens

	// cachedContentW memoizes contentWidth()'s result and cachedContentWOK
	// says whether it may be read: measuring the widest row costs one
	// face.Measure per item (each of which locks the source font per glyph),
	// and ArrangeContent asks for it on EVERY pass — so without this, one
	// wheel notch re-measures the whole list, which is exactly the O(rows)
	// cost the row pool exists to avoid. Named apart from the embedded
	// virtualizer's own contentW (the value layout was last handed) so the
	// two never shadow each other. See invalidateContentWidth for when the
	// memo is dropped, and contentWidth's doc comment for the staleness
	// contract that makes memoizing safe.
	cachedContentW   float32
	cachedContentWOK bool

	cancel func()
}

// NewListView returns a ListView rendering items (v0: plain strings) with
// face, styled from theme.Active() at construction (rebuild to re-theme).
// It subscribes to items.OnChange (the granular channel) so that any list
// mutation invalidates measure+arrange and drops the memoized content width
// (see contentWidth); v0 does not use the Change payload for incremental
// updates (a full re-layout recomputes the visible range and re-texts the
// pool from scratch) — per-row incrementalism is a later phase. Callers
// MUST call Dispose() when done with the ListView (e.g. from a rebuild's
// cancel path) to release this subscription — see Dispose.
// items may be nil, matching the nil-face convention: the ListView builds
// and lays out as a permanently empty list rather than panicking.
func NewListView(face *text.Face, items ListItems) *ListView {
	t := theme.Active()

	l := &ListView{face: face, items: items, selected: -1, colors: t.Color, metrics: t.Metric}
	initVirtualizer(&l.virtualizer, face, t)
	l.count = func() int {
		if l.items == nil {
			return 0
		}
		return l.items.Len()
	}

	// A nil items is a valid, permanently empty list (l.count and
	// contentWidth both already treat it as zero-length): there is nothing
	// to subscribe to, so cancel stays nil and Dispose becomes a no-op.
	if items != nil {
		l.cancel = items.OnChange(func(ListChange) {
			l.invalidateContentWidth()
			l.InvalidateMeasure()
		})
	}
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

// OffsetY returns the current vertical scroll offset, clamped to
// [0, max(0, totalHeight-viewport.H)] as of the last arrange pass —
// mirroring ScrollViewer.OffsetY exactly.
func (l *ListView) OffsetY() float32 {
	return l.offset
}

// OffsetX returns the current horizontal scroll offset, clamped to
// [0, max(0, contentWidth-viewport.W)] as of the last arrange pass —
// mirroring ScrollViewer.OffsetX exactly (contentWidth is l.contentWidth(),
// the widest row's measured text width — see its doc comment).
func (l *ListView) OffsetX() float32 {
	return l.offsetX
}

// ScrollTo requests a new vertical offset, clamped on the next layout pass
// (virtualizer.layout, invoked from ArrangeContent, is the single source of
// truth for clamping — see its doc comment), mirroring
// ScrollViewer.ScrollTo exactly.
func (l *ListView) ScrollTo(y float32) *ListView {
	l.rawOffset = y
	l.InvalidateArrange()
	return l
}

// ScrollToX requests a new horizontal offset, clamped on the next layout
// pass like ScrollTo, mirroring ScrollViewer.ScrollToX exactly.
func (l *ListView) ScrollToX(x float32) *ListView {
	l.rawOffsetX = x
	l.InvalidateArrange()
	return l
}

// clampRowIndex clamps i into [0, n-1] (n == 0 collapses to 0, a harmless
// degenerate result never actually read since every keyboard-nav caller
// below already checked n > 0 first) — unlike clampSelectedIndex (shared
// with ComboBox, allowing -1 as an explicit "no selection" result),
// clampRowIndex always lands on a REAL row: Up/Down/Home/End move the
// selection among existing rows, they never clear it.
func clampRowIndex(i, n int) int {
	if i < 0 {
		return 0
	}
	if i > n-1 {
		return n - 1
	}
	return i
}

// scrollIntoView adjusts rawOffset — clamped on the next layout pass, like
// every other virtualizer offset mutation — so that item index is fully
// visible within the CURRENT viewport (as of the last layout pass): pulling
// the offset up to the row's top edge if the row sits above the viewport,
// or down to the row's bottom edge minus the viewport height if it sits
// below. A no-op for index < 0 (no selection) or before a real layout pass
// has established a nonzero viewport height (nothing to scroll relative to
// yet; the next real layout starts from whatever rawOffset already holds).
//
// Both selection paths route through this — selectUser (row click,
// keyboard nav) AND SetSelectedIndex (the silent, programmatic path
// bind.ListSelected's model-push direction uses). That symmetry is
// deliberate: a bound property driving the selection from model code should
// scroll the selected row into view exactly as if the user had pressed
// Home/End/Up/Down to get there, not leave it realized-but-invisible
// off-screen.
func (l *ListView) scrollIntoView(index int) {
	if index < 0 || l.viewport.H <= 0 || l.rowH <= 0 {
		return
	}
	rowTop := float32(index) * l.rowH
	rowBottom := rowTop + l.rowH
	switch {
	case rowTop < l.offset:
		l.rawOffset = rowTop
		l.InvalidateArrange()
	case rowBottom > l.offset+l.viewport.H:
		l.rawOffset = rowBottom - l.viewport.H
		l.InvalidateArrange()
	}
}

// SelectedIndex returns the current selection, or -1 if none.
// CAUTION: ListView does NOT re-clamp or track selection across external
// list mutations. After a RemoveAt/Remove on the bound list, SelectedIndex()
// may name a shifted or out-of-range logical row with no OnChanged fired.
// Callers needing stable selection should re-set it (via SetSelectedIndex)
// after mutating the list.
func (l *ListView) SelectedIndex() int {
	return l.selected
}

// SetSelectedIndex sets the selection programmatically, clamped into
// [-1, count-1] (see clampSelectedIndex; -1 is always a valid, explicit
// "no selection" value) and auto-scrolled into view (see scrollIntoView).
// Silent: never fires OnChanged, matching the package's uniform setter
// convention (ComboBox.SetSelectedIndex, Slider.SetValue, ...) — this is
// the setter bind.ListSelected's model-push direction calls, so its
// silence is also what keeps that binder echo-safe (see bind's package doc
// comment). Invalidates arrange unconditionally: the selected row's fill/
// text color must be recomputed even when the clamped index is unchanged
// from before (matching Slider.setValueSilent's own "assign, no change
// tracking" simplicity), and scrollIntoView needs a re-layout to apply.
//
// CAUTION: See SelectedIndex's doc comment on list-mutation caveats.
func (l *ListView) SetSelectedIndex(i int) *ListView {
	l.selected = clampSelectedIndex(i, l.count())
	l.scrollIntoView(l.selected)
	l.InvalidateArrange()
	return l
}

// OnChanged sets the callback fired with the new index whenever the user
// changes the selection — by clicking a row or navigating with Up/Down/
// Home/End while focused — but never for a programmatic SetSelectedIndex
// (see its doc comment). Replaces any previously set callback; a nil fn is
// a valid, silent no-op.
func (l *ListView) OnChanged(fn func(int)) *ListView {
	l.onChanged = fn
	return l
}

// selectUser is the user-driven selection path (row click, keyboard nav):
// clamps i via clampSelectedIndex (callers here always pass an already
// in-range row index, but the clamp is a harmless no-op safety net),
// scrolls the result into view, invalidates arrange, and fires OnChanged
// only if the selection actually changed — matching ComboBox.selectUser/
// Slider.setValue's "notify only on real change" convention.
func (l *ListView) selectUser(i int) {
	i = clampSelectedIndex(i, l.count())
	changed := i != l.selected
	l.selected = i
	l.scrollIntoView(i)
	l.InvalidateArrange()
	if changed && l.onChanged != nil {
		l.onChanged(i)
	}
}

// rowAt maps an absolute pointer position to the item index it falls over,
// using the viewport/offset as of the last layout pass (mirroring
// ArrangeContent's own rowRect math). ok is false for a position outside
// the viewport entirely (the thumb gutter to its right, or above/below the
// ListView's own bounds) or landing past the last item (empty space below
// a short list that doesn't fill the viewport) — both "click gutter" and
// "click empty" report ok == false, so callers make no selection change.
func (l *ListView) rowAt(pos render.Point) (idx int, ok bool) {
	if l.rowH <= 0 || !l.viewport.Contains(pos) {
		return 0, false
	}
	idx = int(math.Floor(float64((pos.Y - l.viewport.Y + l.offset) / l.rowH)))
	if idx < 0 || idx >= l.count() {
		return 0, false
	}
	return idx, true
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

// contentWidth returns the max row content width — the widest row's
// face-measured text width plus the same lpad/rpad horizontal inset
// ArrangeContent gives every row (see its row-rect comment) — across every
// item, the "natural" width the widest row would need if never clipped to
// the viewport. This is the contentWidth virtualizer.layout clamps offsetX
// against and ArrangeContent compares against the viewport width to decide
// whether to reserve the horizontal thumb's gutter (see its doc comment).
//
// A nil face can't meaningfully measure text (TextBlock.MeasureContent's own
// nil-face convention: measures to zero), and a nil items has nothing to
// measure either, so both report 0 — content that can't be measured never
// triggers horizontal scrolling, keeping every existing nil-face ListView
// (every current behavior test, and the existing listview.png golden, which
// uses a real face but text far narrower than its viewport) byte-identical.
//
// The result is memoized (see cachedContentW) because ArrangeContent asks
// for it every pass while the full measure is O(items) face.Measure calls;
// measureContentWidth below does the real work, and only ever runs again
// after invalidateContentWidth drops the memo.
//
// STALENESS CONTRACT: the memo assumes a row's measured width can only
// change through the item source's granular change channel (ListItems.
// OnChange — the same notifications that already drive re-layout). That
// holds for the source ListView is written against (*bind.List[string],
// whose Add/Insert/RemoveAt/Set/Replace all notify), but a hand-written
// ListItems whose At(i) silently starts returning a different (wider)
// string without notifying would leave this width stale until the next
// real change. Detecting arbitrary in-place mutation would mean re-measuring
// everything every pass, i.e. exactly the cost being removed here — so such
// a source must announce the edit (a Replace/Reset notification) like any
// other, which is also already required for the row text itself to be
// re-realized.
//
// The face is not a staleness source: it is captured once in NewListView and
// never reassigned (there is no SetFace), and Measure works in logical pixels
// (scale-independent), so a DPI/scale change does not change a row's measured
// width. Changing the face or theme means rebuilding the widget tree, which
// constructs a fresh ListView with a cold memo — so font changes can never
// leave this value stale on a live instance.
func (l *ListView) contentWidth() float32 {
	if !l.cachedContentWOK {
		l.cachedContentW = l.measureContentWidth()
		l.cachedContentWOK = true
	}
	return l.cachedContentW
}

// measureContentWidth is contentWidth's uncached body: the full measure pass
// over every item. Never call it directly from layout code — go through
// contentWidth so the result is memoized.
func (l *ListView) measureContentWidth() float32 {
	if l.face == nil || l.items == nil {
		return 0
	}
	n := l.items.Len()
	var maxW float32
	for i := 0; i < n; i++ {
		if w := l.face.Measure(l.items.At(i)).W; w > maxW {
			maxW = w
		}
	}
	lpad := 2 * l.metrics.PaddingS
	rpad := l.metrics.PaddingS
	return maxW + lpad + rpad
}

// invalidateContentWidth drops the memoized content width so the next
// contentWidth() re-measures. Called from the items subscription installed
// in NewListView — the single point at which the item set can change under a
// ListView (there is no SetItems: an existing ListView stays bound to the
// source it was constructed with, and neither the face nor the padding
// metrics can change after construction either, so no other invalidation
// point exists to miss). See contentWidth's staleness contract.
func (l *ListView) invalidateContentWidth() {
	l.cachedContentWOK = false
}

// contentBounds returns l's own bounds inset by the 2px sunken bevel (see
// theme.MetricTokens.BevelWidth) — the area the well's raised/sunken frame
// encloses, which ArrangeContent further insets by the thumb gutter to reach
// the actual viewport. Rows (and the selection band) render entirely inside
// this rect, never over the bevel itself.
func (l *ListView) contentBounds() render.Rect {
	bw := l.metrics.BevelWidth
	inner := l.Bounds().Inset(render.Thickness{Top: bw, Bottom: bw, Left: bw, Right: bw})
	if inner.W < 0 {
		inner.W = 0
	}
	if inner.H < 0 {
		inner.H = 0
	}
	return inner
}

// ArrangeContent is the single source of truth for offset clamping (via
// virtualizer.layout) and for row realization: it insets bounds by the 2px
// sunken bevel first (see contentBounds' doc comment — rows must sit inside
// the well's frame, not over it), computes the viewport within that inset
// rect (minus the vertical thumb's gutter on the right and, control-variants
// Task 4, the horizontal thumb's gutter on the bottom), clamps the scroll
// offset on both axes, determines the visible row range (recorded in
// visibleFirst for Render's selection-band lookup), resizes the TextBlock
// pool to exactly that many rows (see shrinkPool and the grow/reuse branches
// below — existing pool entries are re-texted in place, never reallocated),
// arranges each pool entry at its row's position offset by the current
// scroll on both axes, and recolors it: HighlightText for the selected
// row's text, WindowText for every other (set unconditionally on every
// pass, unlike the conditional re-text below, since a selection change
// alone — no text change — must still repaint the right rows the next time
// this runs).
//
// The two gutter decisions are independent (the right gutter only reduces
// width, the bottom gutter only reduces height) and each is made against the
// pre-gutter inset dimension on its own axis: vGutter compares totalHeight
// to inset.H, then hGutter compares contentWidth to the resulting viewport.W
// (inset.W minus vGutter — NOT inset.W itself, since vGutter has already
// permanently claimed that space from any row's actual content width; the
// Bottom inset applied afterward doesn't touch W, so this is the same W
// thumbGeometryX later reads back from v.viewport.W — see its doc comment
// for why that agreement matters).
func (l *ListView) ArrangeContent(bounds render.Rect) {
	inset := bounds.Inset(render.Thickness{
		Top: l.metrics.BevelWidth, Bottom: l.metrics.BevelWidth,
		Left: l.metrics.BevelWidth, Right: l.metrics.BevelWidth,
	})
	if inset.W < 0 {
		inset.W = 0
	}
	if inset.H < 0 {
		inset.H = 0
	}

	// Reserve the vertical thumb's gutter only when the content actually
	// scrolls vertically. When it fits (no thumb), the viewport is the full
	// inset rect so rows and the selection band reach its right edge; when
	// it scrolls, the gutter keeps the band clear of the (translucent) thumb
	// so the highlight sits fully beside the scrollbar rather than bleeding
	// through it.
	vGutter := float32(0)
	if l.totalHeight() > inset.H {
		vGutter = l.gutter
	}
	viewport := inset.Inset(render.Thickness{Right: vGutter})
	if viewport.W < 0 {
		viewport.W = 0
	}
	if viewport.H < 0 {
		viewport.H = 0
	}

	// Reserve the horizontal thumb's gutter only when the widest row's
	// natural content actually exceeds the (already vGutter-reduced)
	// viewport width — mirroring the vertical decision above, generalized to
	// the X axis (see the doc comment above for why viewport.W, not inset.W,
	// is the correct comparison here).
	contentW := l.contentWidth()
	hGutter := float32(0)
	if contentW > viewport.W {
		hGutter = l.gutter
	}
	viewport = viewport.Inset(render.Thickness{Bottom: hGutter})
	if viewport.H < 0 {
		viewport.H = 0
	}

	l.layout(viewport, contentW)

	first, last := l.visibleRange()
	n := last - first
	l.visibleFirst = first
	l.shrinkPool(n)

	for i := 0; i < n; i++ {
		idx := first + i
		rowText := l.items.At(idx)

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
			if tb.Text() != rowText {
				tb.SetText(rowText)
			}
		} else {
			// Grow: construct WITH the correct text directly (never via ""
			// then SetText) so populating a freshly grown pool slot never
			// fires TextBlock's own invalidate-parent hook — SetParent
			// below runs after construction, so even if it did fire, there
			// would be no parent yet to climb into.
			tb = NewTextBlock(l.face, rowText)
			// Center the label vertically within the row: the row is
			// rowH = LineHeight + 2*PaddingS tall, so without this the text
			// top-aligns and all the slack falls below it — visible once a
			// selection band is drawn behind the row. Set once at creation
			// (SetAlign invalidates arrange; setting it every pass would
			// re-dirty layout every frame).
			tb.SetAlign(core.Stretch, core.Center)
			core.SetParent(tb, l)
			l.pool = append(l.pool, tb)
		}

		rowColor := l.colors.WindowText
		if idx == l.selected {
			rowColor = l.colors.HighlightText
		}
		tb.SetColor(rowColor) // purely visual (TextBlock.SetColor), no invalidation

		// Inset the row text off the row edges so labels don't sit flush:
		// 2*PaddingS on the left, PaddingS on the right. The selection band
		// (Render) still spans the full row width; only the text is inset,
		// the standard list-row look. X is additionally offset by -l.offsetX
		// (0 for any ListView that doesn't scroll horizontally, so this is a
		// no-op for every existing behavior test/golden): TextBlock.Render
		// draws its natural-width text unclamped from its own bounds
		// top-left (ignoring the W given here, see TextBlock.MeasureContent's
		// own doc comment), so shifting X left as offsetX grows is what
		// reveals text that would otherwise sit past the row's — and thus
		// the ListView's own ClipRect's — right edge.
		lpad := 2 * l.metrics.PaddingS
		rpad := l.metrics.PaddingS
		rowW := viewport.W - lpad - rpad
		if rowW < 0 {
			rowW = 0
		}
		rowRect := render.Rect{
			X: viewport.X + lpad - l.offsetX,
			Y: viewport.Y + float32(idx)*l.rowH - l.offset,
			W: rowW,
			H: l.rowH,
		}
		core.MeasureWidget(tb, render.Size{W: rowW, H: l.rowH})
		core.ArrangeWidget(tb, rowRect)
	}
}

// Render draws the sunken WindowWell frame across l's full bounds first
// (the classic list-box well — see contentBounds' doc comment for how rows
// are kept inside it), then fills the selected row's band with Highlight
// before its TextBlock renders on top (see RenderWidget's documented order:
// a widget's own Render runs before its children's) — a no-op for the band
// when nothing is selected, or the selected index isn't currently realized
// (scrolled out of the visible range). The band is drawn unclipped like
// every other Render method in this package, but always safely within l's
// own bounds regardless: the row rect computed here is the exact geometry
// ArrangeContent already arranged that pool slot's TextBlock at.
func (l *ListView) Render(r render.Renderer) {
	drawSunken(r, l.Bounds(), l.colors.WindowWell, l.colors)

	if l.selected < 0 {
		return
	}
	last := l.visibleFirst + len(l.pool)
	if l.selected < l.visibleFirst || l.selected >= last {
		return
	}
	// The band spans the viewport width. When the list doesn't scroll the
	// viewport is the full inset content rect (ArrangeContent reserves no
	// gutter), so the band reaches its right edge; when it scrolls, the band
	// stops at the gutter so it stays clear of the translucent thumb rather
	// than showing through it.
	rowRect := render.Rect{
		X: l.viewport.X,
		Y: l.viewport.Y + float32(l.selected)*l.rowH - l.offset,
		W: l.viewport.W,
		H: l.rowH,
	}
	r.FillRect(rowRect, l.colors.Highlight)
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

// ClipRect implements core.ClipProvider, clipping realized rows to l's
// bevel-inset content bounds (see contentBounds' doc comment) rather than
// l's raw outer bounds — so a partially-scrolled row at the top/bottom edge
// of the viewport is cropped at the well's inner frame, never bleeding onto
// the sunken bevel itself. The thumb gutter is included (within
// contentBounds), so the thumb itself — drawn in RenderOverlay, which runs
// after the clip is popped — is never cropped either way.
func (l *ListView) ClipRect() (render.Rect, bool) {
	return l.contentBounds(), true
}

// RenderOverlay implements core.OverlayRenderer, drawing the classic
// track+thumb (drawScrollThumb, matching ScrollViewer.RenderOverlay) for
// each axis that has content to scroll to — vertical along the right edge,
// horizontal (control-variants Task 4) along the bottom — above the clipped
// rows, then the focus ring while focused — per the global focus constraint
// shared by every focusable control in this package (Slider, ComboBox,
// ...) — drawn last so it sits above both thumbs.
func (l *ListView) RenderOverlay(r render.Renderer) {
	if track, _, ok := l.thumbGeometry(); ok {
		thumb, _ := l.thumbRect()
		drawScrollThumb(r, track, thumb, l.colors)
	}
	if track, _, ok := l.thumbGeometryX(); ok {
		thumb, _ := l.thumbRectX()
		drawScrollThumb(r, track, thumb, l.colors)
	}
	if l.focused {
		drawFocusRing(r, l.Bounds(), l.colors)
	}
}

// AcceptsFocus implements input.Focusable: a ListView always accepts focus
// (v0 has no SetEnabled/disabled concept, unlike Slider/ComboBox).
func (l *ListView) AcceptsFocus() bool {
	return true
}

// OnFocusChanged implements input.FocusHandler, tracking focus for the
// focus-ring overlay and Up/Down/Home/End keyboard navigation.
func (l *ListView) OnFocusChanged(focused bool) {
	l.focused = focused
}

// OnPointer implements input.PointerHandler, extending ScrollViewer's own
// wheel/thumb-drag handling with row click-to-select:
//
// Wheel scrolls vertically (row scroll) by scrollWheelStep logical px per
// notch by default, and horizontally instead when Shift is held (matching
// ScrollViewer's Shift+Wheel convention). Unlike ScrollViewer, a plain wheel
// never falls back to X even when only X overflows: a ListView's rows are
// its primary scroll axis, so plain wheel always means row scroll. A notch
// is handled only when it actually moved the clamped offset (see
// virtualizer.scrollBy) — a list whose rows already fit, or one pinned at
// the end stop the notch pushes toward, leaves the event unhandled so
// input.Bubble carries it out to an enclosing ScrollViewer rather than
// swallowing it into a dead zone.
//
// A Press inside the current vertical thumb rect starts a vertical drag,
// checked first (matching the original single-axis priority); otherwise a
// Press inside the current horizontal thumb rect starts a horizontal drag;
// otherwise a Press that lands on a real row (rowAt) selects it as a
// user-driven change (selectUser) and is handled, while a Press over a
// gutter or empty space below a short list (rowAt reports ok == false) is
// left unhandled. Either drag records which axis it's tracking (l.drag) so
// a subsequent Move/Release — only acted on while this ListView holds the
// capture — knows which offset to update.
func (l *ListView) OnPointer(e *input.PointerEvent) {
	switch e.Action {
	case input.Wheel:
		delta := -e.Delta.Y * scrollWheelStep
		var moved bool
		if e.Mods&input.ModShift != 0 {
			moved = l.scrollByX(delta)
		} else {
			moved = l.scrollBy(delta)
		}
		if !moved {
			return
		}
		l.InvalidateArrange()
		e.Handled = true
	case input.Press:
		if rect, ok := l.thumbRect(); ok && rect.Contains(e.Pos) {
			l.dragGrabY = e.Pos.Y - rect.Y
			l.drag = scrollDragVertical
			e.Router.Capture(l)
			e.Handled = true
		} else if rect, ok := l.thumbRectX(); ok && rect.Contains(e.Pos) {
			l.dragGrabX = e.Pos.X - rect.X
			l.drag = scrollDragHorizontal
			e.Router.Capture(l)
			e.Handled = true
		} else if idx, ok := l.rowAt(e.Pos); ok {
			l.selectUser(idx)
			e.Handled = true
		}
	case input.Move:
		if e.Router.Captured() == l {
			if l.drag == scrollDragHorizontal {
				l.dragToX(e.Pos.X)
			} else {
				l.dragTo(e.Pos.Y)
			}
			l.InvalidateArrange()
			e.Handled = true
		}
	case input.Release:
		if e.Router.Captured() == l {
			e.Router.Release()
			l.drag = scrollDragNone
			e.Handled = true
		}
	}
}

// OnKey implements input.KeyHandler: Up/Down move the selection by one row
// (clamped into [0, count-1] via clampRowIndex — never landing on -1, even
// starting from no selection: Up/Down from -1 both land on row 0), Home/End
// jump to the first/last row; all four are user-driven (selectUser) and
// auto-scroll the result into view. Ignored entirely for anything but
// Action==Press, or when there are no rows to select among — matching
// Slider/ComboBox's enabled-only-style early-out (ListView has no enabled
// concept, so the guard here is simply "any items at all").
func (l *ListView) OnKey(e *input.KeyEvent) {
	if e.Action != input.Press {
		return
	}
	n := l.count()
	if n == 0 {
		return
	}
	switch e.Key {
	case input.KeyUp:
		l.selectUser(clampRowIndex(l.selected-1, n))
		e.Handled = true
	case input.KeyDown:
		l.selectUser(clampRowIndex(l.selected+1, n))
		e.Handled = true
	case input.KeyHome:
		l.selectUser(clampRowIndex(0, n))
		e.Handled = true
	case input.KeyEnd:
		l.selectUser(clampRowIndex(n-1, n))
		e.Handled = true
	}
}
