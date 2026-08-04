package controls

import (
	"math"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
	"github.com/0xdreadnaught/fluo/theme"
)

// treeIndentStep is the horizontal indent added per depth level (16px, per
// the brief's "indent 16/depth" normative rule): a depth-1 row's content
// starts 16px right of depth-0's, depth-2 another 16px right of that, etc.
//
// treeChevronW is the width of the chevron hit zone reserved at the START of
// each row's indent (i.e. the zone spans [indent, indent+treeChevronW) in
// row-local x, matching the brief's "chevron zone = indent..indent+16
// x-range" normative rule) — reserved even for a leaf row (no children, so
// no chevron glyph is drawn there), so every row's label lines up at the same
// x offset regardless of whether its own row happens to have children.
const (
	treeIndentStep float32 = 16
	treeChevronW   float32 = 16
)

// TreeNode is one node of a tree shown by a TreeView: a label, zero or more
// children, and its own expanded/collapsed state (collapsed by default —
// the zero value of a fresh TreeNode is a valid, collapsed, childless node).
// A TreeNode is shared, mutable state: the SAME *TreeNode instance passed to
// NewTreeView is read back by TreeView.Selected/OnChanged, so callers can
// hold onto node pointers returned from NewTreeNode to inspect or drive them
// later (e.g. SetExpanded from outside the tree — see its own doc comment
// for how that reaches the owning TreeView).
//
// Ownership: a TreeNode belongs to at most one TreeView at a time. owner is
// tagged onto every node reachable from a TreeView's roots — regardless of
// current expand state — during NewTreeView's construction-time walk
// (walkAllNodes), and re-tagged onto any node newly reached by a later
// flatten pass (flattenTree), which covers a node appended to an existing
// owned node's Children slice AFTER construction: it has no owner until the
// first flatten that walks into it (i.e. once its ancestor chain is
// expanded enough to make it visible). Sharing the same node across two
// TreeViews is unsupported: owner simply holds whichever TreeView tagged it
// most recently, so only that one TreeView's layout is invalidated by a
// SetExpanded call — the other silently sees nothing.
type TreeNode struct {
	Label    string
	Children []*TreeNode

	expanded bool
	owner    *TreeView
}

// NewTreeNode returns a new, collapsed TreeNode with the given label and
// (optional) children.
func NewTreeNode(label string, children ...*TreeNode) *TreeNode {
	return &TreeNode{Label: label, Children: children}
}

// Expanded reports whether n's children (if any) are currently shown.
func (n *TreeNode) Expanded() bool {
	return n.expanded
}

// SetExpanded sets n's expanded state directly and, if n is currently owned
// by a TreeView (see the type doc comment's Ownership section), immediately
// invalidates that TreeView's measure — for BOTH true and false — so the
// changed row set is picked up on the very next layout pass, exactly as if
// the change had come from a chevron click or Left/Right keyboard
// navigation. This is what makes a direct external call (bypassing
// TreeView's own toggle/expandRow/collapseOrJumpToParent) actually visible:
// earlier, nothing invalidated the owning TreeView, so a caller mutating a
// node directly (e.g. from bound model code) could silently desync from
// what was on screen until some UNRELATED invalidation happened to trigger
// a relayout. Calling it on a node not (yet) owned by any TreeView is a
// harmless no-op beyond flipping the flag.
func (n *TreeNode) SetExpanded(v bool) *TreeNode {
	n.expanded = v
	if n.owner != nil {
		n.owner.InvalidateMeasure()
	}
	return n
}

// treeRow is one flattened, currently-visible row: node at depth (0 for a
// root), with parent recorded (nil for a root) so Left's "jump to parent"
// keyboard navigation can resolve a row's parent without re-walking the
// tree from scratch.
type treeRow struct {
	node   *TreeNode
	depth  int
	parent *TreeNode
}

// flattenTree walks roots depth-first, emitting one treeRow per node in
// document order. A node's children are only walked (and so only emitted)
// when the node itself is expanded — a collapsed node's entire subtree
// contributes nothing to the flattened rows. This is the "rows = flatten of
// expanded nodes" normative rule the whole of TreeView is built around: the
// row COUNT alone (see the package's flatten-math tests) already proves
// expand/collapse is wired correctly, with no rendering involved.
//
// owner is tagged onto every visited node (see TreeNode's Ownership doc
// comment) — this is what lets a node appended to an existing owned node's
// Children slice AFTER construction (so it missed NewTreeView's own
// construction-time walkAllNodes tagging) still end up owned, the moment a
// flatten pass actually walks into it. owner may be nil (the pure
// flatten-math tests in this package call flattenTree directly with no
// TreeView at all); tagging nodes with a nil owner is harmless.
func flattenTree(roots []*TreeNode, owner *TreeView) []treeRow {
	var rows []treeRow
	var walk func(nodes []*TreeNode, depth int, parent *TreeNode)
	walk = func(nodes []*TreeNode, depth int, parent *TreeNode) {
		for _, n := range nodes {
			n.owner = owner
			rows = append(rows, treeRow{node: n, depth: depth, parent: parent})
			if n.expanded && len(n.Children) > 0 {
				walk(n.Children, depth+1, n)
			}
		}
	}
	walk(roots, 0, nil)
	return rows
}

// walkAllNodes visits every node reachable from roots, regardless of
// current expand state (unlike flattenTree, which only descends into
// expanded subtrees) — used by NewTreeView to tag EVERY node in the tree,
// including ones hidden behind a currently-collapsed ancestor, with their
// owning TreeView at construction time, so a direct SetExpanded call on any
// of them (not merely the currently-visible ones) can find its owner and
// invalidate immediately.
func walkAllNodes(roots []*TreeNode, fn func(*TreeNode)) {
	for _, n := range roots {
		fn(n)
		walkAllNodes(n.Children, fn)
	}
}

// findAncestors returns the ancestor chain (root-to-parent order, EXCLUDING
// target itself) of target within the tree rooted at roots, searching the
// FULL tree structurally (not merely the currently-flattened, visible rows —
// target may be hidden beneath a collapsed ancestor), or nil if target is
// nil or not present anywhere in the tree.
func findAncestors(roots []*TreeNode, target *TreeNode) []*TreeNode {
	if target == nil {
		return nil
	}
	var path []*TreeNode
	var walk func(nodes []*TreeNode) bool
	walk = func(nodes []*TreeNode) bool {
		for _, n := range nodes {
			if n == target {
				return true
			}
			path = append(path, n)
			if walk(n.Children) {
				return true
			}
			path = path[:len(path)-1]
		}
		return false
	}
	if walk(roots) {
		return path
	}
	return nil
}

// TreeView is a clickable, focusable, token-styled tree of TreeNodes: rows
// are the depth-first flatten of every currently-expanded node (see
// flattenTree), each drawn as an optional 'v'/'>' chevron (only when the
// node has children — text glyphs, WindowText, never rotated: '>' means
// collapsed, 'v' means expanded) followed by its label, indented 16px per
// depth level.
//
// v0 is NON-VIRTUALIZED, documented and deliberate: unlike ListView (whose
// items are an unboundedly long external source and so MUST be virtualized),
// a TreeView's content is a finite, already-in-memory tree — every currently
// visible row is measured and drawn every pass, with no pooling/windowing.
// A caller with a very large or deep tree that needs virtualization should
// wrap a TreeView in a ScrollViewer (or similar) rather than expect one built
// in; that is out of scope for this v0.
//
// Selection mirrors ListView's convention: selected is a single *TreeNode
// (nil meaning none), set programmatically (silent, SetSelected — see its
// own doc comment for the auto-expand-ancestors behavior that sets it apart
// from a plain silent setter) or by the user (label click, or Up/Down/Left/
// Right while focused — all funneled through selectUser, which fires
// OnChanged only on an actual change, matching ListView.selectUser/
// ComboBox.selectUser's "notify only on real change" convention).
type TreeView struct {
	core.Element

	face  *text.Face
	roots []*TreeNode

	// rows is the flattened row list as of the last MeasureContent pass
	// (recomputed there — see its doc comment for why measure, not arrange,
	// is the right place). Render, OnPointer, and OnKey all read this
	// cache; none of them re-flatten the tree themselves.
	rows []treeRow

	rowH float32

	selected  *TreeNode
	ta        typeAhead // type-ahead prefix state (see OnKey)
	focused   bool
	onChanged func(*TreeNode)

	colors  theme.ColorTokens
	metrics theme.MetricTokens
}

// NewTreeView returns a TreeView over roots, styled from theme.Active() at
// construction (rebuild to re-theme), drawing labels/chevrons with face
// (face may be nil, per TextBlock's own nil-face convention — see
// defaultRowHeight). Every root starts however its own TreeNode.expanded was
// left (collapsed, unless the caller already called SetExpanded(true) on it
// before/after construction — TreeView holds no copy of roots' state, just
// the *TreeNode pointers themselves).
func NewTreeView(face *text.Face, roots ...*TreeNode) *TreeView {
	th := theme.Active()
	t := &TreeView{face: face, roots: roots, colors: th.Color, metrics: th.Metric}
	t.rowH = defaultRowHeight(face, th)
	walkAllNodes(t.roots, func(n *TreeNode) { n.owner = t })
	return t
}

// Selected returns the currently selected node, or nil if none.
func (t *TreeView) Selected() *TreeNode {
	return t.selected
}

// SetSelected sets the selection programmatically and silently — never
// fires OnChanged, matching fluo's uniform contract that programmatic
// setters are silent (OnChanged reports only user-driven changes).
//
// Normative: unlike selectUser (the user-driven path, which by construction
// only ever targets an already-visible row), SetSelected accepts ANY node
// reachable from the tree's roots — including one currently hidden beneath
// a collapsed ancestor. In that case it auto-expands every ancestor on the
// path to n (via findAncestors, which walks the full tree structurally, not
// just the currently-flattened rows) so the newly selected node actually
// becomes visible on the next layout pass — mirroring how a real tree
// reveals a selection driven by external/bound data rather than a user
// click drilling down to it manually. Only fires InvalidateMeasure when an
// ancestor's expanded state actually changed (n was already visible, or n is
// nil, is a true no-op structurally, not just a silent one).
//
// No reachability check is performed: SetSelected does not verify n is
// actually still reachable from t.roots (a foreign node never part of this
// tree, or one removed from it since construction). findAncestors simply
// returns nil for such a node, so no ancestor gets expanded, and since n
// will never appear in t.rows, Render's `row.node == t.selected` comparison
// never matches — the failure mode is a soft "no row highlighted", not a
// panic or a rejected call.
func (t *TreeView) SetSelected(n *TreeNode) *TreeView {
	t.selected = n
	changed := false
	for _, anc := range findAncestors(t.roots, n) {
		if !anc.expanded {
			anc.SetExpanded(true)
			changed = true
		}
	}
	if changed {
		t.InvalidateMeasure()
	}
	return t
}

// OnChanged sets the callback fired with the newly selected node whenever
// the user changes the selection — by clicking a row's label, or navigating
// with Up/Down/Left/Right while focused — but never for a programmatic
// SetSelected (see its doc comment). Replaces any previously set callback; a
// nil fn is a valid, silent no-op.
func (t *TreeView) OnChanged(fn func(*TreeNode)) *TreeView {
	t.onChanged = fn
	return t
}

// selectUser is the user-driven selection path (label click, keyboard nav):
// always targets a node ALREADY present in t.rows (so no ancestor-expand
// concern applies, unlike SetSelected), and fires OnChanged only if the
// selection actually changed, matching ListView.selectUser/ComboBox.
// selectUser's "notify only on real change" convention. Purely visual
// otherwise (no invalidation): Render reads t.selected directly every frame,
// there is no baked-color pool to refresh, matching Button.SetAccent's
// "no invalidation needed" convention.
func (t *TreeView) selectUser(n *TreeNode) {
	changed := n != t.selected
	t.selected = n
	if changed && t.onChanged != nil {
		t.onChanged(n)
	}
}

// toggle flips node's expanded state and invalidates measure (the flattened
// row count/order changes) — a silent no-op for a leaf node (nothing to
// expand/collapse), matching the brief's "chevron click toggles ... (or
// no-op leaf)" for keyboard Right/Left.
func (t *TreeView) toggle(node *TreeNode) {
	if len(node.Children) == 0 {
		return
	}
	node.SetExpanded(!node.expanded)
	t.InvalidateMeasure()
}

// expandRow implements the keyboard Right path, per the WinUI TreeView
// convention (adopted here as the normative v0 behavior): on a collapsed
// node with children, it expands the node; on a node that already has
// children AND is expanded, it instead moves the selection down to its
// FIRST child (via selectUser — a genuine, user-driven selection change,
// so OnChanged fires normally); on a leaf, it is a true no-op. The first
// child is guaranteed to already be a real row in t.rows by the time this
// runs (row.node.expanded is true, so flattenTree already walked into it).
func (t *TreeView) expandRow(row treeRow) {
	if len(row.node.Children) == 0 {
		return
	}
	if !row.node.expanded {
		row.node.SetExpanded(true)
		t.InvalidateMeasure()
		return
	}
	t.selectUser(row.node.Children[0])
}

// collapseOrJumpToParent implements the keyboard Left path: collapses
// row's node if it has children AND is currently expanded; otherwise (a
// leaf, or a node already collapsed) selects row's parent instead, if any
// (a root row with no parent is a no-op) — the brief's "Left collapses (or
// jumps to parent when leaf/collapsed)".
func (t *TreeView) collapseOrJumpToParent(row treeRow) {
	if len(row.node.Children) > 0 && row.node.expanded {
		row.node.SetExpanded(false)
		t.InvalidateMeasure()
		return
	}
	if row.parent != nil {
		t.selectUser(row.parent)
	}
}

// selectedRowIndex returns t.selected's index within t.rows, or -1 if
// nothing is selected or the selected node isn't currently visible (hidden
// beneath a collapsed ancestor — can only happen right after a SetSelected
// whose auto-expand hasn't been applied by a layout pass yet).
func (t *TreeView) selectedRowIndex() int {
	for i, row := range t.rows {
		if row.node == t.selected {
			return i
		}
	}
	return -1
}

// MeasureContent recomputes t.rows (flattenTree(t.roots, t) — see its doc
// comment for why measure, not arrange, is the right place: a TreeView's
// content is fully known up front, unlike ListView's virtualized/unboundedly
// long content, so its desired size is simply the real total content size,
// matching TextBlock's own "measure the real thing, ignore available"
// convention rather than ListView's fixed-default-clamped-to-available one)
// and returns {max row width, rowH * row count}. A nil face measures every
// row's label to zero width (text.Face.Measure is only called when non-nil)
// and rowH itself collapses to the nil-safe padding-only value (see
// defaultRowHeight).
func (t *TreeView) MeasureContent(available render.Size) render.Size {
	t.rows = flattenTree(t.roots, t)

	var maxW float32
	for _, row := range t.rows {
		w := float32(row.depth)*treeIndentStep + treeChevronW + t.metrics.PaddingS
		if t.face != nil {
			w += t.face.Measure(row.node.Label).W
		}
		if w > maxW {
			maxW = w
		}
	}

	return render.Size{W: maxW, H: t.rowH * float32(len(t.rows))}
}

// contentBounds returns t's own bounds inset by the 2px sunken bevel (see
// theme.MetricTokens.BevelWidth) — the area rows render within so labels/
// chevrons never sit over the well's frame. Both Render and rowAt/OnPointer
// use this SAME rect, so the drawn chevron zones and their hit-test geometry
// always agree.
func (t *TreeView) contentBounds() render.Rect {
	bw := t.metrics.BevelWidth
	inner := t.Bounds().Inset(render.Thickness{Top: bw, Bottom: bw, Left: bw, Right: bw})
	if inner.W < 0 {
		inner.W = 0
	}
	if inner.H < 0 {
		inner.H = 0
	}
	return inner
}

// rowAt maps an absolute pointer position to the row index it falls over,
// using t.rows/t.rowH/t.contentBounds() (the bevel-inset content area, not
// t's raw outer bounds) as of the last layout pass. ok is false for a
// position outside the content area, or (defensively) past the last row.
func (t *TreeView) rowAt(pos render.Point) (idx int, ok bool) {
	bounds := t.contentBounds()
	if t.rowH <= 0 || !bounds.Contains(pos) {
		return 0, false
	}
	idx = int(math.Floor(float64((pos.Y - bounds.Y) / t.rowH)))
	if idx < 0 || idx >= len(t.rows) {
		return 0, false
	}
	return idx, true
}

// Render draws the sunken WindowWell frame across t's full bounds first
// (the classic tree well), then every row inside the bevel-inset content
// area (see contentBounds): the selection band (Highlight, full content
// width) first if the row's node is selected, then the chevron glyph (only
// for a row whose node has children — '>' collapsed, 'v' expanded,
// WindowText) at [indent, indent+treeChevronW), then the label
// (HighlightText when selected, else WindowText) starting at
// indent+treeChevronW+PaddingS. Rows are skipped (well still drawn) with a
// nil face, matching TextBlock's own nil-face-renders-nothing convention.
//
// Row drawing is clipped to that same content area (see ClipRect): a tree
// arranged shorter than its rows need crops the overflow at its own edge
// instead of painting it over the sibling below.
func (t *TreeView) Render(r render.Renderer) {
	drawSunken(r, t.Bounds(), t.colors.WindowWell, t.colors)

	if t.face == nil {
		return
	}

	rect, clip := t.ClipRect()
	if clip {
		r.PushClip(rect)
		defer r.PopClip()
	}

	bounds := t.contentBounds()
	gap := t.metrics.PaddingS

	for i, row := range t.rows {
		rowY := bounds.Y + float32(i)*t.rowH
		indent := float32(row.depth) * treeIndentStep

		if row.node == t.selected {
			r.FillRect(render.Rect{X: bounds.X, Y: rowY, W: bounds.W, H: t.rowH}, t.colors.Highlight)
		}

		if len(row.node.Children) > 0 {
			glyph := ">"
			if row.node.expanded {
				glyph = "v"
			}
			gs := t.face.Measure(glyph)
			gy := rowY + (t.rowH-gs.H)/2
			t.face.Draw(r, render.Point{X: bounds.X + indent, Y: gy}, glyph, t.colors.WindowText)
		}

		labelColor := t.colors.WindowText
		if row.node == t.selected {
			labelColor = t.colors.HighlightText
		}
		ls := t.face.Measure(row.node.Label)
		ly := rowY + (t.rowH-ls.H)/2
		lx := bounds.X + indent + treeChevronW + gap
		t.face.Draw(r, render.Point{X: lx, Y: ly}, row.node.Label, labelColor)
	}
}

// ClipRect implements core.ClipProvider, bounding the rows Render draws.
// Without it a TreeView arranged shorter (or narrower) than its rows need
// painted the overflow straight past its own edge, over whatever sibling
// sits below — visible there, but dead to the pointer, since rowAt rejects
// anything outside the content area.
//
// The rect is the region rows actually occupy: they start at the bevel-inset
// content origin (see contentBounds) but are laid out at the size
// MeasureContent asked for, which does NOT account for that inset — so the
// row region is t's own arranged size taken from the content origin, not
// contentBounds itself. Clipping to contentBounds would crop the last row
// and the widest label by the bevel width even when the tree was given
// exactly the size it asked for.
//
// TreeView draws its rows directly rather than through child widgets
// RenderWidget would clip on its behalf, so — exactly like TextBox — Render
// pushes this same rect itself; ClipRect is the single source of truth both
// paths share. The well's sunken frame is drawn before the push and the
// focus ring in RenderOverlay after the pop, so neither is ever cropped.
//
// The rect is grown out to whole pixels: row heights and label widths are
// font-metric floats, so its edges land mid-pixel, and a clip is applied by
// truncating to a whole scissor rect — an exact rect would shave the
// partially covered edge pixel off content that is entirely the tree's own.
// Rounding out costs at most a pixel of slop and still bounds the overflow.
func (t *TreeView) ClipRect() (render.Rect, bool) {
	b := t.Bounds()
	inner := t.contentBounds()

	x := float32(math.Floor(float64(inner.X)))
	y := float32(math.Floor(float64(inner.Y)))
	right := float32(math.Ceil(float64(inner.X + b.W)))
	bottom := float32(math.Ceil(float64(inner.Y + b.H)))
	return render.Rect{X: x, Y: y, W: right - x, H: bottom - y}, true
}

// RenderOverlay draws the focus ring while focused, per the global focus
// constraint shared by every focusable control in this package.
func (t *TreeView) RenderOverlay(r render.Renderer) {
	if !t.focused {
		return
	}
	drawFocusRing(r, t.Bounds(), t.colors)
}

// AcceptsFocus implements input.Focusable: a TreeView always accepts focus
// (v0 has no SetEnabled/disabled concept, matching ListView).
func (t *TreeView) AcceptsFocus() bool {
	return true
}

// OnFocusChanged implements input.FocusHandler, tracking focus for the
// focus-ring overlay and Up/Down/Left/Right keyboard navigation.
func (t *TreeView) OnFocusChanged(focused bool) {
	t.focused = focused
}

// OnPointer implements input.PointerHandler: a Press landing on a real row
// toggles that row's node if it falls within the row's chevron zone
// ([indent, indent+treeChevronW) in row-local x — see treeChevronW's doc
// comment), else selects that row's node as a user-driven change
// (selectUser). Marked Handled whenever the press lands on a real row at
// all (whether the resulting toggle/select was a "real" state change or
// not); a press outside any row (or with rowH <= 0, e.g. a nil face) is left
// unhandled.
func (t *TreeView) OnPointer(e *input.PointerEvent) {
	if e.Action != input.Press {
		return
	}
	idx, ok := t.rowAt(e.Pos)
	if !ok {
		return
	}
	row := t.rows[idx]
	bounds := t.contentBounds()
	indent := float32(row.depth) * treeIndentStep
	chevX0 := bounds.X + indent
	chevX1 := chevX0 + treeChevronW

	if e.Pos.X >= chevX0 && e.Pos.X < chevX1 {
		t.toggle(row.node)
	} else {
		t.selectUser(row.node)
	}
	e.Handled = true
}

// OnKey implements input.KeyHandler: Up/Down move the selection by one row
// over the flattened rows (clamped via clampRowIndex — shared with
// ListView, so Up/Down from no selection both land on row 0, mirroring
// ListView.OnKey exactly, and so ALWAYS handled once there is at least one
// row); Right expands the selected row, or descends to its first child if
// already expanded (expandRow); Left collapses it, or jumps to its parent
// (collapseOrJumpToParent) — see both methods' own doc comments. Unlike
// Up/Down, Right/Left are only meaningful relative to a CURRENT selection,
// so they are marked Handled only when one exists (idx >= 0); with no
// selection, they fall through unhandled rather than silently doing
// nothing while claiming to have acted. Ignored entirely for anything but
// Action==Press, or when there are no rows at all.
func (t *TreeView) OnKey(e *input.KeyEvent) {
	if e.Action != input.Press {
		return
	}
	n := len(t.rows)
	if n == 0 {
		return
	}

	idx := t.selectedRowIndex()
	switch e.Key {
	case input.KeyUp:
		t.selectUser(t.rows[clampRowIndex(idx-1, n)].node)
		e.Handled = true
	case input.KeyDown:
		t.selectUser(t.rows[clampRowIndex(idx+1, n)].node)
		e.Handled = true
	case input.KeyRight:
		if idx >= 0 {
			t.expandRow(t.rows[idx])
			e.Handled = true
		}
	case input.KeyLeft:
		if idx >= 0 {
			t.collapseOrJumpToParent(t.rows[idx])
			e.Handled = true
		}
	default:
		// Type-ahead over the currently-visible (flattened) rows: a printable
		// key jumps selection to the next row whose label matches the typed
		// prefix. See typeAhead.feed.
		if e.Rune != 0 && e.Mods&(input.ModCtrl|input.ModAlt) == 0 {
			if next, ok := t.ta.feed(e.Time, e.Rune, n, idx, func(i int) string { return t.rows[i].node.Label }); ok {
				t.selectUser(t.rows[next].node)
				e.Handled = true
			}
		}
	}
}
