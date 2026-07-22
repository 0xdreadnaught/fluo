package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
)

func TestBorderMeasureAddsChrome(t *testing.T) {
	b := NewBorder().
		SetPadding(render.Uniform(10)).
		SetBorder(render.RGB(255, 255, 255), 2).
		SetChild(NewFixed(50, 20, render.RGB(0, 120, 215)))
	core.MeasureWidget(b, render.Size{W: 500, H: 500})
	if got := b.DesiredSize(); got != (render.Size{W: 74, H: 44}) { // 50+2*10+2*2, 20+2*10+2*2
		t.Fatalf("desired=%v", got)
	}
}

func TestBorderArrangesChildInset(t *testing.T) {
	f := NewFixed(50, 20, render.RGB(0, 120, 215))
	f.SetAlign(core.Start, core.Start)
	b := NewBorder().SetPadding(render.Uniform(10)).SetChild(f)
	core.MeasureWidget(b, render.Size{W: 500, H: 500})
	core.ArrangeWidget(b, render.Rect{X: 100, Y: 100, W: 70, H: 40})
	if got := f.Bounds(); got != (render.Rect{X: 110, Y: 110, W: 50, H: 20}) {
		t.Fatalf("child bounds=%v", got)
	}
}

func TestBorderNoChild(t *testing.T) {
	b := NewBorder().SetPadding(render.Uniform(8))
	core.MeasureWidget(b, render.Size{W: 500, H: 500})
	if got := b.DesiredSize(); got != (render.Size{W: 16, H: 16}) {
		t.Fatalf("desired=%v", got)
	}
	if b.Children() != nil {
		t.Fatal("no-child Border must return nil Children")
	}
}
