package gl_test

import (
	"testing"

	"github.com/go-gl/gl/v3.3-core/gl"

	"github.com/0xdreadnaught/fluo/render"
	glr "github.com/0xdreadnaught/fluo/render/gl"
	"github.com/0xdreadnaught/fluo/render/gl/gltest"
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
		r.PushClip(render.Rect{X: 0, Y: 0, W: 40, H: 96})                            // nested: intersects
		r.FillRect(render.Rect{X: 0, Y: 0, W: 128, H: 96}, render.RGB(255, 185, 0))
		r.PopClip()
		r.PopClip()
		r.FillRect(render.Rect{X: 100, Y: 70, W: 40, H: 40}, render.RGB(16, 124, 16)) // unclipped again
	})
}
