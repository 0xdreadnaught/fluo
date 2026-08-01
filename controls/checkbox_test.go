package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
)

// --- CheckBox ---

func TestCheckBoxClickTogglesViaRouter(t *testing.T) {
	c := NewCheckBox(buttonFace(t), "Agree")
	c.SetWidth(80)
	c.SetHeight(20)

	var got []bool
	c.OnChanged(func(v bool) { got = append(got, v) })

	r := input.NewRouter()
	r.SetRoot(c)
	layoutButton(c, render.Rect{X: 0, Y: 0, W: 80, H: 20})

	pos := render.Point{X: 9, Y: 10} // inside the 18x18 glyph box
	r.PointerButton(input.ButtonLeft, true, pos, 0)
	r.PointerButton(input.ButtonLeft, false, pos, 0)

	if !c.Checked() {
		t.Fatal("Checked() = false after click, want true")
	}
	if len(got) != 1 || got[0] != true {
		t.Fatalf("OnChanged calls = %v, want [true]", got)
	}

	r.PointerButton(input.ButtonLeft, true, pos, 0)
	r.PointerButton(input.ButtonLeft, false, pos, 0)

	if c.Checked() {
		t.Fatal("Checked() = true after second click, want false")
	}
	if len(got) != 2 || got[1] != false {
		t.Fatalf("OnChanged calls = %v, want [true false]", got)
	}
}

func TestCheckBoxClickOnLabelAlsoToggles(t *testing.T) {
	// The whole composite (glyph + label) is the clickable area, not just
	// the 18x18 box — matching ordinary checkbox UX.
	c := NewCheckBox(buttonFace(t), "Agree to terms")
	c.SetWidth(150)
	c.SetHeight(20)

	r := input.NewRouter()
	r.SetRoot(c)
	layoutButton(c, render.Rect{X: 0, Y: 0, W: 150, H: 20})

	pos := render.Point{X: 100, Y: 10} // well past the box, over the label text
	r.PointerButton(input.ButtonLeft, true, pos, 0)
	r.PointerButton(input.ButtonLeft, false, pos, 0)

	if !c.Checked() {
		t.Fatal("clicking the label did not toggle the checkbox")
	}
}

func TestCheckBoxSpaceTogglesWhenFocused(t *testing.T) {
	c := NewCheckBox(buttonFace(t), "Agree")
	c.SetWidth(80)
	c.SetHeight(20)

	clicks := 0
	c.OnChanged(func(bool) { clicks++ })

	r := input.NewRouter()
	r.SetRoot(c)
	layoutButton(c, render.Rect{X: 0, Y: 0, W: 80, H: 20})

	r.Focus(c)
	r.KeyDown(input.KeySpace, 0, 0)

	if clicks != 1 || !c.Checked() {
		t.Fatalf("Space while focused: clicks=%d checked=%v, want 1/true", clicks, c.Checked())
	}

	r.KeyDown(input.KeyEnter, 0, 0)
	if clicks != 2 || c.Checked() {
		t.Fatalf("Enter while focused: clicks=%d checked=%v, want 2/false", clicks, c.Checked())
	}
}

func TestCheckBoxSetCheckedSilent(t *testing.T) {
	c := NewCheckBox(nil, "Agree")

	fired := false
	c.OnChanged(func(bool) { fired = true })

	c.SetChecked(true)
	if !c.Checked() {
		t.Fatal("Checked() = false after SetChecked(true)")
	}
	if fired {
		t.Fatal("SetChecked(true) fired OnChanged; normative: programmatic SetChecked is silent")
	}

	c.SetChecked(true) // same value: no-op
	if fired {
		t.Fatal("no-op SetChecked fired OnChanged")
	}

	c.SetChecked(false)
	if c.Checked() {
		t.Fatal("Checked() = true after SetChecked(false)")
	}
	if fired {
		t.Fatal("SetChecked(false) fired OnChanged")
	}
}

func TestCheckBoxDisabledIgnoresPointerAndFocus(t *testing.T) {
	c := NewCheckBox(buttonFace(t), "Agree").SetEnabled(false)
	c.SetWidth(80)
	c.SetHeight(20)
	layoutButton(c, render.Rect{X: 0, Y: 0, W: 80, H: 20})

	if c.AcceptsFocus() {
		t.Fatal("AcceptsFocus() = true for a disabled checkbox, want false")
	}

	changed := false
	c.OnChanged(func(bool) { changed = true })

	press := &input.PointerEvent{Action: input.Press, Pos: render.Point{X: 9, Y: 10}}
	c.OnPointer(press)
	if press.Handled {
		t.Fatal("Press on a disabled checkbox set Handled = true, want false")
	}

	release := &input.PointerEvent{Action: input.Release, Pos: render.Point{X: 9, Y: 10}}
	c.OnPointer(release)
	if release.Handled {
		t.Fatal("Release on a disabled checkbox set Handled = true, want false")
	}
	if changed || c.Checked() {
		t.Fatal("disabled checkbox toggled")
	}
}

func TestCheckBoxFocusRingTracked(t *testing.T) {
	c := NewCheckBox(nil, "Agree")
	if c.focused {
		t.Fatal("focused = true before any focus change, want false")
	}
	c.OnFocusChanged(true)
	if !c.focused {
		t.Fatal("focused = false after OnFocusChanged(true), want true")
	}
	c.OnFocusChanged(false)
	if c.focused {
		t.Fatal("focused = true after OnFocusChanged(false), want false")
	}
}

// TestCheckBoxHoverDoesNotChangeRenderedChrome pins the classic-checkbox
// spec Render's doc comment states: the box is a WindowWell sunken well that
// does NOT react to hover (unlike Button, whose face lightens). Any hover
// tint creeping back into the box would break the classic look.
func TestCheckBoxHoverDoesNotChangeRenderedChrome(t *testing.T) {
	c := NewCheckBox(nil, "")
	layoutButton(c, render.Rect{X: 0, Y: 0, W: 80, H: 20})

	rest := &recordRenderer{}
	c.Render(rest)
	// drawSunken's face fill plus its eight 1px bevel edges — guards this
	// comparison against passing vacuously on an empty recording.
	if len(rest.fills) != 9 {
		t.Fatalf("Render emitted %d FillRect calls, want 9 (drawSunken's signature)", len(rest.fills))
	}

	c.click.hover = true
	hover := &recordRenderer{}
	c.Render(hover)

	if len(rest.fills) != len(hover.fills) {
		t.Fatalf("hovered Render emitted %d FillRect calls, want %d (same as rest)", len(hover.fills), len(rest.fills))
	}
	for i := range rest.fills {
		if rest.fills[i] != hover.fills[i] {
			t.Fatalf("fill %d differs on hover: rest %+v, hover %+v", i, rest.fills[i], hover.fills[i])
		}
	}
}

func TestCheckBoxEmptyLabelMeasuresToGlyphOnly(t *testing.T) {
	// Normative: no label text means no PaddingM gap reserved either — the
	// golden (six bare glyphs) depends on this.
	c := NewCheckBox(buttonFace(t), "")
	core.MeasureWidget(c, render.Size{W: 1000, H: 1000})
	d := core.DesiredSizeOf(c)
	if d.W != glyphBoxSize {
		t.Fatalf("DesiredSize().W = %v, want %v (glyph box only, no label/gap)", d.W, glyphBoxSize)
	}
}

// --- RadioButton / RadioGroup ---

func TestRadioGroupExclusivity(t *testing.T) {
	a := NewRadioButton(buttonFace(t), "A")
	b := NewRadioButton(buttonFace(t), "B")
	group := NewRadioGroup()
	group.Add(a).Add(b)

	var groupChanges []int
	group.OnChanged(func(i int) { groupChanges = append(groupChanges, i) })

	a.SetChecked(true) // seed: A starts checked (silent, no callback)
	if groupChanges != nil {
		t.Fatalf("SetChecked fired group OnChanged: %v", groupChanges)
	}

	host := NewStackPanel(Horizontal).Add(a, b)
	r := input.NewRouter()
	r.SetRoot(host)
	layoutButton(host, render.Rect{X: 0, Y: 0, W: 300, H: 20})

	bBox := core.BoundsOf(b)
	pos := render.Point{X: bBox.X + 9, Y: bBox.Y + 9}
	r.PointerButton(input.ButtonLeft, true, pos, 0)
	r.PointerButton(input.ButtonLeft, false, pos, 0)

	if a.Checked() {
		t.Fatal("checking B did not uncheck A")
	}
	if !b.Checked() {
		t.Fatal("B did not become checked")
	}
	if len(groupChanges) != 1 || groupChanges[0] != 1 {
		t.Fatalf("group OnChanged calls = %v, want [1] (B's index)", groupChanges)
	}

	// Re-clicking the already-checked radio must stay checked and fire no
	// callback at all.
	r.PointerButton(input.ButtonLeft, true, pos, 0)
	r.PointerButton(input.ButtonLeft, false, pos, 0)

	if !b.Checked() {
		t.Fatal("B became unchecked after re-clicking, want it to stay checked")
	}
	if len(groupChanges) != 1 {
		t.Fatalf("group OnChanged fired again on re-click: %v, want still [1]", groupChanges)
	}
}

func TestRadioButtonSetCheckedRespectsGroupExclusivity(t *testing.T) {
	a := NewRadioButton(nil, "A")
	b := NewRadioButton(nil, "B")
	group := NewRadioGroup().Add(a).Add(b)

	var changes []int
	group.OnChanged(func(i int) { changes = append(changes, i) })

	a.SetChecked(true)
	b.SetChecked(true) // programmatic: must silently uncheck A

	if a.Checked() {
		t.Fatal("SetChecked(true) on B did not uncheck A")
	}
	if !b.Checked() {
		t.Fatal("B not checked after SetChecked(true)")
	}
	if changes != nil {
		t.Fatalf("SetChecked fired group OnChanged: %v, want none (silent)", changes)
	}
}

func TestRadioGroupSelectedIndexGetterAndSetterAreSilent(t *testing.T) {
	a := NewRadioButton(nil, "A")
	b := NewRadioButton(nil, "B")
	c := NewRadioButton(nil, "C")
	group := NewRadioGroup().Add(a).Add(b).Add(c)

	var groupChanges []int
	group.OnChanged(func(i int) { groupChanges = append(groupChanges, i) })
	var aChanges, bChanges, cChanges []bool
	a.OnChanged(func(v bool) { aChanges = append(aChanges, v) })
	b.OnChanged(func(v bool) { bChanges = append(bChanges, v) })
	c.OnChanged(func(v bool) { cChanges = append(cChanges, v) })

	if group.SelectedIndex() != -1 {
		t.Fatalf("SelectedIndex() = %d on a fresh group, want -1 (none checked)", group.SelectedIndex())
	}

	group.SetSelectedIndex(1)
	if group.SelectedIndex() != 1 {
		t.Fatalf("SelectedIndex() = %d after SetSelectedIndex(1), want 1", group.SelectedIndex())
	}
	if a.Checked() || !b.Checked() || c.Checked() {
		t.Fatalf("checked states = %v %v %v after SetSelectedIndex(1), want false true false", a.Checked(), b.Checked(), c.Checked())
	}

	// Switching selection respects exclusivity.
	group.SetSelectedIndex(2)
	if a.Checked() || b.Checked() || !c.Checked() {
		t.Fatalf("checked states = %v %v %v after SetSelectedIndex(2), want false false true", a.Checked(), b.Checked(), c.Checked())
	}

	// -1 clears every member.
	group.SetSelectedIndex(-1)
	if a.Checked() || b.Checked() || c.Checked() {
		t.Fatal("a member still checked after SetSelectedIndex(-1), want all clear")
	}
	if group.SelectedIndex() != -1 {
		t.Fatalf("SelectedIndex() = %d after SetSelectedIndex(-1), want -1", group.SelectedIndex())
	}

	// Out-of-range clamps rather than panicking or going out of bounds.
	group.SetSelectedIndex(99)
	if group.SelectedIndex() != 2 {
		t.Fatalf("SelectedIndex() = %d after SetSelectedIndex(99), want clamp to 2 (last member)", group.SelectedIndex())
	}
	group.SetSelectedIndex(-99)
	if group.SelectedIndex() != -1 {
		t.Fatalf("SelectedIndex() = %d after SetSelectedIndex(-99), want clamp to -1", group.SelectedIndex())
	}

	if groupChanges != nil {
		t.Fatalf("group OnChanged fired: %v, want none (SetSelectedIndex is silent)", groupChanges)
	}
	if aChanges != nil || bChanges != nil || cChanges != nil {
		t.Fatalf("member OnChanged fired: a=%v b=%v c=%v, want none (SetSelectedIndex is silent)", aChanges, bChanges, cChanges)
	}
}

func TestRadioGroupSelectedIndexOnEmptyGroup(t *testing.T) {
	group := NewRadioGroup()
	if group.SelectedIndex() != -1 {
		t.Fatalf("SelectedIndex() = %d on empty group, want -1", group.SelectedIndex())
	}
	group.SetSelectedIndex(5) // must not panic; clamps to -1 on an empty group
	if group.SelectedIndex() != -1 {
		t.Fatalf("SelectedIndex() = %d after SetSelectedIndex(5) on empty group, want -1", group.SelectedIndex())
	}
}

func TestRadioButtonDisabledIgnoresPointerAndFocus(t *testing.T) {
	rb := NewRadioButton(buttonFace(t), "A").SetEnabled(false)
	rb.SetWidth(80)
	rb.SetHeight(20)
	layoutButton(rb, render.Rect{X: 0, Y: 0, W: 80, H: 20})

	if rb.AcceptsFocus() {
		t.Fatal("AcceptsFocus() = true for a disabled radio button, want false")
	}

	press := &input.PointerEvent{Action: input.Press, Pos: render.Point{X: 9, Y: 10}}
	rb.OnPointer(press)
	if press.Handled {
		t.Fatal("Press on a disabled radio button set Handled = true, want false")
	}
	if rb.Checked() {
		t.Fatal("disabled radio button became checked")
	}
}

func TestRadioButtonFocusRingTracked(t *testing.T) {
	rb := NewRadioButton(nil, "A")
	rb.OnFocusChanged(true)
	if !rb.focused {
		t.Fatal("focused = false after OnFocusChanged(true), want true")
	}
	rb.OnFocusChanged(false)
	if rb.focused {
		t.Fatal("focused = true after OnFocusChanged(false), want false")
	}
}

func TestRadioButtonSpaceActivatesWhenFocused(t *testing.T) {
	rb := NewRadioButton(buttonFace(t), "A")
	rb.SetWidth(80)
	rb.SetHeight(20)

	r := input.NewRouter()
	r.SetRoot(rb)
	layoutButton(rb, render.Rect{X: 0, Y: 0, W: 80, H: 20})

	r.Focus(rb)
	r.KeyDown(input.KeySpace, 0, 0)

	if !rb.Checked() {
		t.Fatal("Space while focused did not check the radio button")
	}

	// Space again on an already-checked radio: stays checked.
	r.KeyDown(input.KeySpace, 0, 0)
	if !rb.Checked() {
		t.Fatal("second Space unchecked the radio button, want it to stay checked")
	}
}
