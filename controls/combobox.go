package controls

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
	"github.com/0xdreadnaught/fluo/theme"
)

// comboPlaceholder is the text shown (in GrayText) when SelectedIndex()
// is -1 (no selection made yet).
const comboPlaceholder = "Select…"

// ComboBox is a clickable, focusable, token-styled dropdown: a Button-like
// field showing the selected item's text (or comboPlaceholder) plus a 'v'
// chevron, which opens a popup (via OverlayHostFor) listing every item as a
// clickable row when clicked, or on Space/Enter/Down while focused.
//
// ComboBox is the first consumer of OverlayHost alongside ToolTipArea (Phase
// 5 Task 8). It follows the overlay's documented conventions throughout:
//
//   - The field's own click (embedded ClickBehavior) opens the popup from
//     OnClick, which HandlePointer's Release branch fires AFTER releasing
//     the pointer capture (see ClickBehavior.HandlePointer) — so ShowPopup is
//     never called while the field itself holds capture, per OverlayHost's
//     documented convention.
//   - onDismiss (passed to ShowPopup) is the SINGLE place c.open is reset to
//     false: both the explicit close paths below (Esc, item selection) route
//     through OverlayHost.ClosePopup, which always fires onDismiss exactly
//     once, whether the popup closed via light-dismiss or an explicit call.
//   - The field stays focused the entire time the popup is open (nothing in
//     the popup itself is focusable), so Esc naturally reaches ComboBox.OnKey
//     via the router's focused-widget key dispatch.
type ComboBox struct {
	core.Element

	click ClickBehavior

	face    *text.Face
	label   *TextBlock
	chevron *TextBlock

	items    []string
	selected int       // -1 == none
	ta       typeAhead // type-ahead prefix state (see OnKey)

	enabled bool
	focused bool
	open    bool

	// popup is the currently open popup widget (nil when closed), kept so
	// closePopup can pass the exact instance back to OverlayHost.ClosePopup.
	popup core.Widget

	onChanged func(int)

	colors  theme.ColorTokens
	metrics theme.MetricTokens
}

// NewComboBox returns an enabled, closed ComboBox with no items and no
// selection (SelectedIndex() == -1, showing comboPlaceholder), drawing text
// with face (face may be nil, per TextBlock).
func NewComboBox(face *text.Face) *ComboBox {
	th := theme.Active()
	c := &ComboBox{
		face:     face,
		selected: -1,
		enabled:  true,
		colors:   th.Color,
		metrics:  th.Metric,
	}
	c.label = NewTextBlock(face, comboPlaceholder)
	core.SetParent(c.label, c)
	c.chevron = NewTextBlock(face, "v")
	c.chevron.SetColor(th.Color.WindowText)
	core.SetParent(c.chevron, c)

	c.click.OnClick = func() { c.openPopup() }
	return c
}

// clampSelectedIndex clamps i into [-1, n-1] (n == 0 collapses to -1, since
// n-1 == -1 in that case — no separate empty-items special case needed).
func clampSelectedIndex(i, n int) int {
	if i < -1 {
		return -1
	}
	if i > n-1 {
		return n - 1
	}
	return i
}

// labelText returns comboPlaceholder when nothing is selected, else the
// selected item's text. Exported-adjacent white-box helper for tests (the
// field's displayed text is otherwise only observable via rendering).
func (c *ComboBox) labelText() string {
	if c.selected < 0 || c.selected >= len(c.items) {
		return comboPlaceholder
	}
	return c.items[c.selected]
}

// SetItems replaces the item list, re-clamping the current selection into
// the new range (see clampSelectedIndex) and refreshing the field's label.
// Does NOT fire OnChanged, even if the clamp changes SelectedIndex() — like
// SetSelectedIndex, this is a silent, programmatic setter (fluo's uniform
// contract: programmatic setters are silent; OnChanged reports only
// user-driven changes).
func (c *ComboBox) SetItems(items []string) *ComboBox {
	c.items = append([]string(nil), items...)
	c.selected = clampSelectedIndex(c.selected, len(c.items))
	c.label.SetText(c.labelText())
	c.InvalidateMeasure()
	return c
}

// SelectedIndex returns the current selection, or -1 if none.
func (c *ComboBox) SelectedIndex() int {
	return c.selected
}

// SetSelectedIndex sets the selection programmatically, clamped into
// [-1, len(items)-1] (see clampSelectedIndex) — -1 is always a valid,
// explicit "no selection" value, not merely a clamp target. Silent: never
// fires OnChanged, matching the CheckBox/ToggleButton/RadioButton
// SetChecked convention (programmatic changes are silent; only user-driven
// ones — a click or Space/Enter/Down-then-click — notify). Fluo's uniform
// contract, restated: programmatic setters are silent; OnChanged reports
// only user-driven changes.
func (c *ComboBox) SetSelectedIndex(i int) *ComboBox {
	c.selected = clampSelectedIndex(i, len(c.items))
	c.label.SetText(c.labelText())
	return c
}

// selectUser is the item-click path: clamps i, updates the field label, and
// fires OnChanged only if the selection actually changed — re-clicking the
// already-selected row is a no-op notification, matching Slider.setValue's
// "notify only on real change" convention rather than CheckBox's
// always-fire-on-user-action one (a combo selection is closer to bound data
// than a boolean toggle).
func (c *ComboBox) selectUser(i int) {
	i = clampSelectedIndex(i, len(c.items))
	changed := i != c.selected
	c.selected = i
	c.label.SetText(c.labelText())
	if changed && c.onChanged != nil {
		c.onChanged(i)
	}
}

// OnChanged sets the callback fired with the new index whenever the user
// selects a (different) item by clicking a popup row — never for a
// programmatic SetSelectedIndex or SetItems (fluo's uniform contract:
// programmatic setters are silent; OnChanged reports only user-driven
// changes). Replaces any previously set callback; a nil fn is a valid,
// silent no-op.
func (c *ComboBox) OnChanged(fn func(int)) *ComboBox {
	c.onChanged = fn
	return c
}

// SetEnabled toggles whether the combo accepts focus and pointer/keyboard
// input (both OnPointer and OnKey ignore all input while disabled, and
// AcceptsFocus returns false). Purely visual/behavioral: no invalidation
// needed.
func (c *ComboBox) SetEnabled(v bool) *ComboBox {
	c.enabled = v
	return c
}

// IsOpen reports whether the popup is currently showing.
func (c *ComboBox) IsOpen() bool {
	return c.open
}

// openPopup is the normative single entry point for showing the popup —
// called from both the field's click (ClickBehavior.OnClick) and the
// keyboard path (OnKey's Space/Enter/Down). A no-op if already open, or if
// this ComboBox isn't (yet) attached beneath an OverlayHost.
func (c *ComboBox) openPopup() {
	if c.open {
		return
	}
	host := OverlayHostFor(c)
	if host == nil {
		return
	}

	c.open = true
	popup := c.buildPopup()
	c.popup = popup

	anchor := c.Bounds()
	// Pass the ComboBox as the popup's OWNER (via the same-package showPopup)
	// so the OverlayHost auto-closes the dropdown if this ComboBox is hidden
	// (a tab switch) or detached (SetContent) while it's open — otherwise the
	// dropdown lingers, its modal capture stays engaged, and c.open never
	// resets, leaving the box dead for life (see popupEntry.owner / ownerLive).
	host.showPopup(popup, anchor, func() {
		c.open = false
		c.popup = nil
	}, true, false, c)
}

// closePopup explicitly closes the open popup (Esc, or a row's own click) by
// routing through OverlayHost.ClosePopup — which fires the onDismiss set in
// openPopup exactly once, resetting c.open/c.popup. A no-op if not open, or
// if the host can no longer be found (defensive; should not happen while
// open, since the popup can only have been shown via a found host).
func (c *ComboBox) closePopup() {
	if !c.open {
		return
	}
	host := OverlayHostFor(c)
	if host != nil && c.popup != nil {
		host.ClosePopup(c.popup)
	}
}

// buildPopup constructs a fresh popup widget from the CURRENT items and
// selection: a Card-background, shadowed comboPopupCard wrapping a vertical
// StackPanel of comboRow items. Built fresh on every open (rather than kept
// around and mutated) so it always reflects whatever SetItems/
// SetSelectedIndex calls happened while closed.
func (c *ComboBox) buildPopup() core.Widget {
	stack := NewStackPanel(Vertical)
	for i, item := range c.items {
		row := newComboRow(c.face, item, i, i == c.selected, c.colors, c.metrics, func(idx int) {
			c.selectUser(idx)
			c.closePopup()
		})
		stack.Add(row)
	}
	return newComboPopupCard(stack, c.colors, c.metrics)
}

// padding returns the field's content inset: PaddingL horizontal, PaddingM
// vertical, matching Button's chrome padding.
func (c *ComboBox) padding() render.Thickness {
	return render.Thickness{
		Left: c.metrics.PaddingL, Right: c.metrics.PaddingL,
		Top: c.metrics.PaddingM, Bottom: c.metrics.PaddingM,
	}
}

// MeasureContent measures the chevron first (it never shrinks to make room
// for the label), then the label in whatever width remains after the
// chevron and the gap between them, and reports their combined width (plus
// padding) as the field's desired size.
func (c *ComboBox) MeasureContent(available render.Size) render.Size {
	pad := c.padding()
	gap := c.metrics.PaddingM

	availW := available.W - pad.Left - pad.Right
	if availW < 0 {
		availW = 0
	}
	availH := available.H - pad.Top - pad.Bottom
	if availH < 0 {
		availH = 0
	}

	core.MeasureWidget(c.chevron, render.Size{W: availW, H: availH})
	chevD := core.DesiredSizeOf(c.chevron)

	labelAvailW := availW - chevD.W - gap
	if labelAvailW < 0 {
		labelAvailW = 0
	}
	core.MeasureWidget(c.label, render.Size{W: labelAvailW, H: availH})
	labelD := core.DesiredSizeOf(c.label)

	h := labelD.H
	if chevD.H > h {
		h = chevD.H
	}

	return render.Size{
		W: labelD.W + gap + chevD.W + pad.Left + pad.Right,
		H: h + pad.Top + pad.Bottom,
	}
}

// fieldSplit divides the field's own bounds into the sunken text well (left)
// and the raised drop-button strip (right, square — its width equals the
// field's height, the classic Windows combo-button proportion), clamping the
// button to bounds.W so a degenerately narrow field never produces a
// negative-width well.
func (c *ComboBox) fieldSplit(bounds render.Rect) (well, dropButton render.Rect) {
	btnW := bounds.H
	if btnW > bounds.W {
		btnW = bounds.W
	}
	well = render.Rect{X: bounds.X, Y: bounds.Y, W: bounds.W - btnW, H: bounds.H}
	dropButton = render.Rect{X: bounds.X + bounds.W - btnW, Y: bounds.Y, W: btnW, H: bounds.H}
	return well, dropButton
}

// ArrangeContent places the label inside the sunken well, inset by the 2px
// bevel plus PaddingS (vertically centered, never stretched), and centers the
// chevron glyph within the raised drop-button strip (see fieldSplit) — the
// classic combo layout: a text well on the left, a square drop button on the
// right, rather than the pre-restyle "label plus trailing chevron" flow.
func (c *ComboBox) ArrangeContent(bounds render.Rect) {
	well, dropButton := c.fieldSplit(bounds)

	bw := c.metrics.BevelWidth
	inner := well.Inset(render.Thickness{
		Left: bw + c.metrics.PaddingS, Right: bw + c.metrics.PaddingS,
		Top: bw, Bottom: bw,
	})
	if inner.W < 0 {
		inner.W = 0
	}
	if inner.H < 0 {
		inner.H = 0
	}

	labelD := core.DesiredSizeOf(c.label)
	labelY := inner.Y + (inner.H-labelD.H)/2
	core.ArrangeWidget(c.label, render.Rect{X: inner.X, Y: labelY, W: labelD.W, H: labelD.H})

	chevD := core.DesiredSizeOf(c.chevron)
	chevX := dropButton.X + (dropButton.W-chevD.W)/2
	chevY := dropButton.Y + (dropButton.H-chevD.H)/2
	core.ArrangeWidget(c.chevron, render.Rect{X: chevX, Y: chevY, W: chevD.W, H: chevD.H})
}

// Children returns the label and chevron.
func (c *ComboBox) Children() []core.Widget {
	return []core.Widget{c.label, c.chevron}
}

// Render paints the classic field chrome — a sunken white text well (see
// fieldSplit) and a raised drop-button strip, sunken instead while the
// button is pressed (ClickBehavior.Pressed(), the field's own click state:
// v0 has no separate press target for just the button, the whole field
// opens the popup) — and recolors the label (GrayText while showing the
// placeholder or disabled, else WindowText) and chevron (WindowText, or
// GrayText while disabled) for the current state; children (label, chevron)
// render separately via core.RenderWidget.
func (c *ComboBox) Render(r render.Renderer) {
	bounds := c.Bounds()
	well, dropButton := c.fieldSplit(bounds)

	drawSunken(r, well, c.colors.WindowWell, c.colors)
	if c.click.Pressed() {
		drawSunken(r, dropButton, c.colors.ButtonFace, c.colors)
	} else {
		drawRaised(r, dropButton, c.colors.ButtonFace, c.colors)
	}

	labelColor := c.colors.WindowText
	switch {
	case !c.enabled:
		labelColor = c.colors.GrayText
	case c.selected < 0:
		labelColor = c.colors.GrayText
	}
	c.label.SetColor(labelColor)

	chevColor := c.colors.WindowText
	if !c.enabled {
		chevColor = c.colors.GrayText
	}
	c.chevron.SetColor(chevColor)
}

// RenderOverlay draws the focus ring while focused, per the global focus
// constraint shared by every focusable control in this package.
func (c *ComboBox) RenderOverlay(r render.Renderer) {
	if !c.focused {
		return
	}
	drawFocusRing(r, c.Bounds(), c.colors)
}

// AcceptsFocus implements input.Focusable: a disabled combo never accepts
// focus.
func (c *ComboBox) AcceptsFocus() bool {
	return c.enabled
}

// OnFocusChanged implements input.FocusHandler, tracking focus for the
// focus-ring overlay and keyboard activation.
func (c *ComboBox) OnFocusChanged(focused bool) {
	c.focused = focused
}

// OnPointer implements input.PointerHandler, delegating to ClickBehavior
// while enabled (disabled ignores pointer input outright, not merely
// failing to fire — e.Handled is left false so the event keeps bubbling).
// ClickBehavior's OnClick (wired to openPopup in NewComboBox) fires on
// release-inside, AFTER the pointer capture has already been released — see
// the type doc comment's overlay-convention note.
func (c *ComboBox) OnPointer(e *input.PointerEvent) {
	if !c.enabled {
		return
	}
	c.click.HandlePointer(e, c)
}

// OnKey implements input.KeyHandler: Space, Enter, or Down open the popup
// (if not already open); Escape closes it (if open). Ignored entirely while
// disabled or for anything but Action==Press.
func (c *ComboBox) OnKey(e *input.KeyEvent) {
	if !c.enabled || e.Action != input.Press {
		return
	}
	switch e.Key {
	case input.KeyEscape:
		if c.open {
			c.closePopup()
			e.Handled = true
		}
	case input.KeySpace, input.KeyEnter, input.KeyDown:
		if !c.open {
			c.openPopup()
		}
		e.Handled = true
	default:
		// Type-ahead: a printable key selects the next item matching the typed
		// prefix, whether the list is open or closed — the standard dropdown
		// convention. See typeAhead.feed.
		if e.Rune != 0 && e.Mods&(input.ModCtrl|input.ModAlt) == 0 {
			if next, ok := c.ta.feed(e.Time, e.Rune, len(c.items), c.selected, func(i int) string { return c.items[i] }); ok {
				c.selectUser(next)
				e.Handled = true
			}
		}
	}
}

// comboPopupCard is the ComboBox popup's outer chrome: a raised ButtonFace
// frame (drawRaised) wrapping a single child (the item StackPanel), inset by
// the 2px bevel so the child sits inside the frame rather than over it — the
// classic list-popup look (a raised border framing a WindowWell list area),
// replacing the pre-restyle rounded, drop-shadowed Card chrome.
type comboPopupCard struct {
	core.Element

	child core.Widget

	colors  theme.ColorTokens
	metrics theme.MetricTokens
}

// newComboPopupCard returns a comboPopupCard wrapping child (re-parented to
// it).
func newComboPopupCard(child core.Widget, colors theme.ColorTokens, metrics theme.MetricTokens) *comboPopupCard {
	card := &comboPopupCard{child: child, colors: colors, metrics: metrics}
	core.SetParent(child, card)
	return card
}

// innerBounds returns the card's own bounds inset by the 2px bevel — the
// WindowWell list area the raised frame encloses (see Render/ArrangeContent).
func (card *comboPopupCard) innerBounds(bounds render.Rect) render.Rect {
	bw := card.metrics.BevelWidth
	inner := bounds.Inset(render.Thickness{Top: bw, Bottom: bw, Left: bw, Right: bw})
	if inner.W < 0 {
		inner.W = 0
	}
	if inner.H < 0 {
		inner.H = 0
	}
	return inner
}

// MeasureContent measures the child within the available space reduced by
// the 2px bevel on every side, then adds the bevel back to its desired size
// — the child (the item StackPanel) never overlaps the raised frame.
func (card *comboPopupCard) MeasureContent(available render.Size) render.Size {
	bw := card.metrics.BevelWidth
	innerAvail := render.Size{W: available.W - 2*bw, H: available.H - 2*bw}
	if innerAvail.W < 0 {
		innerAvail.W = 0
	}
	if innerAvail.H < 0 {
		innerAvail.H = 0
	}
	core.MeasureWidget(card.child, innerAvail)
	d := core.DesiredSizeOf(card.child)
	return render.Size{W: d.W + 2*bw, H: d.H + 2*bw}
}

// ArrangeContent arranges the child to fill the card's inner (bevel-inset)
// bounds, so item rows sit inside the raised frame rather than over it.
func (card *comboPopupCard) ArrangeContent(bounds render.Rect) {
	core.ArrangeWidget(card.child, card.innerBounds(bounds))
}

// Children returns the single child.
func (card *comboPopupCard) Children() []core.Widget {
	return []core.Widget{card.child}
}

// Render draws the raised ButtonFace frame (replacing the old rounded
// shadow+stroke chrome — the bevel itself now reads as the popup's border),
// then fills the inner (bevel-inset) list area with WindowWell, the classic
// combo popup's white list background.
func (card *comboPopupCard) Render(r render.Renderer) {
	bounds := card.Bounds()
	drawRaised(r, bounds, card.colors.ButtonFace, card.colors)
	r.FillRect(card.innerBounds(bounds), card.colors.WindowWell)
}

// comboRow is one item row inside an open ComboBox's popup: a left-aligned
// TextBlock, filled Highlight when it is EITHER hovered OR the currently
// selected item (see Render — classic combos give both the same look),
// clickable (selects + closes on release-inside, via onSelect).
type comboRow struct {
	core.Element

	click ClickBehavior

	label    *TextBlock
	index    int
	selected bool

	onSelect func(index int)

	colors  theme.ColorTokens
	metrics theme.MetricTokens
}

// newComboRow returns a comboRow for item at index, initially marked
// selected per the caller's snapshot of the combo's current selection.
// onSelect (may be nil) fires with index on a successful click.
func newComboRow(face *text.Face, item string, index int, selected bool, colors theme.ColorTokens, metrics theme.MetricTokens, onSelect func(int)) *comboRow {
	row := &comboRow{
		index:    index,
		selected: selected,
		onSelect: onSelect,
		colors:   colors,
		metrics:  metrics,
	}
	row.label = NewTextBlock(face, item)
	row.label.SetColor(colors.WindowText)
	core.SetParent(row.label, row)

	row.click.OnClick = func() {
		if row.onSelect != nil {
			row.onSelect(row.index)
		}
	}
	return row
}

// rowPadding returns the row's content inset: PaddingM horizontal, PaddingS
// vertical — a more compact vertical rhythm than Button's chrome, since rows
// are stacked one after another in a list rather than standing alone.
func (row *comboRow) rowPadding() render.Thickness {
	return render.Thickness{
		Left: row.metrics.PaddingM, Right: row.metrics.PaddingM,
		Top: row.metrics.PaddingS, Bottom: row.metrics.PaddingS,
	}
}

// MeasureContent measures the label within the available space reduced by
// rowPadding, then adds the padding back to its desired size.
func (row *comboRow) MeasureContent(available render.Size) render.Size {
	pad := row.rowPadding()

	availW := available.W - pad.Left - pad.Right
	if availW < 0 {
		availW = 0
	}
	availH := available.H - pad.Top - pad.Bottom
	if availH < 0 {
		availH = 0
	}

	core.MeasureWidget(row.label, render.Size{W: availW, H: availH})
	d := core.DesiredSizeOf(row.label)

	return render.Size{W: d.W + pad.Left + pad.Right, H: d.H + pad.Top + pad.Bottom}
}

// ArrangeContent arranges the label at the padded inner rect's left edge,
// vertically centered.
func (row *comboRow) ArrangeContent(bounds render.Rect) {
	pad := row.rowPadding()
	inner := bounds.Inset(pad)
	if inner.W < 0 {
		inner.W = 0
	}
	if inner.H < 0 {
		inner.H = 0
	}

	d := core.DesiredSizeOf(row.label)
	y := inner.Y + (inner.H-d.H)/2
	core.ArrangeWidget(row.label, render.Rect{X: inner.X, Y: y, W: d.W, H: d.H})
}

// Children returns the label as the row's sole child.
func (row *comboRow) Children() []core.Widget {
	return []core.Widget{row.label}
}

// Render fills the row's bounds with Highlight (and recolors the label
// HighlightText) when the row is EITHER the current selection OR hovered —
// classic combo popups highlight the hovered row exactly like the selected
// one, with no separate visual for the two — else leaves it transparent
// (showing the popup card's own WindowWell through) with WindowText label
// color.
func (row *comboRow) Render(r render.Renderer) {
	highlighted := row.selected || row.click.Hover()

	labelColor := row.colors.WindowText
	if highlighted {
		r.FillRect(row.Bounds(), row.colors.Highlight)
		labelColor = row.colors.HighlightText
	}
	row.label.SetColor(labelColor)
}

// OnPointer implements input.PointerHandler, delegating the entire
// press/release/hover state machine to the embedded ClickBehavior.
func (row *comboRow) OnPointer(e *input.PointerEvent) {
	row.click.HandlePointer(e, row)
}
