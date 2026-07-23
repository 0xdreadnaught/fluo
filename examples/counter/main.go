// Command counter is fluo's smallest example: a Button increments a
// core.Property[int], and a TextBlock shows the current count via
// bind.OneWay. It demonstrates the minimal app.Run + widget-tree +
// binding wiring a consumer needs for a live-updating label.
package main

import (
	"fmt"
	"log"

	"github.com/0xdreadnaught/fluo/app"
	"github.com/0xdreadnaught/fluo/bind"
	"github.com/0xdreadnaught/fluo/controls"
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
	"github.com/0xdreadnaught/fluo/theme"

	"golang.org/x/image/font/gofont/goregular"
)

func main() {
	font, err := text.Load(goregular.TTF)
	if err != nil {
		log.Fatal(err)
	}
	th := theme.Active()
	body := text.NewFace(font, th.Type.BodySize)

	// count is the model: a plain reactive int, owned by main and never
	// recreated. label mirrors it one-way; every OTHER change to count
	// (there are none here but a real app might have several sources)
	// would push through the same binding.
	count := new(core.Property[int])
	label := controls.NewTextBlock(body, "").SetColor(th.Color.TextPrimary)
	cancel := bind.OneWay(count, func(n int) {
		label.SetText(fmt.Sprintf("Clicked %d times", n))
	})
	defer cancel()

	button := controls.NewButton(body, "Click me").OnClick(func() {
		count.Set(count.Get() + 1)
	})

	root := controls.NewBorder().
		SetPadding(render.Uniform(th.Metric.PaddingL)).
		SetChild(controls.NewStackPanel(controls.Vertical).
			SetGap(th.Metric.PaddingM).
			Add(label, button))

	var lastSize render.Size
	rootSet := false

	err = app.Run(app.Config{Title: "fluo counter", Width: 320, Height: 160}, func(c *app.Ctx) {
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
