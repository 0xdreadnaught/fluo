package controls

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
	"github.com/0xdreadnaught/fluo/theme"
)

// expanderHeader is Expander's header row: a 'v'/'>' chevron plus a title,
// drawn with the classic raised/sunken ButtonFace bevel chrome (via its own
// embedded ClickBehavior, mirroring Button's Render) exactly like Button —
// but STRETCHED to whatever width its parent (Expander) arranges it at,
// rather than sized to its own content, since Element's zero-value alignment
// is already Stretch (see core.Element's doc comment) and expanderHeader
// never overrides it: this is what makes the header "full-width" per the
// brief, with no special-case width logic of its own. Clicking anywhere on
// the header (not just the chevron glyph or the title text — the WHOLE row,
// via its own Render painting the bevel across its full arranged bounds)
// toggles the owning Expander, wired via click.OnClick in newExpanderHeader.
type expanderHeader struct {
	core.Element

	click ClickBehavior

	chevron *TextBlock
	title   *TextBlock

	focused bool

	colors  theme.ColorTokens
	metrics theme.MetricTokens
}

// newExpanderHeader returns an expanderHeader showing title in face,
// starting with the collapsed '>' chevron (Expander's own constructor calls
// setExpandedGlyph if the Expander should start expanded — v0 always starts
// collapsed, see NewExpander).
func newExpanderHeader(face *text.Face, title string, colors theme.ColorTokens, metrics theme.MetricTokens) *expanderHeader {
	h := &expanderHeader{colors: colors, metrics: metrics}
	h.chevron = NewTextBlock(face, ">")
	h.chevron.SetColor(colors.WindowText)
	core.SetParent(h.chevron, h)
	h.title = NewTextBlock(face, title)
	h.title.SetColor(colors.WindowText)
	core.SetParent(h.title, h)
	return h
}

// setExpandedGlyph updates the chevron glyph to match expanded ('v') or
// collapsed ('>') — text glyphs, never rotated, per the brief.
func (h *expanderHeader) setExpandedGlyph(expanded bool) {
	glyph := ">"
	if expanded {
		glyph = "v"
	}
	h.chevron.SetText(glyph)
}

// padding returns the header's content inset: PaddingL horizontal, PaddingM
// vertical — identical to Button's chrome padding, since a header row reads
// as a full-width button.
func (h *expanderHeader) padding() render.Thickness {
	return render.Thickness{
		Left: h.metrics.PaddingL, Right: h.metrics.PaddingL,
		Top: h.metrics.PaddingM, Bottom: h.metrics.PaddingM,
	}
}

// MeasureContent measures the chevron first (it never shrinks to make room
// for the title), then the title in whatever width remains after the
// chevron and the gap between them, and reports their combined width (plus
// padding) as the header's own (unstretched) desired size — mirroring
// ComboBox.MeasureContent's chevron-then-label layout, just left-to-right
// reversed (chevron THEN title, rather than label then chevron).
func (h *expanderHeader) MeasureContent(available render.Size) render.Size {
	pad := h.padding()
	gap := h.metrics.PaddingS

	availW := available.W - pad.Left - pad.Right
	if availW < 0 {
		availW = 0
	}
	availH := available.H - pad.Top - pad.Bottom
	if availH < 0 {
		availH = 0
	}

	core.MeasureWidget(h.chevron, render.Size{W: availW, H: availH})
	chevD := core.DesiredSizeOf(h.chevron)

	titleAvailW := availW - chevD.W - gap
	if titleAvailW < 0 {
		titleAvailW = 0
	}
	core.MeasureWidget(h.title, render.Size{W: titleAvailW, H: availH})
	titleD := core.DesiredSizeOf(h.title)

	hgt := chevD.H
	if titleD.H > hgt {
		hgt = titleD.H
	}

	return render.Size{
		W: chevD.W + gap + titleD.W + pad.Left + pad.Right,
		H: hgt + pad.Top + pad.Bottom,
	}
}

// ArrangeContent arranges the chevron at the padded inner rect's left edge
// (vertically centered) and the title to its right with a gap (also
// vertically centered) — never stretched to fill any leftover width between
// them, even though the header ITSELF stretches to fill whatever bounds its
// parent gives it (see the type doc comment).
func (h *expanderHeader) ArrangeContent(bounds render.Rect) {
	pad := h.padding()
	inner := bounds.Inset(pad)
	if inner.W < 0 {
		inner.W = 0
	}
	if inner.H < 0 {
		inner.H = 0
	}

	chevD := core.DesiredSizeOf(h.chevron)
	chevY := inner.Y + (inner.H-chevD.H)/2
	core.ArrangeWidget(h.chevron, render.Rect{X: inner.X, Y: chevY, W: chevD.W, H: chevD.H})

	gap := h.metrics.PaddingS
	titleD := core.DesiredSizeOf(h.title)
	titleX := inner.X + chevD.W + gap
	titleY := inner.Y + (inner.H-titleD.H)/2
	core.ArrangeWidget(h.title, render.Rect{X: titleX, Y: titleY, W: titleD.W, H: titleD.H})
}

// Children returns the chevron and title.
func (h *expanderHeader) Children() []core.Widget {
	return []core.Widget{h.chevron, h.title}
}

// Render paints the header's classic chiseled chrome across its full
// arranged bounds (which, per the type doc comment, stretch to the
// Expander's own width) via the shared bevel helpers, mirroring Button's own
// Render: normal = drawRaised(ButtonFace); hover (not pressed) =
// drawRaised(ButtonLight); pressed = drawSunken(ButtonFace). The header has
// no disabled concept (see AcceptsFocus), so there is no engrave/GrayText
// path to mirror.
func (h *expanderHeader) Render(r render.Renderer) {
	c := h.colors
	bounds := h.Bounds()

	switch {
	case h.click.Pressed():
		drawSunken(r, bounds, c.ButtonFace, c)
	case h.click.Hover():
		drawRaised(r, bounds, c.ButtonLight, c)
	default:
		drawRaised(r, bounds, c.ButtonFace, c)
	}
}

// RenderOverlay draws the focus ring while focused, per the global focus
// constraint shared by every focusable control in this package.
func (h *expanderHeader) RenderOverlay(r render.Renderer) {
	if !h.focused {
		return
	}
	drawFocusRing(r, h.Bounds(), h.colors)
}

// AcceptsFocus implements input.Focusable: the header always accepts focus
// (v0 has no disabled concept for Expander).
func (h *expanderHeader) AcceptsFocus() bool {
	return true
}

// OnFocusChanged implements input.FocusHandler.
func (h *expanderHeader) OnFocusChanged(focused bool) {
	h.focused = focused
}

// OnPointer implements input.PointerHandler, delegating the entire
// press/release/hover state machine to the embedded ClickBehavior. Note this
// is what SCOPES a click to just the header's own bounds without any manual
// position math: input.HitPath only includes expanderHeader on the hit path
// for a point within h.Bounds() itself (Expander's content, arranged below
// the header, has its own separate — and non-overlapping — bounds), so a
// click on the content never reaches here at all.
func (h *expanderHeader) OnPointer(e *input.PointerEvent) {
	h.click.HandlePointer(e, h)
}

// OnKey implements input.KeyHandler: Space or Enter, on Press, activates the
// header (toggles the owning Expander) and marks the event handled.
func (h *expanderHeader) OnKey(e *input.KeyEvent) {
	if e.Action != input.Press {
		return
	}
	if e.Key == input.KeySpace || e.Key == input.KeyEnter {
		h.click.Activate()
		e.Handled = true
	}
}

// Expander is a collapsible container: a full-width, clickable header row
// ('v'/'>' chevron + title — see expanderHeader) that toggles whether its
// content widget is shown below it. Expander itself draws no chrome of its
// own (no Render override — matching StackPanel, a pure layout composite);
// all visible chrome belongs to the header.
//
// Normative: content participates in layout (measured, arranged, and
// rendered) ONLY while expanded — see MeasureContent/ArrangeContent/
// Children, all three of which skip content entirely while collapsed, not
// merely hide it after the fact. SetExpanded is a silent, programmatic
// setter (fluo's uniform contract); OnChanged fires only for a user-driven
// toggle (a header click, or Space/Enter while the header is focused).
type Expander struct {
	core.Element

	header  *expanderHeader
	content core.Widget

	expanded  bool
	onChanged func(bool)
}

// NewExpander returns a collapsed Expander with the given header title,
// drawing header text with face (face may be nil, per TextBlock).
func NewExpander(face *text.Face, header string) *Expander {
	th := theme.Active()
	e := &Expander{}
	e.header = newExpanderHeader(face, header, th.Color, th.Metric)
	core.SetParent(e.header, e)

	e.header.click.OnClick = func() {
		e.setExpandedUser(!e.expanded)
	}
	return e
}

// SetContent sets (replacing any existing) the content widget shown below
// the header while expanded, re-parenting it to this Expander and
// invalidating measure. Any previously set content is detached (its parent
// cleared) so its future invalidations stop climbing into this Expander.
func (e *Expander) SetContent(w core.Widget) *Expander {
	if e.content != nil {
		core.SetParent(e.content, nil)
	}
	e.content = w
	if w != nil {
		core.SetParent(w, e)
	}
	e.InvalidateMeasure()
	return e
}

// Expanded reports whether the content is currently shown.
func (e *Expander) Expanded() bool {
	return e.expanded
}

// SetExpanded sets the expanded state programmatically and silently — never
// fires OnChanged, matching fluo's uniform contract that programmatic
// setters are silent (ToggleButton.SetChecked, CheckBox.SetChecked, ...):
// OnChanged reports only user-driven changes (a header click or Space/
// Enter). A no-op when v already matches the current state (content's
// layout participation is unchanged either way, so there's nothing to
// invalidate).
func (e *Expander) SetExpanded(v bool) *Expander {
	if e.expanded == v {
		return e
	}
	e.expanded = v
	e.header.setExpandedGlyph(v)
	e.InvalidateMeasure()
	return e
}

// setExpandedUser is the header-click/keyboard-activation path: applies the
// same state change as SetExpanded, but fires OnChanged (only on an actual
// change), matching every other composite's SetChecked-vs-user-toggle split
// in this package.
func (e *Expander) setExpandedUser(v bool) {
	if e.expanded == v {
		return
	}
	e.expanded = v
	e.header.setExpandedGlyph(v)
	e.InvalidateMeasure()
	if e.onChanged != nil {
		e.onChanged(v)
	}
}

// OnChanged sets the callback fired with the new expanded value whenever the
// user toggles the header (click or Space/Enter while it's focused) — never
// for a programmatic SetExpanded (fluo's uniform contract: programmatic
// setters are silent; OnChanged reports only user-driven changes). Replaces
// any previously set callback; a nil fn is a valid, silent no-op.
func (e *Expander) OnChanged(fn func(bool)) *Expander {
	e.onChanged = fn
	return e
}

// MeasureContent measures the header unconditionally (it is always shown),
// then — ONLY while expanded and a content widget is set — measures content
// too, in the width available and whatever height remains after the
// header's own desired height. Collapsed (or no content), content is never
// measured at all: this is what "content participates in layout ONLY when
// expanded" means concretely, not merely "content is hidden after being
// measured/arranged as usual" — measuring with the SAME Expander collapsed
// vs expanded genuinely produces two different desired sizes (see the
// package's Expander tests), the observable proof this rule holds.
func (e *Expander) MeasureContent(available render.Size) render.Size {
	core.MeasureWidget(e.header, available)
	headerD := core.DesiredSizeOf(e.header)

	w := headerD.W
	h := headerD.H

	if e.expanded && e.content != nil {
		availH := available.H - headerD.H
		if availH < 0 {
			availH = 0
		}
		core.MeasureWidget(e.content, render.Size{W: available.W, H: availH})
		contentD := core.DesiredSizeOf(e.content)
		if contentD.W > w {
			w = contentD.W
		}
		h += contentD.H
	}

	return render.Size{W: w, H: h}
}

// ArrangeContent arranges the header across the full bounds width at its own
// measured height, then — ONLY while expanded and a content widget is set —
// arranges content directly below it, also at the full bounds width.
// Collapsed (or no content), content is never arranged at all, mirroring
// MeasureContent.
func (e *Expander) ArrangeContent(bounds render.Rect) {
	headerD := core.DesiredSizeOf(e.header)
	core.ArrangeWidget(e.header, render.Rect{X: bounds.X, Y: bounds.Y, W: bounds.W, H: headerD.H})

	if e.expanded && e.content != nil {
		contentD := core.DesiredSizeOf(e.content)
		core.ArrangeWidget(e.content, render.Rect{X: bounds.X, Y: bounds.Y + headerD.H, W: bounds.W, H: contentD.H})
	}
}

// Children returns the header alone while collapsed (or with no content
// set), or the header plus content while expanded — the render-time half of
// "content participates in layout ONLY when expanded" (core.RenderWidget
// draws exactly what Children() returns, so collapsed content is neither
// arranged NOR walked for rendering, not merely arranged-then-skipped).
func (e *Expander) Children() []core.Widget {
	if e.expanded && e.content != nil {
		return []core.Widget{e.header, e.content}
	}
	return []core.Widget{e.header}
}
