package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/theme"
)

// layoutExpander measures then arranges e at the given bounds, mirroring
// layoutListView/layoutTreeView.
func layoutExpander(e *Expander, x, y, w, h float32) {
	core.MeasureWidget(e, render.Size{W: w, H: h})
	core.ArrangeWidget(e, render.Rect{X: x, Y: y, W: w, H: h})
}

// --- Construction / initial state ---

func TestNewExpanderStartsCollapsed(t *testing.T) {
	e := NewExpander(nil, "Details")
	if e.Expanded() {
		t.Fatal("NewExpander starts expanded, want collapsed")
	}
	if e.header.chevron.Text() != ">" {
		t.Fatalf("initial chevron = %q, want %q (collapsed)", e.header.chevron.Text(), ">")
	}
}

// --- Toggle: click anywhere on the header ---

func TestExpanderHeaderClickTogglesAndFiresOnChanged(t *testing.T) {
	e := NewExpander(nil, "Details")
	e.SetContent(NewFixed(50, 20, render.RGB(0, 0, 0)))
	layoutExpander(e, 0, 0, 200, 200)

	var got []bool
	e.OnChanged(func(v bool) { got = append(got, v) })

	r := input.NewRouter()
	press := func() {
		down := &input.PointerEvent{Action: input.Press, Pos: render.Point{X: 5, Y: 5}, Router: r}
		e.header.OnPointer(down)
		up := &input.PointerEvent{Action: input.Release, Pos: render.Point{X: 5, Y: 5}, Router: r}
		e.header.OnPointer(up)
	}

	press()
	if !e.Expanded() {
		t.Fatal("header click did not expand")
	}
	if e.header.chevron.Text() != "v" {
		t.Fatalf("chevron after expand = %q, want %q", e.header.chevron.Text(), "v")
	}

	press()
	if e.Expanded() {
		t.Fatal("second header click did not collapse")
	}

	want := []bool{true, false}
	if len(got) != len(want) {
		t.Fatalf("OnChanged calls = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("OnChanged calls = %v, want %v", got, want)
		}
	}
}

func TestExpanderHeaderClickAnywhereAcrossFullWidthToggles(t *testing.T) {
	// The header stretches to the Expander's full arranged width; a click
	// far to the right of the chevron/title text must still toggle (unlike
	// TreeView, there is no separate chevron-vs-label zone split here — the
	// WHOLE header row is the button, per the brief).
	e := NewExpander(nil, "Details")
	layoutExpander(e, 0, 0, 300, 200)

	r := input.NewRouter()
	pos := render.Point{X: 250, Y: 5} // far right of the header, still within its stretched bounds
	down := &input.PointerEvent{Action: input.Press, Pos: pos, Router: r}
	e.header.OnPointer(down)
	up := &input.PointerEvent{Action: input.Release, Pos: pos, Router: r}
	e.header.OnPointer(up)

	if !e.Expanded() {
		t.Fatal("click at the far right of the stretched header did not toggle")
	}
}

// --- Silent setter ---

func TestExpanderSetExpandedIsSilent(t *testing.T) {
	e := NewExpander(nil, "Details")

	fired := false
	e.OnChanged(func(bool) { fired = true })

	e.SetExpanded(true)
	if !e.Expanded() {
		t.Fatal("SetExpanded(true) did not expand")
	}
	if fired {
		t.Fatal("SetExpanded fired OnChanged, want fully silent (programmatic setter)")
	}

	e.SetExpanded(false)
	if e.Expanded() {
		t.Fatal("SetExpanded(false) did not collapse")
	}
	if fired {
		t.Fatal("SetExpanded fired OnChanged, want fully silent (programmatic setter)")
	}
}

func TestExpanderSetExpandedSameValueIsNoOp(t *testing.T) {
	e := NewExpander(nil, "Details")
	layoutExpander(e, 0, 0, 200, 200)
	if e.NeedsLayout() {
		t.Fatal("expected clean layout before the no-op SetExpanded")
	}

	e.SetExpanded(false) // already collapsed
	if e.NeedsLayout() {
		t.Fatal("SetExpanded(false) on an already-collapsed Expander invalidated layout, want no-op")
	}
}

func TestExpanderSetExpandedUpdatesChevronGlyph(t *testing.T) {
	e := NewExpander(nil, "Details")
	e.SetExpanded(true)
	if got := e.header.chevron.Text(); got != "v" {
		t.Fatalf("chevron = %q, want %q", got, "v")
	}
	e.SetExpanded(false)
	if got := e.header.chevron.Text(); got != ">" {
		t.Fatalf("chevron = %q, want %q", got, ">")
	}
}

// --- Content layout participation ---

func TestExpanderContentNotInChildrenWhileCollapsed(t *testing.T) {
	e := NewExpander(nil, "Details")
	content := NewFixed(50, 20, render.RGB(0, 0, 0))
	e.SetContent(content)

	kids := e.Children()
	if len(kids) != 1 {
		t.Fatalf("Children() while collapsed = %d, want 1 (header only)", len(kids))
	}
	if kids[0] != core.Widget(e.header) {
		t.Fatal("Children()[0] while collapsed is not the header")
	}
}

func TestExpanderContentInChildrenWhileExpanded(t *testing.T) {
	e := NewExpander(nil, "Details")
	content := NewFixed(50, 20, render.RGB(0, 0, 0))
	e.SetContent(content)
	e.SetExpanded(true)

	kids := e.Children()
	if len(kids) != 2 {
		t.Fatalf("Children() while expanded = %d, want 2 (header, content)", len(kids))
	}
	if kids[1] != core.Widget(content) {
		t.Fatal("Children()[1] while expanded is not content")
	}
}

func TestExpanderMeasureDiffersCollapsedVsExpanded(t *testing.T) {
	e := NewExpander(nil, "Details")
	e.SetContent(NewFixed(50, 80, render.RGB(0, 0, 0)))

	core.MeasureWidget(e, render.Size{W: 300, H: 300})
	collapsedH := core.DesiredSizeOf(e).H

	e.SetExpanded(true)
	core.MeasureWidget(e, render.Size{W: 300, H: 300})
	expandedH := core.DesiredSizeOf(e).H

	if expandedH <= collapsedH {
		t.Fatalf("expandedH = %v, collapsedH = %v; want expanded strictly taller (content's 80px must be added)", expandedH, collapsedH)
	}
	if got, want := expandedH-collapsedH, float32(80); got != want {
		t.Fatalf("height delta = %v, want exactly %v (content's own desired height)", got, want)
	}
}

func TestExpanderContentNotMeasuredAtAllWhileCollapsed(t *testing.T) {
	// A collapsed Expander must not even CALL MeasureContent on its content —
	// not just discard the result — per the brief's "content participates in
	// layout ONLY when expanded". Detect this via NeedsLayout: a content
	// widget that starts dirty (the zero value) stays dirty forever if it's
	// never measured.
	e := NewExpander(nil, "Details")
	content := NewFixed(50, 80, render.RGB(0, 0, 0))
	e.SetContent(content)

	if !content.NeedsLayout() {
		t.Fatal("fresh content should start needing layout")
	}

	core.MeasureWidget(e, render.Size{W: 300, H: 300})
	core.ArrangeWidget(e, render.Rect{X: 0, Y: 0, W: 300, H: 300})

	if !content.NeedsLayout() {
		t.Fatal("content's NeedsLayout cleared while Expander stayed collapsed; content must never be measured/arranged while collapsed")
	}
}

func TestExpanderArrangesContentBelowHeader(t *testing.T) {
	e := NewExpander(nil, "Details")
	content := NewFixed(50, 30, render.RGB(0, 0, 0))
	e.SetContent(content)
	e.SetExpanded(true)
	layoutExpander(e, 10, 20, 200, 200)

	headerB := e.header.Bounds()
	contentB := content.Bounds()

	if contentB.Y != headerB.Y+headerB.H {
		t.Fatalf("content.Y = %v, want %v (directly below header)", contentB.Y, headerB.Y+headerB.H)
	}
	if contentB.X != headerB.X {
		t.Fatalf("content.X = %v, want %v (aligned with header)", contentB.X, headerB.X)
	}
}

// --- Header chrome: full-width stretch ---

func TestExpanderHeaderStretchesToFullWidth(t *testing.T) {
	e := NewExpander(nil, "Details")
	layoutExpander(e, 0, 0, 300, 200)

	if got := e.header.Bounds().W; got != 300 {
		t.Fatalf("header width = %v, want 300 (stretched to full arranged width)", got)
	}
}

// --- Theming: real face, real theme sanity ---

func TestExpanderWithRealFaceAndTheme(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	face := testFace(t)
	e := NewExpander(face, "Details")
	e.SetContent(NewTextBlock(face, "Hello"))
	e.SetExpanded(true)
	layoutExpander(e, 0, 0, 200, 200)

	if e.header.Bounds().H <= 0 {
		t.Fatal("header measured to zero height with a real face")
	}
}
