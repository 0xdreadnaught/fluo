# fluo — Fluent-styled retained-mode GUI toolkit for OpenGL/Go

**Date:** 2026-07-22
**Status:** Approved design, pre-implementation
**Name:** `fluo` — confirmed 2026-07-22.

## Purpose

A general-purpose, published Go GUI library for OpenGL applications that looks and
programs like a modern WPF/WinUI app — because the existing option in this space
(Dear ImGui via cimgui-go) looks dated and programs nothing like a desktop toolkit.

Target consumers: any Go app that owns a GL context and wants a real desktop-quality
UI in it. The motivating example is an existing go-gl/glfw viewer app, but that app is
maintained by a different team and is **out of scope** — we ship a library plus a
gallery app; adoption elsewhere is the consumer's job.

## Locked decisions

1. **Retained-mode** — persistent widget tree, WPF's two-pass Measure/Arrange layout,
   routed events, dirty-flag invalidation. Not immediate-mode.
2. **Go fluent API** — UI declared in Go code (`Button("Save").OnClick(...)`).
   No XAML/markup, no parser. A markup layer could compile down to the same tree
   later; explicitly out of scope now.
3. **Own GL renderer + SDF text** — reference backend on go-gl/gl + go-gl/glfw only;
   glyph rasterization via `golang.org/x/image/font` + `sfnt` into a signed-distance-
   field atlas. No new cgo dependencies. All drawing goes through a pure-Go
   `Renderer` interface so backends are swappable.
4. **Fluent/WinUI (Win11) default theme** — rounded corners, layered surfaces, soft
   shadows, accent focus, light + dark — built on a themeable token system so classic
   WPF or brand skins are just alternate theme resources.

## Architecture

Core principle: **layout, events, and binding are pure Go with zero GL dependency.**
Only `render/gl` touches OpenGL. The toolkit's brain is unit-testable headless; GL
appears only in golden-image render tests (hidden-window + offscreen FBO pattern).

| Package | Responsibility | GL? |
|---|---|---|
| `render` | `Renderer` interface + primitives (Color, Point, Size, Rect, Thickness) | no |
| `render/gl` | Reference backend: quad batching, SDF rounded-rect/shadow shaders, scissor-clip stack | **yes** |
| `text` | Font loading, glyph rasterization, SDF atlas, shaping/metrics | no |
| `core` | `Widget` interface, base element, layout engine, invalidation, reactive `Property[T]` | no |
| `input` | Event types, mouse/keyboard, hit-testing, routed (bubbling) events, focus/capture | no |
| `theme` | Theme resource/token model + Fluent light & dark token sets | no |
| `controls` | The widgets | no |
| `bind` | Two-way data binding to external view models | no |
| `app` | Host loop + glfw input pump; embeds into an existing render loop | glue |
| `examples/gallery` | Widget gallery app; grows a page per control | consumer |

### Key subsystem notes

- **Layout** is WPF's algorithm verbatim: `Measure(available) → desired` bottom-up,
  `Arrange(finalRect)` top-down, with `InvalidateMeasure/Arrange/Visual` dirty flags
  so only changed subtrees recompute. This is retained-mode's payoff over ImGui.
- **Reactive `Property[T]`** with change notification lives in `core` from day one —
  invalidation requires change notification anyway. The `bind` package is later sugar
  that compiles down to the same subscriptions.
- **Renderer takes a DPI scale factor from day one.** SDF text makes scaling cheap;
  the late-phase "high-DPI pass" is an audit, not a retrofit.
- **Text metrics land before layout needs them**: `TextBlock.Measure` consumes
  `text` package metrics, so `text` is Phase 1, layout Phase 2.
- **Frame tick/timer service** lives in `input`/`app` (Phase 3): caret blink, tooltip
  delays, and eventually animation all consume it.
- **Hover/press visual states ship with the controls** (Phase 5) as theme-driven
  state swaps; Phase 8's animation system only adds *transitions* between them.

### Error handling

- Library never panics on user error where avoidable; fluent-API misuse (e.g. adding
  a widget to two parents) fails fast with a clear error at tree-mutation time.
- GL errors surface from `render/gl` init as returned errors; per-frame GL error
  checks are debug-build only.

### Testing strategy

- `core`/`input`/`theme`/`bind`: pure-Go headless unit tests (layout geometry
  goldens as table tests — no GPU needed).
- `render/gl`/`text`: golden-image tests against an offscreen FBO in a hidden glfw
  window (skipped when no GL is available, e.g. bare CI).
- `examples/gallery` doubles as the manual/visual regression surface.

## Roadmap

Dependency-ordered phase tree lives in [`ROADMAP.md`](../../../ROADMAP.md) at the
repo root (checkboxes, kept current as work lands). Summary of ordering rationale:

- Phase 1 (renderer + text) is the hardest bottom layer and unblocks everything visual.
  It ends with a **minimal demo host** (glfw window + frame loop + input pump) so every
  later phase can be eyeballed and driven interactively — host *polish* stays in Phase 8.
- Phase 2 (layout) and Phase 3 (input) are pure Go on top of Phase 1's host.
- **Milestone: end of Phase 4** — a themed, laid-out, clickable Fluent button rendered
  in a real GL context. Everything after is breadth.
- Phase 9 (adoption in a real app) is explicitly **out of this repo's scope**; we ship
  the gallery + an integration guide instead.

## Open items

- IME input: known gap, explicit stretch item (Phase 8), not silently forgotten.
- Accessibility hooks: stretch (Phase 8).
