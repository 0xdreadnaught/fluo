// Package text provides pure-Go font loading, SDF glyph rasterization, and
// single-line text layout, built on golang.org/x/image/font/sfnt. Load
// parses TTF bytes into a *Font, which lazily rasterizes each glyph it is
// asked for into a shared signed-distance-field Atlas (one GPU texture per
// Font, reused across every size); NewFace pairs a *Font with a pixel size
// to produce a *Face, the type callers actually draw with — Face.Measure
// sizes a string, and Face.Draw batches its glyph quads into a single
// render.Renderer.DrawSDFQuads call. Package text has no dependency on any
// GL/windowing package; controls.TextBlock and every text-bearing control
// build on the Face API here.
package text
