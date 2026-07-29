package text

import (
	"math"
	"os"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"

	"github.com/0xdreadnaught/fluo/render"
)

// cjkFallbackPath is the real .ttc used to exercise Face's fallback chain
// against an actual font that covers Han characters goregular (a Latin-only
// text face) lacks — see TestLoadCollectionRealFont in font_test.go for the
// same skip-if-missing pattern (we don't embed a large font in the repo
// just for these tests).
const cjkFallbackPath = "/mnt/c/Windows/Fonts/msyh.ttc"

// loadCJKFallback loads member 0 of cjkFallbackPath, skipping the calling
// test if the file isn't present on the machine running it.
func loadCJKFallback(t *testing.T) *Font {
	t.Helper()
	data, err := os.ReadFile(cjkFallbackPath)
	if err != nil {
		t.Skipf("real .ttc not available at %s: %v", cjkFallbackPath, err)
	}
	f, err := LoadCollectionMember(data, 0)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// recordingRenderer is a minimal render.Renderer stub for exercising
// Face.Draw's crisp HD-text path without a GPU: it records DrawGlyphs and
// DrawSDFQuads calls (so a test can assert which one Draw actually used)
// and reports a fixed scale, matching how app.Surface would report the
// current frame's device-pixels-per-logical-pixel factor. Every other
// method is a no-op; Draw only calls Scale, DrawGlyphs, and (were it ever
// reached) DrawSDFQuads.
type recordingRenderer struct {
	scale      float32
	glyphCalls int
	glyphQuads []render.GlyphQuad
	sdfCalls   int
}

func (r *recordingRenderer) Begin(fbWidth, fbHeight int, scale float32)                       {}
func (r *recordingRenderer) End()                                                             {}
func (r *recordingRenderer) FillRect(rect render.Rect, c render.Color)                        {}
func (r *recordingRenderer) FillRoundedRect(rect render.Rect, radius float32, c render.Color) {}
func (r *recordingRenderer) DrawGradientRect(rect render.Rect, from, to render.Color, horizontal bool) {
}
func (r *recordingRenderer) StrokeRoundedRect(rect render.Rect, radius, width float32, c render.Color) {
}
func (r *recordingRenderer) DrawShadow(rect render.Rect, radius, blur float32, c render.Color) {}
func (r *recordingRenderer) DrawBackdropBlur(rect render.Rect, radius float32, tint render.Color) {
}
func (r *recordingRenderer) CreateTexture(w, h int, rgba []byte) render.TextureID           { return 1 }
func (r *recordingRenderer) UpdateTexture(id render.TextureID, x, y, w, h int, rgba []byte) {}
func (r *recordingRenderer) DeleteTexture(id render.TextureID)                              {}
func (r *recordingRenderer) DrawQuad(dst, src render.Rect, tex render.TextureID, tint render.Color) {
}
func (r *recordingRenderer) DrawSDFQuads(quads []render.GlyphQuad, tex render.TextureID, c render.Color) {
	r.sdfCalls++
}
func (r *recordingRenderer) DrawGlyphs(quads []render.GlyphQuad, tex render.TextureID, c render.Color) {
	r.glyphCalls++
	r.glyphQuads = quads
}
func (r *recordingRenderer) Scale() float32 {
	if r.scale <= 0 {
		return 1
	}
	return r.scale
}
func (r *recordingRenderer) PushClip(rect render.Rect) {}
func (r *recordingRenderer) PopClip()                  {}

var _ render.Renderer = (*recordingRenderer)(nil)

// visibleGlyphCount returns the number of runes of s that have non-empty
// coverage at fa's device px for the given scale — i.e. the quad count
// Face.Draw is expected to emit (spaces and other blank glyphs advance the
// pen but contribute no quad).
func visibleGlyphCount(t *testing.T, fa *Face, s string, scale float32) int {
	t.Helper()
	atlas := fa.Font.sharedAtlas()
	px := int(fa.SizePx*scale + 0.5)
	if px < 1 {
		px = 1
	}
	n := 0
	for _, ch := range s {
		gi, _ := fa.Font.glyphIndex(ch)
		e, err := atlas.glyphCoverage(gi, px)
		if err == nil && !e.empty {
			n++
		}
	}
	return n
}

// TestFaceDrawUsesDrawGlyphs is the HD-text Face.Draw test: it asserts Draw
// calls DrawGlyphs (never DrawSDFQuads — SDF is retained for future
// scaled/animated text but is no longer Face.Draw's default path), the
// emitted quad count equals the visible-glyph count (spaces skipped), and
// every quad's device origin (Dst.X*scale, Dst.Y*scale) is pixel-snapped
// (integer-valued, within float rounding) at both scale 1 and scale 2.
func TestFaceDrawUsesDrawGlyphs(t *testing.T) {
	f, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	fa := NewFace(f, 14)
	const s = "Hi fluo"

	for _, scale := range []float32{1, 2} {
		rr := &recordingRenderer{scale: scale}
		fa.Draw(rr, render.Point{X: 3.25, Y: 5.5}, s, render.RGB(255, 255, 255))

		if rr.sdfCalls != 0 {
			t.Errorf("scale=%v: DrawSDFQuads called %d times; Face.Draw must use DrawGlyphs, not the SDF path", scale, rr.sdfCalls)
		}
		if rr.glyphCalls != 1 {
			t.Fatalf("scale=%v: DrawGlyphs called %d times, want 1", scale, rr.glyphCalls)
		}

		want := visibleGlyphCount(t, fa, s, scale)
		if len(rr.glyphQuads) != want {
			t.Errorf("scale=%v: quad count = %d, want %d (visible glyphs)", scale, len(rr.glyphQuads), want)
		}

		for i, q := range rr.glyphQuads {
			dx, dy := q.Dst.X*scale, q.Dst.Y*scale
			if rdx := dx - float32(int(dx+0.5)); rdx > 1e-3 || rdx < -1e-3 {
				t.Errorf("scale=%v quad[%d]: Dst.X*scale = %v not pixel-snapped", scale, i, dx)
			}
			if rdy := dy - float32(int(dy+0.5)); rdy > 1e-3 || rdy < -1e-3 {
				t.Errorf("scale=%v quad[%d]: Dst.Y*scale = %v not pixel-snapped", scale, i, dy)
			}
		}
	}
}

// TestFaceDrawBaselineConsistent is the anti-jitter regression test: it
// renders a string of uniform-height digits (plus a mixed-case string with
// ascenders/descenders) and asserts every emitted glyph quad shares the same
// integer device baseline. Before the rasterGlyph integer-bearing fix, each
// glyph's bearingY carried the outline's fractional minY, so
// quad.Dst.Y*scale - bearingY (device baseline) varied by up to 1px between
// glyphs even though Face.Draw computes a single shared baseline for the
// whole string — the reported "0123456789 bobbing" bug. Checked at scale 1
// and scale 2 since px (and so which coverage-atlas entries are used) is
// recomputed per scale.
func TestFaceDrawBaselineConsistent(t *testing.T) {
	f, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	fa := NewFace(f, 16)
	atlas := fa.Font.sharedAtlas()

	for _, s := range []string{"0123456789", "Hi fluo Qgjy"} {
		for _, scale := range []float32{1, 2} {
			rr := &recordingRenderer{scale: scale}
			fa.Draw(rr, render.Point{X: 2, Y: 3}, s, render.RGB(255, 255, 255))

			px := int(fa.SizePx*scale + 0.5)
			if px < 1 {
				px = 1
			}

			var wantBaseline float32
			have := false
			qi := 0
			for _, ch := range s {
				gi, _ := fa.Font.glyphIndex(ch)
				e, err := atlas.glyphCoverage(gi, px)
				if err != nil || e.empty {
					continue
				}
				if qi >= len(rr.glyphQuads) {
					t.Fatalf("s=%q scale=%v: ran out of quads at visible glyph %d", s, scale, qi)
				}
				q := rr.glyphQuads[qi]
				baseline := q.Dst.Y*scale - e.bearingY

				if rounded := float32(math.Round(float64(baseline))); baseline-rounded > 1e-3 || baseline-rounded < -1e-3 {
					t.Errorf("s=%q scale=%v glyph %q: device baseline %v not integer", s, scale, ch, baseline)
				}
				if !have {
					wantBaseline, have = baseline, true
				} else if baseline-wantBaseline > 1e-3 || baseline-wantBaseline < -1e-3 {
					t.Errorf("s=%q scale=%v glyph %q: device baseline = %v, want %v (same as other glyphs in the string)", s, scale, ch, baseline, wantBaseline)
				}
				qi++
			}
		}
	}
}

// TestFaceOnGlyphDroppedAtlasFull is the atlas-full-surfacing unit test.
// Actually filling the 1024x1024 coverage atlas would take thousands of
// distinct real glyphs, so instead this forces the atlas into the "full"
// state directly (same package, so the shelf-packer fields are reachable)
// and asserts Draw reports each distinct dropped rune to OnGlyphDropped
// exactly once, even though 'A' appears twice in the drawn string.
func TestFaceOnGlyphDroppedAtlasFull(t *testing.T) {
	f, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	fa := NewFace(f, 16)
	atlas := fa.Font.sharedAtlas()

	// Push the coverage shelf packer past the atlas's bottom edge so every
	// subsequent glyphCoverage call fails with "atlas full", without
	// needing to actually rasterize enough real glyphs to fill it.
	atlas.covCursorY = atlasSize
	atlas.covRowH = 0

	var dropped []rune
	fa.OnGlyphDropped = func(r rune) { dropped = append(dropped, r) }

	rr := &recordingRenderer{scale: 1}
	fa.Draw(rr, render.Point{}, "AAB", render.RGB(255, 255, 255))

	if want := []rune{'A', 'B'}; len(dropped) != len(want) || dropped[0] != want[0] || dropped[1] != want[1] {
		t.Fatalf("dropped = %v, want %v (one call per distinct rune, in first-seen order)", dropped, want)
	}
	if len(rr.glyphQuads) != 0 {
		t.Errorf("glyphQuads = %v, want none (every glyph dropped)", rr.glyphQuads)
	}
}

// TestFaceOnGlyphDroppedDefaultNil asserts the default (unset)
// OnGlyphDropped changes nothing: Draw must not panic when the callback is
// nil, even along the dropped-glyph path.
func TestFaceOnGlyphDroppedDefaultNil(t *testing.T) {
	f, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	fa := NewFace(f, 16)
	atlas := fa.Font.sharedAtlas()
	atlas.covCursorY = atlasSize
	atlas.covRowH = 0

	rr := &recordingRenderer{scale: 1}
	fa.Draw(rr, render.Point{}, "A", render.RGB(255, 255, 255)) // must not panic
}

func TestMeasure(t *testing.T) {
	f, _ := Load(goregular.TTF)
	fa := NewFace(f, 16)
	m1, m2 := fa.Measure("M"), fa.Measure("MM")
	if m1.W <= 0 || m2.W <= m1.W {
		t.Errorf("M=%v MM=%v", m1, m2)
	}
	if lh := fa.LineHeight(); lh < 16 || lh > 26 {
		t.Errorf("LineHeight=%v", lh)
	}
	if fa.Measure("").W != 0 {
		t.Error("empty string width != 0")
	}
}

// TestResolveGlyphNoFallback is the graceful-tofu regression test: a Face
// with no fallback chain (the NewFace path) must resolve every rune to its
// own Font, even one goregular has no glyph for — falling back to glyph
// index 0 (.notdef), never an error or panic. This is the same behavior
// Measure and Draw had before Face gained fallback support.
func TestResolveGlyphNoFallback(t *testing.T) {
	f, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	fa := NewFace(f, 16)

	if srcFont, gi := fa.resolveGlyph('A'); srcFont != f || gi == 0 {
		t.Errorf("resolveGlyph('A') = (%p, %v), want (primary, nonzero glyph)", srcFont, gi)
	}
	srcFont, gi := fa.resolveGlyph('中')
	if srcFont != f {
		t.Errorf("resolveGlyph(中) font = %p, want primary %p (no fallback chain)", srcFont, f)
	}
	if gi != 0 {
		t.Errorf("resolveGlyph(中) glyph = %v, want 0 (.notdef; goregular has no CJK coverage)", gi)
	}

	// Must not panic, and must still measure/draw something (the notdef
	// glyph typically has a nonzero advance).
	fa.Measure("中")
	rr := &recordingRenderer{scale: 1}
	fa.Draw(rr, render.Point{}, "中", render.RGB(255, 255, 255))
}

// TestResolveGlyphWithFallback is the fallback-resolution unit test: given
// a primary (goregular, Latin-only) and a fallback that covers Han
// characters, resolveGlyph must resolve a glyph the primary has ('A') to
// the primary, and one only the fallback has (中) to the fallback — using
// the fallback's own glyph index, not the primary's.
func TestResolveGlyphWithFallback(t *testing.T) {
	primary, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	fallback := loadCJKFallback(t)
	fa := NewFaceWithFallback(primary, []*Font{fallback}, 16)

	if srcFont, gi := fa.resolveGlyph('A'); srcFont != primary || gi == 0 {
		t.Errorf("resolveGlyph('A') = (%p, %v), want (primary %p, nonzero glyph)", srcFont, gi, primary)
	}

	srcFont, gi := fa.resolveGlyph('中')
	if srcFont != fallback {
		t.Errorf("resolveGlyph(中) font = %p, want fallback %p", srcFont, fallback)
	}
	wantGi, ok := fallback.glyphIndex('中')
	if !ok {
		t.Fatal("fallback font unexpectedly has no glyph for 中")
	}
	if gi != wantGi {
		t.Errorf("resolveGlyph(中) glyph = %v, want fallback's own index %v", gi, wantGi)
	}
}

// TestAddFallback asserts AddFallback appends to the chain, returns fa for
// chaining, and that a rune resolved only via an appended fallback picks it
// up (same outcome as passing it to NewFaceWithFallback up front).
func TestAddFallback(t *testing.T) {
	primary, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	fallback := loadCJKFallback(t)

	fa := NewFace(primary, 16)
	if len(fa.Fallbacks) != 0 {
		t.Fatalf("fresh NewFace: Fallbacks = %v, want empty", fa.Fallbacks)
	}

	ret := fa.AddFallback(fallback)
	if ret != fa {
		t.Error("AddFallback did not return fa for chaining")
	}
	if len(fa.Fallbacks) != 1 || fa.Fallbacks[0] != fallback {
		t.Errorf("Fallbacks = %v, want [%p]", fa.Fallbacks, fallback)
	}

	if srcFont, _ := fa.resolveGlyph('中'); srcFont != fallback {
		t.Errorf("after AddFallback, resolveGlyph(中) font = %p, want fallback %p", srcFont, fallback)
	}
}

// TestFaceFallbackMeasureAgreement is the Measure-agreement test: measuring
// a mixed string (Latin from the primary, one CJK glyph only the fallback
// covers) must equal the sum of per-glyph advances computed from each
// rune's resolveGlyph source font — the same resolution Draw uses — with
// kerning applied only between consecutive glyphs sharing a source font.
// Single-font Measure (TestMeasure above) is unchanged by this: with no
// fallback chain, every rune resolves to the same font and this reduces to
// the pre-fallback computation.
func TestFaceFallbackMeasureAgreement(t *testing.T) {
	primary, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	fallback := loadCJKFallback(t)
	fa := NewFaceWithFallback(primary, []*Font{fallback}, 16)

	const s = "Hi 中文 fluo"
	var want float32
	var prevFont *Font
	var prevGi sfnt.GlyphIndex
	hasPrev := false
	for _, r := range s {
		srcFont, gi := fa.resolveGlyph(r)
		if hasPrev && prevFont == srcFont {
			want += prevFont.kern(prevGi, gi, fa.SizePx)
		}
		want += srcFont.advance(gi, fa.SizePx)
		prevFont, prevGi, hasPrev = srcFont, gi, true
	}

	got := fa.Measure(s).W
	if got != want {
		t.Errorf("Measure(%q).W = %v, want %v (sum of per-glyph advances from resolved source fonts)", s, got, want)
	}
}

// TestFaceFallbackDrawBatchesPerTexture is the Draw-batching test for a
// mixed string: Face.Draw must call DrawGlyphs once per distinct source
// font's atlas texture — here, twice, once for the primary's Latin glyphs
// and once for the fallback's CJK glyph — rather than once for the whole
// string (which would only be correct when every glyph shares one atlas).
// It also re-confirms (mirroring TestFaceDrawUsesDrawGlyphs) that the
// single-font path still collapses to exactly one DrawGlyphs call.
func TestFaceFallbackDrawBatchesPerTexture(t *testing.T) {
	primary, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	fallback := loadCJKFallback(t)
	fa := NewFaceWithFallback(primary, []*Font{fallback}, 16)

	rr := &recordingRenderer{scale: 1}
	fa.Draw(rr, render.Point{X: 2, Y: 3}, "Hi 中", render.RGB(255, 255, 255))
	if rr.glyphCalls != 2 {
		t.Errorf("mixed-font string: DrawGlyphs called %d times, want 2 (one per distinct source-font atlas)", rr.glyphCalls)
	}

	// Same Face, Latin-only string: every glyph resolves to the primary,
	// so this must still collapse to one DrawGlyphs call.
	rr2 := &recordingRenderer{scale: 1}
	fa.Draw(rr2, render.Point{X: 2, Y: 3}, "Hi", render.RGB(255, 255, 255))
	if rr2.glyphCalls != 1 {
		t.Errorf("Latin-only string on a fallback-capable Face: DrawGlyphs called %d times, want 1", rr2.glyphCalls)
	}
}
