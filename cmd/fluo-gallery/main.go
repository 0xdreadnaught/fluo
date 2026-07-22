// Command fluo-gallery is the widget gallery: it grows a page per control as
// phases land. Phase 3: interactive swatches (pointer/focus/cursor) plus a
// ScrollViewer over a taller-than-viewport content stack.
package main

import (
	"fmt"
	"log"

	"github.com/0xdreadnaught/fluo/app"
	"github.com/0xdreadnaught/fluo/controls"
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"

	"golang.org/x/image/font/gofont/goregular"
)

// accent is the selection-stroke color (Fluent blue), also used for the
// first swatch's fill.
var accent = render.RGB(0, 120, 215)

// swatch is a color block that reacts to pointer hover/press: it embeds
// core.Element for layout, implements input.PointerHandler to track
// hover/selected state, input.Focusable so it participates in tab order and
// press-to-focus, and input.CursorShaper to show a hand cursor. It exists
// here (rather than in package controls) as the consumer-side example of
// wiring a widget up to fluo's event API.
type swatch struct {
	core.Element

	w, h     float32
	color    render.Color
	hover    bool
	selected bool
}

// newSwatch returns a swatch that measures to (w, h) and fills with c.
func newSwatch(w, h float32, c render.Color) *swatch {
	return &swatch{w: w, h: h, color: c}
}

// MeasureContent returns the fixed (w, h) size regardless of the space
// available.
func (s *swatch) MeasureContent(available render.Size) render.Size {
	return render.Size{W: s.w, H: s.h}
}

// Render fills the color block, then draws a 2px accent stroke at the
// swatch's own bounds when selected, and a 2px white stroke inset 3px from
// those bounds when hovered — the inset keeps the two strokes from
// overdrawing each other, so a swatch that is both hovered and selected
// shows both rings distinctly instead of one overpainting the other.
func (s *swatch) Render(r render.Renderer) {
	b := s.Bounds()
	if s.color.A != 0 {
		r.FillRect(b, s.color)
	}
	if s.selected {
		r.StrokeRoundedRect(b, 0, 2, accent)
	}
	if s.hover {
		r.StrokeRoundedRect(b.Inset(render.Uniform(3)), 0, 2, render.RGB(255, 255, 255))
	}
}

// OnPointer implements input.PointerHandler: Enter/Leave toggle hover,
// Press toggles selected and is marked handled.
func (s *swatch) OnPointer(e *input.PointerEvent) {
	switch e.Action {
	case input.Enter:
		s.hover = true
	case input.Leave:
		s.hover = false
	case input.Press:
		s.selected = !s.selected
		e.Handled = true
	}
}

// AcceptsFocus implements input.Focusable: swatches join tab order and can
// be press-to-focused.
func (s *swatch) AcceptsFocus() bool { return true }

// Cursor implements input.CursorShaper: a hand cursor over any swatch.
func (s *swatch) Cursor() input.Cursor { return input.CursorHand }

func main() {
	f, err := text.Load(goregular.TTF)
	if err != nil {
		log.Fatal(err)
	}
	title := text.NewFace(f, 20)
	body := text.NewFace(f, 13)

	swatches := controls.NewWrapPanel().SetGap(8)
	for _, c := range []render.Color{
		accent, render.RGB(255, 185, 0), render.RGB(16, 124, 16),
		render.RGB(232, 17, 35), render.RGB(136, 23, 152), render.RGB(0, 153, 188),
	} {
		swatches.Add(newSwatch(72, 48, c))
	}

	content := controls.NewStackPanel(controls.Vertical).SetGap(8).Add(swatches)
	for i := 1; i <= 20; i++ {
		content.Add(controls.NewTextBlock(body, fmt.Sprintf("Row %02d", i)).SetColor(render.RGBA(255, 255, 255, 140)))
	}

	scroll := controls.NewScrollViewer().SetChild(content)

	nav := controls.NewStackPanel(controls.Vertical).SetGap(4).Add(
		controls.NewTextBlock(body, "Layout").SetColor(render.RGB(255, 255, 255)),
		controls.NewTextBlock(body, "Panels").SetColor(render.RGBA(255, 255, 255, 140)),
		controls.NewTextBlock(body, "Text").SetColor(render.RGBA(255, 255, 255, 140)),
	)

	root := controls.NewDockPanel().
		Add(controls.NewBorder().
			SetBackground(render.RGB(24, 24, 28)).
			SetPadding(render.Thickness{Left: 16, Top: 12, Right: 16, Bottom: 12}).
			SetChild(controls.NewTextBlock(title, "fluo gallery").SetColor(render.RGB(255, 255, 255))),
			controls.DockTop).
		Add(controls.NewBorder().
			SetBackground(render.RGB(28, 28, 33)).
			SetPadding(render.Uniform(12)).
			SetChild(nav),
			controls.DockLeft).
		Add(controls.NewBorder().
			SetPadding(render.Uniform(16)).
			SetChild(scroll),
			controls.DockLeft) // last child fills

	var lastSize render.Size
	var rootSet bool
	err = app.Run(app.Config{Title: "fluo gallery", Width: 640, Height: 420}, func(c *app.Ctx) {
		if !rootSet {
			c.Input.SetRoot(root)
			rootSet = true
		}
		if c.Size != lastSize || root.NeedsLayout() {
			lastSize = c.Size
			core.MeasureWidget(root, c.Size)
			core.ArrangeWidget(root, render.Rect{X: 0, Y: 0, W: c.Size.W, H: c.Size.H})
		}
		core.RenderWidget(root, c.R)
	})
	if err != nil {
		log.Fatal(err)
	}
}
