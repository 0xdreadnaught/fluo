package app_test

import (
	"testing"
	"time"

	"github.com/0xdreadnaught/fluo/app"
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
)

// mockRenderer records the calls Frame makes on it, without touching any
// real GL state — enough for headless assertions about what Surface.Frame
// did on a given call.
type mockRenderer struct {
	begins    int
	lastFBW   int
	lastFBH   int
	lastScale float32
	ends      int
	fillRects int
}

func (m *mockRenderer) Begin(fbWidth, fbHeight int, scale float32) {
	m.begins++
	m.lastFBW, m.lastFBH, m.lastScale = fbWidth, fbHeight, scale
}
func (m *mockRenderer) End()                                                                        { m.ends++ }
func (m *mockRenderer) FillRect(r render.Rect, c render.Color)                                      { m.fillRects++ }
func (m *mockRenderer) FillRoundedRect(r render.Rect, radius float32, c render.Color)               {}
func (m *mockRenderer) DrawGradientRect(r render.Rect, from, to render.Color, horizontal bool)      {}
func (m *mockRenderer) StrokeRoundedRect(r render.Rect, radius, width float32, c render.Color)      {}
func (m *mockRenderer) DrawShadow(r render.Rect, radius, blur float32, c render.Color)              {}
func (m *mockRenderer) DrawBackdropBlur(r render.Rect, radius float32, tint render.Color)           {}
func (m *mockRenderer) CreateTexture(w, h int, rgba []byte) render.TextureID                        { return render.NoTexture }
func (m *mockRenderer) UpdateTexture(id render.TextureID, x, y, w, h int, rgba []byte)              {}
func (m *mockRenderer) DeleteTexture(id render.TextureID)                                           {}
func (m *mockRenderer) DrawQuad(dst, src render.Rect, tex render.TextureID, tint render.Color)      {}
func (m *mockRenderer) DrawSDFQuads(quads []render.GlyphQuad, tex render.TextureID, c render.Color) {}
func (m *mockRenderer) PushClip(r render.Rect)                                                      {}
func (m *mockRenderer) PopClip()                                                                    {}

// probe is a minimal widget: a bare core.Element (Stretch-aligned, so
// ArrangeWidget fills whatever rect it's given — a convenient signal that
// layout ran) that also records pointer events delivered to it, so the
// Pointer* forwarders can be asserted against without reaching through
// Router().
type probe struct {
	core.Element
	renders     int
	measures    int
	pointerHits []string
}

func (p *probe) MeasureContent(available render.Size) render.Size {
	p.measures++
	return available
}

func (p *probe) Render(r render.Renderer) { p.renders++ }

func (p *probe) OnPointer(e *input.PointerEvent) {
	switch e.Action {
	case input.Press:
		p.pointerHits = append(p.pointerHits, "press")
	case input.Release:
		p.pointerHits = append(p.pointerHits, "release")
	case input.Wheel:
		p.pointerHits = append(p.pointerHits, "wheel")
	}
}

func TestFrameLaysOutOnFirstCallAndOnSizeChange(t *testing.T) {
	s := app.NewSurface()
	root := &probe{}
	s.SetRoot(root)
	r := &mockRenderer{}

	// First call: fresh root (NeedsLayout true out of the box) — must lay
	// out and render.
	s.Frame(r, 100, 50, 100, 50)
	if root.measures != 1 {
		t.Fatalf("measures after first Frame = %d, want 1", root.measures)
	}
	if got, want := root.Bounds(), (render.Rect{X: 0, Y: 0, W: 100, H: 50}); got != want {
		t.Fatalf("bounds after first Frame = %+v, want %+v", got, want)
	}
	if root.renders != 1 {
		t.Fatalf("renders after first Frame = %d, want 1", root.renders)
	}

	// Second call, same size, root now clean: must NOT re-measure, but
	// still renders every Frame.
	s.Frame(r, 100, 50, 100, 50)
	if root.measures != 1 {
		t.Fatalf("measures after unchanged-size Frame = %d, want still 1 (skip)", root.measures)
	}
	if root.renders != 2 {
		t.Fatalf("renders after second Frame = %d, want 2", root.renders)
	}

	// Third call, size changed: must re-measure even though the root
	// itself was never invalidated.
	s.Frame(r, 200, 80, 200, 80)
	if root.measures != 2 {
		t.Fatalf("measures after size-change Frame = %d, want 2", root.measures)
	}
	if got, want := root.Bounds(), (render.Rect{X: 0, Y: 0, W: 200, H: 80}); got != want {
		t.Fatalf("bounds after size-change Frame = %+v, want %+v", got, want)
	}

	// Fourth call, same size again but root explicitly dirtied: must
	// re-measure even though size didn't change.
	root.InvalidateMeasure()
	s.Frame(r, 200, 80, 200, 80)
	if root.measures != 3 {
		t.Fatalf("measures after explicit-dirty Frame = %d, want 3", root.measures)
	}
}

func TestFrameComputesScale(t *testing.T) {
	s := app.NewSurface()
	s.SetRoot(&probe{})
	r := &mockRenderer{}

	s.Frame(r, 100, 50, 200, 100) // 2x DPI
	if r.begins != 1 {
		t.Fatalf("begins = %d, want 1", r.begins)
	}
	if r.lastScale != 2 {
		t.Fatalf("scale = %v, want 2", r.lastScale)
	}
	if r.lastFBW != 200 || r.lastFBH != 100 {
		t.Fatalf("fb size = %dx%d, want 200x100", r.lastFBW, r.lastFBH)
	}
	if r.ends != 1 {
		t.Fatalf("ends = %d, want 1", r.ends)
	}
}

func TestFrameScaleGuardsZeroWinWidth(t *testing.T) {
	s := app.NewSurface()
	s.SetRoot(&probe{})
	r := &mockRenderer{}

	s.Frame(r, 0, 0, 0, 0) // minimized window: winW == 0
	if r.lastScale != 1 {
		t.Fatalf("scale with winW==0 = %v, want 1 (guarded default)", r.lastScale)
	}
}

func TestFrameWithNilRootOnlyAdvancesTimers(t *testing.T) {
	s := app.NewSurface()
	r := &mockRenderer{}

	fired := false
	s.Timers().After(-time.Hour, func() { fired = true })

	s.Frame(r, 100, 100, 100, 100)

	if !fired {
		t.Fatal("timer scheduled in the past did not fire on Frame")
	}
	if r.begins != 0 || r.ends != 0 {
		t.Fatalf("begins=%d ends=%d, want 0/0 with no root set", r.begins, r.ends)
	}
}

func TestFrameAdvancesTimers(t *testing.T) {
	s := app.NewSurface()
	s.SetRoot(&probe{})
	r := &mockRenderer{}

	fired := false
	// due = construction-time(NewSurface) - 1h, already due relative to any
	// real time.Now() Frame advances against.
	s.Timers().After(-time.Hour, func() { fired = true })

	s.Frame(r, 10, 10, 10, 10)

	if !fired {
		t.Fatal("timer scheduled in the past did not fire during Frame")
	}
}

func TestPointerForwardersReachRouterAndProbe(t *testing.T) {
	s := app.NewSurface()
	root := &probe{}
	s.SetRoot(root)
	r := &mockRenderer{}
	s.Frame(r, 100, 100, 100, 100) // lay out (default Stretch) so root fills 0,0..100,100

	s.PointerMove(render.Point{X: 5, Y: 5}, 0)
	s.PointerButton(input.ButtonLeft, true, render.Point{X: 5, Y: 5}, 0)
	s.PointerButton(input.ButtonLeft, false, render.Point{X: 5, Y: 5}, 0)
	s.PointerWheel(render.Point{X: 0, Y: 1}, render.Point{X: 5, Y: 5}, 0)

	want := []string{"press", "release", "wheel"}
	if len(root.pointerHits) != len(want) {
		t.Fatalf("pointerHits = %v, want %v", root.pointerHits, want)
	}
	for i, w := range want {
		if root.pointerHits[i] != w {
			t.Fatalf("pointerHits[%d] = %q, want %q (full: %v)", i, root.pointerHits[i], w, root.pointerHits)
		}
	}
}

func TestKeyForwardersReachRouter(t *testing.T) {
	s := app.NewSurface()
	root := &probe{}
	s.SetRoot(root)

	// No focused widget and no panic is the bar here — KeyDown/KeyUp are
	// thin forwarders to Router().KeyDown/KeyUp, already covered in depth by
	// input's own tests; this just proves the forwarders reach it.
	s.KeyDown(input.Key('A'), 'a', 0)
	s.KeyUp(input.Key('A'), 0)
}

func TestRouterReturnsSameRouterUsedBySetRoot(t *testing.T) {
	s := app.NewSurface()
	root := &probe{}
	s.SetRoot(root)

	if s.Router().Root() != core.Widget(root) {
		t.Fatal("Router().Root() is not the widget passed to SetRoot")
	}
}
