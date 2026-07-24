# Overlays, Menus, and Window Chrome

This group covers everything in `controls` that renders above or outside
ordinary document flow. `OverlayHost` is the foundation: a popup/modal layer
that a `MenuBar`'s dropdown, `ShowDialog`'s modal card, and `ToolTipArea`'s
tip all render into — every other type on this page either builds a popup
for an `OverlayHost` to show, or is unrelated window chrome (`TabControl`,
`TitleBar`, `AcrylicSurface`) that happens to live alongside them. Reach for
this page when you need floating UI above your content (menus, dialogs,
tooltips), a tabbed content switcher, or custom (undecorated) window chrome.

**Import:** `github.com/0xdreadnaught/fluo/controls`

## Contents
- [OverlayHost](#overlayhost)
- [MenuBar](#menubar)
- [MenuItems](#menuitems)
- [ShowContextMenu](#showcontextmenu)
- [DialogResult](#dialogresult)
- [DialogSpec](#dialogspec)
- [ShowDialog](#showdialog)
- [ToolTipArea](#tooltiparea)
- [TabControl](#tabcontrol)
- [TitleBar](#titlebar)
- [AcrylicSurface](#acrylicsurface)

---

## OverlayHost

`OverlayHost` hosts the app's content plus a stack of popups rendered above
it. It should be the root (or near-root) widget of the tree — controls that
open a popup (a `MenuBar` title, `ShowDialog`, `ToolTipArea`) find the
nearest one via `OverlayHostFor` rather than being handed a reference
directly. Children are ordered `[content, popups...]` (last popup =
topmost), which is what makes hit-testing find the topmost popup before
content, and rendering paint it last (above everything beneath it).
`OverlayHost` itself draws nothing — `Render` is the inherited
`core.Element` no-op; content and popups render entirely as children.

**Modal vs non-modal popups.** `ShowPopup` opens a MODAL popup (a `MenuBar`
dropdown, a `ShowDialog` card); `ShowPopupNonModal` opens one that is not (a
`ToolTipArea` tip). The two are otherwise identical — same stack, same
z-order for hit-testing and rendering, same detach-on-close via
`ClosePopup` — but only a modal popup engages the capture-based light-dismiss
machinery below; a non-modal popup relies entirely on the router's ordinary
(uncaptured) hit-testing and hover-diffing to place and close itself (its
owner, e.g. `ToolTipArea`, closes it on `Leave`). If a modal popup is also
open at the same time, a non-modal one is routed exactly as if it were
modal — the modal capture governs all pointer routing regardless of which
popup happens to be topmost.

**Modal capture and light dismiss.** A stray press must not reach content
underneath an open modal popup, and content's own widgets are always deeper
in the bubble path than `OverlayHost` (the root) — they'd receive a `Press`
before `OverlayHost` ever ran, under ordinary bubble order. To pre-empt
that, `OverlayHost` captures the wired router (see `SetRouter`) for as long
as at least one modal popup is open: every pointer event routes directly to
`OnPointer`, bypassing hit-testing into content entirely.

**Chain-aware forwarding and dismissal.** `OnPointer` re-hit-tests the whole
popup stack, top-down, for the popup actually containing the event (not
just the topmost one) and forwards into that popup's own subtree, closing
whatever popups sit above it first:
  - A point inside no popup at all: a `Move` clears synthesized hover; a
    `Press` closes every popup on the stack, topmost first, and swallows
    the event.
  - A point inside popup `idx`'s bounds: any popup opened later than `idx`
    (e.g. a deeper submenu the pointer no longer falls within) is closed
    first, then the event is forwarded into `idx`'s own subtree via
    `input.HitPath`/`input.Bubble` — so popup-internal controls (menu item
    rows, a nested submenu) still receive `Press`/`Release`/`Move`/`Wheel`
    normally. This is what makes hovering across a `MenuBar`'s open titles
    slide the open popup, and what makes a parent menu's sibling rows
    reachable again while a submenu is open (hovering one auto-closes the
    submenu above it).

Hover (`Enter`/`Leave`) is the one exception: `input.Router`'s own
hover-diffing never runs while `OverlayHost` holds the capture, so
`OnPointer` replicates that diff-and-notify algorithm itself, scoped to
whichever popup's subtree currently contains the pointer — this is what
lets a `menuSubRow`'s hover-opens-submenu behavior fire from real mouse
movement rather than only from clicks.

**Constructor**

```go
func NewOverlayHost() *OverlayHost
```

Returns an empty `OverlayHost` with no content and no popups.

**Example**

```go
host := controls.NewOverlayHost()
host.SetRouter(router)
host.SetContent(mainPanel)
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [SetContent](#overlayhostsetcontent) | `SetContent(w core.Widget) *OverlayHost` | Sets the single always-visible content widget. |
| [SetRouter](#overlayhostsetrouter) | `SetRouter(r *input.Router)` | Wires the router that drives modal capture and detach-on-close. |
| [ShowPopup](#overlayhostshowpopup) | `ShowPopup(popup core.Widget, anchor render.Rect, onDismiss func())` | Opens a MODAL popup, anchored below (or above) `anchor`. |
| [ShowPopupNonModal](#overlayhostshowpopupnonmodal) | `ShowPopupNonModal(popup core.Widget, anchor render.Rect, onDismiss func())` | Opens a non-modal popup — same placement/stacking, no capture. |
| [ClosePopup](#overlayhostclosepopup) | `ClosePopup(popup core.Widget)` | Closes `popup`, wherever it sits in the stack. Idempotent. |
| [CloseTopPopup](#overlayhostclosetoppopup) | `CloseTopPopup()` | Closes only the topmost popup. |
| [PopupCount](#overlayhostpopupcount) | `PopupCount() int` | Returns how many popups are currently open. |
| [CloseAllPopups](#overlayhostcloseallpopups) | `CloseAllPopups()` | Closes every open popup, topmost first. |
| [MeasureContent](#overlayhostmeasurecontent) | `MeasureContent(available render.Size) render.Size` | Measures content and every open popup. |
| [ArrangeContent](#overlayhostarrangecontent) | `ArrangeContent(bounds render.Rect)` | Arranges content to fill the host, places each popup by its anchor. |
| [Children](#overlayhostchildren) | `Children() []core.Widget` | Returns `[content, popups...]`, topmost popup last. |
| [OnKey](#overlayhostonkey) | `OnKey(e *input.KeyEvent)` | Delegates unfocused key events to content. |
| [OnPointer](#overlayhostonpointer) | `OnPointer(e *input.PointerEvent)` | Chain-aware modal forwarding/dismissal (see above). |

Also see the package-level [OverlayHostFor](#overlayhostfor) function, which
every popup-capable control uses to find its host.

#### OverlayHost.SetContent

Sets (replacing any existing) the single always-visible content widget.

**Syntax**

```go
func (h *OverlayHost) SetContent(w core.Widget) *OverlayHost
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `w` | `core.Widget` | The content widget, re-parented to this host. |

**Returns** — `*OverlayHost` for chaining.

**Example**

```go
host.SetContent(mainPanel)
```

**Notes** — Any previously set content is detached (its parent cleared)
first, matching the `SetChild` convention used elsewhere in `controls`.
Invalidates measure.

**See also** — [Children](#overlayhostchildren)

#### OverlayHost.SetRouter

Wires the `input.Router` that dispatches to this tree.

**Syntax**

```go
func (h *OverlayHost) SetRouter(r *input.Router)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `*input.Router` | The router used to capture pointer input while a modal popup is open, and to clear stale focus/capture/hover via `Detach` when a popup closes. `nil` (the zero value) is valid — it disables both behaviors; `ClosePopup`/`CloseTopPopup` still work, they just skip `Detach`, and light-dismiss falls back to whatever the ordinary (uncaptured) bubble delivers. |

**Example**

```go
host.SetRouter(router)
```

**Notes** — Normally called once, e.g. by the host application right after
constructing its `input.Router`.

**See also** — [ShowPopup](#overlayhostshowpopup)

#### OverlayHost.ShowPopup

Opens `popup` as a MODAL popup.

**Syntax**

```go
func (h *OverlayHost) ShowPopup(popup core.Widget, anchor render.Rect, onDismiss func())
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `popup` | `core.Widget` | The popup widget. Becomes the new topmost popup — hit-tests and renders above every existing popup and content. |
| `anchor` | `render.Rect` | Screen-space rect the popup is opened against (typically the opener's own bounds). The preferred position is directly below `anchor`, left-aligned with it; the popup flips above `anchor` if it would overflow the host's bottom edge, and is always clamped horizontally to stay within the host's bounds. |
| `onDismiss` | `func()` | Fires exactly once, when `popup` is later removed via `ClosePopup`/`CloseTopPopup` — whether from light-dismiss or an explicit close. May be `nil`. |

**Notes** — While at least one modal popup is open, `ShowPopup` captures
the wired router (see `SetRouter`) on this host, so pointer input routes to
`OnPointer` instead of hit-testing into content; re-engaging the capture is
a no-op if this host already holds it. Use `ShowPopupNonModal` for a popup
that should not engage any of this.

**Example**

```go
anchor := core.BoundsOf(triggerButton)
host.ShowPopup(customPopup, anchor, func() {
    fmt.Println("popup dismissed")
})
```

**See also** — [ShowPopupNonModal](#overlayhostshowpopupnonmodal), [ClosePopup](#overlayhostclosepopup), [OnPointer](#overlayhostonpointer)

#### OverlayHost.ShowPopupNonModal

Opens `popup` exactly like `ShowPopup` — same placement, stacking, and
`onDismiss`-once-on-close contract — except it never engages the router.

**Syntax**

```go
func (h *OverlayHost) ShowPopupNonModal(popup core.Widget, anchor render.Rect, onDismiss func())
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `popup` | `core.Widget` | The popup widget. |
| `anchor` | `render.Rect` | Screen-space anchor rect; same placement rules as `ShowPopup`. |
| `onDismiss` | `func()` | Fires once on close. May be `nil`. |

**Notes** — No capture is taken on this host on `popup`'s account, so it is
never light-dismissed: it hit-tests/renders topmost purely from its
position in the popup stack, and closing it is entirely up to its owner
(e.g. `ToolTipArea` closing its own tip on `Leave`). If a modal popup is
also open, that modal capture still governs event routing for every popup
regardless of which is topmost.

**Example**

```go
host.ShowPopupNonModal(tipPopup, ta.Bounds(), func() {
    // tip closed
})
```

**See also** — [ShowPopup](#overlayhostshowpopup), [ToolTipArea](#tooltiparea)

#### OverlayHost.ClosePopup

Removes `popup` from the stack, wherever it sits in it.

**Syntax**

```go
func (h *OverlayHost) ClosePopup(popup core.Widget)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `popup` | `core.Widget` | The popup to close. |

**Notes** — Idempotent: a `popup` not currently in the stack (already
closed, or never opened) is a no-op — `onDismiss` does not fire again on a
repeat call. On an actual close, `popup` is detached, the wired router (if
any) has `Detach(popup)` called on it, `onDismiss` fires, and measure is
invalidated. If no popup on the stack is modal afterward and this host
currently holds the router's pointer capture, the capture is released so
ordinary hit-testing into content resumes.

**Example**

```go
host.ClosePopup(customPopup)
```

**See also** — [CloseTopPopup](#overlayhostclosetoppopup), [CloseAllPopups](#overlayhostcloseallpopups)

#### OverlayHost.CloseTopPopup

Closes only the topmost (last-shown) popup.

**Syntax**

```go
func (h *OverlayHost) CloseTopPopup()
```

**Notes** — A no-op when no popup is open. Follows `ClosePopup`'s contract.

**Example**

```go
host.CloseTopPopup()
```

**See also** — [ClosePopup](#overlayhostclosepopup)

#### OverlayHost.PopupCount

Returns how many popups are currently open.

**Syntax**

```go
func (h *OverlayHost) PopupCount() int
```

**Returns** — `int`, the number of popups currently on the stack.

**Example**

```go
if host.PopupCount() > 0 {
    host.CloseAllPopups()
}
```

#### OverlayHost.CloseAllPopups

Closes every open popup, topmost first, via repeated `CloseTopPopup` calls
— so each popup's `onDismiss` fires exactly once, in topmost-to-bottommost
order.

**Syntax**

```go
func (h *OverlayHost) CloseAllPopups()
```

**Notes** — A no-op when no popup is open. Added for the menu family: a
menu-item click needs to collapse an arbitrarily deep stack of nested
submenu popups in one call. This is a v0 simplification for callers that
know every popup currently open belongs to the same menu/submenu chain — it
closes EVERY popup on this host indiscriminately, including any unrelated
one (e.g. a dropdown left open elsewhere) that happens to also be open at
the same time.

**Example**

```go
host.CloseAllPopups()
```

**See also** — [CloseTopPopup](#overlayhostclosetoppopup)

#### OverlayHost.MeasureContent

Measures content and every open popup, each with the full available space.

**Syntax**

```go
func (h *OverlayHost) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | Space offered by the parent layout pass. |

**Returns** — `render.Size`, content's desired size — popups are
overlay-positioned and never affect it.

**Notes** — Called by the layout engine (`core.MeasureWidget`); not
normally called directly by app code. Popups size to their own content,
same as content itself; neither is narrowed on the host's account.

#### OverlayHost.ArrangeContent

Arranges content to fill the host's full bounds, then places each popup at
its computed position.

**Syntax**

```go
func (h *OverlayHost) ArrangeContent(bounds render.Rect)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `bounds` | `render.Rect` | The host's arranged bounds. |

**Notes** — Called by the layout engine (`core.ArrangeWidget`); not
normally called directly. Each popup is sized to exactly its own desired
size — never stretched or otherwise touched by the popup widget's own
alignment.

#### OverlayHost.Children

Returns `[content, popups...]` — content first, popups in stack order,
topmost last.

**Syntax**

```go
func (h *OverlayHost) Children() []core.Widget
```

**Returns** — `[]core.Widget`. A host with no content yet set simply omits
it (no nil entries).

**Notes** — This order is why hit-testing finds the topmost popup before
content, and why rendering paints it above everything beneath it.

#### OverlayHost.OnKey

Implements `input.KeyHandler`: delegates to content's own `OnKey` (if
content implements `input.KeyHandler`), but only when no widget currently
holds keyboard focus.

**Syntax**

```go
func (h *OverlayHost) OnKey(e *input.KeyEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.KeyEvent` | The key event. |

**Notes** — `OverlayHost` is a purely structural root that draws nothing,
but `input.Router.dispatchKey` delivers an unfocused key event to the bare
router root alone, walking no further into the tree. Delegating here
restores window-level accelerators hosted on content that would otherwise
never fire until something happened to be focused. The focused-widget case
needs no such forwarding (content already receives the event once through
the ordinary bubble), so this only fires when `e.Router.Focused() == nil`.
Popups are deliberately excluded — this never forwards into open popups.

#### OverlayHost.OnPointer

Implements `input.PointerHandler`: the chain-aware modal forwarding and
dismissal described in the type's overview above.

**Syntax**

```go
func (h *OverlayHost) OnPointer(e *input.PointerEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.PointerEvent` | The pointer event. |

**Notes** — Guarded on `hasModalPopup`, not merely an empty stack: with no
modal popup open, this is reached only via the ordinary (uncaptured) bubble
and does nothing, since only NON-modal popups (which never light-dismiss)
are open. Once a modal popup is open, this is always the captured-forwarding
call, and proceeds as described in the type's overview.

**See also** — [ShowPopup](#overlayhostshowpopup), [CloseAllPopups](#overlayhostcloseallpopups)

#### OverlayHostFor

Walks the ancestor chain from `w`, looking for the nearest `*OverlayHost`.

**Syntax**

```go
func OverlayHostFor(w core.Widget) *OverlayHost
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `w` | `core.Widget` | The widget to search from (inclusive of `w` itself). |

**Returns** — `*OverlayHost`, or `nil` if `w` has no `OverlayHost` ancestor
(e.g. it isn't attached to a tree rooted at one).

**Example**

```go
if host := controls.OverlayHostFor(owner); host != nil {
    host.ShowPopup(popup, anchor, nil)
}
```

**See also** — [OverlayHost](#overlayhost)

---

## MenuBar

`MenuBar` is a horizontal row of top-level menu titles (e.g. "File",
"Edit"): clicking one opens its popup menu (see [MenuItems](#menuitems)) — a
raised-bevel-framed MODAL popup, placed directly below the clicked title.
`MenuBar` draws its cells directly (titles measured against its face, no
per-cell child widgets).

**Normative:** `MenuBar` itself is the only focusable part of the whole
menu family (menu popup rows are never focusable) — a title click both
opens the popup and, via the router's ordinary press-to-focus, focuses the
bar itself; it stays focused for as long as any menu popup is open, so
Escape reaches `MenuBar.OnKey` through the router's ordinary focused-widget
key dispatch. `MenuBar.OnKey`'s Escape handler closes EVERY open popup
(top-level and any open submenu chain) via `OverlayHost.CloseAllPopups`.

**Constructor**

```go
func NewMenuBar(face *text.Face) *MenuBar
```

Returns an empty `MenuBar` (no top-level menus yet), drawing titles with
`face` (`face` may be `nil`, per `TextBlock`'s nil-face convention).

**Example**

```go
bar := controls.NewMenuBar(face)
bar.AddMenu("File").
    Add("New", newFile).
    AddSeparator().
    Add("Exit", exitApp)

edit := bar.AddMenu("Edit")
edit.AddSub("Recent").
    Add("report.txt", func() { openRecent("report.txt") }).
    Add("notes.txt", func() { openRecent("notes.txt") })
edit.Add("Paste", pasteFn)
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [AddMenu](#menubaraddmenu) | `AddMenu(title string) *MenuItems` | Appends a top-level entry and returns a fresh builder for its contents. |
| [MeasureContent](#menubarmeasurecontent) | `MeasureContent(available render.Size) render.Size` | Measures the bar's own cells. |
| [Render](#menubarrender) | `Render(r render.Renderer)` | Draws the bar and its cells. |
| [AcceptsFocus](#menubaracceptsfocus) | `AcceptsFocus() bool` | Always `true`. |
| [OnPointer](#menubaronpointer) | `OnPointer(e *input.PointerEvent)` | Hover tracking and opening a menu on press. |
| [OnKey](#menubaronkey) | `OnKey(e *input.KeyEvent)` | Escape closes every open popup. |

#### MenuBar.AddMenu

Appends a new top-level entry titled `title` and returns a fresh
`MenuItems` builder for its popup contents.

**Syntax**

```go
func (m *MenuBar) AddMenu(title string) *MenuItems
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `title` | `string` | The top-level menu title (e.g. "File"). |

**Returns** — `*MenuItems`, a fresh builder — chain `Add`/`AddSeparator`/
`AddSub` calls directly onto it.

**Example**

```go
bar.AddMenu("File").Add("New", newFn).AddSeparator().Add("Exit", exitFn)
```

**Notes** — Invalidates measure.

**See also** — [MenuItems](#menuitems)

#### MenuBar.MeasureContent

Recomputes each cell's width from the current entries and reports their
sum, plus cell height, as the bar's own desired size.

**Syntax**

```go
func (m *MenuBar) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | Space offered by the parent layout pass. |

**Returns** — `render.Size`.

**Notes** — Called by the layout engine; not normally called directly.

#### MenuBar.Render

Fills the bar's bounds `ButtonFace`, then draws each cell: the cell whose
menu is currently open gets a sunken "pressed" look; the hovered cell gets
a navy `Highlight` bar; every other cell is plain `WindowText` over
`ButtonFace`.

**Syntax**

```go
func (m *MenuBar) Render(r render.Renderer)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | The target renderer. |

**Notes** — Skipped entirely with a nil face.

#### MenuBar.AcceptsFocus

Implements `input.Focusable`.

**Syntax**

```go
func (m *MenuBar) AcceptsFocus() bool
```

**Returns** — `bool`, always `true` — v0 has no disabled concept for
`MenuBar`.

#### MenuBar.OnPointer

Implements `input.PointerHandler`: `Move`/`Leave` update the hovered cell
for the hover-fill visual; a `Press` landing on a real cell opens that
cell's menu.

**Syntax**

```go
func (m *MenuBar) OnPointer(e *input.PointerEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.PointerEvent` | The pointer event. |

**Notes** — A press landing on a real cell is marked handled; one missing
every cell is left unhandled (defensive — should not occur in practice).

#### MenuBar.OnKey

Implements `input.KeyHandler`: Escape, while any menu is open, closes every
open popup (the top-level menu and any open submenu chain) via
`OverlayHost.CloseAllPopups`.

**Syntax**

```go
func (m *MenuBar) OnKey(e *input.KeyEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.KeyEvent` | The key event. |

**Notes** — A no-op (event left unhandled) when no menu is open, or for
anything but `Action == input.Press` — matching `ComboBox.OnKey`'s own
Esc-guard convention.

**See also** — [OverlayHost.CloseAllPopups](#overlayhostcloseallpopups)

---

## MenuItems

`MenuItems` is a builder for one popup menu's contents — the top-level
content of a `MenuBar` entry, a nested submenu (`AddSub`), or a
`ShowContextMenu` popup. It records entries as plain data; the actual popup
widget tree is constructed fresh every time the menu is opened, so edits
made to a `MenuItems` while its menu is closed are always reflected the
next time it opens.

There is no exported constructor: a `MenuItems` is obtained from
`MenuBar.AddMenu`, from an existing `MenuItems`' own `AddSub`, or from the
`build` callback passed to [ShowContextMenu](#showcontextmenu).

**Normative:** clicking any item (`Add`) fires that item's `onClick` and
then closes every open menu popup — the whole menu family follows an "item
click fires + closes ALL menus" rule. A submenu (`AddSub`) opens on hover
(immediate v0, no dwell delay), not click.

### Methods

| Method | Signature | Description |
|---|---|---|
| [Add](#menuitemsadd) | `Add(label string, onClick func()) *MenuItems` | Appends a clickable item. |
| [AddSeparator](#menuitemsaddseparator) | `AddSeparator() *MenuItems` | Appends an inert 1px-rule separator row. |
| [AddSub](#menuitemsaddsub) | `AddSub(label string) *MenuItems` | Appends a submenu trigger and returns a fresh builder for it. |

#### MenuItems.Add

Appends a clickable item with the given label.

**Syntax**

```go
func (mi *MenuItems) Add(label string, onClick func()) *MenuItems
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `label` | `string` | The item's display text. |
| `onClick` | `func()` | Fires when the user clicks the row — which ALSO closes every open menu popup, per the package's "item click fires + closes ALL menus" rule. May be `nil`. |

**Returns** — `*MenuItems` (the receiver), for chaining further `Add`/
`AddSeparator`/`AddSub` calls onto the same menu.

**Example**

```go
file.Add("New", newFn).Add("Open", openFn)
```

**See also** — [AddSeparator](#menuitemsaddseparator), [AddSub](#menuitemsaddsub)

#### MenuItems.AddSeparator

Appends an inert 1px-rule separator row.

**Syntax**

```go
func (mi *MenuItems) AddSeparator() *MenuItems
```

**Returns** — `*MenuItems` (the receiver), for chaining.

**Example**

```go
file.Add("New", newFn).AddSeparator().Add("Exit", exitFn)
```

**Notes** — Never highlights on hover, never fires a click, and never
closes anything — a forwarded pointer event that hits it simply finds no
handler and goes nowhere.

#### MenuItems.AddSub

Appends a submenu trigger row labeled `label` and returns a fresh
`MenuItems` builder for its contents.

**Syntax**

```go
func (mi *MenuItems) AddSub(label string) *MenuItems
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `label` | `string` | The submenu trigger's display text. |

**Returns** — A fresh (nested) `*MenuItems` — unlike `Add`/`AddSeparator`
(which return the receiver, for chaining more entries onto the SAME menu),
callers chain directly onto the returned value:
`mi.AddSub("Recent").Add("a.txt", fn).Add("b.txt", fn)`.

**Example**

```go
edit.AddSub("Recent").
    Add("report.txt", openReport).
    Add("notes.txt", openNotes)
```

**Notes** — The submenu opens on hover (immediate v0, no dwell delay),
anchored to the right of its own row, as a second, nested popup on the
host's stack. A submenu opened near the host's right edge, wide enough to
overflow, is clamped back leftward rather than flipping to the left of its
trigger row — it can end up overlapping the trigger and its parent popup
rather than sitting cleanly beside it (a documented placement gap, not
addressed by the chain-aware forwarding/dismissal logic).

**See also** — [OverlayHost](#overlayhost)

---

## ShowContextMenu

`ShowContextMenu` opens an app-invoked context (right-click-style) menu, as
a MODAL popup anchored at a point rather than a rect — its top-left corner
lands exactly at `at`.

**Syntax**

```go
func ShowContextMenu(owner core.Widget, at render.Point, face *text.Face, build func(*MenuItems))
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `owner` | `core.Widget` | Anchors the `OverlayHostFor` lookup — need not be the widget visually "under" `at`, just something attached beneath the same `OverlayHost`. |
| `at` | `render.Point` | Screen-space point (e.g. the pointer position at the moment of the right-click) the popup's top-left corner is placed at. |
| `face` | `*text.Face` | Face for the menu's item labels. May be `nil`, per `TextBlock`'s nil-face convention. |
| `build` | `func(*MenuItems)` | Called once with a fresh `MenuItems` to populate the popup's contents. May be `nil` (an empty menu). |

**Returns** — none.

**Example**

```go
func (p *panel) OnPointer(e *input.PointerEvent) {
    if e.Button == input.ButtonRight && e.Action == input.Press {
        controls.ShowContextMenu(p, e.Pos, face, func(mi *controls.MenuItems) {
            mi.Add("Copy", copyFn)
            mi.Add("Paste", pasteFn)
        })
        e.Handled = true
    }
}
```

**Notes** — v0 has no global right-click infrastructure: nothing in fluo
itself listens for `input.ButtonRight` and calls this automatically — the
app is responsible for invoking it, typically from a widget's own
`OnPointer`. A no-op if `owner` isn't (yet) attached beneath an
`OverlayHost`. Each click on a resulting item row fires that item's own
`onClick` and then closes every open popup, same as any other menu popup.
The popup itself is opened with a `nil` onDismiss — unlike `MenuBar`,
`ShowContextMenu` keeps no state of its own to reset; every call is a
fresh, independent popup.

**Not in the originally-scoped type list** — exported and part of the menu
family, documented here for completeness (see the report at the end of
this task).

**See also** — [MenuItems](#menuitems), [OverlayHost](#overlayhost)

---

## DialogResult

`DialogResult` identifies how a dialog shown via `ShowDialog` was closed.

```go
type DialogResult uint8
```

### Values

| Name | Description |
|---|---|
| `DialogPrimary` | The user clicked the Primary button. |
| `DialogSecondary` | The user clicked the Secondary button. |
| `DialogDismissed` | The user pressed Escape. There is no other dismiss path in v0 — a press on the scrim itself (outside the card) is a documented no-op, not a dismissal. |

**See also** — [DialogSpec](#dialogspec), [ShowDialog](#showdialog)

---

## DialogSpec

`DialogSpec` describes one modal dialog's content and buttons, passed to
`ShowDialog`. It has no constructor and no methods — construct it as a
struct literal.

### Fields

| Name | Type | Description |
|---|---|---|
| `Title` | `string` | Shown in the card's gradient caption strip. Skipped entirely (no caption space reserved) when empty. |
| `Body` | `string` | The dialog's body text, drawn below the caption. |
| `Primary` | `string` | Label for the accent (main-action) button, right-aligned rightmost. An empty label omits the button entirely. |
| `Secondary` | `string` | Label for the default button, right-aligned to the left of Primary. An empty label omits the button entirely. Both `Primary` and `Secondary` empty shows no buttons at all — Escape is then the only way to close the dialog. |
| `OnResult` | `func(DialogResult)` | Fires exactly once with the result the dialog closed with. May be `nil`. |

**Example** — see [ShowDialog](#showdialog)'s worked example below.

**See also** — [DialogResult](#dialogresult), [ShowDialog](#showdialog)

---

## ShowDialog

`ShowDialog` is fluo's dialog-showing function — there is no `NewDialog`
constructor; a dialog is a `DialogSpec` shown via this function.

**Syntax**

```go
func ShowDialog(host *OverlayHost, face *text.Face, d DialogSpec)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `host` | `*OverlayHost` | The host to open the dialog on. A `nil` host is a no-op. |
| `face` | `*text.Face` | Supplies the button/body/caption type ramp — body and caption text are drawn at `theme.Active().Type.BodySize`/`SubtitleSize` via faces derived from `face.Font`. May be `nil`, in which case no text (and no caption strip) is drawn, per `TextBlock`'s nil-face convention. |
| `d` | `DialogSpec` | The dialog's content and buttons. |

**Returns** — none; observe the outcome via `d.OnResult`.

**Example**

```go
d := controls.DialogSpec{
    Title:     "Discard changes?",
    Body:      "You have unsaved changes. Do you want to discard them?",
    Secondary: "Cancel",
    Primary:   "Discard",
    OnResult: func(result controls.DialogResult) {
        switch result {
        case controls.DialogPrimary:
            discardChanges()
        case controls.DialogSecondary, controls.DialogDismissed:
            // user backed out; nothing to do
        }
    },
}
controls.ShowDialog(host, bodyFace, d)
```

**Notes**
- Opens `d` as a single MODAL popup: a full-host scrim (painted invisibly —
  authentic Windows-2000 modals had no dark scrim) wrapping a centered card
  with a gradient caption strip and a right-aligned button row.
- **Scrim neutralizes light-dismiss by construction:** the scrim's desired
  size is unconditionally the full available space, anchored at the host's
  own bounds — so every press, wherever it lands within the host, falls
  inside `OverlayHost.OnPointer`'s "inside the topmost popup" branch. The
  outside-press light-dismiss branch can never run while the dialog is the
  topmost popup. A press on the scrim itself (outside the card) is
  forwarded into the scrim's own subtree like any popup-internal press, but
  finds no `input.PointerHandler` there and goes nowhere — a documented
  v0 no-op, not light-dismiss under a different name.
- Escape closes the dialog with `DialogDismissed`. `ShowDialog` explicitly
  focuses the scrim widget itself (not a button) the moment the popup
  opens, so Escape reaches it regardless of which button (if any) a
  subsequent click focuses.
- **CAUTION:** the dialog steals keyboard focus to its scrim and does NOT
  restore prior focus on close (v0). Callers needing focus restoration must
  track and re-apply the prior focused widget themselves.
- A card button click closes the dialog and records the matching result
  before doing so; Escape records `DialogDismissed` the same way. Either
  path converges on the same one-shot latch: `d.OnResult` fires on the
  first close and silently no-ops on any later one (covers, for instance,
  clicking a button then sending Escape).

**See also** — [DialogSpec](#dialogspec), [DialogResult](#dialogresult), [OverlayHost](#overlayhost)

---

## ToolTipArea

`ToolTipArea` is a transparent, single-child wrapper widget (Border-like:
`MeasureContent`/`ArrangeContent`/`Children` all delegate entirely to
`child`, so the wrapper adds no chrome) that shows a small tip popup after
the pointer dwells over it.

It implements `input.PointerHandler` (`Enter`/`Leave` only) but deliberately
never sets `e.Handled` — a tooltip must not swallow hover events that
`child` (or anything wrapping this `ToolTipArea`) also wants to see. The tip
popup is opened via `OverlayHost.ShowPopupNonModal`, not `ShowPopup`: it is
never light-dismissed, and is closed exclusively on `Leave` — see
[OverlayHost](#overlayhost)'s "Modal vs non-modal popups" section for why a
non-modal popup needs none of the capture-forwarding machinery a modal one
does.

**Constructor**

```go
func NewToolTipArea(child core.Widget, face *text.Face, tip string) *ToolTipArea
```

Returns a `ToolTipArea` wrapping `child` (re-parented to it), showing `tip`
in `face` (`face` may be `nil`, per `TextBlock`'s convention) when hovered.

**Example**

```go
btn := controls.NewButton(face, "Save")
tip := controls.NewToolTipArea(btn, face, "Save the current document")
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [SetTimers](#tooltipareasettimers) | `SetTimers(q *timers.Queue) *ToolTipArea` | Wires the hover-delay driver. |
| [MeasureContent](#tooltipareameasurecontent) | `MeasureContent(available render.Size) render.Size` | Measures `child` unchanged. |
| [ArrangeContent](#tooltipareaarrangecontent) | `ArrangeContent(bounds render.Rect)` | Arranges `child` to fill the wrapper's bounds. |
| [Children](#tooltipareachildren) | `Children() []core.Widget` | Returns the single wrapped child. |
| [OnPointer](#tooltipareaonpointer) | `OnPointer(e *input.PointerEvent)` | Shows the tip after a dwell (or immediately), hides it on `Leave`. |

#### ToolTipArea.SetTimers

Wires `q` as the hover-delay driver.

**Syntax**

```go
func (ta *ToolTipArea) SetTimers(q *timers.Queue) *ToolTipArea
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `q` | `*timers.Queue` | With a queue wired, `Enter` starts a 600ms one-shot timer (`tooltipDelay`) before showing the tip, rather than showing it immediately. `nil` detaches any previously wired queue and reverts to immediate-show-on-Enter. |

**Returns** — `*ToolTipArea` for chaining.

**Example**

```go
tip.SetTimers(timerQueue)
```

**Notes** — Calling `SetTimers` again always stops whatever timer is
currently pending first, matching `TextBox.SetTimers`' "a superseded queue
can never keep affecting this widget" guarantee.

#### ToolTipArea.MeasureContent

Measures `child` with the full available space and reports its desired
size unchanged.

**Syntax**

```go
func (ta *ToolTipArea) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | Space offered by the parent layout pass. |

**Returns** — `render.Size`, `child`'s desired size.

**Notes** — Called by the layout engine; `ToolTipArea` adds no chrome or
padding.

#### ToolTipArea.ArrangeContent

Arranges `child` to fill the wrapper's own bounds exactly.

**Syntax**

```go
func (ta *ToolTipArea) ArrangeContent(bounds render.Rect)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `bounds` | `render.Rect` | The wrapper's arranged bounds. |

#### ToolTipArea.Children

Returns the single wrapped child.

**Syntax**

```go
func (ta *ToolTipArea) Children() []core.Widget
```

**Returns** — `[]core.Widget` with one element.

#### ToolTipArea.OnPointer

Implements `input.PointerHandler`: `Enter` arms the show (immediately, or
after the dwell delay if a `timers.Queue` is wired); `Leave` cancels any
pending timer and hides the tip if already showing.

**Syntax**

```go
func (ta *ToolTipArea) OnPointer(e *input.PointerEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.PointerEvent` | The pointer event (only `Enter`/`Leave` act). |

**Notes** — Never sets `e.Handled` (see the type doc comment above).

**See also** — [SetTimers](#tooltipareasettimers), [OverlayHost.ShowPopupNonModal](#overlayhostshowpopupnonmodal)

---

## TabControl

`TabControl` is a composite of a header strip and a content area showing
the selected tab's content below it.

**Normative, and the key way `TabControl` differs from `Expander`'s
"content participates in layout only while expanded" rule:** every tab's
content widget is measured, arranged, and returned from `Children()`
unconditionally — only the selected one is ever visible. A hidden tab's
content stays reachable in the tree (e.g. for a later `SetSelectedIndex` to
reveal it, or external code walking `Children()`).

Selection mirrors `ListView`/`TreeView`'s convention: `selected` is a plain
`int` (never `-1`), set programmatically (silent, `SetSelectedIndex`) or by
the user (header cell click, or Left/Right while the header strip is
focused — both funneled through the same internal path, which fires
`OnChanged` only on an actual change).

**Constructor**

```go
func NewTabControl(face *text.Face) *TabControl
```

Returns an empty `TabControl` (no tabs yet), drawing header titles with
`face` (`face` may be `nil`), styled from `theme.Active()` at construction
(rebuild to re-theme).

**Example**

```go
tabs := controls.NewTabControl(face)
tabs.AddTab("General", generalPanel)
tabs.AddTab("Advanced", advancedPanel)
tabs.OnChanged(func(i int) {
    fmt.Println("selected tab", i)
})
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [AddTab](#tabcontroladdtab) | `AddTab(title string, content core.Widget) *TabControl` | Appends a new tab. |
| [SelectedIndex](#tabcontrolselectedindex) | `SelectedIndex() int` | Returns the currently selected tab's index. |
| [SetSelectedIndex](#tabcontrolsetselectedindex) | `SetSelectedIndex(i int) *TabControl` | Sets the selection programmatically (silent). |
| [OnChanged](#tabcontrolonchanged) | `OnChanged(fn func(int)) *TabControl` | Sets the callback fired on a user-driven selection change. |
| [MeasureContent](#tabcontrolmeasurecontent) | `MeasureContent(available render.Size) render.Size` | Measures the header strip and every tab's content. |
| [ArrangeContent](#tabcontrolarrangecontent) | `ArrangeContent(bounds render.Rect)` | Arranges the strip and every tab's content. |
| [Render](#tabcontrolrender) | `Render(r render.Renderer)` | Paints the raised body panel behind the content area. |
| [Children](#tabcontrolchildren) | `Children() []core.Widget` | Returns the strip plus EVERY tab's content, unconditionally. |

#### TabControl.AddTab

Appends a new tab with the given title and content widget, re-parenting
`content` to this `TabControl`.

**Syntax**

```go
func (t *TabControl) AddTab(title string, content core.Widget) *TabControl
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `title` | `string` | The tab's header label. |
| `content` | `core.Widget` | The tab's content, re-parented to this control. |

**Returns** — `*TabControl` for chaining.

**Example**

```go
tabs.AddTab("General", generalPanel).AddTab("Advanced", advancedPanel)
```

**Notes** — `content` starts visible only if this is the tab at the
current selected index (index 0 the very first time `AddTab` is called,
since `selected` starts at 0) — every other tab's content starts hidden.

**See also** — [SetSelectedIndex](#tabcontrolsetselectedindex)

#### TabControl.SelectedIndex

Returns the currently selected tab's index.

**Syntax**

```go
func (t *TabControl) SelectedIndex() int
```

**Returns** — `int`, `0` if there are no tabs at all.

#### TabControl.SetSelectedIndex

Sets the selection programmatically.

**Syntax**

```go
func (t *TabControl) SetSelectedIndex(i int) *TabControl
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `i` | `int` | The tab index to select, clamped into `[0, len(tabs)-1]` (`0` if there are no tabs). |

**Returns** — `*TabControl` for chaining.

**Example**

```go
tabs.SetSelectedIndex(1)
```

**Notes** — Silent: never fires `OnChanged`, matching the package's uniform
contract that programmatic setters are silent.

**See also** — [OnChanged](#tabcontrolonchanged)

#### TabControl.OnChanged

Sets the callback fired with the new index whenever the user changes the
selection — by clicking a header cell, or navigating with Left/Right while
the header strip is focused.

**Syntax**

```go
func (t *TabControl) OnChanged(fn func(int)) *TabControl
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `fn` | `func(int)` | Called with the newly selected index. Replaces any previously set callback; `nil` is a valid, silent no-op. |

**Returns** — `*TabControl` for chaining.

**Example**

```go
tabs.OnChanged(func(i int) { fmt.Println("now on tab", i) })
```

**Notes** — Never fires for a programmatic `SetSelectedIndex`.

#### TabControl.MeasureContent

Measures the header strip unconditionally, then every tab's content (not
just the selected one).

**Syntax**

```go
func (t *TabControl) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | Space offered by the parent layout pass. |

**Returns** — `render.Size`: `{max(strip width, max content width), strip height + max content height}`.

**Notes** — A hidden content widget measures to `{0,0}` via the core
engine's own hidden-widget shortcut, so only the selected tab's content
ever actually contributes to the max.

#### TabControl.ArrangeContent

Arranges the strip across the full bounds width at its own measured
height, then every tab's content directly below it.

**Syntax**

```go
func (t *TabControl) ArrangeContent(bounds render.Rect)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `bounds` | `render.Rect` | The control's arranged bounds. |

**Notes** — A hidden content widget arranges to `{0,0}` via the core
engine's own hidden-widget shortcut, mirroring `MeasureContent`.

#### TabControl.Render

Paints the raised `ButtonFace` body panel behind the content area.

**Syntax**

```go
func (t *TabControl) Render(r render.Renderer)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | The target renderer. |

**Notes** — The panel's visual top edge is placed flush with every header
cell's own cell height, so it reads as the separator beneath every
non-selected cell, while the selected cell (painted after, as a child)
extends down over that band and erases it — the classic merged-selected-tab
look.

#### TabControl.Children

Returns the header strip plus every tab's content, in tab order.

**Syntax**

```go
func (t *TabControl) Children() []core.Widget
```

**Returns** — `[]core.Widget`, a fresh slice each call; mutating it does
not affect the control.

**Notes** — Unconditionally includes every tab's content regardless of
which one is currently selected/visible (contrast `Expander.Children`,
which excludes collapsed content entirely).

---

## TitleBar

`TitleBar` is a full-width, fixed-height (32px) window-chrome widget: a
title label at the left and three caption buttons (minimize/maximize/close,
close rightmost) over a `CaptionFrom`→`CaptionTo` horizontal gradient — the
classic active-caption look. `TitleBar` does not track window active/
inactive state; it always draws the active gradient.

`TitleBar` knows nothing about the actual OS window: minimize/maximize/
close are just callbacks (`OnMinimize`/`OnMaximize`/`OnClose`), and dragging
is exposed only as `DragRegion`, a pure geometry query. It powers custom
window chrome for undecorated windows — pair it with `app.Config.Undecorated`
(see `app`'s own package doc): a decorated OS window already draws its own
title bar and caption buttons, so `TitleBar` is meant to be shown instead of
that, not alongside it. The host application's frame callback is what
actually calls glfw's iconify/maximize-restore/close and drives the
window-move loop from `DragRegion` — `app.Ctx.Minimize`/`ToggleMaximize`/
`Close`/`BeginDrag` are the matching hooks — keeping `TitleBar` itself
windowing-library agnostic.

**Constructor**

```go
func NewTitleBar(face *text.Face, title string) *TitleBar
```

Returns a `TitleBar` showing `title` in `face` (`face` may be `nil`), with
its three caption buttons already wired to fire `OnMinimize`/`OnMaximize`/
`OnClose` whenever those are set (a click before they're set is a silent
no-op — the button still shows its hover/pressed feedback either way).

**Example**

```go
titleBar := controls.NewTitleBar(face, "fluo gallery").
    OnMinimize(ctx.Minimize).
    OnMaximize(ctx.ToggleMaximize).
    OnClose(ctx.Close)
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [SetTitle](#titlebarsettitle) | `SetTitle(s string) *TitleBar` | Sets the displayed title text. |
| [OnMinimize](#titlebaronminimize) | `OnMinimize(fn func()) *TitleBar` | Sets the minimize-button callback. |
| [OnMaximize](#titlebaronmaximize) | `OnMaximize(fn func()) *TitleBar` | Sets the maximize-button callback. |
| [OnClose](#titlebaronclose) | `OnClose(fn func()) *TitleBar` | Sets the close-button callback. |
| [MeasureContent](#titlebarmeasurecontent) | `MeasureContent(available render.Size) render.Size` | Stretches to the available width at the fixed bar height. |
| [ArrangeContent](#titlebararrangecontent) | `ArrangeContent(bounds render.Rect)` | Arranges the caption buttons and title. |
| [Render](#titlebarrender) | `Render(r render.Renderer)` | Paints the gradient caption background. |
| [Children](#titlebarchildren) | `Children() []core.Widget` | Returns the title label and three caption buttons. |
| [DragRegion](#titlebardragregion) | `DragRegion(p render.Point) bool` | Reports whether a point is in the draggable area. |

#### TitleBar.SetTitle

Sets the displayed title text.

**Syntax**

```go
func (t *TitleBar) SetTitle(s string) *TitleBar
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `s` | `string` | The new title text. |

**Returns** — `*TitleBar` for chaining.

**Example**

```go
titleBar.SetTitle("Untitled — fluo gallery")
```

#### TitleBar.OnMinimize

Sets the callback fired when the minimize caption button is clicked.

**Syntax**

```go
func (t *TitleBar) OnMinimize(fn func()) *TitleBar
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `fn` | `func()` | Called on click. Replaces any previously set callback; `nil` is a valid, silent no-op. |

**Returns** — `*TitleBar` for chaining.

**Example**

```go
titleBar.OnMinimize(ctx.Minimize)
```

**See also** — [OnMaximize](#titlebaronmaximize), [OnClose](#titlebaronclose)

#### TitleBar.OnMaximize

Sets the callback fired when the maximize caption button is clicked.

**Syntax**

```go
func (t *TitleBar) OnMaximize(fn func()) *TitleBar
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `fn` | `func()` | Called on click. Replaces any previously set callback; `nil` is a valid, silent no-op. |

**Returns** — `*TitleBar` for chaining.

**Example**

```go
titleBar.OnMaximize(ctx.ToggleMaximize)
```

#### TitleBar.OnClose

Sets the callback fired when the close caption button is clicked.

**Syntax**

```go
func (t *TitleBar) OnClose(fn func()) *TitleBar
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `fn` | `func()` | Called on click. Replaces any previously set callback; `nil` is a valid, silent no-op. |

**Returns** — `*TitleBar` for chaining.

**Example**

```go
titleBar.OnClose(ctx.Close)
```

#### TitleBar.MeasureContent

Stretches to the available width at the fixed bar height, regardless of
available height.

**Syntax**

```go
func (t *TitleBar) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | Space offered by the parent layout pass. |

**Returns** — `render.Size`, `{available.W, 32}`.

**Notes** — The three caption buttons each measure to their own fixed
size; the title measures within whatever width remains after them.

#### TitleBar.ArrangeContent

Arranges the three caption buttons right-aligned at the bar's full height
(close rightmost, then maximize, then minimize, abutting with no gap) and
the title at the bar's left edge.

**Syntax**

```go
func (t *TitleBar) ArrangeContent(bounds render.Rect)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `bounds` | `render.Rect` | The bar's arranged bounds. |

#### TitleBar.Render

Paints the bar's own bounds with the classic active-caption horizontal
gradient (`CaptionFrom` → `CaptionTo`).

**Syntax**

```go
func (t *TitleBar) Render(r render.Renderer)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | The target renderer. |

**Notes** — The title and caption buttons render separately, as children.

#### TitleBar.Children

Returns the title label and the three caption buttons, in that order.

**Syntax**

```go
func (t *TitleBar) Children() []core.Widget
```

**Returns** — `[]core.Widget` with 4 elements.

#### TitleBar.DragRegion

Reports whether point `p` is in the draggable area: within the bar's own
bounds and not over any of the three caption buttons.

**Syntax**

```go
func (t *TitleBar) DragRegion(p render.Point) bool
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `p` | `render.Point` | Logical, absolute window coordinates (the same space `core.BoundsOf`/`ArrangeWidget` use). |

**Returns** — `bool`, `true` if `p` is draggable.

**Example**

```go
if titleBar.DragRegion(pressPos) {
    ctx.BeginDrag()
}
```

**Notes** — The host application (`app`'s window driver) uses this to
decide whether a press should start moving the window — see `app.Ctx.BeginDrag`
and `app.Config.Undecorated`.

**See also** — [OnMinimize](#titlebaronminimize), [OnMaximize](#titlebaronmaximize), [OnClose](#titlebaronclose)

---

## AcrylicSurface

`AcrylicSurface` is a single-child decorator that draws a backdrop-blurred,
tinted "acrylic"/mica material behind its child, then renders the child on
top. It is Border-like: `SetChild`, `SetPadding`, and `SetRadius` all
behave the same way (measure/arrange delegate to the child, inset by
padding), except the background is painted via
`render.Renderer.DrawBackdropBlur` instead of a flat fill.

**Constructor**

```go
func NewAcrylicSurface() *AcrylicSurface
```

Returns an empty `AcrylicSurface` with no child, no padding, and its tint
from `theme.Active().Color.AcrylicTint` at construction time (rebuild to
re-theme, matching every other themed control).

**Example**

```go
panel := controls.NewAcrylicSurface().
    SetRadius(8).
    SetPadding(render.Uniform(16)).
    SetChild(content)
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [SetChild](#acrylicsurfacesetchild) | `SetChild(w core.Widget) *AcrylicSurface` | Sets the single child. |
| [SetTint](#acrylicsurfacesettint) | `SetTint(c render.Color) *AcrylicSurface` | Overrides the composited tint. |
| [SetRadius](#acrylicsurfacesetradius) | `SetRadius(r float32) *AcrylicSurface` | Sets the corner radius. |
| [SetPadding](#acrylicsurfacesetpadding) | `SetPadding(t render.Thickness) *AcrylicSurface` | Sets the inset between edge and child. |
| [MeasureContent](#acrylicsurfacemeasurecontent) | `MeasureContent(available render.Size) render.Size` | Measures the child within the padded space. |
| [ArrangeContent](#acrylicsurfacearrangecontent) | `ArrangeContent(bounds render.Rect)` | Arranges the child within the padded space. |
| [Render](#acrylicsurfacerender) | `Render(r render.Renderer)` | Draws the backdrop-blur background. |
| [Children](#acrylicsurfacechildren) | `Children() []core.Widget` | Returns the single child, or `nil`. |

#### AcrylicSurface.SetChild

Sets (replacing any existing) the single child.

**Syntax**

```go
func (a *AcrylicSurface) SetChild(w core.Widget) *AcrylicSurface
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `w` | `core.Widget` | The child, re-parented to this `AcrylicSurface`. |

**Returns** — `*AcrylicSurface` for chaining.

**Example**

```go
panel.SetChild(content)
```

**Notes** — Any previously set child is detached first, so its future
invalidations stop climbing into this `AcrylicSurface`. Invalidates
measure.

#### AcrylicSurface.SetTint

Overrides the tint composited over the blurred backdrop.

**Syntax**

```go
func (a *AcrylicSurface) SetTint(c render.Color) *AcrylicSurface
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `c` | `render.Color` | The tint color. Default: `theme.Active().Color.AcrylicTint` as of construction. |

**Returns** — `*AcrylicSurface` for chaining.

**Example**

```go
panel.SetTint(render.RGBA(20, 20, 30, 160))
```

**Notes** — Purely visual: no invalidation needed since the host redraws
every frame.

#### AcrylicSurface.SetRadius

Sets the corner radius used when drawing the acrylic surface.

**Syntax**

```go
func (a *AcrylicSurface) SetRadius(r float32) *AcrylicSurface
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `float32` | Corner radius, in px. |

**Returns** — `*AcrylicSurface` for chaining.

**Example**

```go
panel.SetRadius(8)
```

**Notes** — Purely visual: no invalidation.

#### AcrylicSurface.SetPadding

Sets the space between the acrylic surface's edge and the child.

**Syntax**

```go
func (a *AcrylicSurface) SetPadding(t render.Thickness) *AcrylicSurface
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `t` | `render.Thickness` | The inset on each side. |

**Returns** — `*AcrylicSurface` for chaining.

**Example**

```go
panel.SetPadding(render.Uniform(16))
```

**Notes** — Layout-relevant: invalidates measure.

#### AcrylicSurface.MeasureContent

Measures the child (if any) with the available space reduced by padding,
then adds the padding back to the child's desired size.

**Syntax**

```go
func (a *AcrylicSurface) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | Space offered by the parent layout pass. |

**Returns** — `render.Size`. With no child, the desired size is the
padding alone.

#### AcrylicSurface.ArrangeContent

Arranges the child (if any) within `bounds` inset by padding.

**Syntax**

```go
func (a *AcrylicSurface) ArrangeContent(bounds render.Rect)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `bounds` | `render.Rect` | The surface's arranged bounds. |

#### AcrylicSurface.Render

Draws the backdrop-blur acrylic background.

**Syntax**

```go
func (a *AcrylicSurface) Render(r render.Renderer)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | The target renderer. |

**Notes** — `core.RenderWidget` calls this BEFORE recursing into children,
so the child is always painted on top of the (already blurred+tinted)
surface. `render.Renderer.DrawBackdropBlur` snapshots whatever has already
been drawn beneath the surface's bounds, blurs it (a separable Gaussian
blur via two scratch framebuffers, in the `render/gl` implementation), and
composites the tinted result back — real cost per call, not a cheap flat
fill. An implementation that cannot obtain a true mid-frame snapshot (or
whose blur shader fails to compile/link, or whose scratch framebuffer comes
back incomplete) degrades to a flat, tinted, translucent rounded fill
instead — equivalent to `FillRoundedRect(bounds, radius, tint)` — a
documented degrade, not a bug, though not the frosted-glass look the type
exists for.

**AcrylicSurface is retained but currently unused in the classic theme:**
`cmd/fluo-gallery` dropped the acrylic backdrop-blur look from its content
pane in the v0.2 classic restyle (it now uses a plain `Border` filled with
`ButtonFace`) — the control itself still lives in `controls`, it simply
has no live consumer in the reference gallery today.

#### AcrylicSurface.Children

Returns the single child in a slice, or `nil` if there is none.

**Syntax**

```go
func (a *AcrylicSurface) Children() []core.Widget
```

**Returns** — `[]core.Widget`, a copy; mutating it does not affect the
panel.
