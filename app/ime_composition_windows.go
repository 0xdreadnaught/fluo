//go:build windows

package app

import (
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/go-gl/glfw/v3.3/glfw"

	"github.com/0xdreadnaught/fluo/input"
)

// user32 and its procs are resolved lazily (syscall.NewLazyDLL/NewProc),
// matching ime_windows.go's own imm32 convention — stdlib syscall only, no
// cgo, no golang.org/x/sys dependency. procImmGetCompositionStringW is the
// one further imm32 entry point this file needs beyond what ime_windows.go
// already resolved (procImmGetContext/procImmReleaseContext, reused
// as-is below).
var (
	user32                = syscall.NewLazyDLL("user32.dll")
	procSetWindowLongPtrW = user32.NewProc("SetWindowLongPtrW")
	procCallWindowProcW   = user32.NewProc("CallWindowProcW")

	procImmGetCompositionStringW = imm32.NewProc("ImmGetCompositionStringW")
)

// gwlpWndProc is GWLP_WNDPROC (winuser.h): the SetWindowLongPtrW index that
// replaces a window's WndProc pointer — this file's subclassing hook.
// GWL_*/GWLP_* indices are ordinary (negative) ints at the Win32 ABI level.
// It is a mutable int32 rather than an untyped constant on purpose: the
// uintptr(int32(...)) conversion at the call sites must happen at RUNTIME,
// because Go rejects the equivalent constant conversion (-4 is not
// representable in unsigned uintptr). At runtime the int32 sign-extends to
// the correct 64-bit bit pattern for the x64 calling convention
// (uintptr(int32(-4)) == 0xFFFFFFFFFFFFFFFC), matching how other Win32
// bindings handle these same negative indices.
//
// ASSUMPTION (unverified without a live Windows build): SetWindowLongPtrW
// is resolved from user32.dll unconditionally here, with no 32-bit/64-bit
// branch to SetWindowLongW — this project's Windows target is assumed to be
// amd64, where SetWindowLongPtrW is user32's real, direct export. On 32-bit
// Windows, SetWindowLongPtrW is documented as a compatibility macro around
// SetWindowLongW, but Microsoft has also exported it as a real symbol in
// 32-bit user32.dll since XP for exactly this kind of code; if that turns
// out not to hold for whatever 32-bit target (if any) this ever needs to
// run on, this Call becomes a NewProc lookup failure at runtime, not a
// build failure.
var gwlpWndProc int32 = -4

// WM_IME_* message IDs and GCS_*/ISC_* flags this file decodes (winuser.h /
// imm.h). wmImeSetContext's ISC_SHOWUICOMPOSITIONWINDOW bit is cleared
// (not the whole message) before chaining to the original WndProc — see
// installIMEComposition's doc comment for why.
const (
	wmImeSetContext       = 0x0281
	wmImeStartComposition = 0x010D
	wmImeComposition      = 0x010F
	wmImeEndComposition   = 0x010E

	gcsCompStr   = 0x0008
	gcsCursorPos = 0x0080
	gcsResultStr = 0x0800

	iscShowUICompositionWindow = 0x80000000
)

// windowsIMEComposition implements imeComposition for the Win32 platform:
// it holds the window's HWND (shared with windowsIMEAnchor via hwndOf —
// ime_windows.go), the previous WndProc (so every message this subclass
// doesn't fully own keeps flowing to glfw's own window procedure), the
// Router composition events are dispatched into, and the Go closure
// wrapping wndProc kept alive for the window's lifetime (cb — see
// installIMEComposition's doc comment on why a reference must be held).
type windowsIMEComposition struct {
	hwnd     syscall.Handle
	origProc uintptr
	router   *input.Router

	cb func(hwnd, msg, wparam, lparam uintptr) uintptr

	// committed tracks whether the composition currently in progress has
	// already dispatched a CompositionCommit — see wndProc's
	// wmImeEndComposition case: a committing WM_IME_COMPOSITION always
	// precedes WM_IME_ENDCOMPOSITION (which already ended the composition
	// on the Router/TextBox side via CompositionCommit), so a further
	// CompositionCancel there would be redundant; it only fires when the
	// composition ended WITHOUT ever committing (e.g. Escape).
	committed bool
}

// installIMEComposition subclasses win's WndProc (via user32
// SetWindowLongPtrW/GWLP_WNDPROC) to decode WM_IME_STARTCOMPOSITION/
// WM_IME_COMPOSITION/WM_IME_ENDCOMPOSITION into router.CompositionUpdate/
// CompositionCommit/CompositionCancel calls — Task 6 Phase B's Windows
// decoder, built on Task 5's HWND acquisition (hwndOf, ime_windows.go) so
// there is a single point deriving it, rather than a second parallel
// GetWin32Window()/unsafe.Pointer conversion here.
//
// Also intercepts WM_IME_SETCONTEXT to suppress the OS's own
// composition-window UI (clearing ISC_SHOWUICOMPOSITIONWINDOW from lParam
// before chaining to the original proc): controls.TextBox now draws its own
// inline preedit (see its renderComposing), so letting the OS ALSO pop up
// its floating composition bubble at the same anchored position (Task 5's
// ImmSetCompositionWindow) would double the visible UI. The candidate LIST
// window (a CJK IME's picker among ambiguous readings, which fluo has no
// in-app equivalent for) is deliberately left alone — only the composition
// bubble bit is cleared, not ISC_SHOWUICANDIDATEWINDOW.
//
// ASSUMPTION (unverified without a live Windows build — this file's
// riskiest one): WM_IME_STARTCOMPOSITION/WM_IME_COMPOSITION/
// WM_IME_ENDCOMPOSITION are NOT forwarded to glfw's original WndProc at all
// (wndProc returns 0 directly for all three) specifically so the OS's own
// default IME handling never ALSO turns a committed result string into
// synthesized WM_CHAR messages, which would double-insert it through
// glfw's existing SetCharCallback commit path. This mirrors the standard
// recipe other IME-aware Win32 text editors use for exactly this reason,
// but it has not been exercised against a live IME here.
//
// syscall.NewCallback's Go function must not be garbage-collected while
// Win32 holds its wrapped pointer, so the closure is stored on the
// returned *windowsIMEComposition (h.cb) for the whole window's lifetime,
// rather than passed to NewCallback as a bare literal and left with no
// other reference.
func installIMEComposition(win *glfw.Window, router *input.Router) imeComposition {
	h := &windowsIMEComposition{hwnd: hwndOf(win), router: router}
	h.cb = h.wndProc
	cbPtr := syscall.NewCallback(h.cb)
	orig, _, _ := procSetWindowLongPtrW.Call(uintptr(h.hwnd), uintptr(int32(gwlpWndProc)), cbPtr)
	h.origProc = orig
	return h
}

// Close restores the window's original WndProc — implements imeComposition;
// Run defers it alongside every other per-window teardown. A no-op if
// installIMEComposition never actually captured a previous WndProc (orig
// would be 0 only if SetWindowLongPtrW itself failed).
func (h *windowsIMEComposition) Close() {
	if h.origProc == 0 {
		return
	}
	procSetWindowLongPtrW.Call(uintptr(h.hwnd), uintptr(int32(gwlpWndProc)), h.origProc)
	h.origProc = 0
}

// callOriginal chains to whatever WndProc this subclass replaced (glfw's
// own), for every message this decoder does not fully own itself.
func (h *windowsIMEComposition) callOriginal(hwnd, msg, wparam, lparam uintptr) uintptr {
	ret, _, _ := procCallWindowProcW.Call(h.origProc, hwnd, msg, wparam, lparam)
	return ret
}

// wndProc is the subclassed WndProc. WM_IME_STARTCOMPOSITION/
// WM_IME_COMPOSITION/WM_IME_ENDCOMPOSITION are fully owned here — see
// installIMEComposition's doc comment for why they are NOT chained to the
// original proc. WM_IME_SETCONTEXT is forwarded to the original proc with
// ISC_SHOWUICOMPOSITIONWINDOW cleared from lParam. Every other message is
// passed through to the original proc completely unmodified.
func (h *windowsIMEComposition) wndProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	switch msg {
	case wmImeSetContext:
		return h.callOriginal(hwnd, msg, wparam, lparam&^iscShowUICompositionWindow)

	case wmImeStartComposition:
		h.committed = false
		return 0

	case wmImeComposition:
		h.handleComposition(lparam)
		return 0

	case wmImeEndComposition:
		if !h.committed {
			h.router.CompositionCancel()
		}
		h.committed = false
		return 0
	}

	return h.callOriginal(hwnd, msg, wparam, lparam)
}

// handleComposition implements the WM_IME_COMPOSITION case: lparam is a
// bitmask of GCS_* flags describing what changed this message. GCS_RESULTSTR
// takes priority — a non-empty result string means the composition just
// committed, dispatched via CompositionCommit (and h.committed latched true
// so the immediately-following WM_IME_ENDCOMPOSITION doesn't also fire a
// cancel). Otherwise, GCS_COMPSTR means the provisional composition string
// itself changed, dispatched via CompositionUpdate with the current
// GCS_CURSORPOS. Both bits can in principle be set on the same message;
// GCS_RESULTSTR wins, since a commit supersedes whatever preedit preceded
// it.
func (h *windowsIMEComposition) handleComposition(lparam uintptr) {
	if lparam&gcsResultStr != 0 {
		if s := h.compositionString(gcsResultStr); s != "" {
			h.committed = true
			h.router.CompositionCommit(s)
			return
		}
	}
	if lparam&gcsCompStr != 0 {
		s := h.compositionString(gcsCompStr)
		caret := h.compositionCursorPos()
		h.router.CompositionUpdate(s, caret)
	}
}

// compositionString reads one of ImmGetCompositionStringW's string-valued
// indices (GCS_COMPSTR or GCS_RESULTSTR) for the current input context,
// decoding its UTF-16 result (ImmGetCompositionStringW always returns
// UTF-16 code units, regardless of the W/A suffix distinction — the W
// variant simply avoids an extra ANSI round-trip) to a Go string via
// unicode/utf16.Decode. Returns "" if there is no input context
// (ImmGetContext returns NULL) or the string is empty — mirroring
// ime_windows.go's SetCaretRect's own "NULL HIMC is a silent no-op"
// convention.
func (h *windowsIMEComposition) compositionString(index uintptr) string {
	himc, _, _ := procImmGetContext.Call(uintptr(h.hwnd))
	if himc == 0 {
		return ""
	}
	defer procImmReleaseContext.Call(uintptr(h.hwnd), himc)

	// First call with no buffer: the return value is the required buffer
	// size in BYTES (per ImmGetCompositionStringW's documented convention
	// for string-valued indices).
	n, _, _ := procImmGetCompositionStringW.Call(himc, index, 0, 0)
	byteLen := int32(n)
	if byteLen <= 0 {
		return ""
	}
	buf := make([]uint16, byteLen/2)
	procImmGetCompositionStringW.Call(himc, index, uintptr(unsafe.Pointer(&buf[0])), uintptr(byteLen))
	return string(utf16.Decode(buf))
}

// compositionCursorPos reads GCS_CURSORPOS for the current input context —
// unlike compositionString's indices, ImmGetCompositionStringW's return
// value for GCS_CURSORPOS IS the answer directly (a character offset into
// the composition string), not a byte length paired with a follow-up
// buffer read. Returns 0 with no input context.
func (h *windowsIMEComposition) compositionCursorPos() int {
	himc, _, _ := procImmGetContext.Call(uintptr(h.hwnd))
	if himc == 0 {
		return 0
	}
	defer procImmReleaseContext.Call(uintptr(h.hwnd), himc)

	pos, _, _ := procImmGetCompositionStringW.Call(himc, gcsCursorPos, 0, 0)
	return int(int32(pos))
}
