# fluo

A retained-mode, Fluent/WinUI-styled GUI toolkit for OpenGL apps in Go.
Pre-alpha — see [ROADMAP.md](ROADMAP.md) and `docs/superpowers/specs/` for the design.

## Requirements

- Go 1.23
- A C compiler for cgo (go-gl/glfw and go-gl/gl are cgo bindings)
- An OpenGL 3.3 (core profile) capable driver

On this dev box (Windows + WSL2), the toolchain runs inside WSL, and the demo
window is displayed via WSLg — it opens as a normal window on the Windows
desktop even though the process runs in Linux.

## Running the demo

```sh
go run ./cmd/fluo-demo
```

Opens a window exercising the current primitives: rounded rects, stroke,
drop shadow, and SDF text, plus a hand-rolled hover/press-reactive button.
Close the window to exit.

```sh
go run ./cmd/fluo-gallery
```

Opens the widget gallery: a resizable window laid out entirely by the
layout engine (DockPanel title bar + nav sidebar + WrapPanel content area).
Grows a page per control as later phases land. The color swatches are
interactive (hover/press/focus/cursor) and the content area scrolls,
demonstrating fluo's input event API.

## Controls

Phase 5 shipped the core control set, all shown together at the top of the
gallery's scroll content, above the color swatches: `Button`/`ToggleButton`,
`CheckBox`, `RadioButton` + `RadioGroup`, `ToggleSwitch`, `TextBox`,
`Slider` wired to a `ProgressBar`, `ComboBox`, and `ToolTipArea`. Every
popup-capable control (`ComboBox`'s dropdown, `ToolTipArea`'s tip) needs a
`controls.OverlayHost` ancestor to render into — the gallery's root is now an
`OverlayHost` wrapping its DockPanel chrome as content, with `SetRouter`
wired to the app's `input.Router` so the host's light-dismiss capture works.
`TextBox`'s caret blink and `ToolTipArea`'s hover-dwell delay both animate
off the same `app.Ctx.Timers` queue (`SetTimers`); a nil queue degrades each
to a reasonable default (solid caret, immediate-show tooltip) rather than
breaking. `TextBox` also wires up `Ctrl+C`/`Ctrl+X`/`Ctrl+V` against
`input.Router`'s clipboard (backed by glfw's clipboard API), so cut/copy/
paste works out of the box. Every control follows the same uniform
setter/`OnChanged` contract: programmatic setters (`SetChecked`, `SetValue`,
`SetText`, `SetSelectedIndex`, `SetRange`, ...) are silent, while `OnChanged`
reports only user-driven changes (click, drag, typed input, keyboard
activation).

## Data binding

Package `bind` connects `core.Property[T]` values to controls. `bind.OneWay`
pushes a property's value into a control (or any `func(T)`) on every change.
The seven two-way binders (`bind.Text`, `bind.Checked`, `bind.SwitchChecked`,
`bind.ToggleChecked`, `bind.Value`, `bind.SelectedIndex`, `bind.Selected`
for `controls.RadioGroup`) additionally OWN
the bound control's `OnChanged` slot — user edits flow into the property via
`p.Set`, and any OTHER change to the property flows back into the control
via its silent setter (`SetText`/`SetChecked`/`SetValue`/`SetSelectedIndex`),
which — per the uniform setter convention above — never re-fires `OnChanged`,
so there is no feedback loop. `bind.Items` binds a `bind.List[T]` (an
observable slice) to a `StackPanel` by clearing and rebuilding its children
from scratch on every list change (v0 — virtualization is Phase 7).

Every binder returns a `cancel func()` that detaches both directions
(idempotent — safe to call more than once, and safe to skip if the control
is being discarded anyway). Since `theme.SetActive` + rebuild throws away
the entire old widget tree and builds a fresh one (see Theming below), any
code that rebinds a rebuilt tree's controls MUST cancel the OLD tree's
bindings first — otherwise the discarded tree's binders keep OWNing
`OnChanged` on controls nobody can see anymore, fighting the new tree's
binders for control of the same property. `fluo-gallery`'s Binding demo is
the reference example: its three models (a `Property[string]`, a
`Property[float32]`, and a `bind.List[string]`) are constructed once in
`main` and never recreated, so they outlive every theme toggle; only the
*bindings* onto them are rebuilt, and every one of buildUI's binder calls
appends its cancel to a slice that main empties (calling each cancel) right
before the next `buildUI` — models outlive views, bindings don't.

## Theming

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

## Golden-image tests

Rendering correctness is checked against golden PNGs in `render/gl/testdata`
and `render/gl/gltest/testdata`. GL tests need a real context and are
auto-skipped when none is available (e.g. headless CI without a GPU).
The `-update` flag is only registered by `render/gl/gltest`, so scope the
flag to that subtree rather than `./...`:

```sh
go test ./render/gl/... -update
```

Review the diffs to the regenerated PNGs before committing them.
