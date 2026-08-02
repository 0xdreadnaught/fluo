package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
)

func TestGridPxAutoStar(t *testing.T) {
	g := NewGrid().
		Cols(Px(50), AutoTrack(), Star(1)).
		Rows(Px(30))
	g.Add(NewFixed(10, 10, render.RGB(1, 2, 3)), 0, 0)
	auto := NewFixed(40, 10, render.RGB(1, 2, 3))
	g.Add(auto, 0, 1)
	star := NewFixed(10, 10, render.RGB(1, 2, 3))
	g.Add(star, 0, 2)
	core.MeasureWidget(g, render.Size{W: 200, H: 100})
	core.ArrangeWidget(g, render.Rect{X: 0, Y: 0, W: 200, H: 100})
	// cols: 50 + 40(auto) + 110(star remainder); row 30 but grid stretches? grid itself Stretch fills 200x100; tracks use bounds.
	if got := auto.Bounds(); got != (render.Rect{X: 50, Y: 0, W: 40, H: 30}) {
		t.Fatalf("auto cell=%v", got)
	}
	if got := star.Bounds(); got != (render.Rect{X: 90, Y: 0, W: 110, H: 30}) {
		t.Fatalf("star cell=%v (stretch fills star track)", got)
	}
}

func TestGridStarWeights(t *testing.T) {
	g := NewGrid().Cols(Star(1), Star(3)).Rows(Star(1))
	a := NewFixed(1, 1, render.RGB(1, 2, 3))
	b := NewFixed(1, 1, render.RGB(1, 2, 3))
	g.Add(a, 0, 0)
	g.Add(b, 0, 1)
	core.MeasureWidget(g, render.Size{W: 400, H: 100})
	core.ArrangeWidget(g, render.Rect{X: 0, Y: 0, W: 400, H: 100})
	if got := a.Bounds().W; got != 100 {
		t.Fatalf("star1 W=%v", got)
	}
	if got := b.Bounds(); got.X != 100 || got.W != 300 {
		t.Fatalf("star3=%v", got)
	}
}

// TestResolveTracksEqualStarsTileExactly pins that equal Star tracks add
// back up to exactly the space they were resolved against. Each share used
// to be computed independently in float32, so three equal Stars across 200px
// summed to 200.00002 — enough for an exact "content wider than viewport"
// comparison downstream to report an overflow of a hundred-thousandth of a
// pixel and grow a scrollbar for it.
func TestResolveTracksEqualStarsTileExactly(t *testing.T) {
	none := func(int) float32 { return 0 }

	for _, n := range []int{2, 3, 4, 5, 6, 7, 9} {
		tracks := make([]Track, n)
		for i := range tracks {
			tracks[i] = Star(1)
		}
		for _, avail := range []float32{100, 200, 201, 300, 333, 640, 1000, 1024} {
			resolved := resolveTracks(tracks, avail, none)
			if got := sumF32(resolved); got != avail {
				t.Fatalf("%d equal Star tracks across %v summed to %v (off by %v), want exactly %v",
					n, avail, got, got-avail, avail)
			}
		}
	}
}

// TestResolveTracksMixedStarsTileExactly is the same rule with Px tracks in
// the mix and unequal weights: the Stars must consume exactly what the Px
// tracks left behind.
func TestResolveTracksMixedStarsTileExactly(t *testing.T) {
	none := func(int) float32 { return 0 }

	tracks := []Track{Px(80), Star(1), Px(60), Star(2), Star(1)}
	for _, avail := range []float32{300, 301, 500, 777} {
		resolved := resolveTracks(tracks, avail, none)
		if got := sumF32(resolved); got != avail {
			t.Fatalf("Px+Star tracks across %v summed to %v (off by %v), want exactly %v",
				avail, got, got-avail, avail)
		}
	}
}

// TestResolveTracksStarWeightsStillProportional guards the fix above from
// distorting the split it reconciles: handing the residual to the last Star
// must not visibly move any track off its weighted share.
func TestResolveTracksStarWeightsStillProportional(t *testing.T) {
	none := func(int) float32 { return 0 }

	resolved := resolveTracks([]Track{Star(1), Star(3)}, 400, none)
	if resolved[0] != 100 || resolved[1] != 300 {
		t.Fatalf("Star(1)/Star(3) across 400 = %v, want [100 300]", resolved)
	}

	resolved = resolveTracks([]Track{Star(1), Star(1), Star(1)}, 200, none)
	for i, w := range resolved {
		if diff := w - 200.0/3.0; diff > 0.001 || diff < -0.001 {
			t.Fatalf("track %d of 3 equal Stars across 200 = %v, want ~66.667", i, w)
		}
	}
}

func TestGridDesiredUnconstrained(t *testing.T) {
	g := NewGrid().Cols(Px(50), AutoTrack()).Rows(AutoTrack())
	g.Add(NewFixed(40, 25, render.RGB(1, 2, 3)), 0, 1)
	core.MeasureWidget(g, render.Size{W: 500, H: 500})
	if got := g.DesiredSize(); got != (render.Size{W: 90, H: 25}) {
		t.Fatalf("desired=%v", got)
	}
}

func TestGridAddOutOfRangePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	NewGrid().Cols(Px(10)).Rows(Px(10)).Add(NewFixed(1, 1, render.Color{}), 0, 5)
}

func TestGridShrinkTracksPanics(t *testing.T) {
	g := NewGrid().Cols(Px(10), Px(10), Px(10)).Rows(Px(10))
	g.Add(NewFixed(1, 1, render.Color{}), 0, 2)
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic")
			}
		}()
		g.Cols(Px(10))
	}()
}

func TestGridShrinkRowsPanics(t *testing.T) {
	g := NewGrid().Rows(Px(10), Px(10), Px(10)).Cols(Px(10))
	g.Add(NewFixed(1, 1, render.Color{}), 2, 0)
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic")
			}
		}()
		g.Rows(Px(10))
	}()
}
