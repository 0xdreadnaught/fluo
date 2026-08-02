package controls

import (
	"math"
	"testing"

	"github.com/0xdreadnaught/fluo/render"
)

// TestVirtualizerRowTopNormalCountParity guards FIX #17's golden-safety claim:
// at normal counts (idx < 2^24) the float64-accumulated rowTop must be
// bit-identical to the legacy inline float32(idx)*rowH formula, so no rendered
// row position — and no golden — shifts.
func TestVirtualizerRowTopNormalCountParity(t *testing.T) {
	v := virtualizer{rowH: 20.5, viewport: render.Rect{X: 10, Y: 30, W: 200, H: 400}}
	v.offset = 137.25

	for _, idx := range []int{0, 1, 2, 137, 1000, 100000, 1 << 23} {
		legacy := v.viewport.Y + float32(idx)*v.rowH - v.offset
		if got := v.rowTop(idx); got != legacy {
			t.Fatalf("rowTop(%d) = %v, want legacy %v (normal counts must be bit-identical)", idx, got, legacy)
		}
	}
}

// TestVirtualizerRowTopHitTestInverse is the core FIX #17 property: draw
// (rowTop) and hit-test (rowIndexAt) are exact inverses — a click in the middle
// of row idx's band resolves back to idx — including at a large idx with a
// fractional row height, where the two must not drift apart.
func TestVirtualizerRowTopHitTestInverse(t *testing.T) {
	v := virtualizer{rowH: 20.5, viewport: render.Rect{X: 0, Y: 0, W: 200, H: 400}}

	for _, idx := range []int{0, 1, 42, 1000, 100000, 1_000_000} {
		mid := v.rowTop(idx) + v.rowH*0.5
		if got := v.rowIndexAt(mid); got != idx {
			t.Fatalf("rowIndexAt(rowTop(%d)+h/2) = %d, want %d (draw/hit-test must be inverses)", idx, got, idx)
		}
	}
}

// TestVirtualizerRowIndexAtLargeOffset exercises the actual "clicks select the
// wrong row at very large counts" bug. When the list is scrolled far down, the
// old inline hit-test added the small pointer y to the large scroll offset in
// float32 before dividing — quantizing the sum (ULP grows with the offset) and
// flooring to the wrong row. rowIndexAt now keeps the whole computation in
// float64. This asserts (a) rowIndexAt always matches the float64-correct row,
// and (b) that at least one pointer position within a screen's worth of clicks
// actually diverges from the old float32 formula, so the test can't pass by the
// two formulas trivially agreeing.
func TestVirtualizerRowIndexAtLargeOffset(t *testing.T) {
	v := virtualizer{rowH: 20.5, viewport: render.Rect{X: 0, Y: 0, W: 200, H: 400}}
	// Scrolled ~14.6M rows down; 300000000 is exactly representable in float32
	// (a multiple of its 32-unit ULP at that magnitude), so the offset itself
	// introduces no rounding — the divergence below comes purely from how each
	// formula combines it with the pointer y.
	v.offset = 300000000

	diverged := false
	for p := 0; p < 4096; p++ {
		posY := float32(p)
		got := v.rowIndexAt(posY)

		want := int(math.Floor((float64(posY) - float64(v.viewport.Y) + float64(v.offset)) / float64(v.rowH)))
		if got != want {
			t.Fatalf("rowIndexAt(%v) = %d, want %d (float64-correct row)", posY, got, want)
		}

		naive := int(math.Floor(float64((posY - v.viewport.Y + v.offset) / v.rowH)))
		if naive != got {
			diverged = true
		}
	}
	if !diverged {
		t.Fatal("no divergence from the old float32 hit-test found; offset too small to exhibit the bug")
	}
}
