package gl_test

import (
	"fmt"
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

// TestButtons is the Phase 5 Task 3 golden: three controls.Button instances
// side by side in a StackPanel — default, accent, and disabled — proving
// their token-driven fills/strokes/label colors render correctly together.
func TestButtons(t *testing.T) {
	theme.SetActive(theme.FluentLight())
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
		r.FillRect(frame, th.Color.WindowBackground)

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
	theme.SetActive(theme.FluentLight())
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
		r.FillRect(frame, th.Color.WindowBackground)

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
	theme.SetActive(theme.FluentLight())
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
		r.FillRect(frame, th.Color.WindowBackground)

		core.MeasureWidget(tb, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(tb, frame)
		core.RenderWidget(tb, r)
	})
}

// TestSliderProgress is the Phase 5 Task 7 golden: a Slider at 0.6 (over the
// default [0,1] range) stacked above a ProgressBar at 0.3, in a vertical
// StackPanel gapped by PaddingM, filling a 200x60 frame.
func TestSliderProgress(t *testing.T) {
	theme.SetActive(theme.FluentLight())
	defer theme.SetActive(nil)
	th := theme.Active()

	testFrame(t, "slider_progress", 200, 60, func(r *glr.Renderer) {
		slider := controls.NewSlider().SetValue(0.6)
		progress := controls.NewProgressBar().SetValue(0.3)

		stack := controls.NewStackPanel(controls.Vertical).SetGap(th.Metric.PaddingM).
			Add(slider, progress)

		frame := render.Rect{X: 0, Y: 0, W: 200, H: 60}
		r.FillRect(frame, th.Color.WindowBackground)

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

// TestComboOpen is the Phase 5 Task 8 golden: an open ComboBox — 3 items
// ("Red", "Green", "Blue"), "Green" (index 1) selected-highlighted — inside
// a 220x160 frame. The popup is opened via the SAME router-driven path a
// real app would use (focus the field, then KeyDown(Enter), landing on
// ComboBox.OnKey's Space/Enter/Down branch) rather than by reaching into the
// unexported openPopup directly, so the golden also exercises the whole
// input-to-popup path end to end, not just the popup's own visuals.
func TestComboOpen(t *testing.T) {
	theme.SetActive(theme.FluentLight())
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
		r.FillRect(frame, th.Color.WindowBackground)

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
// SelectionBackground/SelectionForeground row highlight — scrolled to the
// top (via the Task 3 ScrollTo addition, mirroring ScrollViewer.ScrollTo)
// so the first several rows are visible with the scroll thumb showing
// (content taller than the 140px-high viewport), inside a 200x160 frame.
// Sized and positioned exactly like TestScrollClipRender's ScrollViewer
// golden: explicit SetWidth/SetHeight on the control, placed via Canvas at
// a fixed (10,10) offset rather than measured-and-centered.
func TestListView(t *testing.T) {
	theme.SetActive(theme.FluentLight())
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
		r.FillRect(frame, th.Color.WindowBackground)

		core.MeasureWidget(root, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(root, frame)
		core.RenderWidget(root, r)
	})
}

// TestTreeExpander is the Phase 7 Task 4 golden: a TreeView (left) showing
// two roots — "src" (expanded, with children "core", "controls" (the
// current selection, showing the SelectionBackground/SelectionForeground
// row highlight), and "render") and "docs" (collapsed, hiding its own single
// child "readme.md" — proving the '>' collapsed chevron alongside "src"'s
// 'v' expanded one) — beside an expanded Expander (right) titled "Details"
// containing a TextBlock reading "Hello", inside a 260x180 frame.
func TestTreeExpander(t *testing.T) {
	theme.SetActive(theme.FluentLight())
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
		r.FillRect(frame, th.Color.WindowBackground)

		core.MeasureWidget(root, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(root, frame)
		core.RenderWidget(root, r)
	})
}

// TestTabs is the Phase 7 Task 5 golden: a TabControl with 3 tabs ("One",
// "Two", "Three"), the second ("Two") selected — showing the Accent
// underline beneath its header cell and its TextBlock content ("Tab two
// content") below the strip — inside a 240x120 frame. "One" and "Three"'s
// own content stays attached (per TabControl's normative "hidden tabs
// remain in the tree" rule) but is invisible, contributing nothing to this
// frame.
func TestTabs(t *testing.T) {
	theme.SetActive(theme.FluentLight())
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
		r.FillRect(frame, th.Color.WindowBackground)

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
	theme.SetActive(theme.FluentLight())
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
		r.FillRect(frame, th.Color.WindowBackground)

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

// TestDialog is the Phase 7 Task 7 golden: a ShowDialog modal — scrim
// dimming two colored Fixed blocks sitting behind it in the host's own
// content, a centered shadowed Card with "Delete file?" / "This cannot be
// undone.", and a right-aligned "Cancel" (default)/"Delete" (accent) button
// row — inside a 280x180 frame.
// TestDataGrid is the Phase 7 Task 8 golden: a DataGrid with 3 columns
// ("Name" Px 80, "Email" Star, "Age" Px 60) and 20 rows ("User NN",
// "uNN@x.io", 20+NN), row 2 selected — showing the SelectionBackground/
// SelectionForeground row band — scrolled to the top (offset 0, the
// zero-value default; no explicit scroll needed) so the header (fixed,
// LayerBackground fill + TextSecondary titles + its own bottom border) sits
// above the first several body rows, each with its own 1px ControlStroke
// grid line, with the scroll thumb showing (20 rows taller than the visible
// body), inside a 300x180 frame. Sized and positioned exactly like
// TestListView's own ScrollTo(0)-pinned golden: explicit SetWidth/SetHeight,
// placed via Canvas at a fixed (10,10) offset.
func TestDataGrid(t *testing.T) {
	theme.SetActive(theme.FluentLight())
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
		r.FillRect(frame, th.Color.WindowBackground)

		core.MeasureWidget(root, render.Size{W: frame.W, H: frame.H})
		core.ArrangeWidget(root, frame)
		core.RenderWidget(root, r)
	})
}

func TestDialog(t *testing.T) {
	theme.SetActive(theme.FluentLight())
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
		r.FillRect(frame, th.Color.WindowBackground)

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
