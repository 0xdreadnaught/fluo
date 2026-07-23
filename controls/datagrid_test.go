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

	layoutDataGrid(g, 0, 0, 380, 200) // gutter 12 -> viewport.W = 368

	want := []float32{80, 228, 60} // 368 - 80 - 60 = 228 star remainder
	if len(g.colWidths) != 3 {
		t.Fatalf("colWidths = %v, want len 3", g.colWidths)
	}
	for i, w := range want {
		if g.colWidths[i] != w {
			t.Fatalf("colWidths[%d] = %v, want %v", i, g.colWidths[i], w)
		}
	}
	wantOffsets := []float32{0, 80, 308}
	for i, o := range wantOffsets {
		if g.colOffsets[i] != o {
			t.Fatalf("colOffsets[%d] = %v, want %v", i, g.colOffsets[i], o)
		}
	}

	headerH := defaultRowHeight(face, theme.Active())
	numCols := 3
	// pool[0] is row0/col0.
	b := g.pool[0].Bounds()
	if b.X != 0 || b.Y != headerH || b.W != 80 {
		t.Fatalf("pool[0].Bounds() = %v, want X=0 Y=%v W=80", b, headerH)
	}
	// pool[2] is row0/col2 (Age, Px 60), at the third column offset.
	b2 := g.pool[2].Bounds()
	if b2.X != 308 || b2.W != 60 {
		t.Fatalf("pool[2].Bounds() = %v, want X=308 W=60", b2)
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
	// 48 also makes the header 48 tall: body viewport H = 148-48 = 100 ->
	// rows 0,1,2(partial) visible = 3.
	layoutDataGrid(g, 0, 0, 100, 148)

	if got, want := len(g.pool), 3*2; got != want {
		t.Fatalf("pool size = %d, want %d (3 visible rows x 2 columns)", got, want)
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

	layoutDataGrid(g, 0, 0, 100, 148) // header 48 + body viewport 100 -> rows 0,1,2 visible x 2 cols

	want := []string{"r0c0", "r0c1", "r1c0", "r1c1", "r2c0", "r2c1"}
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

	layoutDataGrid(g, 0, 0, 100, 148) // header 48 + body viewport 100 -> rows 0,1,2 visible

	if len(g.pool) != 6 {
		t.Fatalf("pool size = %d, want 6", len(g.pool))
	}
	before := make([]*TextBlock, len(g.pool))
	copy(before, g.pool)

	g.rawOffset = 2 * 48 // scroll exactly 2 rows: visible rows become 2,3,4
	layoutDataGrid(g, 0, 0, 100, 148)

	if len(g.pool) != 6 {
		t.Fatalf("pool size after scroll = %d, want 6 (unchanged visible count)", len(g.pool))
	}
	for i := range before {
		if g.pool[i] != before[i] {
			t.Fatalf("pool[%d] pointer changed after scroll (want SAME *TextBlock, re-texted)", i)
		}
	}
	if got, want := g.pool[0].Text(), "r2c0"; got != want {
		t.Fatalf("pool[0].Text() after scroll = %q, want %q", got, want)
	}
	if got, want := g.pool[5].Text(), "r4c1"; got != want {
		t.Fatalf("pool[5].Text() after scroll = %q, want %q", got, want)
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
	layoutDataGrid(g, 0, 0, 100, 148) // header 48 + body viewport H=100, rows 0..2 visible, offset 0

	if got := g.offset; got != 0 {
		t.Fatalf("initial offset = %v, want 0", got)
	}

	g.SetSelectedIndex(19) // last row, off-screen while scrolled to top

	layoutDataGrid(g, 0, 0, 100, 148) // re-layout applies the pending rawOffset

	if got := g.SelectedIndex(); got != 19 {
		t.Fatalf("SelectedIndex() = %d, want 19", got)
	}
	wantOffset := float32(20)*48 - 100 // last row's bottom edge minus viewport H
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
	if firstHeaderY != 20 {
		t.Fatalf("header.Y = %v, want 20 (bounds.Y)", firstHeaderY)
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
