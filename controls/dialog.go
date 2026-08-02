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
// host: a full-host dialogScrim (v0.2 classic: painted invisibly — see its
// own doc comment on why classic Win2000 modals had no dark scrim) wrapping
// a centered dialogCard (drawRaised ButtonFace, PaddingL padding) with a
// classic CaptionFrom→CaptionTo gradient caption strip along its top
// showing d.Title (CaptionText) — provided d.Title is non-empty and face is
// non-nil; see dialogCard.captionHeight's own "no caption at all otherwise"
// fallback — d.Body (BodySize, WindowText) below it, and a right-aligned
// button row: d.Secondary (if non-empty) as a default Button, then
// d.Primary (if non-empty) as an accent Button. face supplies the
// button/body/caption type ramp; body and caption text are drawn at
// th.Type.BodySize/SubtitleSize via faces derived from face.Font (face
// itself may be nil, in which case both derived faces stay nil too, per
// TextBlock's own nil-face convention — and, per captionHeight, no caption
// strip is drawn at all). The content stack's own FIRST child is a
// zero-size Fixed placeholder, not a title TextBlock: the title now lives
// solely in the card's caption strip (kept as a real (if invisible) child
// so the stack's shape — title-slot, body, buttonRow — stays exactly 3
// children, matching dialogPopupButtons' own white-box navigation).
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
// Keyboard input is TRAPPED inside the dialog for as long as it is open:
// the popup is opened via OverlayHost.showPopupTrapFocus, which pushes an
// input.Router focus scope rooted at the scrim (and pops it on close). So
// Tab/Shift+Tab cycle only the card's own buttons — wrapping between them
// rather than stepping out into the content the dialog is covering, which is
// still in the tree and would otherwise still be in the tab order — and no
// key reaches a widget behind the dialog, so Space/Enter can't activate a
// button it is covering. Pushing the scope also focuses the scrim widget
// itself (not a button — see input.Router.Focus, which accepts any
// core.Widget, focusable or not), which is what makes the FIRST Tab land on
// the card's first button, and what keeps the scrim focused when the card
// has no buttons at all.
//
// Escape closes the dialog with DialogDismissed, and reaching it falls out
// of that same arrangement: input.Router.dispatchKey bubbles from the scrim
// (or from whichever in-scope button Tab/a click has since focused, through
// the scrim on its way up) to the scrim's own OnKey, regardless of whether
// Primary/Secondary exist. The button-less dialog — DialogSpec with both
// labels empty, whose ONLY close path is Escape — is the case this scoping
// exists for: with no focusable widget inside, focus stays on the scrim,
// Tab moves nothing, and Escape still arrives. (A button click does NOT
// steal focus away either: input.Router.PointerButton only runs its ordinary
// press-to-focus path when nothing is captured, and this dialog's very
// presence as an open modal popup means OverlayHost already holds that
// capture — see OverlayHost's own type doc comment.)
//
// CAUTION: The dialog steals keyboard focus to its scrim and does NOT
// restore prior focus on close (v0) — closing it pops the focus scope, so
// ordinary tab traversal over the content behind resumes, but focus itself
// is left cleared rather than put back where it was. Callers needing focus
// restoration must track and re-apply the prior focused widget themselves.
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

	// The title no longer lives in the content flow — it is drawn in the
	// card's own gradient caption strip (see dialogCard.Render) — but a
	// zero-size placeholder keeps the stack's shape at exactly 3 children
	// (title-slot, body, buttonRow), matching dialogPopupButtons' white-box
	// navigation (children[2] == the button row).
	titleSlot := NewFixed(0, 0, render.Color{})
	titleSlot.SetVisible(false)
	body := NewTextBlock(bodyFace, d.Body)
	body.SetColor(colors.WindowText)

	buttonRow := NewStackPanel(Horizontal).SetGap(metrics.PaddingM)
	buttonRow.SetAlign(core.End, core.Start)

	content := NewStackPanel(Vertical).SetGap(metrics.PaddingM)
	content.Add(titleSlot, body, buttonRow)

	card := newDialogCard(content, d.Title, titleFace, colors, metrics)
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
	// Trapping variant, not plain ShowPopup: this is the one popup family in
	// the package that must own the keyboard outright while it's up, and the
	// push is also what focuses the scrim (see the doc comment above).
	host.showPopupTrapFocus(scrim, anchor, fire)
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

// Render is intentionally a no-op (v0.2 classic): authentic Windows-2000
// modal dialogs painted no scrim at all — the raised dialogCard plus its
// gradient caption strip already read as modal enough on their own. Kept as
// an explicit (empty) method, rather than omitted entirely, so this
// decision is documented in place rather than by silent omission; the
// scrim's full-host MeasureContent/ArrangeContent (see above) are UNCHANGED
// — it still occupies the whole host for hit-testing (the outside-the-card
// no-op press) and Esc-key routing, it simply paints nothing.
func (s *dialogScrim) Render(r render.Renderer) {}

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

// dialogCard is a Dialog's centered card chrome: a classic raised ButtonFace
// bevel (drawRaised, replacing the pre-restyle rounded, drop-shadowed Card
// fill) with PaddingL padding around its single child (the
// title-slot/body/button-row StackPanel — see ShowDialog) — the one
// popup-card variant in this package that adds its own padding rather than
// delegating it entirely to the child, since Dialog has no other wrapper
// for it. When title/titleFace are both non-empty/non-nil, an ADDITIONAL
// gradient caption strip (CaptionFrom→CaptionTo, CaptionText title) is
// reserved along the very top, ABOVE that padded content — see
// captionHeight/Render.
type dialogCard struct {
	core.Element

	child core.Widget

	// title and titleFace drive the gradient caption strip (see
	// captionHeight/Render). titleFace may be nil (ShowDialog's own
	// nil-face convention), and title may be "" (an app that wants no
	// visible title) — either collapses captionHeight to 0, so no caption
	// space is reserved and none is drawn at all — the brief's "skip the
	// caption entirely" fallback, rather than inventing a title API.
	title     string
	titleFace *text.Face

	colors  theme.ColorTokens
	metrics theme.MetricTokens
}

// newDialogCard returns a dialogCard wrapping child (re-parented to it),
// showing title (via titleFace) in its gradient caption strip — see the
// type doc comment for the "no caption at all" fallback when either is
// empty/nil.
func newDialogCard(child core.Widget, title string, titleFace *text.Face, colors theme.ColorTokens, metrics theme.MetricTokens) *dialogCard {
	c := &dialogCard{child: child, title: title, titleFace: titleFace, colors: colors, metrics: metrics}
	core.SetParent(child, c)
	return c
}

// padding returns the card's content inset: PaddingL on all four sides.
func (c *dialogCard) padding() render.Thickness {
	return render.Uniform(c.metrics.PaddingL)
}

// captionHeight returns the space reserved at the card's top for the
// gradient caption strip: BevelWidth (so the strip sits inside the card's
// own raised top/left/right edges, not over them) plus the title face's
// line height plus PaddingS above and below — or 0 entirely when there is
// no title text or no titleFace, per the type doc comment's fallback.
func (c *dialogCard) captionHeight() float32 {
	if c.title == "" || c.titleFace == nil {
		return 0
	}
	return c.metrics.BevelWidth + c.titleFace.LineHeight() + 2*c.metrics.PaddingS
}

// MeasureContent measures child within the available space reduced by
// padding AND captionHeight (reserved above the padded content), then adds
// both back to its desired size — the card sizes to its own content, never
// stretched to available (ArrangeContent, in dialogScrim, arranges it at
// exactly this desired size).
func (c *dialogCard) MeasureContent(available render.Size) render.Size {
	pad := c.padding()
	capH := c.captionHeight()

	availW := available.W - pad.Left - pad.Right
	if availW < 0 {
		availW = 0
	}
	availH := available.H - pad.Top - pad.Bottom - capH
	if availH < 0 {
		availH = 0
	}

	core.MeasureWidget(c.child, render.Size{W: availW, H: availH})
	d := core.DesiredSizeOf(c.child)

	return render.Size{W: d.W + pad.Left + pad.Right, H: d.H + pad.Top + pad.Bottom + capH}
}

// ArrangeContent arranges child within bounds inset by padding, itself
// offset down by captionHeight so the content never overlaps the caption
// strip.
func (c *dialogCard) ArrangeContent(bounds render.Rect) {
	capH := c.captionHeight()
	body := render.Rect{X: bounds.X, Y: bounds.Y + capH, W: bounds.W, H: bounds.H - capH}
	if body.H < 0 {
		body.H = 0
	}

	inner := body.Inset(c.padding())
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

// Render paints the classic raised ButtonFace bevel (drawRaised) — replacing
// the pre-restyle rounded-fill-plus-drop-shadow chrome shared with
// comboPopupCard/menuPopupCard — then, when captionHeight() > 0, a
// CaptionFrom→CaptionTo horizontal gradient strip (DrawGradientRect) just
// inside the card's top/left/right raised edges, with title left-aligned in
// CaptionText, vertically centered within the strip.
func (c *dialogCard) Render(r render.Renderer) {
	bounds := c.Bounds()
	drawRaised(r, bounds, c.colors.ButtonFace, c.colors)

	capH := c.captionHeight()
	if capH <= 0 {
		return
	}

	bw := c.metrics.BevelWidth
	caption := render.Rect{
		X: bounds.X + bw, Y: bounds.Y + bw,
		W: bounds.W - 2*bw, H: capH - bw,
	}
	if caption.W < 0 {
		caption.W = 0
	}
	if caption.H < 0 {
		caption.H = 0
	}
	r.DrawGradientRect(caption, c.colors.CaptionFrom, c.colors.CaptionTo, true)

	ty := caption.Y + (caption.H-c.titleFace.LineHeight())/2
	tx := caption.X + c.metrics.PaddingS
	c.titleFace.Draw(r, render.Point{X: tx, Y: ty}, c.title, c.colors.CaptionText)
}
