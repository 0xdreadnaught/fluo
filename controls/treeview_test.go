package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/theme"
)

// layoutTreeView measures then arranges tv at the given bounds, mirroring
// listview_test.go's layoutListView.
func layoutTreeView(tv *TreeView, x, y, w, h float32) {
	core.MeasureWidget(tv, render.Size{W: w, H: h})
	core.ArrangeWidget(tv, render.Rect{X: x, Y: y, W: w, H: h})
}

// --- Flatten math: row counts / depth / order ---

func TestFlattenTreeAllCollapsedShowsOnlyRoots(t *testing.T) {
	roots := []*TreeNode{
		NewTreeNode("a", NewTreeNode("a1"), NewTreeNode("a2")),
		NewTreeNode("b"),
	}
	rows := flattenTree(roots, nil)
	if got, want := len(rows), 2; got != want {
		t.Fatalf("len(rows) = %d, want %d (both roots collapsed by default)", got, want)
	}
	if rows[0].node.Label != "a" || rows[1].node.Label != "b" {
		t.Fatalf("rows = %+v, want a,b in order", rows)
	}
	if rows[0].depth != 0 || rows[1].depth != 0 {
		t.Fatalf("root depths = %d,%d, want 0,0", rows[0].depth, rows[1].depth)
	}
}

func TestFlattenTreeExpandingRootAddsChildRows(t *testing.T) {
	a1 := NewTreeNode("a1")
	a2 := NewTreeNode("a2")
	a := NewTreeNode("a", a1, a2)
	b := NewTreeNode("b")
	roots := []*TreeNode{a, b}

	if got := len(flattenTree(roots, nil)); got != 2 {
		t.Fatalf("collapsed len = %d, want 2", got)
	}

	a.SetExpanded(true)
	rows := flattenTree(roots, nil)
	if got, want := len(rows), 4; got != want {
		t.Fatalf("expanded len = %d, want %d (a, a1, a2, b)", got, want)
	}
	wantLabels := []string{"a", "a1", "a2", "b"}
	for i, w := range wantLabels {
		if rows[i].node.Label != w {
			t.Fatalf("rows[%d].Label = %q, want %q", i, rows[i].node.Label, w)
		}
	}
	// a1/a2 are depth 1, children of a.
	if rows[1].depth != 1 || rows[2].depth != 1 {
		t.Fatalf("child depths = %d,%d, want 1,1", rows[1].depth, rows[2].depth)
	}
	if rows[1].parent != a || rows[2].parent != a {
		t.Fatalf("child parents != a")
	}
	if rows[0].parent != nil || rows[3].parent != nil {
		t.Fatalf("root parents != nil")
	}
}

func TestFlattenTreeCollapsingRemovesGrandchildRowsToo(t *testing.T) {
	c1 := NewTreeNode("c1")
	b := NewTreeNode("b", c1).SetExpanded(true)
	a := NewTreeNode("a", b).SetExpanded(true)
	roots := []*TreeNode{a}

	// a expanded, b expanded: a, b, c1 -> 3 rows.
	if got := len(flattenTree(roots, nil)); got != 3 {
		t.Fatalf("len = %d, want 3", got)
	}

	a.SetExpanded(false)
	if got := len(flattenTree(roots, nil)); got != 1 {
		t.Fatalf("len after collapsing a = %d, want 1 (b and c1 hidden too)", got)
	}
}

func TestFlattenTreeDeepNestingDepthMath(t *testing.T) {
	d3 := NewTreeNode("d3")
	d2 := NewTreeNode("d2", d3).SetExpanded(true)
	d1 := NewTreeNode("d1", d2).SetExpanded(true)
	d0 := NewTreeNode("d0", d1).SetExpanded(true)

	rows := flattenTree([]*TreeNode{d0}, nil)
	if got, want := len(rows), 4; got != want {
		t.Fatalf("len = %d, want %d", got, want)
	}
	for i, row := range rows {
		if row.depth != i {
			t.Fatalf("rows[%d].depth = %d, want %d", i, row.depth, i)
		}
	}
}

func TestFlattenTreeLeafHasNoChildren(t *testing.T) {
	leaf := NewTreeNode("leaf")
	if len(leaf.Children) != 0 {
		t.Fatal("fresh leaf node has children, want none")
	}
	if leaf.Expanded() {
		t.Fatal("fresh node starts expanded, want collapsed")
	}
}

// --- findAncestors ---

func TestFindAncestorsReturnsRootToParentOrder(t *testing.T) {
	target := NewTreeNode("target")
	mid := NewTreeNode("mid", target)
	root := NewTreeNode("root", mid)

	anc := findAncestors([]*TreeNode{root}, target)
	if len(anc) != 2 || anc[0] != root || anc[1] != mid {
		t.Fatalf("findAncestors = %+v, want [root, mid]", anc)
	}
}

func TestFindAncestorsRootNodeHasNoAncestors(t *testing.T) {
	root := NewTreeNode("root")
	if anc := findAncestors([]*TreeNode{root}, root); anc != nil {
		t.Fatalf("findAncestors(root) = %+v, want nil", anc)
	}
}

func TestFindAncestorsNotInTreeReturnsNil(t *testing.T) {
	root := NewTreeNode("root")
	other := NewTreeNode("other")
	if anc := findAncestors([]*TreeNode{root}, other); anc != nil {
		t.Fatalf("findAncestors(unrelated) = %+v, want nil", anc)
	}
}

func TestFindAncestorsNilTargetReturnsNil(t *testing.T) {
	root := NewTreeNode("root")
	if anc := findAncestors([]*TreeNode{root}, nil); anc != nil {
		t.Fatalf("findAncestors(nil) = %+v, want nil", anc)
	}
}

// --- Chevron vs label click zones ---

func TestTreeViewClickChevronZoneTogglesNotSelects(t *testing.T) {
	child := NewTreeNode("child")
	root := NewTreeNode("root", child)
	tv := NewTreeView(nil, root)
	tv.rowH = 20
	layoutTreeView(tv, 0, 0, 200, 100)

	var got []*TreeNode
	tv.OnChanged(func(n *TreeNode) { got = append(got, n) })

	r := input.NewRouter()
	// x = 5 falls within [0, 16) — the depth-0 chevron zone.
	e := &input.PointerEvent{Action: input.Press, Pos: render.Point{X: 5, Y: 5}, Router: r}
	tv.OnPointer(e)

	if !e.Handled {
		t.Fatal("press in chevron zone not marked Handled")
	}
	if !root.Expanded() {
		t.Fatal("chevron-zone click did not expand root")
	}
	if tv.Selected() != nil {
		t.Fatalf("Selected() = %v after chevron click, want nil (chevron click must not select)", tv.Selected())
	}
	if len(got) != 0 {
		t.Fatalf("OnChanged fired %d times for a chevron click, want 0", len(got))
	}
}

func TestTreeViewClickLabelZoneSelectsNotToggles(t *testing.T) {
	child := NewTreeNode("child")
	root := NewTreeNode("root", child)
	tv := NewTreeView(nil, root)
	tv.rowH = 20
	layoutTreeView(tv, 0, 0, 200, 100)

	var got []*TreeNode
	tv.OnChanged(func(n *TreeNode) { got = append(got, n) })

	r := input.NewRouter()
	// x = 40 is well past the depth-0 chevron zone [0,16).
	e := &input.PointerEvent{Action: input.Press, Pos: render.Point{X: 40, Y: 5}, Router: r}
	tv.OnPointer(e)

	if !e.Handled {
		t.Fatal("press in label zone not marked Handled")
	}
	if root.Expanded() {
		t.Fatal("label-zone click toggled root, want unchanged (still collapsed)")
	}
	if tv.Selected() != root {
		t.Fatalf("Selected() = %v, want root", tv.Selected())
	}
	if len(got) != 1 || got[0] != root {
		t.Fatalf("OnChanged calls = %v, want [root]", got)
	}
}

func TestTreeViewChevronZoneExactBoundary(t *testing.T) {
	child := NewTreeNode("child")
	root := NewTreeNode("root", child)
	tv := NewTreeView(nil, root)
	tv.rowH = 20
	layoutTreeView(tv, 0, 0, 200, 100)

	r := input.NewRouter()
	// x = 16 is the first pixel OUTSIDE the chevron zone [0,16) -> label.
	e := &input.PointerEvent{Action: input.Press, Pos: render.Point{X: 16, Y: 5}, Router: r}
	tv.OnPointer(e)

	if root.Expanded() {
		t.Fatal("x=16 (boundary, exclusive) toggled root, want a label-zone select instead")
	}
	if tv.Selected() != root {
		t.Fatalf("Selected() = %v at x=16, want root (label zone)", tv.Selected())
	}
}

func TestTreeViewChevronZoneIndentedByDepth(t *testing.T) {
	child := NewTreeNode("child", NewTreeNode("grandchild"))
	root := NewTreeNode("root", child).SetExpanded(true)
	tv := NewTreeView(nil, root)
	tv.rowH = 20
	layoutTreeView(tv, 0, 0, 200, 100) // rows: root(depth0), child(depth1)

	r := input.NewRouter()
	// child's chevron zone is [16, 32); x=20 should toggle child, not select.
	e := &input.PointerEvent{Action: input.Press, Pos: render.Point{X: 20, Y: 25}, Router: r}
	tv.OnPointer(e)

	if !child.Expanded() {
		t.Fatal("depth-1 chevron click did not expand child")
	}
	if tv.Selected() != nil {
		t.Fatal("depth-1 chevron click selected a node, want none")
	}
}

func TestTreeViewClickLeafChevronZoneIsNoOpToggleButStillHandled(t *testing.T) {
	leaf := NewTreeNode("leaf") // no children: chevron zone still reserved, drawn empty
	tv := NewTreeView(nil, leaf)
	tv.rowH = 20
	layoutTreeView(tv, 0, 0, 200, 100)

	r := input.NewRouter()
	e := &input.PointerEvent{Action: input.Press, Pos: render.Point{X: 5, Y: 5}, Router: r}
	tv.OnPointer(e)

	if !e.Handled {
		t.Fatal("press in a leaf's chevron zone not marked Handled")
	}
	if tv.Selected() != nil {
		t.Fatalf("Selected() = %v after leaf chevron-zone click, want nil (still a no-op toggle, not a select)", tv.Selected())
	}
}

func TestTreeViewClickOutsideAnyRowUnhandled(t *testing.T) {
	root := NewTreeNode("root")
	tv := NewTreeView(nil, root)
	tv.rowH = 20
	layoutTreeView(tv, 0, 0, 200, 100)

	r := input.NewRouter()
	e := &input.PointerEvent{Action: input.Press, Pos: render.Point{X: 5, Y: 500}, Router: r}
	tv.OnPointer(e)

	if e.Handled {
		t.Fatal("press well below any row marked Handled, want false")
	}
}

// --- Selection: SetSelected (silent, auto-expands hidden ancestors) ---

func TestTreeViewSetSelectedIsSilent(t *testing.T) {
	root := NewTreeNode("root")
	tv := NewTreeView(nil, root)

	fired := false
	tv.OnChanged(func(*TreeNode) { fired = true })

	tv.SetSelected(root)
	if tv.Selected() != root {
		t.Fatalf("Selected() = %v, want root", tv.Selected())
	}
	if fired {
		t.Fatal("SetSelected fired OnChanged, want fully silent (programmatic setter)")
	}
}

func TestTreeViewSetSelectedOnHiddenNodeAutoExpandsAncestors(t *testing.T) {
	target := NewTreeNode("target")
	mid := NewTreeNode("mid", target) // starts collapsed
	root := NewTreeNode("root", mid)  // starts collapsed
	tv := NewTreeView(nil, root)
	tv.rowH = 20
	layoutTreeView(tv, 0, 0, 200, 100) // only "root" visible initially

	if got := len(tv.rows); got != 1 {
		t.Fatalf("initial visible rows = %d, want 1 (root only, both descendants collapsed)", got)
	}

	tv.SetSelected(target)

	if !root.Expanded() || !mid.Expanded() {
		t.Fatal("SetSelected(target) did not auto-expand target's ancestors")
	}
	if tv.Selected() != target {
		t.Fatalf("Selected() = %v, want target", tv.Selected())
	}

	// Re-layout applies the now-expanded ancestors to the flattened rows.
	layoutTreeView(tv, 0, 0, 200, 100)
	if got := len(tv.rows); got != 3 {
		t.Fatalf("visible rows after SetSelected(target) + relayout = %d, want 3 (root, mid, target)", got)
	}
	if got := tv.rows[2].node; got != target {
		t.Fatalf("rows[2].node = %v, want target", got)
	}
}

func TestTreeViewSetSelectedAlreadyVisibleNodeDoesNotInvalidate(t *testing.T) {
	root := NewTreeNode("root")
	tv := NewTreeView(nil, root)
	layoutTreeView(tv, 0, 0, 200, 100)

	tv.SetSelected(root) // already visible: no ancestor needed expanding
	if tv.NeedsLayout() {
		t.Fatal("SetSelected on an already-visible node invalidated layout, want no-op")
	}
}

// --- Keyboard: Up/Down over flattened rows ---

func TestTreeViewKeyboardUpDown(t *testing.T) {
	a := NewTreeNode("a")
	b := NewTreeNode("b")
	c := NewTreeNode("c")
	tv := NewTreeView(nil, a, b, c)
	tv.rowH = 20
	layoutTreeView(tv, 0, 0, 200, 200)
	tv.OnFocusChanged(true)

	var got []*TreeNode
	tv.OnChanged(func(n *TreeNode) { got = append(got, n) })

	press := func(k input.Key) {
		e := &input.KeyEvent{Action: input.Press, Key: k}
		tv.OnKey(e)
		if !e.Handled {
			t.Fatalf("key %v not marked Handled", k)
		}
	}

	press(input.KeyDown) // no prior selection -> lands on row 0 (a)
	if tv.Selected() != a {
		t.Fatalf("Selected() after first Down = %v, want a", tv.Selected())
	}
	press(input.KeyDown)
	if tv.Selected() != b {
		t.Fatalf("Selected() after second Down = %v, want b", tv.Selected())
	}
	press(input.KeyDown)
	if tv.Selected() != c {
		t.Fatalf("Selected() after third Down = %v, want c", tv.Selected())
	}
	press(input.KeyDown) // clamped: stays at c (last row)
	if tv.Selected() != c {
		t.Fatalf("Selected() after Down at last row = %v, want c (clamped)", tv.Selected())
	}
	press(input.KeyUp)
	if tv.Selected() != b {
		t.Fatalf("Selected() after Up = %v, want b", tv.Selected())
	}

	want := []*TreeNode{a, b, c, b}
	if len(got) != len(want) {
		t.Fatalf("OnChanged calls len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("OnChanged calls[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

// --- Keyboard: Right expands / descends into first child / no-op leaf ---

func TestTreeViewKeyRightExpandsCollapsedNodeWithChildren(t *testing.T) {
	child := NewTreeNode("child")
	root := NewTreeNode("root", child)
	tv := NewTreeView(nil, root)
	tv.rowH = 20
	layoutTreeView(tv, 0, 0, 200, 200)
	tv.OnFocusChanged(true)
	tv.SetSelected(root)

	e := &input.KeyEvent{Action: input.Press, Key: input.KeyRight}
	tv.OnKey(e)

	if !e.Handled {
		t.Fatal("Right not marked Handled")
	}
	if !root.Expanded() {
		t.Fatal("Right did not expand root")
	}
}

func TestTreeViewKeyRightOnLeafIsNoOp(t *testing.T) {
	leaf := NewTreeNode("leaf")
	tv := NewTreeView(nil, leaf)
	tv.rowH = 20
	layoutTreeView(tv, 0, 0, 200, 200)
	tv.OnFocusChanged(true)
	tv.SetSelected(leaf)

	e := &input.KeyEvent{Action: input.Press, Key: input.KeyRight}
	tv.OnKey(e)

	if !e.Handled {
		t.Fatal("Right on a leaf not marked Handled (should still be handled, just a no-op)")
	}
	if leaf.Expanded() {
		t.Fatal("Right on a leaf somehow expanded it")
	}
}

// TestTreeViewKeyRightOnExpandedNodeDescendsToFirstChild locks in the
// WinUI-adopted behavior: Right on a node that is ALREADY expanded (and has
// children) moves the selection down to its first child, rather than being
// a no-op.
func TestTreeViewKeyRightOnExpandedNodeDescendsToFirstChild(t *testing.T) {
	child1 := NewTreeNode("child1")
	child2 := NewTreeNode("child2")
	root := NewTreeNode("root", child1, child2).SetExpanded(true)
	tv := NewTreeView(nil, root)
	tv.rowH = 20
	layoutTreeView(tv, 0, 0, 200, 200)
	tv.OnFocusChanged(true)
	tv.SetSelected(root)

	var got []*TreeNode
	tv.OnChanged(func(n *TreeNode) { got = append(got, n) })

	e := &input.KeyEvent{Action: input.Press, Key: input.KeyRight}
	tv.OnKey(e)

	if !e.Handled {
		t.Fatal("Right not marked Handled")
	}
	if tv.Selected() != child1 {
		t.Fatalf("Selected() after Right on an already-expanded node = %v, want child1 (WinUI: descend into first child)", tv.Selected())
	}
	if !root.Expanded() {
		t.Fatal("Right on an already-expanded node collapsed it, want unchanged")
	}
	if len(got) != 1 || got[0] != child1 {
		t.Fatalf("OnChanged calls = %v, want [child1] (descending is a genuine user-driven selection change)", got)
	}
}

func TestTreeViewKeyRightLeftUnhandledWithNoSelection(t *testing.T) {
	root := NewTreeNode("root")
	tv := NewTreeView(nil, root)
	tv.rowH = 20
	layoutTreeView(tv, 0, 0, 200, 200)
	tv.OnFocusChanged(true)
	// No SetSelected call: tv.Selected() == nil, even though rows exist.

	er := &input.KeyEvent{Action: input.Press, Key: input.KeyRight}
	tv.OnKey(er)
	if er.Handled {
		t.Fatal("Right with no current selection marked Handled, want false")
	}

	el := &input.KeyEvent{Action: input.Press, Key: input.KeyLeft}
	tv.OnKey(el)
	if el.Handled {
		t.Fatal("Left with no current selection marked Handled, want false")
	}
}

// --- Keyboard: Left collapses / jumps to parent ---

func TestTreeViewKeyLeftCollapsesExpandedNode(t *testing.T) {
	child := NewTreeNode("child")
	root := NewTreeNode("root", child).SetExpanded(true)
	tv := NewTreeView(nil, root)
	tv.rowH = 20
	layoutTreeView(tv, 0, 0, 200, 200)
	tv.OnFocusChanged(true)
	tv.SetSelected(root)

	e := &input.KeyEvent{Action: input.Press, Key: input.KeyLeft}
	tv.OnKey(e)

	if !e.Handled {
		t.Fatal("Left not marked Handled")
	}
	if root.Expanded() {
		t.Fatal("Left did not collapse the expanded root")
	}
	if tv.Selected() != root {
		t.Fatalf("Selected() after collapsing Left = %v, want root (unchanged)", tv.Selected())
	}
}

func TestTreeViewKeyLeftOnLeafJumpsToParent(t *testing.T) {
	child := NewTreeNode("child")
	root := NewTreeNode("root", child).SetExpanded(true)
	tv := NewTreeView(nil, root)
	tv.rowH = 20
	layoutTreeView(tv, 0, 0, 200, 200)
	tv.OnFocusChanged(true)
	tv.SetSelected(child)

	e := &input.KeyEvent{Action: input.Press, Key: input.KeyLeft}
	tv.OnKey(e)

	if !e.Handled {
		t.Fatal("Left not marked Handled")
	}
	if tv.Selected() != root {
		t.Fatalf("Selected() after Left on a leaf child = %v, want root (jumped to parent)", tv.Selected())
	}
}

func TestTreeViewKeyLeftOnCollapsedNodeWithChildrenJumpsToParent(t *testing.T) {
	grandchild := NewTreeNode("grandchild")
	child := NewTreeNode("child", grandchild) // starts collapsed
	root := NewTreeNode("root", child).SetExpanded(true)
	tv := NewTreeView(nil, root)
	tv.rowH = 20
	layoutTreeView(tv, 0, 0, 200, 200)
	tv.OnFocusChanged(true)
	tv.SetSelected(child)

	e := &input.KeyEvent{Action: input.Press, Key: input.KeyLeft}
	tv.OnKey(e)

	if tv.Selected() != root {
		t.Fatalf("Selected() after Left on an already-collapsed node = %v, want root (jump to parent, nothing to collapse)", tv.Selected())
	}
}

func TestTreeViewKeyLeftOnRootLeafIsNoOp(t *testing.T) {
	root := NewTreeNode("root")
	tv := NewTreeView(nil, root)
	tv.rowH = 20
	layoutTreeView(tv, 0, 0, 200, 200)
	tv.OnFocusChanged(true)
	tv.SetSelected(root)

	e := &input.KeyEvent{Action: input.Press, Key: input.KeyLeft}
	tv.OnKey(e)

	if tv.Selected() != root {
		t.Fatalf("Selected() after Left on a root leaf = %v, want root (no parent to jump to)", tv.Selected())
	}
}

// --- Measure content: width accounts for indent+chevron+label ---

func TestTreeViewMeasureContentHeightIsRowCountTimesRowH(t *testing.T) {
	a := NewTreeNode("a")
	b := NewTreeNode("b")
	tv := NewTreeView(nil, a, b)
	tv.rowH = 20
	core.MeasureWidget(tv, render.Size{W: 500, H: 500})

	if got, want := core.DesiredSizeOf(tv).H, float32(40); got != want {
		t.Fatalf("desired H = %v, want %v (2 rows * rowH 20)", got, want)
	}
}

func TestTreeViewMeasureContentReflectsCurrentExpandState(t *testing.T) {
	child := NewTreeNode("child")
	root := NewTreeNode("root", child)
	tv := NewTreeView(nil, root)
	tv.rowH = 20

	core.MeasureWidget(tv, render.Size{W: 500, H: 500})
	collapsedH := core.DesiredSizeOf(tv).H

	root.SetExpanded(true)
	core.MeasureWidget(tv, render.Size{W: 500, H: 500})
	expandedH := core.DesiredSizeOf(tv).H

	if expandedH <= collapsedH {
		t.Fatalf("expandedH = %v, collapsedH = %v; want expanded strictly taller", expandedH, collapsedH)
	}
}

// --- Theming: real face, real colors (sanity that nothing panics) ---

func TestTreeViewWithRealFaceAndTheme(t *testing.T) {
	theme.SetActive(theme.FluentLight())
	defer theme.SetActive(nil)

	face := testFace(t)
	child := NewTreeNode("child")
	root := NewTreeNode("root", child).SetExpanded(true)
	tv := NewTreeView(face, root)
	layoutTreeView(tv, 0, 0, 200, 200)

	if got := len(tv.rows); got != 2 {
		t.Fatalf("rows = %d, want 2", got)
	}
	if tv.rowH <= 0 {
		t.Fatalf("rowH = %v, want > 0 with a real face", tv.rowH)
	}
}

// --- Direct node mutation invalidates the owning TreeView (owner back-ref) ---

// TestTreeNodeSetExpandedInvalidatesOwningTreeView is the reviewer-mandated
// regression test for the invalidation gap: mutating a node DIRECTLY (not
// via tv.toggle/expandRow/collapseOrJumpToParent) must still mark the
// owning TreeView dirty, since nothing else observes the mutation.
func TestTreeNodeSetExpandedInvalidatesOwningTreeView(t *testing.T) {
	child := NewTreeNode("child")
	root := NewTreeNode("root", child)
	tv := NewTreeView(nil, root)
	layoutTreeView(tv, 0, 0, 200, 100)

	if tv.NeedsLayout() {
		t.Fatal("expected clean layout before the direct SetExpanded call")
	}

	root.SetExpanded(true) // NOT via tv.toggle/expandRow — a direct external mutation

	if !tv.NeedsLayout() {
		t.Fatal("direct node.SetExpanded on an attached node did not invalidate its owning TreeView")
	}
}

// TestTreeNodeSetExpandedInvalidatesEvenWhenNodeCurrentlyHidden proves
// NewTreeView's construction-time walkAllNodes tags EVERY node with its
// owner up front — including ones hidden behind a currently-collapsed
// ancestor, which flattenTree itself would never have walked into.
func TestTreeNodeSetExpandedInvalidatesEvenWhenNodeCurrentlyHidden(t *testing.T) {
	grandchild := NewTreeNode("grandchild")
	child := NewTreeNode("child", grandchild) // hidden: root starts collapsed
	root := NewTreeNode("root", child)
	tv := NewTreeView(nil, root)
	layoutTreeView(tv, 0, 0, 200, 100) // only "root" visible; flattenTree never walked into child

	if got := len(tv.rows); got != 1 {
		t.Fatalf("visible rows = %d, want 1 (root only)", got)
	}
	if tv.NeedsLayout() {
		t.Fatal("expected clean layout before the direct SetExpanded call")
	}

	child.SetExpanded(true) // child was never in a flatten pass, only NewTreeView's full walk

	if !tv.NeedsLayout() {
		t.Fatal("direct SetExpanded on a currently-hidden-but-owned node did not invalidate its owning TreeView")
	}
}

// TestTreeNodeAppendedAfterConstructionGetsOwnerOnceReachable proves the
// "newly-reachable nodes during each flatten" half of the ownership rule: a
// node appended to an existing owned node's Children slice AFTER
// construction has no owner until a flatten pass actually walks into it.
func TestTreeNodeAppendedAfterConstructionGetsOwnerOnceReachable(t *testing.T) {
	root := NewTreeNode("root")
	tv := NewTreeView(nil, root)
	layoutTreeView(tv, 0, 0, 200, 100)

	lateChild := NewTreeNode("late")
	root.Children = append(root.Children, lateChild) // missed NewTreeView's own walk entirely
	root.SetExpanded(true)                           // root itself IS owned: invalidates tv
	layoutTreeView(tv, 0, 0, 200, 100)               // re-flatten walks into lateChild, tagging it

	if tv.NeedsLayout() {
		t.Fatal("expected clean layout after the relayout")
	}

	lateChild.SetExpanded(true)

	if !tv.NeedsLayout() {
		t.Fatal("direct SetExpanded on a node only reachable via a later flatten pass did not invalidate its owning TreeView")
	}
}

// TestTreeNodeSetExpandedWithNoOwnerIsHarmless covers a node never attached
// to any TreeView: SetExpanded must not panic (nil owner) and still flips
// the flag.
func TestTreeNodeSetExpandedWithNoOwnerIsHarmless(t *testing.T) {
	n := NewTreeNode("orphan")
	n.SetExpanded(true)
	if !n.Expanded() {
		t.Fatal("SetExpanded(true) on an unowned node did not flip the flag")
	}
}
