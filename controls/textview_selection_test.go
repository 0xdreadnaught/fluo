package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
)

func layoutTextView(tv *TextView, w, h float32) {
	core.MeasureWidget(tv, render.Size{W: w, H: h})
	core.ArrangeWidget(tv, render.Rect{X: 0, Y: 0, W: w, H: h})
}

// TestTextViewDragSelects: press-drag-release across a (single, wide) row
// selects the dragged rune range.
func TestTextViewDragSelects(t *testing.T) {
	tv := NewTextView(buttonFace(t))
	tv.SetText("abcdef")
	r := input.NewRouter()
	r.SetRoot(tv)
	layoutTextView(tv, 300, 40)

	r.PointerButton(input.ButtonLeft, true, render.Point{X: 0, Y: 5}, 0) // press at rune 0
	r.PointerMove(render.Point{X: 1000, Y: 5}, 0)                        // drag past the end
	r.PointerButton(input.ButtonLeft, false, render.Point{X: 1000, Y: 5}, 0)

	if s, e := tv.Selection(); s != 0 || e != 6 {
		t.Fatalf("Selection = (%d,%d), want (0,6)", s, e)
	}
	if got := tv.SelectedText(); got != "abcdef" {
		t.Fatalf("SelectedText = %q, want %q", got, "abcdef")
	}
}

// TestTextViewCtrlCCopies: Ctrl+C on a focused view with a selection writes the
// selected text to the router clipboard.
func TestTextViewCtrlCCopies(t *testing.T) {
	tv := NewTextView(buttonFace(t))
	tv.SetText("abcdef")
	r := input.NewRouter()
	r.SetRoot(tv)
	clip := &fakeClip{}
	r.SetClipboard(clip)
	layoutTextView(tv, 300, 40)

	r.PointerButton(input.ButtonLeft, true, render.Point{X: 0, Y: 5}, 0)
	r.PointerMove(render.Point{X: 1000, Y: 5}, 0)
	r.PointerButton(input.ButtonLeft, false, render.Point{X: 1000, Y: 5}, 0)
	r.KeyDown(input.KeyC, 0, input.ModCtrl)

	if clip.text != "abcdef" {
		t.Fatalf("clipboard = %q, want %q", clip.text, "abcdef")
	}
}

// TestTextViewCtrlASelectsAll.
func TestTextViewCtrlASelectsAll(t *testing.T) {
	tv := NewTextView(buttonFace(t))
	tv.SetText("hello world")
	r := input.NewRouter()
	r.SetRoot(tv)
	layoutTextView(tv, 300, 40)
	r.Focus(tv)

	r.KeyDown(input.KeyA, 0, input.ModCtrl)
	if s, e := tv.Selection(); s != 0 || e != len([]rune("hello world")) {
		t.Fatalf("Selection after Ctrl+A = (%d,%d), want (0,%d)", s, e, len("hello world"))
	}
}

// TestTextViewSelectionSurvivesAppend: appending at the end never shifts earlier
// rune indices, so a mid-stream selection stays valid.
func TestTextViewSelectionSurvivesAppend(t *testing.T) {
	tv := NewTextView(buttonFace(t))
	tv.SetText("abcdef")
	tv.anchor, tv.caret = 1, 4 // select "bcd"
	tv.Append("ghij")
	if s, e := tv.Selection(); s != 1 || e != 4 {
		t.Fatalf("Selection after Append = (%d,%d), want (1,4) (append must not shift it)", s, e)
	}
	if tv.SelectedText() != "bcd" {
		t.Fatalf("SelectedText = %q, want %q", tv.SelectedText(), "bcd")
	}
}

// TestTextViewSetTextClearsSelection.
func TestTextViewSetTextClearsSelection(t *testing.T) {
	tv := NewTextView(buttonFace(t))
	tv.SetText("abcdef")
	tv.anchor, tv.caret = 1, 4
	tv.SetText("new content")
	if s, e := tv.Selection(); s != e {
		t.Fatalf("Selection after SetText = (%d,%d), want empty", s, e)
	}
}

// TestTextViewClickFocusesButNotInTabCycle covers the PM's tab-order guard: a
// TextView is click-focusable but opts out of the Tab cycle, so FocusNext lands
// on a real tab-stop (a Button) beside it, not on the transcript message — while
// a press still focuses the message.
func TestTextViewClickFocusesButNotInTabCycle(t *testing.T) {
	tv := NewTextView(buttonFace(t))
	tv.SetText("a message")
	tv.SetWidth(100)
	tv.SetHeight(20)
	btn := NewButton(buttonFace(t), "OK")
	btn.SetWidth(40)
	btn.SetHeight(20)
	root := NewCanvas().Add(tv, 0, 0).Add(btn, 0, 40)

	r := input.NewRouter()
	r.SetRoot(root)
	core.MeasureWidget(root, render.Size{W: 200, H: 100})
	core.ArrangeWidget(root, render.Rect{X: 0, Y: 0, W: 200, H: 100})

	// Tab: the only tab-stop is the Button; the TextView is skipped.
	r.FocusNext()
	if r.Focused() != core.Widget(btn) {
		t.Fatalf("FocusNext focused %v, want the Button (TextView must opt out of the Tab cycle)", r.Focused())
	}

	// But a click on the message focuses it (click-focusable).
	r.PointerButton(input.ButtonLeft, true, render.Point{X: 5, Y: 5}, 0)
	r.PointerButton(input.ButtonLeft, false, render.Point{X: 5, Y: 5}, 0)
	if r.Focused() != core.Widget(tv) {
		t.Fatalf("after click, Focused = %v, want the TextView (click-focusable)", r.Focused())
	}
}

// TestTextViewFocusClearsNoStuck is the PM's no-new-stuck-focus guard: a focused
// TextView releases cleanly on Focus(nil) (what a canvas-click ClearFocus does),
// so it can't reintroduce the stuck-focus class.
func TestTextViewFocusClearsNoStuck(t *testing.T) {
	tv := NewTextView(buttonFace(t))
	tv.SetText("msg")
	r := input.NewRouter()
	r.SetRoot(tv)
	layoutTextView(tv, 100, 20)

	r.Focus(tv)
	if r.Focused() != core.Widget(tv) || !tv.focused {
		t.Fatal("precondition: TextView should be focused")
	}
	r.Focus(nil)
	if r.Focused() != nil {
		t.Fatalf("Focused() after Focus(nil) = %v, want nil (no stuck focus)", r.Focused())
	}
	if tv.focused {
		t.Fatal("tv.focused still true after Focus(nil) — OnFocusChanged(false) must run")
	}
}
