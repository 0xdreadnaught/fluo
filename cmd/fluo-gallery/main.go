// Command fluo-gallery is the widget gallery: it grows a page per control as
// phases land. Phase 2: pure layout — panels, borders, text.
package main

import (
	"log"

	"github.com/0xdreadnaught/fluo/app"
	"github.com/0xdreadnaught/fluo/controls"
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"

	"golang.org/x/image/font/gofont/goregular"
)

func main() {
	f, err := text.Load(goregular.TTF)
	if err != nil {
		log.Fatal(err)
	}
	title := text.NewFace(f, 20)
	body := text.NewFace(f, 13)

	swatches := controls.NewWrapPanel().SetGap(8)
	for _, c := range []render.Color{
		render.RGB(0, 120, 215), render.RGB(255, 185, 0), render.RGB(16, 124, 16),
		render.RGB(232, 17, 35), render.RGB(136, 23, 152), render.RGB(0, 153, 188),
	} {
		swatches.Add(controls.NewFixed(72, 48, c))
	}

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
			SetChild(swatches),
			controls.DockLeft) // last child fills

	var lastSize render.Size
	err = app.Run(app.Config{Title: "fluo gallery", Width: 640, Height: 420}, func(c *app.Ctx) {
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
