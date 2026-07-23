package controls

import (
	"testing"
	"time"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/timers"
)

// fakeClip is a minimal input.Clipboard for TextBox interaction tests: a
// single in-memory string, no host/OS involvement (mirrors input_test's own
// fakeClipboard, duplicated here since that one is unexported to its own
// package).
type fakeClip struct{ text string }

func (f *fakeClip) Get() string  { return f.text }
func (f *fakeClip) Set(s string) { f.text = s }

// newFocusedTextBox returns a TextBox with initial text, laid out at a
// generously wide 300x30 rect (so hscroll stays 0 for every short string
// used below, keeping click-x math simple), attached to a real Router as
// root and already focused — the shared setup for every interaction test,
// which per the task brief drives keys via router.KeyDown with the TextBox
// focused.
func newFocusedTextBox(t *testing.T, initial string) (*TextBox, *input.Router) {
	t.Helper()
	tb := NewTextBox(buttonFace(t))
	if initial != "" {
		tb.SetText(initial)
	}
	tb.SetWidth(300)
	tb.SetHeight(30)

	r := input.NewRouter()
	r.SetRoot(tb)
	layoutButton(tb, render.Rect{X: 0, Y: 0, W: 300, H: 30})
	r.Focus(tb)
	return tb, r
}

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

// --- Task 6: keyboard editing ---

func TestTextBoxRuneInsertsAtCaret(t *testing.T) {
	tb, r := newFocusedTextBox(t, "ac")
	tb.SetCaret(1)

	var got []string
	tb.OnChanged(func(s string) { got = append(got, s) })

	r.KeyDown(0, 'b', 0)

	if tb.Text() != "abc" {
		t.Fatalf("Text() = %q, want %q", tb.Text(), "abc")
	}
	if c := tb.Caret(); c != 2 {
		t.Fatalf("Caret() = %d, want 2 (just after inserted rune)", c)
	}
	if len(got) != 1 || got[0] != "abc" {
		t.Fatalf("OnChanged calls = %v, want [%q]", got, "abc")
	}
}

func TestTextBoxTypingWithSelectionReplaces(t *testing.T) {
	tb, r := newFocusedTextBox(t, "hello")
	tb.Select(1, 4) // "ell" selected

	r.KeyDown(0, 'X', 0)

	if tb.Text() != "hXo" {
		t.Fatalf("Text() = %q, want %q (selection replaced by typed rune)", tb.Text(), "hXo")
	}
	if c := tb.Caret(); c != 2 {
		t.Fatalf("Caret() = %d, want 2 (just after the replacement)", c)
	}
	if s, e := tb.Selection(); s != e {
		t.Fatalf("Selection() = (%d,%d), want collapsed after replace", s, e)
	}
}

func TestTextBoxBackspaceDeletesSelectionElseRuneBefore(t *testing.T) {
	tb, r := newFocusedTextBox(t, "hello")
	tb.Select(1, 4) // "ell" selected
	r.KeyDown(input.KeyBackspace, 0, 0)
	if tb.Text() != "ho" {
		t.Fatalf("Backspace with selection: Text() = %q, want %q", tb.Text(), "ho")
	}
	if c := tb.Caret(); c != 1 {
		t.Fatalf("Backspace with selection: Caret() = %d, want 1 (selection start)", c)
	}

	tb2, r2 := newFocusedTextBox(t, "hello")
	tb2.SetCaret(2) // no selection, caret between 'e' and 'l'
	r2.KeyDown(input.KeyBackspace, 0, 0)
	if tb2.Text() != "hllo" {
		t.Fatalf("Backspace no selection: Text() = %q, want %q", tb2.Text(), "hllo")
	}
	if c := tb2.Caret(); c != 1 {
		t.Fatalf("Backspace no selection: Caret() = %d, want 1", c)
	}
}

func TestTextBoxBackspaceAtStartIsNoop(t *testing.T) {
	tb, r := newFocusedTextBox(t, "hi")
	tb.SetCaret(0)
	var calls int
	tb.OnChanged(func(string) { calls++ })
	r.KeyDown(input.KeyBackspace, 0, 0)
	if tb.Text() != "hi" {
		t.Fatalf("Text() = %q, want unchanged %q", tb.Text(), "hi")
	}
	if calls != 0 {
		t.Fatalf("OnChanged calls = %d, want 0 (nothing to delete)", calls)
	}
}

func TestTextBoxDeleteDeletesSelectionElseRuneAfter(t *testing.T) {
	tb, r := newFocusedTextBox(t, "hello")
	tb.Select(1, 4) // "ell" selected
	r.KeyDown(input.KeyDelete, 0, 0)
	if tb.Text() != "ho" {
		t.Fatalf("Delete with selection: Text() = %q, want %q", tb.Text(), "ho")
	}
	if c := tb.Caret(); c != 1 {
		t.Fatalf("Delete with selection: Caret() = %d, want 1", c)
	}

	tb2, r2 := newFocusedTextBox(t, "hello")
	tb2.SetCaret(2) // no selection, caret between 'e' and 'l'
	r2.KeyDown(input.KeyDelete, 0, 0)
	if tb2.Text() != "helo" {
		t.Fatalf("Delete no selection: Text() = %q, want %q", tb2.Text(), "helo")
	}
	if c := tb2.Caret(); c != 2 {
		t.Fatalf("Delete no selection: Caret() = %d, want 2 (unchanged)", c)
	}
}

func TestTextBoxDeleteAtEndIsNoop(t *testing.T) {
	tb, r := newFocusedTextBox(t, "hi")
	tb.SetCaret(2)
	var calls int
	tb.OnChanged(func(string) { calls++ })
	r.KeyDown(input.KeyDelete, 0, 0)
	if tb.Text() != "hi" {
		t.Fatalf("Text() = %q, want unchanged %q", tb.Text(), "hi")
	}
	if calls != 0 {
		t.Fatalf("OnChanged calls = %d, want 0 (nothing to delete)", calls)
	}
}

func TestTextBoxLeftRightMoveCaret(t *testing.T) {
	tb, r := newFocusedTextBox(t, "hello")
	tb.SetCaret(2)

	r.KeyDown(input.KeyLeft, 0, 0)
	if c := tb.Caret(); c != 1 {
		t.Fatalf("Left: Caret() = %d, want 1", c)
	}
	if s, e := tb.Selection(); s != e {
		t.Fatalf("Left (no shift): Selection() = (%d,%d), want collapsed", s, e)
	}

	r.KeyDown(input.KeyRight, 0, 0)
	r.KeyDown(input.KeyRight, 0, 0)
	if c := tb.Caret(); c != 3 {
		t.Fatalf("Right x2: Caret() = %d, want 3", c)
	}
}

func TestTextBoxLeftRightClampAtEdges(t *testing.T) {
	tb, r := newFocusedTextBox(t, "hi")
	tb.SetCaret(0)
	r.KeyDown(input.KeyLeft, 0, 0)
	if c := tb.Caret(); c != 0 {
		t.Fatalf("Left at start: Caret() = %d, want 0 (clamped)", c)
	}

	tb.SetCaret(2)
	r.KeyDown(input.KeyRight, 0, 0)
	if c := tb.Caret(); c != 2 {
		t.Fatalf("Right at end: Caret() = %d, want 2 (clamped)", c)
	}
}

func TestTextBoxShiftLeftRightExtendFromAnchor(t *testing.T) {
	tb, r := newFocusedTextBox(t, "hello world")
	tb.SetCaret(4) // anchor==caret==4

	r.KeyDown(input.KeyRight, 0, input.ModShift)
	r.KeyDown(input.KeyRight, 0, input.ModShift)
	r.KeyDown(input.KeyRight, 0, input.ModShift)
	if s, e := tb.Selection(); s != 4 || e != 7 {
		t.Fatalf("after 3x Shift+Right: Selection() = (%d,%d), want (4,7)", s, e)
	}
	if c := tb.Caret(); c != 7 {
		t.Fatalf("Caret() = %d, want 7", c)
	}

	// Reverse direction: anchor stays pinned at 4, caret walks back past it.
	r.KeyDown(input.KeyLeft, 0, input.ModShift)
	r.KeyDown(input.KeyLeft, 0, input.ModShift)
	r.KeyDown(input.KeyLeft, 0, input.ModShift)
	r.KeyDown(input.KeyLeft, 0, input.ModShift)
	r.KeyDown(input.KeyLeft, 0, input.ModShift)
	if s, e := tb.Selection(); s != 2 || e != 4 {
		t.Fatalf("after reversing past anchor: Selection() = (%d,%d), want (2,4)", s, e)
	}
	if c := tb.Caret(); c != 2 {
		t.Fatalf("Caret() = %d, want 2 (raw caret, left of the normalized anchor)", c)
	}
}

func TestTextBoxHomeEndMoveCaret(t *testing.T) {
	tb, r := newFocusedTextBox(t, "hello world")
	tb.SetCaret(4)

	r.KeyDown(input.KeyHome, 0, 0)
	if c := tb.Caret(); c != 0 {
		t.Fatalf("Home: Caret() = %d, want 0", c)
	}
	if s, e := tb.Selection(); s != e {
		t.Fatalf("Home (no shift): Selection() = (%d,%d), want collapsed", s, e)
	}

	r.KeyDown(input.KeyEnd, 0, 0)
	want := len([]rune("hello world"))
	if c := tb.Caret(); c != want {
		t.Fatalf("End: Caret() = %d, want %d", c, want)
	}
}

func TestTextBoxHomeEndShiftExtends(t *testing.T) {
	tb, r := newFocusedTextBox(t, "hello world")
	tb.SetCaret(4)

	r.KeyDown(input.KeyHome, 0, input.ModShift)
	if s, e := tb.Selection(); s != 0 || e != 4 {
		t.Fatalf("Shift+Home: Selection() = (%d,%d), want (0,4)", s, e)
	}

	tb.SetCaret(4)
	r.KeyDown(input.KeyEnd, 0, input.ModShift)
	want := len([]rune("hello world"))
	if s, e := tb.Selection(); s != 4 || e != want {
		t.Fatalf("Shift+End: Selection() = (%d,%d), want (4,%d)", s, e, want)
	}
}

func TestTextBoxCtrlASelectsAll(t *testing.T) {
	tb, r := newFocusedTextBox(t, "hello world")
	tb.SetCaret(3)

	r.KeyDown(input.KeyA, 0, input.ModCtrl)

	want := len([]rune("hello world"))
	if s, e := tb.Selection(); s != 0 || e != want {
		t.Fatalf("Ctrl+A: Selection() = (%d,%d), want (0,%d)", s, e, want)
	}
}

func TestTextBoxCtrlCCopiesSelection(t *testing.T) {
	tb, r := newFocusedTextBox(t, "hello world")
	clip := &fakeClip{}
	r.SetClipboard(clip)
	tb.Select(2, 7) // "llo w"

	r.KeyDown(input.KeyC, 0, input.ModCtrl)

	if clip.text != "llo w" {
		t.Fatalf("clipboard text = %q, want %q", clip.text, "llo w")
	}
	// Copy must not mutate the text.
	if tb.Text() != "hello world" {
		t.Fatalf("Text() after copy = %q, want unchanged", tb.Text())
	}
}

func TestTextBoxCtrlCNoopWithoutSelection(t *testing.T) {
	tb, r := newFocusedTextBox(t, "hello")
	clip := &fakeClip{text: "untouched"}
	r.SetClipboard(clip)
	tb.SetCaret(2) // no selection

	r.KeyDown(input.KeyC, 0, input.ModCtrl)

	if clip.text != "untouched" {
		t.Fatalf("clipboard text = %q, want unchanged (no selection to copy)", clip.text)
	}
}

func TestTextBoxCtrlXCopiesAndDeletes(t *testing.T) {
	tb, r := newFocusedTextBox(t, "hello world")
	clip := &fakeClip{}
	r.SetClipboard(clip)
	tb.Select(2, 7) // "llo w"

	r.KeyDown(input.KeyX, 0, input.ModCtrl)

	if clip.text != "llo w" {
		t.Fatalf("clipboard text = %q, want %q", clip.text, "llo w")
	}
	if tb.Text() != "heorld" {
		t.Fatalf("Text() after cut = %q, want %q", tb.Text(), "heorld")
	}
	if c := tb.Caret(); c != 2 {
		t.Fatalf("Caret() after cut = %d, want 2", c)
	}
}

func TestTextBoxCtrlVPastesReplacingSelectionAndStripsCRLF(t *testing.T) {
	tb, r := newFocusedTextBox(t, "hello world")
	clip := &fakeClip{text: "X\r\nY\nZ"}
	r.SetClipboard(clip)
	tb.Select(2, 7) // "llo w"

	r.KeyDown(input.KeyV, 0, input.ModCtrl)

	if tb.Text() != "heXYZorld" {
		t.Fatalf("Text() after paste = %q, want %q (CRLF stripped)", tb.Text(), "heXYZorld")
	}
	if c := tb.Caret(); c != len([]rune("heXYZ")) {
		t.Fatalf("Caret() after paste = %d, want %d (just after pasted text)", c, len([]rune("heXYZ")))
	}
}

func TestTextBoxNilClipboardPathsNoop(t *testing.T) {
	tb, r := newFocusedTextBox(t, "hello")
	tb.Select(0, 3) // clipboard is nil on a fresh router (headless default)
	if r.Clipboard() != nil {
		t.Fatal("test setup: expected nil clipboard on a fresh router")
	}

	var calls int
	tb.OnChanged(func(string) { calls++ })

	r.KeyDown(input.KeyC, 0, input.ModCtrl)
	r.KeyDown(input.KeyX, 0, input.ModCtrl)
	r.KeyDown(input.KeyV, 0, input.ModCtrl)

	if tb.Text() != "hello" {
		t.Fatalf("Text() = %q, want unchanged %q (nil clipboard: all three no-op)", tb.Text(), "hello")
	}
	if calls != 0 {
		t.Fatalf("OnChanged calls = %d, want 0", calls)
	}
}

func TestTextBoxDisabledIgnoresAllKeys(t *testing.T) {
	tb := NewTextBox(buttonFace(t))
	tb.SetText("hi")
	tb.SetEnabled(false)
	tb.OnFocusChanged(true) // simulate focused state directly (disabled never gets focused via a real router)

	r := input.NewRouter()

	cases := []input.KeyEvent{
		{Action: input.Press, Rune: 'x', Router: r},
		{Action: input.Press, Key: input.KeyBackspace, Router: r},
		{Action: input.Press, Key: input.KeyDelete, Router: r},
		{Action: input.Press, Key: input.KeyLeft, Router: r},
		{Action: input.Press, Key: input.KeyA, Mods: input.ModCtrl, Router: r},
	}
	for i := range cases {
		e := cases[i]
		tb.OnKey(&e)
		if e.Handled {
			t.Fatalf("case %d: Handled = true on a disabled TextBox, want false (ignored)", i)
		}
	}
	if tb.Text() != "hi" {
		t.Fatalf("Text() = %q, want unchanged %q", tb.Text(), "hi")
	}
}

func TestTextBoxUnfocusedIgnoresKeys(t *testing.T) {
	tb := NewTextBox(buttonFace(t))
	tb.SetText("hi")
	// Never focused: t.focused is false.

	e := &input.KeyEvent{Action: input.Press, Rune: 'x', Router: input.NewRouter()}
	tb.OnKey(e)
	if e.Handled {
		t.Fatal("Handled = true on an unfocused TextBox, want false")
	}
	if tb.Text() != "hi" {
		t.Fatalf("Text() = %q, want unchanged", tb.Text())
	}
}

func TestTextBoxHandledSetOnEveryRecognizedKey(t *testing.T) {
	r := input.NewRouter() // no clipboard wired: exercises the no-op-but-Handled paths for C/X/V too
	tb, _ := newFocusedTextBox(t, "hello")
	r.SetRoot(tb)
	r.Focus(tb)

	cases := []input.KeyEvent{
		{Action: input.Press, Rune: 'q', Router: r},
		{Action: input.Press, Key: input.KeyBackspace, Router: r},
		{Action: input.Press, Key: input.KeyDelete, Router: r},
		{Action: input.Press, Key: input.KeyLeft, Router: r},
		{Action: input.Press, Key: input.KeyRight, Router: r},
		{Action: input.Press, Key: input.KeyHome, Router: r},
		{Action: input.Press, Key: input.KeyEnd, Router: r},
		{Action: input.Press, Key: input.KeyA, Mods: input.ModCtrl, Router: r},
		{Action: input.Press, Key: input.KeyC, Mods: input.ModCtrl, Router: r},
		{Action: input.Press, Key: input.KeyX, Mods: input.ModCtrl, Router: r},
		{Action: input.Press, Key: input.KeyV, Mods: input.ModCtrl, Router: r},
	}
	for i := range cases {
		e := cases[i]
		tb.OnKey(&e)
		if !e.Handled {
			t.Fatalf("case %d (key=%v mods=%v rune=%q): Handled = false, want true", i, e.Key, e.Mods, e.Rune)
		}
	}
}

func TestTextBoxCtrlLetterDoesNotAlsoInsertRune(t *testing.T) {
	// A bare Ctrl+A must select-all, not select-all AND insert 'a' as text —
	// e.Rune may legitimately be non-zero alongside Ctrl on some platforms.
	tb, r := newFocusedTextBox(t, "hi")
	r.KeyDown(input.KeyA, 'a', input.ModCtrl)
	if tb.Text() != "hi" {
		t.Fatalf("Text() = %q, want unchanged %q (Ctrl+A must not insert)", tb.Text(), "hi")
	}
}

// --- Task 6: mouse ---

func TestTextBoxPressSetsCaretAtNearestGlyphBoundary(t *testing.T) {
	tb, r := newFocusedTextBox(t, "abc")
	bounds := tb.Bounds()
	pad := tb.metrics.PaddingM
	y := bounds.Y + bounds.H/2

	toWindowX := func(localX float32) float32 { return bounds.X + pad - tb.hscroll + localX }

	n := len([]rune("abc"))
	for i := 0; i < n; i++ {
		mid := (tb.xOf(i) + tb.xOf(i+1)) / 2

		leftX := toWindowX(mid - 0.05)
		r.PointerButton(input.ButtonLeft, true, render.Point{X: leftX, Y: y}, 0)
		if c := tb.Caret(); c != i {
			t.Fatalf("click just left of midpoint(%d,%d) at x=%v: Caret() = %d, want %d", i, i+1, leftX, c, i)
		}
		r.PointerButton(input.ButtonLeft, false, render.Point{X: leftX, Y: y}, 0)

		rightX := toWindowX(mid + 0.05)
		r.PointerButton(input.ButtonLeft, true, render.Point{X: rightX, Y: y}, 0)
		if c := tb.Caret(); c != i+1 {
			t.Fatalf("click just right of midpoint(%d,%d) at x=%v: Caret() = %d, want %d", i, i+1, rightX, c, i+1)
		}
		r.PointerButton(input.ButtonLeft, false, render.Point{X: rightX, Y: y}, 0)
	}
}

func TestTextBoxPressClearsSelectionAndCaptures(t *testing.T) {
	tb, r := newFocusedTextBox(t, "hello")
	tb.Select(0, 5)

	bounds := tb.Bounds()
	pad := tb.metrics.PaddingM
	pos := render.Point{X: bounds.X + pad + tb.xOf(2), Y: bounds.Y + bounds.H/2}

	r.PointerButton(input.ButtonLeft, true, pos, 0)

	if s, e := tb.Selection(); s != e {
		t.Fatalf("Selection() after Press = (%d,%d), want collapsed", s, e)
	}
	if got := r.Captured(); got != core.Widget(tb) {
		t.Fatalf("Captured() after Press = %v, want the TextBox", got)
	}
}

func TestTextBoxDragCapturedExtendsSelection(t *testing.T) {
	tb, r := newFocusedTextBox(t, "hello world")
	bounds := tb.Bounds()
	pad := tb.metrics.PaddingM
	y := bounds.Y + bounds.H/2
	toWindowX := func(idx int) float32 { return bounds.X + pad - tb.hscroll + tb.xOf(idx) }

	r.PointerButton(input.ButtonLeft, true, render.Point{X: toWindowX(2), Y: y}, 0)
	r.PointerMove(render.Point{X: toWindowX(7), Y: y}, 0)

	if s, e := tb.Selection(); s != 2 || e != 7 {
		t.Fatalf("mid-drag Selection() = (%d,%d), want (2,7)", s, e)
	}
	if c := tb.Caret(); c != 7 {
		t.Fatalf("mid-drag Caret() = %d, want 7", c)
	}

	// Extend further, still anchored at 2.
	r.PointerMove(render.Point{X: toWindowX(4), Y: y}, 0)
	if s, e := tb.Selection(); s != 2 || e != 4 {
		t.Fatalf("after retreat Selection() = (%d,%d), want (2,4) (anchor pinned at press point)", s, e)
	}

	r.PointerButton(input.ButtonLeft, false, render.Point{X: toWindowX(4), Y: y}, 0)
	if got := r.Captured(); got != nil {
		t.Fatalf("Captured() after Release = %v, want nil", got)
	}
	// Release must not itself move the caret/selection.
	if s, e := tb.Selection(); s != 2 || e != 4 {
		t.Fatalf("Selection() after Release = (%d,%d), want unchanged (2,4)", s, e)
	}
}

func TestTextBoxDisabledIgnoresPointer(t *testing.T) {
	tb := NewTextBox(buttonFace(t)).SetEnabled(false)
	tb.SetText("hello")
	tb.SetWidth(300)
	tb.SetHeight(30)
	layoutButton(tb, render.Rect{X: 0, Y: 0, W: 300, H: 30})

	e := &input.PointerEvent{Action: input.Press, Pos: render.Point{X: 10, Y: 15}, Router: input.NewRouter()}
	tb.OnPointer(e)
	if e.Handled {
		t.Fatal("Press on a disabled TextBox set Handled = true, want false")
	}
}

func TestTextBoxCursorIBeamViaCursorShaper(t *testing.T) {
	tb := NewTextBox(nil)
	if c := tb.Cursor(); c != input.CursorIBeam {
		t.Fatalf("Cursor() = %v, want CursorIBeam", c)
	}

	tb2, r := newFocusedTextBox(t, "hi")
	bounds := tb2.Bounds()
	if c := r.PointerMove(render.Point{X: bounds.X + 2, Y: bounds.Y + 2}, 0); c != input.CursorIBeam {
		t.Fatalf("PointerMove over TextBox cursor = %v, want CursorIBeam", c)
	}
}

// --- Task 6 carry-in fixes ---

func TestTextBoxMutatorsMarkArrangeDirty(t *testing.T) {
	tb, r := newFocusedTextBox(t, "hello")
	if tb.NeedsLayout() {
		t.Fatal("NeedsLayout() = true right after setup's layout pass, want false (clean)")
	}

	tb.SetCaret(2)
	if !tb.NeedsLayout() {
		t.Fatal("NeedsLayout() = false after SetCaret, want true (carry-in fix A)")
	}
	layoutButton(tb, tb.Bounds()) // re-clean

	tb.Select(0, 3)
	if !tb.NeedsLayout() {
		t.Fatal("NeedsLayout() = false after Select, want true (carry-in fix A)")
	}
	layoutButton(tb, tb.Bounds())

	r.KeyDown(0, 'x', 0) // rune insert, a full editing mutation via replaceRange
	if !tb.NeedsLayout() {
		t.Fatal("NeedsLayout() = false after a typed rune, want true (carry-in fix A)")
	}
}

func TestTextBoxSetTextEqualIsCompleteNoop(t *testing.T) {
	tb, _ := newFocusedTextBox(t, "hello")
	tb.SetCaret(2)
	layoutButton(tb, tb.Bounds()) // re-clean after the SetCaret above

	var calls int
	tb.OnChanged(func(string) { calls++ })

	tb.SetText("hello") // equals current text

	if calls != 0 {
		t.Fatalf("OnChanged calls = %d, want 0 (carry-in fix B: equal SetText is a no-op)", calls)
	}
	if tb.NeedsLayout() {
		t.Fatal("NeedsLayout() = true after equal SetText, want false (no invalidation)")
	}
	if c := tb.Caret(); c != 2 {
		t.Fatalf("Caret() = %d, want 2 (untouched by the no-op SetText)", c)
	}
}
