package controls

import (
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
