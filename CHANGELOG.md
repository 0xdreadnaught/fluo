# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project intends to adhere to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
once it reaches 1.0.

## [Unreleased]

### Added

- **HD text rendering** — UI glyphs are now rasterized directly at the exact
  device-pixel size with grayscale antialiasing and pixel-snapped baselines,
  replacing the soft fixed-48px SDF-scaled path. `render.Renderer` gains
  `Scale()` and `DrawGlyphs()`; the SDF path is retained for future
  scaled/animated text.
- **Horizontal scrolling** — `ScrollViewer` scrolls both axes (horizontal
  thumb, Shift+wheel), and the shared row virtualizer brings the same to
  `ListView` and `DataGrid` (the DataGrid header scrolls in sync with the
  body so columns stay aligned).
- **Pill and circular buttons** — `Button.SetShape` / `ToggleButton.SetShape`
  with `ShapeRect` (default), `ShapePill` (stadium) and `ShapeCircle`
  (square-aspect), drawn with a raised-rounded bevel that keeps the classic
  3D depth on curved shapes.
- **Vertical sliders and progress bars** — `Slider.SetOrientation` and
  `ProgressBar.SetOrientation` accept `controls.Vertical`; vertical sliders
  put Max at the top, vertical progress fills bottom-to-top.
- **Solid progress bars** — `ProgressBar.SetSolid(true)` renders a single
  continuous fill instead of the classic chunked blocks (still the default).

### Fixed

- Glyph baselines no longer jitter by a pixel between different letters —
  glyph rasterization is baseline-integer-aligned so every glyph shares the
  same snapped baseline.

## [0.2.0] - 2026-07-23

The classic-depth restyle: fluo drops its Fluent/WinUI look for an authentic
Windows-2000 four-tone bevel chrome — every control now draws
raised/sunken 3D edges, square corners, and gradient title bars from theme
tokens rather than the old flat Fluent fills.

### Added

- **Classic color tokens** (`theme.ColorTokens`) — `ButtonFace`,
  `ButtonHighlight`, `ButtonLight`, `ButtonShadow`, `ButtonDarkShadow`
  (the four-tone raised/sunken bevel), `WindowWell`/`WindowText`/`GrayText`
  (recessed content areas), `Highlight`/`HighlightText` (selection), and
  `CaptionFrom`/`CaptionTo`/`CaptionText`/`InactiveCaption` (the title bar's
  gradient and its unfocused fallback). Every control's fill/stroke now
  reads exclusively from these tokens — zero literal colors in `controls`.
- **`render.Renderer.DrawGradientRect`** (+ `*gl.Renderer` implementation) —
  a two-stop linear-gradient fill, used by `controls.TitleBar` for the
  classic `CaptionFrom`→`CaptionTo` caption bar.
- Bevel-drawing helpers in `controls` for the raised/sunken four-tone edge
  treatment shared by `Button`, `CheckBox`, `TextBox`, `ListView`, and every
  other chrome-drawing control.
- `theme.MetricTokens.BevelWidth` — the pixel width of each bevel step;
  `CornerRadius`/`ControlCornerRadius` are now `0` in both bundled themes
  (classic chrome is square, not rounded).

### Changed

- **Breaking:** `theme.FluentLight()`/`theme.FluentDark()` are replaced by
  `theme.Light()`/`theme.Dark()`, returning the classic palette instead of
  the old flat Fluent one. `theme.Theme.Name` is now `"classic-light"` /
  `"classic-dark"`.
- **Breaking:** roughly two dozen pre-v0.2 flat-look `ColorTokens` fields
  (`WindowBackground`, `LayerBackground`, `CardBackground`, `TextPrimary`,
  `Accent`, `ControlFill`, ...) are marked `// deprecated:` and mapped onto
  the classic palette so any code still reading them keeps rendering in
  classic colors during migration; five with zero remaining references
  (`FocusStroke`, `ScrollThumb`, `SelectionText`, `Shadow`,
  `CloseButtonHover`) were removed outright.
- `cmd/fluo-gallery` no longer uses `controls.AcrylicSurface` for its
  content pane (a translucent backdrop-blur doesn't fit the classic look);
  it now uses a plain `controls.Border` filled with `theme.Active().Color.
  ButtonFace`. `AcrylicSurface` itself is unchanged and still available as
  a control for apps that want it.
- Every control's raised/pressed/hover/disabled visual states now render as
  classic bevel transitions (e.g. a pressed `Button` swaps to a sunken
  bevel) rather than the old flat Fluent color swap.

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
