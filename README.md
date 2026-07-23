# fluo

fluo is a retained-mode, Fluent/WinUI-styled GUI toolkit for OpenGL apps in
Go. It gives a Go program a themed widget tree — layout, input routing,
data binding, and a full Fluent control set — over a thin OpenGL 3.3
renderer, without pulling in a browser engine or a native OS toolkit
binding.

fluo is built bottom-up as a stack of small, independently testable
packages (see [Architecture](#architecture) below) rather than one large
framework package, and every control is driven from swappable design
tokens rather than hard-coded styling, so re-theming an app is a data
change, not a rewrite.

## Features

- A full Fluent-styled control set: `Border`, `TextBlock`, `StackPanel`,
  `Grid`, `DockPanel`, `WrapPanel`, `Canvas`, `ScrollViewer`, `Button`,
  `ToggleButton`, `CheckBox`, `RadioButton`/`RadioGroup`, `ToggleSwitch`,
  `TextBox`, `Slider`, `ProgressBar`, `ComboBox`, `ToolTipArea`,
  `ListView`, `TreeView`, `TabControl`, `Expander`, `MenuBar`, modal
  `Dialog`, `DataGrid`, and a custom `TitleBar` for undecorated windows.
- Fluent Light/Dark theming: every control styles itself from
  `theme.Active()`'s color/metric/typography tokens; re-theming means
  swapping the active theme and rebuilding the tree.
- Two-way and one-way data binding (package `bind`) between
  `core.Property[T]` values and controls, under a uniform silent-setter/
  `OnChanged` contract, plus observable-list collection binding.
- Virtualized `ListView`/`DataGrid` for large item sets.
- An acrylic/mica backdrop-blur surface (`controls.AcrylicSurface`) for
  translucent Fluent-style chrome.

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
5. **`theme`** — the color/metric/typography token model (`FluentLight`/
   `FluentDark`).
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

**v0.1 — ready to publish.** The full layer stack above is implemented,
tested (headless layout/binding tests plus GL golden-image tests, auto-skipped
without a GPU), and live-verified: `fluo-gallery` now opens as an undecorated
window with a custom `TitleBar` (drag-to-move, min/max/close), an
`AcrylicSurface` content pane, and animated demo buttons, exercising every
control in the toolkit together. `go vet`, `gofmt`, `go build`, and
`go test` (including the golden suite) are all clean. Publishing the tagged
release itself (creating the GitHub repo, pushing, tagging `v0.1.0`) is an
operator action — see
[`docs/superpowers/RELEASE-CHECKLIST.md`](docs/superpowers/RELEASE-CHECKLIST.md).
Known, deliberately deferred gaps:

- Shape (rounded-rect/stroke/shadow) anti-aliasing softens at display
  scale > 1 and can alias at scale < 1 — text stays crisp at any scale
  (it uses `fwidth`); shapes don't yet.
- The custom Fluent titlebar targets Windows/Linux (glfw-undecorated);
  there is no native macOS traffic-lights integration.
- IME input and accessibility (screen-reader/automation) hooks are not
  implemented.

See [CHANGELOG.md](CHANGELOG.md) for the full v0.1.0 deliverable list and
[ROADMAP.md](ROADMAP.md) for the phase-by-phase history and design spec
pointer.

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

Every control is styled from `theme.Active()` — a `*theme.Theme` bundling
color, metric (radius/padding/stroke), and typography tokens — rather than
hard-coded values. `theme.FluentLight()` and `theme.FluentDark()` are the two
built-in variants (`theme.SetActive` picks one; `Active()` defaults to
Dark). Widgets capture the tokens they need at *construction* time (e.g.
`NewTextBlock`, `NewScrollViewer`), so there is no live re-skinning of an
existing tree: re-theming means calling `theme.SetActive` and then rebuilding
the widget tree from scratch (`fluo-gallery`'s `buildUI` is the reference
example — it is a pure function of `theme.Active()`, called again and swapped
in via `ctx.Input.SetRoot` on every toggle).

In the gallery, press **T** to toggle Light/Dark live (`SetRoot` intentionally
resets hover/capture/focus, since the widget tree itself is fresh). For quick
manual comparisons without a keypress, `FLUO_THEME=light` (or `dark`, the
default) picks the startup theme — a dev convenience, not a supported runtime
API:

```sh
FLUO_THEME=light go run ./cmd/fluo-gallery
```

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
