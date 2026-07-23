package controls

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
	"github.com/0xdreadnaught/fluo/theme"
)

// glyphBoxSize is the fixed edge length of the CheckBox/RadioButton glyph
// box (both are 18x18, per the Phase 5 Task 4 visuals spec — a checkbox's
// square and a radio button's circle differ only in corner radius).
const glyphBoxSize float32 = 18

// checkGlyphInset is how far the checkmark fallback square is inset from
// the checkbox box's edges (normative: inset 5, giving an 8x8 inner square
// on the 18x18 box).
const checkGlyphInset float32 = 5

// checkGlyphFallbackRadius is the corner radius of the fallback inner
// square, scaled down from the box's own ControlCornerRadius (4) to suit
// the much smaller 8x8 square.
const checkGlyphFallbackRadius float32 = 2

// radioInnerSize and radioInnerRadius describe the checked radio button's
// inner dot: a 9x9 filled rounded-rect of radius 4.5 (a circle, since
// radius == half the side), centered within the 18x18 outer circle —
// normative approximation of "outer stroke Accent width 2 + inner Accent
// circle radius 4.5".
const (
	radioInnerSize   float32 = 9
	radioInnerRadius float32 = 4.5
	radioRingWidth   float32 = 2
)

// glyphMeasure computes the desired size of a glyph-box-plus-optional-label
// composite shared by CheckBox and RadioButton: label measures in the space
// left over after the box, and the gap (PaddingM) between them is only
// reserved when the label actually has content — an empty label (as used by
// the toggles.png golden, which draws bare glyphs with no labels) must not
// leave a dangling gap.
func glyphMeasure(label *TextBlock, available render.Size, gap float32) render.Size {
	availW := available.W - glyphBoxSize
	if availW < 0 {
		availW = 0
	}
	core.MeasureWidget(label, render.Size{W: availW, H: available.H})
	ld := core.DesiredSizeOf(label)

	w := glyphBoxSize
	h := glyphBoxSize
	if ld.W > 0 {
		w += gap + ld.W
	}
	if ld.H > h {
		h = ld.H
	}
	return render.Size{W: w, H: h}
}

// glyphArrange positions the glyph box at bounds' left edge (vertically
// centered on bounds.H) and, if label has content, arranges it to the
// box's right with a gap; returns the box's own absolute rect for Render to
// draw into. Mirrors glyphMeasure's "no gap for an empty label" rule.
func glyphArrange(label *TextBlock, bounds render.Rect, gap float32) render.Rect {
	box := render.Rect{
		X: bounds.X, Y: bounds.Y + (bounds.H-glyphBoxSize)/2,
		W: glyphBoxSize, H: glyphBoxSize,
	}

	ld := core.DesiredSizeOf(label)
	if ld.W > 0 {
		lx := bounds.X + glyphBoxSize + gap
		ly := bounds.Y + (bounds.H-ld.H)/2
		core.ArrangeWidget(label, render.Rect{X: lx, Y: ly, W: ld.W, H: ld.H})
	} else {
		core.ArrangeWidget(label, render.Rect{X: bounds.X, Y: bounds.Y, W: 0, H: 0})
	}
	return box
}

// hasCheckmarkGlyph reports whether face's font actually has a glyph for
// U+2713 (✓) — the runtime decision procedure CheckBox uses to choose
// between drawing that glyph or falling back to a drawn shape. A nil face
// (or one with no Font) always falls back. As of this writing, goregular
// (fluo's test/demo face) does NOT have this glyph — see
// text.TestHasGlyph — so CheckBox ships drawing the fallback square with
// that font; a font that does carry the glyph takes the glyph path
// automatically, with no code change required.
func hasCheckmarkGlyph(face *text.Face) bool {
	return face != nil && face.Font != nil && face.Font.HasGlyph('✓')
}

// CheckBox is a clickable, focusable, token-styled 18x18 box with an
// optional label to its right, toggling a boolean checked state. Like
// ToggleButton, it is ClickBehavior-driven and follows the Checked/
// SetChecked/OnChanged/SetEnabled convention: SetChecked is a silent
// programmatic setter, OnChanged fires only for user-driven changes (click
// or Space/Enter while focused).
//
// Visuals (normative): unchecked = ControlFill fill + ControlStroke stroke,
// radius 4; checked = Accent fill, no stroke, plus a checkmark drawn either
// as the U+2713 glyph (if the face's font has it, per hasCheckmarkGlyph) or
// a fallback AccentText-colored inner rounded square inset 5 from the box's
// edges.
type CheckBox struct {
	core.Element

	click ClickBehavior

	face       *text.Face
	label      *TextBlock
	checkGlyph bool // true: draw '✓' via face; false: draw the fallback square

	// box is the glyph box's absolute rect, cached by ArrangeContent for
	// Render (it is a sub-rect of the composite's own Bounds() whenever a
	// label is present, so it can't simply be recomputed from Bounds() at
	// render time).
	box render.Rect

	checked   bool
	enabled   bool
	focused   bool
	onChanged func(bool)

	colors  theme.ColorTokens
	metrics theme.MetricTokens
}

// NewCheckBox returns an unchecked, enabled CheckBox showing label (may be
// "", per the golden's bare-glyph controls) in face (face may be nil, per
// TextBlock — a nil face also forces the fallback checkmark square, since
// there is no font to query for the glyph).
func NewCheckBox(face *text.Face, label string) *CheckBox {
	c := &CheckBox{}
	th := theme.Active()
	c.enabled = true
	c.colors = th.Color
	c.metrics = th.Metric
	c.face = face
	c.checkGlyph = hasCheckmarkGlyph(face)
	c.label = NewTextBlock(face, label)
	core.SetParent(c.label, c)

	c.click.OnClick = func() {
		c.checked = !c.checked
		if c.onChanged != nil {
			c.onChanged(c.checked)
		}
	}
	return c
}

// Checked reports the current toggle state.
func (c *CheckBox) Checked() bool { return c.checked }

// SetChecked sets the toggle state programmatically. Normative: unlike a
// click, SetChecked does NOT fire OnChanged (matching ToggleButton's
// convention) and is a no-op when v already matches the current state.
// Fluo's uniform contract, restated: programmatic setters are silent;
// OnChanged reports only user-driven changes.
func (c *CheckBox) SetChecked(v bool) *CheckBox {
	if c.checked == v {
		return c
	}
	c.checked = v
	return c
}

// OnChanged sets the callback fired with the new checked value whenever the
// user toggles the box (click or Space/Enter) — never for a programmatic
// SetChecked (fluo's uniform contract: programmatic setters are silent;
// OnChanged reports only user-driven changes). Replaces any previously set
// callback; a nil fn is a valid, silent no-op.
func (c *CheckBox) OnChanged(fn func(bool)) *CheckBox {
	c.onChanged = fn
	return c
}

// SetEnabled toggles whether the checkbox accepts focus and pointer/
// keyboard input. Purely visual/behavioral: no invalidation needed.
func (c *CheckBox) SetEnabled(v bool) *CheckBox {
	c.enabled = v
	return c
}

// Label returns the checkbox's label TextBlock, for tests and
// customization.
func (c *CheckBox) Label() *TextBlock {
	return c.label
}

// MeasureContent measures the glyph box plus optional label.
func (c *CheckBox) MeasureContent(available render.Size) render.Size {
	return glyphMeasure(c.label, available, c.metrics.PaddingM)
}

// ArrangeContent arranges the label to the glyph box's right (see
// glyphArrange) and caches the box's own rect for Render.
func (c *CheckBox) ArrangeContent(bounds render.Rect) {
	c.box = glyphArrange(c.label, bounds, c.metrics.PaddingM)
}

// Children returns the label as the checkbox's sole child.
func (c *CheckBox) Children() []core.Widget {
	return []core.Widget{c.label}
}

// stateColors resolves the fill and stroke (zero-alpha means "no stroke")
// for the current checked/enabled state, applying the same hover/pressed
// feedback as Button.stateColors: unchecked walks ControlFill ->
// ControlFillHover -> ControlFillPressed, checked walks Accent ->
// AccentHover -> AccentPressed, both keyed off the embedded ClickBehavior.
func (c *CheckBox) stateColors() (fill, stroke render.Color) {
	th := c.colors
	if !c.enabled {
		if c.checked {
			return th.AccentDisabled, render.Color{}
		}
		return th.ControlFillDisabled, th.ControlStrokeDisabled
	}

	if c.checked {
		fill = th.Accent
		switch {
		case c.click.Pressed():
			fill = th.AccentPressed
		case c.click.Hover():
			fill = th.AccentHover
		}
		return fill, render.Color{}
	}

	fill = th.ControlFill
	switch {
	case c.click.Pressed():
		fill = th.ControlFillPressed
	case c.click.Hover():
		fill = th.ControlFillHover
	}
	return fill, th.ControlStroke
}

// Render paints the 18x18 box (fill + optional stroke, radius 4) and, when
// checked, the checkmark (glyph or fallback square, per c.checkGlyph).
func (c *CheckBox) Render(r render.Renderer) {
	fill, stroke := c.stateColors()
	radius := c.metrics.ControlCornerRadius

	r.FillRoundedRect(c.box, radius, fill)
	if stroke.A > 0 {
		r.StrokeRoundedRect(c.box, radius, c.metrics.StrokeWidth, stroke)
	}
	if c.checked {
		c.drawCheckmark(r)
	}
}

// drawCheckmark draws the U+2713 glyph centered in the box (if c.checkGlyph)
// or the fallback inner rounded square otherwise. Both paths use AccentText
// (white in both bundled Fluent themes) as the checkmark color.
func (c *CheckBox) drawCheckmark(r render.Renderer) {
	glyphColor := c.colors.AccentText
	if !c.enabled {
		glyphColor = c.colors.TextDisabled
	}

	if c.checkGlyph {
		const mark = "✓"
		size := c.face.Measure(mark)
		at := render.Point{
			X: c.box.X + (c.box.W-size.W)/2,
			Y: c.box.Y + (c.box.H-size.H)/2,
		}
		c.face.Draw(r, at, mark, glyphColor)
		return
	}

	inner := c.box.Inset(render.Uniform(checkGlyphInset))
	r.FillRoundedRect(inner, checkGlyphFallbackRadius, glyphColor)
}

// RenderOverlay draws the focus ring around the glyph box while focused,
// per the global focus constraint: StrokeRoundedRect inflated by 2, radius
// = box radius + 2, FocusStroke color and FocusStrokeWidth.
func (c *CheckBox) RenderOverlay(r render.Renderer) {
	if !c.focused {
		return
	}
	drawFocusRing(r, c.box, c.metrics.ControlCornerRadius, c.colors, c.metrics)
}

// AcceptsFocus implements input.Focusable: a disabled checkbox never
// accepts focus.
func (c *CheckBox) AcceptsFocus() bool {
	return c.enabled
}

// OnFocusChanged implements input.FocusHandler, tracking focus for the
// focus-ring overlay and keyboard activation.
func (c *CheckBox) OnFocusChanged(focused bool) {
	c.focused = focused
}

// OnPointer implements input.PointerHandler, delegating to ClickBehavior
// while enabled (disabled ignores pointer input outright, not merely
// failing to fire — e.Handled is left false so the event keeps bubbling).
// The clickable area is the whole composite (box + label), not just the
// box, matching ordinary checkbox UX.
func (c *CheckBox) OnPointer(e *input.PointerEvent) {
	if !c.enabled {
		return
	}
	c.click.HandlePointer(e, c)
}

// OnKey implements input.KeyHandler: Space or Enter, on Press, toggles the
// checkbox (fires OnChanged) and marks the event handled.
func (c *CheckBox) OnKey(e *input.KeyEvent) {
	if !c.enabled || e.Action != input.Press {
		return
	}
	if e.Key == input.KeySpace || e.Key == input.KeyEnter {
		c.click.Activate()
		e.Handled = true
	}
}

// RadioButton is a clickable, focusable, token-styled 18x18 circle with an
// optional label to its right. Unlike CheckBox, clicking never turns a
// checked RadioButton off directly — real radio semantics: selecting one
// deselects its siblings, and re-selecting the already-selected one is a
// no-op (see activate). Add a RadioButton to a RadioGroup for that
// sibling-deselection behavior; a RadioButton with no group behaves like a
// one-way checkbox (click sets it checked, never unchecked, by itself).
//
// Visuals (normative): unchecked = ControlFill fill + ControlStroke stroke,
// radius 9 (a full circle, since 9 == half of 18); checked = ControlFill
// fill + an Accent ring (stroke width 2) + an inner Accent-filled circle
// (9x9, radius 4.5) centered within the outer circle.
type RadioButton struct {
	core.Element

	click ClickBehavior

	label *TextBlock
	box   render.Rect

	checked   bool
	enabled   bool
	focused   bool
	onChanged func(bool)

	group *RadioGroup

	colors  theme.ColorTokens
	metrics theme.MetricTokens
}

// NewRadioButton returns an unchecked, enabled, ungrouped RadioButton
// showing label in face (face may be nil, per TextBlock). Add it to a
// RadioGroup via RadioGroup.Add for mutual-exclusion behavior.
func NewRadioButton(face *text.Face, label string) *RadioButton {
	rb := &RadioButton{}
	th := theme.Active()
	rb.enabled = true
	rb.colors = th.Color
	rb.metrics = th.Metric
	rb.label = NewTextBlock(face, label)
	core.SetParent(rb.label, rb)

	rb.click.OnClick = func() { rb.activate() }
	return rb
}

// activate is the shared click/keyboard entry point. Radio semantics: a
// click on an already-checked radio button is a silent no-op (no callback
// of any kind). Otherwise, if rb belongs to a group, the group handles
// deselecting siblings and firing both rb's own OnChanged and the group's
// OnChanged(index); otherwise rb simply becomes checked and fires its own
// OnChanged(true).
func (rb *RadioButton) activate() {
	if rb.checked {
		return
	}
	if rb.group != nil {
		rb.group.selectExclusive(rb)
		return
	}
	rb.checked = true
	if rb.onChanged != nil {
		rb.onChanged(true)
	}
}

// setCheckedSilent sets checked without ever firing OnChanged — the
// mechanism RadioGroup uses to deselect siblings (a side effect of the
// user's click on a DIFFERENT button, not a direct user action on this
// one) and that SetChecked itself uses for its own programmatic change.
func (rb *RadioButton) setCheckedSilent(v bool) {
	rb.checked = v
}

// Checked reports the current selection state.
func (rb *RadioButton) Checked() bool { return rb.checked }

// SetChecked sets the selection state programmatically and silently (no
// OnChanged, matching CheckBox/ToggleButton's convention). A no-op when v
// already matches the current state. Setting true on a grouped button also
// silently deselects its siblings, preserving the group's mutual-exclusion
// invariant even under direct programmatic control. Fluo's uniform
// contract, restated: programmatic setters are silent; OnChanged reports
// only user-driven changes.
func (rb *RadioButton) SetChecked(v bool) *RadioButton {
	if rb.checked == v {
		return rb
	}
	rb.setCheckedSilent(v)
	if v && rb.group != nil {
		rb.group.deselectOthersSilent(rb)
	}
	return rb
}

// OnChanged sets the callback fired with true whenever the user selects
// this radio button (click or Space/Enter) — never for a programmatic
// SetChecked, and never when this button is deselected as a side effect of
// a sibling being selected (fluo's uniform contract: programmatic setters
// are silent; OnChanged reports only user-driven changes). Replaces any
// previously set callback; a nil fn is a valid, silent no-op.
func (rb *RadioButton) OnChanged(fn func(bool)) *RadioButton {
	rb.onChanged = fn
	return rb
}

// SetEnabled toggles whether the radio button accepts focus and pointer/
// keyboard input.
func (rb *RadioButton) SetEnabled(v bool) *RadioButton {
	rb.enabled = v
	return rb
}

// Label returns the radio button's label TextBlock, for tests and
// customization.
func (rb *RadioButton) Label() *TextBlock {
	return rb.label
}

// MeasureContent measures the glyph circle plus optional label.
func (rb *RadioButton) MeasureContent(available render.Size) render.Size {
	return glyphMeasure(rb.label, available, rb.metrics.PaddingM)
}

// ArrangeContent arranges the label to the glyph circle's right and caches
// the circle's own rect for Render.
func (rb *RadioButton) ArrangeContent(bounds render.Rect) {
	rb.box = glyphArrange(rb.label, bounds, rb.metrics.PaddingM)
}

// Children returns the label as the radio button's sole child.
func (rb *RadioButton) Children() []core.Widget {
	return []core.Widget{rb.label}
}

// stateColors resolves the fill, ring stroke, ring stroke width, and inner
// dot color (zero-alpha dot means "no dot", i.e. unchecked) for the current
// checked/enabled state, applying the same hover/pressed feedback as
// Button/CheckBox: the base circle fill always walks ControlFill ->
// ControlFillHover -> ControlFillPressed (per the embedded ClickBehavior),
// and when checked the ring+dot additionally walk Accent -> AccentHover ->
// AccentPressed together (they're both "the accent chrome").
//
// Disabled marker note: the inner dot uses TextDisabled (not AccentDisabled)
// when disabled+checked, matching CheckBox's disabled checkmark and
// ToggleSwitch's disabled thumb — both put a TextDisabled marker on top of
// an otherwise-disabled-accent-colored chrome, for contrast. The ring itself
// stays AccentDisabled, since it IS that chrome, not the marker on it.
func (rb *RadioButton) stateColors() (fill, ring render.Color, ringWidth float32, dot render.Color) {
	th := rb.colors
	if !rb.enabled {
		if rb.checked {
			return th.ControlFillDisabled, th.AccentDisabled, radioRingWidth, th.TextDisabled
		}
		return th.ControlFillDisabled, th.ControlStrokeDisabled, rb.metrics.StrokeWidth, render.Color{}
	}

	fill = th.ControlFill
	switch {
	case rb.click.Pressed():
		fill = th.ControlFillPressed
	case rb.click.Hover():
		fill = th.ControlFillHover
	}

	if rb.checked {
		accent := th.Accent
		switch {
		case rb.click.Pressed():
			accent = th.AccentPressed
		case rb.click.Hover():
			accent = th.AccentHover
		}
		return fill, accent, radioRingWidth, accent
	}
	return fill, th.ControlStroke, rb.metrics.StrokeWidth, render.Color{}
}

// Render paints the 18x18 circle (fill + ring stroke) and, when checked,
// the centered 9x9 inner dot.
func (rb *RadioButton) Render(r render.Renderer) {
	fill, ring, ringWidth, dot := rb.stateColors()
	radius := rb.box.W / 2

	r.FillRoundedRect(rb.box, radius, fill)
	if ring.A > 0 {
		r.StrokeRoundedRect(rb.box, radius, ringWidth, ring)
	}
	if dot.A > 0 {
		inset := (glyphBoxSize - radioInnerSize) / 2
		inner := rb.box.Inset(render.Uniform(inset))
		r.FillRoundedRect(inner, radioInnerRadius, dot)
	}
}

// RenderOverlay draws the focus ring around the glyph circle while
// focused.
func (rb *RadioButton) RenderOverlay(r render.Renderer) {
	if !rb.focused {
		return
	}
	drawFocusRing(r, rb.box, rb.box.W/2, rb.colors, rb.metrics)
}

// AcceptsFocus implements input.Focusable: a disabled radio button never
// accepts focus.
func (rb *RadioButton) AcceptsFocus() bool {
	return rb.enabled
}

// OnFocusChanged implements input.FocusHandler.
func (rb *RadioButton) OnFocusChanged(focused bool) {
	rb.focused = focused
}

// OnPointer implements input.PointerHandler, delegating to ClickBehavior
// while enabled.
func (rb *RadioButton) OnPointer(e *input.PointerEvent) {
	if !rb.enabled {
		return
	}
	rb.click.HandlePointer(e, rb)
}

// OnKey implements input.KeyHandler: Space or Enter, on Press, activates
// (selects) the radio button.
func (rb *RadioButton) OnKey(e *input.KeyEvent) {
	if !rb.enabled || e.Action != input.Press {
		return
	}
	if e.Key == input.KeySpace || e.Key == input.KeyEnter {
		rb.click.Activate()
		e.Handled = true
	}
}

// RadioGroup coordinates mutual exclusion across a set of RadioButtons:
// selecting one (via click, Space/Enter, or SetChecked(true)) deselects
// every other member, and OnChanged fires with the newly selected member's
// index (its position in Add call order).
//
// v0 caveat: RadioGroup holds plain *RadioButton pointers in members and has
// no Remove — a button, once Add-ed, stays a member (and keeps its index)
// for the group's lifetime. Building a group whose membership changes at
// runtime is out of scope for now; construct a fresh RadioGroup instead.
type RadioGroup struct {
	members   []*RadioButton
	onChanged func(int)
}

// NewRadioGroup returns an empty RadioGroup.
func NewRadioGroup() *RadioGroup {
	return &RadioGroup{}
}

// Add registers rb as a member of the group, wiring it into the group's
// mutual-exclusion behavior. Returns g for chaining (mirroring
// StackPanel.Add's style).
func (g *RadioGroup) Add(rb *RadioButton) *RadioGroup {
	rb.group = g
	g.members = append(g.members, rb)
	return g
}

// OnChanged sets the callback fired with the newly selected member's index
// whenever the user selects a DIFFERENT member of the group than the one
// currently checked (never when re-selecting the same one, and never for a
// programmatic SetChecked — fluo's uniform contract: programmatic setters
// are silent; OnChanged reports only user-driven changes). Replaces any
// previously set callback; a nil fn is a valid, silent no-op.
func (g *RadioGroup) OnChanged(fn func(int)) *RadioGroup {
	g.onChanged = fn
	return g
}

// selectExclusive is RadioButton.activate's group path: deselects every
// other member (silently), selects rb, fires rb's own OnChanged(true), and
// fires the group's OnChanged with rb's index.
func (g *RadioGroup) selectExclusive(rb *RadioButton) {
	idx := g.deselectOthersSilent(rb)
	rb.setCheckedSilent(true)
	if rb.onChanged != nil {
		rb.onChanged(true)
	}
	if idx >= 0 && g.onChanged != nil {
		g.onChanged(idx)
	}
}

// deselectOthersSilent sets every member other than rb to unchecked
// (silently, firing no callbacks) and returns rb's index within the group,
// or -1 if rb is not a member of g.
func (g *RadioGroup) deselectOthersSilent(rb *RadioButton) int {
	idx := -1
	for i, m := range g.members {
		if m == rb {
			idx = i
			continue
		}
		m.setCheckedSilent(false)
	}
	return idx
}
