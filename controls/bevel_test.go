package controls

import (
	"testing"

	"github.com/go-gl/gl/v3.3-core/gl"

	"github.com/0xdreadnaught/fluo/render"
	glr "github.com/0xdreadnaught/fluo/render/gl"
	"github.com/0xdreadnaught/fluo/render/gl/gltest"
	"github.com/0xdreadnaught/fluo/theme"
)

// TestBevels is a golden-image test for the four bevel helpers: a raised
// tab, a sunken well, and a horizontal groove, rendered against a
// ButtonFace-colored background so the chisel edges read clearly.
func TestBevels(t *testing.T) {
	th := theme.Light().Color

	gltest.Run(t, 220, 70, func(fb *gltest.Framebuffer) {
		r, err := glr.New()
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		gl.ClearColor(th.ButtonFace.R, th.ButtonFace.G, th.ButtonFace.B, 1)
		gl.Clear(gl.COLOR_BUFFER_BIT)
		r.Begin(fb.W, fb.H, 1)

		drawRaised(r, render.Rect{X: 8, Y: 8, W: 60, H: 40}, th.ButtonFace, th)
		drawSunken(r, render.Rect{X: 80, Y: 8, W: 60, H: 40}, th.WindowWell, th)
		drawGroove(r, render.Rect{X: 152, Y: 26, W: 60, H: 4}, true, th)

		r.End()
		gltest.CheckGolden(t, "bevel", fb.Image())
	})
}

// recordRenderer is a minimal render.Renderer stub that records every
// FillRect call (rect + color) into a slice; every other method is a
// no-op. Modeled on the recordRenderer in core/engine_test.go, adapted to
// capture FillRect calls since that is all drawRaised/drawSunken/
// drawGroove/drawFocusRect issue.
type recordRenderer struct {
	fills []filledRect
}

// filledRect pins one recorded FillRect call.
type filledRect struct {
	rect  render.Rect
	color render.Color
}

func (r *recordRenderer) Begin(fbWidth, fbHeight int, scale float32) {}
func (r *recordRenderer) End()                                       {}
func (r *recordRenderer) FillRect(rect render.Rect, c render.Color) {
	r.fills = append(r.fills, filledRect{rect: rect, color: c})
}
func (r *recordRenderer) FillRoundedRect(rect render.Rect, radius float32, c render.Color) {}
func (r *recordRenderer) DrawGradientRect(rect render.Rect, from, to render.Color, horizontal bool) {
}
func (r *recordRenderer) StrokeRoundedRect(rect render.Rect, radius, width float32, c render.Color) {
}
func (r *recordRenderer) DrawShadow(rect render.Rect, radius, blur float32, c render.Color) {}
func (r *recordRenderer) DrawBackdropBlur(rect render.Rect, radius float32, tint render.Color) {
}
func (r *recordRenderer) CreateTexture(w, h int, rgba []byte) render.TextureID           { return 0 }
func (r *recordRenderer) UpdateTexture(id render.TextureID, x, y, w, h int, rgba []byte) {}
func (r *recordRenderer) DeleteTexture(id render.TextureID)                              {}
func (r *recordRenderer) DrawQuad(dst, src render.Rect, tex render.TextureID, tint render.Color) {
}
func (r *recordRenderer) DrawSDFQuads(quads []render.GlyphQuad, tex render.TextureID, c render.Color) {
}
func (r *recordRenderer) DrawGlyphs(quads []render.GlyphQuad, tex render.TextureID, c render.Color) {
}
func (r *recordRenderer) Scale() float32            { return 1 }
func (r *recordRenderer) PushClip(rect render.Rect) {}
func (r *recordRenderer) PopClip()                  {}

// TestDrawRaisedEmitsEdges pins drawRaised's geometry without a GPU: the
// first FillRect must be the face fill, followed by the outer top/bottom
// edges and the inner top/bottom edges at the exact rows the brief
// specifies, in the right tones.
func TestDrawRaisedEmitsEdges(t *testing.T) {
	th := theme.Light().Color
	rect := render.Rect{X: 10, Y: 20, W: 60, H: 40}
	face := th.ButtonFace

	rr := &recordRenderer{}
	drawRaised(rr, rect, face, th)

	if len(rr.fills) == 0 {
		t.Fatal("drawRaised emitted no FillRect calls")
	}
	if got := rr.fills[0]; got.rect != rect || got.color != face {
		t.Fatalf("first FillRect = %+v, want face fill %v over %v", got, face, rect)
	}

	// find locates a 1px-thick horizontal edge row (H==1) at y, skipping the
	// face fill and the 1px-wide vertical edges (which can share the same Y).
	find := func(y float32) (filledRect, bool) {
		for _, f := range rr.fills {
			if f.rect.Y == y && f.rect.H == 1 {
				return f, true
			}
		}
		return filledRect{}, false
	}

	if f, ok := find(rect.Y); !ok || f.color != th.ButtonHighlight {
		t.Errorf("outer top edge at y=%v: got %+v, ok=%v, want color ButtonHighlight", rect.Y, f, ok)
	}
	if f, ok := find(rect.Y + rect.H - 1); !ok || f.color != th.ButtonDarkShadow {
		t.Errorf("outer bottom edge at y=%v: got %+v, ok=%v, want color ButtonDarkShadow", rect.Y+rect.H-1, f, ok)
	}
	if f, ok := find(rect.Y + 1); !ok || f.color != th.ButtonLight {
		t.Errorf("inner top edge at y=%v: got %+v, ok=%v, want color ButtonLight", rect.Y+1, f, ok)
	}
	if f, ok := find(rect.Y + rect.H - 2); !ok || f.color != th.ButtonShadow {
		t.Errorf("inner bottom edge at y=%v: got %+v, ok=%v, want color ButtonShadow", rect.Y+rect.H-2, f, ok)
	}
}
