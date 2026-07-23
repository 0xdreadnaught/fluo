package text

import (
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Font wraps a parsed sfnt font together with the scratch buffer sfnt
// requires for its calls. sfnt.Buffer is not safe for concurrent use,
// so every call that touches buf is guarded by mu.
type Font struct {
	sf  *sfnt.Font
	buf sfnt.Buffer
	mu  sync.Mutex

	// atlas is the shared SDF glyph atlas for this font: every Face
	// built from NewFace(f, ...) draws from the same Atlas, since
	// glyph SDFs are rasterized once at sdfRasterPx and reused at any
	// draw size. It is guarded by atlasMu rather than mu: fetching a
	// glyph (Atlas.glyph) calls back into Font.rasterGlyph, which
	// locks mu itself, so guarding the atlas pointer with mu would
	// deadlock any caller that held mu across that call. atlasMu only
	// ever protects the pointer/init, never a rasterGlyph call.
	atlas   *Atlas
	atlasMu sync.Mutex
}

// Load parses raw TrueType/OpenType font bytes into a Font.
func Load(ttf []byte) (*Font, error) {
	sf, err := sfnt.Parse(ttf)
	if err != nil {
		return nil, err
	}
	return &Font{sf: sf}, nil
}

// sharedAtlas returns f's shared SDF glyph atlas, creating it on first
// call. Every Face built for f reuses this same Atlas.
func (f *Font) sharedAtlas() *Atlas {
	f.atlasMu.Lock()
	defer f.atlasMu.Unlock()
	if f.atlas == nil {
		f.atlas = NewAtlas(f)
	}
	return f.atlas
}

// ppem converts a logical pixel size into the fixed-point units-per-em
// value the sfnt package expects.
func ppem(sizePx float32) fixed.Int26_6 {
	return fixed.Int26_6(sizePx * 64)
}

// glyphIndex resolves a rune to its glyph index. ok is false when the
// font has no glyph for r (index 0, the notdef glyph) or on error.
func (f *Font) glyphIndex(r rune) (sfnt.GlyphIndex, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	gi, err := f.sf.GlyphIndex(&f.buf, r)
	if err != nil || gi == 0 {
		return 0, false
	}
	return gi, true
}

// HasGlyph reports whether f actually has a glyph for r, as opposed to
// silently falling back to glyph index 0 (.notdef) the way Face.Measure and
// Face.Draw do. Callers that want to draw a specific rune (e.g. a Unicode
// symbol used as an icon) only when the font can really render it — falling
// back to a drawn shape otherwise — should gate on this rather than
// Measure's advance width, since a .notdef glyph can still have a nonzero
// advance.
func (f *Font) HasGlyph(r rune) bool {
	_, ok := f.glyphIndex(r)
	return ok
}

// advance returns the horizontal advance width of gi at sizePx, in
// logical pixels.
func (f *Font) advance(gi sfnt.GlyphIndex, sizePx float32) float32 {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, err := f.sf.GlyphAdvance(&f.buf, gi, ppem(sizePx), font.HintingNone)
	if err != nil {
		return 0
	}
	return float32(a) / 64
}

// kern returns the horizontal kerning adjustment between glyphs a and
// b at sizePx, in logical pixels. Fonts without a kern table (or any
// other lookup error) simply contribute no adjustment.
func (f *Font) kern(a, b sfnt.GlyphIndex, sizePx float32) float32 {
	f.mu.Lock()
	defer f.mu.Unlock()
	k, err := f.sf.Kern(&f.buf, a, b, ppem(sizePx), font.HintingNone)
	if err != nil {
		return 0
	}
	return float32(k) / 64
}

// metrics returns the font's ascent, descent, and line gap at sizePx,
// in logical pixels. ascent and descent are both returned as positive
// values.
func (f *Font) metrics(sizePx float32) (ascent, descent, lineGap float32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, err := f.sf.Metrics(&f.buf, ppem(sizePx), font.HintingNone)
	if err != nil {
		return 0, 0, 0
	}
	ascent = float32(m.Ascent) / 64
	descent = float32(m.Descent) / 64
	lineGap = float32(m.Height-m.Ascent-m.Descent) / 64
	return ascent, descent, lineGap
}
