# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project intends to adhere to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
once it reaches 1.0.

## [Unreleased]

## [0.15.1] - 2026-08-01

### Fixed

- Pointer-capture release no longer pops a capture the widget never took — a
  clickable whose press wasn't delivered (a disabled menu row, a dialog scrim)
  could otherwise release another widget's capture and break modality.
- Keyboard focus is cleared when its subtree becomes hidden (switching tabs,
  collapsing an expander, `SetVisible(false)`), instead of routing keystrokes
  into an invisible widget.
- The `TextBox` caret-blink timer now runs only while the box is focused,
  instead of ticking (and retaining the widget) on every unfocused box.
- The shared clamp helpers reject NaN instead of passing it through, which
  previously could collapse a virtualized list to zero rows permanently.
- `NewListView(face, nil)`, `ScrollViewer.SetChild(nil)`, and `AddTab(label, nil)`
  no longer panic — a nil child/item source is treated as empty.
- `TreeView` now clips its rows to its own bounds.

## [0.15.0] - 2026-08-01

### Removed

- **BREAKING: the 20 long-deprecated pre-v0.2 color tokens on
  `theme.ColorTokens`** — `WindowBackground`, `LayerBackground`,
  `CardBackground`, `TextPrimary`, `TextSecondary`, `TextDisabled`, `Accent`,
  `AccentHover`, `AccentPressed`, `AccentText`, `ControlFill`,
  `ControlFillHover`, `ControlFillPressed`, `ControlStroke`,
  `SelectionBackground`, `ControlFillDisabled`, `ControlStrokeDisabled`,
  `AccentDisabled`, `SelectionForeground`, and `ScrimBackground`. Every
  control finished migrating to the classic four-tone palette in v0.2, so
  nothing rendered from these fields any more. `AcrylicTint` is unaffected and
  remains a live token. Rendering is unchanged — every golden is
  byte-identical. Code still reading a removed field should switch to the
  classic token it was mapped onto: the background/fill trio to `ButtonFace`,
  `TextPrimary` to `WindowText`, `TextSecondary`/`TextDisabled` to `GrayText`,
  `Accent`/`SelectionBackground` to `Highlight`, and
  `AccentText`/`SelectionForeground` to `HighlightText`.

## [0.14.0] - 2026-08-01

### Added

- **Double-click-to-select-word and triple-click-to-select-line in `TextBox`.**
- **`PointerEvent.ClickCount` / `PointerEvent.Time`** and `Router.SetTimeSource`
  — the router now reports a click count on each press (from the previous
  press's button, time, and position), available to any widget. Hosts opt in
  with a time source; without one, every press reports `ClickCount == 1` and
  behavior is unchanged.

## [0.13.0] - 2026-07-31

### Added

- **Line-number gutter for the multi-line `TextBox`** — `SetLineNumbers(true)`
  reserves a left column and draws one right-aligned number per logical line
  (wrapped rows stay blank), scrolling with the text. Off by default; caret and
  click hit-testing account for the gutter so text positioning stays exact.

## [0.12.0] - 2026-07-31

### Added

- **`SplitPanel`** — a two-pane container with a draggable divider. Lay two
  widgets side by side (`Horizontal`) or stacked (`Vertical`) and drag the
  divider to resize them; `SetSplitRatio`, `SetMinPaneSize`, and an
  `OnSplitChanged` callback control the split, and the divider shows a resize
  cursor. Follows the same `Orientation` convention as `StackPanel`.

## [0.11.0] - 2026-07-31

### Added

- **Editor-ergonomics keys for the multi-line `TextBox`:**
  - **Ctrl+Left / Ctrl+Right** — move the caret by whole words (Ctrl+Shift extends the selection).
  - **Ctrl+Home / Ctrl+End** — jump to the start / end of the whole buffer (the
    only way to reach it in multi-line mode; plain Home/End still stop at the line).
  - **Ctrl+Backspace / Ctrl+Delete** — delete the previous / next word.
  - **PageUp / PageDown** — move the caret by one viewport of rows.
  - New `input.KeyPageUp` / `input.KeyPageDown` constants.

### Changed

- Block indent/outdent (Tab / Shift+Tab over a multi-line selection) now applies
  as a single edit, firing `OnChanged` once per press instead of once per line.

## [0.10.1] - 2026-07-31

### Fixed

- **`SetTabInserts`: Tab no longer destroys a multi-line selection.** Pressing
  Tab (or Shift+Tab) with a selection spanning multiple lines now block-indents
  (or block-outdents) every selected line and keeps the selection, instead of
  replacing the whole selection with spaces. Single-line and no-selection
  behavior is unchanged.

## [0.10.0] - 2026-07-31

### Added

- **`TextBox.SetTabInserts`** — opt-in for a multi-line `TextBox` to treat Tab
  as indentation instead of focus navigation. When enabled, a focused Tab
  inserts spaces at the caret (and is consumed), and Shift+Tab unindents the
  current line. Off by default and inert in single-line mode, so the existing
  Tab-moves-focus behavior is unchanged unless opted in.

## [0.9.0] - 2026-07-30

### Added

- **`TextBox.OnSubmit`** — a single-line `TextBox` now fires an `OnSubmit(text)`
  callback when the user presses Enter (previously Enter was ignored in
  single-line mode). Multi-line mode is unchanged (Enter still inserts a
  newline and never fires `OnSubmit`).
- **Disabled menu items** — `MenuItems.AddDisabled(label, onClick)` adds a menu
  row that renders greyed (`GrayText`) and is non-interactive: it never
  highlights on hover and never invokes its action, while staying visible so
  users can see the option exists.

## [0.8.0] - 2026-07-30

### Added

- **Per-severity color for toast notifications.** `ToastSpec` gained a
  `Severity` field (`SeverityInfo` (default), `SeveritySuccess`,
  `SeverityWarning`, `SeverityError`). Non-info toasts show a colored accent
  stripe driven by new theme tokens (`ColorTokens.SeverityInfo/Success/Warning/
  Error`, defined in both `Light()` and `Dark()`), so success/warning/error read
  at a glance. `SeverityInfo` renders exactly as before, so existing callers are
  unchanged.

## [0.7.0] - 2026-07-30

### Fixed

- **Resolution-independent antialiasing for shape edges.** Rounded-rect, stroke,
  shadow, and acrylic-corner edges now derive their antialias band from the
  screen-space derivative of the signed distance (`fwidth`), the same way text
  already does, instead of a fixed logical-unit width. Shape edges stay crisp at
  display scale > 1 (HiDPI / 2x) and no longer alias at scale < 1. Text is
  unaffected. Shape golden images were regenerated for the sharper edges.

## [0.6.0] - 2026-07-30

### Changed

- **Glyph coverage atlas now grows across pages instead of dropping glyphs.**
  When a font's coverage atlas page fills up, a new page is allocated on demand
  rather than hitting a hard cap, so documents with many distinct glyphs (large
  multi-script text over a font-fallback chain) keep rendering. Draw batches by
  atlas page, so the single-page common case is unchanged. `Face.OnGlyphDropped`
  now fires only for the degenerate case of a glyph larger than a whole page.

## [0.5.0] - 2026-07-30

### Added

- **Word-wrap for multi-line `TextBox`** — `SetWordWrap(true)` soft-wraps long
  lines at the content width. Breaks are display-only (the text buffer keeps
  only real newlines), and the caret, selection, up/down navigation, and click
  hit-testing all track the wrapped visual rows.
- **Visible vertical scroll thumb for multi-line `TextBox`** — when content
  overflows the box, a draggable scroll thumb appears on the right edge using
  the same chrome as `ScrollViewer`, so overflowing text is no longer invisible.
  The thumb only reserves its gutter when shown, so non-overflowing layouts are
  unchanged.

## [0.4.0] - 2026-07-29

### Added

- **Font fallback chains** — `text.NewFaceWithFallback(primary, fallbacks, sizePx)`
  and `Face.AddFallback` build a `Face` from a primary font plus an ordered list
  of fallback fonts. Codepoints the primary font lacks are rasterized from the
  first fallback that has a real glyph, so mixed-script text (for example Latin
  alongside CJK) no longer renders as `.notdef` tofu boxes. Vertical metrics and
  glyph-atlas sizing stay driven by the primary font so layout is unchanged;
  only the per-glyph rasterization source and advance fall through the chain.

## [0.3.1] - 2026-07-27

### Fixed

- The GL golden-image test harness (`render/gl/gltest`) now skips cleanly on a
  headless machine (no display or GL driver) instead of panicking. Some glfw
  builds log the platform error from `Init` and return `nil`, which bypassed
  the existing skip guard; the context-setup phase is now guarded so the test
  is skipped rather than failed.

## [0.3.0] - 2026-07-27

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
- **Font collections** — `text.LoadCollection`/`text.LoadCollectionMember`
  load `.ttc`/`.otc` files and pick a member face by index, alongside the
  existing single-font `text.Load`; `Face.OnGlyphDropped` reports when a
  glyph couldn't be added to a full atlas, so callers can react (grow a new
  face, log, etc.) instead of silently missing glyphs.
- **Input consume-reporting** — `input.Router`'s pointer and keyboard
  dispatch now return whether fluo consumed the event, and the router gains
  `WantCapturePointer`/`WantCaptureKeyboard` queries (mirrored on
  `app.Surface`) reporting whether it currently wants exclusive input — for
  apps that embed fluo alongside their own input handling and need to know
  when to yield.
- **Multi-line TextBox** — `TextBox.SetMultiline(true)` turns on an opt-in
  multi-line mode: Enter inserts a newline, Up/Down move the caret between
  lines (preserving its column), Home/End act on the current line, and the
  box scrolls both axes as needed. Single-line boxes are unchanged.
- **Toast notifications** — `OverlayHost.ShowToast` shows a transient,
  bottom-right-stacked notification with an optional auto-dismiss `Timeout`
  (driven by a `timers.Queue` via `OverlayHost.SetTimers`), a returned
  `dismiss` func for closing it early, and click-to-dismiss.
- **Windows IME support** — the OS candidate window is anchored to the text
  caret, and `TextBox` renders inline preedit (composition) text at the
  caret while composing, instead of leaving IME users without visual
  feedback until they commit.

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
