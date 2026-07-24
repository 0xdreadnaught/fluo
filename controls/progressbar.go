package controls

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/theme"
)

// Fixed geometry for ProgressBar: desired size {160, 8}.
const (
	progressDesiredWidth  float32 = 160
	progressDesiredHeight float32 = 8
)

// progressChunkGap is the gap, in px, between adjacent classic "chunked"
// fill blocks (see Render) — the Windows-2000 progress bar's signature
// marching-blocks look, as opposed to a single solid bar.
const progressChunkGap float32 = 2

// ProgressBar is a non-interactive, token-styled horizontal progress
// indicator over [0, 1]. Unlike every other control in this package, it
// implements NONE of input.Focusable/PointerHandler/KeyHandler/
// FocusHandler — a bare core.Element embed already reports no such methods,
// so ProgressBar is simply never a candidate for router focus and never
// receives pointer/key events in the first place; there is no "ignore
// input" branch to write because there is no input path to ignore it on.
//
// Visuals (normative): the track is a classic sunken well (drawSunken,
// WindowWell) with a 2px bevel; the Value-proportion fill is drawn inside
// that inset as discrete Highlight "chunks" (the Windows-2000 marching-
// blocks look) rather than a single solid bar, with no thumb.
type ProgressBar struct {
	core.Element

	value float32

	colors  theme.ColorTokens
	metrics theme.MetricTokens
}

// NewProgressBar returns a ProgressBar at Value 0.
func NewProgressBar() *ProgressBar {
	th := theme.Active()
	return &ProgressBar{
		colors:  th.Color,
		metrics: th.Metric,
	}
}

// Value returns the current progress value, in [0, 1].
func (p *ProgressBar) Value() float32 { return p.value }

// SetValue sets the progress value, clamped into [0, 1].
func (p *ProgressBar) SetValue(v float32) *ProgressBar {
	p.value = clampF(v, 0, 1)
	return p
}

// MeasureContent always returns the fixed {160, 8} desired size: ProgressBar
// has no content to size around (an explicit SetWidth/SetHeight overrides
// this through core.MeasureWidget's normal precedence, matching Slider).
func (p *ProgressBar) MeasureContent(available render.Size) render.Size {
	return render.Size{W: progressDesiredWidth, H: progressDesiredHeight}
}

// ArrangeContent is a no-op: ProgressBar has no children to position.
func (p *ProgressBar) ArrangeContent(bounds render.Rect) {}

// Children returns nil: ProgressBar is a leaf widget.
func (p *ProgressBar) Children() []core.Widget { return nil }

// Render paints the classic sunken well (drawSunken, WindowWell) and, inside
// it (inset by the well's own 2px bevel), the Value-proportion fill as
// discrete "chunked" Highlight blocks — the Windows-2000 progress bar's
// marching-blocks look, rather than one solid bar. Each block is exactly as
// wide as the inset well is tall (a square chunk) with a progressChunkGap
// gap to the next; only whole blocks that fit entirely within the filled
// region are drawn (a naturally simple stopping rule: iterate x in
// blockWidth+gap steps while the next block still fits inside
// innerLeft+filledWidth), so the fill grows one whole chunk at a time as
// Value increases rather than ever drawing a partial block.
func (p *ProgressBar) Render(r render.Renderer) {
	c := p.colors
	bounds := p.Bounds()

	drawSunken(r, bounds, c.WindowWell, c)

	inner := bounds.Inset(render.Uniform(2))
	if inner.W <= 0 || inner.H <= 0 {
		return
	}

	blockWidth := inner.H
	filledRight := inner.X + inner.W*p.value
	step := blockWidth + progressChunkGap

	for x := inner.X; x+blockWidth <= filledRight; x += step {
		r.FillRect(render.Rect{X: x, Y: inner.Y, W: blockWidth, H: inner.H}, c.Highlight)
	}
}
