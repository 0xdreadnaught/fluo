package controls

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
	"github.com/0xdreadnaught/fluo/theme"
)

// DialogResult identifies how a dialog shown via ShowDialog was closed.
type DialogResult uint8

const (
	// DialogPrimary: the user clicked the Primary button.
	DialogPrimary DialogResult = iota
	// DialogSecondary: the user clicked the Secondary button.
	DialogSecondary
	// DialogDismissed: the user pressed Escape. There is no other dismiss
	// path in v0 — a press on the scrim itself (outside the card) is a
	// documented no-op, not a dismissal; see ShowDialog's doc comment.
	DialogDismissed
)

// DialogSpec describes one modal dialog's content and buttons, passed to
// ShowDialog.
type DialogSpec struct {
	Title, Body string
	// Primary, Secondary are the button labels shown right-aligned in that
	// order (Secondary, then Primary — the accented "main action" ends up
	// rightmost, matching the common OK/Cancel reading order). An EMPTY
	// label omits that button entirely — DialogSpec{Primary: "OK"} shows a
	// single accent button; both empty shows no buttons at all (Escape is
	// then the only way to close the dialog).
	Primary, Secondary string
	// OnResult (may be nil) fires exactly once with the result the dialog
	// closed with — see ShowDialog's doc comment for the single-fire
	// guarantee.
	OnResult func(DialogResult)
}

// ShowDialog opens d as a single MODAL popup (OverlayHost.ShowPopup) on
// host: a full-host scrim (dialogScrim, ScrimBackground) containing a
// centered Card (dialogCard: CornerRadius, ShadowBlur shadow, PaddingL
// padding) with d.Title (SubtitleSize, TextPrimary), d.Body (BodySize,
// TextSecondary), and a right-aligned button row — d.Secondary (if
// non-empty) as a default Button, then d.Primary (if non-empty) as an
// accent Button. face supplies the button/body type ramp; title and body
// text are drawn at th.Type.SubtitleSize/BodySize via faces derived from
// face.Font (face itself may be nil, in which case both derived faces stay
// nil too, per TextBlock's own nil-face convention).
//
// Normative "scrim neutralizes light-dismiss by construction": dialogScrim's
// own desired size is unconditionally the FULL available space (see its own
// doc comment), and ShowDialog anchors it at host's own current bounds — so
// OverlayHost.placePopup's clamping math (see that function's doc comment)
// lands it flush at the host's origin, sized to exactly the host. Every
// press, wherever within the host it lands, therefore falls inside
// OverlayHost.OnPointer's "inside the topmost popup" branch (see that
// method's doc comment) — the outside-press light-dismiss branch can never
// run while this dialog is the topmost popup. A press that lands on the
// scrim itself (outside the centered card) is still forwarded into the
// scrim's own subtree via HitPath/Bubble, exactly like any popup-internal
// press, but simply finds no input.PointerHandler on the hit path (the scrim
// implements none — see its own doc comment) and goes nowhere: a documented
// v0 no-op, not a bug and not light-dismiss under a different name.
//
// Escape closes the dialog with DialogDismissed. Reaching it depends on
// keyboard focus: ShowDialog explicitly focuses the scrim widget itself
// (not a button) the moment the popup opens — see input.Router.Focus, which
// accepts any core.Widget, focusable or not — so input.Router.dispatchKey's
// focused-widget bubble always reaches the scrim's own OnKey regardless of
// whether Primary/Secondary exist or which one (if any) a subsequent click
// happens to focus. (A button click does NOT itself steal focus away here:
// input.Router.PointerButton only runs its ordinary press-to-focus path
// when nothing is captured, and this dialog's very presence as an open
// modal popup means OverlayHost already holds that capture — see
// OverlayHost's own type doc comment.)
//
// CAUTION: The dialog steals keyboard focus to its scrim and does NOT
// restore prior focus on close (v0). Callers needing focus restoration
// must track and re-apply the prior focused widget themselves.
//
// A card button click closes the dialog (via OverlayHost.ClosePopup) and
// records the matching result (DialogPrimary/DialogSecondary) before doing
// so; Escape records DialogDismissed the same way. Either path converges on
// the SAME onDismiss callback (fire, below), which is a one-shot latch: it
// fires d.OnResult on the first call and silently no-ops on any later one.
// This covers the double-fire scenario explicitly (click a button, then
// send Escape): OverlayHost.ClosePopup is independently idempotent — once
// the popup is closed and detached from the tree, a second explicit close
// call is a no-op, and Escape delivered afterward can't even reach the
// (by-then-unreachable) scrim's OnKey to try — but fire's own latch also
// guards the case directly, needing no such reasoning about tree topology.
func ShowDialog(host *OverlayHost, face *text.Face, d DialogSpec) {
	if host == nil {
		return
	}

	th := theme.Active()
	colors, metrics, typ := th.Color, th.Metric, th.Type

	var titleFace, bodyFace *text.Face
	if face != nil {
		titleFace = text.NewFace(face.Font, typ.SubtitleSize)
		bodyFace = text.NewFace(face.Font, typ.BodySize)
	}

	title := NewTextBlock(titleFace, d.Title)
	title.SetColor(colors.TextPrimary)
	body := NewTextBlock(bodyFace, d.Body)
	body.SetColor(colors.TextSecondary)

	buttonRow := NewStackPanel(Horizontal).SetGap(metrics.PaddingM)
	buttonRow.SetAlign(core.End, core.Start)

	content := NewStackPanel(Vertical).SetGap(metrics.PaddingM)
	content.Add(title, body, buttonRow)

	card := newDialogCard(content, colors, metrics)
	scrim := newDialogScrim(card, colors)

	fired := false
	result := DialogDismissed
	fire := func() {
		if fired {
			return
		}
		fired = true
		if d.OnResult != nil {
			d.OnResult(result)
		}
	}
	closeWith := func(r DialogResult) {
		result = r
		host.ClosePopup(scrim)
	}
	scrim.onEscape = func() { closeWith(DialogDismissed) }

	if d.Secondary != "" {
		secondary := NewButton(face, d.Secondary)
		secondary.OnClick(func() { closeWith(DialogSecondary) })
		buttonRow.Add(secondary)
	}
	if d.Primary != "" {
		primary := NewButton(face, d.Primary)
		primary.SetAccent(true)
		primary.OnClick(func() { closeWith(DialogPrimary) })
		buttonRow.Add(primary)
	}

	anchor := core.BoundsOf(host)
	host.ShowPopup(scrim, anchor, fire)
	if host.router != nil {
		host.router.Focus(scrim)
	}
}

// dialogScrim is a Dialog's outer popup widget: a Border-like decorator
// whose own desired size is unconditionally the FULL available space handed
// to it (see MeasureContent) — unlike every other popup content type in
// this package (comboPopupCard, menuPopupCard, the tooltip's Border popup),
// which all size to their own natural content. Combined with ShowDialog
// anchoring it at the host's own bounds, this is what makes it always cover
// the ENTIRE host — see ShowDialog's own doc comment for why that is what
// neutralizes light-dismiss by construction.
//
// Deliberately implements no input.PointerHandler: a forwarded press that
// lands on the scrim itself (outside the centered card) simply finds no
// handler on its hit path and goes nowhere — the same
// non-interactive-by-omission convention menuSeparatorRow and the tooltip's
// plain Border popup use. It DOES implement input.KeyHandler (OnKey, below),
// so Escape reaches it via the focused-widget key bubble no matter what
// currently holds focus inside the card — see ShowDialog's own doc comment
// on why the scrim itself, not a button, is what ShowDialog focuses.
type dialogScrim struct {
	core.Element

	card core.Widget

	// onEscape (wired by ShowDialog) fires on Escape — see OnKey. Nil is a
	// theoretical, silent no-op; ShowDialog always wires it before the popup
	// is ever shown.
	onEscape func()

	colors theme.ColorTokens
}

// newDialogScrim returns a dialogScrim wrapping card (re-parented to it),
// with no onEscape wired yet — ShowDialog sets it right after construction.
func newDialogScrim(card core.Widget, colors theme.ColorTokens) *dialogScrim {
	s := &dialogScrim{card: card, colors: colors}
	core.SetParent(card, s)
	return s
}

// MeasureContent measures card (so ArrangeContent can center it) but
// reports the desired size as available UNCHANGED — see the type doc
// comment's "full-host" paragraph.
func (s *dialogScrim) MeasureContent(available render.Size) render.Size {
	core.MeasureWidget(s.card, available)
	return available
}

// ArrangeContent centers card, at exactly its own desired size, within
// bounds — the scrim's own arranged rect, equal to the whole host (see the
// type doc comment).
func (s *dialogScrim) ArrangeContent(bounds render.Rect) {
	d := core.DesiredSizeOf(s.card)
	x := bounds.X + (bounds.W-d.W)/2
	y := bounds.Y + (bounds.H-d.H)/2
	core.ArrangeWidget(s.card, render.Rect{X: x, Y: y, W: d.W, H: d.H})
}

// Children returns the single card child.
func (s *dialogScrim) Children() []core.Widget {
	return []core.Widget{s.card}
}

// Render fills the scrim's entire bounds (the whole host — see the type doc
// comment) with ScrimBackground, dimming whatever content sits beneath the
// popup.
func (s *dialogScrim) Render(r render.Renderer) {
	r.FillRect(s.Bounds(), s.colors.ScrimBackground)
}

// OnKey implements input.KeyHandler: Escape fires onEscape (wired by
// ShowDialog to close the dialog with DialogDismissed) and marks the event
// handled. Ignored entirely for anything but Action==Press.
func (s *dialogScrim) OnKey(e *input.KeyEvent) {
	if e.Action != input.Press || e.Key != input.KeyEscape {
		return
	}
	if s.onEscape != nil {
		s.onEscape()
	}
	e.Handled = true
}

// dialogCard is a Dialog's centered Card chrome: a Card-background,
// drop-shadowed container (CornerRadius/ShadowBlur/Shadow tokens, matching
// comboPopupCard/menuPopupCard's own shadow+fill Render) with PaddingL
// padding around its single child (the title/body/button-row StackPanel) —
// the one popup-card variant in this package that adds its own padding
// rather than delegating it entirely to the child, since Dialog has no
// other wrapper for it.
type dialogCard struct {
	core.Element

	child core.Widget

	colors  theme.ColorTokens
	metrics theme.MetricTokens
}

// newDialogCard returns a dialogCard wrapping child (re-parented to it).
func newDialogCard(child core.Widget, colors theme.ColorTokens, metrics theme.MetricTokens) *dialogCard {
	c := &dialogCard{child: child, colors: colors, metrics: metrics}
	core.SetParent(child, c)
	return c
}

// padding returns the card's content inset: PaddingL on all four sides.
func (c *dialogCard) padding() render.Thickness {
	return render.Uniform(c.metrics.PaddingL)
}

// MeasureContent measures child within the available space reduced by
// padding, then adds the padding back to its desired size — the card sizes
// to its own content, never stretched to available (ArrangeContent, in
// dialogScrim, arranges it at exactly this desired size).
func (c *dialogCard) MeasureContent(available render.Size) render.Size {
	pad := c.padding()

	availW := available.W - pad.Left - pad.Right
	if availW < 0 {
		availW = 0
	}
	availH := available.H - pad.Top - pad.Bottom
	if availH < 0 {
		availH = 0
	}

	core.MeasureWidget(c.child, render.Size{W: availW, H: availH})
	d := core.DesiredSizeOf(c.child)

	return render.Size{W: d.W + pad.Left + pad.Right, H: d.H + pad.Top + pad.Bottom}
}

// ArrangeContent arranges child within bounds inset by padding.
func (c *dialogCard) ArrangeContent(bounds render.Rect) {
	inner := bounds.Inset(c.padding())
	if inner.W < 0 {
		inner.W = 0
	}
	if inner.H < 0 {
		inner.H = 0
	}
	core.ArrangeWidget(c.child, inner)
}

// Children returns the single child.
func (c *dialogCard) Children() []core.Widget {
	return []core.Widget{c.child}
}

// Render draws the drop shadow first, then the card's own rounded
// CardBackground fill — identical in shape to comboPopupCard.Render /
// menuPopupCard.Render.
func (c *dialogCard) Render(r render.Renderer) {
	bounds := c.Bounds()
	radius := c.metrics.CornerRadius
	r.DrawShadow(bounds, radius, c.metrics.ShadowBlur, c.colors.Shadow)
	r.FillRoundedRect(bounds, radius, c.colors.CardBackground)
}
