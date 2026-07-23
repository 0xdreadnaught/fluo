package controls

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/theme"
)

// Fixed geometry for ProgressBar: a 4px-tall rounded track (radius =
// height/2, matching Slider's track), desired size {160, 8}.
const (
	progressTrackHeight   float32 = 4
	progressDesiredWidth  float32 = 160
	progressDesiredHeight float32 = 8
)

// ProgressBar is a non-interactive, token-styled horizontal progress
// indicator over [0, 1]. Unlike every other control in this package, it
// implements NONE of input.Focusable/PointerHandler/KeyHandler/
// FocusHandler — a bare core.Element embed already reports no such methods,
// so ProgressBar is simply never a candidate for router focus and never
// receives pointer/key events in the first place; there is no "ignore
// input" branch to write because there is no input path to ignore it on.
//
// Visuals (normative): track is a 4px-tall rounded rect, ControlFill only
// (no stroke, unlike Slider's track — a progress bar has no thumb to
// visually anchor a hairline border against); the Accent-filled portion
// spans from the track's left edge over Value proportion of the full
// width, with no thumb.
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

// Render paints the rounded track (ControlFill, no stroke) and the
// Accent-filled portion spanning Value proportion of the full width.
func (p *ProgressBar) Render(r render.Renderer) {
	bounds := p.Bounds()
	radius := progressTrackHeight / 2
	trackY := bounds.Y + (bounds.H-progressTrackHeight)/2

	track := render.Rect{X: bounds.X, Y: trackY, W: bounds.W, H: progressTrackHeight}
	r.FillRoundedRect(track, radius, p.colors.ControlFill)

	filled := render.Rect{X: bounds.X, Y: trackY, W: bounds.W * p.value, H: progressTrackHeight}
	r.FillRoundedRect(filled, radius, p.colors.Accent)
}
