package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/theme"
)

// clipRecorder is a recordRenderer that additionally tracks the live clip
// stack, so it can report what each FillRect actually PAINTS rather than
// only the rect it was handed. The distinction matters for any control that
// deliberately draws past its own edges and relies on a clip to crop the
// overhang — DataGrid's header cells, offset by the horizontal scroll, are
// exactly that: their raw rects start well left of the grid, and only the
// dedicated PushClip in DataGrid.Render keeps the painted pixels inside it.
// Checking raw rects there would flag correct code; checking clipped rects
// asks the question that actually matters, and still catches an overhang
// whose clip is missing, mis-sized, or popped too early.
//
// The embedded recordRenderer still records raw fills/clips/pops as before,
// so a test can assert on both views.
type clipRecorder struct {
	*recordRenderer

	// stack holds the clip in force, each entry already intersected with
	// the one below it (nested clips narrow, they never widen — matching
	// render/gl's own applyClip).
	stack []render.Rect

	// painted holds every FillRect intersected with the clip in force when
	// it was issued; a fill entirely outside the clip lands here as the
	// empty rect (it paints nothing).
	painted []filledRect
}

func newClipRecorder() *clipRecorder {
	return &clipRecorder{recordRenderer: &recordRenderer{}}
}

func (c *clipRecorder) clip() (render.Rect, bool) {
	if n := len(c.stack); n > 0 {
		return c.stack[n-1], true
	}
	return render.Rect{}, false
}

func (c *clipRecorder) FillRect(rect render.Rect, col render.Color) {
	c.recordRenderer.FillRect(rect, col)
	if cur, ok := c.clip(); ok {
		rect = rect.Intersect(cur)
	}
	c.painted = append(c.painted, filledRect{rect: rect, color: col})
}

func (c *clipRecorder) PushClip(rect render.Rect) {
	c.recordRenderer.PushClip(rect)
	if cur, ok := c.clip(); ok {
		rect = rect.Intersect(cur)
	}
	c.stack = append(c.stack, rect)
}

func (c *clipRecorder) PopClip() {
	c.recordRenderer.PopClip()
	if n := len(c.stack); n > 0 {
		c.stack = c.stack[:n-1]
	}
}

// assertPaintedWithinBounds fails if any pixel a render pass actually
// painted (see clipRecorder.painted) falls outside bounds. It is the cheap,
// control-agnostic guard for the whole class of bugs where a control paints
// chrome derived from the WRONG rect — a selection band computed from an
// unclipped row rect bleeding onto the sunken bevel, a scroll thumb placed
// against a gutter that was never reserved, a scrolled header cell with no
// clip to crop it. Every one of those shows up here as an overhang,
// whatever its color or draw order.
//
// Bounds are INCLUSIVE (a fill flush with an edge is fine — drawRaised and
// drawSunken both paint their outermost 1px edge rects exactly on the
// boundary). Empty fills are skipped: a rect clipped away entirely, or
// zero-sized to begin with, paints nothing, so where it nominally sat is
// not observable.
//
// An empty recording is a hard failure, not a pass: asserting that nothing
// escaped while the control drew nothing at all checks nothing, and would
// keep passing if the render method were gutted.
func assertPaintedWithinBounds(t *testing.T, rr *clipRecorder, bounds render.Rect, what string) {
	t.Helper()

	if len(rr.painted) == 0 {
		t.Fatalf("%s: no FillRect calls recorded, so nothing was actually checked", what)
	}
	for i, f := range rr.painted {
		if f.rect.Empty() {
			continue
		}
		if f.rect.X < bounds.X || f.rect.Y < bounds.Y ||
			f.rect.Right() > bounds.Right() || f.rect.Bottom() > bounds.Bottom() {
			t.Errorf("%s: fill %d %+v escapes the control's bounds %+v", what, i, f.rect, bounds)
		}
	}
}

// TestListViewRenderStaysWithinBounds applies assertPaintedWithinBounds to
// both halves of ListView's paint — Render (the sunken well plus the
// selection band) and RenderOverlay (both scroll thumbs plus the focus
// ring) — with the list deliberately overflowing on BOTH axes and scrolled
// to a fractional offset, so the selected row is only PARTLY visible at the
// viewport's top edge. That is the shape that catches a band cropped
// against the wrong rect: an uncropped full-height band for a partial row
// overhangs the viewport, and at the offset chosen here it overhangs the
// control's outer bevel too. ListView pushes no clip of its own during
// Render (core.RenderWidget applies ClipRect to its CHILDREN, after Render
// has already run), so for this control raw and painted rects coincide.
func TestListViewRenderStaysWithinBounds(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	face := testFace(t)
	items := newFakeListItems()
	for i := 0; i < 40; i++ {
		items.items = append(items.items, "a fairly long list item label "+string(rune('a'+i%26)))
	}

	l := NewListView(face, items)
	l.SetRowHeight(20)
	l.SetSelectedIndex(3)
	// Row 3 is the FIRST realized row and is scrolled 10px past the top of
	// the viewport, so its full-height band starts above the control's own
	// bounds — not merely above the viewport, which the 2px bevel inset
	// alone would absorb.
	l.rawOffset = 70
	l.OnFocusChanged(true)

	bounds := render.Rect{X: 10, Y: 20, W: 120, H: 90}
	layoutListView(l, bounds.X, bounds.Y, bounds.W, bounds.H)

	if l.visibleFirst != l.selected {
		t.Fatalf("fixture: visibleFirst = %d, want the selected row %d to be the straddling one", l.visibleFirst, l.selected)
	}
	if _, ok := l.thumbRect(); !ok {
		t.Fatal("fixture: no vertical thumb, so RenderOverlay draws nothing to check")
	}
	// The uncropped band must genuinely overhang, or the assertion below
	// proves nothing about the crop.
	if top := l.rowTop(l.selected); top >= bounds.Y {
		t.Fatalf("fixture: the selected row's band starts at %v, inside bounds.Y %v — nothing overhangs to crop", top, bounds.Y)
	}

	rr := newClipRecorder()
	l.Render(rr)
	assertPaintedWithinBounds(t, rr, bounds, "ListView.Render")

	overlay := newClipRecorder()
	l.RenderOverlay(overlay)
	assertPaintedWithinBounds(t, overlay, bounds, "ListView.RenderOverlay")
}

// TestDataGridRenderStaysWithinBounds is TestListViewRenderStaysWithinBounds'
// counterpart for DataGrid, which has strictly more geometry to get wrong:
// on top of the band and the two thumbs it also paints a header strip whose
// cells are offset by the horizontal scroll. The columns here are Px-only
// and total wider than the viewport (the deliberate overflow shape — a Star
// column resolves to exactly the viewport width and can never overflow),
// and the grid is scrolled right, so the leftmost header cell's raw rect
// starts well LEFT of the grid and the rightmost one ends well past its
// right edge. Both are held inside the grid only by Render's dedicated
// header clip, so this is the test that fails outright if that clip is
// dropped, mis-sized, or popped before the header loops finish — and the
// raw-rect assertions below pin the clip itself rather than trusting the
// crop to have come from somewhere.
func TestDataGridRenderStaysWithinBounds(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	g := NewDataGrid(nil)
	g.SetColumns(
		Column{Title: "Name", Width: Px(120)},
		Column{Title: "Email", Width: Px(140)},
		Column{Title: "Age", Width: Px(120)},
	)
	g.SetRowCount(30)
	g.rowH = 20
	g.SetSelectedIndex(2)
	g.rawOffset = 35 // not a multiple of rowH: the selected row straddles the top
	g.ScrollToX(90)
	g.OnFocusChanged(true)

	bounds := render.Rect{X: 10, Y: 20, W: 200, H: 120}
	layoutDataGrid(g, bounds.X, bounds.Y, bounds.W, bounds.H)

	if g.offsetX == 0 {
		t.Fatal("fixture: the grid did not actually scroll horizontally")
	}
	if _, ok := g.thumbRectX(); !ok {
		t.Fatal("fixture: no horizontal thumb, so the overflow scenario did not materialize")
	}

	rr := newClipRecorder()
	g.Render(rr)
	assertPaintedWithinBounds(t, rr, bounds, "DataGrid.Render")

	overlay := newClipRecorder()
	g.RenderOverlay(overlay)
	assertPaintedWithinBounds(t, overlay, bounds, "DataGrid.RenderOverlay")

	// The header clip is doing real work here, not merely being redundant:
	// at least one raw header rect must actually overhang, or the crop above
	// proved nothing.
	var overhang bool
	for _, f := range rr.fills {
		if f.rect.X < bounds.X || f.rect.Right() > bounds.Right() {
			overhang = true
			break
		}
	}
	if !overhang {
		t.Fatal("fixture: no header cell overhangs the grid, so the header clip was never exercised")
	}

	if len(rr.clips) == 0 {
		t.Fatal("DataGrid.Render pushed no clip: scrolled header cells have nothing cropping them")
	}
	if rr.pops != len(rr.clips) {
		t.Fatalf("DataGrid.Render pushed %d clips but popped %d", len(rr.clips), rr.pops)
	}
	if headerClip := rr.clips[0]; headerClip.X < bounds.X || headerClip.Right() > bounds.Right() {
		t.Fatalf("header clip %+v is not contained by the grid's bounds %+v", headerClip, bounds)
	}
}

// TestDataGridClipRectExcludesHeaderAndBevel pins the clip realized body
// cells are cropped to. Two edges carry the whole contract, and both are
// stated as relationships rather than by restating ClipRect's arithmetic —
// a formula copied into the test would move in lockstep with a wrong change
// to the real one:
//
//   - the TOP must land exactly on the bottom of the header strip Render
//     paints (g.header). A clip that starts any higher lets a
//     partially-scrolled top row push its text up under the column titles;
//     any lower and the first body row is shaved.
//   - the outer edges must sit inside the sunken bevel, so no cell paints
//     over the well's frame.
//
// The right edge deliberately INCLUDES the thumb gutter: the thumb is drawn
// in RenderOverlay, after this clip is popped, so widening past the
// viewport costs nothing and keeps the clip from cropping it.
func TestDataGridClipRectExcludesHeaderAndBevel(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	g := NewDataGrid(nil)
	g.SetColumns(
		Column{Title: "Name", Width: Px(60), Value: func(int) string { return "n" }},
		Column{Title: "Age", Width: Star(1), Value: func(int) string { return "a" }},
	)
	g.SetRowCount(30)
	g.rowH = 20
	g.rawOffset = 35 // fractional: the top row straddles the clip's own top edge

	bounds := render.Rect{X: 10, Y: 20, W: 160, H: 120}
	layoutDataGrid(g, bounds.X, bounds.Y, bounds.W, bounds.H)

	clip, ok := g.ClipRect()
	if !ok {
		t.Fatal("ClipRect() ok = false, want true (DataGrid always clips its cells)")
	}
	if g.header.Empty() {
		t.Fatal("fixture: the header strip is empty, so its bottom edge pins nothing")
	}
	if clip.Y != g.header.Bottom() {
		t.Errorf("clip top = %v, want the header's bottom edge %v — cells can bleed into the column titles",
			clip.Y, g.header.Bottom())
	}

	bw := g.metrics.BevelWidth
	if clip.X != bounds.X+bw {
		t.Errorf("clip left = %v, want bounds.X+BevelWidth = %v", clip.X, bounds.X+bw)
	}
	// This fixture overflows vertically (30 rows) but not horizontally (the
	// Star column soaks up the width), so the last arrange reserved the RIGHT
	// gutter only. The clip's right edge must land on the viewport's — SHORT of
	// the bevel-inset right by exactly the gutter — so cells stop where the
	// vertical thumb's lane begins and never spill into the dead corner. The
	// thumb itself draws in RenderOverlay after the clip pops, so excluding the
	// gutter here does not crop it.
	if clip.Right() != g.viewport.Right() {
		t.Errorf("clip right = %v, want viewport right %v (cells must stop at the gutter)", clip.Right(), g.viewport.Right())
	}
	if clip.Right() >= bounds.Right()-bw {
		t.Errorf("clip right %v does not exclude the reserved gutter (bevel-inset right is %v)", clip.Right(), bounds.Right()-bw)
	}
	// No horizontal overflow, so no bottom gutter: the clip reaches the
	// bevel-inset bottom.
	if clip.Bottom() != bounds.Bottom()-bw {
		t.Errorf("clip bottom = %v, want bounds.Bottom()-BevelWidth = %v", clip.Bottom(), bounds.Bottom()-bw)
	}

	// The clip must be doing real work: with a fractional offset the top
	// realized row genuinely starts above it.
	if top := g.rowTop(g.visibleFirst); top >= clip.Y {
		t.Fatalf("fixture: the top realized row starts at %v, at or below the clip's %v — nothing is actually cropped", top, clip.Y)
	}
}

// TestClipRectsStayNonNegativeWhenTiny covers the degenerate branch both
// ClipRect implementations guard: bounds narrower than the two bevels they
// carve out would otherwise produce a negative-WIDTH rect, which render/gl's
// applyClip hands to gl.Scissor as a negative box — GL_INVALID_VALUE, which
// leaves the PREVIOUS scissor box in force, so the cells end up cropped by
// whatever rect happened to be active instead of their own.
//
// Height is asserted for ListView only. DataGrid's guard clamps the
// bevel-inset rect but the header strip is subtracted AFTERWARDS, unclamped
// (see ClipRect), so a grid shorter than its own header still returns a
// negative H today. That is a logic gap, not a test gap; this test
// deliberately does not assert it, so whichever change closes it does not
// have to touch this file.
func TestClipRectsStayNonNegativeWhenTiny(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	tiny := render.Rect{X: 0, Y: 0, W: 1, H: 1}

	g := NewDataGrid(nil)
	g.SetColumns(Column{Title: "Name", Width: Px(60)})
	g.SetRowCount(5)
	layoutDataGrid(g, tiny.X, tiny.Y, tiny.W, tiny.H)
	if clip, _ := g.ClipRect(); clip.W < 0 {
		t.Errorf("DataGrid.ClipRect() at %v = %+v, want no negative width", tiny, clip)
	}

	l := NewListView(nil, newFakeListItems("a", "b", "c"))
	layoutListView(l, tiny.X, tiny.Y, tiny.W, tiny.H)
	if clip, _ := l.ClipRect(); clip.W < 0 || clip.H < 0 {
		t.Errorf("ListView.ClipRect() at %v = %+v, want no negative extent", tiny, clip)
	}
}

// TestListViewClipRectExcludesBevel is DataGrid's counterpart for ListView,
// which has no header strip: the clip is the bevel-inset content rect minus
// whichever thumb gutters the last arrange reserved, and a partially-scrolled
// row at the top edge must genuinely overhang it (otherwise the row's text
// paints over the sunken well's own frame).
func TestListViewClipRectExcludesBevel(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	items := newFakeListItems()
	for i := 0; i < 40; i++ {
		items.items = append(items.items, "item")
	}
	l := NewListView(testFace(t), items)
	l.SetRowHeight(20)
	l.rawOffset = 70 // fractional: the top realized row straddles the clip

	bounds := render.Rect{X: 10, Y: 20, W: 120, H: 90}
	layoutListView(l, bounds.X, bounds.Y, bounds.W, bounds.H)

	clip, ok := l.ClipRect()
	if !ok {
		t.Fatal("ClipRect() ok = false, want true (ListView always clips its rows)")
	}

	bw := l.metrics.BevelWidth
	// Vertical overflow (40 rows) but no horizontal overflow, so the last
	// arrange reserved the RIGHT gutter only. The clip is the bevel-inset rect
	// with that gutter carved off its right edge — rows stop where the thumb's
	// lane begins, never under it and never into the dead corner. The thumb
	// draws in RenderOverlay after the clip pops, so this does not crop it.
	want := render.Rect{X: bounds.X + bw, Y: bounds.Y + bw, W: bounds.W - 2*bw, H: bounds.H - 2*bw}
	want = want.Inset(render.Thickness{Right: l.gutter})
	if clip != want {
		t.Fatalf("ClipRect() = %+v, want the bevel-inset rect minus the reserved gutter %+v", clip, want)
	}
	if clip.Right() >= bounds.Right()-bw {
		t.Fatalf("clip right %v does not exclude the reserved gutter (bevel-inset right is %v)", clip.Right(), bounds.Right()-bw)
	}
	if top := l.rowTop(l.visibleFirst); top >= clip.Y {
		t.Fatalf("fixture: the top realized row starts at %v, at or below the clip's %v — nothing is actually cropped", top, clip.Y)
	}
}

// TestDataGridChildrenAreTheRealizedCellPool pins Children() as the
// hit-test/render view of the virtualized cell pool: exactly one entry per
// realized (visibleRow, column) pair, every one a live *TextBlock parented
// to the grid, and the returned slice a COPY — Children is handed straight
// to core.RenderWidget's child walk, so a caller mutating it must not be
// able to reach into the pool the next arrange pass reuses.
func TestDataGridChildrenAreTheRealizedCellPool(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	g := NewDataGrid(nil)

	// Before any layout there is nothing realized at all.
	if got := g.Children(); got != nil {
		t.Fatalf("Children() before layout = %v, want nil (empty pool)", got)
	}

	g.SetColumns(
		Column{Title: "Name", Width: Px(60), Value: func(int) string { return "n" }},
		Column{Title: "Age", Width: Px(40), Value: func(int) string { return "a" }},
	)
	g.SetRowCount(30)
	g.rowH = 20
	layoutDataGrid(g, 0, 0, 120, 120)

	want := g.visibleCount * len(g.columns)
	if want == 0 {
		t.Fatal("fixture: no rows were realized, so nothing was actually checked")
	}
	kids := g.Children()
	if len(kids) != want {
		t.Fatalf("len(Children()) = %d, want visibleCount*cols = %d", len(kids), want)
	}
	for i, k := range kids {
		tb, ok := k.(*TextBlock)
		if !ok {
			t.Fatalf("Children()[%d] type = %T, want *TextBlock", i, k)
		}
		if core.ParentOf(tb) != core.Widget(g) {
			t.Fatalf("Children()[%d] is not parented to the grid", i)
		}
	}

	// Mutating the returned slice must not disturb the pool behind it.
	kids[0] = nil
	if again := g.Children(); again[0] == nil {
		t.Fatal("Children() returned a live view of the pool, want a copy")
	}
}

// TestDataGridAcceptsFocus pins the always-focusable contract DataGrid
// shares with ListView (v0 has no disabled concept for either) — the
// precondition for RenderOverlay's focus ring ever being drawn at all.
func TestDataGridAcceptsFocus(t *testing.T) {
	if !NewDataGrid(nil).AcceptsFocus() {
		t.Fatal("DataGrid.AcceptsFocus() = false, want true")
	}
	if !NewListView(nil, newFakeListItems()).AcceptsFocus() {
		t.Fatal("ListView.AcceptsFocus() = false, want true")
	}
}
