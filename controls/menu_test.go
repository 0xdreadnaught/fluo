package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
)

// newTestMenuBar returns a MenuBar with two top-level entries — "File"
// ("New", "Open", a separator, "Exit") and "Edit" (a single "Undo" item) —
// hosted as the sole content of a fresh OverlayHost wired to a fresh Router
// and already laid out at {300,300}, mirroring newTestCombo's setup. Returns
// the recorded onClick call log (labels, in call order) alongside the usual
// bar/host/router triple.
func newTestMenuBar(t *testing.T) (*MenuBar, *OverlayHost, *input.Router, *[]string) {
	t.Helper()
	face := buttonFace(t)

	clicks := &[]string{}
	record := func(label string) func() {
		return func() { *clicks = append(*clicks, label) }
	}

	bar := NewMenuBar(face)
	bar.AddMenu("File").
		Add("New", record("New")).
		Add("Open", record("Open")).
		AddSeparator().
		Add("Exit", record("Exit"))
	bar.AddMenu("Edit").Add("Undo", record("Undo"))

	host := NewOverlayHost()
	r := input.NewRouter()
	host.SetRouter(r)
	host.SetContent(bar)
	r.SetRoot(host)
	layoutOverlay(host, 300, 300)

	return bar, host, r, clicks
}

// menuPopupStackRows white-box-extracts the child rows of a menu popup
// widget (as returned by buildMenuPopup: a *menuPopupCard wrapping a
// *StackPanel), failing the test if the shape doesn't match.
func menuPopupStackRows(t *testing.T, popup core.Widget) []core.Widget {
	t.Helper()
	card, ok := popup.(*menuPopupCard)
	if !ok {
		t.Fatalf("popup type = %T, want *menuPopupCard", popup)
	}
	stack, ok := card.child.(*StackPanel)
	if !ok {
		t.Fatalf("popup card child type = %T, want *StackPanel", card.child)
	}
	return stack.Children()
}

func TestMenuBarClickOpensPopup(t *testing.T) {
	bar, host, r, _ := newTestMenuBar(t)

	if host.PopupCount() != 0 {
		t.Fatalf("PopupCount before click = %d, want 0", host.PopupCount())
	}

	clickAt(r, rectCenter(bar.cellRect(0))) // "File"

	if host.PopupCount() != 1 {
		t.Fatalf("PopupCount after click = %d, want 1", host.PopupCount())
	}
	if bar.openIdx != 0 {
		t.Fatalf("openIdx = %d, want 0", bar.openIdx)
	}
	if got := r.Focused(); got != core.Widget(bar) {
		t.Fatalf("Focused() after click = %v, want bar (press-to-focus while opening)", got)
	}
}

func TestMenuItemClickFiresAndClosesAll(t *testing.T) {
	bar, host, r, clicks := newTestMenuBar(t)

	clickAt(r, rectCenter(bar.cellRect(0))) // opens File
	layoutOverlay(host, 300, 300)           // arrange the popup + its rows

	rows := menuPopupStackRows(t, bar.popup)
	if len(rows) != 4 { // New, Open, separator, Exit
		t.Fatalf("len(rows) = %d, want 4", len(rows))
	}
	newRow, ok := rows[0].(*menuItemRow)
	if !ok {
		t.Fatalf("rows[0] type = %T, want *menuItemRow", rows[0])
	}

	clickAt(r, rectCenter(core.BoundsOf(newRow)))

	if len(*clicks) != 1 || (*clicks)[0] != "New" {
		t.Fatalf("clicks = %v, want [New]", *clicks)
	}
	if host.PopupCount() != 0 {
		t.Fatalf("PopupCount after item click = %d, want 0 (closed)", host.PopupCount())
	}
	if bar.openIdx != -1 {
		t.Fatalf("openIdx after item click = %d, want -1 (bar state reset)", bar.openIdx)
	}
}

func TestMenuSeparatorClickInert(t *testing.T) {
	bar, host, r, clicks := newTestMenuBar(t)

	clickAt(r, rectCenter(bar.cellRect(0))) // opens File
	layoutOverlay(host, 300, 300)

	rows := menuPopupStackRows(t, bar.popup)
	sep, ok := rows[2].(*menuSeparatorRow)
	if !ok {
		t.Fatalf("rows[2] type = %T, want *menuSeparatorRow", rows[2])
	}

	clickAt(r, rectCenter(core.BoundsOf(sep)))

	if host.PopupCount() != 1 {
		t.Fatalf("PopupCount after separator click = %d, want 1 (still open)", host.PopupCount())
	}
	if len(*clicks) != 0 {
		t.Fatalf("clicks after separator click = %v, want none", *clicks)
	}
}

func TestMenuSubmenuOpensOnHoverAndItemClickClosesAll(t *testing.T) {
	bar, host, r, clicks := newTestMenuBar(t)

	editItems := bar.entries[1].items
	editItems.AddSub("Recent").Add("a.txt", func() { *clicks = append(*clicks, "a.txt") })

	clickAt(r, rectCenter(bar.cellRect(1))) // opens Edit
	layoutOverlay(host, 300, 300)

	rows := menuPopupStackRows(t, bar.popup)
	if len(rows) != 2 { // Undo, Recent>
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	subRow, ok := rows[1].(*menuSubRow)
	if !ok {
		t.Fatalf("rows[1] type = %T, want *menuSubRow", rows[1])
	}

	if host.PopupCount() != 1 {
		t.Fatalf("PopupCount before hover = %d, want 1", host.PopupCount())
	}

	// A real router-driven Move over the sub row — forwarded into the open
	// popup's subtree per OverlayHost's capture-forwarding, synthesizing an
	// Enter that fires menuSubRow.onHover.
	r.PointerMove(rectCenter(core.BoundsOf(subRow)), 0)
	layoutOverlay(host, 300, 300) // arrange the new submenu popup

	if host.PopupCount() != 2 {
		t.Fatalf("PopupCount after hover = %d, want 2 (submenu opened)", host.PopupCount())
	}

	card, ok := bar.popup.(*menuPopupCard)
	if !ok {
		t.Fatalf("bar.popup type = %T, want *menuPopupCard", bar.popup)
	}
	subRows := menuPopupStackRows(t, card.subPopup)
	if len(subRows) != 1 {
		t.Fatalf("len(subRows) = %d, want 1", len(subRows))
	}
	item, ok := subRows[0].(*menuItemRow)
	if !ok {
		t.Fatalf("subRows[0] type = %T, want *menuItemRow", subRows[0])
	}

	clickAt(r, rectCenter(core.BoundsOf(item)))

	if len(*clicks) != 1 || (*clicks)[0] != "a.txt" {
		t.Fatalf("clicks = %v, want [a.txt]", *clicks)
	}
	if host.PopupCount() != 0 {
		t.Fatalf("PopupCount after submenu item click = %d, want 0 (both levels closed)", host.PopupCount())
	}
	if bar.openIdx != -1 {
		t.Fatalf("openIdx after submenu item click = %d, want -1", bar.openIdx)
	}
}

// openEditWithRecentSubmenu opens bar's "Edit" menu (adding a "Recent" submenu
// with one item, "a.txt", to it first) and hovers its trigger row open — the
// shared setup for the two PINNED nested-popup-quirk tests below. Returns the
// parent popup's rows (rows[0] == "Undo", rows[1] == "Recent >", per
// newTestMenuBar's Edit contents) and the parent card, so callers can reach
// into either popup.
func openEditWithRecentSubmenu(t *testing.T, bar *MenuBar, host *OverlayHost, r *input.Router) (rows []core.Widget, card *menuPopupCard) {
	t.Helper()

	editItems := bar.entries[1].items
	editItems.AddSub("Recent").Add("a.txt", nil)

	clickAt(r, rectCenter(bar.cellRect(1))) // opens Edit
	layoutOverlay(host, 300, 300)

	rows = menuPopupStackRows(t, bar.popup)
	if len(rows) != 2 { // Undo, Recent>
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	subRow, ok := rows[1].(*menuSubRow)
	if !ok {
		t.Fatalf("rows[1] type = %T, want *menuSubRow", rows[1])
	}

	r.PointerMove(rectCenter(core.BoundsOf(subRow)), 0)
	layoutOverlay(host, 300, 300)

	if host.PopupCount() != 2 {
		t.Fatalf("PopupCount after hover = %d, want 2 (submenu opened)", host.PopupCount())
	}
	card, ok = bar.popup.(*menuPopupCard)
	if !ok {
		t.Fatalf("bar.popup type = %T, want *menuPopupCard", bar.popup)
	}
	return rows, card
}

// TestMenuOutsidePressWithSubmenuClosesAll pins the NEW (Phase 8 Task 1)
// chain-aware dismissal behavior (see OverlayHost.OnPointer's own doc
// comment): with a submenu open (two popups on the stack, the submenu
// topmost), a SINGLE outside press falls inside NO popup on the stack, so
// OverlayHost.OnPointer's outside-press branch closes the WHOLE chain —
// submenu AND parent menu — via CloseAllPopups, in one press. The bar's own
// state (openIdx, reset by the parent's onDismiss) ends up fully closed too.
func TestMenuOutsidePressWithSubmenuClosesAll(t *testing.T) {
	bar, host, r, _ := newTestMenuBar(t)
	openEditWithRecentSubmenu(t, bar, host, r)

	outside := render.Point{X: 1, Y: 1}
	r.PointerButton(input.ButtonLeft, true, outside, 0)
	r.PointerButton(input.ButtonLeft, false, outside, 0)

	if host.PopupCount() != 0 {
		t.Fatalf("PopupCount after outside press = %d, want 0 (whole chain closed in one press)", host.PopupCount())
	}
	if bar.openIdx != -1 {
		t.Fatalf("openIdx after outside press = %d, want -1 (bar state reset)", bar.openIdx)
	}
}

// TestMenuSiblingRowHoverAutoClosesSubmenu pins the NEW (Phase 8 Task 1)
// chain-aware hover behavior (see OverlayHost.OnPointer's own doc comment):
// with a submenu open, a Move over a PARENT sibling row ("Undo", still
// visually in the now-second-from-top parent popup) is no longer
// unreachable — popupAt finds the PARENT popup as the one actually
// containing the pointer (the submenu, anchored beside its own trigger row,
// does not cover Undo's position), so OnPointer closes the submenu above it
// first (auto-close), then hovers/forwards the Move into the parent popup
// normally: undoRow ends up hovered, and the submenu is gone.
func TestMenuSiblingRowHoverAutoClosesSubmenu(t *testing.T) {
	bar, host, r, _ := newTestMenuBar(t)
	rows, _ := openEditWithRecentSubmenu(t, bar, host, r)

	undoRow, ok := rows[0].(*menuItemRow)
	if !ok {
		t.Fatalf("rows[0] type = %T, want *menuItemRow", rows[0])
	}
	if undoRow.click.Hover() {
		t.Fatal("undoRow.click.Hover() = true before the sibling Move, want false")
	}
	if host.PopupCount() != 2 {
		t.Fatalf("PopupCount before sibling Move = %d, want 2 (submenu open)", host.PopupCount())
	}

	r.PointerMove(rectCenter(core.BoundsOf(undoRow)), 0)

	if !undoRow.click.Hover() {
		t.Fatal("undoRow.click.Hover() = false after Move over it, want true (sibling reachable — chain-aware forwarding)")
	}
	if host.PopupCount() != 1 {
		t.Fatalf("PopupCount after sibling Move = %d, want 1 (submenu auto-closed)", host.PopupCount())
	}
}

func TestMenuEscClosesAll(t *testing.T) {
	bar, host, r, _ := newTestMenuBar(t)

	clickAt(r, rectCenter(bar.cellRect(0))) // opens File, focuses bar
	if host.PopupCount() != 1 {
		t.Fatalf("PopupCount after click = %d, want 1", host.PopupCount())
	}
	if got := r.Focused(); got != core.Widget(bar) {
		t.Fatalf("Focused() = %v, want bar", got)
	}

	r.KeyDown(input.KeyEscape, 0, 0)

	if host.PopupCount() != 0 {
		t.Fatalf("PopupCount after Esc = %d, want 0", host.PopupCount())
	}
	if bar.openIdx != -1 {
		t.Fatalf("openIdx after Esc = %d, want -1", bar.openIdx)
	}
}

func TestMenuLightDismissResetsBarAndReopens(t *testing.T) {
	bar, host, r, _ := newTestMenuBar(t)

	clickAt(r, rectCenter(bar.cellRect(0))) // opens File
	if host.PopupCount() != 1 {
		t.Fatalf("PopupCount after click = %d, want 1", host.PopupCount())
	}

	// Press well away from both the bar and the popup: light-dismiss, via
	// OverlayHost's outside-press swallow (see overlay.go OnPointer).
	outside := render.Point{X: 1, Y: 1}
	r.PointerButton(input.ButtonLeft, true, outside, 0)
	r.PointerButton(input.ButtonLeft, false, outside, 0)

	if host.PopupCount() != 0 {
		t.Fatalf("PopupCount after outside press = %d, want 0 (dismissed)", host.PopupCount())
	}
	if bar.openIdx != -1 {
		t.Fatalf("openIdx after light dismiss = %d, want -1 (bar state reset)", bar.openIdx)
	}

	clickAt(r, rectCenter(bar.cellRect(0))) // second click: reopen

	if host.PopupCount() != 1 {
		t.Fatalf("PopupCount after second click = %d, want 1 (reopened)", host.PopupCount())
	}
}

func TestShowContextMenuOpensAtPointAndItemClickFiresAndCloses(t *testing.T) {
	face := buttonFace(t)

	owner := &ovProbe{}
	owner.SetWidth(50)
	owner.SetHeight(50)

	host := NewOverlayHost()
	r := input.NewRouter()
	host.SetRouter(r)
	host.SetContent(owner)
	r.SetRoot(host)
	layoutOverlay(host, 300, 300)

	var clicked bool
	at := render.Point{X: 40, Y: 40}
	ShowContextMenu(owner, at, face, func(mi *MenuItems) {
		mi.Add("Copy", func() { clicked = true })
	})

	if host.PopupCount() != 1 {
		t.Fatalf("PopupCount after ShowContextMenu = %d, want 1", host.PopupCount())
	}

	layoutOverlay(host, 300, 300) // arrange the popup + its rows

	popup := host.popups[len(host.popups)-1].w
	rows := menuPopupStackRows(t, popup)
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	row, ok := rows[0].(*menuItemRow)
	if !ok {
		t.Fatalf("rows[0] type = %T, want *menuItemRow", rows[0])
	}

	clickAt(r, rectCenter(core.BoundsOf(row)))

	if !clicked {
		t.Fatal("context menu item click did not fire its callback")
	}
	if host.PopupCount() != 0 {
		t.Fatalf("PopupCount after item click = %d, want 0 (closed)", host.PopupCount())
	}
}

func TestOverlayHostCloseAllPopups(t *testing.T) {
	host := NewOverlayHost()
	host.SetContent(NewFixed(400, 300, render.RGB(1, 2, 3)))

	var dismissed []string
	host.ShowPopup(NewFixed(10, 10, render.RGB(0, 0, 0)), render.Rect{}, func() { dismissed = append(dismissed, "a") })
	host.ShowPopup(NewFixed(10, 10, render.RGB(0, 0, 0)), render.Rect{}, func() { dismissed = append(dismissed, "b") })
	host.ShowPopup(NewFixed(10, 10, render.RGB(0, 0, 0)), render.Rect{}, func() { dismissed = append(dismissed, "c") })

	if host.PopupCount() != 3 {
		t.Fatalf("PopupCount before CloseAllPopups = %d, want 3", host.PopupCount())
	}

	host.CloseAllPopups()

	if host.PopupCount() != 0 {
		t.Fatalf("PopupCount after CloseAllPopups = %d, want 0", host.PopupCount())
	}
	want := []string{"c", "b", "a"} // topmost first
	if len(dismissed) != len(want) {
		t.Fatalf("dismissed = %v, want %v", dismissed, want)
	}
	for i := range want {
		if dismissed[i] != want[i] {
			t.Fatalf("dismissed = %v, want %v", dismissed, want)
		}
	}
}
