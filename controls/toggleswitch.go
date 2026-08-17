package controls

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/theme"
	"github.com/0xdreadnaught/fluo/timers"
)

// Fixed track/knob metrics for ToggleSwitch, per the Phase 5 Task 4 visuals
// spec, restyled square for v0.2 classic: a 40x20 track with a 12px square
// knob inset 4px from whichever side it sits on.
const (
	switchWidth  float32 = 40
	switchHeight float32 = 20

	thumbSize  float32 = 12
	thumbInset float32 = 4
)

// ToggleSwitch is a clickable, focusable, token-styled 40x20 track-and-knob
// toggle (no label — unlike CheckBox/RadioButton, NewToggleSwitch takes no
// face or label argument). Like CheckBox and ToggleButton, it is
// ClickBehavior-driven and follows the Checked/SetChecked/OnChanged/SetEnabled
// convention: SetChecked is a silent programmatic setter, OnChanged fires
// only for user-driven changes (click or Space/Enter while focused).
//
// Visuals (normative): a sunken track — ButtonFace when off, Highlight when
// on — with a small raised ButtonFace knob inset from the LEFT edge when off
// and from the RIGHT edge when on. Hover and pressed deliberately leave both
// untinted; only the checked state changes any color (see Render).
type ToggleSwitch struct {
	core.Element

	click ClickBehavior

	checked   bool
	enabled   bool
	focused   bool
	onChanged func(bool)

	colors  theme.ColorTokens
	metrics theme.MetricTokens

	animated   bool
	timerQueue *timers.Queue
	fillAnim   *colorAnim
}

// NewToggleSwitch returns an off (unchecked), enabled ToggleSwitch.
func NewToggleSwitch() *ToggleSwitch {
	s := &ToggleSwitch{}
	th := theme.Active()
	s.enabled = true
	s.colors = th.Color
	s.metrics = th.Metric

	s.click.OnClick = func() {
		s.checked = !s.checked
		if s.onChanged != nil {
			s.onChanged(s.checked)
		}
	}
	return s
}

// Checked reports the current on/off state.
func (s *ToggleSwitch) Checked() bool { return s.checked }

// SetChecked sets the on/off state programmatically. Normative: unlike a
// click, SetChecked does NOT fire OnChanged (matching CheckBox/
// ToggleButton's convention) and is a no-op when v already matches the
// current state. Fluo's uniform contract, restated: programmatic setters
// are silent; OnChanged reports only user-driven changes.
func (s *ToggleSwitch) SetChecked(v bool) *ToggleSwitch {
	if s.checked == v {
		return s
	}
	s.checked = v
	return s
}

// OnChanged sets the callback fired with the new checked value whenever the
// user flips the switch (click or Space/Enter) — never for a programmatic
// SetChecked (fluo's uniform contract: programmatic setters are silent;
// OnChanged reports only user-driven changes). Replaces any previously set
// callback; a nil fn is a valid, silent no-op.
func (s *ToggleSwitch) OnChanged(fn func(bool)) *ToggleSwitch {
	s.onChanged = fn
	return s
}

// SetEnabled toggles whether the switch accepts focus and pointer/keyboard
// input.
func (s *ToggleSwitch) SetEnabled(v bool) *ToggleSwitch {
	s.enabled = v
	return s
}

func (s *ToggleSwitch) SetAnimated(v bool) *ToggleSwitch {
	s.animated = v
	return s
}

func (s *ToggleSwitch) SetTimers(q *timers.Queue) *ToggleSwitch {
	s.timerQueue = q
	return s
}

func (s *ToggleSwitch) animatedFill(fill render.Color) render.Color {
	if !s.animated || s.timerQueue == nil {
		return fill
	}
	if s.fillAnim == nil {
		s.fillAnim = newColorAnim(fill)
	} else {
		s.fillAnim.SetTarget(s.timerQueue, fill)
	}
	return s.fillAnim.Current()
}

// MeasureContent always returns the fixed 40x20 pill size: ToggleSwitch has
// no label and no other content to size around.
func (s *ToggleSwitch) MeasureContent(available render.Size) render.Size {
	return render.Size{W: switchWidth, H: switchHeight}
}

// ArrangeContent is a no-op: ToggleSwitch has no children to position.
func (s *ToggleSwitch) ArrangeContent(bounds render.Rect) {}

// Children returns nil: ToggleSwitch is a leaf widget.
func (s *ToggleSwitch) Children() []core.Widget { return nil }

func (s *ToggleSwitch) Render(r render.Renderer) {
	bounds := s.Bounds()
	c := s.colors

	if s.metrics.BevelWidth == 0 {
		s.renderFlat(r, bounds)
		return
	}

	trackFill := c.ButtonFace
	if s.checked {
		trackFill = c.Highlight
	}
	drawSunken(r, bounds, trackFill, c)

	thumbX := bounds.X + thumbInset
	if s.checked {
		thumbX = bounds.Right() - thumbInset - thumbSize
	}
	thumbY := bounds.Y + (bounds.H-thumbSize)/2
	drawRaised(r, render.Rect{X: thumbX, Y: thumbY, W: thumbSize, H: thumbSize}, c.ButtonFace, c)
}

func (s *ToggleSwitch) renderFlat(r render.Renderer, bounds render.Rect) {
	c := s.colors
	trackRadius := bounds.H / 2

	trackFill := c.ButtonShadow
	if s.checked {
		trackFill = c.Highlight
	}
	r.FillRoundedRect(bounds, trackRadius, trackFill)

	var knobFill render.Color
	if s.checked {
		knobFill = c.HighlightText
	} else if s.click.Hover() {
		knobFill = c.WindowText
	} else {
		knobFill = c.GrayText
	}
	knobFill = s.animatedFill(knobFill)

	thumbX := bounds.X + thumbInset
	if s.checked {
		thumbX = bounds.Right() - thumbInset - thumbSize
	}
	thumbY := bounds.Y + (bounds.H-thumbSize)/2
	thumbRadius := thumbSize / 2
	r.FillRoundedRect(render.Rect{X: thumbX, Y: thumbY, W: thumbSize, H: thumbSize}, thumbRadius, knobFill)
}

func (s *ToggleSwitch) RenderOverlay(r render.Renderer) {
	if !s.focused {
		return
	}
	if s.metrics.BevelWidth == 0 {
		bounds := s.Bounds()
		radius := bounds.H / 2
		r.StrokeRoundedRect(bounds.Inset(render.Uniform(-s.metrics.FocusStrokeWidth)),
			radius, s.metrics.FocusStrokeWidth, s.colors.Highlight)
		return
	}
	drawFocusRing(r, s.Bounds(), s.colors)
}

// AcceptsFocus implements input.Focusable: a disabled switch never accepts
// focus.
func (s *ToggleSwitch) AcceptsFocus() bool {
	return s.enabled
}

// OnFocusChanged implements input.FocusHandler.
func (s *ToggleSwitch) OnFocusChanged(focused bool) {
	s.focused = focused
}

// OnPointer implements input.PointerHandler, delegating to ClickBehavior
// while enabled (disabled ignores pointer input outright — e.Handled is
// left false so the event keeps bubbling).
func (s *ToggleSwitch) OnPointer(e *input.PointerEvent) {
	if !s.enabled {
		return
	}
	s.click.HandlePointer(e, s)
}

// OnKey implements input.KeyHandler: Space or Enter, on Press, flips the
// switch (fires OnChanged) and marks the event handled.
func (s *ToggleSwitch) OnKey(e *input.KeyEvent) {
	if !s.enabled || e.Action != input.Press {
		return
	}
	if e.Key == input.KeySpace || e.Key == input.KeyEnter {
		s.click.Activate()
		e.Handled = true
	}
}
