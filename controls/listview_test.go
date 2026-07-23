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
	theme.SetActive(theme.FluentLight())
	defer theme.SetActive(nil)

	face := testFace(t)
	items := newFakeListItems("a", "b")
	l := NewListView(face, items)

	want := face.LineHeight() + 2*theme.FluentLight().Metric.PaddingS
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
