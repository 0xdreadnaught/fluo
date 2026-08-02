package gl_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/go-gl/gl/v3.3-core/gl"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/0xdreadnaught/fluo/bind"
	"github.com/0xdreadnaught/fluo/controls"
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
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

func TestGradientRect(t *testing.T) {
	testFrame(t, "gradient", 120, 80, func(r *glr.Renderer) {
		r.DrawGradientRect(render.Rect{X: 8, Y: 8, W: 104, H: 28}, render.RGB(10, 36, 106), render.RGB(166, 202, 240), true) // horizontal
		r.DrawGradientRect(render.Rect{X: 8, Y: 44, W: 104, H: 28}, render.RGB(0, 0, 0), render.RGB(255, 255, 255), false)   // vertical
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

// TestShapeAA2x is the HiDPI shape-edge golden: a rounded-rect fill, a
// filled circle, a rounded-rect stroke, and a shadowed card — one shape per
// shader mode (3, 3, 4, 5) — laid out in a 160x100 logical frame and
// rendered into a 2x framebuffer (320x200) with Begin's scale set to 2
// instead of 1. Nothing about the draw calls differs from a 1x scene at
// the same logical coordinates; only the framebuffer size and scale do.
// Proves the shape shader's fwidth-based AA band (see shader.go) tracks
// the screen-space derivative rather than a fixed logical-unit width, so
// edges stay a crisp ~1 device-pixel band at 2x instead of visibly
// softening the way the old fixed band did.
func TestShapeAA2x(t *testing.T) {
	gltest.Run(t, 320, 200, func(fb *gltest.Framebuffer) {
		r, err := glr.New()
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		gl.ClearColor(0.12, 0.12, 0.14, 1)
		gl.Clear(gl.COLOR_BUFFER_BIT)
		r.Begin(fb.W, fb.H, 2)

		r.FillRoundedRect(render.Rect{X: 8, Y: 8, W: 60, H: 40}, 8, render.RGB(0, 120, 215))
		r.FillRoundedRect(render.Rect{X: 96, Y: 8, W: 40, H: 40}, 20, render.RGB(255, 185, 0)) // circle
		r.StrokeRoundedRect(render.Rect{X: 8, Y: 54, W: 60, H: 38}, 8, 2, render.RGB(255, 255, 255))
		card := render.Rect{X: 92, Y: 54, W: 60, H: 38}
		r.DrawShadow(card, 8, 10, render.RGBA(0, 0, 0, 140))
		r.FillRoundedRect(card, 8, render.RGB(243, 243, 243))

		r.End()
		gltest.CheckGolden(t, "shape_aa_2x", fb.Image())
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

// TestText2x is the Phase 8 Task 6 high-DPI golden: TestText's exact
// scenario — the identical LOGICAL draw coordinates and font sizes — run
// through the FBO harness at a 2x framebuffer (512x192 instead of
// 256x96) with Begin's scale set to 2 instead of 1. Nothing about the
// draw calls below changes from TestText; only the framebuffer size and
// the scale passed to Begin do. This proves the renderer's scale
// (fbW/winW, the same value app.Surface.Frame derives and passes to
// Begin) actually flows through to glyph quad placement/sizing (via
// the vertex shader's aPos*uScale — see shader.go) and the scissor clip
// path (applyClip's rd.scale multiply), producing a crisp 2x-resolution
// render of the SAME logical layout as text.png — not a blurry pixel
// upscale of the 1x image. See testFrame/TestText for the 1x baseline
// this mirrors; text.png itself is untouched by this task.
func TestText2x(t *testing.T) {
	gltest.Run(t, 512, 192, func(fb *gltest.Framebuffer) {
		r, err := glr.New()
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		gl.ClearColor(0.12, 0.12, 0.14, 1)
		gl.Clear(gl.COLOR_BUFFER_BIT)
		r.Begin(fb.W, fb.H, 2)

		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		text.NewFace(f, 14).Draw(r, render.Point{X: 8, Y: 8}, "Hello, fluo!", render.RGB(255, 255, 255))
		text.NewFace(f, 28).Draw(r, render.Point{X: 8, Y: 40}, "SDF text 0123", render.RGB(0, 120, 215))

		r.End()
		gltest.CheckGolden(t, "text_2x", fb.Image())
	})
}

// TestHDText is the HD-text golden: the same logical line of text drawn via
// Face.Draw (the crisp direct grayscale-AA path) twice into one
// 320x120 framebuffer — once at scale 1 (top row, device px == logical px)
// and once at scale 2 (bottom row, logical coordinates chosen so the 2x
// device-pixel result lands in the frame's bottom half) — each its own
// Begin/End pass against the SAME renderer and framebuffer, exactly as two
// successive app.Surface.Frame calls at different DPI would. Inspected by
// hand for crisp, non-clipped glyph edges at both scales (see the
// implementation plan's Step 4).
func TestHDText(t *testing.T) {
	gltest.Run(t, 320, 120, func(fb *gltest.Framebuffer) {
		r, err := glr.New()
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, 14)

		gl.ClearColor(0.12, 0.12, 0.14, 1)
		gl.Clear(gl.COLOR_BUFFER_BIT)

		// Row 1: scale 1, top half.
		r.Begin(fb.W, fb.H, 1)
		face.Draw(r, render.Point{X: 10, Y: 10}, "HD text 0123 crisp", render.RGB(255, 255, 255))
		r.End()

		// Row 2: scale 2, same fb — logical Y=35 * scale 2 = device Y=70,
		// landing in the bottom half (fb height 120).
		r.Begin(fb.W, fb.H, 2)
		face.Draw(r, render.Point{X: 5, Y: 35}, "HD text 0123 crisp", render.RGB(255, 255, 255))
		r.End()

		gltest.CheckGolden(t, "hdtext", fb.Image())
	})
}

// cjkFallbackPath is a real CJK system .ttc used to exercise Face's
// fallback chain against a font that actually covers Han characters
// goregular (a Latin-only text face) lacks. We don't embed a large font in
// the repo just for this golden, so TestFaceFallback skips if it's absent
// — same pattern as text.TestLoadCollectionRealFont.
const cjkFallbackPath = "/mnt/c/Windows/Fonts/msyh.ttc"

// TestFaceFallback is the font-fallback golden: a Face built via
// text.NewFaceWithFallback (primary goregular + a CJK fallback) draws a
// string mixing Latin glyphs (rendered from the primary's atlas) with a
// Han character the primary lacks (rendered from the fallback's own
// atlas), exercising Face.Draw's per-source-font texture batching — see
// text.Face.Draw. Skipped if the CJK system font isn't present, or (via
// gltest.Run) if no GL context is available.
func TestFaceFallback(t *testing.T) {
	data, err := os.ReadFile(cjkFallbackPath)
	if err != nil {
		t.Skipf("real .ttc not available at %s: %v", cjkFallbackPath, err)
	}
	fallback, err := text.LoadCollectionMember(data, 0)
	if err != nil {
		t.Fatal(err)
	}

	testFrame(t, "face_fallback", 200, 60, func(r *glr.Renderer) {
		primary, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFaceWithFallback(primary, []*text.Font{fallback}, 24)
		face.Draw(r, render.Point{X: 8, Y: 8}, "fluo 中文", render.RGB(255, 255, 255))
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

// TestScrollHorizontal is the control-variants Task 3 golden for
// ScrollViewer's horizontal scrolling: a horizontal StackPanel of 6 alternating
// -color Fixed(60,40) blocks (desired width 380, taller than the 140x60
// ScrollViewer's viewport is wide but not tall) scrolled right via
// ScrollToX(100), showing the horizontal thumb along the bottom edge and no
// vertical thumb (the content fits vertically — this is a purely
// horizontally-overflowing ScrollViewer, per the type doc comment's
// single-axis scenario).
func TestScrollHorizontal(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	testFrame(t, "scroll_horizontal", 160, 120, func(r *glr.Renderer) {
		stack := controls.NewStackPanel(controls.Horizontal).SetGap(4)
		for i := 0; i < 6; i++ {
			c := render.RGB(0, 120, 215)
			if i%2 == 1 {
				c = render.RGB(255, 185, 0)
			}
			stack.Add(controls.NewFixed(60, 40, c))
		}

		sv := controls.NewScrollViewer().SetChild(stack)
		sv.SetWidth(140)
		sv.SetHeight(60)
		sv.ScrollToX(100)

		root := controls.NewCanvas().Add(sv, 10, 10)
		core.MeasureWidget(root, render.Size{W: 160, H: 120})
		core.ArrangeWidget(root, render.Rect{X: 0, Y: 0, W: 160, H: 120})
		core.RenderWidget(root, r)
	})
}

// TestClassicButton is the Phase 4 milestone golden: a themed, laid-out
// button in a real GL context, composed ONLY from theme.Light tokens (no
// literal colors/metrics) — a ButtonFace Border filling the frame (inset
// 8px) with a Highlight/HighlightText button composite centered inside it.
func TestClassicButton(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "classic_button", 200, 80, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		label := controls.NewTextBlock(face, "Accept").SetColor(th.Color.HighlightText)

		button := controls.NewBorder().
			SetBackground(th.Color.Highlight).
			SetRadius(th.Metric.ControlCornerRadius).
			SetPadding(render.Thickness{
				Left: th.Metric.PaddingL, Right: th.Metric.PaddingL,
				Top: th.Metric.PaddingM, Bottom: th.Metric.PaddingM,
			}).
			SetChild(label)
		button.SetAlign(core.Center, core.Center)

		card := controls.NewBorder().
			SetBackground(th.Color.ButtonFace).
			SetRadius(th.Metric.CornerRadius).
			SetChild(button)

		inset := render.Rect{X: 8, Y: 8, W: 200 - 16, H: 80 - 16}
		core.MeasureWidget(card, render.Size{W: inset.W, H: inset.H})
		core.ArrangeWidget(card, inset)
		core.RenderWidget(card, r)
	})
}

// TestButtons is the Phase 5 Task 3 golden: three controls.Button instances
// side by side in a StackPanel — default, accent, and disabled — proving
// their token-driven fills/strokes/label colors render correctly together.
func TestButtons(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "buttons", 320, 60, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		def := controls.NewButton(face, "Button")
		accent := controls.NewButton(face, "Accent").SetAccent(true)
		disabled := controls.NewButton(face, "Disabled").SetEnabled(false)

		row := controls.NewStackPanel(controls.Horizontal).SetGap(12).Add(def, accent, disabled)

		frame := render.Rect{X: 0, Y: 0, W: 320, H: 60}
		r.FillRect(frame, th.Color.ButtonFace)

		core.MeasureWidget(row, render.Size{W: frame.W, H: frame.H})
		desired := core.DesiredSizeOf(row)
		bounds := render.Rect{
			X: (frame.W - desired.W) / 2, Y: (frame.H - desired.H) / 2,
			W: desired.W, H: desired.H,
		}
		core.ArrangeWidget(row, bounds)
		core.RenderWidget(row, r)
	})
}

// TestButtonPill is the control-variants Task 2 golden for ButtonShape:
// four pill (stadium, radius = bounds.H/2) buttons side by side — rest,
// accent (raised + outer StrokeRoundedRect ring), checked (a ToggleButton,
// sunken/pressed-in bevel), and focused (a rounded StrokeRoundedRect focus
// ring instead of the square drawFocusRect) — proving drawRaisedRounded/
// drawSunkenRounded and the rounded focus/accent chrome all render
// correctly together. Focus is set directly via OnFocusChanged (no router
// involved), the same shortcut TestTextBox's golden uses.
func TestButtonPill(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "button_pill", 420, 60, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		rest := controls.NewButton(face, "Play").SetShape(controls.ShapePill)
		accent := controls.NewButton(face, "Go").SetShape(controls.ShapePill).SetAccent(true)
		checked := controls.NewToggleButton(face, "On").SetShape(controls.ShapePill)
		checked.SetChecked(true)
		focused := controls.NewButton(face, "Tab").SetShape(controls.ShapePill)
		focused.OnFocusChanged(true)

		row := controls.NewStackPanel(controls.Horizontal).SetGap(12).Add(rest, accent, checked, focused)

		frame := render.Rect{X: 0, Y: 0, W: 420, H: 60}
		r.FillRect(frame, th.Color.ButtonFace)

		core.MeasureWidget(row, render.Size{W: frame.W, H: frame.H})
		desired := core.DesiredSizeOf(row)
		bounds := render.Rect{
			X: (frame.W - desired.W) / 2, Y: (frame.H - desired.H) / 2,
			W: desired.W, H: desired.H,
		}
		core.ArrangeWidget(row, bounds)
		core.RenderWidget(row, r)
	})
}

// TestButtonCircle is the control-variants Task 2 golden for ButtonShape:
// four circle (radius = min(bounds.W, bounds.H)/2) buttons side by side —
// rest, accent, checked (ToggleButton, sunken), and focused — the circle
// counterpart of TestButtonPill. Each label is a single short glyph, typical
// circle-button content (an icon-like badge); MeasureContent's square-aspect
// forcing (see Button.MeasureContent) is what makes bounds.W == bounds.H
// here despite the label itself not being square.
func TestButtonCircle(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "button_circle", 320, 70, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		rest := controls.NewButton(face, "1").SetShape(controls.ShapeCircle)
		accent := controls.NewButton(face, "2").SetShape(controls.ShapeCircle).SetAccent(true)
		checked := controls.NewToggleButton(face, "3").SetShape(controls.ShapeCircle)
		checked.SetChecked(true)
		focused := controls.NewButton(face, "4").SetShape(controls.ShapeCircle)
		focused.OnFocusChanged(true)

		row := controls.NewStackPanel(controls.Horizontal).SetGap(12).Add(rest, accent, checked, focused)

		frame := render.Rect{X: 0, Y: 0, W: 320, H: 70}
		r.FillRect(frame, th.Color.ButtonFace)

		core.MeasureWidget(row, render.Size{W: frame.W, H: frame.H})
		desired := core.DesiredSizeOf(row)
		bounds := render.Rect{
			X: (frame.W - desired.W) / 2, Y: (frame.H - desired.H) / 2,
			W: desired.W, H: desired.H,
		}
		core.ArrangeWidget(row, bounds)
		core.RenderWidget(row, r)
	})
}

// TestToggles is the Phase 5 Task 4 golden: a checkbox (unchecked, checked),
// a radio button (off, on), and a toggle switch (off, on), side by side in
// a StackPanel. Normative: all six show bare glyphs with no labels (empty
// label strings), so the row's only spacing is the outer StackPanel's
// PaddingM gap between the six controls — the composites' own internal
// label gap never triggers, per glyphMeasure/glyphArrange's "no gap for an
// empty label" rule.
func TestToggles(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "toggles", 360, 60, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		cbOff := controls.NewCheckBox(face, "")
		cbOn := controls.NewCheckBox(face, "").SetChecked(true)
		rbOff := controls.NewRadioButton(face, "")
		rbOn := controls.NewRadioButton(face, "").SetChecked(true)
		swOff := controls.NewToggleSwitch()
		swOn := controls.NewToggleSwitch().SetChecked(true)

		row := controls.NewStackPanel(controls.Horizontal).SetGap(th.Metric.PaddingM).
			Add(cbOff, cbOn, rbOff, rbOn, swOff, swOn)

		frame := render.Rect{X: 0, Y: 0, W: 360, H: 60}
		r.FillRect(frame, th.Color.ButtonFace)

		core.MeasureWidget(row, render.Size{W: frame.W, H: frame.H})
		desired := core.DesiredSizeOf(row)
		bounds := render.Rect{
			X: (frame.W - desired.W) / 2, Y: (frame.H - desired.H) / 2,
			W: desired.W, H: desired.H,
		}
		core.ArrangeWidget(row, bounds)
		core.RenderWidget(row, r)
	})
}

// TestTextBox is the Phase 5 Task 5 golden: a focused TextBox reading
// "Hello fluo" with runes 2..7 selected ("llo f") and the caret at 7,
// filling a 200x40 frame. Focus is set directly via OnFocusChanged (no
// router involved — this task doesn't wire pointer/key handling yet), and
// no timers.Queue is wired, so the caret renders solid (never blinked off)
// per TextBox.caretShown's documented behavior.
func TestTextBox(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "textbox", 200, 40, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		tb := controls.NewTextBox(face)
		tb.SetText("Hello fluo") // resets caret to end, clears selection
		tb.Select(2, 7)          // selects "llo f", caret ends at 7
		tb.OnFocusChanged(true)

		frame := render.Rect{X: 0, Y: 0, W: 200, H: 40}
		r.FillRect(frame, th.Color.ButtonFace)

		core.MeasureWidget(tb, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(tb, frame)
		core.RenderWidget(tb, r)
	})
}

// TestTextBoxPreedit is the Task 6 Phase B golden: a focused TextBox with
// committed text "Hello " and an IME composition in progress — preedit
// "world" spliced in at the caret, with the composition's own caret sitting
// at offset 3 within it (between "wor" and "ld") rather than at the end of
// the preedit run. The composition is driven through the real dispatch
// path (router.CompositionUpdate, not TextBox.OnComposition directly) to
// prove Router→CompositionHandler wiring end-to-end, exactly like
// TestTextBox drives focus through OnFocusChanged. The golden should show
// "Hello " in the normal text color, followed by "world" in the same color
// but with a thin underline rule beneath it (the provisional/uncommitted
// cue — see TextBox.renderComposing), and the caret bar positioned inside
// "world" between "wor" and "ld" rather than after it or at the original
// pre-composition caret position. No selection highlight is drawn (an
// active composition never has one — see renderComposing's doc comment),
// and no timers.Queue is wired, so the caret renders solid.
func TestTextBoxPreedit(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "textbox_preedit", 200, 40, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		tb := controls.NewTextBox(face)
		tb.SetText("Hello ") // caret ends at 6 (end of committed text)

		router := input.NewRouter()
		router.SetRoot(tb)
		router.Focus(tb)                     // fires OnFocusChanged(true) on tb
		router.CompositionUpdate("world", 3) // preedit "world", caret between "wor" and "ld"

		frame := render.Rect{X: 0, Y: 0, W: 200, H: 40}
		r.FillRect(frame, th.Color.ButtonFace)

		core.MeasureWidget(tb, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(tb, frame)
		core.RenderWidget(tb, r)
	})
}

// TestTextBoxMultiline is the multi-line TextBox golden: a focused,
// multi-line TextBox reading "Hello fluo\nSecond line\nThird", filling a
// 220x100 frame. The selection (runes 6..17, "fluo\nSecond") spans the
// first newline entirely and continues partway into the second line, so
// the highlight band is visibly split across two lines — the first
// covering just "fluo" (the tail of line 0) and the second covering just
// "Second" (the head of line 1) — proving per-line selection intersection
// (see TextBox.renderMultiline). The caret (solid — no timers.Queue wired)
// sits at index 17, on line 1 right after "Second" and before " line". As
// in TestTextBox, focus is set directly via OnFocusChanged and no router is
// involved.
func TestTextBoxMultiline(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "textbox_multiline", 220, 100, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		tb := controls.NewTextBox(face).SetMultiline(true)
		tb.SetText("Hello fluo\nSecond line\nThird")
		tb.Select(6, 17) // "fluo\nSecond": spans the first newline
		tb.OnFocusChanged(true)

		frame := render.Rect{X: 0, Y: 0, W: 220, H: 100}
		r.FillRect(frame, th.Color.ButtonFace)

		core.MeasureWidget(tb, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(tb, frame)
		core.RenderWidget(tb, r)
	})
}

// TestTextBoxWordWrap is the opt-in word-wrap golden (see
// TextBox.SetWordWrap): a focused, multi-line, word-wrapped TextBox holding
// one long logical line (no '\n' anywhere in it) that overflows a narrow
// 180px frame width and must re-flow into several visual rows — proving
// computeVisualRows' word-boundary breaking end-to-end, not just a single
// long unwrapped scrolling line. The selection spans a wide middle chunk of
// the text, crossing at least one soft wrap point, so the highlight band is
// visibly split across rows exactly like TestTextBoxMultiline's
// hard-newline-spanning selection is split across lines (see
// TextBox.renderMultilineWrapped) — the wrapping analogue of that same
// per-line intersection. The caret (solid — no timers.Queue wired) sits at
// the selection's far end. No explicit SetWidth/SetHeight: the box
// Stretch-fills the frame (matching every other golden here), so the
// frame's own width is what the text wraps against, and the frame's height
// is generously larger than the wrapped content needs (so no vertical
// scrolling is in play — the whole wrapped paragraph is visible).
func TestTextBoxWordWrap(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "textbox_wordwrap", 180, 140, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		tb := controls.NewTextBox(face).SetMultiline(true).SetWordWrap(true)
		content := "The quick brown fox jumps over the lazy dog and then jumps again near the old fence"
		tb.SetText(content)
		n := len([]rune(content))
		tb.Select(20, n-20) // a wide middle chunk, crossing at least one soft wrap
		tb.OnFocusChanged(true)

		frame := render.Rect{X: 0, Y: 0, W: 180, H: 140}
		r.FillRect(frame, th.Color.ButtonFace)

		core.MeasureWidget(tb, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(tb, frame)
		core.RenderWidget(tb, r)
	})
}

// TestTextBoxVScroll is the visible-vertical-scroll-thumb golden (see
// TextBox's vScrollTrack/vScrollThumbRect/RenderOverlay): a focused,
// multi-line (word-wrap OFF) TextBox holding eight short lines in a 220x100
// frame tall enough for only about four of them — the content overflows
// vertically, so the classic track+raised-thumb chrome (drawScrollThumb,
// the exact same chrome ScrollViewer and the ListView/DataGrid virtualizer
// already share — see bevel.go) is drawn along the right inner edge. The
// caret sits at the very end of the text (SetText's own caret-to-end
// convention), so vscroll is clamped near its maximum and the thumb should
// sit near the BOTTOM of its track, proportionally sized to roughly half
// the track's height (about 4 of 8 lines are visible at once).
func TestTextBoxVScroll(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "textbox_vscroll", 220, 100, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		tb := controls.NewTextBox(face).SetMultiline(true)
		tb.SetText("Line one\nLine two\nLine three\nLine four\nLine five\nLine six\nLine seven\nLine eight")
		tb.OnFocusChanged(true)

		frame := render.Rect{X: 0, Y: 0, W: 220, H: 100}
		r.FillRect(frame, th.Color.ButtonFace)

		core.MeasureWidget(tb, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(tb, frame)
		core.RenderWidget(tb, r)
	})
}

// TestTextBoxGutter is the opt-in line-number gutter golden (see
// TextBox.SetLineNumbers): a focused, multi-line (word-wrap OFF) TextBox
// holding thirteen short lines in a 220x160 frame, with the gutter on. The
// golden should show a two-digit-wide column along the left inner edge
// carrying the numbers 1..13 in GrayText, each RIGHT-aligned against the
// thin vertical rule that separates the column from the text, with every
// line of text starting just to the right of that rule rather than at the
// box's usual padding inset. Thirteen lines is deliberately past nine, so
// the column is sized for two digits and the single-digit numbers 1..9
// visibly hang right-aligned under the "10".."13" below them. The content is
// taller than the viewport, so the vertical scroll thumb is drawn on the
// right at the same time — the two gutters, one per edge, coexist — and the
// caret sits at the end of the text (SetText's own caret-to-end convention),
// so the view is scrolled to the bottom and the visible numbers are the
// TAIL of the range, proving the gutter scrolls in lockstep with the text
// rather than being pinned to the top.
func TestTextBoxGutter(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "textbox_gutter", 220, 160, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		tb := controls.NewTextBox(face).SetMultiline(true).SetLineNumbers(true)
		tb.SetText("package main\n\nimport \"fmt\"\n\nfunc main() {\n    for i := 0; i < 3; i++ {\n        say(i)\n    }\n}\n\nfunc say(n int) {\n    fmt.Println(n)\n}")
		tb.OnFocusChanged(true)

		frame := render.Rect{X: 0, Y: 0, W: 220, H: 160}
		r.FillRect(frame, th.Color.ButtonFace)

		core.MeasureWidget(tb, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(tb, frame)
		core.RenderWidget(tb, r)
	})
}

// TestTextBoxSourceEditor is the v0.15.9 golden: the augment source-editor
// config — multi-line, word-wrap OFF (so long lines horizontally scroll
// rather than wrapping), line numbers on, and taller content than the frame
// so the vertical thumb (and its reserved gutter) shows. The FIRST line is
// far wider than the box; before the fix its text drew straight under the
// thumb (the render clipped only to the full bounds while the gutter was
// reserved only for the scroll decision). This golden captures the fix: the
// long line stops at the thumb's left edge, and — because the caret sits at
// the start (hscroll 0) — the line also stops cleanly at the line-number
// gutter on the left. Short lines (which fit) are unchanged, matching
// textbox_gutter.
func TestTextBoxSourceEditor(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "textbox_source_editor", 220, 120, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		tb := controls.NewTextBox(face).SetMultiline(true).SetLineNumbers(true)
		tb.SetText("def handle(request, context, options, retries, deadline):\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10")
		tb.OnFocusChanged(true)
		tb.SetCaret(0) // hscroll pinned at the start so the long line shows from column 0

		frame := render.Rect{X: 0, Y: 0, W: 220, H: 120}
		r.FillRect(frame, th.Color.ButtonFace)

		core.MeasureWidget(tb, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(tb, frame)
		core.RenderWidget(tb, r)
	})
}

// TestSliderProgress is the Phase 5 Task 7 golden: a Slider at 0.6 (over the
// default [0,1] range) stacked above a ProgressBar at 0.3, in a vertical
// StackPanel gapped by PaddingM, filling a 200x60 frame.
func TestSliderProgress(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "slider_progress", 200, 60, func(r *glr.Renderer) {
		slider := controls.NewSlider().SetValue(0.6)
		progress := controls.NewProgressBar().SetValue(0.3)

		stack := controls.NewStackPanel(controls.Vertical).SetGap(th.Metric.PaddingM).
			Add(slider, progress)

		frame := render.Rect{X: 0, Y: 0, W: 200, H: 60}
		r.FillRect(frame, th.Color.ButtonFace)

		core.MeasureWidget(stack, render.Size{W: frame.W, H: frame.H})
		desired := core.DesiredSizeOf(stack)
		bounds := render.Rect{
			X: (frame.W - desired.W) / 2, Y: (frame.H - desired.H) / 2,
			W: desired.W, H: desired.H,
		}
		core.ArrangeWidget(stack, bounds)
		core.RenderWidget(stack, r)
	})
}

// TestSliderVertical is the control-variants golden for Slider's Vertical
// orientation: a single vertical slider at Value 65 (over [0,100] — Max at
// the TOP per the type doc comment) centered in a 60x200 frame. Confirms
// the raised thumb sits above center (closer to Max/top) and the
// Highlight fill covers the Min side (below the thumb).
func TestSliderVertical(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "slider_vertical", 60, 200, func(r *glr.Renderer) {
		slider := controls.NewSlider().SetOrientation(controls.Vertical).SetRange(0, 100).SetValue(65)

		frame := render.Rect{X: 0, Y: 0, W: 60, H: 200}
		r.FillRect(frame, th.Color.ButtonFace)

		core.MeasureWidget(slider, render.Size{W: frame.W, H: frame.H})
		desired := core.DesiredSizeOf(slider)
		bounds := render.Rect{
			X: (frame.W - desired.W) / 2, Y: (frame.H - desired.H) / 2,
			W: desired.W, H: desired.H,
		}
		core.ArrangeWidget(slider, bounds)
		core.RenderWidget(slider, r)
	})
}

// TestProgressVertical is the control-variants golden for ProgressBar's
// Vertical orientation: a single vertical chunked progress bar at Value
// 0.6, centered in a 60x200 frame. Confirms the chunks stack bottom-to-top
// (Value 0.6 fills roughly the bottom 60% of the well).
func TestProgressVertical(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "progress_vertical", 60, 200, func(r *glr.Renderer) {
		progress := controls.NewProgressBar().SetOrientation(controls.Vertical).SetValue(0.6)

		frame := render.Rect{X: 0, Y: 0, W: 60, H: 200}
		r.FillRect(frame, th.Color.ButtonFace)

		core.MeasureWidget(progress, render.Size{W: frame.W, H: frame.H})
		desired := core.DesiredSizeOf(progress)
		bounds := render.Rect{
			X: (frame.W - desired.W) / 2, Y: (frame.H - desired.H) / 2,
			W: desired.W, H: desired.H,
		}
		core.ArrangeWidget(progress, bounds)
		core.RenderWidget(progress, r)
	})
}

// TestProgressSolid is the control-variants golden for ProgressBar's solid
// fill variant: a single Horizontal solid progress bar at Value 0.6,
// centered in a 200x40 frame. Confirms the fill is one continuous
// Highlight bar (no chunk gaps), contrasting with TestSliderProgress's
// default chunked look.
func TestProgressSolid(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "progress_solid", 200, 40, func(r *glr.Renderer) {
		progress := controls.NewProgressBar().SetSolid(true).SetValue(0.6)

		frame := render.Rect{X: 0, Y: 0, W: 200, H: 40}
		r.FillRect(frame, th.Color.ButtonFace)

		core.MeasureWidget(progress, render.Size{W: frame.W, H: frame.H})
		desired := core.DesiredSizeOf(progress)
		bounds := render.Rect{
			X: (frame.W - desired.W) / 2, Y: (frame.H - desired.H) / 2,
			W: desired.W, H: desired.H,
		}
		core.ArrangeWidget(progress, bounds)
		core.RenderWidget(progress, r)
	})
}

// TestComboOpen is the Phase 5 Task 8 golden: an open ComboBox — 3 items
// ("Red", "Green", "Blue"), "Green" (index 1) selected-highlighted — inside
// a 220x160 frame. The popup is opened via the SAME router-driven path a
// real app would use (focus the field, then KeyDown(Enter), landing on
// ComboBox.OnKey's Space/Enter/Down branch) rather than by reaching into the
// unexported openPopup directly, so the golden also exercises the whole
// input-to-popup path end to end, not just the popup's own visuals.
func TestComboOpen(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "combo_open", 220, 160, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		combo := controls.NewComboBox(face)
		combo.SetItems([]string{"Red", "Green", "Blue"})
		combo.SetSelectedIndex(1)
		combo.SetWidth(120)
		combo.SetHeight(32)
		combo.SetAlign(core.Start, core.Start) // top-left, so the popup opens downward with room to fit
		combo.SetMargin(render.Uniform(10))

		host := controls.NewOverlayHost()
		router := input.NewRouter()
		host.SetRouter(router)
		host.SetContent(combo)
		router.SetRoot(host)

		frame := render.Rect{X: 0, Y: 0, W: 220, H: 160}
		r.FillRect(frame, th.Color.ButtonFace)

		// First layout pass: gives the field real arranged bounds, so the
		// popup (opened below) has a real anchor rect to place against.
		core.MeasureWidget(host, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(host, frame)

		router.Focus(combo)
		router.KeyDown(input.KeyEnter, 0, 0) // opens the popup via ComboBox.OnKey

		// Second layout pass: the host now also has the popup as a child,
		// so it needs placing relative to the field's anchor (see
		// OverlayHost.ArrangeContent/placePopup).
		core.MeasureWidget(host, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(host, frame)

		core.RenderWidget(host, r)
	})
}

// TestListView is the Phase 7 Task 3 golden: a ListView of 12 items
// ("Item 01".."Item 12"), index 3 ("Item 04") selected — showing the
// Highlight/HighlightText row band — scrolled to the
// top (via the Task 3 ScrollTo addition, mirroring ScrollViewer.ScrollTo)
// so the first several rows are visible with the scroll thumb showing
// (content taller than the 140px-high viewport), inside a 200x160 frame.
// Sized and positioned exactly like TestScrollClipRender's ScrollViewer
// golden: explicit SetWidth/SetHeight on the control, placed via Canvas at
// a fixed (10,10) offset rather than measured-and-centered.
func TestListView(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "listview", 200, 160, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		items := bind.NewList[string]()
		for i := 1; i <= 12; i++ {
			items.Add(fmt.Sprintf("Item %02d", i))
		}

		lv := controls.NewListView(face, items)
		lv.SetWidth(180)
		lv.SetHeight(140)
		lv.SetSelectedIndex(3)
		lv.ScrollTo(0) // pin to the top: rows 1.. visible, "Item 04" highlighted

		root := controls.NewCanvas().Add(lv, 10, 10)

		frame := render.Rect{X: 0, Y: 0, W: 200, H: 160}
		r.FillRect(frame, th.Color.ButtonFace)

		core.MeasureWidget(root, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(root, frame)
		core.RenderWidget(root, r)
	})
}

// TestTreeExpander is the Phase 7 Task 4 golden: a TreeView (left) showing
// two roots — "src" (expanded, with children "core", "controls" (the
// current selection, showing the Highlight/HighlightText
// row highlight), and "render") and "docs" (collapsed, hiding its own single
// child "readme.md" — proving the '>' collapsed chevron alongside "src"'s
// 'v' expanded one) — beside an expanded Expander (right) titled "Details"
// containing a TextBlock reading "Hello", inside a 260x180 frame.
func TestTreeExpander(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "tree_expander", 260, 180, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		coreNode := controls.NewTreeNode("core")
		controlsNode := controls.NewTreeNode("controls")
		renderNode := controls.NewTreeNode("render")
		src := controls.NewTreeNode("src", coreNode, controlsNode, renderNode).SetExpanded(true)
		docs := controls.NewTreeNode("docs", controls.NewTreeNode("readme.md"))

		tv := controls.NewTreeView(face, src, docs)
		tv.SetSelected(controlsNode)

		details := controls.NewExpander(face, "Details").SetExpanded(true)
		details.SetContent(controls.NewTextBlock(face, "Hello"))
		details.SetWidth(110)

		root := controls.NewCanvas().
			Add(tv, 10, 10).
			Add(details, 140, 10)

		frame := render.Rect{X: 0, Y: 0, W: 260, H: 180}
		r.FillRect(frame, th.Color.ButtonFace)

		core.MeasureWidget(root, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(root, frame)
		core.RenderWidget(root, r)
	})
}

// TestTabs is the Phase 7 Task 5 golden: a TabControl with 3 tabs ("One",
// "Two", "Three"), the second ("Two") selected — showing the Highlight
// underline beneath its header cell and its TextBlock content ("Tab two
// content") below the strip — inside a 240x120 frame. "One" and "Three"'s
// own content stays attached (per TabControl's normative "hidden tabs
// remain in the tree" rule) but is invisible, contributing nothing to this
// frame.
func TestTabs(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "tabs", 240, 120, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		tc := controls.NewTabControl(face)
		tc.AddTab("One", controls.NewTextBlock(face, "Tab one content"))
		tc.AddTab("Two", controls.NewTextBlock(face, "Tab two content"))
		tc.AddTab("Three", controls.NewTextBlock(face, "Tab three content"))
		tc.SetSelectedIndex(1)
		tc.SetWidth(220)
		tc.SetHeight(100)

		root := controls.NewCanvas().Add(tc, 10, 10)

		frame := render.Rect{X: 0, Y: 0, W: 240, H: 120}
		r.FillRect(frame, th.Color.ButtonFace)

		core.MeasureWidget(root, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(root, frame)
		core.RenderWidget(root, r)
	})
}

// TestMenuOpen is the Phase 7 Task 6 golden: a MenuBar ("File", "Edit") with
// "File" open — showing its Card+shadow popup with "New", "Open", a
// separator, "Exit", and a "Recent >" submenu trigger — and that submenu
// ITSELF expanded (as a second, nested popup) to the right of its row,
// showing "Report.docx" and "Notes.txt" — inside a 260x200 frame. Opened and
// expanded entirely through the real router-driven input path (matching
// TestComboOpen's own approach), not by reaching into any unexported
// controls internals:
//
//   - "File" is the bar's leftmost cell, so a press a few px in from the
//     bar's own left edge (comfortably inside "File"'s hit zone, whatever its
//     exact measured width) opens it.
//   - The "Recent" row's own position isn't observable from this external
//     package, so rather than hardcode its offset, a plain PointerMove sweep
//     (2px steps) down the popup's left edge re-hit-tests after every step
//     and stops the moment PopupCount() reports 2 — i.e. the moment the
//     sweep's Move has landed on "Recent" and its onHover has fired. Moving
//     across every OTHER row first (New/Open/separator/Exit) is harmless:
//     none of them do anything on hover beyond their own (invisible at this
//     resolution) fill state.
func TestMenuOpen(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "menu_open", 260, 200, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		bar := controls.NewMenuBar(face)
		bar.AddMenu("File").
			Add("New", nil).
			Add("Open", nil).
			AddSeparator().
			Add("Exit", nil).
			AddSub("Recent").
			Add("Report.docx", nil).
			Add("Notes.txt", nil)
		bar.AddMenu("Edit").Add("Undo", nil)
		bar.SetAlign(core.Start, core.Start) // top-left; never stretched

		host := controls.NewOverlayHost()
		router := input.NewRouter()
		host.SetRouter(router)
		host.SetContent(bar)
		router.SetRoot(host)

		frame := render.Rect{X: 0, Y: 0, W: 260, H: 200}
		r.FillRect(frame, th.Color.ButtonFace)

		// First layout pass: gives the bar real arranged bounds, so both the
		// click point below and the popup's anchor rect (computed from those
		// same bounds by MenuBar.openMenu) are correct.
		core.MeasureWidget(host, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(host, frame)

		barBounds := core.BoundsOf(bar)
		fileClick := render.Point{X: barBounds.X + 5, Y: barBounds.Y + barBounds.H/2}
		router.PointerButton(input.ButtonLeft, true, fileClick, 0)
		router.PointerButton(input.ButtonLeft, false, fileClick, 0)

		// Second layout pass: arranges the now-open File popup's rows, so
		// the hover sweep below hit-tests against real bounds.
		core.MeasureWidget(host, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(host, frame)

		sweepX := barBounds.X + 6
		opened := false
		for y := barBounds.Y + barBounds.H; y < frame.H; y += 2 {
			router.PointerMove(render.Point{X: sweepX, Y: y}, 0)
			if host.PopupCount() == 2 {
				opened = true
				break
			}
		}
		if !opened {
			t.Fatal("hover sweep never found the Recent submenu row (PopupCount never reached 2)")
		}

		// Third layout pass: arranges the newly-opened submenu popup's rows.
		core.MeasureWidget(host, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(host, frame)

		core.RenderWidget(host, r)
	})
}

// TestMenuBarHoveredCell is the hovered-menu-bar-cell golden, the one
// branch of MenuBar.Render no other golden reaches. menu_open.png shows the
// OPEN cell's sunken look and (inside the popup) a hovered ROW's navy band,
// but every cell in its bar is either open or at rest — the bar's own hover
// state, where a cell that is merely pointed at fills Highlight and flips
// its title to HighlightText, appears nowhere.
//
// A bar of three menus ("File", "Edit", "View") with NO menu open and the
// pointer resting a few px inside the leftmost cell, in a 260x40 frame.
// "File" should read as white-on-navy against its two plain
// WindowText-on-ButtonFace neighbours. The hover is driven through a real
// router.PointerMove (matching TestMenuOpen's router-driven approach) rather
// than by reaching into MenuBar's unexported hover state; no menu is opened
// first, precisely because an open popup would take the pointer capture and
// the Move would never reach the bar at all.
func TestMenuBarHoveredCell(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "menu_hover", 260, 40, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		bar := controls.NewMenuBar(face)
		bar.AddMenu("File").Add("New", nil)
		bar.AddMenu("Edit").Add("Undo", nil)
		bar.AddMenu("View").Add("Zoom", nil)
		bar.SetAlign(core.Start, core.Start) // top-left; never stretched

		host := controls.NewOverlayHost()
		router := input.NewRouter()
		host.SetRouter(router)
		host.SetContent(bar)
		router.SetRoot(host)

		frame := render.Rect{X: 0, Y: 0, W: 260, H: 40}
		r.FillRect(frame, th.Color.ButtonFace)

		core.MeasureWidget(host, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(host, frame)

		// A few px in from the bar's own left edge lands comfortably inside
		// "File"'s hit zone whatever its exact measured width — the same
		// trick TestMenuOpen uses to click that cell.
		barBounds := core.BoundsOf(bar)
		router.PointerMove(render.Point{X: barBounds.X + 5, Y: barBounds.Y + barBounds.H/2}, 0)

		core.RenderWidget(host, r)
	})
}

// TestDataGridOverlay is the DataGrid overlay-chrome golden: everything
// DataGrid.RenderOverlay draws, in one frame. datagrid_hscroll.png already
// shows both scroll thumbs, but nothing in the suite has ever rendered a
// FOCUSED virtualized control, so the focus ring — the last thing
// RenderOverlay paints, above both thumbs — has no pixel coverage at all.
//
// A 240x120 grid of three Px columns (120+140+120 = 380, well past the
// viewport, the deliberate Px-only overflow shape that forces a horizontal
// thumb — a Star column would resolve to exactly the viewport width and
// never overflow) over 30 rows, scrolled right AND down so both thumbs sit
// off their minimum, with focus taken through the real router (matching
// TestComboOpen/TestMenuOpen's router-driven approach) rather than by
// calling OnFocusChanged directly. The golden should show the classic
// 1px Highlight focus rectangle running just inside the grid's sunken outer
// well, unbroken along all four edges and crossing OVER both thumb gutters
// rather than stopping at them, with row 2's Highlight selection band
// visible in the body behind it.
func TestDataGridOverlay(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "datagrid_overlay", 260, 140, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		dg := controls.NewDataGrid(face)
		dg.SetColumns(
			controls.Column{Title: "Name", Width: controls.Px(120), Value: func(row int) string {
				return fmt.Sprintf("User %d", row)
			}},
			controls.Column{Title: "Email", Width: controls.Px(140), Value: func(row int) string {
				return fmt.Sprintf("u%d@example.com", row)
			}},
			controls.Column{Title: "Age", Width: controls.Px(120), Value: func(row int) string {
				return fmt.Sprintf("%d", 20+row)
			}},
		)
		dg.SetRowCount(30)
		dg.SetWidth(240)
		dg.SetHeight(120)
		dg.SetSelectedIndex(2)
		dg.ScrollToX(90)

		root := controls.NewCanvas().Add(dg, 10, 10)

		router := input.NewRouter()
		router.SetRoot(root)

		frame := render.Rect{X: 0, Y: 0, W: 260, H: 140}
		r.FillRect(frame, th.Color.ButtonFace)

		// First layout pass: gives the grid real bounds, so the wheel below
		// hit-tests into its body and the ring has a real rect to trace.
		core.MeasureWidget(root, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(root, frame)

		// Scroll down a notch through the real wheel path (DataGrid has no
		// public vertical ScrollTo — see its ScrollToX/OffsetX pair), so the
		// vertical thumb sits off the top of its track rather than pinned to
		// it, and row 2's selection band stays visible near the top.
		gridBounds := core.BoundsOf(dg)
		router.PointerWheel(
			render.Point{Y: -1},
			render.Point{X: gridBounds.X + gridBounds.W/2, Y: gridBounds.Y + gridBounds.H/2},
			0,
		)
		router.Focus(dg) // fires OnFocusChanged(true): the ring is drawn

		// Second layout pass: re-realizes the body cells at the new offset.
		core.MeasureWidget(root, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(root, frame)

		core.RenderWidget(root, r)
	})
}

// TestMenuDisabledItem is the disabled-menu-row golden: an "Edit" menu
// ("Copy", a disabled "Paste" via AddDisabled, "Cut") opened through the
// real router-driven input path (matching TestMenuOpen's own approach),
// showing the disabled "Paste" row's label rendered greyed (GrayText)
// between its two normal WindowText siblings, inside a 160x140 frame.
func TestMenuDisabledItem(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "menu_disabled", 160, 140, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		bar := controls.NewMenuBar(face)
		bar.AddMenu("Edit").
			Add("Copy", nil).
			AddDisabled("Paste", nil).
			Add("Cut", nil)
		bar.SetAlign(core.Start, core.Start) // top-left; never stretched

		host := controls.NewOverlayHost()
		router := input.NewRouter()
		host.SetRouter(router)
		host.SetContent(bar)
		router.SetRoot(host)

		frame := render.Rect{X: 0, Y: 0, W: 160, H: 140}
		r.FillRect(frame, th.Color.ButtonFace)

		// First layout pass: gives the bar real arranged bounds, so the click
		// point below is correct.
		core.MeasureWidget(host, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(host, frame)

		barBounds := core.BoundsOf(bar)
		editClick := render.Point{X: barBounds.X + 5, Y: barBounds.Y + barBounds.H/2}
		router.PointerButton(input.ButtonLeft, true, editClick, 0)
		router.PointerButton(input.ButtonLeft, false, editClick, 0)

		// Second layout pass: arranges the now-open Edit popup's rows.
		core.MeasureWidget(host, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(host, frame)

		core.RenderWidget(host, r)
	})
}

// TestDataGridHScroll is the control-variants Task 4 golden for the shared
// virtualizer's horizontal scroll: a DataGrid with 3 wide Px columns ("Name"
// 140, "Email" 160, "Age" 140 — 440 total, exceeding the grid's own 260px
// width and thus the viewport, the deliberate Px-only overflow scenario
// contentWidth=sum(colWidths) targets) and 10 rows, scrolled right via
// ScrollToX(120) so "Name" is mostly/fully scrolled past the left edge,
// "Email" sits mid-frame, and "Age" is fully visible — showing the header's
// cells scrolled in lockstep with the body's cells (both read the same
// offsetX, see DataGrid.ArrangeContent/Render's doc comments) so each
// column's title still lines up exactly over its own cells, plus the
// horizontal thumb along the bottom edge (offset right, past its own
// left-most position) alongside the vertical thumb (10 rows taller than the
// visible body), inside a 280x160 frame. Sized and positioned like
// TestDataGrid's own golden: explicit SetWidth/SetHeight, Canvas at (10,10).
func TestDataGridHScroll(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "datagrid_hscroll", 280, 160, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		dg := controls.NewDataGrid(face)
		dg.SetColumns(
			controls.Column{Title: "Name", Width: controls.Px(140), Value: func(row int) string {
				return fmt.Sprintf("User %d", row)
			}},
			controls.Column{Title: "Email", Width: controls.Px(160), Value: func(row int) string {
				return fmt.Sprintf("u%d@example.com", row)
			}},
			controls.Column{Title: "Age", Width: controls.Px(140), Value: func(row int) string {
				return fmt.Sprintf("%d", 20+row)
			}},
		)
		dg.SetRowCount(10)
		dg.SetWidth(260)
		dg.SetHeight(140)
		dg.SetSelectedIndex(1)
		dg.ScrollToX(120)

		root := controls.NewCanvas().Add(dg, 10, 10)

		frame := render.Rect{X: 0, Y: 0, W: 280, H: 160}
		r.FillRect(frame, th.Color.ButtonFace)

		core.MeasureWidget(root, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(root, frame)
		core.RenderWidget(root, r)
	})
}

// TestDialog is the Phase 7 Task 7 golden: a ShowDialog modal — scrim
// dimming two colored Fixed blocks sitting behind it in the host's own
// content, a centered shadowed Card with "Delete file?" / "This cannot be
// undone.", and a right-aligned "Cancel" (default)/"Delete" (accent) button
// row — inside a 280x180 frame.
// TestDataGrid is the Phase 7 Task 8 golden: a DataGrid with 3 columns
// ("Name" Px 80, "Email" Star, "Age" Px 60) and 20 rows ("User NN",
// "uNN@x.io", 20+NN), row 2 selected — showing the Highlight/HighlightText
// row band — scrolled to the top (offset 0, the
// zero-value default; no explicit scroll needed) so the header (fixed,
// raised ButtonFace cells + WindowText titles) sits
// above the first several body rows, each with its own 1px ButtonShadow
// grid line, with the scroll thumb showing (20 rows taller than the visible
// body), inside a 300x180 frame. Sized and positioned exactly like
// TestListView's own ScrollTo(0)-pinned golden: explicit SetWidth/SetHeight,
// placed via Canvas at a fixed (10,10) offset.
func TestDataGrid(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "datagrid", 300, 180, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		dg := controls.NewDataGrid(face)
		dg.SetColumns(
			controls.Column{Title: "Name", Width: controls.Px(80), Value: func(row int) string {
				return fmt.Sprintf("User %d", row)
			}},
			controls.Column{Title: "Email", Width: controls.Star(1), Value: func(row int) string {
				return fmt.Sprintf("u%d@x.io", row)
			}},
			controls.Column{Title: "Age", Width: controls.Px(60), Value: func(row int) string {
				return fmt.Sprintf("%d", 20+row)
			}},
		)
		dg.SetRowCount(20)
		dg.SetWidth(280)
		dg.SetHeight(160)
		dg.SetSelectedIndex(2)

		root := controls.NewCanvas().Add(dg, 10, 10)

		frame := render.Rect{X: 0, Y: 0, W: 300, H: 180}
		r.FillRect(frame, th.Color.ButtonFace)

		core.MeasureWidget(root, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(root, frame)
		core.RenderWidget(root, r)
	})
}

// TestTitleBar is the Phase 8 Task 4 golden (v0.2 classic restyle): a
// TitleBar reading "fluo" with a horizontal gradient caption
// (CaptionFrom->CaptionTo) and its close caption button hovered — in the
// classic look this is a plain gray raised caption button (the old
// CloseButtonHover red was dropped), with a WindowText X glyph. Inside a
// 300x40 frame; the bar itself is exactly titleBarHeight (32) tall,
// vertically centered in the taller frame (4px of window background showing
// above and below it).
func TestTitleBar(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "titlebar", 300, 40, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		tb := controls.NewTitleBar(face, "fluo")

		router := input.NewRouter()
		router.SetRoot(tb)

		frame := render.Rect{X: 0, Y: 0, W: 300, H: 40}
		r.FillRect(frame, th.Color.ButtonFace)

		bounds := render.Rect{X: 0, Y: (frame.H - 32) / 2, W: 300, H: 32}
		core.MeasureWidget(tb, render.Size{W: bounds.W, H: bounds.H})
		core.ArrangeWidget(tb, bounds)

		// Hover the close button: a point 10px in from the bar's right edge,
		// vertically centered — comfortably inside the rightmost (close)
		// caption button regardless of its exact measured width.
		barBounds := core.BoundsOf(tb)
		hoverPoint := render.Point{X: barBounds.Right() - 10, Y: barBounds.Y + barBounds.H/2}
		router.PointerMove(hoverPoint, 0)

		core.RenderWidget(tb, r)
	})
}

// TestAcrylic is the Phase 8 Task 5 golden: an AcrylicSurface panel (radius
// 8, 140x80) laid over three colored vertical stripes that together span
// the whole 200x120 frame, so the panel straddles both stripe boundaries —
// proving the backdrop shows through blurred (colors softened/mixed across
// the boundaries under the panel) rather than a flat opaque tint. A small
// white Fixed swatch is nested inside the surface to prove children still
// render on top of the acrylic background. See DrawBackdropBlur in
// render/gl/blur.go for which path shipped (real snapshot+blur, not a tint
// degrade).
func TestAcrylic(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	testFrame(t, "acrylic", 200, 120, func(r *glr.Renderer) {
		stripes := controls.NewCanvas().
			Add(controls.NewFixed(67, 120, render.RGB(0, 120, 215)), 0, 0).
			Add(controls.NewFixed(67, 120, render.RGB(255, 185, 0)), 67, 0).
			Add(controls.NewFixed(66, 120, render.RGB(16, 124, 16)), 133, 0)

		swatch := controls.NewFixed(30, 14, render.RGB(255, 255, 255))
		swatch.SetAlign(core.Center, core.Center)

		panel := controls.NewAcrylicSurface().
			SetRadius(8).
			SetChild(swatch)
		panel.SetWidth(140)
		panel.SetHeight(80)

		root := controls.NewCanvas().
			Add(stripes, 0, 0).
			Add(panel, 30, 20)

		frame := render.Rect{X: 0, Y: 0, W: 200, H: 120}

		core.MeasureWidget(root, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(root, frame)
		core.RenderWidget(root, r)
	})
}

func TestDialog(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "dialog", 280, 180, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		// Content behind the scrim, so the dim is visible in the golden.
		content := controls.NewCanvas().
			Add(controls.NewFixed(120, 80, render.RGB(0, 120, 215)), 10, 10).
			Add(controls.NewFixed(100, 60, render.RGB(16, 124, 16)), 150, 100)

		host := controls.NewOverlayHost()
		router := input.NewRouter()
		host.SetRouter(router)
		host.SetContent(content)
		router.SetRoot(host)

		frame := render.Rect{X: 0, Y: 0, W: 280, H: 180}
		r.FillRect(frame, th.Color.ButtonFace)

		// First layout pass: gives the host (and its content) real arranged
		// bounds, so ShowDialog's anchor (core.BoundsOf(host)) is correct.
		core.MeasureWidget(host, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(host, frame)

		controls.ShowDialog(host, face, controls.DialogSpec{
			Title:     "Delete file?",
			Body:      "This cannot be undone.",
			Secondary: "Cancel",
			Primary:   "Delete",
		})

		// Second layout pass: arranges the now-open dialog's scrim/card/rows.
		core.MeasureWidget(host, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(host, frame)

		core.RenderWidget(host, r)
	})
}

// TestToastStack is the golden for controls.OverlayHost.ShowToast: two
// stacked toasts ("Saved.", the newer one, and "Connecting...", the older
// one pushed up above it) in the host's bottom-right corner, each a small
// raised ButtonFace bevel card around its message — inside a 220x140 frame.
// Opened via the real ShowToast API (no timers.Queue wired, so both simply
// stay open with no auto-dismiss) rather than by reaching into any
// unexported controls internals.
func TestToastStack(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "toast", 220, 140, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		host := controls.NewOverlayHost()
		host.SetContent(controls.NewFixed(220, 140, th.Color.ButtonFace))

		frame := render.Rect{X: 0, Y: 0, W: 220, H: 140}
		r.FillRect(frame, th.Color.ButtonFace)

		// First layout pass: gives the host real arranged bounds, so
		// arrangeToasts has a real corner to stack against.
		core.MeasureWidget(host, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(host, frame)

		host.ShowToast(controls.ToastSpec{Face: face, Message: "Connecting..."})
		host.ShowToast(controls.ToastSpec{Face: face, Message: "Saved."})

		// Second layout pass: stacks both now-open toasts.
		core.MeasureWidget(host, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(host, frame)

		core.RenderWidget(host, r)
	})
}

// TestToastSeverityStack is the golden for the per-severity accent stripe
// (see controls.Severity and Toast.Render): all four kinds stacked in the
// host's bottom-right corner, newest ("Something failed.", SeverityError) at
// the bottom, oldest ("Heads up.", SeverityInfo) pushed to the top — so
// every accent color is visible in one frame, side by side with the plain
// SeverityInfo card that carries no stripe at all.
func TestToastSeverityStack(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "toast_severity", 220, 260, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil {
			t.Fatal(err)
		}
		face := text.NewFace(f, th.Type.BodySize)

		host := controls.NewOverlayHost()
		host.SetContent(controls.NewFixed(220, 260, th.Color.ButtonFace))

		frame := render.Rect{X: 0, Y: 0, W: 220, H: 260}
		r.FillRect(frame, th.Color.ButtonFace)

		// First layout pass: gives the host real arranged bounds, so
		// arrangeToasts has a real corner to stack against.
		core.MeasureWidget(host, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(host, frame)

		host.ShowToast(controls.ToastSpec{Face: face, Message: "Heads up.", Severity: controls.SeverityInfo})
		host.ShowToast(controls.ToastSpec{Face: face, Message: "Saved.", Severity: controls.SeveritySuccess})
		host.ShowToast(controls.ToastSpec{Face: face, Message: "Check this.", Severity: controls.SeverityWarning})
		host.ShowToast(controls.ToastSpec{Face: face, Message: "Something failed.", Severity: controls.SeverityError})

		// Second layout pass: stacks all four now-open toasts.
		core.MeasureWidget(host, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(host, frame)

		core.RenderWidget(host, r)
	})
}

// TestSplitPanel is the golden for controls.SplitPanel: a Horizontal panel
// (First left, Second right) at a 0.4 split ratio, First filled with
// th.Color.ButtonFace and Second with th.Color.Highlight so the two panes read
// as visibly distinct blocks with the raised divider bevel (drawRaised, via
// bevel.go) running between them.
func TestSplitPanel(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "splitpanel", 200, 120, func(r *glr.Renderer) {
		first := controls.NewBorder().SetBackground(th.Color.ButtonFace)
		second := controls.NewBorder().SetBackground(th.Color.Highlight)

		sp := controls.NewSplitPanel(controls.Horizontal).SetFirst(first).SetSecond(second)
		sp.SetSplitRatio(0.4)

		core.MeasureWidget(sp, render.Size{W: 200, H: 120})
		core.ArrangeWidget(sp, render.Rect{X: 0, Y: 0, W: 200, H: 120})
		core.RenderWidget(sp, r)
	})
}
