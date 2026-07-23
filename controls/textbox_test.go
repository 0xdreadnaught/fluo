package controls

import (
	"testing"
	"time"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/timers"
)

func TestTextBoxSetTextResetsCaretAndClearsSelection(t *testing.T) {
	tb := NewTextBox(nil)
	tb.SetText("hello")
	tb.Select(1, 3)
	if s, e := tb.Selection(); s != 1 || e != 3 {
		t.Fatalf("Selection() before SetText = (%d,%d), want (1,3)", s, e)
	}

	tb.SetText("hi there")
	if got := tb.Text(); got != "hi there" {
		t.Fatalf("Text() = %q, want %q", got, "hi there")
	}
	want := len([]rune("hi there"))
	if c := tb.Caret(); c != want {
		t.Fatalf("Caret() after SetText = %d, want %d (end)", c, want)
	}
	if s, e := tb.Selection(); s != want || e != want {
		t.Fatalf("Selection() after SetText = (%d,%d), want (%d,%d) (cleared)", s, e, want, want)
	}
}

func TestTextBoxSetTextFiresOnChanged(t *testing.T) {
	tb := NewTextBox(nil)
	var got []string
	tb.OnChanged(func(s string) { got = append(got, s) })

	tb.SetText("hi")
	if len(got) != 1 || got[0] != "hi" {
		t.Fatalf("OnChanged calls = %v, want [%q] (SetText fires OnChanged, programmatic parity with typing)", got, "hi")
	}
}

func TestTextBoxSetCaretClampsAndClearsSelection(t *testing.T) {
	tb := NewTextBox(nil)
	tb.SetText("hello") // caret=5, len=5
	tb.Select(0, 3)

	tb.SetCaret(-5)
	if c := tb.Caret(); c != 0 {
		t.Fatalf("SetCaret(-5): Caret() = %d, want 0 (clamped)", c)
	}
	if s, e := tb.Selection(); s != 0 || e != 0 {
		t.Fatalf("SetCaret(-5): Selection() = (%d,%d), want (0,0) (cleared)", s, e)
	}

	tb.Select(0, 3)
	tb.SetCaret(1000)
	if c := tb.Caret(); c != 5 {
		t.Fatalf("SetCaret(1000): Caret() = %d, want 5 (clamped to len)", c)
	}
	if s, e := tb.Selection(); s != 5 || e != 5 {
		t.Fatalf("SetCaret(1000): Selection() = (%d,%d), want (5,5) (cleared)", s, e)
	}

	tb.Select(0, 3)
	tb.SetCaret(2)
	if c := tb.Caret(); c != 2 {
		t.Fatalf("SetCaret(2): Caret() = %d, want 2", c)
	}
	if s, e := tb.Selection(); s != 2 || e != 2 {
		t.Fatalf("SetCaret(2): Selection() = (%d,%d), want (2,2) (cleared)", s, e)
	}
}

func TestTextBoxSelectNormalizes(t *testing.T) {
	tb := NewTextBox(nil)
	tb.SetText("hello world")

	tb.Select(2, 7)
	if s, e := tb.Selection(); s != 2 || e != 7 {
		t.Fatalf("Select(2,7): Selection() = (%d,%d), want (2,7)", s, e)
	}
	if c := tb.Caret(); c != 7 {
		t.Fatalf("Select(2,7): Caret() = %d, want 7 (the caret arg)", c)
	}

	// Reversed: anchor > caret must still normalize start<=end in Selection(),
	// while Caret() reports the raw caret argument (2), not the normalized
	// start.
	tb.Select(7, 2)
	if s, e := tb.Selection(); s != 2 || e != 7 {
		t.Fatalf("Select(7,2): Selection() = (%d,%d), want (2,7) (normalized)", s, e)
	}
	if c := tb.Caret(); c != 2 {
		t.Fatalf("Select(7,2): Caret() = %d, want 2 (the caret arg)", c)
	}
}

func TestTextBoxSelectClampsOutOfRange(t *testing.T) {
	tb := NewTextBox(nil)
	tb.SetText("hi") // len 2

	tb.Select(-3, 999)
	if s, e := tb.Selection(); s != 0 || e != 2 {
		t.Fatalf("Select(-3,999): Selection() = (%d,%d), want (0,2) (both clamped)", s, e)
	}
}

func TestTextBoxXOfMatchesFaceMeasurePrefixMultibyte(t *testing.T) {
	face := buttonFace(t)
	tb := NewTextBox(face)
	s := "héllo" // é is multibyte in UTF-8; runes must be indexed correctly
	tb.SetText(s)

	runes := []rune(s)
	if len(runes) != 5 {
		t.Fatalf("test setup: len(runes) = %d, want 5", len(runes))
	}
	for i := 0; i <= len(runes); i++ {
		want := face.Measure(string(runes[:i])).W
		got := tb.xOf(i)
		if got != want {
			t.Fatalf("xOf(%d) = %v, want %v (face.Measure prefix)", i, got, want)
		}
	}
}

func TestTextBoxHScrollKeepsCaretVisible(t *testing.T) {
	tb := NewTextBox(buttonFace(t))
	tb.SetText("this is a much longer line of text than the box") // caret ends at len (end)
	tb.SetWidth(50)                                               // narrow explicit width

	core.MeasureWidget(tb, render.Size{W: 50, H: 30})
	core.ArrangeWidget(tb, render.Rect{X: 0, Y: 0, W: 50, H: 30})

	if tb.hscroll <= 0 {
		t.Fatalf("hscroll = %v, want > 0 (narrow box, caret at end of long text)", tb.hscroll)
	}

	pad := tb.metrics.PaddingM
	innerW := tb.Bounds().W - 2*pad
	caretX := tb.xOf(tb.Caret()) - tb.hscroll
	const eps = 0.01
	if caretX < -eps || caretX > innerW+eps {
		t.Fatalf("caret display x = %v, want within [0, %v] (inner width)", caretX, innerW)
	}
}

func TestTextBoxPlaceholderShowsWhenEmptyRegardlessOfFocus(t *testing.T) {
	tb := NewTextBox(nil).SetPlaceholder("Enter name")

	s, c := tb.displayText()
	if s != "Enter name" || c != tb.colors.TextDisabled {
		t.Fatalf("unfocused empty: displayText() = (%q,%v), want (%q,TextDisabled)", s, c, "Enter name")
	}

	tb.OnFocusChanged(true)
	s, c = tb.displayText()
	if s != "Enter name" || c != tb.colors.TextDisabled {
		t.Fatalf("focused empty: displayText() = (%q,%v), want (%q,TextDisabled) (placeholder ignores focus)", s, c, "Enter name")
	}

	tb.SetText("x")
	s, c = tb.displayText()
	if s != "x" || c != tb.colors.TextPrimary {
		t.Fatalf("non-empty: displayText() = (%q,%v), want (%q,TextPrimary)", s, c, "x")
	}

	tb.SetEnabled(false)
	s, c = tb.displayText()
	if s != "x" || c != tb.colors.TextDisabled {
		t.Fatalf("non-empty disabled: displayText() = (%q,%v), want (%q,TextDisabled)", s, c, "x")
	}
}

func TestTextBoxCaretSolidWithoutTimers(t *testing.T) {
	tb := NewTextBox(nil)
	if tb.caretShown() {
		t.Fatal("caretShown() = true while unfocused, want false")
	}
	tb.OnFocusChanged(true)
	if !tb.caretShown() {
		t.Fatal("caretShown() = false while focused with no timers wired, want true (solid caret)")
	}
}

func TestTextBoxCaretBlinkTogglesWithTimers(t *testing.T) {
	start := time.Now()
	q := timers.NewQueue(start)
	tb := NewTextBox(nil)
	tb.OnFocusChanged(true)
	tb.SetTimers(q)

	if !tb.caretVisible {
		t.Fatal("caretVisible = false immediately after SetTimers, want true (starts visible)")
	}
	if !tb.caretShown() {
		t.Fatal("caretShown() = false right after SetTimers, want true")
	}

	q.Advance(start.Add(530 * time.Millisecond))
	if tb.caretVisible {
		t.Fatal("caretVisible = true after one 530ms Advance, want false (toggled off)")
	}

	q.Advance(start.Add(1060 * time.Millisecond))
	if !tb.caretVisible {
		t.Fatal("caretVisible = false after two 530ms Advances, want true (toggled back on)")
	}
}

func TestTextBoxSetTimersNilRestoresSolidCaret(t *testing.T) {
	start := time.Now()
	q := timers.NewQueue(start)
	tb := NewTextBox(nil)
	tb.OnFocusChanged(true)
	tb.SetTimers(q)
	q.Advance(start.Add(530 * time.Millisecond))
	if tb.caretVisible {
		t.Fatal("test setup: expected caretVisible false after blink toggle")
	}

	tb.SetTimers(nil)
	if !tb.caretShown() {
		t.Fatal("caretShown() = false after SetTimers(nil), want true (solid caret restored)")
	}

	// The old timer must no longer be able to affect this textbox.
	q.Advance(start.Add(2000 * time.Millisecond))
	if !tb.caretShown() {
		t.Fatal("caretShown() = false after advancing the old (detached) queue, want true")
	}
}

func TestTextBoxDesiredSizeDefaultWidth(t *testing.T) {
	tb := NewTextBox(buttonFace(t))
	core.MeasureWidget(tb, render.Size{W: 1000, H: 1000})
	d := core.DesiredSizeOf(tb)
	if d.W != 160 {
		t.Fatalf("DesiredSize().W = %v, want 160 (default)", d.W)
	}
	wantH := buttonFace(t).LineHeight() + 2*tb.metrics.PaddingM
	if d.H != wantH {
		t.Fatalf("DesiredSize().H = %v, want %v (lineHeight + 2*PaddingM)", d.H, wantH)
	}
}

func TestTextBoxSetEnabledAffectsAcceptsFocus(t *testing.T) {
	tb := NewTextBox(nil)
	if !tb.AcceptsFocus() {
		t.Fatal("AcceptsFocus() = false for a fresh enabled TextBox, want true")
	}
	tb.SetEnabled(false)
	if tb.AcceptsFocus() {
		t.Fatal("AcceptsFocus() = true after SetEnabled(false), want false")
	}
}
