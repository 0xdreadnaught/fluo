// Command fluo-gallery is the widget gallery: it grows a page per control as
// phases land. Phase 3: interactive swatches (pointer/focus/cursor) plus a
// ScrollViewer over a taller-than-viewport content stack. Phase 4: the whole
// tree is built from theme.Active()'s tokens (buildUI), plus a live T-key
// toggle between Fluent Light and Dark. Phase 5: a Controls section at the
// top of the scroll content exercises every core control built this phase
// (Button/ToggleButton/CheckBox/RadioButton+Group/ToggleSwitch/TextBox/
// Slider/ProgressBar/ComboBox/ToolTipArea), and the root becomes an
// OverlayHost (controls.NewOverlayHost) so ComboBox's popup and
// ToolTipArea's tip have somewhere to render above the rest of the tree.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/0xdreadnaught/fluo/app"
	"github.com/0xdreadnaught/fluo/controls"
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
	"github.com/0xdreadnaught/fluo/theme"
	"github.com/0xdreadnaught/fluo/timers"

	"golang.org/x/image/font/gofont/goregular"
)

// swatch is a color block that reacts to pointer hover/press: it embeds
// core.Element for layout, implements input.PointerHandler to track
// hover/selected state, input.Focusable so it participates in tab order and
// press-to-focus, and input.CursorShaper to show a hand cursor. It exists
// here (rather than in package controls) as the consumer-side example of
// wiring a widget up to fluo's event API. The color it fills with is DATA
// (one of the six swatch-palette samples, not a theme token); the ring and
// hover-stroke colors it draws ARE tokens, captured from theme.Active() at
// construction by buildUI so a theme toggle recolors them on rebuild.
type swatch struct {
	core.Element

	w, h       float32
	color      render.Color
	ringColor  render.Color // selected-state stroke (th.Color.Accent)
	hoverColor render.Color // hover-state stroke (th.Color.TextPrimary)
	hover      bool
	selected   bool
}

// newSwatch returns a swatch that measures to (w, h), fills with c, and
// strokes its selected/hover rings with ring/hoverColor.
func newSwatch(w, h float32, c, ring, hoverColor render.Color) *swatch {
	return &swatch{w: w, h: h, color: c, ringColor: ring, hoverColor: hoverColor}
}

// MeasureContent returns the fixed (w, h) size regardless of the space
// available.
func (s *swatch) MeasureContent(available render.Size) render.Size {
	return render.Size{W: s.w, H: s.h}
}

// Render fills the color block, then draws a 2px ring stroke at the
// swatch's own bounds when selected, and a 2px stroke inset 3px from those
// bounds when hovered — the inset keeps the two strokes from overdrawing
// each other, so a swatch that is both hovered and selected shows both
// rings distinctly instead of one overpainting the other.
func (s *swatch) Render(r render.Renderer) {
	b := s.Bounds()
	if s.color.A != 0 {
		r.FillRect(b, s.color)
	}
	if s.selected {
		r.StrokeRoundedRect(b, 0, 2, s.ringColor)
	}
	if s.hover {
		r.StrokeRoundedRect(b.Inset(render.Uniform(3)), 0, 2, s.hoverColor)
	}
}

// OnPointer implements input.PointerHandler: Enter/Leave toggle hover,
// Press toggles selected and is marked handled.
func (s *swatch) OnPointer(e *input.PointerEvent) {
	switch e.Action {
	case input.Enter:
		s.hover = true
	case input.Leave:
		s.hover = false
	case input.Press:
		s.selected = !s.selected
		e.Handled = true
	}
}

// AcceptsFocus implements input.Focusable: swatches join tab order and can
// be press-to-focused.
func (s *swatch) AcceptsFocus() bool { return true }

// Cursor implements input.CursorShaper: a hand cursor over any swatch.
func (s *swatch) Cursor() input.Cursor { return input.CursorHand }

// galleryRoot is a trivial single-child wrapper (same shape as
// controls.Border) around the real DockPanel tree, whose only job is
// implementing input.KeyHandler for the theme-toggle shortcut. It must be an
// actual node in the tree recorded as its child's ancestor (via
// core.SetParent in newGalleryRoot) — not a promoted-method embed of
// *DockPanel — so that input.Router's key-bubbling (which walks
// core.ParentOf from whichever widget holds focus) reaches OnKey whenever
// something under the dock is focused.
//
// Since Phase 5 Task 9, galleryRoot is no longer the router's own root: it
// sits as buildUI's OverlayHost's content, one level below the host (see
// buildUI). input.Router.dispatchKey delivers to the focused widget's own
// core.ParentOf chain (which still includes galleryRoot, and above it the
// host) whenever something is focused, but falls back to the bare router
// root ALONE — the OverlayHost, which implements no KeyHandler — when
// nothing is (e.g. immediately after SetRoot, which always clears focus).
// So the T-key toggle fires reliably once any focusable control has been
// interacted with, but not from a pristine, nothing-yet-focused launch — a
// documented v0 tradeoff of routing ComboBox/ToolTipArea popups through a
// single OverlayHost root, not an oversight.
type galleryRoot struct {
	core.Element

	child  core.Widget
	toggle func()
}

// newGalleryRoot wraps child, re-parenting it, and calls toggle on KeyT.
func newGalleryRoot(child core.Widget, toggle func()) *galleryRoot {
	g := &galleryRoot{child: child, toggle: toggle}
	core.SetParent(child, g)
	return g
}

func (g *galleryRoot) MeasureContent(available render.Size) render.Size {
	core.MeasureWidget(g.child, available)
	return core.DesiredSizeOf(g.child)
}

func (g *galleryRoot) ArrangeContent(bounds render.Rect) {
	core.ArrangeWidget(g.child, bounds)
}

func (g *galleryRoot) Children() []core.Widget { return []core.Widget{g.child} }

// OnKey implements input.KeyHandler: an unmodified KeyT press-down toggles
// the active theme. The actual SetActive/rebuild/SetRoot happens in main's
// frame callback on the NEXT frame (a pending flag set here), not inline —
// see toggle's construction in main for why.
func (g *galleryRoot) OnKey(e *input.KeyEvent) {
	if e.Action == input.Press && e.Key == input.KeyT && e.Mods == 0 {
		g.toggle()
		e.Handled = true
	}
}

// swatchPalette is DATA: sample colors shown in the gallery's color-swatch
// row, not theme chrome, so (unlike everything else buildUI draws) these
// stay literal across a theme toggle.
var swatchPalette = []render.Color{
	render.RGB(0, 120, 215), render.RGB(255, 185, 0), render.RGB(16, 124, 16),
	render.RGB(232, 17, 35), render.RGB(136, 23, 152), render.RGB(0, 153, 188),
}

// swatchColorNames names swatchPalette's entries in the same order, reused
// verbatim as the Controls section's ComboBox items — the gallery's one
// piece of "data that appears in two widgets", kept as a single slice so the
// two can never drift out of sync.
var swatchColorNames = []string{"Blue", "Yellow", "Green", "Red", "Purple", "Teal"}

// buildControlsSection builds the Phase 5 Controls section: one HStack per
// control family (Button/ToggleButton, CheckBox/RadioButton+Group/
// ToggleSwitch, TextBox/ComboBox, Slider/ProgressBar), stacked vertically
// with th's PaddingM gap, all styled from th like everything else buildUI
// draws. tq (may be nil, e.g. before the app's first frame hands buildUI a
// real timers.Queue) is threaded into the TextBox's caret blink and the
// accent button's ToolTipArea dwell timer via their respective SetTimers —
// nil disables the timing behavior but leaves both controls otherwise
// functional (solid caret, immediate-show tooltip), matching each control's
// own documented no-queue convention. counter/onToggle wire up the demo
// button's click count and the theme-toggle shortcut respectively (onToggle
// is consumed by galleryRoot, not here).
func buildControlsSection(th *theme.Theme, body *text.Face, counter *int, tq *timers.Queue) core.Widget {
	// Row 1: Button (with click counter), accent Button (tooltipped),
	// ToggleButton.
	counterLabel := controls.NewTextBlock(body, fmt.Sprintf("Clicked %d times", *counter)).
		SetColor(th.Color.TextSecondary)
	demoButton := controls.NewButton(body, "Click me").OnClick(func() {
		*counter++
		counterLabel.SetText(fmt.Sprintf("Clicked %d times", *counter))
	})
	accentButton := controls.NewButton(body, "Accent").SetAccent(true)
	accentTip := controls.NewToolTipArea(accentButton, body, "Accent button").SetTimers(tq)
	toggleButton := controls.NewToggleButton(body, "Toggle")

	row1 := controls.NewStackPanel(controls.Horizontal).SetGap(th.Metric.PaddingM).
		Add(demoButton, counterLabel, accentTip, toggleButton)

	// Row 2: CheckBox, RadioButton "A"+"B" in a RadioGroup, ToggleSwitch.
	checkBox := controls.NewCheckBox(body, "Enable")

	radioA := controls.NewRadioButton(body, "A")
	radioB := controls.NewRadioButton(body, "B")
	controls.NewRadioGroup().Add(radioA).Add(radioB)

	toggleSwitch := controls.NewToggleSwitch()

	row2 := controls.NewStackPanel(controls.Horizontal).SetGap(th.Metric.PaddingM).
		Add(checkBox, radioA, radioB, toggleSwitch)

	// Row 3: TextBox (placeholder + caret blink), ComboBox (swatch color
	// names).
	textBox := controls.NewTextBox(body).SetPlaceholder("Type here…").SetTimers(tq)
	textBox.SetWidth(200)
	comboBox := controls.NewComboBox(body).SetItems(swatchColorNames)

	row3 := controls.NewStackPanel(controls.Horizontal).SetGap(th.Metric.PaddingM).
		Add(textBox, comboBox)

	// Row 4: Slider (wired to drive a ProgressBar's value).
	progressBar := controls.NewProgressBar()
	slider := controls.NewSlider()
	slider.OnChanged(func(v float32) { progressBar.SetValue(v) })
	slider.SetValue(0.3) // fires OnChanged above, seeding progressBar to match

	row4 := controls.NewStackPanel(controls.Horizontal).SetGap(th.Metric.PaddingM).
		Add(slider, progressBar)

	return controls.NewStackPanel(controls.Vertical).SetGap(th.Metric.PaddingM).
		Add(row1, row2, row3, row4)
}

// buildUI builds the gallery's entire widget tree from th's tokens — colors,
// paddings, radii, and type sizes all come from th, so the whole tree is a
// pure function of the active theme (see FLUO_THEME and the T-key toggle in
// main: re-theming means calling buildUI again and swapping roots, never
// mutating an existing tree in place). counter/onToggle wire up the demo
// button's click count and the theme-toggle shortcut respectively; tq is
// forwarded to buildControlsSection (see its doc comment).
//
// Since Phase 5 Task 9, the returned root is a *controls.OverlayHost (rather
// than *galleryRoot directly): ComboBox's popup and ToolTipArea's tip (both
// used in the Controls section) need an OverlayHost ancestor to render into
// (OverlayHostFor walks up looking for one), so the host must sit above
// everything that uses either control — see main's SetRouter wiring, which
// an OverlayHost needs to drive its light-dismiss capture.
func buildUI(th *theme.Theme, font *text.Font, counter *int, onToggle func(), tq *timers.Queue) *controls.OverlayHost {
	title := text.NewFace(font, th.Type.SubtitleSize)
	body := text.NewFace(font, th.Type.BodySize)

	swatches := controls.NewWrapPanel().SetGap(th.Metric.PaddingM)
	for _, c := range swatchPalette {
		swatches.Add(newSwatch(72, 48, c, th.Color.Accent, th.Color.TextPrimary))
	}

	controlsSection := buildControlsSection(th, body, counter, tq)

	content := controls.NewStackPanel(controls.Vertical).SetGap(th.Metric.PaddingM).
		Add(controlsSection, swatches)
	for i := 1; i <= 20; i++ {
		content.Add(controls.NewTextBlock(body, fmt.Sprintf("Row %02d", i)).SetColor(th.Color.TextSecondary))
	}

	scroll := controls.NewScrollViewer().SetChild(content)

	nav := controls.NewStackPanel(controls.Vertical).SetGap(th.Metric.PaddingS).Add(
		controls.NewTextBlock(body, "Layout").SetColor(th.Color.TextPrimary),
		controls.NewTextBlock(body, "Panels").SetColor(th.Color.TextSecondary),
		controls.NewTextBlock(body, "Text").SetColor(th.Color.TextSecondary),
	)

	dock := controls.NewDockPanel().
		Add(controls.NewBorder().
			SetBackground(th.Color.LayerBackground).
			SetPadding(render.Thickness{
				Left: th.Metric.PaddingL, Right: th.Metric.PaddingL,
				Top: th.Metric.PaddingM, Bottom: th.Metric.PaddingM,
			}).
			SetChild(controls.NewTextBlock(title, "fluo gallery").SetColor(th.Color.TextPrimary)),
			controls.DockTop).
		Add(controls.NewBorder().
			SetBackground(th.Color.LayerBackground).
			SetPadding(render.Uniform(th.Metric.PaddingM)).
			SetChild(nav),
			controls.DockLeft).
		Add(controls.NewBorder().
			SetBackground(th.Color.WindowBackground).
			SetPadding(render.Uniform(th.Metric.PaddingL)).
			SetChild(scroll),
			controls.DockLeft) // last child fills

	host := controls.NewOverlayHost()
	host.SetContent(newGalleryRoot(dock, onToggle))
	return host
}

func main() {
	f, err := text.Load(goregular.TTF)
	if err != nil {
		log.Fatal(err)
	}

	// FLUO_THEME is a dev convenience for manual light/dark verification
	// (e.g. `FLUO_THEME=light go run ./cmd/fluo-gallery`), not a supported
	// runtime API — the real toggle is the T key.
	initial := theme.FluentDark()
	if os.Getenv("FLUO_THEME") == "light" {
		initial = theme.FluentLight()
	}
	theme.SetActive(initial)

	var counter int
	var togglePending bool
	// timerQueue starts nil (no app.Ctx exists yet to supply one) and is
	// filled in from the first frame's c.Timers below — see buildControlsSection's
	// doc comment for why a nil queue is a safe, if less lively, fallback.
	var timerQueue *timers.Queue
	build := func() *controls.OverlayHost {
		return buildUI(theme.Active(), f, &counter, func() { togglePending = true }, timerQueue)
	}

	var root *controls.OverlayHost
	var lastSize render.Size
	rootSet := false

	err = app.Run(app.Config{Title: "fluo gallery", Width: 640, Height: 420}, func(c *app.Ctx) {
		if !rootSet {
			timerQueue = c.Timers
			root = build()
			c.Input.SetRoot(root)
			root.SetRouter(c.Input) // OverlayHost needs the router for light-dismiss capture
			rootSet = true
		}
		if togglePending {
			togglePending = false
			next := theme.FluentLight()
			if theme.Active().Name == "fluent-light" {
				next = theme.FluentDark()
			}
			theme.SetActive(next)
			root = build()
			c.Input.SetRoot(root) // SetRoot resets hover/capture/focus by design
			root.SetRouter(c.Input)
			lastSize = render.Size{}
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
