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

	// atCalls counts At calls, so the content-width cache tests can tell a
	// real measure pass (which reads every item) from a cached one (which
	// reads none). Face.Measure itself can't be counted — text.Face is a
	// concrete type, not an interface — but contentWidth reads exactly one
	// item per Measure, so this counts the same thing.
	atCalls int
}

func newFakeListItems(items ...string) *fakeListItems {
	return &fakeListItems{items: append([]string(nil), items...)}
}

func (f *fakeListItems) Len() int { return len(f.items) }

func (f *fakeListItems) At(i int) string {
	f.atCalls++
	return f.items[i]
}

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

func (f *fakeListItems) RemoveAt(i int) {
	f.items = append(f.items[:i], f.items[i+1:]...)
	for _, fn := range f.subs {
		fn(ListChange{Kind: ListChangeRemove, Index: i})
	}
}

func (f *fakeListItems) Set(i int, s string) {
	f.items[i] = s
	for _, fn := range f.subs {
		fn(ListChange{Kind: ListChangeReplace, Index: i})
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

	// viewport H = 96 (100 - 2*BevelWidth: ArrangeContent insets bounds into
	// the sunken well before computing the viewport — see contentBounds);
	// rows 0,1 fully fit exactly (96/48 == 2), no partial row -> 2 rows.
	layoutListView(l, 0, 0, 100, 100)

	if got := len(l.pool); got != 2 {
		t.Fatalf("pool size = %d, want 2 (rows 0,1; viewport 96 lands exactly on the rowH=48 boundary)", got)
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
	// 2 visible rows: 0,1 (viewport H = 96 = 100 - 2*BevelWidth, landing
	// exactly on the rowH=48 boundary — see contentBounds).
	layoutListView(l, 0, 0, 100, 100)

	if len(l.pool) != 2 {
		t.Fatalf("pool size = %d, want 2", len(l.pool))
	}
	before := make([]*TextBlock, len(l.pool))
	copy(before, l.pool)

	// Scroll by exactly 2 rows: visible count stays 2 (rows 2,3).
	l.rawOffset = 2 * 48
	layoutListView(l, 0, 0, 100, 100)

	if len(l.pool) != 2 {
		t.Fatalf("pool size after scroll = %d, want 2 (unchanged visible count)", len(l.pool))
	}
	for i := range before {
		if l.pool[i] != before[i] {
			t.Fatalf("pool[%d] pointer changed after scroll (want SAME *TextBlock, re-texted)", i)
		}
	}
	if got, want := l.pool[0].Text(), "2"; got != want {
		t.Fatalf("pool[0].Text() after scroll = %q, want %q (re-texted in place)", got, want)
	}
	if got, want := l.pool[1].Text(), "3"; got != want {
		t.Fatalf("pool[1].Text() after scroll = %q, want %q (re-texted in place)", got, want)
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
	// track.H is the viewport height: 96, not the raw arranged 100 — the 2px
	// bevel inset (ArrangeContent, see contentBounds) shrinks it on top+bottom.
	if track.H != 96 {
		t.Fatalf("track.H = %v, want 96 (viewport height, 100 - 2*BevelWidth)", track.H)
	}
	// thumbH = track.H^2/total = 96*96/960 ~= 9.6, clamped up to the
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

// TestListViewOverscrollLeavesNoDeadZone pins the end-stop dead zone on the
// vertical axis: a scroll request past the end must not stay stranded in the
// raw accumulator, or every notch back has to burn off the overshoot before
// the clamped offset moves at all.
func TestListViewOverscrollLeavesNoDeadZone(t *testing.T) {
	items := newFakeListItems(make([]string, 20)...)
	l := NewListView(nil, items).SetRowHeight(48)
	l.rawOffset = 10000 // far past the end
	layoutListView(l, 0, 0, 100, 100)

	maxY := float32(20*48) - l.viewport.H
	if got := l.offset; got != maxY {
		t.Fatalf("offset after over-scrolling = %v, want the %v end stop", got, maxY)
	}
	if l.rawOffset != l.offset {
		t.Fatalf("rawOffset = %v after layout clamped offset to %v — the accumulator kept the overshoot", l.rawOffset, l.offset)
	}

	up := &input.PointerEvent{Action: input.Wheel, Delta: render.Point{Y: 1}, Router: input.NewRouter()}
	l.OnPointer(up)
	if !up.Handled {
		t.Fatal("a notch back off the end stop went unhandled")
	}
	layoutListView(l, 0, 0, 100, 100)
	if got := l.offset; got != maxY-scrollWheelStep {
		t.Fatalf("offset one notch back from the end stop = %v, want %v", got, maxY-scrollWheelStep)
	}
}

// TestListViewWheelBubblesToOuterScrollerWhenRowsFit pins the nested-scroller
// dead zone. A ListView whose rows already fit its own viewport scrolls
// nothing, so it has to leave the notch unhandled — input.Bubble stops at the
// first handler that sets Handled, so consuming it regardless (marking every
// wheel handled, as this did) turned a ScrollViewer wrapping a short list
// into a region where the wheel moved neither one.
func TestListViewWheelBubblesToOuterScrollerWhenRowsFit(t *testing.T) {
	// 2 rows of 48 = 96px of content inside the ListView's own 240px default
	// desired height (what the viewer arranges it at, measuring it with
	// unbounded height), inside a 50px-tall viewer: the list has nothing to
	// scroll, the viewer has 190px of it.
	items := newFakeListItems("a", "b")
	l := NewListView(nil, items).SetRowHeight(48)
	s := NewScrollViewer().SetChild(l)
	layoutScrollViewer(s, 0, 0, 100, 50)

	if _, ok := l.thumbRect(); ok {
		t.Fatal("fixture: the inner list must fit its own viewport (no thumb)")
	}
	if !s.canScrollY() {
		t.Fatal("fixture: the outer viewer must have room to scroll")
	}

	e := &input.PointerEvent{Action: input.Wheel, Delta: render.Point{Y: -1}, Router: input.NewRouter()}
	input.Bubble([]core.Widget{s, l}, e)
	if !e.Handled {
		t.Fatal("wheel over ScrollViewer{ListView} went unhandled by both")
	}

	layoutScrollViewer(s, 0, 0, 100, 50)
	if got := l.offset; got != 0 {
		t.Fatalf("inner list offset = %v, want 0 (nothing to scroll)", got)
	}
	if got := s.OffsetY(); got != scrollWheelStep {
		t.Fatalf("outer viewer offset = %v, want %v — the inner list swallowed a notch it could not act on", got, scrollWheelStep)
	}
}

// TestListViewWheelAtEndStopNotHandled is the same rule at the other end of
// the range: a list scrolled to its last row must not keep consuming
// downward notches, while a notch back up is still its own.
func TestListViewWheelAtEndStopNotHandled(t *testing.T) {
	items := newFakeListItems(make([]string, 4)...)
	l := NewListView(nil, items).SetRowHeight(48)
	l.rawOffset = 4*48 - 96 // exactly the end stop: content 192, viewport 96
	layoutListView(l, 0, 0, 100, 100)
	if got := l.offset; got != 96 {
		t.Fatalf("fixture: offset = %v, want the 96 end stop", got)
	}

	down := &input.PointerEvent{Action: input.Wheel, Delta: render.Point{Y: -1}, Router: input.NewRouter()}
	l.OnPointer(down)
	if down.Handled {
		t.Fatal("a downward notch at the bottom end stop was consumed, scrolling nothing")
	}

	up := &input.PointerEvent{Action: input.Wheel, Delta: render.Point{Y: 1}, Router: input.NewRouter()}
	l.OnPointer(up)
	if !up.Handled {
		t.Fatal("a notch back up from the end stop must still be handled")
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
	// viewport 96 (100 - 2*BevelWidth, see contentBounds): rows 0,1 visible, offset 0.
	layoutListView(l, 0, 0, 100, 100)

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
	wantOffset := float32(20)*48 - 96 // last row's bottom edge minus viewport H (96)
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
		want := th.Color.WindowText
		if i == 1 {
			want = th.Color.HighlightText
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

	if got, want := l.pool[2].Color(), th.Color.HighlightText; got != want {
		t.Fatalf("pool[2].Color() = %v, want %v", got, want)
	}
	if got, want := l.pool[0].Color(), th.Color.WindowText; got != want {
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

// TestListViewSelectionBandStaysInBounds pins that the selection band for a
// partially-visible row is cropped. Render draws the band before ClipRect is
// pushed (core.RenderWidget runs a widget's own Render first), so nothing
// crops it for us: with a row height that doesn't divide the viewport, the
// bottom row is partly visible and its full-height band used to paint out
// past the well and over whatever sits below the control.
func TestListViewSelectionBandStaysInBounds(t *testing.T) {
	// viewport {2,2,84,96}: 96 is not a multiple of the 30px rows, so a row
	// hangs off an edge at most offsets. The offsets below are set directly
	// rather than through SetSelectedIndex, whose scroll-into-view would
	// align the selected row flush with an edge and hide the bug.
	cases := []struct {
		name     string
		selected int
		offset   float32
	}{
		{"partial bottom row", 3, 14}, // band 78..108, viewport ends at 98
		{"partial top row", 1, 40},    // band -8..22, viewport starts at 2
	}

	bounds := render.Rect{X: 0, Y: 0, W: 100, H: 100}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items := newFakeListItems(make([]string, 10)...)
			l := NewListView(nil, items).SetRowHeight(30)
			l.selected = tc.selected
			l.rawOffset = tc.offset
			layoutListView(l, bounds.X, bounds.Y, bounds.W, bounds.H)

			if l.selected < l.visibleFirst || l.selected >= l.visibleFirst+len(l.pool) {
				t.Fatalf("fixture: row %d is not realized, so no band is drawn at all", l.selected)
			}

			rr := &recordRenderer{}
			l.Render(rr)

			if len(rr.fills) == 0 {
				t.Fatal("ListView.Render emitted no fills at all")
			}
			var band bool
			for _, f := range rr.fills {
				if f.color == l.colors.Highlight {
					band = true
				}
				if f.rect.Bottom() > bounds.Bottom() {
					t.Fatalf("Render emitted %v, reaching %v — past the control's bottom edge at %v",
						f.rect, f.rect.Bottom(), bounds.Bottom())
				}
				if f.rect.Y < bounds.Y {
					t.Fatalf("Render emitted %v, starting at %v — above the control's top edge at %v",
						f.rect, f.rect.Y, bounds.Y)
				}
			}
			if !band {
				t.Fatal("fixture: no selection band was drawn, so nothing was actually checked")
			}
		})
	}
}

// --- Horizontal scroll (control-variants Task 4) ---

// wideText is far wider, at typical face sizes, than any viewport used by
// the tests below — chosen to reliably trigger horizontal overflow without
// depending on exact face metrics.
const wideText = "This is a very long row of text that is far wider than the viewport width used in these tests"

func TestListViewNilFaceNeverShowsHorizontalThumb(t *testing.T) {
	// A nil face can't measure text (contentWidth() reports 0, see its doc
	// comment), so this must behave byte-identically to before Task 4:
	// no horizontal thumb, ever, regardless of row content.
	items := newFakeListItems(wideText, "b", "c")
	l := NewListView(nil, items).SetRowHeight(48)
	layoutListView(l, 0, 0, 100, 100)

	if _, ok := l.thumbRectX(); ok {
		t.Fatal("expected no horizontal thumb for a nil-face ListView")
	}
	if l.offsetX != 0 {
		t.Fatalf("offsetX = %v, want 0 (nothing to scroll horizontally)", l.offsetX)
	}
}

func TestListViewShortTextNeverShowsHorizontalThumb(t *testing.T) {
	// A real face but short row text (well under the viewport width) is the
	// existing listview.png golden's own shape — must not spuriously grow a
	// horizontal thumb.
	face := testFace(t)
	items := newFakeListItems("Item 01", "Item 02", "Item 03")
	l := NewListView(face, items).SetRowHeight(48)
	layoutListView(l, 0, 0, 200, 100)

	if _, ok := l.thumbRectX(); ok {
		t.Fatal("expected no horizontal thumb: short row text fits well within the viewport")
	}
}

func TestListViewHorizontalThumbShowsForWideRowText(t *testing.T) {
	face := testFace(t)
	items := newFakeListItems(wideText, "short")
	l := NewListView(face, items).SetRowHeight(48)
	layoutListView(l, 0, 0, 100, 100)

	if _, ok := l.thumbRectX(); !ok {
		t.Fatal("expected a horizontal thumb: row text is far wider than the 100px viewport")
	}
}

// TestListViewVerticalThumbStaysInBoundsWithBottomGutter pins the gutter
// ordering. The rows fit the inset height on their own (2*48 = 96 <= 100),
// so no right gutter used to be reserved — but the wide row text reserves a
// bottom gutter afterwards, and against that reduced height the rows no
// longer fit, so the virtualizer reports a vertical thumb anyway. With no
// right gutter to draw it in, its track started at the content's right edge
// and ran a full gutter's width past the control, over whatever sits beside
// it and unhittable.
func TestListViewVerticalThumbStaysInBoundsWithBottomGutter(t *testing.T) {
	face := testFace(t)
	items := newFakeListItems(wideText, wideText)
	l := NewListView(face, items).SetRowHeight(48)

	bounds := render.Rect{X: 0, Y: 0, W: 100, H: 104}
	layoutListView(l, bounds.X, bounds.Y, bounds.W, bounds.H)

	if _, ok := l.thumbRectX(); !ok {
		t.Fatal("fixture: the wide rows must reserve a bottom gutter for a horizontal thumb")
	}

	track, _, ok := l.thumbGeometry()
	if !ok {
		return // no vertical thumb reported at all is also a correct answer
	}
	if track.Right() > bounds.Right() {
		t.Fatalf("vertical thumb track %v runs to %v, past the control's right edge at %v",
			track, track.Right(), bounds.Right())
	}
	thumb, _ := l.thumbRect()
	if thumb.Right() > bounds.Right() {
		t.Fatalf("vertical thumb %v runs to %v, past the control's right edge at %v",
			thumb, thumb.Right(), bounds.Right())
	}
}

func TestListViewOffsetXShiftsRowX(t *testing.T) {
	face := testFace(t)
	items := newFakeListItems(wideText)
	l := NewListView(face, items).SetRowHeight(48)
	layoutListView(l, 0, 0, 100, 100)

	beforeX := l.pool[0].Bounds().X

	l.rawOffsetX = 40
	layoutListView(l, 0, 0, 100, 100)
	if l.offsetX != 40 {
		t.Fatalf("offsetX = %v, want 40 (well within the clamped range for this much overflow)", l.offsetX)
	}
	afterX := l.pool[0].Bounds().X

	if got, want := beforeX-afterX, float32(40); got != want {
		t.Fatalf("row X shift = %v, want %v (row X is offset by -offsetX)", got, want)
	}
}

func TestListViewShiftWheelScrollsXPlainWheelScrollsY(t *testing.T) {
	face := testFace(t)
	items := newFakeListItems(wideText, "b", "c", "d", "e", "f", "g", "h")
	l := NewListView(face, items).SetRowHeight(48)
	layoutListView(l, 0, 0, 100, 100)

	r := input.NewRouter()

	// Plain wheel: Y only.
	plain := &input.PointerEvent{Action: input.Wheel, Delta: render.Point{Y: -2}, Router: r}
	l.OnPointer(plain)
	if !plain.Handled {
		t.Fatal("plain wheel not marked Handled")
	}
	layoutListView(l, 0, 0, 100, 100)
	if l.offset == 0 {
		t.Fatal("offset (Y) = 0 after plain wheel, want nonzero (plain wheel scrolls Y)")
	}
	if l.offsetX != 0 {
		t.Fatalf("offsetX = %v after plain wheel, want 0 (plain wheel never scrolls X)", l.offsetX)
	}
	yAfterPlain := l.offset

	// Shift+wheel: X only, Y unchanged.
	shift := &input.PointerEvent{Action: input.Wheel, Delta: render.Point{Y: -2}, Mods: input.ModShift, Router: r}
	l.OnPointer(shift)
	if !shift.Handled {
		t.Fatal("shift+wheel not marked Handled")
	}
	layoutListView(l, 0, 0, 100, 100)
	if l.offset != yAfterPlain {
		t.Fatalf("offset (Y) = %v after shift+wheel, want unchanged %v (shift+wheel scrolls X only)", l.offset, yAfterPlain)
	}
	if l.offsetX == 0 {
		t.Fatal("offsetX = 0 after shift+wheel, want nonzero (shift+wheel scrolls X)")
	}
}

func TestListViewRowSelectionUnaffectedByHorizontalOffset(t *testing.T) {
	// Horizontal scroll must not disturb the (Y-only) row hit-test.
	face := testFace(t)
	items := newFakeListItems(wideText, "1", "2", "3", "4")
	l := NewListView(face, items).SetRowHeight(48)
	layoutListView(l, 0, 0, 100, 100)

	l.rawOffsetX = 30
	layoutListView(l, 0, 0, 100, 100)
	if l.offsetX == 0 {
		t.Fatal("expected a nonzero offsetX for this test to be meaningful")
	}

	r := input.NewRouter()
	pos := render.Point{X: l.viewport.X + 5, Y: l.viewport.Y + 60} // row 1's band
	e := &input.PointerEvent{Action: input.Press, Pos: pos, Router: r}
	l.OnPointer(e)

	if !e.Handled {
		t.Fatal("press on a real row not marked Handled")
	}
	if got, want := l.SelectedIndex(), 1; got != want {
		t.Fatalf("SelectedIndex() = %d, want %d (row hit-test unaffected by horizontal scroll)", got, want)
	}
}

func TestListViewScrollToXAndOffsetX(t *testing.T) {
	face := testFace(t)
	items := newFakeListItems(wideText)
	l := NewListView(face, items).SetRowHeight(48)
	layoutListView(l, 0, 0, 100, 100)

	l.ScrollToX(25)
	if !l.NeedsLayout() {
		t.Fatal("ScrollToX did not invalidate arrange")
	}
	layoutListView(l, 0, 0, 100, 100)
	if got := l.OffsetX(); got != 25 {
		t.Fatalf("OffsetX() = %v, want 25", got)
	}
}

// TestListViewNilItemsBuildsAndLaysOut covers a nil item source. l.count and
// contentWidth both already treated nil as an empty list, but the OnChange
// subscription in the constructor dereferenced it unguarded, so
// NewListView(face, nil) panicked before any of that ran. A nil items must
// now build, measure, arrange and render as a permanently empty list, and
// Dispose (no subscription to release) must stay a no-op.
func TestListViewNilItemsBuildsAndLaysOut(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	l := NewListView(testFace(t), nil)
	layoutListView(l, 0, 0, 120, 80)

	if got := l.count(); got != 0 {
		t.Fatalf("count = %d, want 0 (nil items is an empty list)", got)
	}
	if got := len(l.Children()); got != 0 {
		t.Fatalf("Children = %d, want 0 (no rows to realize)", got)
	}
	l.Dispose()
}

// --- contentWidth memoization ---

// wantContentWidth is the full, uncached max-row-width computation the cache
// must agree with: the widest row's measured text width plus the lpad/rpad
// inset ArrangeContent gives every row.
func wantContentWidth(face *text.Face, items []string, m theme.MetricTokens) float32 {
	var maxW float32
	for _, s := range items {
		if w := face.Measure(s).W; w > maxW {
			maxW = w
		}
	}
	return maxW + 2*m.PaddingS + m.PaddingS
}

func TestListViewContentWidthMatchesFullMeasure(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	face := testFace(t)
	rows := []string{"short", wideText, "middling row"}
	l := NewListView(face, newFakeListItems(rows...))

	want := wantContentWidth(face, rows, theme.Light().Metric)
	if got := l.contentWidth(); got != want {
		t.Fatalf("contentWidth() = %v, want %v (widest row + lpad + rpad)", got, want)
	}
	// Cold cache and warm cache must agree.
	if got := l.contentWidth(); got != want {
		t.Fatalf("second contentWidth() = %v, want %v (cached value must match the measured one)", got, want)
	}
}

func TestListViewContentWidthCachedAfterFirstCall(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	items := newFakeListItems("a", "b", "c", "d")
	l := NewListView(testFace(t), items)

	items.atCalls = 0
	first := l.contentWidth()
	if items.atCalls != items.Len() {
		t.Fatalf("cold contentWidth() read %d items, want %d (a full measure pass)", items.atCalls, items.Len())
	}

	items.atCalls = 0
	if got := l.contentWidth(); got != first {
		t.Fatalf("cached contentWidth() = %v, want %v", got, first)
	}
	if items.atCalls != 0 {
		t.Fatalf("cached contentWidth() read %d items, want 0 (no re-measure without a list change)", items.atCalls)
	}
}

func TestListViewContentWidthRemeasuredAfterListChange(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	face := testFace(t)
	items := newFakeListItems("a", "b")
	l := NewListView(face, items)

	narrow := l.contentWidth() // warms the cache

	// An append grows the memo incrementally: only the newly-added row is
	// measured (one At call), never the whole list, and the cache stays warm.
	items.atCalls = 0
	items.Add(wideText)
	if items.atCalls != 1 {
		t.Fatalf("an append measured %d items, want 1 (only the newly-added row, not a full re-measure)", items.atCalls)
	}
	wide := l.contentWidth()
	if items.atCalls != 1 {
		t.Fatalf("contentWidth() after an incremental append read %d items total, want 1 (the cache must stay warm)", items.atCalls)
	}
	if wide <= narrow {
		t.Fatalf("contentWidth() after adding a wide row = %v, want > %v", wide, narrow)
	}
	if want := wantContentWidth(face, items.items, theme.Light().Metric); wide != want {
		t.Fatalf("contentWidth() after an add = %v, want %v", wide, want)
	}

	// Removing the wide row must shrink it back.
	items.RemoveAt(2)
	if got := l.contentWidth(); got != narrow {
		t.Fatalf("contentWidth() after removing the wide row = %v, want %v (back to the narrow set)", got, narrow)
	}

	// Replacing a row in place (through the change channel) is picked up too.
	items.Set(0, wideText)
	if got := l.contentWidth(); got != wide {
		t.Fatalf("contentWidth() after replacing row 0 with the wide text = %v, want %v", got, wide)
	}
}

// TestListViewContentWidthAppendNarrowerRowKeepsMax proves the incremental
// append path leaves the memo at its existing max when the added row is
// narrower: only the new row is measured (one At call), and the width is
// unchanged.
func TestListViewContentWidthAppendNarrowerRowKeepsMax(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	face := testFace(t)
	items := newFakeListItems("a", wideText, "c")
	l := NewListView(face, items)

	wide := l.contentWidth() // warms the cache; widest row is wideText

	items.atCalls = 0
	items.Add("z") // far narrower than wideText
	if items.atCalls != 1 {
		t.Fatalf("appending a narrow row measured %d items, want 1 (only the added row)", items.atCalls)
	}
	if got := l.contentWidth(); got != wide {
		t.Fatalf("contentWidth() after appending a narrow row = %v, want %v (max unchanged)", got, wide)
	}
	if items.atCalls != 1 {
		t.Fatalf("contentWidth() after a narrow append read %d items total, want 1 (cache stayed warm)", items.atCalls)
	}
}

// TestListViewContentWidthAppendColdCacheStaysCold proves an append against a
// never-measured (cold) memo does not eagerly measure anything: the memo stays
// cold and the next contentWidth() does the one full measure.
func TestListViewContentWidthAppendColdCacheStaysCold(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	face := testFace(t)
	items := newFakeListItems("a", "b")
	l := NewListView(face, items)

	// Never call contentWidth(): the memo is cold.
	items.atCalls = 0
	items.Add(wideText)
	if items.atCalls != 0 {
		t.Fatalf("append against a cold cache measured %d items, want 0 (nothing to grow yet)", items.atCalls)
	}
	want := wantContentWidth(face, items.items, theme.Light().Metric)
	if got := l.contentWidth(); got != want {
		t.Fatalf("contentWidth() after cold-cache append = %v, want %v (full measure)", got, want)
	}
}

// TestListViewContentWidthRemoveWidestRecomputes proves a Remove of the widest
// row shrinks the memo — the one change kind an append's incremental grow can
// never handle (it can only widen), so it must fully re-measure.
func TestListViewContentWidthRemoveWidestRecomputes(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	face := testFace(t)
	items := newFakeListItems("a", wideText, "c")
	l := NewListView(face, items)

	wide := l.contentWidth() // widest row is wideText at index 1

	items.RemoveAt(1) // drop the widest row
	got := l.contentWidth()
	if got >= wide {
		t.Fatalf("contentWidth() after removing the widest row = %v, want < %v (must shrink)", got, wide)
	}
	if want := wantContentWidth(face, items.items, theme.Light().Metric); got != want {
		t.Fatalf("contentWidth() after removing the widest row = %v, want %v", got, want)
	}
}

// TestListViewArrangeDoesNotRemeasureEveryRow is the point of the cache:
// scrolling a long list must not re-measure every row, only re-realize the
// handful of visible ones.
func TestListViewArrangeDoesNotRemeasureEveryRow(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	rows := make([]string, 200)
	for i := range rows {
		rows[i] = "Item"
	}
	items := newFakeListItems(rows...)
	l := NewListView(testFace(t), items).SetRowHeight(20)
	layoutListView(l, 0, 0, 120, 100)

	realized := len(l.pool)
	if realized == 0 || realized >= items.Len() {
		t.Fatalf("realized %d of %d rows, want a small virtualized slice", realized, items.Len())
	}

	// A wheel notch + re-arrange: only the realized rows may be read.
	items.atCalls = 0
	l.OnPointer(&input.PointerEvent{Action: input.Wheel, Delta: render.Point{Y: -1}, Router: input.NewRouter()})
	layoutListView(l, 0, 0, 120, 100)
	if items.atCalls > len(l.pool) {
		t.Fatalf("arrange after a scroll read %d items, want at most %d (the realized rows) — the width measure must be cached", items.atCalls, len(l.pool))
	}
}

func TestListViewContentWidthNilSources(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	nilItems := NewListView(testFace(t), nil)
	if got := nilItems.contentWidth(); got != 0 {
		t.Fatalf("contentWidth() with nil items = %v, want 0", got)
	}
	if got := nilItems.contentWidth(); got != 0 {
		t.Fatalf("cached contentWidth() with nil items = %v, want 0", got)
	}

	nilFace := NewListView(nil, newFakeListItems(wideText))
	if got := nilFace.contentWidth(); got != 0 {
		t.Fatalf("contentWidth() with a nil face = %v, want 0", got)
	}
}

// TestListViewHorizontalThumbAppearsAfterWideRowAdded checks the cache
// invalidation end to end: an added wide row must grow the horizontal thumb
// on the next arrange, not stay hidden behind a stale width.
func TestListViewHorizontalThumbAppearsAfterWideRowAdded(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	items := newFakeListItems("a", "b")
	l := NewListView(testFace(t), items).SetRowHeight(48)
	layoutListView(l, 0, 0, 100, 100)
	if _, ok := l.thumbRectX(); ok {
		t.Fatal("expected no horizontal thumb for short rows")
	}

	items.Add(wideText)
	layoutListView(l, 0, 0, 100, 100)
	if _, ok := l.thumbRectX(); !ok {
		t.Fatal("expected a horizontal thumb after adding a row far wider than the viewport")
	}
}

// TestListViewOnActivate covers the v0.20.0 activation enrichment: Enter on the
// selected row and a double-click both fire OnActivate; a single click selects
// without activating.
func TestListViewOnActivate(t *testing.T) {
	l := NewListView(nil, newFakeListItems("a", "b", "c")).SetRowHeight(20)
	var activated []int
	l.OnActivate(func(i int) { activated = append(activated, i) })

	r := input.NewRouter()
	r.SetRoot(l)
	layoutListView(l, 0, 0, 200, 100)
	r.Focus(l)

	l.selectUser(1)
	r.KeyDown(input.KeyEnter, 0, 0)
	if len(activated) != 1 || activated[0] != 1 {
		t.Fatalf("Enter activate = %v, want [1]", activated)
	}

	activated = nil
	l.OnPointer(&input.PointerEvent{Action: input.Press, Pos: render.Point{X: 10, Y: 5}, ClickCount: 2, Router: r})
	if len(activated) != 1 || activated[0] != 0 {
		t.Fatalf("double-click activate = %v, want [0]", activated)
	}

	activated = nil
	l.OnPointer(&input.PointerEvent{Action: input.Press, Pos: render.Point{X: 10, Y: 25}, ClickCount: 1, Router: r})
	if len(activated) != 0 {
		t.Fatalf("single click activated = %v, want none (select only)", activated)
	}
	if l.selected != 1 {
		t.Fatalf("single click selected = %d, want 1", l.selected)
	}
}
