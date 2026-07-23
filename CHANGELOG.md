# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project intends to adhere to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
once it reaches 1.0.

## [0.1.0] - unreleased

Initial feature-complete pre-release: a full Fluent/WinUI-styled retained-mode
GUI toolkit for OpenGL apps in Go, built bottom-up over eight phases.

### Added

- **Renderer + SDF text** (`render`, `render/gl`, `text`) — a backend-agnostic
  `render.Renderer` interface (rects, rounded rects, stroke, drop shadow,
  backdrop-blur, textured quads, clip stack) with an OpenGL 3.3 core-profile
  implementation; pure-Go TTF loading and signed-distance-field glyph
  rasterization with a shared per-font atlas, crisp at any scale.
- **Layout engine** (`core`) — a `Widget`/`Element` foundation with two-pass
  Measure→Arrange, margins/min-max/alignment/visibility, invalidation, and
  the reactive `Property[T]` primitive.
- **Input** (`input`) — hit-testing over the arranged tree, bubbling pointer
  and keyboard events, pointer capture, focus management with tab traversal,
  and cursor-shape hints, driven from glfw callbacks by `app.Run`.
- **Theming** (`theme`) — a token system (color, metric, typography) with
  bundled `FluentLight`/`FluentDark` variants; every control styles itself
  from `theme.Active()` rather than hard-coded values.
- **Controls** (`controls`) — the core set (`Border`, `TextBlock`, `Fixed`,
  `StackPanel`, `Grid`, `DockPanel`, `WrapPanel`, `Canvas`, `ScrollViewer`,
  `Button`, `ToggleButton`, `CheckBox`, `RadioButton`/`RadioGroup`,
  `ToggleSwitch`, `TextBox`, `Slider`, `ProgressBar`, `ComboBox`,
  `ToolTipArea`, `OverlayHost`) plus the advanced set (`ListView`,
  `TreeView`, `TabControl`, `Expander`, `MenuBar`, `Dialog`/`ShowDialog`,
  `DataGrid`), all keyboard-accessible and covered by headless + golden-image
  tests.
- **Data binding** (`bind`) — one-way and two-way binding between
  `core.Property[T]` and controls under a uniform silent-setter/`OnChanged`
  contract, plus `List[T]` (an observable slice) and `Items`/`ListSelected`
  collection binding.
- **App shell** (`app`, `anim`, `timers`) — `app.Run`/`app.Surface` for
  embedding fluo in a glfw-owned or caller-owned GL render loop; a custom
  Fluent titlebar (`controls.TitleBar`) for undecorated windows; an acrylic/
  mica backdrop-blur surface (`controls.AcrylicSurface`); an easing/`Tween`
  animation system (`anim`) driven by a frame-tick `timers.Queue`.
- Package documentation (`doc.go` in every package) and three runnable
  example programs (`examples/counter`, `examples/form`, `examples/todo`).

### Known deferred (not gaps — see README's Status section)

- Shape (rounded-rect/stroke/shadow) anti-aliasing softens at scale > 1 and
  can alias at scale < 1 (text stays crisp at any scale via `fwidth`).
- No native macOS window-chrome integration for the custom titlebar
  (Windows/Linux via glfw-undecorated is the supported path).
- IME input and accessibility (screen-reader/automation) hooks.
