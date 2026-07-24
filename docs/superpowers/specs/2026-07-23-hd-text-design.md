# fluo HD Text — Direct Grayscale-AA Glyph Rendering — Design

**Date:** 2026-07-23
**Status:** Approved design, pre-implementation
**One-liner:** Replace SDF-scaled UI text with direct grayscale-antialiased glyphs rasterized at the exact on-screen device-pixel size and pixel-snapped, for crisp "HD" text; keep the SDF path available for future scaled/animated text.

## Motivation

UI text renders soft and mushy with clipped/dropped thin features. Root cause (three compounding issues in the current pipeline):
1. Every glyph is rasterized once at a fixed `sdfRasterPx = 48` then scaled — body text (14px) is a 3.4× *down*-scale.
2. `sdfFromMask` extracts the distance field from a **1-bit** coverage threshold (alpha ≥ 128), discarding the rasterizer's antialiasing — the true sub-pixel edge position is lost, so curves staircase and thin features (crossbars, tails, dots) drop out.
3. No mipmaps — bilinear minification of that imprecise SDF softens edges further.

Direct grayscale AA at the display size is how native Windows renders text and is the most authentic match for the classic (Win2000) look, which never used SDF.

## Locked Decisions

1. **Direct grayscale AA** for UI text: rasterize each glyph at its actual on-screen device-pixel size, grayscale coverage, sampled ~1:1.
2. **Keep the SDF machinery** (`sdf.go`, `DrawSDFQuads`, shader SDF mode) available for future scaled/animated text — just not used by `Face.Draw`'s default path. (Partially reverses the original locked "SDF text" decision; SDF retained, not deleted.)
3. **Preserve the one-scale-point invariant**: device-pixel conversion still happens exactly once, on the GPU via `uScale` at `Begin`. The text layer authors in logical px; it reads the frame scale only to choose the raster resolution and to pixel-snap.
4. **Pixel-snap** baseline and per-glyph pen X to whole device pixels.
5. **Layout unchanged**: advances, kerning, and `Measure` stay in logical px — no geometry/layout change; behavior tests hold. Only rendered glyph pixels change.

## Architecture

### Scale access (renderer interface)
Add to `render.Renderer`:
```go
// Scale returns the current frame's device-pixels-per-logical-pixel factor
// (as passed to Begin). Valid between Begin and End. The text layer uses it
// to rasterize glyphs at device resolution and to pixel-snap; no other code
// should multiply by scale (the vertex shader applies uScale exactly once).
Scale() float32
```
`gl.Renderer` returns `rd.scale`. The two test mock renderers gain a `Scale()` returning a default of 1.

### Grayscale glyph cache (`text` package)
`rasterGlyph` already returns a grayscale `*image.Alpha` coverage mask + bearings. The crisp path packs that mask **directly** (no `sdfFromMask`). Entries are keyed by `(glyphIndex, pixelPx)` where `pixelPx = int(round(SizePx * scale))`.

Add a coverage atlas keyed by size. Concretely:
- New `coverageEntry` mirroring `atlasEntry` but for a coverage mask: `uv render.Rect`, `w,h int`, `bearingX, bearingY float32` (in **device px** at this pixelPx), plus the advance is NOT stored here (advances come from the logical-px metrics path, unchanged).
- The `Atlas` gains a coverage map keyed by a `struct{ gi sfnt.GlyphIndex; px int }` (or a nested map). It packs coverage masks with the same shelf packer + `atlasSize=1024` backing image, on the **same** atlas texture as before OR a dedicated coverage atlas — implementer's choice, but SDF and coverage entries must not collide in the packed image. Recommended: a dedicated coverage image+texture in `Atlas` (fields `covImg`, `covTex`, `covCursorX/Y`, `covRowH`, `covEntries`) so the two systems are independent and neither disturbs the other's goldens/packing.
- `glyphCoverage(gi sfnt.GlyphIndex, px int) (coverageEntry, error)`: raster at `px` (a small integer point size) via `rasterGlyph(gi, float32(px), covPad)`, pack the raw mask, cache. `covPad` is a small padding (e.g. 1px) so bilinear sampling at the atlas seam doesn't bleed a neighbor; the coverage mask from `rasterGlyph` already includes `pad` transparent border, so `covPad` can be 0 and rely on rasterGlyph's own pad — implementer picks, but there MUST be ≥1px transparent separation between packed coverage glyphs. Document the choice.
- Upload path mirrors `ensureTexture` (coverage → RGBA where all channels = coverage, or store coverage in .r and let the shader read .r; match whatever the shader mode reads).

### Coverage upload format
The shader coverage mode reads `texture(uTex, vUV).r`. The upload writes the coverage byte into at least the `.r` channel (writing it into all 4, as `ensureTexture` already does, is fine and keeps one upload routine).

### Shader — new coverage-text mode
Add a fragment mode (next free integer, e.g. `mode == 7`) to `render/gl/shader.go`:
```glsl
} else if (vMode == 7) {          // coverage-AA text
    c.a *= texture(uTex, vUV).r;
}
```
No smoothstep, no fwidth — the grayscale coverage already IS the antialiasing. Placed so it does not disturb existing modes (1 textured, 2 SDF text, 3–6 shapes/acrylic). Update the mode dispatch chain accordingly (it currently uses `vMode >= 3` for the shape block; keep 7 out of that block — handle it in the `== ` ladder before the `>= 3` branch, or restructure cleanly).

### Renderer — coverage draw method
Add to `render.Renderer` and `gl.Renderer`:
```go
// DrawGlyphs draws grayscale-coverage glyph quads from a coverage atlas
// (alpha = texture.r) tinted with c. Quads' Dst is in logical px (the
// vertex shader applies uScale); Src is 0..1 atlas UV. Used by text.Face
// for crisp UI text. Distinct from DrawSDFQuads (SDF path, retained).
DrawGlyphs(quads []GlyphQuad, tex TextureID, c Color)
```
`gl.Renderer.DrawGlyphs` emits each quad with `rd.quad(7, ...)`. The two mock renderers get a no-op.

### `Face.Draw` — crisp path
Rewrite `Face.Draw` to:
1. `scale := r.Scale()` (fallback to 1 if ≤ 0).
2. `px := int(round(fa.SizePx * scale))`, clamped to ≥ 1.
3. Baseline in logical px: `baseline := at.Y + fa.Ascent()`; snap to device: `baseline = round(baseline*scale)/scale`.
4. Pen X starts at `at.X`. For each rune:
   - advance/kern computed from the **logical-px** metrics path (unchanged: `fa.Font.advance(gi, fa.SizePx)`, `fa.Font.kern(prev, gi, fa.SizePx)`).
   - snap the glyph's device origin: `gx := round((penX + covEntry.bearingX/scale) * scale) / scale` — i.e. compute the logical draw X of the glyph's left edge, snap it to a device pixel. (bearingX from the coverage entry is in device px at `px`; convert to logical by `/scale`.) Equivalent snapping for the quad's Y from the snapped baseline + `bearingY/scale`.
   - quad `Dst = { X: gx, Y: gy, W: covEntry.w/scale, H: covEntry.h/scale }`, `Src = covEntry.uv`. (`w/scale` logical × `scale` on GPU = `w` device px = raster px → 1:1.)
   - `penX += advance` (+ kern before). **penX is NOT snapped cumulatively** — only each glyph's draw origin is snapped, so accumulated advance error does not compound. (Document this: layout width via Measure stays exact/logical; only per-glyph draw origins snap.)
5. Skip glyphs with empty coverage (space) but still advance.
6. `r.DrawGlyphs(quads, atlas.ensureCoverageTexture(r), c)`.

`Measure`, `LineHeight`, `Ascent` unchanged (logical px).

### What stays
- `sdf.go`, `sdfFromMask`, `sdfRasterPx`, `sdfPad`, `DrawSDFQuads`, shader mode 2 — all retained for future scaled/animated text. The existing SDF golden(s) that exercise `DrawSDFQuads` directly (e.g. `text.png`/`text_2x.png` if they call the SDF path) stay valid IF they call `DrawSDFQuads`; if they call `Face.Draw` they will change to crisp — see Testing.

## Testing

- **Behavior/layout tests**: unchanged (Measure/advance/kern logic untouched). Must stay green without edits.
- **Goldens**: `Face.Draw` output changes for every text-bearing golden — the two text goldens (`text.png`, `text_2x.png`) plus effectively every control golden (buttons/listview/datagrid/tabs/menu/dialog/titlebar/textbox/combo/tree/etc. all contain text). Regenerate ALL and human-inspect each for crispness (no clipping, clean edges). This is a large but expected regen.
- **New tests**:
  - A coverage-atlas unit test: `glyphCoverage(gi, px)` returns an entry whose `w/h` ≈ the glyph's device bounds at `px`, caches on second call, and different `px` yields distinct entries.
  - A `Face.Draw` test (via a recording renderer) asserting: it calls `DrawGlyphs` (not `DrawSDFQuads`); the quad count equals the visible-glyph count; a quad's device origin (`Dst.X*scale`) is integer-valued (pixel-snapped) at scale 1 and 2.
  - A golden `hdtext.png`: a line of text rendered via `Face.Draw` at scale 1 AND the same at scale 2 (two rows), inspected for crisp edges.
- **`Scale()`**: trivial test that `Begin(w,h,s)` then `Scale()==s` on the gl renderer is covered indirectly; the mocks return 1.

## Non-Goals / Deferred

- Gamma-correct (linear-space) AA blending — current renderer blends straight alpha everywhere; text matches. Note as a future refinement.
- Glyph hinting / grid-fitting — the sfnt vector rasterizer is unhinted; grayscale AA at exact px is already a large improvement.
- Sub-pixel (LCD/ClearType) AA — out of scope.
- Atlas growth/eviction beyond the existing 1024² + "atlas full" error — deferred as before; coverage across (glyph × size) increases atlas pressure, so `log`/note if it ever fills (still just the existing error).
- Removing the SDF path — explicitly retained.

## Migration Notes

- `render.Renderer` gains `Scale() float32` and `DrawGlyphs(...)` — breaking for external Renderer impls (none exist; two internal test mocks updated).
- No public API change to `text.Face` (same `Draw`/`Measure` signatures) or to controls.
- README/CHANGELOG: note HD text (direct grayscale AA) under the next version; mention SDF retained for scaled text.
