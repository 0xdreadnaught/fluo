package controls

import (
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
	"github.com/0xdreadnaught/fluo/theme"
)

// fakeListItems is a minimal ListItems test double: a plain string slice
// with a granular OnChange channel, standing in for *bind.List[string] so
// these tests don't need to depend on the bind package — bind itself
// depends on controls (see ListChange's doc comment in listview.go for why
// ListItems is an interface at all), so a controls-package test importing
// bind would risk exactly the cycle that interface exists to avoid.
type fakeListItems struct {
	items  []string
	subs   map[int]func(ListChange)
	nextID int
}

func newFakeListItems(items ...string) *fakeListItems {
	return &fakeListItems{items: append([]string(nil), items...)}
}

func (f *fakeListItems) Len() int        { return len(f.items) }
func (f *fakeListItems) At(i int) string { return f.items[i] }

func (f *fakeListItems) OnChange(fn func(ListChange)) (cancel func()) {
	if f.subs == nil {
		f.subs = make(map[int]func(ListChange))
	}
	id := f.nextID
	f.nextID++
	f.subs[id] = fn
	return func() { delete(f.subs, id) }
}

func (f *fakeListItems) Add(s string) {
	f.items = append(f.items, s)
	idx := len(f.items) - 1
	for _, fn := range f.subs {
		fn(ListChange{Kind: ListChangeAdd, Index: idx})
	}
}

// testFace returns a real Face (goregular @ 14px) for tests that need
// actual row-height math; nil-face tests construct ListView with nil
// directly.
func testFace(t *testing.T) *text.Face {
	t.Helper()
	f, err := text.Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	return text.NewFace(f, 14)
}

// layoutListView measures then arranges l at the given bounds, the pattern
// every test below shares (mirrors scrollviewer_test.go's
// layoutScrollViewer).
func layoutListView(l *ListView, x, y, w, h float32) {
	core.MeasureWidget(l, render.Size{W: w, H: h})
	core.ArrangeWidget(l, render.Rect{X: x, Y: y, W: w, H: h})
}

// --- RowHeight ---

func TestListViewDefaultRowHeight(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	face := testFace(t)
	items := newFakeListItems("a", "b")
	l := NewListView(face, items)

	want := face.LineHeight() + 2*theme.Light().Metric.PaddingS
	if got := l.RowHeight(); got != want {
		t.Fatalf("RowHeight() = %v, want %v", got, want)
	}
}

func TestListViewSetRowHeightOverride(t *testing.T) {
	items := newFakeListItems("a", "b")
	l := NewListView(testFace(t), items)

	l.SetRowHeight(30)
	if got := l.RowHeight(); got != 30 {
		t.Fatalf("RowHeight() after SetRowHeight(30) = %v, want 30", got)
	}
}

// --- Visible-range math (via ArrangeContent + pool inspection) ---

func TestListViewVisibleRangeAtOffsetZero(t *testing.T) {
	items := newFakeListItems("0", "1", "2", "3", "4", "5", "6", "7", "8", "9")
	l := NewListView(nil, items).SetRowHeight(48)

	// viewport H = 100 (no gutter contribution matters here beyond width);
	// rows 0..2 fully fit (144 > 100), row 2 partially visible -> 3 rows.
	layoutListView(l, 0, 0, 100, 100)

	if got := len(l.pool); got != 3 {
		t.Fatalf("pool size = %d, want 3 (rows 0,1,2 with row 2 partial)", got)
	}
	for i, tb := range l.pool {
		if got, want := tb.Text(), items.items[i]; got != want {
			t.Fatalf("pool[%d].Text() = %q, want %q", i, got, want)
		}
	}
}

func TestListViewVisibleRangeScrolledShiftsFirstIndex(t *testing.T) {
	items := newFakeListItems("0", "1", "2", "3", "4", "5", "6", "7", "8", "9")
	l := NewListView(nil, items).SetRowHeight(48)
	l.rawOffset = 2 * 48 // scroll by exactly 2 rows
	layoutListView(l, 0, 0, 100, 100)

	if got, want := l.pool[0].Text(), "2"; got != want {
		t.Fatalf("pool[0].Text() = %q, want %q (offset 2*rowH shifts first index to 2)", got, want)
	}
}

func TestListViewShortListShowsAllRows(t *testing.T) {
	items := newFakeListItems("a", "b")
	l := NewListView(nil, items).SetRowHeight(48)
	layoutListView(l, 0, 0, 100, 500) // viewport much taller than content

	if got := len(l.pool); got != 2 {
		t.Fatalf("pool size = %d, want 2 (all rows, short list)", got)
	}
}

func TestListViewEmptyListShowsNoRows(t *testing.T) {
	items := newFakeListItems()
	l := NewListView(nil, items).SetRowHeight(48)
	layoutListView(l, 0, 0, 100, 100)

	if got := len(l.pool); got != 0 {
		t.Fatalf("pool size = %d, want 0 (empty list)", got)
	}
}

func TestListViewPartialRowAtTopEdgeIncluded(t *testing.T) {
	items := newFakeListItems("0", "1", "2", "3", "4")
	l := NewListView(nil, items).SetRowHeight(48)
	l.rawOffset = 10 // not a multiple of rowH: row 0 is 10px scrolled past, still visible
	layoutListView(l, 0, 0, 100, 90)

	if got, want := l.pool[0].Text(), "0"; got != want {
		t.Fatalf("pool[0].Text() = %q, want %q (partially-scrolled row 0 still included)", got, want)
	}
}

// --- Pool reuse / pointer identity ---

func TestListViewPoolReusesTextBlocksAcrossScroll(t *testing.T) {
	items := newFakeListItems("0", "1", "2", "3", "4", "5", "6", "7", "8", "9")
	l := NewListView(nil, items).SetRowHeight(48)
	layoutListView(l, 0, 0, 100, 100) // 3 visible rows: 0,1,2

	if len(l.pool) != 3 {
		t.Fatalf("pool size = %d, want 3", len(l.pool))
	}
	before := make([]*TextBlock, len(l.pool))
	copy(before, l.pool)

	// Scroll by exactly 2 rows: visible count stays 3 (rows 2,3,4).
	l.rawOffset = 2 * 48
	layoutListView(l, 0, 0, 100, 100)

	if len(l.pool) != 3 {
		t.Fatalf("pool size after scroll = %d, want 3 (unchanged visible count)", len(l.pool))
	}
	for i := range before {
		if l.pool[i] != before[i] {
			t.Fatalf("pool[%d] pointer changed after scroll (want SAME *TextBlock, re-texted)", i)
		}
	}
	if got, want := l.pool[0].Text(), "2"; got != want {
		t.Fatalf("pool[0].Text() after scroll = %q, want %q (re-texted in place)", got, want)
	}
	if got, want := l.pool[2].Text(), "4"; got != want {
		t.Fatalf("pool[2].Text() after scroll = %q, want %q (re-texted in place)", got, want)
	}
}

// --- Total height / thumb geometry ---

func TestListViewTotalHeightIsCountTimesRowH(t *testing.T) {
	items := newFakeListItems("0", "1", "2", "3", "4")
	l := NewListView(nil, items).SetRowHeight(48)
	layoutListView(l, 0, 0, 100, 100)

	want := float32(5 * 48)
	if got := l.totalHeight(); got != want {
		t.Fatalf("totalHeight() = %v, want %v", got, want)
	}
}

func TestListViewThumbGeometryMatchesScrollViewerConventions(t *testing.T) {
	// 20 rows * 48 = 960 content height, viewport 100 -> definitely scrollable.
	items := newFakeListItems(make([]string, 20)...)
	l := NewListView(nil, items).SetRowHeight(48)
	layoutListView(l, 10, 20, 100, 100)

	track, thumbH, ok := l.thumbGeometry()
	if !ok {
		t.Fatal("expected a thumb (content 960 > viewport 100)")
	}
	// Track sits in the right gutter strip, full viewport height.
	if track.W != l.gutter {
		t.Fatalf("track.W = %v, want gutter %v", track.W, l.gutter)
	}
	if track.H != 100 {
		t.Fatalf("track.H = %v, want 100 (viewport height)", track.H)
	}
	// thumbH = track.H^2/total = 100*100/960 ~= 10.4, clamped up to the
	// shared 24px minimum (scrollThumbMinH), matching ScrollViewer.
	if thumbH != scrollThumbMinH {
		t.Fatalf("thumbH = %v, want %v (scrollThumbMinH floor)", thumbH, scrollThumbMinH)
	}
}

func TestListViewNoThumbWhenContentFitsViewport(t *testing.T) {
	items := newFakeListItems("a", "b")
	l := NewListView(nil, items).SetRowHeight(48)
	layoutListView(l, 0, 0, 100, 500)

	if _, ok := l.thumbRect(); ok {
		t.Fatal("expected no thumb: content (96) fits entirely within viewport (500)")
	}
}

// --- Wheel ---

func TestListViewWheelScrollsAndHandles(t *testing.T) {
	items := newFakeListItems(make([]string, 20)...)
	l := NewListView(nil, items).SetRowHeight(48)
	layoutListView(l, 0, 0, 100, 100)
	if got := l.offset; got != 0 {
		t.Fatalf("initial offset = %v, want 0", got)
	}

	r := input.NewRouter()
	e := &input.PointerEvent{Action: input.Wheel, Delta: render.Point{Y: -2}, Router: r}
	l.OnPointer(e)
	if !e.Handled {
		t.Fatal("wheel event not marked Handled")
	}

	layoutListView(l, 0, 0, 100, 100)
	// -Delta.Y * 48 = -(-2)*48 = 96.
	if got := l.offset; got != 96 {
		t.Fatalf("offset after wheel = %v, want 96", got)
	}
}

// --- List mutation via granular channel ---

func TestListViewAddReflectsAfterRelayout(t *testing.T) {
	items := newFakeListItems("a", "b")
	l := NewListView(nil, items).SetRowHeight(48)
	layoutListView(l, 0, 0, 100, 500)

	if got := len(l.pool); got != 2 {
		t.Fatalf("pool size before Add = %d, want 2", got)
	}

	items.Add("c")
	if !l.NeedsLayout() {
		t.Fatal("Add via granular OnChange did not invalidate ListView")
	}

	layoutListView(l, 0, 0, 100, 500)
	if got := len(l.pool); got != 3 {
		t.Fatalf("pool size after Add + re-layout = %d, want 3", got)
	}
	if got, want := l.pool[2].Text(), "c"; got != want {
		t.Fatalf("pool[2].Text() = %q, want %q", got, want)
	}
}

// --- Selection: click ---

func TestListViewClickRowSelectsAndFiresOnChanged(t *testing.T) {
	items := newFakeListItems("0", "1", "2", "3", "4")
	l := NewListView(nil, items).SetRowHeight(48)
	layoutListView(l, 0, 0, 100, 100) // rows 0,1,2 visible (2 partial)

	var got []int
	l.OnChanged(func(v int) { got = append(got, v) })

	r := input.NewRouter()
	e := &input.PointerEvent{Action: input.Press, Pos: render.Point{X: 10, Y: 60}, Router: r}
	l.OnPointer(e)

	if !e.Handled {
		t.Fatal("press on a real row not marked Handled")
	}
	if got, want := l.SelectedIndex(), 1; got != want {
		t.Fatalf("SelectedIndex() = %d, want %d (y=60 falls in row 1, rowH 48)", got, want)
	}
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("OnChanged calls = %v, want [1]", got)
	}
}

func TestListViewClickSameRowTwiceFiresOnChangedOnce(t *testing.T) {
	items := newFakeListItems("0", "1", "2")
	l := NewListView(nil, items).SetRowHeight(48)
	layoutListView(l, 0, 0, 100, 500)

	fires := 0
	l.OnChanged(func(int) { fires++ })

	r := input.NewRouter()
	press := func() {
		e := &input.PointerEvent{Action: input.Press, Pos: render.Point{X: 10, Y: 10}, Router: r}
		l.OnPointer(e)
	}
	press()
	press()

	if l.SelectedIndex() != 0 {
		t.Fatalf("SelectedIndex() = %d, want 0", l.SelectedIndex())
	}
	if fires != 1 {
		t.Fatalf("OnChanged fired %d times for two clicks on the SAME row, want 1 (only real changes notify)", fires)
	}
}

func TestListViewClickGutterDoesNotSelect(t *testing.T) {
	items := newFakeListItems("0", "1", "2")
	l := NewListView(nil, items).SetRowHeight(48)
	layoutListView(l, 0, 0, 100, 100) // 3*48=144 content > 100 viewport: thumb present

	r := input.NewRouter()
	// x inside the gutter strip, y below the thumb's own span (thumbH well
	// under 100 - see TestListViewThumbGeometryMatchesScrollViewerConventions
	// for the shared thumb-geometry math) so this exercises a genuine
	// "click past the thumb, still in the gutter" case, not a thumb-drag.
	e := &input.PointerEvent{Action: input.Press, Pos: render.Point{X: 100 - l.gutter/2, Y: 95}, Router: r}
	l.OnPointer(e)

	if e.Handled {
		t.Fatal("gutter click (past the thumb) marked Handled, want false")
	}
	if l.SelectedIndex() != -1 {
		t.Fatalf("SelectedIndex() = %d after gutter click, want -1 (unchanged)", l.SelectedIndex())
	}
}

func TestListViewClickEmptySpaceBelowShortListDoesNotSelect(t *testing.T) {
	items := newFakeListItems("0", "1") // 2*48=96 content, well under the viewport
	l := NewListView(nil, items).SetRowHeight(48)
	layoutListView(l, 0, 0, 100, 500)

	r := input.NewRouter()
	e := &input.PointerEvent{Action: input.Press, Pos: render.Point{X: 10, Y: 200}, Router: r} // past both rows
	l.OnPointer(e)

	if e.Handled {
		t.Fatal("click on empty space below a short list marked Handled, want false")
	}
	if l.SelectedIndex() != -1 {
		t.Fatalf("SelectedIndex() = %d after empty-space click, want -1 (unchanged)", l.SelectedIndex())
	}
}

// --- Selection: SetSelectedIndex (silent, clamped) ---

func TestListViewSetSelectedIndexIsSilentAndClamped(t *testing.T) {
	items := newFakeListItems("0", "1", "2")
	l := NewListView(nil, items).SetRowHeight(48)

	fired := false
	l.OnChanged(func(int) { fired = true })

	l.SetSelectedIndex(1)
	if got := l.SelectedIndex(); got != 1 {
		t.Fatalf("SelectedIndex() = %d, want 1", got)
	}

	l.SetSelectedIndex(99) // clamps to n-1
	if got := l.SelectedIndex(); got != 2 {
		t.Fatalf("SelectedIndex() = %d after over-range SetSelectedIndex(99), want 2 (clamped to n-1)", got)
	}

	l.SetSelectedIndex(-5) // -1 is an explicit, always-valid "no selection"
	if got := l.SelectedIndex(); got != -1 {
		t.Fatalf("SelectedIndex() = %d after under-range SetSelectedIndex(-5), want -1", got)
	}

	if fired {
		t.Fatal("SetSelectedIndex fired OnChanged, want fully silent (programmatic setter)")
	}
}

// --- Selection: keyboard (focused) ---

func TestListViewKeyboardUpDownHomeEndWhenFocused(t *testing.T) {
	items := newFakeListItems("0", "1", "2", "3", "4")
	l := NewListView(nil, items).SetRowHeight(48)
	layoutListView(l, 0, 0, 100, 500) // all 5 rows visible, nothing to auto-scroll
	l.OnFocusChanged(true)

	var got []int
	l.OnChanged(func(v int) { got = append(got, v) })

	press := func(k input.Key) {
		e := &input.KeyEvent{Action: input.Press, Key: k}
		l.OnKey(e)
		if !e.Handled {
			t.Fatalf("key %v not marked Handled", k)
		}
	}

	press(input.KeyDown) // no prior selection -> lands on row 0
	if got := l.SelectedIndex(); got != 0 {
		t.Fatalf("SelectedIndex() after first Down = %d, want 0", got)
	}
	press(input.KeyDown)
	if got := l.SelectedIndex(); got != 1 {
		t.Fatalf("SelectedIndex() after second Down = %d, want 1", got)
	}
	press(input.KeyUp)
	if got := l.SelectedIndex(); got != 0 {
		t.Fatalf("SelectedIndex() after Up = %d, want 0", got)
	}
	press(input.KeyUp) // clamped: stays at 0, does not go to -1
	if got := l.SelectedIndex(); got != 0 {
		t.Fatalf("SelectedIndex() after Up at row 0 = %d, want clamped 0 (never -1)", got)
	}
	press(input.KeyEnd)
	if got := l.SelectedIndex(); got != 4 {
		t.Fatalf("SelectedIndex() after End = %d, want 4 (last row)", got)
	}
	press(input.KeyHome)
	if got := l.SelectedIndex(); got != 0 {
		t.Fatalf("SelectedIndex() after Home = %d, want 0", got)
	}

	want := []int{0, 1, 0, 4, 0}
	if len(got) != len(want) {
		t.Fatalf("OnChanged calls = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("OnChanged calls = %v, want %v", got, want)
		}
	}
}

func TestListViewSelectLastRowAutoScrollsIntoView(t *testing.T) {
	items := newFakeListItems(make([]string, 20)...) // 20 rows: 20*48=960 content
	l := NewListView(nil, items).SetRowHeight(48)
	layoutListView(l, 0, 0, 100, 100) // viewport 100: rows 0..2 visible, offset 0

	if got := l.offset; got != 0 {
		t.Fatalf("initial offset = %v, want 0", got)
	}

	l.OnFocusChanged(true)
	e := &input.KeyEvent{Action: input.Press, Key: input.KeyEnd}
	l.OnKey(e) // selects the last row (19), off-screen while scrolled to top

	layoutListView(l, 0, 0, 100, 100) // re-layout applies the pending rawOffset

	if got := l.SelectedIndex(); got != 19 {
		t.Fatalf("SelectedIndex() = %d, want 19", got)
	}
	wantOffset := float32(20)*48 - 100 // last row's bottom edge minus viewport H
	if got := l.offset; got != wantOffset {
		t.Fatalf("offset after auto-scroll = %v, want %v (last row fully visible)", got, wantOffset)
	}
}

// --- Selection: pool re-color ---

func TestListViewPoolRecolorsSelectedRow(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	items := newFakeListItems("0", "1", "2")
	l := NewListView(nil, items).SetRowHeight(48)
	l.SetSelectedIndex(1)
	layoutListView(l, 0, 0, 100, 500) // all rows realized

	if len(l.pool) != 3 {
		t.Fatalf("pool size = %d, want 3", len(l.pool))
	}
	for i, tb := range l.pool {
		want := th.Color.TextPrimary
		if i == 1 {
			want = th.Color.SelectionForeground
		}
		if got := tb.Color(); got != want {
			t.Fatalf("pool[%d].Color() = %v, want %v (selected == %d)", i, got, want, l.SelectedIndex())
		}
	}
}

func TestListViewPoolRecolorsAfterSelectionChangesWithoutRetext(t *testing.T) {
	// Guards ArrangeContent's unconditional-recolor requirement: a selection
	// change alone (no text change) must still repaint the right rows on the
	// NEXT arrange pass, even though re-texting itself stays conditional.
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	items := newFakeListItems("0", "1", "2")
	l := NewListView(nil, items).SetRowHeight(48)
	layoutListView(l, 0, 0, 100, 500)

	l.SetSelectedIndex(2)
	layoutListView(l, 0, 0, 100, 500)

	if got, want := l.pool[2].Color(), th.Color.SelectionForeground; got != want {
		t.Fatalf("pool[2].Color() = %v, want %v", got, want)
	}
	if got, want := l.pool[0].Color(), th.Color.TextPrimary; got != want {
		t.Fatalf("pool[0].Color() = %v, want %v", got, want)
	}
}

// --- Dispose ---

func TestListViewDisposeStopsInvalidation(t *testing.T) {
	items := newFakeListItems("a", "b")
	l := NewListView(nil, items).SetRowHeight(48)
	layoutListView(l, 0, 0, 100, 500) // clean

	if l.NeedsLayout() {
		t.Fatal("expected clean layout state before Dispose")
	}

	l.Dispose()
	items.Add("c") // post-Dispose mutation must NOT invalidate

	if l.NeedsLayout() {
		t.Fatal("post-Dispose Add invalidated the ListView; Dispose should have released the subscription")
	}
}

func TestListViewDisposeIsIdempotent(t *testing.T) {
	items := newFakeListItems("a")
	l := NewListView(nil, items).SetRowHeight(48)

	l.Dispose()
	l.Dispose() // must not panic
}
