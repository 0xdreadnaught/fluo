package text

import (
	"golang.org/x/image/font/sfnt"

	"github.com/0xdreadnaught/fluo/render"
)

// Face renders text from a Font at a fixed pixel size. The glyph SDFs
// it draws from live in Font's shared Atlas (see Font.sharedAtlas), so
// creating many Faces at different sizes for the same Font does not
// duplicate rasterization work; only the per-size layout math below
// (advances, kerning, bearing scale) varies with SizePx.
type Face struct {
	Font   *Font
	SizePx float32
}

// NewFace returns a Face for f at sizePx, ensuring f's shared glyph
// atlas exists.
func NewFace(f *Font, sizePx float32) *Face {
	f.sharedAtlas()
	return &Face{Font: f, SizePx: sizePx}
}

// LineHeight returns the recommended distance between successive
// baselines when text is set with fa: ascent + descent + lineGap, all
// at fa.SizePx.
func (fa *Face) LineHeight() float32 {
	ascent, descent, lineGap := fa.Font.metrics(fa.SizePx)
	return ascent + descent + lineGap
}

// Ascent returns the distance from the baseline up to the top of the
// line at fa.SizePx.
func (fa *Face) Ascent() float32 {
	ascent, _, _ := fa.Font.metrics(fa.SizePx)
	return ascent
}

// Measure returns the size s occupies when drawn with fa: width is the
// sum of glyph advances plus kerning between consecutive glyphs,
// height is LineHeight (text is laid out on a single line for now).
// Runes with no glyph in the font fall back to glyph index 0
// (.notdef), matching Draw.
func (fa *Face) Measure(s string) render.Size {
	var w float32
	var prev sfnt.GlyphIndex
	hasPrev := false
	for _, r := range s {
		gi, _ := fa.Font.glyphIndex(r)
		if hasPrev {
			w += fa.Font.kern(prev, gi, fa.SizePx)
		}
		w += fa.Font.advance(gi, fa.SizePx)
		prev, hasPrev = gi, true
	}
	return render.Size{W: w, H: fa.LineHeight()}
}

// Draw renders s with fa, with at as the top-left corner of the text
// box; the baseline sits at at.Y + fa.Ascent(). Glyph quads are
// gathered from the Font's shared SDF atlas and submitted to r in a
// single DrawSDFQuads batch. Runes with no glyph in the font fall back
// to glyph index 0 (.notdef); glyphs with no visible mask (e.g. space)
// are skipped but still advance the pen.
func (fa *Face) Draw(r render.Renderer, at render.Point, s string, c render.Color) {
	atlas := fa.Font.sharedAtlas()
	k := fa.SizePx / sdfRasterPx

	var quads []render.GlyphQuad
	penX := at.X
	baseline := at.Y + fa.Ascent()
	var prev sfnt.GlyphIndex
	hasPrev := false
	for _, ch := range s {
		gi, _ := fa.Font.glyphIndex(ch)
		if hasPrev {
			penX += fa.Font.kern(prev, gi, fa.SizePx)
		}

		if e, err := atlas.glyph(gi); err == nil && e.w > 0 {
			quads = append(quads, render.GlyphQuad{
				Dst: render.Rect{
					X: penX + e.bearingX*k,
					Y: baseline + e.bearingY*k,
					W: float32(e.w) * k,
					H: float32(e.h) * k,
				},
				Src: e.uv,
			})
		}

		penX += fa.Font.advance(gi, fa.SizePx)
		prev, hasPrev = gi, true
	}

	if len(quads) > 0 {
		r.DrawSDFQuads(quads, atlas.ensureTexture(r), c)
	}
}
