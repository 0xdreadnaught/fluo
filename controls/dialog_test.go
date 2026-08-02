package controls

import (
	"fmt"
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
)

// newTestDialogHost returns a fresh OverlayHost wired to a fresh Router,
// already laid out at {300,300} — the shared setup every ShowDialog test
// below uses (mirroring newTestCombo/newTestMenuBar's own setup).
func newTestDialogHost(t *testing.T) (*OverlayHost, *input.Router) {
	t.Helper()
	host := NewOverlayHost()
	r := input.NewRouter()
	host.SetRouter(r)
	r.SetRoot(host)
	layoutOverlay(host, 300, 300)
	return host, r
}

// dialogPopupButtons white-box-extracts the button row's current children
// (as *Button) from host's topmost popup — assumed to be a dialog opened via
// ShowDialog — failing the test if the shape doesn't match ShowDialog's own
// construction (dialogScrim -> dialogCard -> StackPanel[title, body,
// buttonRow] -> buttonRow's own Button children, Secondary before Primary
// when both are present).
func dialogPopupButtons(t *testing.T, host *OverlayHost) []*Button {
	t.Helper()
	if host.PopupCount() == 0 {
		t.Fatal("dialogPopupButtons: no popup open")
	}
	popup := host.popups[len(host.popups)-1].w
	scrim, ok := popup.(*dialogScrim)
	if !ok {
		t.Fatalf("popup type = %T, want *dialogScrim", popup)
	}
	card, ok := scrim.card.(*dialogCard)
	if !ok {
		t.Fatalf("scrim.card type = %T, want *dialogCard", scrim.card)
	}
	stack, ok := card.child.(*StackPanel)
	if !ok {
		t.Fatalf("card.child type = %T, want *StackPanel", card.child)
	}
	children := stack.Children()
	if len(children) != 3 {
		t.Fatalf("len(card content children) = %d, want 3 (title, body, buttonRow)", len(children))
	}
	row, ok := children[2].(*StackPanel)
	if !ok {
		t.Fatalf("children[2] type = %T, want *StackPanel (button row)", children[2])
	}
	rowChildren := row.Children()
	buttons := make([]*Button, len(rowChildren))
	for i, w := range rowChildren {
		b, ok := w.(*Button)
		if !ok {
			t.Fatalf("button row child %d type = %T, want *Button", i, w)
		}
		buttons[i] = b
	}
	return buttons
}

func TestDialogPrimaryClickFiresOnceAndCloses(t *testing.T) {
	host, r := newTestDialogHost(t)
	face := buttonFace(t)

	var results []DialogResult
	ShowDialog(host, face, DialogSpec{
		Title: "Delete file?", Body: "This cannot be undone.",
		Primary: "Delete", Secondary: "Cancel",
		OnResult: func(res DialogResult) { results = append(results, res) },
	})

	if got := host.PopupCount(); got != 1 {
		t.Fatalf("PopupCount after ShowDialog = %d, want 1", got)
	}

	layoutOverlay(host, 300, 300) // arrange the popup + its rows

	buttons := dialogPopupButtons(t, host)
	if len(buttons) != 2 {
		t.Fatalf("len(buttons) = %d, want 2 (Cancel, Delete)", len(buttons))
	}
	secondary, primary := buttons[0], buttons[1]
	if !primary.accent {
		t.Fatal("primary button is not accent")
	}
	if secondary.accent {
		t.Fatal("secondary button is accent, want default chrome")
	}

	clickAt(r, rectCenter(core.BoundsOf(primary)))

	if len(results) != 1 || results[0] != DialogPrimary {
		t.Fatalf("results = %v, want [DialogPrimary]", results)
	}
	if got := host.PopupCount(); got != 0 {
		t.Fatalf("PopupCount after primary click = %d, want 0 (closed)", got)
	}
}

func TestDialogSecondaryClickFiresSecondary(t *testing.T) {
	host, r := newTestDialogHost(t)
	face := buttonFace(t)

	var results []DialogResult
	ShowDialog(host, face, DialogSpec{
		Title: "Delete file?", Body: "This cannot be undone.",
		Primary: "Delete", Secondary: "Cancel",
		OnResult: func(res DialogResult) { results = append(results, res) },
	})
	layoutOverlay(host, 300, 300)

	buttons := dialogPopupButtons(t, host)
	secondary := buttons[0]

	clickAt(r, rectCenter(core.BoundsOf(secondary)))

	if len(results) != 1 || results[0] != DialogSecondary {
		t.Fatalf("results = %v, want [DialogSecondary]", results)
	}
	if got := host.PopupCount(); got != 0 {
		t.Fatalf("PopupCount after secondary click = %d, want 0 (closed)", got)
	}
}

func TestDialogEscFiresDismissedOnce(t *testing.T) {
	host, r := newTestDialogHost(t)
	face := buttonFace(t)

	var results []DialogResult
	ShowDialog(host, face, DialogSpec{
		Title: "Delete file?", Body: "This cannot be undone.",
		Primary: "Delete", Secondary: "Cancel",
		OnResult: func(res DialogResult) { results = append(results, res) },
	})

	if got := host.PopupCount(); got != 1 {
		t.Fatalf("PopupCount before Esc = %d, want 1", got)
	}
	if got := r.Focused(); got == nil {
		t.Fatal("Focused() after ShowDialog = nil, want the dialog's scrim (Esc routing requires it)")
	}

	r.KeyDown(input.KeyEscape, 0, 0)

	if len(results) != 1 || results[0] != DialogDismissed {
		t.Fatalf("results = %v, want [DialogDismissed]", results)
	}
	if got := host.PopupCount(); got != 0 {
		t.Fatalf("PopupCount after Esc = %d, want 0 (closed)", got)
	}

	// A second Esc, with the dialog already closed, must not misbehave (no
	// panic, and — since dispatchKey now has nothing of the dialog's left on
	// the focused chain — no further result).
	r.KeyDown(input.KeyEscape, 0, 0)
	if len(results) != 1 {
		t.Fatalf("results after second Esc = %v, want still [DialogDismissed]", results)
	}
}

// TestDialogDoubleFireGuard is the PINNED single-fire regression: clicking
// Primary (which closes the dialog and fires DialogPrimary) followed by an
// Escape key-down that arrives afterward must not produce a second
// OnResult call — see ShowDialog's own doc comment on why this holds (both
// tree-topology, since the popup is detached and Esc has nothing of the
// dialog's left to reach, AND fire's own one-shot latch, independently).
func TestDialogDoubleFireGuard(t *testing.T) {
	host, r := newTestDialogHost(t)
	face := buttonFace(t)

	var results []DialogResult
	ShowDialog(host, face, DialogSpec{
		Title: "Delete file?", Body: "This cannot be undone.",
		Primary: "Delete", Secondary: "Cancel",
		OnResult: func(res DialogResult) { results = append(results, res) },
	})
	layoutOverlay(host, 300, 300)

	buttons := dialogPopupButtons(t, host)
	primary := buttons[1]

	clickAt(r, rectCenter(core.BoundsOf(primary)))
	r.KeyDown(input.KeyEscape, 0, 0) // arrives after the dialog is already closed

	if len(results) != 1 || results[0] != DialogPrimary {
		t.Fatalf("results = %v, want exactly [DialogPrimary] (single fire)", results)
	}
	if got := host.PopupCount(); got != 0 {
		t.Fatalf("PopupCount after click+Esc = %d, want 0", got)
	}
}

// TestDialogScrimPressOutsideCardNoOp proves the scrim's own "outside the
// card" press is a documented no-op (v0), not light-dismiss: a press well
// away from the (small, centered) card, but still inside the 300x300 host,
// must leave the dialog open and fire nothing.
func TestDialogScrimPressOutsideCardNoOp(t *testing.T) {
	host, r := newTestDialogHost(t)
	face := buttonFace(t)

	var results []DialogResult
	ShowDialog(host, face, DialogSpec{
		Title: "Delete file?", Body: "This cannot be undone.",
		Primary: "Delete", Secondary: "Cancel",
		OnResult: func(res DialogResult) { results = append(results, res) },
	})
	layoutOverlay(host, 300, 300)

	outsideCard := render.Point{X: 5, Y: 5} // host's top-left corner, well outside the centered card
	clickAt(r, outsideCard)

	if got := host.PopupCount(); got != 1 {
		t.Fatalf("PopupCount after scrim press = %d, want 1 (still open)", got)
	}
	if len(results) != 0 {
		t.Fatalf("results after scrim press = %v, want none", results)
	}
}

// TestDialogSecondaryOmittedOnlyOneButton proves an empty Secondary label
// omits that button entirely — the row has just the accent Primary button.
func TestDialogSecondaryOmittedOnlyOneButton(t *testing.T) {
	host, _ := newTestDialogHost(t)
	face := buttonFace(t)

	ShowDialog(host, face, DialogSpec{
		Title: "Notice", Body: "Something happened.",
		Primary: "OK",
	})
	layoutOverlay(host, 300, 300)

	buttons := dialogPopupButtons(t, host)
	if len(buttons) != 1 {
		t.Fatalf("len(buttons) = %d, want 1 (Secondary omitted)", len(buttons))
	}
	if !buttons[0].accent {
		t.Fatal("sole remaining button is not accent, want the Primary button")
	}
}

// TestDialogPopupCountLifecycle is the explicit lifecycle regression named
// in the task brief: PopupCount is 0 before ShowDialog, 1 while open, and 0
// again once a result has been produced.
func TestDialogPopupCountLifecycle(t *testing.T) {
	host, r := newTestDialogHost(t)
	face := buttonFace(t)

	if got := host.PopupCount(); got != 0 {
		t.Fatalf("PopupCount before ShowDialog = %d, want 0", got)
	}

	ShowDialog(host, face, DialogSpec{
		Title: "Delete file?", Body: "This cannot be undone.",
		Primary: "Delete", Secondary: "Cancel",
	})
	if got := host.PopupCount(); got != 1 {
		t.Fatalf("PopupCount while open = %d, want 1", got)
	}

	r.KeyDown(input.KeyEscape, 0, 0)
	if got := host.PopupCount(); got != 0 {
		t.Fatalf("PopupCount after result = %d, want 0", got)
	}
}

// --- modal focus trap ---------------------------------------------------

// newTestDialogHostWithBackground is newTestDialogHost plus a single
// focusable Button as CONTENT, standing in for the app behind a dialog: it
// stays in the tree (and so in the unscoped tab order) for the dialog's whole
// lifetime, which is exactly what the focus scope has to keep Tab and
// Space/Enter away from. The returned counter is incremented by the
// background button's own OnClick.
func newTestDialogHostWithBackground(t *testing.T) (*OverlayHost, *input.Router, *Button, *int) {
	t.Helper()
	host, r := newTestDialogHost(t)
	clicks := new(int)
	bg := NewButton(buttonFace(t), "background")
	bg.OnClick(func() { *clicks++ })
	host.SetContent(bg)
	layoutOverlay(host, 300, 300)
	return host, r, bg, clicks
}

// topPopup returns host's topmost popup widget, failing if none is open.
func topPopup(t *testing.T, host *OverlayHost) core.Widget {
	t.Helper()
	if host.PopupCount() == 0 {
		t.Fatal("topPopup: no popup open")
	}
	return host.popups[len(host.popups)-1].w
}

// focusLabel names w for a focus assertion's failure message. Printing a
// laid-out Button or scrim with %v dumps pages of struct (bounds, the whole
// theme token block, every embedded behavior), which buries the one thing
// the assertion is about: which widget focus actually landed on.
func focusLabel(w core.Widget) string {
	switch v := w.(type) {
	case nil:
		return "<nil>"
	case *Button:
		return "button " + v.Label().Text()
	case *dialogScrim:
		return "dialog scrim"
	default:
		return fmt.Sprintf("%T", v)
	}
}

func TestDialogTrapsTabWithinTheCard(t *testing.T) {
	host, r, bg, _ := newTestDialogHostWithBackground(t)

	ShowDialog(host, buttonFace(t), DialogSpec{
		Title: "Delete file?", Body: "This cannot be undone.",
		Primary: "Delete", Secondary: "Cancel",
	})
	layoutOverlay(host, 300, 300)

	scrim := topPopup(t, host)
	if got := r.Focused(); got != scrim {
		t.Fatalf("Focused() after ShowDialog = %s, want the scrim (the focus scope homes focus onto it)", focusLabel(got))
	}

	buttons := dialogPopupButtons(t, host)
	if len(buttons) != 2 {
		t.Fatalf("len(buttons) = %d, want 2 (Cancel, Delete)", len(buttons))
	}

	// Tab enters the card, crosses it, and WRAPS inside it — the background
	// button is never reachable, though it is still visible and focusable.
	want := []core.Widget{buttons[0], buttons[1], buttons[0], buttons[1]}
	for i, w := range want {
		r.KeyDown(input.KeyTab, 0, 0)
		if got := r.Focused(); got != w {
			t.Fatalf("Focused() after Tab #%d = %s, want %s", i+1, focusLabel(got), focusLabel(w))
		}
		if got := r.Focused(); got == core.Widget(bg) {
			t.Fatalf("Tab #%d escaped the dialog onto the background button", i+1)
		}
	}

	// Shift+Tab is trapped the same way, stepping backward within the card.
	back := []core.Widget{buttons[0], buttons[1], buttons[0]}
	for i, w := range back {
		r.KeyDown(input.KeyTab, 0, input.ModShift)
		if got := r.Focused(); got != w {
			t.Fatalf("Focused() after Shift+Tab #%d = %s, want %s", i+1, focusLabel(got), focusLabel(w))
		}
	}
}

func TestButtonlessDialogStaysClosableWithEscapeAfterTab(t *testing.T) {
	// The regression this whole feature exists for: a dialog whose ONLY close
	// path is Escape. Tab used to walk focus out of it into the background,
	// after which Escape bubbled from a background widget and never reached
	// the scrim — leaving the dialog permanently open.
	host, r, bg, _ := newTestDialogHostWithBackground(t)

	var results []DialogResult
	ShowDialog(host, buttonFace(t), DialogSpec{
		Title: "Working", Body: "Please wait.",
		OnResult: func(res DialogResult) { results = append(results, res) },
	})
	layoutOverlay(host, 300, 300)

	scrim := topPopup(t, host)
	if buttons := dialogPopupButtons(t, host); len(buttons) != 0 {
		t.Fatalf("len(buttons) = %d, want 0 (both labels empty)", len(buttons))
	}

	r.KeyDown(input.KeyTab, 0, 0)
	if got := r.Focused(); got == core.Widget(bg) {
		t.Fatal("Tab escaped a button-less dialog onto the background button")
	}
	if got := r.Focused(); got != scrim {
		t.Fatalf("Focused() after Tab = %s, want the scrim (nothing focusable inside to move to)", focusLabel(got))
	}

	r.KeyDown(input.KeyEscape, 0, 0)

	if got := host.PopupCount(); got != 0 {
		t.Fatalf("PopupCount after Escape = %d, want 0", got)
	}
	if len(results) != 1 || results[0] != DialogDismissed {
		t.Fatalf("results = %v, want [DialogDismissed]", results)
	}
}

func TestDialogBlocksKeyboardActivationOfBackgroundButton(t *testing.T) {
	host, r, bg, clicks := newTestDialogHostWithBackground(t)

	ShowDialog(host, buttonFace(t), DialogSpec{Title: "Busy", Body: "Please wait."})
	layoutOverlay(host, 300, 300)

	// Even with the background button explicitly focused (whatever focus a
	// caller may have left behind it), it must not see Space or Enter.
	r.Focus(bg)
	r.KeyDown(input.KeySpace, ' ', 0)
	r.KeyDown(input.KeyEnter, 0, 0)
	if *clicks != 0 {
		t.Fatalf("background clicks while the dialog is open = %d, want 0", *clicks)
	}

	// Escape still reaches the scrim from there, since the out-of-scope focus
	// is ignored and dispatch falls back to the scope root.
	r.KeyDown(input.KeyEscape, 0, 0)
	if got := host.PopupCount(); got != 0 {
		t.Fatalf("PopupCount after Escape = %d, want 0", got)
	}

	// ...and once it's closed the very same key activates it again.
	r.KeyDown(input.KeySpace, ' ', 0)
	if *clicks != 1 {
		t.Fatalf("background clicks after the dialog closed = %d, want 1", *clicks)
	}
}

func TestDialogCloseRestoresBackgroundTabTraversal(t *testing.T) {
	host, r, bg, _ := newTestDialogHostWithBackground(t)

	ShowDialog(host, buttonFace(t), DialogSpec{Title: "Delete?", Body: "Gone forever.", Primary: "OK"})
	layoutOverlay(host, 300, 300)

	r.KeyDown(input.KeyTab, 0, 0)
	if got := r.Focused(); got == core.Widget(bg) {
		t.Fatal("Tab escaped the dialog onto the background button")
	}

	r.KeyDown(input.KeyEscape, 0, 0)
	if got := host.PopupCount(); got != 0 {
		t.Fatalf("PopupCount after Escape = %d, want 0", got)
	}

	r.KeyDown(input.KeyTab, 0, 0)
	if got := r.Focused(); got != core.Widget(bg) {
		t.Fatalf("Focused() after Tab with no dialog open = %s, want the background button", focusLabel(got))
	}
}

func TestNestedDialogsTrapTheTopmost(t *testing.T) {
	host, r, bg, _ := newTestDialogHostWithBackground(t)
	face := buttonFace(t)

	var outerResults, innerResults []DialogResult
	ShowDialog(host, face, DialogSpec{
		Title: "Outer", Body: "First.", Primary: "OuterOK",
		OnResult: func(res DialogResult) { outerResults = append(outerResults, res) },
	})
	layoutOverlay(host, 300, 300)
	outerButtons := dialogPopupButtons(t, host)

	ShowDialog(host, face, DialogSpec{
		Title: "Inner", Body: "Second.", Primary: "InnerOK",
		OnResult: func(res DialogResult) { innerResults = append(innerResults, res) },
	})
	layoutOverlay(host, 300, 300)
	innerScrim := topPopup(t, host)
	innerButtons := dialogPopupButtons(t, host)

	if got := r.Focused(); got != innerScrim {
		t.Fatalf("Focused() after the nested ShowDialog = %s, want the inner scrim", focusLabel(got))
	}

	// Focus is trapped in the TOPMOST dialog: its single button, wrapping to
	// itself, never the outer dialog's button and never the background.
	for i := 0; i < 3; i++ {
		r.KeyDown(input.KeyTab, 0, 0)
		if got := r.Focused(); got != core.Widget(innerButtons[0]) {
			t.Fatalf("Focused() after Tab #%d = %s, want the inner dialog's button", i+1, focusLabel(got))
		}
	}

	// Closing it hands the trap back to the dialog beneath.
	r.KeyDown(input.KeyEscape, 0, 0)
	if got := host.PopupCount(); got != 1 {
		t.Fatalf("PopupCount after the inner Escape = %d, want 1", got)
	}
	if len(innerResults) != 1 || innerResults[0] != DialogDismissed {
		t.Fatalf("innerResults = %v, want [DialogDismissed]", innerResults)
	}
	if len(outerResults) != 0 {
		t.Fatalf("outerResults = %v, want [] (the outer dialog is still open)", outerResults)
	}

	for i := 0; i < 2; i++ {
		r.KeyDown(input.KeyTab, 0, 0)
		if got := r.Focused(); got != core.Widget(outerButtons[0]) {
			t.Fatalf("Focused() after Tab #%d in the outer dialog = %s, want its button", i+1, focusLabel(got))
		}
		if got := r.Focused(); got == core.Widget(bg) {
			t.Fatal("Tab escaped the outer dialog onto the background button")
		}
	}

	r.KeyDown(input.KeyEscape, 0, 0)
	if got := host.PopupCount(); got != 0 {
		t.Fatalf("PopupCount after the outer Escape = %d, want 0", got)
	}
	if len(outerResults) != 1 || outerResults[0] != DialogDismissed {
		t.Fatalf("outerResults = %v, want [DialogDismissed]", outerResults)
	}

	r.KeyDown(input.KeyTab, 0, 0)
	if got := r.Focused(); got != core.Widget(bg) {
		t.Fatalf("Focused() after both dialogs closed = %s, want the background button", focusLabel(got))
	}
}
