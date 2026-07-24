# Control Variants Implementation Plan

> Implement per the spec `docs/superpowers/specs/2026-07-24-control-variants-design.md` — it holds the full design; this plan is the ordered task list. All new behavior is OPT-IN (existing defaults unchanged) so existing goldens must NOT move — only new goldens are added.

**Goal:** Horizontal scrolling (ScrollViewer + virtualized ListView/DataGrid), pill & circular buttons (raised-rounded), vertical sliders & progress bars, and a solid progress-fill variant.

**Tech Stack:** Go 1.23, go-gl. Build/test via WSL only.

## Global Constraints
- Module `github.com/0xdreadnaught/fluo`; go 1.23 PINNED — no go.mod/go.sum changes, no `go get`.
- Build/test ONLY via WSL: `wsl -e bash -lc 'cd /mnt/c/Users/dread/source/fluo && GOTOOLCHAIN=local <cmd>'` (go: /usr/local/go/bin/go, gofmt: /usr/local/go/bin/gofmt).
- Keyed literals; `go vet ./...` + `gofmt -l .` clean; doc comments on exported identifiers.
- ZERO literal colors in controls (theme tokens only). Classic look retained.
- Additive/opt-in: default construction unchanged. Existing behavior tests AND existing goldens stay green/unchanged — if an existing golden changes, the new code leaked into a default path: fix the code, don't accept the golden. New goldens are added + human-inspected.
- Commit trailer: `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`.

---

### Task 1: Vertical Slider + Vertical/Solid ProgressBar
**Files:** `controls/slider.go`, `controls/progressbar.go`, their `_test.go`; new goldens `slider_vertical.png`, `progress_vertical.png`, `progress_solid.png`.
Per spec §1–§2. Add `orientation Orientation` (default Horizontal) + `SetOrientation` to both; `solid bool` (default false) + `SetSolid` to ProgressBar. Parameterize the axis (W/X vs H/Y) so thumbCenter/localToValue/Render/Measure share one path. Vertical slider: Max-at-top, fill Min side (below thumb). Vertical progress: fill bottom→top. Solid: single Highlight fill vs chunked.
- [ ] Add orientation/solid fields + setters; parameterize Measure (swap dims for vertical).
- [ ] Slider: axis-generalize thumbCenter/localToValue/Render (Max-at-top vertical). Tests: vertical value↔position mapping (top=Max, bottom=Min), horizontal path unchanged.
- [ ] ProgressBar: orientation + solid in Render (chunked & solid both honor orientation; bottom→top vertical). Tests: solid vs chunked fill length, vertical fill direction.
- [ ] Build green; behavior tests green (existing horizontal/chunked unchanged). Add 3 goldens (`-update`), READ each: vertical slider (raised thumb on vertical groove, navy fill below), vertical progress (chunks/solid bottom-up), solid horizontal progress (single navy fill). Existing goldens UNCHANGED.
- [ ] vet/gofmt clean; grep no new literal colors. Commit `feat(controls): vertical Slider + vertical/solid ProgressBar`.

### Task 2: Pill & Circular Buttons + rounded bevel helpers
**Files:** `controls/bevel.go`, `controls/button.go`, `controls/button_test.go` (or bevel_test); new goldens `button_pill.png`, `button_circle.png`.
Per spec §3. Add `ButtonShape` (`ShapeRect`/`ShapePill`/`ShapeCircle`), `SetShape` on Button + ToggleButton. Add `drawRaisedRounded`/`drawSunkenRounded` to bevel.go (offset rounded-fill layering per spec; clamp `radius-1 >= 0`). Pill radius=H/2, circle radius=min(W,H)/2 + square-aspect Measure. Rounded focus ring (StrokeRoundedRect) for pill/circle; default-border via StrokeRoundedRect for accent. Rect path unchanged.
- [ ] Add `drawRaisedRounded`/`drawSunkenRounded` (+ a small recorder or golden reasoning). 
- [ ] ButtonShape + SetShape; circle square-aspect Measure; radius per shape.
- [ ] Button.Render rounded path (rest/hover/pressed/checked + engrave + rounded focus + accent border). ToggleButton inherits shape.
- [ ] Build green; tests: circle Measure returns square, shape radius, ShapeRect Measure/Render unchanged. Add 2 goldens (`-update`), READ: pill (raised stadium, sunken on press), circle (raised round). Existing button goldens UNCHANGED.
- [ ] vet/gofmt clean; grep no new literals. Commit `feat(controls): pill & circular button shapes (raised-rounded)`.

### Task 3: Horizontal Scroll — ScrollViewer
**Files:** `controls/scrollviewer.go`, `controls/scrollviewer_test.go`; new golden `scroll_horizontal.png`.
Per spec §4. Add `offsetX` alongside vertical offset; clamp per-axis; child arranged at `(-offsetX,-offsetY)` with full desired width; horizontal thumb+track via `drawScrollThumb` when contentW>viewportW (generalize the conditional gutter; square the bottom-right corner when both thumbs show). Shift+wheel→X (plain wheel→X if only X overflows); horizontal thumb drag mirrors vertical.
- [ ] Generalize offset/clamp/arrange to both axes (axis-parameterized helper where practical). Existing vertical-only behavior tests stay green UNCHANGED.
- [ ] Horizontal thumb render + drag + shift-wheel input. Tests: X clamp, offsetX arrange, shift-wheel scrolls X, H-thumb visible iff contentW>viewportW.
- [ ] Build green; add golden `scroll_horizontal.png` (`-update`), READ: horizontal thumb along bottom, content offset. Existing scroll golden UNCHANGED.
- [ ] vet/gofmt clean; grep no new literals. Commit `feat(controls): horizontal scrolling in ScrollViewer`.

### Task 4: Horizontal Scroll — Virtualizer (ListView + DataGrid)
**Files:** `controls/virtualizer.go`, `controls/listview.go`, `controls/datagrid.go`, their `_test.go`; new golden `datagrid_hscroll.png`.
Per spec §5. Virtualizer gains a horizontal `offsetX` + host-provided `contentWidth`; rows/cells render offset `-offsetX`, clipped to the bevel-inset viewport; horizontal thumb via `drawScrollThumb` when contentWidth>viewportW. DataGrid content width = sum(colWidths), header scrolls in sync (same offsetX, vertically fixed). ListView content width = max row width. Shift+wheel→X; H-thumb drag. Draw + hit-test stay in agreement (add `-offsetX` to both). Vertical virtualization + selection unchanged.
- [ ] Virtualizer offsetX + contentWidth plumbing + H-thumb + shift-wheel + clamp.
- [ ] DataGrid: contentWidth=sum(colWidths), header+body share offsetX (header vertically fixed), draw+hit-test offset. Tests: header/body X-sync, H-thumb threshold, hit-test with offsetX.
- [ ] ListView: contentWidth=max row width, draw+hit-test offset. Test: X offset applied.
- [ ] Build green; existing vertical behavior/selection tests UNCHANGED (if a shared-refactor white-box constant shifts, adjudicate like the v0.2 precedent — never weaken an assertion). Add golden `datagrid_hscroll.png` (`-update`), READ: columns scrolled right, header aligned with cells, H-thumb. Existing listview/datagrid goldens UNCHANGED unless a proven mechanical shift.
- [ ] vet/gofmt clean; grep no new literals. Commit `feat(controls): horizontal scrolling in ListView & DataGrid`.

### Task 5: Gallery showcase + CHANGELOG
**Files:** `cmd/fluo-gallery/main.go`, `CHANGELOG.md`.
Add a small showcase of the new variants to a gallery page (a vertical slider, a vertical progress + a solid progress, a pill button + a circle button, and a horizontally-scrollable area). CHANGELOG entry under the next version listing all 6 additions. Gallery compiles + vets clean.
- [ ] Add showcase widgets (use the new setters). CHANGELOG entry.
- [ ] Build gallery + vet clean; full `go test ./...` green. Commit `feat(gallery,docs): showcase control variants + CHANGELOG`.

---

## Self-review (resolved)
- Spec coverage: §1→T1, §2→T1, §3→T2, §4→T3, §5→T4, testing/gallery→T5 (+ per-task goldens/tests). All 6 features mapped.
- Opt-in invariant repeated in every task (existing goldens must not move).
- Types consistent across tasks: `Orientation` (reused), `ButtonShape`/`ShapeRect|Pill|Circle`, `SetOrientation`/`SetSolid`/`SetShape`, `drawRaisedRounded`/`drawSunkenRounded`, virtualizer `offsetX`/`contentWidth`.
