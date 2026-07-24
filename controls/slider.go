package controls

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/theme"
)

// Fixed geometry for Slider, per the Phase 5 Task 7 visuals spec (superseded
// by the v0.2 classic restyle's square-corner thumb): a 4px-tall sunken
// groove track with a 16x16 raised RECTANGULAR thumb (drawRaised — see
// Render), square corners like the rest of this family.
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

// Slider is a focusable, token-styled continuous value picker over [Min,
// Max] (defaulting to [0, 1]). Value is reported and set as a plain
// float32; SetRange/SetValue both clamp into the current range.
//
// Orientation defaults to Horizontal (see SetOrientation) and generalizes
// the normative geometry below onto whichever axis is the slider's MAIN
// axis (X for Horizontal, Y for Vertical):
//
// Horizontal (see the const block above): the accent-filled portion of the
// track runs from the track's left edge to the thumb's CENTER x, and the
// thumb's center itself is only ever placed within the "usable track span"
// [thumbRadius, width-thumbRadius] — a thumb centered at x=thumbRadius sits
// flush against the track's left edge (Value==Min), and one centered at
// x=width-thumbRadius sits flush against the right edge (Value==Max).
//
// Vertical: Max is at the TOP, Min is at the bottom (the reverse of a
// naive top-to-bottom mapping) — a thumb centered at y=thumbRadius (the
// track's top edge) is Value==Max, and one centered at
// y=height-thumbRadius (the bottom edge) is Value==Min. The filled portion
// mirrors horizontal's "fill the Min side": it runs from the thumb's
// center DOWN to the track's bottom edge.
//
// This inset is why proportion() and its inverse valueFromLocal() both
// divide by (mainAxisLength - 2*thumbRadius) rather than the raw bounds
// length: a naive pos/length mapping would let the thumb's edge overhang
// the track by a full radius at either extreme. thumbCenter/valueFromLocal/
// Render/MeasureContent all share this one axis-parameterized
// implementation, keyed off s.orientation.
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
	orientation     Orientation

	enabled bool
	focused bool
	hover   bool

	onChanged func(float32)

	colors  theme.ColorTokens
	metrics theme.MetricTokens
}

// NewSlider returns an enabled, Horizontal Slider ranging over [0, 1] with
// Value 0.
func NewSlider() *Slider {
	th := theme.Active()
	return &Slider{
		min:         0,
		max:         1,
		enabled:     true,
		orientation: Horizontal,
		colors:      th.Color,
		metrics:     th.Metric,
	}
}

// SetOrientation sets the slider's orientation — Horizontal (the default)
// lays the track left-to-right with Min at the left edge and Max at the
// right; Vertical lays the track top-to-bottom with Max at the TOP and Min
// at the bottom (see the type doc comment). Takes effect on the next
// Measure/Arrange/Render pass.
func (s *Slider) SetOrientation(o Orientation) *Slider {
	s.orientation = o
	return s
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

// setValue is the USER-driven mutation primitive drag
// (setValueFromWindowPos), click-on-track, and the arrow-key handler all
// funnel through: clamp into
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

// axisLength returns the slider's extent along its MAIN axis: bounds.W for
// Horizontal, bounds.H for Vertical — the length thumbCenter/valueFromLocal
// use for the usable-span inset.
func (s *Slider) axisLength() float32 {
	bounds := s.Bounds()
	if s.orientation == Vertical {
		return bounds.H
	}
	return bounds.W
}

// thumbCenter returns the thumb's center coordinate along the MAIN axis, in
// absolute (window-space) coordinates — X for Horizontal, Y for Vertical —
// placed within the usable track span [thumbRadius, axisLength-thumbRadius]
// per the type doc comment's inset rule. This is the exact inverse of
// valueFromLocal (given the same bounds), so a press exactly at a rendered
// thumb's center reproduces its current value.
//
// Horizontal: center = origin + thumbRadius + proportion*span (Min at the
// origin, Max at the far end). Vertical: center = origin + thumbRadius +
// (1-proportion)*span — Max at the origin (top), Min at the far end
// (bottom), per the type doc comment's vertical convention.
func (s *Slider) thumbCenter() float32 {
	bounds := s.Bounds()
	span := s.axisLength() - 2*sliderThumbRadius
	if span < 0 {
		span = 0
	}
	p := s.proportion()
	if s.orientation == Vertical {
		return bounds.Y + sliderThumbRadius + (1-p)*span
	}
	return bounds.X + sliderThumbRadius + p*span
}

// valueFromLocal maps local (an offset in logical px from the track's
// origin along the MAIN axis — the left edge for Horizontal, the TOP edge
// for Vertical — i.e. already relative to Bounds().X or Bounds().Y) to a
// value in [Min, Max], via the same usable-span inset thumbCenter uses:
// local below thumbRadius clamps to one extreme, local above
// axisLength-thumbRadius clamps to the other. Horizontal: increasing local
// maps toward Max. Vertical: increasing local (moving down) maps toward
// Min, since Max is at the top (see the type doc comment).
func (s *Slider) valueFromLocal(local float32) float32 {
	span := s.axisLength() - 2*sliderThumbRadius
	if span <= 0 {
		return s.min
	}
	t := clampF((local-sliderThumbRadius)/span, 0, 1)
	if s.orientation == Vertical {
		t = 1 - t
	}
	return s.min + t*(s.max-s.min)
}

// setValueFromWindowPos converts a pointer event's window-space position
// into the track-local coordinate valueFromLocal operates in (by
// subtracting Bounds().X for Horizontal or Bounds().Y for Vertical) and
// applies it via setValue — the shared implementation for both
// click-on-track (Press) and drag (captured Move).
func (s *Slider) setValueFromWindowPos(pos render.Point) {
	bounds := s.Bounds()
	if s.orientation == Vertical {
		s.setValue(s.valueFromLocal(pos.Y - bounds.Y))
		return
	}
	s.setValue(s.valueFromLocal(pos.X - bounds.X))
}

// MeasureContent returns the fixed desired size: {160, 24} for Horizontal
// (the default), swapped to {24, 160} (tall-narrow) for Vertical. Slider
// has no content to size around (an explicit SetWidth/SetHeight overrides
// this through core.MeasureWidget's normal precedence, matching
// ToggleSwitch/TextBox).
func (s *Slider) MeasureContent(available render.Size) render.Size {
	if s.orientation == Vertical {
		return render.Size{W: sliderDesiredHeight, H: sliderDesiredWidth}
	}
	return render.Size{W: sliderDesiredWidth, H: sliderDesiredHeight}
}

// ArrangeContent is a no-op: Slider has no children to position.
func (s *Slider) ArrangeContent(bounds render.Rect) {}

// Children returns nil: Slider is a leaf widget.
func (s *Slider) Children() []core.Widget { return nil }

// thumbColor resolves the thumb's color: Accent at rest, AccentHover while
// the pointer hovers the slider, AccentDisabled while disabled (checked
// first — a disabled slider ignores hover entirely, per OnPointer never
// updating s.hover while disabled).
//
// Render no longer calls thumbColor (the classic restyle draws the thumb via
// drawRaised keyed directly off s.hover — ButtonLight/ButtonFace, not this
// method's Accent-family colors); it is kept solely because
// TestSliderHoverTracked exercises it directly as its regression proof that
// hover and rest resolve to different colors (the pre-restyle deprecated
// tokens this method reads, Accent/AccentHover, are still distinct in both
// classic themes, so the test still passes unchanged).
func (s *Slider) thumbColor() render.Color {
	if !s.enabled {
		return s.colors.AccentDisabled
	}
	if s.hover {
		return s.colors.AccentHover
	}
	return s.colors.Accent
}

// Render paints the classic trackbar: a thin sunken groove (drawSunken,
// ButtonFace) across the slider's full main-axis length, a Highlight-filled
// band covering the track's Min side up to the thumb's center overlaid on
// top of it, and a square 16x16 raised thumb (drawRaised) centered on the
// thumb position — ButtonLight while hovered.
//
// Horizontal: the groove runs left-to-right and the fill spans the track's
// left edge to the thumb center. Vertical: the groove runs top-to-bottom
// and the fill spans the thumb center DOWN to the track's bottom edge (Min
// is at the bottom — see the type doc comment).
func (s *Slider) Render(r render.Renderer) {
	c := s.colors
	bounds := s.Bounds()

	var track render.Rect
	if s.orientation == Vertical {
		trackX := bounds.X + (bounds.W-sliderTrackHeight)/2
		track = render.Rect{X: trackX, Y: bounds.Y, W: sliderTrackHeight, H: bounds.H}
	} else {
		trackY := bounds.Y + (bounds.H-sliderTrackHeight)/2
		track = render.Rect{X: bounds.X, Y: trackY, W: bounds.W, H: sliderTrackHeight}
	}
	drawSunken(r, track, c.ButtonFace, c)

	thumbPos := s.thumbCenter()

	var filled render.Rect
	if s.orientation == Vertical {
		filled = render.Rect{X: track.X, Y: thumbPos, W: track.W, H: bounds.Y + bounds.H - thumbPos}
	} else {
		filled = render.Rect{X: bounds.X, Y: track.Y, W: thumbPos - bounds.X, H: sliderTrackHeight}
	}
	r.FillRect(filled, c.Highlight)

	thumbFace := c.ButtonFace
	if s.hover {
		thumbFace = c.ButtonLight
	}
	var thumb render.Rect
	if s.orientation == Vertical {
		thumb = render.Rect{
			X: bounds.X + (bounds.W-sliderThumbSize)/2, Y: thumbPos - sliderThumbRadius,
			W: sliderThumbSize, H: sliderThumbSize,
		}
	} else {
		thumb = render.Rect{
			X: thumbPos - sliderThumbRadius, Y: bounds.Y + (bounds.H-sliderThumbSize)/2,
			W: sliderThumbSize, H: sliderThumbSize,
		}
	}
	drawRaised(r, thumb, thumbFace, c)
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
// straight to the clicked position (via setValueFromWindowPos), and the
// pointer is captured on Press so a subsequent drag survives leaving the
// slider's own bounds — Move only updates the value while this slider holds
// the capture, matching TextBox's click-to-caret/drag-to-select pattern.
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
		s.setValueFromWindowPos(e.Pos)
		e.Router.Capture(s)
		e.Handled = true
	case input.Move:
		if e.Router.Captured() == s {
			s.setValueFromWindowPos(e.Pos)
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
