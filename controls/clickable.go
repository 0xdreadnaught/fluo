package controls

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/theme"
)

// ClickBehavior implements the standard press state machine shared by every
// button-like widget: a Press over the widget captures the pointer (so drag
// tracking survives the pointer leaving the widget's bounds) and marks hover
// state via Enter/Leave; the matching Release fires OnClick only if the
// pointer is still over the widget's bounds at release time, and always
// releases the capture. Embed by value in a composite widget (e.g. Button)
// and drive it from the owner's own OnPointer via HandlePointer, passing the
// owner itself so containment can be tested against its live bounds.
//
// ClickBehavior has no notion of "enabled" — an owner that wants disabled
// widgets to ignore pointer input entirely (not merely fail to fire) must
// skip calling HandlePointer altogether while disabled, so e.Handled stays
// false and the event keeps bubbling.
type ClickBehavior struct {
	hover, pressed bool

	// OnClick fires when a Press over the widget is followed by a Release
	// still over it (see HandlePointer), or via a keyboard activation path
	// calling Activate directly. Nil is a valid, silent no-op.
	OnClick func()
}

// HandlePointer runs the click state machine for e, using core.BoundsOf(owner)
// to test containment. owner is also what's registered with the router's
// pointer capture (input.Router.Capture/Release), so it must be the exact
// core.Widget value the caller's own OnPointer receiver serves as (see
// Button.OnPointer) — e.Router is assumed non-nil, as it always is for events
// delivered through an input.Router.
//
// Enter/Leave only update Hover. Press marks pressed, captures the pointer,
// and marks e.Handled. Release (delivered directly to the captured owner
// regardless of e.Pos, per input.Router's capture semantics) clears pressed,
// releases the capture, marks e.Handled, and fires OnClick only if the
// widget was pressed AND e.Pos still falls within its current bounds —
// releasing outside does not fire. Move and Wheel are ignored: hover across a
// drag is not tracked mid-capture (input.Router itself does not deliver
// Enter/Leave while a capture is active), a documented v0 simplification.
func (c *ClickBehavior) HandlePointer(e *input.PointerEvent, owner core.Widget) {
	switch e.Action {
	case input.Enter:
		c.hover = true
	case input.Leave:
		c.hover = false
	case input.Press:
		c.pressed = true
		e.Router.Capture(owner)
		e.Handled = true
	case input.Release:
		wasPressed := c.pressed
		c.pressed = false
		e.Router.Release()
		if wasPressed && core.BoundsOf(owner).Contains(e.Pos) {
			c.Activate()
		}
		e.Handled = true
	}
}

// Hover reports whether the pointer is currently over the widget, as last
// updated by an Enter/Leave delivered to HandlePointer.
func (c *ClickBehavior) Hover() bool { return c.hover }

// Pressed reports whether the widget is currently mid-press (a Press was
// delivered and no matching Release has landed yet).
func (c *ClickBehavior) Pressed() bool { return c.pressed }

// Activate fires OnClick directly, bypassing the press/release state
// machine entirely. This is the keyboard-activation path (Space/Enter on a
// focused button) as well as the mechanism HandlePointer's Release branch
// itself uses on a successful click.
func (c *ClickBehavior) Activate() {
	if c.OnClick != nil {
		c.OnClick()
	}
}

// drawFocusRing draws the focus-ring overlay shared by every focusable
// control (Button, CheckBox, RadioButton, ToggleSwitch, TextBox, Slider,
// ListView, tabStrip): the classic solid-line inset focus rectangle (see
// drawFocusRect in bevel.go), using colors.Highlight. Callers pass their own
// bounds and are expected to have already checked their focused flag.
//
// The rect is inset (drawn just inside bounds) rather than outset (bounds
// inflated outward) so it always falls WITHIN the widget's own rectangle. An
// outset ring's outer band lies outside the widget and is cropped by any
// clipping ancestor a focusable sits flush against — e.g. the leftmost tab
// or a list flush against a ScrollViewer's clip edge would lose the ring's
// left side. Keeping the ring inside the bounds makes it immune to ancestor
// clipping without needing a separate adorner layer.
func drawFocusRing(r render.Renderer, bounds render.Rect, colors theme.ColorTokens) {
	drawFocusRect(r, bounds, colors)
}
