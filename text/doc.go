// Package text provides pure-Go font loading, glyph rasterization, and
// single-line text layout, built on golang.org/x/image/font/sfnt. Load
// parses TTF bytes into a *Font; NewFace pairs a *Font with a pixel size to
// produce a *Face, the type callers actually draw with — Face.Measure sizes
// a string, and Face.Draw emits its glyph quads in a single batched call.
//
// Face.Draw renders HD text: each glyph is rasterized directly at the exact
// device-pixel size for the current frame's scale (grayscale antialiasing),
// cached per (glyph, pixel size) in a coverage Atlas, and drawn with
// baseline and per-glyph origins snapped to whole device pixels via
// render.Renderer.DrawGlyphs. Glyph rasterization is baseline-integer-
// aligned so every glyph on a line shares the same snapped baseline.
//
// Package text has no dependency on any GL/windowing package;
// controls.TextBlock and every text-bearing control build on the Face API
// here.
package text
