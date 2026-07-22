// Package text provides pure-Go font loading and glyph outline
// rasterization on top of golang.org/x/image/font/sfnt. It has no
// dependency on any GL/windowing package; higher layers (Task 10-11)
// build glyph atlases and text layout on top of the primitives here.
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
}

// Load parses raw TrueType/OpenType font bytes into a Font.
func Load(ttf []byte) (*Font, error) {
	sf, err := sfnt.Parse(ttf)
	if err != nil {
		return nil, err
	}
	return &Font{sf: sf}, nil
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
