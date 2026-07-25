# bind

The `bind` package connects `core.Property[T]` values — fluo's reactive
value type (see the core wiki page's [Property](core.md#property) entry) —
to `controls` widgets. A `core.Property[T]` is always the source of truth:
every binder in this package either pushes the property's value into a
control (one-way) or additionally wires the control's `OnChanged` slot back
into the property (two-way). Two-way binders are **echo-safe** by
construction, and that safety rests on a library-wide convention rather than
anything specific to `bind`: every control setter used here (`SetText`,
`SetChecked`, `SetValue`, `SetSelectedIndex`, …) is a **silent setter** —
programmatic calls never fire the control's `OnChanged` callback, only a
user-driven interaction does. A model push therefore updates the control
without looping back into `p.Set`, and a user edit's `p.Set(v)` is a no-op
whenever `v` already equals the property's current value (per
`Property.Set`'s own equality guard), so the binder's own subscriber never
re-applies a value the control already shows. No feedback loop is possible
in either direction.

Every binder — `OneWay` and each two-way binder — returns a **cancel func**.
Calling it detaches the binding: it cancels the property subscription and
(two-way only) clears the control's `OnChanged` slot back to `nil`. Both
underlying operations are idempotent, so the composed cancel is idempotent
too — calling it more than once is a harmless no-op. **Always call cancel
when a bound control is discarded.** The documented usage pattern is
"models outlive views": a `core.Property[T]` typically lives in a
view-model that survives across rebuilds, while the bound control is torn
down and recreated — cancel the old binding before (or as part of)
discarding the old control, or the binder's subscriber closure keeps the
dead control alive and keeps writing to it. Binding the same control twice
without canceling the first is undefined: the second bind takes over
`OnChanged` ownership, and the first bind's cancel will clear the second
bind's hook out from under it.

Package `bind` also provides `List[T]`, an observable slice for collection
binding, and `Items`, which keeps a `StackPanel`'s children rebuilt from a
`List`'s contents on every change. `List[T]`'s granular change type is a
type alias for a type declared in `controls` (see [Change](#change) below)
rather than an independent definition — a deliberately inverted import
seam so `*bind.List[T]` can satisfy `controls.ListView`'s `ListItems`
interface without `controls` importing `bind`. For the consumer side of
that interface (`ListItems`, `ListChange`) and how `ListView` virtualizes
against it, see the collections wiki page.

**Import:** `github.com/0xdreadnaught/fluo/bind`

## Contents
- [OneWay](#oneway)
- [Two-way binders](#two-way-binders)
  - [Text](#text)
  - [Checked](#checked)
  - [SwitchChecked](#switchchecked)
  - [ToggleChecked](#togglechecked)
  - [Value](#value)
  - [SelectedIndex](#selectedindex)
  - [Selected](#selected)
  - [ListSelected](#listselected)
- [List[T]](#listt)
- [ChangeKind / Change](#changekind--change)
- [Items](#items)

---

## OneWay

One-way binding: applies `p`'s current value immediately, then re-applies
it every time `p` changes thereafter. There is no reverse direction — this
function does not touch any control's `OnChanged` slot, so it works with an
arbitrary `apply` callback rather than a specific control type.

**Syntax**

```go
func OneWay[T comparable](p *core.Property[T], apply func(T)) (cancel func())
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `p` | `*core.Property[T]` | The source property. Must have a `comparable` element type (the constraint `core.Property[T]` itself requires). |
| `apply` | `func(T)` | Called immediately with `p.Get()`, then again with the new value on every subsequent change. |

**Returns** — `cancel func()`, the underlying `Property.OnChange` subscription's own cancel func (already idempotent). Calling it stops further changes to `p` from invoking `apply`.

**Example**

```go
name := &core.Property[string]{}
name.Set("Ada")

label := controls.NewTextBlock(face, "")
cancel := bind.OneWay(name, func(v string) { label.SetText(v) })
defer cancel()
```

**See also** — [Text](#text), [core.Property](core.md#property)

---

## Two-way binders

Every two-way binder below shares identical mechanics (see the package
overview above): on bind, the control is set to `p`'s current value via its
silent setter; the binder then installs its own `OnChanged` callback that
pushes user edits into `p` via `p.Set`, and subscribes to `p` so any other
change (a "model push") is applied back to the control via the same silent
setter. Calling the returned cancel detaches both directions.

Naming note: among this checked-family of binders, `Checked` names
`CheckBox`'s binder — the package's canonical bool control — so the others
are prefixed by their control (`SwitchChecked`, `ToggleChecked`) since
`Checked` was already taken. This is a naming-only distinction; the
mechanics are identical.

| Function | Signature | Description |
|---|---|---|
| [Text](#text) | `func Text(p *core.Property[string], tb *controls.TextBox) (cancel func())` | Two-way binds a string property to a `TextBox`. |
| [Checked](#checked) | `func Checked(p *core.Property[bool], cb *controls.CheckBox) (cancel func())` | Two-way binds a bool property to a `CheckBox`. |
| [SwitchChecked](#switchchecked) | `func SwitchChecked(p *core.Property[bool], sw *controls.ToggleSwitch) (cancel func())` | Two-way binds a bool property to a `ToggleSwitch`. |
| [ToggleChecked](#togglechecked) | `func ToggleChecked(p *core.Property[bool], tb *controls.ToggleButton) (cancel func())` | Two-way binds a bool property to a `ToggleButton`. |
| [Value](#value) | `func Value(p *core.Property[float32], s *controls.Slider) (cancel func())` | Two-way binds a float32 property to a `Slider`. |
| [SelectedIndex](#selectedindex) | `func SelectedIndex(p *core.Property[int], cb *controls.ComboBox) (cancel func())` | Two-way binds an int property to a `ComboBox`'s selected index. |
| [Selected](#selected) | `func Selected(p *core.Property[int], g *controls.RadioGroup) (cancel func())` | Two-way binds an int property to a `RadioGroup`'s selected index. |
| [ListSelected](#listselected) | `func ListSelected(p *core.Property[int], lv *controls.ListView) (cancel func())` | Two-way binds an int property to a `ListView`'s selected index. |

#### Text

Two-way binds `p` to `tb`. While bound, `Text` owns `tb`'s `OnChanged` slot,
replacing any previously set callback.

**Syntax**

```go
func Text(p *core.Property[string], tb *controls.TextBox) (cancel func())
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `p` | `*core.Property[string]` | The source property. |
| `tb` | `*controls.TextBox` | The control to bind. Its `SetText` is used for the initial apply and every model push; its `OnChanged` is replaced. |

**Returns** — `cancel func()`. Calls the property subscription's cancel, then sets `tb.OnChanged(nil)`.

**Example**

```go
name := &core.Property[string]{}
tb := controls.NewTextBox(face)
cancel := bind.Text(name, tb)
defer cancel()
```

**See also** — [Checked](#checked), [OneWay](#oneway)

#### Checked

Two-way binds `p` to `cb`. While bound, `Checked` owns `cb`'s `OnChanged`
slot, replacing any previously set callback.

**Syntax**

```go
func Checked(p *core.Property[bool], cb *controls.CheckBox) (cancel func())
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `p` | `*core.Property[bool]` | The source property. |
| `cb` | `*controls.CheckBox` | The control to bind. Its `SetChecked` is used for the initial apply and every model push; its `OnChanged` is replaced. |

**Returns** — `cancel func()`. Calls the property subscription's cancel, then sets `cb.OnChanged(nil)`.

**Example**

```go
enabled := &core.Property[bool]{}
cb := controls.NewCheckBox(face, "Enable notifications")
cancel := bind.Checked(enabled, cb)
defer cancel()
```

**See also** — [SwitchChecked](#switchchecked), [ToggleChecked](#togglechecked)

#### SwitchChecked

Two-way binds `p` to `sw`. While bound, `SwitchChecked` owns `sw`'s
`OnChanged` slot, replacing any previously set callback.

**Syntax**

```go
func SwitchChecked(p *core.Property[bool], sw *controls.ToggleSwitch) (cancel func())
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `p` | `*core.Property[bool]` | The source property. |
| `sw` | `*controls.ToggleSwitch` | The control to bind. Its `SetChecked` is used for the initial apply and every model push; its `OnChanged` is replaced. |

**Returns** — `cancel func()`. Calls the property subscription's cancel, then sets `sw.OnChanged(nil)`.

**Example**

```go
darkMode := &core.Property[bool]{}
sw := controls.NewToggleSwitch()
cancel := bind.SwitchChecked(darkMode, sw)
defer cancel()
```

**See also** — [Checked](#checked), [ToggleChecked](#togglechecked)

#### ToggleChecked

Two-way binds `p` to `tb`. While bound, `ToggleChecked` owns `tb`'s
`OnChanged` slot, replacing any previously set callback.

**Syntax**

```go
func ToggleChecked(p *core.Property[bool], tb *controls.ToggleButton) (cancel func())
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `p` | `*core.Property[bool]` | The source property. |
| `tb` | `*controls.ToggleButton` | The control to bind. Its `SetChecked` is used for the initial apply and every model push; its `OnChanged` is replaced. |

**Returns** — `cancel func()`. Calls the property subscription's cancel, then sets `tb.OnChanged(nil)`.

**Example**

```go
bold := &core.Property[bool]{}
tb := controls.NewToggleButton(face, "B")
cancel := bind.ToggleChecked(bold, tb)
defer cancel()
```

**See also** — [Checked](#checked), [SwitchChecked](#switchchecked)

#### Value

Two-way binds `p` to `s`. While bound, `Value` owns `s`'s `OnChanged` slot,
replacing any previously set callback.

**Syntax**

```go
func Value(p *core.Property[float32], s *controls.Slider) (cancel func())
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `p` | `*core.Property[float32]` | The source property. |
| `s` | `*controls.Slider` | The control to bind. Its `SetValue` is used for the initial apply and every model push; its `OnChanged` is replaced. |

**Returns** — `cancel func()`. Calls the property subscription's cancel, then sets `s.OnChanged(nil)`.

**Example**

```go
volume := &core.Property[float32]{}
s := controls.NewSlider()
cancel := bind.Value(volume, s)
defer cancel()
```

**Notes** — Clamp-divergence caveat: if `p`'s value falls outside the slider's valid range, the control silently displays a clamped value while `p` retains the unclamped one.

**See also** — [SelectedIndex](#selectedindex)

#### SelectedIndex

Two-way binds `p` to `cb`. While bound, `SelectedIndex` owns `cb`'s
`OnChanged` slot, replacing any previously set callback.

**Syntax**

```go
func SelectedIndex(p *core.Property[int], cb *controls.ComboBox) (cancel func())
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `p` | `*core.Property[int]` | The source property. |
| `cb` | `*controls.ComboBox` | The control to bind. Its `SetSelectedIndex` is used for the initial apply and every model push; its `OnChanged` is replaced. |

**Returns** — `cancel func()`. Calls the property subscription's cancel, then sets `cb.OnChanged(nil)`.

**Example**

```go
choice := &core.Property[int]{}
cb := controls.NewComboBox(face)
cancel := bind.SelectedIndex(choice, cb)
defer cancel()
```

**Notes** — Clamp-divergence caveat: if `p`'s value falls outside the control's valid range, the control silently displays a clamped value while `p` retains the unclamped one.

**See also** — [Selected](#selected), [ListSelected](#listselected)

#### Selected

Two-way binds `p` to `g`. While bound, `Selected` owns `g`'s `OnChanged`
slot — the group's own `OnChanged(index)`, not any individual member
`RadioButton`'s — replacing any previously set callback.

**Syntax**

```go
func Selected(p *core.Property[int], g *controls.RadioGroup) (cancel func())
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `p` | `*core.Property[int]` | The source property. |
| `g` | `*controls.RadioGroup` | The group to bind. Its `SetSelectedIndex` is used for the initial apply and every model push; its own `OnChanged` is replaced. |

**Returns** — `cancel func()`. Calls the property subscription's cancel, then sets `g.OnChanged(nil)`.

**Example**

```go
size := &core.Property[int]{}
g := controls.NewRadioGroup()
g.Add(controls.NewRadioButton(face, "Small"))
g.Add(controls.NewRadioButton(face, "Large"))
cancel := bind.Selected(size, g)
defer cancel()
```

**Notes** — Clamp-divergence caveat: if `p`'s value falls outside the group's valid member range, the group silently displays a clamped selection (or none, for `-1`) while `p` retains the unclamped one.

**See also** — [SelectedIndex](#selectedindex), [ListSelected](#listselected)

#### ListSelected

Two-way binds `p` to `lv`. While bound, `ListSelected` owns `lv`'s
`OnChanged` slot, replacing any previously set callback.

**Syntax**

```go
func ListSelected(p *core.Property[int], lv *controls.ListView) (cancel func())
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `p` | `*core.Property[int]` | The source property. |
| `lv` | `*controls.ListView` | The list view to bind. Its `SetSelectedIndex` is used for the initial apply and every model push; its `OnChanged` is replaced. |

**Returns** — `cancel func()`. Calls the property subscription's cancel, then sets `lv.OnChanged(nil)`.

**Example**

```go
row := &core.Property[int]{}
lv := controls.NewListView(face, items)
cancel := bind.ListSelected(row, lv)
defer cancel()
```

**Notes** — Clamp-divergence caveat: if `p`'s value falls outside the list's valid range, the control silently displays a clamped selection (or none, for `-1`) while `p` retains the unclamped one. Auto-scroll caveat: `lv.SetSelectedIndex` — the silent setter this binder's model-push direction calls, both on the initial apply and on every subsequent `p.OnChange` — also scrolls the newly-selected row into view. So unlike every other two-way binder above, a `ListSelected` model push doesn't just silently update what the control reports selected — it can also move the viewport, exactly as a user-driven Home/End/Up/Down move would.

**See also** — [Selected](#selected), [SelectedIndex](#selectedindex), [List[T]](#listt)

---

## List[T]

`List[T]` is an observable slice for collection binding: an ordered,
indexable sequence of `T` with two notification channels — a coarse
`OnChanged` fired on every mutation, and a granular `OnChange` fired with
the specific [Change](#changekind--change) that occurred. It is the
observable collection that powers `ListView`/`DataGrid` via [Items](#items)
and the `controls.ListItems` interface (see the collections wiki page for
the consumer side). Not goroutine-safe.

**Constructor**

```go
func NewList[T any](items ...T) *List[T]
```

Creates a new `List` with the provided initial items, copied into an
internal slice (`items` is not aliased).

**Example**

```go
todos := bind.NewList("Buy milk", "Walk dog")
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [Len](#listtlen) | `func (l *List[T]) Len() int` | Returns the number of items in the list. |
| [At](#listtat) | `func (l *List[T]) At(i int) T` | Returns the item at index `i`. |
| [Add](#listtadd) | `func (l *List[T]) Add(items ...T)` | Appends items and notifies subscribers. |
| [Insert](#listtinsert) | `func (l *List[T]) Insert(i int, item T)` | Inserts an item at index `i`. |
| [RemoveAt](#listtremoveat) | `func (l *List[T]) RemoveAt(i int)` | Removes the item at index `i`. |
| [Set](#listtset) | `func (l *List[T]) Set(i int, item T)` | Replaces the item at index `i`. |
| [Replace](#listtreplace) | `func (l *List[T]) Replace(items ...T)` | Replaces the entire list contents. |
| [OnChanged](#listtonchanged) | `func (l *List[T]) OnChanged(f func()) (cancel func())` | Subscribes to coarse (any-mutation) change notifications. |
| [OnChange](#listtonchange) | `func (l *List[T]) OnChange(f func(Change)) (cancel func())` | Subscribes to granular change notifications. |

#### List[T].Len

Returns the number of items in the list.

**Syntax**

```go
func (l *List[T]) Len() int
```

**Returns** — `int`, the current item count.

**Example**

```go
n := todos.Len()
```

#### List[T].At

Returns the item at index `i`.

**Syntax**

```go
func (l *List[T]) At(i int) T
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `i` | `int` | Zero-based index. |

**Returns** — `T`, the item at `i`.

**Example**

```go
first := todos.At(0)
```

**Notes** — Panics if `i` is out of range `[0, Len())` (fail-fast, documented).

#### List[T].Add

Appends items to the list and notifies subscribers (if `len(items) > 0`).

**Syntax**

```go
func (l *List[T]) Add(items ...T)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `items` | `...T` | Items to append, in order. A call with zero items is a no-op — no notification fires. |

**Example**

```go
todos.Add("Read book")
```

**Notes** — Fires the coarse `OnChanged` once, then one granular `ChangeAdd` event per appended item (each with its own final index).

**See also** — [Insert](#listtinsert), [Replace](#listtreplace)

#### List[T].Insert

Inserts `item` at index `i`.

**Syntax**

```go
func (l *List[T]) Insert(i int, item T)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `i` | `int` | Insertion index; must be in `[0, Len()]` (`Len()` appends at the end). |
| `item` | `T` | The item to insert. |

**Notes** — Panics if `i` is out of range. Fires the coarse `OnChanged`, then one granular `ChangeAdd` event at index `i`.

**Example**

```go
todos.Insert(0, "Urgent: renew passport")
```

**See also** — [Add](#listtadd)

#### List[T].RemoveAt

Removes the item at index `i`.

**Syntax**

```go
func (l *List[T]) RemoveAt(i int)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `i` | `int` | Zero-based index of the item to remove. |

**Notes** — Panics if `i` is out of range. Fires the coarse `OnChanged`, then a granular `ChangeRemove` event at index `i`.

**Example**

```go
todos.RemoveAt(0)
```

#### List[T].Set

Replaces the item at index `i` with a new value and notifies subscribers.

**Syntax**

```go
func (l *List[T]) Set(i int, item T)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `i` | `int` | Zero-based index of the item to replace. |
| `item` | `T` | The replacement value. |

**Notes** — Panics if `i` is out of range. Fires the coarse `OnChanged`, then a granular `ChangeReplace` event at index `i`.

**Example**

```go
todos.Set(0, "Buy oat milk")
```

#### List[T].Replace

Replaces the entire list with new items and notifies subscribers (once).

**Syntax**

```go
func (l *List[T]) Replace(items ...T)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `items` | `...T` | The full new contents, copied into a fresh internal slice. |

**Notes** — Fires the coarse `OnChanged` once, then a single granular `ChangeReset` event (`Index: -1`), regardless of how many items changed.

**Example**

```go
todos.Replace("Buy milk", "Walk dog", "Pay rent")
```

**See also** — [Add](#listtadd)

#### List[T].OnChanged

Registers a subscriber to be called when the list changes, with no detail
about what changed.

**Syntax**

```go
func (l *List[T]) OnChanged(f func()) (cancel func())
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `f` | `func()` | Called after every mutation (`Add`, `Insert`, `RemoveAt`, `Set`, `Replace`). |

**Returns** — `cancel func()`, idempotent; removes the subscriber.

**Example**

```go
cancel := todos.OnChanged(func() {
    fmt.Println("todos changed, now", todos.Len(), "items")
})
defer cancel()
```

**See also** — [OnChange](#listtonchange), [Items](#items)

#### List[T].OnChange

Registers a subscriber to be called with granular change details.

**Syntax**

```go
func (l *List[T]) OnChange(f func(Change)) (cancel func())
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `f` | `func(Change)` | Called once per granular mutation with a [Change](#changekind--change) describing it. |

**Returns** — `cancel func()`, idempotent; removes the subscriber.

**Example**

```go
cancel := todos.OnChange(func(c bind.Change) {
    if c.Kind == bind.ChangeRemove {
        fmt.Println("removed index", c.Index)
    }
})
defer cancel()
```

**Notes** — Granular events fire after the coarse `OnChanged` notification, and only after the mutation is fully applied — a subscriber processing the first event of a multi-item `Add` already observes the final list state.

**See also** — [OnChanged](#listtonchanged), [ChangeKind / Change](#changekind--change)

---

## ChangeKind / Change

```go
type ChangeKind = controls.ListChangeKind

const (
    ChangeAdd     = controls.ListChangeAdd
    ChangeRemove  = controls.ListChangeRemove
    ChangeReplace = controls.ListChangeReplace
    ChangeReset   = controls.ListChangeReset
)

type Change = controls.ListChange
```

`ChangeKind` and `Change` are **type aliases**, not independent
definitions — `ChangeKind` is exactly `controls.ListChangeKind` and `Change`
is exactly `controls.ListChange`. They are declared in `controls` rather
than `bind` because `controls.ListView` needs a `ListItems` interface whose
`OnChange` method names this payload type, and `bind` already imports
`controls` (for `Items`, and for `ListSelected`) — the reverse edge would
be an import cycle. Declaring the canonical type in `controls` and
aliasing it here keeps `bind.ChangeKind`/`bind.Change`'s published names
and values exactly as they would be for an independent definition, while
`*bind.List[T]` also structurally satisfies `controls.ListItems` as the
same underlying type. See the collections wiki page for
`controls.ListChangeKind`/`controls.ListChange`/`controls.ListItems` from
the consumer side.

| Name | Value |
|---|---|
| `ChangeAdd` | `controls.ListChangeAdd` — an `Add` or `Insert` produced this event; `Index` is the new item's position. |
| `ChangeRemove` | `controls.ListChangeRemove` — a `RemoveAt` produced this event; `Index` is the removed position. |
| `ChangeReplace` | `controls.ListChangeReplace` — a `Set` produced this event; `Index` is the replaced position. |
| `ChangeReset` | `controls.ListChangeReset` — a `Replace` produced this event; `Index` is always `-1`. |

### Fields (Change)

| Name | Type | Description |
|---|---|---|
| `Kind` | `ChangeKind` | Which mutation occurred. |
| `Index` | `int` | The affected position, or `-1` for a full reset. |

**Notes** — `*bind.List[string]` satisfies `controls.ListItems` (`Len`,
`At(int) string`, `OnChange(func(ListChange)) func()`) precisely because of
this aliasing — `controls.NewListView` can accept a `*bind.List[string]`
directly without `controls` importing `bind`. `ListItems.At` is typed
`string`, so today only `*bind.List[string]` — not an arbitrary `List[T]`
— can back a `ListView`.

**See also** — [List[T]](#listt)

---

## Items

Binds a `List[T]` to a `StackPanel`: on any list change, the panel is
cleared and rebuilt — `panel.Clear()` then `panel.Add(makeItem(item, index))`
for each item, in order.

**Syntax**

```go
func Items[T any](l *List[T], panel *controls.StackPanel, makeItem func(item T, index int) core.Widget) (cancel func())
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `l` | `*List[T]` | The observable list to render. |
| `panel` | `*controls.StackPanel` | The panel whose children are fully rebuilt on every list change. |
| `makeItem` | `func(item T, index int) core.Widget` | Builds the widget for one item. Called once per item, in list order, on every rebuild pass. |

**Returns** — `cancel func()`. Cancels the `List.OnChanged` subscription driving the rebuild; the panel's existing children are left as-is (not cleared) when canceled.

**Example**

```go
todos := bind.NewList("Buy milk", "Walk dog")
panel := controls.NewStackPanel(controls.Vertical)

cancel := bind.Items(todos, panel, func(item string, index int) core.Widget {
    return controls.NewTextBlock(face, item)
})
defer cancel()

todos.Add("Pay rent") // panel is cleared and rebuilt with 3 rows
```

**Notes** — v0 does a full rebuild on every change (virtualization is a
later phase) — prefer this for small-to-medium lists, or a `ListView` bound
directly to the `List` (see [ListSelected](#listselected)) for large,
virtualized collections. Reentrancy: if a list mutation occurs during
`makeItem()` (while a rebuild is in progress), the rebuild coalesces into
one additional rebuild pass after the outer one completes — each pass
snapshots the current items and iterates the snapshot, so iteration is
always bounded; a `makeItem()` that mutates the list on *every* invocation
across every pass will never converge and is unsupported.

**See also** — [List[T]](#listt), [OneWay](#oneway)
