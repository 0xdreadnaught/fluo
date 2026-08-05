package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/input"
)

// TestTextViewCtrlShiftCBubbles is the v0.23.1 regression: a focused TextView
// must NOT swallow Ctrl+Shift+C as a copy — it leaves it unhandled so a host
// binding fires. Plain Ctrl+C still copies.
func TestTextViewCtrlShiftCBubbles(t *testing.T) {
	tv := NewTextView(buttonFace(t))
	tv.SetText("abcdef")
	tv.Select(0, 6)
	r := input.NewRouter()
	r.SetRoot(tv)
	clip := &fakeClip{}
	r.SetClipboard(clip)
	layoutTextView(tv, 300, 40)
	r.Focus(tv)

	if consumed := r.KeyDown(input.KeyC, 0, input.ModCtrl|input.ModShift); consumed {
		t.Fatal("Ctrl+Shift+C consumed by focused TextView, want unhandled (must bubble to the host)")
	}
	if clip.text != "" {
		t.Fatalf("Ctrl+Shift+C copied %q, want no copy", clip.text)
	}

	// Sanity: plain Ctrl+C still copies.
	if consumed := r.KeyDown(input.KeyC, 0, input.ModCtrl); !consumed || clip.text != "abcdef" {
		t.Fatalf("plain Ctrl+C: consumed=%v clip=%q, want true/abcdef", consumed, clip.text)
	}
}

// TestTextBoxCtrlShiftCBubblesButKeepsShiftBindings: a focused TextBox leaves
// Ctrl+Shift+C unhandled, still copies on plain Ctrl+C, and — critically — keeps
// Ctrl+Shift+Z (redo), proving the shift-guard is per-case on C/X/V/A only, not
// the whole Ctrl block.
func TestTextBoxCtrlShiftCBubblesButKeepsShiftBindings(t *testing.T) {
	tb, r := newFocusedTextBox(t, "abcdef")
	tb.Select(0, 6)
	clip := &fakeClip{}
	r.SetClipboard(clip)

	if consumed := r.KeyDown(input.KeyC, 0, input.ModCtrl|input.ModShift); consumed || clip.text != "" {
		t.Fatalf("Ctrl+Shift+C: consumed=%v clip=%q, want false/empty (must bubble)", consumed, clip.text)
	}
	if consumed := r.KeyDown(input.KeyC, 0, input.ModCtrl); !consumed || clip.text != "abcdef" {
		t.Fatalf("plain Ctrl+C: consumed=%v clip=%q, want true/abcdef", consumed, clip.text)
	}

	// Ctrl+Shift+Z (redo) must STILL be consumed — the guard must not touch it.
	if consumed := r.KeyDown(input.KeyZ, 0, input.ModCtrl|input.ModShift); !consumed {
		t.Fatal("Ctrl+Shift+Z not consumed — the shift-guard must be per-case (C/X/V/A), leaving redo intact")
	}
}
