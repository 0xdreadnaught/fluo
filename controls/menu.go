package controls

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
	"github.com/0xdreadnaught/fluo/theme"
)

// menuMinWidth is a menu popup's minimum width so short-label menus
// ("New"/"Open"/"Exit") aren't cramped — WinUI context menus keep a
// comfortable minimum regardless of their widest item.
const menuMinWidth = 140

// menuRowPadding returns a menu popup row's content inset: PaddingM
// horizontal, PaddingS vertical — identical to ComboBox's comboRow padding
// (rows are stacked one after another, so the same compact vertical rhythm
// applies here as in every other popup-row list in this package).
func menuRowPadding(metrics theme.MetricTokens) render.Thickness {
	return render.Thickness{
		Left: metrics.PaddingM, Right: metrics.PaddingM,
		Top: metrics.PaddingS, Bottom: metrics.PaddingS,
	}
}

// menuEntryKind distinguishes the three kinds of entry a MenuItems builder
// can hold; menuEntry's other fields are only meaningful for the matching
// kind (see menuEntry's own doc comment).
type menuEntryKind uint8

const (
	menuEntryItem menuEntryKind = iota
	menuEntrySeparator
	menuEntrySub
)

// menuEntry is one entry recorded by a MenuItems builder: a clickable item
// (label + onClick), an inert separator (label/onClick/sub all unused), or a
// submenu trigger (label + its own nested MenuItems builder, sub). Built up
// by MenuItems.Add/AddSeparator/AddSub and read back only when a popup is
// actually shown (buildMenuPopup) — the same "data now, widgets built fresh
// on every open" split ComboBox uses for its own items (see
// ComboBox.buildPopup's doc comment), so editing a MenuItems after it was
// last shown is always reflected the next time it opens.
type menuEntry struct {
	kind    menuEntryKind
	label   string
	onClick func()
	sub     *MenuItems
}

// MenuItems is a builder for one popup menu's contents — the top-level
// content of a MenuBar entry, a nested submenu (AddSub), or a ShowContextMenu
// popup. It records entries as plain data (see menuEntry); the actual popup
// widget tree is constructed fresh every time the menu is opened, by
// buildMenuPopup.
type MenuItems struct {
	entries []*menuEntry

	face    *text.Face
	colors  theme.ColorTokens
	metrics theme.MetricTokens
}

// newMenuItems returns an empty MenuItems, styled from the given tokens and
// drawing labels with face (face may be nil, per TextBlock's own nil-face
// convention).
func newMenuItems(face *text.Face, colors theme.ColorTokens, metrics theme.MetricTokens) *MenuItems {
	return &MenuItems{face: face, colors: colors, metrics: metrics}
}

// Add appends a clickable item with the given label, firing onClick (may be
// nil) when the user clicks its row — which ALSO closes every open menu
// popup, per the package's normative "item click fires + closes ALL menus"
// rule (see buildMenuPopup). Returns mi for chaining further Add/
// AddSeparator/AddSub calls onto the SAME menu.
func (mi *MenuItems) Add(label string, onClick func()) *MenuItems {
	mi.entries = append(mi.entries, &menuEntry{kind: menuEntryItem, label: label, onClick: onClick})
	return mi
}

// AddSeparator appends an inert 1px-rule separator row (see
// menuSeparatorRow) — it never highlights on hover, never fires a click, and
// never closes anything: forwarded pointer events simply find no handler on
// its row and go nowhere (matching ToolTipArea's tip's own
// non-interactive-by-omission convention). Returns mi for chaining.
func (mi *MenuItems) AddSeparator() *MenuItems {
	mi.entries = append(mi.entries, &menuEntry{kind: menuEntrySeparator})
	return mi
}

// AddSub appends a submenu trigger row labeled label and returns a FRESH
// MenuItems builder for ITS contents — unlike Add/AddSeparator (which return
// mi itself, for chaining more entries onto the SAME menu), AddSub's return
// value is the nested menu, so callers chain directly onto it:
// mi.AddSub("Recent").Add("a.txt", ...).Add("b.txt", ...). The submenu opens
// on hover (immediate v0 — no dwell delay), anchored to the right of its own
// row, as a second, NESTED popup on the host's stack (see menuPopupCard.openSub).
func (mi *MenuItems) AddSub(label string) *MenuItems {
	sub := newMenuItems(mi.face, mi.colors, mi.metrics)
	mi.entries = append(mi.entries, &menuEntry{kind: menuEntrySub, label: label, sub: sub})
	return sub
}

// buildMenuPopup constructs a fresh popup widget (a menuPopupCard wrapping a
// vertical StackPanel of rows) from items' CURRENT entries — built fresh on
// every open, exactly like ComboBox.buildPopup, so edits made to items while
// closed are always reflected. closeAll (never nil in practice — every
// caller passes host.CloseAllPopups) is threaded into every item row's click
// handler AND recursively into every nested submenu's own buildMenuPopup
// call, so a click anywhere in an arbitrarily deep submenu chain collapses
// the whole chain in one call — see the package doc's "item click closes ALL
// menus" normative rule.
func buildMenuPopup(items *MenuItems, closeAll func()) *menuPopupCard {
	stack := NewStackPanel(Vertical)
	card := newMenuPopupCard(stack, items.colors, items.metrics)

	for _, e := range items.entries {
		switch e.kind {
		case menuEntryItem:
			onClick := e.onClick
			row := newMenuItemRow(items.face, e.label, items.colors, items.metrics, func() {
				if onClick != nil {
					onClick()
				}
				closeAll()
			})
			stack.Add(row)
		case menuEntrySeparator:
			stack.Add(newMenuSeparatorRow(items.colors, items.metrics))
		case menuEntrySub:
			sub := e.sub
			row := newMenuSubRow(items.face, e.label, items.colors, items.metrics)
			row.onHover = func() {
				card.openSub(row, sub, closeAll)
			}
			stack.Add(row)
		}
	}
	return card
}

// menuPopupCard is a menu popup's outer chrome: a raised ButtonFace bevel
// (drawRaised) wrapping a single child (the entry StackPanel) — the bevel
// itself replaces the pre-restyle rounded, drop-shadowed Card chrome (much
// like ComboBox's own comboPopupCard — see its doc comment) — plus the
// bookkeeping for AT MOST one open submenu at a time (openSubRow/subPopup):
// opening a second submenu while one is already open closes the first first
// (see openSub).
type menuPopupCard struct {
	core.Element

	child core.Widget

	// openSubRow/subPopup track the currently-open submenu (nil/nil if
	// none): openSubRow is the menuSubRow whose hover opened it (compared by
	// identity in openSub, to no-op a re-hover over the same row), subPopup
	// is the popup widget itself, needed to close it via
	// OverlayHost.ClosePopup before opening a different one.
	openSubRow *menuSubRow
	subPopup   core.Widget

	colors  theme.ColorTokens
	metrics theme.MetricTokens
}

// newMenuPopupCard returns a menuPopupCard wrapping child (re-parented to
// it).
func newMenuPopupCard(child core.Widget, colors theme.ColorTokens, metrics theme.MetricTokens) *menuPopupCard {
	card := &menuPopupCard{child: child, colors: colors, metrics: metrics}
	core.SetParent(child, card)
	return card
}

// MeasureContent measures the child with the full available space and
// reports its desired size unchanged (no padding/chrome to add back) —
// identical to comboPopupCard.MeasureContent.
func (card *menuPopupCard) MeasureContent(available render.Size) render.Size {
	core.MeasureWidget(card.child, available)
	d := core.DesiredSizeOf(card.child)
	if d.W < menuMinWidth {
		d.W = menuMinWidth
	}
	return d
}

// ArrangeContent arranges the child to fill the card's own bounds exactly.
func (card *menuPopupCard) ArrangeContent(bounds render.Rect) {
	core.ArrangeWidget(card.child, bounds)
}

// Children returns the single child (the entry StackPanel). The submenu
// popup, if one is open, is DELIBERATELY excluded: it lives on the
// OverlayHost's own popup stack as a second, independent entry (see
// openSub), not inside this card's own subtree — that is what makes it
// hit-test/render as the topmost popup, above THIS card, rather than merely
// as a descendant of it.
func (card *menuPopupCard) Children() []core.Widget {
	return []core.Widget{card.child}
}

// Render draws the classic raised ButtonFace bevel (drawRaised) framing the
// popup — replacing the pre-restyle rounded CardBackground fill, drop
// shadow, and hairline stroke: the bevel itself now reads as the popup's
// border, with no separate shadow/stroke needed.
func (card *menuPopupCard) Render(r render.Renderer) {
	drawRaised(r, card.Bounds(), card.colors.ButtonFace, card.colors)
}

// openSub opens sub's popup anchored to the right of row (a no-op if row's
// submenu is already the one open — compared by identity), first closing
// whatever OTHER submenu was previously open on this card (at most one at a
// time). closeAll is threaded straight through to the nested buildMenuPopup
// call, so a click inside the new submenu still collapses the whole chain.
//
// The anchor passed to ShowPopup is a ZERO-SIZE rect at {row's right edge,
// row's own top edge} rather than row's full bounds: OverlayHost.placePopup
// always computes y = anchor.Bottom() (falling back to anchor.Y-desired.H
// only on overflow) and x = anchor.X — with H=0, anchor.Bottom() == anchor.Y,
// so the submenu's TOP edge lands flush with the row's own top, and its LEFT
// edge lands at the row's right edge, which is exactly "anchored right of the
// item" with no new placement code needed in OverlayHost itself.
//
// Called from a menuSubRow's OnPointer on Enter — i.e. from WITHIN a
// forwarded pointer event OverlayHost.OnPointer is delivering while it holds
// the router's capture for the (already open) parent popup. ShowPopup's own
// router.Capture(host) call this triggers is therefore just re-engaging a
// capture the host already holds (idempotent — see input.Router.Capture's
// doc comment), not nesting a new one on top of some capture row itself
// holds: rows never capture on Enter (only Press does, via ClickBehavior),
// so there is no h→w→h stack to worry about here — see OverlayHost.ShowPopup's
// own doc comment for the general caveat this sidesteps.
//
// Chain-aware forwarding/dismissal (Phase 8 Task 1 — see
// OverlayHost.OnPointer's own doc comment for the full (a)/(b)/(c)
// semantics) replaced the earlier "the submenu is a second, independent
// entry on the host's popup stack, and forwarding/dismissal only ever
// targets the topmost entry" v0 simplification: an outside press now
// light-dismisses the ENTIRE open chain (this submenu AND the parent menu
// beneath it) in one press, via OverlayHost's outside-press CloseAllPopups
// branch — see TestMenuOutsidePressWithSubmenuClosesAll
// (controls/menu_test.go); and every OTHER row in the PARENT popup (siblings
// of row) is reachable again while the submenu is open — hovering one
// auto-closes this submenu first (OverlayHost.OnPointer closes whatever sits
// above the popup a Move's position actually falls in), then hovers/forwards
// into the parent row normally — see
// TestMenuSiblingRowHoverAutoClosesSubmenu.
//
// STILL PINNED (not addressed by chain-aware forwarding, since it's a
// placement question, not a forwarding/dismissal one): the anchor's clamping
// (via OverlayHost.placePopup's existing x-clamp) means a submenu opened
// from a row near the host's RIGHT edge, wide enough to overflow, is clamped
// back leftward rather than flipping to open on the LEFT of row instead — it
// can end up overlapping row (and the parent popup) rather than sitting
// cleanly beside it. Revisit (not done here): a left-flip for a submenu
// anchor that would overflow the right edge, mirroring placePopup's existing
// above/below flip.
func (card *menuPopupCard) openSub(row *menuSubRow, sub *MenuItems, closeAll func()) {
	if card.openSubRow == row {
		return
	}
	host := OverlayHostFor(card)
	if host == nil {
		return
	}
	if card.subPopup != nil {
		host.ClosePopup(card.subPopup)
	}

	rowBounds := core.BoundsOf(row)
	anchor := render.Rect{X: rowBounds.Right(), Y: rowBounds.Y}

	popup := buildMenuPopup(sub, closeAll)
	card.openSubRow = row
	card.subPopup = popup
	host.ShowPopup(popup, anchor, func() {
		card.openSubRow = nil
		card.subPopup = nil
	})
}

// menuItemRow is one clickable item row inside an open menu popup: a
// left-aligned TextBlock, filled the classic navy Highlight on hover, firing
// onClick (which, per buildMenuPopup, both runs the entry's own callback and
// closes every open menu popup) on a release-inside click.
type menuItemRow struct {
	core.Element

	click ClickBehavior
	label *TextBlock

	colors  theme.ColorTokens
	metrics theme.MetricTokens
}

// newMenuItemRow returns a menuItemRow showing label in face (face may be
// nil, per TextBlock). onClick (may be nil) fires on a successful click.
func newMenuItemRow(face *text.Face, label string, colors theme.ColorTokens, metrics theme.MetricTokens, onClick func()) *menuItemRow {
	row := &menuItemRow{colors: colors, metrics: metrics}
	row.label = NewTextBlock(face, label)
	row.label.SetColor(colors.WindowText)
	core.SetParent(row.label, row)
	row.click.OnClick = onClick
	return row
}

// MeasureContent measures the label within the available space reduced by
// menuRowPadding, then adds the padding back to its desired size —
// identical in shape to comboRow.MeasureContent.
func (row *menuItemRow) MeasureContent(available render.Size) render.Size {
	pad := menuRowPadding(row.metrics)

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
func (row *menuItemRow) ArrangeContent(bounds render.Rect) {
	pad := menuRowPadding(row.metrics)
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
func (row *menuItemRow) Children() []core.Widget {
	return []core.Widget{row.label}
}

// Render fills the row's bounds with the classic navy Highlight (and
// recolors the label HighlightText) while hovered, else leaves it
// transparent (showing the popup card's own raised ButtonFace through) with
// WindowText label color — items never show a persistent "selected" fill
// (contrast comboRow): a menu item is a one-shot action, not a selection.
func (row *menuItemRow) Render(r render.Renderer) {
	if row.click.Hover() {
		r.FillRect(row.Bounds(), row.colors.Highlight)
		row.label.SetColor(row.colors.HighlightText)
		return
	}
	row.label.SetColor(row.colors.WindowText)
}

// OnPointer implements input.PointerHandler, delegating the entire
// press/release/hover state machine to the embedded ClickBehavior.
func (row *menuItemRow) OnPointer(e *input.PointerEvent) {
	row.click.HandlePointer(e, row)
}

// menuSeparatorHeight is a menuSeparatorRow's fixed row height: enough
// vertical breathing room above and below its single 1px rule that it reads
// as a visual break between item groups rather than another item's own row.
const menuSeparatorHeight float32 = 9

// menuSeparatorRow is an inert 1px-rule separator row inside an open menu
// popup: it implements neither input.PointerHandler nor any focus interface,
// so a forwarded pointer event that hits it (per OverlayHost's
// capture-forwarding while a modal popup is open) simply finds no handler on
// its leaf and goes nowhere — the row never highlights on hover and never
// fires anything, matching ToolTipArea's non-interactive tip popup
// (see its own doc comment).
type menuSeparatorRow struct {
	core.Element

	colors  theme.ColorTokens
	metrics theme.MetricTokens
}

// newMenuSeparatorRow returns a menuSeparatorRow.
func newMenuSeparatorRow(colors theme.ColorTokens, metrics theme.MetricTokens) *menuSeparatorRow {
	return &menuSeparatorRow{colors: colors, metrics: metrics}
}

// MeasureContent reports a fixed menuSeparatorHeight, with zero width — the
// row never widens the popup on its own account; the popup's actual width
// is always driven by its widest item/submenu row.
func (row *menuSeparatorRow) MeasureContent(render.Size) render.Size {
	return render.Size{W: 0, H: menuSeparatorHeight}
}

// Render draws a classic etched groove (drawGroove) spanning the row's own
// height, inset by PaddingM on both sides — replacing the pre-restyle
// single 1px ControlStroke rule with the two-tone (ButtonShadow then
// ButtonHighlight) chiseled line.
func (row *menuSeparatorRow) Render(r render.Renderer) {
	bounds := row.Bounds()
	line := render.Rect{
		X: bounds.X + row.metrics.PaddingM,
		Y: bounds.Y,
		W: bounds.W - 2*row.metrics.PaddingM,
		H: bounds.H,
	}
	if line.W > 0 {
		drawGroove(r, line, true, row.colors)
	}
}

// menuSubRow is a submenu-trigger row inside an open menu popup: a
// left-aligned TextBlock label plus a right-aligned '>' chevron (WindowText),
// both filled the classic navy Highlight on hover exactly like menuItemRow.
// Its submenu opens on HOVER (Enter), not click — see onHover, wired by
// buildMenuPopup to menuPopupCard.openSub.
type menuSubRow struct {
	core.Element

	click   ClickBehavior
	label   *TextBlock
	chevron *TextBlock

	// onHover fires whenever this row receives a pointer Enter — wired by
	// buildMenuPopup, after construction, to open this row's submenu via the
	// owning menuPopupCard.openSub. Nil is a valid, silent no-op (should not
	// happen in practice: buildMenuPopup always wires it for a menuEntrySub
	// row before adding it to the stack).
	onHover func()

	colors  theme.ColorTokens
	metrics theme.MetricTokens
}

// newMenuSubRow returns a menuSubRow showing label in face (face may be nil,
// per TextBlock) with no onHover wired yet — buildMenuPopup sets it right
// after construction, once the row (and its owning card) both exist.
func newMenuSubRow(face *text.Face, label string, colors theme.ColorTokens, metrics theme.MetricTokens) *menuSubRow {
	row := &menuSubRow{colors: colors, metrics: metrics}
	row.label = NewTextBlock(face, label)
	row.label.SetColor(colors.WindowText)
	core.SetParent(row.label, row)
	row.chevron = NewTextBlock(face, ">")
	row.chevron.SetColor(colors.WindowText)
	core.SetParent(row.chevron, row)
	return row
}

// MeasureContent measures the chevron first (it never shrinks to make room
// for the label), then the label in whatever width remains after the
// chevron and a gap, and reports their combined width (plus padding) as the
// row's desired size — mirroring ComboBox.MeasureContent's own
// chevron-then-label layout.
func (row *menuSubRow) MeasureContent(available render.Size) render.Size {
	pad := menuRowPadding(row.metrics)
	gap := row.metrics.PaddingM

	availW := available.W - pad.Left - pad.Right
	if availW < 0 {
		availW = 0
	}
	availH := available.H - pad.Top - pad.Bottom
	if availH < 0 {
		availH = 0
	}

	core.MeasureWidget(row.chevron, render.Size{W: availW, H: availH})
	chevD := core.DesiredSizeOf(row.chevron)

	labelAvailW := availW - chevD.W - gap
	if labelAvailW < 0 {
		labelAvailW = 0
	}
	core.MeasureWidget(row.label, render.Size{W: labelAvailW, H: availH})
	labelD := core.DesiredSizeOf(row.label)

	h := labelD.H
	if chevD.H > h {
		h = chevD.H
	}

	return render.Size{
		W: labelD.W + gap + chevD.W + pad.Left + pad.Right,
		H: h + pad.Top + pad.Bottom,
	}
}

// ArrangeContent arranges the label at the padded inner rect's left edge
// (vertically centered) and the chevron at its right edge (also vertically
// centered).
func (row *menuSubRow) ArrangeContent(bounds render.Rect) {
	pad := menuRowPadding(row.metrics)
	inner := bounds.Inset(pad)
	if inner.W < 0 {
		inner.W = 0
	}
	if inner.H < 0 {
		inner.H = 0
	}

	chevD := core.DesiredSizeOf(row.chevron)
	chevX := inner.X + inner.W - chevD.W
	chevY := inner.Y + (inner.H-chevD.H)/2
	core.ArrangeWidget(row.chevron, render.Rect{X: chevX, Y: chevY, W: chevD.W, H: chevD.H})

	labelD := core.DesiredSizeOf(row.label)
	labelY := inner.Y + (inner.H-labelD.H)/2
	core.ArrangeWidget(row.label, render.Rect{X: inner.X, Y: labelY, W: labelD.W, H: labelD.H})
}

// Children returns the label and chevron.
func (row *menuSubRow) Children() []core.Widget {
	return []core.Widget{row.label, row.chevron}
}

// Render fills the row's bounds with the classic navy Highlight (and
// recolors both the label and the chevron HighlightText) while hovered,
// else leaves it transparent with WindowText label/chevron — mirroring
// menuItemRow.Render.
func (row *menuSubRow) Render(r render.Renderer) {
	if row.click.Hover() {
		r.FillRect(row.Bounds(), row.colors.Highlight)
		row.label.SetColor(row.colors.HighlightText)
		row.chevron.SetColor(row.colors.HighlightText)
		return
	}
	row.label.SetColor(row.colors.WindowText)
	row.chevron.SetColor(row.colors.WindowText)
}

// OnPointer implements input.PointerHandler: the embedded ClickBehavior
// tracks hover/press exactly like menuItemRow (a Press/Release with no
// OnClick wired is a harmless no-op — v0 has no click behavior of its own
// for a submenu trigger, only hover), and a pointer Enter additionally fires
// onHover — opening the submenu immediately, no dwell delay (per the
// package's normative "submenu opens on hover, immediate v0" rule).
func (row *menuSubRow) OnPointer(e *input.PointerEvent) {
	row.click.HandlePointer(e, row)
	if e.Action == input.Enter && row.onHover != nil {
		row.onHover()
	}
}

// menuBarEntry is one top-level MenuBar entry: its header title and the
// MenuItems builder for its popup contents.
type menuBarEntry struct {
	title string
	items *MenuItems
}

// MenuBar is a horizontal row of top-level menu titles (e.g. "File", "Edit"):
// clicking one opens its popup menu (see MenuItems) — a raised-bevel-framed
// MODAL popup (ShowPopup), placed directly below the clicked title —
// exactly like ComboBox's own dropdown. MenuBar draws its cells directly
// (titles measured against face, no per-cell child widgets), mirroring
// TabControl's tabStrip.
//
// Normative: MenuBar itself is the ONLY focusable part of the whole menu
// family (menu popup rows are never focusable) — a title click both opens
// the popup AND, via the router's ordinary press-to-focus (see
// input.Router.focusFromPath), focuses the bar itself; it STAYS focused for
// as long as any menu popup is open, so Esc reaches MenuBar.OnKey through the
// router's ordinary focused-widget key dispatch, exactly as Esc reaches
// ComboBox.OnKey while its own popup is open (see ComboBox's type doc
// comment). MenuBar.OnKey's Esc handler closes EVERY open popup (top-level
// AND any open submenu chain) via OverlayHost.CloseAllPopups — not merely the
// topmost one.
type MenuBar struct {
	core.Element

	face    *text.Face
	entries []*menuBarEntry

	// cellWidths is recomputed on every MeasureContent pass from the
	// CURRENT entries (title width + 2*PaddingL per cell), mirroring
	// tabStrip.cellWidths. Render/cellAt/cellRect all read this cache rather
	// than re-measuring themselves.
	cellWidths []float32

	hoverIdx int // -1 == no cell hovered
	openIdx  int // -1 == no menu open

	// popup is the currently open TOP-LEVEL popup widget (nil when none is
	// open), kept only so a future explicit-close path could reference it —
	// closing today always goes through OverlayHost.CloseAllPopups rather
	// than a targeted ClosePopup(m.popup) call, but this mirrors ComboBox's
	// own popup bookkeeping for consistency.
	popup core.Widget

	colors  theme.ColorTokens
	metrics theme.MetricTokens
}

// NewMenuBar returns an empty MenuBar (no top-level menus yet), drawing
// titles with face (face may be nil, per TextBlock's own nil-face
// convention — Render then draws nothing, collapsing every cell to just its
// padding).
func NewMenuBar(face *text.Face) *MenuBar {
	th := theme.Active()
	return &MenuBar{
		face:     face,
		hoverIdx: -1,
		openIdx:  -1,
		colors:   th.Color,
		metrics:  th.Metric,
	}
}

// AddMenu appends a new top-level entry titled title and returns a FRESH
// MenuItems builder for its popup contents — callers chain Add/AddSeparator/
// AddSub calls directly onto the returned value, e.g.
// bar.AddMenu("File").Add("New", newFn).AddSeparator().Add("Exit", exitFn).
func (m *MenuBar) AddMenu(title string) *MenuItems {
	items := newMenuItems(m.face, m.colors, m.metrics)
	m.entries = append(m.entries, &menuBarEntry{title: title, items: items})
	m.InvalidateMeasure()
	return items
}

// cellHeight is every cell's text+padding box height: PaddingM above and
// below the line height (nil-face-safe, matching tabStrip.cellHeight's own
// nil-face-collapses-to-padding-only convention).
func (m *MenuBar) cellHeight() float32 {
	var lineH float32
	if m.face != nil {
		lineH = m.face.LineHeight()
	}
	return lineH + 2*m.metrics.PaddingM
}

// MeasureContent recomputes m.cellWidths from the CURRENT entries — each
// cell's width is its title's measured width plus 2*PaddingL (a slightly
// roomier horizontal pad than tabStrip's 2*PaddingM cells, since a menu bar
// reads as a row of well-separated top-level commands rather than tightly
// packed tab headers) — and reports their sum, plus cellHeight(), as the
// bar's own desired size.
func (m *MenuBar) MeasureContent(available render.Size) render.Size {
	widths := make([]float32, len(m.entries))
	var totalW float32
	for i, e := range m.entries {
		w := 2 * m.metrics.PaddingL
		if m.face != nil {
			w += m.face.Measure(e.title).W
		}
		widths[i] = w
		totalW += w
	}
	m.cellWidths = widths

	return render.Size{W: totalW, H: m.cellHeight()}
}

// cellRect returns cell i's absolute rect, computed from m.Bounds() and
// m.cellWidths as of the last layout pass — used both for hit-testing
// (indirectly, via cellAt) and as the anchor rect passed to
// OverlayHost.ShowPopup when opening that cell's menu.
func (m *MenuBar) cellRect(i int) render.Rect {
	bounds := m.Bounds()
	var x float32
	for j := 0; j < i; j++ {
		x += m.cellWidths[j]
	}
	return render.Rect{X: bounds.X + x, Y: bounds.Y, W: m.cellWidths[i], H: bounds.H}
}

// cellAt maps an absolute pointer position to the cell index it falls over,
// using m.Bounds()/m.cellWidths as of the last layout pass — identical in
// shape to tabStrip.cellAt. ok is false for a position outside the bar's own
// bounds, or (defensively) past the last cell.
func (m *MenuBar) cellAt(pos render.Point) (idx int, ok bool) {
	bounds := m.Bounds()
	if !bounds.Contains(pos) {
		return 0, false
	}
	x := pos.X - bounds.X
	var acc float32
	for i, w := range m.cellWidths {
		if x >= acc && x < acc+w {
			return i, true
		}
		acc += w
	}
	return 0, false
}

// Render fills the bar's own bounds ButtonFace, then draws each cell: the
// cell whose menu is currently OPEN gets a classic sunken (drawSunken)
// "pressed" look; otherwise the HOVERED cell gets a navy Highlight bar (and
// HighlightText title); every other cell is plain WindowText title over the
// bar's own ButtonFace. Skipped entirely with a nil face, matching
// TextBlock/tabStrip's own nil-face-renders-nothing convention.
func (m *MenuBar) Render(r render.Renderer) {
	if m.face == nil {
		return
	}
	bounds := m.Bounds()
	cellH := m.cellHeight()
	c := m.colors

	r.FillRect(bounds, c.ButtonFace)

	var x float32
	for i, e := range m.entries {
		w := m.cellWidths[i]
		cell := render.Rect{X: bounds.X + x, Y: bounds.Y, W: w, H: bounds.H}

		textColor := c.WindowText
		switch {
		case i == m.openIdx:
			drawSunken(r, cell, c.ButtonFace, c)
		case i == m.hoverIdx:
			r.FillRect(cell, c.Highlight)
			textColor = c.HighlightText
		}

		ts := m.face.Measure(e.title)
		ty := bounds.Y + (cellH-ts.H)/2
		tx := bounds.X + x + m.metrics.PaddingL
		m.face.Draw(r, render.Point{X: tx, Y: ty}, e.title, textColor)

		x += w
	}
}

// AcceptsFocus implements input.Focusable: the bar always accepts focus (v0
// has no disabled concept for MenuBar) — see the type doc comment for why
// this matters (Esc-to-close routing).
func (m *MenuBar) AcceptsFocus() bool {
	return true
}

// openMenu opens entry idx's popup — a no-op if it's already the open one.
// Any OTHER currently open menu (and its own open submenu chain, if any) is
// closed first via OverlayHost.CloseAllPopups, so at most one top-level menu
// is ever open at a time. A no-op (leaves everything closed) if this MenuBar
// isn't (yet) attached beneath an OverlayHost.
func (m *MenuBar) openMenu(idx int) {
	if idx == m.openIdx {
		return
	}
	host := OverlayHostFor(m)
	if host == nil {
		return
	}
	host.CloseAllPopups()

	entry := m.entries[idx]
	closeAll := func() { host.CloseAllPopups() }
	popup := buildMenuPopup(entry.items, closeAll)

	m.openIdx = idx
	m.popup = popup

	anchor := m.cellRect(idx)
	host.ShowPopup(popup, anchor, func() {
		m.openIdx = -1
		m.popup = nil
	})
}

// OnPointer implements input.PointerHandler: Move/Leave update hoverIdx (-1
// while off the bar entirely, or between/past cells) for the hover-fill
// visual; a Press landing on a real cell opens that cell's menu (openMenu)
// and is marked handled — a press missing every cell (defensive; cellAt/ok
// covers this) is left unhandled.
func (m *MenuBar) OnPointer(e *input.PointerEvent) {
	switch e.Action {
	case input.Move:
		if idx, ok := m.cellAt(e.Pos); ok {
			m.hoverIdx = idx
		} else {
			m.hoverIdx = -1
		}
	case input.Leave:
		m.hoverIdx = -1
	case input.Press:
		if idx, ok := m.cellAt(e.Pos); ok {
			m.openMenu(idx)
			e.Handled = true
		}
	}
}

// OnKey implements input.KeyHandler: Escape, while any menu is open, closes
// EVERY open popup (the top-level menu and any open submenu chain beneath
// it) via OverlayHost.CloseAllPopups, and marks the event handled. A no-op
// (event left unhandled) when no menu is currently open, or for anything but
// Action==Press — matching ComboBox.OnKey's own Esc-guard convention.
func (m *MenuBar) OnKey(e *input.KeyEvent) {
	if e.Action != input.Press || e.Key != input.KeyEscape {
		return
	}
	if m.openIdx == -1 {
		return
	}
	if host := OverlayHostFor(m); host != nil {
		host.CloseAllPopups()
	}
	e.Handled = true
}

// ShowContextMenu opens an app-invoked context (right-click-style) menu,
// built by build, as a MODAL popup anchored at at (screen-space, e.g. the
// pointer position at the moment of the right-click) — its top-left corner
// lands exactly at at (see menuPopupCard.openSub's doc comment for why a
// zero-size anchor rect achieves this via OverlayHost's existing placement
// math, with no new placement code needed).
//
// v0 has no global right-click infrastructure: nothing in fluo itself
// listens for input.ButtonRight and calls this automatically. The app is
// responsible for invoking it — typically from a widget's own OnPointer,
// on seeing e.Button == input.ButtonRight && e.Action == input.Press for a
// press over its own bounds (owner is the anchor for OverlayHostFor, so it
// need not be the same widget that's visually "under" at, just something
// attached beneath the same OverlayHost). Documented v0 simplification, not
// an oversight.
//
// A no-op if owner isn't (yet) attached beneath an OverlayHost. Each click on
// a resulting item row fires that item's own onClick and then closes every
// open popup (see buildMenuPopup) — same as any other menu popup. The popup
// itself is opened with a nil onDismiss: unlike MenuBar (which resets
// openIdx/popup on dismiss) or ComboBox (which resets open/popup),
// ShowContextMenu keeps no state of its own to reset — every call is a fresh,
// independent popup.
func ShowContextMenu(owner core.Widget, at render.Point, face *text.Face, build func(*MenuItems)) {
	host := OverlayHostFor(owner)
	if host == nil {
		return
	}

	th := theme.Active()
	items := newMenuItems(face, th.Color, th.Metric)
	if build != nil {
		build(items)
	}

	closeAll := func() { host.CloseAllPopups() }
	popup := buildMenuPopup(items, closeAll)

	anchor := render.Rect{X: at.X, Y: at.Y}
	host.ShowPopup(popup, anchor, nil)
}
