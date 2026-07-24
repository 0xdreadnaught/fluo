package controls

import (
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
)

// buttonFace loads a real face for layout-accurate button tests (nil faces
// are fine for pure state-machine tests, but interaction tests want a
// non-degenerate label so bounds/desired size aren't all zero).
func buttonFace(t *testing.T) *text.Face {
	t.Helper()
	f, err := text.Load(goregular.TTF)
	if err != nil {
		t.Fatalf("text.Load: %v", err)
	}
	return text.NewFace(f, 14)
}

// layoutButton measures then arranges w (any core.Widget) at the given
// absolute rect, the pattern every interaction test below shares.
func layoutButton(w core.Widget, bounds render.Rect) {
	core.MeasureWidget(w, render.Size{W: bounds.W, H: bounds.H})
	core.ArrangeWidget(w, bounds)
}

func TestButtonClickFiresViaRouter(t *testing.T) {
	b := NewButton(buttonFace(t), "Click me")
	b.SetWidth(80)
	b.SetHeight(30)

	clicks := 0
	b.OnClick(func() { clicks++ })

	r := input.NewRouter()
	r.SetRoot(b)
	layoutButton(b, render.Rect{X: 0, Y: 0, W: 80, H: 30})

	pos := render.Point{X: 40, Y: 15}
	r.PointerButton(input.ButtonLeft, true, pos, 0)
	r.PointerButton(input.ButtonLeft, false, pos, 0)

	if clicks != 1 {
		t.Fatalf("clicks = %d, want 1 (press+release inside)", clicks)
	}
	// PointerButton's press-to-focus also focuses the button (AcceptsFocus).
	if got := r.Focused(); got != core.Widget(b) {
		t.Fatalf("Focused() after click = %v, want button", got)
	}
}

func TestButtonReleaseOutsideDoesNotFire(t *testing.T) {
	b := NewButton(buttonFace(t), "Click me")
	b.SetWidth(80)
	b.SetHeight(30)

	clicks := 0
	b.OnClick(func() { clicks++ })

	r := input.NewRouter()
	r.SetRoot(b)
	layoutButton(b, render.Rect{X: 0, Y: 0, W: 80, H: 30})

	r.PointerButton(input.ButtonLeft, true, render.Point{X: 40, Y: 15}, 0)
	// Still captured, so this Release is delivered directly regardless of
	// position — well outside the button's {0,0,80,30} bounds.
	r.PointerButton(input.ButtonLeft, false, render.Point{X: 900, Y: 900}, 0)

	if clicks != 0 {
		t.Fatalf("clicks = %d, want 0 (release outside must not fire)", clicks)
	}
	if got := r.Captured(); got != nil {
		t.Fatalf("Captured() after outside release = %v, want nil", got)
	}
}

func TestButtonDisabledIgnoresPointerAndFocus(t *testing.T) {
	b := NewButton(buttonFace(t), "Nope").SetEnabled(false)
	b.SetWidth(80)
	b.SetHeight(30)
	layoutButton(b, render.Rect{X: 0, Y: 0, W: 80, H: 30})

	if b.AcceptsFocus() {
		t.Fatal("AcceptsFocus() = true for a disabled button, want false")
	}

	clicked := false
	b.OnClick(func() { clicked = true })

	press := &input.PointerEvent{Action: input.Press, Pos: render.Point{X: 40, Y: 15}}
	b.OnPointer(press)
	if press.Handled {
		t.Fatal("Press on a disabled button set Handled = true, want false (ignored, not handled)")
	}

	release := &input.PointerEvent{Action: input.Release, Pos: render.Point{X: 40, Y: 15}}
	b.OnPointer(release)
	if release.Handled {
		t.Fatal("Release on a disabled button set Handled = true, want false")
	}
	if clicked {
		t.Fatal("disabled button fired OnClick")
	}

	// Same story end-to-end via a real router: no capture, no click.
	r := input.NewRouter()
	r.SetRoot(b)
	pos := render.Point{X: 40, Y: 15}
	r.PointerButton(input.ButtonLeft, true, pos, 0)
	if got := r.Captured(); got != nil {
		t.Fatalf("Captured() after press on disabled button = %v, want nil", got)
	}
	r.PointerButton(input.ButtonLeft, false, pos, 0)
	if clicked {
		t.Fatal("disabled button fired OnClick via router")
	}
}

func TestButtonSpaceActivatesWhenFocused(t *testing.T) {
	b := NewButton(buttonFace(t), "Go")
	b.SetWidth(80)
	b.SetHeight(30)

	clicks := 0
	b.OnClick(func() { clicks++ })

	r := input.NewRouter()
	r.SetRoot(b)
	layoutButton(b, render.Rect{X: 0, Y: 0, W: 80, H: 30})

	r.Focus(b)
	r.KeyDown(input.KeySpace, 0, 0)

	if clicks != 1 {
		t.Fatalf("clicks = %d, want 1 (Space while focused activates)", clicks)
	}
}

func TestButtonEnterActivatesWhenFocused(t *testing.T) {
	b := NewButton(buttonFace(t), "Go")
	b.SetWidth(80)
	b.SetHeight(30)

	clicks := 0
	b.OnClick(func() { clicks++ })

	r := input.NewRouter()
	r.SetRoot(b)
	layoutButton(b, render.Rect{X: 0, Y: 0, W: 80, H: 30})

	r.Focus(b)
	r.KeyDown(input.KeyEnter, 0, 0)

	if clicks != 1 {
		t.Fatalf("clicks = %d, want 1 (Enter while focused activates)", clicks)
	}
}

func TestButtonKeyIgnoredWhenNotFocused(t *testing.T) {
	b := NewButton(buttonFace(t), "Go")
	b.SetWidth(80)
	b.SetHeight(30)

	clicks := 0
	b.OnClick(func() { clicks++ })

	// The button must NOT be the router's root itself here: with nothing
	// focused, input.Router.dispatchKey falls back to delivering straight to
	// the root (see router.go), so testing "unfocused" requires a root that
	// ISN'T the button, or that fallback would deliver to it anyway and this
	// test would pass for the wrong reason.
	host := NewStackPanel(Horizontal).Add(b)

	r := input.NewRouter()
	r.SetRoot(host)
	layoutButton(host, render.Rect{X: 0, Y: 0, W: 200, H: 30})

	r.KeyDown(input.KeySpace, 0, 0)
	if clicks != 0 {
		t.Fatalf("clicks = %d, want 0 (no focus, Space must not activate)", clicks)
	}
}

func TestButtonHoverAndPressedObservable(t *testing.T) {
	b := NewButton(buttonFace(t), "Hover me")
	b.SetWidth(80)
	b.SetHeight(30)

	r := input.NewRouter()
	r.SetRoot(b)
	layoutButton(b, render.Rect{X: 0, Y: 0, W: 80, H: 30})

	if b.click.Hover() || b.click.Pressed() {
		t.Fatal("Hover()/Pressed() must both start false")
	}

	r.PointerMove(render.Point{X: 40, Y: 15}, 0)
	if !b.click.Hover() {
		t.Fatal("Hover() = false after Move inside, want true")
	}

	r.PointerButton(input.ButtonLeft, true, render.Point{X: 40, Y: 15}, 0)
	if !b.click.Pressed() {
		t.Fatal("Pressed() = false after Press, want true")
	}

	r.PointerButton(input.ButtonLeft, false, render.Point{X: 40, Y: 15}, 0)
	if b.click.Pressed() {
		t.Fatal("Pressed() = true after Release, want false")
	}
}

func TestButtonLabelAccessor(t *testing.T) {
	b := NewButton(nil, "Hi")
	if got := b.Label().Text(); got != "Hi" {
		t.Fatalf("Label().Text() = %q, want %q", got, "Hi")
	}
}

func TestToggleButtonTogglesAndFiresOnChanged(t *testing.T) {
	tb := NewToggleButton(buttonFace(t), "Toggle")
	tb.SetWidth(80)
	tb.SetHeight(30)

	var got []bool
	tb.OnChanged(func(v bool) { got = append(got, v) })

	r := input.NewRouter()
	r.SetRoot(tb)
	layoutButton(tb, render.Rect{X: 0, Y: 0, W: 80, H: 30})

	pos := render.Point{X: 40, Y: 15}
	r.PointerButton(input.ButtonLeft, true, pos, 0)
	r.PointerButton(input.ButtonLeft, false, pos, 0)

	if !tb.Checked() {
		t.Fatal("Checked() = false after click, want true")
	}
	if len(got) != 1 || got[0] != true {
		t.Fatalf("OnChanged calls = %v, want [true]", got)
	}

	r.PointerButton(input.ButtonLeft, true, pos, 0)
	r.PointerButton(input.ButtonLeft, false, pos, 0)

	if tb.Checked() {
		t.Fatal("Checked() = true after second click, want false")
	}
	if len(got) != 2 || got[1] != false {
		t.Fatalf("OnChanged calls = %v, want [true false]", got)
	}
}

func TestToggleButtonSetCheckedNoCallback(t *testing.T) {
	tb := NewToggleButton(buttonFace(t), "Toggle")

	fired := false
	tb.OnChanged(func(bool) { fired = true })

	tb.SetChecked(true)
	if !tb.Checked() {
		t.Fatal("Checked() = false after SetChecked(true)")
	}
	if fired {
		t.Fatal("SetChecked(true) fired OnChanged; normative: programmatic SetChecked is silent")
	}

	tb.SetChecked(true) // same value: no-op
	if fired {
		t.Fatal("no-op SetChecked fired OnChanged")
	}

	tb.SetChecked(false)
	if tb.Checked() {
		t.Fatal("Checked() = true after SetChecked(false)")
	}
	if fired {
		t.Fatal("SetChecked(false) fired OnChanged")
	}
}

func TestToggleButtonCheckedRendersAccent(t *testing.T) {
	tb := NewToggleButton(nil, "T")
	if tb.accent {
		t.Fatal("accent = true before checking, want false")
	}

	tb.SetChecked(true)
	if !tb.accent {
		t.Fatal("accent = false after SetChecked(true), want true (checked renders accent-on)")
	}

	tb.SetChecked(false)
	if tb.accent {
		t.Fatal("accent = true after SetChecked(false), want false")
	}
}

func TestToggleButtonOnClickPanics(t *testing.T) {
	tb := NewToggleButton(nil, "T")
	defer func() {
		if recover() == nil {
			t.Fatal("ToggleButton.OnClick did not panic, want it to (would clobber internal toggle wiring)")
		}
	}()
	tb.OnClick(func() {})
}

func TestToggleButtonSetAccentPanics(t *testing.T) {
	tb := NewToggleButton(nil, "T")
	defer func() {
		if recover() == nil {
			t.Fatal("ToggleButton.SetAccent did not panic, want it to (would desync chrome from Checked())")
		}
	}()
	tb.SetAccent(true)
}

func TestButtonShapeDefaultIsRect(t *testing.T) {
	b := NewButton(nil, "X")
	if b.shape != ShapeRect {
		t.Fatalf("default shape = %v, want ShapeRect (zero value)", b.shape)
	}
	tb := NewToggleButton(nil, "T")
	if tb.shape != ShapeRect {
		t.Fatalf("ToggleButton default shape = %v, want ShapeRect", tb.shape)
	}
}

// TestButtonCircleMeasureContentReturnsSquare pins spec §3's circle-aspect
// rule: MeasureContent forces a SQUARE (side = max(paddedWidth,
// paddedHeight)) for ShapeCircle, so the circle (radius = min(W,H)/2, see
// shapeRadius) fully encloses a label that isn't itself square. A plain
// ShapeRect button with the same label is used as the width oracle: the
// circle's square side must equal that rect's (necessarily wider-than-tall)
// width, and — for the test to be meaningful at all — that rect must
// actually be wider than tall.
func TestButtonCircleMeasureContentReturnsSquare(t *testing.T) {
	circle := NewButton(buttonFace(t), "Wide label text").SetShape(ShapeCircle)
	got := circle.MeasureContent(render.Size{W: 1000, H: 1000})
	if got.W != got.H {
		t.Fatalf("ShapeCircle MeasureContent = %+v, want a square (W == H)", got)
	}

	rectB := NewButton(buttonFace(t), "Wide label text")
	rectSize := rectB.MeasureContent(render.Size{W: 1000, H: 1000})
	if rectSize.W <= rectSize.H {
		t.Fatalf("test setup: rect size = %+v, want W > H for this test to be meaningful", rectSize)
	}
	if got.W != rectSize.W {
		t.Fatalf("circle square side = %v, want max(paddedW, paddedH) = rect W %v", got.W, rectSize.W)
	}
}

// TestButtonShapeRadius pins shapeRadius's per-shape formula: 0 for
// ShapeRect (never actually consulted by the rect Render path), bounds.H/2
// for ShapePill regardless of aspect, and min(bounds.W, bounds.H)/2 for
// ShapeCircle for both a wide and a tall bounds rect.
func TestButtonShapeRadius(t *testing.T) {
	b := &Button{}

	cases := []struct {
		shape  ButtonShape
		bounds render.Rect
		want   float32
	}{
		{ShapeRect, render.Rect{W: 100, H: 30}, 0},
		{ShapePill, render.Rect{W: 100, H: 30}, 15},
		{ShapeCircle, render.Rect{W: 100, H: 30}, 15}, // min(100,30)/2
		{ShapeCircle, render.Rect{W: 30, H: 100}, 15}, // min(30,100)/2
	}
	for _, c := range cases {
		b.shape = c.shape
		if got := b.shapeRadius(c.bounds); got != c.want {
			t.Errorf("shape=%v bounds=%+v: shapeRadius = %v, want %v", c.shape, c.bounds, got, c.want)
		}
	}
}

// TestButtonSetShapeInvalidatesMeasureOnlyOnChange pins SetShape's
// changed-check: switching to a genuinely different shape invalidates
// layout (ShapeCircle's square aspect, in particular, changes desired
// size), but re-setting the SAME shape is a silent no-op, matching
// Border.SetBorder's changed-check convention.
func TestButtonSetShapeInvalidatesMeasureOnlyOnChange(t *testing.T) {
	b := NewButton(buttonFace(t), "X")
	b.SetWidth(80)
	b.SetHeight(30)
	layoutButton(b, render.Rect{X: 0, Y: 0, W: 80, H: 30})

	if b.NeedsLayout() {
		t.Fatal("clean after initial layout")
	}

	b.SetShape(ShapeRect) // no-op: already ShapeRect
	if b.NeedsLayout() {
		t.Fatal("SetShape to the SAME shape invalidated layout, want a no-op")
	}

	b.SetShape(ShapePill)
	if !b.NeedsLayout() {
		t.Fatal("SetShape to a DIFFERENT shape did not invalidate layout")
	}
}

// TestToggleButtonSetShapeReturnsToggleButton is a compile-time-flavored
// regression: ToggleButton.SetShape is shadowed (not left promoted) so it
// returns *ToggleButton, not the promoted Button.SetShape's *Button — if
// this test compiles at all, the chain (.SetShape(...).OnChanged(...))
// proves the return type is right; the runtime assertion below confirms it
// also actually set the shape on the embedded Button.
func TestToggleButtonSetShapeReturnsToggleButton(t *testing.T) {
	tb := NewToggleButton(nil, "T")
	tb.SetShape(ShapePill).OnChanged(func(bool) {})
	if tb.shape != ShapePill {
		t.Fatalf("shape = %v, want ShapePill", tb.shape)
	}
}

// TestButtonRectShapeRenderMatchesDrawRaised is a regression pinning that a
// default (ShapeRect) button's Render is BYTE-IDENTICAL to before
// ButtonShape existed: exactly drawRaised's signature (1 face fill + 4
// outer edges + 4 inner edges = 9 FillRect calls, first call = the face
// fill over the full bounds), never a FillRoundedRect/StrokeRoundedRect
// call (recordRenderer's no-op stubs for those would silently swallow a
// leak into the rect path without this call-count check).
func TestButtonRectShapeRenderMatchesDrawRaised(t *testing.T) {
	b := NewButton(buttonFace(t), "X")
	b.SetWidth(80)
	b.SetHeight(30)
	layoutButton(b, render.Rect{X: 0, Y: 0, W: 80, H: 30})

	rr := &recordRenderer{}
	b.Render(rr)

	if len(rr.fills) != 9 {
		t.Fatalf("ShapeRect Render emitted %d FillRect calls, want 9 (drawRaised's signature)", len(rr.fills))
	}
	if got := rr.fills[0]; got.rect != b.Bounds() || got.color != b.colors.ButtonFace {
		t.Fatalf("first FillRect = %+v, want face fill %v over %v", got, b.colors.ButtonFace, b.Bounds())
	}
}

func TestButtonStateColorsAccentDisabledUsesAccentDisabledFill(t *testing.T) {
	b := NewButton(nil, "X").SetAccent(true).SetEnabled(false)
	fill, stroke, label := b.stateColors()
	if fill != b.colors.AccentDisabled {
		t.Fatalf("accent+disabled fill = %v, want AccentDisabled %v", fill, b.colors.AccentDisabled)
	}
	if stroke.A != 0 {
		t.Fatalf("accent+disabled stroke = %v, want no stroke (zero alpha)", stroke)
	}
	if label != b.colors.TextDisabled {
		t.Fatalf("accent+disabled label = %v, want TextDisabled %v", label, b.colors.TextDisabled)
	}
}

func TestToggleButtonLabelParentSurvivesEmbedding(t *testing.T) {
	// Regression for the copy-parent trap initButton's doc comment
	// describes: the label's parent must be &tb.Button (the ToggleButton's
	// own embedded field), not some discarded intermediate *Button, or
	// InvalidateMeasure climbing from the label would silently stop short
	// of any real container.
	tb := NewToggleButton(buttonFace(t), "Toggle")
	stack := NewStackPanel(Horizontal).Add(tb)
	layoutButton(stack, render.Rect{X: 0, Y: 0, W: 200, H: 40})

	if stack.NeedsLayout() {
		t.Fatal("clean after initial layout")
	}
	tb.Label().SetText("a much wider label than before")
	if !stack.NeedsLayout() {
		t.Fatal("SetText on the toggle button's label must invalidate the stack panel's layout")
	}
}
