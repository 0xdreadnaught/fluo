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
