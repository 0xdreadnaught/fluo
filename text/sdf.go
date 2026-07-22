package text

import (
	"image"
	"image/color"
	"math"
)

// sdfPad is both the padding baked into rasterGlyph's output mask and
// the spread (in pixels) at which the signed distance field saturates.
// The two must match: the SDF needs the padding ring to have room to
// encode the full ±sdfPad distance range.
const sdfPad = 8

// sdfRasterPx is the fixed size, in logical pixels, at which every
// glyph is rasterized before its SDF is computed and packed into the
// atlas. Text at other sizes is produced by scaling the SDF quad at
// draw time rather than re-rasterizing.
const sdfRasterPx = 48

// sdfFromMask converts a coverage mask into a signed distance field:
// 128 at the edge, >128 inside the glyph, <128 outside, saturating at
// ±sdfPad pixels from the nearest edge.
//
// A pixel is "inside" when its alpha is >= 128; out-of-bounds pixels
// are always treated as outside. An edge pixel is an inside pixel
// with at least one 4-connected neighbor that is outside. Distance is
// computed by brute force against the full set of edge pixels, which
// is plenty fast at the glyph sizes this package deals with (a few
// dozen pixels square).
func sdfFromMask(m *image.Alpha) *image.Alpha {
	b := m.Bounds()
	w, h := b.Dx(), b.Dy()

	inside := func(x, y int) bool {
		if x < 0 || x >= w || y < 0 || y >= h {
			return false
		}
		return m.AlphaAt(b.Min.X+x, b.Min.Y+y).A >= 128
	}

	type pt struct{ x, y int }
	var edges []pt
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !inside(x, y) {
				continue
			}
			if !inside(x-1, y) || !inside(x+1, y) || !inside(x, y-1) || !inside(x, y+1) {
				edges = append(edges, pt{x, y})
			}
		}
	}

	out := image.NewAlpha(b)
	if len(edges) == 0 {
		// No edges at all: mask is either fully empty or fully filled
		// with no boundary in view. Per spec, return all zeros.
		return out
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			minSq := -1
			for _, e := range edges {
				dx, dy := x-e.x, y-e.y
				sq := dx*dx + dy*dy
				if minSq < 0 || sq < minSq {
					minSq = sq
				}
			}
			d := float32(math.Sqrt(float64(minSq)))
			sign := float32(-1)
			if inside(x, y) {
				sign = 1
			}
			v := 128 + sign*d*128/float32(sdfPad)
			out.SetAlpha(b.Min.X+x, b.Min.Y+y, color.Alpha{A: clampAlpha(v)})
		}
	}

	return out
}

// clampAlpha rounds and clamps a float SDF value into the [0, 255]
// range an 8-bit alpha channel can hold.
func clampAlpha(v float32) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v + 0.5)
}
