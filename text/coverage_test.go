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

// TestGlyphCoveragePageGrowth is the atlas-growth unit test: once the
// current coverage page has no room left for a new glyph, glyphCoverage
// must allocate a fresh page and pack the glyph there — instead of the
// old fixed 1024x1024 hard cap that reported "atlas full" and dropped the
// glyph.
func TestGlyphCoveragePageGrowth(t *testing.T) {
	f, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	a := NewAtlas(f)

	giA, _ := f.glyphIndex('A')
	eA, err := a.glyphCoverage(giA, 16)
	if err != nil {
		t.Fatal(err)
	}
	if eA.page != 0 {
		t.Fatalf("first glyph landed on page %d, want 0", eA.page)
	}
	if len(a.covPages) != 1 {
		t.Fatalf("covPages = %d after one glyph, want 1", len(a.covPages))
	}

	// Push page 0's shelf packer past its bottom edge, as if many glyphs
	// had already filled it, without needing to actually rasterize
	// thousands of real glyphs to do so.
	a.covPages[0].cursorY = atlasSize
	a.covPages[0].rowH = 0

	giB, _ := f.glyphIndex('B')
	eB, err := a.glyphCoverage(giB, 16)
	if err != nil {
		t.Fatalf("glyphCoverage(B) after page 0 full: %v, want a new page grown instead of an error", err)
	}
	if eB.page != 1 {
		t.Errorf("glyphCoverage(B) landed on page %d, want 1 (freshly grown page)", eB.page)
	}
	if len(a.covPages) != 2 {
		t.Errorf("covPages = %d, want 2 after growth", len(a.covPages))
	}
	if eB.uv.W <= 0 || eB.uv.Right() > 1 || eB.uv.Bottom() > 1 {
		t.Errorf("glyphCoverage(B) on grown page: bad uv %v", eB.uv)
	}

	// The already-packed 'A' entry (on page 0) must be unaffected by the
	// growth: still cached, still reporting page 0.
	eA2, err := a.glyphCoverage(giA, 16)
	if err != nil {
		t.Fatal(err)
	}
	if eA2 != eA {
		t.Errorf("glyphCoverage(A, 16) changed after growth: %v, want unchanged %v", eA2, eA)
	}
}

// TestGlyphCoverageTooLargeForPage is the degenerate-glyph unit test: a
// glyph rasterized so large it cannot fit on any page — bigger than a
// whole empty 1024x1024 page — is the one case page growth can't fix, so
// glyphCoverage must still report an error for it instead of growing
// pages forever.
func TestGlyphCoverageTooLargeForPage(t *testing.T) {
	f, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	a := NewAtlas(f)
	gi, _ := f.glyphIndex('A')

	if _, err := a.glyphCoverage(gi, atlasSize*3); err == nil {
		t.Error("glyphCoverage at an oversized px: want an error (glyph larger than a whole page), got nil")
	}
	if len(a.covPages) != 1 {
		t.Errorf("covPages = %d, want 1 (no page grown for a glyph that can never fit)", len(a.covPages))
	}
}
