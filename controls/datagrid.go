package controls

import (
	"fmt"
	"math"

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
// directly — LayerBackground fill, TextSecondary titles, a 1px ControlStroke
// bottom border) sitting above a VIRTUALIZED body that reuses the same
// uniform-row virtualizer ListView is built on (see virtualizer.go) for its
// viewport/scroll/thumb math. Only the body scrolls; the header's own rect
// depends solely on the grid's arranged bounds, never on scroll offset.
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
// separately from selection) fills ControlFillHover; the selected row fills
// SelectionBackground with SelectionForeground cell text.
//
// v0 grid lines: horizontal only — a 1px ControlStroke line beneath every
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
	// that has since scrolled to a different on-screen position —
	// OnPointer clears it to -1 in both of those cases rather than let the
	// hover band paint on the wrong row.
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
	idx = int(math.Floor(float64((pos.Y - g.viewport.Y + g.offset) / g.rowH)))
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
// column-width resolution, virtualizer offset clamping (via
// virtualizer.layout against the BODY viewport only — header space is
// carved off bounds first), and body cell realization: it resizes the
// TextBlock pool to exactly visibleRows*numCols (see shrinkPool and the
// grow/reuse branches below, mirroring ListView.ArrangeContent's own
// convention of re-texting existing entries in place rather than
// reallocating), arranges each pool entry at its (row,col) cell rect, and
// recolors it: SelectionForeground for the selected row's cells, TextPrimary
// otherwise (set unconditionally on every pass, like ListView, so a
// selection change alone still repaints the right cells next pass).
func (g *DataGrid) ArrangeContent(bounds render.Rect) {
	headerH := g.headerHeight()
	g.header = render.Rect{X: bounds.X, Y: bounds.Y, W: bounds.W, H: headerH}

	body := bounds.Inset(render.Thickness{Top: headerH})
	viewport := body.Inset(render.Thickness{Right: g.gutter})
	if viewport.W < 0 {
		viewport.W = 0
	}
	if viewport.H < 0 {
		viewport.H = 0
	}

	g.layout(viewport)

	numCols := len(g.columns)
	colWidths := g.resolveColumnWidths(viewport.W)
	colOffsets := make([]float32, numCols)
	x := viewport.X
	for i, w := range colWidths {
		colOffsets[i] = x
		x += w
	}
	g.colWidths = colWidths
	g.colOffsets = colOffsets

	first, last := g.visibleRange()
	n := last - first
	g.visibleFirst = first
	g.visibleCount = n
	g.shrinkPool(n * numCols)

	for row := 0; row < n; row++ {
		rowIdx := first + row
		rowY := viewport.Y + float32(rowIdx)*g.rowH - g.offset

		for c := 0; c < numCols; c++ {
			slot := row*numCols + c

			var text string
			if g.columns[c].Value != nil {
				text = g.columns[c].Value(rowIdx)
			}

			var tb *TextBlock
			if slot < len(g.pool) {
				// Reuse: re-text ONLY when the value actually changed, per
				// ListView.ArrangeContent's own reuse convention (see its
				// doc comment for why unconditional SetText would leave
				// this grid spuriously measure-dirty).
				tb = g.pool[slot]
				if tb.Text() != text {
					tb.SetText(text)
				}
			} else {
				// Grow: construct WITH the correct text directly, before
				// SetParent, so a freshly grown slot never fires
				// TextBlock's invalidate-parent hook against a not-yet-set
				// parent.
				tb = NewTextBlock(g.face, text)
				core.SetParent(tb, g)
				g.pool = append(g.pool, tb)
			}

			color := g.colors.TextPrimary
			if rowIdx == g.selected {
				color = g.colors.SelectionForeground
			}
			tb.SetColor(color)

			cellRect := render.Rect{X: colOffsets[c], Y: rowY, W: colWidths[c], H: g.rowH}
			core.MeasureWidget(tb, render.Size{W: colWidths[c], H: g.rowH})
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

// Render draws the header (LayerBackground fill, TextSecondary titles, 1px
// ControlStroke bottom border — all independent of scroll offset, per the
// type doc comment) followed by the body's per-row backgrounds: the
// selected row's SelectionBackground band, the hovered row's
// ControlFillHover band (skipped for the selected row itself, so selection
// always wins visually), and every visible row's own 1px ControlStroke
// bottom grid line (v0: horizontal only, no vertical column separators —
// see the type doc comment). All of this runs before RenderWidget draws
// this DataGrid's children (the cell TextBlock pool), so cell text always
// paints on top of these bands/lines.
func (g *DataGrid) Render(r render.Renderer) {
	r.FillRect(g.header, g.colors.LayerBackground)

	if g.face != nil {
		for i, col := range g.columns {
			if i >= len(g.colOffsets) {
				break
			}
			ts := g.face.Measure(col.Title)
			tp := render.Point{
				X: g.colOffsets[i] + g.metrics.PaddingS,
				Y: g.header.Y + (g.header.H-ts.H)/2,
			}
			g.face.Draw(r, tp, col.Title, g.colors.TextSecondary)
		}
	}

	sw := g.metrics.StrokeWidth
	headerBorder := render.Rect{X: g.header.X, Y: g.header.Bottom() - sw, W: g.header.W, H: sw}
	r.FillRect(headerBorder, g.colors.ControlStroke)

	for row := 0; row < g.visibleCount; row++ {
		rowIdx := g.visibleFirst + row
		rowY := g.viewport.Y + float32(rowIdx)*g.rowH - g.offset
		rowRect := render.Rect{X: g.viewport.X, Y: rowY, W: g.viewport.W, H: g.rowH}

		switch {
		case rowIdx == g.selected:
			r.FillRect(rowRect, g.colors.SelectionBackground)
		case rowIdx == g.hoverRow:
			r.FillRect(rowRect, g.colors.ControlFillHover)
		}

		gridLine := render.Rect{X: g.viewport.X, Y: rowRect.Bottom() - sw, W: g.viewport.W, H: sw}
		r.FillRect(gridLine, g.colors.ControlStroke)
	}
}

// ClipRect implements core.ClipProvider, clipping realized cells to the
// grid's own bounds minus the header strip — so a partially-scrolled row at
// the top edge of the body never bleeds its text up into the header (the
// header itself is drawn in Render, BEFORE this clip is even pushed, so it
// is never affected either way). Matches ListView.ClipRect in every other
// respect (gutter included, so the thumb — drawn in RenderOverlay, after
// this clip is popped — is never cropped).
func (g *DataGrid) ClipRect() (render.Rect, bool) {
	return g.Bounds().Inset(render.Thickness{Top: g.headerHeight()}), true
}

// RenderOverlay implements core.OverlayRenderer, drawing the thumb above the
// clipped body rows when there is content to scroll to, then the focus ring
// while focused — matching ListView.RenderOverlay exactly.
func (g *DataGrid) RenderOverlay(r render.Renderer) {
	if rect, ok := g.thumbRect(); ok {
		r.FillRoundedRect(rect, g.thumbRadius, g.thumbColor)
	}
	if g.focused {
		drawFocusRing(r, g.Bounds(), g.metrics.ControlCornerRadius, g.colors, g.metrics)
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
// wheel/thumb-drag/row-click handling with row hover tracking: Wheel scrolls
// by scrollWheelStep logical px per notch and is always handled; a Press
// inside the current thumb rect starts a drag and is handled; otherwise a
// Press landing on a real body row (rowAt) selects it as a user-driven
// change (selectUser) and is handled, while a Press over the header, the
// gutter, or empty space below a short grid (rowAt reports ok == false) is
// left unhandled. Move updates hoverRow (for the ControlFillHover band) when
// not mid-drag; Leave clears it.
func (g *DataGrid) OnPointer(e *input.PointerEvent) {
	switch e.Action {
	case input.Wheel:
		g.scrollBy(-e.Delta.Y * scrollWheelStep)
		// The offset just moved but this isn't a Move (no fresh pointer
		// position to re-hit-test against), so whatever row hoverRow named
		// no longer necessarily sits under the pointer on screen — clear it
		// rather than paint ControlFillHover on the wrong row (see the row
		// hover doc comment on hoverRow's field).
		g.hoverRow = -1
		g.InvalidateArrange()
		e.Handled = true
	case input.Press:
		if rect, ok := g.thumbRect(); ok && rect.Contains(e.Pos) {
			g.dragGrabY = e.Pos.Y - rect.Y
			e.Router.Capture(g)
			e.Handled = true
		} else if idx, ok := g.rowAt(e.Pos); ok {
			g.selectUser(idx)
			e.Handled = true
		}
	case input.Move:
		if e.Router.Captured() == g {
			g.dragTo(e.Pos.Y)
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
