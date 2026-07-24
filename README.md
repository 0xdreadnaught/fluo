# fluo

fluo is a retained-mode, classic Windows-2000-styled GUI toolkit for OpenGL
apps in Go. It gives a Go program a themed widget tree — layout, input
routing, data binding, and a full classic-chrome control set (four-tone
raised/sunken bevels, square corners, gradient title bars) — over a thin
OpenGL 3.3 renderer, without pulling in a browser engine or a native OS
toolkit binding.

fluo is built bottom-up as a stack of small, independently testable
packages (see [Architecture](#architecture) below) rather than one large
framework package, and every control is driven from swappable design
tokens rather than hard-coded styling, so re-theming an app is a data
change, not a rewrite.

## Features

- A full classic-styled control set: `Border`, `TextBlock`, `StackPanel`,
  `Grid`, `DockPanel`, `WrapPanel`, `Canvas`, `ScrollViewer`, `Button`,
  `ToggleButton`, `CheckBox`, `RadioButton`/`RadioGroup`, `ToggleSwitch`,
  `TextBox`, `Slider`, `ProgressBar`, `ComboBox`, `ToolTipArea`,
  `ListView`, `TreeView`, `TabControl`, `Expander`, `MenuBar`, modal
  `Dialog`, `DataGrid`, and a custom `TitleBar` for undecorated windows —
  every raised/sunken surface drawn as an authentic Windows-2000 four-tone
  bevel (`ButtonHighlight`/`ButtonLight`/`ButtonShadow`/`ButtonDarkShadow`
  around a flat `ButtonFace`), square corners, no drop shadows or blur.
- Light/Dark theming: every control styles itself from `theme.Active()`'s
  color/metric/typography tokens; re-theming means swapping the active
  theme and rebuilding the tree. `theme.Light()` and `theme.Dark()` are the
  two bundled variants — see [Theming](#theming) below for building your own.
- Two-way and one-way data binding (package `bind`) between
  `core.Property[T]` values and controls, under a uniform silent-setter/
  `OnChanged` contract, plus observable-list collection binding.
- Virtualized `ListView`/`DataGrid` for large item sets.
- A translucent backdrop-blur surface (`controls.AcrylicSurface`) is still
  available as a control for apps that want it, though it's not part of the
  classic look and the gallery no longer uses it (see Theming below).

## Requirements

- Go 1.23
- A C compiler for cgo (`go-gl/glfw` and `go-gl/gl` are cgo bindings)
- An OpenGL 3.3 (core profile) capable driver

## Install

```sh
go get github.com/0xdreadnaught/fluo
```

## Quickstart

```go
package main

import (
	"log"

	"github.com/0xdreadnaught/fluo/app"
	"github.com/0xdreadnaught/fluo/controls"
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"

	"golang.org/x/image/font/gofont/goregular"
)

func main() {
	font, _ := text.Load(goregular.TTF)
	root := controls.NewTextBlock(text.NewFace(font, 16), "Hello, fluo!")

	log.Fatal(app.Run(app.Config{Title: "hello", Width: 320, Height: 200}, func(c *app.Ctx) {
		c.Input.SetRoot(root)
		core.MeasureWidget(root, c.Size)
		core.ArrangeWidget(root, render.Rect{W: c.Size.W, H: c.Size.H})
		core.RenderWidget(root, c.R)
	}))
}
```

See [`examples/counter`](examples/counter) for the same idea with a proper
one-time root-set guard and a data-bound label.

## Architecture

fluo is layered bottom to top; each layer only depends on the ones below it:

1. **`render`** — the `Renderer` interface and geometry primitives
   (`Color`/`Point`/`Size`/`Rect`); **`render/gl`** implements it on OpenGL 3.3.
2. **`text`** — font loading and SDF glyph rasterization on top of `render`.
3. **`core`** — the `Widget`/`Element` layout engine (Measure/Arrange/Render)
   and the reactive `Property[T]`.
4. **`input`** — hit-testing, event bubbling, capture, and focus over a
   `core.Widget` tree.
5. **`theme`** — the color/metric/typography token model (`theme.Light()`/
   `theme.Dark()`).
6. **`controls`** — the built-in widget set, styled from `theme` and wired
   to `input`.
7. **`bind`** — one-way/two-way binding between `core.Property[T]`/
   `bind.List[T]` and `controls`.
8. **`anim`** / **`timers`** — easing/`Tween` animation and the frame-tick
   timer service controls use for caret blink, tooltip dwell, and
   cross-fades.
9. **`app`** — `Run`/`Surface`, the glfw window host (or embeddable
   surface) that ties every layer above together into a running program.

`go doc ./<package>` on any of the above gives a one-paragraph summary of
its role and key types.

## Examples

- [`examples/counter`](examples/counter) — a `Button` incrementing a
  `Property[int]` shown in a `TextBlock` via `bind.OneWay`.
- [`examples/form`](examples/form) — a `TextBox`/`CheckBox`/`Slider` each
  two-way bound to a small model, echoed by a summary `TextBlock`.
- [`examples/todo`](examples/todo) — a `TextBox` + "Add" `Button` appending
  to a `bind.List[string]` rendered by a `ListView`, with a live item count.
- [`cmd/fluo-gallery`](cmd/fluo-gallery) — the full widget gallery: every
  control in the toolkit, composed together, with a live theme toggle
  (press **T**) and a page switcher.
- [`cmd/fluo-demo`](cmd/fluo-demo) — the original bare-primitives demo
  (rounded rects, stroke, shadow, SDF text) from before the control set
  existed.

Run any of them with `go run`, e.g. `go run ./examples/todo` or
`go run ./cmd/fluo-gallery`.

## Status

**v0.2 — classic-depth restyle complete.** The full layer stack above is
implemented, tested (headless layout/binding tests plus GL golden-image
tests, auto-skipped without a GPU), and live-verified: `fluo-gallery` opens
as an undecorated window with a custom `TitleBar` (drag-to-move, min/max/
close, gradient caption), a classic `ButtonFace` content pane, and animated
demo buttons, exercising every control in the toolkit together in the
Windows-2000 four-tone bevel look. `go vet`, `gofmt`, `go build`, and
`go test` (including the golden suite) are all clean. Publishing a tagged
release itself (creating the GitHub repo, pushing, tagging) is an operator
action — see
[`docs/superpowers/RELEASE-CHECKLIST.md`](docs/superpowers/RELEASE-CHECKLIST.md).
Known, deliberately deferred gaps:

- Shape (rounded-rect/stroke/shadow) anti-aliasing softens at display
  scale > 1 and can alias at scale < 1 — text stays crisp at any scale
  (it uses `fwidth`); shapes don't yet (moot for the classic themes' square,
  zero-radius corners, but still relevant to stroke edges).
- The custom titlebar targets Windows/Linux (glfw-undecorated); there is no
  native macOS traffic-lights integration.
- IME input and accessibility (screen-reader/automation) hooks are not
  implemented.

See [CHANGELOG.md](CHANGELOG.md) for the full v0.1.0 and v0.2.0 deliverable
lists and [ROADMAP.md](ROADMAP.md) for the phase-by-phase history and design
spec pointer.

## License

[MIT](LICENSE)

## Details

### Controls

`Button`/`ToggleButton`, `CheckBox`, `RadioButton` + `RadioGroup`,
`ToggleSwitch`, `TextBox`, `Slider` wired to a `ProgressBar`, `ComboBox`,
and `ToolTipArea` are all shown together in `fluo-gallery`'s Controls page.
Every popup-capable control (`ComboBox`'s dropdown, `ToolTipArea`'s tip)
needs a `controls.OverlayHost` ancestor to render into, with `SetRouter`
wired to the app's `input.Router` so the host's light-dismiss capture
works. `TextBox`'s caret blink and `ToolTipArea`'s hover-dwell delay both
animate off a `timers.Queue` (`SetTimers`); a nil queue degrades each to a
reasonable default (solid caret, immediate-show tooltip) rather than
breaking. `TextBox` also wires up `Ctrl+C`/`Ctrl+X`/`Ctrl+V` against
`input.Router`'s clipboard (backed by glfw's clipboard API), so cut/copy/
paste works out of the box. Every control follows the same uniform
setter/`OnChanged` contract: programmatic setters (`SetChecked`, `SetValue`,
`SetText`, `SetSelectedIndex`, `SetRange`, ...) are silent, while `OnChanged`
reports only user-driven changes (click, drag, typed input, keyboard
activation).

### Advanced controls

`ListView` (virtualized, single-column, selection via
`SelectedIndex`/`OnChanged`/`SetSelectedIndex`), `TreeView` (`TreeNode`
trees with expand/collapse and the same selection contract), `TabControl`
(`AddTab`), `Expander` (collapsible content), `MenuBar`/`MenuItems`/
`ShowContextMenu` (top-level menus, separators, and hover-opened
submenus), `ShowDialog` (a modal `DialogSpec` with Primary/Secondary
buttons and a `DialogResult`), and `DataGrid` (virtualized, multi-column,
`Column.Width` as a `Px`/`Star` `Track`) round out the control set.
`ListView` is fluo's one disposable control (`Dispose()` releases its
subscription to the item source's change channel) — every `ListView` a
consumer builds should have its `Dispose` collected into the same rebuild
cancel path as its ordinary binder cancels, so a rebuild never leaves a
stale subscription behind. `fluo-gallery`'s Advanced page exercises all of
these together.

### Data binding

Package `bind` connects `core.Property[T]` values to controls. `bind.OneWay`
pushes a property's value into a control (or any `func(T)`) on every change.
The seven two-way binders (`bind.Text`, `bind.Checked`, `bind.SwitchChecked`,
`bind.ToggleChecked`, `bind.Value`, `bind.SelectedIndex`, `bind.Selected`
for `controls.RadioGroup`) additionally OWN the bound control's `OnChanged`
slot — user edits flow into the property via `p.Set`, and any OTHER change
to the property flows back into the control via its silent setter
(`SetText`/`SetChecked`/`SetValue`/`SetSelectedIndex`), which — per the
uniform setter convention above — never re-fires `OnChanged`, so there is
no feedback loop. `bind.Items` binds a `bind.List[T]` (an observable slice)
to a `StackPanel` by clearing and rebuilding its children from scratch on
every list change; `ListView`/`DataGrid` bind a `bind.List[T]` directly
(virtualized, no rebuild needed) via `bind.ListSelected` for selection.

Every binder returns a `cancel func()` that detaches both directions
(idempotent — safe to call more than once, and safe to skip if the control
is being discarded anyway). Since `theme.SetActive` + rebuild throws away
the entire old widget tree and builds a fresh one (see Theming below), any
code that rebinds a rebuilt tree's controls MUST cancel the OLD tree's
bindings first — otherwise the discarded tree's binders keep OWNing
`OnChanged` on controls nobody can see anymore, fighting the new tree's
binders for control of the same property. `fluo-gallery`'s Binding demo is
the reference example: its models are constructed once in `main` and never
recreated, so they outlive every theme toggle; only the *bindings* onto
them are rebuilt, and every binder call appends its cancel to a slice that
main empties (calling each cancel) right before the next rebuild — models
outlive views, bindings don't.

### Theming

Every color a control draws comes from `theme.Active().Color`, a
`theme.ColorTokens` struct — there are no hard-coded colors anywhere in
`controls`. The classic four-tone bevel fields are `ButtonFace`,
`ButtonHighlight`, `ButtonLight`, `ButtonShadow`, `ButtonDarkShadow`
(raised/sunken 3D chrome), `WindowWell`/`WindowText`/`GrayText` (recessed
content areas like text boxes and lists), `Highlight`/`HighlightText`
(selection), and `CaptionFrom`/`CaptionTo`/`CaptionText`/`InactiveCaption`
(the title bar's left-to-right gradient and its unfocused-window fallback).
Metric tokens (`theme.MetricTokens`: corner radii, bevel width, padding
scale, ...) and type tokens (`theme.TypeTokens`: caption/body/subtitle/title
sizes) round out a `*theme.Theme`.

fluo ships two bundled themes, `theme.Light()` (the default — an authentic
Windows-2000 "Standard" gray) and `theme.Dark()` (the same bevel structure,
dark-beveled); `theme.SetActive` picks the active one and `theme.Active()`
reads it back (defaulting to `Light()` if nothing has been set). Widgets
capture the tokens they need at *construction* time (e.g. `NewTextBlock`,
`NewScrollViewer`), so there is no live re-skinning of an existing tree:
re-theming means calling `theme.SetActive` and then rebuilding the widget
tree from scratch (`fluo-gallery`'s `buildUI` is the reference example — it
is a pure function of `theme.Active()`, called again and swapped in via
`ctx.Input.SetRoot` on every toggle).

In the gallery, press **T** to toggle Light/Dark live (`SetRoot` intentionally
resets hover/capture/focus, since the widget tree itself is fresh). For quick
manual comparisons without a keypress, `FLUO_THEME=light` (or `dark`, the
default) picks the startup theme — a dev convenience, not a supported runtime
API:

```sh
FLUO_THEME=light go run ./cmd/fluo-gallery
```

#### Building a custom theme

A custom theme is just a `*theme.Theme` built by copying one of the bundled
themes and overriding whichever tokens you want — there's no separate
"theme builder" API, because `Theme`/`ColorTokens`/`MetricTokens`/
`TypeTokens` are all plain structs:

```go
package main

import (
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/theme"
)

// oceanTheme starts from the bundled Light theme and swaps in a blue-tinted
// highlight/caption palette, keeping every other classic token (bevels,
// well, text) untouched.
func oceanTheme() *theme.Theme {
	t := *theme.Light() // copy: Light() already returns a fresh *Theme per call
	t.Name = "ocean"
	t.Color.Highlight = render.RGB(0, 90, 158)
	t.Color.HighlightText = render.RGB(255, 255, 255)
	t.Color.CaptionFrom = render.RGB(0, 60, 110)
	t.Color.CaptionTo = render.RGB(90, 160, 210)
	t.Color.CaptionText = render.RGB(255, 255, 255)
	return &t
}

func main() {
	theme.SetActive(oceanTheme())
	// ... build the widget tree as usual; every control reads
	// theme.Active().Color.Highlight / .CaptionFrom / etc. at construction.
}
```

Because `Light()`/`Dark()` return a fresh `*Theme` value each call (not a
shared pointer), copying with `t := *theme.Light()` is safe — mutating `t`
never affects the bundled theme or any other copy of it.

### Golden-image tests

Rendering correctness is checked against golden PNGs in `render/gl/testdata`
and `render/gl/gltest/testdata`. GL tests need a real context and are
auto-skipped when none is available (e.g. headless CI without a GPU).
The `-update` flag is only registered by `render/gl/gltest`, so scope the
flag to that subtree rather than `./...`:

```sh
go test ./render/gl/... -update
```

Review the diffs to the regenerated PNGs before committing them.
