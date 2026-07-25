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
