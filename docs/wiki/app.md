# app, anim, timers

Three packages that make up fluo's runtime: `app` is the desktop window
host you start a real program with (`Run`) or the embeddable retained-root
building block it's built on (`Surface`); `timers` is the frame-driven
scheduling queue that `Surface`/`Run` advance every frame and that controls
use for caret blink, tooltip hover-dwell, and animation; `anim` is the
opt-in tween/easing layer built on top of `timers` that controls use to
cross-fade between states instead of snapping. Reach for `app` first — it's
where every fluo program starts — and drop down to `timers`/`anim` when a
custom widget needs its own frame-driven behavior.

**Import:**
`github.com/0xdreadnaught/fluo/app`
`github.com/0xdreadnaught/fluo/anim`
`github.com/0xdreadnaught/fluo/timers`

## Contents
- [Minimal program](#minimal-program)
- [app](#app)
  - [Config](#config)
  - [MouseState](#mousestate)
  - [Ctx](#ctx)
  - [Run](#run)
  - [Surface](#surface)
- [anim](#anim)
  - [Easing](#easing)
  - [Tween](#tween)
- [timers](#timers)
  - [Queue](#queue)
  - [Timer](#timer)

---

## Minimal program

The shortest real fluo program: open a window, build a widget tree once,
install it as the frame callback's root, and let `Run` drive layout/render/
input every vsync.

```go
package main

import (
	"log"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/0xdreadnaught/fluo/app"
	"github.com/0xdreadnaught/fluo/controls"
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
)

func main() {
	font, err := text.Load(goregular.TTF)
	if err != nil {
		log.Fatal(err)
	}
	face := text.NewFace(font, 14)

	var root core.Widget
	var lastSize render.Size

	err = app.Run(app.Config{Title: "hello fluo", Width: 320, Height: 200}, func(c *app.Ctx) {
		if root == nil {
			btn := controls.NewButton(face, "Click me").OnClick(func() {
				log.Println("clicked")
			})
			root = controls.NewStackPanel(controls.Vertical).SetGap(8).Add(btn)
			c.Input.SetRoot(root)
		}
		if c.Size != lastSize {
			lastSize = c.Size
			core.MeasureWidget(root, c.Size)
			core.ArrangeWidget(root, render.Rect{X: 0, Y: 0, W: c.Size.W, H: c.Size.H})
		}
		core.RenderWidget(root, c.R)
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

---

## app

`Run` opens a GLFW window with an OpenGL 3.3 core context, wires up
`render/gl`'s `Renderer`, and drives a `frame(*Ctx)` callback every vsync
until the window closes. `Surface` is the lower-level building block `Run`
itself is built on: it owns layout, rendering, an `input.Router`, and a
`timers.Queue` against a caller-supplied GL context, for embedding fluo
into a host that already owns its own window and frame loop instead of
calling `Run` directly. **They are two separate APIs on purpose**: `Run`
owns the glfw window, GL context, and event callbacks end to end (the
normal case — a program that just wants a fluo window); `Surface` owns
none of that, so a consumer already running its own engine loop (or a
non-glfw windowing layer) can drive layout/render/input against its own
context without going through glfw at all. `Run`'s implementation is
literally a `Surface` plus glfw plumbing around it.

### Config

`Config` describes the window `Run` opens.

#### Fields

| Name | Type | Description |
|---|---|---|
| `Title` | `string` | Window title. |
| `Width` | `int` | Logical window width, in px. |
| `Height` | `int` | Logical window height, in px. |
| `Undecorated` | `bool` | When true, creates the window with no OS-drawn title bar or border (`glfw.WindowHint(glfw.Decorated, glfw.False)`) — pair with a [`controls.TitleBar`](controls-overlays.md#titlebar) drawn inside the frame callback for custom window chrome. Wire the `TitleBar`'s `OnMinimize`/`OnMaximize`/`OnClose` to `Ctx.Minimize`/`Ctx.ToggleMaximize`/`Ctx.Close`, and call `Ctx.BeginDrag` from the frame callback whenever a press lands inside the `TitleBar`'s own `DragRegion`. |

**Example — undecorated window with a TitleBar**

```go
var titleBar *controls.TitleBar

app.Run(app.Config{Title: "custom chrome", Width: 640, Height: 420, Undecorated: true}, func(c *app.Ctx) {
	if titleBar == nil {
		titleBar = controls.NewTitleBar(face, "custom chrome").
			OnMinimize(c.Minimize).
			OnMaximize(c.ToggleMaximize).
			OnClose(c.Close)
		// ... install titleBar into the tree, c.Input.SetRoot(root) ...
	}
	if c.Mouse.Down && titleBar.DragRegion(c.Mouse.Pos) {
		c.BeginDrag()
	}
	// ... layout/render ...
})
```

**Notes** — `BeginDrag` should be called once per press (on the down-edge),
not every frame the button stays held, or the drag's recorded start
position keeps re-arming and cancels out all movement. See
[`Ctx.BeginDrag`](#ctxbegindrag) and [`Run`](#run)'s doc comment for the
cursor-delta math this implements.

**See also** — [Ctx](#ctx), [Run](#run)

---

### MouseState

`MouseState` is the mouse position and left-button state for the current
frame, in logical pixels.

#### Fields

| Name | Type | Description |
|---|---|---|
| `Pos` | `render.Point` | Cursor position, logical px, same coordinate space as `Ctx.Size`. |
| `Down` | `bool` | Left mouse button held this frame. |

**See also** — [Ctx](#ctx)

---

### Ctx

`Ctx` is passed to the frame callback every vsync. It carries everything a
frame callback needs: the renderer, the window's size and scale, the
current mouse state (kept for compatibility — `Input` is the router that
actually dispatches events), the input router, the timer queue, and the
window-control hooks (`Close`/`Minimize`/`ToggleMaximize`/`BeginDrag`) for
wiring a `controls.TitleBar` when `Config.Undecorated` opts out of OS
window chrome.

#### Fields

| Name | Type | Description |
|---|---|---|
| `R` | `render.Renderer` | The frame's renderer. |
| `Size` | `render.Size` | Logical window size == window coordinates (see the DPI Notes below). |
| `Scale` | `float32` | `fbWidth / windowWidth` (1 when `windowWidth == 0`). For the renderer's benefit only — does not feed back into `Size` or `Mouse.Pos`. |
| `Mouse` | `MouseState` | Kept for compat; `Pos` in logical px. |
| `Input` | `*input.Router` | Root set once via `ctx.Input.SetRoot(root)`. |
| `Timers` | `*timers.Queue` | Advanced by the host every frame before `frame()` runs. |
| `Close` | `func()` | Requests app exit. |
| `Minimize` | `func()` | Iconifies the window (glfw `win.Iconify()`). Wire a `controls.TitleBar`'s `OnMinimize` to this — a decorated window already gets a native minimize button, so this hook only matters once `Config.Undecorated` has removed the OS chrome. Always non-nil. |
| `ToggleMaximize` | `func()` | Toggles the window between maximized and restored (glfw `win.Maximize()`/`win.Restore()`, tracked via an internal maximized flag the host owns across frames). Wire a `controls.TitleBar`'s `OnMaximize` to this. Always non-nil. |
| `BeginDrag` | `func()` | Starts an interactive window move (see [Run](#run)'s Notes for the cursor-delta implementation). Call it from the frame callback when a press lands inside a `controls.TitleBar`'s `DragRegion`; the move continues, tracked entirely by the host's own cursor/button callbacks, until the next left-button release. Always non-nil. |

**Notes — DPI coordinate space** — `Ctx.Size` is taken from
`win.GetSize()` — the window's logical size in GLFW's own window
coordinate system — NOT from the framebuffer size or content scale.
`win.GetCursorPos()` is reported in that same window coordinate system by
construction, and hit-testing (`input.Router`/`HitPath`) operates purely on
the widget tree's arranged bounds, which are laid out against `Ctx.Size`.
So cursor position, window size, and hit-test bounds are ALL expressed in
the same coordinate space on every platform GLFW supports, independent of
OS UI scaling or the monitor's backing-store density — there is no
separate logical-vs-physical-pixel conversion for the app layer to get
wrong. `Scale` is derived purely for the renderer's benefit (device pixels
per window coordinate, so the GL renderer can size its framebuffer-space
viewport and glyph atlases correctly).

**See also** — [Config](#config), [Run](#run), [Surface](#surface)

---

### Run

Opens the window and calls `frame` every vsync until closed. Blocks. Must
be called from `main()`; locks the OS thread.

**Syntax**

```go
func Run(cfg Config, frame func(*Ctx)) error
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `cfg` | `Config` | The window to open. |
| `frame` | `func(*Ctx)` | Called every vsync with a fresh `*Ctx`. |

**Returns** — `error`. Non-nil if GLFW fails to initialize, the window
fails to create, or GL fails to initialize.

**Example**

```go
err := app.Run(app.Config{Title: "fluo demo", Width: 480, Height: 320}, func(c *app.Ctx) {
	c.R.FillRect(render.Rect{X: 0, Y: 0, W: c.Size.W, H: c.Size.H}, render.RGB(32, 32, 36))
})
if err != nil {
	log.Fatal(err)
}
```

**Notes** — Undecorated window dragging (WSLg/X11 approach): glfw exposes
no native "start an interactive move" call the way a real window manager's
nonclient-area drag does, so `Ctx.BeginDrag` reimplements it at the
cursor-delta level: it records the cursor's current position in glfw's
window-relative coordinate system, then on every subsequent cursor-pos
callback (until the next left-button release) re-queries `win.GetPos()`
fresh and adds the window-relative cursor delta to it — because `GetPos`
is re-read live rather than cached, the component of the reported cursor
position that shifted merely because the window itself just moved cancels
out algebraically, leaving a pure screen-space follow. This works
unmodified under WSLg's XWayland and any other X11/Win32 glfw backend.
What's NOT reproduced is a true OS-native interactive move (the window
manager itself owning the drag, with edge-snapping etc.) — a documented v0
simplification. Also self-heals: if the left button is released without a
matching Release callback ever arriving (focus lost mid-drag, an XWayland
event hiccup), the cursor-pos callback checks the live button state on
every move and bails the moment it's no longer held, and losing window
focus mid-drag clears `dragging` directly rather than waiting for a
release that may never come.

**See also** — [Config](#config), [Ctx](#ctx), [Surface](#surface)

---

### Surface

`Surface` renders a fluo widget tree and routes input into a caller-owned
GL context. The caller owns the glfw window, GL context, and frame loop;
`Surface` owns layout, rendering, the `Router`, and a `Timers` queue.
`Run` (this package's own glfw-owning host) is built on top of a
`Surface`; a consumer embedding fluo in its own engine loop drives one
directly instead.

**Constructor**

```go
func NewSurface() *Surface
```

Creates a `Surface` with its own `Router` and `Timers` queue. Call
`SetRoot` before the first `Frame` that should render anything; a
`Surface` with no root just advances its `Timers` each `Frame` (a valid,
if unusual, use — e.g. driving timers/animations decoupled from any
visible tree).

**Example**

```go
surf := app.NewSurface()
surf.SetRoot(root)

// inside the caller's own frame loop, against its own GL context:
surf.Frame(myRenderer, winW, winH, fbW, fbH)
```

#### Methods

| Method | Signature | Description |
|---|---|---|
| [SetRoot](#surfacesetroot) | `SetRoot(w core.Widget) *Surface` | Installs w as the widget tree and the router's hit-test root. |
| [Router](#surfacerouter) | `Router() *input.Router` | Returns the Surface's input router. |
| [Timers](#surfacetimers) | `Timers() *timers.Queue` | Returns the Surface's timer queue. |
| [Frame](#surfaceframe) | `Frame(r render.Renderer, winW, winH, fbW, fbH int)` | Draws one frame: advance timers, layout if needed, render. |
| [PointerMove](#surfacepointermove) | `PointerMove(p render.Point, mods input.Modifiers) input.Cursor` | Forwards to `Router().PointerMove`. |
| [PointerButton](#surfacepointerbutton) | `PointerButton(b input.Button, press bool, p render.Point, mods input.Modifiers)` | Forwards to `Router().PointerButton`. |
| [PointerWheel](#surfacepointerwheel) | `PointerWheel(delta, p render.Point, mods input.Modifiers)` | Forwards to `Router().PointerWheel`. |
| [KeyDown](#surfacekeydown) | `KeyDown(k input.Key, r rune, mods input.Modifiers)` | Forwards to `Router().KeyDown`. |
| [KeyUp](#surfacekeyup) | `KeyUp(k input.Key, mods input.Modifiers)` | Forwards to `Router().KeyUp`. |

#### Surface.SetRoot

Installs `w` as both the widget tree `Frame` lays out/renders and the
Router's hit-test root.

**Syntax**

```go
func (s *Surface) SetRoot(w core.Widget) *Surface
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `w` | `core.Widget` | The new root widget. |

**Returns** — `*Surface` for chaining.

**Notes** — Calls `Router().SetRoot`, which itself resets hover/capture/
focus. Also resets the tracked last-seen size so the next `Frame` always
re-lays-out the new tree regardless of whether the window size happens to
match the old tree's.

**See also** — [Surface.Router](#surfacerouter), [Surface.Frame](#surfaceframe)

#### Surface.Router

Returns the Surface's input router.

**Syntax**

```go
func (s *Surface) Router() *input.Router
```

**Returns** — `*input.Router`.

**Notes** — Exposed for callers that want to reach it directly (e.g. to
Capture/Focus a widget); the `Pointer*`/`Key*` forwarders exist for
callers translating their own event types that would rather not import
the `input` package's `Router` just to feed events.

**See also** — [Surface.SetRoot](#surfacesetroot)

#### Surface.Timers

Returns the Surface's timer queue.

**Syntax**

```go
func (s *Surface) Timers() *timers.Queue
```

**Returns** — `*timers.Queue`.

**See also** — [Queue](#queue)

#### Surface.Frame

Draws one frame.

**Syntax**

```go
func (s *Surface) Frame(r render.Renderer, winW, winH, fbW, fbH int)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | The renderer to draw with. |
| `winW`, `winH` | `int` | The window's logical size (window coordinates). |
| `fbW`, `fbH` | `int` | The framebuffer size in device px. |

**Notes** — Advances the `Timers` queue against `time.Now()`, then — if
the root is set — (re)lays it out when the logical size has changed since
the last `Frame` call or the root reports `NeedsLayout` (a fresh root, or
one mutated since the last frame), and renders it via `r`. A nil root
(never set via `SetRoot`) is valid and just advances timers. `scale =
fbW/winW`, guarding `winW == 0` (matching the convention `Ctx.Scale`
documents).

**Example**

```go
surf.Frame(r, winW, winH, fbW, fbH)
```

**See also** — [Surface.SetRoot](#surfacesetroot), [Queue.Advance](#queueadvance)

#### Surface.PointerMove

Forwards to `Router().PointerMove`.

**Syntax**

```go
func (s *Surface) PointerMove(p render.Point, mods input.Modifiers) input.Cursor
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `p` | `render.Point` | Cursor position. |
| `mods` | `input.Modifiers` | Active modifier keys. |

**Returns** — `input.Cursor`, the cursor shape the hit widget under `p` requests.

**See also** — [Surface.Router](#surfacerouter)

#### Surface.PointerButton

Forwards to `Router().PointerButton`.

**Syntax**

```go
func (s *Surface) PointerButton(b input.Button, press bool, p render.Point, mods input.Modifiers)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `b` | `input.Button` | Which mouse button. |
| `press` | `bool` | true for a press, false for a release. |
| `p` | `render.Point` | Cursor position. |
| `mods` | `input.Modifiers` | Active modifier keys. |

**See also** — [Surface.Router](#surfacerouter)

#### Surface.PointerWheel

Forwards to `Router().PointerWheel`.

**Syntax**

```go
func (s *Surface) PointerWheel(delta, p render.Point, mods input.Modifiers)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `delta` | `render.Point` | Raw scroll notch delta. |
| `p` | `render.Point` | Cursor position. |
| `mods` | `input.Modifiers` | Active modifier keys. |

**See also** — [Surface.Router](#surfacerouter)

#### Surface.KeyDown

Forwards to `Router().KeyDown`.

**Syntax**

```go
func (s *Surface) KeyDown(k input.Key, r rune, mods input.Modifiers)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `k` | `input.Key` | The key pressed. |
| `r` | `rune` | The rune produced (text input), or 0. |
| `mods` | `input.Modifiers` | Active modifier keys. |

**See also** — [Surface.KeyUp](#surfacekeyup)

#### Surface.KeyUp

Forwards to `Router().KeyUp`.

**Syntax**

```go
func (s *Surface) KeyUp(k input.Key, mods input.Modifiers)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `k` | `input.Key` | The key released. |
| `mods` | `input.Modifiers` | Active modifier keys. |

**See also** — [Surface.KeyDown](#surfacekeydown)

---

## anim

`anim` provides easing functions and a frame-driven `Tween` that
interpolates a normalized progress value 0→1 over a duration, using a
`timers.Queue` as its clock. It is the shared foundation controls use to
smoothly cross-fade colors instead of snapping between states (see
`controls.Button`'s animated fill, below). Animation is opt-in end to end:
nothing in fluo starts a `Tween` on your behalf.

**Opting in.** A control that supports animation (e.g. `controls.Button`)
exposes `SetAnimated(true)` plus a `SetTimers(q *timers.Queue)` — both must
be set for the cross-fade to actually run; `SetAnimated(true)` with no
queue wired falls back to an instant jump, and the library-wide default
(`SetAnimated` never called) is today's exact snap-to-state behavior,
byte-identical to before animation existed:

```go
btn := controls.NewButton(face, "Accent").
	SetAnimated(true).
	SetTimers(c.Timers) // c is the frame *app.Ctx; or surf.Timers()
```

### Easing

`Easing` maps a normalized progress `t` in `[0,1]` to an eased progress,
also in `[0,1]`. Every `Easing` in this package satisfies `f(0)==0` and
`f(1)==1`.

```go
type Easing func(t float32) float32
```

#### Functions

| Function | Signature | Description |
|---|---|---|
| [Linear](#linear) | `func Linear(t float32) float32` | Returns t unchanged — no easing. |
| [EaseOut](#easeout) | `func EaseOut(t float32) float32` | Cubic ease-out: fast start, decelerates into the endpoint. |
| [EaseInOut](#easeinout) | `func EaseInOut(t float32) float32` | Cubic ease-in-out: accelerates out of 0, decelerates into 1. |

#### Linear

Returns `t` unchanged — no easing.

**Syntax**

```go
func Linear(t float32) float32
```

**Returns** — `float32`, equal to `t`.

**See also** — [EaseOut](#easeout), [EaseInOut](#easeinout)

#### EaseOut

Cubic ease-out curve (`1-(1-t)^3`): progress starts fast and decelerates
into the endpoint, front-loading motion so it reads as "arriving" rather
than "departing".

**Syntax**

```go
func EaseOut(t float32) float32
```

**Returns** — `float32`. Monotonically increasing over `[0,1]`.

**Notes** — This is the curve `controls`' `colorAnim` uses for its
~120ms hover/press fill cross-fade.

**See also** — [Linear](#linear), [EaseInOut](#easeinout), [Tween](#tween)

#### EaseInOut

Cubic ease-in-out curve: accelerates out of 0, decelerates into 1,
symmetric about the midpoint.

**Syntax**

```go
func EaseInOut(t float32) float32
```

**Returns** — `float32`. Monotonically increasing over `[0,1]`.

**See also** — [Linear](#linear), [EaseOut](#easeout), [Tween](#tween)

---

### Tween

`Tween` interpolates a normalized progress 0→1 over a duration, driven by
a `timers.Queue`'s `Advance`: each internal tick computes `elapsed/d`
(clamped to `[0,1]`), calls `onUpdate(ease(t))`, and — once `t` reaches 1
— calls `onDone` exactly once and stops itself (no further `onUpdate`/
`onDone` calls, even if the queue keeps advancing).

**Constructor**

```go
func NewTween(q *timers.Queue, d time.Duration, ease Easing, onUpdate func(float32), onDone func()) *Tween
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `q` | `*timers.Queue` | The clock driving the tween's ticks. |
| `d` | `time.Duration` | Total duration to interpolate over. |
| `ease` | `Easing` | The curve applied to raw progress before `onUpdate`. |
| `onUpdate` | `func(float32)` | Called with `ease(t)` on every internal tick. |
| `onDone` | `func()` | Called exactly once when progress reaches 1. |

`onUpdate` is called at least once even for a zero (or negative) duration,
synchronously inside `NewTween`, with `onDone` following immediately — a
degenerate but valid "already done" tween.

**Example**

```go
tw := anim.NewTween(c.Timers, 120*time.Millisecond, anim.EaseOut,
	func(v float32) { alpha = v },
	func() { fmt.Println("fade complete") },
)
```

#### Methods

| Method | Signature | Description |
|---|---|---|
| [Stop](#tweenstop) | `Stop()` | Halts the tween immediately without firing onDone. |

#### Tween.Stop

Halts the tween immediately: no further `onUpdate` calls, and `onDone` is
NOT called (`Stop` is a cancellation, not a completion).

**Syntax**

```go
func (tw *Tween) Stop()
```

**Notes** — Idempotent — calling `Stop` again (or after the tween has
already completed on its own) is a no-op. Not goroutine-safe (matches
`timers.Queue` itself). If the host stalls and then advances the queue by
more than one internal tick interval in a single `Advance` call, the
underlying `timers.Queue` catches up by firing every due tick within that
call, so `onUpdate` (and, at most once, `onDone`) may be called several
times within that one `Advance` — bounded by roughly `d`/tick-interval,
never unbounded. Keep `onUpdate` cheap for this reason.

**Example**

```go
tw.Stop() // e.g. the widget was disposed mid-fade
```

**See also** — [NewTween](#tween)

---

## timers

`timers` provides a frame-driven timer service: `Queue` holds a set of
pending `Timer` callbacks, and the host calls `Advance(now)` once per
frame to fire whichever are due, in due-time order. `After` schedules a
one-shot callback; `Every` schedules a repeating one; both return a
`*Timer` whose `Stop` cancels it. `Queue` is not goroutine-safe and
carries no wall-clock dependency of its own — `Advance` is driven entirely
by the caller's notion of "now" — which is why `app.Surface` owns one and
advances it every `Frame`, and why controls like `TextBox` (caret blink)
and `ToolTipArea` (hover-dwell) accept a `*timers.Queue` via `SetTimers`
rather than starting their own goroutine timers.

### Queue

`Queue` is a frame-driven timer service: the host calls `Advance(now)`
once per frame; due timers fire on that call, in due-time order. Not
goroutine-safe.

**Constructor**

```go
func NewQueue(start time.Time) *Queue
```

Creates a new timer queue starting at the given time.

**Example**

```go
q := timers.NewQueue(time.Now())
q.After(2*time.Second, func() { fmt.Println("fired") })

// once per frame:
q.Advance(time.Now())
```

#### Methods

| Method | Signature | Description |
|---|---|---|
| [After](#queueafter) | `After(d time.Duration, fn func()) *Timer` | Schedules a one-shot callback to fire after duration d. |
| [Every](#queueevery) | `Every(d time.Duration, fn func()) *Timer` | Schedules a repeating callback to fire every duration d. |
| [Advance](#queueadvance) | `Advance(now time.Time)` | Fires all timers due at or before now, in due-time order. |
| [NextDue](#queuenextdue) | `NextDue() (time.Time, bool)` | Returns the time of the next due timer. |
| [Len](#queuelen) | `Len() int` | Returns the number of pending timers in the queue. |

#### Queue.After

Schedules a one-shot callback to fire after duration `d`.

**Syntax**

```go
func (q *Queue) After(d time.Duration, fn func()) *Timer
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `d` | `time.Duration` | Delay from the queue's current time before `fn` fires. |
| `fn` | `func()` | The callback to run once, when due. |

**Returns** — `*Timer`, whose `Stop` cancels the callback before it fires.

**Example**

```go
q.After(500*time.Millisecond, func() { tooltip.Show() })
```

**See also** — [Queue.Every](#queueevery), [Timer.Stop](#timerstop)

#### Queue.Every

Schedules a repeating callback to fire every duration `d` (first fire at
`start+d`).

**Syntax**

```go
func (q *Queue) Every(d time.Duration, fn func()) *Timer
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `d` | `time.Duration` | Period between fires. If `d <= 0`, treated as `After` (one-shot). |
| `fn` | `func()` | The callback to run on every fire. |

**Returns** — `*Timer`, whose `Stop` cancels future fires.

**Example**

```go
caretTimer = q.Every(530*time.Millisecond, func() { caretOn = !caretOn })
```

**Notes** — This is the mechanism `anim.NewTween` and `TextBox`'s caret
blink are both built on.

**See also** — [Queue.After](#queueafter), [Timer.Stop](#timerstop), [Tween](#tween)

#### Queue.Advance

Fires all timers due at or before the given time, in due-time order.

**Syntax**

```go
func (q *Queue) Advance(now time.Time)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `now` | `time.Time` | The caller's current notion of "now". |

**Example**

```go
q.Advance(time.Now())
```

**Notes** — Timers added during callbacks participate in the same
`Advance` call: the loop keeps firing the earliest due timer, re-sorting
after each fire, until nothing left in the queue is due at or before
`now`. A repeating timer whose callback doesn't stop it is rescheduled in
place (`due += period`) rather than removed. If the host stalls and then
calls `Advance` with a `now` far in the future, every timer that fell due
in that gap fires within this single call, in order — this is what lets a
`Tween` "catch up" after a frame-loop stall instead of silently skipping
ticks. Not goroutine-safe.

**See also** — [Queue.NextDue](#queuenextdue), [Surface.Frame](#surfaceframe)

#### Queue.NextDue

Returns the time of the next due timer, or false if the queue is empty.

**Syntax**

```go
func (q *Queue) NextDue() (time.Time, bool)
```

**Returns** — `(time.Time, bool)`. The second value is false when the
queue has no pending timers, in which case the `time.Time` is the zero
value.

**Example**

```go
if due, ok := q.NextDue(); ok {
	fmt.Println("next timer fires at", due)
}
```

**See also** — [Queue.Len](#queuelen)

#### Queue.Len

Returns the number of pending timers in the queue.

**Syntax**

```go
func (q *Queue) Len() int
```

**Returns** — `int`.

**Example**

```go
if q.Len() == 0 {
	// nothing scheduled
}
```

**See also** — [Queue.NextDue](#queuenextdue)

---

### Timer

`Timer` represents a scheduled callback, either one-shot (`After`) or
repeating (`Every`). Constructed only by `Queue.After`/`Queue.Every` —
there is no public constructor.

#### Methods

| Method | Signature | Description |
|---|---|---|
| [Stop](#timerstop) | `Stop()` | Cancels the timer and removes it from the queue immediately. |

#### Timer.Stop

Cancels the timer and removes it from the queue immediately.

**Syntax**

```go
func (t *Timer) Stop()
```

**Notes** — Idempotent.

**Example**

```go
caretTimer.Stop() // e.g. the TextBox lost focus
```

**See also** — [Queue.After](#queueafter), [Queue.Every](#queueevery)
