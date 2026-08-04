package controls

import (
	"math"
	"strings"
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

// TestTextBoxSetTextIsSilent is the Phase 5 final-fix regression for the
// uniform silent-setter convention (Important #3, controller decision
// option A): SetText joins CheckBox/ToggleSwitch/ToggleButton/ComboBox/
// Slider in never firing OnChanged from a programmatic setter — even when
// it produces a REAL change to Text(). Only user-driven edits (typing,
// Backspace/Delete, Ctrl+X, Ctrl+V — see TestTextBoxRuneInsertsAtCaret for
// the fires-on-user-input half of this contract) notify.
func TestTextBoxSetTextIsSilent(t *testing.T) {
	tb := NewTextBox(nil)
	var got []string
	tb.OnChanged(func(s string) { got = append(got, s) })

	tb.SetText("hi")
	if len(got) != 0 {
		t.Fatalf("OnChanged calls = %v, want none (SetText is a silent programmatic setter)", got)
	}
	if tb.Text() != "hi" {
		t.Fatalf("Text() = %q, want %q (SetText still mutates, just silently)", tb.Text(), "hi")
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

// oldCaretIndexAtX is the pre-running-pen O(n²) hit-test: for each rune
// boundary it recomputes xOf(i) = Measure(runes[:i]) from scratch and compares
// x against the midpoint (xOf(i)+xOf(i+1))/2. Kept in the test as the exact
// reference the O(n) caretIndexInRunes must reproduce byte-for-byte.
func oldCaretIndexAtX(tb *TextBox, x float32) int {
	n := len(tb.runes)
	for i := 0; i < n; i++ {
		mid := (tb.xOf(i) + tb.xOf(i+1)) / 2
		if x < mid {
			return i
		}
	}
	return n
}

// TestTextBoxCaretIndexRunningPenMatchesPrefixMeasure pins the P3 speedup:
// the O(n) running-pen hit-test must return the identical caret index as the
// old O(n²) prefix-Measure scan for every x — including x on and around each
// glyph midpoint, on a kerned/mixed multibyte string — and index->x->index
// must round-trip. Covers the single-line path (caretIndexAtX) and, via
// colAtX, the per-line path with the same helper.
func TestTextBoxCaretIndexRunningPenMatchesPrefixMeasure(t *testing.T) {
	face := buttonFace(t)
	for _, s := range []string{"AVAWaToVoTaWATATaYoP.", "héllo wörld", "The quick brown fox", "iiiWWWmmm"} {
		tb := NewTextBox(face)
		tb.SetText(s)
		runes := []rune(s)

		// Full x-domain: negatives (left of text), 0, every boundary and each
		// boundary's exact midpoint and a hair either side, past the end.
		var xs []float32
		xs = append(xs, -100, -1, -0.001, 0)
		for i := 0; i <= len(runes); i++ {
			xi := tb.xOf(i)
			xs = append(xs, xi-0.001, xi, xi+0.001)
			if i < len(runes) {
				mid := (tb.xOf(i) + tb.xOf(i+1)) / 2
				xs = append(xs, mid-0.001, mid, mid+0.001)
			}
		}
		xs = append(xs, tb.xOf(len(runes))+100)

		for _, x := range xs {
			want := oldCaretIndexAtX(tb, x)
			if got := tb.caretIndexAtX(x); got != want {
				t.Fatalf("caretIndexAtX(%v) on %q = %d, want %d (old prefix-Measure)", x, s, got, want)
			}
			// colAtX over the single logical line (line 0) shares the helper
			// and must agree with the whole-buffer scan for a newline-free string.
			if got := tb.colAtX(0, x); got != want {
				t.Fatalf("colAtX(0,%v) on %q = %d, want %d", x, s, got, want)
			}
		}

		// index -> x -> index round-trips: clicking exactly at a boundary's x
		// resolves back to that boundary (its own midpoint region).
		for i := 0; i <= len(runes); i++ {
			if got := tb.caretIndexAtX(tb.xOf(i)); got != i {
				t.Fatalf("round-trip on %q: caretIndexAtX(xOf(%d)=%v) = %d, want %d", s, i, tb.xOf(i), got, i)
			}
		}
	}
}

// TestTextBoxCaretIndexAtXNilFace locks the nil-face hit-test outcome, which
// the running pen short-circuits rather than walking: with no face there are
// no glyphs to click between, xOf reports 0 for every boundary, so every
// midpoint is 0 — a click left of the text lands at 0 and anything from 0
// rightward falls through to the end.
func TestTextBoxCaretIndexAtXNilFace(t *testing.T) {
	tb := NewTextBox(nil)
	tb.SetText("hello")
	n := len(tb.runes)
	for _, tc := range []struct {
		x    float32
		want int
	}{{-100, 0}, {-0.001, 0}, {0, n}, {0.001, n}, {500, n}} {
		if got := tb.caretIndexAtX(tc.x); got != tc.want {
			t.Errorf("nil face: caretIndexAtX(%v) = %d, want %d", tc.x, got, tc.want)
		}
	}

	empty := NewTextBox(nil)
	if got := empty.caretIndexAtX(-1); got != 0 {
		t.Errorf("nil face, empty text: caretIndexAtX(-1) = %d, want 0", got)
	}
}

// TestTextBoxMultilineCullsOffscreenLines pins the P4 speedup: a tall
// multi-line box scrolled so most lines sit outside its bounds must draw only
// the lines whose row actually intersects the viewport. With the whole text
// selected, every drawn line emits one Highlight FillRect, so counting those
// (by color) reveals exactly which lines were drawn — the off-screen ones
// must be skipped, and no highlight may land outside the clip bounds.
func TestTextBoxMultilineCullsOffscreenLines(t *testing.T) {
	tb := NewTextBox(buttonFace(t))
	tb.SetMultiline(true)

	const total = 50
	var sb strings.Builder
	for i := 0; i < total; i++ {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString("wxyz") // non-empty so a fully-selected line highlights
	}
	tb.SetText(sb.String())
	tb.SetWidth(200)
	tb.SetHeight(60)
	layoutButton(tb, render.Rect{X: 0, Y: 0, W: 200, H: 60})

	// Select everything so each drawn line paints its highlight, then scroll
	// well down so the top lines fall off the top edge and the bottom lines
	// off the bottom — leaving only a middle band on screen.
	tb.Select(0, len([]rune(tb.Text())))
	lh := tb.lineHeight()
	if lh <= 0 {
		t.Fatalf("test setup: lineHeight = %v, want > 0", lh)
	}
	tb.vscroll = 15 * lh

	rr := &recordRenderer{}
	core.RenderWidget(tb, rr)

	bounds := tb.Bounds()
	pad := tb.metrics.PaddingM
	viewTop, viewBot := bounds.Y, bounds.Y+bounds.H

	// Independently compute which line indices the row intersects the viewport.
	wantVisible := 0
	for i := 0; i < total; i++ {
		lineY := bounds.Y + pad - tb.vscroll + float32(i)*lh
		if lineY+lh > viewTop && lineY < viewBot {
			wantVisible++
		}
	}
	if wantVisible == 0 || wantVisible >= total {
		t.Fatalf("test setup: wantVisible = %d, want a small nonzero band (< %d)", wantVisible, total)
	}

	got := 0
	for _, f := range rr.fills {
		if f.color != tb.colors.Highlight {
			continue
		}
		got++
		if f.rect.Y+lh <= viewTop || f.rect.Y >= viewBot {
			t.Errorf("highlight fill drawn off-screen at Y=%v (viewport %v..%v)", f.rect.Y, viewTop, viewBot)
		}
	}
	if got != wantVisible {
		t.Fatalf("drew %d highlighted lines, want exactly %d visible (off-screen lines not culled)", got, wantVisible)
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
	if s != "Enter name" || c != tb.colors.GrayText {
		t.Fatalf("unfocused empty: displayText() = (%q,%v), want (%q,GrayText)", s, c, "Enter name")
	}

	tb.OnFocusChanged(true)
	s, c = tb.displayText()
	if s != "Enter name" || c != tb.colors.GrayText {
		t.Fatalf("focused empty: displayText() = (%q,%v), want (%q,GrayText) (placeholder ignores focus)", s, c, "Enter name")
	}

	tb.SetText("x")
	s, c = tb.displayText()
	if s != "x" || c != tb.colors.WindowText {
		t.Fatalf("non-empty: displayText() = (%q,%v), want (%q,WindowText)", s, c, "x")
	}

	tb.SetEnabled(false)
	s, c = tb.displayText()
	if s != "x" || c != tb.colors.GrayText {
		t.Fatalf("non-empty disabled: displayText() = (%q,%v), want (%q,GrayText)", s, c, "x")
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

// The blink timer must exist only while the box is focused. A repeating
// timer's closure retains the TextBox and only OnFocusChanged(false) stops
// it, so a never-focused box that schedules one holds it for the queue's
// whole lifetime — one stranded timer per TextBox per tree rebuild, each of
// which Queue.Advance re-sorts on every fire.
func TestTextBoxBlinkTimerOnlyWhileFocused(t *testing.T) {
	start := time.Now()
	q := timers.NewQueue(start)
	tb := NewTextBox(nil)
	tb.SetTimers(q)

	if tb.blinkTimer != nil {
		t.Fatal("blinkTimer non-nil after SetTimers on an unfocused box, want nil")
	}
	if got := q.Len(); got != 0 {
		t.Fatalf("q.Len() after SetTimers on an unfocused box = %d, want 0", got)
	}

	// Caret-affecting mutations call restartBlink too — the common rebuild
	// path — and must not schedule anything either while unfocused.
	tb.SetText("hello")
	tb.SetCaret(2)
	if got := q.Len(); got != 0 {
		t.Fatalf("q.Len() after unfocused SetText/SetCaret = %d, want 0", got)
	}

	// Nothing pending means nothing to toggle the caret.
	q.Advance(start.Add(2 * caretBlinkPeriod))
	if !tb.caretVisible {
		t.Fatal("caretVisible = false on an unfocused box after two blink periods, want true (no timer running)")
	}

	// Focus starts it.
	tb.OnFocusChanged(true)
	if tb.blinkTimer == nil {
		t.Fatal("blinkTimer nil after gaining focus, want a running timer")
	}
	if got := q.Len(); got != 1 {
		t.Fatalf("q.Len() after gaining focus = %d, want 1", got)
	}
	q.Advance(start.Add(3 * caretBlinkPeriod))
	if tb.caretVisible {
		t.Fatal("caretVisible = true after a blink period while focused, want false (timer running)")
	}

	// Blur stops it again.
	tb.OnFocusChanged(false)
	if tb.blinkTimer != nil {
		t.Fatal("blinkTimer non-nil after losing focus, want nil")
	}
	if got := q.Len(); got != 0 {
		t.Fatalf("q.Len() after losing focus = %d, want 0", got)
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

// TestTextBoxLeftRightCollapsesSelectionToEdgeWPFParity is a Phase 5
// final-fix triage fold: an unshifted Left/Right press with an ACTIVE
// selection collapses the caret straight to that selection's start/end
// (standard desktop-text-box, WPF-parity convention), rather than moving
// delta runes from the raw caret position (which would additionally step
// past the selection's edge on the very first press).
func TestTextBoxLeftRightCollapsesSelectionToEdgeWPFParity(t *testing.T) {
	tb, r := newFocusedTextBox(t, "hello world")

	tb.Select(2, 7) // caret=7 (raw), normalized selection (2,7)
	r.KeyDown(input.KeyLeft, 0, 0)
	if c := tb.Caret(); c != 2 {
		t.Fatalf("Left with selection (2,7): Caret() = %d, want 2 (selection start, not 6)", c)
	}
	if s, e := tb.Selection(); s != e {
		t.Fatalf("Left with selection: Selection() = (%d,%d), want collapsed", s, e)
	}

	tb.Select(2, 7)
	r.KeyDown(input.KeyRight, 0, 0)
	if c := tb.Caret(); c != 7 {
		t.Fatalf("Right with selection (2,7): Caret() = %d, want 7 (selection end, not 8)", c)
	}
	if s, e := tb.Selection(); s != e {
		t.Fatalf("Right with selection: Selection() = (%d,%d), want collapsed", s, e)
	}

	// Once collapsed, a further unshifted press moves by 1 as usual.
	r.KeyDown(input.KeyLeft, 0, 0)
	if c := tb.Caret(); c != 6 {
		t.Fatalf("Left after collapse: Caret() = %d, want 6 (plain -1 move)", c)
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

// TestTextBoxSetEnabledFalseMidDragReleasesCapture is the Phase 5 final-fix
// regression for the mid-drag disable wedge (Important #2, joint with
// Slider): capturing a drag-to-select via Press, then disabling the box
// WHILE still captured, must release the router's capture — otherwise every
// subsequent pointer event keeps routing to this now-disabled, unwilling
// TextBox forever (deliverCaptured, never hit-testing again), wedging the
// whole app's pointer input. Proven end-to-end: after the disable+release,
// Captured() is nil and a fresh press actually reaches an unrelated probe.
func TestTextBoxSetEnabledFalseMidDragReleasesCapture(t *testing.T) {
	tb := NewTextBox(buttonFace(t))
	tb.SetText("hello world")
	tb.SetWidth(300)
	tb.SetHeight(30)

	probe := &ovProbe{}
	probe.SetWidth(50)
	probe.SetHeight(50)

	canvas := NewCanvas().Add(tb, 0, 0).Add(probe, 350, 0)

	r := input.NewRouter()
	r.SetRoot(canvas)
	layoutButton(canvas, render.Rect{X: 0, Y: 0, W: 500, H: 60})

	r.PointerButton(input.ButtonLeft, true, render.Point{X: 10, Y: 15}, 0) // press: captures for drag-to-select
	if got := r.Captured(); got != core.Widget(tb) {
		t.Fatalf("Captured() after Press = %v, want tb", got)
	}

	tb.SetEnabled(false) // disabled WHILE still mid-drag

	r.PointerButton(input.ButtonLeft, false, render.Point{X: 10, Y: 15}, 0) // Release, delivered captured
	if got := r.Captured(); got != nil {
		t.Fatalf("Captured() after Release while disabled mid-drag = %v, want nil (no wedge)", got)
	}

	r.PointerButton(input.ButtonLeft, true, render.Point{X: 360, Y: 10}, 0) // must reach the probe now
	if got := probe.events; len(got) != 1 || got[0] != "press" {
		t.Fatalf("probe.events = %v, want [press] (pointer flow restored, not permanently wedged)", got)
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

// --- Fix: caret blink phase resets on input / focus lifecycle ---

func TestTextBoxKeystrokeShowsCaretImmediatelyMidBlinkOff(t *testing.T) {
	start := time.Now()
	q := timers.NewQueue(start)
	tb, r := newFocusedTextBox(t, "hi")
	tb.SetTimers(q)

	q.Advance(start.Add(caretBlinkPeriod))
	if tb.caretVisible {
		t.Fatal("test setup: expected caretVisible false after one blink period")
	}

	r.KeyDown(0, 'x', 0) // typed rune: a caret-affecting mutation via replaceRange

	if !tb.caretVisible {
		t.Fatal("caretVisible = false immediately after a keystroke landed mid-blink-off, want true (phase reset)")
	}
	if !tb.caretShown() {
		t.Fatal("caretShown() = false immediately after the keystroke, want true")
	}
}

func TestTextBoxSetCaretAndSelectResetBlinkPhaseMidOff(t *testing.T) {
	start := time.Now()
	q := timers.NewQueue(start)
	tb, _ := newFocusedTextBox(t, "hello")
	tb.SetTimers(q)

	q.Advance(start.Add(caretBlinkPeriod))
	if tb.caretVisible {
		t.Fatal("test setup: expected caretVisible false after one blink period")
	}
	tb.SetCaret(2)
	if !tb.caretVisible {
		t.Fatal("caretVisible = false immediately after SetCaret, want true (phase reset)")
	}

	// SetCaret's restartBlink rescheduled the timer for start+2*period (one
	// period from when SetCaret ran); advance past that to turn it off again.
	q.Advance(start.Add(2 * caretBlinkPeriod))
	if tb.caretVisible {
		t.Fatal("test setup: expected caretVisible false after another blink period")
	}
	tb.Select(0, 3)
	if !tb.caretVisible {
		t.Fatal("caretVisible = false immediately after Select, want true (phase reset)")
	}
}

func TestTextBoxBlinkStopsOnUnfocusAndResetsOnRefocus(t *testing.T) {
	start := time.Now()
	q := timers.NewQueue(start)
	tb, r := newFocusedTextBox(t, "hi")
	tb.SetTimers(q)

	if q.Len() != 1 {
		t.Fatalf("q.Len() right after SetTimers while focused = %d, want 1", q.Len())
	}

	q.Advance(start.Add(caretBlinkPeriod))
	if tb.caretVisible {
		t.Fatal("test setup: expected caretVisible false after one blink period")
	}

	r.Focus(nil) // blur
	if q.Len() != 0 {
		t.Fatalf("q.Len() after blur = %d, want 0 (blink timer stopped while unfocused)", q.Len())
	}

	// Advancing the queue further while unfocused must not resurrect anything.
	q.Advance(start.Add(10 * caretBlinkPeriod))
	if q.Len() != 0 {
		t.Fatalf("q.Len() after advancing while unfocused = %d, want 0", q.Len())
	}

	r.Focus(tb) // refocus
	if !tb.caretVisible {
		t.Fatal("caretVisible = false immediately after refocus, want true (phase reset)")
	}
	if q.Len() != 1 {
		t.Fatalf("q.Len() after refocus = %d, want 1 (blink timer restarted)", q.Len())
	}
}

// --- Ctrl+Left/Right word motion, Ctrl+Home/End, Ctrl+Backspace/Delete ---

// --- Multi-line mode ---

// newFocusedMultilineTextBox mirrors newFocusedTextBox, but with multi-line
// mode enabled and a taller box (300x120, several line-heights) so vertical
// navigation/scrolling tests below have real room to work with.
func newFocusedMultilineTextBox(t *testing.T, initial string) (*TextBox, *input.Router) {
	t.Helper()
	tb := NewTextBox(buttonFace(t)).SetMultiline(true)
	if initial != "" {
		tb.SetText(initial)
	}
	tb.SetWidth(300)
	tb.SetHeight(120)

	r := input.NewRouter()
	r.SetRoot(tb)
	layoutButton(tb, render.Rect{X: 0, Y: 0, W: 300, H: 120})
	r.Focus(tb)
	return tb, r
}

func TestTextBoxMultilineDefaultFalseAndSetter(t *testing.T) {
	tb := NewTextBox(nil)
	if tb.Multiline() {
		t.Fatal("Multiline() = true for a fresh TextBox, want false (default)")
	}
	tb.SetMultiline(true)
	if !tb.Multiline() {
		t.Fatal("Multiline() = false after SetMultiline(true), want true")
	}
}

func TestTextBoxMultilineEnterInsertsNewline(t *testing.T) {
	tb, r := newFocusedMultilineTextBox(t, "ab")
	tb.SetCaret(1)

	var got []string
	tb.OnChanged(func(s string) { got = append(got, s) })

	r.KeyDown(input.KeyEnter, 0, 0)

	if tb.Text() != "a\nb" {
		t.Fatalf("Text() = %q, want %q", tb.Text(), "a\nb")
	}
	if c := tb.Caret(); c != 2 {
		t.Fatalf("Caret() = %d, want 2 (just after the inserted newline)", c)
	}
	if len(got) != 1 || got[0] != "a\nb" {
		t.Fatalf("OnChanged calls = %v, want [%q]", got, "a\nb")
	}
}

// TestTextBoxSingleLineEnterUnhandled locks the "DO NOT change single-line
// Enter behavior" requirement: single-line mode never had a KeyEnter case
// before multi-line mode existed, so Enter must still fall through
// unhandled (no mutation, no OnChanged) — a host that wants Enter-to-submit
// on a single-line box sees the unhandled key bubble past it exactly as it
// always has.
func TestTextBoxSingleLineEnterUnhandled(t *testing.T) {
	tb, r := newFocusedTextBox(t, "ab")
	var calls int
	tb.OnChanged(func(string) { calls++ })

	e := &input.KeyEvent{Action: input.Press, Key: input.KeyEnter, Router: r}
	tb.OnKey(e)

	if e.Handled {
		t.Fatal("Handled = true for Enter on a single-line TextBox, want false (unchanged)")
	}
	if tb.Text() != "ab" {
		t.Fatalf("Text() = %q, want unchanged %q", tb.Text(), "ab")
	}
	if calls != 0 {
		t.Fatalf("OnChanged calls = %d, want 0", calls)
	}
}

// --- OnSubmit (Enter, single-line only) ---

// TestTextBoxOnSubmitFiresOnSingleLineEnter locks OnSubmit's core contract:
// pressing Enter in single-line mode with an OnSubmit callback set fires it
// with the CURRENT text, marks the key handled (so it doesn't bubble, like
// every other key this control owns — see OnKey's own doc comment), and
// mutates nothing (no insertion, no OnChanged) — Enter here is purely a
// notification, not an edit.
func TestTextBoxOnSubmitFiresOnSingleLineEnter(t *testing.T) {
	tb, r := newFocusedTextBox(t, "ab")

	var submitted []string
	tb.OnSubmit(func(s string) { submitted = append(submitted, s) })
	var changed int
	tb.OnChanged(func(string) { changed++ })

	handled := r.KeyDown(input.KeyEnter, 0, 0)

	if !handled {
		t.Fatal("KeyDown(Enter) consumed = false, want true (handled)")
	}
	if len(submitted) != 1 || submitted[0] != "ab" {
		t.Fatalf("OnSubmit calls = %v, want [%q]", submitted, "ab")
	}
	if tb.Text() != "ab" {
		t.Fatalf("Text() = %q, want unchanged %q", tb.Text(), "ab")
	}
	if changed != 0 {
		t.Fatalf("OnChanged calls = %d, want 0 (OnSubmit fires instead, not an edit)", changed)
	}
}

// TestTextBoxOnSubmitNoneSetIsNoOp is TestTextBoxSingleLineEnterUnhandled's
// OnSubmit-aware counterpart: with no OnSubmit set (the zero value), Enter in
// single-line mode is still a harmless no-op — unhandled, no mutation —
// exactly as it was before OnSubmit existed.
func TestTextBoxOnSubmitNoneSetIsNoOp(t *testing.T) {
	tb, r := newFocusedTextBox(t, "ab")

	handled := r.KeyDown(input.KeyEnter, 0, 0)

	if handled {
		t.Fatal("KeyDown(Enter) consumed = true with no OnSubmit set, want false (unchanged no-op)")
	}
	if tb.Text() != "ab" {
		t.Fatalf("Text() = %q, want unchanged %q", tb.Text(), "ab")
	}
}

// TestTextBoxOnSubmitNotFiredInMultiline locks the "multiline is UNCHANGED"
// requirement: with OnSubmit set on a multiline box, Enter still inserts a
// '\n' exactly as TestTextBoxMultilineEnterInsertsNewline already checks,
// and OnSubmit is never called — multiline Enter has its own, unrelated
// meaning (see SetMultiline).
func TestTextBoxOnSubmitNotFiredInMultiline(t *testing.T) {
	tb, r := newFocusedMultilineTextBox(t, "ab")
	tb.SetCaret(1)

	var submitted []string
	tb.OnSubmit(func(s string) { submitted = append(submitted, s) })

	r.KeyDown(input.KeyEnter, 0, 0)

	if tb.Text() != "a\nb" {
		t.Fatalf("Text() = %q, want %q (newline still inserted)", tb.Text(), "a\nb")
	}
	if len(submitted) != 0 {
		t.Fatalf("OnSubmit calls = %v, want none (multiline Enter never submits)", submitted)
	}
}

// TestTextBoxOnSubmitNotFiredBySetText locks fluo's uniform setter
// convention (programmatic setters are silent) for OnSubmit too: a
// programmatic SetText must never fire it, only a real user Enter keypress
// does (see TestTextBoxOnSubmitFiresOnSingleLineEnter).
func TestTextBoxOnSubmitNotFiredBySetText(t *testing.T) {
	tb := NewTextBox(buttonFace(t))

	var submitted []string
	tb.OnSubmit(func(s string) { submitted = append(submitted, s) })

	tb.SetText("hello")

	if len(submitted) != 0 {
		t.Fatalf("OnSubmit calls after SetText = %v, want none (silent programmatic setter)", submitted)
	}
}

// TestTextBoxSingleLineUpDownUnhandled is Enter's counterpart for Up/Down:
// neither was ever part of the single-line keyboard map, so both must stay
// unhandled (and leave the caret untouched) in single-line mode.
func TestTextBoxSingleLineUpDownUnhandled(t *testing.T) {
	tb, r := newFocusedTextBox(t, "hello")
	tb.SetCaret(2)

	up := &input.KeyEvent{Action: input.Press, Key: input.KeyUp, Router: r}
	tb.OnKey(up)
	if up.Handled {
		t.Fatal("Up: Handled = true on a single-line TextBox, want false")
	}

	down := &input.KeyEvent{Action: input.Press, Key: input.KeyDown, Router: r}
	tb.OnKey(down)
	if down.Handled {
		t.Fatal("Down: Handled = true on a single-line TextBox, want false")
	}
	if c := tb.Caret(); c != 2 {
		t.Fatalf("Caret() = %d, want unchanged 2", c)
	}
}

// TestTextBoxLineColMapping locks lineCol/indexOfLineCol as exact inverses
// against a text with three lines of differing length ("ab", "cde", "f"),
// including the boundary indices immediately before/after each '\n' and the
// very end of the text.
func TestTextBoxLineColMapping(t *testing.T) {
	tb := NewTextBox(nil).SetMultiline(true)
	tb.SetText("ab\ncde\nf") // a b \n c d e \n f — indices 0..8

	cases := []struct {
		idx       int
		line, col int
	}{
		{0, 0, 0},
		{1, 0, 1},
		{2, 0, 2}, // just before the first '\n': end of line 0
		{3, 1, 0}, // just after the first '\n': start of line 1
		{5, 1, 2},
		{6, 1, 3}, // just before the second '\n': end of line 1
		{7, 2, 0}, // just after the second '\n': start of line 2
		{8, 2, 1}, // len(runes): end of the whole text
	}
	for _, c := range cases {
		if line, col := tb.lineCol(c.idx); line != c.line || col != c.col {
			t.Fatalf("lineCol(%d) = (%d,%d), want (%d,%d)", c.idx, line, col, c.line, c.col)
		}
		if idx := tb.indexOfLineCol(c.line, c.col); idx != c.idx {
			t.Fatalf("indexOfLineCol(%d,%d) = %d, want %d", c.line, c.col, idx, c.idx)
		}
	}
}

func TestTextBoxUpDownPreservesDesiredColumn(t *testing.T) {
	tb, r := newFocusedMultilineTextBox(t, "abc\nde\nfghij")
	tb.SetCaret(11) // line 2 ("fghij"), col 4 — desired column becomes 4

	r.KeyDown(input.KeyUp, 0, 0)
	if line, col := tb.lineCol(tb.Caret()); line != 1 || col != 2 {
		t.Fatalf("after Up onto shorter 'de': lineCol = (%d,%d), want (1,2) (clamped to line end)", line, col)
	}

	r.KeyDown(input.KeyUp, 0, 0)
	if line, col := tb.lineCol(tb.Caret()); line != 0 || col != 3 {
		t.Fatalf("after 2nd Up onto shorter 'abc': lineCol = (%d,%d), want (0,3) (still tracking desired col 4, clamped to 3)", line, col)
	}

	r.KeyDown(input.KeyDown, 0, 0)
	r.KeyDown(input.KeyDown, 0, 0)
	if c := tb.Caret(); c != 11 {
		t.Fatalf("after 2x Down back onto the longer line: Caret() = %d, want 11 (desired col 4 restored, not stuck at 3)", c)
	}
}

func TestTextBoxUpDownClampAtFirstAndLastLine(t *testing.T) {
	tb, r := newFocusedMultilineTextBox(t, "ab\ncd")

	tb.SetCaret(1)
	r.KeyDown(input.KeyUp, 0, 0)
	if line, _ := tb.lineCol(tb.Caret()); line != 0 {
		t.Fatalf("Up at the first line: line = %d, want 0 (clamped)", line)
	}

	tb.SetCaret(4)
	r.KeyDown(input.KeyDown, 0, 0)
	if line, _ := tb.lineCol(tb.Caret()); line != 1 {
		t.Fatalf("Down at the last line: line = %d, want 1 (clamped)", line)
	}
}

func TestTextBoxShiftUpDownExtendSelection(t *testing.T) {
	tb, r := newFocusedMultilineTextBox(t, "abc\ndef\nghi")
	tb.SetCaret(9) // line2, col1

	r.KeyDown(input.KeyUp, 0, input.ModShift)
	if s, e := tb.Selection(); s != 5 || e != 9 {
		t.Fatalf("Shift+Up: Selection() = (%d,%d), want (5,9)", s, e)
	}

	r.KeyDown(input.KeyUp, 0, input.ModShift)
	if s, e := tb.Selection(); s != 1 || e != 9 {
		t.Fatalf("Shift+Up again: Selection() = (%d,%d), want (1,9) (anchor pinned at 9)", s, e)
	}
}

func TestTextBoxHomeEndPerLineInMultiline(t *testing.T) {
	tb, r := newFocusedMultilineTextBox(t, "abc\nde")
	tb.SetCaret(5) // line 1 ("de"), col 1

	r.KeyDown(input.KeyHome, 0, 0)
	if c := tb.Caret(); c != 4 {
		t.Fatalf("Home: Caret() = %d, want 4 (start of the CURRENT line 'de', not 0)", c)
	}

	tb.SetCaret(5)
	r.KeyDown(input.KeyEnd, 0, 0)
	if c := tb.Caret(); c != 6 {
		t.Fatalf("End: Caret() = %d, want 6 (end of the CURRENT line 'de')", c)
	}
}

func TestTextBoxLeftRightCrossLineBoundaries(t *testing.T) {
	tb, r := newFocusedMultilineTextBox(t, "ab\ncd")
	tb.SetCaret(2) // just before the '\n'

	r.KeyDown(input.KeyRight, 0, 0)
	if line, col := tb.lineCol(tb.Caret()); line != 1 || col != 0 {
		t.Fatalf("Right over the newline: lineCol = (%d,%d), want (1,0)", line, col)
	}

	r.KeyDown(input.KeyLeft, 0, 0)
	if line, col := tb.lineCol(tb.Caret()); line != 0 || col != 2 {
		t.Fatalf("Left back over the newline: lineCol = (%d,%d), want (0,2)", line, col)
	}
}

func TestTextBoxSelectionCutCopyPasteAcrossNewlines(t *testing.T) {
	tb, r := newFocusedMultilineTextBox(t, "abc\ndef\nghi")
	clip := &fakeClip{}
	r.SetClipboard(clip)

	tb.Select(1, 9) // "bc\ndef\ng" — spans both newlines
	r.KeyDown(input.KeyC, 0, input.ModCtrl)
	if clip.text != "bc\ndef\ng" {
		t.Fatalf("clipboard after copy = %q, want %q", clip.text, "bc\ndef\ng")
	}
	if tb.Text() != "abc\ndef\nghi" {
		t.Fatalf("Text() after copy = %q, want unchanged", tb.Text())
	}

	r.KeyDown(input.KeyX, 0, input.ModCtrl)
	if tb.Text() != "ahi" {
		t.Fatalf("Text() after cut = %q, want %q", tb.Text(), "ahi")
	}

	tb.SetCaret(1) // between 'a' and "hi"
	r.KeyDown(input.KeyV, 0, input.ModCtrl)
	if tb.Text() != "abc\ndef\nghi" {
		t.Fatalf("Text() after paste = %q, want %q (newlines preserved on paste)", tb.Text(), "abc\ndef\nghi")
	}
}

func TestTextBoxMultilinePasteNormalizesCRLF(t *testing.T) {
	tb, r := newFocusedMultilineTextBox(t, "")
	clip := &fakeClip{text: "one\r\ntwo\rthree"}
	r.SetClipboard(clip)

	r.KeyDown(input.KeyV, 0, input.ModCtrl)

	if tb.Text() != "one\ntwo\nthree" {
		t.Fatalf("Text() after paste = %q, want %q (CRLF and lone CR normalized to LF, not stripped)", tb.Text(), "one\ntwo\nthree")
	}
}

func TestTextBoxMultilineDesiredHeightIsTaller(t *testing.T) {
	face := buttonFace(t)
	tb := NewTextBox(face).SetMultiline(true)
	core.MeasureWidget(tb, render.Size{W: 1000, H: 1000})
	d := core.DesiredSizeOf(tb)

	want := face.LineHeight()*textBoxMultilineDefaultLines + 2*tb.metrics.PaddingM
	if d.H != want {
		t.Fatalf("DesiredSize().H = %v, want %v (%d line-heights + padding)", d.H, want, textBoxMultilineDefaultLines)
	}
}

// TestTextBoxVerticalScrollKeepsCaretVisible mirrors
// TestTextBoxHScrollKeepsCaretVisible for the new vertical axis: ten lines
// in a box tall enough for only ~2 must scroll vscroll so the caret's own
// line (the last one here) stays within the inner (padding-inset) height.
func TestTextBoxVerticalScrollKeepsCaretVisible(t *testing.T) {
	tb := NewTextBox(buttonFace(t)).SetMultiline(true)
	tb.SetText("l0\nl1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9") // 10 lines; caret ends on the last
	tb.SetWidth(200)
	tb.SetHeight(50) // narrow explicit height: room for roughly 2 lines

	core.MeasureWidget(tb, render.Size{W: 200, H: 50})
	core.ArrangeWidget(tb, render.Rect{X: 0, Y: 0, W: 200, H: 50})

	if tb.vscroll <= 0 {
		t.Fatalf("vscroll = %v, want > 0 (short box, caret on the last of many lines)", tb.vscroll)
	}

	pad := tb.metrics.PaddingM
	innerH := tb.Bounds().H - 2*pad
	lh := tb.lineHeight()
	line, _ := tb.lineCol(tb.Caret())
	caretY := float32(line)*lh - tb.vscroll
	const eps = 0.01
	if caretY < -eps || caretY > innerH-lh+eps {
		t.Fatalf("caret line display y = %v, want within [0, %v] (inner height minus one line)", caretY, innerH-lh)
	}
}

// TestTextBoxDisabledMultilineIgnoresKeys mirrors
// TestTextBoxDisabledIgnoresAllKeys for the multi-line-only keys (Enter,
// Up, Down): a disabled multi-line TextBox must ignore those too.
func TestTextBoxDisabledMultilineIgnoresKeys(t *testing.T) {
	tb := NewTextBox(buttonFace(t)).SetMultiline(true)
	tb.SetText("ab\ncd")
	tb.SetEnabled(false)
	tb.OnFocusChanged(true)

	r := input.NewRouter()
	cases := []input.KeyEvent{
		{Action: input.Press, Key: input.KeyEnter, Router: r},
		{Action: input.Press, Key: input.KeyUp, Router: r},
		{Action: input.Press, Key: input.KeyDown, Router: r},
	}
	for i := range cases {
		e := cases[i]
		tb.OnKey(&e)
		if e.Handled {
			t.Fatalf("case %d: Handled = true on a disabled TextBox, want false", i)
		}
	}
	if tb.Text() != "ab\ncd" {
		t.Fatalf("Text() = %q, want unchanged", tb.Text())
	}
}

// --- IME anchor: CaretScreenRect ---

func TestTextBoxCaretScreenRectFalseWhenUnfocused(t *testing.T) {
	tb := NewTextBox(buttonFace(t))
	tb.SetText("hello")
	tb.SetWidth(300)
	tb.SetHeight(30)
	layoutButton(tb, render.Rect{X: 0, Y: 0, W: 300, H: 30})

	if _, ok := tb.CaretScreenRect(); ok {
		t.Fatal("CaretScreenRect() ok = true while unfocused, want false")
	}
}

// TestTextBoxCaretScreenRectSingleLine locks CaretScreenRect's math against
// the exact same expression Render uses to place the drawn caret bar
// (textX+xOf(caret), vertically centered by lineHeight()) — so the IME
// anchor and the visible caret can never drift apart.
func TestTextBoxCaretScreenRectSingleLine(t *testing.T) {
	tb, _ := newFocusedTextBox(t, "hello")
	tb.SetCaret(3)

	got, ok := tb.CaretScreenRect()
	if !ok {
		t.Fatal("CaretScreenRect() ok = false while focused, want true")
	}

	bounds := tb.Bounds()
	pad := tb.metrics.PaddingM
	lh := tb.lineHeight()
	wantX := bounds.X + pad - tb.hscroll + tb.xOf(3)
	wantY := bounds.Y + (bounds.H-lh)/2

	if got.X != wantX || got.Y != wantY || got.H != lh {
		t.Fatalf("CaretScreenRect() = %+v, want X=%v Y=%v H=%v", got, wantX, wantY, lh)
	}
}

// TestTextBoxCaretScreenRectMultiline mirrors the single-line case for a
// multi-line box: the caret sits on its own (line, col), so both X and Y
// must reflect renderMultiline's per-line placement (xOfInLine + vscroll),
// not the single-line xOf/centered-Y math above.
func TestTextBoxCaretScreenRectMultiline(t *testing.T) {
	tb, _ := newFocusedMultilineTextBox(t, "abc\nde")
	tb.SetCaret(5) // line 1 ("de"), col 1

	got, ok := tb.CaretScreenRect()
	if !ok {
		t.Fatal("CaretScreenRect() ok = false while focused, want true")
	}

	bounds := tb.Bounds()
	pad := tb.metrics.PaddingM
	lh := tb.lineHeight()
	line, col := tb.lineCol(tb.Caret())
	wantX := bounds.X + pad - tb.hscroll + tb.xOfInLine(line, col)
	wantY := bounds.Y + pad - tb.vscroll + float32(line)*lh

	if got.X != wantX || got.Y != wantY || got.H != lh {
		t.Fatalf("CaretScreenRect() = %+v, want X=%v Y=%v H=%v (line=%d col=%d)", got, wantX, wantY, lh, line, col)
	}
}

// TestTextBoxWordBoundaryHelpers locks prevWordBoundary/nextWordBoundary's
// word definition directly against "foo, bar\nbaz_1 x": a run of whitespace
// (which includes '\n' — word motion crosses line boundaries for free) is
// always skipped first, then a run of same-class runes — "word" runes
// (letters/digits/'_') and punctuation runs are each their own stop, so ","
// is its own boundary distinct from "foo".
func TestTextBoxWordBoundaryHelpers(t *testing.T) {
	tb := NewTextBox(nil)
	tb.SetText("foo, bar\nbaz_1 x") // indices: f0o1o2,3 4b5a6r7\n8b9a10z11_12 1 13 14x15, len 16

	nextCases := []struct{ from, want int }{
		{0, 3}, {3, 4}, {4, 8}, {8, 14}, {14, 16}, {16, 16},
	}
	for _, c := range nextCases {
		if got := tb.nextWordBoundary(c.from); got != c.want {
			t.Fatalf("nextWordBoundary(%d) = %d, want %d", c.from, got, c.want)
		}
	}

	prevCases := []struct{ from, want int }{
		{16, 15}, {15, 9}, {9, 5}, {5, 3}, {3, 0}, {0, 0},
	}
	for _, c := range prevCases {
		if got := tb.prevWordBoundary(c.from); got != c.want {
			t.Fatalf("prevWordBoundary(%d) = %d, want %d", c.from, got, c.want)
		}
	}
}

// TestTextBoxCtrlRightMovesByWord is the successor to the pre-word-motion
// "Ctrl+Right degrades to plain Right" contract: Ctrl+Right/Left now move
// the caret by whole words (see nextWordBoundary/prevWordBoundary) rather
// than one rune at a time — the intentional behavior this feature adds, not
// a regression of the older placeholder behavior.
func TestTextBoxCtrlRightMovesByWord(t *testing.T) {
	tb, r := newFocusedTextBox(t, "foo, bar baz")
	tb.SetCaret(0)

	wantStops := []int{3, 4, 8, 12, 12} // clamped at the end on the 5th press
	for i, want := range wantStops {
		e := &input.KeyEvent{Action: input.Press, Key: input.KeyRight, Mods: input.ModCtrl, Router: r}
		tb.OnKey(e)
		if !e.Handled {
			t.Fatalf("press %d: Ctrl+Right Handled = false, want true", i)
		}
		if c := tb.Caret(); c != want {
			t.Fatalf("press %d: Ctrl+Right Caret() = %d, want %d", i, c, want)
		}
		if s, e2 := tb.Selection(); s != e2 {
			t.Fatalf("press %d: Ctrl+Right Selection() = (%d,%d), want collapsed", i, s, e2)
		}
	}
}

func TestTextBoxCtrlLeftMovesByWord(t *testing.T) {
	tb, r := newFocusedTextBox(t, "foo, bar baz")
	tb.SetCaret(12)

	wantStops := []int{9, 5, 3, 0, 0}
	for i, want := range wantStops {
		r.KeyDown(input.KeyLeft, 0, input.ModCtrl)
		if c := tb.Caret(); c != want {
			t.Fatalf("press %d: Ctrl+Left Caret() = %d, want %d", i, c, want)
		}
	}
}

func TestTextBoxCtrlShiftLeftRightExtendSelectionByWord(t *testing.T) {
	tb, r := newFocusedTextBox(t, "foo bar baz")
	tb.SetCaret(4) // anchor==caret==4, start of "bar"

	r.KeyDown(input.KeyRight, 0, input.ModCtrl|input.ModShift)
	if s, e := tb.Selection(); s != 4 || e != 7 {
		t.Fatalf("Ctrl+Shift+Right: Selection() = (%d,%d), want (4,7)", s, e)
	}

	r.KeyDown(input.KeyLeft, 0, input.ModCtrl|input.ModShift)
	r.KeyDown(input.KeyLeft, 0, input.ModCtrl|input.ModShift)
	if s, e := tb.Selection(); s != 0 || e != 4 {
		t.Fatalf("after 2x Ctrl+Shift+Left: Selection() = (%d,%d), want (0,4) (anchor pinned at 4)", s, e)
	}
	if c := tb.Caret(); c != 0 {
		t.Fatalf("Caret() = %d, want 0", c)
	}
}

// TestTextBoxCtrlHomeEndSingleLineMatchesPlainHomeEnd locks the "Ctrl+Home/
// End == Home/End in single-line mode" requirement: both target the same
// 0/len(runes) bounds there, since plain Home/End already reach the whole
// text in single-line mode (homeTarget/endTarget).
func TestTextBoxCtrlHomeEndSingleLineMatchesPlainHomeEnd(t *testing.T) {
	tb, r := newFocusedTextBox(t, "hello world")
	tb.SetCaret(4)

	r.KeyDown(input.KeyHome, 0, input.ModCtrl)
	if c := tb.Caret(); c != 0 {
		t.Fatalf("Ctrl+Home: Caret() = %d, want 0", c)
	}

	tb.SetCaret(4)
	r.KeyDown(input.KeyEnd, 0, input.ModCtrl)
	want := len([]rune("hello world"))
	if c := tb.Caret(); c != want {
		t.Fatalf("Ctrl+End: Caret() = %d, want %d", c, want)
	}
}

// TestTextBoxCtrlHomeEndMultilineJumpsToBufferBounds is Ctrl+Home/End's core
// multi-line contract: plain Home/End there only reach the caret's OWN line
// (homeTarget/endTarget), so Ctrl+Home/End is the only way to reach the
// whole buffer's start/end.
func TestTextBoxCtrlHomeEndMultilineJumpsToBufferBounds(t *testing.T) {
	tb, r := newFocusedMultilineTextBox(t, "abc\ndef\nghi")
	tb.SetCaret(5) // line 1 ("def"), col 1

	r.KeyDown(input.KeyHome, 0, input.ModCtrl)
	if c := tb.Caret(); c != 0 {
		t.Fatalf("Ctrl+Home: Caret() = %d, want 0 (buffer start)", c)
	}
	if s, e := tb.Selection(); s != e {
		t.Fatalf("Ctrl+Home: Selection() = (%d,%d), want collapsed", s, e)
	}

	tb.SetCaret(5)
	r.KeyDown(input.KeyEnd, 0, input.ModCtrl)
	want := len([]rune("abc\ndef\nghi"))
	if c := tb.Caret(); c != want {
		t.Fatalf("Ctrl+End: Caret() = %d, want %d (buffer end)", c, want)
	}
}

func TestTextBoxCtrlShiftHomeEndExtendSelectionToBufferBounds(t *testing.T) {
	tb, r := newFocusedMultilineTextBox(t, "abc\ndef\nghi")
	tb.SetCaret(5)

	r.KeyDown(input.KeyHome, 0, input.ModCtrl|input.ModShift)
	if s, e := tb.Selection(); s != 0 || e != 5 {
		t.Fatalf("Ctrl+Shift+Home: Selection() = (%d,%d), want (0,5)", s, e)
	}

	tb.SetCaret(5)
	r.KeyDown(input.KeyEnd, 0, input.ModCtrl|input.ModShift)
	want := len([]rune("abc\ndef\nghi"))
	if s, e := tb.Selection(); s != 5 || e != want {
		t.Fatalf("Ctrl+Shift+End: Selection() = (%d,%d), want (5,%d)", s, e, want)
	}
}

// --- Ctrl+Backspace/Delete: word deletion ---

func TestTextBoxCtrlBackspaceDeletesPrecedingWord(t *testing.T) {
	tb, r := newFocusedTextBox(t, "foo bar baz")
	tb.SetCaret(len([]rune("foo bar baz")))

	var got []string
	tb.OnChanged(func(s string) { got = append(got, s) })

	e := &input.KeyEvent{Action: input.Press, Key: input.KeyBackspace, Mods: input.ModCtrl, Router: r}
	tb.OnKey(e)

	if !e.Handled {
		t.Fatal("Ctrl+Backspace: Handled = false, want true")
	}
	if tb.Text() != "foo bar " {
		t.Fatalf("Text() = %q, want %q", tb.Text(), "foo bar ")
	}
	if c := tb.Caret(); c != len([]rune("foo bar ")) {
		t.Fatalf("Caret() = %d, want %d", c, len([]rune("foo bar ")))
	}
	if len(got) != 1 || got[0] != "foo bar " {
		t.Fatalf("OnChanged calls = %v, want [%q]", got, "foo bar ")
	}
}

func TestTextBoxCtrlDeleteDeletesFollowingWord(t *testing.T) {
	tb, r := newFocusedTextBox(t, "foo bar baz")
	tb.SetCaret(0)

	e := &input.KeyEvent{Action: input.Press, Key: input.KeyDelete, Mods: input.ModCtrl, Router: r}
	tb.OnKey(e)

	if !e.Handled {
		t.Fatal("Ctrl+Delete: Handled = false, want true")
	}
	if tb.Text() != " bar baz" {
		t.Fatalf("Text() = %q, want %q", tb.Text(), " bar baz")
	}
	if c := tb.Caret(); c != 0 {
		t.Fatalf("Caret() = %d, want 0", c)
	}
}

// TestTextBoxCtrlBackspaceDeleteDeleteSelectionInstead locks the
// selection-first convention Ctrl+Backspace/Delete share with plain
// Backspace/Delete: an active selection is deleted outright, ignoring word
// boundaries entirely.
func TestTextBoxCtrlBackspaceDeleteDeleteSelectionInstead(t *testing.T) {
	tb, r := newFocusedTextBox(t, "foo bar baz")
	tb.Select(1, 6) // "oo ba" selected, mid-word on both ends

	r.KeyDown(input.KeyBackspace, 0, input.ModCtrl)
	if tb.Text() != "fr baz" {
		t.Fatalf("Ctrl+Backspace with selection: Text() = %q, want %q", tb.Text(), "fr baz")
	}

	tb2, r2 := newFocusedTextBox(t, "foo bar baz")
	tb2.Select(1, 6)
	r2.KeyDown(input.KeyDelete, 0, input.ModCtrl)
	if tb2.Text() != "fr baz" {
		t.Fatalf("Ctrl+Delete with selection: Text() = %q, want %q", tb2.Text(), "fr baz")
	}
}

// --- PageUp/PageDown: viewport-height caret motion (multi-line only) ---

func TestTextBoxPageUpDownFallbackRowsWithoutFace(t *testing.T) {
	tb := NewTextBox(nil).SetMultiline(true)
	if got := tb.pageRows(); got != pageRowsFallback {
		t.Fatalf("pageRows() with nil face = %d, want %d (fallback)", got, pageRowsFallback)
	}
}

func TestTextBoxPageUpDownFallbackRowsWhenUnarranged(t *testing.T) {
	tb := NewTextBox(buttonFace(t)).SetMultiline(true) // never measured/arranged: Bounds() is zero
	if got := tb.pageRows(); got != pageRowsFallback {
		t.Fatalf("pageRows() unarranged = %d, want %d (fallback)", got, pageRowsFallback)
	}
}

func TestTextBoxPageDownUpMoveByViewportRowsAndClamp(t *testing.T) {
	// Build enough lines that PageDown/PageUp never has to clamp for the
	// middle-of-buffer assertions below, sized relative to the box's own
	// pageRows() so this test doesn't hardcode a font-metric-dependent row
	// count.
	probe, _ := newFocusedMultilineTextBox(t, "x")
	pr := probe.pageRows()
	if pr < 1 {
		t.Fatalf("test setup: pageRows() = %d, want >= 1", pr)
	}
	total := pr*3 + 4

	lines := make([]string, total)
	for i := range lines {
		lines[i] = "l"
	}
	tb, r := newFocusedMultilineTextBox(t, strings.Join(lines, "\n"))

	tb.SetCaret(tb.indexOfLineCol(pr, 0))
	r.KeyDown(input.KeyPageDown, 0, 0)
	if line, _ := tb.lineCol(tb.Caret()); line != 2*pr {
		t.Fatalf("PageDown from line %d: line = %d, want %d (pr=%d)", pr, line, 2*pr, pr)
	}

	r.KeyDown(input.KeyPageUp, 0, 0)
	if line, _ := tb.lineCol(tb.Caret()); line != pr {
		t.Fatalf("PageUp back: line = %d, want %d", line, pr)
	}

	// Clamp at the last line.
	tb.SetCaret(tb.indexOfLineCol(total-1, 0))
	r.KeyDown(input.KeyPageDown, 0, 0)
	if line, _ := tb.lineCol(tb.Caret()); line != total-1 {
		t.Fatalf("PageDown at last line: line = %d, want %d (clamped)", line, total-1)
	}

	// Clamp at the first line.
	tb.SetCaret(0)
	r.KeyDown(input.KeyPageUp, 0, 0)
	if line, _ := tb.lineCol(tb.Caret()); line != 0 {
		t.Fatalf("PageUp at first line: line = %d, want 0 (clamped)", line)
	}
}

func TestTextBoxShiftPageUpDownExtendSelection(t *testing.T) {
	probe, _ := newFocusedMultilineTextBox(t, "x")
	pr := probe.pageRows()
	total := pr*3 + 4
	lines := make([]string, total)
	for i := range lines {
		lines[i] = "l"
	}
	tb, r := newFocusedMultilineTextBox(t, strings.Join(lines, "\n"))

	start := tb.indexOfLineCol(pr, 0)
	tb.SetCaret(start)
	r.KeyDown(input.KeyPageDown, 0, input.ModShift)
	if s, e := tb.Selection(); s != start {
		t.Fatalf("Shift+PageDown: Selection() start = %d, want %d (anchor pinned)", s, start)
	} else if line, _ := tb.lineCol(e); line != 2*pr {
		t.Fatalf("Shift+PageDown: selection end line = %d, want %d", line, 2*pr)
	}
}

func TestTextBoxPageUpDownSingleLineUnhandled(t *testing.T) {
	tb, r := newFocusedTextBox(t, "hello")
	tb.SetCaret(2)

	up := &input.KeyEvent{Action: input.Press, Key: input.KeyPageUp, Router: r}
	tb.OnKey(up)
	if up.Handled {
		t.Fatal("PageUp: Handled = true on a single-line TextBox, want false")
	}

	down := &input.KeyEvent{Action: input.Press, Key: input.KeyPageDown, Router: r}
	tb.OnKey(down)
	if down.Handled {
		t.Fatal("PageDown: Handled = true on a single-line TextBox, want false")
	}
	if c := tb.Caret(); c != 2 {
		t.Fatalf("Caret() = %d, want unchanged 2", c)
	}
}

// TestTextBoxOnCompositionUpdateSetsPreeditWithoutTouchingCommittedText is
// Task 6 Phase B's core contract: an Active composition update stores the
// provisional preedit string for display, and does NOT mutate Text() or
// fire OnChanged — the committed buffer stays exactly what it was before
// the composition began, until (and unless) it actually commits.
func TestTextBoxOnCompositionUpdateSetsPreeditWithoutTouchingCommittedText(t *testing.T) {
	tb, r := newFocusedTextBox(t, "he")
	var changed []string
	tb.OnChanged(func(s string) { changed = append(changed, s) })

	r.CompositionUpdate("ん", 1)

	if got := tb.Text(); got != "he" {
		t.Fatalf("Text() during composition = %q, want %q (committed text untouched)", got, "he")
	}
	if len(changed) != 0 {
		t.Fatalf("OnChanged calls = %v, want none (a provisional composition update never fires it)", changed)
	}
	if !tb.composing {
		t.Fatal("composing = false after CompositionUpdate, want true")
	}
	if got := string(tb.preedit); got != "ん" {
		t.Fatalf("preedit = %q, want %q", got, "ん")
	}
	if tb.preeditCaret != 1 {
		t.Fatalf("preeditCaret = %d, want 1", tb.preeditCaret)
	}
}

// TestTextBoxOnCompositionCommitInsertsAndClearsPreedit verifies the commit
// path funnels through the same insertText/replaceRange mutation every
// other user edit uses: Text() reflects the inserted string, OnChanged
// fires with it, the caret lands just after it, and the preedit is cleared
// (composing back to false).
func TestTextBoxOnCompositionCommitInsertsAndClearsPreedit(t *testing.T) {
	tb, r := newFocusedTextBox(t, "he")
	tb.SetCaret(2)
	var changed []string
	tb.OnChanged(func(s string) { changed = append(changed, s) })

	r.CompositionUpdate("ん", 1)
	r.CompositionCommit("ん")

	if got := tb.Text(); got != "heん" {
		t.Fatalf("Text() after commit = %q, want %q", got, "heん")
	}
	if len(changed) != 1 || changed[0] != "heん" {
		t.Fatalf("OnChanged calls = %v, want [%q]", changed, "heん")
	}
	if want := len([]rune("heん")); tb.Caret() != want {
		t.Fatalf("Caret() after commit = %d, want %d (just after the inserted text)", tb.Caret(), want)
	}
	if tb.composing {
		t.Fatal("composing = true after commit, want false")
	}
	if len(tb.preedit) != 0 {
		t.Fatalf("preedit after commit = %q, want empty", string(tb.preedit))
	}
}

// TestTextBoxOnCompositionCancelClearsPreeditWithoutInserting is the
// Escape/cancel path: no insertion, no OnChanged, preedit cleared.
func TestTextBoxOnCompositionCancelClearsPreeditWithoutInserting(t *testing.T) {
	tb, r := newFocusedTextBox(t, "he")
	var changed []string
	tb.OnChanged(func(s string) { changed = append(changed, s) })

	r.CompositionUpdate("ん", 1)
	r.CompositionCancel()

	if got := tb.Text(); got != "he" {
		t.Fatalf("Text() after cancel = %q, want %q (nothing inserted)", got, "he")
	}
	if len(changed) != 0 {
		t.Fatalf("OnChanged calls = %v, want none", changed)
	}
	if tb.composing {
		t.Fatal("composing = true after cancel, want false")
	}
	if len(tb.preedit) != 0 {
		t.Fatalf("preedit after cancel = %q, want empty", string(tb.preedit))
	}
}

// TestTextBoxOnCompositionIgnoredWhenNotFocused mirrors OnKey's own
// disabled/unfocused guard: an unfocused (or disabled) TextBox must not
// react to composition events at all — no preedit, no mutation.
func TestTextBoxOnCompositionIgnoredWhenNotFocused(t *testing.T) {
	tb := NewTextBox(buttonFace(t))
	tb.SetText("he")
	// Deliberately never focused (OnFocusChanged(true) never called).

	tb.OnComposition(input.CompositionEvent{Preedit: "ん", CaretPos: 1, Active: true})

	if tb.composing {
		t.Fatal("composing = true while unfocused, want false")
	}
	if got := tb.Text(); got != "he" {
		t.Fatalf("Text() = %q, want %q (unchanged)", got, "he")
	}
}

// TestTextBoxKeyDownIgnoredWhileComposing is the defensive OnKey guard: a
// rune KeyDown delivered mid-composition must not insert into the committed
// buffer (an active composition owns keyboard input until it ends — see
// OnKey's own doc comment).
func TestTextBoxKeyDownIgnoredWhileComposing(t *testing.T) {
	tb, r := newFocusedTextBox(t, "he")

	r.CompositionUpdate("ん", 1)

	e := &input.KeyEvent{Action: input.Press, Key: 0, Rune: 'x', Router: r}
	tb.OnKey(e)

	if e.Handled {
		t.Fatal("OnKey during composition: Handled = true, want false (ignored)")
	}
	if got := tb.Text(); got != "he" {
		t.Fatalf("Text() after KeyDown during composition = %q, want %q (unchanged)", got, "he")
	}
}

// TestTextBoxKeyDownStillWorksWhenNotComposing is the regression guard for
// the OnKey composing check above: with no composition active (the default,
// zero-value state), a normal rune KeyDown must behave exactly as before
// Phase B — inserted at the caret, Handled set.
func TestTextBoxKeyDownStillWorksWhenNotComposing(t *testing.T) {
	tb, r := newFocusedTextBox(t, "he")
	tb.SetCaret(2)

	e := &input.KeyEvent{Action: input.Press, Key: 0, Rune: 'y', Router: r}
	tb.OnKey(e)

	if !e.Handled {
		t.Fatal("OnKey: Handled = false, want true")
	}
	if got := tb.Text(); got != "hey" {
		t.Fatalf("Text() = %q, want %q", got, "hey")
	}
}

// --- Word wrap (opt-in; see SetWordWrap) ---

// TestTextBoxSetWordWrapGetterAndSingleLineNoop locks the basic API shape:
// false by default, chainable/settable, and — per SetWordWrap's own doc
// comment — a no-op flag (wrapping() stays false) until Multiline() is also
// true.
func TestTextBoxSetWordWrapGetterAndSingleLineNoop(t *testing.T) {
	tb := NewTextBox(nil)
	if tb.WordWrap() {
		t.Fatal("WordWrap() = true for a fresh TextBox, want false (default)")
	}

	tb.SetWordWrap(true)
	if !tb.WordWrap() {
		t.Fatal("WordWrap() = false after SetWordWrap(true), want true")
	}
	if tb.wrapping() {
		t.Fatal("wrapping() = true while Multiline() is false, want false (word-wrap is only meaningful in multiline mode)")
	}

	tb.SetMultiline(true)
	if !tb.wrapping() {
		t.Fatal("wrapping() = false once multiline is also on, want true")
	}
}

// TestTextBoxWordWrapOffKeepsUnwrappedHorizontalScrollBehavior is the
// invariant lock for "wrap OFF (default) ... byte-for-byte unchanged": a
// long single line in a narrow multi-line box, with SetWordWrap left at its
// default false, must still scroll horizontally exactly like an unwrapped
// multi-line box always has (see TestTextBoxHScrollKeepsCaretVisible, its
// single-line counterpart) rather than wrapping.
func TestTextBoxWordWrapOffKeepsUnwrappedHorizontalScrollBehavior(t *testing.T) {
	tb := NewTextBox(buttonFace(t)).SetMultiline(true) // WordWrap left at its default (false)
	tb.SetText("this is a much longer single line of text than the box is wide")
	tb.SetWidth(80)
	tb.SetHeight(60)

	core.MeasureWidget(tb, render.Size{W: 80, H: 60})
	core.ArrangeWidget(tb, render.Rect{X: 0, Y: 0, W: 80, H: 60})

	if tb.hscroll <= 0 {
		t.Fatalf("hscroll = %v, want > 0 (wrap off: a long line must still scroll horizontally, unchanged)", tb.hscroll)
	}
}

// --- computeVisualRows / wrapLogicalLine: pure layout tests ---

// TestComputeVisualRowsDeterministicThreeRowFixture is the shared fixture
// every TextBox-level wrap test below builds on: six repeated "aa" words
// ("aa aa aa aa aa aa", 17 runes) wrapped at a content width sized to fit
// exactly two words — face.Measure("aa aa").W — so every row boundary is
// fully predictable ahead of time (identical glyphs throughout means no
// per-letter width ambiguity): three rows, each "aa aa" (5 runes), with the
// separating space at indices 5 and 11 dropped from display (the "eat the
// wrap-point space" rule — see wrapLogicalLine's case 2) and nothing left
// over at the buffer's own end. Proves N-rows-at-a-width and
// breaking-at-spaces together in one deterministic case.
func TestComputeVisualRowsDeterministicThreeRowFixture(t *testing.T) {
	face := buttonFace(t)
	width := face.Measure("aa aa").W
	runes := []rune(wordWrapFixtureText)

	rows := computeVisualRows(runes, face, width)

	want := []visualRow{{start: 0, end: 5}, {start: 6, end: 11}, {start: 12, end: 17}}
	if len(rows) != len(want) {
		t.Fatalf("computeVisualRows rows = %+v, want %+v", rows, want)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Fatalf("rows[%d] = %+v, want %+v (full: %+v)", i, rows[i], want[i], rows)
		}
	}
}

// wordWrapFixtureText is the six-repeated-"aa"-words buffer the
// deterministic 3-row fixture above (and every TextBox-level test that
// builds on it below) shares.
const wordWrapFixtureText = "aa aa aa aa aa aa"

// TestComputeVisualRowsCharBreaksOverlongWord covers the "a single word
// wider than the content width gets character-broken" case: a run of
// identical runes with no spaces at all to word-break on, so every boundary
// must be a character break. Checked as general invariants (contiguous
// coverage, each row measuring within width) rather than exact boundaries,
// since the precise split only needs to be SOME valid character break, not
// a specific one.
func TestComputeVisualRowsCharBreaksOverlongWord(t *testing.T) {
	face := buttonFace(t)
	runes := []rune("aaaaaaaaaa") // 10 a's, no spaces anywhere
	width := face.Measure("aaa").W

	rows := computeVisualRows(runes, face, width)

	if len(rows) < 2 {
		t.Fatalf("len(rows) = %d, want > 1 (must actually wrap)", len(rows))
	}
	pos := 0
	for i, row := range rows {
		if row.start != pos {
			t.Fatalf("row %d starts at %d, want %d (contiguous: no spaces exist to drop)", i, row.start, pos)
		}
		if w := face.Measure(string(runes[row.start:row.end])).W; w > width+0.01 {
			t.Fatalf("row %d %q measures %v, want <= width %v", i, string(runes[row.start:row.end]), w, width)
		}
		pos = row.end
	}
	if pos != len(runes) {
		t.Fatalf("rows end at %d, want %d (full coverage of the buffer)", pos, len(runes))
	}
}

// TestComputeVisualRowsRealNewlineAlwaysBreaksRegardlessOfWidth proves a
// real '\n' is always a row boundary, even at a width so wide the two
// logical lines would otherwise happily share a single row.
func TestComputeVisualRowsRealNewlineAlwaysBreaksRegardlessOfWidth(t *testing.T) {
	face := buttonFace(t)
	runes := []rune("ab\ncd")
	const width = 100000 // absurdly wide: no soft wrap would ever trigger

	rows := computeVisualRows(runes, face, width)

	want := []visualRow{{start: 0, end: 2}, {start: 3, end: 5}}
	if len(rows) != len(want) || rows[0] != want[0] || rows[1] != want[1] {
		t.Fatalf("computeVisualRows(%q) = %+v, want %+v (the '\\n' at index 2 must still force a row boundary)", string(runes), rows, want)
	}
}

// TestComputeVisualRowsRowCountShrinksAsWidthGrows checks the monotonic
// shape of the layout across widths without hardcoding exact per-width
// boundaries (font-metric-sensitive): fitting the whole line at its own
// full measured width always yields exactly one row, and a narrower width
// always yields strictly more rows, with a middle width falling in between.
func TestComputeVisualRowsRowCountShrinksAsWidthGrows(t *testing.T) {
	face := buttonFace(t)
	text := "the quick brown fox jumps over the lazy dog"
	runes := []rune(text)
	full := face.Measure(text).W

	wide := computeVisualRows(runes, face, full)
	if len(wide) != 1 {
		t.Fatalf("rows at the text's own full width = %+v, want exactly 1 row", wide)
	}

	narrow := computeVisualRows(runes, face, full/4)
	if len(narrow) <= len(wide) {
		t.Fatalf("rows at width/4 = %d, want > %d (must wrap more at a narrower width)", len(narrow), len(wide))
	}

	mid := computeVisualRows(runes, face, full/2)
	if len(mid) < len(wide) || len(mid) > len(narrow) {
		t.Fatalf("rows at width/2 = %d, want between %d (full width) and %d (width/4)", len(mid), len(wide), len(narrow))
	}
}

// --- TextBox-level wrap tests: caret, selection, hit-test, height ---

// newFocusedWordWrapFixture builds a focused, multi-line, word-wrapped
// TextBox around wordWrapFixtureText at exactly the content width the
// deterministic 3-row fixture above assumes (face.Measure("aa aa").W, plus
// padding for the box's own explicit width) — row0=[0,5), row1=[6,11),
// row2=[12,17), each "aa aa". Height is generous (200) so no vertical
// scrolling comes into play, keeping buffer index and row index in lockstep
// for every test built on this fixture.
func newFocusedWordWrapFixture(t *testing.T) (*TextBox, *input.Router) {
	t.Helper()
	face := buttonFace(t)
	tb := NewTextBox(face).SetMultiline(true).SetWordWrap(true)
	boxWidth := face.Measure("aa aa").W + 2*tb.metrics.PaddingM
	const boxHeight = 200
	tb.SetWidth(boxWidth)
	tb.SetHeight(boxHeight)
	tb.SetText(wordWrapFixtureText)

	r := input.NewRouter()
	r.SetRoot(tb)
	layoutButton(tb, render.Rect{X: 0, Y: 0, W: boxWidth, H: boxHeight})
	r.Focus(tb)
	return tb, r
}

// TestTextBoxWordWrapRowColAndIndexRoundTrip locks rowCol/indexOfRowCol as
// exact inverses against the fixture's three rows, including the boundary
// indices at each soft break (5/6 and 11/12) — proving the "a caret exactly
// at a row boundary sits at the END of the upper row" tie-break rule (see
// rowCol's own doc comment) the same way TestTextBoxLineColMapping locks it
// for real newlines.
func TestTextBoxWordWrapRowColAndIndexRoundTrip(t *testing.T) {
	tb, _ := newFocusedWordWrapFixture(t)

	cases := []struct {
		idx      int
		row, col int
	}{
		{0, 0, 0},
		{4, 0, 4},
		{5, 0, 5}, // right before the dropped space: end of row0
		{6, 1, 0}, // right after the dropped space: start of row1
		{10, 1, 4},
		{11, 1, 5}, // end of row1
		{12, 2, 0}, // start of row2
		{16, 2, 4},
		{17, 2, 5}, // end of the whole text
	}
	for _, c := range cases {
		if row, col := tb.rowCol(c.idx); row != c.row || col != c.col {
			t.Fatalf("rowCol(%d) = (%d,%d), want (%d,%d)", c.idx, row, col, c.row, c.col)
		}
		if idx := tb.indexOfRowCol(c.row, c.col); idx != c.idx {
			t.Fatalf("indexOfRowCol(%d,%d) = %d, want %d", c.row, c.col, idx, c.idx)
		}
	}
}

// TestTextBoxWordWrapUpDownMoveByVisualRow is TestTextBoxUpDownPreservesDesiredColumn's
// wrapping analogue: Up/Down must move between VISUAL rows (not logical
// lines — there is only one logical line in the whole fixture) preserving
// the desired column across the moves.
func TestTextBoxWordWrapUpDownMoveByVisualRow(t *testing.T) {
	tb, r := newFocusedWordWrapFixture(t)
	tb.SetCaret(14) // row2, col2

	r.KeyDown(input.KeyUp, 0, 0)
	if row, col := tb.rowCol(tb.Caret()); row != 1 || col != 2 {
		t.Fatalf("after Up: rowCol = (%d,%d), want (1,2)", row, col)
	}
	if c := tb.Caret(); c != 8 {
		t.Fatalf("after Up: Caret() = %d, want 8 (row1 col2)", c)
	}

	r.KeyDown(input.KeyUp, 0, 0)
	if c := tb.Caret(); c != 2 {
		t.Fatalf("after 2nd Up: Caret() = %d, want 2 (row0 col2)", c)
	}

	r.KeyDown(input.KeyDown, 0, 0)
	r.KeyDown(input.KeyDown, 0, 0)
	if c := tb.Caret(); c != 14 {
		t.Fatalf("after 2x Down back: Caret() = %d, want 14 (desired col 2 restored)", c)
	}
}

// TestTextBoxWordWrapHomeEndHitVisualRowBounds is
// TestTextBoxHomeEndPerLineInMultiline's wrapping analogue: Home/End must
// target the caret's own VISUAL row, not the (single, whole-buffer) logical
// line.
func TestTextBoxWordWrapHomeEndHitVisualRowBounds(t *testing.T) {
	tb, r := newFocusedWordWrapFixture(t)
	tb.SetCaret(8) // row1, col2

	r.KeyDown(input.KeyHome, 0, 0)
	if c := tb.Caret(); c != 6 {
		t.Fatalf("Home: Caret() = %d, want 6 (start of row1, not the whole text)", c)
	}

	tb.SetCaret(8)
	r.KeyDown(input.KeyEnd, 0, 0)
	if c := tb.Caret(); c != 11 {
		t.Fatalf("End: Caret() = %d, want 11 (end of row1)", c)
	}
}

// TestTextBoxWordWrapLeftRightCrossSoftBreaksWithoutTouchingBuffer proves
// Left/Right stay purely index-based across a soft break — no buffer
// mutation, unlike Enter's hard-'\n' insertion — exactly as they already do
// across a real '\n' (see TestTextBoxLeftRightCrossLineBoundaries).
func TestTextBoxWordWrapLeftRightCrossSoftBreaksWithoutTouchingBuffer(t *testing.T) {
	tb, r := newFocusedWordWrapFixture(t)
	before := tb.Text()
	tb.SetCaret(5) // end of row0, right before the dropped space

	r.KeyDown(input.KeyRight, 0, 0)
	if c := tb.Caret(); c != 6 {
		t.Fatalf("Right over the soft break: Caret() = %d, want 6", c)
	}
	if row, col := tb.rowCol(tb.Caret()); row != 1 || col != 0 {
		t.Fatalf("after Right: rowCol = (%d,%d), want (1,0)", row, col)
	}

	r.KeyDown(input.KeyLeft, 0, 0)
	if c := tb.Caret(); c != 5 {
		t.Fatalf("Left back over the soft break: Caret() = %d, want 5", c)
	}
	if tb.Text() != before {
		t.Fatalf("Text() changed to %q, want unchanged %q (soft wrap never edits the buffer)", tb.Text(), before)
	}
}

// TestTextBoxWordWrapSelectionSpanningSoftBreakPerRow exercises the exact
// per-row intersection formula renderMultilineWrapped draws selection
// highlight with (clampInt(start,lo,hi), clampInt(end,lo,hi)) against each
// row's own [start,end) — the wrapping analogue of
// TestTextBoxMultiline's hard-newline-spanning selection golden, proven
// here numerically rather than pixel-for-pixel.
func TestTextBoxWordWrapSelectionSpanningSoftBreakPerRow(t *testing.T) {
	tb, _ := newFocusedWordWrapFixture(t)
	tb.Select(3, 8) // spans row0's tail (cols 3-5) and row1's head (cols 0-2)

	rows := tb.visualRows(tb.contentWidth())
	if len(rows) != 3 {
		t.Fatalf("test setup: len(rows) = %d, want 3", len(rows))
	}
	start, end := tb.Selection()

	if s, e := clampInt(start, rows[0].start, rows[0].end), clampInt(end, rows[0].start, rows[0].end); s != 3 || e != 5 {
		t.Fatalf("row0 selection intersection = (%d,%d), want (3,5)", s, e)
	}
	if s, e := clampInt(start, rows[1].start, rows[1].end), clampInt(end, rows[1].start, rows[1].end); s != 6 || e != 8 {
		t.Fatalf("row1 selection intersection = (%d,%d), want (6,8)", s, e)
	}
	if s, e := clampInt(start, rows[2].start, rows[2].end), clampInt(end, rows[2].start, rows[2].end); s != e {
		t.Fatalf("row2 selection intersection = (%d,%d), want empty (selection doesn't reach row2)", s, e)
	}
}

// TestTextBoxWordWrapClickHitTestLandsInWrappedRow proves a click resolves
// to the right buffer index within a wrapped row: targets row1 ("aa aa",
// rowStart=6) at column 2 (buffer index 8), using the same
// "just right of the (i,i+1) midpoint lands on i+1" convention
// TestTextBoxPressSetsCaretAtNearestGlyphBoundary already locks for the
// unwrapped case.
func TestTextBoxWordWrapClickHitTestLandsInWrappedRow(t *testing.T) {
	tb, r := newFocusedWordWrapFixture(t)
	bounds := tb.Bounds()
	pad := tb.metrics.PaddingM
	lh := tb.lineHeight()

	y := bounds.Y + pad + 1*lh + lh/2 // vertically centered within row1
	mid := (tb.xOfInRow(1, 1) + tb.xOfInRow(1, 2)) / 2
	x := bounds.X + pad + mid + 0.05 // just right of the col1/col2 midpoint

	r.PointerButton(input.ButtonLeft, true, render.Point{X: x, Y: y}, 0)

	if c := tb.Caret(); c != 8 {
		t.Fatalf("click at row1 col2: Caret() = %d, want 8", c)
	}
	if row, col := tb.rowCol(tb.Caret()); row != 1 || col != 2 {
		t.Fatalf("click: rowCol = (%d,%d), want (1,2)", row, col)
	}
}

// TestTextBoxWordWrapVerticalScrollCountsVisualRows is
// TestTextBoxVerticalScrollKeepsCaretVisible's wrapping analogue: a box
// short enough for only ~1 visual row must scroll vscroll so the caret's
// own ROW (not logical line — there is only one) stays visible, counting
// visual rows via rowCount/rowCol rather than lineCount/lineCol.
func TestTextBoxWordWrapVerticalScrollCountsVisualRows(t *testing.T) {
	face := buttonFace(t)
	tb := NewTextBox(face).SetMultiline(true).SetWordWrap(true)
	boxWidth := face.Measure("aa aa").W + 2*tb.metrics.PaddingM
	tb.SetWidth(boxWidth)
	tb.SetText(wordWrapFixtureText) // 3 visual rows
	tb.SetCaret(17)                 // end of text, row2 (the last row)

	lh := face.LineHeight()
	boxHeight := lh*1.5 + 2*tb.metrics.PaddingM // room for barely more than 1 row

	core.MeasureWidget(tb, render.Size{W: boxWidth, H: boxHeight})
	core.ArrangeWidget(tb, render.Rect{X: 0, Y: 0, W: boxWidth, H: boxHeight})

	if tb.vscroll <= 0 {
		t.Fatalf("vscroll = %v, want > 0 (short box, caret on the last of 3 visual rows)", tb.vscroll)
	}

	pad := tb.metrics.PaddingM
	innerH := tb.Bounds().H - 2*pad
	row, _ := tb.rowCol(tb.Caret())
	caretY := float32(row)*lh - tb.vscroll
	const eps = 0.01
	if caretY < -eps || caretY > innerH-lh+eps {
		t.Fatalf("caret row display y = %v, want within [0, %v]", caretY, innerH-lh)
	}
}

// TestTextBoxWordWrapMeasureContentHeightReflectsRowCount is the height
// half of the wrap feature (see MeasureContent): the desired height must be
// exactly the wrapped row count times lineHeight, plus padding — not the
// fixed textBoxMultilineDefaultLines default an unwrapped multi-line box
// reports (see TestTextBoxMultilineDesiredHeightIsTaller).
func TestTextBoxWordWrapMeasureContentHeightReflectsRowCount(t *testing.T) {
	face := buttonFace(t)
	tb := NewTextBox(face).SetMultiline(true).SetWordWrap(true)
	boxWidth := face.Measure("aa aa").W + 2*tb.metrics.PaddingM
	tb.SetWidth(boxWidth)
	tb.SetText(wordWrapFixtureText) // wraps into exactly 3 rows at this width

	core.MeasureWidget(tb, render.Size{W: 1000, H: 1000}) // available is irrelevant: explicit width wins
	d := core.DesiredSizeOf(tb)

	wantH := face.LineHeight()*3 + 2*tb.metrics.PaddingM
	if d.H != wantH {
		t.Fatalf("DesiredSize().H = %v, want %v (3 wrapped rows + padding)", d.H, wantH)
	}
	if d.W != boxWidth {
		t.Fatalf("DesiredSize().W = %v, want %v (explicit SetWidth still honored)", d.W, boxWidth)
	}
}

// TestTextBoxWordWrapMeasureContentUsesAvailableWidthWithoutExplicitSetWidth
// proves MeasureContent actually consults `available` while wrapping (the
// one case where it does — see its own doc comment) when there is no
// explicit SetWidth to override it: the wrap width comes straight from
// available.W.
func TestTextBoxWordWrapMeasureContentUsesAvailableWidthWithoutExplicitSetWidth(t *testing.T) {
	face := buttonFace(t)
	tb := NewTextBox(face).SetMultiline(true).SetWordWrap(true)
	tb.SetText(wordWrapFixtureText)

	contentW := face.Measure("aa aa").W
	available := render.Size{W: contentW + 2*tb.metrics.PaddingM, H: 1000}

	core.MeasureWidget(tb, available)
	d := core.DesiredSizeOf(tb)

	wantH := face.LineHeight()*3 + 2*tb.metrics.PaddingM
	if d.H != wantH {
		t.Fatalf("DesiredSize().H = %v, want %v (wraps against the AVAILABLE width)", d.H, wantH)
	}
}

// TestTextBoxWordWrapMeasureContentFallsBackOnInfiniteAvailableWidth is the
// Inf-safety regression for wrapMeasureWidth: an unconstrained (+Inf)
// available.W (e.g. a container offering infinite cross-axis space) must
// still produce a finite desired size rather than wrapping against
// infinity.
func TestTextBoxWordWrapMeasureContentFallsBackOnInfiniteAvailableWidth(t *testing.T) {
	tb := NewTextBox(buttonFace(t)).SetMultiline(true).SetWordWrap(true)
	tb.SetText("hello")

	core.MeasureWidget(tb, render.Size{W: float32(math.Inf(1)), H: 1000})
	d := core.DesiredSizeOf(tb)

	if math.IsInf(float64(d.W), 1) || math.IsInf(float64(d.H), 1) {
		t.Fatalf("DesiredSize() = %+v, want finite (Inf-safe fallback to textBoxDefaultWidth)", d)
	}
}

// newFocusedCharBreakFixture builds a focused, multi-line, word-wrapped
// TextBox around a single over-long word with no spaces ("aaaaaaaaaa"), at a
// content width sized to fit exactly three glyphs (face.Measure("aaa").W).
// Because there is no space to break on, wrapLogicalLine character-breaks it
// (cases 3/4), yielding CONTIGUOUS rows [0,3)[3,6)[6,9)[9,10) — each row's
// end is the next row's start, unlike the dropped-wrap-space fixture whose
// rows have a one-index gap. This is the fixture the contiguous-boundary
// affinity tests below need (the shared word-wrap fixture only exercises gap
// boundaries, which is why the affinity bug went unnoticed).
func newFocusedCharBreakFixture(t *testing.T) (*TextBox, *input.Router) {
	t.Helper()
	face := buttonFace(t)
	tb := NewTextBox(face).SetMultiline(true).SetWordWrap(true)
	boxWidth := face.Measure("aaa").W + 2*tb.metrics.PaddingM
	const boxHeight = 200
	tb.SetWidth(boxWidth)
	tb.SetHeight(boxHeight)
	tb.SetText("aaaaaaaaaa")

	r := input.NewRouter()
	r.SetRoot(tb)
	layoutButton(tb, render.Rect{X: 0, Y: 0, W: boxWidth, H: boxHeight})
	r.Focus(tb)

	rows := tb.visualRows(tb.contentWidth())
	if len(rows) < 2 || rows[0].end != rows[1].start {
		t.Fatalf("test setup: rows = %+v, want contiguous char-break rows (rows[0].end == rows[1].start)", rows)
	}
	return tb, r
}

// TestTextBoxWordWrapContiguousBoundaryStartAffinity locks the fix for the
// contiguous-wrap boundary: an index that is the START of a char-broken
// continuation row must map to that row's head (col 0), not the far-right
// END of the row above. rows[1].start (== rows[0].end == 3) is exactly such
// an index. Draw (rowCol), Home, Down, and CaretScreenRect must all agree it
// sits at row 1 col 0.
func TestTextBoxWordWrapContiguousBoundaryStartAffinity(t *testing.T) {
	tb, r := newFocusedCharBreakFixture(t)
	rows := tb.visualRows(tb.contentWidth())
	boundary := rows[1].start // == rows[0].end: ambiguous contiguous index

	// rowCol: start-of-continuation affinity, not end-of-upper-row.
	if row, col := tb.rowCol(boundary); row != 1 || col != 0 {
		t.Fatalf("rowCol(%d) = (%d,%d), want (1,0) (start of the continuation row, not the end of row 0)", boundary, row, col)
	}

	// Draw / hit-test round-trip: a click at the head of row 1 resolves to
	// the same index, and rowCol maps it back to (1,0).
	bounds := tb.Bounds()
	pad := tb.metrics.PaddingM
	lh := tb.lineHeight()
	y := bounds.Y + pad + 1*lh + lh/2 // vertically centered within row 1
	x := bounds.X + pad               // far left of row 1
	r.PointerButton(input.ButtonLeft, true, render.Point{X: x, Y: y}, 0)
	if c := tb.Caret(); c != boundary {
		t.Fatalf("click at row1 head: Caret() = %d, want %d", c, boundary)
	}
	if row, col := tb.rowCol(tb.Caret()); row != 1 || col != 0 {
		t.Fatalf("click round-trip: rowCol = (%d,%d), want (1,0)", row, col)
	}

	// Home from within row 1 stops at the row's own start (the boundary),
	// not the whole text's start.
	tb.SetCaret(rows[1].start + 1) // row 1, col 1
	r.KeyDown(input.KeyHome, 0, 0)
	if c := tb.Caret(); c != boundary {
		t.Fatalf("Home from within row 1: Caret() = %d, want %d (row 1 start)", c, boundary)
	}

	// Down from row 0 lands at row 1's start (col 0 desired), on row 1.
	tb.SetCaret(0)
	r.KeyDown(input.KeyDown, 0, 0)
	if c := tb.Caret(); c != boundary {
		t.Fatalf("Down from row 0 col 0: Caret() = %d, want %d (row 1 start)", c, boundary)
	}
	if row, col := tb.rowCol(tb.Caret()); row != 1 || col != 0 {
		t.Fatalf("Down: rowCol = (%d,%d), want (1,0)", row, col)
	}

	// CaretScreenRect returns row 1's rect (one line-height down, far left),
	// not row 0's far-right end.
	tb.SetCaret(boundary)
	got, ok := tb.CaretScreenRect()
	if !ok {
		t.Fatal("CaretScreenRect() ok = false while focused, want true")
	}
	wantX := bounds.X + pad + tb.gutterWidth() - tb.hscroll + tb.xOfInRow(1, 0)
	wantY := bounds.Y + pad - tb.vscroll + 1*lh
	if got.X != wantX || got.Y != wantY || got.H != lh {
		t.Fatalf("CaretScreenRect() = %+v, want X=%v Y=%v H=%v (row 1 col 0)", got, wantX, wantY, lh)
	}
	// Guard against the specific regression: the caret must NOT be reported a
	// whole line-height up on row 0.
	row0Y := bounds.Y + pad - tb.vscroll + 0*lh
	if got.Y == row0Y {
		t.Fatalf("CaretScreenRect().Y = %v is on row 0; the contiguous-boundary caret must render on row 1", got.Y)
	}
}

// TestTextBoxCaretScreenRectWrappingComposingUsesLogicalCoords locks the fix
// for the IME-anchor coordinate mismatch: while composing, renderMultiline
// draws the caret via renderComposingMultiline in LOGICAL-line coordinates
// (regardless of wrapping), so CaretScreenRect must report the rect in that
// same space — matching the drawn caret exactly, not the visual-row space it
// would use outside composition. The box is tall/narrow enough that the
// committed text occupies several visual rows, so a visual-row rect would be
// offset from the logical-line rect by whole line-heights.
func TestTextBoxCaretScreenRectWrappingComposingUsesLogicalCoords(t *testing.T) {
	tb, r := newFocusedCharBreakFixture(t) // "aaaaaaaaaa", 4 visual rows, 1 logical line
	tb.SetCaret(len([]rune("aaaaaaaaaa")))

	// Begin an IME composition. renderMultiline now routes drawing through
	// renderComposingMultiline (logical-line coordinates).
	r.CompositionUpdate("ん", 1)
	if !tb.composing {
		t.Fatal("composing = false after CompositionUpdate, want true")
	}

	got, ok := tb.CaretScreenRect()
	if !ok {
		t.Fatal("CaretScreenRect() ok = false while focused, want true")
	}

	// Expected rect == exactly what renderComposingMultiline draws: logical
	// (line, col) via lineCol/xOfInLine, plus the preedit-caret offset.
	bounds := tb.Bounds()
	pad := tb.metrics.PaddingM
	lh := tb.lineHeight()
	line, col := tb.lineCol(tb.caret)
	textX := bounds.X + pad + tb.gutterWidth() - tb.hscroll
	wantX := textX + tb.xOfInLine(line, col) + tb.preeditMeasure(tb.preeditCaret)
	wantY := bounds.Y + pad - tb.vscroll + float32(line)*lh
	if got.X != wantX || got.Y != wantY || got.H != lh {
		t.Fatalf("CaretScreenRect() while composing = %+v, want X=%v Y=%v H=%v (logical-line coords)", got, wantX, wantY, lh)
	}

	// The committed text is a single logical line (line 0), so the composing
	// caret's Y must be the top row — NOT offset down by the visual rows the
	// wrapped layout would have placed it on. Prove it differs from the
	// visual-row Y CaretScreenRect would report outside composition.
	visRow, _ := tb.rowCol(tb.caret)
	visualY := bounds.Y + pad - tb.vscroll + float32(visRow)*lh
	if visRow == 0 {
		t.Fatalf("test setup: caret's visual row = 0, want a lower row so the offset is observable")
	}
	if got.Y == visualY {
		t.Fatalf("CaretScreenRect().Y = %v equals the visual-row Y; while composing it must use the logical-line Y %v (offset by %d line-heights)", got.Y, wantY, visRow)
	}
}

// --- Vertical scroll thumb (shown only while content overflows) ---

// newOverflowingMultilineTextBox builds a focused, multi-line (word-wrap
// OFF) TextBox with ten short lines in a box short enough (50px tall) that
// only a few can be visible at once — the shared fixture for the vertical
// scroll thumb tests below. The caret sits at the very end (SetText's own
// caret-to-end convention), so vscroll clamps near its maximum.
func newOverflowingMultilineTextBox(t *testing.T) *TextBox {
	t.Helper()
	tb := NewTextBox(buttonFace(t)).SetMultiline(true)
	tb.SetText("l0\nl1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9")
	tb.SetWidth(200)
	tb.SetHeight(50)

	core.MeasureWidget(tb, render.Size{W: 200, H: 50})
	core.ArrangeWidget(tb, render.Rect{X: 0, Y: 0, W: 200, H: 50})
	return tb
}

// TestTextBoxVScrollThumbHiddenWhenContentFits is the threshold's "hidden"
// half: a short box with content that easily fits must show no thumb, and
// must reserve no gutter (contentWidth stays exactly fullContentWidth) —
// the "byte-for-byte unchanged when non-overflowing" invariant, checked
// directly rather than only via a golden.
func TestTextBoxVScrollThumbHiddenWhenContentFits(t *testing.T) {
	tb := NewTextBox(buttonFace(t)).SetMultiline(true)
	tb.SetText("line one\nline two")
	tb.SetWidth(200)
	tb.SetHeight(200) // generous: content easily fits

	core.MeasureWidget(tb, render.Size{W: 200, H: 200})
	core.ArrangeWidget(tb, render.Rect{X: 0, Y: 0, W: 200, H: 200})

	if tb.vScrollShown {
		t.Fatal("vScrollShown = true for content that fits, want false")
	}
	if _, ok := tb.vScrollTrack(); ok {
		t.Fatal("vScrollTrack ok = true for content that fits, want false (no thumb)")
	}
	if got, want := tb.contentWidth(), tb.fullContentWidth(); got != want {
		t.Fatalf("contentWidth() = %v, want %v (== fullContentWidth: no gutter reserved when not overflowing)", got, want)
	}
}

// TestTextBoxVScrollThumbShownWhenContentOverflows is the threshold's
// "shown" half: content taller than the viewport must show the thumb, with
// a track inset by PaddingM on all sides (matching the text's own inset)
// and scrollGutter wide.
func TestTextBoxVScrollThumbShownWhenContentOverflows(t *testing.T) {
	tb := newOverflowingMultilineTextBox(t)

	if !tb.vScrollShown {
		t.Fatal("vScrollShown = false for overflowing content, want true")
	}
	track, ok := tb.vScrollTrack()
	if !ok {
		t.Fatal("vScrollTrack ok = false, want true (content overflows the viewport)")
	}
	bounds := tb.Bounds()
	pad := tb.metrics.PaddingM
	gutter := tb.metrics.ScrollGutter
	want := render.Rect{X: bounds.Right() - pad - gutter, Y: bounds.Y + pad, W: gutter, H: bounds.H - 2*pad}
	if track != want {
		t.Fatalf("vScrollTrack() = %+v, want %+v", track, want)
	}
}

// TestTextBoxVScrollGutterOnlyReservedWhenShown proves the gutter's
// reservation is conditional (see computeShowVScroll/ArrangeContent): exactly
// fullContentWidth minus the gutter once the thumb is shown, checked against
// the overflowing fixture (the fits-case half of this invariant is already
// covered by TestTextBoxVScrollThumbHiddenWhenContentFits).
func TestTextBoxVScrollGutterOnlyReservedWhenShown(t *testing.T) {
	tb := newOverflowingMultilineTextBox(t)

	want := tb.fullContentWidth() - tb.metrics.ScrollGutter
	if got := tb.contentWidth(); got != want {
		t.Fatalf("contentWidth() = %v, want %v (fullContentWidth minus the gutter, once shown)", got, want)
	}
}

// TestTextBoxVScrollThumbGeometryReflectsProportionAndOffset checks the
// thumb's size and position against the exact same shared formulas
// (scrollThumbLength/scrollThumbPos, scrollviewer.go) it's built from, then
// proves it actually tracks vscroll: moving the caret to the very start and
// re-arranging must snap vscroll to 0 and the thumb to the top of the track.
func TestTextBoxVScrollThumbGeometryReflectsProportionAndOffset(t *testing.T) {
	tb := newOverflowingMultilineTextBox(t) // caret at the end: vscroll near max

	track, ok := tb.vScrollTrack()
	if !ok {
		t.Fatal("vScrollTrack ok = false, want true")
	}
	thumb, ok := tb.vScrollThumbRect()
	if !ok {
		t.Fatal("vScrollThumbRect ok = false, want true")
	}

	total := tb.totalContentHeight()
	wantH := scrollThumbLength(track.H, total)
	if thumb.H != wantH {
		t.Fatalf("thumb.H = %v, want %v (scrollThumbLength)", thumb.H, wantH)
	}
	maxOffset := total - track.H
	wantY := scrollThumbPos(track.Y, track.H, thumb.H, tb.vscroll, maxOffset)
	if thumb.Y != wantY {
		t.Fatalf("thumb.Y = %v, want %v (scrollThumbPos at the current vscroll)", thumb.Y, wantY)
	}

	tb.SetCaret(0)
	core.ArrangeWidget(tb, tb.Bounds())
	if tb.vscroll != 0 {
		t.Fatalf("vscroll after caret-to-start = %v, want 0", tb.vscroll)
	}
	thumb2, ok := tb.vScrollThumbRect()
	if !ok {
		t.Fatal("vScrollThumbRect ok = false after caret-to-start, want true")
	}
	if thumb2.Y != track.Y {
		t.Fatalf("thumb.Y after caret-to-start = %v, want %v (top of the track)", thumb2.Y, track.Y)
	}
}

// TestTextBoxVScrollThumbDragScrolls is the draggability requirement: a
// Press inside the thumb captures the pointer and sets vDragging (instead
// of the usual caret placement), a Move while captured scrolls via
// dragVScroll WITHOUT moving the caret, and Release ends the drag and
// clears the router's capture.
func TestTextBoxVScrollThumbDragScrolls(t *testing.T) {
	tb := NewTextBox(buttonFace(t)).SetMultiline(true)
	tb.SetText("l0\nl1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9")
	tb.SetWidth(200)
	tb.SetHeight(50)

	r := input.NewRouter()
	r.SetRoot(tb)
	layoutButton(tb, render.Rect{X: 0, Y: 0, W: 200, H: 50})
	r.Focus(tb)

	caretBefore := tb.Caret()
	track, ok := tb.vScrollTrack()
	if !ok {
		t.Fatal("vScrollTrack ok = false, want true (test setup: content must overflow)")
	}
	thumb, ok := tb.vScrollThumbRect()
	if !ok {
		t.Fatal("vScrollThumbRect ok = false, want true")
	}

	press := render.Point{X: thumb.X + thumb.W/2, Y: thumb.Y + thumb.H/2}
	r.PointerButton(input.ButtonLeft, true, press, 0)
	if !tb.vDragging {
		t.Fatal("vDragging = false after pressing the thumb, want true")
	}
	if got := r.Captured(); got != core.Widget(tb) {
		t.Fatalf("Captured() after pressing the thumb = %v, want tb", got)
	}

	r.PointerMove(render.Point{X: press.X, Y: track.Y}, 0) // drag to the very top
	if tb.vscroll != 0 {
		t.Fatalf("vscroll after dragging the thumb to the top = %v, want 0", tb.vscroll)
	}
	if tb.Caret() != caretBefore {
		t.Fatalf("Caret() changed by a thumb drag = %d, want unchanged %d (a thumb drag must not move the caret)", tb.Caret(), caretBefore)
	}

	r.PointerButton(input.ButtonLeft, false, render.Point{X: press.X, Y: track.Y}, 0)
	if tb.vDragging {
		t.Fatal("vDragging = true after Release, want false")
	}
	if got := r.Captured(); got != nil {
		t.Fatalf("Captured() after Release = %v, want nil", got)
	}
}

// TestTextBoxVScrollSingleLineNeverShowsThumb locks the single-line carve-out:
// vScrollShown must stay false (and the thumb absent) regardless of content,
// since single-line TextBox has no vertical axis to scroll at all.
func TestTextBoxVScrollSingleLineNeverShowsThumb(t *testing.T) {
	tb := NewTextBox(buttonFace(t)) // no SetMultiline
	tb.SetText("hello")
	tb.SetWidth(50)
	tb.SetHeight(10) // deliberately too short for even one line

	core.MeasureWidget(tb, render.Size{W: 50, H: 10})
	core.ArrangeWidget(tb, render.Rect{X: 0, Y: 0, W: 50, H: 10})

	if tb.vScrollShown {
		t.Fatal("vScrollShown = true for a single-line TextBox, want false")
	}
	if _, ok := tb.vScrollTrack(); ok {
		t.Fatal("vScrollTrack ok = true for a single-line TextBox, want false")
	}
}

// --- Tab-inserts-indent (opt-in, multi-line only; see SetTabInserts) ---

// TestTextBoxTabInsertsDefaultFalseAndSetter locks SetTabInserts/TabInserts'
// own getter/setter contract, mirroring TestTextBoxMultilineDefaultFalseAndSetter.
func TestTextBoxTabInsertsDefaultFalseAndSetter(t *testing.T) {
	tb := NewTextBox(nil)
	if tb.TabInserts() {
		t.Fatal("TabInserts() = true for a fresh TextBox, want false (default)")
	}
	tb.SetTabInserts(true)
	if !tb.TabInserts() {
		t.Fatal("TabInserts() = false after SetTabInserts(true), want true")
	}
}

// TestTextBoxTabDefaultBubblesToFocusNav locks the unchanged-by-default
// requirement: with SetTabInserts never called, Tab on a focused multiline
// TextBox is NOT consumed by the box itself (dispatchKey sees e.Handled still
// false) and inserts nothing — it bubbles to the Router's own Tab
// focus-cycling bookkeeping exactly as it did before this feature existed
// (see TestKeyDownConsumedTabBookkeeping's matching "consumed = false"
// convention).
func TestTextBoxTabDefaultBubblesToFocusNav(t *testing.T) {
	tb, r := newFocusedMultilineTextBox(t, "ab")
	tb.SetCaret(1)

	var calls int
	tb.OnChanged(func(string) { calls++ })

	if consumed := r.KeyDown(input.KeyTab, 0, 0); consumed {
		t.Fatal("KeyDown(Tab) consumed = true with SetTabInserts never called, want false (unchanged)")
	}
	if tb.Text() != "ab" {
		t.Fatalf("Text() = %q, want unchanged %q", tb.Text(), "ab")
	}
	if c := tb.Caret(); c != 1 {
		t.Fatalf("Caret() = %d, want unchanged 1", c)
	}
	if calls != 0 {
		t.Fatalf("OnChanged calls = %d, want 0", calls)
	}
}

// TestTextBoxTabInsertsSpacesWhenEnabled locks the core Tab-inserts-indent
// contract: enabled, multiline, and focused, Tab inserts exactly
// tabInsertSpaces spaces at the caret via the same path plain typing uses
// (advancing the caret, firing OnChanged), and is CONSUMED so it never
// reaches the Router's focus-cycling. A second Tab press keeps inserting.
func TestTextBoxTabInsertsSpacesWhenEnabled(t *testing.T) {
	tb, r := newFocusedMultilineTextBox(t, "ab")
	tb.SetTabInserts(true)
	tb.SetCaret(1) // between 'a' and 'b'

	var got []string
	tb.OnChanged(func(s string) { got = append(got, s) })

	if consumed := r.KeyDown(input.KeyTab, 0, 0); !consumed {
		t.Fatal("KeyDown(Tab) consumed = false with SetTabInserts(true), want true")
	}
	want := "a" + strings.Repeat(" ", tabInsertSpaces) + "b"
	if tb.Text() != want {
		t.Fatalf("Text() after one Tab = %q, want %q", tb.Text(), want)
	}
	if c := tb.Caret(); c != 1+tabInsertSpaces {
		t.Fatalf("Caret() after one Tab = %d, want %d", c, 1+tabInsertSpaces)
	}

	// A second Tab press keeps inserting at the (now advanced) caret.
	r.KeyDown(input.KeyTab, 0, 0)
	want2 := "a" + strings.Repeat(" ", 2*tabInsertSpaces) + "b"
	if tb.Text() != want2 {
		t.Fatalf("Text() after two Tabs = %q, want %q", tb.Text(), want2)
	}
	if len(got) != 2 {
		t.Fatalf("OnChanged calls = %d, want 2 (one per Tab press)", len(got))
	}
}

// TestTextBoxTabInsertsReplacesSelection locks the "replace an active
// selection like typing would" requirement, mirroring
// TestTextBoxTypingWithSelectionReplaces.
func TestTextBoxTabInsertsReplacesSelection(t *testing.T) {
	tb, r := newFocusedMultilineTextBox(t, "hello")
	tb.SetTabInserts(true)
	tb.Select(1, 4) // "ell" selected

	r.KeyDown(input.KeyTab, 0, 0)

	want := "h" + strings.Repeat(" ", tabInsertSpaces) + "o"
	if tb.Text() != want {
		t.Fatalf("Text() = %q, want %q (selection replaced by Tab's inserted spaces)", tb.Text(), want)
	}
	if c := tb.Caret(); c != 1+tabInsertSpaces {
		t.Fatalf("Caret() = %d, want %d (just after the replacement)", c, 1+tabInsertSpaces)
	}
	if s, e := tb.Selection(); s != e {
		t.Fatalf("Selection() = (%d,%d), want collapsed after replace", s, e)
	}
}

// TestTextBoxTabInsertsSingleLineStillBubbles locks the multiline-only
// gating: SetTabInserts(true) on a SINGLE-LINE box must leave Tab exactly as
// unhandled as the default case (Tab remains the Router's focus-nav key for
// a single-line input, since indenting one has no sensible meaning).
func TestTextBoxTabInsertsSingleLineStillBubbles(t *testing.T) {
	tb, r := newFocusedTextBox(t, "ab")
	tb.SetTabInserts(true)
	tb.SetCaret(1)

	if consumed := r.KeyDown(input.KeyTab, 0, 0); consumed {
		t.Fatal("KeyDown(Tab) consumed = true on a single-line TextBox with SetTabInserts(true), want false")
	}
	if tb.Text() != "ab" {
		t.Fatalf("Text() = %q, want unchanged %q", tb.Text(), "ab")
	}
}

// TestTextBoxShiftTabUnindentsLeadingSpaces locks the Shift+Tab unindent
// step: it removes up to tabInsertSpaces leading spaces from the caret's
// current line, is a no-op mutation (but still handled — see OnKey's own
// "recognized combos are always handled, even as a no-op" convention) when
// there are none, and only ever removes a SHORTER leading run when the line
// has fewer than tabInsertSpaces leading spaces.
func TestTextBoxShiftTabUnindentsLeadingSpaces(t *testing.T) {
	indent := strings.Repeat(" ", tabInsertSpaces)
	tb, r := newFocusedMultilineTextBox(t, indent+"line one\n"+"ab\n"+"  c")
	tb.SetTabInserts(true)

	// Caret at the end of the fully-indented first line: removes the whole
	// leading run and walks the caret back by exactly that many runes.
	tb.SetCaret(len(indent) + len("line one"))
	if consumed := r.KeyDown(input.KeyTab, 0, input.ModShift); !consumed {
		t.Fatal("KeyDown(Shift+Tab) consumed = false with a full leading indent present, want true")
	}
	if got, want := tb.Text(), "line one\nab\n  c"; got != want {
		t.Fatalf("Text() after Shift+Tab = %q, want %q", got, want)
	}
	if c := tb.Caret(); c != len("line one") {
		t.Fatalf("Caret() after Shift+Tab = %d, want %d", c, len("line one"))
	}

	// Second line ("ab") has no leading spaces at all: a no-op mutation, but
	// still marked handled (never leaks through to focus-nav).
	tb.SetCaret(len("line one\n") + 1) // inside "ab"
	if consumed := r.KeyDown(input.KeyTab, 0, input.ModShift); !consumed {
		t.Fatal("KeyDown(Shift+Tab) consumed = false on a line with no leading spaces, want true (recognized no-op)")
	}
	if got, want := tb.Text(), "line one\nab\n  c"; got != want {
		t.Fatalf("Text() after no-op Shift+Tab = %q, want unchanged %q", got, want)
	}

	// Third line ("  c") has fewer than tabInsertSpaces (2 < 4) leading
	// spaces: only that shorter run is removed.
	third := len("line one\n") + len("ab\n")
	tb.SetCaret(third + 2) // just after the two leading spaces, before 'c'
	r.KeyDown(input.KeyTab, 0, input.ModShift)
	if got, want := tb.Text(), "line one\nab\nc"; got != want {
		t.Fatalf("Text() after Shift+Tab on a short indent = %q, want %q", got, want)
	}
	if c := tb.Caret(); c != third {
		t.Fatalf("Caret() after Shift+Tab on a short indent = %d, want %d (line start)", c, third)
	}
}

// TestTextBoxTabInsertsDoesNotAffectSetText locks the "programmatic
// mutation unaffected" requirement: SetText's caret/selection reset happens
// exactly as it always has, regardless of SetTabInserts.
func TestTextBoxTabInsertsDoesNotAffectSetText(t *testing.T) {
	tb := NewTextBox(buttonFace(t)).SetMultiline(true)
	tb.SetTabInserts(true)

	tb.SetText("x\ny")
	if got := tb.Text(); got != "x\ny" {
		t.Fatalf("Text() = %q, want %q", got, "x\ny")
	}
	want := len([]rune("x\ny"))
	if c := tb.Caret(); c != want {
		t.Fatalf("Caret() after SetText = %d, want %d (end)", c, want)
	}
}

// --- Tab block-indent/outdent for a multi-line selection ---
//
// A selection spanning more than one logical line is a "block selection"
// (see blockSelection): Tab/Shift+Tab indent/outdent every touched line in
// place (indentSelectedLines/outdentSelectedLines) instead of running the
// single-line path (insertText's selection-replace, which would otherwise
// delete the whole selected block) or unindentCurrentLine (which would only
// touch the caret's own line).

// TestTextBoxTabBlockIndentsEveryTouchedLine locks the core block-indent
// contract: a selection spanning three logical lines gains tabInsertSpaces
// leading spaces on EVERY touched line, no existing text is deleted, and the
// selection is restored to still span the whole block (so a second Tab press
// keeps indenting further).
func TestTextBoxTabBlockIndentsEveryTouchedLine(t *testing.T) {
	tb, r := newFocusedMultilineTextBox(t, "aaa\nbbb\nccc")
	tb.SetTabInserts(true)
	tb.Select(1, 9) // spans all three lines without covering any of them fully

	if consumed := r.KeyDown(input.KeyTab, 0, 0); !consumed {
		t.Fatal("KeyDown(Tab) consumed = false with a multi-line selection, want true")
	}
	want := "    aaa\n    bbb\n    ccc"
	if got := tb.Text(); got != want {
		t.Fatalf("Text() after block Tab = %q, want %q (every line indented, nothing deleted)", got, want)
	}
	wantRunes := len([]rune(want))
	if s, e := tb.Selection(); s != 0 || e != wantRunes {
		t.Fatalf("Selection() after block Tab = (%d,%d), want (0,%d) (whole block reselected)", s, e, wantRunes)
	}

	// A second Tab press keeps indenting: the restored selection still spans
	// all three lines, so it is routed through the block path again.
	r.KeyDown(input.KeyTab, 0, 0)
	want2 := "        aaa\n        bbb\n        ccc"
	if got := tb.Text(); got != want2 {
		t.Fatalf("Text() after second block Tab = %q, want %q", got, want2)
	}
}

// TestTextBoxShiftTabBlockOutdentsMixedIndentation locks the block-outdent
// contract with three differently-indented lines: a full tabInsertSpaces
// run, a shorter run, and no leading spaces at all — each loses only what it
// actually has, ending at zero for all three, with no crash on the
// already-unindented line.
func TestTextBoxShiftTabBlockOutdentsMixedIndentation(t *testing.T) {
	indent := strings.Repeat(" ", tabInsertSpaces)
	tb, r := newFocusedMultilineTextBox(t, indent+"aaa\n  bbb\nccc")
	tb.SetTabInserts(true)
	tb.Select(0, len([]rune(indent+"aaa\n  bbb\nccc")))

	if consumed := r.KeyDown(input.KeyTab, 0, input.ModShift); !consumed {
		t.Fatal("KeyDown(Shift+Tab) consumed = false with a multi-line selection, want true")
	}
	want := "aaa\nbbb\nccc"
	if got := tb.Text(); got != want {
		t.Fatalf("Text() after block Shift+Tab = %q, want %q", got, want)
	}
	wantRunes := len([]rune(want))
	if s, e := tb.Selection(); s != 0 || e != wantRunes {
		t.Fatalf("Selection() after block Shift+Tab = (%d,%d), want (0,%d) (whole block reselected)", s, e, wantRunes)
	}
}

// TestTextBoxTabBlockSelectionEndAtLineStartExcludesTrailingLine documents
// the touchedLines convention: when the selection's END sits exactly at the
// start (column 0) of a line — i.e. it runs through the previous line's
// newline without selecting any of THIS line's own text — that trailing
// line is excluded from the block indent, even though the selection is
// still a two-line block selection (it spans a '\n').
func TestTextBoxTabBlockSelectionEndAtLineStartExcludesTrailingLine(t *testing.T) {
	tb, r := newFocusedMultilineTextBox(t, "aaa\nbbb\nccc")
	tb.SetTabInserts(true)
	tb.Select(0, len([]rune("aaa\n"))) // through line 0's newline, into col 0 of line 1

	r.KeyDown(input.KeyTab, 0, 0)

	want := "    aaa\nbbb\nccc"
	if got := tb.Text(); got != want {
		t.Fatalf("Text() = %q, want %q (only line 0 indented, line 1 untouched)", got, want)
	}
	wantRunes := len([]rune("    aaa"))
	if s, e := tb.Selection(); s != 0 || e != wantRunes {
		t.Fatalf("Selection() = (%d,%d), want (0,%d) (reselects only the touched line)", s, e, wantRunes)
	}
}

// TestTextBoxTabSingleLineSelectionInMultiLineBufferStillReplaces locks that
// a selection confined to ONE logical line — even inside a buffer that has
// other lines — is NOT a block selection: Tab still replaces the selected
// text with spaces exactly as TestTextBoxTabInsertsReplacesSelection checks
// for a buffer with no newlines at all.
func TestTextBoxTabSingleLineSelectionInMultiLineBufferStillReplaces(t *testing.T) {
	tb, r := newFocusedMultilineTextBox(t, "aaa\nbbb\nccc")
	tb.SetTabInserts(true)
	tb.Select(4, 7) // "bbb", entirely within line 1

	r.KeyDown(input.KeyTab, 0, 0)

	want := "aaa\n" + strings.Repeat(" ", tabInsertSpaces) + "\nccc"
	if got := tb.Text(); got != want {
		t.Fatalf("Text() = %q, want %q (single-line selection replaced, not block-indented)", got, want)
	}
}

// TestTextBoxTabBlockIndentFiresOnChangedOnce locks the indentSelectedLines
// coalescing fix: a block Tab press touching three lines used to call
// replaceRange (and therefore fire OnChanged) once PER line; it now applies
// the whole block as a single edit, so exactly one OnChanged call comes out
// of one Tab press, regardless of how many lines it touches.
func TestTextBoxTabBlockIndentFiresOnChangedOnce(t *testing.T) {
	tb, r := newFocusedMultilineTextBox(t, "aaa\nbbb\nccc")
	tb.SetTabInserts(true)
	tb.Select(1, 9)

	var got []string
	tb.OnChanged(func(s string) { got = append(got, s) })

	r.KeyDown(input.KeyTab, 0, 0)

	want := "    aaa\n    bbb\n    ccc"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("OnChanged calls = %v, want exactly [%q]", got, want)
	}
}

// TestTextBoxShiftTabBlockOutdentFiresOnChangedOnce is
// TestTextBoxTabBlockIndentFiresOnChangedOnce's outdent counterpart.
func TestTextBoxShiftTabBlockOutdentFiresOnChangedOnce(t *testing.T) {
	indent := strings.Repeat(" ", tabInsertSpaces)
	tb, r := newFocusedMultilineTextBox(t, indent+"aaa\n  bbb\nccc")
	tb.SetTabInserts(true)
	tb.Select(0, len([]rune(indent+"aaa\n  bbb\nccc")))

	var got []string
	tb.OnChanged(func(s string) { got = append(got, s) })

	r.KeyDown(input.KeyTab, 0, input.ModShift)

	want := "aaa\nbbb\nccc"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("OnChanged calls = %v, want exactly [%q]", got, want)
	}
}

// TestTextBoxShiftTabBlockOutdentNoLeadingSpacesIsNoop locks the coalesced
// outdentSelectedLines' no-op path: when NO touched line has any leading
// space to remove, the block edit must be a genuine no-op — no replaceRange,
// no OnChanged — exactly like the original per-line loop (which never called
// replaceRange when a line had nothing to strip), while still restoring the
// selection to span the touched block.
func TestTextBoxShiftTabBlockOutdentNoLeadingSpacesIsNoop(t *testing.T) {
	tb, r := newFocusedMultilineTextBox(t, "aaa\nbbb\nccc")
	tb.SetTabInserts(true)
	tb.Select(0, len([]rune("aaa\nbbb\nccc")))

	var calls int
	tb.OnChanged(func(string) { calls++ })

	r.KeyDown(input.KeyTab, 0, input.ModShift)

	if calls != 0 {
		t.Fatalf("OnChanged calls = %d, want 0 (nothing to outdent)", calls)
	}
	if tb.Text() != "aaa\nbbb\nccc" {
		t.Fatalf("Text() = %q, want unchanged %q", tb.Text(), "aaa\nbbb\nccc")
	}
	wantRunes := len([]rune("aaa\nbbb\nccc"))
	if s, e := tb.Selection(); s != 0 || e != wantRunes {
		t.Fatalf("Selection() = (%d,%d), want (0,%d) (still restored to the touched block)", s, e, wantRunes)
	}
}

// --- Line-number gutter (opt-in; see TextBox.SetLineNumbers) ---

// newGutterTextBox returns a multi-line TextBox holding s, laid out at a
// fixed 200x120 rect (tall and wide enough that the sample strings below
// neither overflow vertically — no vScroll gutter competing for width — nor
// scroll horizontally), focused so the caret is actually drawn, with the
// line-number gutter set per lineNumbers. The shared fixture for every
// gutter test below.
func newGutterTextBox(t *testing.T, s string, lineNumbers bool) *TextBox {
	t.Helper()
	tb := NewTextBox(buttonFace(t)).SetMultiline(true).SetLineNumbers(lineNumbers)
	tb.SetText(s)
	tb.SetWidth(200)
	tb.SetHeight(120)
	tb.OnFocusChanged(true)
	layoutButton(tb, render.Rect{X: 0, Y: 0, W: 200, H: 120})
	return tb
}

// caretRectFromRender renders tb into a recording renderer and returns the
// caret bar's own FillRect — identified by caretWidth in WindowText, which
// nothing else this control draws matches (the selection band is Highlight,
// the sunken chrome is the Button* shades, the gutter rule is GrayText). It
// is deliberately read back out of a REAL render pass rather than recomputed
// from the same helpers the renderer uses, so the draw/hit-test agreement
// checks below compare two genuinely independent code paths.
func caretRectFromRender(t *testing.T, tb *TextBox) render.Rect {
	t.Helper()
	rr := &recordRenderer{}
	core.RenderWidget(tb, rr)

	var found []render.Rect
	for _, f := range rr.fills {
		if f.rect.W == caretWidth && f.color == tb.colors.WindowText {
			found = append(found, f.rect)
		}
	}
	if len(found) != 1 {
		t.Fatalf("found %d caret rects in the render pass, want exactly 1", len(found))
	}
	return found[0]
}

// clickIndex dispatches a Press at (x,y) through tb.OnPointer and returns
// where the caret landed — the real hit-test path, capture and all.
func clickIndex(t *testing.T, tb *TextBox, x, y float32) int {
	t.Helper()
	r := input.NewRouter()
	r.SetRoot(tb)
	e := &input.PointerEvent{Action: input.Press, Pos: render.Point{X: x, Y: y}, Router: r}
	tb.OnPointer(e)
	r.Release()
	return tb.Caret()
}

// assertDrawHitTestAgree walks the given rune indices and checks the whole
// round trip at each: put the caret there, render, take the caret bar's
// drawn x straight out of that render pass, click a hair to its right, and
// require the caret to land back on the same index. Draw and hit-test must
// offset by the gutter identically or this fails — a gutter added to only
// one of the two shifts every click by its full width.
func assertDrawHitTestAgree(t *testing.T, tb *TextBox, indices []int) {
	t.Helper()
	for _, i := range indices {
		tb.SetCaret(i)
		layoutButton(tb, tb.Bounds())

		caret := caretRectFromRender(t, tb)
		got := clickIndex(t, tb, caret.X+0.5, caret.Y+tb.lineHeight()/2)
		if got != i {
			t.Fatalf("caret drawn at x=%v for index %d, but a click there landed at index %d", caret.X, i, got)
		}
	}
}

// TestTextBoxLineNumbersDefaultOff is the regression lock on the default
// path: a multi-line TextBox that never calls SetLineNumbers must reserve no
// gutter at all — LineNumbers() false, gutterWidth() 0, the content width
// exactly the padding-inset width, the text origin still bounds.X+PaddingM,
// and click-to-caret still landing where it always did.
func TestTextBoxLineNumbersDefaultOff(t *testing.T) {
	tb := newGutterTextBox(t, "Hello fluo\nSecond line\nThird", false)

	if tb.LineNumbers() {
		t.Fatal("LineNumbers() = true by default, want false")
	}
	if got := tb.gutterWidth(); got != 0 {
		t.Fatalf("gutterWidth() with the flag off = %v, want 0", got)
	}
	if got, want := tb.contentWidth(), tb.Bounds().W-2*tb.metrics.PaddingM; got != want {
		t.Fatalf("contentWidth() with the flag off = %v, want %v (padding-inset, nothing reserved)", got, want)
	}
	// localTextX's pre-feature formula, spelled out literally.
	probe := tb.Bounds().X + tb.metrics.PaddingM + 20
	if got, want := tb.localTextX(probe), float32(20)+tb.hscroll; got != want {
		t.Fatalf("localTextX(%v) = %v, want %v (no gutter subtracted)", probe, got, want)
	}

	assertDrawHitTestAgree(t, tb, []int{0, 3, 10, 11, 15, 22})
}

// TestTextBoxLineNumbersInsetsContent is the enabled counterpart: turning the
// gutter on reserves a positive width, and the text's share of the box
// narrows by exactly that much — the mirror image of the vertical thumb's own
// right-edge reservation (see TestTextBoxVScrollGutterOnlyReservedWhenShown).
func TestTextBoxLineNumbersInsetsContent(t *testing.T) {
	off := newGutterTextBox(t, "Hello fluo\nSecond line\nThird", false)
	on := newGutterTextBox(t, "Hello fluo\nSecond line\nThird", true)

	gw := on.gutterWidth()
	if gw <= 0 {
		t.Fatalf("gutterWidth() with the flag on = %v, want > 0", gw)
	}
	if !on.LineNumbers() {
		t.Fatal("LineNumbers() = false after SetLineNumbers(true), want true")
	}
	if got, want := on.contentWidth(), off.contentWidth()-gw; got != want {
		t.Fatalf("contentWidth() with the gutter on = %v, want %v (off minus the gutter)", got, want)
	}
	// The gutter must fit a whole line number plus the rule and its air.
	digits := on.face.Measure("0").W
	if want := digits + 2*on.metrics.PaddingS + lineNumberRuleWidth; gw != want {
		t.Fatalf("gutterWidth() = %v, want %v (one digit + 2*PaddingS + rule)", gw, want)
	}
}

// TestTextBoxLineNumbersDrawAndHitTestAgree is the feature's whole risk in
// one test: with the gutter on, the caret's DRAWN x (read out of a real
// render pass) and the click hit-test must still agree at every index, so a
// click lands on the character under the pointer rather than a gutter-width
// further along.
func TestTextBoxLineNumbersDrawAndHitTestAgree(t *testing.T) {
	tb := newGutterTextBox(t, "Hello fluo\nSecond line\nThird", true)
	assertDrawHitTestAgree(t, tb, []int{0, 3, 10, 11, 15, 22})
}

// TestTextBoxLineNumbersShiftCaretAndClicks proves the offset is real in both
// directions rather than two consistent no-ops: the same caret index draws
// exactly gutterWidth further right with the gutter on, and the same window
// x hit-tests to an EARLIER index (the gutter has pushed that column's text
// rightward, so a fixed x now falls on an earlier character).
func TestTextBoxLineNumbersShiftCaretAndClicks(t *testing.T) {
	off := newGutterTextBox(t, "Hello fluo\nSecond line\nThird", false)
	on := newGutterTextBox(t, "Hello fluo\nSecond line\nThird", true)

	off.SetCaret(8)
	on.SetCaret(8)
	layoutButton(off, off.Bounds())
	layoutButton(on, on.Bounds())

	offCaret := caretRectFromRender(t, off)
	onCaret := caretRectFromRender(t, on)
	if got, want := onCaret.X, offCaret.X+on.gutterWidth(); got != want {
		t.Fatalf("caret x with the gutter on = %v, want %v (off + gutterWidth)", got, want)
	}

	// A click at a fixed window x, well into the first line's text.
	x := off.Bounds().X + off.metrics.PaddingM + 40
	y := off.Bounds().Y + off.metrics.PaddingM + off.lineHeight()/2
	offIdx := clickIndex(t, off, x, y)
	onIdx := clickIndex(t, on, x, y)
	if onIdx >= offIdx {
		t.Fatalf("click at x=%v landed at index %d with the gutter on and %d with it off, want strictly earlier with the gutter", x, onIdx, offIdx)
	}
}

// TestTextBoxLineNumbersGutterWidthGrowsWithLineCount checks the gutter is
// sized to the widest number that can actually appear: a 9-line box needs one
// digit, a 150-line box needs three, so the latter's gutter is strictly wider
// — by exactly the extra two digits' worth of width.
func TestTextBoxLineNumbersGutterWidthGrowsWithLineCount(t *testing.T) {
	small := newGutterTextBox(t, strings.TrimSuffix(strings.Repeat("x\n", 9), "\n"), true)
	big := newGutterTextBox(t, strings.TrimSuffix(strings.Repeat("x\n", 150), "\n"), true)

	if got := small.lineCount(); got != 9 {
		t.Fatalf("small lineCount() = %d, want 9", got)
	}
	if got := big.lineCount(); got != 150 {
		t.Fatalf("big lineCount() = %d, want 150", got)
	}
	if big.gutterWidth() <= small.gutterWidth() {
		t.Fatalf("gutterWidth() = %v for 150 lines and %v for 9, want strictly wider for 150", big.gutterWidth(), small.gutterWidth())
	}
	oneDigit := small.face.Measure("0").W
	threeDigits := big.face.Measure("000").W
	if got, want := big.gutterWidth()-small.gutterWidth(), threeDigits-oneDigit; got != want {
		t.Fatalf("gutter width difference = %v, want %v (two extra digits)", got, want)
	}
}

// TestTextBoxLineNumbersDigitCount pins digitCount's own boundaries, since
// the gutter width is entirely derived from it.
func TestTextBoxLineNumbersDigitCount(t *testing.T) {
	for _, tc := range []struct{ n, want int }{
		{0, 1}, {1, 1}, {9, 1}, {10, 2}, {99, 2}, {100, 3}, {999, 3}, {1000, 4},
	} {
		if got := digitCount(tc.n); got != tc.want {
			t.Fatalf("digitCount(%d) = %d, want %d", tc.n, got, tc.want)
		}
	}
}

// TestTextBoxLineNumbersTrackLogicalLinesWhenWrapped is the word-wrap rule:
// numbers count REAL ('\n'-delimited) lines, not displayed rows, so a
// wrapped logical line shows exactly one number — on its first visual row —
// and every continuation row is left blank.
func TestTextBoxLineNumbersTrackLogicalLinesWhenWrapped(t *testing.T) {
	tb := NewTextBox(buttonFace(t)).SetMultiline(true).SetWordWrap(true).SetLineNumbers(true)
	tb.SetText("The quick brown fox jumps over the lazy dog\nshort\nanother long line that will certainly wrap")
	tb.SetWidth(160)
	tb.SetHeight(300) // generous: no vertical overflow, so no thumb gutter
	layoutButton(tb, render.Rect{X: 0, Y: 0, W: 160, H: 300})

	rows := tb.visualRows(tb.contentWidth())
	if len(rows) <= tb.lineCount() {
		t.Fatalf("visual rows = %d for %d logical lines, want more (nothing wrapped)", len(rows), tb.lineCount())
	}

	nums := rowLineNumbers([]rune(tb.Text()), rows)
	var numbered []int
	for i, n := range nums {
		if n == 0 {
			continue
		}
		numbered = append(numbered, n)
		// The numbered row must be that logical line's FIRST row.
		if got, want := rows[i].start, tb.lineStart(n-1); got != want {
			t.Fatalf("line %d numbered on a row starting at %d, want the line's own first row (starting at %d)", n, got, want)
		}
	}
	if len(numbered) != tb.lineCount() {
		t.Fatalf("numbered rows = %d, want %d (exactly one per logical line)", len(numbered), tb.lineCount())
	}
	for i, n := range numbered {
		if n != i+1 {
			t.Fatalf("numbered[%d] = %d, want %d (1-based, in order)", i, n, i+1)
		}
	}
}

// TestTextBoxLineNumbersUnwrappedRowsAreOneToOne is the unwrapped half of the
// same mapping: with no wrapping there is one row per logical line, so row i
// simply carries number i+1 and no row is ever left blank — empty lines
// (including a trailing one) included.
func TestTextBoxLineNumbersUnwrappedRowsAreOneToOne(t *testing.T) {
	runes := []rune("alpha\n\nbeta\ngamma\n")
	rows := logicalLineRows(runes)
	nums := rowLineNumbers(runes, rows)

	if len(nums) != 5 {
		t.Fatalf("rows = %d, want 5 (four newlines, empty second and trailing lines included)", len(nums))
	}
	for i, n := range nums {
		if n != i+1 {
			t.Fatalf("nums[%d] = %d, want %d", i, n, i+1)
		}
	}
}

// TestTextBoxLineNumbersIgnoredWhenSingleLine pins the multiline-only gating
// (the same shape SetWordWrap/SetTabInserts use): a single-line box keeps the
// flag's value but reserves nothing and lays out exactly as before.
func TestTextBoxLineNumbersIgnoredWhenSingleLine(t *testing.T) {
	tb := NewTextBox(buttonFace(t)).SetLineNumbers(true)
	tb.SetText("Hello fluo")
	tb.SetWidth(300)
	tb.SetHeight(30)
	tb.OnFocusChanged(true)
	layoutButton(tb, render.Rect{X: 0, Y: 0, W: 300, H: 30})

	if !tb.LineNumbers() {
		t.Fatal("LineNumbers() = false, want true (the flag is still recorded, just inert)")
	}
	if tb.showLineNumbers() {
		t.Fatal("showLineNumbers() = true for a single-line box, want false")
	}
	if got := tb.gutterWidth(); got != 0 {
		t.Fatalf("gutterWidth() for a single-line box = %v, want 0", got)
	}
	if got, want := tb.contentWidth(), tb.Bounds().W-2*tb.metrics.PaddingM; got != want {
		t.Fatalf("contentWidth() = %v, want %v (unchanged)", got, want)
	}

	assertDrawHitTestAgree(t, tb, []int{0, 1, 5, 10})
}

// TestTextBoxLineNumbersNilFaceReservesNothing covers the degenerate face:
// with no glyph widths to size a number column by, the gutter collapses to
// nothing rather than reserving an arbitrary width (matching every other
// nil-face convention in textbox.go), so nothing shifts.
func TestTextBoxLineNumbersNilFaceReservesNothing(t *testing.T) {
	tb := NewTextBox(nil).SetMultiline(true).SetLineNumbers(true)
	tb.SetText("one\ntwo")
	tb.SetWidth(200)
	tb.SetHeight(120)
	layoutButton(tb, render.Rect{X: 0, Y: 0, W: 200, H: 120})

	if got := tb.gutterWidth(); got != 0 {
		t.Fatalf("gutterWidth() with a nil face = %v, want 0", got)
	}
	if got, want := tb.contentWidth(), tb.Bounds().W-2*tb.metrics.PaddingM; got != want {
		t.Fatalf("contentWidth() = %v, want %v (unchanged)", got, want)
	}
}

// TestTextBoxLineNumbersRuleAndNumbersScrollWithText proves the gutter is
// drawn at all and rides the same scroll offset the text does: the
// separating rule is a GrayText hairline spanning the padding-inset height,
// and the row y-origin the numbers are placed at is the vscroll-shifted one
// the caret itself lands on.
func TestTextBoxLineNumbersRuleAndNumbersScrollWithText(t *testing.T) {
	tb := NewTextBox(buttonFace(t)).SetMultiline(true).SetLineNumbers(true)
	tb.SetText(strings.TrimSuffix(strings.Repeat("line\n", 20), "\n"))
	tb.SetWidth(200)
	tb.SetHeight(80) // far too short for 20 lines: vscroll ends up non-zero
	tb.OnFocusChanged(true)
	layoutButton(tb, render.Rect{X: 0, Y: 0, W: 200, H: 80})

	if tb.vscroll <= 0 {
		t.Fatalf("vscroll = %v, want > 0 (caret at the end of overflowing content)", tb.vscroll)
	}

	rr := &recordRenderer{}
	core.RenderWidget(tb, rr)

	bounds := tb.Bounds()
	pad := tb.metrics.PaddingM
	wantRule := render.Rect{
		X: bounds.X + pad + tb.gutterWidth() - tb.metrics.PaddingS - lineNumberRuleWidth,
		Y: bounds.Y + pad,
		W: lineNumberRuleWidth,
		H: bounds.H - 2*pad,
	}
	var found bool
	for _, f := range rr.fills {
		if f.rect == wantRule && f.color == tb.colors.GrayText {
			found = true
		}
	}
	if !found {
		t.Fatalf("no GrayText separator rule at %+v in the render pass", wantRule)
	}

	// The numbers share the text's own row origin, so the caret's own row
	// y is exactly the scrolled origin renderLineNumbers places them at.
	caret := caretRectFromRender(t, tb)
	line, _ := tb.lineCol(tb.caret)
	wantY := bounds.Y + pad - tb.vscroll + float32(line)*tb.lineHeight()
	if caret.Y != wantY {
		t.Fatalf("caret y = %v, want %v (scrolled row origin the gutter numbers also use)", caret.Y, wantY)
	}
}

// multiClickText is the fixture the double-/triple-click tests below share:
// two logical lines, punctuation both glued to a word ("alpha,") and standing
// alone between words, so word spans have to respect the punctuation-is-its-
// own-word rule and stop at the line break.
//
//	index: 0..4 alpha  5 ,  6 space  7..10 beta  11 \n
//	       12..16 gamma  17 space  18..22 delta   (len 23)
const multiClickText = "alpha, beta\ngamma delta"

// newMultiClickTextBox builds a focused, laid-out, non-wrapping multi-line
// TextBox over s, sized so both lines fit without scrolling.
func newMultiClickTextBox(t *testing.T, s string) *TextBox {
	t.Helper()
	tb := NewTextBox(buttonFace(t)).SetMultiline(true)
	tb.SetText(s)
	tb.SetWidth(300)
	tb.SetHeight(120)
	tb.OnFocusChanged(true)
	layoutButton(tb, render.Rect{X: 0, Y: 0, W: 300, H: 120})
	return tb
}

// pointAtIndex inverts caretIndexAtPos for a non-wrapping TextBox: the
// window-space point that hit-tests to rune index i, vertically centered in
// its own row. Every caller re-checks the round trip through caretIndexAtPos
// before relying on it, so a geometry slip here fails loudly rather than
// quietly testing the wrong word.
func pointAtIndex(t *testing.T, tb *TextBox, i int) (x, y float32) {
	t.Helper()
	line, col := tb.lineCol(i)
	x = tb.Bounds().X + tb.metrics.PaddingM + tb.gutterWidth() - tb.hscroll + tb.xOfInLine(line, col)
	y = tb.Bounds().Y + tb.metrics.PaddingM - tb.vscroll + (float32(line)+0.5)*tb.lineHeight()
	if got := tb.caretIndexAtPos(x, y); got != i {
		t.Fatalf("pointAtIndex(%d) = (%v,%v), which hit-tests to index %d instead", i, x, y, got)
	}
	return x, y
}

// pressWithClicks dispatches a synthetic Press at index i carrying the given
// ClickCount, through a throwaway router whose capture it drops again — the
// shape of a press the real Router would deliver mid-run.
func pressWithClicks(t *testing.T, tb *TextBox, i, clicks int) {
	t.Helper()
	x, y := pointAtIndex(t, tb, i)
	r := input.NewRouter()
	r.SetRoot(tb)
	e := &input.PointerEvent{Action: input.Press, Pos: render.Point{X: x, Y: y}, ClickCount: clicks, Router: r}
	tb.OnPointer(e)
	r.Release()
}

// TestTextBoxDoubleClickSelectsWord covers the word span a ClickCount 2 press
// selects at each interesting position: inside a word, on a word's leading
// and trailing edges (the trailing edge must keep the word just ended rather
// than skip to the next one), on punctuation standing as its own word, and at
// the very end of the text.
func TestTextBoxDoubleClickSelectsWord(t *testing.T) {
	for _, tc := range []struct {
		at   int
		want string
	}{
		{0, "alpha"},  // leading edge of the first word
		{2, "alpha"},  // inside it
		{5, "alpha"},  // trailing edge: the word that ends here wins
		{6, ","},      // trailing edge of the comma, its own word
		{7, "beta"},   // leading edge, straight after a space
		{9, "beta"},   // inside it
		{11, "beta"},  // end of line 0: must not reach across the '\n'
		{12, "gamma"}, // start of line 1, likewise
		{14, "gamma"},
		{20, "delta"},
		{23, "delta"}, // very end of the text
	} {
		tb := newMultiClickTextBox(t, multiClickText)
		pressWithClicks(t, tb, tc.at, 2)

		start, end := tb.Selection()
		if got := string(tb.runes[start:end]); got != tc.want {
			t.Fatalf("double-click at index %d selected %q [%d,%d), want %q", tc.at, got, start, end, tc.want)
		}
	}
}

// TestTextBoxDoubleClickInWhitespacePlacesCaret covers wordSpanAt's no-word
// case: a double-click with whitespace on both sides has nothing to select,
// so it falls back to plain caret placement rather than selecting the run of
// spaces.
func TestTextBoxDoubleClickInWhitespacePlacesCaret(t *testing.T) {
	tb := newMultiClickTextBox(t, "hi   there")
	pressWithClicks(t, tb, 3, 2) // "hi_|__there", spaces either side

	if start, end := tb.Selection(); start != end {
		t.Fatalf("double-click in whitespace selected [%d,%d), want an empty selection", start, end)
	}
	if got := tb.Caret(); got != 3 {
		t.Fatalf("double-click in whitespace: caret = %d, want 3", got)
	}
}

// TestTextBoxTripleClickSelectsLine covers ClickCount 3: the whole LOGICAL
// line under the pointer, stopping short of its terminating newline. A fourth
// press in the same run keeps that line selected rather than snapping back to
// a bare caret, since the Router keeps counting upward past 3.
func TestTextBoxTripleClickSelectsLine(t *testing.T) {
	for _, tc := range []struct {
		at     int
		clicks int
		want   string
	}{
		{2, 3, "alpha, beta"},
		{11, 3, "alpha, beta"}, // clicking at the line's end still takes it whole
		{14, 3, "gamma delta"},
		{14, 4, "gamma delta"}, // beyond a triple, the line selection holds
	} {
		tb := newMultiClickTextBox(t, multiClickText)
		pressWithClicks(t, tb, tc.at, tc.clicks)

		start, end := tb.Selection()
		if got := string(tb.runes[start:end]); got != tc.want {
			t.Fatalf("%d-click at index %d selected %q [%d,%d), want %q", tc.clicks, tc.at, got, start, end, tc.want)
		}
	}
}

// TestTextBoxSingleClickStillPlacesCaret is the regression lock: a press with
// ClickCount 1 — and equally one carrying the zero value, which is every
// synthetic event built before this field existed — places the caret and
// clears the selection exactly as it always did.
func TestTextBoxSingleClickStillPlacesCaret(t *testing.T) {
	for _, clicks := range []int{0, 1} {
		tb := newMultiClickTextBox(t, multiClickText)
		tb.Select(0, 5) // a selection that the press must clear
		pressWithClicks(t, tb, 9, clicks)

		if got := tb.Caret(); got != 9 {
			t.Fatalf("press with ClickCount %d: caret = %d, want 9", clicks, got)
		}
		if start, end := tb.Selection(); start != end {
			t.Fatalf("press with ClickCount %d selected [%d,%d), want an empty selection", clicks, start, end)
		}
	}
}

// TestTextBoxDragAfterDoubleClickExtendsFromWordStart pins the drag that
// follows a multi-click: it stays coherent, extending per-rune from the word
// the double-click selected (Select leaves the anchor at the range start), and
// notably does not crash or leave a stale anchor behind.
func TestTextBoxDragAfterDoubleClickExtendsFromWordStart(t *testing.T) {
	tb := newMultiClickTextBox(t, multiClickText)

	r := input.NewRouter()
	r.SetRoot(tb)

	px, py := pointAtIndex(t, tb, 9) // inside "beta"
	tb.OnPointer(&input.PointerEvent{Action: input.Press, Pos: render.Point{X: px, Y: py}, ClickCount: 2, Router: r})
	if start, end := tb.Selection(); start != 7 || end != 11 {
		t.Fatalf("double-click selected [%d,%d), want [7,11)", start, end)
	}
	if r.Captured() != core.Widget(tb) {
		t.Fatal("double-click did not capture the pointer, so no drag could follow")
	}

	mx, my := pointAtIndex(t, tb, 2) // drag back into "alpha"
	tb.OnPointer(&input.PointerEvent{Action: input.Move, Pos: render.Point{X: mx, Y: my}, Router: r})
	if start, end := tb.Selection(); start != 2 || end != 7 {
		t.Fatalf("drag after a double-click selected [%d,%d), want [2,7) (anchored at the word start)", start, end)
	}

	tb.OnPointer(&input.PointerEvent{Action: input.Release, Pos: render.Point{X: mx, Y: my}, Router: r})
	if r.Captured() != nil {
		t.Fatal("release after a double-click drag left the pointer captured")
	}
}

// TestTextBoxDoubleClickThroughRealRouter is the end-to-end check that the
// two halves meet: a real input.Router on a driven clock, two presses close
// enough in time and space to form a double-click, and a TextBox that selects
// the word — with nobody setting ClickCount by hand anywhere along the way.
func TestTextBoxDoubleClickThroughRealRouter(t *testing.T) {
	tb := newMultiClickTextBox(t, multiClickText)

	clock := 0.0
	r := input.NewRouter()
	r.SetRoot(tb)
	r.SetTimeSource(func() float64 { return clock })

	x, y := pointAtIndex(t, tb, 9) // inside "beta"
	at := render.Point{X: x, Y: y}

	r.PointerButton(input.ButtonLeft, true, at, 0)
	if got := tb.Caret(); got != 9 {
		t.Fatalf("first click: caret = %d, want 9", got)
	}
	r.PointerButton(input.ButtonLeft, false, at, 0)

	clock += 0.1
	r.PointerButton(input.ButtonLeft, true, at, 0)
	if start, end := tb.Selection(); start != 7 || end != 11 {
		t.Fatalf("second click %vs later selected [%d,%d), want [7,11) (the word \"beta\")", 0.1, start, end)
	}
	r.PointerButton(input.ButtonLeft, false, at, 0)

	// A third press, still inside the run, promotes to the whole line.
	clock += 0.1
	r.PointerButton(input.ButtonLeft, true, at, 0)
	if start, end := tb.Selection(); start != 0 || end != 11 {
		t.Fatalf("third click selected [%d,%d), want [0,11) (the whole first line)", start, end)
	}
}

// TestComputeVisualRowsSingleRuneWiderThanWidth covers wrapLogicalLine's
// case 1 — a single rune too wide for the wrap width, which every other
// wrap test misses by using widths of three glyphs or more. Two things only
// this case exercises: each rune must get its own row (a row can never be
// empty, so the break has to consume the rune that overflowed), and the
// final break lands exactly on rowStart == end, which the guard after the
// loop must recognize — an unconditional trailing append there would tack a
// spurious empty row onto the end of a line already fully covered, giving
// the TextBox a phantom last line to place the caret on.
func TestComputeVisualRowsSingleRuneWiderThanWidth(t *testing.T) {
	face := buttonFace(t)
	runes := []rune("abc")
	// Narrower than any single glyph: every rune overflows on its own.
	width := face.Measure("a").W / 4

	rows := computeVisualRows(runes, face, width)

	want := []visualRow{{start: 0, end: 1}, {start: 1, end: 2}, {start: 2, end: 3}}
	if len(rows) != len(want) {
		t.Fatalf("computeVisualRows(%q, width=%v) = %+v, want %+v (one row per rune, no trailing empty row)", string(runes), width, rows, want)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Fatalf("rows[%d] = %+v, want %+v (full: %+v)", i, rows[i], want[i], rows)
		}
	}
	if last := rows[len(rows)-1]; last.start == last.end {
		t.Fatalf("last row %+v is empty; the text is already fully covered by the rows before it", last)
	}
}

// --- Word-wrap-OFF multi-line scroll: mousewheel + text clip (v0.15.9) ---
//
// The exact config the augment SOURCE editor uses (multi-line, word-wrap OFF,
// line numbers on, taller than one screen) exercised two paths no earlier
// test covered: (1) the wheel never scrolled a TextBox at all (OnPointer had
// no Wheel case), and (2) the right vscroll gutter was reserved for the
// scroll DECISION (contentWidth) but not at RENDER, so a long line drew under
// the thumb. Every WORKING box uses word-wrap ON, which incidentally wraps at
// contentWidth and never reaches the edge — so only this fixture reproduces
// the bug.

// newLongLineOverflowingTextBox builds a focused multi-line (word-wrap OFF)
// TextBox with line numbers on, one very long first line, and enough short
// lines after it to overflow the viewport height (so the vertical thumb —
// and thus its reserved gutter — is shown). This is the source-editor config.
func newLongLineOverflowingTextBox(t *testing.T) *TextBox {
	t.Helper()
	long := strings.Repeat("wwwwwwwwww ", 20) // ~220 cols, far wider than the box: horizontal overflow with wrap OFF
	tb := NewTextBox(buttonFace(t)).SetMultiline(true)
	tb.SetLineNumbers(true)
	tb.SetText(long + "\nl1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9")
	tb.SetWidth(200)
	tb.SetHeight(50)
	core.MeasureWidget(tb, render.Size{W: 200, H: 50})
	core.ArrangeWidget(tb, render.Rect{X: 0, Y: 0, W: 200, H: 50})
	return tb
}

// TestTextBoxMultilineWordWrapOffClipsTextToContentWidth proves the render
// clip that keeps a word-wrap-OFF long line off the thumb: the text is drawn
// inside textClipRect, whose RIGHT edge is exactly the thumb track's left
// edge (contentWidth, gutter reserved) and whose LEFT edge is the line-number
// gutter's right edge — so nothing the pass paints lands under the thumb or
// over the line numbers. Before the fix the text clipped only to the full
// bounds, so long lines drew straight under the scrollbar.
func TestTextBoxMultilineWordWrapOffClipsTextToContentWidth(t *testing.T) {
	tb := newLongLineOverflowingTextBox(t)
	if !tb.vScrollShown {
		t.Fatal("vScrollShown = false, want true (content overflows; the gutter must be reserved to test the clip)")
	}
	// A selection across the whole long first line forces a fill that, unclipped,
	// would run far past the right edge — the clip must crop it. Anchor at the
	// line end with the caret at 0 keeps hscroll pinned at 0 (updateHScroll
	// follows the caret), so the fill starts at the gutter and runs the full
	// line width off the right — the exact overflow the clip must crop.
	tb.Select(len([]rune(strings.Repeat("wwwwwwwwww ", 20))), 0)
	core.ArrangeWidget(tb, render.Rect{X: 0, Y: 0, W: 200, H: 50})

	bounds := tb.Bounds()
	clip := tb.textClipRect(bounds)
	pad := tb.metrics.PaddingM

	track, ok := tb.vScrollTrack()
	if !ok {
		t.Fatal("vScrollTrack ok = false, want true")
	}
	if clip.Right() != track.X {
		t.Fatalf("textClipRect right edge = %v, want %v (== thumb track left edge: text must stop at the gutter, not under the thumb)", clip.Right(), track.X)
	}
	if wantX := bounds.X + pad + tb.gutterWidth(); clip.X != wantX {
		t.Fatalf("textClipRect left edge = %v, want %v (== line-number gutter right edge)", clip.X, wantX)
	}

	rr := newClipRecorder()
	tb.Render(rr)

	found := false
	for _, cr := range rr.clips {
		if cr == clip {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no PushClip recorded at the text clip rect %+v; clips = %v", clip, rr.clips)
	}
	// Check the TEXT-layer fill (the selection highlight) specifically — the
	// sunken well and bevel are chrome drawn at full bounds OUTSIDE the text
	// clip and legitimately span the whole width, so they're not the subject.
	sawSel := false
	for _, p := range rr.painted {
		if p.color != tb.colors.Highlight || p.rect.W <= 0 || p.rect.H <= 0 {
			continue
		}
		sawSel = true
		if p.rect.Right() > clip.Right()+0.01 {
			t.Fatalf("the selection fill painted to x = %v, past the text clip right edge %v (drawing under the thumb)", p.rect.Right(), clip.Right())
		}
	}
	if !sawSel {
		t.Fatal("no selection fill was painted; the overflow-under-thumb case was not actually exercised")
	}
}

// TestTextBoxMultilineWheelScrollsVerticallyAndBubblesAtEnd proves the wheel
// scrolls the multi-line view vertically (independent of the caret, mirroring
// the thumb drag) and is Handled only when the clamped offset actually moved
// — so a notch at the bottom stop leaves the event unhandled and bubbles to
// an enclosing scroller instead of dying in a dead zone.
func TestTextBoxMultilineWheelScrollsVerticallyAndBubblesAtEnd(t *testing.T) {
	tb := newOverflowingMultilineTextBox(t)
	tb.SetCaret(0)
	core.ArrangeWidget(tb, render.Rect{X: 0, Y: 0, W: 200, H: 50}) // updateVScroll snaps to the top
	if tb.vscroll != 0 {
		t.Fatalf("vscroll = %v after caret-to-start, want 0", tb.vscroll)
	}

	down := &input.PointerEvent{Action: input.Wheel, Delta: render.Point{Y: -1}}
	tb.OnPointer(down)
	if !down.Handled {
		t.Fatal("wheel down on scrollable content: Handled = false, want true")
	}
	if tb.vscroll <= 0 {
		t.Fatalf("vscroll = %v after a wheel-down notch, want > 0", tb.vscroll)
	}

	maxScroll := tb.totalContentHeight() - tb.contentHeight()
	tb.vscroll = maxScroll
	past := &input.PointerEvent{Action: input.Wheel, Delta: render.Point{Y: -1}}
	tb.OnPointer(past)
	if past.Handled {
		t.Fatal("wheel down at the bottom stop: Handled = true, want false (must bubble to an enclosing scroller)")
	}
	if tb.vscroll != maxScroll {
		t.Fatalf("vscroll = %v after a no-op notch, want unchanged %v", tb.vscroll, maxScroll)
	}
}

// TestTextBoxMultilineShiftWheelScrollsHorizontally proves Shift+Wheel scrolls
// a word-wrap-OFF box horizontally (bounded by the same caret-line width the
// caret-follow uses), and is Handled only when it moved.
func TestTextBoxMultilineShiftWheelScrollsHorizontally(t *testing.T) {
	tb := NewTextBox(buttonFace(t)).SetMultiline(true)
	tb.SetText(strings.Repeat("wwwwwwwwww ", 20)) // one long line, wrap OFF
	tb.SetWidth(200)
	tb.SetHeight(50)
	core.MeasureWidget(tb, render.Size{W: 200, H: 50})
	core.ArrangeWidget(tb, render.Rect{X: 0, Y: 0, W: 200, H: 50})
	tb.SetCaret(0)
	core.ArrangeWidget(tb, render.Rect{X: 0, Y: 0, W: 200, H: 50}) // updateHScroll snaps to the left
	if tb.hscroll != 0 {
		t.Fatalf("hscroll = %v after caret-to-start, want 0", tb.hscroll)
	}

	e := &input.PointerEvent{Action: input.Wheel, Delta: render.Point{Y: -1}, Mods: input.ModShift}
	tb.OnPointer(e)
	if !e.Handled {
		t.Fatal("shift+wheel on an overflowing long line: Handled = false, want true")
	}
	if tb.hscroll <= 0 {
		t.Fatalf("hscroll = %v after a shift+wheel notch, want > 0", tb.hscroll)
	}
}

// TestTextBoxSingleLineWheelBubbles proves a single-line box (the AppliesTo
// config) has nothing to wheel-scroll and leaves the event unhandled so it
// bubbles — the wheel behavior is scoped to the multi-line path.
func TestTextBoxSingleLineWheelBubbles(t *testing.T) {
	tb := NewTextBox(buttonFace(t))
	tb.SetText("hello")
	tb.SetWidth(200)
	tb.SetHeight(24)
	core.MeasureWidget(tb, render.Size{W: 200, H: 24})
	core.ArrangeWidget(tb, render.Rect{X: 0, Y: 0, W: 200, H: 24})

	e := &input.PointerEvent{Action: input.Wheel, Delta: render.Point{Y: -1}}
	tb.OnPointer(e)
	if e.Handled {
		t.Fatal("single-line wheel: Handled = true, want false (nothing to scroll; must bubble)")
	}
}

// TestTextBoxMultilineWheelScrollsThroughRouterDispatch drives the wheel
// through the REAL routing path — input.Router.PointerWheel does its own
// HitPath at the point and Bubbles the event leaf→root to OnPointer — rather
// than calling TextBox.OnPointer directly. This is the "does the router
// actually DELIVER Wheel to a hovered widget's OnPointer" guard: a dispatch
// gap (router never reaches OnPointer) would leave vscroll untouched here even
// though the isolated OnPointer test passes. The viewer wires its GLFW scroll
// callback straight to this same Surface/Router.PointerWheel, so this exercises
// the exact call path the app uses.
func TestTextBoxMultilineWheelScrollsThroughRouterDispatch(t *testing.T) {
	tb := newOverflowingMultilineTextBox(t)
	tb.SetCaret(0)
	core.ArrangeWidget(tb, render.Rect{X: 0, Y: 0, W: 200, H: 50}) // vscroll -> 0
	if tb.vscroll != 0 {
		t.Fatalf("vscroll = %v after caret-to-start, want 0", tb.vscroll)
	}

	r := input.NewRouter()
	r.SetRoot(tb)
	// A wheel notch at a point inside the box — routed, not hand-delivered.
	r.PointerWheel(render.Point{Y: -1}, render.Point{X: 20, Y: 20}, 0)

	if tb.vscroll <= 0 {
		t.Fatalf("vscroll = %v after Router.PointerWheel, want > 0 (real dispatch must reach OnPointer, not just a direct call)", tb.vscroll)
	}
}

// TestTextBoxTextViewportRectNarrowsForGutter proves the exported
// TextViewportRect (unlike ClipRect) narrows below the bounds width when the
// vertical-scroll gutter is reserved — the ground-truth render signal a
// harness needs (comparing ClipRect().W to bounds width is a tautology, since
// ClipRect always returns the full bounds).
func TestTextBoxTextViewportRectNarrowsForGutter(t *testing.T) {
	tb := newLongLineOverflowingTextBox(t) // overflows -> thumb shown -> gutter reserved
	if !tb.vScrollShown {
		t.Fatal("want vScrollShown (overflowing) so a gutter is reserved")
	}
	bounds := tb.Bounds()

	// ClipRect is the tautology: always the full bounds width.
	clip, _ := tb.ClipRect()
	if clip.W != bounds.W {
		t.Fatalf("ClipRect().W = %v, want %v (ClipRect is always full bounds by design)", clip.W, bounds.W)
	}

	// TextViewportRect is the real signal: narrower than the bounds, and its
	// right edge is the thumb track's left edge.
	vp, ok := tb.TextViewportRect()
	if !ok {
		t.Fatal("TextViewportRect ok = false for a multi-line box, want true")
	}
	if vp.W >= bounds.W {
		t.Fatalf("TextViewportRect().W = %v, want < bounds width %v (gutter reserved)", vp.W, bounds.W)
	}
	track, _ := tb.vScrollTrack()
	if vp.Right() != track.X {
		t.Fatalf("TextViewportRect right edge = %v, want %v (thumb track left edge)", vp.Right(), track.X)
	}

	// Single-line: no viewport.
	sl := NewTextBox(buttonFace(t))
	if _, ok := sl.TextViewportRect(); ok {
		t.Fatal("single-line TextViewportRect ok = true, want false")
	}
}

// TestTextBoxMultilineWheelScrollsWhenNestedInContainer drives the wheel
// through Router.PointerWheel at the box's real on-screen position while the
// box is a CHILD of a StackPanel (not the root) — proving HitPath traverses
// the container to reach the nested TextBox and Bubble delivers the wheel to
// it. This is the nesting the real augment box has (a StackPanel inside a
// SplitPanel); a HitPath that stopped at the container would leave vscroll
// untouched here.
func TestTextBoxMultilineWheelScrollsWhenNestedInContainer(t *testing.T) {
	tb := NewTextBox(buttonFace(t)).SetMultiline(true)
	tb.SetText("l0\nl1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9")
	tb.SetWidth(200)
	tb.SetHeight(50)

	panel := NewStackPanel(Vertical)
	panel.Add(tb)
	core.MeasureWidget(panel, render.Size{W: 200, H: 50})
	core.ArrangeWidget(panel, render.Rect{X: 0, Y: 0, W: 200, H: 50})
	tb.SetCaret(0)
	core.ArrangeWidget(panel, render.Rect{X: 0, Y: 0, W: 200, H: 50})
	if tb.vscroll != 0 {
		t.Fatalf("vscroll = %v after caret-to-start, want 0", tb.vscroll)
	}

	r := input.NewRouter()
	r.SetRoot(panel)
	b := core.BoundsOf(tb)
	if b.W == 0 || b.H == 0 {
		t.Fatalf("nested TextBox bounds are zero (%+v) — container did not arrange it", b)
	}
	r.PointerWheel(render.Point{Y: -1}, render.Point{X: b.X + b.W/2, Y: b.Y + b.H/2}, 0)

	if tb.vscroll <= 0 {
		t.Fatalf("vscroll = %v after a wheel routed to the NESTED box, want > 0 (HitPath must traverse the container)", tb.vscroll)
	}
}

// TestTextBoxSourceEditorRealArrangementReservesGutter reproduces the augment
// source editor's EXACT construction (Multiline + WordWrap OFF + LineNumbers,
// SetHeight(460), NO SetWidth) inside a Vertical StackPanel arranged as a
// split pane — the real config Eric's -selftest-augment-source-wheel-diag
// drives. It proves the gutter reservation FIRES for this config (vScrollShown
// true, contentWidth narrowed by ScrollGutter) and, crucially, that the RIGHT
// probe to see it is TextViewportRect (narrows) NOT ClipRect (always full
// bounds — the tautology that made the real box look unfixed).
func TestTextBoxSourceEditorRealArrangementReservesGutter(t *testing.T) {
	var lines []string
	for i := 1; i <= 80; i++ {
		if i == 1 {
			lines = append(lines, "def handle(request, context, options, retries, deadline, backoff, jitter):")
		} else {
			lines = append(lines, "    step = compute(i)")
		}
	}
	tb := NewTextBox(buttonFace(t))
	tb.SetMultiline(true)
	tb.SetTabInserts(true)
	tb.SetLineNumbers(true)
	tb.SetText(strings.Join(lines, "\n"))
	tb.SetHeight(460)
	// NO SetWidth — grows with the split, like the real box.

	right := NewStackPanel(Vertical).SetGap(4)
	right.Add(NewTextBlock(buttonFace(t), "Source"))
	right.Add(tb)
	core.MeasureWidget(right, render.Size{W: 600, H: 900})
	core.ArrangeWidget(right, render.Rect{X: 0, Y: 0, W: 600, H: 900})

	b := tb.Bounds()
	if b.H != 460 {
		t.Fatalf("box height = %v, want 460 (SetHeight honored through the StackPanel)", b.H)
	}
	if !tb.vScrollShown {
		t.Fatal("vScrollShown = false for an 80-line box in a 460px viewport, want true (gutter reservation must fire for LineNumbers+WordWrapOff)")
	}
	if tb.contentWidth() >= tb.fullContentWidth() {
		t.Fatalf("contentWidth %v not reduced below fullContentWidth %v (gutter not subtracted)", tb.contentWidth(), tb.fullContentWidth())
	}

	// The tautology: ClipRect is ALWAYS full bounds — the wrong probe.
	if clip, _ := tb.ClipRect(); clip.W != b.W {
		t.Fatalf("ClipRect().W = %v, want %v (always full bounds by design)", clip.W, b.W)
	}
	// The real signal: TextViewportRect narrows and stops at the thumb.
	vp, ok := tb.TextViewportRect()
	if !ok || vp.W >= b.W {
		t.Fatalf("TextViewportRect = %+v ok=%v, want narrowed below bounds width %v", vp, ok, b.W)
	}
	track, _ := tb.vScrollTrack()
	if vp.Right() != track.X {
		t.Fatalf("TextViewportRect right edge = %v, want %v (thumb track left edge — text stops before the thumb)", vp.Right(), track.X)
	}

	// DRAW-TIME proof (the PM's exact question): renderMultiline must actually
	// PushClip the narrowed viewport during Render, not merely compute it and
	// then draw to full bounds. Select the whole long first line with the caret
	// at 0 (hscroll pinned to 0) so its highlight fill runs the full line width
	// off the right — if the draw used full bounds it would paint under the
	// thumb; the recorded, clip-intersected fill must stop at vp.Right().
	tb.Select(len([]rune(lines[0])), 0)
	core.ArrangeWidget(right, render.Rect{X: 0, Y: 0, W: 600, H: 900})
	vp, _ = tb.TextViewportRect() // stable across Select, re-read defensively

	rr := newClipRecorder()
	tb.Render(rr)

	foundClip := false
	for _, cr := range rr.clips {
		if cr == vp {
			foundClip = true
			break
		}
	}
	if !foundClip {
		t.Fatalf("renderMultiline did not PushClip the narrowed text viewport %+v at draw time; clips = %v", vp, rr.clips)
	}
	sawSel := false
	for _, p := range rr.painted {
		if p.color != tb.colors.Highlight || p.rect.W <= 0 || p.rect.H <= 0 {
			continue
		}
		sawSel = true
		if p.rect.Right() > vp.Right()+0.01 {
			t.Fatalf("selection fill painted to x = %v, past the text viewport right edge %v (drawing under the thumb at DRAW time)", p.rect.Right(), vp.Right())
		}
	}
	if !sawSel {
		t.Fatal("no selection fill painted; the overflow-under-thumb draw case was not exercised")
	}
}

// TestTextBoxSourceEditorInSplitPanelStillNarrows closes the last fluo-side
// embedding gap: the real augment box isn't just in a StackPanel, it's in a
// StackPanel that is the SECOND pane of a Horizontal SplitPanel (fluo_augment
// .go's split.SetSecond(right)). The split changes only the box's WIDTH; the
// gutter reservation keys off VERTICAL overflow (SetHeight(460) vs 80 lines),
// so it must still fire and the text viewport must still narrow.
func TestTextBoxSourceEditorInSplitPanelStillNarrows(t *testing.T) {
	var lines []string
	for i := 1; i <= 80; i++ {
		lines = append(lines, "    step = compute(i)")
	}
	tb := NewTextBox(buttonFace(t))
	tb.SetMultiline(true)
	tb.SetTabInserts(true)
	tb.SetLineNumbers(true)
	tb.SetText(strings.Join(lines, "\n"))
	tb.SetHeight(460)

	right := NewStackPanel(Vertical).SetGap(4)
	right.Add(NewTextBlock(buttonFace(t), "Source"))
	right.Add(tb)
	left := NewStackPanel(Vertical)
	left.Add(NewTextBlock(buttonFace(t), "left"))

	split := NewSplitPanel(Horizontal)
	split.SetFirst(left)
	split.SetSecond(right)
	split.SetSplitRatio(0.35)
	split.SetWidth(900)
	core.MeasureWidget(split, render.Size{W: 900, H: 900})
	core.ArrangeWidget(split, render.Rect{X: 0, Y: 0, W: 900, H: 900})

	b := tb.Bounds()
	if b.W == 0 || b.H == 0 {
		t.Fatalf("box not arranged inside the SplitPanel: %+v", b)
	}
	if b.H != 460 {
		t.Fatalf("box height inside split = %v, want 460 (SetHeight honored)", b.H)
	}
	if !tb.vScrollShown {
		t.Fatal("vScrollShown = false inside a SplitPanel, want true (vertical overflow is split-independent)")
	}
	vp, ok := tb.TextViewportRect()
	if !ok || vp.W >= b.W {
		t.Fatalf("TextViewportRect = %+v ok=%v, want narrowed below bounds width %v (split embedding must not defeat the gutter)", vp, ok, b.W)
	}

	// Draw-time: the narrowed viewport is actually pushed while embedded.
	rr := newClipRecorder()
	tb.Render(rr)
	found := false
	for _, cr := range rr.clips {
		if cr == vp {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("renderMultiline did not PushClip %+v inside the SplitPanel; clips = %v", vp, rr.clips)
	}
}

// TestTextBoxSingleLineClipsScrolledTextToPadding proves the single-line render
// path clips horizontally-scrolled text (glyphs, selection, caret) to the
// padding-inset content area, so scrolled-off content doesn't bleed half-glyphs
// over the sunken bevel. This is the single-line analogue of the multiline
// textClipRect fix (a single-line box never draws a thumb track, so nothing
// covers the bleed).
func TestTextBoxSingleLineClipsScrolledTextToPadding(t *testing.T) {
	const s = "this is a fairly long single line of text that overflows the narrow box"
	tb := NewTextBox(buttonFace(t))
	tb.SetText(s)
	tb.SetWidth(140)
	tb.SetHeight(24)
	core.MeasureWidget(tb, render.Size{W: 140, H: 24})
	core.ArrangeWidget(tb, render.Rect{X: 0, Y: 0, W: 140, H: 24})

	// Caret at the end -> hscroll > 0 (the line has scrolled left to keep the
	// caret visible), so the head of the line is off the left content edge.
	tb.Select(0, len([]rune(s))) // anchor 0, caret end: full selection, caret at end
	core.ArrangeWidget(tb, render.Rect{X: 0, Y: 0, W: 140, H: 24})
	if tb.hscroll <= 0 {
		t.Fatalf("precondition: hscroll = %v, want > 0 (text scrolled to show the end)", tb.hscroll)
	}

	bounds := tb.Bounds()
	pad := tb.metrics.PaddingM
	clipL := bounds.X + pad
	clipR := bounds.X + bounds.W - pad

	rr := newClipRecorder()
	tb.Render(rr)

	found := false
	for _, cr := range rr.clips {
		if cr.X == clipL && cr.W == bounds.W-2*pad {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("single-line content clip not pushed (want x=%v w=%v); clips = %v", clipL, bounds.W-2*pad, rr.clips)
	}

	sawSel := false
	for _, p := range rr.painted {
		if p.color != tb.colors.Highlight || p.rect.W <= 0 || p.rect.H <= 0 {
			continue
		}
		sawSel = true
		if p.rect.X < clipL-0.01 || p.rect.Right() > clipR+0.01 {
			t.Fatalf("selection fill %+v bleeds outside the content area [%v,%v]", p.rect, clipL, clipR)
		}
	}
	if !sawSel {
		t.Fatal("no selection fill painted; the scrolled-bleed case was not exercised")
	}
}

// TestTextBoxHorizontalScrollbarReservesBottomLane covers the horizontal
// scrollbar: a wrap-off box whose widest line overflows gets a bottom-lane
// scrollbar (mirroring the vertical one's right lane); the text clip excludes
// that lane so text never paints behind it; the two lanes leave a dead corner;
// and dragging the thumb scrolls horizontally independent of the caret.
func TestTextBoxHorizontalScrollbarReservesBottomLane(t *testing.T) {
	tb := NewTextBox(buttonFace(t)).SetMultiline(true).SetLineNumbers(true)
	long := strings.Repeat("wwwwwwwwww ", 20) // far wider than the box
	tb.SetText(long + "\nl1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9")
	tb.SetWidth(200)
	tb.SetHeight(80)
	core.MeasureWidget(tb, render.Size{W: 200, H: 80})
	core.ArrangeWidget(tb, render.Rect{X: 0, Y: 0, W: 200, H: 80})

	if !tb.hScrollShown {
		t.Fatal("hScrollShown = false, want true (widest line overflows the content width)")
	}
	if !tb.vScrollShown {
		t.Fatal("vScrollShown = false, want true (rows overflow the height)")
	}

	bounds := tb.Bounds()
	pad := tb.metrics.PaddingM
	g := tb.metrics.ScrollGutter

	ht, ok := tb.hScrollTrack()
	if !ok {
		t.Fatal("hScrollTrack ok = false")
	}
	if wantY := bounds.Y + bounds.H - pad - g; ht.Y != wantY {
		t.Fatalf("h-track Y = %v, want %v (bottom lane)", ht.Y, wantY)
	}
	if ht.H != g {
		t.Fatalf("h-track H = %v, want gutter %v", ht.H, g)
	}
	if wantX := bounds.X + pad + tb.gutterWidth(); ht.X != wantX {
		t.Fatalf("h-track X = %v, want %v (starts after the line-number gutter)", ht.X, wantX)
	}

	vtrack, _ := tb.vScrollTrack()
	if ht.Right() > vtrack.X+0.01 {
		t.Fatalf("h-track right = %v extends past the v-lane left edge %v (should stop, leaving the corner)", ht.Right(), vtrack.X)
	}

	vp := tb.textClipRect(bounds)
	if wantBottom := bounds.Y + bounds.H - pad - g; vp.Y+vp.H != wantBottom {
		t.Fatalf("text clip bottom = %v, want %v (excludes the h-lane)", vp.Y+vp.H, wantBottom)
	}
	if vp.Right() != vtrack.X {
		t.Fatalf("text clip right = %v, want v-track X %v (excludes the v-lane)", vp.Right(), vtrack.X)
	}

	before := tb.hscroll
	thumb, _ := tb.hScrollThumbRect()
	tb.hDragging, tb.hDragGrab = true, 0
	tb.dragHScroll(thumb.X + 40) // drag the thumb right
	if tb.hscroll <= before {
		t.Fatalf("dragHScroll did not move hscroll (%v -> %v)", before, tb.hscroll)
	}
}
