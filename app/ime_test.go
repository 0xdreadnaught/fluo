package app

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
)

// recordingIMEAnchor is a platform-agnostic fake imeAnchor: it records every
// rect SetCaretRect is called with, so updateIMEAnchor's wiring can be
// asserted without any Windows/imm32 dependency (see ime_windows.go, which
// this test never touches).
type recordingIMEAnchor struct {
	calls []render.Rect
}

func (r *recordingIMEAnchor) SetCaretRect(rect render.Rect) {
	r.calls = append(r.calls, rect)
}

// caretProbe is a minimal input.CaretRector: a bare core.Element reporting a
// fixed rect while "focused" (rect/ok are just test-set fields, no real
// TextBox involved) — enough to drive Router.FocusedCaretRect without
// pulling in the controls package.
type caretProbe struct {
	core.Element
	rect render.Rect
	ok   bool
}

func (c *caretProbe) CaretScreenRect() (render.Rect, bool) { return c.rect, c.ok }
func (c *caretProbe) AcceptsFocus() bool                   { return true }

func TestUpdateIMEAnchorCalledWithFocusedCaretRect(t *testing.T) {
	router := input.NewRouter()
	c := &caretProbe{rect: render.Rect{X: 10, Y: 20, W: 2, H: 16}, ok: true}
	router.SetRoot(c)
	router.Focus(c)

	anchor := &recordingIMEAnchor{}
	updateIMEAnchor(router, anchor)

	if len(anchor.calls) != 1 {
		t.Fatalf("SetCaretRect calls = %d, want 1", len(anchor.calls))
	}
	if got, want := anchor.calls[0], c.rect; got != want {
		t.Fatalf("SetCaretRect rect = %v, want %v", got, want)
	}
}

func TestUpdateIMEAnchorNotCalledWithoutFocus(t *testing.T) {
	router := input.NewRouter()
	c := &caretProbe{rect: render.Rect{X: 10, Y: 20, W: 2, H: 16}, ok: true}
	router.SetRoot(c)
	// Deliberately never focused.

	anchor := &recordingIMEAnchor{}
	updateIMEAnchor(router, anchor)

	if len(anchor.calls) != 0 {
		t.Fatalf("SetCaretRect calls = %d, want 0 (nothing focused)", len(anchor.calls))
	}
}

func TestUpdateIMEAnchorNotCalledWhenCaretRectorReportsFalse(t *testing.T) {
	router := input.NewRouter()
	c := &caretProbe{ok: false} // focused, but reports no caret right now
	router.SetRoot(c)
	router.Focus(c)

	anchor := &recordingIMEAnchor{}
	updateIMEAnchor(router, anchor)

	if len(anchor.calls) != 0 {
		t.Fatalf("SetCaretRect calls = %d, want 0 (CaretRector reported ok=false)", len(anchor.calls))
	}
}
