package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/theme"
)

func TestAcrylicSurfaceMeasureAddsPadding(t *testing.T) {
	a := NewAcrylicSurface().
		SetPadding(render.Uniform(10)).
		SetChild(NewFixed(50, 20, render.RGB(0, 120, 215)))
	core.MeasureWidget(a, render.Size{W: 500, H: 500})
	if got := a.DesiredSize(); got != (render.Size{W: 70, H: 40}) { // 50+2*10, 20+2*10
		t.Fatalf("desired=%v", got)
	}
}

func TestAcrylicSurfaceArrangesChildInset(t *testing.T) {
	f := NewFixed(50, 20, render.RGB(0, 120, 215))
	f.SetAlign(core.Start, core.Start)
	a := NewAcrylicSurface().SetPadding(render.Uniform(10)).SetChild(f)
	core.MeasureWidget(a, render.Size{W: 500, H: 500})
	core.ArrangeWidget(a, render.Rect{X: 100, Y: 100, W: 70, H: 40})
	if got := f.Bounds(); got != (render.Rect{X: 110, Y: 110, W: 50, H: 20}) {
		t.Fatalf("child bounds=%v", got)
	}
}

func TestAcrylicSurfaceDetachesReplacedChild(t *testing.T) {
	oldChild := NewFixed(10, 10, render.RGB(1, 2, 3))
	newChild := NewFixed(20, 20, render.RGB(4, 5, 6))

	a := NewAcrylicSurface().SetChild(oldChild)
	core.MeasureWidget(a, render.Size{W: 100, H: 100})
	core.ArrangeWidget(a, render.Rect{X: 0, Y: 0, W: 100, H: 100})

	a.SetChild(newChild)
	core.MeasureWidget(a, render.Size{W: 100, H: 100})
	core.ArrangeWidget(a, render.Rect{X: 0, Y: 0, W: 100, H: 100})
	if a.NeedsLayout() {
		t.Fatal("surface should be clean after measure+arrange following SetChild")
	}

	oldChild.InvalidateMeasure()
	if a.NeedsLayout() {
		t.Fatal("invalidating the detached old child must not dirty the surface")
	}
}

func TestAcrylicSurfaceNoChild(t *testing.T) {
	a := NewAcrylicSurface().SetPadding(render.Uniform(8))
	core.MeasureWidget(a, render.Size{W: 500, H: 500})
	if got := a.DesiredSize(); got != (render.Size{W: 16, H: 16}) {
		t.Fatalf("desired=%v", got)
	}
	if a.Children() != nil {
		t.Fatal("no-child AcrylicSurface must return nil Children")
	}
}

func TestAcrylicSurfaceDefaultTintFromTheme(t *testing.T) {
	theme.SetActive(theme.FluentLight())
	defer theme.SetActive(nil)

	a := NewAcrylicSurface()
	if a.tint != theme.Active().Color.AcrylicTint {
		t.Fatalf("default tint = %v, want theme AcrylicTint %v", a.tint, theme.Active().Color.AcrylicTint)
	}
}

func TestAcrylicSurfaceSetTintOverrides(t *testing.T) {
	a := NewAcrylicSurface()
	custom := render.RGBA(10, 20, 30, 200)
	a.SetTint(custom)
	if a.tint != custom {
		t.Fatalf("tint=%v, want %v", a.tint, custom)
	}
}
