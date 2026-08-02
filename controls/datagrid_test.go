package controls

import (
	"fmt"
	"strings"
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/theme"
)

// layoutDataGrid measures then arranges g at the given bounds, mirroring
// listview_test.go's layoutListView.
func layoutDataGrid(g *DataGrid, x, y, w, h float32) {
	core.MeasureWidget(g, render.Size{W: w, H: h})
	core.ArrangeWidget(g, render.Rect{X: x, Y: y, W: w, H: h})
}

// --- Column resolution (Px/Star) ---

func TestDataGridResolveColumnWidthsPxAndStar(t *testing.T) {
	g := NewDataGrid(nil)
	g.SetColumns(
		Column{Title: "Name", Width: Px(80)},
		Column{Title: "Email", Width: Star(1)},
		Column{Title: "Age", Width: Px(60)},
	)

	widths := g.resolveColumnWidths(300)
	if len(widths) != 3 {
		t.Fatalf("len(widths) = %d, want 3", len(widths))
	}
	if widths[0] != 80 {
		t.Fatalf("widths[0] = %v, want 80 (Px)", widths[0])
	}
	if widths[2] != 60 {
		t.Fatalf("widths[2] = %v, want 60 (Px)", widths[2])
	}
	if widths[1] != 160 {
		t.Fatalf("widths[1] = %v, want 160 (Star gets 300-80-60 remaining)", widths[1])
	}
}

func TestDataGridResolveColumnWidthsStarWeights(t *testing.T) {
	g := NewDataGrid(nil)
	g.SetColumns(
		Column{Title: "A", Width: Star(1)},
		Column{Title: "B", Width: Star(3)},
	)

	widths := g.resolveColumnWidths(400)
	if widths[0] != 100 {
		t.Fatalf("widths[0] = %v, want 100 (weight 1/4 of 400)", widths[0])
	}
	if widths[1] != 300 {
		t.Fatalf("widths[1] = %v, want 300 (weight 3/4 of 400)", widths[1])
	}
}

func TestDataGridSetColumnsAutoTrackPanicsWithColumnIndex(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected SetColumns to panic on an AutoTrack column")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "column 1") {
			t.Fatalf("panic = %v, want a message naming column index 1", r)
		}
	}()

	g := NewDataGrid(nil)
	g.SetColumns(
		Column{Title: "Fine", Width: Px(50)},
		Column{Title: "Bad", Width: AutoTrack()},
	)
}

// --- Column resolution via a real ArrangeContent pass (alignment) ---

func TestDataGridArrangeResolvesColumnsAgainstViewportWidth(t *testing.T) {
	face := testFace(t)
	g := NewDataGrid(face)
	g.SetColumns(
		Column{Title: "Name", Width: Px(80), Value: func(row int) string { return fmt.Sprintf("N%d", row) }},
		Column{Title: "Email", Width: Star(1), Value: func(row int) string { return fmt.Sprintf("E%d", row) }},
		Column{Title: "Age", Width: Px(60), Value: func(row int) string { return fmt.Sprintf("A%d", row) }},
	)
	g.SetRowCount(5)

	// gutter 12, plus the 2px bevel inset on both left+right (ArrangeContent
	// insets bounds by BevelWidth before resolving columns — see
	// theme.MetricTokens.BevelWidth): viewport.W = 380 - 2*2 - 12 = 364.
	layoutDataGrid(g, 0, 0, 380, 200)

	want := []float32{80, 224, 60} // 364 - 80 - 60 = 224 star remainder
	if len(g.colWidths) != 3 {
		t.Fatalf("colWidths = %v, want len 3", g.colWidths)
	}
	for i, w := range want {
		if g.colWidths[i] != w {
			t.Fatalf("colWidths[%d] = %v, want %v", i, g.colWidths[i], w)
		}
	}
	// Offsets start at viewport.X = 2 (bounds.X=0 + BevelWidth), not 0.
	wantOffsets := []float32{2, 82, 306}
	for i, o := range wantOffsets {
		if g.colOffsets[i] != o {
			t.Fatalf("colOffsets[%d] = %v, want %v", i, g.colOffsets[i], o)
		}
	}

	headerH := defaultRowHeight(face, theme.Active())
	numCols := 3
	// pool[0] is row0/col0. Its text is vertically centered within the row
	// (row 0's rect top is bw+headerH, height rowH), so
	// Y = bw+headerH+(rowH-textH)/2.
	rowH := defaultRowHeight(face, theme.Active())
	pad := theme.Active().Metric.PaddingS
	bw := theme.Active().Metric.BevelWidth
	b := g.pool[0].Bounds()
	// Cell text is inset PaddingS left of colOffsets[0] (itself already
	// bevel-inset) to align with the header, and vertically centered in the
	// row (whose own top is pushed down by the bevel too).
	wantY := bw + headerH + (rowH-b.H)/2
	if b.X != bw+pad || b.Y != wantY || b.W != 80-2*pad {
		t.Fatalf("pool[0].Bounds() = %v, want X=%v Y=%v W=%v", b, bw+pad, wantY, 80-2*pad)
	}
	// pool[2] is row0/col2 (Age, Px 60), at the third column offset + inset.
	b2 := g.pool[2].Bounds()
	if b2.X != 306+pad || b2.W != 60-2*pad {
		t.Fatalf("pool[2].Bounds() = %v, want X=%v W=%v", b2, 306+pad, 60-2*pad)
	}
	_ = numCols
}

// --- Virtualized cell pool: size, reuse, and Value-func-driven text ---

func TestDataGridCellPoolSizeIsVisibleRowsTimesColumns(t *testing.T) {
	g := NewDataGrid(nil)
	g.SetColumns(Column{Width: Px(50)}, Column{Width: Star(1)})
	g.SetRowCount(20)
	g.rowH = 48

	// headerHeight() reuses rowH (see its doc comment), so overriding rowH to
	// 48 also makes the header 48 tall. ArrangeContent insets bounds by
	// 2*BevelWidth (4px) before carving off the header, so body viewport H =
	// (148-4)-48 = 96 -> rows 0,1 fit exactly (96/48==2), no partial row = 2.
	layoutDataGrid(g, 0, 0, 100, 148)

	if got, want := len(g.pool), 2*2; got != want {
		t.Fatalf("pool size = %d, want %d (2 visible rows x 2 columns)", got, want)
	}
}

func TestDataGridValueFuncsDriveCellText(t *testing.T) {
	g := NewDataGrid(nil)
	g.SetColumns(
		Column{Width: Px(50), Value: func(row int) string { return fmt.Sprintf("r%dc0", row) }},
		Column{Width: Star(1), Value: func(row int) string { return fmt.Sprintf("r%dc1", row) }},
	)
	g.SetRowCount(20)
	g.rowH = 48

	// header 48 + bevel-inset body viewport 96 (148-2*BevelWidth-48) -> rows
	// 0,1 visible x 2 cols (see TestDataGridCellPoolSizeIsVisibleRowsTimesColumns).
	layoutDataGrid(g, 0, 0, 100, 148)

	want := []string{"r0c0", "r0c1", "r1c0", "r1c1"}
	for i, w := range want {
		if got := g.pool[i].Text(); got != w {
			t.Fatalf("pool[%d].Text() = %q, want %q", i, got, w)
		}
	}
}

func TestDataGridPoolReusesTextBlocksAcrossScroll(t *testing.T) {
	g := NewDataGrid(nil)
	g.SetColumns(
		Column{Width: Px(50), Value: func(row int) string { return fmt.Sprintf("r%dc0", row) }},
		Column{Width: Star(1), Value: func(row int) string { return fmt.Sprintf("r%dc1", row) }},
	)
	g.SetRowCount(20)
	g.rowH = 48

	// header 48 + bevel-inset body viewport 96 (148-2*BevelWidth-48) -> rows
	// 0,1 visible.
	layoutDataGrid(g, 0, 0, 100, 148)

	if len(g.pool) != 4 {
		t.Fatalf("pool size = %d, want 4", len(g.pool))
	}
	before := make([]*TextBlock, len(g.pool))
	copy(before, g.pool)

	g.rawOffset = 2 * 48 // scroll exactly 2 rows: visible rows become 2,3
	layoutDataGrid(g, 0, 0, 100, 148)

	if len(g.pool) != 4 {
		t.Fatalf("pool size after scroll = %d, want 4 (unchanged visible count)", len(g.pool))
	}
	for i := range before {
		if g.pool[i] != before[i] {
			t.Fatalf("pool[%d] pointer changed after scroll (want SAME *TextBlock, re-texted)", i)
		}
	}
	if got, want := g.pool[0].Text(), "r2c0"; got != want {
		t.Fatalf("pool[0].Text() after scroll = %q, want %q", got, want)
	}
	if got, want := g.pool[3].Text(), "r3c1"; got != want {
		t.Fatalf("pool[3].Text() after scroll = %q, want %q", got, want)
	}
}

// --- RowCount changes ---

func TestDataGridRowCountChangeReflectedAfterRelayout(t *testing.T) {
	g := NewDataGrid(nil)
	g.SetColumns(Column{Width: Px(50)}, Column{Width: Star(1)})
	g.SetRowCount(2)
	g.rowH = 48

	layoutDataGrid(g, 0, 0, 100, 500) // tall viewport: all 2 rows visible

	if got := len(g.pool); got != 4 {
		t.Fatalf("pool size before SetRowCount = %d, want 4 (2 rows x 2 cols)", got)
	}

	g.SetRowCount(5)
	if !g.NeedsLayout() {
		t.Fatal("SetRowCount did not invalidate the DataGrid")
	}

	layoutDataGrid(g, 0, 0, 100, 500)
	if got := len(g.pool); got != 10 {
		t.Fatalf("pool size after SetRowCount(5) + relayout = %d, want 10 (5 rows x 2 cols)", got)
	}
	if got := g.RowCount(); got != 5 {
		t.Fatalf("RowCount() = %d, want 5", got)
	}
}

// --- Selection: SetSelectedIndex (silent, clamped) ---

func TestDataGridSetSelectedIndexIsSilentAndClamped(t *testing.T) {
	g := NewDataGrid(nil)
	g.SetColumns(Column{Width: Px(50)})
	g.SetRowCount(3)

	fired := false
	g.OnChanged(func(int) { fired = true })

	g.SetSelectedIndex(1)
	if got := g.SelectedIndex(); got != 1 {
		t.Fatalf("SelectedIndex() = %d, want 1", got)
	}

	g.SetSelectedIndex(99)
	if got := g.SelectedIndex(); got != 2 {
		t.Fatalf("SelectedIndex() after over-range SetSelectedIndex(99) = %d, want 2 (clamped)", got)
	}

	g.SetSelectedIndex(-5)
	if got := g.SelectedIndex(); got != -1 {
		t.Fatalf("SelectedIndex() after under-range SetSelectedIndex(-5) = %d, want -1", got)
	}

	if fired {
		t.Fatal("SetSelectedIndex fired OnChanged, want fully silent")
	}
}

// --- Selection: click ---

func TestDataGridClickRowSelectsAndFiresOnChanged(t *testing.T) {
	g := NewDataGrid(nil)
	g.SetColumns(Column{Width: Px(50)})
	g.SetRowCount(5)
	g.rowH = 48

	layoutDataGrid(g, 0, 0, 100, 148) // headerH=48 (reuses rowH); rows 0,1,2(partial) visible

	var got []int
	g.OnChanged(func(v int) { got = append(got, v) })

	r := input.NewRouter()
	e := &input.PointerEvent{Action: input.Press, Pos: render.Point{X: 10, Y: 48 + 60}, Router: r}
	g.OnPointer(e)

	if !e.Handled {
		t.Fatal("press on a real row not marked Handled")
	}
	if got, want := g.SelectedIndex(), 1; got != want {
		t.Fatalf("SelectedIndex() = %d, want %d", got, want)
	}
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("OnChanged calls = %v, want [1]", got)
	}
}

func TestDataGridClickOnHeaderDoesNotSelect(t *testing.T) {
	g := NewDataGrid(nil)
	g.SetColumns(Column{Width: Px(50)})
	g.SetRowCount(5)
	g.rowH = 48

	layoutDataGrid(g, 0, 0, 100, 108)

	r := input.NewRouter()
	e := &input.PointerEvent{Action: input.Press, Pos: render.Point{X: 10, Y: 4}, Router: r} // inside the 8px header
	g.OnPointer(e)

	if e.Handled {
		t.Fatal("press on the header marked Handled, want false")
	}
	if g.SelectedIndex() != -1 {
		t.Fatalf("SelectedIndex() = %d after header click, want -1 (unchanged)", g.SelectedIndex())
	}
}

// --- Row hover ---

// TestDataGridHoverClearedOnWheel guards against hoverRow going stale: it is
// an absolute row index, so scrolling via Wheel without a fresh Move to
// re-hit-test against must clear it rather than leave a stale hover
// index pointing at whatever row now occupies that slot's new on-screen
// position (see hoverRow's field doc comment).
func TestDataGridHoverClearedOnWheel(t *testing.T) {
	g := NewDataGrid(nil)
	g.SetColumns(Column{Width: Px(50)})
	g.SetRowCount(20)
	g.rowH = 48

	layoutDataGrid(g, 0, 0, 100, 148) // headerH=48; rows 0,1,2 visible

	r := input.NewRouter()
	move := &input.PointerEvent{Action: input.Move, Pos: render.Point{X: 10, Y: 48 + 10}, Router: r} // row 0
	g.OnPointer(move)
	if g.hoverRow != 0 {
		t.Fatalf("hoverRow after Move = %d, want 0", g.hoverRow)
	}

	wheel := &input.PointerEvent{Action: input.Wheel, Delta: render.Point{Y: -1}, Router: r}
	g.OnPointer(wheel)
	if !wheel.Handled {
		t.Fatal("wheel event not marked Handled")
	}
	if g.hoverRow != -1 {
		t.Fatalf("hoverRow after Wheel = %d, want -1 (cleared, stale after scroll)", g.hoverRow)
	}
}

// TestDataGridOverscrollXLeavesNoDeadZone pins the end-stop dead zone on the
// virtualizer's side of the clamp: ScrollToX writes rawOffsetX unbounded, so
// a request far past the end used to sit there while offsetX stayed pinned at
// the max, and every notch back had to burn off the whole overshoot before
// anything moved.
func TestDataGridOverscrollXLeavesNoDeadZone(t *testing.T) {
	g := NewDataGrid(nil)
	g.SetColumns(Column{Width: Px(300)})
	g.SetRowCount(2)
	g.rowH = 48
	layoutDataGrid(g, 0, 0, 100, 200)

	maxX := g.contentW - g.viewport.W
	if maxX <= 2*scrollWheelStep {
		t.Fatalf("fixture: horizontal range %v is too short to over-scroll meaningfully", maxX)
	}

	g.ScrollToX(10000)
	layoutDataGrid(g, 0, 0, 100, 200)
	if got := g.OffsetX(); got != maxX {
		t.Fatalf("offsetX after over-scrolling = %v, want the %v end stop", got, maxX)
	}

	r := input.NewRouter()
	back := &input.PointerEvent{Action: input.Wheel, Delta: render.Point{Y: 1}, Mods: input.ModShift, Router: r}
	g.OnPointer(back)
	if !back.Handled {
		t.Fatal("a notch back off the horizontal end stop went unhandled — the raw accumulator kept the overshoot")
	}
	layoutDataGrid(g, 0, 0, 100, 200)
	if got := g.OffsetX(); got != maxX-scrollWheelStep {
		t.Fatalf("offsetX one notch back from the end stop = %v, want %v", got, maxX-scrollWheelStep)
	}
}

// TestDataGridWheelNotHandledWhenRowsFit pins the same wheel-consumed-but-
// nothing-moved rule ListView and ScrollViewer follow: a grid whose rows all
// fit scrolls nothing, so the notch must stay unhandled for an enclosing
// scroller (input.Bubble stops at the first handler that sets Handled).
// Nothing moved, so the hovered row is still the hovered row.
func TestDataGridWheelNotHandledWhenRowsFit(t *testing.T) {
	g := NewDataGrid(nil)
	g.SetColumns(Column{Width: Px(50)})
	g.SetRowCount(2)
	g.rowH = 48

	layoutDataGrid(g, 0, 0, 100, 400) // headerH=48, 96px of rows, way short of the body
	if _, ok := g.thumbRect(); ok {
		t.Fatal("fixture: the grid must fit its own viewport (no thumb)")
	}

	r := input.NewRouter()
	g.OnPointer(&input.PointerEvent{Action: input.Move, Pos: render.Point{X: 10, Y: 48 + 10}, Router: r})
	if g.hoverRow != 0 {
		t.Fatalf("fixture: hoverRow after Move = %d, want 0", g.hoverRow)
	}

	wheel := &input.PointerEvent{Action: input.Wheel, Delta: render.Point{Y: -1}, Router: r}
	g.OnPointer(wheel)
	if wheel.Handled {
		t.Fatal("wheel over a grid with nothing to scroll was marked Handled")
	}
	if g.hoverRow != 0 {
		t.Fatalf("hoverRow after a wheel that scrolled nothing = %d, want 0 (the rows did not move)", g.hoverRow)
	}
}

// TestDataGridHoverClearedOnThumbDrag mirrors the Wheel case above for the
// other offset-changing-without-a-fresh-hit-test path: dragging the thumb.
func TestDataGridHoverClearedOnThumbDrag(t *testing.T) {
	g := NewDataGrid(nil)
	g.SetColumns(Column{Width: Px(50)})
	g.SetRowCount(20)
	g.rowH = 48

	layoutDataGrid(g, 0, 0, 100, 148) // headerH=48; rows 0,1,2 visible; content 960 > viewport 100 -> thumb present

	r := input.NewRouter()
	move := &input.PointerEvent{Action: input.Move, Pos: render.Point{X: 10, Y: 48 + 10}, Router: r} // row 0
	g.OnPointer(move)
	if g.hoverRow != 0 {
		t.Fatalf("hoverRow after Move = %d, want 0", g.hoverRow)
	}

	rect, ok := g.thumbRect()
	if !ok {
		t.Fatal("expected a thumb (content taller than viewport)")
	}

	press := &input.PointerEvent{Action: input.Press, Pos: render.Point{X: rect.X, Y: rect.Y}, Router: r}
	g.OnPointer(press) // captures g on the real router (OnPointer's own e.Router.Capture(g))
	if !press.Handled {
		t.Fatal("press on the thumb not marked Handled")
	}

	drag := &input.PointerEvent{Action: input.Move, Pos: render.Point{X: rect.X, Y: rect.Y + 20}, Router: r}
	g.OnPointer(drag)
	if !drag.Handled {
		t.Fatal("drag move not marked Handled")
	}
	if g.hoverRow != -1 {
		t.Fatalf("hoverRow after thumb drag = %d, want -1 (cleared, stale after scroll)", g.hoverRow)
	}
}

// --- Selection: keyboard Up/Down ---

func TestDataGridKeyboardUpDownWhenFocused(t *testing.T) {
	g := NewDataGrid(nil)
	g.SetColumns(Column{Width: Px(50)})
	g.SetRowCount(5)
	g.rowH = 48
	layoutDataGrid(g, 0, 0, 100, 500) // all rows visible, nothing to auto-scroll
	g.OnFocusChanged(true)

	var got []int
	g.OnChanged(func(v int) { got = append(got, v) })

	press := func(k input.Key) {
		e := &input.KeyEvent{Action: input.Press, Key: k}
		g.OnKey(e)
		if !e.Handled {
			t.Fatalf("key %v not marked Handled", k)
		}
	}

	press(input.KeyDown) // no prior selection -> lands on row 0
	if got := g.SelectedIndex(); got != 0 {
		t.Fatalf("SelectedIndex() after first Down = %d, want 0", got)
	}
	press(input.KeyDown)
	if got := g.SelectedIndex(); got != 1 {
		t.Fatalf("SelectedIndex() after second Down = %d, want 1", got)
	}
	press(input.KeyUp)
	if got := g.SelectedIndex(); got != 0 {
		t.Fatalf("SelectedIndex() after Up = %d, want 0", got)
	}
	press(input.KeyUp) // clamped: stays at 0
	if got := g.SelectedIndex(); got != 0 {
		t.Fatalf("SelectedIndex() after Up at row 0 = %d, want clamped 0", got)
	}

	want := []int{0, 1, 0}
	if len(got) != len(want) {
		t.Fatalf("OnChanged calls = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("OnChanged calls = %v, want %v", got, want)
		}
	}
}

func TestDataGridKeyboardIgnoredWhenNoRows(t *testing.T) {
	g := NewDataGrid(nil)
	g.SetColumns(Column{Width: Px(50)})
	g.OnFocusChanged(true)

	e := &input.KeyEvent{Action: input.Press, Key: input.KeyDown}
	g.OnKey(e)
	if e.Handled {
		t.Fatal("Down with zero rows marked Handled, want false")
	}
}

// --- Selection: auto-scroll into view ---

func TestDataGridSetSelectedIndexAutoScrollsIntoView(t *testing.T) {
	g := NewDataGrid(nil)
	g.SetColumns(Column{Width: Px(50)})
	g.SetRowCount(20) // 20*48 = 960 content
	g.rowH = 48
	// header 48 + bevel-inset body viewport H=96 (148-2*BevelWidth-48), rows
	// 0,1 visible, offset 0.
	layoutDataGrid(g, 0, 0, 100, 148)

	if got := g.offset; got != 0 {
		t.Fatalf("initial offset = %v, want 0", got)
	}

	g.SetSelectedIndex(19) // last row, off-screen while scrolled to top

	layoutDataGrid(g, 0, 0, 100, 148) // re-layout applies the pending rawOffset

	if got := g.SelectedIndex(); got != 19 {
		t.Fatalf("SelectedIndex() = %d, want 19", got)
	}
	wantOffset := float32(20)*48 - 96 // last row's bottom edge minus viewport H (96)
	if got := g.offset; got != wantOffset {
		t.Fatalf("offset after auto-scroll = %v, want %v", got, wantOffset)
	}
}

// TestDataGridBandAndGridLinesStayInBounds pins that the selection band and
// the per-row grid line for a partially-visible row are cropped to the body
// viewport. Render draws both before ClipRect is pushed (core.RenderWidget
// runs a widget's own Render first), so nothing crops them for us: with a row
// height that doesn't divide the body height, the bottom row is partly
// visible and its band and line used to paint out past the well and over
// whatever sits below the control.
func TestDataGridBandAndGridLinesStayInBounds(t *testing.T) {
	// header 20 tall, body viewport {2,22,84,86}: 86 is not a multiple of the
	// 20px rows, so a row hangs off an edge at most offsets. The offset is set
	// directly rather than through SetSelectedIndex, whose scroll-into-view
	// would align the selected row flush with an edge and hide the bug.
	cases := []struct {
		name     string
		selected int
		offset   float32
	}{
		{"partial bottom row", 4, 5}, // band 97..117, viewport ends at 108
		{"partial top row", 0, 5},    // band 17..37, viewport starts at 22
	}

	bounds := render.Rect{X: 0, Y: 0, W: 100, H: 110}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewDataGrid(nil)
			g.SetColumns(Column{Width: Px(50)})
			g.SetRowCount(10)
			g.rowH = 20
			g.selected = tc.selected
			g.rawOffset = tc.offset
			layoutDataGrid(g, bounds.X, bounds.Y, bounds.W, bounds.H)

			lastRowBottom := g.viewport.Y + float32(g.visibleFirst+g.visibleCount)*g.rowH - g.offset
			if lastRowBottom <= g.viewport.Bottom() {
				t.Fatalf("fixture: the last visible row ends at %v, inside the viewport's %v — nothing overhangs to check",
					lastRowBottom, g.viewport.Bottom())
			}

			rr := &recordRenderer{}
			g.Render(rr)

			if len(rr.fills) == 0 {
				t.Fatal("DataGrid.Render emitted no fills at all")
			}
			var band bool
			for _, f := range rr.fills {
				if f.color == g.colors.Highlight {
					band = true
					// The band belongs to the body: it must not bleed up into
					// the header strip drawn above it either.
					if f.rect.Y < g.viewport.Y {
						t.Fatalf("selection band %v starts at %v, above the body viewport at %v",
							f.rect, f.rect.Y, g.viewport.Y)
					}
				}
				if f.rect.Bottom() > bounds.Bottom() {
					t.Fatalf("Render emitted %v, reaching %v — past the control's bottom edge at %v",
						f.rect, f.rect.Bottom(), bounds.Bottom())
				}
			}
			if !band {
				t.Fatal("fixture: no selection band was drawn, so nothing was actually checked")
			}
		})
	}
}

// --- Header fixed while body scrolls ---

func TestDataGridHeaderYConstantWhileBodyScrolls(t *testing.T) {
	g := NewDataGrid(nil)
	g.SetColumns(Column{Width: Px(50)})
	g.SetRowCount(20)
	g.rowH = 48

	layoutDataGrid(g, 10, 20, 100, 116)
	firstHeaderY := g.header.Y
	// header.Y = bounds.Y + BevelWidth: the header's raised cell buttons sit
	// inset from the outer sunken well's frame, not flush against bounds.Y.
	if firstHeaderY != 22 {
		t.Fatalf("header.Y = %v, want 22 (bounds.Y=20 + BevelWidth=2)", firstHeaderY)
	}

	g.rawOffset = 300 // scroll the body well past the top
	layoutDataGrid(g, 10, 20, 100, 116)

	if g.header.Y != firstHeaderY {
		t.Fatalf("header.Y after scroll = %v, want unchanged %v", g.header.Y, firstHeaderY)
	}
	if g.offset == 0 {
		t.Fatal("expected the body offset to have actually moved for this to be a meaningful check")
	}
}

// --- Dispose is a harmless no-op ---

func TestDataGridDisposeIsNoOp(t *testing.T) {
	g := NewDataGrid(nil)
	g.SetColumns(Column{Width: Px(50)})
	g.SetRowCount(3)
	layoutDataGrid(g, 0, 0, 100, 100)

	g.Dispose() // must not panic, must not affect state
	if g.RowCount() != 3 {
		t.Fatalf("RowCount() after Dispose = %d, want 3", g.RowCount())
	}
}

// --- Horizontal scroll (control-variants Task 4) ---

func TestDataGridNoHorizontalThumbWhenColumnsFitViewport(t *testing.T) {
	// The existing datagrid.png golden's own shape: a Star column makes
	// sum(colWidths) resolve to exactly the viewport width, never more — see
	// ArrangeContent's doc comment on why this can never spuriously overflow.
	g := NewDataGrid(nil)
	g.SetColumns(Column{Width: Px(80)}, Column{Width: Star(1)}, Column{Width: Px(60)})
	g.SetRowCount(5)
	layoutDataGrid(g, 0, 0, 300, 180)

	if _, ok := g.thumbRectX(); ok {
		t.Fatal("expected no horizontal thumb: a Star column always fills exactly to the viewport width")
	}
	if g.offsetX != 0 {
		t.Fatalf("offsetX = %v, want 0", g.offsetX)
	}
}

// TestDataGridNoHorizontalThumbWhenEqualStarsDrift is the case the test
// above happens not to hit: its Star column is the only one, so its share is
// the whole remainder and nothing can round. Several EQUAL Stars each round
// their own share in float32 and used to overshoot the width they were
// resolved against — 3 across a 200-wide grid summed to 200.00002 — which
// the exact contentW > viewport.W comparison read as real overflow: a
// horizontal thumb with a scroll range of ~1.5e-5 px, plus a reserved bottom
// gutter that costs a visible row.
func TestDataGridNoHorizontalThumbWhenEqualStarsDrift(t *testing.T) {
	for _, n := range []int{3, 6} {
		cols := make([]Column, n)
		for i := range cols {
			cols[i] = Column{Width: Star(1)}
		}
		g := NewDataGrid(nil)
		g.SetColumns(cols...)
		g.SetRowCount(5)
		g.rowH = 20
		layoutDataGrid(g, 0, 0, 200, 200)

		if g.contentW > g.viewport.W {
			t.Fatalf("%d equal Star columns: contentW %v exceeds viewport.W %v by %v — the shares must tile it exactly",
				n, g.contentW, g.viewport.W, g.contentW-g.viewport.W)
		}
		if _, ok := g.thumbRectX(); ok {
			t.Fatalf("%d equal Star columns reported a horizontal thumb; they exactly fill the viewport", n)
		}
		// No phantom overflow means no bottom gutter either, so the body keeps
		// the row the gutter would have cost it.
		wantH := float32(200) - 2*g.metrics.BevelWidth - g.headerHeight()
		if g.viewport.H != wantH {
			t.Fatalf("%d equal Star columns: viewport.H = %v, want %v — a bottom gutter was reserved for phantom overflow",
				n, g.viewport.H, wantH)
		}
	}
}

func TestDataGridHorizontalThumbShowsWhenColumnsOverflow(t *testing.T) {
	// Px-only columns whose sum exceeds the viewport: the deliberate
	// horizontal-overflow scenario this feature targets.
	g := NewDataGrid(nil)
	g.SetColumns(Column{Width: Px(150)}, Column{Width: Px(150)}, Column{Width: Px(150)})
	g.SetRowCount(5)
	layoutDataGrid(g, 0, 0, 200, 200)

	if _, ok := g.thumbRectX(); !ok {
		t.Fatal("expected a horizontal thumb: 3x150=450 Px columns exceed the viewport width")
	}
}

func TestDataGridHeaderBodyOffsetXInSync(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	colors := theme.Active().Color
	pad := theme.Active().Metric.PaddingS

	g := NewDataGrid(nil) // nil face: Render's title loop is skipped, only cell/header fills recorded
	g.SetColumns(
		Column{Title: "A", Width: Px(80)},
		Column{Title: "B", Width: Px(80)},
		Column{Title: "C", Width: Px(80)},
	)
	g.SetRowCount(5)
	g.rowH = 48

	// 3*80=240 content width vs a 100px-wide grid: definitely overflows
	// horizontally (see TestDataGridHorizontalThumbShowsWhenColumnsOverflow).
	layoutDataGrid(g, 0, 0, 100, 200)
	g.rawOffsetX = 50
	layoutDataGrid(g, 0, 0, 100, 200)

	if g.offsetX == 0 {
		t.Fatal("expected a nonzero clamped offsetX for this test to be meaningful")
	}

	rr := &recordRenderer{}
	g.Render(rr)

	// drawRaised's very first FillRect for each header cell is the cell's
	// own face fill, passed the exact cellRect (see bevel.go's doc comment):
	// unlike any inner/outer edge (all 1px thick on one axis), only that
	// face fill matches BOTH the header's own Y and its full H.
	var headerXs []float32
	for _, f := range rr.fills {
		if f.color == colors.ButtonFace && f.rect.Y == g.header.Y && f.rect.H == g.header.H {
			headerXs = append(headerXs, f.rect.X)
		}
	}
	if len(headerXs) != len(g.columns) {
		t.Fatalf("found %d header cell face fills, want %d", len(headerXs), len(g.columns))
	}
	for i, x := range headerXs {
		want := g.colOffsets[i] - g.offsetX
		if x != want {
			t.Fatalf("header cell %d X = %v, want %v (colOffsets[%d] - offsetX)", i, x, want, i)
		}
	}

	// Body cells (row 0) must scroll by the SAME offsetX — see
	// ArrangeContent's cell-rect inset (colOffsets[c] + PaddingS - offsetX).
	for c := range g.columns {
		want := g.colOffsets[c] + pad - g.offsetX
		if got := g.pool[c].Bounds().X; got != want {
			t.Fatalf("pool[%d] (row0 col%d) Bounds().X = %v, want %v", c, c, got, want)
		}
	}
}

func TestDataGridShiftWheelScrollsXPlainWheelScrollsY(t *testing.T) {
	g := NewDataGrid(nil)
	g.SetColumns(Column{Width: Px(150)}, Column{Width: Px(150)})
	g.SetRowCount(20)
	g.rowH = 48
	layoutDataGrid(g, 0, 0, 100, 148)

	r := input.NewRouter()

	plain := &input.PointerEvent{Action: input.Wheel, Delta: render.Point{Y: -2}, Router: r}
	g.OnPointer(plain)
	if !plain.Handled {
		t.Fatal("plain wheel not marked Handled")
	}
	layoutDataGrid(g, 0, 0, 100, 148)
	if g.offset == 0 {
		t.Fatal("offset (Y) = 0 after plain wheel, want nonzero")
	}
	if g.offsetX != 0 {
		t.Fatalf("offsetX = %v after plain wheel, want 0 (plain wheel never scrolls X)", g.offsetX)
	}
	yAfterPlain := g.offset

	shift := &input.PointerEvent{Action: input.Wheel, Delta: render.Point{Y: -2}, Mods: input.ModShift, Router: r}
	g.OnPointer(shift)
	if !shift.Handled {
		t.Fatal("shift+wheel not marked Handled")
	}
	layoutDataGrid(g, 0, 0, 100, 148)
	if g.offset != yAfterPlain {
		t.Fatalf("offset (Y) = %v after shift+wheel, want unchanged %v", g.offset, yAfterPlain)
	}
	if g.offsetX == 0 {
		t.Fatal("offsetX = 0 after shift+wheel, want nonzero (shift+wheel scrolls X)")
	}
}

func TestDataGridRowClickCorrectWithNonzeroOffsetX(t *testing.T) {
	g := NewDataGrid(nil)
	g.SetColumns(Column{Width: Px(150)}, Column{Width: Px(150)})
	g.SetRowCount(5)
	g.rowH = 48
	layoutDataGrid(g, 0, 0, 100, 200)

	g.rawOffsetX = 80
	layoutDataGrid(g, 0, 0, 100, 200)
	if g.offsetX == 0 {
		t.Fatal("expected a nonzero offsetX for this test to be meaningful")
	}

	var got []int
	g.OnChanged(func(v int) { got = append(got, v) })

	r := input.NewRouter()
	// Row 1's Y-band is unaffected by horizontal scroll: clicking there must
	// still resolve to row 1 regardless of the scrolled columns.
	pos := render.Point{X: g.viewport.X + 5, Y: g.viewport.Y + g.rowH + 10}
	e := &input.PointerEvent{Action: input.Press, Pos: pos, Router: r}
	g.OnPointer(e)

	if !e.Handled {
		t.Fatal("press on a real row not marked Handled")
	}
	if got, want := g.SelectedIndex(), 1; got != want {
		t.Fatalf("SelectedIndex() = %d, want %d (row hit-test unaffected by horizontal scroll)", got, want)
	}
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("OnChanged calls = %v, want [1]", got)
	}
}

func TestDataGridScrollToXAndOffsetX(t *testing.T) {
	g := NewDataGrid(nil)
	g.SetColumns(Column{Width: Px(150)}, Column{Width: Px(150)})
	g.SetRowCount(5)
	layoutDataGrid(g, 0, 0, 100, 200)

	g.ScrollToX(30)
	if !g.NeedsLayout() {
		t.Fatal("ScrollToX did not invalidate arrange")
	}
	layoutDataGrid(g, 0, 0, 100, 200)
	if got := g.OffsetX(); got != 30 {
		t.Fatalf("OffsetX() = %v, want 30", got)
	}
}
