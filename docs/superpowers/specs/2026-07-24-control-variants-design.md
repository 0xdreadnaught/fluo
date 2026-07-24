# fluo Control Variants — Design

**Date:** 2026-07-24
**Status:** Approved design, pre-implementation
**One-liner:** Add horizontal scrolling (ScrollViewer + virtualized ListView/DataGrid), circular & pill buttons (raised-rounded), vertical sliders & progress bars, and a solid progress-bar fill variant — all classic-themed, token-driven.

## Scope (6 features)
1. Horizontal scrollbars — ScrollViewer AND the shared virtualizer (ListView, DataGrid).
2. Circular buttons.
3. Pill buttons.
4. Vertical sliders.
5. Vertical progress bars.
6. Solid progress-bar fill variant (alongside the existing chunked default).

## Global constraints (inherited)
- go 1.23 pinned; build/test via WSL; keyed literals; `go vet`/`gofmt` clean; doc comments on exports.
- Zero literal colors in controls (theme tokens only). Classic look retained.
- Additive, backward-compatible APIs: every new behavior is opt-in via a setter; existing construction is unchanged (horizontal slider, chunked progress, rect button, vertical-only scroll stay the defaults).
- Behavior/layout tests unchanged except where a new default geometry legitimately shifts them (documented per the plan, like the v0.2 bevel-inset precedent). Golden images regenerate + human-inspected.

## Reused primitives
- `controls.Orientation` (`Horizontal`/`Vertical`) already exists (StackPanel). Reuse it for Slider/ProgressBar orientation.
- `controls/bevel.go` helpers (drawRaised/drawSunken/drawGroove/drawFocusRect) and the shared `drawScrollThumb`.
- Renderer `FillRoundedRect`/`StrokeRoundedRect` for rounded button chrome.

---

## 1. Vertical Slider & Vertical Progress Bar

### Slider (`controls/slider.go`)
- Add `orientation Orientation` (default `Horizontal`) + `SetOrientation(o Orientation) *Slider`.
- **Measure:** horizontal → current `{160, 24}`; vertical → swapped `{24, 160}`.
- **Value axis:** vertical convention = **Max at top, Min at bottom**. The usable span is `[thumbRadius, H-thumbRadius]` along Y; thumb center `y = bounds.Y + thumbRadius + (1 - proportion)*span`. `localToValue` reads the pointer's Y (inverted) for vertical.
- **Render:** the track groove runs along the long axis; the filled portion is the **Min side** (below the thumb for vertical, matching horizontal's left-of-thumb). Thumb is the same raised rectangular thumb, oriented across the track. Focus rect unchanged (whole bounds).
- All the thumb-inset span math generalizes: factor the axis (pick `W`/`X` for horizontal, `H`/`Y` for vertical) so `thumbCenter`/`localToValue`/`Render` share one code path parameterized by orientation.

### ProgressBar (`controls/progressbar.go`)
- Add `orientation Orientation` (default `Horizontal`) + `SetOrientation(o) *ProgressBar`.
- **Measure:** horizontal → current default; vertical → swapped (tall-narrow).
- **Fill direction:** horizontal fills left→right; vertical fills **bottom→top**.
- Chunked and solid (below) both honor orientation.

## 2. Solid Progress-Bar Variant (`controls/progressbar.go`)
- Add `solid bool` (default `false` = chunked) + `SetSolid(v bool) *ProgressBar`.
- **Render:** `solid` → a single `Highlight` fill inside the sunken well spanning `value` proportion along the orientation (no chunk gaps). `!solid` → existing chunked blocks. Both inset inside the 2px sunken bevel.

## 3. Pill & Circular Buttons (`controls/button.go`, `controls/bevel.go`)

### Shape API
- New `ButtonShape` type with `ShapeRect` (0, default), `ShapePill`, `ShapeCircle`.
- `Button.SetShape(s ButtonShape) *Button` (also on `ToggleButton` for consistency; default `ShapeRect`).
- **Radius:** `ShapePill` → `radius = bounds.H / 2` (stadium). `ShapeCircle` → `radius = min(bounds.W, bounds.H) / 2`.
- **Circle aspect:** for `ShapeCircle`, `MeasureContent` returns a square whose side = `max(contentW, contentH) + padding` so the circle encloses the label/glyph.
- `ShapeRect` render path is unchanged (square four-tone bevel).

### Raised-rounded rendering (new bevel helpers)
Add to `controls/bevel.go` (token-driven, no literals):
```go
// drawRaisedRounded paints a raised 3D look on a rounded rect: a face-filled
// shape with a light top-left / dark bottom-right 1px bevel, approximated by
// layering offset rounded fills (StrokeRoundedRect strokes a single color, so
// directional lighting is faked by offset).
func drawRaisedRounded(r render.Renderer, rect render.Rect, radius float32, face render.Color, c theme.ColorTokens)
func drawSunkenRounded(r render.Renderer, rect render.Rect, radius float32, fill render.Color, c theme.ColorTokens)
```
Implementation (raised): (1) `FillRoundedRect(rect, radius, c.ButtonDarkShadow)` — full, becomes the bottom-right dark rim; (2) `FillRoundedRect(rect offset (-1,-1), radius, c.ButtonHighlight)` — full, becomes the top-left light rim, leaving 1px dark at bottom-right; (3) `FillRoundedRect(rect inset 1px, radius-1, face)` — the interior, leaving the 1px directional rim. Sunken swaps ButtonDarkShadow↔ButtonHighlight and offsets (+1,+1).
- **Button.Render** for `ShapePill`/`ShapeCircle`: `drawRaisedRounded(bounds, radius, face, c)` at rest (`face` = ButtonFace, hover = ButtonLight); pressed / checked-toggle → `drawSunkenRounded` + label +1,+1 nudge. Label/engrave colors identical to the rect path.
- **Focus (rounded):** for pill/circle, draw a rounded focus ring — `StrokeRoundedRect(bounds inset 1px, radius-1, FocusStrokeWidth, c.Highlight)` — instead of the square `drawFocusRect`.
- Default-button outer border (`SetAccent`) for rounded shapes: a 1px `ButtonDarkShadow` `StrokeRoundedRect` just outside the shape (matches the square default-border intent).

## 4. Horizontal Scroll — ScrollViewer (`controls/scrollviewer.go`)

- Generalize the single-axis (vertical) model to both axes. Add horizontal offset `offsetX` alongside the existing vertical offset; clamp each independently to `[0, contentSize - viewportSize]` per axis.
- **Measure/Arrange:** give the child its full desired size on the overflowing axis (already done vertically) so it can exceed the viewport horizontally; arrange the child offset by `(-offsetX, -offsetY)`; the existing clip covers both axes.
- **Thumbs:** reuse `drawScrollThumb` for a horizontal thumb+track along the bottom edge, shown only when `contentW > viewportW` (mirrors the existing vertical thumb shown when `contentH > viewportH`). Reserve a gutter on the axis that has a thumb (the existing conditional-gutter logic generalizes; when both thumbs show, reserve both and leave the bottom-right corner square).
- **Input:** vertical wheel → Y; **Shift+wheel → X**; if only the X axis overflows, a plain wheel scrolls X. Horizontal thumb drag mirrors the vertical drag math on X.
- Both-axes drag/geometry share one axis-parameterized helper where practical.

## 5. Horizontal Scroll — Virtualizer (ListView + DataGrid)

The shared row virtualizer (`controls/virtualizer.go` + used by `controls/listview.go`, `controls/datagrid.go`) gains a horizontal offset for content wider than the viewport.
- **Content width:** DataGrid = `sum(colWidths)`; ListView = the max row content width (or its fixed row width if content is clipped) — the virtualizer takes a `contentWidth` from its host.
- **Rendering:** rows/cells render offset by `-offsetX` and are clipped to the (bevel-inset) content viewport; a horizontal thumb+track (via `drawScrollThumb`) shows along the bottom when `contentWidth > viewportW`.
- **DataGrid header:** scrolls horizontally IN SYNC with the body (same `offsetX`) but stays vertically fixed — so columns line up with their cells while horizontally scrolling.
- **Input:** Shift+wheel → X (plain wheel stays Y for row scrolling); horizontal thumb drag.
- Selection band, hit-testing, and the existing vertical virtualization are unchanged except for the added `-offsetX` on the horizontal axis (draw + hit-test stay in agreement, as with the v0.2 bevel inset).

## Testing
- New behavior tests: vertical slider value↔position mapping (Max-at-top), vertical progress fill direction, solid vs chunked fill, button shape measure (circle square aspect) + radius, ScrollViewer X clamp/offset + shift-wheel, virtualizer X offset + header sync + H-thumb visibility threshold.
- Goldens: new — `slider_vertical`, `progress_vertical`, `progress_solid`, `button_pill`, `button_circle`, `scroll_horizontal`, `datagrid_hscroll`. Regenerate any existing golden whose control gained a default-neutral code path only if it actually changes (it should not — new behavior is opt-in). Human-inspect all new goldens.
- Behavior/layout tests for existing default paths stay green unchanged; if a shared refactor (axis parameterization) shifts a white-box constant, treat like the v0.2 precedent (update the constant only if it's a proven mechanical shift, never weaken an assertion) — flag for adjudication.
- Live gallery: add a small showcase (a vertical slider, vertical + solid progress, a pill and a circle button, a horizontally-scrollable area) to a gallery page for visual acceptance.

## Non-goals / deferred
- Scrollbar arrow buttons (still thumb-only, as in v0.2).
- Diagonal/2D thumb, momentum/kinetic scrolling, scroll animation.
- Circular/pill toggle-switch or other controls — shapes apply to Button/ToggleButton only.
- Per-axis independent scroll policies (always auto: thumb shows iff that axis overflows).

## Migration notes
- Purely additive API: `Slider.SetOrientation`, `ProgressBar.SetOrientation`/`SetSolid`, `Button.SetShape`/`ToggleButton.SetShape`, `ButtonShape` + `ShapeRect/ShapePill/ShapeCircle`, plus internal ScrollViewer/virtualizer horizontal fields. No breaking changes to existing call sites.
- CHANGELOG: note the new variants under the next version.
