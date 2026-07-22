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
- [ ] Theme resource model (color/metric/typography/effect tokens)
- [ ] Fluent Light theme
- [ ] Fluent Dark theme
- [ ] Wire existing widgets to tokens (no hard-coded styling anywhere)
- [ ] **MILESTONE: themed, laid-out, clickable Fluent button in a real GL context**

## Phase 5 · Core controls
- [ ] Overlay/popup layer (prerequisite for several controls)
- [ ] `Button`, `ToggleButton`, `CheckBox`, `RadioButton`, `ToggleSwitch`
      — incl. theme-driven hover/press/disabled visual states (animation comes later)
- [ ] `TextBox` (caret + blink, selection, editing — biggest single control)
- [ ] Clipboard cut/copy/paste (glfw clipboard API)
- [ ] `Slider`, `ProgressBar`
- [ ] `ComboBox` (uses popup)
- [ ] `ToolTip` (uses timer service)

## Phase 6 · Data binding
- [ ] One-way bind (property → UI) via `Property` subscriptions
- [ ] Two-way bind for inputs (`TextBox`/`CheckBox`/`Slider` ↔ `*T`)
- [ ] `ItemsSource` / collection binding

## Phase 7 · Advanced controls
- [ ] `ItemsControl` / `ListView` (+ virtualization)
- [ ] `TreeView`, `TabControl`, `Expander`
- [ ] `Menu` / `MenuBar` / `ContextMenu`
- [ ] Dialog + modal layer
- [ ] `DataGrid`

## Phase 8 · App shell & polish
- [ ] Clean embedding API: integrate into an app's existing GL render loop
- [ ] Custom Fluent titlebar + window chrome
- [ ] Acrylic / mica surface effect (blur shader)
- [ ] Animation system (easing, transitions between the Phase-5 visual states)
- [ ] High-DPI audit pass end-to-end
- [ ] Docs site / examples / integration guide
- [ ] Tag + publish v0.1
- [ ] *(stretch)* IME input
- [ ] *(stretch)* Accessibility hooks

## Out of scope for this repo
Adoption in any specific existing app (e.g. replacing cimgui elsewhere) is the
consumer's project, not ours — the gallery + integration guide are our deliverables.
