# input

The `input` package turns raw pointer/keyboard/wheel events into dispatch
against a `core.Widget` tree. `Router` is the package's central type: it
hit-tests pointer events against the tree rooted at `SetRoot`, bubbles them
(and Enter/Leave transitions) along the resulting hit path, tracks an
optional nested pointer capture (`Capture`/`Release`) that redirects all
pointer events straight to one widget, and tracks keyboard focus
(`Focus`/`Focused`/`FocusNext`/`FocusPrev`), bubbling `KeyDown`/`KeyUp` from
the focused widget up its `core.ParentOf` ancestor chain. Widgets opt into
event handling by implementing one or more of `PointerHandler`, `KeyHandler`,
`Focusable`, or `CursorShaper`; `PointerEvent` and `KeyEvent` (each with a
settable `Handled` flag) are the payloads delivered to them. `Clipboard`
abstracts host clipboard access (wired via `SetClipboard`) so `TextBox`'s
Ctrl+C/X/V works without `input` depending on any windowing package. Reach
for this package when you're building a custom widget that needs to react to
pointer/keyboard input, or a custom host that drives a `Router` from a
window's callbacks — the `app` package is `Router`'s principal driver,
translating glfw callbacks into `Router` calls every frame.

**Import:** `github.com/0xdreadnaught/fluo/input`

## Contents
- [Router](#router)
- [PointerEvent](#pointerevent)
- [KeyEvent](#keyevent)
- [PointerHandler](#pointerhandler)
- [KeyHandler](#keyhandler)
- [FocusHandler](#focushandler)
- [Focusable](#focusable)
- [CursorShaper](#cursorshaper)
- [Clipboard](#clipboard)
- [HitPath](#hitpath)
- [Bubble](#bubble)
- [Constants reference](#constants-reference)

---

## Router

`Router` owns the widget tree's input-dispatch state: which widgets are
currently hovered (for Enter/Leave), which widget (if any) holds an active
pointer capture, and which widget (if any) holds keyboard focus. Pointer
events (`PointerMove`/`PointerButton`/`PointerWheel`) hit-test and bubble
along the resulting path, or go straight to the captured widget while one
holds the grab. Key events (`KeyDown`/`KeyUp`) instead bubble from the
focused widget up its `core.ParentOf` ancestor chain, since the focused
widget need not be under the pointer at all; an unhandled Tab/Shift+Tab
`KeyDown` moves focus via `FocusNext`/`FocusPrev`.

**Constructor**

```go
func NewRouter() *Router
```

Creates an empty `Router` with no root, no hover, no capture, and no focus.
Call `SetRoot` before dispatching any events — `PointerMove`,
`PointerButton`, and `PointerWheel` are no-ops (beyond capture delivery)
until a root is set.

**Example**

```go
router := input.NewRouter()
router.SetRoot(rootWidget)
router.SetClipboard(hostClipboard)
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [SetRoot](#routersetroot) | `SetRoot(w core.Widget)` | Sets the root widget, resetting hover, capture, and focus. |
| [Root](#routerroot) | `Root() core.Widget` | Returns the root widget. |
| [SetClipboard](#routersetclipboard) | `SetClipboard(c Clipboard)` | Installs host clipboard access. |
| [Clipboard](#routerclipboard) | `Clipboard() Clipboard` | Returns the installed clipboard, or nil. |
| [Capture](#routercapture) | `Capture(w core.Widget)` | Routes subsequent pointer events to `w` exclusively. |
| [Release](#routerrelease) | `Release()` | Ends the current (topmost) pointer capture. |
| [Captured](#routercaptured) | `Captured() core.Widget` | Returns the widget holding the pointer capture, or nil. |
| [Detach](#routerdetach) | `Detach(w core.Widget)` | Clears hover/capture/focus references into `w`'s subtree. |
| [PointerMove](#routerpointermove) | `PointerMove(p render.Point, mods Modifiers) Cursor` | Routes a pointer-move; returns the cursor to display. |
| [PointerButton](#routerpointerbutton) | `PointerButton(b Button, press bool, p render.Point, mods Modifiers)` | Routes a press or release. |
| [PointerWheel](#routerpointerwheel) | `PointerWheel(delta render.Point, p render.Point, mods Modifiers)` | Routes a wheel/scroll event. |
| [Focus](#routerfocus) | `Focus(w core.Widget)` | Sets (or clears) keyboard focus. |
| [Focused](#routerfocused) | `Focused() core.Widget` | Returns the focused widget, or nil. |
| [FocusNext](#routerfocusnext) | `FocusNext()` | Moves focus to the next focusable+visible widget, wrapping. |
| [FocusPrev](#routerfocusprev) | `FocusPrev()` | Moves focus to the previous focusable+visible widget, wrapping. |
| [KeyDown](#routerkeydown) | `KeyDown(k Key, rn rune, mods Modifiers)` | Routes a key-press; handles Tab navigation if unconsumed. |
| [KeyUp](#routerkeyup) | `KeyUp(k Key, mods Modifiers)` | Routes a key-release. |

#### Router.SetRoot

Sets the root widget for this router.

**Syntax**

```go
func (r *Router) SetRoot(w core.Widget)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `w` | `core.Widget` | The new root of the tree this router dispatches into. |

**Notes** — `SetRoot` resets hover, capture, and focus: the previous tree's
hover path and pointer capture are discarded directly (no Enter/Leave or
capture-release notifications fire for widgets that are about to become
unreachable), while focus is cleared via `Focus(nil)` so `OnFocusChanged(false)`
still fires normally on whatever widget previously held it.

**See also** — [Root](#routerroot), [Detach](#routerdetach)

#### Router.Root

Returns the root widget for this router.

**Syntax**

```go
func (r *Router) Root() core.Widget
```

**Returns** — `core.Widget`, the current root (as last set by `SetRoot`), or
nil if none has been set.

**See also** — [SetRoot](#routersetroot)

#### Router.SetClipboard

Installs the host-provided system clipboard access.

**Syntax**

```go
func (r *Router) SetClipboard(c Clipboard)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `c` | `Clipboard` | The host clipboard implementation. Passing `nil` (the zero value) puts the router back into headless mode. |

**See also** — [Clipboard (method)](#routerclipboard), [Clipboard (interface)](#clipboard)

#### Router.Clipboard

Returns the host-provided system clipboard access, or nil if none was set.

**Syntax**

```go
func (r *Router) Clipboard() Clipboard
```

**Returns** — `Clipboard`, or nil for headless/test routers.

**Notes** — Callers must nil-check before use.

**See also** — [SetClipboard](#routersetclipboard)

#### Router.Capture

Routes all subsequent pointer events to `w` exclusively, bypassing
hit-testing and hover, until a matching `Release` is called.

**Syntax**

```go
func (r *Router) Capture(w core.Widget)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `w` | `core.Widget` | The widget to capture the pointer for. |

**Notes** — Capture NESTS rather than simply overwriting: calling `Capture`
while another widget already holds the grab pushes `w` on top, and a later
`Release` pops back to that previous captor instead of clearing capture
outright. This matters whenever a capture can begin from inside an event
that's itself being delivered under someone else's capture — e.g. an
`OverlayHost` holds a modal capture while a popup is open and forwards
pointer events into the popup's own subtree; if a widget inside that popup
(a `ScrollViewer` thumb, say) captures for its own drag, releasing that drag
must restore the host's modal capture, not silently drop it.

`Capture` is idempotent when `w` already IS the current top of the stack: it
does nothing rather than pushing a second identical entry (otherwise a
caller that re-asserts its own capture repeatedly would accumulate one stack
entry per call, and a single matching `Release` would only pop one of them —
leaving `Captured()` still reporting `w` even after the caller believes it
fully released). `Capture` with a genuinely different widget than the
current top still nests normally.

**CAVEAT** — the idempotence check only looks at the CURRENT top, not the
whole stack, so a stack shaped `h→w→h` (h captured, then w nested over it,
then h re-asserts its own capture again while w is still on top — e.g. a
second modal popup opened while a popup-internal drag from the first is
still in progress) is NOT deduplicated: h is pushed again, genuinely nesting
a third entry over w. A single `Release` from w's own handler then only pops
the duplicate h, restoring w — not the outer h a caller might expect. This
is a known, currently undefended v0 edge case (no current caller triggers
it).

**Example**

```go
func (b *Button) OnPointer(e *input.PointerEvent) {
    if e.Action == input.Press {
        e.Router.Capture(b)
        e.Handled = true
    }
}
```

**See also** — [Release](#routerrelease), [Captured](#routercaptured)

#### Router.Release

Ends the current (topmost) pointer capture, if any.

**Syntax**

```go
func (r *Router) Release()
```

**Notes** — Restores whichever capture (if any) was active before it — see
`Capture`'s nesting behavior. Calling `Release` with no capture active is a
no-op. Only once the stack is fully unwound (`Captured() == nil`) does the
next `PointerMove` recompute the hover path from scratch — hover is left
untouched for the entire time ANY capture, nested or not, is active, so it
is diffed against whatever it was before the outermost capture began.

**See also** — [Capture](#routercapture), [Captured](#routercaptured)

#### Router.Captured

Returns the widget currently holding the pointer capture.

**Syntax**

```go
func (r *Router) Captured() core.Widget
```

**Returns** — `core.Widget`, the top of the nested capture stack, or nil if
none.

**See also** — [Capture](#routercapture), [Release](#routerrelease)

#### Router.Detach

Clears hover/capture/focus references that point at `w` or any widget in
`w`'s subtree.

**Syntax**

```go
func (r *Router) Detach(w core.Widget)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `w` | `core.Widget` | The widget (and its subtree, walked via `Children()`) to purge from router state. A nil `w` is a no-op. |

**Notes** — Call before removing a subtree (e.g. a closing popup), so the
router doesn't keep dispatching to, or holding capture/focus on, widgets
that are about to become unreachable. Focus is cleared through `Focus(nil)`
when the focused widget is `w` or in `w`'s subtree, so `OnFocusChanged(false)`
still fires on it exactly as it does for `SetRoot`. Capture and hover, by
contrast, are cleared silently — capture has no release-notification
concept, and hover's Enter/Leave pair is meaningless for a widget that's
being torn down rather than merely un-hovered.

Capture is a stack: every entry — not just the current top — is checked
against `w`'s subtree and filtered out if it falls inside it, so a detach
that removes a middle entry doesn't leave a dangling reference behind it
either. After filtering, the new top of the surviving stack (if any)
becomes the active capture, exactly as if that widget's own `Release` had
already run.

**See also** — [SetRoot](#routersetroot), [Capture](#routercapture)

#### Router.PointerMove

Routes a pointer-move at `p` (logical px).

**Syntax**

```go
func (r *Router) PointerMove(p render.Point, mods Modifiers) Cursor
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `p` | `render.Point` | The pointer position, in logical window px. |
| `mods` | `Modifiers` | Currently-held keyboard modifiers. |

**Returns** — `Cursor`. While captured, the cursor is the captured widget's
own (`CursorArrow` if it doesn't implement `CursorShaper`). Otherwise it's
the first `CursorShaper` cursor found walking the new hit-test path leaf→root,
or `CursorArrow` if none of the path's widgets shape a cursor.

**Example**

```go
cursor := router.PointerMove(render.Point{X: 120, Y: 40}, 0)
glfwWindow.SetCursor(toGLFWCursor(cursor))
```

**Notes** — While captured, the event goes only to the captured widget (no
hover, no bubbling) — hover is not tracked during a capture, a documented v0
simplification (see `controls.ClickBehavior.HandlePointer`'s doc comment).
Otherwise it hit-tests via [HitPath](#hitpath), updates hover (delivering
Enter/Leave to widgets that changed hover state), and bubbles a `Move` event
leaf→root via [Bubble](#bubble). A nil root (no `SetRoot` call yet) is a
no-op beyond capture delivery.

**See also** — [PointerButton](#routerpointerbutton), [PointerWheel](#routerpointerwheel), [CursorShaper](#cursorshaper)

#### Router.PointerButton

Routes a press or release at `p` (logical px).

**Syntax**

```go
func (r *Router) PointerButton(b Button, press bool, p render.Point, mods Modifiers)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `b` | `Button` | Which mouse button. |
| `press` | `bool` | `true` for a press (dispatched as `Press`), `false` for a release (dispatched as `Release`). |
| `p` | `render.Point` | The pointer position, in logical window px. |
| `mods` | `Modifiers` | Currently-held keyboard modifiers. |

**Example**

```go
router.PointerButton(input.ButtonLeft, true, render.Point{X: 120, Y: 40}, 0)
router.PointerButton(input.ButtonLeft, false, render.Point{X: 120, Y: 40}, 0)
```

**Notes** — While captured, the event goes only to the captured widget,
`Target` = captured, no bubbling — including when `p` falls outside the
captured widget's bounds. Otherwise it hit-tests via `HitPath`, applies
press-to-focus on a press (focuses the first `Focusable` widget on the path
that reports `AcceptsFocus() == true`, walking leaf→root; if none qualify,
focus is retained rather than cleared), and bubbles leaf→root via `Bubble`,
stopping at the first handler that sets `e.Handled`.

**See also** — [PointerMove](#routerpointermove), [Focus](#routerfocus)

#### Router.PointerWheel

Routes a wheel/scroll event at `p` (logical px).

**Syntax**

```go
func (r *Router) PointerWheel(delta render.Point, p render.Point, mods Modifiers)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `delta` | `render.Point` | The scroll delta (x, y). |
| `p` | `render.Point` | The pointer position, in logical window px. |
| `mods` | `Modifiers` | Currently-held keyboard modifiers. |

**Example**

```go
router.PointerWheel(render.Point{Y: -3}, render.Point{X: 200, Y: 300}, 0)
```

**Notes** — Bubbles leaf→root like a pointer event (`Action: Wheel`), or
goes only to the captured widget if one holds the pointer grab.

**See also** — [PointerMove](#routerpointermove), [PointerButton](#routerpointerbutton)

#### Router.Focus

Sets `w` as the focused widget, or clears focus entirely when `w` is nil.

**Syntax**

```go
func (r *Router) Focus(w core.Widget)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `w` | `core.Widget` | The widget to focus, or nil to clear focus. |

**Notes** — A no-op if `w` is already focused. Otherwise fires
`OnFocusChanged(false)` on the previously focused widget (if it implements
`FocusHandler`), then `OnFocusChanged(true)` on `w` (likewise), in that
order. Focus is not reentrant: focus changes requested from within an
`OnFocusChanged` callback (via `Focus`, `FocusNext`, or `FocusPrev`, from
either the blurring or the focusing widget) are ignored — without this
guard, two widgets that each try to reclaim focus from within
`OnFocusChanged` could recurse into each other unboundedly.

**See also** — [Focused](#routerfocused), [FocusNext](#routerfocusnext), [FocusPrev](#routerfocusprev), [FocusHandler](#focushandler)

#### Router.Focused

Returns the widget currently holding keyboard focus, or nil.

**Syntax**

```go
func (r *Router) Focused() core.Widget
```

**Returns** — `core.Widget`, or nil if nothing is focused.

**See also** — [Focus](#routerfocus)

#### Router.FocusNext

Moves focus to the next focusable+visible widget in document order.

**Syntax**

```go
func (r *Router) FocusNext()
```

**Notes** — Document order is DFS from the root, wrapping from the last
widget back to the first. If nothing is currently focused (or the focused
widget is no longer in the list), it focuses the first entry. A no-op if
there are no focusable widgets at all. A hidden widget's entire subtree is
skipped, matching `HitPath`'s treatment of hidden subtrees. This is what an
unhandled `KeyTab` (without Shift) triggers from `KeyDown`.

**See also** — [FocusPrev](#routerfocusprev), [Focus](#routerfocus), [KeyDown](#routerkeydown)

#### Router.FocusPrev

Moves focus to the previous focusable+visible widget in document order.

**Syntax**

```go
func (r *Router) FocusPrev()
```

**Notes** — Same traversal as `FocusNext`, reversed: wraps from the first
widget back to the last. If nothing is currently focused (or the focused
widget is no longer in the list), it focuses the last entry. A no-op if
there are no focusable widgets at all. This is what an unhandled
`KeyTab`+`ModShift` triggers from `KeyDown`.

**See also** — [FocusNext](#routerfocusnext), [Focus](#routerfocus), [KeyDown](#routerkeydown)

#### Router.KeyDown

Routes a key-press.

**Syntax**

```go
func (r *Router) KeyDown(k Key, rn rune, mods Modifiers)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `k` | `Key` | The key code pressed. |
| `rn` | `rune` | The produced character for char-input events, else `0`. |
| `mods` | `Modifiers` | Currently-held keyboard modifiers. |

**Example**

```go
router.KeyDown(input.KeyTab, 0, 0)
router.KeyDown(input.KeyA, 'a', 0)
```

**Notes** — The event bubbles from the focused widget up through its parent
chain (`core.ParentOf`), stopping as soon as `e.Handled` is set; with no
focused widget, delivery is to the root only. If, after that, `KeyTab`
remains unhandled, the router itself consumes it: `ModShift` moves focus to
the previous focusable widget (`FocusPrev`), otherwise to the next
(`FocusNext`), and the event is marked handled — this is router-internal
bookkeeping, not delivered to any widget.

**See also** — [KeyUp](#routerkeyup), [FocusNext](#routerfocusnext), [FocusPrev](#routerfocusprev)

#### Router.KeyUp

Routes a key-release.

**Syntax**

```go
func (r *Router) KeyUp(k Key, mods Modifiers)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `k` | `Key` | The key code released. |
| `mods` | `Modifiers` | Currently-held keyboard modifiers. |

**Notes** — Routed the same way `KeyDown` is (focused widget, bubbling up
the parent chain), with no Tab handling.

**See also** — [KeyDown](#routerkeydown)

---

## PointerEvent

`PointerEvent` represents a mouse or touch pointer event: the payload
delivered to a `PointerHandler`'s `OnPointer` for every pointer action
(`Press`, `Release`, `Move`, `Wheel`, `Enter`, `Leave`).

### Fields

| Name | Type | Description |
|---|---|---|
| `Action` | `Action` | `Press`, `Release`, `Move`, `Wheel`, `Enter`, or `Leave`. |
| `Pos` | `render.Point` | Logical px, in window space. |
| `Button` | `Button` | Set on `Press`/`Release` only. |
| `Delta` | `render.Point` | Scroll delta (x, y); set on `Wheel` only. |
| `Mods` | `Modifiers` | Keyboard modifier states. |
| `Target` | `core.Widget` | The hit leaf widget (or the captured widget, while one holds the grab). |
| `Router` | `*Router` | The router delivering this event, for `Capture`/`Focus` calls from handlers. |
| `Handled` | `bool` | Set by a handler to stop propagation up the bubble path. |

**Example**

```go
func (b *Button) OnPointer(e *input.PointerEvent) {
    switch e.Action {
    case input.Press:
        e.Router.Capture(b)
        e.Handled = true
    case input.Release:
        e.Router.Release()
        e.Handled = true
    }
}
```

**See also** — [PointerHandler](#pointerhandler), [Router.Capture](#routercapture), [Bubble](#bubble)

---

## KeyEvent

`KeyEvent` represents a keyboard event: the payload delivered to a
`KeyHandler`'s `OnKey` for `KeyDown`/`KeyUp`.

### Fields

| Name | Type | Description |
|---|---|---|
| `Action` | `Action` | `Press` or `Release`. |
| `Key` | `Key` | The keyboard key code. |
| `Rune` | `rune` | Character code for char-input events; else `0`. |
| `Mods` | `Modifiers` | Keyboard modifier states. |
| `Router` | `*Router` | The router delivering this event, for `Focus`/`FocusNext`/`FocusPrev` calls from handlers. |
| `Handled` | `bool` | Set by a handler to stop propagation up the parent chain. |

**Example**

```go
func (t *TextBox) OnKey(e *input.KeyEvent) {
    if e.Action == input.Press && e.Key == input.KeyEnter {
        t.submit()
        e.Handled = true
    }
}
```

**See also** — [KeyHandler](#keyhandler), [Router.KeyDown](#routerkeydown)

---

## PointerHandler

Optional interface for widgets that handle pointer events.

```go
type PointerHandler interface {
    OnPointer(e *PointerEvent)
}
```

A widget implements `PointerHandler` to receive `PointerEvent`s bubbled to
it by `Router` — via `Bubble` for `Press`/`Release`/`Move`/`Wheel`, or
delivered directly (not bubbled) for `Enter`/`Leave` and for capture
delivery.

**Example**

```go
func (w *MyWidget) OnPointer(e *input.PointerEvent) {
    if e.Action == input.Press {
        e.Handled = true
    }
}
```

**See also** — [PointerEvent](#pointerevent), [Bubble](#bubble), [Router.Capture](#routercapture)

---

## KeyHandler

Optional interface for widgets that handle keyboard events.

```go
type KeyHandler interface {
    OnKey(e *KeyEvent)
}
```

A widget implements `KeyHandler` to receive `KeyEvent`s bubbled to it by
`Router.KeyDown`/`Router.KeyUp` along the focused widget's `core.ParentOf`
ancestor chain.

**See also** — [KeyEvent](#keyevent), [Router.KeyDown](#routerkeydown), [Router.KeyUp](#routerkeyup)

---

## FocusHandler

Optional interface for widgets that react to focus changes.

```go
type FocusHandler interface {
    OnFocusChanged(focused bool)
}
```

`Router.Focus` calls `OnFocusChanged(false)` on a widget losing focus and
`OnFocusChanged(true)` on a widget gaining it, in that order.

**Notes** — Focus changes requested from within `OnFocusChanged` (via
`Focus`, `FocusNext`, or `FocusPrev`) are ignored — see [Router.Focus](#routerfocus)'s
reentrancy guard.

**See also** — [Focusable](#focusable), [Router.Focus](#routerfocus)

---

## Focusable

Optional interface for widgets that can accept keyboard focus.

```go
type Focusable interface {
    AcceptsFocus() bool
}
```

`Router` consults `AcceptsFocus()` in press-to-focus (`PointerButton`) and in
Tab traversal (`FocusNext`/`FocusPrev`): only widgets implementing
`Focusable` with `AcceptsFocus() == true` — and that are currently visible
(`core.IsVisible`) — are focus candidates.

**See also** — [FocusHandler](#focushandler), [Router.FocusNext](#routerfocusnext), [Router.FocusPrev](#routerfocusprev)

---

## CursorShaper

Optional interface for widgets that define a custom cursor.

```go
type CursorShaper interface {
    Cursor() Cursor
}
```

`Router.PointerMove` walks the hit-test path leaf→root and returns the first
`CursorShaper`'s `Cursor()`, or `CursorArrow` if none of the path's widgets
shape one. While a capture is active, only the captured widget's own
`Cursor()` (if it implements `CursorShaper`) is consulted.

**See also** — [Cursor](#cursor), [Router.PointerMove](#routerpointermove)

---

## Clipboard

`Clipboard` abstracts host-provided system clipboard access.

```go
type Clipboard interface {
    Get() string
    Set(s string)
}
```

Wired into a `Router` via `SetClipboard` so widgets (e.g. `TextBox`'s
Ctrl+C/X/V handling) can read/write the system clipboard without `input`
depending on any windowing package. A `Router` with no clipboard installed
returns nil from `Router.Clipboard`; callers must nil-check.

### Functions

| Function | Signature | Description |
|---|---|---|
| `Get` | `Get() string` | Returns the current clipboard text. |
| `Set` | `Set(s string)` | Sets the clipboard text. |

**See also** — [Router.SetClipboard](#routersetclipboard), [Router.Clipboard](#routerclipboard)

---

## HitPath

Returns the widget chain root→…→topmost leaf whose arranged bounds contain a
point.

**Syntax**

```go
func HitPath(root core.Widget, p render.Point) []core.Widget
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `root` | `core.Widget` | The subtree root to test. A nil root returns nil. |
| `p` | `render.Point` | The point to test, in the same (window) space as the widget tree's arranged bounds. |

**Returns** — `[]core.Widget`, the chain root→…→topmost leaf whose arranged
bounds contain `p`, or nil if the root itself misses.

**Example**

```go
path := input.HitPath(root, render.Point{X: 50, Y: 60})
if len(path) > 0 {
    leaf := path[len(path)-1]
}
```

**Notes** — Hidden widgets (`core.IsVisible` false) are skipped along with
their subtrees. Children are tested LAST-to-first (topmost painted wins). A
widget is on the path only if its own bounds contain `p` — the bounds gate
applies at every level, including the root: a point outside a widget's
bounds never reaches its children.

**See also** — [Bubble](#bubble), [Router.PointerMove](#routerpointermove)

---

## Bubble

Delivers a `PointerEvent` leaf→root along a hit-test path.

**Syntax**

```go
func Bubble(path []core.Widget, e *PointerEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `path` | `[]core.Widget` | The path to bubble along, root→leaf order (as returned by `HitPath`). A nil/empty path is a no-op. |
| `e` | `*PointerEvent` | The event to deliver. `e.Target` is set to the leaf (`path`'s last element) before delivery. |

**Notes** — Every widget on `path` that implements `PointerHandler` receives
`e` in leaf→root order (the same `*PointerEvent` instance is reused for the
whole walk, so a handler's `e.Handled` is visible to the loop), stopping as
soon as `e.Handled` is set. Exported so callers outside this package can
replay the same leaf→root delivery over a path they compute themselves —
e.g. `controls.OverlayHost` forwards captured pointer events into an open
popup's subtree via `HitPath(popup, e.Pos)` + `Bubble`, since it can't reach
`Router`'s private dispatch while it holds the pointer capture. `Router`'s
own dispatch (`PointerMove`/`PointerButton`/`PointerWheel`) uses this same
function internally; behavior is unchanged.

**Example**

```go
path := input.HitPath(popup, e.Pos)
input.Bubble(path, e)
```

**See also** — [HitPath](#hitpath), [PointerHandler](#pointerhandler)

---

## Constants reference

### Button

`Button` identifies a mouse button.

| Constant | Value | Description |
|---|---|---|
| `ButtonNone` | `0` | No button. |
| `ButtonLeft` | `1` | Left mouse button. |
| `ButtonRight` | `2` | Right mouse button. |
| `ButtonMiddle` | `3` | Middle mouse button. |

### Action

`Action` describes the event action for pointer and key events.

| Constant | Value | Description |
|---|---|---|
| `Press` | `0` | Button/key pressed down. |
| `Release` | `1` | Button/key released. |
| `Move` | `2` | Pointer moved (uncaptured or captured). |
| `Wheel` | `3` | Wheel/scroll input. |
| `Enter` | `4` | Pointer entered a widget's bounds (direct delivery, not bubbled). |
| `Leave` | `5` | Pointer left a widget's bounds (direct delivery, not bubbled). |

### Modifiers

`Modifiers` represents keyboard modifier states as bitflags — combine with
`|`, test with `&`.

| Constant | Value | Description |
|---|---|---|
| `ModShift` | `1` | Shift held. |
| `ModCtrl` | `2` | Ctrl held. |
| `ModAlt` | `4` | Alt held. |
| `ModSuper` | `8` | Super/Cmd/Win held. |

**Example**

```go
if e.Mods&input.ModCtrl != 0 && e.Key == input.KeyC {
    clip.Set(selectedText)
}
```

### Key

`Key` identifies a keyboard key. Values match GLFW keycodes numerically.
Named keys (`KeyEscape`…`KeyEnd`) use GLFW's 256+ numbering scheme;
printable/letter keys (`KeyT`, `KeySpace`, `KeyA`, `KeyC`, `KeyV`, `KeyX`,
`KeyY`, `KeyZ`) are instead numbered by ASCII code, since that's how GLFW
reports them.

| Constant | Value | Description |
|---|---|---|
| `KeyEscape` | `256` | Escape. |
| `KeyEnter` | `257` | Enter/Return. |
| `KeyTab` | `258` | Tab — drives `Router.FocusNext`/`FocusPrev` when unhandled. |
| `KeyBackspace` | `259` | Backspace. |
| `KeyDelete` | `261` | Delete. |
| `KeyRight` | `262` | Right arrow. |
| `KeyLeft` | `263` | Left arrow. |
| `KeyDown` | `264` | Down arrow. |
| `KeyUp` | `265` | Up arrow. |
| `KeyHome` | `268` | Home. |
| `KeyEnd` | `269` | End. |
| `KeySpace` | `32` | Space — used for Space-as-activate on focused controls. |
| `KeyA` | `65` | `A` — select-all (Ctrl+A). |
| `KeyC` | `67` | `C` — copy (Ctrl+C). |
| `KeyT` | `84` | `T` — the gallery's Light/Dark theme-toggle shortcut. |
| `KeyV` | `86` | `V` — paste (Ctrl+V). |
| `KeyX` | `88` | `X` — cut (Ctrl+X). |
| `KeyY` | `89` | `Y` — redo (Ctrl+Y). |
| `KeyZ` | `90` | `Z` — undo (Ctrl+Z). |

### Cursor

`Cursor` identifies a cursor shape, as returned by `Router.PointerMove` and
`CursorShaper.Cursor`.

| Constant | Value | Description |
|---|---|---|
| `CursorArrow` | `0` | Default arrow. |
| `CursorIBeam` | `1` | Text-input I-beam. |
| `CursorHand` | `2` | Hand/pointer (clickable). |
| `CursorHResize` | `3` | Horizontal resize. |
| `CursorVResize` | `4` | Vertical resize. |

**See also** — [CursorShaper](#cursorshaper), [Router.PointerMove](#routerpointermove)
