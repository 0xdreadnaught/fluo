# fluo Phase 2: Layout Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The retained-mode core: `Widget` interface, WPF-style two-pass Measure/Arrange with margin/alignment/invalidation, reactive `Property[T]`, and the first widgets (Border, TextBlock, Fixed, StackPanel, Grid, DockPanel, WrapPanel, Canvas) — all headless-testable, plus one GL integration golden and a gallery app.

**Architecture:** Package `core` holds the engine: a `Widget` interface whose common state lives in an embeddable `Element` struct (the Go substitute for WPF's base-class template methods — engine free functions `MeasureWidget`/`ArrangeWidget`/`RenderWidget` orchestrate, concrete widgets implement only `MeasureContent`/`ArrangeContent`/`Render`). Package `controls` holds the widgets. All coordinates are ABSOLUTE window-space logical px in Phase 2 (no transforms; ScrollViewer revisits this in Phase 3).

**Tech Stack:** Pure Go (stdlib only) for `core` and `controls` — no GL imports. Existing `render` (interface/primitives) and `text` (Face) packages. GL only in the one integration golden + gallery.

## Global Constraints

- Module `github.com/0xdreadnaught/fluo`; `go 1.23`; go.mod is PINNED — no new deps, no version changes. Build/test ONLY in WSL: `wsl -e bash -lc 'cd /mnt/c/Users/dread/source/fluo && <cmd>'`. GOTOOLCHAIN=local is set — if the toolchain complains, STOP and report, never upgrade.
- `core` and `controls` must NOT import any GL/glfw package. `core` imports only `render` (+ stdlib); `controls` imports `core`, `render`, `text` (+ stdlib).
- All public API coordinates: logical px, y-down, origin top-left. Phase 2 layout bounds are absolute window-space.
- `go vet ./...` must pass → keyed struct literals for all cross-package composites.
- Doc comments on every exported identifier (published library).
- Commit per task with the task's message + trailer `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`.
- Auto-size sentinel: `core.Auto = float32(-1)`; any negative width/height means auto. NaN is never used.
- Zero-value usability: `Element`'s zero value is visible, auto-sized, Stretch-aligned, zero margin, dirty (needs measure+arrange).
- Infinity (`float32(math.Inf(1))`) is a legal Measure `available` value; every `MeasureContent` must return a FINITE desired size.

## File Structure

```
core/
├── property.go        Property[T] reactive value
├── property_test.go
├── element.go         Element (embeddable base state) + Alignment + Auto
├── widget.go          Widget interface + engine: MeasureWidget/ArrangeWidget/RenderWidget
├── engine_test.go     engine semantics tests (margin/alignment/min-max/auto/visibility/invalidation)
controls/
├── border.go          Border (background/border/radius/padding, one child)
├── textblock.go       TextBlock (single-line, Face-based)
├── fixed.go           Fixed (fixed-size colored block; spacer + test workhorse)
├── stackpanel.go      StackPanel (H/V) + Orientation
├── grid.go            Grid (Px/Auto/Star tracks, no spans in v0)
├── dockpanel.go       DockPanel (+Dock enum)
├── wrappanel.go       WrapPanel (horizontal flow, v0)
├── canvas.go          Canvas (absolute x,y)
└── *_test.go          headless geometry tests per widget
render/gl/renderer_test.go   + TestLayoutRender integration golden
cmd/fluo-gallery/main.go     gallery skeleton
```

Deliberate v0 cuts (documented here so reviewers don't flag them as missing): Grid has NO row/col spans and does NOT re-measure children after track resolution (single Inf-measure algorithm, documented per-task); WrapPanel is horizontal-only; TextBlock is single-line (no wrapping). Each gets added when a consumer needs it.

---

### Task 1: Property[T]

**Files:**
- Create: `core/property.go`
- Test: `core/property_test.go`

**Interfaces:**
- Consumes: nothing (pure stdlib).
- Produces:

```go
package core

// Property[T] is a reactive value: Set notifies subscribers on real changes.
type Property[T comparable] struct { /* value T; subs map[int]func(old, new T); nextID int */ }
func (p *Property[T]) Get() T
func (p *Property[T]) Set(v T)                                  // no-op if v == current
func (p *Property[T]) OnChange(f func(old, new T)) (cancel func())
```

- [ ] **Step 1: failing tests** — `core/property_test.go`:

```go
package core

import "testing"

func TestPropertySetGet(t *testing.T) {
	var p Property[int]
	if p.Get() != 0 { t.Fatalf("zero value: got %d", p.Get()) }
	p.Set(5)
	if p.Get() != 5 { t.Fatalf("got %d", p.Get()) }
}

func TestPropertyNotify(t *testing.T) {
	var p Property[string]
	var gotOld, gotNew string
	calls := 0
	p.OnChange(func(o, n string) { gotOld, gotNew = o, n; calls++ })
	p.Set("a")
	if calls != 1 || gotOld != "" || gotNew != "a" { t.Fatalf("calls=%d old=%q new=%q", calls, gotOld, gotNew) }
	p.Set("a") // same value: no notify
	if calls != 1 { t.Fatalf("no-op Set notified: calls=%d", calls) }
}

func TestPropertyCancel(t *testing.T) {
	var p Property[int]
	calls := 0
	cancel := p.OnChange(func(_, _ int) { calls++ })
	p.Set(1)
	cancel()
	p.Set(2)
	if calls != 1 { t.Fatalf("calls=%d after cancel", calls) }
	// cancel is idempotent
	cancel()
}

func TestPropertyMultipleSubs(t *testing.T) {
	var p Property[int]
	a, b := 0, 0
	p.OnChange(func(_, _ int) { a++ })
	p.OnChange(func(_, _ int) { b++ })
	p.Set(7)
	if a != 1 || b != 1 { t.Fatalf("a=%d b=%d", a, b) }
}
```

- [ ] **Step 2: run** `go test ./core/` — FAIL (undefined Property).
- [ ] **Step 3: implement** — map-of-id subscribers; `Set` captures old, assigns, then calls each sub as `f(old, v)`; `OnChange` lazily inits the map, returns a closure deleting its id (idempotent because delete of missing key is a no-op). Not goroutine-safe (single-threaded UI loop — say so in the doc comment).
- [ ] **Step 4: run** `go test ./core/` — PASS; `go vet ./...` clean.
- [ ] **Step 5: commit** `feat(core): reactive Property[T]`

---

### Task 2: Element, Widget interface, layout engine

**Files:**
- Create: `core/element.go`, `core/widget.go`
- Test: `core/engine_test.go`

**Interfaces:**
- Consumes: `render` primitives (Size, Rect, Thickness, Renderer).
- Produces (THE contract for every widget and later phase — exact):

```go
package core

const Auto float32 = -1  // negative Width/Height = size-to-content

type Alignment uint8
const (
	Stretch Alignment = iota
	Start
	Center
	End
)

// Element is the embeddable base of every widget. Its ZERO VALUE is a valid
// widget state: visible, auto-sized, Stretch-aligned, dirty.
type Element struct { /* unexported: margin Thickness; width, height float32 (=Auto via ctor? see note); minW, minH, maxW, maxH float32; halign, valign Alignment; hidden bool; desired Size; bounds Rect; needsMeasure, needsArrange bool; parent Widget */ }
// NOTE on zero value: width/height zero-values must MEAN auto. Since Auto=-1
// but zero value is 0, Element interprets "explicit size" as  > 0, and
// SetWidth(Auto) stores -1; both 0 and -1 therefore mean auto. maxW/maxH
// zero-value means "no max" (treated as +Inf). needsMeasure/needsArrange are
// inverted flags internally (measuredOnce bools) so the zero value is dirty.

func (e *Element) SetMargin(t render.Thickness)
func (e *Element) SetWidth(w float32)   // >0 fixes the axis; <=0 = auto
func (e *Element) SetHeight(h float32)
func (e *Element) SetMinSize(w, h float32)
func (e *Element) SetMaxSize(w, h float32) // <=0 = unbounded
func (e *Element) SetAlign(h, v Alignment)
func (e *Element) SetVisible(v bool)    // false also invalidates parent measure
func (e *Element) Visible() bool
func (e *Element) DesiredSize() render.Size // valid after MeasureWidget
func (e *Element) Bounds() render.Rect      // valid after ArrangeWidget (absolute)
func (e *Element) InvalidateMeasure()       // marks self + ancestors measure-dirty
func (e *Element) InvalidateArrange()       // marks self arrange-dirty (+ ancestors arrange-dirty)
func (e *Element) NeedsLayout() bool        // measure-dirty || arrange-dirty

// Widget is implemented by embedding Element (which supplies element()) and
// defining content behavior. External packages CAN implement it: embedding
// core.Element promotes the unexported element() method.
type Widget interface {
	element() *Element
	MeasureContent(available render.Size) render.Size // content's desired size (finite!), Inf-safe
	ArrangeContent(bounds render.Rect)                // position children within bounds
	Render(r render.Renderer)                         // draw self only (children drawn by RenderWidget)
	Children() []Widget                               // nil for leaves
}
// Element provides default MeasureContent (Size{}), ArrangeContent (no-op),
// Render (no-op), Children (nil) — a bare embed is a valid empty widget.

func SetParent(child, parent Widget) // child.element().parent = parent; panels call this in Add

// Engine — the ONLY way layout runs. Parents call these on children inside
// their own MeasureContent/ArrangeContent.
func MeasureWidget(w Widget, available render.Size)
func ArrangeWidget(w Widget, final render.Rect)
func RenderWidget(w Widget, r render.Renderer) // skips hidden; w.Render then children in order
```

**Engine semantics (normative — the tests encode exactly this):**

MeasureWidget(w, available):
1. hidden → desired = {0,0}, done.
2. inner = available minus margins (clamped ≥0 per axis; Inf−x = Inf).
3. Per axis with explicit size (>0): inner axis = that size.
4. inner clamped to [min, max] (max≤0 ⇒ +Inf).
5. content = w.MeasureContent(inner).
6. Per axis with explicit size: content axis = explicit size. Then clamp content to [min, max].
7. desired = content + margins. Clear measure-dirty.

ArrangeWidget(w, final):
1. hidden → bounds = {}, done.
2. slot = final inset by margins (W/H clamped ≥0).
3. contentDesired per axis = max(0, desired − margins), capped at slot.
4. Per axis: if alignment == Stretch AND no explicit size (>0): size = slot extent, pos = slot origin. Else size = contentDesired; pos by alignment — Start→slot origin; Center (and Stretch-with-explicit-size)→origin+(slot−size)/2; End→origin+slot−size.
5. bounds = computed rect (absolute). Call w.ArrangeContent(bounds). Clear arrange-dirty.

Invalidation: InvalidateMeasure sets measure+arrange dirty on self, then walks parents setting both until hitting an already-measure-dirty ancestor (or root). InvalidateArrange same but arrange-only.

- [ ] **Step 1: failing tests** — `core/engine_test.go` (package core; uses an internal stub):

```go
package core

import (
	"math"
	"testing"

	"github.com/0xdreadnaught/fluo/render"
)

// stub: fixed content size, records last arranged bounds.
type stub struct {
	Element
	contentW, contentH float32
	lastContent        render.Rect
}

func (s *stub) MeasureContent(render.Size) render.Size {
	return render.Size{W: s.contentW, H: s.contentH}
}
func (s *stub) ArrangeContent(b render.Rect) { s.lastContent = b }

func TestMeasureMarginAndDesired(t *testing.T) {
	s := &stub{contentW: 50, contentH: 20}
	s.SetMargin(render.Thickness{Left: 5, Top: 10, Right: 15, Bottom: 20})
	MeasureWidget(s, render.Size{W: 200, H: 200})
	if got := s.DesiredSize(); got != (render.Size{W: 70, H: 50}) {
		t.Fatalf("desired=%v", got)
	}
}

func TestMeasureExplicitSizeWins(t *testing.T) {
	s := &stub{contentW: 50, contentH: 20}
	s.SetWidth(80)
	MeasureWidget(s, render.Size{W: 200, H: 200})
	if got := s.DesiredSize(); got != (render.Size{W: 80, H: 20}) {
		t.Fatalf("desired=%v", got)
	}
}

func TestMeasureClampMinMax(t *testing.T) {
	s := &stub{contentW: 50, contentH: 20}
	s.SetMinSize(60, 0)
	s.SetMaxSize(0, 10) // W unbounded, H capped
	MeasureWidget(s, render.Size{W: 200, H: 200})
	if got := s.DesiredSize(); got != (render.Size{W: 60, H: 10}) {
		t.Fatalf("desired=%v", got)
	}
}

func TestMeasureInfAvailable(t *testing.T) {
	inf := float32(math.Inf(1))
	s := &stub{contentW: 50, contentH: 20}
	s.SetMargin(render.Uniform(4))
	MeasureWidget(s, render.Size{W: inf, H: inf})
	if got := s.DesiredSize(); got != (render.Size{W: 58, H: 28}) {
		t.Fatalf("desired=%v", got)
	}
}

func TestArrangeStretchFillsSlot(t *testing.T) {
	s := &stub{contentW: 50, contentH: 20}
	s.SetMargin(render.Uniform(10))
	MeasureWidget(s, render.Size{W: 200, H: 100})
	ArrangeWidget(s, render.Rect{X: 0, Y: 0, W: 200, H: 100})
	if got := s.Bounds(); got != (render.Rect{X: 10, Y: 10, W: 180, H: 80}) {
		t.Fatalf("bounds=%v", got)
	}
	if s.lastContent != s.Bounds() { t.Fatalf("ArrangeContent got %v", s.lastContent) }
}

func TestArrangeAlignments(t *testing.T) {
	for _, tc := range []struct {
		h, v Alignment
		want render.Rect
	}{
		{Start, Start, render.Rect{X: 0, Y: 0, W: 50, H: 20}},
		{Center, Center, render.Rect{X: 75, Y: 40, W: 50, H: 20}},
		{End, End, render.Rect{X: 150, Y: 80, W: 50, H: 20}},
	} {
		s := &stub{contentW: 50, contentH: 20}
		s.SetAlign(tc.h, tc.v)
		MeasureWidget(s, render.Size{W: 200, H: 100})
		ArrangeWidget(s, render.Rect{X: 0, Y: 0, W: 200, H: 100})
		if got := s.Bounds(); got != tc.want {
			t.Errorf("align %v/%v: bounds=%v want %v", tc.h, tc.v, got, tc.want)
		}
	}
}

func TestArrangeExplicitSizeCentersUnderStretch(t *testing.T) {
	s := &stub{contentW: 50, contentH: 20}
	s.SetWidth(60) // halign stays Stretch
	MeasureWidget(s, render.Size{W: 200, H: 100})
	ArrangeWidget(s, render.Rect{X: 0, Y: 0, W: 200, H: 100})
	b := s.Bounds()
	if b.W != 60 || b.X != 70 { t.Fatalf("bounds=%v (want W=60 X=70)", b) }
	if b.H != 100 || b.Y != 0 { t.Fatalf("bounds=%v (H should stretch)", b) }
}

func TestHiddenMeasuresZeroAndSkipsRender(t *testing.T) {
	s := &stub{contentW: 50, contentH: 20}
	s.SetVisible(false)
	MeasureWidget(s, render.Size{W: 200, H: 100})
	if got := s.DesiredSize(); got != (render.Size{}) { t.Fatalf("desired=%v", got) }
}

func TestInvalidationBubbles(t *testing.T) {
	parent := &stub{contentW: 100, contentH: 100}
	child := &stub{contentW: 10, contentH: 10}
	SetParent(child, parent)
	MeasureWidget(parent, render.Size{W: 200, H: 200})
	MeasureWidget(child, render.Size{W: 200, H: 200})
	ArrangeWidget(parent, render.Rect{X: 0, Y: 0, W: 200, H: 200})
	ArrangeWidget(child, render.Rect{X: 0, Y: 0, W: 10, H: 10})
	if parent.NeedsLayout() || child.NeedsLayout() { t.Fatal("should be clean after layout") }
	child.InvalidateMeasure()
	if !child.NeedsLayout() || !parent.NeedsLayout() { t.Fatal("invalidation must bubble to parent") }
}
```

- [ ] **Step 2: run** `go test ./core/` — FAIL (undefined symbols).
- [ ] **Step 3: implement** `element.go` + `widget.go` exactly per the normative semantics above. Helper functions (unexported): `insetSize(s Size, m Thickness) Size`, `insetRect(r Rect, m Thickness) Rect`, `clampAxis(v, min, max float32) float32` (max≤0 ⇒ +Inf), all Inf-safe (`inf - x` stays inf naturally in float math — no special casing needed; clamp of Inf against a finite max yields the max).
- [ ] **Step 4: run** `go test ./core/` — PASS; `go vet ./...` clean.
- [ ] **Step 5: commit** `feat(core): Element, Widget interface, Measure/Arrange engine`

---

### Task 3: Border, TextBlock, Fixed

**Files:**
- Create: `controls/border.go`, `controls/textblock.go`, `controls/fixed.go`
- Test: `controls/border_test.go`, `controls/textblock_test.go`

**Interfaces:**
- Consumes: `core` engine (Task 2), `render`, `text.Face` (Phase 1: `NewFace(f,sizePx)`, `Measure(s) Size`, `LineHeight()`, `Draw(r, at, s, c)`), `goregular.TTF` in tests.
- Produces:

```go
package controls

// Fixed: a solid-color fixed-size block (spacer / color swatch / test widget).
func NewFixed(w, h float32, c render.Color) *Fixed
type Fixed struct{ core.Element /* w, h float32; color render.Color */ }
// MeasureContent returns {w,h}; Render fills Bounds() with color (skip if A==0).

// Border: single-child decorator with background, stroke, radius, padding.
func NewBorder() *Border
type Border struct{ core.Element /* child core.Widget; background, borderColor render.Color; borderWidth, radius float32; padding render.Thickness */ }
func (b *Border) SetChild(w core.Widget) *Border      // also core.SetParent(w, b)
func (b *Border) SetBackground(c render.Color) *Border
func (b *Border) SetBorder(c render.Color, width float32) *Border
func (b *Border) SetRadius(r float32) *Border
func (b *Border) SetPadding(t render.Thickness) *Border
// MeasureContent: child measured with available minus padding+borderWidth on
// all sides; desired = child desired + those. No child: padding+border only.
// ArrangeContent: child arranged in bounds inset by padding+borderWidth.
// Render: FillRoundedRect(bounds, radius, background) if bg.A>0;
//         StrokeRoundedRect(bounds, radius, borderWidth, borderColor) if width>0 && color.A>0.
// Children: [child] or nil.

// TextBlock: single-line text leaf.
func NewTextBlock(face *text.Face, s string) *TextBlock
type TextBlock struct{ core.Element /* face *text.Face; text Property[string]; color render.Color */ }
func (t *TextBlock) SetText(s string) *TextBlock   // via Property; change → InvalidateMeasure
func (t *TextBlock) Text() string
func (t *TextBlock) SetColor(c render.Color) *TextBlock
// MeasureContent: face.Measure(text) (nil face → zero). Render: face.Draw at Bounds() top-left.
```

- [ ] **Step 1: failing tests**

`controls/border_test.go`:

```go
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
	if got := b.DesiredSize(); got != (render.Size{W: 16, H: 16}) { t.Fatalf("desired=%v", got) }
	if b.Children() != nil { t.Fatal("no-child Border must return nil Children") }
}
```

`controls/textblock_test.go`:

```go
package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"

	"golang.org/x/image/font/gofont/goregular"
)

func TestTextBlockMeasuresText(t *testing.T) {
	f, err := text.Load(goregular.TTF)
	if err != nil { t.Fatal(err) }
	face := text.NewFace(f, 14)
	tb := NewTextBlock(face, "Hello")
	core.MeasureWidget(tb, render.Size{W: 500, H: 500})
	want := face.Measure("Hello")
	if got := tb.DesiredSize(); got != want { t.Fatalf("desired=%v want %v", got, want) }
}

func TestTextBlockSetTextInvalidates(t *testing.T) {
	f, _ := text.Load(goregular.TTF)
	face := text.NewFace(f, 14)
	tb := NewTextBlock(face, "a")
	core.MeasureWidget(tb, render.Size{W: 500, H: 500})
	if tb.NeedsLayout() { t.Fatal("clean after measure+... (arrange not run; NeedsLayout covers arrange too)") }
	_ = tb // see note below
	tb.SetText("wider text")
	if !tb.NeedsLayout() { t.Fatal("SetText must invalidate measure") }
	if tb.Text() != "wider text" { t.Fatalf("Text()=%q", tb.Text()) }
	tb.SetText("wider text") // no-op set must not panic or re-invalidate spuriously
}

func TestTextBlockNilFace(t *testing.T) {
	tb := NewTextBlock(nil, "x")
	core.MeasureWidget(tb, render.Size{W: 100, H: 100})
	if got := tb.DesiredSize(); got != (render.Size{}) { t.Fatalf("desired=%v", got) }
}
```

NOTE for the first assertion in `TestTextBlockSetTextInvalidates`: after only Measure, arrange is still dirty, so `NeedsLayout()` is TRUE. Fix the test to run Arrange first:

```go
	core.ArrangeWidget(tb, render.Rect{X: 0, Y: 0, W: 200, H: 30})
	if tb.NeedsLayout() { t.Fatal("clean after measure+arrange") }
```

(Use this corrected form — measure, then arrange, then assert clean.)

- [ ] **Step 2: run** `go test ./controls/` — FAIL.
- [ ] **Step 3: implement** the three widgets per the Produces block. TextBlock wires `text Property[string]` → `OnChange(func(_, _ string) { t.InvalidateMeasure() })` in the constructor. Render uses `render.Point{X: b.X, Y: b.Y}` from Bounds.
- [ ] **Step 4: run** `go test ./...` (WSL) — PASS incl. GL packages; vet clean.
- [ ] **Step 5: commit** `feat(controls): Border, TextBlock, Fixed`

---

### Task 4: StackPanel

**Files:**
- Create: `controls/stackpanel.go`
- Test: `controls/stackpanel_test.go`

**Interfaces:**
- Consumes: Tasks 2-3.
- Produces:

```go
package controls

type Orientation uint8
const (
	Vertical Orientation = iota
	Horizontal
)

func NewStackPanel(o Orientation) *StackPanel
type StackPanel struct{ core.Element /* orient Orientation; children []core.Widget; gap float32 */ }
func (s *StackPanel) Add(children ...core.Widget) *StackPanel // SetParent + InvalidateMeasure
func (s *StackPanel) SetGap(g float32) *StackPanel            // spacing between children
func (s *StackPanel) Children() []core.Widget
// MeasureContent (Vertical): child available = (available.W, +Inf);
//   desired = (max child W, sum child H + gaps). Horizontal mirrors axes.
// ArrangeContent (Vertical): y cursor from bounds.Y; each child gets slot
//   {bounds.X, y, bounds.W, childDesired.H}; y += childDesired.H + gap.
//   (Cross-axis stretch/alignment is handled by ArrangeWidget per child.)
```

- [ ] **Step 1: failing tests** — `controls/stackpanel_test.go`:

```go
package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
)

func vstack(gap float32, kids ...core.Widget) *StackPanel {
	return NewStackPanel(Vertical).SetGap(gap).Add(kids...)
}

func TestVStackMeasure(t *testing.T) {
	sp := vstack(0,
		NewFixed(50, 20, render.RGB(1, 2, 3)),
		NewFixed(80, 30, render.RGB(1, 2, 3)),
	)
	core.MeasureWidget(sp, render.Size{W: 200, H: 200})
	if got := sp.DesiredSize(); got != (render.Size{W: 80, H: 50}) { t.Fatalf("desired=%v", got) }
}

func TestVStackGap(t *testing.T) {
	sp := vstack(6,
		NewFixed(10, 10, render.RGB(1, 2, 3)),
		NewFixed(10, 10, render.RGB(1, 2, 3)),
		NewFixed(10, 10, render.RGB(1, 2, 3)),
	)
	core.MeasureWidget(sp, render.Size{W: 200, H: 200})
	if got := sp.DesiredSize().H; got != 42 { t.Fatalf("H=%v (3*10+2*6)", got) } // 42
}

func TestVStackArrangePositionsAndStretch(t *testing.T) {
	a := NewFixed(50, 20, render.RGB(1, 2, 3)) // Fixed content 50 wide, but panel slot is 120: Stretch default
	b := NewFixed(80, 30, render.RGB(1, 2, 3))
	b.SetAlign(core.Start, core.Start)
	sp := vstack(0, a, b)
	core.MeasureWidget(sp, render.Size{W: 120, H: 300})
	core.ArrangeWidget(sp, render.Rect{X: 10, Y: 10, W: 120, H: 300})
	if got := a.Bounds(); got != (render.Rect{X: 10, Y: 10, W: 120, H: 20}) {
		t.Fatalf("a=%v (stretch cross-axis)", got)
	}
	if got := b.Bounds(); got != (render.Rect{X: 10, Y: 30, W: 80, H: 30}) {
		t.Fatalf("b=%v (start-aligned)", got)
	}
}

func TestHStackMeasureAndArrange(t *testing.T) {
	sp := NewStackPanel(Horizontal).Add(
		NewFixed(50, 20, render.RGB(1, 2, 3)),
		NewFixed(30, 40, render.RGB(1, 2, 3)),
	)
	core.MeasureWidget(sp, render.Size{W: 200, H: 200})
	if got := sp.DesiredSize(); got != (render.Size{W: 80, H: 40}) { t.Fatalf("desired=%v", got) }
	core.ArrangeWidget(sp, render.Rect{X: 0, Y: 0, W: 200, H: 60})
	kids := sp.Children()
	if got := kids[1].(*Fixed).Bounds().X; got != 50 { t.Fatalf("second child X=%v", got) }
}

func TestStackSkipsHiddenChildren(t *testing.T) {
	hid := NewFixed(50, 50, render.RGB(1, 2, 3))
	hid.SetVisible(false)
	sp := vstack(10, NewFixed(10, 10, render.RGB(1, 2, 3)), hid, NewFixed(10, 10, render.RGB(1, 2, 3)))
	core.MeasureWidget(sp, render.Size{W: 200, H: 200})
	// hidden child contributes 0 size AND no gap: 10+10 + one 10 gap = 30
	if got := sp.DesiredSize().H; got != 30 { t.Fatalf("H=%v", got) }
}
```

- [ ] **Step 2: run** — FAIL.
- [ ] **Step 3: implement.** Hidden children: still `MeasureWidget` them (cheap, keeps them clean) but contribute no extent and no gap. Gap counts only between two consecutive VISIBLE children.
- [ ] **Step 4: run** `go test ./controls/ ./core/` — PASS; vet clean.
- [ ] **Step 5: commit** `feat(controls): StackPanel`

---

### Task 5: Grid

**Files:**
- Create: `controls/grid.go`
- Test: `controls/grid_test.go`

**Interfaces:**
- Consumes: Tasks 2-3.
- Produces:

```go
package controls

// Track defines one row or column.
type Track struct { /* kind: px|auto|star; value float32 */ }
func Px(v float32) Track
func AutoTrack() Track
func Star(weight float32) Track

func NewGrid() *Grid
type Grid struct{ core.Element /* rows, cols []Track; cells []gridCell{child core.Widget; row, col int} */ }
func (g *Grid) Rows(tracks ...Track) *Grid
func (g *Grid) Cols(tracks ...Track) *Grid
func (g *Grid) Add(w core.Widget, row, col int) *Grid // SetParent; out-of-range row/col panics with a clear message
func (g *Grid) Children() []core.Widget
```

**Sizing algorithm (v0, normative — simpler than WPF's, documented in the file's doc comment):** No spans. Children measured ONCE with per-axis available = Px value for Px tracks, +Inf for Auto and Star tracks. Track resolution per axis given total `avail`:
1. Px track = its value.
2. Auto track = max desired (of that axis) over children whose cell is that track; 0 if none.
3. Remaining = max(0, avail − ΣPx − ΣAuto); each Star gets remaining × weight/Σweights. If avail is Inf (grid measured unconstrained), a Star track sizes like Auto (max child desired).
4. Desired size = Σ resolved tracks (with Star-as-Auto when unconstrained; when constrained, desired = min(avail, Σ) — i.e. report Σ of the constrained resolution).
Known v0 limitation (do NOT "fix"): children are not re-measured against final track sizes, so a child in a Star track keeps its Inf-measured desired; ArrangeWidget's stretch/alignment absorbs the difference. Grids of grids therefore resolve tracks again in ArrangeContent from actual bounds — resolution runs in BOTH MeasureContent (cached for desired) and ArrangeContent (from real extent).

- [ ] **Step 1: failing tests** — `controls/grid_test.go`:

```go
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
	if got := a.Bounds().W; got != 100 { t.Fatalf("star1 W=%v", got) }
	if got := b.Bounds(); got.X != 100 || got.W != 300 { t.Fatalf("star3=%v", got) }
}

func TestGridDesiredUnconstrained(t *testing.T) {
	g := NewGrid().Cols(Px(50), AutoTrack()).Rows(AutoTrack())
	g.Add(NewFixed(40, 25, render.RGB(1, 2, 3)), 0, 1)
	core.MeasureWidget(g, render.Size{W: 500, H: 500})
	if got := g.DesiredSize(); got != (render.Size{W: 90, H: 25}) { t.Fatalf("desired=%v", got) }
}

func TestGridAddOutOfRangePanics(t *testing.T) {
	defer func() {
		if recover() == nil { t.Fatal("expected panic") }
	}()
	NewGrid().Cols(Px(10)).Rows(Px(10)).Add(NewFixed(1, 1, render.Color{}), 0, 5)
}
```

- [ ] **Step 2: run** — FAIL.
- [ ] **Step 3: implement** per the normative algorithm. One unexported `resolveTracks(tracks []Track, cells, axis, avail float32) []float32` shared by measure and arrange; measure caches nothing beyond desired (arrange re-resolves). Default when `Rows`/`Cols` never called: a single `Star(1)` track.
- [ ] **Step 4: run** — PASS; vet clean.
- [ ] **Step 5: commit** `feat(controls): Grid with Px/Auto/Star tracks`

---

### Task 6: DockPanel + WrapPanel

**Files:**
- Create: `controls/dockpanel.go`, `controls/wrappanel.go`
- Test: `controls/dockpanel_test.go`, `controls/wrappanel_test.go`

**Interfaces:**
- Consumes: Tasks 2-3.
- Produces:

```go
package controls

type Dock uint8
const (
	DockLeft Dock = iota
	DockTop
	DockRight
	DockBottom
)

func NewDockPanel() *DockPanel
type DockPanel struct{ core.Element /* items []dockItem{child core.Widget; dock Dock}; lastFills bool = true */ }
func (d *DockPanel) Add(w core.Widget, dock Dock) *DockPanel
func (d *DockPanel) SetLastChildFill(v bool) *DockPanel
func (d *DockPanel) Children() []core.Widget
// Measure: walk items; child available = remaining (available − consumed edges,
//   clamped ≥0); consumed grows by child desired on the docked axis; desired =
//   WPF accumulation: for Left/Right track maxH = max(maxH, usedH+d.H), usedW += d.W;
//   Top/Bottom mirrored; final desired = (max(maxW, usedW), max(maxH, usedH)).
// Arrange: carve from remaining rect per dock; last child gets the whole
//   remainder when lastFills (its dock value ignored).

func NewWrapPanel() *WrapPanel // horizontal flow, wraps to new rows
type WrapPanel struct{ core.Element /* children []core.Widget; gap float32 */ }
func (w *WrapPanel) Add(children ...core.Widget) *WrapPanel
func (w *WrapPanel) SetGap(g float32) *WrapPanel // both between items and rows
func (w *WrapPanel) Children() []core.Widget
// Measure: children measured with (available.W, +Inf); flow left→right, wrap
//   when cursor+childW > available.W (first item of a row never wraps);
//   desired = (max row width, Σ row heights + gaps). Row height = max child H in row.
// Arrange: same flow against bounds.W; child slot = its desired size at the
//   flow position (row-height slots so per-child valign works within the row).
```

- [ ] **Step 1: failing tests**

`controls/dockpanel_test.go`:

```go
package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
)

func TestDockLayout(t *testing.T) {
	top := NewFixed(0, 30, render.RGB(1, 2, 3))    // full-width bar (W stretches)
	left := NewFixed(80, 0, render.RGB(1, 2, 3))   // sidebar
	fill := NewFixed(10, 10, render.RGB(1, 2, 3))  // content
	d := NewDockPanel().Add(top, DockTop).Add(left, DockLeft).Add(fill, DockLeft)
	core.MeasureWidget(d, render.Size{W: 400, H: 300})
	core.ArrangeWidget(d, render.Rect{X: 0, Y: 0, W: 400, H: 300})
	if got := top.Bounds(); got != (render.Rect{X: 0, Y: 0, W: 400, H: 30}) { t.Fatalf("top=%v", got) }
	if got := left.Bounds(); got != (render.Rect{X: 0, Y: 30, W: 80, H: 270}) { t.Fatalf("left=%v", got) }
	if got := fill.Bounds(); got != (render.Rect{X: 80, Y: 30, W: 320, H: 270}) { t.Fatalf("fill=%v", got) }
}

func TestDockNoFill(t *testing.T) {
	a := NewFixed(50, 50, render.RGB(1, 2, 3))
	a.SetAlign(core.Start, core.Start)
	d := NewDockPanel().SetLastChildFill(false).Add(a, DockLeft)
	core.MeasureWidget(d, render.Size{W: 400, H: 300})
	core.ArrangeWidget(d, render.Rect{X: 0, Y: 0, W: 400, H: 300})
	if got := a.Bounds(); got != (render.Rect{X: 0, Y: 0, W: 50, H: 50}) { t.Fatalf("a=%v", got) }
}

func TestDockDesired(t *testing.T) {
	d := NewDockPanel().SetLastChildFill(false).
		Add(NewFixed(80, 20, render.RGB(1, 2, 3)), DockLeft).
		Add(NewFixed(60, 40, render.RGB(1, 2, 3)), DockTop)
	core.MeasureWidget(d, render.Size{W: 400, H: 300})
	if got := d.DesiredSize(); got != (render.Size{W: 140, H: 60}) { t.Fatalf("desired=%v", got) }
}
```

`controls/wrappanel_test.go`:

```go
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
```

- [ ] **Step 2: run** — FAIL.
- [ ] **Step 3: implement** per the Produces comments. Hidden children: skip entirely (no extent, no gap), same convention as StackPanel.
- [ ] **Step 4: run** — PASS; vet clean.
- [ ] **Step 5: commit** `feat(controls): DockPanel and WrapPanel`

---

### Task 7: Canvas + GL integration golden

**Files:**
- Create: `controls/canvas.go`
- Test: `controls/canvas_test.go`; add `TestLayoutRender` to `render/gl/renderer_test.go` (+ golden `layout.png`)

**Interfaces:**
- Consumes: everything above; `gltest` harness (Phase 1).
- Produces:

```go
package controls

func NewCanvas() *Canvas
type Canvas struct{ core.Element /* items []canvasItem{child core.Widget; x, y float32} */ }
func (c *Canvas) Add(w core.Widget, x, y float32) *Canvas
func (c *Canvas) Children() []core.Widget
// Measure: children measured with (+Inf, +Inf); Canvas desires {0,0} (WPF semantics).
// Arrange: each child at {bounds.X+x, bounds.Y+y, childDesired−margins}.
```

- [ ] **Step 1: failing canvas test** — `controls/canvas_test.go`:

```go
package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
)

func TestCanvasAbsolute(t *testing.T) {
	a := NewFixed(30, 30, render.RGB(1, 2, 3))
	c := NewCanvas().Add(a, 15, 25)
	core.MeasureWidget(c, render.Size{W: 200, H: 200})
	if got := c.DesiredSize(); got != (render.Size{}) { t.Fatalf("canvas desires 0, got %v", got) }
	core.ArrangeWidget(c, render.Rect{X: 100, Y: 100, W: 200, H: 200})
	if got := a.Bounds(); got != (render.Rect{X: 115, Y: 125, W: 30, H: 30}) { t.Fatalf("a=%v", got) }
}
```

- [ ] **Step 2:** run → FAIL; implement canvas.go; run → PASS.
- [ ] **Step 3: GL integration golden** — append to `render/gl/renderer_test.go`:

```go
func TestLayoutRender(t *testing.T) {
	testFrame(t, "layout", 220, 150, func(r *glr.Renderer) {
		f, err := text.Load(goregular.TTF)
		if err != nil { t.Fatal(err) }
		face := text.NewFace(f, 14)
		root := controls.NewBorder().
			SetBackground(render.RGB(32, 32, 36)).
			SetRadius(8).
			SetPadding(render.Uniform(12)).
			SetChild(controls.NewStackPanel(controls.Vertical).SetGap(8).Add(
				controls.NewTextBlock(face, "fluo layout").SetColor(render.RGB(255, 255, 255)),
				controls.NewFixed(0, 24, render.RGB(0, 120, 215)), // stretches full width
				func() core.Widget {
					y := controls.NewFixed(60, 18, render.RGB(255, 185, 0))
					y.SetAlign(core.End, core.Start)
					return y
				}(),
			))
		core.MeasureWidget(root, render.Size{W: 220, H: 150})
		core.ArrangeWidget(root, render.Rect{X: 10, Y: 10, W: 200, H: 130})
		core.RenderWidget(root, r)
	})
}
```

`-update`, then READ `render/gl/testdata/layout.png` and verify: dark rounded card at (10,10) 200x130; inside it top-to-bottom with 12px padding — white "fluo layout" text, a blue bar spanning the padded width (24 tall), a yellow 60x18 block hugging the RIGHT padded edge. Then plain run PASSES.

- [ ] **Step 4: run** full `go test ./...` (WSL) — PASS; vet clean.
- [ ] **Step 5: commit** `feat(controls): Canvas + layout-engine GL golden`

---

### Task 8: Gallery skeleton + docs

**Files:**
- Create: `cmd/fluo-gallery/main.go`
- Modify: `README.md` (one line in the demo section), `ROADMAP.md` (tick delivered Phase 2 boxes)

**Interfaces:**
- Consumes: everything; `app.Run` (Phase 1).
- Produces: `go run ./cmd/fluo-gallery` — a resizable window laid out entirely by the engine: DockPanel root with a title bar (DockTop, Border+TextBlock), a nav sidebar (DockLeft, StackPanel of TextBlocks), and a content area (WrapPanel of colored Fixed swatches inside a padded Border). Rebuilds layout when the window size changes; renders via `core.RenderWidget` every frame.

- [ ] **Step 1: implement** `cmd/fluo-gallery/main.go`:

```go
// Command fluo-gallery is the widget gallery: it grows a page per control as
// phases land. Phase 2: pure layout — panels, borders, text.
package main

import (
	"log"

	"github.com/0xdreadnaught/fluo/app"
	"github.com/0xdreadnaught/fluo/controls"
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"

	"golang.org/x/image/font/gofont/goregular"
)

func main() {
	f, err := text.Load(goregular.TTF)
	if err != nil { log.Fatal(err) }
	title := text.NewFace(f, 20)
	body := text.NewFace(f, 13)

	swatches := controls.NewWrapPanel().SetGap(8)
	for _, c := range []render.Color{
		render.RGB(0, 120, 215), render.RGB(255, 185, 0), render.RGB(16, 124, 16),
		render.RGB(232, 17, 35), render.RGB(136, 23, 152), render.RGB(0, 153, 188),
	} {
		swatches.Add(controls.NewFixed(72, 48, c))
	}

	nav := controls.NewStackPanel(controls.Vertical).SetGap(4).Add(
		controls.NewTextBlock(body, "Layout").SetColor(render.RGB(255, 255, 255)),
		controls.NewTextBlock(body, "Panels").SetColor(render.RGBA(255, 255, 255, 140)),
		controls.NewTextBlock(body, "Text").SetColor(render.RGBA(255, 255, 255, 140)),
	)

	root := controls.NewDockPanel().
		Add(controls.NewBorder().
			SetBackground(render.RGB(24, 24, 28)).
			SetPadding(render.Thickness{Left: 16, Top: 12, Right: 16, Bottom: 12}).
			SetChild(controls.NewTextBlock(title, "fluo gallery").SetColor(render.RGB(255, 255, 255))),
			controls.DockTop).
		Add(controls.NewBorder().
			SetBackground(render.RGB(28, 28, 33)).
			SetPadding(render.Uniform(12)).
			SetChild(nav),
			controls.DockLeft).
		Add(controls.NewBorder().
			SetPadding(render.Uniform(16)).
			SetChild(swatches),
			controls.DockLeft) // last child fills

	var lastSize render.Size
	err = app.Run(app.Config{Title: "fluo gallery", Width: 640, Height: 420}, func(c *app.Ctx) {
		if c.Size != lastSize || root.NeedsLayout() {
			lastSize = c.Size
			core.MeasureWidget(root, c.Size)
			core.ArrangeWidget(root, render.Rect{X: 0, Y: 0, W: c.Size.W, H: c.Size.H})
		}
		core.RenderWidget(root, c.R)
	})
	if err != nil { log.Fatal(err) }
}
```

- [ ] **Step 2: build + vet + full test suite** (WSL) — all pass.
- [ ] **Step 3: visual verification** — launch bounded: `wsl -e bash -lc 'cd /mnt/c/Users/dread/source/fluo && timeout 60 go run ./cmd/fluo-gallery' &`, wait ~8s, capture window-bound: `python tools/winshot.py "fluo gallery" <scratchpad>\fluo-gallery.png`, READ it and verify: dark title bar across the top ("fluo gallery"), darker nav column at left (3 labels, first bright), content area right with 6 colored swatches wrapping. Then `wsl -e bash -lc "pkill -f fluo-gallery"` and confirm dead via `pgrep -f fluo-gallery || echo dead`. NEVER leave it running.
- [ ] **Step 4: docs** — README: add `go run ./cmd/fluo-gallery` beside the demo line. ROADMAP: tick every Phase 2 checkbox EXCEPT "Invalidation + reactive Property[T]"… correction: tick that too (Tasks 1-2 delivered it). Tick: Widget interface/base element, two-pass Measure→Arrange, invalidation+Property, first widgets, Grid/DockPanel/WrapPanel/Canvas, headless tests, gallery skeleton — i.e. ALL Phase 2 boxes.
- [ ] **Step 5: commit** `feat(gallery): layout gallery skeleton; complete Phase 2`

---

## Self-review notes (resolved inline)

- Spec coverage: every ROADMAP Phase 2 node has a task (base element→2, Measure/Arrange→2, invalidation+Property→1+2, Border/TextBlock/StackPanel→3+4, Grid/Dock/Wrap/Canvas→5+6+7, headless tests→every task, gallery→8). Fluent-API style honored as concrete-typed chaining setters (`Set*` returning the concrete pointer).
- Type consistency: `core.Widget`'s five methods used identically in Tasks 3-8; `NewFixed(w,h,color)` signature consistent across all test files; `SetGap`/`Add` chaining returns concrete types everywhere.
- The Task 3 TextBlock test includes its own inline correction (arrange-before-assert) — implementer uses the corrected form.
- Deliberate cuts (Grid spans/remeasure, WrapPanel vertical, TextBlock wrapping) are declared in File Structure so reviewers treat them as scope, not gaps.
