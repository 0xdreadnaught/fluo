package controls

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
	"github.com/0xdreadnaught/fluo/theme"
	"github.com/0xdreadnaught/fluo/timers"
)

// Button is a clickable, focusable, token-styled push button showing a text
// label. It is a composite widget: its own Render paints the fill/stroke
// chrome (varying by accent/hover/pressed/disabled state), RenderOverlay
// paints the focus ring while focused, and its label TextBlock is arranged
// centered within the padded content rect. Colors and metrics are captured
// from theme.Active() at construction (rebuild to re-theme, matching every
// other control in this package).
type Button struct {
	core.Element

	click ClickBehavior

	label   *TextBlock
	accent  bool
	enabled bool
	focused bool

	colors  theme.ColorTokens
	metrics theme.MetricTokens

	// animated and timerQueue gate the opt-in cross-fade behavior (see
	// SetAnimated/SetTimers): fillAnim is only ever consulted (and lazily
	// created) when BOTH are set — nil timerQueue is the "SetAnimated(true)
	// but no queue wired yet" instant fallback, matching TextBox's
	// caret-blink wiring pattern. Default zero values (false, nil) leave
	// Render's fill resolution byte-identical to before this feature
	// existed.
	animated   bool
	timerQueue *timers.Queue
	fillAnim   *colorAnim
}

// initButton fills b in place: builds the label, marks it enabled, captures
// theme.Active()'s tokens, and parents the label to b. Factored out of
// NewButton so NewToggleButton can initialize the Button EMBEDDED (by value)
// inside a *ToggleButton directly at its final address, rather than building
// a standalone *Button and copying it: copying a Button whose label already
// has its parent set to the (about-to-be-discarded) standalone pointer would
// leave the label's parent dangling, silently breaking InvalidateMeasure's
// climb to the real container.
func initButton(b *Button, face *text.Face, label string) {
	th := theme.Active()
	b.enabled = true
	b.colors = th.Color
	b.metrics = th.Metric
	b.label = NewTextBlock(face, label)
	core.SetParent(b.label, b)
}

// NewButton returns an enabled, non-accent Button showing label in face
// (face may be nil, per TextBlock).
func NewButton(face *text.Face, label string) *Button {
	b := &Button{}
	initButton(b, face, label)
	return b
}

// OnClick sets the callback fired on a successful click (pointer
// press-release-inside, or Space/Enter while focused). Replaces any
// previously set callback; a nil fn is a valid, silent no-op.
func (b *Button) OnClick(fn func()) *Button {
	b.click.OnClick = fn
	return b
}

// SetAccent switches between the default (ControlFill-family) chrome and the
// accent (Accent-family, no stroke) chrome. Purely visual: no invalidation
// needed since the host redraws every frame.
func (b *Button) SetAccent(a bool) *Button {
	b.accent = a
	return b
}

// SetEnabled toggles whether the button accepts focus and pointer/keyboard
// input. Disabling a currently-focused button does not itself clear router
// focus (the button has no router reference) — callers that need that must
// clear focus explicitly; a documented v0 simplification. Purely visual
// otherwise: no invalidation needed.
func (b *Button) SetEnabled(v bool) *Button {
	b.enabled = v
	return b
}

// Label returns the button's label TextBlock, for tests and customization
// (e.g. overriding its color).
func (b *Button) Label() *TextBlock {
	return b.label
}

// SetAnimated opts this button into cross-fading its fill (rest/hover/
// pressed/disabled) transitions over colorAnimDuration (~120ms, EaseOut)
// instead of snapping — PROVIDED a timers.Queue has also been wired via
// SetTimers; see SetTimers's doc comment for the "animated but no queue"
// instant fallback. Default false, matching fluo's opt-in-animation
// convention: every Button built before this feature existed (nothing
// calls SetAnimated) renders with today's exact snap-to-state colors,
// byte-identical, so no existing test or golden needs to change. Purely
// visual: no invalidation needed — the tween is driven by timerQueue's own
// Advance, and the host redraws every frame regardless (see Render).
func (b *Button) SetAnimated(v bool) *Button {
	b.animated = v
	return b
}

// SetTimers wires q as the driver for this button's animated fill
// cross-fades (see SetAnimated); has no effect unless SetAnimated(true) is
// also set. Passing nil detaches any previously wired queue, reverting to
// the instant (snap) fallback even with SetAnimated(true) still set — the
// same "nil detaches" convention as TextBox.SetTimers. A button that is
// SetAnimated(true) but never had SetTimers called (timerQueue stays nil)
// behaves exactly like an unanimated one: instant, current behavior.
func (b *Button) SetTimers(q *timers.Queue) *Button {
	b.timerQueue = q
	return b
}

// animatedFill resolves the fill Render should actually paint: fill
// unchanged unless this button is BOTH animated and has a timers.Queue
// wired, in which case fill is threaded through b.fillAnim (lazily created
// on first use, seeded at fill so the very first animated frame never
// fades in from a zero-value color) so state-driven fill changes (e.g.
// rest->hover) cross-fade instead of snapping. Called once per Render with
// the state's target fill; colorAnim.SetTarget's own same-target guard
// means repeated calls across steady-state frames (nothing about the
// button's state changed) are cheap no-ops rather than restarting a tween
// every frame.
func (b *Button) animatedFill(fill render.Color) render.Color {
	if !b.animated || b.timerQueue == nil {
		return fill
	}
	if b.fillAnim == nil {
		b.fillAnim = newColorAnim(fill)
	} else {
		b.fillAnim.SetTarget(b.timerQueue, fill)
	}
	return b.fillAnim.Current()
}

// padding returns the content inset: PaddingL horizontal, PaddingM vertical.
func (b *Button) padding() render.Thickness {
	return render.Thickness{
		Left: b.metrics.PaddingL, Right: b.metrics.PaddingL,
		Top: b.metrics.PaddingM, Bottom: b.metrics.PaddingM,
	}
}

// MeasureContent measures the label within the available space reduced by
// padding, then adds the padding back to its desired size.
func (b *Button) MeasureContent(available render.Size) render.Size {
	pad := b.padding()

	availW := available.W - pad.Left - pad.Right
	if availW < 0 {
		availW = 0
	}
	availH := available.H - pad.Top - pad.Bottom
	if availH < 0 {
		availH = 0
	}

	core.MeasureWidget(b.label, render.Size{W: availW, H: availH})
	d := core.DesiredSizeOf(b.label)

	return render.Size{W: d.W + pad.Left + pad.Right, H: d.H + pad.Top + pad.Bottom}
}

// ArrangeContent arranges the label centered (both axes) within bounds inset
// by padding. Centering (rather than filling) matters whenever the button
// ends up wider/taller than its own desired size, e.g. stretched by a parent
// panel.
func (b *Button) ArrangeContent(bounds render.Rect) {
	pad := b.padding()
	inner := bounds.Inset(pad)
	if inner.W < 0 {
		inner.W = 0
	}
	if inner.H < 0 {
		inner.H = 0
	}

	d := core.DesiredSizeOf(b.label)
	x := inner.X + (inner.W-d.W)/2
	y := inner.Y + (inner.H-d.H)/2
	core.ArrangeWidget(b.label, render.Rect{X: x, Y: y, W: d.W, H: d.H})
}

// stateColors resolves the fill, stroke (zero-alpha means "no stroke"), and
// label color for the button's current state:
//
//   - disabled: ControlFillDisabled/ControlStrokeDisabled/TextDisabled, or
//     for an accent button, AccentDisabled fill with no stroke.
//   - accent (enabled): Accent/AccentHover/AccentPressed fill, no stroke,
//     AccentText label.
//   - default (enabled): ControlFill/Hover/Pressed fill, ControlStroke
//     stroke, TextPrimary label.
func (b *Button) stateColors() (fill, stroke, label render.Color) {
	c := b.colors

	if !b.enabled {
		if b.accent {
			return c.AccentDisabled, render.Color{}, c.TextDisabled
		}
		return c.ControlFillDisabled, c.ControlStrokeDisabled, c.TextDisabled
	}

	if b.accent {
		fill = c.Accent
		switch {
		case b.click.Pressed():
			fill = c.AccentPressed
		case b.click.Hover():
			fill = c.AccentHover
		}
		return fill, render.Color{}, c.AccentText
	}

	fill = c.ControlFill
	switch {
	case b.click.Pressed():
		fill = c.ControlFillPressed
	case b.click.Hover():
		fill = c.ControlFillHover
	}
	return fill, c.ControlStroke, c.TextPrimary
}

// Render paints the button's fill and (if visible) stroke, and recolors the
// label for the current state; children (the label) render separately via
// core.RenderWidget. The fill is resolved through animatedFill, which is a
// no-op pass-through unless SetAnimated(true) and a non-nil SetTimers queue
// are both set (see animatedFill).
func (b *Button) Render(r render.Renderer) {
	fill, stroke, labelColor := b.stateColors()
	fill = b.animatedFill(fill)
	bounds := b.Bounds()
	radius := b.metrics.ControlCornerRadius

	r.FillRoundedRect(bounds, radius, fill)
	if stroke.A > 0 {
		r.StrokeRoundedRect(bounds, radius, b.metrics.StrokeWidth, stroke)
	}
	b.label.SetColor(labelColor)
}

// RenderOverlay draws the focus ring while focused, per the global focus
// constraint: StrokeRoundedRect on the button's bounds inflated by 2, radius
// = control radius + 2, FocusStroke color and FocusStrokeWidth.
func (b *Button) RenderOverlay(r render.Renderer) {
	if !b.focused {
		return
	}
	drawFocusRing(r, b.Bounds(), b.metrics.ControlCornerRadius, b.colors, b.metrics)
}

// Children returns the label as the button's sole child.
func (b *Button) Children() []core.Widget {
	return []core.Widget{b.label}
}

// AcceptsFocus implements input.Focusable: a disabled button never accepts
// focus.
func (b *Button) AcceptsFocus() bool {
	return b.enabled
}

// OnFocusChanged implements input.FocusHandler, tracking focus for the
// focus-ring overlay and keyboard activation.
func (b *Button) OnFocusChanged(focused bool) {
	b.focused = focused
}

// OnPointer implements input.PointerHandler, delegating the entire
// press/release/hover state machine to the embedded ClickBehavior while
// disabled (ignoring pointer input outright, not merely failing to fire —
// e.Handled is left false so the event keeps bubbling).
func (b *Button) OnPointer(e *input.PointerEvent) {
	if !b.enabled {
		return
	}
	b.click.HandlePointer(e, b)
}

// OnKey implements input.KeyHandler: Space or Enter, on Press, activates the
// button (fires OnClick) and marks the event handled. OnKey is only ever
// invoked while this button is focused or an ancestor of the focused widget
// (input.Router's key dispatch walks up from the focused widget), and a
// disabled button can never hold focus in the first place; the enabled
// check here is a defensive no-op guard, not load-bearing.
func (b *Button) OnKey(e *input.KeyEvent) {
	if !b.enabled || e.Action != input.Press {
		return
	}
	if e.Key == input.KeySpace || e.Key == input.KeyEnter {
		b.click.Activate()
		e.Handled = true
	}
}

// ToggleButton is a Button that toggles a boolean checked state on click,
// rendering the checked state as accent-on (Accent-family fill, no stroke,
// AccentText label) regardless of accent — it has no independent accent
// flag of its own; checked IS the accent look, achieved by driving the
// embedded Button's own SetAccent as checked changes.
//
// ToggleButton embeds Button BY VALUE and never overrides most of its
// methods (OnPointer, Render, RenderOverlay, ...): Go method promotion is
// static, so a promoted method always runs with a *Button receiver (pointing
// at the embedded field), never a *ToggleButton one, and cannot itself know
// it was reached via a ToggleButton to run different logic. Rather than
// fight that, ToggleButton hooks its behavior through DATA the promoted
// code already reads: the embedded ClickBehavior's OnClick function field.
// NewToggleButton wires click.OnClick to a closure that toggles state, syncs
// SetAccent, and fires the user's OnChanged. OnClick and SetAccent are the
// two methods ToggleButton DOES shadow (see below) — precisely because
// leaving them promoted would let a caller silently clobber that wiring or
// desync the chrome from Checked().
//
// One more embedding nuance, for whoever next embeds a composite control by
// value like this: input.Router.Detach walks a subtree via Children(), which
// only ever sees tree nodes as actually added (the *ToggleButton), never the
// embedded &tb.Button — so a ToggleButton removed mid-press (e.g. a popup
// item closed while captured) leaks its pointer-capture entry, unlike a
// plain Button, which Detach's subtree walk finds directly.
//
// Outermost-capture identity trap, same root cause: Button.OnPointer calls
// b.click.HandlePointer(e, b), passing the promoted method's OWN receiver —
// which, reached through a *ToggleButton, is &tb.Button (the embedded
// field's address), never tb itself (see ClickBehavior.HandlePointer's doc
// comment on owner). So input.Router.Captured() during a ToggleButton's
// press-drag reports &tb.Button, NOT the *ToggleButton a caller likely holds
// a reference to — comparing Captured() against the outer widget identity
// (e.g. `router.Captured() == myToggleButton`) will never match this
// control, only `== &myToggleButton.Button` would. This is harmless
// internally (Capture and the matching Release/comparison inside
// HandlePointer always use that same &tb.Button value), but it is a trap
// for any OTHER code that tries to recognize "is this ToggleButton
// currently capturing?" by outer identity.
type ToggleButton struct {
	Button

	checked   bool
	onChanged func(bool)
}

// NewToggleButton returns an unchecked, enabled ToggleButton showing label
// in face.
func NewToggleButton(face *text.Face, label string) *ToggleButton {
	t := &ToggleButton{}
	initButton(&t.Button, face, label)

	t.click.OnClick = func() {
		t.checked = !t.checked
		t.Button.SetAccent(t.checked)
		if t.onChanged != nil {
			t.onChanged(t.checked)
		}
	}
	return t
}

// OnClick is shadowed (NOT promoted from Button) and panics: a ToggleButton
// wires its own internal ClickBehavior.OnClick in NewToggleButton to drive
// toggle+notify, and Button.OnClick's normal "replace the callback"
// semantics would silently clobber that wiring, permanently breaking
// Checked/OnChanged with no compile-time signal. Use OnChanged instead.
func (t *ToggleButton) OnClick(fn func()) *ToggleButton {
	panic("controls: ToggleButton.OnClick is not supported (it would replace the internal toggle wiring) — use OnChanged instead")
}

// SetAccent is shadowed (NOT promoted from Button) and panics: ToggleButton
// has no independent accent flag — its chrome is driven entirely by Checked
// (see the type doc comment) — so an external SetAccent call would silently
// desync the rendered chrome from Checked() until the next toggle overwrites
// it again. Use SetChecked instead.
func (t *ToggleButton) SetAccent(a bool) *ToggleButton {
	panic("controls: ToggleButton.SetAccent is not supported (checked state alone drives the accent chrome) — use SetChecked instead")
}

// Checked reports the current toggle state.
func (t *ToggleButton) Checked() bool {
	return t.checked
}

// SetChecked sets the toggle state programmatically. Normative: unlike a
// click, SetChecked does NOT fire OnChanged — it is the plain setter
// counterpart to the getter Checked, matching the rest of fluo's SetX
// convention (SetText, SetAccent, ...), while OnChanged is reserved for
// user-driven changes (clicks, keyboard activation). A no-op (no re-render
// implications either way) when v already matches the current state.
// Fluo's uniform contract, restated: programmatic setters are silent;
// OnChanged reports only user-driven changes.
func (t *ToggleButton) SetChecked(v bool) *ToggleButton {
	if t.checked == v {
		return t
	}
	t.checked = v
	t.Button.SetAccent(v)
	return t
}

// OnChanged sets the callback fired with the new checked value whenever the
// user toggles the button (click or Space/Enter) — never for a programmatic
// SetChecked (fluo's uniform contract: programmatic setters are silent;
// OnChanged reports only user-driven changes). Replaces any previously set
// callback; a nil fn is a valid, silent no-op.
func (t *ToggleButton) OnChanged(fn func(bool)) *ToggleButton {
	t.onChanged = fn
	return t
}
