package text

import (
	"errors"
	"image"
	"image/draw"

	"golang.org/x/image/font/sfnt"

	"github.com/0xdreadnaught/fluo/render"
)

// atlasSize is the fixed width/height, in pixels, of every backing image
// and GPU texture in an Atlas — that is, of each individual coverage page
// (see coveragePage). The atlas itself grows past one atlasSize x
// atlasSize page by appending more of them (see glyphCoverage); the page
// size stays fixed.
const atlasSize = 1024

// Atlas packs grayscale coverage masks for a single Font into an ordered
// list of fixed-size pages, uploading each to its own GPU texture lazily
// and incrementally. It is not safe for concurrent use.
//
// Growth (see glyphCoverage) appends pages; it never removes or reflows
// one, so entry.page indices already handed out stay valid for the
// Atlas's lifetime.
type Atlas struct {
	f *Font

	covPages   []*coveragePage
	covEntries map[coverageKey]coverageEntry
}

// coveragePage is a single fixed atlasSize x atlasSize backing image for
// the coverage atlas, plus its own shelf-packer cursor, dirty-rect list,
// and GPU texture. The coverage atlas starts with one page and grows by
// appending more (see Atlas.glyphCoverage) rather than capping out, so a
// document with many distinct glyphs — e.g. a large multilingual string
// spanning several fallback fonts — is never forced to drop a glyph just
// because earlier ones filled page 0.
type coveragePage struct {
	img                    *image.Alpha
	cursorX, cursorY, rowH int
	dirty                  []image.Rectangle
	tex                    render.TextureID
}

// newCoveragePage returns an empty page ready for shelf-packing.
func newCoveragePage() *coveragePage {
	return &coveragePage{img: image.NewAlpha(image.Rect(0, 0, atlasSize, atlasSize))}
}

// coverageKey identifies a cached coverage entry: a glyph rasterized at a
// specific integer device-pixel size, since coverage masks are NOT
// resolution-independent — a new size means a new raster.
type coverageKey struct {
	gi sfnt.GlyphIndex
	px int
}

// coverageEntry describes where a single glyph's grayscale coverage mask
// (rasterized at a specific px, see coverageKey) lives within Atlas's
// coverage image, plus the bearings needed to place it at draw time. No
// advance is stored here: layout/advance always comes from the logical-px
// metrics path (Font.advance), independent of rasterization.
type coverageEntry struct {
	page               int         // index into Atlas.covPages this entry's mask is packed into
	uv                 render.Rect // 0..1 within that page's texture
	w, h               int         // px in the atlas (== device px at this size)
	bearingX, bearingY float32     // device px, at this entry's px size
	empty              bool        // true when the mask has no non-zero coverage (e.g. space): Face.Draw skips emitting a quad but still advances the pen
}

// NewAtlas creates an empty Atlas backed by f. Glyphs are rasterized
// and packed lazily as glyphCoverage is called.
func NewAtlas(f *Font) *Atlas {
	return &Atlas{
		f:          f,
		covPages:   []*coveragePage{newCoveragePage()},
		covEntries: make(map[coverageKey]coverageEntry),
	}
}

// covPad is the transparent border rasterGlyph already bakes into every
// coverage mask it returns (see rasterGlyph's pad parameter). Reusing that
// existing border — rather than adding a second one — is enough to give
// >=1px of transparent separation between neighboring packed glyphs, since
// the shelf packer places mask bounds (including their pad ring) edge to
// edge: two adjacent glyphs each contribute covPad transparent pixels at
// their shared border, so bilinear sampling at the atlas seam never bleeds
// a neighbor's coverage in.
const covPad = 1

// glyphCoverage returns the coverageEntry for gi rasterized at px device
// pixels, rasterizing and packing it into the atlas on first request for
// this (gi, px) pair. Subsequent calls for the same pair return the cached
// entry; a different px yields a distinct entry, since coverage masks are
// rasterized directly at their draw size.
//
// The atlas grows rather than caps out: when the mask doesn't fit
// on the last page (the shelf packer has no room left in it), a fresh page
// is appended and the glyph is packed there instead — see coveragePage.
// The only case still reported as an error is a single mask too large to
// ever fit on a whole empty page, which no amount of growth can fix; the
// glyph itself is degenerate (see the atlasSize doc comment), not the
// atlas.
func (a *Atlas) glyphCoverage(gi sfnt.GlyphIndex, px int) (coverageEntry, error) {
	key := coverageKey{gi: gi, px: px}
	if e, ok := a.covEntries[key]; ok {
		return e, nil
	}

	mask, bearingX, bearingY, err := a.f.rasterGlyph(gi, float32(px), covPad)
	if err != nil {
		return coverageEntry{}, err
	}

	b := mask.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > atlasSize || h > atlasSize {
		return coverageEntry{}, errors.New("glyph too large for a single atlas page")
	}

	pageIdx := len(a.covPages) - 1
	page := a.covPages[pageIdx]

	// Shelf packing: advance to a new row if the glyph doesn't fit in the
	// remaining width of the current one.
	if page.cursorX+w > atlasSize {
		page.cursorY += page.rowH
		page.cursorX = 0
		page.rowH = 0
	}
	if page.cursorY+h > atlasSize {
		// This page has no room left: grow the atlas with a fresh page
		// rather than dropping the glyph.
		page = newCoveragePage()
		a.covPages = append(a.covPages, page)
		pageIdx = len(a.covPages) - 1
	}

	dst := image.Rect(page.cursorX, page.cursorY, page.cursorX+w, page.cursorY+h)
	draw.Draw(page.img, dst, mask, b.Min, draw.Src)
	page.dirty = append(page.dirty, dst)

	e := coverageEntry{
		page: pageIdx,
		uv: render.Rect{
			X: float32(page.cursorX) / atlasSize,
			Y: float32(page.cursorY) / atlasSize,
			W: float32(w) / atlasSize,
			H: float32(h) / atlasSize,
		},
		w:        w,
		h:        h,
		bearingX: bearingX,
		bearingY: bearingY,
		empty:    maskEmpty(mask),
	}
	a.covEntries[key] = e

	page.cursorX += w
	if h > page.rowH {
		page.rowH = h
	}

	return e, nil
}

// maskEmpty reports whether m has no non-zero-alpha pixel at all — true
// for glyphs with no outline (e.g. space), which rasterGlyph represents as
// an all-transparent pad-square mask.
func maskEmpty(m *image.Alpha) bool {
	for _, v := range m.Pix {
		if v != 0 {
			return false
		}
	}
	return true
}

// ensureCoverageTexture returns the GPU texture for coverage-atlas page
// index page, creating it on first use and uploading any regions of that
// page that have changed since the last call. Each page owns its texture,
// so their GPU resources stay independent of one another.
func (a *Atlas) ensureCoverageTexture(r render.Renderer, page int) render.TextureID {
	p := a.covPages[page]
	if p.tex == render.NoTexture {
		p.tex = r.CreateTexture(atlasSize, atlasSize, nil)
	}

	for _, rect := range p.dirty {
		w, h := rect.Dx(), rect.Dy()
		rgba := make([]byte, w*h*4)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				v := p.img.AlphaAt(rect.Min.X+x, rect.Min.Y+y).A
				i := (y*w + x) * 4
				rgba[i+0] = v
				rgba[i+1] = v
				rgba[i+2] = v
				rgba[i+3] = v
			}
		}
		r.UpdateTexture(p.tex, rect.Min.X, rect.Min.Y, w, h, rgba)
	}
	p.dirty = nil

	return p.tex
}
