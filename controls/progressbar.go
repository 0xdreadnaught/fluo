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

// ProgressBar is a non-interactive, token-styled progress indicator over
// [0, 1]. Unlike every other control in this package, it implements NONE of
// input.Focusable/PointerHandler/KeyHandler/FocusHandler — a bare
// core.Element embed already reports no such methods, so ProgressBar is
// simply never a candidate for router focus and never receives pointer/key
// events in the first place; there is no "ignore input" branch to write
// because there is no input path to ignore it on.
//
// Orientation defaults to Horizontal (see SetOrientation): the track runs
// left-to-right and the fill grows left-to-right. Vertical runs
// top-to-bottom and the fill grows bottom-to-top (see SetOrientation).
//
// Visuals (normative): the track is a classic sunken well (drawSunken,
// WindowWell) with a 2px bevel. The Value-proportion fill, inside that
// inset, is either the default "chunked" look (solid == false — discrete
// Highlight blocks, the Windows-2000 marching-blocks look) or a single
// solid Highlight fill spanning the value proportion with no gaps
// (solid == true, see SetSolid). Never has a thumb.
type ProgressBar struct {
	core.Element

	value       float32
	orientation Orientation
	solid       bool

	colors  theme.ColorTokens
	metrics theme.MetricTokens
}

// NewProgressBar returns a Horizontal, chunked ProgressBar at Value 0.
func NewProgressBar() *ProgressBar {
	th := theme.Active()
	return &ProgressBar{
		orientation: Horizontal,
		colors:      th.Color,
		metrics:     th.Metric,
	}
}

// Value returns the current progress value, in [0, 1].
func (p *ProgressBar) Value() float32 { return p.value }

// SetValue sets the progress value, clamped into [0, 1].
func (p *ProgressBar) SetValue(v float32) *ProgressBar {
	p.value = clampF(v, 0, 1)
	return p
}

// SetOrientation sets the progress bar's orientation — Horizontal (the
// default) fills left-to-right; Vertical fills bottom-to-top (see the type
// doc comment). Takes effect on the next Measure/Arrange/Render pass.
func (p *ProgressBar) SetOrientation(o Orientation) *ProgressBar {
	p.orientation = o
	return p
}

// SetSolid sets whether the fill is a single solid Highlight bar (true) or
// the default discrete "chunked" blocks (false, see the type doc comment).
func (p *ProgressBar) SetSolid(v bool) *ProgressBar {
	p.solid = v
	return p
}

// MeasureContent returns the fixed desired size: {160, 8} for Horizontal
// (the default), swapped to {8, 160} (tall-narrow) for Vertical.
// ProgressBar has no content to size around (an explicit SetWidth/
// SetHeight overrides this through core.MeasureWidget's normal precedence,
// matching Slider).
func (p *ProgressBar) MeasureContent(available render.Size) render.Size {
	if p.orientation == Vertical {
		return render.Size{W: progressDesiredHeight, H: progressDesiredWidth}
	}
	return render.Size{W: progressDesiredWidth, H: progressDesiredHeight}
}

// ArrangeContent is a no-op: ProgressBar has no children to position.
func (p *ProgressBar) ArrangeContent(bounds render.Rect) {}

// Children returns nil: ProgressBar is a leaf widget.
func (p *ProgressBar) Children() []core.Widget { return nil }

// Render paints the classic sunken well (drawSunken, WindowWell) and, inside
// it (inset by the well's own 2px bevel), the Value-proportion fill —
// dispatched to renderChunked or renderSolid per s.solid, each of which
// honors s.orientation. Horizontal grows the fill left-to-right; Vertical
// grows it bottom-to-top (see the type doc comment).
func (p *ProgressBar) Render(r render.Renderer) {
	c := p.colors
	bounds := p.Bounds()

	drawSunken(r, bounds, c.WindowWell, c)

	inner := bounds.Inset(render.Uniform(2))
	if inner.W <= 0 || inner.H <= 0 {
		return
	}

	if p.solid {
		p.renderSolid(r, inner, c)
		return
	}
	p.renderChunked(r, inner, c)
}

// renderChunked paints the default "chunked" Highlight fill — the
// Windows-2000 progress bar's marching-blocks look, rather than one solid
// bar. Each block is exactly as wide (Horizontal) or tall (Vertical) as the
// inset well is deep on the CROSS axis (a square chunk) with a
// progressChunkGap gap to the next; only whole blocks that fit entirely
// within the filled region are drawn (a naturally simple stopping rule:
// iterate in blockSize+gap steps while the next block still fits inside the
// filled span), so the fill grows one whole chunk at a time as Value
// increases rather than ever drawing a partial block.
//
// Horizontal (unchanged from the pre-orientation implementation): blocks
// grow left-to-right from inner's left edge. Vertical: blocks grow
// bottom-to-top from inner's bottom edge, mirroring the horizontal geometry
// onto the Y axis.
func (p *ProgressBar) renderChunked(r render.Renderer, inner render.Rect, c theme.ColorTokens) {
	if p.orientation == Vertical {
		blockHeight := inner.W
		filledHeight := inner.H * p.value
		step := blockHeight + progressChunkGap
		for h := float32(0); h+blockHeight <= filledHeight; h += step {
			y := inner.Y + inner.H - h - blockHeight
			r.FillRect(render.Rect{X: inner.X, Y: y, W: inner.W, H: blockHeight}, c.Highlight)
		}
		return
	}

	blockWidth := inner.H
	filledRight := inner.X + inner.W*p.value
	step := blockWidth + progressChunkGap

	for x := inner.X; x+blockWidth <= filledRight; x += step {
		r.FillRect(render.Rect{X: x, Y: inner.Y, W: blockWidth, H: inner.H}, c.Highlight)
	}
}

// renderSolid paints the solid-fill variant (SetSolid(true)): a single
// Highlight rect spanning the Value proportion of the inset well along the
// main axis, with no chunk gaps. Horizontal fills left-to-right from
// inner's left edge; Vertical fills bottom-to-top from inner's bottom edge
// (see the type doc comment).
func (p *ProgressBar) renderSolid(r render.Renderer, inner render.Rect, c theme.ColorTokens) {
	if p.orientation == Vertical {
		filledH := inner.H * p.value
		if filledH <= 0 {
			return
		}
		r.FillRect(render.Rect{X: inner.X, Y: inner.Y + inner.H - filledH, W: inner.W, H: filledH}, c.Highlight)
		return
	}

	filledW := inner.W * p.value
	if filledW <= 0 {
		return
	}
	r.FillRect(render.Rect{X: inner.X, Y: inner.Y, W: filledW, H: inner.H}, c.Highlight)
}
