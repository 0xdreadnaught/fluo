package text

import (
	"image"
	"image/draw"
	"math"

	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

// rasterGlyph renders the outline of gi at sizePx into a padded alpha
// mask. pad extra pixels are added around the tight outline bounds on
// every side, which gives room for antialiasing/filtering when the
// mask is later packed into an atlas (Task 11).
//
// bearingX and bearingY are the offset, in logical pixels, from the
// pen position (the glyph's baseline origin) to the mask's top-left
// corner. Because sfnt segment coordinates are y-down with the
// baseline at y=0, a glyph that rises above the baseline (nearly all
// of them) has a negative bearingY.
func (f *Font) rasterGlyph(gi sfnt.GlyphIndex, sizePx float32, pad int) (mask *image.Alpha, bearingX, bearingY float32, err error) {
	// LoadGlyph's returned Segments alias f.buf's backing array and
	// become invalid once the buffer is reused by another call, so we
	// must copy them out before releasing the lock.
	f.mu.Lock()
	raw, err := f.sf.LoadGlyph(&f.buf, gi, ppem(sizePx), nil)
	var segs sfnt.Segments
	if err == nil {
		segs = append(sfnt.Segments(nil), raw...)
	}
	f.mu.Unlock()
	if err != nil {
		return nil, 0, 0, err
	}

	if len(segs) == 0 {
		// e.g. space: no outline. Return an empty pad-square mask so
		// callers don't need a special case.
		size := 2 * pad
		if size <= 0 {
			size = 1
		}
		return image.NewAlpha(image.Rect(0, 0, size, size)), float32(-pad), float32(-pad), nil
	}

	// 1) bounds: min/max over every segment's used Args points.
	bounds := segs.Bounds()
	minX := float32(bounds.Min.X) / 64
	minY := float32(bounds.Min.Y) / 64
	maxX := float32(bounds.Max.X) / 64
	maxY := float32(bounds.Max.Y) / 64

	// 2) mask size: ceil(bounds) + 2*pad on each axis.
	w := int(math.Ceil(float64(maxX-minX))) + 2*pad
	h := int(math.Ceil(float64(maxY-minY))) + 2*pad
	if w <= 0 {
		w = 1
	}
	if h <= 0 {
		h = 1
	}

	// 3) rasterize, offsetting every point so the tight outline bounds
	// (expanded by pad) land inside [0, w) x [0, h).
	offX := -minX + float32(pad)
	offY := -minY + float32(pad)

	rast := vector.NewRasterizer(w, h)
	rast.DrawOp = draw.Src

	pt := func(p fixed.Point26_6) (float32, float32) {
		return float32(p.X)/64 + offX, float32(p.Y)/64 + offY
	}

	for _, seg := range segs {
		switch seg.Op {
		case sfnt.SegmentOpMoveTo:
			x, y := pt(seg.Args[0])
			rast.MoveTo(x, y)
		case sfnt.SegmentOpLineTo:
			x, y := pt(seg.Args[0])
			rast.LineTo(x, y)
		case sfnt.SegmentOpQuadTo:
			x1, y1 := pt(seg.Args[0])
			x2, y2 := pt(seg.Args[1])
			rast.QuadTo(x1, y1, x2, y2)
		case sfnt.SegmentOpCubeTo:
			x1, y1 := pt(seg.Args[0])
			x2, y2 := pt(seg.Args[1])
			x3, y3 := pt(seg.Args[2])
			rast.CubeTo(x1, y1, x2, y2, x3, y3)
		}
	}

	mask = image.NewAlpha(image.Rect(0, 0, w, h))
	rast.Draw(mask, mask.Bounds(), image.Opaque, image.Point{})

	// 5) bearings: offset from pen position to mask top-left.
	bearingX = minX - float32(pad)
	bearingY = minY - float32(pad)

	return mask, bearingX, bearingY, nil
}
