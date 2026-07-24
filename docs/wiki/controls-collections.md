# controls: collections & scrolling

This group covers fluo's list-shaped and scroll-shaped widgets. `ListView`
and `DataGrid` are **virtualized**: instead of holding one widget per row of
data, each keeps a small pool of `TextBlock`s sized to the current viewport
and re-texts/re-arranges that pool as the view scrolls — a row's on-screen
widget is not a stable, per-item identity, it's whichever pool slot the
virtualizer currently maps to that screen position. `TreeView` is
deliberately **not** virtualized: its content is a finite, already-in-memory
tree, so every currently-visible row is measured and drawn in full on every
pass, with no pooling. `Expander` is a plain collapsible container with no
chrome of its own beyond its header. `ScrollViewer` wraps a single arbitrary
child with two-axis scrolling (vertical always, horizontal added alongside
it) and is the scrollbar/wheel/drag machinery every list-shaped control here
mirrors internally. `ListItems`, `ListChange`, and `ListChangeKind` are the
minimal observable-sequence contract `ListView` consumes — the seam the
`bind` package's `List[T]` satisfies structurally, so a `*bind.List[string]`
can be passed to `NewListView` directly without `controls` importing `bind`.

**Import:** `github.com/0xdreadnaught/fluo/controls`

## Contents
- [ListView](#listview)
- [TreeView](#treeview)
- [TreeNode](#treenode)
- [DataGrid](#datagrid)
- [Column](#column)
- [Expander](#expander)
- [ScrollViewer](#scrollviewer)
- [ListItems](#listitems)
- [ListChange](#listchange)
- [ListChangeKind](#listchangekind)

---

## ListView

`ListView` virtualizes a single-column, uniform-row-height list of strings:
only the rows intersecting the current viewport are realized, into a pool of
reused `TextBlock`s (see `ArrangeContent`). v0 is string rows only — custom
row factories are a later phase. Selection is a single index (`-1` meaning
none), set either programmatically (`SetSelectedIndex`, silent) or by the
user (row click, or Up/Down/Home/End while focused — both funneled through
an internal `selectUser` path that fires `OnChanged` only on an actual
change). Both selection paths auto-scroll the result into view.

**Constructor**

```go
func NewListView(face *text.Face, items ListItems) *ListView
```

Returns a `ListView` rendering `items` with `face`, styled from
`theme.Active()` at construction (rebuild to re-theme). It subscribes to
`items.OnChange` — the granular change channel — so that any list mutation
invalidates measure and arrange; v0 does not use the change payload for
incremental updates, a full re-layout recomputes the visible range and
re-texts the pool from scratch. `face` may be `nil` (rows measure/draw as
empty, matching `TextBlock`'s own nil-face convention).

**Example**

```go
items := bind.NewList("Alice", "Bob", "Carol")
lv := controls.NewListView(face, items)
lv.OnChanged(func(i int) {
    fmt.Println("selected row:", i)
})
// ... add lv to a widget tree ...
defer lv.Dispose()
```

**Notes** — `ListView` is fluo's first disposable control: every other v0
control holds no external resource, but a `ListView`'s subscription to
`items` otherwise outlives the `ListView` itself (the list holds the
callback, not the other way around). See [Dispose](#listviewdispose).

### Methods

| Method | Signature | Description |
|---|---|---|
| [RowHeight](#listviewrowheight) | `RowHeight() float32` | Returns the current row height. |
| [SetRowHeight](#listviewsetrowheight) | `SetRowHeight(h float32) *ListView` | Overrides the row height. |
| [OffsetY](#listviewoffsety) | `OffsetY() float32` | Returns the current vertical scroll offset. |
| [OffsetX](#listviewoffsetx) | `OffsetX() float32` | Returns the current horizontal scroll offset. |
| [ScrollTo](#listviewscrollto) | `ScrollTo(y float32) *ListView` | Requests a new vertical offset. |
| [ScrollToX](#listviewscrolltox) | `ScrollToX(x float32) *ListView` | Requests a new horizontal offset. |
| [SelectedIndex](#listviewselectedindex) | `SelectedIndex() int` | Returns the current selection, or -1. |
| [SetSelectedIndex](#listviewsetselectedindex) | `SetSelectedIndex(i int) *ListView` | Sets the selection programmatically. |
| [OnChanged](#listviewonchanged) | `OnChanged(fn func(int)) *ListView` | Sets the user-driven selection callback. |
| [Dispose](#listviewdispose) | `Dispose()` | Releases the subscription to `items`. |
| [Children](#listviewchildren) | `Children() []core.Widget` | Returns the currently realized row pool. |
| [MeasureContent](#listviewmeasurecontent) | `MeasureContent(available render.Size) render.Size` | Reports the fixed {160,240} default desired size. |
| [ArrangeContent](#listviewarrangecontent) | `ArrangeContent(bounds render.Rect)` | Clamps scroll offsets and realizes visible rows. |
| [Render](#listviewrender) | `Render(r render.Renderer)` | Draws the sunken well and the selection band. |
| [ClipRect](#listviewcliprect) | `ClipRect() (render.Rect, bool)` | Clips rows to the bevel-inset content bounds. |
| [RenderOverlay](#listviewrenderoverlay) | `RenderOverlay(r render.Renderer)` | Draws scroll thumbs and the focus ring. |
| [AcceptsFocus](#listviewacceptsfocus) | `AcceptsFocus() bool` | Always `true`. |
| [OnFocusChanged](#listviewonfocuschanged) | `OnFocusChanged(focused bool)` | Tracks focus state. |
| [OnPointer](#listviewonpointer) | `OnPointer(e *input.PointerEvent)` | Handles wheel, thumb drag, and row click-to-select. |
| [OnKey](#listviewonkey) | `OnKey(e *input.KeyEvent)` | Handles Up/Down/Home/End row navigation. |

#### ListView.RowHeight

Returns the current row height (the default set at construction, or a later
`SetRowHeight` override).

**Syntax**

```go
func (l *ListView) RowHeight() float32
```

**Returns** — `float32`, the row height in logical px.

**See also** — [SetRowHeight](#listviewsetrowheight)

#### ListView.SetRowHeight

Overrides the row height.

**Syntax**

```go
func (l *ListView) SetRowHeight(h float32) *ListView
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `h` | `float32` | New row height in logical px. |

**Returns** — `*ListView` for chaining.

**Example**

```go
lv.SetRowHeight(28)
```

**Notes** — Purely an arrange-time concern (which rows are visible and where
they land, not `ListView`'s own fixed desired size), so it invalidates
arrange only, not measure.

#### ListView.OffsetY

Returns the current vertical scroll offset.

**Syntax**

```go
func (l *ListView) OffsetY() float32
```

**Returns** — `float32`, clamped to `[0, max(0, totalHeight-viewport.H)]` as
of the last arrange pass — mirrors `ScrollViewer.OffsetY` exactly.

#### ListView.OffsetX

Returns the current horizontal scroll offset.

**Syntax**

```go
func (l *ListView) OffsetX() float32
```

**Returns** — `float32`, clamped to `[0, max(0, contentWidth-viewport.W)]`
as of the last arrange pass, where `contentWidth` is the widest row's
measured text width plus padding — mirrors `ScrollViewer.OffsetX` exactly.

#### ListView.ScrollTo

Requests a new vertical offset.

**Syntax**

```go
func (l *ListView) ScrollTo(y float32) *ListView
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `y` | `float32` | Requested vertical offset (pre-clamp). |

**Returns** — `*ListView` for chaining.

**Notes** — Clamped on the next layout pass (`virtualizer.layout`, invoked
from `ArrangeContent`, is the single source of truth for clamping), so
`OffsetY` may not reflect `y` until layout runs again.

#### ListView.ScrollToX

Requests a new horizontal offset.

**Syntax**

```go
func (l *ListView) ScrollToX(x float32) *ListView
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `x` | `float32` | Requested horizontal offset (pre-clamp). |

**Returns** — `*ListView` for chaining.

**Notes** — Clamped on the next layout pass, mirroring `ScrollTo` on the X
axis.

#### ListView.SelectedIndex

Returns the current selection, or `-1` if none.

**Syntax**

```go
func (l *ListView) SelectedIndex() int
```

**Returns** — `int`, the selected row index or `-1`.

**Notes** — `ListView` does **not** re-clamp or track selection across
external list mutations: after a `RemoveAt`/`Remove` on the bound list,
`SelectedIndex` may name a shifted or out-of-range logical row with no
`OnChanged` fired. Callers needing stable selection should re-set it (via
`SetSelectedIndex`) after mutating the list.

#### ListView.SetSelectedIndex

Sets the selection programmatically.

**Syntax**

```go
func (l *ListView) SetSelectedIndex(i int) *ListView
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `i` | `int` | Requested index, clamped into `[-1, count-1]`; `-1` is always a valid, explicit "no selection" value. |

**Returns** — `*ListView` for chaining.

**Example**

```go
lv.SetSelectedIndex(2)
```

**Notes** — Silent: never fires `OnChanged`, matching the package's uniform
setter convention (`ComboBox.SetSelectedIndex`, `Slider.SetValue`, ...) —
this is also the setter `bind.ListSelected`'s model-push direction calls, so
its silence keeps that binder echo-safe. Auto-scrolls the resulting
selection into view. Invalidates arrange unconditionally, even when the
clamped index is unchanged, because the selected row's fill/text color must
still be recomputed. See `SelectedIndex`'s Notes for list-mutation caveats.

#### ListView.OnChanged

Sets the callback fired with the new index whenever the user changes the
selection.

**Syntax**

```go
func (l *ListView) OnChanged(fn func(int)) *ListView
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `fn` | `func(int)` | Called with the new selected index on a user-driven change. A `nil` value is a valid, silent no-op. Replaces any previously set callback. |

**Returns** — `*ListView` for chaining.

**Notes** — Fires by clicking a row or navigating with Up/Down/Home/End
while focused — never for a programmatic `SetSelectedIndex`.

#### ListView.Dispose

Releases `l`'s subscription to `items`' granular change channel.

**Syntax**

```go
func (l *ListView) Dispose()
```

**Notes** — Callers **must** call `Dispose` when a `ListView` is removed
from the tree (e.g. a rebuild's cancel path, alongside any `bind` cancels)
to stop it reacting to further list mutations — the list otherwise holds
the callback and keeps the `ListView` reachable/reactive indefinitely. Safe
to call more than once (the underlying cancel is idempotent); calling it
leaves the `ListView` otherwise usable, just no longer reactive to the
list.

**See also** — [DataGrid.Dispose](#datagriddispose)

#### ListView.Children

Returns the currently realized row pool.

**Syntax**

```go
func (l *ListView) Children() []core.Widget
```

**Returns** — `[]core.Widget`, a copy of the pool (for hit-testing/render);
mutating the returned slice does not affect the `ListView`. `nil` when the
pool is empty.

**Notes** — This is the pool of reused `TextBlock`s, not one widget per data
item — its length is bounded by the visible row count, not `items.Len()`.

#### ListView.MeasureContent

Reports `ListView`'s fixed desired size.

**Syntax**

```go
func (l *ListView) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | Space offered by the parent layout. |

**Returns** — `render.Size`, `{160, 240}` clamped to `available` on each
axis — a `ListView` never asks its parent for more room than offered, even
though (unlike `ScrollViewer`) its desired size never depends on its
virtualized, unboundedly long content at all.

**Notes** — Invoked by the layout engine (`core.MeasureWidget`), not
typically called directly by application code.

#### ListView.ArrangeContent

The single source of truth for scroll-offset clamping (on both axes) and
row realization.

**Syntax**

```go
func (l *ListView) ArrangeContent(bounds render.Rect)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `bounds` | `render.Rect` | Bounds assigned by the parent layout. |

**Notes** — Insets `bounds` by the 2px sunken bevel, computes the viewport
within that rect (minus the vertical thumb's gutter on the right, reserved
only when content overflows vertically, and the horizontal thumb's gutter
on the bottom, reserved only when the widest row's natural width exceeds
the resulting viewport width), clamps the scroll offset on both axes,
determines the visible row range, and resizes the `TextBlock` pool to
exactly that many rows. **Pool reuse contract**: existing pool entries are
re-texted in place (only when the underlying value actually changed) rather
than reallocated; a pool slot's identity does not correspond to a stable
data-row identity — as the user scrolls, `pool[i]` is re-texted to whatever
row now occupies that screen position. Never assume a 1:1 widget-per-item
relationship, and never cache a reference to a specific row's `TextBlock`
across a scroll. Invoked by the layout engine (`core.ArrangeWidget`).

#### ListView.Render

Draws the sunken well frame and the selection band.

**Syntax**

```go
func (l *ListView) Render(r render.Renderer)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | Target renderer. |

**Notes** — Draws the sunken `WindowWell` frame across `l`'s full bounds
first (the classic list-box well), then fills the selected row's band with
`Highlight` before its `TextBlock` renders on top — a no-op for the band
when nothing is selected, or the selected index isn't currently realized
(scrolled out of the visible range). Invoked by the render engine, not
typically called directly.

#### ListView.ClipRect

Clips realized rows to the bevel-inset content bounds.

**Syntax**

```go
func (l *ListView) ClipRect() (render.Rect, bool)
```

**Returns** — `(render.Rect, bool)`: the clip rect and `true`, always.
Implements `core.ClipProvider`.

**Notes** — Clips to the well's inner frame (bevel-inset), not `l`'s raw
outer bounds, so a partially-scrolled row at the top/bottom edge of the
viewport is cropped at the well's inner frame rather than bleeding onto the
sunken bevel. The thumb gutter is included in this rect, so the scroll
thumb itself (drawn in `RenderOverlay`, which runs after the clip is
popped) is never cropped either way.

#### ListView.RenderOverlay

Draws the scroll thumbs and the focus ring.

**Syntax**

```go
func (l *ListView) RenderOverlay(r render.Renderer)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | Target renderer. |

**Notes** — Draws the classic track+thumb for each axis that has content to
scroll to — vertical along the right edge, horizontal along the bottom —
above the clipped rows, then the focus ring while focused, drawn last so it
sits above both thumbs. Implements `core.OverlayRenderer`.

#### ListView.AcceptsFocus

Reports whether `l` accepts keyboard focus.

**Syntax**

```go
func (l *ListView) AcceptsFocus() bool
```

**Returns** — `bool`, always `true` — v0 has no `SetEnabled`/disabled
concept, unlike `Slider`/`ComboBox`. Implements `input.Focusable`.

#### ListView.OnFocusChanged

Tracks focus for the focus-ring overlay and keyboard navigation.

**Syntax**

```go
func (l *ListView) OnFocusChanged(focused bool)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `focused` | `bool` | New focus state. |

**Notes** — Implements `input.FocusHandler`.

#### ListView.OnPointer

Handles wheel scroll, thumb drag, and row click-to-select.

**Syntax**

```go
func (l *ListView) OnPointer(e *input.PointerEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.PointerEvent` | The pointer event; `e.Handled` is set when consumed. |

**Notes** — Wheel scrolls vertically by a fixed step per notch by default,
and horizontally instead when Shift is held (matching `ScrollViewer`'s
Shift+Wheel convention) — always handled; unlike `ScrollViewer`, a plain
wheel never falls back to X even when only X overflows, since a
`ListView`'s rows are its primary scroll axis. A Press inside the current
vertical thumb rect starts a vertical drag (checked first); otherwise a
Press inside the horizontal thumb rect starts a horizontal drag; otherwise
a Press landing on a real row selects it as a user-driven change, while a
Press over a gutter or empty space below a short list is left unhandled.
Implements `input.PointerHandler`.

#### ListView.OnKey

Handles Up/Down/Home/End row navigation.

**Syntax**

```go
func (l *ListView) OnKey(e *input.KeyEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.KeyEvent` | The key event; `e.Handled` is set when consumed. |

**Notes** — Up/Down move the selection by one row (never landing on `-1`,
even starting from no selection — both land on row 0), Home/End jump to the
first/last row; all four are user-driven (fire `OnChanged` on a real
change) and auto-scroll the result into view. Ignored entirely for anything
but a key-press action, or when there are no rows to select among.
Implements `input.KeyHandler`.

---

## TreeView

`TreeView` is a clickable, focusable, token-styled tree of `TreeNode`s: rows
are the depth-first flatten of every currently-expanded node, each drawn as
an optional `'v'`/`'>'` chevron (only when the node has children; a text
glyph, never rotated — `'>'` collapsed, `'v'` expanded) followed by its
label, indented 16px per depth level.

**v0 is non-virtualized** — deliberate and documented: unlike `ListView`
(whose items are an unboundedly long external source and so must be
virtualized), a `TreeView`'s content is a finite, already-in-memory tree —
every currently visible row is measured and drawn every pass, with no
pooling or windowing. A caller with a very large or deep tree that needs
virtualization should wrap a `TreeView` in a `ScrollViewer` (or similar)
rather than expect one built in; that is out of scope for v0.

Selection mirrors `ListView`'s convention: a single `*TreeNode` (`nil`
meaning none), set programmatically (`SetSelected`, silent — see its Notes
for the auto-expand-ancestors behavior that sets it apart from a plain
silent setter) or by the user (label click, or Up/Down/Left/Right while
focused).

**Constructor**

```go
func NewTreeView(face *text.Face, roots ...*TreeNode) *TreeView
```

Returns a `TreeView` over `roots`, styled from `theme.Active()` at
construction, drawing labels/chevrons with `face` (`face` may be `nil`, per
`TextBlock`'s own nil-face convention — the tree still renders its well
frame but no rows). Every root starts however its own `TreeNode.Expanded`
was left (collapsed by default) — `TreeView` holds no copy of `roots`'
state, just the `*TreeNode` pointers themselves.

**Example**

```go
root := controls.NewTreeNode("src",
    controls.NewTreeNode("main.go"),
    controls.NewTreeNode("controls",
        controls.NewTreeNode("listview.go"),
        controls.NewTreeNode("treeview.go"),
    ),
)
root.SetExpanded(true)

tv := controls.NewTreeView(face, root)
tv.OnChanged(func(n *controls.TreeNode) {
    fmt.Println("selected:", n.Label)
})
```

**Notes** — Ownership: a `TreeNode` belongs to at most one `TreeView` at a
time (see [TreeNode](#treenode)'s Notes). Sharing the same node across two
`TreeView`s is unsupported.

### Methods

| Method | Signature | Description |
|---|---|---|
| [Selected](#treeviewselected) | `Selected() *TreeNode` | Returns the currently selected node, or nil. |
| [SetSelected](#treeviewsetselected) | `SetSelected(n *TreeNode) *TreeView` | Sets the selection programmatically. |
| [OnChanged](#treeviewonchanged) | `OnChanged(fn func(*TreeNode)) *TreeView` | Sets the user-driven selection callback. |
| [MeasureContent](#treeviewmeasurecontent) | `MeasureContent(available render.Size) render.Size` | Recomputes the flattened rows and reports their total size. |
| [Render](#treeviewrender) | `Render(r render.Renderer)` | Draws the well, chevrons, and labels. |
| [RenderOverlay](#treeviewrenderoverlay) | `RenderOverlay(r render.Renderer)` | Draws the focus ring while focused. |
| [AcceptsFocus](#treeviewacceptsfocus) | `AcceptsFocus() bool` | Always `true`. |
| [OnFocusChanged](#treeviewonfocuschanged) | `OnFocusChanged(focused bool)` | Tracks focus state. |
| [OnPointer](#treeviewonpointer) | `OnPointer(e *input.PointerEvent)` | Handles chevron toggle and row click-to-select. |
| [OnKey](#treeviewonkey) | `OnKey(e *input.KeyEvent)` | Handles Up/Down/Left/Right navigation. |

#### TreeView.Selected

Returns the currently selected node, or `nil` if none.

**Syntax**

```go
func (t *TreeView) Selected() *TreeNode
```

**Returns** — `*TreeNode`, or `nil`.

#### TreeView.SetSelected

Sets the selection programmatically.

**Syntax**

```go
func (t *TreeView) SetSelected(n *TreeNode) *TreeView
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `n` | `*TreeNode` | Node to select; may be any node reachable from the tree's roots, including one hidden beneath a collapsed ancestor. |

**Returns** — `*TreeView` for chaining.

**Example**

```go
tv.SetSelected(root.Children[1])
```

**Notes** — Silent: never fires `OnChanged`, matching fluo's uniform
contract that programmatic setters are silent. Unlike the user-driven path
(which only ever targets an already-visible row), `SetSelected` accepts any
node reachable from the tree's roots — including one currently hidden
beneath a collapsed ancestor — and auto-expands every ancestor on the path
to `n` so the newly selected node actually becomes visible on the next
layout pass. No reachability check is performed: a foreign node never part
of this tree (or one removed since construction) simply never appears in
the flattened rows, so no ancestor gets expanded and nothing is highlighted
— a soft "no row highlighted" failure mode, not a panic.

#### TreeView.OnChanged

Sets the callback fired with the newly selected node whenever the user
changes the selection.

**Syntax**

```go
func (t *TreeView) OnChanged(fn func(*TreeNode)) *TreeView
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `fn` | `func(*TreeNode)` | Called with the newly selected node on a user-driven change. A `nil` value is a valid, silent no-op. Replaces any previously set callback. |

**Returns** — `*TreeView` for chaining.

**Notes** — Fires by clicking a row's label, or navigating with
Up/Down/Left/Right while focused — never for a programmatic `SetSelected`.

#### TreeView.MeasureContent

Recomputes the flattened row list and reports the tree's total content
size.

**Syntax**

```go
func (t *TreeView) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | Space offered by the parent layout (ignored — see Notes). |

**Returns** — `render.Size`, `{max row width, rowH * row count}`.

**Notes** — Recomputes `t`'s internal flattened-row cache here (measure
time, not arrange), since a `TreeView`'s content is fully known up front —
unlike `ListView`'s virtualized content, its desired size is simply the
real total content size, ignoring `available`, matching `TextBlock`'s own
"measure the real thing" convention. A `nil` face measures every row's
label to zero width; `rowH` itself collapses to the nil-safe padding-only
value.

#### TreeView.Render

Draws the sunken well, chevrons, and labels.

**Syntax**

```go
func (t *TreeView) Render(r render.Renderer)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | Target renderer. |

**Notes** — Draws the sunken `WindowWell` frame across `t`'s full bounds
first, then every row: the selection band (`Highlight`, full content width)
if the row's node is selected, the chevron glyph (only for a row whose node
has children), then the label (`HighlightText` when selected, else
`WindowText`). Rows are skipped entirely (well still drawn) with a `nil`
face.

#### TreeView.RenderOverlay

Draws the focus ring while focused.

**Syntax**

```go
func (t *TreeView) RenderOverlay(r render.Renderer)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | Target renderer. |

#### TreeView.AcceptsFocus

Reports whether `t` accepts keyboard focus.

**Syntax**

```go
func (t *TreeView) AcceptsFocus() bool
```

**Returns** — `bool`, always `true` — v0 has no disabled concept, matching
`ListView`.

#### TreeView.OnFocusChanged

Tracks focus for the focus-ring overlay and keyboard navigation.

**Syntax**

```go
func (t *TreeView) OnFocusChanged(focused bool)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `focused` | `bool` | New focus state. |

#### TreeView.OnPointer

Handles chevron toggle and row click-to-select.

**Syntax**

```go
func (t *TreeView) OnPointer(e *input.PointerEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.PointerEvent` | The pointer event; `e.Handled` is set when consumed. |

**Notes** — A Press landing on a real row toggles that row's node if it
falls within the row's chevron hit zone (`[indent, indent+16px)` in
row-local x, reserved even for a leaf row so every row's label lines up
regardless of whether it has a chevron), else selects that row's node as a
user-driven change. Marked handled whenever the press lands on a real row
at all — whether the resulting toggle/select was a real state change or
not; a press outside any row (or with a nil face) is left unhandled.

#### TreeView.OnKey

Handles Up/Down/Left/Right keyboard navigation.

**Syntax**

```go
func (t *TreeView) OnKey(e *input.KeyEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.KeyEvent` | The key event; `e.Handled` is set when consumed. |

**Notes** — Up/Down move the selection by one row over the flattened rows
(never landing on no-selection once there is at least one row). Right
expands the selected row, or descends to its first child if already
expanded; Left collapses it, or jumps to its parent. Unlike Up/Down,
Right/Left are only meaningful relative to a current selection, so they are
marked handled only when one exists — with no selection they fall through
unhandled rather than silently doing nothing while claiming to have acted.
Ignored entirely for anything but a key-press action, or when there are no
rows at all.

---

## TreeNode

`TreeNode` is one node of a tree shown by a `TreeView`: a label, zero or
more children, and its own expanded/collapsed state (collapsed by default —
the zero value of a fresh `TreeNode` is a valid, collapsed, childless node).
A `TreeNode` is shared, mutable state: the same `*TreeNode` instance passed
to `NewTreeView` is read back by `TreeView.Selected`/`OnChanged`, so callers
can hold onto node pointers to inspect or drive them later (e.g. calling
`SetExpanded` from outside the tree).

**Constructor**

```go
func NewTreeNode(label string, children ...*TreeNode) *TreeNode
```

Returns a new, collapsed `TreeNode` with the given `label` and (optional)
`children`.

**Example**

```go
node := controls.NewTreeNode("Documents",
    controls.NewTreeNode("report.docx"),
    controls.NewTreeNode("notes.txt"),
)
```

**Notes** — Ownership: a `TreeNode` belongs to at most one `TreeView` at a
time. Every node reachable from a `TreeView`'s roots is tagged with that
`TreeView` during construction (regardless of current expand state), and
re-tagged onto any node newly reached by a later flatten pass — which
covers a node appended to an existing owned node's `Children` slice
**after** construction, once its ancestor chain is expanded enough to make
it visible. Sharing the same node across two `TreeView`s is unsupported:
the node simply holds whichever `TreeView` tagged it most recently, so only
that one `TreeView`'s layout is invalidated by a `SetExpanded` call — the
other silently sees nothing.

### Fields

| Name | Type | Description |
|---|---|---|
| `Label` | `string` | The node's displayed text. |
| `Children` | `[]*TreeNode` | The node's child nodes, in display order. Only walked (and shown) while the node itself is expanded. |

### Methods

| Method | Signature | Description |
|---|---|---|
| [Expanded](#treenodeexpanded) | `Expanded() bool` | Reports whether the node's children are currently shown. |
| [SetExpanded](#treenodesetexpanded) | `SetExpanded(v bool) *TreeNode` | Sets the expanded state directly. |

#### TreeNode.Expanded

Reports whether `n`'s children (if any) are currently shown.

**Syntax**

```go
func (n *TreeNode) Expanded() bool
```

**Returns** — `bool`.

#### TreeNode.SetExpanded

Sets `n`'s expanded state directly.

**Syntax**

```go
func (n *TreeNode) SetExpanded(v bool) *TreeNode
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `bool` | New expanded state. |

**Returns** — `*TreeNode` for chaining.

**Example**

```go
node.SetExpanded(true)
```

**Notes** — If `n` is currently owned by a `TreeView` (see the type's
Ownership note), immediately invalidates that `TreeView`'s measure — for
both `true` and `false` — so the changed row set is picked up on the very
next layout pass, exactly as if the change had come from a chevron click or
Left/Right keyboard navigation. This is what makes a direct external call
(bypassing `TreeView`'s own click/keyboard paths) actually visible: without
it, a caller mutating a node directly (e.g. from bound model code) could
silently desync from what's on screen. Calling it on a node not (yet) owned
by any `TreeView` is a harmless no-op beyond flipping the flag.

---

## DataGrid

`DataGrid` is a virtualized, multi-column grid: a fixed header row (drawn
directly, each column its own raised bevel cell) sitting above a
**virtualized body** that reuses the same uniform-row virtualizer `ListView`
is built on for its viewport/scroll/thumb math. Only the body scrolls; the
header's own rect depends solely on the grid's arranged bounds, never on
scroll offset — but the header scrolls **horizontally in sync** with the
body (both read the same horizontal offset — see `ArrangeContent`'s Notes).

Column widths are resolved against the body viewport's width exactly like
`Grid`'s own column tracks: `Px` columns get their fixed size, `Star`
columns split the remaining space by weight. `AutoTrack` is **not**
supported in v0 — a `DataGrid` column has no natural "desired content
width" the way a `Grid` cell's child widget does — `SetColumns` panics
immediately, naming the offending column's index.

Cells are realized into a pool of `TextBlock`s sized exactly `visibleRows ×
numColumns` (row-major), reused and re-texted across scroll/column changes
the same way `ListView`'s row pool is. Selection is a single row index
(`-1` == none), with the same silent-setter / user-driven-`OnChanged` /
scroll-into-view contract as `ListView`. A hovered row is tracked
internally but never painted — classic lists have no hover fill.

v0 grid lines are horizontal only — a 1px line beneath every realized row;
there are no vertical column separators.

**Constructor**

```go
func NewDataGrid(face *text.Face) *DataGrid
```

Returns an empty `DataGrid` (no columns, zero rows) drawing header titles
and cell text with `face`, styled from `theme.Active()` at construction.

**Example**

```go
grid := controls.NewDataGrid(face)
grid.SetColumns(
    controls.Column{Title: "Name", Width: controls.Px(120), Value: func(row int) string { return names[row] }},
    controls.Column{Title: "Email", Width: controls.Star(1), Value: func(row int) string { return emails[row] }},
)
grid.SetRowCount(len(names))
grid.OnChanged(func(i int) {
    fmt.Println("selected row:", i)
})
defer grid.Dispose()
```

**Notes** — Unlike `ListView`, `DataGrid` exposes no vertical `OffsetY`
getter or `ScrollTo` setter — only the horizontal offset (`OffsetX`/
`ScrollToX`) is exposed programmatically. Vertical position can still be
driven indirectly via `SetSelectedIndex`'s auto-scroll, or directly by the
user (wheel/drag/Up/Down).

### Methods

| Method | Signature | Description |
|---|---|---|
| [SetColumns](#datagridsetcolumns) | `SetColumns(cols ...Column) *DataGrid` | Replaces the column set. |
| [SetRowCount](#datagridsetrowcount) | `SetRowCount(n int) *DataGrid` | Sets the row count. |
| [RowCount](#datagridrowcount) | `RowCount() int` | Returns the current row count. |
| [OffsetX](#datagridoffsetx) | `OffsetX() float32` | Returns the current horizontal scroll offset. |
| [ScrollToX](#datagridscrolltox) | `ScrollToX(x float32) *DataGrid` | Requests a new horizontal offset. |
| [SelectedIndex](#datagridselectedindex) | `SelectedIndex() int` | Returns the current selection, or -1. |
| [SetSelectedIndex](#datagridsetselectedindex) | `SetSelectedIndex(i int) *DataGrid` | Sets the selection programmatically. |
| [OnChanged](#datagridonchanged) | `OnChanged(fn func(int)) *DataGrid` | Sets the user-driven selection callback. |
| [Dispose](#datagriddispose) | `Dispose()` | No-op; present for uniform cancel paths. |
| [Children](#datagridchildren) | `Children() []core.Widget` | Returns the currently realized cell pool. |
| [MeasureContent](#datagridmeasurecontent) | `MeasureContent(available render.Size) render.Size` | Reports the fixed {240,200} default desired size. |
| [ArrangeContent](#datagridarrangecontent) | `ArrangeContent(bounds render.Rect)` | Resolves columns and realizes visible cells. |
| [Render](#datagridrender) | `Render(r render.Renderer)` | Draws the well, header cells, selection band, and grid lines. |
| [ClipRect](#datagridcliprect) | `ClipRect() (render.Rect, bool)` | Clips body cells below the header strip. |
| [RenderOverlay](#datagridrenderoverlay) | `RenderOverlay(r render.Renderer)` | Draws scroll thumbs and the focus ring. |
| [AcceptsFocus](#datagridacceptsfocus) | `AcceptsFocus() bool` | Always `true`. |
| [OnFocusChanged](#datagridonfocuschanged) | `OnFocusChanged(focused bool)` | Tracks focus state. |
| [OnPointer](#datagridonpointer) | `OnPointer(e *input.PointerEvent)` | Handles wheel, thumb drag, row click, and hover. |
| [OnKey](#datagridonkey) | `OnKey(e *input.KeyEvent)` | Handles Up/Down row navigation. |

#### DataGrid.SetColumns

Replaces the column set.

**Syntax**

```go
func (g *DataGrid) SetColumns(cols ...Column) *DataGrid
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `cols` | `...Column` | New column set, in display order. Panics if any column's `Width` is an `AutoTrack`. |

**Returns** — `*DataGrid` for chaining.

**Example**

```go
grid.SetColumns(
    controls.Column{Title: "ID", Width: controls.Px(60)},
    controls.Column{Title: "Name", Width: controls.Star(1)},
)
```

**Notes** — Re-validates every column's `Width`: `AutoTrack` is not
supported in v0, so any `AutoTrack` column panics immediately, naming its
index — matching `Grid`'s own "fail fast on invalid input" convention.
Invalidates measure.

#### DataGrid.SetRowCount

Sets the row count.

**Syntax**

```go
func (g *DataGrid) SetRowCount(n int) *DataGrid
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `n` | `int` | New row count — drives both the virtualizer's total height and which row indices `Column.Value` funcs are called with. |

**Returns** — `*DataGrid` for chaining.

**Notes** — v0's data model is a plain count plus per-column `Value` funcs,
no row objects. Re-clamps an out-of-range selection into the new range
silently (a bulk data-model change, not a user selection action).
Invalidates measure.

#### DataGrid.RowCount

Returns the current row count.

**Syntax**

```go
func (g *DataGrid) RowCount() int
```

**Returns** — `int`.

#### DataGrid.OffsetX

Returns the current horizontal scroll offset.

**Syntax**

```go
func (g *DataGrid) OffsetX() float32
```

**Returns** — `float32`, clamped to `[0, max(0, contentWidth-viewport.W)]`
as of the last arrange pass, where `contentWidth` is `sum(colWidths)` —
mirrors `ScrollViewer.OffsetX` exactly.

#### DataGrid.ScrollToX

Requests a new horizontal offset.

**Syntax**

```go
func (g *DataGrid) ScrollToX(x float32) *DataGrid
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `x` | `float32` | Requested horizontal offset (pre-clamp). |

**Returns** — `*DataGrid` for chaining.

**Notes** — Clamped on the next layout pass, mirroring
`ScrollViewer.ScrollToX` exactly. Both the header and body scroll in sync,
so this moves both together.

#### DataGrid.SelectedIndex

Returns the current selection, or `-1` if none.

**Syntax**

```go
func (g *DataGrid) SelectedIndex() int
```

**Returns** — `int`.

#### DataGrid.SetSelectedIndex

Sets the selection programmatically.

**Syntax**

```go
func (g *DataGrid) SetSelectedIndex(i int) *DataGrid
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `i` | `int` | Requested index, clamped into `[-1, RowCount()-1]`. |

**Returns** — `*DataGrid` for chaining.

**Notes** — Silent, and auto-scrolled into view — matching
`ListView.SetSelectedIndex`'s contract exactly.

#### DataGrid.OnChanged

Sets the callback fired with the new row index whenever the user changes
the selection.

**Syntax**

```go
func (g *DataGrid) OnChanged(fn func(int)) *DataGrid
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `fn` | `func(int)` | Called with the new selected row on a user-driven change. A `nil` value is a valid, silent no-op. Replaces any previously set callback. |

**Returns** — `*DataGrid` for chaining.

**Notes** — Fires by clicking a row or pressing Up/Down while focused —
never for a programmatic `SetSelectedIndex`.

#### DataGrid.Dispose

No-op.

**Syntax**

```go
func (g *DataGrid) Dispose()
```

**Notes** — v0's row-count-plus-`Column.Value`-funcs data model holds no
external subscription to release, unlike `ListView`'s `items` channel.
Present purely so a rebuild's cancel path can call `Dispose` uniformly on
every virtualized control without a type switch.

**See also** — [ListView.Dispose](#listviewdispose)

#### DataGrid.Children

Returns the currently realized cell pool.

**Syntax**

```go
func (g *DataGrid) Children() []core.Widget
```

**Returns** — `[]core.Widget`, a copy of the pool (for hit-testing/render);
mutating the returned slice does not affect the `DataGrid`. `nil` when the
pool is empty.

**Notes** — Row-major (`pool[row*numCols+col]`), bounded by `visibleRows *
numColumns`, not `RowCount() * numColumns`.

#### DataGrid.MeasureContent

Reports `DataGrid`'s fixed desired size.

**Syntax**

```go
func (g *DataGrid) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | Space offered by the parent layout. |

**Returns** — `render.Size`, `{240, 200}` clamped to `available` on each
axis — matching `ListView`'s own "virtualized content never grows the
desired size" convention.

#### DataGrid.ArrangeContent

The single source of truth for the header's fixed rect, column-width
resolution, scroll-offset clamping, and body cell realization.

**Syntax**

```go
func (g *DataGrid) ArrangeContent(bounds render.Rect)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `bounds` | `render.Rect` | Bounds assigned by the parent layout. |

**Notes** — `bounds` is first inset by the 2px sunken bevel; the header's
raised cell buttons and every body row render inside the outer well's
frame, never over it. Column widths are resolved against the body viewport
width (after the always-reserved vertical thumb gutter). The horizontal
thumb's gutter is reserved on the bottom only when `sum(colWidths)` exceeds
that viewport width — with any Star column present this can only happen
when the Px columns alone already overflow. `colOffsets` are the logical
(unscrolled) column positions; both `Render`'s header cells and the body
cells here apply `-offsetX` at paint/arrange time, so the header and body
always read the same offset and stay in horizontal sync. Same pool-reuse
contract as `ListView.ArrangeContent`: cells are re-texted in place, not
reallocated — a pool slot does not correspond to a stable `(row, col)`
identity across a scroll.

#### DataGrid.Render

Draws the outer well, header cells, selection band, and grid lines.

**Syntax**

```go
func (g *DataGrid) Render(r render.Renderer)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | Target renderer. |

**Notes** — Draws the outer sunken `WindowWell` frame first, then the
header (each column its own raised `ButtonFace` cell with a `WindowText`
title, rather than one flat strip), then the body's per-row backgrounds:
the selected row's `Highlight` band (a hovered row paints no fill at all in
the classic look), and every visible row's own 1px bottom grid line — v0 is
horizontal-only, no vertical separators. Header cells/titles are painted at
`colOffsets[i]-offsetX` — the same offset the body cells are arranged with
— so **the header scrolls horizontally in lockstep with the body** while
staying vertically fixed. The header is clipped horizontally with its own
`PushClip`/`PopClip` pair (it paints outside `ClipRect`'s scope, since
`ClipRect` excludes the header strip) so scrolled header cells never bleed
past the grid's own left/right edges once column content overflows.

#### DataGrid.ClipRect

Clips realized body cells to the bevel-inset content bounds minus the
header strip.

**Syntax**

```go
func (g *DataGrid) ClipRect() (render.Rect, bool)
```

**Returns** — `(render.Rect, bool)`: the clip rect and `true`, always.
Implements `core.ClipProvider`.

**Notes** — Excludes the header strip so a partially-scrolled body row
never bleeds its text up into the header (the header itself is drawn in
`Render`, before this clip is even pushed, so it's never affected either
way), nor onto the outer sunken bevel. Otherwise matches
`ListView.ClipRect` (gutter included, so the thumb is never cropped).

#### DataGrid.RenderOverlay

Draws the scroll thumbs and the focus ring.

**Syntax**

```go
func (g *DataGrid) RenderOverlay(r render.Renderer)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | Target renderer. |

**Notes** — Draws the classic track+thumb for each axis with content to
scroll to — vertical along the right edge, horizontal along the bottom —
above the clipped body rows, then the focus ring while focused, matching
`ListView.RenderOverlay` exactly. Implements `core.OverlayRenderer`.

#### DataGrid.AcceptsFocus

Reports whether `g` accepts keyboard focus.

**Syntax**

```go
func (g *DataGrid) AcceptsFocus() bool
```

**Returns** — `bool`, always `true` — v0 has no disabled concept, matching
`ListView`.

#### DataGrid.OnFocusChanged

Tracks focus for the focus-ring overlay and keyboard navigation.

**Syntax**

```go
func (g *DataGrid) OnFocusChanged(focused bool)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `focused` | `bool` | New focus state. |

#### DataGrid.OnPointer

Handles wheel scroll, thumb drag, row click-to-select, and hover tracking.

**Syntax**

```go
func (g *DataGrid) OnPointer(e *input.PointerEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.PointerEvent` | The pointer event; `e.Handled` is set when consumed. |

**Notes** — Wheel/Press/Move/Release mirror `ListView.OnPointer` exactly
(Shift+Wheel for horizontal, vertical-thumb-drag checked before
horizontal-thumb-drag before row click). Additionally tracks an internal
hovered-row index on `Move` when not mid-drag, cleared on `Leave` — the
classic look paints no hover fill, so this is tracked purely for other
consumers, not for `Render`. The hover index is also cleared on `Wheel` and
during a thumb drag, since the offset moved without a fresh position to
re-hit-test against and would otherwise name a stale row.

#### DataGrid.OnKey

Handles Up/Down row navigation.

**Syntax**

```go
func (g *DataGrid) OnKey(e *input.KeyEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.KeyEvent` | The key event; `e.Handled` is set when consumed. |

**Notes** — Up/Down move the selection by one row (clamped into
`[0, RowCount()-1]`, never landing on `-1`), as a user-driven change,
auto-scrolled into view. Ignored entirely for anything but a key-press
action, or when there are no rows at all.

---

## Column

`Column` describes one `DataGrid` column: its header title, its sizing
track, and the func that produces a given row's displayed text for this
column.

Struct with no constructor or methods — build a `Column` as a literal.

### Fields

| Name | Type | Description |
|---|---|---|
| `Title` | `string` | The column's header title. |
| `Width` | `Track` | The column's sizing track — `controls.Px(n)` for a fixed pixel width, or `controls.Star(weight)` for a proportional share of the remaining space, weighted against the grid's other Star columns. `controls.AutoTrack()` is rejected: `DataGrid.SetColumns` panics on any column carrying one. |
| `Value` | `func(row int) string` | Produces row `row`'s displayed text for this column. May be `nil`, in which case every cell in the column renders empty text. |

**Example**

```go
col := controls.Column{
    Title: "Price",
    Width: controls.Star(1),
    Value: func(row int) string {
        return fmt.Sprintf("$%.2f", prices[row])
    },
}
grid.SetColumns(col)
```

**Notes** — Column widths are resolved against the body viewport's width
the same way `Grid`'s own row/column tracks resolve (`Px` columns get their
fixed size first, then `Star` columns split whatever space remains by
weight). `Value` is called with the row's absolute index — the same index
`SelectedIndex`/`OnChanged` report — not an index relative to the
currently-visible window.

**See also** — [DataGrid](#datagrid), [DataGrid.SetColumns](#datagridsetcolumns)

---

## Expander

`Expander` is a collapsible container: a full-width, clickable header row
(`'v'`/`'>'` chevron + title) that toggles whether its content widget is
shown below it. `Expander` itself draws no chrome of its own (no `Render`
override — it's a pure layout composite, matching `StackPanel`); all
visible chrome belongs to the header.

**Normative**: content participates in layout — measured, arranged, and
rendered — **only while expanded**. `MeasureContent`, `ArrangeContent`, and
`Children` all skip content entirely while collapsed, not merely hide it
after the fact: measuring the same `Expander` collapsed vs. expanded
genuinely produces two different desired sizes.

**Constructor**

```go
func NewExpander(face *text.Face, header string) *Expander
```

Returns a collapsed `Expander` with the given header title, drawing header
text with `face` (`face` may be `nil`, per `TextBlock`'s own nil-face
convention).

**Example**

```go
exp := controls.NewExpander(face, "Advanced options")
exp.SetContent(controls.NewTextBlock(face, "Extra settings go here."))
exp.OnChanged(func(expanded bool) {
    fmt.Println("expanded:", expanded)
})
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [SetContent](#expandersetcontent) | `SetContent(w core.Widget) *Expander` | Sets the content widget shown while expanded. |
| [Expanded](#expanderexpanded) | `Expanded() bool` | Reports whether content is currently shown. |
| [SetExpanded](#expandersetexpanded) | `SetExpanded(v bool) *Expander` | Sets the expanded state programmatically. |
| [OnChanged](#expanderonchanged) | `OnChanged(fn func(bool)) *Expander` | Sets the user-driven toggle callback. |
| [MeasureContent](#expandermeasurecontent) | `MeasureContent(available render.Size) render.Size` | Measures the header, and content only while expanded. |
| [ArrangeContent](#expanderarrangecontent) | `ArrangeContent(bounds render.Rect)` | Arranges the header, and content only while expanded. |
| [Children](#expanderchildren) | `Children() []core.Widget` | Returns the header alone, or header+content while expanded. |

#### Expander.SetContent

Sets (replacing any existing) the content widget shown below the header
while expanded.

**Syntax**

```go
func (e *Expander) SetContent(w core.Widget) *Expander
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `w` | `core.Widget` | New content widget. |

**Returns** — `*Expander` for chaining.

**Notes** — Re-parents `w` to this `Expander` and invalidates measure. Any
previously set content is detached (its parent cleared) so its future
invalidations stop climbing into this `Expander`.

#### Expander.Expanded

Reports whether the content is currently shown.

**Syntax**

```go
func (e *Expander) Expanded() bool
```

**Returns** — `bool`.

#### Expander.SetExpanded

Sets the expanded state programmatically.

**Syntax**

```go
func (e *Expander) SetExpanded(v bool) *Expander
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `bool` | New expanded state. |

**Returns** — `*Expander` for chaining.

**Notes** — Silent — never fires `OnChanged`, matching fluo's uniform
contract that programmatic setters are silent (`ToggleButton.SetChecked`,
`CheckBox.SetChecked`, ...). A no-op when `v` already matches the current
state (content's layout participation is unchanged either way, so there's
nothing to invalidate).

#### Expander.OnChanged

Sets the callback fired with the new expanded value whenever the user
toggles the header.

**Syntax**

```go
func (e *Expander) OnChanged(fn func(bool)) *Expander
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `fn` | `func(bool)` | Called with the new expanded state on a user-driven toggle. A `nil` value is a valid, silent no-op. Replaces any previously set callback. |

**Returns** — `*Expander` for chaining.

**Notes** — Fires by a header click, or Space/Enter while the header is
focused — never for a programmatic `SetExpanded`.

#### Expander.MeasureContent

Measures the header unconditionally, and content only while expanded.

**Syntax**

```go
func (e *Expander) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | Space offered by the parent layout. |

**Returns** — `render.Size`: header's desired size alone while collapsed
(or with no content set); header size plus content's desired size (content
measured with the width available and whatever height remains after the
header) while expanded.

**Notes** — Content is never measured at all while collapsed — this is the
concrete meaning of "content participates in layout only when expanded,"
not merely "content is hidden after being measured as usual."

#### Expander.ArrangeContent

Arranges the header across the full bounds width, and content directly
below it only while expanded.

**Syntax**

```go
func (e *Expander) ArrangeContent(bounds render.Rect)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `bounds` | `render.Rect` | Bounds assigned by the parent layout. |

**Notes** — Arranges the header at its own measured height across the full
bounds width; content (only while expanded and set) is arranged directly
below it, also at the full bounds width. Content is never arranged at all
while collapsed, mirroring `MeasureContent`.

#### Expander.Children

Returns the header alone while collapsed, or the header plus content while
expanded.

**Syntax**

```go
func (e *Expander) Children() []core.Widget
```

**Returns** — `[]core.Widget`: `{header}` while collapsed or with no
content set; `{header, content}` while expanded.

**Notes** — This is the render-time half of "content participates in
layout only when expanded": the render engine draws exactly what
`Children` returns, so collapsed content is neither arranged nor walked
for rendering, not merely arranged-then-skipped.

---

## ScrollViewer

`ScrollViewer` scrolls a single child on both axes: vertically (the
original behavior) and horizontally (added alongside it). It clips its
child to its own bounds, draws overlay thumbs — vertical on the right,
horizontal along the bottom — when the child overflows the corresponding
axis, and responds to mouse wheel and thumb-drag input on either axis:
**plain wheel scrolls vertically, Shift+Wheel scrolls horizontally** (a
plain wheel also falls back to horizontal when there is no vertical
overflow but there is horizontal overflow, so a purely
horizontally-scrolling `ScrollViewer` is still wheel-scrollable without
requiring Shift).

The two axes are **not symmetric**, by design, so a `ScrollViewer` whose
content only overflows vertically stays byte-identical to the original
single-axis implementation: the vertical thumb's gutter is reserved
**unconditionally** (regardless of whether the content is actually taller
than the viewport), while the horizontal thumb's gutter is reserved only
when the child's natural width actually exceeds the `ScrollViewer`'s own
outer bounds width.

**Constructor**

```go
func NewScrollViewer() *ScrollViewer
```

Returns an empty `ScrollViewer` with no child and offset 0, styled from
`theme.Active()` at construction; rebuild to re-theme.

**Example**

```go
sv := controls.NewScrollViewer()
sv.SetChild(controls.NewTextBlock(face, longDocument))
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [SetChild](#scrollviewersetchild) | `SetChild(w core.Widget) *ScrollViewer` | Sets the scrolled child. |
| [OffsetY](#scrollvieweroffsety) | `OffsetY() float32` | Returns the current vertical scroll offset. |
| [OffsetX](#scrollvieweroffsetx) | `OffsetX() float32` | Returns the current horizontal scroll offset. |
| [ScrollTo](#scrollviewerscrollto) | `ScrollTo(y float32) *ScrollViewer` | Requests a new vertical offset. |
| [ScrollBy](#scrollviewerscrollby) | `ScrollBy(dy float32)` | Requests a relative vertical scroll. |
| [ScrollToX](#scrollviewerscrolltox) | `ScrollToX(x float32) *ScrollViewer` | Requests a new horizontal offset. |
| [ScrollByX](#scrollviewerscrollbyx) | `ScrollByX(dx float32)` | Requests a relative horizontal scroll. |
| [Children](#scrollviewerchildren) | `Children() []core.Widget` | Returns the scrolled child in a slice, or nil. |
| [MeasureContent](#scrollviewermeasurecontent) | `MeasureContent(available render.Size) render.Size` | Measures the child against a gutter-reduced width, unbounded height. |
| [ArrangeContent](#scrollviewerarrangecontent) | `ArrangeContent(bounds render.Rect)` | Clamps offsets on both axes and arranges the child. |
| [ClipRect](#scrollviewercliprect) | `ClipRect() (render.Rect, bool)` | Clips the child to the viewer's own bounds. |
| [RenderOverlay](#scrollviewerrenderoverlay) | `RenderOverlay(r render.Renderer)` | Draws the scroll thumbs. |
| [OnPointer](#scrollvieweronpointer) | `OnPointer(e *input.PointerEvent)` | Handles wheel scroll and thumb drag. |

#### ScrollViewer.SetChild

Sets (replacing any existing) the single scrolled child.

**Syntax**

```go
func (s *ScrollViewer) SetChild(w core.Widget) *ScrollViewer
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `w` | `core.Widget` | New scrolled child. |

**Returns** — `*ScrollViewer` for chaining.

**Notes** — Re-parents `w` to this `ScrollViewer` and invalidates measure.
Any previously set child is detached (its parent cleared), matching the
`Border` convention: its future invalidations stop climbing into this
`ScrollViewer`.

#### ScrollViewer.OffsetY

Returns the current vertical scroll offset.

**Syntax**

```go
func (s *ScrollViewer) OffsetY() float32
```

**Returns** — `float32`, clamped to `[0, max(0, childH-viewportH)]` as of
the last arrange pass.

#### ScrollViewer.OffsetX

Returns the current horizontal scroll offset.

**Syntax**

```go
func (s *ScrollViewer) OffsetX() float32
```

**Returns** — `float32`, clamped to `[0, max(0, childW-viewportW)]` as of
the last arrange pass.

#### ScrollViewer.ScrollTo

Requests a new vertical offset.

**Syntax**

```go
func (s *ScrollViewer) ScrollTo(y float32) *ScrollViewer
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `y` | `float32` | Requested vertical offset (pre-clamp). |

**Returns** — `*ScrollViewer` for chaining.

**Notes** — The value is stored raw and clamped on the next arrange pass
(`ArrangeContent` is the single source of truth for clamping), so
`OffsetY` may not reflect `y` until layout runs again.

#### ScrollViewer.ScrollBy

Requests a relative change to the vertical offset.

**Syntax**

```go
func (s *ScrollViewer) ScrollBy(dy float32)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `dy` | `float32` | Delta added to the raw vertical offset. |

**Notes** — Clamped on the next arrange pass like `ScrollTo`.

#### ScrollViewer.ScrollToX

Requests a new horizontal offset.

**Syntax**

```go
func (s *ScrollViewer) ScrollToX(x float32) *ScrollViewer
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `x` | `float32` | Requested horizontal offset (pre-clamp). |

**Returns** — `*ScrollViewer` for chaining.

**Notes** — Clamped on the next arrange pass like `ScrollTo`.

#### ScrollViewer.ScrollByX

Requests a relative change to the horizontal offset.

**Syntax**

```go
func (s *ScrollViewer) ScrollByX(dx float32)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `dx` | `float32` | Delta added to the raw horizontal offset. |

**Notes** — Clamped on the next arrange pass like `ScrollBy`.

#### ScrollViewer.Children

Returns the single scrolled child in a slice, or `nil` if there is none.

**Syntax**

```go
func (s *ScrollViewer) Children() []core.Widget
```

**Returns** — `[]core.Widget`, a copy; mutating it does not affect the
viewer.

#### ScrollViewer.MeasureContent

Measures the child against a gutter-reduced width and unbounded height.

**Syntax**

```go
func (s *ScrollViewer) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | Space offered by the parent layout. |

**Returns** — `render.Size`: the min of (child size + vertical gutter on
the width axis) and `available`, per axis — a `ScrollViewer` never asks its
parent for more room than it was offered, even if its content is
taller/wider.

**Notes** — Measures the child with the available width reduced by the
always-on vertical thumb gutter, and unbounded height (so the child
reports its full natural content height). The width offered to the child
stays **bounded**, not unbounded like height — content that adapts its own
reported size to the available budget (e.g. wrapping text) still wraps to
that budget rather than reporting some larger natural width. Only content
whose own `MeasureContent` ignores available width (e.g. `Fixed`) can
report a desired width exceeding the viewport, which is what enables
horizontal scroll for it.

#### ScrollViewer.ArrangeContent

The single source of truth for offset clamping on both axes.

**Syntax**

```go
func (s *ScrollViewer) ArrangeContent(bounds render.Rect)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `bounds` | `render.Rect` | Bounds assigned by the parent layout. |

**Notes** — Reserves the vertical thumb's gutter on the right
unconditionally. Decides whether to reserve the horizontal thumb's gutter
on the bottom by comparing the child's natural width against `bounds.W` —
the viewer's own **outer** width, not the vertical-gutter-reduced inner
viewport width (content whose natural width sits strictly between
`bounds.W - vgutter` and `bounds.W` is arranged slightly wider than the
inner viewport with no thumb ever shown for that sliver — a narrow,
harmless edge case). Clamps `rawOffset`/`rawOffsetX` into
`[0, max(0, childH/W-viewportH/W)]`. Arranges the child at
`{viewport.X-offsetX, viewport.Y-offset, arrangeW, childH}`, where
`arrangeW` is at least `viewport.W` (preserving Stretch-to-fill for
non-overflowing content) but extends to the child's full desired width
when that exceeds `viewport.W`, letting it overflow horizontally exactly
as `childH` already lets it overflow vertically. `ClipRect` crops whatever
scrolls past the viewport on either axis.

#### ScrollViewer.ClipRect

Clips the child to the viewer's own full bounds.

**Syntax**

```go
func (s *ScrollViewer) ClipRect() (render.Rect, bool)
```

**Returns** — `(render.Rect, bool)`: `s.Bounds()` and `true`, always.
Implements `core.ClipProvider`.

**Notes** — Both thumb gutters are included, so the thumbs themselves
(drawn in `RenderOverlay`, which runs after the clip is popped) are never
cropped. Unchanged across both axes: the full bounds were always the clip
rect, on both X and Y, even before horizontal scrolling existed.

#### ScrollViewer.RenderOverlay

Draws the classic scrollbar track+thumb for each axis with content to
scroll to.

**Syntax**

```go
func (s *ScrollViewer) RenderOverlay(r render.Renderer)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | Target renderer. |

**Notes** — Vertical along the right edge, horizontal along the bottom
edge, above the clipped child. When both are shown, the two tracks (sized
from the viewport, itself inset on both the reserved right and bottom
gutters) naturally stop short of each other, leaving the bottom-right
corner square and undrawn by either track. Implements
`core.OverlayRenderer`.

#### ScrollViewer.OnPointer

Handles wheel scroll and thumb drag on either axis.

**Syntax**

```go
func (s *ScrollViewer) OnPointer(e *input.PointerEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.PointerEvent` | The pointer event; `e.Handled` is set when consumed. |

**Notes** — Wheel scrolls vertically by a fixed step per notch by default
(matching the pre-horizontal-scroll behavior exactly when there is
vertical content to scroll to); **Shift+Wheel scrolls horizontally**
instead; a plain wheel also scrolls horizontally when there is no vertical
content to scroll to but there is horizontal content — so a purely
horizontally-overflowing `ScrollViewer` is still wheel-scrollable without
requiring Shift. Wheel is always handled. A Press inside the current
vertical thumb rect starts a vertical drag (checked first, matching the
original single-axis priority); otherwise a Press inside the current
horizontal thumb rect starts a horizontal drag. A Press matching neither
thumb is left unhandled so it bubbles through to the scrolled content.
Implements `input.PointerHandler`.

---

## ListItems

`ListItems` is the minimal observable string source a `ListView`
virtualizes: a read-only indexed sequence (`Len`/`At`) plus a granular
change-notification channel. `*bind.List[string]` satisfies this interface
structurally — see [ListChange](#listchange)'s Notes for why `ListView`
declares this as an interface rather than accepting a concrete
`*bind.List[string]` parameter directly (avoiding an import cycle between
`controls` and `bind`).

**Syntax**

```go
type ListItems interface {
    Len() int
    At(i int) string
    OnChange(f func(ListChange)) (cancel func())
}
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [Len](#listitemslen) | `Len() int` | Returns the number of items. |
| [At](#listitemsat) | `At(i int) string` | Returns the item at index `i`. |
| [OnChange](#listitemsonchange) | `OnChange(f func(ListChange)) (cancel func())` | Subscribes to granular change events. |

**Example**

```go
var items controls.ListItems = bind.NewList("Alice", "Bob")
lv := controls.NewListView(face, items)
```

#### ListItems.Len

Returns the number of items in the sequence.

**Syntax**

```go
Len() int
```

**Returns** — `int`, the current item count.

#### ListItems.At

Returns the item at index `i`.

**Syntax**

```go
At(i int) string
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `i` | `int` | Index in `[0, Len())`. Out-of-range behavior is implementation-defined — `bind.List[T].At` panics. |

**Returns** — `string`, the item's displayed text.

#### ListItems.OnChange

Subscribes to granular change events.

**Syntax**

```go
OnChange(f func(ListChange)) (cancel func())
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `f` | `func(ListChange)` | Called with each granular mutation event, after it is fully applied. |

**Returns** — `cancel func()`, which removes the subscription when called;
expected to be idempotent (`bind.List[T]`'s implementation is).

**Notes** — `ListView` subscribes once at construction and invalidates its
own measure on every event; v0 does not use the change payload for
incremental updates — a full re-layout recomputes the visible range and
re-texts the pool from scratch. See the `bind` package's `List[T]` for the
reference implementation and the model-authoring side of this contract
(`Add`, `Insert`, `RemoveAt`, `Set`, `Replace`, and the coarse `OnChanged`
alongside this granular `OnChange`).

**See also** — [ListChange](#listchange), [ListChangeKind](#listchangekind)

---

## ListChange

`ListChange` is the payload delivered to a `ListItems.OnChange` subscriber:
one granular list-mutation event.

Struct with no constructor or methods.

### Fields

| Name | Type | Description |
|---|---|---|
| `Kind` | `ListChangeKind` | What kind of mutation occurred. |
| `Index` | `int` | The affected index, or `-1` for a full reset (`ListChangeReset`). |

**Example**

```go
cancel := items.OnChange(func(c controls.ListChange) {
    switch c.Kind {
    case controls.ListChangeAdd:
        fmt.Println("added at", c.Index)
    case controls.ListChangeReset:
        fmt.Println("list reset")
    }
})
defer cancel()
```

**Notes** — `ListChange` mirrors `bind.Change` exactly — `bind.Change` is a
type alias for this type, not an independent redefinition, so a
`bind.List[T]`'s granular subscribers and a `ListView`'s see the identical
value. This is the one deliberately inverted seam in an otherwise
`bind`-depends-on-`controls` codebase: `ListChange`/`ListChangeKind` are
declared in `controls` (because `ListItems.OnChange` must name this
payload type) and `bind` re-exports them as aliases, rather than
`controls` importing `bind`.

**See also** — [ListItems](#listitems), [ListChangeKind](#listchangekind)

---

## ListChangeKind

`ListChangeKind` enumerates the granular list-mutation kinds a `ListView`
reacts to. It mirrors `bind.ChangeKind` exactly — `bind.ChangeKind` is a
type alias for this type.

**Syntax**

```go
type ListChangeKind uint8
```

### Values

| Name | Value | Description |
|---|---|---|
| `ListChangeAdd` | `0` | One item was added (`Index` names its position). |
| `ListChangeRemove` | `1` | One item was removed (`Index` names its former position). |
| `ListChangeReplace` | `2` | One item's value was replaced in place (`Index` names its position). |
| `ListChangeReset` | `3` | The entire list was replaced (`Index` is `-1`). |

**Example**

```go
if change.Kind == controls.ListChangeReset {
    // treat as a full rebuild
}
```

**Notes** — Declared in `controls` rather than `bind` so that
`ListView`'s `ListItems` interface can name this payload type without
`controls` importing `bind` — see [ListChange](#listchange)'s Notes for the
full rationale. `bind.List[T]`'s mutating methods (`Add`, `Insert`,
`RemoveAt`, `Set`, `Replace`) each fire exactly one of these four kinds
after the coarse `OnChanged` notification; a subscriber processing the
first event of a multi-item `Add` already observes the final list state.

**See also** — [ListItems](#listitems), [ListChange](#listchange)
