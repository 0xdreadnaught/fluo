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
}

// NewAtlas creates an empty Atlas backed by f. Glyphs are rasterized
// and packed lazily as glyph is called.
func NewAtlas(f *Font) *Atlas {
	return &Atlas{
		f:       f,
		img:     image.NewAlpha(image.Rect(0, 0, atlasSize, atlasSize)),
		entries: make(map[sfnt.GlyphIndex]atlasEntry),
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
