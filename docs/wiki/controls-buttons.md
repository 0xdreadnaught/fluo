# Button controls

The button family covers every fluo control built around a single click/tap
gesture: `Button` (a push button, optionally shaped as a pill or circle),
`ToggleButton` (a `Button` that latches a boolean on click), `CheckBox`,
`RadioButton` + `RadioGroup` (mutually-exclusive radio selection), and
`ToggleSwitch` (a track-and-knob on/off control). All five are
`ClickBehavior`-driven — they embed the same press/hover/release state
machine rather than reimplementing pointer handling — and all follow fluo's
uniform contract: programmatic setters (`SetChecked`, `SetEnabled`, ...) are
silent, while `OnChanged` fires only for user-driven changes (a click, or
Space/Enter while focused). Reach for this package whenever you need a
clickable control, a two-state toggle, or an exclusive-choice group.

**Import:** `github.com/0xdreadnaught/fluo/controls`

## Contents
- [Button](#button)
- [ButtonShape](#buttonshape)
- [ToggleButton](#togglebutton)
- [CheckBox](#checkbox)
- [RadioButton](#radiobutton)
- [RadioGroup](#radiogroup)
- [ToggleSwitch](#toggleswitch)
- [ClickBehavior](#clickbehavior)

---

## Button

`Button` is a clickable, focusable, token-styled push button showing a text
label. It is a composite widget: its own `Render` paints the fill/stroke
chrome (varying by accent/hover/pressed/disabled state), `RenderOverlay`
paints the focus ring while focused, and its label `TextBlock` is arranged
centered within the padded content rect. Colors and metrics are captured from
`theme.Active()` at construction — rebuild to re-theme, matching every other
control in this package.

**Constructor**

```go
func NewButton(face *text.Face, label string) *Button
```

Returns an enabled, non-accent `Button` showing `label` in `face` (`face` may
be `nil`, per `TextBlock`).

**Example**

```go
ok := controls.NewButton(face, "OK")
ok.OnClick(func() {
    fmt.Println("clicked")
})
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [OnClick](#buttononclick) | `OnClick(fn func()) *Button` | Sets the callback fired on a successful click or Space/Enter. |
| [SetAccent](#buttonsetaccent) | `SetAccent(a bool) *Button` | Marks/unmarks this button as the default button. |
| [SetShape](#buttonsetshape) | `SetShape(s ButtonShape) *Button` | Sets the button's outer silhouette. |
| [SetEnabled](#buttonsetenabled) | `SetEnabled(v bool) *Button` | Toggles whether the button accepts focus and input. |
| [Label](#buttonlabel) | `Label() *TextBlock` | Returns the label `TextBlock`. |
| [SetAnimated](#buttonsetanimated) | `SetAnimated(v bool) *Button` | Opts into cross-fading fill transitions. |
| [SetTimers](#buttonsettimers) | `SetTimers(q *timers.Queue) *Button` | Wires the driver for the animated fill cross-fade. |
| [MeasureContent](#buttonmeasurecontent) | `MeasureContent(available render.Size) render.Size` | Measures the label plus padding (forced square for `ShapeCircle`). |
| [ArrangeContent](#buttonarrangecontent) | `ArrangeContent(bounds render.Rect)` | Arranges the label centered within the padded bounds. |
| [Render](#buttonrender) | `Render(r render.Renderer)` | Paints the button's chrome and label state. |
| [RenderOverlay](#buttonrenderoverlay) | `RenderOverlay(r render.Renderer)` | Paints the focus ring while focused. |
| [Children](#buttonchildren) | `Children() []core.Widget` | Returns the label as the button's sole child. |
| [AcceptsFocus](#buttonacceptsfocus) | `AcceptsFocus() bool` | Reports whether the button can take focus. |
| [OnFocusChanged](#buttononfocuschanged) | `OnFocusChanged(focused bool)` | Tracks focus for the ring overlay and keyboard activation. |
| [OnPointer](#buttononpointer) | `OnPointer(e *input.PointerEvent)` | Routes pointer events to the embedded `ClickBehavior`. |
| [OnKey](#buttononkey) | `OnKey(e *input.KeyEvent)` | Activates the button on Space/Enter while focused. |

#### Button.OnClick

Sets the callback fired on a successful click.

**Syntax**

```go
func (b *Button) OnClick(fn func()) *Button
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `fn` | `func()` | Called on a pointer press-release-inside, or Space/Enter while focused. Replaces any previously set callback. `nil` is a valid, silent no-op. |

**Returns** — `*Button` for chaining.

**Example**

```go
b.OnClick(func() { fmt.Println("clicked") })
```

**See also** — [ClickBehavior.Activate](#clickbehavioractivate), [ToggleButton](#togglebutton)

#### Button.SetAccent

Marks (or unmarks) this button as the default button.

**Syntax**

```go
func (b *Button) SetAccent(a bool) *Button
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `a` | `bool` | `true` draws the same raised/sunken `ButtonFace` bevel as any other button, plus an extra outer 1px `ButtonDarkShadow` border just outside the bevel — the classic "default button" marker. |

**Returns** — `*Button` for chaining.

**Example**

```go
b.SetAccent(true)
```

**Notes** — Purely visual; no invalidation needed since the host redraws
every frame.

**See also** — [ButtonShape](#buttonshape)

#### Button.SetShape

Sets the button's outer silhouette.

**Syntax**

```go
func (b *Button) SetShape(s ButtonShape) *Button
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `s` | `ButtonShape` | The silhouette to render; see [ButtonShape](#buttonshape). Default `ShapeRect`. |

**Returns** — `*Button` for chaining.

**Example**

```go
b.SetShape(controls.ShapePill)
```

**Notes** — Switching to/from `ShapeCircle` changes what `MeasureContent`
returns (a forced square aspect), so this invalidates measure exactly when
the shape actually changes — a no-op call (same shape) leaves layout
untouched. Switching between `ShapeRect`/`ShapePill` never affects desired
size (only how `Render` paints the chrome), but is still routed through the
same changed-check for one simple rule rather than special-casing which
shape pairs need it.

**See also** — [ButtonShape](#buttonshape), [Button.MeasureContent](#buttonmeasurecontent)

#### Button.SetEnabled

Toggles whether the button accepts focus and pointer/keyboard input.

**Syntax**

```go
func (b *Button) SetEnabled(v bool) *Button
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `bool` | `false` disables the button: it stops accepting focus (`AcceptsFocus`) and ignores pointer input outright (`OnPointer`). |

**Returns** — `*Button` for chaining.

**Example**

```go
b.SetEnabled(false)
```

**Notes** — Disabling a currently-focused button does not itself clear
router focus (the button has no router reference) — callers that need that
must clear focus explicitly; a documented v0 simplification. Purely visual
otherwise: no invalidation needed.

#### Button.Label

Returns the button's label `TextBlock`.

**Syntax**

```go
func (b *Button) Label() *TextBlock
```

**Returns** — `*TextBlock`, for tests and customization (e.g. overriding its
color).

**Example**

```go
b.Label().SetColor(render.RGB(200, 0, 0))
```

#### Button.SetAnimated

Opts this button into cross-fading its fill transitions.

**Syntax**

```go
func (b *Button) SetAnimated(v bool) *Button
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `bool` | `true` cross-fades rest/hover/pressed/disabled fill transitions over ~120ms (EaseOut) instead of snapping — PROVIDED a `timers.Queue` has also been wired via `SetTimers`. Default `false`. |

**Returns** — `*Button` for chaining.

**Example**

```go
b.SetAnimated(true).SetTimers(tq)
```

**Notes** — Matches fluo's opt-in-animation convention: every `Button` built
before this feature existed renders with today's exact snap-to-state colors,
byte-identical, so no existing test or golden needs to change. Purely
visual: no invalidation needed — the tween is driven by `timerQueue`'s own
`Advance`, and the host redraws every frame regardless.

**See also** — [Button.SetTimers](#buttonsettimers)

#### Button.SetTimers

Wires the driver for this button's animated fill cross-fades.

**Syntax**

```go
func (b *Button) SetTimers(q *timers.Queue) *Button
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `q` | `*timers.Queue` | The queue driving the cross-fade tween. Has no effect unless `SetAnimated(true)` is also set. `nil` detaches any previously wired queue, reverting to the instant (snap) fallback even with `SetAnimated(true)` still set. |

**Returns** — `*Button` for chaining.

**Example**

```go
b.SetTimers(tq)
```

**Notes** — A button that is `SetAnimated(true)` but never had `SetTimers`
called (`timerQueue` stays `nil`) behaves exactly like an unanimated one:
instant, current behavior. Caveat: swapping to a *different* non-nil queue
while a fade is already in flight does not redirect that in-flight tween —
it keeps ticking on the queue it was started on until it settles, even
though the button's queue reference now points elsewhere.

**See also** — [Button.SetAnimated](#buttonsetanimated)

#### Button.MeasureContent

Measures the label plus padding, forcing a square result for `ShapeCircle`.

**Syntax**

```go
func (b *Button) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | Space offered by the parent layout pass. |

**Returns** — `render.Size`: the label measured within `available` reduced
by padding (`PaddingL` horizontal, `PaddingM` vertical), then padding added
back. For `ShapeCircle` only, the result is additionally forced square
(`side = max(paddedWidth, paddedHeight)`) so the circle (radius =
`min(W,H)/2`) fully encloses the label; `ShapeRect` and `ShapePill` keep the
natural content-shaped rect unchanged.

**Notes** — Part of the `core.Widget` interface; invoked by
`core.MeasureWidget` during layout, not normally called directly by
application code.

**See also** — [ButtonShape](#buttonshape), [Button.ArrangeContent](#buttonarrangecontent)

#### Button.ArrangeContent

Arranges the label centered within the padded bounds.

**Syntax**

```go
func (b *Button) ArrangeContent(bounds render.Rect)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `bounds` | `render.Rect` | The button's final arranged rect from its parent layout pass. |

**Notes** — Centering (rather than filling) matters whenever the button ends
up wider/taller than its own desired size, e.g. stretched by a parent panel.
The computed rect is cached for `Render`'s press-nudge. Part of the
`core.Widget` interface; invoked by `core.ArrangeWidget`, not normally called
directly.

**See also** — [Button.MeasureContent](#buttonmeasurecontent)

#### Button.Render

Paints the button's classic chiseled chrome and label.

**Syntax**

```go
func (b *Button) Render(r render.Renderer)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | The target renderer for this frame. |

**Notes** — Chrome: normal = raised `ButtonFace`; hover (not pressed/sunken)
= raised `ButtonLight`; pressed, or a checked `ToggleButton`, = sunken
`ButtonFace` plus a +1,+1 label nudge (the classic press squish). A plain
`Button` with `SetAccent(true)` additionally gets a 1px `ButtonDarkShadow`
border just outside its raised bevel. Label: `WindowText` normally; disabled
draws a classic engrave (the glyphs once in `ButtonHighlight` offset +1,+1,
then again in `GrayText` at the nominal position). Part of the `core.Widget`
interface; invoked by `core.RenderWidget`, not normally called directly.

**See also** — [ButtonShape](#buttonshape), [Button.RenderOverlay](#buttonrenderoverlay)

#### Button.RenderOverlay

Draws the focus ring while focused.

**Syntax**

```go
func (b *Button) RenderOverlay(r render.Renderer)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | The target renderer for this frame. |

**Notes** — Inset a few px from the button's bounds so it sits within the
raised/sunken bevel rather than overlapping it. `ShapeRect` draws the
classic square focus rect; pill/circle instead draw a rounded ring following
the button's own curve. Implements `core.OverlayRenderer`; invoked by
`core.RenderWidget` after children render, not normally called directly.

**See also** — [ClickBehavior](#clickbehavior)

#### Button.Children

Returns the label as the button's sole child.

**Syntax**

```go
func (b *Button) Children() []core.Widget
```

**Returns** — `[]core.Widget` containing exactly the label `TextBlock`.

**Notes** — Part of the `core.Widget` interface; invoked by the layout/render
walk, not normally called directly.

#### Button.AcceptsFocus

Reports whether the button can currently take focus.

**Syntax**

```go
func (b *Button) AcceptsFocus() bool
```

**Returns** — `bool`, `true` unless the button is disabled.

**Notes** — Implements `input.Focusable`: a disabled button never accepts
focus.

#### Button.OnFocusChanged

Tracks focus for the ring overlay and keyboard activation.

**Syntax**

```go
func (b *Button) OnFocusChanged(focused bool)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `focused` | `bool` | The button's new focus state. |

**Notes** — Implements `input.FocusHandler`; invoked by the input router,
not normally called directly.

#### Button.OnPointer

Delegates the press/release/hover state machine to `ClickBehavior`.

**Syntax**

```go
func (b *Button) OnPointer(e *input.PointerEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.PointerEvent` | The pointer event to handle. |

**Notes** — Implements `input.PointerHandler`. While disabled, pointer input
is ignored outright (not merely failing to fire) — `e.Handled` is left
`false` so the event keeps bubbling.

**See also** — [ClickBehavior.HandlePointer](#clickbehaviorhandlepointer)

#### Button.OnKey

Activates the button on Space/Enter while focused.

**Syntax**

```go
func (b *Button) OnKey(e *input.KeyEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.KeyEvent` | The key event to handle. |

**Notes** — Implements `input.KeyHandler`: Space or Enter, on Press, fires
`OnClick` and marks the event handled. `OnKey` only ever runs while the
button is focused or an ancestor of the focused widget, and a disabled
button can never hold focus in the first place — the enabled check here is
a defensive no-op guard, not load-bearing.

**See also** — [ClickBehavior.Activate](#clickbehavioractivate)

---

## ButtonShape

`ButtonShape` selects a `Button`'s outer silhouette. Only `Button` and
`ToggleButton` support shapes (via `Button.SetShape`/`ToggleButton.SetShape`)
— it is not a general `Border`/control-wide concept.

### Values

| Constant | Value | Description |
|---|---|---|
| `ShapeRect` | `0` (zero value, default) | The classic square four-tone bevel, unchanged from before `ButtonShape` existed — every `Button` built before this feature was added renders byte-identically. |
| `ShapePill` | `1` | Renders a stadium (fully rounded ends); radius = `bounds.H / 2`. Never forces a square measure — a pill's radius already encloses its label without that. |
| `ShapeCircle` | `2` | Renders a circle; radius = `min(bounds.W, bounds.H) / 2`. `MeasureContent`'s desired size is additionally forced **square** (`side = max(paddedWidth, paddedHeight)`) so the circle fully encloses its label. |

**Example**

```go
round := controls.NewButton(face, "+").SetShape(controls.ShapeCircle)
pill := controls.NewButton(face, "Pill").SetShape(controls.ShapePill)
```

**See also** — [Button.SetShape](#buttonsetshape), [Button.MeasureContent](#buttonmeasurecontent)

---

## ToggleButton

`ToggleButton` is a `Button` that toggles a boolean checked state on click,
rendering the checked state as accent-on (`Accent`-family fill, no stroke,
`AccentText` label) regardless of accent — it has no independent accent flag
of its own; checked *is* the accent look, achieved by driving the embedded
`Button`'s own `SetAccent` as checked changes.

`ToggleButton` embeds `Button` **by value** and inherits its full method set
by promotion — `SetEnabled`, `Label`, `SetAnimated`, `SetTimers`,
`MeasureContent`, `ArrangeContent`, `Render`, `RenderOverlay`, `Children`,
`AcceptsFocus`, `OnFocusChanged`, `OnPointer`, `OnKey` all work unchanged and
are documented under [Button](#button) above. Two of `Button`'s methods are
deliberately shadowed (not left promoted) because leaving them promoted
would let a caller silently clobber the toggle wiring or desync the chrome
from `Checked()`; a third is shadowed purely to fix its return type for
fluent chaining.

**Constructor**

```go
func NewToggleButton(face *text.Face, label string) *ToggleButton
```

Returns an unchecked, enabled `ToggleButton` showing `label` in `face`.

**Example**

```go
t := controls.NewToggleButton(face, "Toggle")
t.OnChanged(func(checked bool) {
    fmt.Println("checked:", checked)
})
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [OnClick](#togglebuttononclick) | `OnClick(fn func()) *ToggleButton` | Shadowed: panics. Use `OnChanged` instead. |
| [SetAccent](#togglebuttonsetaccent) | `SetAccent(a bool) *ToggleButton` | Shadowed: panics. Use `SetChecked` instead. |
| [SetShape](#togglebuttonsetshape) | `SetShape(s ButtonShape) *ToggleButton` | Sets the outer silhouette, re-typed for chaining. |
| [Checked](#togglebuttonchecked) | `Checked() bool` | Reports the current toggle state. |
| [SetChecked](#togglebuttonsetchecked) | `SetChecked(v bool) *ToggleButton` | Sets the toggle state programmatically (silent). |
| [OnChanged](#togglebuttononchanged) | `OnChanged(fn func(bool)) *ToggleButton` | Sets the callback fired on a user-driven toggle. |

#### ToggleButton.OnClick

Shadowed and panics.

**Syntax**

```go
func (t *ToggleButton) OnClick(fn func()) *ToggleButton
```

**Notes** — A `ToggleButton` wires its own internal `ClickBehavior.OnClick`
in `NewToggleButton` to drive toggle+notify; `Button.OnClick`'s normal
"replace the callback" semantics would silently clobber that wiring,
permanently breaking `Checked`/`OnChanged` with no compile-time signal. This
method always panics with `"controls: ToggleButton.OnClick is not supported
(it would replace the internal toggle wiring) — use OnChanged instead"`. Use
[OnChanged](#togglebuttononchanged) instead.

**See also** — [ToggleButton.OnChanged](#togglebuttononchanged)

#### ToggleButton.SetAccent

Shadowed and panics.

**Syntax**

```go
func (t *ToggleButton) SetAccent(a bool) *ToggleButton
```

**Notes** — `ToggleButton` has no independent accent flag — its chrome is
driven entirely by `Checked` — so an external `SetAccent` call would
silently desync the rendered chrome from `Checked()` until the next toggle
overwrites it again. This method always panics with `"controls:
ToggleButton.SetAccent is not supported (checked state alone drives the
accent chrome) — use SetChecked instead"`. Use
[SetChecked](#togglebuttonsetchecked) instead.

**See also** — [ToggleButton.SetChecked](#togglebuttonsetchecked)

#### ToggleButton.SetShape

Sets the outer silhouette; default `ShapeRect`.

**Syntax**

```go
func (t *ToggleButton) SetShape(s ButtonShape) *ToggleButton
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `s` | `ButtonShape` | The silhouette to render; see [ButtonShape](#buttonshape). |

**Returns** — `*ToggleButton` for chaining.

**Example**

```go
t.SetShape(controls.ShapePill)
```

**Notes** — Shadowed purely for its return type: a promoted
`Button.SetShape` would return `*Button`, breaking `ToggleButton`'s fluent
chaining (`tb.SetShape(...).OnChanged(...)` wouldn't compile). Unlike
`OnClick`/`SetAccent` above, there is no wiring conflict here — this simply
forwards to the embedded `Button` and returns `t`.

**See also** — [ButtonShape](#buttonshape)

#### ToggleButton.Checked

Reports the current toggle state.

**Syntax**

```go
func (t *ToggleButton) Checked() bool
```

**Returns** — `bool`, the current checked state.

**Example**

```go
if t.Checked() {
    fmt.Println("on")
}
```

#### ToggleButton.SetChecked

Sets the toggle state programmatically.

**Syntax**

```go
func (t *ToggleButton) SetChecked(v bool) *ToggleButton
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `bool` | The new checked state. |

**Returns** — `*ToggleButton` for chaining.

**Example**

```go
t.SetChecked(true)
```

**Notes** — Normative: unlike a click, `SetChecked` does **not** fire
`OnChanged` — it is the plain setter counterpart to the getter `Checked`,
matching the rest of fluo's `SetX` convention, while `OnChanged` is reserved
for user-driven changes. A no-op when `v` already matches the current
state. Fluo's uniform contract: programmatic setters are silent; `OnChanged`
reports only user-driven changes.

**See also** — [ToggleButton.OnChanged](#togglebuttononchanged)

#### ToggleButton.OnChanged

Sets the callback fired on a user-driven toggle.

**Syntax**

```go
func (t *ToggleButton) OnChanged(fn func(bool)) *ToggleButton
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `fn` | `func(bool)` | Called with the new checked value whenever the user toggles the button (click or Space/Enter). Never called for a programmatic `SetChecked`. Replaces any previously set callback; `nil` is a valid, silent no-op. |

**Returns** — `*ToggleButton` for chaining.

**Example**

```go
t.OnChanged(func(checked bool) { fmt.Println(checked) })
```

**See also** — [ToggleButton.SetChecked](#togglebuttonsetchecked)

---

## CheckBox

`CheckBox` is a clickable, focusable, token-styled 18x18 box with an optional
label to its right, toggling a boolean checked state. Like `ToggleButton`, it
is `ClickBehavior`-driven and follows the `Checked`/`SetChecked`/`OnChanged`/
`SetEnabled` convention: `SetChecked` is a silent programmatic setter,
`OnChanged` fires only for user-driven changes.

Visuals (normative): the 18x18 box is always drawn as a classic sunken well
(`WindowWell` fill, unaffected by checked/hover/pressed state); checked
additionally draws a `WindowText` checkmark — either the U+2713 glyph, if
the face's font has it, or a fallback `WindowText`-colored inner square
inset 5 from the box's edges.

**Constructor**

```go
func NewCheckBox(face *text.Face, label string) *CheckBox
```

Returns an unchecked, enabled `CheckBox` showing `label` (may be `""`) in
`face` (`face` may be `nil` — a nil face also forces the fallback checkmark
square, since there is no font to query for the glyph).

**Example**

```go
cb := controls.NewCheckBox(face, "Enable")
cb.OnChanged(func(checked bool) {
    fmt.Println("checked:", checked)
})
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [Checked](#checkboxchecked) | `Checked() bool` | Reports the current toggle state. |
| [SetChecked](#checkboxsetchecked) | `SetChecked(v bool) *CheckBox` | Sets the toggle state programmatically (silent). |
| [OnChanged](#checkboxonchanged) | `OnChanged(fn func(bool)) *CheckBox` | Sets the callback fired on a user-driven toggle. |
| [SetEnabled](#checkboxsetenabled) | `SetEnabled(v bool) *CheckBox` | Toggles whether the box accepts focus and input. |
| [Label](#checkboxlabel) | `Label() *TextBlock` | Returns the label `TextBlock`. |
| [MeasureContent](#checkboxmeasurecontent) | `MeasureContent(available render.Size) render.Size` | Measures the glyph box plus optional label. |
| [ArrangeContent](#checkboxarrangecontent) | `ArrangeContent(bounds render.Rect)` | Arranges the label to the box's right. |
| [Children](#checkboxchildren) | `Children() []core.Widget` | Returns the label as the box's sole child. |
| [Render](#checkboxrender) | `Render(r render.Renderer)` | Paints the sunken box and checkmark. |
| [RenderOverlay](#checkboxrenderoverlay) | `RenderOverlay(r render.Renderer)` | Paints the focus ring around the box while focused. |
| [AcceptsFocus](#checkboxacceptsfocus) | `AcceptsFocus() bool` | Reports whether the box can take focus. |
| [OnFocusChanged](#checkboxonfocuschanged) | `OnFocusChanged(focused bool)` | Tracks focus for the ring overlay and keyboard activation. |
| [OnPointer](#checkboxonpointer) | `OnPointer(e *input.PointerEvent)` | Routes pointer events to the embedded `ClickBehavior`. |
| [OnKey](#checkboxonkey) | `OnKey(e *input.KeyEvent)` | Toggles the box on Space/Enter while focused. |

#### CheckBox.Checked

Reports the current toggle state.

**Syntax**

```go
func (c *CheckBox) Checked() bool
```

**Returns** — `bool`, the current checked state.

**Example**

```go
if cb.Checked() { /* ... */ }
```

#### CheckBox.SetChecked

Sets the toggle state programmatically.

**Syntax**

```go
func (c *CheckBox) SetChecked(v bool) *CheckBox
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `bool` | The new checked state. |

**Returns** — `*CheckBox` for chaining.

**Example**

```go
cb.SetChecked(true)
```

**Notes** — Unlike a click, `SetChecked` does not fire `OnChanged`, matching
`ToggleButton`'s convention, and is a no-op when `v` already matches the
current state. Fluo's uniform contract: programmatic setters are silent;
`OnChanged` reports only user-driven changes.

**See also** — [CheckBox.OnChanged](#checkboxonchanged)

#### CheckBox.OnChanged

Sets the callback fired on a user-driven toggle.

**Syntax**

```go
func (c *CheckBox) OnChanged(fn func(bool)) *CheckBox
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `fn` | `func(bool)` | Called with the new checked value whenever the user toggles the box (click or Space/Enter). Never called for a programmatic `SetChecked`. Replaces any previously set callback; `nil` is a valid, silent no-op. |

**Returns** — `*CheckBox` for chaining.

**Example**

```go
cb.OnChanged(func(checked bool) { fmt.Println(checked) })
```

#### CheckBox.SetEnabled

Toggles whether the checkbox accepts focus and pointer/keyboard input.

**Syntax**

```go
func (c *CheckBox) SetEnabled(v bool) *CheckBox
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `bool` | `false` disables the checkbox. |

**Returns** — `*CheckBox` for chaining.

**Example**

```go
cb.SetEnabled(false)
```

**Notes** — Purely visual/behavioral: no invalidation needed.

#### CheckBox.Label

Returns the checkbox's label `TextBlock`.

**Syntax**

```go
func (c *CheckBox) Label() *TextBlock
```

**Returns** — `*TextBlock`, for tests and customization.

**Example**

```go
cb.Label().SetColor(render.RGB(0, 100, 0))
```

#### CheckBox.MeasureContent

Measures the glyph box plus optional label.

**Syntax**

```go
func (c *CheckBox) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | Space offered by the parent layout pass. |

**Returns** — `render.Size`: the fixed 18x18 glyph box, plus (if the label
has content) a `PaddingM` gap and the label's own measured width; the gap is
only reserved when the label actually has content, so an empty label
doesn't leave a dangling gap.

**Notes** — Part of the `core.Widget` interface; invoked by
`core.MeasureWidget`, not normally called directly.

**See also** — [CheckBox.ArrangeContent](#checkboxarrangecontent)

#### CheckBox.ArrangeContent

Arranges the label to the glyph box's right.

**Syntax**

```go
func (c *CheckBox) ArrangeContent(bounds render.Rect)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `bounds` | `render.Rect` | The checkbox's final arranged rect from its parent layout pass. |

**Notes** — Also caches the box's own absolute rect for `Render` to draw
into. Part of the `core.Widget` interface; invoked by `core.ArrangeWidget`,
not normally called directly.

#### CheckBox.Children

Returns the label as the checkbox's sole child.

**Syntax**

```go
func (c *CheckBox) Children() []core.Widget
```

**Returns** — `[]core.Widget` containing exactly the label `TextBlock`.

**Notes** — Part of the `core.Widget` interface; not normally called
directly.

#### CheckBox.Render

Paints the sunken box and, if checked, the checkmark.

**Syntax**

```go
func (c *CheckBox) Render(r render.Renderer)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | The target renderer for this frame. |

**Notes** — The 18x18 box is always drawn as a classic sunken well
(`WindowWell` fill), unaffected by hover/pressed, per the classic-checkbox
spec. Part of the `core.Widget` interface; not normally called directly.

**See also** — [CheckBox.RenderOverlay](#checkboxrenderoverlay)

#### CheckBox.RenderOverlay

Draws the classic focus rectangle around the glyph box while focused.

**Syntax**

```go
func (c *CheckBox) RenderOverlay(r render.Renderer)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | The target renderer for this frame. |

**Notes** — Implements `core.OverlayRenderer`; not normally called directly.

#### CheckBox.AcceptsFocus

Reports whether the checkbox can currently take focus.

**Syntax**

```go
func (c *CheckBox) AcceptsFocus() bool
```

**Returns** — `bool`, `true` unless the checkbox is disabled.

**Notes** — Implements `input.Focusable`: a disabled checkbox never accepts
focus.

#### CheckBox.OnFocusChanged

Tracks focus for the ring overlay and keyboard activation.

**Syntax**

```go
func (c *CheckBox) OnFocusChanged(focused bool)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `focused` | `bool` | The checkbox's new focus state. |

**Notes** — Implements `input.FocusHandler`; not normally called directly.

#### CheckBox.OnPointer

Delegates to the embedded `ClickBehavior` while enabled.

**Syntax**

```go
func (c *CheckBox) OnPointer(e *input.PointerEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.PointerEvent` | The pointer event to handle. |

**Notes** — Implements `input.PointerHandler`. Disabled ignores pointer
input outright (not merely failing to fire) — `e.Handled` is left `false` so
the event keeps bubbling. The clickable area is the whole composite (box +
label), not just the box, matching ordinary checkbox UX.

**See also** — [ClickBehavior.HandlePointer](#clickbehaviorhandlepointer)

#### CheckBox.OnKey

Toggles the checkbox on Space/Enter while focused.

**Syntax**

```go
func (c *CheckBox) OnKey(e *input.KeyEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.KeyEvent` | The key event to handle. |

**Notes** — Implements `input.KeyHandler`: Space or Enter, on Press, toggles
the checkbox (fires `OnChanged`) and marks the event handled.

**See also** — [ClickBehavior.Activate](#clickbehavioractivate)

---

## RadioButton

`RadioButton` is a clickable, focusable, token-styled 18x18 circle with an
optional label to its right. Unlike `CheckBox`, clicking never turns a
checked `RadioButton` off directly — real radio semantics: selecting one
deselects its siblings, and re-selecting the already-selected one is a
no-op. Add a `RadioButton` to a [RadioGroup](#radiogroup) for that
sibling-deselection behavior; a `RadioButton` with no group behaves like a
one-way checkbox (click sets it checked, never unchecked, by itself).

Visuals (normative): the 18x18 circle (radius 9) is always drawn as a
classic sunken-looking well — `WindowWell` fill plus a 1px `ButtonShadow`
ring, the one square-corner exception in this family — regardless of checked
state; checked additionally draws an inner `WindowText` dot (9x9, radius
4.5) centered within the outer circle.

**Constructor**

```go
func NewRadioButton(face *text.Face, label string) *RadioButton
```

Returns an unchecked, enabled, ungrouped `RadioButton` showing `label` in
`face` (`face` may be `nil`). Add it to a `RadioGroup` via `RadioGroup.Add`
for mutual-exclusion behavior.

**Example**

```go
rb := controls.NewRadioButton(face, "Small")
rb.OnChanged(func(selected bool) {
    fmt.Println("selected:", selected)
})
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [Checked](#radiobuttonchecked) | `Checked() bool` | Reports the current selection state. |
| [SetChecked](#radiobuttonsetchecked) | `SetChecked(v bool) *RadioButton` | Sets the selection state programmatically (silent). |
| [OnChanged](#radiobuttononchanged) | `OnChanged(fn func(bool)) *RadioButton` | Sets the callback fired on a user-driven selection. |
| [SetEnabled](#radiobuttonsetenabled) | `SetEnabled(v bool) *RadioButton` | Toggles whether the button accepts focus and input. |
| [Label](#radiobuttonlabel) | `Label() *TextBlock` | Returns the label `TextBlock`. |
| [MeasureContent](#radiobuttonmeasurecontent) | `MeasureContent(available render.Size) render.Size` | Measures the glyph circle plus optional label. |
| [ArrangeContent](#radiobuttonarrangecontent) | `ArrangeContent(bounds render.Rect)` | Arranges the label to the circle's right. |
| [Children](#radiobuttonchildren) | `Children() []core.Widget` | Returns the label as the button's sole child. |
| [Render](#radiobuttonrender) | `Render(r render.Renderer)` | Paints the sunken circle and selection dot. |
| [RenderOverlay](#radiobuttonrenderoverlay) | `RenderOverlay(r render.Renderer)` | Paints the focus ring around the circle while focused. |
| [AcceptsFocus](#radiobuttonacceptsfocus) | `AcceptsFocus() bool` | Reports whether the button can take focus. |
| [OnFocusChanged](#radiobuttononfocuschanged) | `OnFocusChanged(focused bool)` | Tracks focus for the ring overlay. |
| [OnPointer](#radiobuttononpointer) | `OnPointer(e *input.PointerEvent)` | Routes pointer events to the embedded `ClickBehavior`. |
| [OnKey](#radiobuttononkey) | `OnKey(e *input.KeyEvent)` | Selects the button on Space/Enter while focused. |

#### RadioButton.Checked

Reports the current selection state.

**Syntax**

```go
func (rb *RadioButton) Checked() bool
```

**Returns** — `bool`, the current selection state.

**Example**

```go
if rb.Checked() { /* ... */ }
```

#### RadioButton.SetChecked

Sets the selection state programmatically.

**Syntax**

```go
func (rb *RadioButton) SetChecked(v bool) *RadioButton
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `bool` | The new selection state. |

**Returns** — `*RadioButton` for chaining.

**Example**

```go
rb.SetChecked(true)
```

**Notes** — Silent (no `OnChanged`), matching `CheckBox`/`ToggleButton`'s
convention. A no-op when `v` already matches the current state. Setting
`true` on a grouped button also silently deselects its siblings, preserving
the group's mutual-exclusion invariant even under direct programmatic
control. Fluo's uniform contract: programmatic setters are silent;
`OnChanged` reports only user-driven changes.

**See also** — [RadioGroup](#radiogroup), [RadioButton.OnChanged](#radiobuttononchanged)

#### RadioButton.OnChanged

Sets the callback fired on a user-driven selection.

**Syntax**

```go
func (rb *RadioButton) OnChanged(fn func(bool)) *RadioButton
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `fn` | `func(bool)` | Called with `true` whenever the user selects this radio button (click or Space/Enter). Never called for a programmatic `SetChecked`, and never when this button is deselected as a side effect of a sibling being selected. Replaces any previously set callback; `nil` is a valid, silent no-op. |

**Returns** — `*RadioButton` for chaining.

**Example**

```go
rb.OnChanged(func(selected bool) { fmt.Println(selected) })
```

#### RadioButton.SetEnabled

Toggles whether the radio button accepts focus and pointer/keyboard input.

**Syntax**

```go
func (rb *RadioButton) SetEnabled(v bool) *RadioButton
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `bool` | `false` disables the button. |

**Returns** — `*RadioButton` for chaining.

**Example**

```go
rb.SetEnabled(false)
```

#### RadioButton.Label

Returns the radio button's label `TextBlock`.

**Syntax**

```go
func (rb *RadioButton) Label() *TextBlock
```

**Returns** — `*TextBlock`, for tests and customization.

**Example**

```go
rb.Label().SetColor(render.RGB(0, 0, 0))
```

#### RadioButton.MeasureContent

Measures the glyph circle plus optional label.

**Syntax**

```go
func (rb *RadioButton) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | Space offered by the parent layout pass. |

**Returns** — `render.Size`: the fixed 18x18 glyph circle, plus (if the
label has content) a `PaddingM` gap and the label's measured width.

**Notes** — Shares its measure logic with `CheckBox` (both are an
18x18-glyph-plus-optional-label composite). Part of the `core.Widget`
interface; not normally called directly.

**See also** — [RadioButton.ArrangeContent](#radiobuttonarrangecontent)

#### RadioButton.ArrangeContent

Arranges the label to the glyph circle's right.

**Syntax**

```go
func (rb *RadioButton) ArrangeContent(bounds render.Rect)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `bounds` | `render.Rect` | The radio button's final arranged rect from its parent layout pass. |

**Notes** — Also caches the circle's own absolute rect for `Render`. Part of
the `core.Widget` interface; not normally called directly.

#### RadioButton.Children

Returns the label as the radio button's sole child.

**Syntax**

```go
func (rb *RadioButton) Children() []core.Widget
```

**Returns** — `[]core.Widget` containing exactly the label `TextBlock`.

**Notes** — Part of the `core.Widget` interface; not normally called
directly.

#### RadioButton.Render

Paints the sunken-looking circle and, if checked, the inner dot.

**Syntax**

```go
func (rb *RadioButton) Render(r render.Renderer)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | The target renderer for this frame. |

**Notes** — The 18x18 circle is drawn with `WindowWell` fill plus a 1px
`ButtonShadow` ring — the spec's approved approximation of a full bevel on a
round glyph (`RadioButton` is the one exception to the family's square
corners). When checked, an inner `WindowText` dot (9x9, radius 4.5,
centered) marks the selection. Part of the `core.Widget` interface; not
normally called directly.

**See also** — [RadioButton.RenderOverlay](#radiobuttonrenderoverlay)

#### RadioButton.RenderOverlay

Draws the classic focus rectangle around the glyph circle while focused.

**Syntax**

```go
func (rb *RadioButton) RenderOverlay(r render.Renderer)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | The target renderer for this frame. |

**Notes** — Implements `core.OverlayRenderer`; not normally called directly.

#### RadioButton.AcceptsFocus

Reports whether the radio button can currently take focus.

**Syntax**

```go
func (rb *RadioButton) AcceptsFocus() bool
```

**Returns** — `bool`, `true` unless the button is disabled.

**Notes** — Implements `input.Focusable`: a disabled radio button never
accepts focus.

#### RadioButton.OnFocusChanged

Tracks focus for the ring overlay.

**Syntax**

```go
func (rb *RadioButton) OnFocusChanged(focused bool)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `focused` | `bool` | The button's new focus state. |

**Notes** — Implements `input.FocusHandler`; not normally called directly.

#### RadioButton.OnPointer

Delegates to the embedded `ClickBehavior` while enabled.

**Syntax**

```go
func (rb *RadioButton) OnPointer(e *input.PointerEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.PointerEvent` | The pointer event to handle. |

**Notes** — Implements `input.PointerHandler`. Disabled ignores pointer
input outright, leaving `e.Handled` `false` so the event keeps bubbling.

**See also** — [ClickBehavior.HandlePointer](#clickbehaviorhandlepointer)

#### RadioButton.OnKey

Selects the radio button on Space/Enter while focused.

**Syntax**

```go
func (rb *RadioButton) OnKey(e *input.KeyEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.KeyEvent` | The key event to handle. |

**Notes** — Implements `input.KeyHandler`: Space or Enter, on Press,
activates (selects) the radio button and marks the event handled.

**See also** — [ClickBehavior.Activate](#clickbehavioractivate)

---

## RadioGroup

`RadioGroup` coordinates mutual exclusion across a set of `RadioButton`s:
selecting one (via click, Space/Enter, or `SetChecked(true)`) deselects
every other member, and `OnChanged` fires with the newly selected member's
index (its position in `Add` call order).

**Constructor**

```go
func NewRadioGroup() *RadioGroup
```

Returns an empty `RadioGroup`.

**Example — wiring three radio buttons into one group**

```go
group := controls.NewRadioGroup()

small := controls.NewRadioButton(face, "Small")
medium := controls.NewRadioButton(face, "Medium")
large := controls.NewRadioButton(face, "Large")

group.Add(small).Add(medium).Add(large)
group.OnChanged(func(i int) {
    fmt.Println("selected index:", i)
})

medium.SetChecked(true) // preselect "Medium", silently (no OnChanged)
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [Add](#radiogroupadd) | `Add(rb *RadioButton) *RadioGroup` | Registers a `RadioButton` as a group member. |
| [OnChanged](#radiogrouponchanged) | `OnChanged(fn func(int)) *RadioGroup` | Sets the callback fired with the newly selected index. |
| [SelectedIndex](#radiogroupselectedindex) | `SelectedIndex() int` | Returns the currently checked member's index, or -1. |
| [SetSelectedIndex](#radiogroupsetselectedindex) | `SetSelectedIndex(i int) *RadioGroup` | Sets the checked member by index programmatically (silent). |

#### RadioGroup.Add

Registers `rb` as a member of the group.

**Syntax**

```go
func (g *RadioGroup) Add(rb *RadioButton) *RadioGroup
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `rb` | `*RadioButton` | The radio button to add, wired into the group's mutual-exclusion behavior. |

**Returns** — `*RadioGroup` for chaining (mirroring `StackPanel.Add`'s
style).

**Example**

```go
group.Add(optionA).Add(optionB)
```

**Notes** — v0 caveat: `RadioGroup` holds plain `*RadioButton` pointers in
its member list and has no `Remove` — a button, once added, stays a member
(and keeps its index) for the group's lifetime. Building a group whose
membership changes at runtime is out of scope for now; construct a fresh
`RadioGroup` instead.

#### RadioGroup.OnChanged

Sets the callback fired with the newly selected member's index.

**Syntax**

```go
func (g *RadioGroup) OnChanged(fn func(int)) *RadioGroup
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `fn` | `func(int)` | Called with the newly selected member's index whenever the user selects a *different* member than the one currently checked. Never called when re-selecting the same member, and never for a programmatic `SetSelectedIndex`/`SetChecked`. Replaces any previously set callback; `nil` is a valid, silent no-op. |

**Returns** — `*RadioGroup` for chaining.

**Example**

```go
group.OnChanged(func(i int) { fmt.Println("selected index", i) })
```

**See also** — [RadioButton.OnChanged](#radiobuttononchanged)

#### RadioGroup.SelectedIndex

Returns the currently checked member's index.

**Syntax**

```go
func (g *RadioGroup) SelectedIndex() int
```

**Returns** — `int`, the index (in `Add` call order) of the currently
checked member, or `-1` if no member is checked (including the empty
group).

**Example**

```go
idx := group.SelectedIndex()
```

**See also** — [RadioGroup.SetSelectedIndex](#radiogroupsetselectedindex)

#### RadioGroup.SetSelectedIndex

Sets the checked member by index programmatically.

**Syntax**

```go
func (g *RadioGroup) SetSelectedIndex(i int) *RadioGroup
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `i` | `int` | The index (in `Add` call order) to select. Clamped to `[-1, len(members)-1]`; `-1` (or any `i` on an empty group) clears every member's checked state. |

**Returns** — `*RadioGroup` for chaining.

**Example**

```go
group.SetSelectedIndex(1) // preselect the second member, silently
```

**Notes** — Silent — no member's `OnChanged` nor the group's own `OnChanged`
fires, matching fluo's uniform contract that programmatic setters are
silent. Respects group exclusivity: a valid `i` silently deselects every
other member, same as `SetChecked(true)` on a grouped `RadioButton`.

**See also** — [RadioGroup.SelectedIndex](#radiogroupselectedindex)

---

## ToggleSwitch

`ToggleSwitch` is a clickable, focusable, token-styled 40x20 track-and-knob
toggle (no label — unlike `CheckBox`/`RadioButton`, `NewToggleSwitch` takes
no face or label argument). Like `CheckBox` and `ToggleButton`, it is
`ClickBehavior`-driven and follows the `Checked`/`SetChecked`/`OnChanged`/
`SetEnabled` convention: `SetChecked` is a silent programmatic setter,
`OnChanged` fires only for user-driven changes.

Visuals (normative): off = `ControlFill` fill + `ControlStroke` stroke, with
a `TextSecondary` thumb square (12px) inset 4px from the left edge; on =
`Accent` fill, no stroke, with an `AccentText` thumb inset 4px from the
right edge instead.

**Constructor**

```go
func NewToggleSwitch() *ToggleSwitch
```

Returns an off (unchecked), enabled `ToggleSwitch`.

**Example**

```go
sw := controls.NewToggleSwitch()
sw.OnChanged(func(on bool) {
    fmt.Println("on:", on)
})
```

**Notes** — Per the v0.2 classic-depth design spec, `ToggleSwitch` has no
counterpart in the Win2000 control set the rest of this palette imitates —
it is called out there as a documented fluo extension: a small raised knob
sliding within a sunken track, rendered with the same theme tokens as
everything else rather than copying a native control that never existed in
that era.

### Methods

| Method | Signature | Description |
|---|---|---|
| [Checked](#toggleswitchchecked) | `Checked() bool` | Reports the current on/off state. |
| [SetChecked](#toggleswitchsetchecked) | `SetChecked(v bool) *ToggleSwitch` | Sets the on/off state programmatically (silent). |
| [OnChanged](#toggleswitchonchanged) | `OnChanged(fn func(bool)) *ToggleSwitch` | Sets the callback fired on a user-driven flip. |
| [SetEnabled](#toggleswitchsetenabled) | `SetEnabled(v bool) *ToggleSwitch` | Toggles whether the switch accepts focus and input. |
| [MeasureContent](#toggleswitchmeasurecontent) | `MeasureContent(available render.Size) render.Size` | Always returns the fixed 40x20 pill size. |
| [ArrangeContent](#toggleswitcharrangecontent) | `ArrangeContent(bounds render.Rect)` | No-op: no children to position. |
| [Children](#toggleswitchchildren) | `Children() []core.Widget` | Returns `nil`: a leaf widget. |
| [Render](#toggleswitchrender) | `Render(r render.Renderer)` | Paints the track and sliding knob. |
| [RenderOverlay](#toggleswitchrenderoverlay) | `RenderOverlay(r render.Renderer)` | Paints the focus ring around the track while focused. |
| [AcceptsFocus](#toggleswitchacceptsfocus) | `AcceptsFocus() bool` | Reports whether the switch can take focus. |
| [OnFocusChanged](#toggleswitchonfocuschanged) | `OnFocusChanged(focused bool)` | Tracks focus for the ring overlay. |
| [OnPointer](#toggleswitchonpointer) | `OnPointer(e *input.PointerEvent)` | Routes pointer events to the embedded `ClickBehavior`. |
| [OnKey](#toggleswitchonkey) | `OnKey(e *input.KeyEvent)` | Flips the switch on Space/Enter while focused. |

#### ToggleSwitch.Checked

Reports the current on/off state.

**Syntax**

```go
func (s *ToggleSwitch) Checked() bool
```

**Returns** — `bool`, the current on/off state.

**Example**

```go
if sw.Checked() { /* ... */ }
```

#### ToggleSwitch.SetChecked

Sets the on/off state programmatically.

**Syntax**

```go
func (s *ToggleSwitch) SetChecked(v bool) *ToggleSwitch
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `bool` | The new on/off state. |

**Returns** — `*ToggleSwitch` for chaining.

**Example**

```go
sw.SetChecked(true)
```

**Notes** — Unlike a click, does not fire `OnChanged` (matching `CheckBox`/
`ToggleButton`'s convention) and is a no-op when `v` already matches the
current state. Fluo's uniform contract: programmatic setters are silent;
`OnChanged` reports only user-driven changes.

**See also** — [ToggleSwitch.OnChanged](#toggleswitchonchanged)

#### ToggleSwitch.OnChanged

Sets the callback fired on a user-driven flip.

**Syntax**

```go
func (s *ToggleSwitch) OnChanged(fn func(bool)) *ToggleSwitch
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `fn` | `func(bool)` | Called with the new checked value whenever the user flips the switch (click or Space/Enter). Never called for a programmatic `SetChecked`. Replaces any previously set callback; `nil` is a valid, silent no-op. |

**Returns** — `*ToggleSwitch` for chaining.

**Example**

```go
sw.OnChanged(func(on bool) { fmt.Println(on) })
```

#### ToggleSwitch.SetEnabled

Toggles whether the switch accepts focus and pointer/keyboard input.

**Syntax**

```go
func (s *ToggleSwitch) SetEnabled(v bool) *ToggleSwitch
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `bool` | `false` disables the switch. |

**Returns** — `*ToggleSwitch` for chaining.

**Example**

```go
sw.SetEnabled(false)
```

#### ToggleSwitch.MeasureContent

Always returns the fixed 40x20 pill size.

**Syntax**

```go
func (s *ToggleSwitch) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | Space offered by the parent layout pass (ignored). |

**Returns** — `render.Size{W: 40, H: 20}`, always — `ToggleSwitch` has no
label and no other content to size around.

**Notes** — Part of the `core.Widget` interface; not normally called
directly.

#### ToggleSwitch.ArrangeContent

No-op.

**Syntax**

```go
func (s *ToggleSwitch) ArrangeContent(bounds render.Rect)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `bounds` | `render.Rect` | The switch's final arranged rect (unused). |

**Notes** — `ToggleSwitch` has no children to position. Part of the
`core.Widget` interface; not normally called directly.

#### ToggleSwitch.Children

Returns `nil`.

**Syntax**

```go
func (s *ToggleSwitch) Children() []core.Widget
```

**Returns** — `nil` — `ToggleSwitch` is a leaf widget.

**Notes** — Part of the `core.Widget` interface; not normally called
directly.

#### ToggleSwitch.Render

Paints the track and sliding knob.

**Syntax**

```go
func (s *ToggleSwitch) Render(r render.Renderer)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | The target renderer for this frame. |

**Notes** — Paints the 40x20 track as a classic sunken well (`ButtonFace`
fill, or `Highlight` when on) and the knob as a small raised square sliding
to the left (off) or right (on) inset position. Part of the `core.Widget`
interface; not normally called directly.

**See also** — [ToggleSwitch.RenderOverlay](#toggleswitchrenderoverlay)

#### ToggleSwitch.RenderOverlay

Draws the classic focus rectangle around the track while focused.

**Syntax**

```go
func (s *ToggleSwitch) RenderOverlay(r render.Renderer)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | The target renderer for this frame. |

**Notes** — Implements `core.OverlayRenderer`; not normally called directly.

#### ToggleSwitch.AcceptsFocus

Reports whether the switch can currently take focus.

**Syntax**

```go
func (s *ToggleSwitch) AcceptsFocus() bool
```

**Returns** — `bool`, `true` unless the switch is disabled.

**Notes** — Implements `input.Focusable`: a disabled switch never accepts
focus.

#### ToggleSwitch.OnFocusChanged

Tracks focus for the ring overlay.

**Syntax**

```go
func (s *ToggleSwitch) OnFocusChanged(focused bool)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `focused` | `bool` | The switch's new focus state. |

**Notes** — Implements `input.FocusHandler`; not normally called directly.

#### ToggleSwitch.OnPointer

Delegates to the embedded `ClickBehavior` while enabled.

**Syntax**

```go
func (s *ToggleSwitch) OnPointer(e *input.PointerEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.PointerEvent` | The pointer event to handle. |

**Notes** — Implements `input.PointerHandler`. Disabled ignores pointer
input outright, leaving `e.Handled` `false` so the event keeps bubbling.

**See also** — [ClickBehavior.HandlePointer](#clickbehaviorhandlepointer)

#### ToggleSwitch.OnKey

Flips the switch on Space/Enter while focused.

**Syntax**

```go
func (s *ToggleSwitch) OnKey(e *input.KeyEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.KeyEvent` | The key event to handle. |

**Notes** — Implements `input.KeyHandler`: Space or Enter, on Press, flips
the switch (fires `OnChanged`) and marks the event handled.

**See also** — [ClickBehavior.Activate](#clickbehavioractivate)

---

## ClickBehavior

`ClickBehavior` implements the standard press state machine shared by every
button-like widget in this family: a `Press` over the widget captures the
pointer (so drag tracking survives the pointer leaving the widget's bounds)
and marks hover state via `Enter`/`Leave`; the matching `Release` fires
`OnClick` only if the pointer is still over the widget's bounds at release
time, and always releases the capture. Embed it **by value** in a composite
widget (as `Button`, `CheckBox`, `RadioButton`, and `ToggleSwitch` all do)
and drive it from the owner's own `OnPointer` via `HandlePointer`, passing
the owner itself so containment can be tested against its live bounds.

`ClickBehavior` has no notion of "enabled" — an owner that wants disabled
widgets to ignore pointer input entirely (not merely fail to fire) must skip
calling `HandlePointer` altogether while disabled, so `e.Handled` stays
`false` and the event keeps bubbling. This is exactly the pattern every
button-family `OnPointer` in this package follows: check `enabled` first,
and only call `HandlePointer` when true.

`ClickBehavior`'s zero value is ready to use; there is no constructor.

**Example — embedding ClickBehavior in a composite widget**

```go
type myWidget struct {
    core.Element
    click controls.ClickBehavior
}

func (w *myWidget) OnPointer(e *input.PointerEvent) {
    w.click.HandlePointer(e, w)
}
```

### Fields

| Name | Type | Description |
|---|---|---|
| `OnClick` | `func()` | Fires when a `Press` over the widget is followed by a `Release` still over it (see `HandlePointer`), or via a keyboard activation path calling `Activate` directly. `nil` is a valid, silent no-op. |

### Methods

| Method | Signature | Description |
|---|---|---|
| [HandlePointer](#clickbehaviorhandlepointer) | `HandlePointer(e *input.PointerEvent, owner core.Widget)` | Runs the click state machine for a pointer event. |
| [Hover](#clickbehaviorhover) | `Hover() bool` | Reports whether the pointer is currently over the widget. |
| [Pressed](#clickbehaviorpressed) | `Pressed() bool` | Reports whether the widget is currently mid-press. |
| [Activate](#clickbehavioractivate) | `Activate()` | Fires `OnClick` directly, bypassing the press/release machine. |

#### ClickBehavior.HandlePointer

Runs the click state machine for a pointer event.

**Syntax**

```go
func (c *ClickBehavior) HandlePointer(e *input.PointerEvent, owner core.Widget)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.PointerEvent` | The pointer event to handle. `e.Router` is assumed non-nil, as it always is for events delivered through an `input.Router`. |
| `owner` | `core.Widget` | The widget to test containment against (via `core.BoundsOf(owner)`) and to register with the router's pointer capture. Must be the exact `core.Widget` value the caller's own `OnPointer` receiver serves as. |

**Notes** — `Enter`/`Leave` only update `Hover`. `Press` marks pressed,
captures the pointer, and marks `e.Handled`. `Release` (delivered directly
to the captured owner regardless of `e.Pos`, per `input.Router`'s capture
semantics) clears pressed, releases the capture, marks `e.Handled`, and
fires `OnClick` only if the widget was pressed AND `e.Pos` still falls
within its current bounds — releasing outside does not fire. `Move` and
`Wheel` are ignored: hover across a drag is not tracked mid-capture (a
documented v0 simplification). For an embedder reached through outer type
embedding (e.g. `ToggleButton` embedding `Button` embedding
`ClickBehavior`), `owner` is the *embedded* field's address, not the
outermost widget — see `ToggleButton`'s type doc comment for the resulting
identity trap when comparing `input.Router.Captured()` against an outer
reference.

**See also** — [ClickBehavior.Activate](#clickbehavioractivate)

#### ClickBehavior.Hover

Reports whether the pointer is currently over the widget.

**Syntax**

```go
func (c *ClickBehavior) Hover() bool
```

**Returns** — `bool`, as last updated by an `Enter`/`Leave` delivered to
`HandlePointer`.

**Example**

```go
if b.click.Hover() { /* ... */ }
```

#### ClickBehavior.Pressed

Reports whether the widget is currently mid-press.

**Syntax**

```go
func (c *ClickBehavior) Pressed() bool
```

**Returns** — `bool`, `true` while a `Press` was delivered and no matching
`Release` has landed yet.

**Example**

```go
if b.click.Pressed() { /* ... */ }
```

#### ClickBehavior.Activate

Fires `OnClick` directly, bypassing the press/release state machine.

**Syntax**

```go
func (c *ClickBehavior) Activate()
```

**Notes** — This is the keyboard-activation path (Space/Enter on a focused
button) as well as the mechanism `HandlePointer`'s `Release` branch itself
uses on a successful click.

**Example**

```go
b.click.Activate()
```

**See also** — [ClickBehavior.HandlePointer](#clickbehaviorhandlepointer)
