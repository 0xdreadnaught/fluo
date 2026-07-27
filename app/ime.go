package app

import (
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
)

// imeAnchor abstracts positioning the OS input-method-editor's candidate/
// composition window behind a small, platform-agnostic seam: newIMEAnchor's
// Windows implementation (ime_windows.go) anchors it to the window's HWND
// via imm32, so a CJK candidate list shows up AT the text caret instead of a
// default screen corner; every other platform's implementation (ime_other.go)
// is a no-op, since there is no OS-level IME window to anchor there. Either
// way, Run's frame loop calls SetCaretRect unconditionally (see
// updateIMEAnchor) without any build-tag branching of its own.
type imeAnchor interface {
	// SetCaretRect repositions the OS IME candidate/composition window to
	// track r, the focused text caret's rectangle in window logical
	// coordinates (see input.Router.FocusedCaretRect).
	SetCaretRect(r render.Rect)
}

// updateIMEAnchor queries router.FocusedCaretRect and, only when a focused
// widget currently reports a caret rect (see CaretRector), forwards it to
// anchor.SetCaretRect — cheap on every other frame, since it's then just the
// one FocusedCaretRect call and nothing more. Factored out of Run's frame
// loop as its own small, pure function so it can be unit-tested (with a
// recording fake imeAnchor) without any platform/glfw dependency — see
// ime_test.go.
func updateIMEAnchor(router *input.Router, anchor imeAnchor) {
	if r, ok := router.FocusedCaretRect(); ok {
		anchor.SetCaretRect(r)
	}
}

// imeComposition is app's seam for installing (and later tearing down) the
// platform decoder that turns raw OS IME composition messages into
// input.Router.CompositionUpdate/CompositionCommit/CompositionCancel calls
// — Task 6 Phase B's Windows-only counterpart to imeAnchor's Phase A
// candidate-window anchoring above. installIMEComposition's Windows
// implementation (ime_composition_windows.go) subclasses the glfw window's
// WndProc via user32 SetWindowLongPtrW to intercept
// WM_IME_STARTCOMPOSITION/WM_IME_COMPOSITION/WM_IME_ENDCOMPOSITION; every
// other platform's implementation (ime_composition_other.go) is a no-op, on
// the same reasoning as noopIMEAnchor.
//
// Unlike imeAnchor (SetCaretRect, polled once per frame from Run's own
// loop), there is nothing for Run to call after setup beyond eventual
// teardown: composition delivery happens entirely inside the (possibly
// subclassed) WndProc, asynchronously to the frame loop, dispatching
// straight into the router as OS messages arrive. Close releases whatever
// platform resources setup acquired (restoring the original WndProc on
// Windows); Run defers it.
type imeComposition interface {
	Close()
}
