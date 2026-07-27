//go:build !windows

package app

import (
	"github.com/go-gl/glfw/v3.3/glfw"

	"github.com/0xdreadnaught/fluo/render"
)

// noopIMEAnchor implements imeAnchor everywhere but Windows: none of fluo's
// other build targets have an OS IME candidate/composition window to anchor
// (see ime_windows.go), so SetCaretRect does nothing.
type noopIMEAnchor struct{}

func (noopIMEAnchor) SetCaretRect(render.Rect) {}

// newIMEAnchor's non-Windows stub ignores win entirely — the Win32-specific
// HWND accessor it would otherwise need doesn't exist in a non-Windows glfw
// build (see ime_windows.go) — and returns the no-op anchor, so Run's shared
// frame loop can call the imeAnchor seam unconditionally on every platform.
func newIMEAnchor(win *glfw.Window) imeAnchor {
	return noopIMEAnchor{}
}
