package text

import (
	"math"
	"sync"

	"golang.org/x/image/font/sfnt"

	"github.com/0xdreadnaught/fluo/render"
)

// Face renders text from a Font at a fixed pixel size. The glyph
// coverage masks it draws from (see Draw) live in Font's shared Atlas
// (see Font.sharedAtlas), so creating many Faces at different sizes for
// the same Font does not duplicate the Atlas itself, though each
// distinct (glyph, device px) pair is rasterized once at that size (see
// Atlas.glyphCoverage) since coverage masks aren't
// resolution-independent.
//
// Fallbacks holds an ordered fallback chain: for each rune, Measure and
// Draw resolve the source font as the first of [Font, Fallbacks...] that
// actually has a glyph for it (see resolveGlyph), so a codepoint the
// primary font lacks can still render instead of falling back to
// .notdef, so long as some font later in the chain covers it. Line
// metrics (Ascent, LineHeight) and the device-px rasterization size
// always come from Font alone, never from a fallback, so mixing in a
// fallback glyph never shifts a line's baseline. Each font in the chain
// — primary or fallback — rasterizes into its own Atlas (see
// Font.sharedAtlas), and that Atlas's coverage side can itself grow
// across several pages (see Atlas.glyphCoverage), so Draw groups glyph
// quads by (source font, page) and issues one DrawGlyphs call per
// distinct page texture.
type Face struct {
	Font      *Font
	Fallbacks []*Font
	SizePx    float32

	// OnGlyphDropped, if set, is called when Draw cannot place a rune's
	// coverage mask at all. The coverage atlas grows (see
	// Atlas.glyphCoverage): when a page fills up, a new one is allocated
	// rather than dropping the glyph, so ordinary overflow no longer
	// reaches this callback. The only case that still does is a single
	// glyph mask larger than a whole empty page — degenerate, and not
	// something growth can fix. Default nil: Draw silently skips the
	// glyph, same as before this field existed, so leaving it unset
	// changes nothing. It fires at most once per distinct rune over fa's
	// lifetime, so a persistently oversized glyph reports once rather than
	// spamming every frame that rune is drawn.
	OnGlyphDropped func(r rune)

	droppedMu sync.Mutex
	dropped   map[rune]bool

	// measureCur/measurePrev memoize Measure: they map an exact input
	// string to the exact render.Size Measure computes for it, so a
	// repeated Measure of the same string skips the ~3 Font.mu acquisitions
	// per rune (glyphIndex, advance, kern) plus the metrics lock the
	// uncached path costs and returns O(1). The stored Size is whatever the
	// current code path produced, so a hit is bit-identical to a miss —
	// same rune resolution, advances, kerning, and rounding. The key is
	// just the string: SizePx (and the font/fallback chain) are fixed per
	// Face, so they need not be part of the key.
	//
	// Bound + eviction: the measured-string space is NOT bounded in general
	// — a TextBox being edited, or a live-updating label ("1 item", "2
	// items", a clock, a byte counter), yields a fresh distinct string on
	// nearly every change — so an unbounded map would leak. Instead this is
	// a two-generation (CLOCK / second-chance) approximate-LRU: inserts
	// land in measureCur; once it fills to measureCacheGen entries it
	// rotates (measurePrev is dropped, measureCur becomes measurePrev, a
	// fresh measureCur starts), so total live entries never exceed
	// 2*measureCacheGen. A hit in measurePrev is promoted back into
	// measureCur, so hot strings survive rotations while one-shot churn
	// ages out. Eviction is a couple of map ops on rotation and nothing at
	// all on a current-generation hit (no per-hit LRU-list churn) —
	// trivially cheaper than the per-rune sfnt work a hit avoids.
	//
	// The generation size is sized to the on-screen working set, not
	// trimmed: a single generation (measureCacheGen) already exceeds the
	// few-hundred stable labels/cells a busy view re-measures every frame
	// (e.g. a 50x5 DataGrid's ~250 cells plus panel/header labels and a
	// TextBox's active substrings), so in steady state that whole set lives
	// in measureCur and never triggers a rotation — every frame is pure
	// hits, no thrash. The second generation only cushions transitions.
	//
	// Validity contract: entries stay valid for the life of the Face, which
	// is fixed by construction — Font, SizePx, and the fallback chain are
	// all set when the Face is built and never mutated at runtime, with one
	// exception: AddFallback grows the chain and so can change how a rune
	// resolves, so it invalidates the cache. (SizePx and Fallbacks are
	// exported; assigning them directly instead of building a fresh Face
	// bypasses that invalidation and is unsupported, same as it always was
	// for any Face-derived state.)
	//
	// Thread-safety: guarded by measureMu, matching how droppedMu guards
	// the dropped set. Measure runs on the single frame thread in normal
	// use, but the lock keeps it consistent with the rest of Face's shared
	// state and cheap when uncontended.
	measureMu   sync.Mutex
	measureCur  map[string]render.Size
	measurePrev map[string]render.Size
}

// measureCacheGen caps a single generation of Face's Measure cache; with two
// generations (see measureCur/measurePrev) total live entries stay under
// 2*measureCacheGen, bounding memory while keeping recently measured strings.
// Sized larger than a typical view's per-frame working set of measured
// strings so that set stays resident in one generation without rotating —
// i.e. the common case hits rather than thrashing.
const measureCacheGen = 1024

// reportDropped invokes fa.OnGlyphDropped for r, if set, the first time r is
// reported dropped for fa; later calls for the same r are ignored so a full
// atlas doesn't call back every frame.
func (fa *Face) reportDropped(r rune) {
	if fa.OnGlyphDropped == nil {
		return
	}
	fa.droppedMu.Lock()
	already := fa.dropped[r]
	if !already {
		if fa.dropped == nil {
			fa.dropped = make(map[rune]bool)
		}
		fa.dropped[r] = true
	}
	fa.droppedMu.Unlock()
	if !already {
		fa.OnGlyphDropped(r)
	}
}

// NewFace returns a Face for f at sizePx, ensuring f's shared glyph
// atlas exists. The returned Face has no fallback chain: any rune f
// lacks a glyph for renders as .notdef, exactly as before Face gained
// fallback support. Use NewFaceWithFallback to set one up front, or
// AddFallback to grow one afterward.
func NewFace(f *Font, sizePx float32) *Face {
	f.sharedAtlas()
	return &Face{Font: f, SizePx: sizePx}
}

// NewFaceWithFallback returns a Face for primary at sizePx with an
// ordered fallback chain: for each rune, Measure and Draw use the first
// font in [primary, fallbacks...] that actually has a glyph for it (see
// resolveGlyph), falling back to primary's own .notdef glyph if none of
// them do. Line metrics and rasterization size still come from primary
// alone (see the Face doc comment). Every font's shared atlas —
// primary's and each fallback's — is ensured up front, same as NewFace
// does for a single font. fallbacks is copied, so the caller's slice
// can be reused or modified afterward without affecting the Face.
func NewFaceWithFallback(primary *Font, fallbacks []*Font, sizePx float32) *Face {
	primary.sharedAtlas()
	fbs := make([]*Font, len(fallbacks))
	copy(fbs, fallbacks)
	for _, fb := range fbs {
		fb.sharedAtlas()
	}
	return &Face{Font: primary, Fallbacks: fbs, SizePx: sizePx}
}

// AddFallback appends font to the end of fa's fallback chain (see
// NewFaceWithFallback), ensuring font's shared atlas exists, and returns
// fa for chaining.
func (fa *Face) AddFallback(font *Font) *Face {
	font.sharedAtlas()
	fa.Fallbacks = append(fa.Fallbacks, font)
	// Growing the chain can change how a rune resolves (see resolveGlyph),
	// so any memoized Measure widths are no longer trustworthy — drop them.
	fa.measureMu.Lock()
	fa.measureCur, fa.measurePrev = nil, nil
	fa.measureMu.Unlock()
	return fa
}

// resolveGlyph returns the first font in [fa.Font, fa.Fallbacks...] that
// has a real glyph for r (see Font.HasGlyph), and that font's glyph
// index for r. If none of them do, it returns fa.Font and fa.Font's own
// glyph index for r — typically glyph 0 (.notdef) — so a rune no font in
// the chain covers still renders exactly as a fallback-less Face would:
// gracefully, never an error or panic. Measure and Draw both resolve
// through this method so their advances (and so text width and caret
// positions) never disagree.
func (fa *Face) resolveGlyph(r rune) (*Font, sfnt.GlyphIndex) {
	if gi, ok := fa.Font.glyphIndex(r); ok {
		return fa.Font, gi
	}
	for _, fb := range fa.Fallbacks {
		if gi, ok := fb.glyphIndex(r); ok {
			return fb, gi
		}
	}
	gi, _ := fa.Font.glyphIndex(r)
	return fa.Font, gi
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

// PrefixWidths returns the pen width at every rune boundary of runes:
// len(runes)+1 values, [0] always 0 and [i] the width of the first i
// runes — so PrefixWidths(runes)[i] is exactly what Measure reports for
// string(runes[:i]), for every i. It walks the same left-to-right pen
// Measure does (resolveGlyph per rune, kerning added only between two
// glyphs from the same source font, then that glyph's advance), just
// emitting each partial sum instead of only the last one, so the two
// agree bit-for-bit by construction — kerning included, which matters
// for any font that carries a kern table, where summing advances alone
// would drift from the positions Draw actually puts the glyphs at.
//
// It exists for callers that need every boundary of a run rather than
// one width — caret hit-testing walking a click across a line, say —
// and gives them all of them in a single O(n) pass instead of
// re-measuring an ever-growing prefix per boundary.
func (fa *Face) PrefixWidths(runes []rune) []float32 {
	widths := make([]float32, len(runes)+1)
	var w float32
	var prevFont *Font
	var prev sfnt.GlyphIndex
	hasPrev := false
	for i, r := range runes {
		srcFont, gi := fa.resolveGlyph(r)
		if hasPrev && prevFont == srcFont {
			w += prevFont.kern(prev, gi, fa.SizePx)
		}
		w += srcFont.advance(gi, fa.SizePx)
		prevFont, prev, hasPrev = srcFont, gi, true
		widths[i+1] = w
	}
	return widths
}

// Measure returns the size s occupies when drawn with fa: width is the
// sum of glyph advances plus kerning between consecutive glyphs, height
// is LineHeight (text is laid out on a single line for now; LineHeight
// always comes from fa.Font, never a fallback — see the Face doc
// comment). Each rune resolves to a source font via resolveGlyph — the
// same resolution Draw uses, so the two never disagree — and its
// advance comes from that source font; runes no font in the chain
// covers fall back to fa.Font's glyph index 0 (.notdef), matching Draw.
// Kerning is only applied between two consecutive glyphs resolved from
// the same source font, since a kern table's glyph indices aren't
// meaningful across fonts.
func (fa *Face) Measure(s string) render.Size {
	fa.measureMu.Lock()
	if sz, ok := fa.measureCur[s]; ok {
		fa.measureMu.Unlock()
		return sz
	}
	if sz, ok := fa.measurePrev[s]; ok {
		// Older-generation hit: promote it into the current generation so a
		// still-hot string survives the next rotation instead of aging out.
		fa.measurePut(s, sz)
		fa.measureMu.Unlock()
		return sz
	}
	fa.measureMu.Unlock()

	sz := fa.measureUncached(s)

	fa.measureMu.Lock()
	fa.measurePut(s, sz)
	fa.measureMu.Unlock()
	return sz
}

// measurePut inserts s->sz into the current generation of the Measure cache,
// rotating generations first if the current one is full (see the
// measureCur/measurePrev doc). The caller must hold measureMu.
func (fa *Face) measurePut(s string, sz render.Size) {
	if fa.measureCur == nil {
		fa.measureCur = make(map[string]render.Size)
	} else if len(fa.measureCur) >= measureCacheGen {
		// Rotate: the current generation ages into prev (dropping the old
		// prev), and a fresh current starts. Bounds total live entries at
		// 2*measureCacheGen.
		fa.measurePrev = fa.measureCur
		fa.measureCur = make(map[string]render.Size, measureCacheGen)
	}
	fa.measureCur[s] = sz
}

// measureUncached computes the render.Size for s from scratch — the exact
// pre-memoization Measure computation. Measure caches its result; this
// method holds the actual layout logic so a cache miss and a never-cached
// path stay identical to the byte.
func (fa *Face) measureUncached(s string) render.Size {
	var w float32
	var prevFont *Font
	var prev sfnt.GlyphIndex
	hasPrev := false
	for _, r := range s {
		srcFont, gi := fa.resolveGlyph(r)
		if hasPrev && prevFont == srcFont {
			w += prevFont.kern(prev, gi, fa.SizePx)
		}
		w += srcFont.advance(gi, fa.SizePx)
		prevFont, prev, hasPrev = srcFont, gi, true
	}
	return render.Size{W: w, H: fa.LineHeight()}
}

// glyphBatch accumulates the glyph quads Draw resolves to a single
// (source font, page) pair, so they can be submitted to the renderer in
// one DrawGlyphs call against that page's texture. A batch is identified
// by its (atlas, page): two fonts never share an Atlas (each has its own),
// and a font's own atlas can itself span several pages once it grows past
// the first one (see Atlas.glyphCoverage), so the atlas pointer and page
// together distinguish a batch — equivalently to a (source font, page)
// key, since a font maps 1:1 to its shared Atlas.
type glyphBatch struct {
	atlas *Atlas
	page  int
	quads []render.GlyphQuad
}

// Draw renders s with fa, with at as the top-left corner of the text box;
// the baseline sits at at.Y + fa.Ascent(). This is the crisp HD-text path:
// each glyph is rasterized directly (grayscale-AA coverage) at the
// exact device-pixel size for the current frame's scale (from r.Scale()),
// and both the baseline and each glyph's draw origin are snapped to whole
// device pixels for a sharp result. Layout stays exact: advances and
// kerning are computed from the unsnapped logical-px metrics path and the
// pen is never snapped cumulatively, only each glyph's own draw origin —
// so accumulated advance/Measure width never drifts from the snapped
// pixels actually drawn. Each rune resolves to a source font via
// resolveGlyph — the same resolution Measure uses — and rasterizes into
// that font's own shared coverage atlas; since a fallback chain can mix
// glyphs from several fonts (and so several atlas textures) in one string
// — and a single font's own atlas can itself span several pages once it
// grows past the first one (see Atlas.glyphCoverage) — quads are gathered
// per (source font, page) and submitted to r as one DrawGlyphs call per
// distinct atlas-page texture. A Face with no fallback chain whose string
// fits on one page resolves every rune to the same (font, page), so this
// still collapses to exactly one DrawGlyphs call, unchanged from before
// Face gained fallback support. Runes no font in the chain covers fall
// back to fa.Font's glyph index 0 (.notdef); glyphs with no visible
// coverage (e.g. space) are skipped but still advance the pen.
func (fa *Face) Draw(r render.Renderer, at render.Point, s string, c render.Color) {
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

	// batches groups quads by (source font, atlas page), in first-seen
	// order, so Draw issues one DrawGlyphs call per distinct page texture
	// below. The common case is a single batch (one font, one page), and
	// even a fallback chain touches only a handful, so a linear scan over
	// this slice finds the matching batch without allocating a map per
	// call. First-seen order and grouping are identical to a map keyed by
	// (font, page), so the DrawGlyphs calls emitted are unchanged.
	var batches []*glyphBatch

	penX := at.X // logical px; advances by the UNSNAPPED metric each glyph, never snapped itself
	var prevFont *Font
	var prev sfnt.GlyphIndex
	hasPrev := false
	for _, ch := range s {
		srcFont, gi := fa.resolveGlyph(ch)
		if hasPrev && prevFont == srcFont {
			penX += prevFont.kern(prev, gi, fa.SizePx)
		}

		atlas := srcFont.sharedAtlas()
		if e, err := atlas.glyphCoverage(gi, px); err == nil && !e.empty {
			// e.bearing{X,Y} are device px at this entry's px size;
			// convert to logical (/scale) before combining with the
			// logical pen/baseline, then snap the glyph's own draw
			// origin to a whole device pixel.
			gx := float32(math.Round(float64(penX+e.bearingX/scale)*float64(scale))) / scale
			gy := float32(math.Round(float64(baseline+e.bearingY/scale)*float64(scale))) / scale
			var b *glyphBatch
			for _, cand := range batches {
				if cand.atlas == atlas && cand.page == e.page {
					b = cand
					break
				}
			}
			if b == nil {
				b = &glyphBatch{atlas: atlas, page: e.page}
				batches = append(batches, b)
			}
			b.quads = append(b.quads, render.GlyphQuad{
				Dst: render.Rect{
					X: gx,
					Y: gy,
					W: float32(e.w) / scale,
					H: float32(e.h) / scale,
				},
				Src: e.uv,
			})
		} else if err != nil {
			// The glyph couldn't be placed anywhere (see glyphCoverage: the
			// degenerate oversized-glyph case, since ordinary overflow
			// grows a new page instead); still advance the pen below, same
			// as before OnGlyphDropped existed — this only reports the
			// failure, it never changes it.
			fa.reportDropped(ch)
		}

		penX += srcFont.advance(gi, fa.SizePx)
		prevFont, prev, hasPrev = srcFont, gi, true
	}

	for _, b := range batches {
		r.DrawGlyphs(b.quads, b.atlas.ensureCoverageTexture(r, b.page), c)
	}
}
