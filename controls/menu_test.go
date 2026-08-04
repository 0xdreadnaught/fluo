package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/theme"
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

// TestMenuItemClickOpeningDialogSurvives pins the close-then-fire order
// buildMenuPopup's item handler uses. An item whose onClick opens a popup of
// its own — here a ShowDialog scrim, but a ShowContextMenu popup or a
// ComboBox dropdown hit the same path — used to have it destroyed on the
// spot: the handler ran onClick first and CloseAllPopups second, so the
// just-opened dialog (now topmost on the host's stack) was the FIRST thing
// that close swept away, firing a phantom DialogDismissed for a dialog the
// user never got to see. Closing first leaves the stack empty before onClick
// runs, so the dialog it opens is the only popup left standing.
func TestMenuItemClickOpeningDialogSurvives(t *testing.T) {
	face := buttonFace(t)

	host := NewOverlayHost()
	r := input.NewRouter()
	host.SetRouter(r)

	var results []DialogResult
	bar := NewMenuBar(face)
	bar.AddMenu("Help").Add("About", func() {
		ShowDialog(host, face, DialogSpec{
			Title:    "About",
			Body:     "fluo",
			Primary:  "OK",
			OnResult: func(res DialogResult) { results = append(results, res) },
		})
	})

	host.SetContent(bar)
	r.SetRoot(host)
	layoutOverlay(host, 300, 300)

	clickAt(r, rectCenter(bar.cellRect(0))) // opens Help
	layoutOverlay(host, 300, 300)           // arrange the popup + its rows

	rows := menuPopupStackRows(t, bar.popup)
	aboutRow, ok := rows[0].(*menuItemRow)
	if !ok {
		t.Fatalf("rows[0] type = %T, want *menuItemRow", rows[0])
	}

	clickAt(r, rectCenter(core.BoundsOf(aboutRow)))

	if host.PopupCount() != 1 {
		t.Fatalf("PopupCount after item click = %d, want 1 (the dialog the item opened)", host.PopupCount())
	}
	if _, ok := host.popups[0].w.(*dialogScrim); !ok {
		t.Fatalf("remaining popup type = %T, want *dialogScrim", host.popups[0].w)
	}
	if len(results) != 0 {
		t.Fatalf("OnResult fired %v, want no result yet (the dialog is still open)", results)
	}
	if bar.openIdx != -1 {
		t.Fatalf("openIdx after item click = %d, want -1 (the menu itself did close)", bar.openIdx)
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

// TestMenuDisabledItemRendersGrayText constructs a disabled menuItemRow
// directly (white-box, via newMenuItemRow) and asserts Render leaves its
// label colored GrayText — regardless of hover, which a disabled row can
// never actually be in via real pointer input (see the hover test below),
// but Render's own guard is unconditional either way (see its doc comment).
func TestMenuDisabledItemRendersGrayText(t *testing.T) {
	face := buttonFace(t)
	th := theme.Active()

	row := newMenuItemRow(face, "Paste", false, th.Color, th.Metric, nil)
	row.Render(nil) // the disabled branch never touches the renderer

	if got := row.label.Color(); got != th.Color.GrayText {
		t.Fatalf("disabled row label color = %v, want GrayText %v", got, th.Color.GrayText)
	}
}

// TestMenuDisabledItemInertAndSiblingUnaffected exercises a real menu with
// one disabled row ("Paste", via AddDisabled) next to an enabled one
// ("Cut"), driven entirely through the live router (matching
// TestMenuItemClickFiresAndClosesAll's own approach): a genuine
// router.PointerMove over the disabled row never lights up its hover (see
// menuItemRow.OnPointer's disabled guard), and a click on it neither fires
// its callback nor closes the menu — while the very same pointer path over
// its enabled sibling still hovers, fires, and closes the menu exactly as
// before AddDisabled existed.
func TestMenuDisabledItemInertAndSiblingUnaffected(t *testing.T) {
	face := buttonFace(t)

	var pasteFired, cutFired bool
	bar := NewMenuBar(face)
	bar.AddMenu("Edit").
		AddDisabled("Paste", func() { pasteFired = true }).
		Add("Cut", func() { cutFired = true })

	host := NewOverlayHost()
	r := input.NewRouter()
	host.SetRouter(r)
	host.SetContent(bar)
	r.SetRoot(host)
	layoutOverlay(host, 300, 300)

	clickAt(r, rectCenter(bar.cellRect(0))) // opens Edit
	layoutOverlay(host, 300, 300)           // arrange the popup + its rows

	rows := menuPopupStackRows(t, bar.popup)
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	pasteRow, ok := rows[0].(*menuItemRow)
	if !ok {
		t.Fatalf("rows[0] type = %T, want *menuItemRow", rows[0])
	}
	cutRow, ok := rows[1].(*menuItemRow)
	if !ok {
		t.Fatalf("rows[1] type = %T, want *menuItemRow", rows[1])
	}

	if pasteRow.enabled {
		t.Fatal("pasteRow.enabled = true, want false (AddDisabled)")
	}
	if !cutRow.enabled {
		t.Fatal("cutRow.enabled = false, want true (Add)")
	}

	// Disabled row: no hover, no fire, menu stays open.
	r.PointerMove(rectCenter(core.BoundsOf(pasteRow)), 0)
	if pasteRow.click.Hover() {
		t.Fatal("pasteRow.click.Hover() = true after a live Move over a disabled row, want false")
	}

	clickAt(r, rectCenter(core.BoundsOf(pasteRow)))
	if pasteFired {
		t.Fatal("disabled Paste row's callback fired, want inert")
	}
	if host.PopupCount() != 1 {
		t.Fatalf("PopupCount after clicking a disabled row = %d, want 1 (menu stays open)", host.PopupCount())
	}

	// Enabled sibling: hover, fire, and close, completely unaffected.
	r.PointerMove(rectCenter(core.BoundsOf(cutRow)), 0)
	if !cutRow.click.Hover() {
		t.Fatal("cutRow.click.Hover() = false after a live Move over an enabled row, want true")
	}

	clickAt(r, rectCenter(core.BoundsOf(cutRow)))
	if !cutFired {
		t.Fatal("enabled Cut row's callback did not fire")
	}
	if host.PopupCount() != 0 {
		t.Fatalf("PopupCount after enabled item click = %d, want 0 (closed)", host.PopupCount())
	}
}

// --- MenuBar hover-switch (v0.17.1 enrichment) --------------------------

// TestMenuHoverSwitchesWhileOpen is the headline: with one top-level menu
// already open, moving the pointer onto a DIFFERENT top-level title switches
// the open menu to it (closing the first, opening the second) — the textbook
// desktop menu-bar flow. Drives the full path: the open popup holds the
// router capture, so the Move reaches OverlayHost, which forwards it to the
// bar (the popup owner), whose OnPointer switches menus.
func TestMenuHoverSwitchesWhileOpen(t *testing.T) {
	bar, host, r, _ := newTestMenuBar(t)

	clickAt(r, rectCenter(bar.cellRect(0))) // open File
	if bar.openIdx != 0 || host.PopupCount() != 1 {
		t.Fatalf("after opening File: openIdx=%d PopupCount=%d, want 0/1", bar.openIdx, host.PopupCount())
	}

	r.PointerMove(rectCenter(bar.cellRect(1)), 0) // hover Edit

	if bar.openIdx != 1 {
		t.Fatalf("openIdx after hovering Edit = %d, want 1 (menu switched on hover)", bar.openIdx)
	}
	if host.PopupCount() != 1 {
		t.Fatalf("PopupCount after switch = %d, want 1 (File's popup closed, Edit's opened)", host.PopupCount())
	}
}

// TestMenuHoverDoesNotOpenWhenClosed is the first gate: with NO menu open,
// hovering a title must not open anything — the first open stays press-driven.
func TestMenuHoverDoesNotOpenWhenClosed(t *testing.T) {
	bar, host, r, _ := newTestMenuBar(t)

	r.PointerMove(rectCenter(bar.cellRect(1)), 0) // hover Edit with nothing open

	if bar.openIdx != -1 {
		t.Fatalf("openIdx after hover with nothing open = %d, want -1 (hover must not open)", bar.openIdx)
	}
	if host.PopupCount() != 0 {
		t.Fatalf("PopupCount = %d, want 0 (hover on a closed bar opens nothing)", host.PopupCount())
	}
	if bar.hoverIdx != 1 {
		t.Fatalf("hoverIdx = %d, want 1 (hover VISUAL still tracks, it just doesn't open)", bar.hoverIdx)
	}
}

// TestMenuHoverSameTitleNoop is the second gate: hovering within the
// already-open title does not churn the popup (openMenu no-ops on the same
// index).
func TestMenuHoverSameTitleNoop(t *testing.T) {
	bar, host, r, _ := newTestMenuBar(t)

	clickAt(r, rectCenter(bar.cellRect(0))) // open File
	openPopup := bar.popup

	r.PointerMove(rectCenter(bar.cellRect(0)), 0) // hover the SAME title

	if bar.openIdx != 0 {
		t.Fatalf("openIdx after same-title hover = %d, want 0 (unchanged)", bar.openIdx)
	}
	if host.PopupCount() != 1 || bar.popup != openPopup {
		t.Fatalf("same-title hover churned the popup (count=%d, same=%v), want no change", host.PopupCount(), bar.popup == openPopup)
	}
}

// TestMenuItemsClearRebuildsDynamically covers the Recent-Maps enrichment
// (v0.19.0): a held *MenuItems can be emptied and rebuilt, and because
// buildMenuPopup reads entries fresh on every open, the next open reflects the
// new contents. Mirrors Eric's consumption pattern (hold the AddSub submenu,
// Clear()+Add() on a state change).
func TestMenuItemsClearRebuildsDynamically(t *testing.T) {
	bar := NewMenuBar(buttonFace(t))
	recent := bar.AddMenu("File").AddSub("Recent")
	recent.Add("old-a", nil).Add("old-b", nil)
	if len(recent.entries) != 2 {
		t.Fatalf("entries after 2 Adds = %d, want 2", len(recent.entries))
	}

	if recent.Clear() != recent {
		t.Fatal("Clear() did not return the receiver for chaining")
	}
	if len(recent.entries) != 0 {
		t.Fatalf("entries after Clear = %d, want 0 (menu emptied)", len(recent.entries))
	}

	recent.Add("x", nil).Add("y", nil).Add("z", nil)
	rows := menuPopupStackRows(t, buildMenuPopup(recent, func() {}))
	if len(rows) != 3 {
		t.Fatalf("rebuilt popup rows = %d, want 3 (x,y,z reflected fresh on open)", len(rows))
	}
}

// TestMenuItemsClearOnEmptyIsSafe guards the degenerate call.
func TestMenuItemsClearOnEmptyIsSafe(t *testing.T) {
	mi := NewMenuBar(buttonFace(t)).AddMenu("Empty")
	if mi.Clear(); len(mi.entries) != 0 {
		t.Fatalf("Clear() on an empty menu left %d entries, want 0", len(mi.entries))
	}
}
