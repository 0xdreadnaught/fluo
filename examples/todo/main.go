// Command todo demonstrates list binding: a TextBox + "Add" Button append
// to a bind.List[string], a ListView renders it, and a TextBlock tracks
// the item count — all driven off the same List, with no manual widget
// rebuilding.
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

	items := bind.NewList[string]()

	entry := controls.NewTextBox(body).SetPlaceholder("New item…")
	entry.SetWidth(200)

	// ListView renders items directly — no bind.Items rebuild step needed,
	// since ListView virtualizes and reads a bind.List (via ListItems)
	// natively.
	list := controls.NewListView(body, items)
	list.SetWidth(240)
	list.SetHeight(160)

	count := controls.NewTextBlock(body, "0 items").SetColor(th.Color.TextSecondary)
	updateCount := func() { count.SetText(fmt.Sprintf("%d items", items.Len())) }
	cancelCount := items.OnChanged(updateCount)
	updateCount()

	addItem := func() {
		if s := entry.Text(); s != "" {
			items.Add(s)
			entry.SetText("")
		}
	}
	addButton := controls.NewButton(body, "Add").OnClick(addItem)

	defer func() {
		cancelCount()
		list.Dispose()
	}()

	entryRow := controls.NewStackPanel(controls.Horizontal).SetGap(th.Metric.PaddingM).
		Add(entry, addButton)

	root := controls.NewBorder().
		SetPadding(render.Uniform(th.Metric.PaddingL)).
		SetChild(controls.NewStackPanel(controls.Vertical).
			SetGap(th.Metric.PaddingM).
			Add(entryRow, list, count))

	var lastSize render.Size
	rootSet := false

	err = app.Run(app.Config{Title: "fluo todo", Width: 360, Height: 320}, func(c *app.Ctx) {
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
