# fluo Phase 3: Input & Events Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Interactive fluo: hit-testing, bubbling pointer/key events, focus + tab navigation, pointer capture, cursor shapes, a frame-driven timer service, ScrollViewer, and the DPI coordinate-space gate — wired into the host and proven in the gallery.

**Architecture:** Package `timers` (pure Go, fake-clock testable). Package `input` (pure Go): event types, `HitPath` over the arranged tree via `core.BoundsOf`, and a `Router` owning hover/capture/focus state that dispatches to widgets through small OPT-IN interfaces (`PointerHandler`, `KeyHandler`, …) — `core.Widget` itself stays untouched. Widgets that clip or draw above their children get two new opt-in engine hooks in `core` (`ClipProvider`, `OverlayRenderer`). `app` pumps glfw callbacks into the Router and resolves the DPI gate by construction: logical space := GLFW window/screen coordinate space (cursor and size are then consistent on every platform; renderer scale = fbW/windowW).

**Tech Stack:** Existing pinned deps only. `timers`/`input` are GL-free; `app` and the ScrollViewer golden touch GL.

## Global Constraints

- Everything from Phases 1-2 still binds: module `github.com/0xdreadnaught/fluo`; go.mod PINNED (no new deps); WSL-only Go (`wsl -e bash -lc 'cd /mnt/c/Users/dread/source/fluo && <cmd>'`, GOTOOLCHAIN=local); keyed literals; vet + gofmt gates (CI enforces gofmt — run `/usr/local/go/bin/gofmt -w` on touched files before committing); doc comments on all exported identifiers; commit per task with trailer `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`.
- `timers` and `input` must NOT import GL/glfw. `input` imports `core`, `render`, stdlib. `controls` may import `input` (for ScrollViewer). `app` may import everything.
- Key/button/cursor constants are fluo-owned types — NEVER expose glfw types in any public API. `app` translates at the boundary.
- Event structs are passed as POINTERS through handler chains so `Handled` mutations propagate.
- Hit-testing and all event coordinates are in logical px (= GLFW window coordinate space per the DPI resolution above).
- The widget idiom from Phase 2 (embed `core.Element`, fluent setters, `core.SetParent`, accessors `DesiredSizeOf`/`BoundsOf`/`IsVisible`) is mandatory for ScrollViewer.

## File Structure

```
timers/
├── timers.go            Queue, Timer: After/Every/Stop/Advance/NextDue
└── timers_test.go
input/
├── events.go            Button/Action/Modifiers/Key/Cursor consts; PointerEvent/KeyEvent; handler interfaces
├── hittest.go           HitPath(root, p) []core.Widget
├── hittest_test.go
├── router.go            Router: hover/capture/focus state + dispatch + tab navigation
└── router_test.go
core/
├── widget.go            + ClipProvider, OverlayRenderer opt-in interfaces; RenderWidget honors them
└── engine_test.go       + tests for both hooks
controls/
├── scrollviewer.go      vertical-only v0 ScrollViewer
└── scrollviewer_test.go
app/window.go            router pump, cursor shapes, timers, DPI fix; Ctx gains Input/Timers
render/gl/renderer_test.go   + TestScrollClipRender golden (scroll.png)
cmd/fluo-gallery/main.go     interactive page: clickable swatches + scrolling list
```

Declared v0 cuts: ScrollViewer is vertical-only (no horizontal bar); no double-click/click-count synthesis; no key-repeat distinction (glfw repeat arrives as Press); IME remains a Phase-8 stretch item.

---

### Task 1: timers

**Files:** Create `timers/timers.go`; Test `timers/timers_test.go`

**Interfaces:**
- Consumes: stdlib `time` only.
- Produces:

```go
package timers

// Queue is a frame-driven timer service: the host calls Advance(now) once per
// frame; due timers fire on that call, in due-time order. Not goroutine-safe.
type Queue struct{ /* items []*Timer; now time.Time */ }
func NewQueue(start time.Time) *Queue
func (q *Queue) After(d time.Duration, fn func()) *Timer  // one-shot
func (q *Queue) Every(d time.Duration, fn func()) *Timer  // repeating, first fire at start+d
func (q *Queue) Advance(now time.Time)                    // fires everything due, in order
func (q *Queue) NextDue() (time.Time, bool)               // earliest pending due time
func (q *Queue) Len() int
type Timer struct{ /* due time.Time; period time.Duration; fn func(); stopped bool */ }
func (t *Timer) Stop()                                    // idempotent
```

- [ ] **Step 1: failing tests** — `timers/timers_test.go`:

```go
package timers

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestAfterFiresOnceInOrder(t *testing.T) {
	q := NewQueue(t0)
	var got []int
	q.After(30*time.Millisecond, func() { got = append(got, 30) })
	q.After(10*time.Millisecond, func() { got = append(got, 10) })
	q.Advance(t0.Add(20 * time.Millisecond))
	if len(got) != 1 || got[0] != 10 { t.Fatalf("got %v", got) }
	q.Advance(t0.Add(40 * time.Millisecond))
	if len(got) != 2 || got[1] != 30 { t.Fatalf("got %v", got) }
	q.Advance(t0.Add(100 * time.Millisecond)) // one-shots do not refire
	if len(got) != 2 { t.Fatalf("refired: %v", got) }
}

func TestEveryRepeats(t *testing.T) {
	q := NewQueue(t0)
	n := 0
	q.Every(10*time.Millisecond, func() { n++ })
	q.Advance(t0.Add(35 * time.Millisecond)) // catches up: fires at 10,20,30
	if n != 3 { t.Fatalf("n=%d", n) }
}

func TestStop(t *testing.T) {
	q := NewQueue(t0)
	n := 0
	tm := q.Every(10*time.Millisecond, func() { n++ })
	q.Advance(t0.Add(10 * time.Millisecond))
	tm.Stop()
	tm.Stop() // idempotent
	q.Advance(t0.Add(50 * time.Millisecond))
	if n != 1 { t.Fatalf("n=%d", n) }
	if q.Len() != 0 { t.Fatalf("stopped timer still queued") }
}

func TestNextDue(t *testing.T) {
	q := NewQueue(t0)
	if _, ok := q.NextDue(); ok { t.Fatal("empty queue has NextDue") }
	q.After(20*time.Millisecond, func() {})
	q.After(10*time.Millisecond, func() {})
	due, ok := q.NextDue()
	if !ok || !due.Equal(t0.Add(10*time.Millisecond)) { t.Fatalf("due=%v ok=%v", due, ok) }
}

func TestAddDuringFire(t *testing.T) {
	q := NewQueue(t0)
	n := 0
	q.After(10*time.Millisecond, func() {
		q.After(5*time.Millisecond, func() { n++ }) // due at 15, within same Advance window
	})
	q.Advance(t0.Add(30 * time.Millisecond))
	if n != 1 { t.Fatalf("timer added during fire did not fire in same Advance: n=%d", n) }
}
```

- [ ] **Step 2:** FAIL → implement: keep `items` sorted-on-demand; `Advance` loops "find earliest due ≤ now, fire it, reschedule (Every: due += period) or remove (After/stopped)" so timers added during callbacks participate; guard against zero/negative periods (treat Every(d<=0) as After).
- [ ] **Step 3:** green; full suite + vet + gofmt.
- [ ] **Step 4: commit** `feat(timers): frame-driven timer queue`

---

### Task 2: input events + HitPath

**Files:** Create `input/events.go`, `input/hittest.go`; Test `input/hittest_test.go`

**Interfaces:**
- Consumes: `core` (Widget, BoundsOf, IsVisible, Children), `render`.
- Produces (exact — Router and app build on these):

```go
package input

type Button uint8
const (ButtonNone Button = iota; ButtonLeft; ButtonRight; ButtonMiddle)
type Action uint8
const (Press Action = iota; Release; Move; Wheel; Enter; Leave)
type Modifiers uint8
const (ModShift Modifiers = 1 << iota; ModCtrl; ModAlt; ModSuper)
type Key int32
const ( // values match GLFW keycodes numerically; fluo-owned type
	KeyEscape Key = 256; KeyEnter Key = 257; KeyTab Key = 258; KeyBackspace Key = 259
	KeyDelete Key = 261; KeyRight Key = 262; KeyLeft Key = 263; KeyDown Key = 264; KeyUp Key = 265
	KeyHome Key = 268; KeyEnd Key = 269
)
type Cursor uint8
const (CursorArrow Cursor = iota; CursorIBeam; CursorHand; CursorHResize; CursorVResize)

type PointerEvent struct {
	Action  Action
	Pos     render.Point   // logical px
	Button  Button         // Press/Release only
	Delta   render.Point   // Wheel only (x,y scroll)
	Mods    Modifiers
	Target  core.Widget    // hit leaf (or captured widget)
	Router  *Router        // for Capture/Focus calls from handlers
	Handled bool
}
type KeyEvent struct {
	Action  Action         // Press or Release
	Key     Key
	Rune    rune           // char input events: the rune; else 0
	Mods    Modifiers
	Router  *Router
	Handled bool
}

// Opt-in interfaces — widgets implement any subset.
type PointerHandler interface{ OnPointer(e *PointerEvent) }
type KeyHandler interface{ OnKey(e *KeyEvent) }
type FocusHandler interface{ OnFocusChanged(focused bool) }
type Focusable interface{ AcceptsFocus() bool }
type CursorShaper interface{ Cursor() Cursor }

// HitPath returns the widget chain root→…→topmost leaf whose arranged bounds
// contain p. Hidden widgets (core.IsVisible false) are skipped along with
// their subtrees. Children are tested LAST-to-first (topmost painted wins).
// A widget is on the path only if its own bounds contain p. Empty if the root
// misses. Root need not contain p for children to be tested? NO — the root
// must contain p (bounds gate applies at every level).
func HitPath(root core.Widget, p render.Point) []core.Widget
```

- [ ] **Step 1: failing tests** — `input/hittest_test.go` (package input_test; build a tree of `controls.Fixed` inside `controls.NewCanvas` — Canvas gives absolute positions; measure/arrange first):

```go
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
	if len(path) != 1 { t.Fatalf("hidden child hit: %v", path) }
}

func TestHitPathNested(t *testing.T) {
	leaf := controls.NewFixed(20, 20, render.RGB(1, 0, 0))
	inner := controls.NewCanvas().Add(leaf, 5, 5)
	inner.SetWidth(50); inner.SetHeight(50)
	outer := controls.NewCanvas().Add(inner, 10, 10)
	layout(outer, 100, 100)
	path := input.HitPath(outer, render.Point{X: 20, Y: 20}) // inside leaf (15..35)
	if len(path) != 3 || path[2] != core.Widget(leaf) { t.Fatalf("path=%v", path) }
}
```

- [ ] **Step 2:** FAIL → implement events.go (pure declarations) + hittest.go (recursive descent per the doc comment).
- [ ] **Step 3:** green; suite+vet+gofmt. **Step 4: commit** `feat(input): event types and hit-testing`

---

### Task 3: Router — pointer dispatch, hover, capture

**Files:** Create `input/router.go`; Test `input/router_test.go`

**Interfaces:**
- Consumes: Tasks 1-2.
- Produces:

```go
package input

func NewRouter() *Router
func (r *Router) SetRoot(w core.Widget)
func (r *Router) Root() core.Widget
// Host entry points (logical px):
func (r *Router) PointerMove(p render.Point, mods Modifiers) Cursor
func (r *Router) PointerButton(b Button, press bool, p render.Point, mods Modifiers)
func (r *Router) PointerWheel(delta render.Point, p render.Point, mods Modifiers)
// Handler services:
func (r *Router) Capture(w core.Widget)   // all pointer events -> w until Release
func (r *Router) Release()
func (r *Router) Captured() core.Widget
```

Dispatch semantics (normative):
1. No capture: compute `path := HitPath(root, p)`. Bubble leaf→root: for each widget implementing `PointerHandler`, call `OnPointer(e)`; stop when `e.Handled`. `e.Target` = path leaf.
2. Captured: skip hit-testing; deliver the event ONLY to the captured widget (Target = captured; no bubbling), including Move/Release outside its bounds. Hover enter/leave is suppressed while captured; on Release-of-capture the next PointerMove recomputes hover.
3. Hover: on PointerMove (uncaptured), diff old vs new path; widgets leaving get `Action: Leave`, entering get `Action: Enter` (direct delivery, no bubbling, Handled ignored). Store new path.
4. Cursor: after a move, walk the new path leaf→root; first widget implementing `CursorShaper` wins; default CursorArrow. Return it from PointerMove.
5. Wheel bubbles like a pointer event with `Action: Wheel`.

- [ ] **Step 1: failing tests** — `input/router_test.go` (package input_test). Define ONE reusable test widget type:

```go
// probe: records events; configurable handled/focusable/cursor.
type probe struct {
	core.Element
	name      string
	events    []string // e.g. "press", "enter", "leave", "wheel", "key:9", "focus:true"
	handlePtr bool     // mark pointer events handled
	focusable bool
	cursor    input.Cursor
	router    *input.Router // captured on press when capturing==true
	capturing bool
}
func (p *probe) AcceptsFocus() bool { return p.focusable }
func (p *probe) Cursor() input.Cursor { return p.cursor }
func (p *probe) OnPointer(e *input.PointerEvent) {
	switch e.Action {
	case input.Press:
		p.events = append(p.events, "press")
		if p.capturing { e.Router.Capture(p) }
	case input.Release: p.events = append(p.events, "release")
	case input.Move: p.events = append(p.events, "move")
	case input.Enter: p.events = append(p.events, "enter")
	case input.Leave: p.events = append(p.events, "leave")
	case input.Wheel: p.events = append(p.events, "wheel")
	}
	if p.handlePtr { e.Handled = true }
}
func (p *probe) OnFocusChanged(f bool) { p.events = append(p.events, fmt.Sprintf("focus:%v", f)) }
```

Tests (all after building a Canvas tree + layout() like Task 2):
- `TestBubbleStopsAtHandled`: leaf probe (handlePtr=true) inside parent probe; press at leaf → leaf gets "press", parent gets nothing.
- `TestBubbleReachesParent`: leaf handlePtr=false → both get "press", leaf first.
- `TestHoverEnterLeave`: two side-by-side probes; Move over A → A "enter"; Move over B → A "leave", B "enter"; move within B again → no new enter.
- `TestCursorFromPath`: probe with cursor=CursorHand → PointerMove over it returns CursorHand; over empty canvas returns CursorArrow.
- `TestCaptureRoutesAll`: probe capturing=true; press on it, then Move at a point OUTSIDE its bounds → probe still receives "move" (and no enter/leave anywhere); Release; after `r.Release()` implicit? NO — capture released explicitly by handler; test calls r.Release() then moves → hover resumes.
- `TestWheelBubbles`: wheel over leaf → leaf gets "wheel".

Write each as real code in the test file (they are short; follow the patterns above exactly).

- [ ] **Step 2:** FAIL → implement router.go per the normative semantics. **Step 3:** green; suite+vet+gofmt. **Step 4: commit** `feat(input): pointer routing, hover, capture`

---

### Task 4: Router — keyboard, focus, tab navigation

**Files:** Modify `input/router.go`; Test: extend `input/router_test.go`

**Interfaces:**
- Produces (added to Router):

```go
func (r *Router) KeyDown(k Key, rn rune, mods Modifiers)  // rn!=0 => char input
func (r *Router) KeyUp(k Key, mods Modifiers)
func (r *Router) Focus(w core.Widget)  // nil clears; fires OnFocusChanged(false) then (true)
func (r *Router) Focused() core.Widget
func (r *Router) FocusNext()           // DFS order over AcceptsFocus()==true visible widgets; wraps
func (r *Router) FocusPrev()
```

Semantics (normative):
1. Key events go to the focused widget and bubble to root (parent chain via repeated hit? NO — bubbling uses the PARENT chain: from focused widget up through parents; core has no ParentOf accessor... **add one**: `core.ParentOf(w Widget) Widget` beside the other accessors, 3 lines, with test). Stop on Handled.
2. If a `KeyDown(KeyTab)` is not handled by any widget: `mods&ModShift` → FocusPrev else FocusNext, and the event becomes handled.
3. Press-to-focus: in Task 3's PointerButton press path (uncaptured), BEFORE bubbling: walk path leaf→root, focus the first widget with `AcceptsFocus() == true` (if not already focused). Clicking empty space (no focusable in path) clears focus.
4. Focus changes fire `OnFocusChanged(false)` on the old, `(true)` on the new (when they implement FocusHandler).
5. FocusNext/Prev: collect focusable+visible widgets by DFS over the root's tree (document order), find current, step +1/-1 with wraparound; empty list → no-op.

- [ ] **Step 1: failing tests** (extend router_test.go):
- `TestClickFocusesAndDefocuses`: focusable probe; press on it → "focus:true"; press on empty canvas → "focus:false".
- `TestKeyGoesToFocused`: focus probe via click; KeyDown(KeyEnter) → probe records "key" (extend probe with OnKey appending "key"); unfocused sibling gets nothing.
- `TestTabCycles`: three focusable probes A,B,C; click A; Tab → B focused; Tab → C; Tab → wraps to A; Shift+Tab → back to C.
- `TestKeyBubbles`: focused leaf (not handling keys) inside parent implementing OnKey with Handled=true → parent records the key event.
- `TestParentOf`: core accessor returns the canvas for an added child, nil for root.
- [ ] **Step 2:** FAIL → implement (including `core.ParentOf` + its doc comment). **Step 3:** green; suite+vet+gofmt. **Step 4: commit** `feat(input,core): keyboard routing, focus, tab navigation, ParentOf`

---

### Task 5: core render hooks + app integration (DPI gate)

**Files:** Modify `core/widget.go` (+`core/engine_test.go`), `app/window.go`

**Interfaces:**
- Produces in `core`:

```go
// ClipProvider lets a widget clip its children's rendering (e.g. ScrollViewer).
type ClipProvider interface{ ClipRect() (render.Rect, bool) }
// OverlayRenderer draws ABOVE the widget's children (scrollbars, adorners).
type OverlayRenderer interface{ RenderOverlay(r render.Renderer) }
// RenderWidget order becomes: w.Render → [PushClip] → children → [PopClip] → RenderOverlay.
```

- Produces in `app` (Ctx extended; Run signature unchanged):

```go
type Ctx struct {
	R      render.Renderer
	Size   render.Size      // logical px == window coords (DPI resolution)
	Scale  float32          // fbW / windowW (1 when windowW==0)
	Mouse  MouseState       // kept for compat; Pos in logical px
	Input  *input.Router    // set root once: ctx.Input.SetRoot(root)
	Timers *timers.Queue    // Advance()d by the host each frame before the frame callback
	Close  func()
}
```

app wiring (normative):
1. **DPI gate resolution:** `Scale = float32(fbW)/float32(winW)` (guard div-by-zero → 1). `Size = {winW, winH}` from `win.GetSize()` — NOT framebuffer/contentScale. Cursor pos used raw. Cursor, Size, and hit-testing are now the same coordinate space by construction on every platform. Document this in a comment block referencing the Phase-1 review gate; tick the ROADMAP DPI-gate box in Task 7.
2. glfw callbacks (set before the loop): CursorPos → `router.PointerMove` (store returned Cursor; apply via lazily-created standard glfw cursors when it changes); MouseButton → `router.PointerButton` (translate button + mods); Scroll → `router.PointerWheel` (delta ×48 logical px per notch? NO — pass raw notches; ScrollViewer applies its own step); Key → `router.KeyDown/KeyUp` (translate mods; ignore glfw Repeat? treat Repeat as KeyDown); Char → `router.KeyDown(0, r, mods)`.
3. Timers: `queue := timers.NewQueue(time.Now())`; each frame before the callback: `queue.Advance(time.Now())`.
4. Modifier translation helper `modsFrom(glfw.ModifierKey) input.Modifiers`; button helper likewise. Keys pass through numerically (`input.Key(k)`) — values match.
- [ ] **Step 1: failing core tests** — `core/engine_test.go`: a recorder `render.Renderer` stub already exists? NO — write a minimal `nullRenderer` in the test file implementing `render.Renderer` recording PushClip/PopClip calls and draw order markers via injected funcs. Tests: `TestRenderWidgetClipsChildren` (widget with ClipRect → PushClip before children, PopClip after) and `TestRenderWidgetOverlayAfterChildren` (order: self, child, overlay).
- [ ] **Step 2:** FAIL → implement hooks in RenderWidget. green.
- [ ] **Step 3:** app/window.go wiring per the normative list (no automated test — glfw glue; the gallery task verifies live). Build + vet + gofmt + full suite.
- [ ] **Step 4: commit** `feat(core,app): clip/overlay render hooks; input+timer host wiring; DPI coordinate-space fix`

---

### Task 6: ScrollViewer + golden

**Files:** Create `controls/scrollviewer.go`; Test `controls/scrollviewer_test.go` + `TestScrollClipRender` golden in `render/gl/renderer_test.go`

**Interfaces:**
- Produces:

```go
package controls

// ScrollViewer scrolls a single child vertically (v0: no horizontal bar).
func NewScrollViewer() *ScrollViewer
func (s *ScrollViewer) SetChild(w core.Widget) *ScrollViewer  // detaches old (Border convention)
func (s *ScrollViewer) OffsetY() float32
func (s *ScrollViewer) ScrollTo(y float32) *ScrollViewer      // clamped on next arrange
func (s *ScrollViewer) ScrollBy(dy float32)
// Implements: core.ClipProvider (own bounds), core.OverlayRenderer (thumb),
// input.PointerHandler (Wheel: ScrollBy(-Delta.Y*48), Handled=true; thumb drag via capture).
```

Semantics: MeasureContent: child measured (available.W minus thumbGutter=12, +Inf H); desired = min(child+gutter, available) per axis. ArrangeContent: viewport = bounds minus gutter on the right; clamp offset to [0, max(0, childH−viewportH)]; child arranged at {viewport.X, viewport.Y − offset, viewport.W, childDesiredH}. Thumb (overlay): when childH > viewportH: track = right gutter strip; thumbH = max(24, viewportH·viewportH/childH); thumbY proportional to offset; drawn `FillRoundedRect(radius 4)` in a neutral gray `render.RGBA(255,255,255,60)` (theme comes Phase 4). Drag: press within thumb rect → capture, store grab offset; Move while captured → offset from pointer delta; Release → release capture. Wheel anywhere in bounds scrolls.

- [ ] **Step 1: failing headless tests** — offset clamping (ScrollTo beyond end clamps to max; negative clamps 0), child position reflects offset (child BoundsOf.Y == viewport.Y − offset), wheel event via a manual `input.PointerEvent{Action: input.Wheel, Delta: render.Point{Y: -2}, Router: r}` handed to `OnPointer` moves offset by +96 and sets Handled, small-child case (no scrolling possible: offset pinned 0; thumb absent — expose `func (s *ScrollViewer) thumbRect() (render.Rect, bool)` unexported and test via a tiny exported-for-test? NO — test observable behavior only: offset stays 0). Write the real test code following controls/*_test.go patterns.
- [ ] **Step 2:** FAIL → implement. green.
- [ ] **Step 3: golden** — `TestScrollClipRender` in render/gl/renderer_test.go: 160×120 frame; ScrollViewer (120×100 at 10,10 via explicit size + Canvas) containing a vertical StackPanel of 8 Fixed bars (alternating blue/yellow, 30 tall); ScrollTo(45); measure/arrange/RenderWidget. Verify by reading scroll.png: bars visibly cut at viewport top/bottom edges (clip working), partial bars at both edges (offset 45 = 1.5 bars), thumb visible on right. `-update`, inspect, plain run passes.
- [ ] **Step 4:** suite+vet+gofmt. **Step 5: commit** `feat(controls): vertical ScrollViewer with clip, thumb, wheel`

---

### Task 7: interactive gallery + docs

**Files:** Modify `cmd/fluo-gallery/main.go`, `README.md`, `ROADMAP.md`

Gallery upgrade (complete code in this task's brief when extracted — write it fresh):
- Keep the Phase-2 chrome (title bar, nav, content dock).
- Content becomes a `ScrollViewer` containing a vertical StackPanel: the swatch WrapPanel (unchanged) PLUS 20 rows of `TextBlock` ("Row 01".."Row 20") so scrolling is meaningful.
- Make swatches interactive: wrap each `Fixed` in a new tiny gallery-local type `swatch` (embeds core.Element, implements input.PointerHandler + input.Focusable + input.CursorShaper): Enter/Leave toggles a `hover bool`, Press toggles `selected`; Render: the color block, +2px white stroke when hovered, +2px accent stroke when selected; Cursor: CursorHand. (~40 lines, in main.go — it is the consumer-side example of the event API.)
- Frame loop: `ctx.Input.SetRoot(root)` once; relayout guard unchanged.
- [ ] **Step 1:** implement; build+vet+gofmt+suite.
- [ ] **Step 2: live verification** — launch bounded (`timeout 60 go run ./cmd/fluo-gallery` in WSL, background), wait ~8s, `python tools/winshot.py "fluo gallery" <scratchpad>\fluo-gallery-p3.png`, READ: chrome intact + row list visible + scroll thumb on the content area's right edge. Kill (`pkill -f fluo-gallery`), verify dead. (Hover/press states can't be captured statically — the headless router tests already cover the mechanics.)
- [ ] **Step 3: docs** — ROADMAP: tick ALL Phase 3 boxes including the DPI gate (resolved by construction — note "resolved: logical space = window coords; scale = fb/window" on the line). README: one line under the gallery mention: interactive input demo.
- [ ] **Step 4: commit** `feat(gallery): interactive swatches + scrolling; complete Phase 3`

---

## Self-review notes (resolved inline)

- Every ROADMAP Phase 3 node has a task: events/mouse/keyboard/wheel→2-5, hit-testing→2, routed events→3-4, focus+capture→3-4, tab nav→4, cursor shapes→2/5, timer service→1, ScrollViewer→6, DPI gate→5/7.
- Type consistency: `probe` defined once in router_test.go; PointerEvent/KeyEvent fields match between Tasks 2-6; `core.ParentOf` introduced in Task 4 before any later use.
- Plan-level decisions recorded: opt-in handler interfaces (Widget untouched); logical space = GLFW window coords (DPI gate closed by construction); wheel step 48px owned by ScrollViewer, not the host.
