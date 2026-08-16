package controls

import (
	"math"
	"strconv"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
	"github.com/0xdreadnaught/fluo/theme"
)

const (
	numberBoxDesiredWidth float32 = 120
	numberBoxSpinnerWidth float32 = 16
	numberBoxArrowRows            = 4
	numberBoxCaretWidth   float32 = 1.5
)

// NumberBox is a focusable, token-styled numeric input with up/down spinner
// buttons. It displays a float64 value formatted as text and supports
// direct keyboard entry, arrow-key stepping, and pointer-driven spinner
// buttons.
//
// Value is clamped into [Min, Max] on every mutation; the defaults are
// [-MaxFloat64, MaxFloat64] (effectively unconstrained). Step (default 1)
// controls the increment for spinner buttons and Up/Down arrow keys;
// Shift+Up/Down steps by 10x.
//
// OnChanged follows fluo's uniform setter convention: programmatic setters
// (SetValue, SetRange) are silent; OnChanged fires only on user-driven
// changes (spinner clicks, arrow keys, committed text edits).
//
// When focused, the control enters edit mode: the text area shows an
// editable buffer with a caret. Typing inserts digits, minus, or decimal
// point at the caret. Enter commits the edit; Escape reverts. Focus loss
// commits. Up/Down step the value and re-sync the buffer.
type NumberBox struct {
	core.Element

	face *text.Face

	value    float64
	min      float64
	max      float64
	step     float64
	decimals int

	enabled bool
	focused bool

	hoverUp   bool
	hoverDown bool
	pressUp   bool
	pressDown bool

	editRunes []rune
	editCaret int
	hscroll   float32

	onChanged func(float64)

	colors  theme.ColorTokens
	metrics theme.MetricTokens
}

func NewNumberBox(face *text.Face) *NumberBox {
	th := theme.Active()
	return &NumberBox{
		face:     face,
		min:      -math.MaxFloat64,
		max:      math.MaxFloat64,
		step:     1,
		decimals: -1,
		enabled:  true,
		colors:   th.Color,
		metrics:  th.Metric,
	}
}

func (n *NumberBox) Value() float64 { return n.value }
func (n *NumberBox) Min() float64   { return n.min }
func (n *NumberBox) Max() float64   { return n.max }
func (n *NumberBox) Step() float64  { return n.step }

func (n *NumberBox) SetValue(v float64) *NumberBox {
	n.setValueSilent(v)
	return n
}

func (n *NumberBox) SetRange(min, max float64) *NumberBox {
	n.min = min
	n.max = max
	n.setValueSilent(n.value)
	return n
}

func (n *NumberBox) SetStep(s float64) *NumberBox {
	n.step = s
	return n
}

func (n *NumberBox) SetDecimals(d int) *NumberBox {
	n.decimals = d
	return n
}

func (n *NumberBox) SetEnabled(v bool) *NumberBox {
	n.enabled = v
	return n
}

func (n *NumberBox) OnChanged(fn func(float64)) *NumberBox {
	n.onChanged = fn
	return n
}

func clamp64(v, lo, hi float64) float64 {
	if hi < lo {
		hi = lo
	}
	if math.IsNaN(v) {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (n *NumberBox) setValueSilent(v float64) {
	n.value = clamp64(v, n.min, n.max)
}

func (n *NumberBox) setValue(v float64) {
	before := n.value
	n.setValueSilent(v)
	if n.value != before && n.onChanged != nil {
		n.onChanged(n.value)
	}
}

func (n *NumberBox) formatValue() string {
	if n.decimals < 0 {
		return strconv.FormatFloat(n.value, 'f', -1, 64)
	}
	return strconv.FormatFloat(n.value, 'f', n.decimals, 64)
}

func (n *NumberBox) lineHeight() float32 {
	if n.face == nil {
		return 0
	}
	return n.face.LineHeight()
}

func (n *NumberBox) syncEditBuffer() {
	s := n.formatValue()
	n.editRunes = []rune(s)
	n.editCaret = len(n.editRunes)
	n.hscroll = 0
}

func (n *NumberBox) commitEdit() {
	s := string(n.editRunes)
	v, err := strconv.ParseFloat(s, 64)
	if err == nil {
		n.setValue(v)
	}
	n.syncEditBuffer()
}

func (n *NumberBox) fieldSplit(bounds render.Rect) (textWell, upBtn, downBtn render.Rect) {
	sw := numberBoxSpinnerWidth
	if sw > bounds.W {
		sw = bounds.W
	}
	textWell = render.Rect{X: bounds.X, Y: bounds.Y, W: bounds.W - sw, H: bounds.H}
	halfH := bounds.H / 2
	upBtn = render.Rect{X: bounds.X + bounds.W - sw, Y: bounds.Y, W: sw, H: halfH}
	downBtn = render.Rect{X: bounds.X + bounds.W - sw, Y: bounds.Y + halfH, W: sw, H: bounds.H - halfH}
	return
}

func (n *NumberBox) textContentWidth() float32 {
	bounds := n.Bounds()
	well, _, _ := n.fieldSplit(bounds)
	pad := n.metrics.PaddingM
	w := well.W - 2*pad - 4
	if w < 0 {
		w = 0
	}
	return w
}

func (n *NumberBox) ensureCaretVisible() {
	if n.face == nil {
		return
	}
	cw := n.textContentWidth()
	if cw <= 0 {
		return
	}
	caretX := n.face.Measure(string(n.editRunes[:n.editCaret])).W
	if caretX-n.hscroll > cw {
		n.hscroll = caretX - cw
	}
	if caretX-n.hscroll < 0 {
		n.hscroll = caretX
	}
}

func (n *NumberBox) MeasureContent(available render.Size) render.Size {
	lh := n.lineHeight()
	pad := n.metrics.PaddingM
	h := lh + 2*pad
	if h < numberBoxSpinnerWidth {
		h = numberBoxSpinnerWidth
	}
	return render.Size{W: numberBoxDesiredWidth, H: h}
}

func (n *NumberBox) ArrangeContent(bounds render.Rect) {}

func (n *NumberBox) Children() []core.Widget { return nil }

func drawSpinnerArrow(r render.Renderer, bounds render.Rect, up bool, c render.Color) {
	totalH := float32(numberBoxArrowRows)
	cx := bounds.X + bounds.W/2
	cy := bounds.Y + (bounds.H-totalH)/2
	for i := 0; i < numberBoxArrowRows; i++ {
		var w float32
		if up {
			w = float32(1 + 2*i)
		} else {
			w = float32(1 + 2*(numberBoxArrowRows-1-i))
		}
		r.FillRect(render.Rect{
			X: cx - w/2,
			Y: cy + float32(i),
			W: w,
			H: 1,
		}, c)
	}
}

func (n *NumberBox) Render(r render.Renderer) {
	c := n.colors
	bounds := n.Bounds()
	well, upBtn, downBtn := n.fieldSplit(bounds)

	if n.metrics.BevelWidth == 0 {
		n.renderFlat(r, c, bounds, well, upBtn, downBtn)
		return
	}

	fill := c.WindowWell
	if !n.enabled {
		fill = c.ButtonFace
	}
	drawSunken(r, well, fill, c)

	upFace := c.ButtonFace
	if n.hoverUp && n.enabled {
		upFace = c.ButtonLight
	}
	if n.pressUp {
		drawSunken(r, upBtn, c.ButtonFace, c)
	} else {
		drawRaised(r, upBtn, upFace, c)
	}

	downFace := c.ButtonFace
	if n.hoverDown && n.enabled {
		downFace = c.ButtonLight
	}
	if n.pressDown {
		drawSunken(r, downBtn, c.ButtonFace, c)
	} else {
		drawRaised(r, downBtn, downFace, c)
	}

	arrowColor := c.WindowText
	if !n.enabled {
		arrowColor = c.GrayText
	}
	drawSpinnerArrow(r, upBtn, true, arrowColor)
	drawSpinnerArrow(r, downBtn, false, arrowColor)

	n.renderText(r, well, c)
}

func (n *NumberBox) renderFlat(r render.Renderer, c theme.ColorTokens, bounds, well, upBtn, downBtn render.Rect) {
	radius := n.metrics.ControlCornerRadius

	fill := c.WindowWell
	if !n.enabled {
		fill = c.ButtonFace
	}
	r.FillRoundedRect(bounds, radius, fill)
	r.StrokeRoundedRect(bounds, radius, n.metrics.StrokeWidth, c.ButtonShadow)

	r.FillRect(render.Rect{X: upBtn.X, Y: upBtn.Y, W: 1, H: bounds.H}, c.ButtonShadow)

	upFill := c.ButtonFace
	if n.pressUp {
		upFill = c.ButtonShadow
	} else if n.hoverUp && n.enabled {
		upFill = c.ButtonLight
	}
	r.FillRect(upBtn.Inset(render.Uniform(1)), upFill)

	downFill := c.ButtonFace
	if n.pressDown {
		downFill = c.ButtonShadow
	} else if n.hoverDown && n.enabled {
		downFill = c.ButtonLight
	}
	r.FillRect(downBtn.Inset(render.Uniform(1)), downFill)

	r.FillRect(render.Rect{X: upBtn.X + 1, Y: upBtn.Y + upBtn.H - 0.5, W: upBtn.W - 2, H: 0.5}, c.ButtonShadow)

	arrowColor := c.WindowText
	if !n.enabled {
		arrowColor = c.GrayText
	}
	drawSpinnerArrow(r, upBtn, true, arrowColor)
	drawSpinnerArrow(r, downBtn, false, arrowColor)

	n.renderText(r, well, c)
}

func (n *NumberBox) renderText(r render.Renderer, well render.Rect, c theme.ColorTokens) {
	if n.face == nil {
		return
	}

	pad := n.metrics.PaddingM
	lh := n.lineHeight()
	textY := well.Y + (well.H-lh)/2

	clipRect := render.Rect{X: well.X + pad, Y: well.Y, W: well.W - 2*pad, H: well.H}
	if clipRect.W < 0 {
		clipRect.W = 0
	}
	r.PushClip(clipRect)
	defer r.PopClip()

	textColor := c.WindowText
	if !n.enabled {
		textColor = c.GrayText
	}

	var displayText string
	if n.focused {
		displayText = string(n.editRunes)
	} else {
		displayText = n.formatValue()
	}

	textX := well.X + pad + 2 - n.hscroll
	n.face.Draw(r, render.Point{X: textX, Y: textY}, displayText, textColor)

	if n.focused && n.enabled {
		caretStr := string(n.editRunes[:n.editCaret])
		caretX := well.X + pad + 2 - n.hscroll + n.face.Measure(caretStr).W
		r.FillRect(render.Rect{X: caretX, Y: textY, W: numberBoxCaretWidth, H: lh}, c.WindowText)
	}
}

func (n *NumberBox) RenderOverlay(r render.Renderer) {
	if !n.focused {
		return
	}
	bounds := n.Bounds()
	if n.metrics.BevelWidth == 0 {
		r.StrokeRoundedRect(bounds.Inset(render.Uniform(-n.metrics.FocusStrokeWidth)),
			n.metrics.ControlCornerRadius, n.metrics.FocusStrokeWidth, n.colors.Highlight)
		return
	}
	drawFocusRing(r, bounds, n.colors)
}

func (n *NumberBox) AcceptsFocus() bool {
	return n.enabled
}

func (n *NumberBox) OnFocusChanged(focused bool) {
	wasFocused := n.focused
	n.focused = focused
	if focused && !wasFocused {
		n.syncEditBuffer()
	} else if !focused && wasFocused {
		n.commitEdit()
	}
}

func (n *NumberBox) OnPointer(e *input.PointerEvent) {
	if !n.enabled {
		if e.Router != nil && e.Router.Captured() == n {
			e.Router.Release()
		}
		return
	}

	bounds := n.Bounds()
	_, upBtn, downBtn := n.fieldSplit(bounds)

	switch e.Action {
	case input.Enter:
		n.updateSpinnerHover(e.Pos, upBtn, downBtn)
	case input.Leave:
		n.hoverUp = false
		n.hoverDown = false
	case input.Move:
		if e.Router != nil && e.Router.Captured() == n {
			n.pressUp = n.pressUp && upBtn.Contains(e.Pos)
			n.pressDown = n.pressDown && downBtn.Contains(e.Pos)
		} else {
			n.updateSpinnerHover(e.Pos, upBtn, downBtn)
		}
	case input.Press:
		if upBtn.Contains(e.Pos) {
			n.pressUp = true
			n.setValue(n.value + n.step)
			n.syncEditBuffer()
			if e.Router != nil {
				e.Router.Capture(n)
			}
			e.Handled = true
		} else if downBtn.Contains(e.Pos) {
			n.pressDown = true
			n.setValue(n.value - n.step)
			n.syncEditBuffer()
			if e.Router != nil {
				e.Router.Capture(n)
			}
			e.Handled = true
		}
	case input.Release:
		n.pressUp = false
		n.pressDown = false
		if e.Router != nil && e.Router.Captured() == n {
			e.Router.Release()
			e.Handled = true
		}
	}
}

func (n *NumberBox) updateSpinnerHover(pos render.Point, upBtn, downBtn render.Rect) {
	n.hoverUp = upBtn.Contains(pos)
	n.hoverDown = downBtn.Contains(pos)
}

func (n *NumberBox) OnKey(e *input.KeyEvent) {
	if !n.enabled || e.Action != input.Press {
		return
	}

	shift := e.Mods&input.ModShift != 0

	switch e.Key {
	case input.KeyUp:
		step := n.step
		if shift {
			step *= 10
		}
		n.setValue(n.value + step)
		n.syncEditBuffer()
		e.Handled = true
		return
	case input.KeyDown:
		step := n.step
		if shift {
			step *= 10
		}
		n.setValue(n.value - step)
		n.syncEditBuffer()
		e.Handled = true
		return
	case input.KeyEnter:
		n.commitEdit()
		e.Handled = true
		return
	case input.KeyEscape:
		n.syncEditBuffer()
		e.Handled = true
		return
	case input.KeyBackspace:
		if n.editCaret > 0 {
			n.editRunes = append(n.editRunes[:n.editCaret-1], n.editRunes[n.editCaret:]...)
			n.editCaret--
			n.ensureCaretVisible()
		}
		e.Handled = true
		return
	case input.KeyDelete:
		if n.editCaret < len(n.editRunes) {
			n.editRunes = append(n.editRunes[:n.editCaret], n.editRunes[n.editCaret+1:]...)
			n.ensureCaretVisible()
		}
		e.Handled = true
		return
	case input.KeyLeft:
		if n.editCaret > 0 {
			n.editCaret--
			n.ensureCaretVisible()
		}
		e.Handled = true
		return
	case input.KeyRight:
		if n.editCaret < len(n.editRunes) {
			n.editCaret++
			n.ensureCaretVisible()
		}
		e.Handled = true
		return
	case input.KeyHome:
		n.editCaret = 0
		n.ensureCaretVisible()
		e.Handled = true
		return
	case input.KeyEnd:
		n.editCaret = len(n.editRunes)
		n.ensureCaretVisible()
		e.Handled = true
		return
	}

	if e.Rune != 0 && e.Mods&input.ModCtrl == 0 {
		if isNumericRune(e.Rune) {
			runes := make([]rune, 0, len(n.editRunes)+1)
			runes = append(runes, n.editRunes[:n.editCaret]...)
			runes = append(runes, e.Rune)
			runes = append(runes, n.editRunes[n.editCaret:]...)
			n.editRunes = runes
			n.editCaret++
			n.ensureCaretVisible()
			e.Handled = true
		}
	}
}

func isNumericRune(r rune) bool {
	return (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '+' || r == 'e' || r == 'E'
}
