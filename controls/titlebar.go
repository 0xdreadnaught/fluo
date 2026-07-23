package controls

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
	"github.com/0xdreadnaught/fluo/theme"
)

// titleBarHeight is TitleBar's normative fixed height (Fluent custom
// titlebars run ~32px). captionButtonWidth is each of the three caption
// buttons' fixed width — noticeably wider than tall, matching Fluent's own
// caption button proportions — at the bar's full height.
const (
	titleBarHeight     float32 = 32
	captionButtonWidth float32 = 46
)

// captionGlyphSize is the edge length of the square each caption button's
// glyph shape is drawn within, centered in its own cell.
const captionGlyphSize float32 = 10

// captionKind identifies which of the three caption buttons a captionButton
// is, driving both its hover fill (close gets the distinct CloseButtonHover
// red; minimize/maximize get the ordinary ControlFillHover/Pressed) and
// which glyph shape Render draws.
type captionKind uint8

const (
	captionMinimize captionKind = iota
	captionMaximize
	captionClose
)

// captionButton is TitleBar's caption-cell widget: a fixed-size
// (captionButtonWidth x titleBarHeight), always-enabled, ClickBehavior-
// driven cell. It draws no text label — Render paints a hover/pressed fill
// (fully transparent at rest, so the TitleBar's own LayerBackground fill
// shows through) and a small drawn shape glyph (a line, a square outline, or
// an X) rather than a text/font glyph: drawn shapes render identically
// regardless of what glyphs a font happens to carry, unlike e.g. CheckBox's
// U+2713 fallback dance (see hasCheckmarkGlyph) — simpler and more reliable
// for chrome this small.
type captionButton struct {
	core.Element

	click ClickBehavior
	kind  captionKind

	colors theme.ColorTokens
}

// newCaptionButton returns an enabled captionButton of the given kind,
// styled from theme.Active() at construction (rebuild to re-theme, matching
// every other control in this package).
func newCaptionButton(kind captionKind) *captionButton {
	return &captionButton{kind: kind, colors: theme.Active().Color}
}

// MeasureContent always reports the fixed caption-cell size, ignoring
// available space — TitleBar arranges these at that exact size directly
// (see TitleBar.ArrangeContent), so what a parent happens to offer never
// matters.
func (b *captionButton) MeasureContent(available render.Size) render.Size {
	return render.Size{W: captionButtonWidth, H: titleBarHeight}
}

// fill resolves the cell's background: fully transparent at rest, else
// ControlFillHover/ControlFillPressed for minimize/maximize, or the
// distinct CloseButtonHover red for close — for BOTH its hover and pressed
// states, matching Fluent's own close button (which uses one red for both,
// relying on the OS compositor's press-flash for pressed feedback, not a
// second red shade — fluo does not attempt to reproduce that flash here).
func (b *captionButton) fill() render.Color {
	hot := b.click.Hover() || b.click.Pressed()
	if !hot {
		return render.Color{}
	}
	if b.kind == captionClose {
		return b.colors.CloseButtonHover
	}
	if b.click.Pressed() {
		return b.colors.ControlFillPressed
	}
	return b.colors.ControlFillHover
}

// glyphColor is TextPrimary normally, or AccentText (white, in both bundled
// Fluent themes) while the close button's red fill is showing, for contrast.
func (b *captionButton) glyphColor() render.Color {
	if b.kind == captionClose && (b.click.Hover() || b.click.Pressed()) {
		return b.colors.AccentText
	}
	return b.colors.TextPrimary
}

// Render paints the hover/pressed fill (if any, per fill) then the button's
// glyph shape, both centered within the cell's own bounds.
func (b *captionButton) Render(r render.Renderer) {
	bounds := b.Bounds()
	if fill := b.fill(); fill.A > 0 {
		r.FillRect(bounds, fill)
	}

	g := render.Rect{
		X: bounds.X + (bounds.W-captionGlyphSize)/2,
		Y: bounds.Y + (bounds.H-captionGlyphSize)/2,
		W: captionGlyphSize, H: captionGlyphSize,
	}
	color := b.glyphColor()
	switch b.kind {
	case captionMinimize:
		// A single horizontal line ("–") centered in the glyph box.
		r.FillRect(render.Rect{X: g.X, Y: g.Y + g.H/2 - 0.5, W: g.W, H: 1}, color)
	case captionMaximize:
		// A square outline ("☐"): a zero-radius stroked rect.
		r.StrokeRoundedRect(g, 0, 1, color)
	case captionClose:
		drawX(r, g, 1, color)
	}
}

// OnPointer implements input.PointerHandler, delegating the whole press/
// hover state machine to the embedded ClickBehavior — caption buttons have
// no disabled state (TitleBar never disables them).
func (b *captionButton) OnPointer(e *input.PointerEvent) {
	b.click.HandlePointer(e, b)
}

// drawX draws an X (two crossing diagonal strokes) inside rect using
// thickness x thickness dabs stepped pixel-by-pixel along both diagonals.
// render.Renderer has no primitive for an arbitrary-angle line (only
// axis-aligned rects/rounded-rects), so a diagonal "stroke" is approximated
// by laying a small filled square at every integer step along the diagonal,
// packed closely enough (one step per pixel of the glyph's larger axis) to
// read as a solid line at caption-glyph sizes.
func drawX(r render.Renderer, rect render.Rect, thickness float32, c render.Color) {
	steps := int(rect.W)
	if int(rect.H) > steps {
		steps = int(rect.H)
	}
	if steps < 1 {
		steps = 1
	}
	for i := 0; i <= steps; i++ {
		t := float32(i) / float32(steps)
		dab(r, rect.X+t*rect.W, rect.Y+t*rect.H, thickness, c)        // top-left -> bottom-right
		dab(r, rect.X+rect.W-t*rect.W, rect.Y+t*rect.H, thickness, c) // top-right -> bottom-left
	}
}

// dab fills a thickness x thickness square centered at (cx, cy).
func dab(r render.Renderer, cx, cy, thickness float32, c render.Color) {
	half := thickness / 2
	r.FillRect(render.Rect{X: cx - half, Y: cy - half, W: thickness, H: thickness}, c)
}

// TitleBar is a full-width, fixed-height (titleBarHeight) window-chrome
// widget: a title label at the left (BodySize face, TextPrimary — the
// caller supplies a BodySize face, matching every other control in this
// package) and three caption buttons (minimize/maximize/close) right-
// aligned (close rightmost), filled with LayerBackground.
//
// TitleBar knows nothing about the actual OS window: minimize/maximize/
// close are just callbacks (OnMinimize/OnMaximize/OnClose), and dragging is
// exposed only as DragRegion, a pure geometry query. app/window.go is what
// actually calls glfw's Iconify/Maximize-Restore/SetShouldClose from those
// callbacks and drives the window-move loop from DragRegion — keeping this
// widget windowing-library agnostic like every other control in this
// package (see the package-level "no glfw types outside app" constraint).
type TitleBar struct {
	core.Element

	title           *TextBlock
	min, max, close *captionButton

	colors  theme.ColorTokens
	metrics theme.MetricTokens

	onMinimize, onMaximize, onClose func()
}

// NewTitleBar returns a TitleBar showing title in face (face may be nil, per
// TextBlock), with its three caption buttons already wired to fire
// OnMinimize/OnMaximize/OnClose whenever those are set (nil until set, in
// which case a click is a silent no-op — the button still shows its
// hover/pressed feedback either way).
func NewTitleBar(face *text.Face, title string) *TitleBar {
	t := &TitleBar{}
	th := theme.Active()
	t.colors = th.Color
	t.metrics = th.Metric

	t.title = NewTextBlock(face, title)
	core.SetParent(t.title, t)

	t.min = newCaptionButton(captionMinimize)
	t.max = newCaptionButton(captionMaximize)
	t.close = newCaptionButton(captionClose)
	core.SetParent(t.min, t)
	core.SetParent(t.max, t)
	core.SetParent(t.close, t)

	t.min.click.OnClick = func() {
		if t.onMinimize != nil {
			t.onMinimize()
		}
	}
	t.max.click.OnClick = func() {
		if t.onMaximize != nil {
			t.onMaximize()
		}
	}
	t.close.click.OnClick = func() {
		if t.onClose != nil {
			t.onClose()
		}
	}

	return t
}

// SetTitle sets the displayed title text. Returns t for chaining.
func (t *TitleBar) SetTitle(s string) *TitleBar {
	t.title.SetText(s)
	return t
}

// OnMinimize sets the callback fired when the minimize caption button is
// clicked. Replaces any previously set callback; a nil fn is a valid,
// silent no-op. Returns t for chaining.
func (t *TitleBar) OnMinimize(fn func()) *TitleBar {
	t.onMinimize = fn
	return t
}

// OnMaximize sets the callback fired when the maximize caption button is
// clicked. Replaces any previously set callback; a nil fn is a valid,
// silent no-op. Returns t for chaining.
func (t *TitleBar) OnMaximize(fn func()) *TitleBar {
	t.onMaximize = fn
	return t
}

// OnClose sets the callback fired when the close caption button is clicked.
// Replaces any previously set callback; a nil fn is a valid, silent no-op.
// Returns t for chaining.
func (t *TitleBar) OnClose(fn func()) *TitleBar {
	t.onClose = fn
	return t
}

// MeasureContent stretches to the available width at the fixed
// titleBarHeight, regardless of available height — an ordinary full-width
// titlebar sitting under a window's top edge. The three caption buttons
// each measure to their own fixed size; the title measures within whatever
// width remains after them (clamped to >= 0).
func (t *TitleBar) MeasureContent(available render.Size) render.Size {
	captionSize := render.Size{W: captionButtonWidth, H: titleBarHeight}
	core.MeasureWidget(t.min, captionSize)
	core.MeasureWidget(t.max, captionSize)
	core.MeasureWidget(t.close, captionSize)

	titleAvailW := available.W - 3*captionButtonWidth - t.metrics.PaddingL
	if titleAvailW < 0 {
		titleAvailW = 0
	}
	core.MeasureWidget(t.title, render.Size{W: titleAvailW, H: titleBarHeight})

	return render.Size{W: available.W, H: titleBarHeight}
}

// ArrangeContent arranges the three caption buttons right-aligned at the
// bar's full height (close rightmost, then maximize, then minimize, each
// captionButtonWidth wide and abutting the next with no gap, matching
// Fluent's own flush caption-button row) and the title at the bar's left
// edge, inset by PaddingL and vertically centered, occupying whatever width
// remains before the caption buttons begin.
func (t *TitleBar) ArrangeContent(bounds render.Rect) {
	closeRect := render.Rect{X: bounds.Right() - captionButtonWidth, Y: bounds.Y, W: captionButtonWidth, H: bounds.H}
	maxRect := render.Rect{X: closeRect.X - captionButtonWidth, Y: bounds.Y, W: captionButtonWidth, H: bounds.H}
	minRect := render.Rect{X: maxRect.X - captionButtonWidth, Y: bounds.Y, W: captionButtonWidth, H: bounds.H}

	core.ArrangeWidget(t.close, closeRect)
	core.ArrangeWidget(t.max, maxRect)
	core.ArrangeWidget(t.min, minRect)

	titleD := core.DesiredSizeOf(t.title)
	tx := bounds.X + t.metrics.PaddingL
	ty := bounds.Y + (bounds.H-titleD.H)/2
	tw := minRect.X - tx
	if tw < 0 {
		tw = 0
	}
	core.ArrangeWidget(t.title, render.Rect{X: tx, Y: ty, W: tw, H: titleD.H})
}

// Render fills the bar's own bounds with LayerBackground; the title and
// caption buttons render separately as children (see Children).
func (t *TitleBar) Render(r render.Renderer) {
	r.FillRect(t.Bounds(), t.colors.LayerBackground)
}

// Children returns the title label and the three caption buttons, in that
// order.
func (t *TitleBar) Children() []core.Widget {
	return []core.Widget{t.title, t.min, t.max, t.close}
}

// DragRegion reports whether point p (logical, absolute window coordinates
// — the same space core.BoundsOf/ArrangeWidget use) is in the draggable
// area: within the bar's own bounds AND not over any of the three caption
// buttons. The host (app/window.go) uses this to decide whether a press
// should start moving the window (see Ctx.BeginDrag).
func (t *TitleBar) DragRegion(p render.Point) bool {
	if !t.Bounds().Contains(p) {
		return false
	}
	for _, b := range [...]*captionButton{t.min, t.max, t.close} {
		if core.BoundsOf(b).Contains(p) {
			return false
		}
	}
	return true
}
