package controls

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
	"github.com/0xdreadnaught/fluo/theme"
)

// TextBlock is a single-line text leaf widget.
type TextBlock struct {
	core.Element

	face  *text.Face
	text  core.Property[string]
	color render.Color
}

// NewTextBlock returns a TextBlock that draws s with face. face may be nil,
// in which case the widget measures to zero and renders nothing. The default
// color is styled from theme.Active() at construction; rebuild to re-theme.
// SetColor overrides the theme default.
func NewTextBlock(face *text.Face, s string) *TextBlock {
	tb := &TextBlock{face: face, color: theme.Active().Color.TextPrimary}
	tb.text.OnChange(func(_, _ string) { tb.InvalidateMeasure() })
	tb.text.Set(s)
	return tb
}

// SetText sets the displayed text. Changing it invalidates measure; setting
// the same value is a no-op (Property only notifies on real changes).
func (t *TextBlock) SetText(s string) *TextBlock {
	t.text.Set(s)
	return t
}

// Text returns the currently displayed text.
func (t *TextBlock) Text() string {
	return t.text.Get()
}

// SetColor sets the text color. Purely visual: no invalidation needed.
func (t *TextBlock) SetColor(c render.Color) *TextBlock {
	t.color = c
	return t
}

// Color returns the text's current color, whether the theme default set at
// construction or a later SetColor override.
func (t *TextBlock) Color() render.Color {
	return t.color
}

// MeasureContent returns the size the text occupies when drawn with face; a
// nil face measures to zero.
func (t *TextBlock) MeasureContent(available render.Size) render.Size {
	if t.face == nil {
		return render.Size{}
	}
	return t.face.Measure(t.text.Get())
}

// Render draws the text at the top-left of the widget's bounds; skipped
// when there is no face, no text, or a fully transparent color.
func (t *TextBlock) Render(r render.Renderer) {
	s := t.text.Get()
	if t.face == nil || s == "" || t.color.A == 0 {
		return
	}
	b := t.Bounds()
	t.face.Draw(r, render.Point{X: b.X, Y: b.Y}, s, t.color)
}
