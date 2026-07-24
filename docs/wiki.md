# fluo Wiki

fluo is a retained-mode GUI library for Go. It renders through its own OpenGL
backend behind a small `Renderer` interface, lays out with a WPF-style two-pass
Measure/Arrange model, and ships a classic Windows-2000 four-tone bevel look
driven entirely by theme tokens.

**Module:** `github.com/0xdreadnaught/fluo`
**Requires:** Go 1.23, a working cgo toolchain, and OpenGL 3.3 (via go-gl/glfw).

```
go get github.com/0xdreadnaught/fluo
```

## Getting started

A complete program: a button increments a reactive property, a label mirrors it.

```go
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

	// The model: a reactive value owned by main, never recreated.
	count := new(core.Property[int])

	label := controls.NewTextBlock(body, "").SetColor(th.Color.WindowText)
	cancel := bind.OneWay(count, func(n int) {
		label.SetText(fmt.Sprintf("Clicked %d times", n))
	})
	defer cancel()

	button := controls.NewButton(body, "Click me").OnClick(func() {
		count.Set(count.Get() + 1)
	})

	root := controls.NewBorder().
		SetPadding(render.Uniform(th.Metric.PaddingL)).
		SetChild(controls.NewStackPanel(controls.Vertical).
			SetGap(th.Metric.PaddingM).
			Add(label, button))

	var lastSize render.Size
	rootSet := false

	err = app.Run(app.Config{Title: "fluo counter", Width: 320, Height: 160}, func(c *app.Ctx) {
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
```

Runnable variants live in [`examples/`](../examples) (`counter`, `form`, `todo`)
and the full control showcase is [`cmd/fluo-gallery`](../cmd/fluo-gallery).

## Reference

| Page | Covers |
|---|---|
| [Core](wiki/core.md) | `Widget`, `Element`, the Measure/Arrange model, `Property[T]` |
| [Layout controls](wiki/controls-layout.md) | Border, TextBlock, Fixed, StackPanel, Grid, DockPanel, WrapPanel, Canvas |
| [Button controls](wiki/controls-buttons.md) | Button, ToggleButton, CheckBox, RadioButton, RadioGroup, ToggleSwitch |
| [Input controls](wiki/controls-input.md) | TextBox, Slider, ProgressBar, ComboBox |
| [Collections & scrolling](wiki/controls-collections.md) | ListView, TreeView, DataGrid, Expander, ScrollViewer |
| [Overlays & chrome](wiki/controls-overlays.md) | OverlayHost, menus, dialogs, tooltips, TabControl, TitleBar |
| [Theming](wiki/theming.md) | `Theme`, color/metric/type tokens, `Light`/`Dark`, custom themes |
| [Binding](wiki/binding.md) | one-way and two-way binders, observable `List[T]` |
| [Input routing](wiki/input.md) | `Router`, events, hit-testing, capture, focus |
| [Rendering & text](wiki/rendering.md) | the `Renderer` interface, `Font`/`Face` |
| [App host](wiki/app.md) | `app.Run`, `Config`, `Ctx`, `Surface`, animation, timers |

## Conventions

These contracts hold library-wide. Learning them once explains most of the API.

**Builder chaining.** Control setters return their receiver, so construction reads
as one expression: `controls.NewButton(body, "OK").SetAccent(true).OnClick(fn)`.
The exception is `core.Element`'s own setters (`SetMargin`, `SetAlign`,
`SetWidth`, …), which return nothing and must be called as statements.

**Silent setters.** Programmatic setters never fire change callbacks — only
user-driven changes do. `SetValue` on a slider updates it quietly; a user
dragging the thumb fires the change. This is what makes two-way binding
echo-safe, and it is the single most important convention in the library.

**Themes are captured at construction.** Every widget reads `theme.Active()` once
when it is built and copies out the tokens it needs. Changing the active theme
does not re-skin existing widgets — call `theme.SetActive`, then rebuild the
tree. See [Theming](wiki/theming.md).

**Binders return a cancel func.** Every binder hands back a `func()` that
unsubscribes. Models outlive views, so failing to cancel leaks the view. See
[Binding](wiki/binding.md).

**Logical pixels.** All layout and drawing coordinates are logical pixels.
Device-pixel conversion happens exactly once, on the GPU, from the scale passed
to `Renderer.Begin`. Application and control code never multiplies by scale.

**New behavior is opt-in.** Variants added over time — vertical sliders, solid
progress fills, pill/circle buttons, horizontal scrolling — default to the
original behavior and are enabled by an explicit setter.

## Project docs

- [README](../README.md) — overview and quick start
- [CHANGELOG](../CHANGELOG.md) — release history
- [ROADMAP](../ROADMAP.md) — planned work
