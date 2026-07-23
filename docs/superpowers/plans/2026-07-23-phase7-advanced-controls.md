# fluo Phase 7: Advanced Controls Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The advanced control set: virtualizing ListView, TreeView, TabControl, Expander, MenuBar/MenuItem/ContextMenu, modal Dialog, and DataGrid — completing the WPF-style control library.

**Architecture:** Granular list events land as a NEW `List.OnChange(Change)` channel (the coarse `OnChanged(func())` signature is published API — never changed). ListView and DataGrid share one internal uniform-row-height `virtualizer` (viewport math + row pool + scroll state). Menus/dialogs build on OverlayHost (menus = modal popups; submenus respect the release-before-ShowPopup convention; Dialog = modal popup whose widget is a full-host scrim, which neutralizes light-dismiss by construction). All selection visuals use SelectionBackground + a NEW SelectionForeground token.

**Tech Stack:** Pinned deps. Headless tests + one golden per control family.

## Global Constraints

- All prior constraints bind (go.mod PINNED; WSL-only Go; keyed literals; vet+gofmt; doc comments; per-task commits + trailer `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`; existing goldens never touched; tokens only; controls idiom: Element embed, silent setters, OnChanged=user-only, focus ring via drawFocusRing, disabled semantics, ClickBehavior where button-like).
- New goldens: light theme + defer SetActive(nil).
- Declared v0 cuts: uniform row height only (ListView/DataGrid); TreeView non-virtualized (flattens expanded nodes); no DataGrid editing/sorting/column-resize; no menu keyboard navigation beyond Esc-close (arrow-nav later); single-selection only everywhere; no animations.

## File Structure

```
theme/theme.go|fluent.go   + SelectionForeground, ScrimBackground tokens
bind/list.go               + Change{Kind,Index} + OnChange(func(Change)) (cancel func())
controls/virtualizer.go    internal uniform-row virtualizer (shared ListView/DataGrid)
controls/listview.go       ListView
controls/treeview.go       TreeView (+TreeNode)
controls/tabcontrol.go     TabControl
controls/expander.go       Expander
controls/menu.go           MenuBar, MenuItem, ContextMenu helpers
controls/dialog.go         Dialog
controls/datagrid.go       DataGrid (+Column)
cmd/fluo-gallery/main.go   advanced page(s)
render/gl/renderer_test.go + goldens: listview.png, treeview.png, tabs_expander.png, menu_open.png, dialog.png, datagrid.png
```

---

### Task 1: foundations — granular list events + tokens

**Files:** `bind/list.go`(+tests), `theme/theme.go`, `theme/fluent.go`(+tests)

**Produces (exact):**
```go
// bind
type ChangeKind uint8
const (ChangeAdd ChangeKind = iota; ChangeRemove; ChangeReplace; ChangeReset)
type Change struct{ Kind ChangeKind; Index int } // Index -1 for ChangeReset
func (l *List[T]) OnChange(f func(Change)) (cancel func()) // granular; fires alongside OnChanged
// Mutation mapping (normative): Add→one ChangeAdd per appended item (index of each);
// Insert→ChangeAdd{i}; RemoveAt→ChangeRemove{i}; Set→ChangeReplace{i}; Replace→ChangeReset{-1}.
// theme ColorTokens additions:
SelectionForeground render.Color // dark: RGB(255,255,255); light: RGB(26,26,26)
ScrimBackground     render.Color // dark: RGBA(0,0,0,120); light: RGBA(0,0,0,90)
```
- [ ] TDD: mapping per mutation (counts + kinds + indices; Add of 3 items → 3 ChangeAdd with correct indices); both channels fire; cancel independent per channel; token presence + light/dark differ. Coarse-channel tests unmodified. Commit `feat(bind,theme): granular list events; selection/scrim tokens`

### Task 2: virtualizer + ListView core

**Files:** `controls/virtualizer.go`, `controls/listview.go`(+tests)

**Produces:**
```go
// internal: type virtualizer struct — owns: rowH float32, count func() int,
// offsetY (clamped in arrange like ScrollViewer), viewport height; computes
// visible range [first,last]; exposes total height; wheel/thumb logic copied
// from ScrollViewer's conventions (gutter 12, thumb min 24, wheel 48).
func NewListView(face *text.Face, items *bind.List[string]) *ListView
// v0 items are strings rendered as TextBlock rows (custom row factories arrive later).
func (l *ListView) RowHeight() float32          // = face.LineHeight()+2*PaddingS by default
func (l *ListView) SetRowHeight(h float32) *ListView
```
Normative: subscribes to items.OnChange (granular) — any change invalidates (v0 uses it only to invalidate measure+arrange; per-row incrementalism later; document). Rows realized ONLY for the visible range each arrange pass into a reused pool of TextBlocks (pool size = visible count; pool entries re-texted, not reallocated). ClipProvider; RenderOverlay thumb; wheel scrolls; desired {160, 240} default. Detach convention: ListView's Children() returns the CURRENT pool (for hit-testing/render). cancel of the list subscription on... controls have no dispose — normative: ListView exposes `Dispose()` releasing the list subscription (FIRST disposable control — document; gallery calls it in the rebuild cancel path). 
- [ ] TDD: visible-range math (offset 0 → rows 0..k; scrolled → correct first index; short list → all); pool reuse (same widget pointers re-texted after scroll); total-height/thumb; wheel; list Add → reflects. Commit `feat(controls): uniform-row virtualizer and ListView core`

### Task 3: ListView selection + keyboard + golden

**Files:** `controls/listview.go`(+tests), golden `listview.png`

Selection: single index (−1 none); click row selects (user → OnChanged(int)); SetSelectedIndex silent+clamped; selected row = SelectionBackground fill + SelectionForeground text; keyboard when focused: Up/Down move (clamped), Home/End, selection scrolls into view (offset adjusted). Focusable + focus ring. `bind.ListSelected(p *core.Property[int], lv *controls.ListView)` added to bind with the standard mechanic.
- [ ] TDD per behavior + binder both-directions. Golden (light, 200x160): 12 items, index 3 selected, scrolled so rows 1..8 visible, thumb visible. Commit `feat(controls): ListView selection, keyboard, binding`

### Task 4: TreeView + Expander

**Files:** `controls/treeview.go`, `controls/expander.go`(+tests), golden `treeview.png` (tree + an expanded Expander side by side... separate goldens complicate; single golden `tree_expander.png` 260x180: tree left, expander right)

**Produces:**
```go
type TreeNode struct{ /* Label string; Children []*TreeNode; expanded bool */ }
func NewTreeNode(label string, children ...*TreeNode) *TreeNode
func (n *TreeNode) SetExpanded(v bool) *TreeNode; func (n *TreeNode) Expanded() bool
func NewTreeView(face *text.Face, roots ...*TreeNode) *TreeView
// selection like ListView (selected *TreeNode; OnChanged(func(*TreeNode)); SetSelected silent)
// rows = flatten(visible nodes); indent 16/depth; chevron = '>' rotated? NO rotation: '>' when
// collapsed, 'v' when expanded (text glyphs, TextSecondary); click chevron toggles, click label
// selects; keyboard: Up/Down rows, Right expands, Left collapses (or moves to parent if leaf/collapsed).
func NewExpander(face *text.Face, header string) *Expander
func (e *Expander) SetContent(w core.Widget) *Expander; SetExpanded/Expanded/OnChanged(func(bool))
// header = full-width button row ('v'/'>' + title, ControlFill fills, ClickBehavior);
// content shown below when expanded (participates in layout only when expanded).
```
- [ ] TDD: flatten math (expand/collapse changes row count); chevron vs label click zones; keyboard; Expander toggle + content layout participation; selection. Golden per above. Commit `feat(controls): TreeView, Expander`

### Task 5: TabControl

**Files:** `controls/tabcontrol.go`(+tests), golden `tabs.png`

`NewTabControl(face)` + `AddTab(title string, content core.Widget) *TabControl`; strip of header buttons (selected: Accent underline 2px + TextPrimary; others TextSecondary, hover ControlFillHover), content area shows selected tab's content (others hidden via SetVisible(false) — they stay in the tree); SelectedIndex()/SetSelectedIndex silent/OnChanged user; strip keyboard Left/Right when focused; focus ring on the strip region... normative: the STRIP is the focusable unit. Golden (light, 240x120): 3 tabs, second selected, simple content.
- [ ] Commit `feat(controls): TabControl`

### Task 6: menus

**Files:** `controls/menu.go`(+tests), golden `menu_open.png`

**Produces:**
```go
func NewMenuBar(face *text.Face) *MenuBar
func (m *MenuBar) AddMenu(title string) *MenuItems       // top-level entry
type MenuItems struct{ /* builder for a popup menu */ }
func (mi *MenuItems) Add(label string, onClick func()) *MenuItems
func (mi *MenuItems) AddSeparator() *MenuItems
func (mi *MenuItems) AddSub(label string) *MenuItems      // nested submenu
func ShowContextMenu(owner core.Widget, at render.Point, face *text.Face, build func(*MenuItems))
```
Normative: menu popups = MODAL (ShowPopup) Card+shadow like combo; item rows hover ControlFillHover, click fires + closes ALL menus (CloseAll... add `OverlayHost.CloseAllPopups()` if absent); separators = 1px ControlStroke inset rows; submenu opens on hover (immediate v0) anchored right of the item — NOTE the h→w→h capture caveat: submenus open from WITHIN a forwarded event — per the documented convention, no capture is held by rows (rows don't capture) so ShowPopup's idempotent host re-capture is safe; nested popup = the topmost, its rows receive events (forwarding targets topmost). ContextMenu: right-click (ButtonRight press) on the OWNER's subtree — v0: ShowContextMenu is invoked BY the app (e.g. a widget's own OnPointer sees ButtonRight) — no global right-click infrastructure; document. Esc closes topmost (menu popup rows aren't focusable; MenuBar keeps focus — MenuBar.OnKey Esc → CloseAll).
- [ ] TDD: bar click opens; item click fires+closes all; separator inert; submenu hover-opens (forwarded-move driven) + its item click closes all; Esc; light-dismiss resets bar state. Golden (light, 260x200): open menu with 3 items + separator + submenu expanded. Commit `feat(controls): MenuBar, menus, ContextMenu`

### Task 7: Dialog

**Files:** `controls/dialog.go`(+tests), golden `dialog.png`

```go
func ShowDialog(host *OverlayHost, face *text.Face, d DialogSpec)
type DialogSpec struct {
	Title, Body        string
	Primary, Secondary string      // button labels; empty = omitted
	OnResult           func(DialogResult)
}
type DialogResult uint8
const (DialogPrimary DialogResult = iota; DialogSecondary; DialogDismissed)
```
Normative: one modal popup = full-host scrim Border (ScrimBackground) containing a centered Card (CornerRadius, ShadowBlur shadow, PaddingL): Title (SubtitleSize TextPrimary), Body (BodySize TextSecondary), button row right-aligned (Primary = accent Button, Secondary = default Button). Scrim covers the whole host ⇒ every press is inside the popup ⇒ light-dismiss can't fire (by construction — document). Esc → DialogDismissed. Buttons close + fire result exactly once. Scrim presses (outside the card) do nothing (v0; document).
- [ ] TDD: result routing per button; Esc; single-fire; scrim click no-op; popup count lifecycle. Golden (light, 280x180): dialog with title/body/two buttons over dimmed content. Commit `feat(controls): modal Dialog`

### Task 8: DataGrid

**Files:** `controls/datagrid.go`(+tests), golden `datagrid.png`

```go
type Column struct{ Title string; Width theme-free... exact: Width Track (reuse controls.Track Px/Star!); Value func(row int) string }
func NewDataGrid(face *text.Face) *DataGrid
func (g *DataGrid) SetColumns(cols ...Column) *DataGrid
func (g *DataGrid) SetRowCount(n int) *DataGrid          // v0 data model = count + Column.Value funcs
func (g *DataGrid) RowCount() int
// selection like ListView (SelectedIndex/SetSelectedIndex silent/OnChanged); reuses the virtualizer
// (header row fixed on top; body virtualized below); column widths resolved like Grid's
// resolveTracks (Px/Star; AutoTrack NOT supported v0 — panic with clear message);
// header: LayerBackground fill, TextSecondary, 1px ControlStroke bottom; cells: TextBlock pool
// (pool = visibleRows × cols); row hover ControlFillHover; selected row SelectionBackground/Foreground;
// grid lines: 1px ControlStroke horizontal per row (v0: no vertical lines; document).
```
- [ ] TDD: column resolution (Px/Star, Auto panics); virtualized cell pool reuse; selection + keyboard (Up/Down); values rendered from Value funcs; RowCount changes. Golden (light, 300x180): 3 columns (Px 80, Star, Px 60), 20 rows, row 2 selected, scrolled to top, thumb visible. Commit `feat(controls): DataGrid`

### Task 9: gallery + docs

**Files:** `cmd/fluo-gallery/main.go`, `README.md`, `ROADMAP.md`

- Nav gains real pages! The nav TextBlocks ("Layout"/"Panels"/"Text") become a page switcher (v0: replace nav with a ListView of page names bound to which content builds — dogfooding): pages "Controls" (existing rows), "Advanced" (new): MenuBar on top (File→New/Open/sep/Exit(no-op), Edit→submenu demo), TabControl with 3 tabs — tab1: ListView (30 items) + selected-index TextBlock via bind.ListSelected; tab2: TreeView (nested sample) + Expander; tab3: DataGrid (3 cols, 50 rows) + a "Show dialog" Button firing ShowDialog (result mirrored in a TextBlock).
- ListView Dispose discipline on rebuild (add to the cancels path).
- Live verify: launch, winshot `fluo-gallery-p7.png` (Advanced page visible as default startup page for the screenshot — make Advanced the initial page), READ carefully. Kill+confirm.
- Docs: ROADMAP tick ALL Phase 7 boxes; README advanced-controls paragraph.
- [ ] Commit `feat(gallery): advanced controls page; complete Phase 7`

---

## Self-review notes
- ROADMAP coverage: ItemsControl/ListView+virtualization→2-3, TreeView/TabControl/Expander→4-5, Menu family→6, Dialog+modal→7, DataGrid→8, gallery→9, granular events+tokens→1.
- Phase-6 contracts honored: OnChange is a NEW channel; Items untouched; ListView consumes List directly (not Items).
- Capture conventions from Phase 5 explicitly cited in Task 6 (submenus).
- Track reuse in DataGrid keeps column sizing consistent with Grid.
