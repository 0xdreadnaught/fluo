package controls

import (
	"time"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
	"github.com/0xdreadnaught/fluo/theme"
	"github.com/0xdreadnaught/fluo/timers"
)

// textBoxDefaultWidth is the desired content width reported by
// MeasureContent when no explicit width has been set on the TextBox (via
// core.Element.SetWidth) — a sane single-line-input default, per the Phase 5
// Task 5 spec. An explicit SetWidth overrides it through the normal
// core.MeasureWidget explicit-size precedence (see core/widget.go); TextBox's
// own MeasureContent never needs to look at the available size to honor
// that.
const textBoxDefaultWidth float32 = 160

// caretBlinkPeriod is the interval at which caretVisible toggles once a
// timers.Queue has been wired via SetTimers (normative: 530ms, matching
// common desktop UI caret-blink cadences).
const caretBlinkPeriod = 530 * time.Millisecond

// caretWidth is the drawn width of the caret bar (normative: 1.5, a hair
// wider than a hairline 1px rule so it stays visible after SDF/AA rounding).
const caretWidth float32 = 1.5

// TextBox is a single-line, focusable, token-styled text input. This is the
// Phase 5 Task 5 slice: the data model (text/caret/selection, rune-indexed)
// and rendering (chrome, selection highlight, caret, horizontal scroll,
// placeholder) are complete, but no pointer/keyboard EDITING is wired yet —
// that is Task 6 (OnKey/OnPointer/CursorShaper). TextBox already implements
// input.Focusable and input.FocusHandler (AcceptsFocus/OnFocusChanged) since
// focus is purely a rendering concern here (the focus-ring overlay and the
// focused border color); a router can already Focus() a TextBox today, it
// just can't yet edit it.
//
// Rune indices throughout (Caret, Selection, SetCaret, Select) are RUNE
// indices into Text(), not byte offsets — text is stored as []rune
// internally so multibyte characters (e.g. "héllo") never split a codepoint.
// v0 simplification: every index-based operation is O(n) (converts via
// []rune / string(runes[...]) as needed); fine for realistic single-line
// input lengths.
type TextBox struct {
	core.Element

	face *text.Face

	runes       []rune
	placeholder string

	// caret is the actual caret position; anchor is the other end of the
	// selection (equal to caret when there is no selection). Selection()
	// normalizes these into start<=end; Caret() always reports the raw
	// caret value (which may be either endpoint after Select).
	caret, anchor int

	enabled bool
	focused bool

	// hscroll is the horizontal text-scroll offset in logical px, clamped
	// during ArrangeContent (see updateHScroll) so the caret always stays
	// within the inner (padding-inset) width — the ScrollViewer
	// clamp-in-arrange pattern applied to a single scroll axis.
	hscroll float32

	// timerQueue and blinkTimer drive caret blinking once SetTimers wires a
	// non-nil queue; nil timerQueue means "solid caret" (caretShown ignores
	// caretVisible entirely in that case). blinkTimer is stopped and
	// replaced whenever SetTimers is called again, so a superseded queue can
	// never keep toggling a textbox that has moved on to a different one
	// (or none).
	timerQueue   *timers.Queue
	blinkTimer   *timers.Timer
	caretVisible bool

	onChanged func(string)

	colors  theme.ColorTokens
	metrics theme.MetricTokens
}

// NewTextBox returns an enabled, empty, unfocused TextBox drawing text with
// face (face may be nil, in which case it measures/draws no text — a
// degenerate but valid state, matching TextBlock's nil-face convention).
// Colors and metrics are captured from theme.Active() at construction
// (rebuild to re-theme, matching every other control in this package).
func NewTextBox(face *text.Face) *TextBox {
	th := theme.Active()
	return &TextBox{
		face:         face,
		enabled:      true,
		caretVisible: true,
		colors:       th.Color,
		metrics:      th.Metric,
	}
}

// Text returns the current text content.
func (t *TextBox) Text() string {
	return string(t.runes)
}

// SetText replaces the text content, resets the caret to the end, and
// clears any selection. Normative: unlike most SetX setters in this package,
// SetText DOES fire OnChanged (even when s equals the current text) — giving
// programmatic SetText the same notification parity as typing, which
// Phase 6's data-binding work depends on.
func (t *TextBox) SetText(s string) *TextBox {
	t.runes = []rune(s)
	t.caret = len(t.runes)
	t.anchor = t.caret
	if t.onChanged != nil {
		t.onChanged(s)
	}
	return t
}

// SetPlaceholder sets the text shown (in TextDisabled color) whenever
// Text() == "", regardless of focus state — simpler and more common than
// hiding it while focused, and the normative choice for this control.
func (t *TextBox) SetPlaceholder(s string) *TextBox {
	t.placeholder = s
	return t
}

// SetEnabled toggles whether the box accepts focus and (once Task 6 wires
// it) pointer/keyboard editing. Purely visual/behavioral: no invalidation
// needed.
func (t *TextBox) SetEnabled(v bool) *TextBox {
	t.enabled = v
	return t
}

// OnChanged sets the callback fired with the new text whenever it changes —
// today, only via SetText (Task 6 will additionally fire it from every
// editing operation). Replaces any previously set callback; a nil fn is a
// valid, silent no-op.
func (t *TextBox) OnChanged(fn func(string)) *TextBox {
	t.onChanged = fn
	return t
}

// SetTimers wires q as the caret-blink driver: SetTimers schedules a
// repeating callback every caretBlinkPeriod that flips caretVisible, and the
// caret is drawn only while caretVisible is true. Passing nil detaches any
// previously wired queue and reverts to a solid (always-visible-while-
// focused) caret. Calling SetTimers again (with a different queue, or nil)
// always stops the previously scheduled timer first, so a superseded queue
// can never keep toggling this textbox's caret after the fact.
func (t *TextBox) SetTimers(q *timers.Queue) *TextBox {
	if t.blinkTimer != nil {
		t.blinkTimer.Stop()
		t.blinkTimer = nil
	}
	t.timerQueue = q
	t.caretVisible = true
	if q != nil {
		t.blinkTimer = q.Every(caretBlinkPeriod, func() {
			t.caretVisible = !t.caretVisible
		})
	}
	return t
}

// Caret returns the current caret rune index (0..len(runes)).
func (t *TextBox) Caret() int {
	return t.caret
}

// Selection returns the selected rune range, normalized so start<=end.
// Returns (caret,caret) when there is no selection (anchor==caret).
func (t *TextBox) Selection() (start, end int) {
	if t.anchor < t.caret {
		return t.anchor, t.caret
	}
	return t.caret, t.anchor
}

// SetCaret moves the caret to rune index i (clamped to [0, len(runes)]) and
// clears any selection (anchor becomes equal to the new caret).
func (t *TextBox) SetCaret(i int) *TextBox {
	i = clampInt(i, 0, len(t.runes))
	t.caret = i
	t.anchor = i
	return t
}

// Select sets the selection to [anchor, caret) (each independently clamped
// to [0, len(runes)]); anchor may be greater than caret (Selection() always
// normalizes), and Caret() reports the raw caret argument afterward — it is
// the actual caret position, which is not necessarily the normalized range
// start.
func (t *TextBox) Select(anchor, caret int) *TextBox {
	t.anchor = clampInt(anchor, 0, len(t.runes))
	t.caret = clampInt(caret, 0, len(t.runes))
	return t
}

// AcceptsFocus implements input.Focusable: a disabled textbox never accepts
// focus.
func (t *TextBox) AcceptsFocus() bool {
	return t.enabled
}

// OnFocusChanged implements input.FocusHandler, tracking focus for the
// focused border color, the focus-ring overlay, and caret visibility.
func (t *TextBox) OnFocusChanged(focused bool) {
	t.focused = focused
}

// lineHeight returns face.LineHeight(), or 0 for a nil face (matching
// TextBlock's nil-face convention).
func (t *TextBox) lineHeight() float32 {
	if t.face == nil {
		return 0
	}
	return t.face.LineHeight()
}

// xOf returns the x-offset (in logical px, from the start of the text) of
// rune index i: face.Measure(runes[:i]).W. i must already be within
// [0, len(runes)] (every caller clamps via SetCaret/Select first). Returns 0
// for a nil face.
func (t *TextBox) xOf(i int) float32 {
	if t.face == nil {
		return 0
	}
	return t.face.Measure(string(t.runes[:i])).W
}

// displayText resolves what Render actually draws as the main text run and
// in what color: the placeholder (TextDisabled) whenever there is no text,
// regardless of focus; otherwise the real text, TextDisabled if disabled
// else TextPrimary.
func (t *TextBox) displayText() (s string, color render.Color) {
	if len(t.runes) == 0 {
		return t.placeholder, t.colors.TextDisabled
	}
	if !t.enabled {
		return string(t.runes), t.colors.TextDisabled
	}
	return string(t.runes), t.colors.TextPrimary
}

// caretShown reports whether the caret should be drawn this frame: never
// while unfocused; while focused, always if no timers.Queue is wired (solid
// caret), else per the current blink phase (caretVisible).
func (t *TextBox) caretShown() bool {
	if !t.focused {
		return false
	}
	if t.timerQueue == nil {
		return true
	}
	return t.caretVisible
}

// MeasureContent reports the fixed content size: textBoxDefaultWidth (an
// explicit SetWidth overrides this through core.MeasureWidget's normal
// explicit-size precedence, so available is never consulted) by
// lineHeight()+2*PaddingM.
func (t *TextBox) MeasureContent(available render.Size) render.Size {
	return render.Size{
		W: textBoxDefaultWidth,
		H: t.lineHeight() + 2*t.metrics.PaddingM,
	}
}

// ArrangeContent is the single source of truth for hscroll clamping (the
// ScrollViewer clamp-in-arrange pattern applied to one horizontal axis): it
// recomputes the padding-inset inner width from the arranged bounds and
// clamps hscroll so the caret stays visible within it.
func (t *TextBox) ArrangeContent(bounds render.Rect) {
	innerW := bounds.W - 2*t.metrics.PaddingM
	if innerW < 0 {
		innerW = 0
	}
	t.updateHScroll(innerW)
}

// updateHScroll clamps hscroll into a range that keeps the caret's display
// x-position (xOf(caret)-hscroll) within [0, innerW], while never scrolling
// past the point where the end of the text would leave a gap on the right
// (hscroll is also capped at max(0, totalTextWidth-innerW)).
func (t *TextBox) updateHScroll(innerW float32) {
	caretX := t.xOf(t.caret)

	if caretX-t.hscroll < 0 {
		t.hscroll = caretX
	}
	if caretX-t.hscroll > innerW {
		t.hscroll = caretX - innerW
	}
	if t.hscroll < 0 {
		t.hscroll = 0
	}

	maxScroll := t.xOf(len(t.runes)) - innerW
	if maxScroll < 0 {
		maxScroll = 0
	}
	if t.hscroll > maxScroll {
		t.hscroll = maxScroll
	}
}

// ClipRect implements core.ClipProvider, clipping to the textbox's own full
// bounds (the entire chrome rect, stroke included) — the same rect Render
// itself clips text/selection/caret drawing to (see Render).
func (t *TextBox) ClipRect() (render.Rect, bool) {
	return t.Bounds(), true
}

// Render paints the chrome (fill/stroke, focused/disabled state), then the
// selection highlight, main text run (or placeholder), and caret, all
// clipped to the textbox's own bounds (see ClipRect) since TextBox draws its
// content directly rather than through a child widget that RenderWidget
// would clip via ClipProvider on its own.
func (t *TextBox) Render(r render.Renderer) {
	bounds := t.Bounds()
	radius := t.metrics.ControlCornerRadius

	fill := t.colors.ControlFill
	stroke := t.colors.ControlStroke
	switch {
	case !t.enabled:
		fill = t.colors.ControlFillDisabled
		stroke = t.colors.ControlStrokeDisabled
	case t.focused:
		stroke = t.colors.FocusStroke
	}

	r.FillRoundedRect(bounds, radius, fill)
	r.StrokeRoundedRect(bounds, radius, t.metrics.StrokeWidth, stroke)

	rect, clip := t.ClipRect()
	if clip {
		r.PushClip(rect)
		defer r.PopClip()
	}

	pad := t.metrics.PaddingM
	lh := t.lineHeight()
	textY := bounds.Y + (bounds.H-lh)/2
	textX := bounds.X + pad - t.hscroll

	if start, end := t.Selection(); start != end {
		selX0 := bounds.X + pad + t.xOf(start) - t.hscroll
		selX1 := bounds.X + pad + t.xOf(end) - t.hscroll
		r.FillRect(render.Rect{X: selX0, Y: textY, W: selX1 - selX0, H: lh}, t.colors.SelectionBackground)
	}

	if s, color := t.displayText(); t.face != nil && s != "" {
		t.face.Draw(r, render.Point{X: textX, Y: textY}, s, color)
	}

	if t.caretShown() {
		cx := bounds.X + pad + t.xOf(t.caret) - t.hscroll
		r.FillRect(render.Rect{X: cx, Y: textY, W: caretWidth, H: lh}, t.colors.Accent)
	}
}

// RenderOverlay draws the focus ring while focused, per the global focus
// constraint: StrokeRoundedRect on the textbox's bounds inflated by 2,
// radius = control radius + 2, FocusStroke color and FocusStrokeWidth. This
// is IN ADDITION to Render's own focused-border color (FocusStroke at the
// normal StrokeWidth) — v0 has no WinUI-style bottom accent bar, just the
// stroke recolor plus the shared ring.
func (t *TextBox) RenderOverlay(r render.Renderer) {
	if !t.focused {
		return
	}
	drawFocusRing(r, t.Bounds(), t.metrics.ControlCornerRadius, t.colors, t.metrics)
}

// clampInt clamps v into [lo, hi].
func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
