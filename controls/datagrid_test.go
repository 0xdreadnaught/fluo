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
// re-hit-test against must clear it rather than leave the ControlFillHover
// band painting on whatever row now occupies that index's new on-screen
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
