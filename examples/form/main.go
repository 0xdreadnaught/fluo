// Command form demonstrates binding several controls to one small model:
// a TextBox, CheckBox, and Slider each two-way bound to a field of model,
// and a TextBlock one-way bound to all three, echoing the combined state
// on every change.
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

// model is the form's state: three independent Properties, constructed
// once and outliving every bind/rebind — the same "models outlive views"
// convention cmd/fluo-gallery follows.
type model struct {
	name   core.Property[string]
	agree  core.Property[bool]
	volume core.Property[float32]
}

func main() {
	font, err := text.Load(goregular.TTF)
	if err != nil {
		log.Fatal(err)
	}
	th := theme.Active()
	body := text.NewFace(font, th.Type.BodySize)

	m := &model{}
	m.volume.Set(0.5)

	nameBox := controls.NewTextBox(body).SetPlaceholder("Your name…")
	nameBox.SetWidth(220)
	agreeBox := controls.NewCheckBox(body, "I agree to the terms")
	volumeSlider := controls.NewSlider()

	summary := controls.NewTextBlock(body, "").SetColor(th.Color.TextSecondary)
	refresh := func() {
		summary.SetText(fmt.Sprintf("name=%q agree=%v volume=%.2f",
			m.name.Get(), m.agree.Get(), m.volume.Get()))
	}

	// Two-way binds own each control's OnChanged; the three OneWay binds
	// below just re-render the summary whenever ANY of the three changes.
	cancels := []func(){
		bind.Text(&m.name, nameBox),
		bind.Checked(&m.agree, agreeBox),
		bind.Value(&m.volume, volumeSlider),
		bind.OneWay(&m.name, func(string) { refresh() }),
		bind.OneWay(&m.agree, func(bool) { refresh() }),
		bind.OneWay(&m.volume, func(float32) { refresh() }),
	}
	defer func() {
		for _, cancel := range cancels {
			cancel()
		}
	}()

	root := controls.NewBorder().
		SetPadding(render.Uniform(th.Metric.PaddingL)).
		SetChild(controls.NewStackPanel(controls.Vertical).
			SetGap(th.Metric.PaddingM).
			Add(nameBox, agreeBox, volumeSlider, summary))

	var lastSize render.Size
	rootSet := false

	err = app.Run(app.Config{Title: "fluo form", Width: 360, Height: 260}, func(c *app.Ctx) {
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
