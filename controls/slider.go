package controls

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/theme"
)

// Fixed geometry for Slider, per the Phase 5 Task 7 visuals spec: a 4px-tall
// rounded track (radius = height/2, a stadium cross-section) with a 16x16
// circular thumb (radius 8, i.e. half its own diameter — the same
// "square rect + radius == half side" trick ToggleSwitch's thumb and
// RadioButton's outer circle use to draw a circle via FillRoundedRect).
// sliderDesiredWidth/Height are the fixed MeasureContent size {160, 24}: an
// explicit SetWidth/SetHeight (inherited from core.Element) overrides this
// through core.MeasureWidget's normal explicit-size precedence, exactly as
// for every other fixed-size control in this package (ToggleSwitch, TextBox).
const (
	sliderTrackHeight   float32 = 4
	sliderThumbSize     float32 = 16
	sliderThumbRadius   float32 = sliderThumbSize / 2
	sliderDesiredWidth  float32 = 160
	sliderDesiredHeight float32 = 24
)

// Slider is a horizontal, focusable, token-styled continuous value picker
// over [Min, Max] (defaulting to [0, 1]). Value is reported and set as a
// plain float32; SetRange/SetValue both clamp into the current range.
//
// Normative geometry (see the const block above): the accent-filled portion
// of the track runs from the track's left edge to the thumb's CENTER x, and
// the thumb's center itself is only ever placed within the "usable track
// span" [thumbRadius, width-thumbRadius] — a thumb centered at x=thumbRadius
// sits flush against the track's left edge (Value==Min), and one centered at
// x=width-thumbRadius sits flush against the right edge (Value==Max). This
// inset is why proportion() and its inverse valueFromLocalX() both divide by
// (width - 2*thumbRadius) rather than the raw bounds width: a naive
// pos/width mapping would let the thumb's edge overhang the track by a full
// radius at either extreme.
//
// OnChanged parity follows fluo's uniform setter convention (matching
// CheckBox/ToggleSwitch/ToggleButton/ComboBox/TextBox): programmatic
// setters are silent, OnChanged reports only user-driven changes. SetValue
// and SetRange's re-clamp of the current value both go through
// setValueSilent and never fire OnChanged, even when the clamped result
// differs from the current value. Only USER-driven paths — drag,
// click-on-track, and the arrow-key handler, all funneled through setValue
// — fire OnChanged, and only when the clamped result actually differs from
// the current value.
type Slider struct {
	core.Element

	min, max, value float32

	enabled bool
	focused bool
	hover   bool

	onChanged func(float32)

	colors  theme.ColorTokens
	metrics theme.MetricTokens
}

// NewSlider returns an enabled Slider ranging over [0, 1] with Value 0.
func NewSlider() *Slider {
	th := theme.Active()
	return &Slider{
		min:     0,
		max:     1,
		enabled: true,
		colors:  th.Color,
		metrics: th.Metric,
	}
}

// Min returns the current minimum of the range.
func (s *Slider) Min() float32 { return s.min }

// Max returns the current maximum of the range.
func (s *Slider) Max() float32 { return s.max }

// Value returns the current value.
func (s *Slider) Value() float32 { return s.value }

// SetRange sets the [min, max] range and re-clamps the current value into
// it via setValueSilent — silent, matching SetValue and the package's
// uniform setter convention (see the type doc comment), even when the
// re-clamp actually moves the value (e.g. shrinking Max below the current
// Value). Passing max < min is not guarded here; clampF (the underlying
// clamp primitive) resolves that degenerate case by collapsing to min,
// matching every other clampF caller in this package.
func (s *Slider) SetRange(min, max float32) *Slider {
	s.min = min
	s.max = max
	s.setValueSilent(s.value)
	return s
}

// SetValue sets the value programmatically, clamped into [Min, Max].
// Silent: never fires OnChanged, even when the clamped result differs from
// the current value — see the type doc comment's OnChanged-parity note.
func (s *Slider) SetValue(v float32) *Slider {
	s.setValueSilent(v)
	return s
}

// setValueSilent is the shared mutation primitive SetValue and SetRange's
// re-clamp both funnel through: clamp v into [min, max] and assign the
// result to s.value, WITHOUT ever firing OnChanged — the programmatic,
// always-silent half of the type doc comment's OnChanged-parity note. An
// equal-value call is a harmless no-op (assigning s.value its own current
// value).
func (s *Slider) setValueSilent(v float32) {
	s.value = clampF(v, s.min, s.max)
}

// setValue is the USER-driven mutation primitive drag (setValueFromWindowX),
// click-on-track, and the arrow-key handler all funnel through: clamp into
// [min, max] via setValueSilent, then fire OnChanged if (and only if) that
// clamped value differs from the value beforehand.
func (s *Slider) setValue(v float32) {
	before := s.value
	s.setValueSilent(v)
	if s.value != before && s.onChanged != nil {
		s.onChanged(s.value)
	}
}

// OnChanged sets the callback fired with the new value whenever the user
// changes it — by drag, click-on-track, or arrow keys — but never for a
// programmatic SetValue or SetRange (see the type doc comment). Replaces
// any previously set callback; a nil fn is a valid, silent no-op.
func (s *Slider) OnChanged(fn func(float32)) *Slider {
	s.onChanged = fn
	return s
}

// SetEnabled toggles whether the slider accepts focus and pointer/keyboard
// input. Purely visual/behavioral: no invalidation needed.
func (s *Slider) SetEnabled(v bool) *Slider {
	s.enabled = v
	return s
}

// proportion reports how far Value sits between Min and Max, as a value in
// [0, 1]. Returns 0 for a degenerate (max <= min) range rather than
// dividing by zero or a negative span.
func (s *Slider) proportion() float32 {
	if s.max <= s.min {
		return 0
	}
	return (s.value - s.min) / (s.max - s.min)
}

// thumbCenterX returns the thumb's center x in absolute (window-space)
// coordinates, placed within the usable track span [thumbRadius,
// width-thumbRadius] per the type doc comment's inset rule. This is the
// exact inverse of valueFromLocalX (given the same bounds width), so a
// press exactly at a rendered thumb's center reproduces its current value.
func (s *Slider) thumbCenterX() float32 {
	bounds := s.Bounds()
	span := bounds.W - 2*sliderThumbRadius
	if span < 0 {
		span = 0
	}
	return bounds.X + sliderThumbRadius + s.proportion()*span
}

// valueFromLocalX maps localX (an x offset in logical px from the track's
// left edge, i.e. already relative to Bounds().X) to a value in [Min, Max],
// via the same usable-span inset thumbCenterX uses: localX below
// thumbRadius clamps to Min, localX above width-thumbRadius clamps to Max.
func (s *Slider) valueFromLocalX(localX float32) float32 {
	bounds := s.Bounds()
	span := bounds.W - 2*sliderThumbRadius
	if span <= 0 {
		return s.min
	}
	t := clampF((localX-sliderThumbRadius)/span, 0, 1)
	return s.min + t*(s.max-s.min)
}

// setValueFromWindowX converts a pointer event's window-space x (e.Pos.X)
// into the track-local space valueFromLocalX operates in (by subtracting
// Bounds().X) and applies it via setValue — the shared implementation for
// both click-on-track (Press) and drag (captured Move).
func (s *Slider) setValueFromWindowX(windowX float32) {
	s.setValue(s.valueFromLocalX(windowX - s.Bounds().X))
}

// MeasureContent always returns the fixed {160, 24} desired size: Slider
// has no content to size around (an explicit SetWidth/SetHeight overrides
// this through core.MeasureWidget's normal precedence, matching
// ToggleSwitch/TextBox).
func (s *Slider) MeasureContent(available render.Size) render.Size {
	return render.Size{W: sliderDesiredWidth, H: sliderDesiredHeight}
}

// ArrangeContent is a no-op: Slider has no children to position.
func (s *Slider) ArrangeContent(bounds render.Rect) {}

// Children returns nil: Slider is a leaf widget.
func (s *Slider) Children() []core.Widget { return nil }

// trackColors resolves the track's fill and stroke (ControlFill/
// ControlStroke normally, ControlFillDisabled/ControlStrokeDisabled while
// disabled) — the track itself has no hover/pressed feedback, unlike the
// thumb (see thumbColor).
func (s *Slider) trackColors() (fill, stroke render.Color) {
	if !s.enabled {
		return s.colors.ControlFillDisabled, s.colors.ControlStrokeDisabled
	}
	return s.colors.ControlFill, s.colors.ControlStroke
}

// filledColor resolves the accent-filled portion's color: Accent normally,
// AccentDisabled while disabled.
func (s *Slider) filledColor() render.Color {
	if !s.enabled {
		return s.colors.AccentDisabled
	}
	return s.colors.Accent
}

// thumbColor resolves the thumb's color: Accent at rest, AccentHover while
// the pointer hovers the slider, AccentDisabled while disabled (checked
// first — a disabled slider ignores hover entirely, per OnPointer never
// updating s.hover while disabled).
func (s *Slider) thumbColor() render.Color {
	if !s.enabled {
		return s.colors.AccentDisabled
	}
	if s.hover {
		return s.colors.AccentHover
	}
	return s.colors.Accent
}

// Render paints the rounded track (fill + hairline stroke), the
// accent-filled portion from the track's left edge to the thumb's center,
// and the 16x16 circular thumb on top.
func (s *Slider) Render(r render.Renderer) {
	bounds := s.Bounds()
	radius := sliderTrackHeight / 2
	trackY := bounds.Y + (bounds.H-sliderTrackHeight)/2

	track := render.Rect{X: bounds.X, Y: trackY, W: bounds.W, H: sliderTrackHeight}
	fill, stroke := s.trackColors()
	r.FillRoundedRect(track, radius, fill)
	if stroke.A > 0 {
		r.StrokeRoundedRect(track, radius, s.metrics.StrokeWidth, stroke)
	}

	thumbX := s.thumbCenterX()
	filled := render.Rect{X: bounds.X, Y: trackY, W: thumbX - bounds.X, H: sliderTrackHeight}
	r.FillRoundedRect(filled, radius, s.filledColor())

	thumb := render.Rect{
		X: thumbX - sliderThumbRadius, Y: bounds.Y + (bounds.H-sliderThumbSize)/2,
		W: sliderThumbSize, H: sliderThumbSize,
	}
	r.FillRoundedRect(thumb, sliderThumbRadius, s.thumbColor())
}

// RenderOverlay draws the focus ring while focused, per the global focus
// constraint shared by every focusable control in this package.
func (s *Slider) RenderOverlay(r render.Renderer) {
	if !s.focused {
		return
	}
	drawFocusRing(r, s.Bounds(), s.colors)
}

// AcceptsFocus implements input.Focusable: a disabled slider never accepts
// focus.
func (s *Slider) AcceptsFocus() bool {
	return s.enabled
}

// OnFocusChanged implements input.FocusHandler, tracking focus for the
// focus-ring overlay and keyboard arrow-key handling.
func (s *Slider) OnFocusChanged(focused bool) {
	s.focused = focused
}

// OnPointer implements input.PointerHandler: click-on-track jumps the value
// straight to the clicked x (via setValueFromWindowX), and the pointer is
// captured on Press so a subsequent drag survives leaving the slider's own
// bounds — Move only updates the value while this slider holds the
// capture, matching TextBox's click-to-caret/drag-to-select pattern.
// Ignored entirely while disabled (not handled, so pointer events bubble
// past a disabled slider rather than being swallowed by it) — but a
// SetEnabled(false) landing MID-DRAG (this slider still holds the router's
// capture from an earlier Press) releases that capture first, before the
// disabled early-return: otherwise every subsequent pointer event would
// keep routing here via deliverCaptured (never hit-testing) and find a
// disabled slider unwilling to do anything with it — a permanent wedge with
// no widget reachable by the pointer at all, not merely this one ignoring
// input as intended.
func (s *Slider) OnPointer(e *input.PointerEvent) {
	if !s.enabled {
		if e.Router != nil && e.Router.Captured() == s {
			e.Router.Release()
		}
		return
	}
	switch e.Action {
	case input.Enter:
		s.hover = true
	case input.Leave:
		s.hover = false
	case input.Press:
		s.setValueFromWindowX(e.Pos.X)
		e.Router.Capture(s)
		e.Handled = true
	case input.Move:
		if e.Router.Captured() == s {
			s.setValueFromWindowX(e.Pos.X)
			e.Handled = true
		}
	case input.Release:
		if e.Router.Captured() == s {
			e.Router.Release()
			e.Handled = true
		}
	}
}

// OnKey implements input.KeyHandler: Left/Right nudge the value by
// (Max-Min)/100, or (Max-Min)/10 with Shift held, on Press. OnKey is only
// ever invoked while this slider is focused or an ancestor of the focused
// widget (input.Router's key dispatch walks up from the focused widget), so
// there is no separate focused check here — matching Button/CheckBox's
// enabled-only guard.
func (s *Slider) OnKey(e *input.KeyEvent) {
	if !s.enabled || e.Action != input.Press {
		return
	}
	step := (s.max - s.min) / 100
	if e.Mods&input.ModShift != 0 {
		step = (s.max - s.min) / 10
	}
	switch e.Key {
	case input.KeyLeft:
		s.setValue(s.value - step)
		e.Handled = true
	case input.KeyRight:
		s.setValue(s.value + step)
		e.Handled = true
	}
}
