package controls

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
	"github.com/0xdreadnaught/fluo/theme"
)

// tabUnderlineThickness is the height of the 2px Accent underline drawn
// beneath the SELECTED header cell (per the brief's "selected: 2px Accent
// underline" normative rule) — reserved as extra height below every cell's
// own text+padding box, so the underline never overlaps a descender.
const tabUnderlineThickness float32 = 2

// tabItem is one tab owned by a TabControl: its header title and its
// content widget. Both are set once, at AddTab time — v0 has no
// RemoveTab/SetTitle.
type tabItem struct {
	title   string
	content core.Widget
}

// tabVisibilitySetter is satisfied by any widget embedding core.Element
// (which promotes SetVisible) — effectively every widget in this package.
// TabControl needs to toggle a tab's content visibility while it
// UNCONDITIONALLY remains in Children() (see TabControl's doc comment for
// why — the brief's "hidden tabs stay in the tree" normative rule), which
// core.Widget itself has no notion of; this narrow interface lets
// applySelection flip visibility on an arbitrary core.Widget content value
// without depending on its concrete type. A content widget that (unusually)
// does not implement it is simply never hidden/shown by selection — a soft
// failure mode, not a panic.
type tabVisibilitySetter interface{ SetVisible(bool) }

func setTabContentVisible(w core.Widget, visible bool) {
	if vs, ok := w.(tabVisibilitySetter); ok {
		vs.SetVisible(visible)
	}
}

// tabStrip is TabControl's header row: one cell per tab, drawn directly
// (no per-cell child widgets — mirroring TreeView's row-drawing convention,
// since the cell count/widths are entirely driven by tab titles measured
// against face) rather than composed from TextBlocks like ComboBox/Expander.
//
// Normative: the strip is TabControl's SINGLE focusable unit. The focus
// ring (RenderOverlay) hugs the SELECTED tab's header cell — never the
// content area below it — matching WinUI, where the focus rectangle tracks
// the active tab header rather than the whole tab list. Left/Right while the
// strip is focused switch tabs (OnKey), clamped at the ends (no wrap),
// matching ListView/TreeView's clampRowIndex convention exactly; the ring
// follows the selection since focus and selection move together.
type tabStrip struct {
	core.Element

	face  *text.Face
	owner *TabControl

	// cellWidths is recomputed on every MeasureContent pass from the
	// owner's CURRENT tabs (title width + 2*PaddingM per cell — the
	// brief's normative header hit-zone width). Render/OnPointer/OnKey all
	// read this cache rather than re-measuring themselves.
	cellWidths []float32

	hoverIdx int // -1 == no cell hovered
	focused  bool

	colors  theme.ColorTokens
	metrics theme.MetricTokens
}

// newTabStrip returns a tabStrip for owner, drawing titles with face (face
// may be nil, per TextBlock's own nil-face convention — Measure/Draw are
// simply skipped, collapsing every cell to just its 2*PaddingM padding).
func newTabStrip(face *text.Face, owner *TabControl, colors theme.ColorTokens, metrics theme.MetricTokens) *tabStrip {
	return &tabStrip{face: face, owner: owner, hoverIdx: -1, colors: colors, metrics: metrics}
}

// cellHeight is the text+padding box height for every cell: PaddingM above
// and below the line height (nil-face-safe, matching defaultRowHeight's own
// nil-face-collapses-to-padding-only convention) — NOT including the
// underline reserve (see tabUnderlineThickness).
func (s *tabStrip) cellHeight() float32 {
	var lineH float32
	if s.face != nil {
		lineH = s.face.LineHeight()
	}
	return lineH + 2*s.metrics.PaddingM
}

// MeasureContent recomputes s.cellWidths from the owner's current tabs —
// each cell's width is its title's measured width plus 2*PaddingM (the
// brief's normative header hit-zone width; a nil face measures every title
// to zero, collapsing every cell to just its padding) — and reports their
// sum, plus cellHeight()+tabUnderlineThickness, as the strip's own desired
// size.
func (s *tabStrip) MeasureContent(available render.Size) render.Size {
	tabs := s.owner.tabs
	widths := make([]float32, len(tabs))
	var totalW float32
	for i, tab := range tabs {
		w := 2 * s.metrics.PaddingM
		if s.face != nil {
			w += s.face.Measure(tab.title).W
		}
		widths[i] = w
		totalW += w
	}
	s.cellWidths = widths

	return render.Size{W: totalW, H: s.cellHeight() + tabUnderlineThickness}
}

// cellAt maps an absolute pointer position to the header cell index it
// falls over, using s.Bounds()/s.cellWidths as of the last layout pass —
// contiguous zones in tab order, each exactly cellWidths[i] wide (the
// brief's normative "header hit zones = title width + 2*PaddingM").
// ok is false for a position outside the strip's own bounds, or (
// defensively) past the last cell.
func (s *tabStrip) cellAt(pos render.Point) (idx int, ok bool) {
	bounds := s.Bounds()
	if !bounds.Contains(pos) {
		return 0, false
	}
	x := pos.X - bounds.X
	var acc float32
	for i, w := range s.cellWidths {
		if x >= acc && x < acc+w {
			return i, true
		}
		acc += w
	}
	return 0, false
}

// Render draws each cell: ControlFillHover behind the hovered cell
// (regardless of selection), the title (TextPrimary for the selected cell,
// TextSecondary otherwise), and — for the selected cell only — a 2px Accent
// underline spanning its full width just below the text+padding box.
// Skipped entirely with a nil face, matching TextBlock/TreeView's own
// nil-face-renders-nothing convention.
func (s *tabStrip) Render(r render.Renderer) {
	if s.face == nil {
		return
	}
	bounds := s.Bounds()
	cellH := s.cellHeight()
	selected := s.owner.SelectedIndex()

	var x float32
	for i, tab := range s.owner.tabs {
		w := s.cellWidths[i]

		if i == s.hoverIdx {
			r.FillRect(render.Rect{X: bounds.X + x, Y: bounds.Y, W: w, H: bounds.H}, s.colors.ControlFillHover)
		}

		color := s.colors.TextSecondary
		if i == selected {
			color = s.colors.TextPrimary
		}
		ts := s.face.Measure(tab.title)
		ty := bounds.Y + (cellH-ts.H)/2
		tx := bounds.X + x + s.metrics.PaddingM
		s.face.Draw(r, render.Point{X: tx, Y: ty}, tab.title, color)

		if i == selected {
			underline := render.Rect{X: bounds.X + x, Y: bounds.Y + cellH, W: w, H: tabUnderlineThickness}
			r.FillRect(underline, s.colors.Accent)
		}

		x += w
	}
}

// cellRect returns tab i's header cell rect (the header box only, excluding
// the selection underline), using s.Bounds()/s.cellWidths as of the last
// layout pass — cells are contiguous in tab order, each cellWidths[i] wide,
// exactly as Render and cellAt walk them. Out-of-range i yields a zero-width
// rect at the accumulated offset.
func (s *tabStrip) cellRect(i int) render.Rect {
	bounds := s.Bounds()
	var x float32
	for k := 0; k < i && k < len(s.cellWidths); k++ {
		x += s.cellWidths[k]
	}
	var w float32
	if i >= 0 && i < len(s.cellWidths) {
		w = s.cellWidths[i]
	}
	return render.Rect{X: bounds.X + x, Y: bounds.Y, W: w, H: s.cellHeight()}
}

// RenderOverlay draws the focus ring around the SELECTED tab's header cell
// while the strip is focused, hugging that one cell rather than the whole
// strip — matching WinUI, where the focus rectangle tracks the active tab
// header, not the entire tab list. Arrow keys move the selection (OnKey), so
// the focused cell is always the selected one. The strip remains the single
// focusable unit; this only changes where the ring is drawn.
func (s *tabStrip) RenderOverlay(r render.Renderer) {
	if s.focused && s.face != nil && len(s.owner.tabs) > 0 {
		drawFocusRing(r, s.cellRect(s.owner.SelectedIndex()), s.colors)
	}
}

// AcceptsFocus implements input.Focusable: the strip always accepts focus
// (v0 has no disabled concept for TabControl).
func (s *tabStrip) AcceptsFocus() bool {
	return true
}

// OnFocusChanged implements input.FocusHandler, tracking focus for the
// focus-ring overlay and Left/Right keyboard navigation.
func (s *tabStrip) OnFocusChanged(focused bool) {
	s.focused = focused
}

// OnPointer implements input.PointerHandler: Move/Leave update hoverIdx (-1
// while off the strip entirely, or between/past cells) for the hover-fill
// visual; a Press landing on a real cell selects it as a user-driven change
// (owner.selectUser) and is marked handled — a press missing every cell
// (defensive; cellAt/ok covers this) is left unhandled.
func (s *tabStrip) OnPointer(e *input.PointerEvent) {
	switch e.Action {
	case input.Move:
		if idx, ok := s.cellAt(e.Pos); ok {
			s.hoverIdx = idx
		} else {
			s.hoverIdx = -1
		}
	case input.Leave:
		s.hoverIdx = -1
	case input.Press:
		if idx, ok := s.cellAt(e.Pos); ok {
			s.owner.selectUser(idx)
			e.Handled = true
		}
	}
}

// OnKey implements input.KeyHandler: Left/Right move the selection by one
// tab, clamped into [0, len(tabs)-1] via clampRowIndex (shared with
// ListView/TreeView) — CLAMPED, no wrap, per the brief's normative rule —
// as a user-driven change (owner.selectUser). Ignored entirely for anything
// but Action==Press, or when there are no tabs at all.
func (s *tabStrip) OnKey(e *input.KeyEvent) {
	if e.Action != input.Press {
		return
	}
	n := len(s.owner.tabs)
	if n == 0 {
		return
	}
	switch e.Key {
	case input.KeyLeft:
		s.owner.selectUser(clampRowIndex(s.owner.selected-1, n))
		e.Handled = true
	case input.KeyRight:
		s.owner.selectUser(clampRowIndex(s.owner.selected+1, n))
		e.Handled = true
	}
}

// TabControl is a composite of a header strip (see tabStrip) and a content
// area showing the selected tab's content below it. TabControl itself draws
// no chrome of its own (no Render override — matching Expander/StackPanel,
// a pure layout composite); all visible chrome belongs to the strip.
//
// Normative, and the key way TabControl differs from Expander's "content
// participates in layout only while expanded" rule: every tab's content
// widget is measured, arranged, AND returned from Children() UNCONDITIONALLY
// — only the selected one is ever visible (SetVisible(true), the rest
// SetVisible(false)). This means a hidden tab's content stays reachable in
// the tree (for e.g. a later SetSelectedIndex to reveal, or external code
// walking Children()) and measures/arranges to {0,0} via the CORE ENGINE's
// own hidden-widget shortcut (core.MeasureWidget/ArrangeWidget both
// early-out on a hidden element — see core/widget.go), not via any special
// casing here: TabControl's own MeasureContent/ArrangeContent simply run
// every tab's content through core.MeasureWidget/ArrangeWidget every pass,
// same as DockPanel arranges every child regardless of IsVisible.
//
// Selection mirrors ListView/TreeView's convention: selected is a plain int
// (never -1 — a TabControl with at least one tab always has one selected,
// matching clampRowIndex's "never lands on -1" contract; 0 with zero tabs is
// a harmless degenerate value never actually observable as a real
// selection), set programmatically (silent, SetSelectedIndex) or by the
// user (header cell click, or Left/Right while the strip is focused — both
// funneled through selectUser, which fires OnChanged only on an actual
// change).
type TabControl struct {
	core.Element

	strip *tabStrip
	tabs  []tabItem

	selected  int
	onChanged func(int)
}

// NewTabControl returns an empty TabControl (no tabs yet), drawing header
// titles with face (face may be nil, per tabStrip's own nil-face
// convention), styled from theme.Active() at construction (rebuild to
// re-theme).
func NewTabControl(face *text.Face) *TabControl {
	th := theme.Active()
	t := &TabControl{}
	t.strip = newTabStrip(face, t, th.Color, th.Metric)
	core.SetParent(t.strip, t)
	return t
}

// AddTab appends a new tab with the given title and content widget,
// re-parenting content to this TabControl. content starts visible only if
// this is the tab AT the current selected index (index 0 the very first
// time AddTab is called, since selected starts at 0) — every other tab's
// content starts hidden (see the type doc comment's "unconditionally in the
// tree, only the selected one visible" rule). Returns t for chaining, per
// the brief's normative AddTab signature.
func (t *TabControl) AddTab(title string, content core.Widget) *TabControl {
	core.SetParent(content, t)
	idx := len(t.tabs)
	t.tabs = append(t.tabs, tabItem{title: title, content: content})
	setTabContentVisible(content, idx == t.selected)
	t.strip.InvalidateMeasure()
	return t
}

// SelectedIndex returns the currently selected tab's index (0 if there are
// no tabs at all — see the type doc comment).
func (t *TabControl) SelectedIndex() int {
	return t.selected
}

// clampTabIndex clamps i into [0, n-1], like clampRowIndex, but ALSO
// collapses n == 0 to a real 0 rather than clampRowIndex's -1 (clampRowIndex
// itself documents n==0 as "never actually read since every caller already
// checked n > 0 first" — true for ListView/TreeView's own OnKey guards, but
// NOT true here: SetSelectedIndex/selectUser can run with zero tabs, and
// TabControl's selected must stay a harmless 0, never -1 — see the type doc
// comment).
func clampTabIndex(i, n int) int {
	if n == 0 {
		return 0
	}
	return clampRowIndex(i, n)
}

// applySelection is the shared core of SetSelectedIndex/selectUser: assigns
// the new (already-clamped) index, flips every tab's content visibility to
// match (see setTabContentVisible), and invalidates measure (a hidden↔
// visible content swap can change the control's own desired size, so — like
// Expander's expand/collapse — this must be a measure invalidation, not
// merely an arrange one).
func (t *TabControl) applySelection(i int) {
	t.selected = i
	for idx, tab := range t.tabs {
		setTabContentVisible(tab.content, idx == i)
	}
	t.InvalidateMeasure()
}

// SetSelectedIndex sets the selection programmatically, clamped into
// [0, len(tabs)-1] via clampTabIndex (n == 0 collapses to 0, a harmless
// degenerate result — see the type doc comment). Silent: never fires
// OnChanged, matching the package's uniform contract that programmatic
// setters are silent (OnChanged reports only user-driven changes).
func (t *TabControl) SetSelectedIndex(i int) *TabControl {
	t.applySelection(clampTabIndex(i, len(t.tabs)))
	return t
}

// OnChanged sets the callback fired with the new index whenever the user
// changes the selection — by clicking a header cell, or navigating with
// Left/Right while the strip is focused — but never for a programmatic
// SetSelectedIndex (see its doc comment). Replaces any previously set
// callback; a nil fn is a valid, silent no-op.
func (t *TabControl) OnChanged(fn func(int)) *TabControl {
	t.onChanged = fn
	return t
}

// selectUser is the user-driven selection path (header cell click, strip
// Left/Right): clamps i via clampTabIndex, applies the same visibility
// swap as SetSelectedIndex, and fires OnChanged only if the selection
// actually changed — matching ListView.selectUser/TreeView.selectUser's
// "notify only on real change" convention.
func (t *TabControl) selectUser(i int) {
	i = clampTabIndex(i, len(t.tabs))
	changed := i != t.selected
	t.applySelection(i)
	if changed && t.onChanged != nil {
		t.onChanged(i)
	}
}

// MeasureContent measures the strip unconditionally, then EVERY tab's
// content (not just the selected one — see the type doc comment) in the
// width available and whatever height remains after the strip's own
// desired height; a hidden content widget measures to {0,0} via the core
// engine's own hidden shortcut, so only the selected tab's content ever
// actually contributes to the max below. Reports {max(strip width, max
// content width), strip height + max content height}.
func (t *TabControl) MeasureContent(available render.Size) render.Size {
	core.MeasureWidget(t.strip, available)
	stripD := core.DesiredSizeOf(t.strip)

	availH := available.H - stripD.H
	if availH < 0 {
		availH = 0
	}

	w := stripD.W
	var contentH float32
	for _, tab := range t.tabs {
		core.MeasureWidget(tab.content, render.Size{W: available.W, H: availH})
		d := core.DesiredSizeOf(tab.content)
		if d.W > w {
			w = d.W
		}
		if d.H > contentH {
			contentH = d.H
		}
	}

	return render.Size{W: w, H: stripD.H + contentH}
}

// ArrangeContent arranges the strip across the full bounds width at its own
// measured height, then EVERY tab's content (not just the selected one)
// directly below it, filling the remaining width/height — a hidden content
// widget arranges to {0,0} via the core engine's own hidden shortcut (see
// core.ArrangeWidget), mirroring MeasureContent.
func (t *TabControl) ArrangeContent(bounds render.Rect) {
	stripD := core.DesiredSizeOf(t.strip)
	core.ArrangeWidget(t.strip, render.Rect{X: bounds.X, Y: bounds.Y, W: bounds.W, H: stripD.H})

	contentY := bounds.Y + stripD.H
	contentH := bounds.H - stripD.H
	if contentH < 0 {
		contentH = 0
	}

	for _, tab := range t.tabs {
		core.ArrangeWidget(tab.content, render.Rect{X: bounds.X, Y: contentY, W: bounds.W, H: contentH})
	}
}

// Children returns the strip plus EVERY tab's content, in tab order —
// unconditionally, regardless of which one is currently selected/visible
// (see the type doc comment's "hidden tabs remain in the tree" normative
// rule; contrast Expander.Children, which excludes collapsed content
// entirely). A fresh slice each call; mutating it does not affect the
// control.
func (t *TabControl) Children() []core.Widget {
	out := make([]core.Widget, 0, len(t.tabs)+1)
	out = append(out, t.strip)
	for _, tab := range t.tabs {
		out = append(out, tab.content)
	}
	return out
}
