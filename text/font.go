package text

import (
	"fmt"
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

// collectionMagic is the four-byte "ttcf" tag that opens a TrueType/OpenType
// Collection (.ttc): multiple fonts packed into one file, e.g. the CJK
// system faces that ship several weights/scripts under one name. sfnt.Parse
// rejects this data with a low-level "invalid single font" message that
// doesn't say what to do about it; Load checks for the tag itself so it can
// point the caller at LoadCollection instead.
const collectionMagic = "ttcf"

// Load parses raw TrueType/OpenType font bytes into a Font. Font collection
// data (.ttc, identified by the "ttcf" tag) is rejected with an error
// pointing to LoadCollection or LoadCollectionMember, since a collection
// holds more than one font and Load returns exactly one.
func Load(ttf []byte) (*Font, error) {
	if len(ttf) >= 4 && string(ttf[:4]) == collectionMagic {
		return nil, fmt.Errorf("text: Load: data is a font collection (.ttc); use LoadCollection or LoadCollectionMember instead")
	}
	sf, err := sfnt.Parse(ttf)
	if err != nil {
		return nil, err
	}
	return newFont(sf), nil
}

// LoadCollection parses raw font-collection bytes (.ttc: a TrueType/OpenType
// Collection holding multiple fonts, e.g. the CJK system faces that bundle
// several weights or scripts in one file) into one Font per member, in
// collection order.
func LoadCollection(data []byte) ([]*Font, error) {
	c, err := sfnt.ParseCollection(data)
	if err != nil {
		return nil, err
	}
	n := c.NumFonts()
	fonts := make([]*Font, n)
	for i := 0; i < n; i++ {
		sf, err := c.Font(i)
		if err != nil {
			return nil, fmt.Errorf("text: LoadCollection: font %d: %w", i, err)
		}
		fonts[i] = newFont(sf)
	}
	return fonts, nil
}

// LoadCollectionMember parses raw font-collection bytes (see LoadCollection)
// and returns only the member at index, without materializing every other
// font in the collection.
func LoadCollectionMember(data []byte, index int) (*Font, error) {
	c, err := sfnt.ParseCollection(data)
	if err != nil {
		return nil, err
	}
	sf, err := c.Font(index)
	if err != nil {
		return nil, fmt.Errorf("text: LoadCollectionMember: font %d: %w", index, err)
	}
	return newFont(sf), nil
}

// newFont builds a Font from an already-parsed sfnt.Font. Load,
// LoadCollection, and LoadCollectionMember all funnel through this so a
// single font and a collection member are constructed identically.
func newFont(sf *sfnt.Font) *Font {
	return &Font{sf: sf}
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
