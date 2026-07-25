package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
)

func TestToggleSwitchClickTogglesViaRouter(t *testing.T) {
	s := NewToggleSwitch()

	var got []bool
	s.OnChanged(func(v bool) { got = append(got, v) })

	r := input.NewRouter()
	r.SetRoot(s)
	layoutButton(s, render.Rect{X: 0, Y: 0, W: 40, H: 20})

	pos := render.Point{X: 20, Y: 10}
	r.PointerButton(input.ButtonLeft, true, pos, 0)
	r.PointerButton(input.ButtonLeft, false, pos, 0)

	if !s.Checked() {
		t.Fatal("Checked() = false after click, want true")
	}
	if len(got) != 1 || got[0] != true {
		t.Fatalf("OnChanged calls = %v, want [true]", got)
	}

	r.PointerButton(input.ButtonLeft, true, pos, 0)
	r.PointerButton(input.ButtonLeft, false, pos, 0)

	if s.Checked() {
		t.Fatal("Checked() = true after second click, want false")
	}
	if len(got) != 2 || got[1] != false {
		t.Fatalf("OnChanged calls = %v, want [true false]", got)
	}
}

func TestToggleSwitchSpaceTogglesWhenFocused(t *testing.T) {
	s := NewToggleSwitch()

	clicks := 0
	s.OnChanged(func(bool) { clicks++ })

	r := input.NewRouter()
	r.SetRoot(s)
	layoutButton(s, render.Rect{X: 0, Y: 0, W: 40, H: 20})

	r.Focus(s)
	r.KeyDown(input.KeySpace, 0, 0)
	if clicks != 1 || !s.Checked() {
		t.Fatalf("Space while focused: clicks=%d checked=%v, want 1/true", clicks, s.Checked())
	}

	r.KeyDown(input.KeyEnter, 0, 0)
	if clicks != 2 || s.Checked() {
		t.Fatalf("Enter while focused: clicks=%d checked=%v, want 2/false", clicks, s.Checked())
	}
}

func TestToggleSwitchSetCheckedSilent(t *testing.T) {
	s := NewToggleSwitch()

	fired := false
	s.OnChanged(func(bool) { fired = true })

	s.SetChecked(true)
	if !s.Checked() {
		t.Fatal("Checked() = false after SetChecked(true)")
	}
	if fired {
		t.Fatal("SetChecked(true) fired OnChanged; normative: programmatic SetChecked is silent")
	}

	s.SetChecked(true) // no-op
	if fired {
		t.Fatal("no-op SetChecked fired OnChanged")
	}

	s.SetChecked(false)
	if s.Checked() {
		t.Fatal("Checked() = true after SetChecked(false)")
	}
	if fired {
		t.Fatal("SetChecked(false) fired OnChanged")
	}
}

func TestToggleSwitchDisabledIgnoresPointerAndFocus(t *testing.T) {
	s := NewToggleSwitch().SetEnabled(false)
	layoutButton(s, render.Rect{X: 0, Y: 0, W: 40, H: 20})

	if s.AcceptsFocus() {
		t.Fatal("AcceptsFocus() = true for a disabled toggle switch, want false")
	}

	changed := false
	s.OnChanged(func(bool) { changed = true })

	press := &input.PointerEvent{Action: input.Press, Pos: render.Point{X: 20, Y: 10}}
	s.OnPointer(press)
	if press.Handled {
		t.Fatal("Press on a disabled toggle switch set Handled = true, want false")
	}
	release := &input.PointerEvent{Action: input.Release, Pos: render.Point{X: 20, Y: 10}}
	s.OnPointer(release)
	if release.Handled {
		t.Fatal("Release on a disabled toggle switch set Handled = true, want false")
	}
	if changed || s.Checked() {
		t.Fatal("disabled toggle switch toggled")
	}
}

func TestToggleSwitchFocusRingTracked(t *testing.T) {
	s := NewToggleSwitch()
	if s.focused {
		t.Fatal("focused = true before any focus change, want false")
	}
	s.OnFocusChanged(true)
	if !s.focused {
		t.Fatal("focused = false after OnFocusChanged(true), want true")
	}
	s.OnFocusChanged(false)
	if s.focused {
		t.Fatal("focused = true after OnFocusChanged(false), want false")
	}
}

func TestToggleSwitchHoverFillDiffersFromRestFill(t *testing.T) {
	s := NewToggleSwitch()
	restFill, _, _ := s.stateColors()
	s.click.hover = true
	hoverFill, _, _ := s.stateColors()
	if hoverFill == restFill {
		t.Fatalf("hover fill == rest fill (%v), want ControlFillHover to differ from ControlFill", hoverFill)
	}
}

func TestToggleSwitchMeasuresToFixedPillSize(t *testing.T) {
	s := NewToggleSwitch()
	core.MeasureWidget(s, render.Size{W: 1000, H: 1000})
	d := core.DesiredSizeOf(s)
	if d.W != 40 || d.H != 20 {
		t.Fatalf("DesiredSize() = %v, want {40 20}", d)
	}
}
