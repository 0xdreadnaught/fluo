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
