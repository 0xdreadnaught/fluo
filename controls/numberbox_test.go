package controls

import (
	"math"
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
)

func layoutNumberBox(w core.Widget, bounds render.Rect) {
	core.MeasureWidget(w, render.Size{W: bounds.W, H: bounds.H})
	core.ArrangeWidget(w, bounds)
}

func TestNumberBoxDefaults(t *testing.T) {
	n := NewNumberBox(nil)
	if n.Value() != 0 {
		t.Fatalf("Value() = %v, want 0", n.Value())
	}
	if n.Min() != -math.MaxFloat64 {
		t.Fatalf("Min() = %v, want -MaxFloat64", n.Min())
	}
	if n.Max() != math.MaxFloat64 {
		t.Fatalf("Max() = %v, want MaxFloat64", n.Max())
	}
	if n.Step() != 1 {
		t.Fatalf("Step() = %v, want 1", n.Step())
	}
	if !n.enabled {
		t.Fatal("enabled = false, want true")
	}
}

func TestNumberBoxSetValueClampsToRange(t *testing.T) {
	n := NewNumberBox(nil).SetRange(0, 100)

	n.SetValue(-5)
	if n.Value() != 0 {
		t.Fatalf("SetValue(-5) = %v, want 0", n.Value())
	}

	n.SetValue(200)
	if n.Value() != 100 {
		t.Fatalf("SetValue(200) = %v, want 100", n.Value())
	}

	n.SetValue(42)
	if n.Value() != 42 {
		t.Fatalf("SetValue(42) = %v, want 42", n.Value())
	}
}

func TestNumberBoxSetRangeReClampsValue(t *testing.T) {
	n := NewNumberBox(nil).SetRange(0, 100)
	n.SetValue(80)

	n.SetRange(0, 50)
	if n.Value() != 50 {
		t.Fatalf("Value() after shrinking Max = %v, want 50", n.Value())
	}

	n.SetRange(60, 100)
	if n.Value() != 60 {
		t.Fatalf("Value() after raising Min above Value = %v, want 60", n.Value())
	}
}

func TestNumberBoxSetValueAndSetRangeAreSilent(t *testing.T) {
	n := NewNumberBox(nil).SetRange(0, 100)

	var got []float64
	n.OnChanged(func(v float64) { got = append(got, v) })

	n.SetValue(50)
	n.SetValue(50)
	n.SetRange(0, 30)

	if len(got) != 0 {
		t.Fatalf("OnChanged calls = %v, want none", got)
	}
	if n.Value() != 30 {
		t.Fatalf("Value() = %v, want 30", n.Value())
	}
}

func TestNumberBoxArrowKeysStepValue(t *testing.T) {
	n := NewNumberBox(nil).SetRange(0, 100).SetStep(5)
	n.SetValue(50)
	layoutNumberBox(n, render.Rect{X: 0, Y: 0, W: 120, H: 24})

	r := input.NewRouter()
	r.SetRoot(n)
	r.Focus(n)

	var got []float64
	n.OnChanged(func(v float64) { got = append(got, v) })

	r.KeyDown(input.KeyUp, 0, 0)
	if n.Value() != 55 {
		t.Fatalf("Value() after Up = %v, want 55", n.Value())
	}

	r.KeyDown(input.KeyDown, 0, 0)
	if n.Value() != 50 {
		t.Fatalf("Value() after Down = %v, want 50", n.Value())
	}

	if len(got) != 2 {
		t.Fatalf("OnChanged call count = %d, want 2", len(got))
	}
}

func TestNumberBoxShiftArrowKeysStepBy10x(t *testing.T) {
	n := NewNumberBox(nil).SetRange(0, 1000).SetStep(5)
	n.SetValue(100)
	layoutNumberBox(n, render.Rect{X: 0, Y: 0, W: 120, H: 24})

	r := input.NewRouter()
	r.SetRoot(n)
	r.Focus(n)

	r.KeyDown(input.KeyUp, 0, input.ModShift)
	if n.Value() != 150 {
		t.Fatalf("Value() after Shift+Up = %v, want 150", n.Value())
	}

	r.KeyDown(input.KeyDown, 0, input.ModShift)
	if n.Value() != 100 {
		t.Fatalf("Value() after Shift+Down = %v, want 100", n.Value())
	}
}

func TestNumberBoxArrowKeysClamped(t *testing.T) {
	n := NewNumberBox(nil).SetRange(0, 10).SetStep(5)
	n.SetValue(8)
	layoutNumberBox(n, render.Rect{X: 0, Y: 0, W: 120, H: 24})

	r := input.NewRouter()
	r.SetRoot(n)
	r.Focus(n)

	r.KeyDown(input.KeyUp, 0, 0)
	if n.Value() != 10 {
		t.Fatalf("Value() after Up at 8 with step 5 = %v, want 10 (clamped)", n.Value())
	}
}

func TestNumberBoxDisabledIgnoresInput(t *testing.T) {
	n := NewNumberBox(nil).SetRange(0, 100).SetEnabled(false)
	layoutNumberBox(n, render.Rect{X: 0, Y: 0, W: 120, H: 24})

	if n.AcceptsFocus() {
		t.Fatal("AcceptsFocus() = true for disabled, want false")
	}

	e := &input.KeyEvent{Action: input.Press, Key: input.KeyUp}
	n.OnKey(e)
	if e.Handled {
		t.Fatal("disabled NumberBox handled key event")
	}
	if n.Value() != 0 {
		t.Fatalf("Value() after Up on disabled = %v, want 0", n.Value())
	}
}

func TestNumberBoxSpinnerButtonPress(t *testing.T) {
	n := NewNumberBox(nil).SetRange(0, 100).SetStep(1)
	n.SetValue(50)
	layoutNumberBox(n, render.Rect{X: 0, Y: 0, W: 120, H: 24})

	var got []float64
	n.OnChanged(func(v float64) { got = append(got, v) })

	r := input.NewRouter()
	r.SetRoot(n)

	upBtnCenter := render.Point{X: 112, Y: 3}
	r.PointerButton(input.ButtonLeft, true, upBtnCenter, 0)
	r.PointerButton(input.ButtonLeft, false, upBtnCenter, 0)

	if n.Value() != 51 {
		t.Fatalf("Value() after up button click = %v, want 51", n.Value())
	}

	downBtnCenter := render.Point{X: 112, Y: 18}
	r.PointerButton(input.ButtonLeft, true, downBtnCenter, 0)
	r.PointerButton(input.ButtonLeft, false, downBtnCenter, 0)

	if n.Value() != 50 {
		t.Fatalf("Value() after down button click = %v, want 50", n.Value())
	}

	if len(got) != 2 {
		t.Fatalf("OnChanged call count = %d, want 2", len(got))
	}
}

func TestNumberBoxFocusGainSyncsEditBuffer(t *testing.T) {
	n := NewNumberBox(nil).SetRange(0, 100)
	n.SetValue(42)

	n.OnFocusChanged(true)
	if string(n.editRunes) != "42" {
		t.Fatalf("editRunes after focus = %q, want %q", string(n.editRunes), "42")
	}
	if n.editCaret != 2 {
		t.Fatalf("editCaret = %d, want 2", n.editCaret)
	}
}

func TestNumberBoxFocusLossCommitsEdit(t *testing.T) {
	n := NewNumberBox(nil).SetRange(0, 100)
	n.SetValue(42)

	var got []float64
	n.OnChanged(func(v float64) { got = append(got, v) })

	n.OnFocusChanged(true)
	n.editRunes = []rune("75")
	n.editCaret = 2
	n.OnFocusChanged(false)

	if n.Value() != 75 {
		t.Fatalf("Value() after focus loss commit = %v, want 75", n.Value())
	}
	if len(got) != 1 || got[0] != 75 {
		t.Fatalf("OnChanged = %v, want [75]", got)
	}
}

func TestNumberBoxEnterCommitsEdit(t *testing.T) {
	n := NewNumberBox(nil).SetRange(0, 100)
	n.SetValue(10)
	layoutNumberBox(n, render.Rect{X: 0, Y: 0, W: 120, H: 24})

	r := input.NewRouter()
	r.SetRoot(n)
	r.Focus(n)

	var got []float64
	n.OnChanged(func(v float64) { got = append(got, v) })

	n.editRunes = []rune("33")
	n.editCaret = 2

	r.KeyDown(input.KeyEnter, 0, 0)

	if n.Value() != 33 {
		t.Fatalf("Value() after Enter = %v, want 33", n.Value())
	}
	if len(got) != 1 || got[0] != 33 {
		t.Fatalf("OnChanged = %v, want [33]", got)
	}
}

func TestNumberBoxEscapeRevertsEdit(t *testing.T) {
	n := NewNumberBox(nil).SetRange(0, 100)
	n.SetValue(42)
	layoutNumberBox(n, render.Rect{X: 0, Y: 0, W: 120, H: 24})

	r := input.NewRouter()
	r.SetRoot(n)
	r.Focus(n)

	n.editRunes = []rune("99")
	n.editCaret = 2

	r.KeyDown(input.KeyEscape, 0, 0)

	if string(n.editRunes) != "42" {
		t.Fatalf("editRunes after Escape = %q, want %q", string(n.editRunes), "42")
	}
	if n.Value() != 42 {
		t.Fatalf("Value() after Escape = %v, want 42", n.Value())
	}
}

func TestNumberBoxTypingInsertsNumericRunes(t *testing.T) {
	n := NewNumberBox(nil).SetRange(0, 1000)
	n.SetValue(0)
	layoutNumberBox(n, render.Rect{X: 0, Y: 0, W: 120, H: 24})

	r := input.NewRouter()
	r.SetRoot(n)
	r.Focus(n)

	n.editRunes = []rune{}
	n.editCaret = 0

	r.KeyDown(0, '1', 0)
	r.KeyDown(0, '2', 0)
	r.KeyDown(0, '3', 0)

	if string(n.editRunes) != "123" {
		t.Fatalf("editRunes after typing = %q, want %q", string(n.editRunes), "123")
	}
	if n.editCaret != 3 {
		t.Fatalf("editCaret = %d, want 3", n.editCaret)
	}
}

func TestNumberBoxRejectsNonNumericRunes(t *testing.T) {
	n := NewNumberBox(nil)
	n.SetValue(0)
	layoutNumberBox(n, render.Rect{X: 0, Y: 0, W: 120, H: 24})

	r := input.NewRouter()
	r.SetRoot(n)
	r.Focus(n)

	n.editRunes = []rune("5")
	n.editCaret = 1

	r.KeyDown(0, 'a', 0)
	r.KeyDown(0, 'x', 0)

	if string(n.editRunes) != "5" {
		t.Fatalf("editRunes after typing 'ax' = %q, want %q (unchanged)", string(n.editRunes), "5")
	}
}

func TestNumberBoxBackspaceDeletesCharacter(t *testing.T) {
	n := NewNumberBox(nil)
	n.SetValue(0)
	layoutNumberBox(n, render.Rect{X: 0, Y: 0, W: 120, H: 24})

	r := input.NewRouter()
	r.SetRoot(n)
	r.Focus(n)

	n.editRunes = []rune("123")
	n.editCaret = 3

	r.KeyDown(input.KeyBackspace, 0, 0)
	if string(n.editRunes) != "12" || n.editCaret != 2 {
		t.Fatalf("after Backspace: text=%q caret=%d, want %q 2", string(n.editRunes), n.editCaret, "12")
	}
}

func TestNumberBoxDeleteRemovesForward(t *testing.T) {
	n := NewNumberBox(nil)
	n.SetValue(0)
	layoutNumberBox(n, render.Rect{X: 0, Y: 0, W: 120, H: 24})

	r := input.NewRouter()
	r.SetRoot(n)
	r.Focus(n)

	n.editRunes = []rune("123")
	n.editCaret = 1

	r.KeyDown(input.KeyDelete, 0, 0)
	if string(n.editRunes) != "13" || n.editCaret != 1 {
		t.Fatalf("after Delete: text=%q caret=%d, want %q 1", string(n.editRunes), n.editCaret, "13")
	}
}

func TestNumberBoxLeftRightMovesCaret(t *testing.T) {
	n := NewNumberBox(nil)
	n.SetValue(0)
	layoutNumberBox(n, render.Rect{X: 0, Y: 0, W: 120, H: 24})

	r := input.NewRouter()
	r.SetRoot(n)
	r.Focus(n)

	n.editRunes = []rune("123")
	n.editCaret = 3

	r.KeyDown(input.KeyLeft, 0, 0)
	if n.editCaret != 2 {
		t.Fatalf("caret after Left = %d, want 2", n.editCaret)
	}

	r.KeyDown(input.KeyRight, 0, 0)
	if n.editCaret != 3 {
		t.Fatalf("caret after Right = %d, want 3", n.editCaret)
	}

	r.KeyDown(input.KeyHome, 0, 0)
	if n.editCaret != 0 {
		t.Fatalf("caret after Home = %d, want 0", n.editCaret)
	}

	r.KeyDown(input.KeyEnd, 0, 0)
	if n.editCaret != 3 {
		t.Fatalf("caret after End = %d, want 3", n.editCaret)
	}
}

func TestNumberBoxInvalidEditRevertsOnCommit(t *testing.T) {
	n := NewNumberBox(nil).SetRange(0, 100)
	n.SetValue(42)

	n.OnFocusChanged(true)
	n.editRunes = []rune("abc")
	n.editCaret = 3

	var got []float64
	n.OnChanged(func(v float64) { got = append(got, v) })

	n.OnFocusChanged(false)

	if n.Value() != 42 {
		t.Fatalf("Value() after invalid commit = %v, want 42 (unchanged)", n.Value())
	}
	if len(got) != 0 {
		t.Fatalf("OnChanged fired on invalid commit: %v", got)
	}
}

func TestNumberBoxFormatValueDecimals(t *testing.T) {
	n := NewNumberBox(nil)
	n.SetValue(1.5)
	if got := n.formatValue(); got != "1.5" {
		t.Fatalf("formatValue() with decimals=-1 = %q, want %q", got, "1.5")
	}

	n.SetDecimals(2)
	if got := n.formatValue(); got != "1.50" {
		t.Fatalf("formatValue() with decimals=2 = %q, want %q", got, "1.50")
	}

	n.SetDecimals(0)
	n.SetValue(3.7)
	if got := n.formatValue(); got != "4" {
		t.Fatalf("formatValue() with decimals=0, value=3.7 = %q, want %q", got, "4")
	}
}

func TestNumberBoxMeasuresToFixedDesiredSize(t *testing.T) {
	n := NewNumberBox(nil)
	core.MeasureWidget(n, render.Size{W: 1000, H: 1000})
	d := core.DesiredSizeOf(n)
	if d.W != 120 {
		t.Fatalf("DesiredSize().W = %v, want 120", d.W)
	}
}

func TestNumberBoxFocusRingTracked(t *testing.T) {
	n := NewNumberBox(nil)
	if n.focused {
		t.Fatal("focused = true before any focus change")
	}
	n.OnFocusChanged(true)
	if !n.focused {
		t.Fatal("focused = false after OnFocusChanged(true)")
	}
	n.OnFocusChanged(false)
	if n.focused {
		t.Fatal("focused = true after OnFocusChanged(false)")
	}
}

func TestNumberBoxRenderDoesNotPanic(t *testing.T) {
	n := NewNumberBox(nil).SetRange(0, 100)
	n.SetValue(42)
	layoutNumberBox(n, render.Rect{X: 0, Y: 0, W: 120, H: 24})

	rr := &recordRenderer{}
	n.Render(rr)

	if len(rr.fills) == 0 {
		t.Fatal("Render produced no FillRect calls")
	}
}

func TestNumberBoxUpStepSyncsEditBuffer(t *testing.T) {
	n := NewNumberBox(nil).SetRange(0, 100).SetStep(1)
	n.SetValue(10)
	layoutNumberBox(n, render.Rect{X: 0, Y: 0, W: 120, H: 24})

	r := input.NewRouter()
	r.SetRoot(n)
	r.Focus(n)

	r.KeyDown(input.KeyUp, 0, 0)

	if n.Value() != 11 {
		t.Fatalf("Value() = %v, want 11", n.Value())
	}
	if string(n.editRunes) != "11" {
		t.Fatalf("editRunes = %q, want %q", string(n.editRunes), "11")
	}
}

func TestClamp64(t *testing.T) {
	if v := clamp64(5, 0, 10); v != 5 {
		t.Fatalf("clamp64(5,0,10) = %v, want 5", v)
	}
	if v := clamp64(-5, 0, 10); v != 0 {
		t.Fatalf("clamp64(-5,0,10) = %v, want 0", v)
	}
	if v := clamp64(15, 0, 10); v != 10 {
		t.Fatalf("clamp64(15,0,10) = %v, want 10", v)
	}
	if v := clamp64(math.NaN(), 0, 10); v != 0 {
		t.Fatalf("clamp64(NaN,0,10) = %v, want 0", v)
	}
	if v := clamp64(5, 10, 0); v != 10 {
		t.Fatalf("clamp64(5,10,0) degenerate = %v, want 10 (hi < lo collapses to lo)", v)
	}
}
