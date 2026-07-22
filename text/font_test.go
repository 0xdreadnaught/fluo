package text

import (
	"testing"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"
)

// mustGlyph resolves r to a glyph index or fails the test.
func mustGlyph(t *testing.T, f *Font, r rune) sfnt.GlyphIndex {
	t.Helper()
	gi, ok := f.glyphIndex(r)
	if !ok {
		t.Fatalf("no glyph for %q", r)
	}
	return gi
}

func TestLoadAndMetrics(t *testing.T) {
	f, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	asc, desc, _ := f.metrics(16)
	if asc <= 8 || asc >= 20 || desc <= 0 {
		t.Errorf("asc=%v desc=%v", asc, desc)
	}
	if _, ok := f.glyphIndex('A'); !ok {
		t.Error("no glyph for A")
	}
	if a := f.advance(mustGlyph(t, f, 'M'), 16); a <= 4 || a >= 20 {
		t.Errorf("advance=%v", a)
	}
}

func TestRasterGlyph(t *testing.T) {
	f, _ := Load(goregular.TTF)
	mask, bx, by, err := f.rasterGlyph(mustGlyph(t, f, 'A'), 48, 4)
	if err != nil {
		t.Fatal(err)
	}
	b := mask.Bounds()
	if b.Dx() < 20 || b.Dy() < 30 {
		t.Errorf("mask too small: %v", b)
	}
	sum := 0
	for _, a := range mask.Pix {
		sum += int(a)
	}
	if sum == 0 {
		t.Error("mask is empty")
	}
	if by >= 0 {
		t.Errorf("bearingY should be negative (above baseline), got %v", by)
	}
	_ = bx
}
