package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"

	"golang.org/x/image/font/gofont/goregular"
)

// TestTabStripFocusRingHugsSelectedCell pins the WinUI-style focus ring: the
// ring targets the selected tab's header cell rect, not the whole strip.
func TestTabStripFocusRingHugsSelectedCell(t *testing.T) {
	f, err := text.Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	face := text.NewFace(f, 14)
	blk := func() core.Widget { return NewFixed(1, 1, render.Color{}) }
	tc := NewTabControl(face).AddTab("One", blk()).AddTab("Two", blk()).AddTab("Three", blk())
	layoutTabControl(tc, 0, 0, 300, 120)
	s := tc.strip

	// Header cells are contiguous in tab order, each cellWidths[i] wide.
	var acc float32
	for i := 0; i < 3; i++ {
		r := s.cellRect(i)
		if r.X != acc || r.W != s.cellWidths[i] {
			t.Fatalf("cellRect(%d) = %v, want X=%v W=%v", i, r, acc, s.cellWidths[i])
		}
		acc += r.W
	}

	// The ring hugs the selected cell — narrower than, and not equal to, the
	// full strip bounds.
	tc.SetSelectedIndex(1)
	ring := s.cellRect(tc.SelectedIndex())
	if ring == s.Bounds() || ring.W >= s.Bounds().W {
		t.Fatalf("ring %v should hug the selected cell, not span the strip %v", ring, s.Bounds())
	}
}

// layoutTabControl measures then arranges tc at the given absolute rect,
// mirroring layoutTreeView/layoutListView's shared test helper pattern.
func layoutTabControl(tc *TabControl, x, y, w, h float32) {
	core.MeasureWidget(tc, render.Size{W: w, H: h})
	core.ArrangeWidget(tc, render.Rect{X: x, Y: y, W: w, H: h})
}

// --- AddTab ordering ---

func TestTabControlAddTabOrdering(t *testing.T) {
	tc := NewTabControl(nil)
	one := NewTextBlock(nil, "one content")
	two := NewTextBlock(nil, "two content")
	three := NewTextBlock(nil, "three content")

	tc.AddTab("One", one)
	tc.AddTab("Two", two)
	tc.AddTab("Three", three)

	if got := len(tc.tabs); got != 3 {
		t.Fatalf("len(tabs) = %d, want 3", got)
	}
	wantTitles := []string{"One", "Two", "Three"}
	wantContent := []core.Widget{one, two, three}
	for i, want := range wantTitles {
		if tc.tabs[i].title != want {
			t.Fatalf("tabs[%d].title = %q, want %q", i, tc.tabs[i].title, want)
		}
		if tc.tabs[i].content != wantContent[i] {
			t.Fatalf("tabs[%d].content mismatch at index %d", i, i)
		}
	}
}

// AddTab must return the SAME *TabControl (chaining), per the brief's
// normative "AddTab(title, content) *TabControl" signature.
func TestTabControlAddTabReturnsSelfForChaining(t *testing.T) {
	tc := NewTabControl(nil)
	got := tc.AddTab("One", NewTextBlock(nil, "a")).AddTab("Two", NewTextBlock(nil, "b"))
	if got != tc {
		t.Fatalf("AddTab did not return the same *TabControl for chaining")
	}
	if len(tc.tabs) != 2 {
		t.Fatalf("len(tabs) = %d, want 2", len(tc.tabs))
	}
}

// --- Content visibility follows selection; hidden tabs stay in the tree ---

func TestTabControlContentVisibilityFollowsSelection(t *testing.T) {
	tc := NewTabControl(nil)
	one := NewTextBlock(nil, "one")
	two := NewTextBlock(nil, "two")
	three := NewTextBlock(nil, "three")
	tc.AddTab("One", one)
	tc.AddTab("Two", two)
	tc.AddTab("Three", three)

	// Tab 0 selected by default (the very first AddTab).
	if !one.Visible() || two.Visible() || three.Visible() {
		t.Fatalf("initial visibility = %v,%v,%v, want true,false,false", one.Visible(), two.Visible(), three.Visible())
	}

	tc.SetSelectedIndex(1)
	if one.Visible() || !two.Visible() || three.Visible() {
		t.Fatalf("after SetSelectedIndex(1) visibility = %v,%v,%v, want false,true,false", one.Visible(), two.Visible(), three.Visible())
	}

	// ALL three must remain reachable via Children() regardless of
	// visibility (the brief's normative "stay in the tree" rule).
	children := tc.Children()
	if len(children) != 4 { // strip + 3 contents
		t.Fatalf("len(Children()) = %d, want 4 (strip + 3 tab contents)", len(children))
	}
	found := map[core.Widget]bool{}
	for _, c := range children {
		found[c] = true
	}
	for name, w := range map[string]core.Widget{"one": one, "two": two, "three": three} {
		if !found[w] {
			t.Fatalf("Children() missing tab content %q even though hidden", name)
		}
	}
}

// Hidden tabs' content measures/arranges to {0,0} via the core engine's own
// hidden-widget shortcut, but the widgets themselves remain attached.
func TestTabControlHiddenContentMeasuresToZero(t *testing.T) {
	tc := NewTabControl(nil)
	one := NewTextBlock(nil, "one content here")
	two := NewTextBlock(nil, "two content here too")
	tc.AddTab("One", one)
	tc.AddTab("Two", two)

	layoutTabControl(tc, 0, 0, 200, 100)

	if d := core.DesiredSizeOf(two); d.W != 0 || d.H != 0 {
		t.Fatalf("hidden tab DesiredSizeOf = %+v, want {0,0}", d)
	}
	if b := core.BoundsOf(two); b.W != 0 || b.H != 0 {
		t.Fatalf("hidden tab BoundsOf = %+v, want zero-size rect", b)
	}
	// The selected tab's own bounds are real (non-degenerate).
	if b := core.BoundsOf(one); b.W <= 0 || b.H <= 0 {
		t.Fatalf("selected tab BoundsOf = %+v, want a real non-zero rect", b)
	}
}

// --- Header click selects (user-fired OnChanged) ---

func TestTabControlHeaderClickSelectsAndFiresOnChanged(t *testing.T) {
	tc := NewTabControl(testFace(t))
	tc.AddTab("One", NewTextBlock(nil, "1"))
	tc.AddTab("Two", NewTextBlock(nil, "2"))
	tc.AddTab("Three", NewTextBlock(nil, "3"))

	layoutTabControl(tc, 0, 0, 300, 100)

	var got int
	fired := 0
	tc.OnChanged(func(i int) { got = i; fired++ })

	// Click inside cell 2 ("Three")'s hit zone.
	cellRect2 := render.Rect{
		X: tc.strip.Bounds().X + tc.strip.cellWidths[0] + tc.strip.cellWidths[1],
		Y: tc.strip.Bounds().Y,
		W: tc.strip.cellWidths[2],
		H: tc.strip.Bounds().H,
	}
	pos := render.Point{X: cellRect2.X + 2, Y: cellRect2.Y + 2}

	r := input.NewRouter()
	e := &input.PointerEvent{Action: input.Press, Pos: pos, Router: r}
	tc.strip.OnPointer(e)

	if !e.Handled {
		t.Fatalf("Press over a real header cell was not marked handled")
	}
	if fired != 1 {
		t.Fatalf("OnChanged fired %d times, want 1", fired)
	}
	if got != 2 {
		t.Fatalf("OnChanged got index %d, want 2", got)
	}
	if tc.SelectedIndex() != 2 {
		t.Fatalf("SelectedIndex() = %d, want 2", tc.SelectedIndex())
	}

	// Re-clicking the ALREADY-selected cell must not re-fire (matches
	// ListView/ComboBox's "notify only on real change" convention).
	e2 := &input.PointerEvent{Action: input.Press, Pos: pos, Router: r}
	tc.strip.OnPointer(e2)
	if fired != 1 {
		t.Fatalf("OnChanged fired again on a no-op re-click: fired = %d, want 1", fired)
	}
}

// --- SetSelectedIndex is silent and clamped ---

func TestTabControlSetSelectedIndexSilentAndClamped(t *testing.T) {
	tc := NewTabControl(nil)
	tc.AddTab("One", NewTextBlock(nil, "1"))
	tc.AddTab("Two", NewTextBlock(nil, "2"))
	tc.AddTab("Three", NewTextBlock(nil, "3"))

	fired := 0
	tc.OnChanged(func(int) { fired++ })

	tc.SetSelectedIndex(5) // out of range: [0,2]
	if got := tc.SelectedIndex(); got != 2 {
		t.Fatalf("SetSelectedIndex(5) clamped to %d, want 2", got)
	}
	if fired != 0 {
		t.Fatalf("SetSelectedIndex fired OnChanged %d times, want 0 (silent)", fired)
	}

	tc.SetSelectedIndex(-3) // below range
	if got := tc.SelectedIndex(); got != 0 {
		t.Fatalf("SetSelectedIndex(-3) clamped to %d, want 0", got)
	}
	if fired != 0 {
		t.Fatalf("SetSelectedIndex fired OnChanged %d times, want 0 (silent)", fired)
	}

	tc.SetSelectedIndex(1)
	if got := tc.SelectedIndex(); got != 1 {
		t.Fatalf("SetSelectedIndex(1) = %d, want 1", got)
	}
	if fired != 0 {
		t.Fatalf("SetSelectedIndex fired OnChanged %d times, want 0 (silent)", fired)
	}
}

// --- Left/Right when focused switch tabs; clamped, no wrap ---

func TestTabControlLeftRightWhenFocused(t *testing.T) {
	tc := NewTabControl(nil)
	tc.AddTab("One", NewTextBlock(nil, "1"))
	tc.AddTab("Two", NewTextBlock(nil, "2"))
	tc.AddTab("Three", NewTextBlock(nil, "3"))

	var got int
	fired := 0
	tc.OnChanged(func(i int) { got = i; fired++ })

	tc.strip.OnFocusChanged(true)

	right := &input.KeyEvent{Action: input.Press, Key: input.KeyRight}
	tc.strip.OnKey(right)
	if !right.Handled {
		t.Fatalf("KeyRight on the focused strip was not marked handled")
	}
	if fired != 1 || got != 1 {
		t.Fatalf("after Right: fired=%d got=%d, want fired=1 got=1", fired, got)
	}

	tc.strip.OnKey(&input.KeyEvent{Action: input.Press, Key: input.KeyRight})
	if fired != 2 || got != 2 {
		t.Fatalf("after second Right: fired=%d got=%d, want fired=2 got=2", fired, got)
	}

	// At the last tab: Right must CLAMP, not wrap, and must not re-fire
	// (no actual change).
	tc.strip.OnKey(&input.KeyEvent{Action: input.Press, Key: input.KeyRight})
	if got := tc.SelectedIndex(); got != 2 {
		t.Fatalf("Right past the last tab landed on %d, want clamped at 2 (no wrap)", got)
	}
	if fired != 2 {
		t.Fatalf("Right past the last tab re-fired OnChanged: fired = %d, want 2", fired)
	}

	left := &input.KeyEvent{Action: input.Press, Key: input.KeyLeft}
	tc.strip.OnKey(left)
	if !left.Handled {
		t.Fatalf("KeyLeft on the focused strip was not marked handled")
	}
	if fired != 3 || got != 1 {
		t.Fatalf("after Left: fired=%d got=%d, want fired=3 got=1", fired, got)
	}

	// Back down to 0, then Left again must clamp (no wrap to the last tab).
	tc.strip.OnKey(&input.KeyEvent{Action: input.Press, Key: input.KeyLeft})
	tc.strip.OnKey(&input.KeyEvent{Action: input.Press, Key: input.KeyLeft})
	if got := tc.SelectedIndex(); got != 0 {
		t.Fatalf("Left past the first tab landed on %d, want clamped at 0 (no wrap)", got)
	}
}

// Left/Right must be ignored on a release (only Action==Press acts).
func TestTabControlOnKeyIgnoresRelease(t *testing.T) {
	tc := NewTabControl(nil)
	tc.AddTab("One", NewTextBlock(nil, "1"))
	tc.AddTab("Two", NewTextBlock(nil, "2"))
	tc.strip.OnFocusChanged(true)

	e := &input.KeyEvent{Action: input.Release, Key: input.KeyRight}
	tc.strip.OnKey(e)
	if e.Handled {
		t.Fatalf("KeyRight Release was marked handled, want ignored")
	}
	if tc.SelectedIndex() != 0 {
		t.Fatalf("SelectedIndex() = %d after a Release, want unchanged 0", tc.SelectedIndex())
	}
}

// --- Header hit zones: x-ranges = title width + 2*PaddingM, contiguous ---

func TestTabControlHeaderHitZones(t *testing.T) {
	tc := NewTabControl(testFace(t))
	tc.AddTab("One", NewTextBlock(nil, "1"))
	tc.AddTab("A Much Longer Tab", NewTextBlock(nil, "2"))
	tc.AddTab("C", NewTextBlock(nil, "3"))

	layoutTabControl(tc, 0, 0, 400, 100)

	widths := tc.strip.cellWidths
	if len(widths) != 3 {
		t.Fatalf("len(cellWidths) = %d, want 3", len(widths))
	}
	// "A Much Longer Tab" must measure wider than "One" or "C".
	if widths[1] <= widths[0] || widths[1] <= widths[2] {
		t.Fatalf("cellWidths = %v, want the long title's cell strictly widest", widths)
	}

	bounds := tc.strip.Bounds()
	// A point just inside the very start of cell 0.
	if idx, ok := tc.strip.cellAt(render.Point{X: bounds.X + 1, Y: bounds.Y + 1}); !ok || idx != 0 {
		t.Fatalf("cellAt(start of cell 0) = %d,%v, want 0,true", idx, ok)
	}
	// A point just inside the start of cell 1 (immediately after cell 0's
	// zone ends) — proves the zones are contiguous with no gap.
	x1 := bounds.X + widths[0] + 1
	if idx, ok := tc.strip.cellAt(render.Point{X: x1, Y: bounds.Y + 1}); !ok || idx != 1 {
		t.Fatalf("cellAt(start of cell 1) = %d,%v, want 1,true", idx, ok)
	}
	// A point just inside the start of cell 2.
	x2 := bounds.X + widths[0] + widths[1] + 1
	if idx, ok := tc.strip.cellAt(render.Point{X: x2, Y: bounds.Y + 1}); !ok || idx != 2 {
		t.Fatalf("cellAt(start of cell 2) = %d,%v, want 2,true", idx, ok)
	}
	// A point one px BEFORE cell 1 starts must still land in cell 0.
	xEdge := bounds.X + widths[0] - 1
	if idx, ok := tc.strip.cellAt(render.Point{X: xEdge, Y: bounds.Y + 1}); !ok || idx != 0 {
		t.Fatalf("cellAt(edge, cell 0 side) = %d,%v, want 0,true", idx, ok)
	}
	// Past the last cell entirely: no hit.
	xPast := bounds.X + widths[0] + widths[1] + widths[2] + 5
	if _, ok := tc.strip.cellAt(render.Point{X: xPast, Y: bounds.Y + 1}); ok {
		t.Fatalf("cellAt(past the last cell) reported ok=true, want false")
	}
}

// --- Strip is the focusable unit ---

func TestTabStripAcceptsFocus(t *testing.T) {
	tc := NewTabControl(nil)
	if !tc.strip.AcceptsFocus() {
		t.Fatalf("strip.AcceptsFocus() = false, want true")
	}
}

func TestTabStripFocusTracking(t *testing.T) {
	tc := NewTabControl(nil)
	if tc.strip.focused {
		t.Fatalf("strip.focused = true before any OnFocusChanged, want false")
	}
	tc.strip.OnFocusChanged(true)
	if !tc.strip.focused {
		t.Fatalf("strip.focused = false after OnFocusChanged(true), want true")
	}
	tc.strip.OnFocusChanged(false)
	if tc.strip.focused {
		t.Fatalf("strip.focused = true after OnFocusChanged(false), want false")
	}
}

// TabControl itself must never accept focus directly — only the strip does
// (the brief's normative "the strip is the focusable unit" rule).
func TestTabControlItselfIsNotFocusable(t *testing.T) {
	tc := NewTabControl(nil)
	if _, ok := core.Widget(tc).(input.Focusable); ok {
		t.Fatalf("*TabControl implements input.Focusable; only its strip should")
	}
}

// Focus routed through a real Router lands on the strip, not the content
// area, when a TabControl is clicked (proves press-to-focus resolves to
// the strip specifically, end to end).
func TestTabControlRouterFocusesStrip(t *testing.T) {
	tc := NewTabControl(testFace(t))
	tc.AddTab("One", NewTextBlock(testFace(t), "hello"))
	tc.AddTab("Two", NewTextBlock(testFace(t), "world"))

	r := input.NewRouter()
	r.SetRoot(tc)
	layoutTabControl(tc, 0, 0, 300, 100)

	pos := render.Point{X: tc.strip.Bounds().X + 5, Y: tc.strip.Bounds().Y + 5}
	r.PointerButton(input.ButtonLeft, true, pos, 0)
	r.PointerButton(input.ButtonLeft, false, pos, 0)

	if got := r.Focused(); got != core.Widget(tc.strip) {
		t.Fatalf("Focused() after clicking the strip = %v, want the strip", got)
	}
}

// --- Empty TabControl: harmless degenerate behavior ---

func TestTabControlNoTabsIsHarmless(t *testing.T) {
	tc := NewTabControl(nil)
	if got := tc.SelectedIndex(); got != 0 {
		t.Fatalf("SelectedIndex() on an empty TabControl = %d, want 0", got)
	}
	tc.SetSelectedIndex(3) // must not panic
	if got := tc.SelectedIndex(); got != 0 {
		t.Fatalf("SetSelectedIndex(3) on empty = %d, want 0", got)
	}
	layoutTabControl(tc, 0, 0, 100, 50) // must not panic
	if got := len(tc.Children()); got != 1 {
		t.Fatalf("len(Children()) on empty TabControl = %d, want 1 (strip only)", got)
	}

	tc.strip.OnFocusChanged(true)
	e := &input.KeyEvent{Action: input.Press, Key: input.KeyRight}
	tc.strip.OnKey(e)
	if e.Handled {
		t.Fatalf("KeyRight handled with zero tabs, want ignored")
	}
}
