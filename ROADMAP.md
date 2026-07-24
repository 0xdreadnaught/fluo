# fluo Roadmap

Dependency-ordered. A phase may start before the prior one is 100% done, but no node
before its dependencies. Check items off as they land. Design spec:
[`docs/superpowers/specs/2026-07-22-fluo-gui-toolkit-design.md`](docs/superpowers/specs/2026-07-22-fluo-gui-toolkit-design.md)

## Phase 0 · Scaffolding
- [x] Module layout, LICENSE, README
- [x] CI: headless `go test` (GL tests auto-skip without a GPU)
- [x] Golden-image test harness (hidden glfw window + offscreen FBO pattern)

## Phase 1 · Renderer + text  *(hardest bottom layer; unblocks everything visual)*
- [x] `render.Renderer` interface + primitives (Color/Point/Size/Rect/Thickness)
- [x] DPI scale factor plumbed through the renderer from day one
- [x] GL backend: batched colored-quad pipeline
- [x] SDF text: font load → glyph raster → atlas → `DrawText` (crisp at any scale)
- [x] Rounded-rect + anti-aliased fill/stroke shader
- [x] Clip stack (scissor/stencil)
- [x] Drop shadow + image draw
- [x] Golden-image tests for each primitive
- [x] **Minimal demo host**: glfw window + frame loop + input pump (polish waits for Phase 8)

## Phase 2 · Layout engine  *(pure Go, headless-testable)*
- [x] `Widget` interface + base element (margin/padding/alignment/size/visibility)
- [x] Two-pass Measure → Arrange
- [x] Invalidation + reactive `Property[T]`
- [x] First widgets: `Border`, `TextBlock`, `StackPanel` (exercise the engine)
- [x] `Grid`, `DockPanel`, `WrapPanel`, `Canvas`
- [x] Headless layout-geometry tests
- [x] Gallery example app skeleton (grows a page per control from here on)

## Phase 3 · Input & events  *(pure Go core + thin glfw pump)*
- [x] Event types; mouse/keyboard/wheel from glfw
- [x] Hit-testing over the arranged tree
- [x] Routed events (bubbling first)
- [x] Focus management + pointer capture
- [x] Tab navigation / focus traversal order
- [x] Cursor shapes (I-beam, hand, resize)
- [x] Frame tick / timer service (caret blink, tooltip delay, later animation)
- [x] `ScrollViewer` (needs input + clip)
- [x] **DPI gate (from Phase-1 final review):** verify `Ctx.Mouse.Pos` and `Ctx.Size`
      share the same logical coordinate space on a scale≠1 display before wiring
      hit-testing — the Phase-1 host assumes cursor pos is logical (true at scale 1;
      unverified at 2x). Resolved in Phase 3 Task 5: `Ctx.Size` now comes from
      `win.GetSize()` (GLFW window coordinates) instead of framebuffer/content-scale,
      matching `win.GetCursorPos()`'s coordinate space by construction — see the
      DPI comment block on `app.Run` in `app/window.go`. (resolved: logical space =
      window coords; scale = fb/window)

## Phase 4 · Theming
- [x] Theme resource model (color/metric/typography/effect tokens)
- [x] Fluent Light theme
- [x] Fluent Dark theme
- [x] Wire existing widgets to tokens (no hard-coded styling anywhere)
- [x] **MILESTONE: themed, laid-out, clickable Fluent button in a real GL context** —
      `render/gl` golden `TestFluentButton` (composed purely from `theme.FluentLight()`
      tokens) plus the live `fluo-gallery` demo button (Accent/AccentHover/AccentPressed
      states, click counter) and its T-key Light/Dark toggle.

## Phase 5 · Core controls
- [x] Overlay/popup layer (prerequisite for several controls)
- [x] `Button`, `ToggleButton`, `CheckBox`, `RadioButton`, `ToggleSwitch`
      — incl. theme-driven hover/press/disabled visual states (animation comes later)
- [x] `TextBox` (caret + blink, selection, editing — biggest single control)
- [x] Clipboard cut/copy/paste (glfw clipboard API)
- [x] `Slider`, `ProgressBar`
- [x] `ComboBox` (uses popup)
- [x] `ToolTip` (uses timer service)
- [x] **MILESTONE: gallery Controls section** — `fluo-gallery`'s scroll content
      grows a Controls section above the swatches exercising every Phase 5
      control (Button/ToggleButton/CheckBox/RadioButton+Group/ToggleSwitch/
      TextBox/Slider/ProgressBar/ComboBox/ToolTipArea) from a real
      `controls.OverlayHost` root, wired to the app's `Ctx.Timers` queue for
      live caret blink and tooltip dwell.

## Phase 6 · Data binding
- [x] One-way bind (property → UI) via `Property` subscriptions
- [x] Two-way bind for inputs (`TextBox`/`CheckBox`/`Slider` ↔ `*T`)
- [x] `ItemsSource` / collection binding
- [x] **MILESTONE: gallery Binding demo** — `fluo-gallery`'s Controls section
      gains a `core.Property[string]` two-way bound to the TextBox (plus
      one-way to a mirror TextBlock), a `core.Property[float32]` two-way
      bound to the Slider and one-way to the ProgressBar (replacing the
      Phase 5 direct `Slider.OnChanged` wiring), and a `bind.List[string]`
      rendered into a StackPanel via `bind.Items`, appended to by an "Add"
      Button. The three models outlive every theme-toggle rebuild; every
      binder's cancel func is collected and invoked before the next
      `buildUI` call — the package's first real cancel consumer.

## Phase 7 · Advanced controls
- [x] `ItemsControl` / `ListView` (+ virtualization)
- [x] `TreeView`, `TabControl`, `Expander`
- [x] `Menu` / `MenuBar` / `ContextMenu`
- [x] Dialog + modal layer
- [x] `DataGrid`
- [x] **MILESTONE: gallery Advanced page** — `fluo-gallery`'s nav sidebar
      becomes a real page switcher: a 2-row `ListView` of page names
      ("Controls"/"Advanced", two-way bound via `bind.ListSelected` to a
      `core.Property[int]` main owns across rebuilds), dogfooding `ListView`
      itself as the nav widget. "Advanced" (the new page, and the gallery's
      startup default) holds a `MenuBar` (File → New/Open/separator/Exit,
      Edit → a submenu demo) above a 3-tab `TabControl`: List (a 30-row
      `ListView` plus a selected-index `TextBlock`, again via
      `bind.ListSelected`), Tree (a small nested `TreeView` beside an
      `Expander` mirroring the selected node's label), and Data (a
      3-column/50-row `DataGrid` plus a button firing `ShowDialog`, its
      result mirrored into a `TextBlock` via `bind.OneWay`). Every
      `ListView`'s `Dispose` (fluo's one disposable control) is collected
      into the rebuild's cancel path alongside the ordinary binder cancels.

## Phase 8 · App shell & polish
- [x] Clean embedding API: integrate into an app's existing GL render loop —
      `app.Surface` (`NewSurface`/`SetRoot`/`Router`/`Timers`/`Frame`); `app.Run`
      refactored onto it, behavior unchanged.
- [x] Custom Fluent titlebar + window chrome — `controls.TitleBar` (title +
      min/max/close caption buttons, `DragRegion`) + `app.Config.Undecorated`
      + `Ctx.Minimize`/`Ctx.ToggleMaximize`/`Ctx.BeginDrag`; `fluo-gallery`
      now opens undecorated with a live TitleBar wired to all three.
- [x] Acrylic / mica surface effect (blur shader) — `render.Renderer.
      DrawBackdropBlur` + `*gl.Renderer` impl (framebuffer-snapshot separable
      blur, tinted-translucency degrade documented) + `controls.AcrylicSurface`,
      wrapping `fluo-gallery`'s content pane.
- [x] Animation system (easing, transitions between the Phase-5 visual
      states) — `anim` (`Easing`/`Tween`) + `controls.colorAnim`, opt-in via
      `SetAnimated`/`SetTimers` on `Button` (default off; every existing
      golden/test stays byte-identical); `fluo-gallery`'s four Button-family
      demo controls (Click me / Accent / Toggle / Add) opt in.
- [x] High-DPI audit pass end-to-end — code audit + scale-parameterized
      tests (`render/gl` `text_2x.png` golden, hit-test scale-independence
      test); residual manual-verification gap (no live 2x display in WSLg)
      documented.
- [x] Docs site / examples / integration guide — root + per-package `doc.go`
      (all 17 packages), `examples/{counter,form,todo}`, expanded README.
- [x] Chain-aware popup forwarding (Phase-7 backlog) — `OverlayHost`
      generalized to walk the whole popup stack: an outside press now
      dismisses the entire open chain (not just the topmost popup), and
      hover/Move forwarding targets whichever popup in the stack actually
      contains the pointer, making sibling submenu rows reachable without
      first closing the open one. This is `OverlayHost`-level only — it
      does not add top-level `MenuBar` hover-switching (hovering "Edit"
      while "File" is open does not auto-open "Edit"; `MenuBar.OnPointer`
      still only opens a menu on `Press`), so `fluo-gallery`'s Advanced-page
      MenuBar does not slide File↔Edit on hover.
- [x] v0.1 publish PREP complete — `go.mod`/`LICENSE`/doc coverage verified,
      `go vet`/`gofmt`/`go build`/`go test` (incl. golden suite) all clean;
      see `docs/superpowers/RELEASE-CHECKLIST.md` for the operator's actual
      publish steps (tag + push — out of scope for this repo's automation).
- [ ] *(stretch)* IME input
- [ ] *(stretch)* Accessibility hooks

**fluo is v0.1-ready.** Every non-stretch Phase 0–8 item is implemented,
tested, and live-verified (`fluo-gallery`); publish itself (create the GitHub
repo, push, tag `v0.1.0`) is an operator action — see
[`docs/superpowers/RELEASE-CHECKLIST.md`](docs/superpowers/RELEASE-CHECKLIST.md).

## Phase 9 · v0.2 classic-depth restyle
- [x] **v0.2 complete** — every control restyled from Fluent flat chrome to
      the classic Windows-2000 four-tone bevel look (`theme.Light()`/
      `theme.Dark()` replace `FluentLight`/`FluentDark`; `DrawGradientRect`
      for the caption bar; zero literal colors in `controls`); `fluo-gallery`
      updated to match (classic `ButtonFace` content pane in place of
      `AcrylicSurface`, T-key toggle confirmed against the new theme names);
      see [`docs/superpowers/specs/2026-07-23-v0.2-classic-depth-design.md`](docs/superpowers/specs/2026-07-23-v0.2-classic-depth-design.md)
      and [CHANGELOG.md](CHANGELOG.md)'s `[0.2.0]` entry for the full list.

## Out of scope for this repo
Adoption in any specific existing app (e.g. replacing cimgui elsewhere) is the
consumer's project, not ours — the gallery + integration guide are our deliverables.
