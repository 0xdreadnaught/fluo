// Command fluo-gallery is the widget gallery: it grows a page per control as
// phases land. Phase 3: interactive swatches (pointer/focus/cursor) plus a
// ScrollViewer over a taller-than-viewport content stack. Phase 4: the whole
// tree is built from theme.Active()'s tokens (buildUI), plus a live T-key
// toggle between Fluent Light and Dark, and a Fluent button preview that
// exercises hover/press-capture state, previewing Phase 5's Button.
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

// button is a gallery-local Fluent button preview: a Border-like composite
// that fills with Accent/AccentHover/AccentPressed by pointer state (hover
// via Enter/Leave, pressed via Press/Release with capture — release-inside
// fires onClick), token-styled from theme.Active() at construction like
// every other control here (rebuild to re-theme). This is the LIVE Phase 4
// milestone: it previews the press-capture-release-inside semantics that
// Phase 5's Button/Clickable helper will formalize.
type button struct {
	core.Element

	label core.Widget

	fill, hoverFill, pressFill render.Color
	radius                     float32
	padding                    render.Thickness

	hover, pressed bool

	onClick func()
}

// newButton returns a button styled from th, with a TextBlock(face, text)
// label colored th.Color.AccentText, that calls onClick on release-inside.
func newButton(th *theme.Theme, face *text.Face, label string, onClick func()) *button {
	b := &button{
		fill:      th.Color.Accent,
		hoverFill: th.Color.AccentHover,
		pressFill: th.Color.AccentPressed,
		radius:    th.Metric.ControlCornerRadius,
		padding: render.Thickness{
			Left: th.Metric.PaddingL, Right: th.Metric.PaddingL,
			Top: th.Metric.PaddingM, Bottom: th.Metric.PaddingM,
		},
		onClick: onClick,
	}
	lbl := controls.NewTextBlock(face, label).SetColor(th.Color.AccentText)
	b.label = lbl
	core.SetParent(lbl, b)
	return b
}

// MeasureContent measures the label reduced by padding, then adds the
// padding back — the same chrome-then-content shape as controls.Border.
func (b *button) MeasureContent(available render.Size) render.Size {
	availW := available.W - b.padding.Left - b.padding.Right
	if availW < 0 {
		availW = 0
	}
	availH := available.H - b.padding.Top - b.padding.Bottom
	if availH < 0 {
		availH = 0
	}
	core.MeasureWidget(b.label, render.Size{W: availW, H: availH})
	d := core.DesiredSizeOf(b.label)
	return render.Size{
		W: d.W + b.padding.Left + b.padding.Right,
		H: d.H + b.padding.Top + b.padding.Bottom,
	}
}

// ArrangeContent arranges the label within bounds inset by padding.
func (b *button) ArrangeContent(bounds render.Rect) {
	inner := bounds.Inset(b.padding)
	if inner.W < 0 {
		inner.W = 0
	}
	if inner.H < 0 {
		inner.H = 0
	}
	core.ArrangeWidget(b.label, inner)
}

// Render fills the rounded background with the color for the current state:
// pressed wins over hover, hover wins over the resting fill.
func (b *button) Render(r render.Renderer) {
	fill := b.fill
	switch {
	case b.pressed:
		fill = b.pressFill
	case b.hover:
		fill = b.hoverFill
	}
	r.FillRoundedRect(b.Bounds(), b.radius, fill)
}

// Children returns the single label child.
func (b *button) Children() []core.Widget { return []core.Widget{b.label} }

// AcceptsFocus implements input.Focusable, so the button joins tab order.
func (b *button) AcceptsFocus() bool { return true }

// Cursor implements input.CursorShaper: a hand cursor over the button.
func (b *button) Cursor() input.Cursor { return input.CursorHand }

// OnPointer implements input.PointerHandler: Press captures the pointer and
// shows the pressed fill; Move (while captured) tracks whether the pointer
// is still over the button for the hover fill; Release (while captured)
// releases the capture and fires onClick only if the release lands inside
// the button's own bounds (a drag-away-and-release cancels the click).
func (b *button) OnPointer(e *input.PointerEvent) {
	switch e.Action {
	case input.Enter:
		b.hover = true
	case input.Leave:
		b.hover = false
	case input.Press:
		b.pressed = true
		e.Router.Capture(b)
		e.Handled = true
	case input.Move:
		if e.Router.Captured() == b {
			b.hover = b.Bounds().Contains(e.Pos)
			e.Handled = true
		}
	case input.Release:
		if e.Router.Captured() == b {
			e.Router.Release()
			inside := b.Bounds().Contains(e.Pos)
			b.pressed = false
			b.hover = inside
			if inside && b.onClick != nil {
				b.onClick()
			}
			e.Handled = true
		}
	}
}

// galleryRoot is the gallery's root widget: a trivial single-child wrapper
// (same shape as controls.Border) around the real DockPanel tree, whose only
// job is implementing input.KeyHandler for the theme-toggle shortcut. It
// must be the actual root object recorded as children's ancestor (via
// core.SetParent in newGalleryRoot) — not a promoted-method embed of
// *DockPanel — so that input.Router's key-bubbling (which walks
// core.ParentOf from whichever widget holds focus) reaches OnKey no matter
// what has focus, not only when nothing does.
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
	if e.Action == input.Press && e.Key == input.KeyT {
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

// buildUI builds the gallery's entire widget tree from th's tokens — colors,
// paddings, radii, and type sizes all come from th, so the whole tree is a
// pure function of the active theme (see FLUO_THEME and the T-key toggle in
// main: re-theming means calling buildUI again and swapping roots, never
// mutating an existing tree in place). counter/onToggle wire up the demo
// button's click count and the theme-toggle shortcut respectively.
func buildUI(th *theme.Theme, font *text.Font, counter *int, onToggle func()) *galleryRoot {
	title := text.NewFace(font, th.Type.SubtitleSize)
	body := text.NewFace(font, th.Type.BodySize)

	swatches := controls.NewWrapPanel().SetGap(th.Metric.PaddingM)
	for _, c := range swatchPalette {
		swatches.Add(newSwatch(72, 48, c, th.Color.Accent, th.Color.TextPrimary))
	}

	counterLabel := controls.NewTextBlock(body, fmt.Sprintf("Clicked %d times", *counter)).
		SetColor(th.Color.TextSecondary)
	demoButton := newButton(th, body, "Click me", func() {
		*counter++
		counterLabel.SetText(fmt.Sprintf("Clicked %d times", *counter))
	})
	demoRow := controls.NewStackPanel(controls.Horizontal).
		SetGap(th.Metric.PaddingM).
		Add(demoButton, counterLabel)

	content := controls.NewStackPanel(controls.Vertical).SetGap(th.Metric.PaddingM).
		Add(demoRow, swatches)
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

	return newGalleryRoot(dock, onToggle)
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
	build := func() *galleryRoot {
		return buildUI(theme.Active(), f, &counter, func() { togglePending = true })
	}

	root := build()
	var lastSize render.Size
	rootSet := false

	err = app.Run(app.Config{Title: "fluo gallery", Width: 640, Height: 420}, func(c *app.Ctx) {
		if !rootSet {
			c.Input.SetRoot(root)
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
