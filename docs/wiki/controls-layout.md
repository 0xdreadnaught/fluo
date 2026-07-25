# Layout & primitive controls

The layout containers in `controls` — `Border`, `Fixed`, `StackPanel`, `Grid`,
`DockPanel`, `WrapPanel`, and `Canvas` — arrange child `core.Widget`s without
drawing interactive chrome of their own; `TextBlock` is the single-line text
leaf most of them end up hosting. Reach for this group whenever you're
building the skeleton of a screen: stacking a toolbar and a body
(`StackPanel`/`DockPanel`), laying out a form grid (`Grid`), flowing a tag
list (`WrapPanel`), or free-positioning an overlay (`Canvas`).

Every type here embeds `core.Element` and participates in the same two-pass
Measure/Arrange layout model — see the `core` package reference for the
model itself (`MeasureWidget`/`ArrangeWidget`, margins, min/max, alignment).
This page only documents what each type adds: construction, child
management, and the `MeasureContent`/`ArrangeContent` overrides that
implement its particular distribution strategy. The common per-widget
geometry API (`SetMargin`, `SetWidth`/`SetHeight`, `SetMinSize`/`SetMaxSize`,
`SetAlign`, `SetVisible`, `DesiredSize`, `Bounds`, …) is inherited from
`core.Element` and is not repeated per type.

**Import:** `github.com/0xdreadnaught/fluo/controls`

## Contents
- [Border](#border)
- [TextBlock](#textblock)
- [Fixed](#fixed)
- [StackPanel](#stackpanel)
- [Grid](#grid)
- [DockPanel](#dockpanel)
- [WrapPanel](#wrappanel)
- [Canvas](#canvas)
- [Orientation](#orientation)
- [Dock](#dock)
- [Track](#track)

---

## Border

A single-child decorator that draws a background fill, an optional stroke,
and optional padding around its child.

**Constructor**

```go
func NewBorder() *Border
```

Returns an empty `Border` with no child, no padding, and no stroke.

**Example**

```go
b := controls.NewBorder().
    SetBackground(render.RGB(255, 255, 255)).
    SetBorder(render.RGB(128, 128, 128), 1).
    SetPadding(render.Uniform(8))
b.SetChild(controls.NewTextBlock(face, "Hello"))
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [SetChild](#bordersetchild) | `SetChild(w core.Widget) *Border` | Sets (replacing any existing) the single child. |
| [SetBackground](#bordersetbackground) | `SetBackground(c render.Color) *Border` | Sets the fill color drawn behind the border/child. |
| [SetBorder](#bordersetborder) | `SetBorder(c render.Color, width float32) *Border` | Sets the stroke color and width. |
| [SetRadius](#bordersetradius) | `SetRadius(r float32) *Border` | Sets the corner radius for the background and stroke. |
| [SetPadding](#bordersetpadding) | `SetPadding(t render.Thickness) *Border` | Sets the space between the border chrome and the child. |
| [MeasureContent](#bordermeasurecontent) | `MeasureContent(available render.Size) render.Size` | Measures the child inside the chrome inset; desired size is child + chrome. |
| [ArrangeContent](#borderarrangecontent) | `ArrangeContent(bounds render.Rect)` | Arranges the child within bounds inset by chrome. |
| [Render](#borderrender) | `Render(r render.Renderer)` | Draws the background fill and stroke. |
| [Children](#borderchildren) | `Children() []core.Widget` | Returns the single child in a slice, or nil. |

#### Border.SetChild

Sets (replacing any existing) the single child, re-parenting it to this
`Border` and invalidating measure.

**Syntax**

```go
func (b *Border) SetChild(w core.Widget) *Border
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `w` | `core.Widget` | The new child. Any previously set child is detached (`core.SetParent(old, nil)`) so its future invalidations stop climbing into this `Border`. |

**Returns** — `*Border` for chaining.

**Example**

```go
border.SetChild(controls.NewFixed(100, 40, render.RGB(200, 0, 0)))
```

**See also** — [Border.Children](#borderchildren)

#### Border.SetBackground

Sets the fill color drawn behind the border/child.

**Syntax**

```go
func (b *Border) SetBackground(c render.Color) *Border
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `c` | `render.Color` | The background fill. A fully transparent color (`A == 0`) is skipped at render time. |

**Returns** — `*Border` for chaining.

**Notes** — Purely visual: does not invalidate layout, since the host
redraws every frame regardless.

**See also** — [Border.SetBorder](#bordersetborder)

#### Border.SetBorder

Sets the stroke color and width.

**Syntax**

```go
func (b *Border) SetBorder(c render.Color, width float32) *Border
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `c` | `render.Color` | The stroke color. |
| `width` | `float32` | The stroke thickness in pixels. |

**Returns** — `*Border` for chaining.

**Notes** — A width change affects layout (it eats into the child's
available space via the chrome inset), so `SetBorder` invalidates measure
only when `width` actually changes.

#### Border.SetRadius

Sets the corner radius used when drawing the background and stroke.

**Syntax**

```go
func (b *Border) SetRadius(r float32) *Border
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `float32` | Corner radius in pixels. |

**Returns** — `*Border` for chaining.

**Notes** — Purely visual: no invalidation.

#### Border.SetPadding

Sets the space between the border chrome and the child.

**Syntax**

```go
func (b *Border) SetPadding(t render.Thickness) *Border
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `t` | `render.Thickness` | Per-side padding, added to the stroke width to form the total chrome inset. |

**Returns** — `*Border` for chaining.

**Notes** — Layout-relevant: invalidates measure.

**Example**

```go
border.SetPadding(render.Thickness{Left: 8, Top: 4, Right: 8, Bottom: 4})
```

#### Border.MeasureContent

Measures the child (if any) with the available space reduced by chrome
(padding + border width on all four sides), then adds the chrome back to
the child's desired size.

**Syntax**

```go
func (b *Border) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | Space offered to the `Border` by its parent (already inset by margins/min-max — see the `core` package reference). |

**Returns** — `render.Size`. With no child, the desired size is the chrome
alone.

**See also** — [Border.ArrangeContent](#borderarrangecontent)

#### Border.ArrangeContent

Arranges the child (if any) within `bounds` inset by chrome.

**Syntax**

```go
func (b *Border) ArrangeContent(bounds render.Rect)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `bounds` | `render.Rect` | The `Border`'s final absolute bounds, as computed by `ArrangeWidget`. |

**Notes** — A no-op when there is no child.

#### Border.Render

Draws the background fill (if visible) and the stroke (if visible).

**Syntax**

```go
func (b *Border) Render(r render.Renderer)
```

**Notes** — Skips the fill when `background.A == 0`, and the stroke when
`borderWidth <= 0` or `borderColor.A == 0`.

#### Border.Children

Returns the single child in a slice, or nil if there is none.

**Syntax**

```go
func (b *Border) Children() []core.Widget
```

**Returns** — `[]core.Widget`. A copy; mutating it does not affect the
`Border`.

---

## TextBlock

A single-line text leaf widget: draws a string with a given `text.Face`.

**Constructor**

```go
func NewTextBlock(face *text.Face, s string) *TextBlock
```

`face` may be nil, in which case the widget measures to zero and renders
nothing. The default color is styled from `theme.Active()` at construction
time; `SetColor` overrides it.

**Example**

```go
font, _ := text.Load(ttfBytes)
face := text.NewFace(font, 14)
tb := controls.NewTextBlock(face, "Hello, fluo")
```

**Notes** — The theme default reads `theme.Active().Color.WindowText`, the
classic primary-text token (see the [theming](theming.md#colortokens) page).
The color is captured at construction, so changing the active theme does not
recolor an existing `TextBlock` — rebuild the tree after `theme.SetActive`.

### Methods

| Method | Signature | Description |
|---|---|---|
| [SetText](#textblocksettext) | `SetText(s string) *TextBlock` | Sets the displayed text. |
| [Text](#textblocktext) | `Text() string` | Returns the currently displayed text. |
| [SetColor](#textblocksetcolor) | `SetColor(c render.Color) *TextBlock` | Sets the text color. |
| [Color](#textblockcolor) | `Color() render.Color` | Returns the text's current color. |
| [MeasureContent](#textblockmeasurecontent) | `MeasureContent(available render.Size) render.Size` | Returns the size the text occupies when drawn with `face`. |
| [Render](#textblockrender) | `Render(r render.Renderer)` | Draws the text at the top-left of the widget's bounds. |

#### TextBlock.SetText

Sets the displayed text.

**Syntax**

```go
func (t *TextBlock) SetText(s string) *TextBlock
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `s` | `string` | The new text. |

**Returns** — `*TextBlock` for chaining.

**Notes** — Changing the text invalidates measure; setting the same value
is a no-op, since the text is stored in a `core.Property[string]` that only
notifies on real changes.

#### TextBlock.Text

Returns the currently displayed text.

**Syntax**

```go
func (t *TextBlock) Text() string
```

**Returns** — `string`.

#### TextBlock.SetColor

Sets the text color.

**Syntax**

```go
func (t *TextBlock) SetColor(c render.Color) *TextBlock
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `c` | `render.Color` | The new text color, overriding the theme default. |

**Returns** — `*TextBlock` for chaining.

**Notes** — Purely visual: no invalidation needed.

#### TextBlock.Color

Returns the text's current color, whether the theme default set at
construction or a later `SetColor` override.

**Syntax**

```go
func (t *TextBlock) Color() render.Color
```

**Returns** — `render.Color`.

#### TextBlock.MeasureContent

Returns the size the text occupies when drawn with `face`.

**Syntax**

```go
func (t *TextBlock) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | Ignored beyond the nil-face case below — `TextBlock` measures to its natural text extent regardless of space offered. |

**Returns** — `render.Size`. A nil `face` measures to zero (`render.Size{}`).

#### TextBlock.Render

Draws the text at the top-left of the widget's bounds.

**Syntax**

```go
func (t *TextBlock) Render(r render.Renderer)
```

**Notes** — Skipped entirely when there is no face, no text, or a fully
transparent color (`color.A == 0`).

---

## Fixed

A solid-color fixed-size block: useful as a spacer, a color swatch, or a
lightweight test widget with a known desired size.

**Constructor**

```go
func NewFixed(w, h float32, c render.Color) *Fixed
```

Returns a `Fixed` that always measures to `(w, h)` and paints its bounds
with `c`.

**Example**

```go
spacer := controls.NewFixed(0, 16, render.Color{}) // transparent vertical gutter
swatch := controls.NewFixed(24, 24, render.RGB(200, 40, 40))
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [MeasureContent](#fixedmeasurecontent) | `MeasureContent(available render.Size) render.Size` | Returns the fixed `(w, h)` size regardless of space available. |
| [Render](#fixedrender) | `Render(r render.Renderer)` | Fills the widget's bounds with its color. |

#### Fixed.MeasureContent

Returns the fixed `(w, h)` size regardless of the space available.

**Syntax**

```go
func (f *Fixed) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | Ignored — `Fixed` never shrinks or grows to fit. |

**Returns** — `render.Size{W: w, H: h}`, the constructor's fixed values.

#### Fixed.Render

Fills the widget's bounds with its color.

**Syntax**

```go
func (f *Fixed) Render(r render.Renderer)
```

**Notes** — Fully transparent colors (`A == 0`) are skipped.

---

## StackPanel

Arranges children in a row (`Horizontal`) or column (`Vertical`) with
optional spacing between them — the workhorse container for toolbars,
button rows, and simple vertical forms.

**Constructor**

```go
func NewStackPanel(o Orientation) *StackPanel
```

Returns a new `StackPanel` with the given [Orientation](#orientation).

**Example**

```go
toolbar := controls.NewStackPanel(controls.Horizontal).
    SetGap(8).
    Add(controls.NewFixed(20, 20, render.RGB(0, 0, 0)),
        controls.NewTextBlock(face, "Ready"))
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [Add](#stackpaneladd) | `Add(children ...core.Widget) *StackPanel` | Appends children to the stack. |
| [SetGap](#stackpanelsetgap) | `SetGap(g float32) *StackPanel` | Sets the spacing between adjacent children. |
| [Clear](#stackpanelclear) | `Clear() *StackPanel` | Removes all children. |
| [Children](#stackpanelchildren) | `Children() []core.Widget` | Returns a copy of the children slice. |
| [MeasureContent](#stackpanelmeasurecontent) | `MeasureContent(available render.Size) render.Size` | Measures all children and computes the total stacked size. |
| [ArrangeContent](#stackpanelarrangecontent) | `ArrangeContent(bounds render.Rect)` | Arranges children end-to-end along the stack axis. |

#### StackPanel.Add

Appends children to this `StackPanel`, re-parenting each one and
invalidating measure.

**Syntax**

```go
func (s *StackPanel) Add(children ...core.Widget) *StackPanel
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `children` | `...core.Widget` | One or more widgets to append, in order. |

**Returns** — `*StackPanel` for chaining.

#### StackPanel.SetGap

Sets the spacing between adjacent children.

**Syntax**

```go
func (s *StackPanel) SetGap(g float32) *StackPanel
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `g` | `float32` | Gap in pixels, inserted between adjacent *visible* children only. |

**Returns** — `*StackPanel` for chaining.

**Notes** — Layout-relevant: invalidates measure.

#### StackPanel.Clear

Removes all children, detaching each one and invalidating measure.

**Syntax**

```go
func (s *StackPanel) Clear() *StackPanel
```

**Returns** — `*StackPanel` for chaining, matching `Add`/`SetGap`.

**Notes** — Detaches every child via `core.SetParent(child, nil)`, so none
of them keep this panel recorded as their layout parent. A no-op in terms
of child count when the panel already has none, but it still invalidates
measure.

#### StackPanel.Children

Returns a copy of the children slice.

**Syntax**

```go
func (s *StackPanel) Children() []core.Widget
```

**Returns** — `[]core.Widget`. Mutating it does not affect the panel.

#### StackPanel.MeasureContent

Measures all children and computes the total stacked size.

**Syntax**

```go
func (s *StackPanel) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | Space offered to the panel. |

**Returns** — `render.Size`.
- `Vertical`: children are measured with `(available.W, +Inf)`; desired
  size is `(max child W, sum of child H + gaps between visible children)`.
- `Horizontal`: children are measured with `(+Inf, available.H)`; desired
  size is `(sum of child W + gaps between visible children, max child H)`.

**Notes** — Every child is measured regardless of visibility, but
`MeasureWidget` forces a hidden child's desired size to zero before
`MeasureContent` ever sees it, so hidden children never affect `maxW`/
`maxH` either. Only visible children contribute to the stacked extent and
gaps.

#### StackPanel.ArrangeContent

Arranges children end-to-end along the stack axis.

**Syntax**

```go
func (s *StackPanel) ArrangeContent(bounds render.Rect)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `bounds` | `render.Rect` | The panel's final absolute bounds. |

**Notes** — `Vertical`: each child gets the slot
`{bounds.X, y, bounds.W, childDesired.H}`, advancing `y` by the child's
height plus the gap; cross-axis alignment (stretch to `bounds.W` or not) is
handled by `ArrangeWidget`, not by the panel. `Horizontal` mirrors this on
the X axis. Hidden children are still arranged (they short-circuit cheaply
inside `ArrangeWidget`) but contribute no gap.

**See also** — [Orientation](#orientation)

---

## Grid

Arranges children into a fixed set of rows and columns, each an
independently-sized [Track](#track) (`Px`/`Auto`/`Star`) — the container to
reach for whenever a stack or dock isn't precise enough, e.g. a form with a
fixed-width label column and a stretching input column.

**v0 limitations** (by design, not bugs):
- No cell spans: every child occupies exactly one row and one column.
- Children are measured exactly once per measure pass — with per-axis
  available space equal to the `Px` value for `Px` tracks, and `+Inf` for
  `Auto`/`Star` tracks. They are never re-measured against the final
  resolved track size; a child in a `Star` track keeps its Inf-measured
  desired size, and `ArrangeWidget`'s stretch/alignment absorbs the
  difference between that desired size and the cell it's arranged into.
- When `Rows` or `Cols` is never called, that axis defaults to a single
  `Star(1)` track.
- `Rows`/`Cols` re-validate every already-`Add`ed cell against the new
  track count, panicking rather than letting a later layout pass index out
  of range.

**Constructor**

```go
func NewGrid() *Grid
```

Returns an empty `Grid` with no rows, columns, or children.

**Example**

```go
g := controls.NewGrid()
g.Cols(controls.Px(160), controls.Star(1), controls.Star(2))
g.Rows(controls.AutoTrack(), controls.Star(1))

g.Add(navRail,   0, 0)
g.Add(toolbar,   0, 1)
g.Add(inspector, 0, 2)
g.Add(navBody,   1, 0)
g.Add(canvas,    1, 1)
g.Add(props,     1, 2)
```

### Track sizing worked example

Given the `Grid` above, an available size of `800×600`, and row-0 children
whose tallest desired height is `40`, `resolveTracks` resolves each axis
independently:

| Track | Kind | Resolves to | Why |
|---|---|---|---|
| Col 0 | `Px(160)` | `160` | Fixed — taken off the top first. |
| Col 1 | `Star(1)` | `213.3` | `(800 − 160) × 1/(1+2)` — one third of the leftover width. |
| Col 2 | `Star(2)` | `426.7` | `(800 − 160) × 2/(1+2)` — two thirds of the leftover width. |
| Row 0 | `AutoTrack()` | `40` | The tallest row-0 child's desired height. |
| Row 1 | `Star(1)` | `560` | `600 − 40` — the only `Star` row gets all of the leftover height. |

The same resolution runs twice: once in `MeasureContent` against the space
*offered* to the grid, and again in `ArrangeContent` against the grid's
actual final bounds (which may differ if the grid itself was stretched or
constrained by its own parent).

### Methods

| Method | Signature | Description |
|---|---|---|
| [Rows](#gridrows) | `Rows(tracks ...Track) *Grid` | Sets the row tracks for this axis. |
| [Cols](#gridcols) | `Cols(tracks ...Track) *Grid` | Sets the column tracks for this axis. |
| [Add](#gridadd) | `Add(w core.Widget, row, col int) *Grid` | Places `w` at `(row, col)`. |
| [Children](#gridchildren) | `Children() []core.Widget` | Returns the widgets added, in `Add` order. |
| [MeasureContent](#gridmeasurecontent) | `MeasureContent(available render.Size) render.Size` | Measures every child once, then resolves tracks against `available`. |
| [ArrangeContent](#gridarrangecontent) | `ArrangeContent(bounds render.Rect)` | Re-resolves tracks against `bounds`, then arranges each child into its cell. |

#### Grid.Rows

Sets the row tracks for this axis, replacing any previously set rows.

**Syntax**

```go
func (g *Grid) Rows(tracks ...Track) *Grid
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `tracks` | `...Track` | The row tracks, in order, top to bottom. |

**Returns** — `*Grid` for chaining.

**Notes** — Layout-relevant: invalidates measure. Panics if any
already-`Add`ed cell's row index no longer fits the new track count.

**See also** — [Track](#track), [Grid.Cols](#gridcols)

#### Grid.Cols

Sets the column tracks for this axis, replacing any previously set
columns.

**Syntax**

```go
func (g *Grid) Cols(tracks ...Track) *Grid
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `tracks` | `...Track` | The column tracks, in order, left to right. |

**Returns** — `*Grid` for chaining.

**Notes** — Layout-relevant: invalidates measure. Panics if any
already-`Add`ed cell's column index no longer fits the new track count.

**See also** — [Track](#track), [Grid.Rows](#gridrows)

#### Grid.Add

Places `w` at `(row, col)`, re-parenting it to this `Grid` and
invalidating measure.

**Syntax**

```go
func (g *Grid) Add(w core.Widget, row, col int) *Grid
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `w` | `core.Widget` | The child to place. |
| `row` | `int` | Row index. Validated against the effective row track count (falls back to a single default `Star(1)` track if `Rows` was never called). |
| `col` | `int` | Column index. Validated against the effective column track count, same fallback rule. |

**Returns** — `*Grid` for chaining.

**Notes** — An out-of-range `row` or `col` panics.

#### Grid.Children

Returns the widgets added to this `Grid`, in `Add` order.

**Syntax**

```go
func (g *Grid) Children() []core.Widget
```

**Returns** — `[]core.Widget`. A copy; mutating it does not affect the
panel.

#### Grid.MeasureContent

Measures every child exactly once, then resolves the row and column
tracks against the space available to the grid.

**Syntax**

```go
func (g *Grid) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | Space offered to the grid. |

**Returns** — `render.Size`, the sum of the resolved column widths and row
heights. Per-child measurement uses per-axis available space equal to the
`Px` value for a `Px` track, or `+Inf` for `Auto`/`Star` tracks.

**See also** — [Track sizing worked example](#track-sizing-worked-example)

#### Grid.ArrangeContent

Re-resolves the row and column tracks against the grid's actual final
extent, then arranges each child into the rect of the track intersection
`(row, col)` it occupies.

**Syntax**

```go
func (g *Grid) ArrangeContent(bounds render.Rect)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `bounds` | `render.Rect` | The grid's final absolute bounds. |

**Notes** — `ArrangeWidget` handles stretch/alignment of each child within
its cell rect.

---

## DockPanel

Arranges children along its edges using the [Dock](#dock) enumeration; the
last child can optionally fill the remaining space — the classic
"toolbar + statusbar + sidebar + content" app-shell layout.

**Constructor**

```go
func NewDockPanel() *DockPanel
```

Returns a new `DockPanel` with `lastFills` defaulting to `true`.

**Example**

```go
shell := controls.NewDockPanel().
    Add(toolbar, controls.DockTop).
    Add(statusBar, controls.DockBottom).
    Add(sidebar, controls.DockLeft).
    Add(content, controls.DockLeft) // last item — fills, per lastFills
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [Add](#dockpaneladd) | `Add(w core.Widget, dock Dock) *DockPanel` | Appends a child with the given `Dock` value. |
| [SetLastChildFill](#dockpanelsetlastchildfill) | `SetLastChildFill(v bool) *DockPanel` | Sets whether the last child fills the remaining space. |
| [Children](#dockpanelchildren) | `Children() []core.Widget` | Returns the slice of child widgets. |
| [MeasureContent](#dockpanelmeasurecontent) | `MeasureContent(available render.Size) render.Size` | Measures all children and computes the desired size. |
| [ArrangeContent](#dockpanelarrangecontent) | `ArrangeContent(bounds render.Rect)` | Arranges children by carving space from the bounds per dock edge. |

#### DockPanel.Add

Appends a child to this `DockPanel` with the given `Dock` value,
re-parenting it and invalidating measure.

**Syntax**

```go
func (d *DockPanel) Add(w core.Widget, dock Dock) *DockPanel
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `w` | `core.Widget` | The child to append. |
| `dock` | `Dock` | The edge it docks to. Order matters: each item carves its slice off whatever space is left after the earlier items. |

**Returns** — `*DockPanel` for chaining.

**See also** — [Dock](#dock), [DockPanel.SetLastChildFill](#dockpanelsetlastchildfill)

#### DockPanel.SetLastChildFill

Sets whether the last child fills the remaining space.

**Syntax**

```go
func (d *DockPanel) SetLastChildFill(v bool) *DockPanel
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `bool` | `true` (the constructor default) makes the last *visible* child fill whatever space remains after the others are carved off; `false` docks it to its own edge like every other child. |

**Returns** — `*DockPanel` for chaining.

**Notes** — Layout-relevant: invalidates measure.

#### DockPanel.Children

Returns the slice of child widgets.

**Syntax**

```go
func (d *DockPanel) Children() []core.Widget
```

**Returns** — `[]core.Widget`. A copy; mutating it does not affect the
panel.

#### DockPanel.MeasureContent

Measures all children and computes the desired size using WPF
final-totals accumulation.

**Syntax**

```go
func (d *DockPanel) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | Space offered to the panel. |

**Returns** — `render.Size`. Each child is measured with the remaining
available space (clamped ≥ 0), then `remaining` is reduced by the child's
desired extent on its docked axis. Final desired size uses two-pass
accumulation: for `DockLeft`/`DockRight` items, `maxH = max(maxH, usedH +
d.H)` and `usedW += d.W`; `DockTop`/`DockBottom` mirror this on the other
axis; final desired is `(max(maxW, usedW), max(maxH, usedH))`.

**Notes** — The two-pass approach correctly handles cases where children
in one dock direction depend on the total extent of children in the
perpendicular direction (e.g. a left sidebar's needed height depends on how
much a top toolbar already used).

#### DockPanel.ArrangeContent

Arranges children by carving space from the bounds per dock edge.

**Syntax**

```go
func (d *DockPanel) ArrangeContent(bounds render.Rect)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `bounds` | `render.Rect` | The panel's final absolute bounds. |

**Notes** — The last *visible* child gets the whole remainder when
`lastFills` is true; otherwise every item (including the last) carves its
own slice off the appropriate edge, in `Add` order.

---

## WrapPanel

Arranges children horizontally in rows, wrapping to a new row when there
is not enough space. `Gap` specifies spacing between items in a row and
between rows.

**Constructor**

```go
func NewWrapPanel() *WrapPanel
```

Returns a new, empty `WrapPanel`.

**Example**

```go
tags := controls.NewWrapPanel().SetGap(6).
    Add(controls.NewFixed(60, 24, render.RGB(220, 220, 220)),
        controls.NewFixed(80, 24, render.RGB(220, 220, 220)),
        controls.NewFixed(50, 24, render.RGB(220, 220, 220)))
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [Add](#wrappaneladd) | `Add(children ...core.Widget) *WrapPanel` | Appends children, re-parenting each. |
| [SetGap](#wrappanelsetgap) | `SetGap(g float32) *WrapPanel` | Sets the spacing between items in a row and between rows. |
| [Children](#wrappanelchildren) | `Children() []core.Widget` | Returns a copy of the children slice. |
| [MeasureContent](#wrappanelmeasurecontent) | `MeasureContent(available render.Size) render.Size` | Measures all children and computes desired size using horizontal flow. |
| [ArrangeContent](#wrappanelarrangecontent) | `ArrangeContent(bounds render.Rect)` | Arranges children in rows, recomputing flow against `bounds.W`. |

#### WrapPanel.Add

Appends children to this `WrapPanel`, re-parenting each one and
invalidating measure.

**Syntax**

```go
func (w *WrapPanel) Add(children ...core.Widget) *WrapPanel
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `children` | `...core.Widget` | One or more widgets to append, in order. |

**Returns** — `*WrapPanel` for chaining.

#### WrapPanel.SetGap

Sets the spacing between items in a row and between rows.

**Syntax**

```go
func (w *WrapPanel) SetGap(g float32) *WrapPanel
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `g` | `float32` | Gap in pixels, used both between items within a row and between rows. |

**Returns** — `*WrapPanel` for chaining.

**Notes** — Layout-relevant: invalidates measure.

#### WrapPanel.Children

Returns a copy of the children slice.

**Syntax**

```go
func (w *WrapPanel) Children() []core.Widget
```

**Returns** — `[]core.Widget`. Mutating it does not affect the panel.

#### WrapPanel.MeasureContent

Measures all children and computes desired size using horizontal flow.

**Syntax**

```go
func (w *WrapPanel) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | Space offered to the panel; `available.W` is the wrap width. |

**Returns** — `render.Size`. Children are measured with
`(available.W, +Inf)`, then flowed into rows, wrapping to a new row when
`cursor + childW > available.W` (the first item in a row never wraps, even
if it alone overflows). Desired size is
`(max row width, sum of row heights + gaps between rows)`.

**Notes** — Hidden children are measured and excluded from flow — no slot,
no gap contribution.

#### WrapPanel.ArrangeContent

Arranges children in rows, recomputing flow against `bounds.W`.

**Syntax**

```go
func (w *WrapPanel) ArrangeContent(bounds render.Rect)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `bounds` | `render.Rect` | The panel's final absolute bounds. |

**Notes** — Each child's slot is
`{rowX, rowY, childDesired.W, rowHeight}`. Hidden children are arranged
with an empty rect; `core.ArrangeWidget` handles the short-circuit.

---

## Canvas

Positions children at fixed `(x, y)` offsets rather than flowing or
stretching them — the escape hatch for free-form/absolute layout, matching
WPF `Canvas` semantics.

Each child is measured with unbounded space and arranged at its own
desired size: no stretch on a canvas, so a child with `Stretch` alignment
simply gets its desired size, not the `Canvas`'s full extent. `Canvas`
itself always desires zero size — it is only useful given an explicit size
or a filled slot; it does not size to its children.

**Constructor**

```go
func NewCanvas() *Canvas
```

Returns a new, empty `Canvas`.

**Example**

```go
overlay := controls.NewCanvas().
    Add(badge, 200, 0).
    Add(cursor, 10, 340)
overlay.SetWidth(400)
overlay.SetHeight(400)
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [Add](#canvasadd) | `Add(w core.Widget, x, y float32) *Canvas` | Places `w` at the given `(x, y)` offset. |
| [Children](#canvaschildren) | `Children() []core.Widget` | Returns the child widgets in the order they were added. |
| [MeasureContent](#canvasmeasurecontent) | `MeasureContent(available render.Size) render.Size` | Measures every child with unbounded space; always desires zero. |
| [ArrangeContent](#canvasarrangecontent) | `ArrangeContent(bounds render.Rect)` | Places each child at `bounds.{X,Y}` offset by its `(x, y)`. |

#### Canvas.Add

Places `w` at the given `(x, y)` offset within this `Canvas`, re-parenting
it and invalidating measure.

**Syntax**

```go
func (c *Canvas) Add(w core.Widget, x, y float32) *Canvas
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `w` | `core.Widget` | The child to place. |
| `x` | `float32` | Horizontal offset, relative to the `Canvas`'s own bounds. |
| `y` | `float32` | Vertical offset, relative to the `Canvas`'s own bounds. |

**Returns** — `*Canvas` for chaining.

#### Canvas.Children

Returns the child widgets in the order they were added.

**Syntax**

```go
func (c *Canvas) Children() []core.Widget
```

**Returns** — `[]core.Widget`. A copy; mutating it does not affect the
panel.

#### Canvas.MeasureContent

Measures every child with unbounded `(+Inf, +Inf)` available space — a
canvas imposes no constraint on its children — and always desires zero
itself.

**Syntax**

```go
func (c *Canvas) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | Ignored — every child is measured with `+Inf` regardless. |

**Returns** — `render.Size{}`, always. Per WPF semantics, a `Canvas`'s own
extent is driven by its parent, not by its children's positions/sizes.

#### Canvas.ArrangeContent

Places each child at `bounds.{X,Y}` offset by its `(x, y)`, using its own
desired size as the slot.

**Syntax**

```go
func (c *Canvas) ArrangeContent(bounds render.Rect)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `bounds` | `render.Rect` | The canvas's own final absolute bounds. |

**Notes** — `ArrangeWidget`'s own margin inset then nets the child's final
bounds down to `childDesired − margins` (per axis, floored at zero) — the
same pattern `StackPanel`/`DockPanel` use, since margins are only reachable
through `ArrangeWidget`. Every child is always arranged, even hidden ones —
`ArrangeWidget` zeroes a hidden child's bounds itself.

---

## Orientation

Specifies the direction in which a [StackPanel](#stackpanel) stacks its
children.

```go
type Orientation uint8
```

### Constants

| Name | Value | Description |
|---|---|---|
| `Vertical` | `0` | Stacks children top to bottom. |
| `Horizontal` | `1` | Stacks children left to right. |

**See also** — [StackPanel](#stackpanel)

---

## Dock

Specifies the edge to which a [DockPanel](#dockpanel) child is docked.

```go
type Dock uint8
```

### Constants

| Name | Value | Description |
|---|---|---|
| `DockLeft` | `0` | Docks a child to the left edge. |
| `DockTop` | `1` | Docks a child to the top edge. |
| `DockRight` | `2` | Docks a child to the right edge. |
| `DockBottom` | `3` | Docks a child to the bottom edge. |

**See also** — [DockPanel](#dockpanel)

---

## Track

Defines the sizing behavior of one [Grid](#grid) row or column: a fixed
pixel size (`Px`), sized-to-content (`AutoTrack`), or a proportional share
of the remaining space (`Star`).

```go
type Track struct {
    // unexported
}
```

`Track` has no exported fields or methods — it's an opaque value built
exclusively by the three package-level functions below and consumed by
`Grid.Rows`/`Grid.Cols`. See [Grid's worked
example](#track-sizing-worked-example) for how the three kinds resolve
together against real numbers.

### Functions

| Function | Signature | Description |
|---|---|---|
| [Px](#px) | `func Px(v float32) Track` | Returns a track with a fixed size of `v`. |
| [AutoTrack](#autotrack) | `func AutoTrack() Track` | Returns a track sized to the largest desired extent of its children. |
| [Star](#star) | `func Star(weight float32) Track` | Returns a track that receives a proportional share of the remaining space. |

#### Px

Returns a track with a fixed size of `v`.

**Syntax**

```go
func Px(v float32) Track
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `float32` | The fixed size, in pixels. |

**Returns** — `Track`.

**Example**

```go
grid.Cols(controls.Px(160), controls.Star(1))
```

**See also** — [AutoTrack](#autotrack), [Star](#star)

#### AutoTrack

Returns a track sized to the largest desired extent (on that axis) of the
children assigned to it.

**Syntax**

```go
func AutoTrack() Track
```

**Returns** — `Track`.

**Example**

```go
grid.Rows(controls.AutoTrack(), controls.Star(1))
```

**See also** — [Px](#px), [Star](#star)

#### Star

Returns a track that receives a proportional share of the space remaining
after `Px` and `Auto` tracks are resolved, weighted by `weight` against the
other `Star` tracks on the same axis.

**Syntax**

```go
func Star(weight float32) Track
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `weight` | `float32` | This track's share of the remaining space, relative to the sum of all `Star` weights on the axis (e.g. `Star(1)` next to `Star(2)` splits the remainder 1:2). |

**Returns** — `Track`.

**Notes** — If the axis's available space is `+Inf` (an unbounded
measure), a `Star` track resolves like `AutoTrack` instead — there is no
"remaining space" to divide. If every `Star` track on an axis has
`weight == 0`, no weight exists to divide the remaining space by; those
tracks stay at zero and the leftover space is left unallocated. This is a
deliberate v0 choice, not a bug — silently treating a zero-weight `Star`
as "take everything" or "take nothing but still reserve space" would both
be surprising. Give a track a positive weight if you want it to consume
the remainder.

**Example**

```go
grid.Cols(controls.Px(160), controls.Star(1), controls.Star(2)) // 1:2 split of the leftover width
```

**See also** — [Px](#px), [AutoTrack](#autotrack), [Grid](#grid)
