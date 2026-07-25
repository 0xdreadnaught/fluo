package text

import (
	"math"

	"golang.org/x/image/font/sfnt"

	"github.com/0xdreadnaught/fluo/render"
)

// Face renders text from a Font at a fixed pixel size. The glyph
// coverage masks it draws from (see Draw) live in Font's shared Atlas
// (see Font.sharedAtlas), so creating many Faces at different sizes for
// the same Font does not duplicate the Atlas itself, though each
// distinct (glyph, device px) pair is rasterized once at that size (see
// Atlas.glyphCoverage) since — unlike the retained SDF path — coverage
// masks aren't resolution-independent.
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

// Draw renders s with fa, with at as the top-left corner of the text box;
// the baseline sits at at.Y + fa.Ascent(). This is the crisp HD-text path:
// each glyph is rasterized directly (grayscale-AA coverage, no SDF) at the
// exact device-pixel size for the current frame's scale (from r.Scale()),
// and both the baseline and each glyph's draw origin are snapped to whole
// device pixels for a sharp result. Layout stays exact: advances and
// kerning are computed from the unsnapped logical-px metrics path and the
// pen is never snapped cumulatively, only each glyph's own draw origin —
// so accumulated advance/Measure width never drifts from the snapped
// pixels actually drawn. Glyph quads are gathered from the Font's shared
// coverage atlas and submitted to r in a single DrawGlyphs batch. Runes
// with no glyph in the font fall back to glyph index 0 (.notdef); glyphs
// with no visible coverage (e.g. space) are skipped but still advance the
// pen.
func (fa *Face) Draw(r render.Renderer, at render.Point, s string, c render.Color) {
	atlas := fa.Font.sharedAtlas()

	scale := r.Scale()
	if scale <= 0 {
		scale = 1
	}
	px := int(math.Round(float64(fa.SizePx) * float64(scale)))
	if px < 1 {
		px = 1
	}

	// Baseline in logical px, snapped to a whole device pixel.
	baseline := at.Y + fa.Ascent()
	baseline = float32(math.Round(float64(baseline)*float64(scale))) / scale

	var quads []render.GlyphQuad
	penX := at.X // logical px; advances by the UNSNAPPED metric each glyph, never snapped itself
	var prev sfnt.GlyphIndex
	hasPrev := false
	for _, ch := range s {
		gi, _ := fa.Font.glyphIndex(ch)
		if hasPrev {
			penX += fa.Font.kern(prev, gi, fa.SizePx)
		}

		if e, err := atlas.glyphCoverage(gi, px); err == nil && !e.empty {
			// e.bearing{X,Y} are device px at this entry's px size;
			// convert to logical (/scale) before combining with the
			// logical pen/baseline, then snap the glyph's own draw
			// origin to a whole device pixel.
			gx := float32(math.Round(float64(penX+e.bearingX/scale)*float64(scale))) / scale
			gy := float32(math.Round(float64(baseline+e.bearingY/scale)*float64(scale))) / scale
			quads = append(quads, render.GlyphQuad{
				Dst: render.Rect{
					X: gx,
					Y: gy,
					W: float32(e.w) / scale,
					H: float32(e.h) / scale,
				},
				Src: e.uv,
			})
		}

		penX += fa.Font.advance(gi, fa.SizePx)
		prev, hasPrev = gi, true
	}

	if len(quads) > 0 {
		r.DrawGlyphs(quads, atlas.ensureCoverageTexture(r), c)
	}
}
