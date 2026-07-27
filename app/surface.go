package app

import (
	"time"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/timers"
)

// Surface renders a fluo widget tree and routes input, into a caller-owned GL
// context. The caller owns the glfw window, GL context, and frame loop;
// Surface owns layout, rendering, the Router, and a Timers queue. Run (this
// package's own glfw-owning host) is built on top of a Surface; a consumer
// embedding fluo in its own engine loop drives one directly instead.
type Surface struct {
	root   core.Widget
	router *input.Router
	timers *timers.Queue

	// lastSize is the logical size Frame last laid out against, so Frame can
	// tell "size changed" apart from "root is merely dirty" without the
	// caller having to track it. Starts as the zero Size, which is never a
	// real window size, so the very first Frame call always lays out.
	lastSize render.Size
}

// NewSurface creates a Surface with its own Router and Timers queue. Call
// SetRoot before the first Frame that should render anything; a Surface with
// no root just advances its Timers each Frame (a valid, if unusual, use —
// e.g. driving timers/animations decoupled from any visible tree).
func NewSurface() *Surface {
	return &Surface{
		router: input.NewRouter(),
		timers: timers.NewQueue(time.Now()),
	}
}

// SetRoot installs w as both the widget tree Frame lays out/renders and the
// Router's hit-test root (via Router().SetRoot, which itself resets
// hover/capture/focus). Resets the tracked last-seen size so the next Frame
// always re-lays-out the new tree regardless of whether the window size
// happens to match the old tree's. Returns s for chaining.
func (s *Surface) SetRoot(w core.Widget) *Surface {
	s.root = w
	s.router.SetRoot(w)
	s.lastSize = render.Size{}
	return s
}

// Router returns the Surface's input router. Exposed for callers that want
// to reach it directly (e.g. to Capture/Focus a widget); the Pointer*/Key*
// forwarders below exist for callers translating their own event types that
// would rather not import the input package's Router just to feed events.
func (s *Surface) Router() *input.Router { return s.router }

// Timers returns the Surface's timer queue.
func (s *Surface) Timers() *timers.Queue { return s.timers }

// layoutDirty is satisfied by every concrete widget in this codebase (each
// embeds core.Element, which implements it) but is asserted narrowly here —
// via the Widget interface alone there is no way to ask "do you need
// layout" — rather than importing a stronger requirement into core.Widget
// itself.
type layoutDirty interface{ NeedsLayout() bool }

// Frame draws one frame: advances the Timers queue against time.Now(), then
// — if the root is set — (re)lays it out when the logical size has changed
// since the last Frame call or the root reports NeedsLayout (a fresh root,
// or one mutated since the last frame), and renders it via r. A nil root
// (never set via SetRoot) is valid and just advances timers.
//
// winW/winH are the window's logical size (window coordinates); fbW/fbH are
// the framebuffer size in device px. scale = fbW/winW, guarding winW == 0
// (matching the convention Ctx.Scale documents in window.go).
func (s *Surface) Frame(r render.Renderer, winW, winH, fbW, fbH int) {
	s.timers.Advance(time.Now())

	if s.root == nil {
		return
	}

	var scale float32 = 1
	if winW != 0 {
		scale = float32(fbW) / float32(winW)
	}

	size := render.Size{W: float32(winW), H: float32(winH)}
	needsLayout := size != s.lastSize
	if ld, ok := s.root.(layoutDirty); ok {
		needsLayout = needsLayout || ld.NeedsLayout()
	}
	if needsLayout {
		s.lastSize = size
		core.MeasureWidget(s.root, size)
		core.ArrangeWidget(s.root, render.Rect{X: 0, Y: 0, W: size.W, H: size.H})
	}

	r.Begin(fbW, fbH, scale)
	core.RenderWidget(s.root, r)
	r.End()
}

// PointerMove forwards to Router().PointerMove — see its doc for behavior
// and the returned Cursor.
func (s *Surface) PointerMove(p render.Point, mods input.Modifiers) input.Cursor {
	return s.router.PointerMove(p, mods)
}

// PointerButton forwards to Router().PointerButton. The returned consumed
// reports whether a fluo widget took the press/release, for a caller
// driving its own canvas input alongside fluo in the same window (see
// input.Router.PointerButton's doc comment) — when true, the caller should
// not also act on this event itself.
func (s *Surface) PointerButton(b input.Button, press bool, p render.Point, mods input.Modifiers) bool {
	return s.router.PointerButton(b, press, p, mods)
}

// PointerWheel forwards to Router().PointerWheel.
func (s *Surface) PointerWheel(delta, p render.Point, mods input.Modifiers) {
	s.router.PointerWheel(delta, p, mods)
}

// KeyDown forwards to Router().KeyDown. The returned consumed reports
// whether a fluo widget currently holding keyboard focus took the key (see
// input.Router.KeyDown's doc comment) — when true, the caller should not
// also act on this key itself.
func (s *Surface) KeyDown(k input.Key, r rune, mods input.Modifiers) bool {
	return s.router.KeyDown(k, r, mods)
}

// KeyUp forwards to Router().KeyUp.
func (s *Surface) KeyUp(k input.Key, mods input.Modifiers) {
	s.router.KeyUp(k, mods)
}

// WantCapturePointer forwards to Router().WantCapturePointer: true when
// fluo currently wants pointer input (an active capture, or the pointer
// hovering interactive fluo UI). A caller driving its own canvas input
// alongside fluo in the same window can check this ahead of an event to
// decide whether fluo is the one that should act on the pointer right now.
func (s *Surface) WantCapturePointer() bool {
	return s.router.WantCapturePointer()
}

// WantCaptureKeyboard forwards to Router().WantCaptureKeyboard: true when a
// fluo widget currently holds keyboard focus. A caller driving its own
// canvas input alongside fluo in the same window can check this ahead of a
// key event to decide whether fluo is the one that should act on it.
func (s *Surface) WantCaptureKeyboard() bool {
	return s.router.WantCaptureKeyboard()
}
