package text

import (
	"fmt"
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
// Face.Draw's crisp HD-text path without a GPU: it records DrawGlyphs
// calls and reports a fixed scale, matching how app.Surface would report
// the current frame's device-pixels-per-logical-pixel factor. Every other
// method is a no-op; Draw only calls Scale and DrawGlyphs.
// It also hands out a DISTINCT TextureID per CreateTexture call and records
// the texture each DrawGlyphs call was handed (glyphTex, one entry per
// call): a batch is only correct if it is drawn with ITS OWN atlas page's
// texture, and discarding that argument would let a Face that always
// returned page 0's texture — rendering every page-1 glyph as whatever
// happens to sit at those UVs on page 0 — pass silently.
type recordingRenderer struct {
	scale      float32
	glyphCalls int
	glyphQuads []render.GlyphQuad
	glyphTex   []render.TextureID
	nextTex    render.TextureID
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
func (r *recordingRenderer) CreateTexture(w, h int, rgba []byte) render.TextureID {
	r.nextTex++ // from 1: render.NoTexture is 0
	return r.nextTex
}
func (r *recordingRenderer) UpdateTexture(id render.TextureID, x, y, w, h int, rgba []byte) {}
func (r *recordingRenderer) DeleteTexture(id render.TextureID)                              {}
func (r *recordingRenderer) DrawQuad(dst, src render.Rect, tex render.TextureID, tint render.Color) {
}
func (r *recordingRenderer) DrawSDFQuads(quads []render.GlyphQuad, tex render.TextureID, c render.Color) {
}
func (r *recordingRenderer) DrawGlyphs(quads []render.GlyphQuad, tex render.TextureID, c render.Color) {
	r.glyphCalls++
	r.glyphQuads = quads
	r.glyphTex = append(r.glyphTex, tex)
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
// batches every glyph into a single DrawGlyphs call, the emitted quad count
// equals the visible-glyph count (spaces skipped), and every quad's device
// origin (Dst.X*scale, Dst.Y*scale) is pixel-snapped (integer-valued,
// within float rounding) at both scale 1 and scale 2.
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

// TestFaceDrawGrowsAtlasInsteadOfDropping is the atlas-growth regression
// test for Face.Draw. Before growth, forcing the coverage shelf packer
// past its page's bottom edge made every subsequent glyphCoverage call
// fail with "atlas full" and drop the glyph; now it must instead grow a
// second page and keep drawing. This also doubles as the multi-page
// batching test: 'A' (already packed on page 0) and 'B' (forced onto the
// newly grown page 1) must produce one DrawGlyphs call per page used, the
// same "batch per texture" grouping fallback fonts already exercise (see
// TestFaceFallbackDrawBatchesPerTexture), now applied across pages of one
// font's own atlas too.
func TestFaceDrawGrowsAtlasInsteadOfDropping(t *testing.T) {
	f, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	fa := NewFace(f, 16)
	atlas := fa.Font.sharedAtlas()

	// Pack 'A' onto page 0 the normal way, then push that page's packer
	// past its bottom edge so the next distinct glyph has no room left on
	// it, without needing to actually rasterize enough real glyphs to
	// fill a whole 1024x1024 page.
	giA, _ := f.glyphIndex('A')
	if _, err := atlas.glyphCoverage(giA, 16); err != nil {
		t.Fatal(err)
	}
	atlas.covPages[0].cursorY = atlasSize
	atlas.covPages[0].rowH = 0

	var dropped []rune
	fa.OnGlyphDropped = func(r rune) { dropped = append(dropped, r) }

	rr := &recordingRenderer{scale: 1}
	fa.Draw(rr, render.Point{}, "AB", render.RGB(255, 255, 255))

	if len(dropped) != 0 {
		t.Fatalf("dropped = %v, want none (page growth replaces dropping on ordinary overflow)", dropped)
	}
	if len(atlas.covPages) != 2 {
		t.Fatalf("covPages = %d, want 2 (a fresh page grown for 'B')", len(atlas.covPages))
	}
	if rr.glyphCalls != 2 {
		t.Errorf("DrawGlyphs called %d times, want 2 (one per page: 'A' on page 0, 'B' on the grown page 1)", rr.glyphCalls)
	}
	if len(rr.glyphQuads) != 1 {
		t.Errorf("last DrawGlyphs call had %d quads, want 1", len(rr.glyphQuads))
	}

	// Each batch must be drawn with its OWN page's texture. Without this,
	// an ensureCoverageTexture that ignored its page argument and always
	// handed back page 0's texture would still produce two correctly split
	// batches — and page 1's glyphs would sample page 0 at page 1's UVs,
	// i.e. render as garbage — with every other assertion here passing.
	if len(rr.glyphTex) != 2 {
		t.Fatalf("recorded %d DrawGlyphs textures, want 2", len(rr.glyphTex))
	}
	if rr.glyphTex[0] == render.NoTexture || rr.glyphTex[1] == render.NoTexture {
		t.Fatalf("DrawGlyphs textures = %v, want two real textures", rr.glyphTex)
	}
	if rr.glyphTex[0] == rr.glyphTex[1] {
		t.Errorf("both DrawGlyphs calls used texture %v; want one texture per page (page 0 and the grown page 1 are separate GPU textures)", rr.glyphTex[0])
	}
	// And each must be the texture that page actually owns.
	for page, tex := range rr.glyphTex {
		if want := atlas.covPages[page].tex; tex != want {
			t.Errorf("DrawGlyphs call %d used texture %v, want page %d's own %v", page, tex, page, want)
		}
	}
}

// TestFaceOnGlyphDroppedDegenerateGlyph is the atlas-full-surfacing unit
// test post-growth: OnGlyphDropped now only fires for a glyph mask too
// large to ever fit on a single empty coverage-atlas page — ordinary
// overflow grows a new page instead (see
// TestFaceDrawGrowsAtlasInsteadOfDropping) and never reaches this
// callback. A SizePx several times atlasSize forces every real glyph's
// device-px raster past a whole page's dimensions, without needing a
// production knob to shrink the page itself.
func TestFaceOnGlyphDroppedDegenerateGlyph(t *testing.T) {
	f, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	fa := NewFace(f, atlasSize*3)

	var dropped []rune
	fa.OnGlyphDropped = func(r rune) { dropped = append(dropped, r) }

	rr := &recordingRenderer{scale: 1}
	fa.Draw(rr, render.Point{}, "AAB", render.RGB(255, 255, 255))

	if want := []rune{'A', 'B'}; len(dropped) != len(want) || dropped[0] != want[0] || dropped[1] != want[1] {
		t.Fatalf("dropped = %v, want %v (one call per distinct rune, in first-seen order)", dropped, want)
	}
	if len(rr.glyphQuads) != 0 {
		t.Errorf("glyphQuads = %v, want none (every glyph too large to place)", rr.glyphQuads)
	}
}

// TestFaceOnGlyphDroppedDefaultNil asserts the default (unset)
// OnGlyphDropped changes nothing: Draw must not panic when the callback is
// nil, even along the dropped-glyph path (see
// TestFaceOnGlyphDroppedDegenerateGlyph for why an oversized SizePx is
// what still reaches that path after growth).
func TestFaceOnGlyphDroppedDefaultNil(t *testing.T) {
	f, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	fa := NewFace(f, atlasSize*3)

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

// TestMeasureCaches is the memoization test: Measure returns a stable Size
// across repeated calls, populates measureCache with exactly the value it
// returned, and — proven by poisoning the cached entry and seeing the
// poisoned value come back — reads that entry on a hit instead of
// recomputing. A never-cached string still computes the same result the
// uncached path would, so a hit and a miss agree to the byte.
func TestMeasureCaches(t *testing.T) {
	f, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	fa := NewFace(f, 16)
	const s = "Hi fluo"

	first := fa.Measure(s)
	if again := fa.Measure(s); again != first {
		t.Fatalf("Measure not stable across calls: %v vs %v", again, first)
	}

	fa.measureMu.Lock()
	cached, ok := fa.measureCur[s]
	fa.measureMu.Unlock()
	if !ok {
		t.Fatal("measure cache not populated after Measure")
	}
	if cached != first {
		t.Fatalf("cached %v != returned %v", cached, first)
	}

	// Poison the entry: a genuine cache hit must return this, not recompute.
	poison := render.Size{W: first.W + 1234, H: first.H + 1}
	fa.measureMu.Lock()
	fa.measureCur[s] = poison
	fa.measureMu.Unlock()
	if got := fa.Measure(s); got != poison {
		t.Fatalf("Measure recomputed instead of hitting cache: got %v, want poisoned %v", got, poison)
	}

	// A different (uncached) string must compute the true value.
	got := fa.Measure("different string")
	want := fa.measureUncached("different string")
	if got != want {
		t.Fatalf("miss path Measure = %v, want uncached %v", got, want)
	}
}

// TestMeasureCacheBounded asserts the cache can't grow without limit: after
// measuring far more distinct strings than a generation holds, the two
// generations together stay within 2*measureCacheGen entries (older ones are
// evicted by generation rotation), so live-updating labels can't leak.
func TestMeasureCacheBounded(t *testing.T) {
	f, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	fa := NewFace(f, 16)
	// Several full generations' worth of distinct strings.
	for i := 0; i < measureCacheGen*5; i++ {
		fa.Measure(fmt.Sprintf("string-%d", i))
	}
	fa.measureMu.Lock()
	total := len(fa.measureCur) + len(fa.measurePrev)
	fa.measureMu.Unlock()
	if total > 2*measureCacheGen {
		t.Fatalf("measure cache grew past bound: %d entries > 2*%d", total, measureCacheGen)
	}
}

// TestMeasureCachePromotesHotEntry asserts the two-generation eviction keeps a
// still-measured string alive across rotations: a string measured every so
// often should never fall out even as unrelated churn rotates generations,
// because a prev-generation hit is promoted back into the current generation.
func TestMeasureCachePromotesHotEntry(t *testing.T) {
	f, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	fa := NewFace(f, 16)
	const hot = "hot-string"
	want := fa.Measure(hot)

	// Churn distinct strings, re-touching hot within each generation so it
	// stays promoted, across several rotations.
	for i := 0; i < measureCacheGen*4; i++ {
		fa.Measure(fmt.Sprintf("cold-%d", i))
		if i%64 == 0 {
			fa.Measure(hot)
		}
	}

	fa.measureMu.Lock()
	_, inCur := fa.measureCur[hot]
	_, inPrev := fa.measurePrev[hot]
	fa.measureMu.Unlock()
	if !inCur && !inPrev {
		t.Fatal("hot string evicted despite repeated measurement")
	}
	if got := fa.Measure(hot); got != want {
		t.Fatalf("hot string Measure = %v, want stable %v", got, want)
	}
}

// TestMeasureCacheInvalidatedOnAddFallback asserts AddFallback drops any
// memoized Measure widths: appending a fallback can change how a rune
// resolves, so a stale cached width would be wrong. Uses a second goregular
// as the fallback purely to exercise invalidation (no CJK font needed).
func TestMeasureCacheInvalidatedOnAddFallback(t *testing.T) {
	f, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	fa := NewFace(f, 16)
	fa.Measure("hello")

	fa.measureMu.Lock()
	populated := len(fa.measureCur) + len(fa.measurePrev)
	fa.measureMu.Unlock()
	if populated == 0 {
		t.Fatal("measure cache not populated after Measure")
	}

	fb, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	fa.AddFallback(fb)

	fa.measureMu.Lock()
	after := len(fa.measureCur) + len(fa.measurePrev)
	fa.measureMu.Unlock()
	if after != 0 {
		t.Fatalf("measure cache not cleared by AddFallback: %d entries remain", after)
	}
}

// TestFaceDrawBatchOrderAndReuse is the P5 regression test for Draw's
// per-(font,page) grouping after the batch map was replaced by a linear
// scan. Forcing 'B' onto a grown second page while 'A' stays on page 0,
// the string "ABA" must still emit exactly two DrawGlyphs calls in
// first-seen page order (page 0 then page 1): the trailing 'A' must reuse
// page 0's existing batch, not open a third. Textures confirm the order.
func TestFaceDrawBatchOrderAndReuse(t *testing.T) {
	f, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	fa := NewFace(f, 16)
	atlas := fa.Font.sharedAtlas()

	// Pack 'A' on page 0, then push page 0's packer past its bottom edge so
	// the next distinct glyph ('B') must grow a new page (same setup as
	// TestFaceDrawGrowsAtlasInsteadOfDropping).
	giA, _ := f.glyphIndex('A')
	if _, err := atlas.glyphCoverage(giA, 16); err != nil {
		t.Fatal(err)
	}
	atlas.covPages[0].cursorY = atlasSize
	atlas.covPages[0].rowH = 0

	rr := &recordingRenderer{scale: 1}
	fa.Draw(rr, render.Point{}, "ABA", render.RGB(255, 255, 255))

	if len(atlas.covPages) != 2 {
		t.Fatalf("covPages = %d, want 2 (a page grown for 'B')", len(atlas.covPages))
	}
	// Two calls, not three: the trailing 'A' reused page 0's batch.
	if rr.glyphCalls != 2 {
		t.Fatalf("DrawGlyphs called %d times, want 2 (trailing 'A' must reuse page 0's batch)", rr.glyphCalls)
	}
	if len(rr.glyphTex) != 2 {
		t.Fatalf("recorded %d DrawGlyphs textures, want 2", len(rr.glyphTex))
	}
	// First-seen order: page 0 first, then the grown page 1.
	if rr.glyphTex[0] != atlas.covPages[0].tex {
		t.Errorf("first DrawGlyphs texture = %v, want page 0's %v", rr.glyphTex[0], atlas.covPages[0].tex)
	}
	if rr.glyphTex[1] != atlas.covPages[1].tex {
		t.Errorf("second DrawGlyphs texture = %v, want page 1's %v", rr.glyphTex[1], atlas.covPages[1].tex)
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

// TestFacePrefixWidthsMatchMeasurePrefixes pins the contract PrefixWidths
// exists for: widths[i] must equal Measure(string(runes[:i])).W for EVERY
// boundary i, exactly — not within a tolerance. Callers that walk a run
// boundary by boundary (caret hit-testing) rely on that, and the only way to
// get it is to reuse Measure's own per-step accumulation, kerning included:
// a pen that summed advances alone would agree here only by the accident of
// goregular carrying no kern pairs, and would drift on a font that does.
// The mixed-script subtest additionally covers the fallback chain, where
// consecutive glyphs come from different source fonts and kerning between
// them must be suppressed on both paths alike.
func TestFacePrefixWidthsMatchMeasurePrefixes(t *testing.T) {
	f, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}

	check := func(t *testing.T, fa *Face, s string) {
		t.Helper()
		runes := []rune(s)
		widths := fa.PrefixWidths(runes)
		if len(widths) != len(runes)+1 {
			t.Fatalf("PrefixWidths(%q) returned %d widths, want %d", s, len(widths), len(runes)+1)
		}
		if widths[0] != 0 {
			t.Errorf("PrefixWidths(%q)[0] = %v, want 0", s, widths[0])
		}
		for i := range widths {
			want := fa.Measure(string(runes[:i])).W
			if widths[i] != want {
				t.Errorf("PrefixWidths(%q)[%d] = %v, want %v (Measure of the prefix)", s, i, widths[i], want)
			}
		}
	}

	fa := NewFace(f, 16)
	for _, s := range []string{"", "M", "AVAWaToVoTaWATATaYoP.", "The quick brown fox", "héllo wörld", "iiiWWWmmm"} {
		check(t, fa, s)
	}

	t.Run("fallback", func(t *testing.T) {
		fb := NewFaceWithFallback(f, []*Font{loadCJKFallback(t)}, 16)
		check(t, fb, "Hi 中文 fluo")
	})
}
