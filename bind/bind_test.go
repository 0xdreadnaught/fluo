package bind

import (
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/0xdreadnaught/fluo/controls"
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
)

// testFace loads a real face for layout-accurate interaction tests (nil
// faces are fine for pure state tests, but router-driven tests want a
// non-degenerate control so bounds/desired size aren't all zero).
func testFace(t *testing.T) *text.Face {
	t.Helper()
	f, err := text.Load(goregular.TTF)
	if err != nil {
		t.Fatalf("text.Load: %v", err)
	}
	return text.NewFace(f, 14)
}

// layout measures then arranges w at the given absolute rect — the pattern
// every router-driven test below shares (mirrors controls' own
// layoutButton/layoutSlider helpers).
func layout(w core.Widget, bounds render.Rect) {
	core.MeasureWidget(w, render.Size{W: bounds.W, H: bounds.H})
	core.ArrangeWidget(w, bounds)
}

// clickAt fires a press-then-release at pos through r.
func clickAt(r *input.Router, pos render.Point) {
	r.PointerButton(input.ButtonLeft, true, pos, 0)
	r.PointerButton(input.ButtonLeft, false, pos, 0)
}

// --- OneWay ---

func TestOneWayAppliesCurrentValueImmediately(t *testing.T) {
	var p core.Property[int]
	p.Set(5)

	var got []int
	OneWay(&p, func(v int) { got = append(got, v) })

	if len(got) != 1 || got[0] != 5 {
		t.Fatalf("apply calls = %v, want [5] (applied immediately on bind)", got)
	}
}

func TestOneWayReappliesOnEveryChange(t *testing.T) {
	var p core.Property[int]

	var got []int
	OneWay(&p, func(v int) { got = append(got, v) })

	p.Set(1)
	p.Set(2)

	if len(got) != 3 || got[0] != 0 || got[1] != 1 || got[2] != 2 {
		t.Fatalf("apply calls = %v, want [0 1 2]", got)
	}
}

func TestOneWayCancelDetachesAndIsIdempotent(t *testing.T) {
	var p core.Property[int]

	var got []int
	cancel := OneWay(&p, func(v int) { got = append(got, v) })

	p.Set(1)
	cancel()
	p.Set(2)

	if len(got) != 2 {
		t.Fatalf("apply calls = %v after cancel, want len 2 (no further applies)", got)
	}

	cancel() // idempotent: must not panic
	p.Set(3)
	if len(got) != 2 {
		t.Fatalf("apply calls = %v after second cancel + Set, want len 2 still", got)
	}
}

// --- Text (the fully worked-out binder: bind applies current, model push,
// echo-safe user edit via the real router-driven typing path, cancel
// detaches both directions + idempotent) ---

func TestTextBindAppliesCurrentValueImmediately(t *testing.T) {
	var p core.Property[string]
	p.Set("hello")

	tb := controls.NewTextBox(testFace(t))
	Text(&p, tb)

	if tb.Text() != "hello" {
		t.Fatalf("tb.Text() = %q after bind, want %q (current value applied)", tb.Text(), "hello")
	}
}

func TestTextBindModelPushUpdatesControlSilently(t *testing.T) {
	var p core.Property[string]
	tb := controls.NewTextBox(testFace(t))
	Text(&p, tb)

	fires := 0
	p.OnChange(func(_, _ string) { fires++ })

	p.Set("world")

	if tb.Text() != "world" {
		t.Fatalf("tb.Text() = %q after model push, want %q", tb.Text(), "world")
	}
	if fires != 1 {
		t.Fatalf("p's own OnChange observer fired %d times, want 1 (one real Set)", fires)
	}
}

// TestTextBindUserEditViaRouterUpdatesModelEchoSafely drives an actual user
// edit through TextBox's real interaction path (a focused router dispatching
// a typed rune), the same mechanism every TextBox interaction test in
// controls/textbox_test.go uses. It proves both directions of the contract
// at once: the user's edit reaches p, AND the resulting model-push loopback
// (p's subscriber calling tb.SetText) never re-fires tb.OnChanged (which
// would re-Set p, which would loop) — TextBox.SetText's own equality guard
// makes that loopback call a no-op, so the test completing at all (rather
// than hanging/stack-overflowing) is itself evidence of no infinite loop,
// and the asserted final states confirm no corruption resulted.
func TestTextBindUserEditViaRouterUpdatesModelEchoSafely(t *testing.T) {
	var p core.Property[string]
	p.Set("hi")

	tb := controls.NewTextBox(testFace(t))
	tb.SetWidth(300)
	tb.SetHeight(30)
	Text(&p, tb)

	r := input.NewRouter()
	r.SetRoot(tb)
	layout(tb, render.Rect{X: 0, Y: 0, W: 300, H: 30})
	r.Focus(tb)

	r.KeyDown(input.KeyEnd, 0, 0) // caret to end, so the typed rune appends
	r.KeyDown(0, '!', 0)          // user types '!' -> "hi!"

	if p.Get() != "hi!" {
		t.Fatalf("p.Get() = %q after user edit, want %q (pushed into model)", p.Get(), "hi!")
	}
	if tb.Text() != "hi!" {
		t.Fatalf("tb.Text() = %q, want %q", tb.Text(), "hi!")
	}
}

// TestRebindAfterCancelDoesNotFight proves that canceling one binding before
// installing a second on a DIFFERENT control leaves the discarded control
// (and its binding) completely inert: the first bind's cancel fully detaches
// both directions (per the package doc's cancel guarantee), so the second
// bind on the new control neither fights nor gets fought by the first.
func TestRebindAfterCancelDoesNotFight(t *testing.T) {
	var p core.Property[string]
	p.Set("start")

	tbA := controls.NewTextBox(testFace(t))
	tbA.SetWidth(300)
	tbA.SetHeight(30)
	cancelA := Text(&p, tbA)
	cancelA()

	tbB := controls.NewTextBox(testFace(t))
	tbB.SetWidth(300)
	tbB.SetHeight(30)
	Text(&p, tbB)

	if tbA.Text() != "start" || tbB.Text() != "start" {
		t.Fatalf("after rebind, tbA=%q tbB=%q, want both %q", tbA.Text(), tbB.Text(), "start")
	}

	// A model push after rebind must reach ONLY B; A (cancelled) stays silent.
	p.Set("model-push")
	if tbB.Text() != "model-push" {
		t.Fatalf("tbB.Text() = %q after model push, want %q", tbB.Text(), "model-push")
	}
	if tbA.Text() != "start" {
		t.Fatalf("tbA.Text() = %q after model push, want unchanged %q (A cancelled, must stay silent)", tbA.Text(), "start")
	}

	// A user edit on B, driven through the real router, must reach p.
	r := input.NewRouter()
	r.SetRoot(tbB)
	layout(tbB, render.Rect{X: 0, Y: 0, W: 300, H: 30})
	r.Focus(tbB)
	r.KeyDown(input.KeyEnd, 0, 0)
	r.KeyDown(0, '!', 0)

	if p.Get() != "model-push!" {
		t.Fatalf("p.Get() = %q after user edit on B, want %q (pushed into model)", p.Get(), "model-push!")
	}
	if tbA.Text() != "start" {
		t.Fatalf("tbA.Text() = %q after B's user edit, want unchanged %q (A fully silent throughout)", tbA.Text(), "start")
	}
}

func TestTextBindCancelDetachesBothDirections(t *testing.T) {
	var p core.Property[string]
	p.Set("hi")

	tb := controls.NewTextBox(testFace(t))
	tb.SetWidth(300)
	tb.SetHeight(30)
	cancel := Text(&p, tb)

	r := input.NewRouter()
	r.SetRoot(tb)
	layout(tb, render.Rect{X: 0, Y: 0, W: 300, H: 30})
	r.Focus(tb)

	cancel()

	// Edits after cancel must not touch p.
	r.KeyDown(input.KeyEnd, 0, 0)
	r.KeyDown(0, '!', 0)
	if p.Get() != "hi" {
		t.Fatalf("p.Get() = %q after post-cancel edit, want %q (unaffected)", p.Get(), "hi")
	}

	// p.Set after cancel must not touch tb.
	p.Set("changed")
	if tb.Text() != "hi!" {
		t.Fatalf("tb.Text() = %q after post-cancel model Set, want %q (unaffected)", tb.Text(), "hi!")
	}

	cancel() // idempotent: must not panic
}

// --- Checked / SwitchChecked / ToggleChecked / Value / SelectedIndex: one
// both-directions test per binder. ---

func TestBindCheckedBothDirections(t *testing.T) {
	var p core.Property[bool]
	p.Set(true)

	cb := controls.NewCheckBox(testFace(t), "Agree")
	cb.SetWidth(80)
	cb.SetHeight(20)
	Checked(&p, cb)

	if !cb.Checked() {
		t.Fatal("Checked() = false after bind, want true (current value applied)")
	}

	r := input.NewRouter()
	r.SetRoot(cb)
	layout(cb, render.Rect{X: 0, Y: 0, W: 80, H: 20})

	clickAt(r, render.Point{X: 9, Y: 10}) // user click: toggles true -> false
	if p.Get() != false {
		t.Fatalf("p.Get() = %v after user click, want false (pushed into model)", p.Get())
	}

	p.Set(true) // model push
	if !cb.Checked() {
		t.Fatal("Checked() = false after model push, want true")
	}
}

func TestBindSwitchCheckedBothDirections(t *testing.T) {
	var p core.Property[bool]

	sw := controls.NewToggleSwitch()
	SwitchChecked(&p, sw)

	if sw.Checked() {
		t.Fatal("Checked() = true after bind, want false (current value applied)")
	}

	r := input.NewRouter()
	r.SetRoot(sw)
	layout(sw, render.Rect{X: 0, Y: 0, W: 40, H: 20})

	clickAt(r, render.Point{X: 20, Y: 10}) // user click: false -> true
	if p.Get() != true {
		t.Fatalf("p.Get() = %v after user click, want true", p.Get())
	}

	p.Set(false) // model push
	if sw.Checked() {
		t.Fatal("Checked() = true after model push, want false")
	}
}

func TestBindToggleCheckedBothDirections(t *testing.T) {
	var p core.Property[bool]

	tb := controls.NewToggleButton(testFace(t), "On")
	ToggleChecked(&p, tb)

	if tb.Checked() {
		t.Fatal("Checked() = true after bind, want false")
	}

	r := input.NewRouter()
	r.SetRoot(tb)
	layout(tb, render.Rect{X: 0, Y: 0, W: 80, H: 30})

	clickAt(r, render.Point{X: 40, Y: 15}) // user click: false -> true
	if p.Get() != true {
		t.Fatalf("p.Get() = %v after user click, want true", p.Get())
	}

	p.Set(false) // model push
	if tb.Checked() {
		t.Fatal("Checked() = true after model push, want false")
	}
}

func TestBindValueBothDirections(t *testing.T) {
	var p core.Property[float32]
	p.Set(50)

	s := controls.NewSlider().SetRange(0, 100)
	Value(&p, s)

	if s.Value() != 50 {
		t.Fatalf("Value() = %v after bind, want 50 (current value applied)", s.Value())
	}

	r := input.NewRouter()
	r.SetRoot(s)
	layout(s, render.Rect{X: 0, Y: 0, W: 160, H: 24})

	// Click the right edge of the usable track span: jumps to Max (100).
	clickAt(r, render.Point{X: 152, Y: 12})
	if p.Get() != 100 {
		t.Fatalf("p.Get() = %v after user click-on-track, want 100", p.Get())
	}

	p.Set(25) // model push
	if s.Value() != 25 {
		t.Fatalf("Value() = %v after model push, want 25", s.Value())
	}
}

// popupRowBounds locates the absolute bounds of popup row index within
// host's currently open (topmost) popup, walking down via the plain
// core.Widget interface (Children()/core.BoundsOf) only — bind is external
// to the controls package, so it has no access to ComboBox's unexported
// popup/comboRow types, but every widget in the chain (comboPopupCard's
// single child, the StackPanel, each comboRow) still satisfies core.Widget,
// which is all this needs.
func popupRowBounds(t *testing.T, host *controls.OverlayHost, index int) render.Rect {
	t.Helper()
	hostKids := host.Children() // [content, popup]
	if len(hostKids) < 2 {
		t.Fatalf("host.Children() = %d widgets, want >= 2 (content + open popup)", len(hostKids))
	}
	popup := hostKids[len(hostKids)-1]

	cardKids := popup.Children() // [stack]
	if len(cardKids) != 1 {
		t.Fatalf("popup.Children() = %d widgets, want 1 (the item stack)", len(cardKids))
	}
	stack := cardKids[0]

	rowKids := stack.Children()
	if index < 0 || index >= len(rowKids) {
		t.Fatalf("row index %d out of range (stack has %d rows)", index, len(rowKids))
	}
	return core.BoundsOf(rowKids[index])
}

func TestBindSelectedIndexBothDirections(t *testing.T) {
	var p core.Property[int]
	p.Set(-1)

	combo := controls.NewComboBox(testFace(t))
	combo.SetItems([]string{"Red", "Green", "Blue"})
	combo.SetWidth(120)
	combo.SetHeight(32)
	SelectedIndex(&p, combo)

	if combo.SelectedIndex() != -1 {
		t.Fatalf("SelectedIndex() = %d after bind, want -1", combo.SelectedIndex())
	}

	host := controls.NewOverlayHost()
	r := input.NewRouter()
	host.SetRouter(r)
	host.SetContent(combo)
	r.SetRoot(host)
	layout(host, render.Rect{X: 0, Y: 0, W: 300, H: 300})

	comboBounds := core.BoundsOf(combo)
	clickAt(r, render.Point{X: comboBounds.X + comboBounds.W/2, Y: comboBounds.Y + comboBounds.H/2}) // opens the popup
	layout(host, render.Rect{X: 0, Y: 0, W: 300, H: 300})

	// Select "Green" (index 1) by clicking its row's center.
	rowBounds := popupRowBounds(t, host, 1)
	clickAt(r, render.Point{X: rowBounds.X + rowBounds.W/2, Y: rowBounds.Y + rowBounds.H/2})

	if p.Get() != 1 {
		t.Fatalf("p.Get() = %d after user item click, want 1", p.Get())
	}

	p.Set(2) // model push
	if combo.SelectedIndex() != 2 {
		t.Fatalf("SelectedIndex() = %d after model push, want 2", combo.SelectedIndex())
	}
}

func TestBindSelectedBothDirections(t *testing.T) {
	var p core.Property[int]
	p.Set(-1)

	a := controls.NewRadioButton(testFace(t), "A")
	b := controls.NewRadioButton(testFace(t), "B")
	group := controls.NewRadioGroup().Add(a).Add(b)
	Selected(&p, group)

	if group.SelectedIndex() != -1 {
		t.Fatalf("SelectedIndex() = %d after bind, want -1 (current value applied)", group.SelectedIndex())
	}
	if a.Checked() || b.Checked() {
		t.Fatal("a member is checked after bind, want none (p was -1)")
	}

	host := controls.NewStackPanel(controls.Horizontal).Add(a, b)
	r := input.NewRouter()
	r.SetRoot(host)
	layout(host, render.Rect{X: 0, Y: 0, W: 300, H: 20})

	bBounds := core.BoundsOf(b)
	clickAt(r, render.Point{X: bBounds.X + 9, Y: bBounds.Y + 9}) // user click: selects B (index 1)

	if p.Get() != 1 {
		t.Fatalf("p.Get() = %d after user click, want 1 (pushed into model)", p.Get())
	}

	p.Set(0) // model push
	if group.SelectedIndex() != 0 {
		t.Fatalf("SelectedIndex() = %d after model push, want 0", group.SelectedIndex())
	}
	if !a.Checked() || b.Checked() {
		t.Fatalf("checked states = %v %v after model push, want true false", a.Checked(), b.Checked())
	}
}
