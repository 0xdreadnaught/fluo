//go:build !windows

package app

import (
	"github.com/go-gl/glfw/v3.3/glfw"

	"github.com/0xdreadnaught/fluo/input"
)

// noopIMEComposition implements imeComposition everywhere but Windows: none
// of fluo's other build targets need a WndProc subclass to decode IME
// composition messages (see ime_composition_windows.go) — every other glfw
// platform's own text-input handling already delivers committed characters
// through the ordinary SetCharCallback path, with no separate inline-preedit
// concept for this seam to intercept. Close is a no-op.
type noopIMEComposition struct{}

func (noopIMEComposition) Close() {}

// installIMEComposition's non-Windows stub ignores both arguments — there
// is no WndProc to subclass in a non-Windows glfw build (see
// ime_composition_windows.go) — and returns the no-op, so Run's shared
// setup can call the imeComposition seam unconditionally on every platform.
func installIMEComposition(win *glfw.Window, router *input.Router) imeComposition {
	return noopIMEComposition{}
}
