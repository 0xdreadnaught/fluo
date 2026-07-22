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
