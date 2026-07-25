// Command fluo-demo is a minimal demo host exercising every fluo
// primitive: rounded rects, stroke, shadow, SDF text, and a manually
// hand-rolled hover/press-reactive button (this becomes controls.Button
// in Phase 5).
package main

import (
	"log"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/0xdreadnaught/fluo/app"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
)

func main() {
	font, err := text.Load(goregular.TTF)
	if err != nil {
		log.Fatal(err)
	}
	title, body := text.NewFace(font, 24), text.NewFace(font, 14)

	err = app.Run(app.Config{Title: "fluo demo", Width: 480, Height: 320}, func(c *app.Ctx) {
		card := render.Rect{X: 40, Y: 40, W: c.Size.W - 80, H: c.Size.H - 80}
		c.R.DrawShadow(card, 8, 16, render.RGBA(0, 0, 0, 120))
		c.R.FillRoundedRect(card, 8, render.RGB(32, 32, 36))
		c.R.StrokeRoundedRect(card, 8, 1, render.RGBA(255, 255, 255, 24))

		title.Draw(c.R, render.Point{X: card.X + 24, Y: card.Y + 20}, "fluo", render.RGB(255, 255, 255))
		body.Draw(c.R, render.Point{X: card.X + 24, Y: card.Y + 56}, "Phase 1: renderer + SDF text",
			render.RGBA(255, 255, 255, 160))

		// hover-reactive accent button (manual for now — this becomes controls.Button in Phase 5)
		btn := render.Rect{X: card.X + 24, Y: card.Bottom() - 56, W: 120, H: 32}
		bg := render.RGB(0, 120, 215)
		if btn.Contains(c.Mouse.Pos) {
			bg = render.RGB(16, 132, 226)
			if c.Mouse.Down {
				bg = render.RGB(0, 100, 180)
			}
		}
		c.R.FillRoundedRect(btn, 4, bg)

		label := "Click me"
		w := body.Measure(label).W
		body.Draw(c.R, render.Point{X: btn.X + (btn.W-w)/2, Y: btn.Y + (btn.H-body.LineHeight())/2},
			label, render.RGB(255, 255, 255))
	})
	if err != nil {
		log.Fatal(err)
	}
}
