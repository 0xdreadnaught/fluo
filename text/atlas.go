package text

import (
	"errors"
	"image"
	"image/draw"

	"golang.org/x/image/font/sfnt"

	"github.com/0xdreadnaught/fluo/render"
)

// atlasSize is the fixed width/height, in pixels, of an Atlas's
// backing image and GPU texture. Growing beyond this is a later-phase
// concern (see glyph's "atlas full" error).
const atlasSize = 1024

// atlasEntry describes where a single glyph's SDF lives within an
// Atlas, plus the layout metrics needed to place and size it at draw
// time. uv, bearings, and advance are all expressed at sdfRasterPx;
// callers scale by (drawSizePx / sdfRasterPx) to get other sizes.
type atlasEntry struct {
	uv                 render.Rect // 0..1 within the atlas texture
	w, h               int         // px in the atlas
	bearingX, bearingY float32     // at sdfRasterPx scale
	advance            float32     // at sdfRasterPx scale
}

// Atlas packs SDF glyph masks for a single Font into one square alpha
// image, uploading it to a GPU texture lazily and incrementally. It
// is not safe for concurrent use.
type Atlas struct {
	f *Font

	img *image.Alpha

	// Shelf packer state: the next placement is at (cursorX, cursorY);
	// rowH is the tallest glyph placed in the current row so far.
	cursorX, cursorY int
	rowH             int

	entries map[sfnt.GlyphIndex]atlasEntry
	dirty   []image.Rectangle

	tex render.TextureID

	// Coverage atlas: a dedicated backing image/texture/shelf-cursor/
	// entries for direct grayscale-AA glyphs (text.Face's HD draw path,
	// see glyphCoverage), kept fully independent of the SDF fields above
	// so packing and goldens never collide between the two systems.
	covImg                 *image.Alpha
	covCursorX, covCursorY int
	covRowH                int
	covEntries             map[coverageKey]coverageEntry
	covDirty               []image.Rectangle
	covTex                 render.TextureID
}

// coverageKey identifies a cached coverage entry: a glyph rasterized at a
// specific integer device-pixel size, since (unlike the SDF path) coverage
// masks are NOT resolution-independent — a new size means a new raster.
type coverageKey struct {
	gi sfnt.GlyphIndex
	px int
}

// coverageEntry describes where a single glyph's grayscale coverage mask
// (rasterized at a specific px, see coverageKey) lives within Atlas's
// coverage image, plus the bearings needed to place it at draw time.
// Unlike atlasEntry, no advance is stored here: layout/advance always
// comes from the logical-px metrics path (Font.advance), unchanged by
// which rendering path draws the glyph.
type coverageEntry struct {
	uv                 render.Rect // 0..1 within the coverage atlas texture
	w, h               int         // px in the atlas (== device px at this size)
	bearingX, bearingY float32     // device px, at this entry's px size
	empty              bool        // true when the mask has no non-zero coverage (e.g. space): Face.Draw skips emitting a quad but still advances the pen
}

// NewAtlas creates an empty Atlas backed by f. Glyphs are rasterized
// and packed lazily as glyph is called.
func NewAtlas(f *Font) *Atlas {
	return &Atlas{
		f:          f,
		img:        image.NewAlpha(image.Rect(0, 0, atlasSize, atlasSize)),
		entries:    make(map[sfnt.GlyphIndex]atlasEntry),
		covImg:     image.NewAlpha(image.Rect(0, 0, atlasSize, atlasSize)),
		covEntries: make(map[coverageKey]coverageEntry),
	}
}

// glyph returns the atlasEntry for gi, rasterizing, computing its SDF,
// and packing it into the atlas on first request. Subsequent calls
// for the same gi return the cached entry.
func (a *Atlas) glyph(gi sfnt.GlyphIndex) (atlasEntry, error) {
	if e, ok := a.entries[gi]; ok {
		return e, nil
	}

	mask, bearingX, bearingY, err := a.f.rasterGlyph(gi, sdfRasterPx, sdfPad)
	if err != nil {
		return atlasEntry{}, err
	}
	sdf := sdfFromMask(mask)

	b := sdf.Bounds()
	w, h := b.Dx(), b.Dy()

	// Shelf packing: advance to a new row if the glyph doesn't fit in
	// the remaining width of the current one.
	if a.cursorX+w > atlasSize {
		a.cursorY += a.rowH
		a.cursorX = 0
		a.rowH = 0
	}
	if a.cursorY+h > atlasSize {
		return atlasEntry{}, errors.New("atlas full")
	}

	dst := image.Rect(a.cursorX, a.cursorY, a.cursorX+w, a.cursorY+h)
	draw.Draw(a.img, dst, sdf, b.Min, draw.Src)
	a.dirty = append(a.dirty, dst)

	e := atlasEntry{
		uv: render.Rect{
			X: float32(a.cursorX) / atlasSize,
			Y: float32(a.cursorY) / atlasSize,
			W: float32(w) / atlasSize,
			H: float32(h) / atlasSize,
		},
		w:        w,
		h:        h,
		bearingX: bearingX,
		bearingY: bearingY,
		advance:  a.f.advance(gi, sdfRasterPx),
	}
	a.entries[gi] = e

	a.cursorX += w
	if h > a.rowH {
		a.rowH = h
	}

	return e, nil
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
// pixels, rasterizing and packing it into the dedicated coverage atlas on
// first request for this (gi, px) pair. Subsequent calls for the same pair
// return the cached entry; a different px yields a distinct entry, since
// coverage masks (unlike SDFs) are rasterized directly at their draw size.
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

	// Shelf packing: advance to a new row if the glyph doesn't fit in the
	// remaining width of the current one.
	if a.covCursorX+w > atlasSize {
		a.covCursorY += a.covRowH
		a.covCursorX = 0
		a.covRowH = 0
	}
	if a.covCursorY+h > atlasSize {
		return coverageEntry{}, errors.New("atlas full")
	}

	dst := image.Rect(a.covCursorX, a.covCursorY, a.covCursorX+w, a.covCursorY+h)
	draw.Draw(a.covImg, dst, mask, b.Min, draw.Src)
	a.covDirty = append(a.covDirty, dst)

	e := coverageEntry{
		uv: render.Rect{
			X: float32(a.covCursorX) / atlasSize,
			Y: float32(a.covCursorY) / atlasSize,
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

	a.covCursorX += w
	if h > a.covRowH {
		a.covRowH = h
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

// ensureCoverageTexture returns a's coverage-atlas GPU texture, creating it
// on first use and uploading any regions that have changed since the last
// call. Mirrors ensureTexture but for the dedicated coverage image/texture
// (see the covImg/covTex fields), keeping the two atlases' GPU resources
// independent.
func (a *Atlas) ensureCoverageTexture(r render.Renderer) render.TextureID {
	if a.covTex == render.NoTexture {
		a.covTex = r.CreateTexture(atlasSize, atlasSize, nil)
	}

	for _, rect := range a.covDirty {
		w, h := rect.Dx(), rect.Dy()
		rgba := make([]byte, w*h*4)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				v := a.covImg.AlphaAt(rect.Min.X+x, rect.Min.Y+y).A
				i := (y*w + x) * 4
				rgba[i+0] = v
				rgba[i+1] = v
				rgba[i+2] = v
				rgba[i+3] = v
			}
		}
		r.UpdateTexture(a.covTex, rect.Min.X, rect.Min.Y, w, h, rgba)
	}
	a.covDirty = nil

	return a.covTex
}

// ensureTexture returns a's GPU texture, creating it on first use and
// uploading any regions that have changed since the last call.
func (a *Atlas) ensureTexture(r render.Renderer) render.TextureID {
	if a.tex == render.NoTexture {
		a.tex = r.CreateTexture(atlasSize, atlasSize, nil)
	}

	for _, rect := range a.dirty {
		w, h := rect.Dx(), rect.Dy()
		rgba := make([]byte, w*h*4)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				v := a.img.AlphaAt(rect.Min.X+x, rect.Min.Y+y).A
				i := (y*w + x) * 4
				rgba[i+0] = v
				rgba[i+1] = v
				rgba[i+2] = v
				rgba[i+3] = v
			}
		}
		r.UpdateTexture(a.tex, rect.Min.X, rect.Min.Y, w, h, rgba)
	}
	a.dirty = nil

	return a.tex
}
