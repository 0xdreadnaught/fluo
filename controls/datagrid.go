package controls

import (
	"fmt"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
	"github.com/0xdreadnaught/fluo/theme"
)

// dataGridDefaultW and dataGridDefaultH are DataGrid's default desired size
// (v0: fixed, mirroring ListView's own {160,240} default — see
// ListView.MeasureContent's doc comment for why a virtualized control with
// unboundedly long content still reports a fixed structural default rather
// than sizing to its content). A caller overrides via SetWidth/SetHeight.
const (
	dataGridDefaultW float32 = 240
	dataGridDefaultH float32 = 200
)

// Column describes one DataGrid column: its header title, its sizing Track
// (Px or Star — see the type doc comment on DataGrid for why AutoTrack is
// rejected), and the func that produces a given row's displayed text for
// this column. Value may be nil, in which case every cell in the column
// renders empty text.
type Column struct {
	Title string
	Width Track
	Value func(row int) string
}

// DataGrid is a virtualized, multi-column grid: a FIXED header row (drawn
// directly — each column its own raised bevel, drawRaised, ButtonFace, with a
// WindowText title) sitting above a VIRTUALIZED body that reuses the same
// uniform-row virtualizer ListView is built on (see virtualizer.go) for its
// viewport/scroll/thumb math. Only the body scrolls; the header's own rect
// depends solely on the grid's arranged bounds, never on scroll offset. The
// outer frame is a sunken WindowWell (drawSunken), with the header's raised
// cells and every body row inset inside it by BevelWidth (see
// ArrangeContent).
//
// Column widths are resolved against the body viewport's width exactly like
// Grid's own column tracks (see resolveTracks in grid.go, reused directly
// here) — Px columns get their fixed size, Star columns split the remaining
// space by weight. AutoTrack is NOT supported in v0 (a DataGrid column has
// no natural "desired content width" concept the way a Grid cell's child
// widget does): SetColumns panics immediately, naming the offending column's
// index, rather than deferring to a resolution-time surprise.
//
// Cells are realized into a pool of TextBlocks sized exactly
// visibleRows × numColumns (row-major: pool[row*numCols+col]), reused and
// re-texted across scroll/column changes the same way ListView's row pool
// is — see ArrangeContent. Selection is a single row index (-1 == none),
// with the same silent-setter/user-driven-OnChanged/scroll-into-view
// contract as ListView (SetSelectedIndex silent+clamped; SetSelectedIndex/
// row click/Up+Down keyboard all route through selectUser or
// scrollIntoView). A hovered row (row hit under the pointer, tracked
// separately from selection) is tracked but NOT painted — classic lists have
// no hover fill; the selected row paints a Highlight band with
// HighlightText cell text.
//
// v0 grid lines: horizontal only — a 1px ButtonShadow line beneath every
// realized row. There are no vertical column separators in v0; a later
// phase may add them as an opt-in.
type DataGrid struct {
	core.Element
	virtualizer

	face    *text.Face
	columns []Column

	rowCount int

	// pool holds exactly visibleCount*len(columns) TextBlocks as of the last
	// ArrangeContent pass, row-major: pool[row*numCols+col] where row is
	// relative to visibleFirst (see ArrangeContent).
	pool         []*TextBlock
	visibleFirst int
	visibleCount int

	// colWidths/colOffsets are the last-resolved column geometry (against
	// the body viewport's width), cached so Render can draw header titles
	// and grid lines without re-resolving tracks itself.
	colWidths  []float32
	colOffsets []float32

	// header is the fixed header row's rect as of the last ArrangeContent
	// pass — depends only on bounds, never on scroll offset (see the type
	// doc comment).
	header render.Rect

	selected int // -1 == none

	// hoverRow is the row index last reported under the pointer by a Move
	// (-1 == none). It is an ABSOLUTE row index, not a screen position, so
	// any offset change that happens without a fresh Move to re-hit-test
	// against (Wheel, thumb dragTo) would otherwise leave it naming a row
	// that has since scrolled to a different on-screen position — OnPointer
	// clears it to -1 in both of those cases so it never names a stale row.
	// Render does not paint hoverRow at all (the classic look has no hover
	// fill); it is tracked purely for other consumers.
	hoverRow int
	focused  bool

	onChanged func(int)

	colors  theme.ColorTokens
	metrics theme.MetricTokens
}

// NewDataGrid returns an empty DataGrid (no columns, zero rows) drawing
// header titles and cell text with face, styled from theme.Active() at
// construction (rebuild to re-theme).
func NewDataGrid(face *text.Face) *DataGrid {
	t := theme.Active()

	g := &DataGrid{face: face, selected: -1, hoverRow: -1, colors: t.Color, metrics: t.Metric}
	initVirtualizer(&g.virtualizer, face, t)
	g.count = func() int { return g.rowCount }
	return g
}

// headerHeight is the fixed header row's height — v0 reuses the exact same
// formula as a body row (defaultRowHeight, captured into rowH at
// construction), so the header and every body row line up on an identical
// vertical rhythm.
func (g *DataGrid) headerHeight() float32 {
	return g.rowH
}

// SetColumns replaces the column set, re-validating every column's Width:
// AutoTrack is not supported in v0 (see the type doc comment), so any
// AutoTrack column panics immediately, naming its index — matching Grid's
// own Rows/Cols "fail fast on invalid input" convention. Invalidates
// measure (a column change can change cell content and header titles, even
// though DataGrid's own desired size doesn't depend on them — matching
// ListView's SetItems-via-OnChange convention of erring conservative).
func (g *DataGrid) SetColumns(cols ...Column) *DataGrid {
	for i, c := range cols {
		if c.Width.kind == trackAuto {
			panic(fmt.Sprintf("controls: DataGrid column %d (%q): AutoTrack is not supported in v0; use Px or Star", i, c.Title))
		}
	}
	g.columns = cols
	g.InvalidateMeasure()
	return g
}

// SetRowCount sets the row count driving both the virtualizer's total
// height and which row indices Column.Value funcs are called with (v0's
// data model: a plain count plus per-column Value funcs, no row objects).
// Re-clamps an out-of-range selection into the new range (see
// clampSelectedIndex) — silently, since this is a bulk data-model change,
// not a user selection action. Invalidates measure.
func (g *DataGrid) SetRowCount(n int) *DataGrid {
	g.rowCount = n
	g.selected = clampSelectedIndex(g.selected, n)
	g.InvalidateMeasure()
	return g
}

// RowCount returns the current row count.
func (g *DataGrid) RowCount() int {
	return g.rowCount
}

// OffsetX returns the current horizontal scroll offset, clamped to
// [0, max(0, contentWidth-viewport.W)] as of the last arrange pass —
// mirroring ScrollViewer.OffsetX exactly (contentWidth is sum(colWidths),
// see ArrangeContent's doc comment).
func (g *DataGrid) OffsetX() float32 {
	return g.offsetX
}

// ScrollToX requests a new horizontal offset, clamped on the next layout
// pass like SetSelectedIndex's auto-scroll, mirroring ScrollViewer.ScrollToX
// exactly. Both the header and body scroll in sync (see ArrangeContent's
// doc comment), so this moves both together.
func (g *DataGrid) ScrollToX(x float32) *DataGrid {
	g.rawOffsetX = x
	g.InvalidateArrange()
	return g
}

// resolveColumnWidths resolves the current columns' Width tracks against
// avail (the body viewport's width), reusing Grid's own resolveTracks
// exactly like a Grid column axis. Since SetColumns already rejects
// AutoTrack, the maxDesired callback only matters for a Star track under an
// unbounded (+Inf) avail — a DataGrid column has no child-widget-derived
// "desired width" the way a Grid cell does, so that degenerate case
// resolves to 0, matching Grid's own "no children on this track" result.
func (g *DataGrid) resolveColumnWidths(avail float32) []float32 {
	tracks := make([]Track, len(g.columns))
	for i, c := range g.columns {
		tracks[i] = c.Width
	}
	return resolveTracks(tracks, avail, func(int) float32 { return 0 })
}

// SelectedIndex returns the current selection, or -1 if none.
func (g *DataGrid) SelectedIndex() int {
	return g.selected
}

// scrollIntoView mirrors ListView.scrollIntoView exactly, against the
// virtualizer's own (body-only) viewport/offset — see that doc comment for
// the full rationale (both selectUser and SetSelectedIndex route through
// this).
func (g *DataGrid) scrollIntoView(index int) {
	if index < 0 || g.viewport.H <= 0 || g.rowH <= 0 {
		return
	}
	rowTop := float32(index) * g.rowH
	rowBottom := rowTop + g.rowH
	switch {
	case rowTop < g.offset:
		g.rawOffset = rowTop
		g.InvalidateArrange()
	case rowBottom > g.offset+g.viewport.H:
		g.rawOffset = rowBottom - g.viewport.H
		g.InvalidateArrange()
	}
}

// SetSelectedIndex sets the selection programmatically, clamped into
// [-1, RowCount()-1] and auto-scrolled into view — silent, matching
// ListView.SetSelectedIndex's contract exactly (see its doc comment).
func (g *DataGrid) SetSelectedIndex(i int) *DataGrid {
	g.selected = clampSelectedIndex(i, g.RowCount())
	g.scrollIntoView(g.selected)
	g.InvalidateArrange()
	return g
}

// OnChanged sets the callback fired with the new row index whenever the
// user changes the selection — by clicking a row or pressing Up/Down while
// focused — but never for a programmatic SetSelectedIndex. Replaces any
// previously set callback; a nil fn is a valid, silent no-op.
func (g *DataGrid) OnChanged(fn func(int)) *DataGrid {
	g.onChanged = fn
	return g
}

// selectUser is the user-driven selection path (row click, Up/Down),
// mirroring ListView.selectUser exactly: clamps, scrolls into view,
// invalidates arrange, and fires OnChanged only on an actual change.
func (g *DataGrid) selectUser(i int) {
	i = clampSelectedIndex(i, g.RowCount())
	changed := i != g.selected
	g.selected = i
	g.scrollIntoView(i)
	g.InvalidateArrange()
	if changed && g.onChanged != nil {
		g.onChanged(i)
	}
}

// rowAt maps an absolute pointer position to the BODY row index it falls
// over, using the virtualizer's own viewport/offset as of the last layout
// pass — mirroring ListView.rowAt exactly (a position over the header, the
// thumb gutter, or past the last row all report ok == false).
func (g *DataGrid) rowAt(pos render.Point) (idx int, ok bool) {
	if g.rowH <= 0 || !g.viewport.Contains(pos) {
		return 0, false
	}
	idx = g.rowIndexAt(pos.Y)
	if idx < 0 || idx >= g.RowCount() {
		return 0, false
	}
	return idx, true
}

// Dispose is a no-op for DataGrid (v0's row count + Column.Value funcs data
// model holds no external subscription to release, unlike ListView's items
// channel) — present so callers that uniformly Dispose() every virtualized
// control in a rebuild's cancel path don't need a type switch.
func (g *DataGrid) Dispose() {}

// Children returns the CURRENT realized cell pool (for hit-testing/render),
// a copy so mutating it does not affect the DataGrid.
func (g *DataGrid) Children() []core.Widget {
	if len(g.pool) == 0 {
		return nil
	}
	out := make([]core.Widget, len(g.pool))
	for i, tb := range g.pool {
		out[i] = tb
	}
	return out
}

// MeasureContent reports DataGrid's fixed {240,200} default desired size
// (see dataGridDefaultW/H), never exceeding available — matching
// ListView's own "virtualized content never grows the desired size"
// convention.
func (g *DataGrid) MeasureContent(available render.Size) render.Size {
	w := dataGridDefaultW
	if w > available.W {
		w = available.W
	}
	h := dataGridDefaultH
	if h > available.H {
		h = available.H
	}
	return render.Size{W: w, H: h}
}

// ArrangeContent is the single source of truth for the header's fixed rect,
// column-width resolution, virtualizer offset clamping on BOTH axes (via
// virtualizer.layout against the BODY viewport only — header space is
// carved off bounds first), and body cell realization. bounds is first
// inset by the 2px sunken bevel (see theme.MetricTokens.BevelWidth): the
// header's raised cell buttons and every body row render inside the outer
// well's frame, never over it, mirroring ListView.ArrangeContent's own
// bevel inset. It resizes the TextBlock pool to exactly visibleRows*numCols
// (see shrinkPool and the grow/reuse branches below, mirroring
// ListView.ArrangeContent's own convention of re-texting existing entries in
// place rather than reallocating), arranges each pool entry at its (row,col)
// cell rect offset by the current scroll on both axes, and recolors it:
// HighlightText for the selected row's cells, WindowText otherwise (set
// unconditionally on every pass, like ListView, so a selection change alone
// still repaints the right cells next pass).
//
// Horizontal scrolling (control-variants Task 4): contentWidth is
// sum(colWidths) — resolved, as before, against viewport.W AFTER the
// vertical thumb's gutter is subtracted (g.gutter is reserved
// UNCONDITIONALLY here, exactly as before Task 4 — see the type doc
// comment's own note on this being the pre-existing, unlike-ListView
// convention this method must not disturb). The horizontal thumb's gutter
// is then reserved on the BOTTOM only when contentWidth exceeds that same
// viewport.W: with any Star column, sum(colWidths) resolves to exactly
// viewport.W (Star fills the remainder), so contentWidth can only exceed it
// when columns are Px-only and their total exceeds the available width —
// the deliberate horizontal-overflow scenario this feature targets. Because
// colWidths are resolved against, and contentWidth is compared against, the
// SAME viewport.W value, this can never spuriously disagree with the
// pre-existing column resolution (no golden regression risk: the existing
// datagrid.png fixture's Star column makes contentWidth==viewport.W
// exactly, never >). g.colOffsets are the LOGICAL (unscrolled) column
// positions; Render and the cell rects below apply -g.offsetX to them at
// paint/arrange time (mirroring ListView's row-rect treatment), so the
// header (drawn separately in Render, see its own clip/offset doc comment)
// and the body cells here always read the same g.offsetX and stay in sync.
func (g *DataGrid) ArrangeContent(bounds render.Rect) {
	bw := g.metrics.BevelWidth
	inset := bounds.Inset(render.Thickness{Top: bw, Bottom: bw, Left: bw, Right: bw})
	if inset.W < 0 {
		inset.W = 0
	}
	if inset.H < 0 {
		inset.H = 0
	}

	headerH := g.headerHeight()
	g.header = render.Rect{X: inset.X, Y: inset.Y, W: inset.W, H: headerH}

	body := inset.Inset(render.Thickness{Top: headerH})
	viewport := body.Inset(render.Thickness{Right: g.gutter})
	if viewport.W < 0 {
		viewport.W = 0
	}
	if viewport.H < 0 {
		viewport.H = 0
	}

	numCols := len(g.columns)
	colWidths := g.resolveColumnWidths(viewport.W)
	colOffsets := make([]float32, numCols)
	x := viewport.X
	var contentW float32
	for i, w := range colWidths {
		colOffsets[i] = x
		x += w
		contentW += w
	}
	g.colWidths = colWidths
	g.colOffsets = colOffsets

	hGutter := float32(0)
	if contentW > viewport.W {
		hGutter = g.gutter
	}
	viewport = viewport.Inset(render.Thickness{Bottom: hGutter})
	if viewport.H < 0 {
		viewport.H = 0
	}

	g.layout(viewport, contentW)

	first, last := g.visibleRange()
	n := last - first
	g.visibleFirst = first
	g.visibleCount = n
	g.shrinkPool(n * numCols)

	for row := 0; row < n; row++ {
		rowIdx := first + row
		rowY := g.rowTop(rowIdx)

		for c := 0; c < numCols; c++ {
			slot := row*numCols + c

			var cellText string
			if g.columns[c].Value != nil {
				cellText = g.columns[c].Value(rowIdx)
			}

			var tb *TextBlock
			if slot < len(g.pool) {
				// Reuse: re-text ONLY when the value actually changed, per
				// ListView.ArrangeContent's own reuse convention (see its
				// doc comment for why unconditional SetText would leave
				// this grid spuriously measure-dirty).
				tb = g.pool[slot]
				if tb.Text() != cellText {
					tb.SetText(cellText)
				}
			} else {
				// Grow: construct WITH the correct text directly, before
				// SetParent, so a freshly grown slot never fires
				// TextBlock's invalidate-parent hook against a not-yet-set
				// parent.
				tb = NewTextBlock(g.face, cellText)
				// Center cell text vertically in its row so body cells line
				// up with the header (whose titles are centered in Render);
				// without this the cells top-align and sit above the header
				// baseline. Set once at creation (SetAlign dirties arrange).
				tb.SetAlign(core.Stretch, core.Center)
				core.SetParent(tb, g)
				g.pool = append(g.pool, tb)
			}

			color := g.colors.WindowText
			if rowIdx == g.selected {
				color = g.colors.HighlightText
			}
			tb.SetColor(color)

			// Inset the cell text by PaddingS on the left so it lines up with
			// the header titles, which Render draws at colOffset+PaddingS;
			// without the matching inset every column's body sits PaddingS to
			// the left of its own header. Trim width by 2*PaddingS to keep a
			// symmetric right gutter before the next column.
			pad := g.metrics.PaddingS
			cellW := colWidths[c] - 2*pad
			if cellW < 0 {
				cellW = 0
			}
			cellRect := render.Rect{X: colOffsets[c] + pad - g.offsetX, Y: rowY, W: cellW, H: g.rowH}
			core.MeasureWidget(tb, render.Size{W: cellW, H: g.rowH})
			core.ArrangeWidget(tb, cellRect)
		}
	}
}

// shrinkPool detaches and drops any pool entries beyond the first n,
// keeping the pool size exactly equal to the current visibleRows*numCols
// count — mirroring ListView.shrinkPool exactly.
func (g *DataGrid) shrinkPool(n int) {
	if len(g.pool) > n {
		for _, tb := range g.pool[n:] {
			core.SetParent(tb, nil)
		}
		g.pool = g.pool[:n]
	}
}

// Render draws the outer sunken WindowWell frame across g's full bounds
// first (the classic grid well — the header's raised cell buttons and every
// body row sit inside it, per ArrangeContent's bevel inset), then the
// header: each column gets its own raised ButtonFace cell (drawRaised) with
// a WindowText title, rather than one flat strip — followed by the body's
// per-row backgrounds: the selected row's Highlight band (a hovered row
// paints no fill at all in the classic look — hoverRow is still tracked, by
// OnPointer, purely for other consumers; only the selected band paints
// here), and every visible row's own 1px ButtonShadow bottom grid line (v0:
// horizontal only, no vertical column separators — see the type doc
// comment). All of this runs before RenderWidget draws this DataGrid's
// children (the cell TextBlock pool), so cell text always paints on top of
// these bands/lines.
//
// Header cells/titles are painted at g.colOffsets[i]-g.offsetX — the SAME
// g.offsetX the body cells were arranged with (see ArrangeContent) — so the
// header scrolls horizontally in lockstep with the body while staying
// vertically fixed (g.header.Y never depends on scroll). Unlike the body
// cells (children, automatically cropped by ClipRect when scrolled), the
// header is painted directly here, BEFORE RenderWidget ever pushes that
// clip (see ClipRect's own doc comment: it excludes the header strip
// precisely because the header paints outside its scope) — so once
// contentWidth can exceed the viewport, scrolled header cells could bleed
// past the grid's own left/right edges with no clip to crop them. A
// dedicated PushClip/PopClip pair around the header loops crops that here;
// for any DataGrid whose columns fit the viewport (every existing golden
// and behavior test), the header never actually reaches this clip's edges,
// so painted pixels are unchanged.
func (g *DataGrid) Render(r render.Renderer) {
	drawSunken(r, g.Bounds(), g.colors.WindowWell, g.colors)

	// Clip horizontally only (X/W) — vertically spans the full outer bounds
	// (Y/H) rather than the header's own, possibly sub-pixel, H: the header
	// never draws outside its own row vertically regardless, so there is
	// nothing to crop on that axis, and clipping tightly to a fractional H
	// risks truncating a boundary row of the header's own bevel edges under
	// GL scissor rounding (float->int32 truncation in applyClip). Only X
	// needs a real crop, to keep scrolled header cells from bleeding past
	// the grid's own left/right edges (see the doc comment above).
	bw := g.metrics.BevelWidth
	headerClip := render.Rect{X: g.Bounds().X + bw, Y: g.Bounds().Y, W: g.Bounds().W - 2*bw, H: g.Bounds().H}
	if headerClip.W < 0 {
		headerClip.W = 0
	}
	r.PushClip(headerClip)

	for i := range g.columns {
		if i >= len(g.colOffsets) {
			break
		}
		cellRect := render.Rect{X: g.colOffsets[i] - g.offsetX, Y: g.header.Y, W: g.colWidths[i], H: g.header.H}
		drawRaised(r, cellRect, g.colors.ButtonFace, g.colors)
	}

	if g.face != nil {
		for i, col := range g.columns {
			if i >= len(g.colOffsets) {
				break
			}
			ts := g.face.Measure(col.Title)
			tp := render.Point{
				X: g.colOffsets[i] - g.offsetX + g.metrics.PaddingS,
				Y: g.header.Y + (g.header.H-ts.H)/2,
			}
			g.face.Draw(r, tp, col.Title, g.colors.WindowText)
		}
	}

	r.PopClip()

	// Both the selection band and the grid lines are cropped to the body
	// viewport. They are drawn here, before ClipRect is pushed (RenderWidget
	// runs a widget's own Render BEFORE its clip — only the cell TextBlocks
	// are clipped by it), and the visible range deliberately includes the
	// partial rows at either edge whenever the row height doesn't divide the
	// viewport height. Without the crop the top row's band bleeds up into the
	// header and the bottom row's band and line spill past the body onto the
	// sunken bevel.
	sw := g.metrics.StrokeWidth
	for row := 0; row < g.visibleCount; row++ {
		rowIdx := g.visibleFirst + row
		rowY := g.rowTop(rowIdx)
		rowRect := render.Rect{X: g.viewport.X, Y: rowY, W: g.viewport.W, H: g.rowH}

		if rowIdx == g.selected {
			if band := rowRect.Intersect(g.viewport); !band.Empty() {
				r.FillRect(band, g.colors.Highlight)
			}
		}

		gridLine := render.Rect{X: g.viewport.X, Y: rowRect.Bottom() - sw, W: g.viewport.W, H: sw}
		if gridLine = gridLine.Intersect(g.viewport); gridLine.Empty() {
			continue
		}
		r.FillRect(gridLine, g.colors.ButtonShadow)
	}
}

// ClipRect implements core.ClipProvider, clipping realized cells to the
// grid's bevel-inset content bounds minus the header strip — so a
// partially-scrolled row never bleeds its text up into the header (the
// header itself is drawn in Render, BEFORE this clip is even pushed, so it
// is never affected either way) NOR onto the outer sunken bevel (see
// ArrangeContent's matching inset). Matches ListView.ClipRect in every other
// respect (gutter included, so the thumb — drawn in RenderOverlay, after
// this clip is popped — is never cropped).
func (g *DataGrid) ClipRect() (render.Rect, bool) {
	bw := g.metrics.BevelWidth
	inset := g.Bounds().Inset(render.Thickness{Top: bw, Bottom: bw, Left: bw, Right: bw})
	if inset.W < 0 {
		inset.W = 0
	}
	if inset.H < 0 {
		inset.H = 0
	}
	return inset.Inset(render.Thickness{Top: g.headerHeight()}), true
}

// RenderOverlay implements core.OverlayRenderer, drawing the classic
// track+thumb (drawScrollThumb) for each axis that has content to scroll to
// — vertical along the right edge, horizontal (control-variants Task 4)
// along the bottom — above the clipped body rows, then the focus ring while
// focused — matching ListView.RenderOverlay exactly.
func (g *DataGrid) RenderOverlay(r render.Renderer) {
	if track, _, ok := g.thumbGeometry(); ok {
		thumb, _ := g.thumbRect()
		drawScrollThumb(r, track, thumb, g.colors)
	}
	if track, _, ok := g.thumbGeometryX(); ok {
		thumb, _ := g.thumbRectX()
		drawScrollThumb(r, track, thumb, g.colors)
	}
	if g.focused {
		drawFocusRing(r, g.Bounds(), g.colors)
	}
}

// AcceptsFocus implements input.Focusable: a DataGrid always accepts focus
// (v0 has no SetEnabled/disabled concept, matching ListView).
func (g *DataGrid) AcceptsFocus() bool {
	return true
}

// OnFocusChanged implements input.FocusHandler, tracking focus for the
// focus-ring overlay and Up/Down keyboard navigation.
func (g *DataGrid) OnFocusChanged(focused bool) {
	g.focused = focused
}

// OnPointer implements input.PointerHandler, extending ListView's own
// wheel/thumb-drag/row-click handling with row hover tracking:
//
// Wheel scrolls vertically (row scroll) by scrollWheelStep logical px per
// notch by default, and horizontally instead when Shift is held — mirroring
// ListView.OnPointer's own Wheel handling exactly, including never falling
// back to X on a plain wheel, and including handling the notch only when it
// actually moved the clamped offset (see virtualizer.scrollBy) so a grid
// that can't scroll any further passes the wheel out to an enclosing
// scroller instead of swallowing it.
//
// A Press inside the current vertical thumb rect starts a vertical drag
// (checked first, matching the original priority); otherwise a Press inside
// the current horizontal thumb rect starts a horizontal drag; otherwise a
// Press landing on a real body row (rowAt) selects it as a user-driven
// change (selectUser) and is handled, while a Press over the header, a
// gutter, or empty space below a short grid (rowAt reports ok == false) is
// left unhandled. Either drag records which axis it's tracking (g.drag), so
// a subsequent Move/Release — only acted on while this DataGrid holds the
// capture — knows which offset to update.
//
// Move updates hoverRow when not mid-drag; Leave clears it — hoverRow is
// tracked purely for other consumers, since Render paints no hover fill in
// the classic look. A horizontal drag clears hoverRow for the same reason a
// vertical one already does (see the field doc comment): the offset moved
// without a fresh Y hit-test to re-derive it from.
func (g *DataGrid) OnPointer(e *input.PointerEvent) {
	switch e.Action {
	case input.Wheel:
		delta := -e.Delta.Y * scrollWheelStep
		var moved bool
		if e.Mods&input.ModShift != 0 {
			moved = g.scrollByX(delta)
		} else {
			moved = g.scrollBy(delta)
		}
		if !moved {
			return
		}
		// The offset just moved but this isn't a Move (no fresh pointer
		// position to re-hit-test against), so whatever row hoverRow named
		// no longer necessarily sits under the pointer on screen — clear it
		// rather than leave it naming the wrong row (see the row hover doc
		// comment on hoverRow's field). A notch that scrolled nothing is
		// returned from above before reaching here: the rows didn't move, so
		// hoverRow still names the row under the pointer.
		g.hoverRow = -1
		g.InvalidateArrange()
		e.Handled = true
	case input.Press:
		if rect, ok := g.thumbRect(); ok && rect.Contains(e.Pos) {
			g.dragGrabY = e.Pos.Y - rect.Y
			g.drag = scrollDragVertical
			e.Router.Capture(g)
			e.Handled = true
		} else if rect, ok := g.thumbRectX(); ok && rect.Contains(e.Pos) {
			g.dragGrabX = e.Pos.X - rect.X
			g.drag = scrollDragHorizontal
			e.Router.Capture(g)
			e.Handled = true
		} else if idx, ok := g.rowAt(e.Pos); ok {
			g.selectUser(idx)
			e.Handled = true
		}
	case input.Move:
		if e.Router.Captured() == g {
			if g.drag == scrollDragHorizontal {
				g.dragToX(e.Pos.X)
			} else {
				g.dragTo(e.Pos.Y)
			}
			// Same reasoning as Wheel above: a thumb drag also moves the
			// offset independent of any row hit test, so any previously
			// tracked hover row is now stale.
			g.hoverRow = -1
			g.InvalidateArrange()
			e.Handled = true
		} else if idx, ok := g.rowAt(e.Pos); ok {
			g.hoverRow = idx
		} else {
			g.hoverRow = -1
		}
	case input.Leave:
		g.hoverRow = -1
	case input.Release:
		if e.Router.Captured() == g {
			e.Router.Release()
			g.drag = scrollDragNone
			e.Handled = true
		}
	}
}

// OnKey implements input.KeyHandler: Up/Down move the selection by one row
// (clamped into [0, RowCount()-1] via clampRowIndex — never landing on -1,
// even starting from no selection), as a user-driven change (selectUser),
// auto-scrolled into view. Ignored entirely for anything but
// Action==Press, or when there are no rows at all.
func (g *DataGrid) OnKey(e *input.KeyEvent) {
	if e.Action != input.Press {
		return
	}
	n := g.RowCount()
	if n == 0 {
		return
	}
	switch e.Key {
	case input.KeyUp:
		g.selectUser(clampRowIndex(g.selected-1, n))
		e.Handled = true
	case input.KeyDown:
		g.selectUser(clampRowIndex(g.selected+1, n))
		e.Handled = true
	}
}
