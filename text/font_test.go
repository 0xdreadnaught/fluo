package text

import (
	"math"
	"os"
	"strings"
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

// TestHasGlyph exercises the glyph-presence decision procedure controls
// (e.g. CheckBox's checkmark) use at construction time to choose between
// drawing a real glyph or falling back to a drawn shape. 'A' must be
// present in any real text font; U+E0000 is a Unicode private-use codepoint
// (Supplementary Private Use Area-B) that no font — least of all goregular,
// a plain Latin text face — assigns, giving a reliable "definitely absent"
// case. The test also logs whether goregular has U+2713 (the checkmark
// candidate glyph): as of this font, it does not (glyphIndex resolves to
// the notdef glyph 0), which is the fact that decides CheckBox's checkmark
// fallback path — see controls/checkbox.go.
func TestHasGlyph(t *testing.T) {
	f, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	if !f.HasGlyph('A') {
		t.Error("HasGlyph('A') = false, want true (goregular is a Latin text font)")
	}
	if f.HasGlyph(0xE0000) {
		t.Error("HasGlyph(U+E0000) = true, want false (unassigned private-use codepoint)")
	}
	t.Logf("goregular HasGlyph(U+2713 checkmark) = %v", f.HasGlyph('✓'))
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

// TestRasterGlyphIntegerBearings is the anti-jitter unit test for the fix
// in rasterGlyph: bearingX/bearingY must be whole device pixels for every
// glyph, since Face.Draw snaps the shared device baseline to an integer
// pixel once and then combines it with each glyph's bearingY unrounded — if
// bearingY carried a fractional part (as it did pre-fix, coming straight
// from the glyph's fractional outline minY), different glyphs would land on
// different baseline rows even though the pen's baseline is the same. Covers
// letters and digits (the reported "0123456789 bobbing" symptom) plus a
// couple of ascender/descender-heavy glyphs likely to have very different
// fractional minY/minX.
func TestRasterGlyphIntegerBearings(t *testing.T) {
	f, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range "0123456789AgjQ" {
		gi := mustGlyph(t, f, r)
		mask, bx, by, err := f.rasterGlyph(gi, 37, 4) // odd, non-power-of-two px to surface fractional bounds
		if err != nil {
			t.Fatalf("rasterGlyph(%q): %v", r, err)
		}

		if bx != float32(math.Trunc(float64(bx))) {
			t.Errorf("rasterGlyph(%q): bearingX = %v, not integer", r, bx)
		}
		if by != float32(math.Trunc(float64(by))) {
			t.Errorf("rasterGlyph(%q): bearingY = %v, not integer", r, by)
		}

		b := mask.Bounds()
		if b.Dx() <= 0 || b.Dy() <= 0 {
			t.Errorf("rasterGlyph(%q): empty mask bounds %v", r, b)
		}
		sum := 0
		for _, a := range mask.Pix {
			sum += int(a)
		}
		if sum == 0 {
			t.Errorf("rasterGlyph(%q): mask has no coverage", r)
		}
	}
}

// TestLoadRejectsCollection is the .ttc-detection unit test: Load must
// reject collection data with an error that points the caller at
// LoadCollection, rather than surfacing sfnt.Parse's opaque "invalid single
// font" message. The fabricated header only needs the 4-byte "ttcf" tag
// Load checks for; it never reaches sfnt.Parse.
func TestLoadRejectsCollection(t *testing.T) {
	fake := []byte("ttcf\x00\x00\x00\x00\x00\x00\x00\x00")
	_, err := Load(fake)
	if err == nil {
		t.Fatal("Load(collection bytes) = nil error, want an error directing to LoadCollection")
	}
	if !strings.Contains(err.Error(), "LoadCollection") {
		t.Errorf("Load(collection bytes) error = %q, want it to mention LoadCollection", err)
	}
}

// TestLoadCollectionRealFont exercises LoadCollection and
// LoadCollectionMember against a real .ttc, skipping if one isn't present
// on the machine running the test (we don't want to embed a large font in
// the repo just for this).
func TestLoadCollectionRealFont(t *testing.T) {
	const path = "/mnt/c/Windows/Fonts/msyh.ttc"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("real .ttc not available at %s: %v", path, err)
	}

	fonts, err := LoadCollection(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(fonts) < 1 {
		t.Fatal("LoadCollection: NumFonts < 1")
	}

	// Member 0 of a CJK system face should have (and be able to rasterize)
	// a glyph for a common Han character.
	gi, ok := fonts[0].glyphIndex('中')
	if !ok {
		t.Fatal("msyh.ttc member 0: no glyph for 中")
	}
	mask, _, _, err := fonts[0].rasterGlyph(gi, 24, 2)
	if err != nil {
		t.Fatal(err)
	}
	if b := mask.Bounds(); b.Dx() <= 0 || b.Dy() <= 0 {
		t.Errorf("msyh.ttc member 0: empty raster for 中: %v", b)
	}

	// LoadCollectionMember(data, 0) should agree with LoadCollection's
	// first member.
	member0, err := LoadCollectionMember(data, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := member0.glyphIndex('中'); !ok {
		t.Error("LoadCollectionMember(data, 0): no glyph for 中")
	}
}
