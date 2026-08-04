package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
)

// TestGlyphDrawSnapsToPixelGridAt1x is the subpixel-positioning check: does a
// fractional widget origin make text render blurred/ghosted at 1:1? It does
// NOT, because face.Draw already snaps every glyph's draw origin to a whole
// device pixel (gx/gy = round(pen*scale)/scale). Drawing the same string from
// an integer pen and from a half-pixel pen must, at scale 1, emit glyph quads
// whose Dst origins are all whole integers — so a single fluo text draw is
// always pixel-crisp regardless of a fractional container position, and cannot
// ghost from subpixel placement.
func TestGlyphDrawSnapsToPixelGridAt1x(t *testing.T) {
	face := buttonFace(t)
	black := render.Color{A: 255}

	var atInt, atHalf recordRenderer
	face.Draw(&atInt, render.Point{X: 100, Y: 10}, "Ag", black)
	face.Draw(&atHalf, render.Point{X: 100.5, Y: 10.5}, "Ag", black)

	if len(atInt.glyphs) == 0 {
		t.Fatal("fixture: no glyphs drawn")
	}
	for label, rec := range map[string]recordRenderer{"integer-origin": atInt, "half-pixel-origin": atHalf} {
		for i, q := range rec.glyphs {
			if q.Dst.X != float32(int(q.Dst.X)) || q.Dst.Y != float32(int(q.Dst.Y)) {
				t.Fatalf("%s glyph %d Dst origin not pixel-snapped: X=%v Y=%v (a fractional widget position must still snap to the grid)",
					label, i, q.Dst.X, q.Dst.Y)
			}
		}
	}
}

// TestDoubleMeasureDoesNotDoubleGlyphs is the reproducer for the "ghosted /
// double-exposed drag-bar title" investigation. A consumer that centers a
// panel on open runs a manual core.MeasureWidget pre-pass BEFORE the normal
// per-frame measure/arrange/draw, so a title TextBlock is MEASURED TWICE in one
// frame. The hypothesis under test: a second measure accumulates glyph quads,
// so the title draws doubled.
//
// It does not. fluo's measure path is pure — TextBlock.MeasureContent returns
// face.Measure (a memoized Size, no quads), and core.MeasureWidget only sets
// the desired size (no draw, no glyph build). face.Draw builds its glyph
// batches in a fresh LOCAL slice per call. So measuring N times then drawing
// once emits exactly one set of glyphs; the ghost can only come from a double
// DRAW, which this test also demonstrates as the contrast.
func TestDoubleMeasureDoesNotDoubleGlyphs(t *testing.T) {
	tb := NewTextBlock(buttonFace(t), "Augmentations")
	bounds := render.Rect{X: 0, Y: 0, W: 200, H: 30}
	layoutButton(tb, bounds)

	// Baseline: one measure, one draw.
	core.MeasureWidget(tb, render.Size{W: bounds.W, H: bounds.H})
	var once recordRenderer
	core.RenderWidget(tb, &once)
	base := once.glyphQuads
	if base == 0 {
		t.Fatal("fixture: the face drew no glyphs — test would be vacuous")
	}

	// The centering pre-pass: MEASURE the same title again (twice this frame)
	// before the single normal draw. A pure/idempotent measure emits the SAME
	// glyph count — no accumulation.
	core.MeasureWidget(tb, render.Size{W: bounds.W, H: bounds.H})
	core.MeasureWidget(tb, render.Size{W: bounds.W, H: bounds.H})
	var doubleMeasured recordRenderer
	core.RenderWidget(tb, &doubleMeasured)
	if doubleMeasured.glyphQuads != base {
		t.Fatalf("glyph quads after DOUBLE-MEASURE = %d, want %d — measure must not accumulate glyphs (hypothesis refuted only if equal)",
			doubleMeasured.glyphQuads, base)
	}

	// Contrast: a genuine double DRAW is what ghosts — it emits twice the
	// glyphs. This pins the doubling to draw count, not measure count, so the
	// ghost originates wherever the title's Render is invoked twice per frame,
	// not in fluo's measure.
	var doubleDrawn recordRenderer
	core.RenderWidget(tb, &doubleDrawn)
	core.RenderWidget(tb, &doubleDrawn)
	if doubleDrawn.glyphQuads != 2*base {
		t.Fatalf("glyph quads after DOUBLE-DRAW = %d, want %d (a real double-draw is the ghost mechanism)",
			doubleDrawn.glyphQuads, 2*base)
	}
}
