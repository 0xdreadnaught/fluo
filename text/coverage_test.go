package text

import (
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

// TestGlyphCoveragePacking is the HD-text coverage-atlas unit test: a
// coverageEntry's device bounds are plausible for the requested px, second
// calls for the same (glyph, px) hit the cache, and a different px yields a
// distinct entry (coverage, unlike SDF, is rasterized directly at its draw
// size rather than resolution-independently scaled).
func TestGlyphCoveragePacking(t *testing.T) {
	f, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	a := NewAtlas(f)
	gi, _ := f.glyphIndex('A')

	e1, err := a.glyphCoverage(gi, 16)
	if err != nil {
		t.Fatal(err)
	}
	if e1.w <= 0 || e1.h <= 0 {
		t.Fatalf("glyphCoverage(A, 16): non-positive size %dx%d", e1.w, e1.h)
	}
	// A capital 'A' at 16px should be within a generous device-bounds
	// envelope (rasterGlyph's pad ring adds a couple of pixels on each
	// side); this just guards against a gross unit error (e.g. accidental
	// scaling), not exact glyph metrics.
	if e1.w > 32 || e1.h > 32 {
		t.Errorf("glyphCoverage(A, 16): implausibly large %dx%d", e1.w, e1.h)
	}
	if e1.uv.W <= 0 || e1.uv.Right() > 1 || e1.uv.Bottom() > 1 {
		t.Errorf("glyphCoverage(A, 16): bad uv %v", e1.uv)
	}

	e2, err := a.glyphCoverage(gi, 16)
	if err != nil {
		t.Fatal(err)
	}
	if e1 != e2 {
		t.Error("glyphCoverage(A, 16) not cached on second call")
	}

	e3, err := a.glyphCoverage(gi, 32)
	if err != nil {
		t.Fatal(err)
	}
	if e3 == e1 {
		t.Error("glyphCoverage(A, 32) should differ from glyphCoverage(A, 16)")
	}
	if e3.w <= e1.w || e3.h <= e1.h {
		t.Errorf("glyphCoverage(A, 32) = %dx%d should be larger than px=16's %dx%d", e3.w, e3.h, e1.w, e1.h)
	}
}
