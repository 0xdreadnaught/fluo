# core

The `core` package is fluo's layout engine and widget foundation. Every
control satisfies `Widget` by embedding `Element`, which supplies
margin/size/alignment/visibility state and a zero-value-valid default
implementation of `MeasureContent`/`ArrangeContent`/`Render`/`Children` — a
bare `Element` embed is already a valid, empty, leaf widget. Layout runs as a
WPF-style two-pass model, driven exclusively through two package-level
functions: `MeasureWidget` (available space in, desired size out) and
`ArrangeWidget` (a final rect in, absolute bounds out). `RenderWidget` then
draws a widget and its children in the correct order, honoring the optional
`ClipProvider` and `OverlayRenderer` hook interfaces. `DesiredSizeOf`/
`BoundsOf`/`IsVisible`/`ParentOf` read back the layout results those entry
points computed, and `SetParent` records the parent/child edge containers use
for invalidation and ancestor walks. `Property[T]` is core's other pillar: a
reactive value whose `Set` notifies subscribers on real changes — the
primitive the `bind` package builds one-way and two-way binding on top of.
Every other fluo package (`controls`, `input`, `bind`, `app`) is built on the
`Widget`/`Element` contract and `Property` defined here.

**Import:** `github.com/0xdreadnaught/fluo/core`

## Contents
- [Widget](#widget)
- [Element](#element)
- [Property[T]](#propertyt)
- [Alignment](#alignment)
- [Auto](#auto)
- [ClipProvider](#clipprovider)
- [OverlayRenderer](#overlayrenderer)

---

## Widget

`Widget` is implemented by embedding `Element` (which supplies the unexported
`element()` method) and defining content behavior. Because embedding
`core.Element` promotes `element()`, any struct that embeds `Element` and
overrides zero or more of the content methods satisfies this interface —
external packages outside fluo can implement `Widget` the same way its own
controls do.

```go
type Widget interface {
	element() *Element
	MeasureContent(available render.Size) render.Size
	ArrangeContent(bounds render.Rect)
	Render(r render.Renderer)
	Children() []Widget
}
```

### Interface methods

| Method | Signature | Description |
|---|---|---|
| [Element.MeasureContent](#elementmeasurecontent) | `MeasureContent(available render.Size) render.Size` | Returns the content's desired size given the space available to it. |
| [Element.ArrangeContent](#elementarrangecontent) | `ArrangeContent(bounds render.Rect)` | Positions/arranges children within the widget's own absolute bounds. |
| [Element.Render](#elementrender) | `Render(r render.Renderer)` | Draws the widget itself only; children are drawn separately. |
| [Element.Children](#elementchildren) | `Children() []Widget` | Returns this widget's children, or nil for a leaf. |

`available` (to `MeasureContent`) may carry `+Inf` components; implementations
must be Inf-safe and must return a finite size. Embedders override these on
top of `Element`'s zero-behavior defaults — see [Element](#element) for the
default implementations and for the setters/state every widget inherits by
embedding it.

Widget itself deliberately exposes no getters for layout state; that's what
the package-level functions below are for.

### Functions

| Function | Signature | Description |
|---|---|---|
| [SetParent](#setparent) | `func SetParent(child, parent Widget)` | Records a widget's layout parent; panics on double-parenting. |
| [MeasureWidget](#measurewidget) | `func MeasureWidget(w Widget, available render.Size)` | Runs the measure pass: available space in, desired size out. |
| [ArrangeWidget](#arrangewidget) | `func ArrangeWidget(w Widget, final render.Rect)` | Runs the arrange pass: a final rect in, absolute bounds out. |
| [RenderWidget](#renderwidget) | `func RenderWidget(w Widget, r render.Renderer)` | Draws w and its children in the correct order. |
| [DesiredSizeOf](#desiredsizeof) | `func DesiredSizeOf(w Widget) render.Size` | Reads back the size computed by the last MeasureWidget call. |
| [BoundsOf](#boundsof) | `func BoundsOf(w Widget) render.Rect` | Reads back the bounds computed by the last ArrangeWidget call. |
| [IsVisible](#isvisible) | `func IsVisible(w Widget) bool` | Reports whether w participates in layout and rendering. |
| [ParentOf](#parentof) | `func ParentOf(w Widget) Widget` | Returns w's layout parent, as last recorded by SetParent. |

#### SetParent

Records `parent` as `child`'s layout parent. Container widgets call this from
their `Add` (or equivalent) methods.

**Syntax**

```go
func SetParent(child, parent Widget)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `child` | `Widget` | The widget being attached to (or detached from) a parent. |
| `parent` | `Widget` | The new parent. `nil` detaches `child`: its future invalidations no longer climb into any ancestor. |

**Example**

```go
func (p *StackPanel) Add(w core.Widget) {
	p.children = append(p.children, w)
	core.SetParent(w, p)
}
```

**Notes** — Fail-fast double-parenting guard: if `parent` is non-nil and
`child` already has a different non-nil parent, `SetParent` panics rather
than silently re-homing the child (which would leave the old parent's child
slice pointing at a widget that no longer reports it as an ancestor). Detach
first with `SetParent(child, nil)` before re-adding elsewhere. Re-setting the
same parent is a no-op; setting `nil` is always allowed.

**See also** — [ParentOf](#parentof), [Element.InvalidateMeasure](#elementinvalidatemeasure)

#### MeasureWidget

The only entry point that runs a widget's measure pass.

**Syntax**

```go
func MeasureWidget(w Widget, available render.Size)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `w` | `Widget` | The widget to measure. Takes the interface (not `*Element`) so the concrete widget's overridden `MeasureContent` is the one invoked. |
| `available` | `render.Size` | The space available to `w`, before margins/explicit size/min-max are applied. May carry `+Inf` components. |

Computes `w`'s desired size and stores it (readable back via
[DesiredSizeOf](#desiredsizeof)), following these steps:

1. A hidden widget (`Visible() == false`) measures to a zero size and stops here.
2. `inner` = `available` minus the widget's margin.
3. An explicit width/height (set via `SetWidth`/`SetHeight`) wins over `inner` on that axis — that's the space offered to the widget's own content.
4. `inner` is clamped to `[min, max]` per axis (`SetMinSize`/`SetMaxSize`).
5. `w.MeasureContent(inner)` is called to get the content's desired size.
6. An explicit width/height wins again on the *resulting* content size, then it is re-clamped to `[min, max]`.
7. The final desired size is content size plus margins.

**Returns** — nothing; results are read back with [DesiredSizeOf](#desiredsizeof).

**Example**

```go
core.MeasureWidget(root, render.Size{W: 800, H: 600})
size := core.DesiredSizeOf(root)
```

**See also** — [ArrangeWidget](#arrangewidget), [Element.MeasureContent](#elementmeasurecontent), [DesiredSizeOf](#desiredsizeof)

#### ArrangeWidget

The only entry point that runs a widget's arrange pass.

**Syntax**

```go
func ArrangeWidget(w Widget, final render.Rect)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `w` | `Widget` | The widget to arrange. Takes the interface for the same dispatch reason as `MeasureWidget`. |
| `final` | `render.Rect` | The final rect the parent has allocated to `w`, in absolute coordinates. |

Computes `w`'s absolute bounds (readable back via [BoundsOf](#boundsof)) and
recursively arranges its content:

1. A hidden widget arranges to a zero rect and stops here.
2. `slot` = `final` inset by the widget's margin.
3. The content's desired size (from the prior `MeasureWidget` call) is reduced by the margin per axis, then capped to fit within `slot`.
4. Per axis: if alignment is `Stretch` and no explicit size was set, the widget fills the whole slot on that axis; otherwise it sizes to its (capped) desired content size and is positioned within the slot per `Start`/`Center`/`End`/`Stretch`-with-explicit-size (`Stretch` with an explicit size centers, like `Center`).
5. The resulting rect becomes `w`'s absolute bounds; `w.ArrangeContent(bounds)` is called so the widget can position its own children within it.

**Returns** — nothing; results are read back with [BoundsOf](#boundsof).

**Example**

```go
core.ArrangeWidget(root, render.Rect{X: 0, Y: 0, W: 800, H: 600})
bounds := core.BoundsOf(root)
```

**See also** — [MeasureWidget](#measurewidget), [Element.ArrangeContent](#elementarrangecontent), [BoundsOf](#boundsof), [Alignment](#alignment)

#### RenderWidget

Draws `w` and, in order, its children. Hidden widgets (and their entire
subtree) are skipped.

**Syntax**

```go
func RenderWidget(w Widget, r render.Renderer)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `w` | `Widget` | The widget (and subtree) to draw. |
| `r` | `render.Renderer` | The renderer to draw into. |

Full draw order: `w.Render(r)` → `[PushClip` if `w` is a
[ClipProvider](#clipprovider) with `ok == true]` → children (each recursively
via `RenderWidget`) → `[PopClip`, matching the push above`]` →
`[w.RenderOverlay(r)` if `w` is an [OverlayRenderer](#overlayrenderer)`]`.
Clipping wraps only the children; the widget's own `Render` and any
`RenderOverlay` draw unclipped, so a widget can always paint its own chrome
(and any overlay adornments) outside the region it clips its content to.

**Returns** — nothing.

**Example**

```go
core.RenderWidget(root, renderer)
```

**See also** — [ClipProvider](#clipprovider), [OverlayRenderer](#overlayrenderer), [Element.Render](#elementrender)

#### DesiredSizeOf

Returns `w`'s desired size as computed by the last `MeasureWidget` call.

**Syntax**

```go
func DesiredSizeOf(w Widget) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `w` | `Widget` | The widget to read the measured size of. |

**Returns** — `render.Size`. This is how parents (and external custom panels)
read a child's measurement back — `Widget` itself deliberately exposes no
getters.

**Example**

```go
size := core.DesiredSizeOf(child)
```

**See also** — [MeasureWidget](#measurewidget), [BoundsOf](#boundsof)

#### BoundsOf

Returns `w`'s arranged bounds in absolute window space, as computed by the
last `ArrangeWidget` call.

**Syntax**

```go
func BoundsOf(w Widget) render.Rect
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `w` | `Widget` | The widget to read the arranged bounds of. |

**Returns** — `render.Rect`. Valid after `ArrangeWidget` has run; like
`DesiredSizeOf`, it exists so parents (and external custom panels) can read a
child's layout result back without `Widget` itself exposing getters.

**Example**

```go
bounds := core.BoundsOf(child)
```

**See also** — [ArrangeWidget](#arrangewidget), [DesiredSizeOf](#desiredsizeof)

#### IsVisible

Reports whether `w` participates in layout and rendering.

**Syntax**

```go
func IsVisible(w Widget) bool
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `w` | `Widget` | The widget to check. |

**Returns** — `bool`. Parents use it to decide gap/extent contribution for
hidden children.

**Example**

```go
if core.IsVisible(child) {
	visibleCount++
}
```

**See also** — [Element.SetVisible](#elementsetvisible), [Element.Visible](#elementvisible)

#### ParentOf

Returns `w`'s layout parent, as last recorded by `SetParent`, or `nil` if `w`
has none.

**Syntax**

```go
func ParentOf(w Widget) Widget
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `w` | `Widget` | The widget to find the parent of. |

**Returns** — `Widget`, or `nil` — either because `w` was never added to a
container (a root) or because it was explicitly detached via
`SetParent(w, nil)`. Used by `input.Router` to walk the ancestor chain for
keyboard-event bubbling.

**Example**

```go
for p := core.ParentOf(w); p != nil; p = core.ParentOf(p) {
	// walk toward the root
}
```

**See also** — [SetParent](#setparent)

---

## Element

`Element` is the embeddable base of every widget. Its **zero value is a
valid widget state**: visible, auto-sized, `Stretch`-aligned, dirty (needs
layout) — no constructor is required or provided. A bare `Element` embed with
no overrides is already a valid, empty, leaf widget, since `Element` supplies
default implementations of all four `Widget` content methods.

**Zero-value notes** (from the source doc comment):
- Width/height: only a value `> 0` counts as an explicit size. Both the zero
  value (`0`) and [Auto](#auto) (`-1`) mean "size to content".
- Max width/height: `<= 0` means "no maximum" (treated as `+Inf`). Min
  width/height have no such special-casing; their zero value (`0`) is simply
  "no minimum".
- Hidden: zero value `false`, so a fresh `Element` is visible.
- Horizontal/vertical alignment: zero value `Stretch` (`iota` `0`).
- The measure/arrange "clean" flags are stored **inverted** so that the zero
  value (`false`) means "dirty" — a fresh `Element` always reports
  `NeedsLayout() == true` without any constructor call.

**Example**

```go
type Badge struct {
	core.Element
	label string
}

func (b *Badge) MeasureContent(available render.Size) render.Size {
	return render.Size{W: 60, H: 20}
}

func (b *Badge) Render(r render.Renderer) {
	r.FillRect(core.BoundsOf(b), render.RGB(200, 40, 40))
}
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [MeasureContent](#elementmeasurecontent) | `MeasureContent(available render.Size) render.Size` | Default: returns a zero size. Override to report content's desired size. |
| [ArrangeContent](#elementarrangecontent) | `ArrangeContent(bounds render.Rect)` | Default: does nothing. Override to position children. |
| [Render](#elementrender) | `Render(r render.Renderer)` | Default: does nothing. Override to draw the widget itself. |
| [Children](#elementchildren) | `Children() []Widget` | Default: returns nil (a leaf widget). Override to expose children. |
| [SetMargin](#elementsetmargin) | `SetMargin(t render.Thickness)` | Sets the outer margin. |
| [SetWidth](#elementsetwidth) | `SetWidth(w float32)` | Fixes the width axis, or sets it to auto. |
| [SetHeight](#elementsetheight) | `SetHeight(h float32)` | Fixes the height axis, or sets it to auto. |
| [SetMinSize](#elementsetminsize) | `SetMinSize(w, h float32)` | Sets the minimum content size per axis. |
| [SetMaxSize](#elementsetmaxsize) | `SetMaxSize(w, h float32)` | Sets the maximum content size per axis. |
| [SetAlign](#elementsetalign) | `SetAlign(h, v Alignment)` | Sets horizontal/vertical alignment. |
| [SetVisible](#elementsetvisible) | `SetVisible(v bool)` | Toggles visibility. |
| [Visible](#elementvisible) | `Visible() bool` | Reports whether the widget is visible. |
| [DesiredSize](#elementdesiredsize) | `DesiredSize() render.Size` | Returns the size computed by the last MeasureWidget call. |
| [Bounds](#elementbounds) | `Bounds() render.Rect` | Returns the absolute rect computed by the last ArrangeWidget call. |
| [InvalidateMeasure](#elementinvalidatemeasure) | `InvalidateMeasure()` | Marks this element (and, transitively, its ancestors) dirty for measure and arrange. |
| [InvalidateArrange](#elementinvalidatearrange) | `InvalidateArrange()` | Marks this element (and its ancestors) dirty for arrange only. |
| [NeedsLayout](#elementneedslayout) | `NeedsLayout() bool` | Reports whether this element requires a measure and/or arrange pass. |

**Notes** — Unlike the builder methods elsewhere in fluo, `Element`'s setters
return nothing (no chaining); a widget's own `SetX` methods (e.g.
`Button.SetText`) typically wrap these and return `*Button` for chaining
instead.

#### Element.MeasureContent

Returns the content's desired size given the space available to it (already
inset by margins/explicit size/min-max). Default implementation returns a
zero size; embedders override this to report the size their content
actually wants.

**Syntax**

```go
func (e *Element) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | Space available to the content, already inset. May carry `+Inf` components; implementations must be Inf-safe and must return a finite size. |

**Returns** — `render.Size`, the content's desired size.

**See also** — [MeasureWidget](#measurewidget)

#### Element.ArrangeContent

Positions/arranges children within the widget's own absolute bounds (already
computed by the engine). Default implementation does nothing; embedders
override this to position their children within the bounds `ArrangeWidget`
computed.

**Syntax**

```go
func (e *Element) ArrangeContent(bounds render.Rect)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `bounds` | `render.Rect` | The widget's own absolute bounds, as just computed by `ArrangeWidget`. |

**See also** — [ArrangeWidget](#arrangewidget)

#### Element.Render

Draws the widget itself only; children are drawn separately by
`RenderWidget`. Default implementation does nothing.

**Syntax**

```go
func (e *Element) Render(r render.Renderer)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | The renderer to draw into. |

**See also** — [RenderWidget](#renderwidget)

#### Element.Children

Returns this widget's children, or nil for a leaf widget. Default
implementation returns nil, marking a bare `Element` embed as a leaf;
embedders override this to expose their children.

**Syntax**

```go
func (e *Element) Children() []Widget
```

**Returns** — `[]Widget`, or `nil` for a leaf.

**See also** — [RenderWidget](#renderwidget)

#### Element.SetMargin

Sets the outer margin.

**Syntax**

```go
func (e *Element) SetMargin(t render.Thickness)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `t` | `render.Thickness` | The outer margin on all four sides. |

**Returns** — nothing.

**Example**

```go
badge.SetMargin(render.Uniform(4))
```

**Notes** — Layout-relevant: invalidates measure.

**See also** — [Element.InvalidateMeasure](#elementinvalidatemeasure)

#### Element.SetWidth

Fixes the width axis when `w > 0`; `w <= 0` means auto (size to content).

**Syntax**

```go
func (e *Element) SetWidth(w float32)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `w` | `float32` | Explicit width in pixels, or `<= 0` (including [Auto](#auto)) for auto-sizing. |

**Returns** — nothing.

**Example**

```go
badge.SetWidth(120)
```

**Notes** — Invalidates measure.

**See also** — [Element.SetHeight](#elementsetheight), [Auto](#auto)

#### Element.SetHeight

Fixes the height axis when `h > 0`; `h <= 0` means auto.

**Syntax**

```go
func (e *Element) SetHeight(h float32)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `h` | `float32` | Explicit height in pixels, or `<= 0` (including [Auto](#auto)) for auto-sizing. |

**Returns** — nothing.

**Notes** — Invalidates measure.

**See also** — [Element.SetWidth](#elementsetwidth), [Auto](#auto)

#### Element.SetMinSize

Sets the minimum content size per axis.

**Syntax**

```go
func (e *Element) SetMinSize(w, h float32)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `w` | `float32` | Minimum content width. Zero value is "no minimum". |
| `h` | `float32` | Minimum content height. Zero value is "no minimum". |

**Returns** — nothing.

**Notes** — Invalidates measure.

**See also** — [Element.SetMaxSize](#elementsetmaxsize)

#### Element.SetMaxSize

Sets the maximum content size per axis; `<= 0` means unbounded.

**Syntax**

```go
func (e *Element) SetMaxSize(w, h float32)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `w` | `float32` | Maximum content width. `<= 0` means no maximum (treated as `+Inf`). |
| `h` | `float32` | Maximum content height. `<= 0` means no maximum (treated as `+Inf`). |

**Returns** — nothing.

**Notes** — Invalidates measure.

**See also** — [Element.SetMinSize](#elementsetminsize)

#### Element.SetAlign

Sets horizontal/vertical alignment.

**Syntax**

```go
func (e *Element) SetAlign(h, v Alignment)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `h` | `Alignment` | Horizontal alignment within the slot the parent allocates. |
| `v` | `Alignment` | Vertical alignment within the slot the parent allocates. |

**Returns** — nothing.

**Example**

```go
badge.SetAlign(core.End, core.Start)
```

**Notes** — Purely an arrange-time concern, so it only invalidates arrange
(not measure).

**See also** — [Alignment](#alignment), [Element.InvalidateArrange](#elementinvalidatearrange)

#### Element.SetVisible

Toggles visibility.

**Syntax**

```go
func (e *Element) SetVisible(v bool)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `bool` | `true` to show, `false` to hide. |

**Returns** — nothing.

**Notes** — Either direction changes what the parent sees during its own
measure pass (a hidden widget measures to zero), so both transitions
invalidate measure, which bubbles to the parent chain.

**See also** — [Element.Visible](#elementvisible), [IsVisible](#isvisible)

#### Element.Visible

Reports whether the widget is visible.

**Syntax**

```go
func (e *Element) Visible() bool
```

**Returns** — `bool`.

**See also** — [Element.SetVisible](#elementsetvisible), [IsVisible](#isvisible)

#### Element.DesiredSize

Returns the size computed by the last `MeasureWidget` call.

**Syntax**

```go
func (e *Element) DesiredSize() render.Size
```

**Returns** — `render.Size`.

**See also** — [DesiredSizeOf](#desiredsizeof), [MeasureWidget](#measurewidget)

#### Element.Bounds

Returns the absolute rect computed by the last `ArrangeWidget` call.

**Syntax**

```go
func (e *Element) Bounds() render.Rect
```

**Returns** — `render.Rect`.

**See also** — [BoundsOf](#boundsof), [ArrangeWidget](#arrangewidget)

#### Element.InvalidateMeasure

Marks this element (and, transitively, its ancestors) as needing both
re-measure and re-arrange.

**Syntax**

```go
func (e *Element) InvalidateMeasure()
```

**Returns** — nothing.

**Notes** — The walk up the parent chain stops as soon as it reaches an
ancestor that was already measure-dirty, since that ancestor's own ancestors
must already be dirty too — an O(depth-to-first-dirty-ancestor) walk, not a
full-tree walk.

**See also** — [Element.InvalidateArrange](#elementinvalidatearrange), [Element.NeedsLayout](#elementneedslayout)

#### Element.InvalidateArrange

Marks this element (and its ancestors) as needing re-arrange only.

**Syntax**

```go
func (e *Element) InvalidateArrange()
```

**Returns** — nothing.

**Notes** — Stops climbing once it reaches an already arrange-dirty
ancestor, same short-circuit as `InvalidateMeasure`.

**See also** — [Element.InvalidateMeasure](#elementinvalidatemeasure), [Element.NeedsLayout](#elementneedslayout)

#### Element.NeedsLayout

Reports whether this element requires a measure and/or arrange pass.

**Syntax**

```go
func (e *Element) NeedsLayout() bool
```

**Returns** — `bool` — `true` if either the measure or the arrange flag is
dirty.

**See also** — [Element.InvalidateMeasure](#elementinvalidatemeasure), [Element.InvalidateArrange](#elementinvalidatearrange)

---

## Property[T]

`Property[T]` is a reactive value: `Set` notifies subscribers on real
changes. Not goroutine-safe; intended for single-threaded UI loops. There is
no constructor — the zero value (`var p core.Property[string]`, or as a
struct field) is ready to use.

**Example**

```go
type Model struct {
	Name core.Property[string]
}

m := &Model{}
cancel := m.Name.OnChange(func(old, new string) {
	fmt.Println("name changed:", old, "->", new)
})
m.Name.Set("Ada")
cancel()
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [Get](#propertytget) | `Get() T` | Returns the current value. |
| [Set](#propertytset) | `Set(v T)` | Assigns `v` if it differs from the current value and notifies all subscribers. |
| [OnChange](#propertytonchange) | `OnChange(f func(old, new T)) func()` | Registers a subscriber; returns an idempotent cancel function. |

#### Property[T].Get

Returns the current value.

**Syntax**

```go
func (p *Property[T]) Get() T
```

**Returns** — `T`.

**See also** — [Property[T].Set](#propertytset)

#### Property[T].Set

Assigns `v` if it differs from the current value and notifies all
subscribers.

**Syntax**

```go
func (p *Property[T]) Set(v T)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `T` | The new value. `T` must satisfy `comparable`. |

**Returns** — nothing.

**Example**

```go
p.Set(42)
```

**Notes** — If `v` equals the current value, this is a no-op: no assignment,
no notification. The order in which multiple subscribers are notified during
a single `Set` is unspecified. Canceling a subscription (calling the func
returned by `OnChange`) during a notification is safe. Subscribing a new
callback during a notification may or may not be observed by that same
in-flight notification. A reentrant `Set` from within a callback is allowed;
the outer notification simply continues delivering the old/new value pair it
started with, unaffected by the reentrant call.

`Property[T]` itself draws no distinction between a "programmatic" and a
"user-driven" `Set` — every real change notifies every subscriber
unconditionally. The library-wide **silent-setter convention** documented on
`controls` widgets (a control's own `SetX` methods, e.g. `SetChecked`, never
fire that control's `OnChanged` callback; only user interaction does) is a
separate, control-layer pattern built independently on top of this
primitive — see `bind`'s package doc for how one-way/two-way binding composes
the two so a model push and a user edit never echo each other.

**See also** — [Property[T].OnChange](#propertytonchange), [Property[T].Get](#propertytget)

#### Property[T].OnChange

Registers a subscriber to be called when the value changes.

**Syntax**

```go
func (p *Property[T]) OnChange(f func(old, new T)) func()
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `f` | `func(old, new T)` | Called with the previous and new value on every real (value-changing) `Set`. |

**Returns** — `func()`, a cancel function that removes the subscriber.
Idempotent — calling it more than once is a harmless no-op.

**Example**

```go
cancel := p.OnChange(func(old, new int) {
	fmt.Println(old, "->", new)
})
defer cancel()
```

**See also** — [Property[T].Set](#propertytset)

---

## Alignment

`Alignment` controls how a widget is positioned (and possibly sized) within
the slot its parent allocates to it along one axis. Underlying type `uint8`.

| Name | Value | Description |
|---|---|---|
| `Stretch` | `0` (`iota`) | Fills the entire slot on this axis, unless an explicit width/height is set, in which case it centers instead. Default (zero value). |
| `Start` | `1` | Aligns the widget to the slot's near edge (left/top) at its desired size. |
| `Center` | `2` | Centers the widget within the slot at its desired size. |
| `End` | `3` | Aligns the widget to the slot's far edge (right/bottom) at its desired size. |

**Example**

```go
badge.SetAlign(core.Center, core.Stretch)
```

**See also** — [Element.SetAlign](#elementsetalign), [ArrangeWidget](#arrangewidget)

---

## Auto

`Auto` is the sentinel meaning "size to content" for width/height.

**Syntax**

```go
const Auto float32 = -1
```

**Example**

```go
badge.SetWidth(core.Auto)
```

**Notes** — The zero value of a `float32` (`0`) also means auto (see the
zero-value notes on [Element](#element)); `Auto` exists purely for
readability at call sites.

**See also** — [Element.SetWidth](#elementsetwidth), [Element.SetHeight](#elementsetheight)

---

## ClipProvider

Optional interface for widgets that clip their children's rendering to a
rectangle (e.g. `ScrollViewer` clipping its scrolled content to its
viewport).

```go
type ClipProvider interface{ ClipRect() (render.Rect, bool) }
```

**Returns** (from `ClipRect`) — the rectangle to clip to, and whether
clipping should actually be applied this frame. Returning `ok == false` (or
not implementing the interface at all) renders children unclipped.

**Example**

```go
func (v *ScrollViewer) ClipRect() (render.Rect, bool) {
	return core.BoundsOf(v), true
}
```

**See also** — [RenderWidget](#renderwidget), [OverlayRenderer](#overlayrenderer)

---

## OverlayRenderer

Optional interface for widgets that draw content above their children —
scrollbars, adorners, selection handles, and the like.

```go
type OverlayRenderer interface{ RenderOverlay(r render.Renderer) }
```

`RenderOverlay` runs after children (and after any `PopClip`), so overlays
are never clipped by the widget's own `ClipRect`.

**Example**

```go
func (v *ScrollViewer) RenderOverlay(r render.Renderer) {
	drawScrollThumb(r, v.thumbBounds(), theme.Active().Color)
}
```

**See also** — [RenderWidget](#renderwidget), [ClipProvider](#clipprovider)
