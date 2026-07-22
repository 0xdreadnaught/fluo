package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
)

func TestWrapFlows(t *testing.T) {
	w := NewWrapPanel().SetGap(0)
	var kids []*Fixed
	for i := 0; i < 3; i++ {
		k := NewFixed(60, 20, render.RGB(1, 2, 3))
		k.SetAlign(core.Start, core.Start)
		kids = append(kids, k)
		w.Add(k)
	}
	core.MeasureWidget(w, render.Size{W: 130, H: 500}) // fits 2 per row
	if got := w.DesiredSize(); got != (render.Size{W: 120, H: 40}) { t.Fatalf("desired=%v", got) }
	core.ArrangeWidget(w, render.Rect{X: 0, Y: 0, W: 130, H: 500})
	if got := kids[1].Bounds(); got != (render.Rect{X: 60, Y: 0, W: 60, H: 20}) { t.Fatalf("k1=%v", got) }
	if got := kids[2].Bounds(); got != (render.Rect{X: 0, Y: 20, W: 60, H: 20}) { t.Fatalf("k2 wrapped=%v", got) }
}

func TestWrapGap(t *testing.T) {
	w := NewWrapPanel().SetGap(10).
		Add(NewFixed(60, 20, render.RGB(1, 2, 3)), NewFixed(60, 20, render.RGB(1, 2, 3)))
	core.MeasureWidget(w, render.Size{W: 200, H: 500})
	if got := w.DesiredSize(); got != (render.Size{W: 130, H: 20}) { t.Fatalf("desired=%v", got) }
}

func TestWrapOversizeChildGetsOwnRow(t *testing.T) {
	w := NewWrapPanel().
		Add(NewFixed(300, 10, render.RGB(1, 2, 3)), NewFixed(20, 10, render.RGB(1, 2, 3)))
	core.MeasureWidget(w, render.Size{W: 100, H: 500})
	if got := w.DesiredSize().H; got != 20 { t.Fatalf("H=%v (two rows)", got) }
}
