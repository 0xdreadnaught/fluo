package input_test

import (
	"testing"

	"github.com/0xdreadnaught/fluo/controls"
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
)

func layout(root core.Widget, w, h float32) {
	core.MeasureWidget(root, render.Size{W: w, H: h})
	core.ArrangeWidget(root, render.Rect{X: 0, Y: 0, W: w, H: h})
}

func TestHitPathTopmostWins(t *testing.T) {
	a := controls.NewFixed(50, 50, render.RGB(1, 0, 0))
	b := controls.NewFixed(50, 50, render.RGB(0, 1, 0)) // added later => painted on top
	c := controls.NewCanvas().Add(a, 0, 0).Add(b, 25, 25)
	layout(c, 100, 100)
	path := input.HitPath(c, render.Point{X: 40, Y: 40}) // inside both; b wins
	if len(path) != 2 || path[0] != core.Widget(c) || path[1] != core.Widget(b) {
		t.Fatalf("path=%v", path)
	}
}

func TestHitPathMiss(t *testing.T) {
	c := controls.NewCanvas().Add(controls.NewFixed(10, 10, render.RGB(1, 0, 0)), 0, 0)
	layout(c, 100, 100)
	if p := input.HitPath(c, render.Point{X: 50, Y: 50}); len(p) != 1 {
		t.Fatalf("expected canvas-only path (canvas stretches to 100x100), got %v", p)
	}
	if p := input.HitPath(c, render.Point{X: 200, Y: 200}); p != nil {
		t.Fatalf("outside root: %v", p)
	}
}

func TestHitPathSkipsHidden(t *testing.T) {
	a := controls.NewFixed(50, 50, render.RGB(1, 0, 0))
	a.SetVisible(false)
	c := controls.NewCanvas().Add(a, 0, 0)
	layout(c, 100, 100)
	path := input.HitPath(c, render.Point{X: 10, Y: 10})
	if len(path) != 1 {
		t.Fatalf("hidden child hit: %v", path)
	}
}

func TestHitPathNested(t *testing.T) {
	leaf := controls.NewFixed(20, 20, render.RGB(1, 0, 0))
	inner := controls.NewCanvas().Add(leaf, 5, 5)
	inner.SetWidth(50)
	inner.SetHeight(50)
	outer := controls.NewCanvas().Add(inner, 10, 10)
	layout(outer, 100, 100)
	path := input.HitPath(outer, render.Point{X: 20, Y: 20}) // inside leaf (15..35)
	if len(path) != 3 || path[2] != core.Widget(leaf) {
		t.Fatalf("path=%v", path)
	}
}
