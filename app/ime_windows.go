//go:build windows

package app

import (
	"syscall"
	"unsafe"

	"github.com/go-gl/glfw/v3.3/glfw"

	"github.com/0xdreadnaught/fluo/render"
)

// imm32 and its procs are resolved lazily (syscall.NewLazyDLL/NewProc defer
// the actual LoadLibrary/GetProcAddress until first Call) — stdlib syscall
// only, no cgo, matching the rest of this codebase's platform-glue
// convention (no golang.org/x/sys dependency either).
var (
	imm32                       = syscall.NewLazyDLL("imm32.dll")
	procImmGetContext           = imm32.NewProc("ImmGetContext")
	procImmReleaseContext       = imm32.NewProc("ImmReleaseContext")
	procImmSetCompositionWindow = imm32.NewProc("ImmSetCompositionWindow")
	procImmSetCandidateWindow   = imm32.NewProc("ImmSetCandidateWindow")
)

// CFS_POINT and CFS_CANDIDATEPOS are imm32 COMPOSITIONFORM/CANDIDATEFORM
// dwStyle flags (winuser.h): CFS_POINT pins the composition window's
// ptCurrentPos exactly (no surrounding rcArea confinement — Phase A doesn't
// intercept composition text at all, so there is no rect to confine it to
// yet); CFS_CANDIDATEPOS is CANDIDATEFORM's matching "exact point" style for
// the candidate LIST window, which is the part actually visible to the user
// before inline composition exists.
const (
	cfsPoint        = 0x0002
	cfsCandidatePos = 0x0040
)

// point and rect mirror Win32's POINT/RECT layout exactly (LONG-sized
// fields), defined locally so this file needs nothing beyond stdlib syscall.
type point struct{ X, Y int32 }
type rect struct{ Left, Top, Right, Bottom int32 }

// compositionForm mirrors imm32's COMPOSITIONFORM (winuser.h): dwStyle
// (CFS_* above), ptCurrentPos, and rcArea (unused while dwStyle is
// CFS_POINT — the field is still present so the struct's memory layout
// matches what ImmSetCompositionWindow expects to read).
type compositionForm struct {
	DwStyle      uint32
	PtCurrentPos point
	RcArea       rect
}

// candidateForm mirrors imm32's CANDIDATEFORM (winuser.h): dwIndex (which
// candidate list, always 0 — a single default list), dwStyle
// (CFS_CANDIDATEPOS above), ptCurrentPos, and rcArea (likewise unused for
// this dwStyle).
type candidateForm struct {
	DwIndex      uint32
	DwStyle      uint32
	PtCurrentPos point
	RcArea       rect
}

// windowsIMEAnchor implements imeAnchor for the Win32 platform: it holds the
// window's HWND, queried once at construction (see newIMEAnchor) and
// immutable for the window's lifetime, and repositions the OS IME
// composition/candidate windows via imm32 on every SetCaretRect call.
type windowsIMEAnchor struct {
	hwnd syscall.Handle
}

// newIMEAnchor queries win's HWND via glfw's Win32-native accessor
// ((*glfw.Window).GetWin32Window() C.HWND, native_windows.go in the go-gl/
// glfw binding) and wraps it as a windowsIMEAnchor. The conversion from
// glfw's cgo-typed C.HWND to syscall.Handle goes through unsafe.Pointer
// without this file itself importing "C" or cgo: HWND is a Win32 opaque
// pointer type (DECLARE_HANDLE), so the returned value is pointer-shaped and
// convertible via unsafe.Pointer purely from Go's conversion rules, with no
// need to name the cgo type here.
func newIMEAnchor(win *glfw.Window) imeAnchor {
	hwnd := syscall.Handle(uintptr(unsafe.Pointer(win.GetWin32Window())))
	return &windowsIMEAnchor{hwnd: hwnd}
}

// SetCaretRect implements imeAnchor: anchors the OS IME composition window's
// current-position point to the caret's top-left corner (CFS_POINT) and the
// candidate list's position to the caret's bottom-left corner
// (CFS_CANDIDATEPOS) — the conventional "candidates open just below the
// caret" placement most IMEs use. r is in window logical/client
// coordinates, matching glfw's own window coordinate system (see
// app/window.go's DPI doc comment on Run): ImmSetCompositionWindow/
// ImmSetCandidateWindow both expect client-area coordinates, which for a
// glfw-created window IS this same coordinate space (glfw's content area is
// the Win32 client rect the HWND was created with).
//
// A zero HIMC (ImmGetContext returns NULL — e.g. no IME is active for this
// thread/window right now) is a silent no-op, matching imm32's own
// documented failure convention for a window with no associated input
// context.
func (a *windowsIMEAnchor) SetCaretRect(r render.Rect) {
	himc, _, _ := procImmGetContext.Call(uintptr(a.hwnd))
	if himc == 0 {
		return
	}
	defer procImmReleaseContext.Call(uintptr(a.hwnd), himc)

	cf := compositionForm{
		DwStyle:      cfsPoint,
		PtCurrentPos: point{X: int32(r.X), Y: int32(r.Y)},
	}
	procImmSetCompositionWindow.Call(himc, uintptr(unsafe.Pointer(&cf)))

	cdf := candidateForm{
		DwIndex:      0,
		DwStyle:      cfsCandidatePos,
		PtCurrentPos: point{X: int32(r.X), Y: int32(r.Y + r.H)},
	}
	procImmSetCandidateWindow.Call(himc, uintptr(unsafe.Pointer(&cdf)))
}
