package gl_test

import (
	"testing"

	"github.com/go-gl/gl/v3.3-core/gl"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/0xdreadnaught/fluo/controls"
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
	glr "github.com/0xdreadnaught/fluo/render/gl"
	"github.com/0xdreadnaught/fluo/render/gl/gltest"
	"github.com/0xdreadnaught/fluo/text"
	"github.com/0xdreadnaught/fluo/theme"
)

func testFrame(t *testing.T, name string, w, h int, draw func(r *glr.Renderer)) {
	gltest.Run(t, w, h, func(fb *gltest.Framebuffer) {
		r, err := glr.New()
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		gl.ClearColor(0.12, 0.12, 0.14, 1)
		gl.Clear(gl.COLOR_BUFFER_BIT)
		r.Begin(fb.W, fb.H, 1)
		draw(r)
		r.End()
		gltest.CheckGolden(t, name, fb.Image())
	})
}

func TestFillRect(t *testing.T) {
	testFrame(t, "fill_rect", 128, 96, func(r *glr.Renderer) {
		r.FillRect(render.Rect{X: 8, Y: 8, W: 60, H: 40}, render.RGB(0, 120, 215))
		r.FillRect(render.Rect{X: 40, Y: 30, W: 60, H: 40}, render.RGBA(255, 255, 255, 128)) // blend check
	})
}

func TestClip(t *testing.T) {
	testFrame(t, "clip", 128, 96, func(r *glr.Renderer) {
		r.PushClip(render.Rect{X: 20, Y: 20, W: 50, H: 30})
		r.FillRect(render.Rect{X: 0, Y: 0, W: 128, H: 96}, render.RGB(0, 120, 215)) // fills only the clip
		r.PushClip(render.Rect{X: 0, Y: 0, W: 40, H: 96})                           // nested: intersects
		r.FillRect(render.Rect{X: 0, Y: 0, W: 128, H: 96}, render.RGB(255, 185, 0))
		r.PopClip()
		r.PopClip()
		r.FillRect(render.Rect{X: 100, Y: 70, W: 40, H: 40}, render.RGB(16, 124, 16)) // unclipped again
	})
}

func TestRoundedFill(t *testing.T) {
	testFrame(t, "rounded_fill", 128, 96, func(r *glr.Renderer) {
		r.FillRoundedRect(render.Rect{X: 10, Y: 10, W: 80, H: 50}, 8, render.RGB(0, 120, 215))
		r.FillRoundedRect(render.Rect{X: 60, Y: 40, W: 50, H: 50}, 25, render.RGB(255, 185, 0)) // circle
	})
}

func TestRoundedStroke(t *testing.T) {
	testFrame(t, "rounded_stroke", 128, 96, func(r *glr.Renderer) {
		r.StrokeRoundedRect(render.Rect{X: 10, Y: 10, W: 100, H: 70}, 8, 2, render.RGB(255, 255, 255))
	})
}

func TestShadow(t *testing.T) {
	testFrame(t, "shadow", 128, 96, func(r *glr.Renderer) {
		card := render.Rect{X: 24, Y: 20, W: 80, H: 56}
		r.DrawShadow(card, 8, 12, render.RGBA(0, 0, 0, 140))
		r.FillRoundedRect(card, 8, render.RGB(243, 243, 243))
	})
}

func TestTexture(t *testing.T) {
	testFrame(t, "texture", 128, 96, func(r *glr.Renderer) {
		px := make([]byte, 8*8*4)
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				v := byte(255)
				if (x+y)%2 == 0 {
					v = 40
				}
				i := (y*8 + x) * 4
				px[i], px[i+1], px[i+2], px[i+3] = v, v, v, 255
			}
		}
		id := r.CreateTexture(8, 8, px)
		r.DrawQuad(render.Rect{X: 8, Y: 8, W: 48, H: 48}, render.Rect{X: 0, Y: 0, W: 1, H: 1}, id, render.RGB(255, 255, 255))
		r.DrawQuad(render.Rect{X: 64, Y: 8, W: 48, H: 48}, render.Rect{X: 0, Y: 0, W: 0.5, H: 0.5}, id, render.RGB(0, 120, 215)) // sub-rect + tint
	})
}

func TestText(t *testing.T) {
	testFrame(t, "text", 256, 96, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		text.NewFace(f, 14).Draw(r, render.Point{X: 8, Y: 8}, "Hello, fluo!", render.RGB(255, 255, 255))
		text.NewFace(f, 28).Draw(r, render.Point{X: 8, Y: 40}, "SDF text 0123", render.RGB(0, 120, 215))
	})
}

func TestLayoutRender(t *testing.T) {
	testFrame(t, "layout", 220, 150, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, 14)
		root := controls.NewBorder().
			SetBackground(render.RGB(32, 32, 36)).
			SetRadius(8).
			SetPadding(render.Uniform(12)).
			SetChild(controls.NewStackPanel(controls.Vertical).SetGap(8).Add(
				controls.NewTextBlock(face, "fluo layout").SetColor(render.RGB(255, 255, 255)),
				controls.NewFixed(0, 24, render.RGB(0, 120, 215)), // stretches full width
				func() core.Widget {
					y := controls.NewFixed(60, 18, render.RGB(255, 185, 0))
					y.SetAlign(core.End, core.Start)
					return y
				}(),
			))
		core.MeasureWidget(root, render.Size{W: 220, H: 150})
		core.ArrangeWidget(root, render.Rect{X: 10, Y: 10, W: 200, H: 130})
		core.RenderWidget(root, r)
	})
}

func TestScrollClipRender(t *testing.T) {
	testFrame(t, "scroll", 160, 120, func(r *glr.Renderer) {
		stack := controls.NewStackPanel(controls.Vertical)
		for i := 0; i < 8; i++ {
			c := render.RGB(0, 120, 215)
			if i%2 == 1 {
				c = render.RGB(255, 185, 0)
			}
			stack.Add(controls.NewFixed(120, 30, c))
		}

		sv := controls.NewScrollViewer().SetChild(stack)
		sv.SetWidth(120)
		sv.SetHeight(100)
		sv.ScrollTo(45)

		root := controls.NewCanvas().Add(sv, 10, 10)
		core.MeasureWidget(root, render.Size{W: 160, H: 120})
		core.ArrangeWidget(root, render.Rect{X: 0, Y: 0, W: 160, H: 120})
		core.RenderWidget(root, r)
	})
}

// TestFluentButton is the Phase 4 milestone golden: a themed, laid-out
// Fluent button in a real GL context, composed ONLY from theme.FluentLight
// tokens (no literal colors/metrics) — a card-colored Border filling the
// frame (inset 8px) with an accent button composite centered inside it.
func TestFluentButton(t *testing.T) {
	theme.SetActive(theme.FluentLight())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "fluent_button", 200, 80, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		label := controls.NewTextBlock(face, "Accept").SetColor(th.Color.AccentText)

		button := controls.NewBorder().
			SetBackground(th.Color.Accent).
			SetRadius(th.Metric.ControlCornerRadius).
			SetPadding(render.Thickness{
				Left: th.Metric.PaddingL, Right: th.Metric.PaddingL,
				Top: th.Metric.PaddingM, Bottom: th.Metric.PaddingM,
			}).
			SetChild(label)
		button.SetAlign(core.Center, core.Center)

		card := controls.NewBorder().
			SetBackground(th.Color.CardBackground).
			SetRadius(th.Metric.CornerRadius).
			SetChild(button)

		inset := render.Rect{X: 8, Y: 8, W: 200 - 16, H: 80 - 16}
		core.MeasureWidget(card, render.Size{W: inset.W, H: inset.H})
		core.ArrangeWidget(card, inset)
		core.RenderWidget(card, r)
	})
}
