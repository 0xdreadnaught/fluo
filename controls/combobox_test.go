package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/theme"
)

// clickAt fires a press-then-release at pos through r — the standard
// "user click" simulation this file's tests share.
func clickAt(r *input.Router, pos render.Point) {
	r.PointerButton(input.ButtonLeft, true, pos, 0)
	r.PointerButton(input.ButtonLeft, false, pos, 0)
}

// rectCenter returns the center point of rc.
func rectCenter(rc render.Rect) render.Point {
	return render.Point{X: rc.X + rc.W/2, Y: rc.Y + rc.H/2}
}

// newTestCombo returns a ComboBox with the given items, an explicit
// 120x32 size (so its arranged bounds are predictable regardless of face
// metrics), hosted as the sole content of a fresh OverlayHost wired to a
// fresh Router and already laid out at {300,300} — the setup every
// interaction test below shares.
func newTestCombo(t *testing.T, items []string) (*ComboBox, *OverlayHost, *input.Router) {
	t.Helper()
	combo := NewComboBox(buttonFace(t))
	combo.SetItems(items)
	combo.SetWidth(120)
	combo.SetHeight(32)

	host := NewOverlayHost()
	r := input.NewRouter()
	host.SetRouter(r)
	host.SetContent(combo)
	r.SetRoot(host)
	layoutOverlay(host, 300, 300)

	return combo, host, r
}

// popupRows white-box-extracts the *comboRow children of combo's currently
// open popup (combo.popup -> *comboPopupCard -> *StackPanel -> children),
// failing the test if the popup isn't open or the shape doesn't match.
func popupRows(t *testing.T, c *ComboBox) []*comboRow {
	t.Helper()
	card, ok := c.popup.(*comboPopupCard)
	if !ok {
		t.Fatalf("combo.popup type = %T, want *comboPopupCard (or nil: not open?)", c.popup)
	}
	stack, ok := card.child.(*StackPanel)
	if !ok {
		t.Fatalf("popup card child type = %T, want *StackPanel", card.child)
	}
	children := stack.Children()
	rows := make([]*comboRow, len(children))
	for i, w := range children {
		row, ok := w.(*comboRow)
		if !ok {
			t.Fatalf("popup child %d type = %T, want *comboRow", i, w)
		}
		rows[i] = row
	}
	return rows
}

func TestComboBoxClickOpensPopup(t *testing.T) {
	combo, host, r := newTestCombo(t, []string{"Red", "Green", "Blue"})

	if host.PopupCount() != 0 {
		t.Fatalf("PopupCount before click = %d, want 0", host.PopupCount())
	}

	clickAt(r, rectCenter(core.BoundsOf(combo)))

	if host.PopupCount() != 1 {
		t.Fatalf("PopupCount after click = %d, want 1", host.PopupCount())
	}
	if !combo.IsOpen() {
		t.Fatal("IsOpen() = false after click, want true")
	}
}

func TestComboBoxItemClickSelectsClosesAndFires(t *testing.T) {
	combo, host, r := newTestCombo(t, []string{"Red", "Green", "Blue"})

	var got []int
	combo.OnChanged(func(i int) { got = append(got, i) })

	clickAt(r, rectCenter(core.BoundsOf(combo))) // opens
	layoutOverlay(host, 300, 300)                // arrange the popup + its rows

	rows := popupRows(t, combo)
	if len(rows) != 3 {
		t.Fatalf("len(rows) = %d, want 3", len(rows))
	}

	clickAt(r, rectCenter(core.BoundsOf(rows[1]))) // click "Green"

	if host.PopupCount() != 0 {
		t.Fatalf("PopupCount after item click = %d, want 0 (closed)", host.PopupCount())
	}
	if combo.IsOpen() {
		t.Fatal("IsOpen() = true after item click, want false")
	}
	if got := combo.SelectedIndex(); got != 1 {
		t.Fatalf("SelectedIndex() = %d, want 1", got)
	}
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("OnChanged calls = %v, want [1]", got)
	}
}

func TestComboBoxEscClosesPopup(t *testing.T) {
	combo, host, r := newTestCombo(t, []string{"Red", "Green", "Blue"})

	clickAt(r, rectCenter(core.BoundsOf(combo))) // opens + press-to-focus
	if host.PopupCount() != 1 {
		t.Fatalf("PopupCount after click = %d, want 1", host.PopupCount())
	}
	if got := r.Focused(); got != core.Widget(combo) {
		t.Fatalf("Focused() after click = %v, want combo (press-to-focus while opening)", got)
	}

	r.KeyDown(input.KeyEscape, 0, 0)

	if host.PopupCount() != 0 {
		t.Fatalf("PopupCount after Esc = %d, want 0", host.PopupCount())
	}
	if combo.IsOpen() {
		t.Fatal("IsOpen() = true after Esc, want false")
	}
}

func TestComboBoxLightDismissResetsOpenFlagAndReopens(t *testing.T) {
	combo, host, r := newTestCombo(t, []string{"Red", "Green", "Blue"})

	clickAt(r, rectCenter(core.BoundsOf(combo))) // opens
	if host.PopupCount() != 1 {
		t.Fatalf("PopupCount after click = %d, want 1", host.PopupCount())
	}

	// Press well away from both the field and the popup: light-dismiss,
	// via OverlayHost's outside-press swallow (see overlay.go OnPointer).
	outside := render.Point{X: 1, Y: 1}
	r.PointerButton(input.ButtonLeft, true, outside, 0)
	r.PointerButton(input.ButtonLeft, false, outside, 0)

	if host.PopupCount() != 0 {
		t.Fatalf("PopupCount after outside press = %d, want 0 (dismissed)", host.PopupCount())
	}
	if combo.IsOpen() {
		t.Fatal("IsOpen() = true after light dismiss, want false (onDismiss must reset it)")
	}

	clickAt(r, rectCenter(core.BoundsOf(combo))) // second click: reopen

	if host.PopupCount() != 1 {
		t.Fatalf("PopupCount after second click = %d, want 1 (reopened)", host.PopupCount())
	}
}

func TestComboBoxKeyboardOpensWhenFocused(t *testing.T) {
	for _, k := range []input.Key{input.KeySpace, input.KeyEnter, input.KeyDown} {
		combo, host, r := newTestCombo(t, []string{"Red", "Green", "Blue"})

		r.Focus(combo)
		r.KeyDown(k, 0, 0)

		if host.PopupCount() != 1 {
			t.Fatalf("key %v: PopupCount = %d, want 1", k, host.PopupCount())
		}
		if !combo.IsOpen() {
			t.Fatalf("key %v: IsOpen() = false, want true", k)
		}
	}
}

func TestComboBoxSetSelectedIndexSilentAndClamped(t *testing.T) {
	combo := NewComboBox(nil)
	combo.SetItems([]string{"Red", "Green", "Blue"})

	fired := false
	combo.OnChanged(func(int) { fired = true })

	combo.SetSelectedIndex(1)
	if got := combo.SelectedIndex(); got != 1 {
		t.Fatalf("SelectedIndex() = %d, want 1", got)
	}

	combo.SetSelectedIndex(-1)
	if got := combo.SelectedIndex(); got != -1 {
		t.Fatalf("SelectedIndex() = %d, want -1", got)
	}

	combo.SetSelectedIndex(99)
	if got := combo.SelectedIndex(); got != 2 {
		t.Fatalf("SetSelectedIndex(99) clamp = %d, want 2 (len-1)", got)
	}

	combo.SetSelectedIndex(-99)
	if got := combo.SelectedIndex(); got != -1 {
		t.Fatalf("SetSelectedIndex(-99) clamp = %d, want -1", got)
	}

	if fired {
		t.Fatal("SetSelectedIndex fired OnChanged; want silent (programmatic setter)")
	}
}

func TestComboBoxDisabledInert(t *testing.T) {
	combo := NewComboBox(buttonFace(t)).SetEnabled(false)
	combo.SetItems([]string{"Red", "Green"})
	combo.SetWidth(120)
	combo.SetHeight(32)

	if combo.AcceptsFocus() {
		t.Fatal("AcceptsFocus() = true for a disabled combo, want false")
	}

	host := NewOverlayHost()
	r := input.NewRouter()
	host.SetRouter(r)
	host.SetContent(combo)
	r.SetRoot(host)
	layoutOverlay(host, 300, 300)

	clickAt(r, rectCenter(core.BoundsOf(combo)))
	if host.PopupCount() != 0 {
		t.Fatalf("PopupCount after click on disabled combo = %d, want 0", host.PopupCount())
	}

	// Direct OnKey call confirms the disabled guard independent of focus
	// (a disabled combo never accepts focus in the first place, per
	// AcceptsFocus above, so this can't happen via the router — but OnKey's
	// own guard is the defensive backstop).
	e := &input.KeyEvent{Action: input.Press, Key: input.KeyEnter}
	combo.OnKey(e)
	if e.Handled {
		t.Fatal("OnKey(Enter) on a disabled combo set Handled = true, want false")
	}
	if host.PopupCount() != 0 {
		t.Fatalf("PopupCount after OnKey(Enter) on disabled combo = %d, want 0", host.PopupCount())
	}
}

func TestComboBoxFieldShowsSelectedText(t *testing.T) {
	combo := NewComboBox(nil)
	if got := combo.labelText(); got != comboPlaceholder {
		t.Fatalf("labelText() before SetItems = %q, want placeholder %q", got, comboPlaceholder)
	}

	combo.SetItems([]string{"Red", "Green", "Blue"})
	if got := combo.labelText(); got != comboPlaceholder {
		t.Fatalf("labelText() after SetItems with no selection = %q, want placeholder", got)
	}
	if got := combo.label.Text(); got != comboPlaceholder {
		t.Fatalf("label.Text() = %q, want placeholder (synced by SetItems)", got)
	}

	combo.SetSelectedIndex(1)
	if got := combo.labelText(); got != "Green" {
		t.Fatalf("labelText() after SetSelectedIndex(1) = %q, want %q", got, "Green")
	}
	if got := combo.label.Text(); got != "Green" {
		t.Fatalf("label.Text() = %q, want %q (synced by SetSelectedIndex)", got, "Green")
	}
}

func TestComboRowHoverTracksEnterLeave(t *testing.T) {
	colors := theme.Active().Color
	metrics := theme.Active().Metric

	row := newComboRow(nil, "Red", 0, false, colors, metrics, nil)
	row.SetWidth(100)
	row.SetHeight(24)
	core.MeasureWidget(row, render.Size{W: 100, H: 24})
	core.ArrangeWidget(row, render.Rect{X: 0, Y: 0, W: 100, H: 24})

	if row.click.Hover() {
		t.Fatal("Hover() = true before any Enter, want false")
	}

	row.OnPointer(&input.PointerEvent{Action: input.Enter})
	if !row.click.Hover() {
		t.Fatal("Hover() = false after Enter, want true")
	}

	row.OnPointer(&input.PointerEvent{Action: input.Leave})
	if row.click.Hover() {
		t.Fatal("Hover() = true after Leave, want false")
	}
}
