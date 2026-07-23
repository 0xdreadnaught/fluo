// Package app provides a minimal desktop window host: it opens a GLFW
// window with an OpenGL 3.3 core context, wires up render/gl's Renderer,
// and drives a frame(*Ctx) callback every vsync. It is intentionally thin
// — layout, input routing, and window chrome are later phases; this is
// just enough surface for the fluo-demo app and for Phase 2/3 to build on.
package app

import (
	"fmt"
	"runtime"

	"github.com/go-gl/gl/v3.3-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"

	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	glr "github.com/0xdreadnaught/fluo/render/gl"
	"github.com/0xdreadnaught/fluo/timers"
)

func init() { runtime.LockOSThread() }

// Config describes the window to open.
type Config struct {
	Title         string
	Width, Height int // logical px

	// Undecorated, when true, creates the window with no OS-drawn title bar
	// or border (glfw.WindowHint(glfw.Decorated, glfw.False)) — pair with a
	// controls.TitleBar drawn inside the frame callback for custom Fluent
	// window chrome. Wire the TitleBar's OnMinimize/OnMaximize/OnClose to
	// Ctx.Minimize/Ctx.ToggleMaximize/Ctx.Close, and call Ctx.BeginDrag from
	// the frame callback whenever a press lands inside the TitleBar's own
	// DragRegion — see those Ctx fields' doc comments for exactly what each
	// does.
	Undecorated bool
}

// MouseState is the mouse position and left-button state for the current
// frame, in logical pixels.
type MouseState struct {
	Pos  render.Point // logical px
	Down bool         // left button
}

// Ctx is passed to the frame callback every vsync.
type Ctx struct {
	R      render.Renderer
	Size   render.Size   // logical px == window coords (see the DPI note on Run)
	Scale  float32       // fbWidth / windowWidth (1 when windowWidth == 0)
	Mouse  MouseState    // kept for compat; Pos in logical px
	Input  *input.Router // root set once via ctx.Input.SetRoot(root)
	Timers *timers.Queue // Advance()d by the host each frame before frame() runs
	Close  func()        // request app exit

	// Minimize iconifies the window (glfw win.Iconify()). Wire a
	// controls.TitleBar's OnMinimize to this — a decorated window already
	// gets a native minimize button, so this hook only matters once
	// Config.Undecorated has removed the OS chrome. Always non-nil.
	Minimize func()
	// ToggleMaximize toggles the window between maximized and restored
	// (glfw win.Maximize()/win.Restore(), tracked via an internal maximized
	// flag the host owns across frames). Wire a controls.TitleBar's
	// OnMaximize to this. Always non-nil.
	ToggleMaximize func()
	// BeginDrag starts an interactive window move: see Run's doc comment
	// for exactly how the move is implemented (cursor-delta + SetPos, since
	// glfw exposes no native "start an interactive move" call). Call it
	// from the frame callback when a press lands inside a
	// controls.TitleBar's DragRegion; the move continues, tracked entirely
	// by the host's own cursor/button callbacks, until the next left-button
	// release. Always non-nil.
	BeginDrag func()
}

// standardCursors lazily creates and caches the glfw standard cursor
// objects keyed by input.Cursor, so each shape is only allocated once for
// the lifetime of the window.
type standardCursors struct {
	cache map[input.Cursor]*glfw.Cursor
}

func newStandardCursors() *standardCursors {
	return &standardCursors{cache: make(map[input.Cursor]*glfw.Cursor)}
}

// get returns the glfw cursor for shape, creating and caching it on first
// use. input.CursorArrow maps to a nil *glfw.Cursor, which glfw's SetCursor
// treats as "restore the default arrow" — no need to special-case it here
// beyond that nil is a valid, meaningful cache entry.
func (s *standardCursors) get(shape input.Cursor) *glfw.Cursor {
	if c, ok := s.cache[shape]; ok {
		return c
	}
	var std glfw.StandardCursor
	switch shape {
	case input.CursorIBeam:
		std = glfw.IBeamCursor
	case input.CursorHand:
		std = glfw.HandCursor
	case input.CursorHResize:
		std = glfw.HResizeCursor
	case input.CursorVResize:
		std = glfw.VResizeCursor
	default: // input.CursorArrow and anything unrecognized
		std = glfw.ArrowCursor
	}
	c := glfw.CreateStandardCursor(std)
	s.cache[shape] = c
	return c
}

// glfwClipboard adapts a *glfw.Window's clipboard string accessors to
// input.Clipboard. The unexported field means only Run (in this package)
// can construct one; callers elsewhere just see input.Clipboard.
type glfwClipboard struct {
	win *glfw.Window
}

func (c glfwClipboard) Get() string  { return c.win.GetClipboardString() }
func (c glfwClipboard) Set(s string) { c.win.SetClipboardString(s) }

// modsFrom translates glfw's modifier bitmask into input.Modifiers. No glfw
// types appear in any exported app signature — this (and buttonFrom) are the
// only places glfw's input vocabulary is translated into fluo's.
func modsFrom(m glfw.ModifierKey) input.Modifiers {
	var out input.Modifiers
	if m&glfw.ModShift != 0 {
		out |= input.ModShift
	}
	if m&glfw.ModControl != 0 {
		out |= input.ModCtrl
	}
	if m&glfw.ModAlt != 0 {
		out |= input.ModAlt
	}
	if m&glfw.ModSuper != 0 {
		out |= input.ModSuper
	}
	return out
}

// buttonFrom translates a glfw mouse button to input.Button. Buttons glfw
// defines beyond left/right/middle (side buttons, etc.) have no input.Button
// equivalent and map to input.ButtonNone.
func buttonFrom(b glfw.MouseButton) input.Button {
	switch b {
	case glfw.MouseButtonLeft:
		return input.ButtonLeft
	case glfw.MouseButtonRight:
		return input.ButtonRight
	case glfw.MouseButtonMiddle:
		return input.ButtonMiddle
	default:
		return input.ButtonNone
	}
}

// Run opens the window and calls frame every vsync until closed. Blocks.
// Must be called from main(); locks the OS thread.
//
// DPI coordinate-space fix (resolves the Phase-1 review gate): Ctx.Size is
// taken from win.GetSize() — the window's logical size in GLFW's own window
// coordinate system — NOT from the framebuffer size or content scale.
// win.GetCursorPos() is also reported in that same window coordinate system
// by construction (that is what GLFW defines "cursor position" to mean), and
// hit-testing (input.Router/HitPath) operates purely on the Widget tree's
// arranged bounds, which are laid out against Ctx.Size. So cursor position,
// window size, and hit-test bounds are ALL expressed in the same coordinate
// space on every platform GLFW supports, independent of the OS's UI scaling
// or the monitor's backing-store density — there is no separate "logical vs
// physical pixel" conversion for the app layer to get wrong. Scale is
// derived purely for the renderer's benefit (device pixels per window
// coordinate, so glr.Renderer can size its framebuffer-space viewport and
// glyph atlases correctly); it does not feed back into Size or Mouse.Pos.
//
// Undecorated window dragging (WSLg/X11 approach): glfw exposes no native
// "start an interactive move" call the way a real window manager's
// nonclient-area drag does (that's an EWMH/X11-manager-level concept, and
// glfw is deliberately window-manager agnostic), so Ctx.BeginDrag
// reimplements it at the cursor-delta level instead: it records the
// cursor's CURRENT position in glfw's own window-relative coordinate
// system (win.GetCursorPos(), the same space Ctx.Mouse.Pos is reported
// in). On every subsequent cursor-pos callback (until the next left-button
// release), the window is moved by re-querying win.GetPos() FRESH each
// time and adding the window-relative cursor delta (current minus
// recorded-at-grab) to it: because GetPos is re-read live rather than
// cached, the component of the reported cursor position that shifted
// merely because the window itself just moved cancels out algebraically,
// leaving a pure screen-space follow. This is the standard technique for
// glfw-driven borderless-window dragging, and it works unmodified under
// WSLg's XWayland (and any other X11/Win32 glfw backend): SetPos is an
// ordinary "move this top-level window" request that the compositor/window
// manager honors like any other client-initiated move. What's NOT
// reproduced is a true OS-native interactive move (the window manager
// itself owning the drag, with edge-snapping etc.) — a documented v0
// simplification.
func Run(cfg Config, frame func(*Ctx)) error {
	if err := glfw.Init(); err != nil {
		return fmt.Errorf("glfw: %w", err)
	}
	defer glfw.Terminate()

	glfw.WindowHint(glfw.ContextVersionMajor, 3)
	glfw.WindowHint(glfw.ContextVersionMinor, 3)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
	if cfg.Undecorated {
		glfw.WindowHint(glfw.Decorated, glfw.False)
	}

	win, err := glfw.CreateWindow(cfg.Width, cfg.Height, cfg.Title, nil, nil)
	if err != nil {
		return fmt.Errorf("glfw window: %w", err)
	}
	defer win.Destroy()

	win.MakeContextCurrent()
	if err := gl.Init(); err != nil {
		return fmt.Errorf("gl init: %w", err)
	}
	glfw.SwapInterval(1)

	r, err := glr.New()
	if err != nil {
		return fmt.Errorf("renderer: %w", err)
	}

	closeFn := func() { win.SetShouldClose(true) }
	minimizeFn := func() { win.Iconify() }

	// toggleMaximizeFn queries the window's live Maximized attribute (rather
	// than tracking a host-owned bool, which could desync from the actual
	// window state — e.g. a maximized window restored by some OTHER means,
	// were fluo ever to grow one) to decide which of Restore/Maximize to
	// call next.
	toggleMaximizeFn := func() {
		if win.GetAttrib(glfw.Maximized) == glfw.True {
			win.Restore()
		} else {
			win.Maximize()
		}
	}

	// dragging, dragStartX, and dragStartY implement Ctx.BeginDrag's window
	// move (see Run's doc comment above for the cursor-delta math).
	// dragStartX/Y are the window-relative cursor position at the moment
	// BeginDrag was called; dragging gates the cursor-pos callback's move
	// branch and is cleared on the next left-button release.
	var dragging bool
	var dragStartX, dragStartY float64
	beginDragFn := func() {
		dragging = true
		dragStartX, dragStartY = win.GetCursorPos()
	}

	surf := NewSurface()
	router := surf.Router()
	router.SetClipboard(glfwClipboard{win: win})
	cursors := newStandardCursors()
	curCursor := input.CursorArrow
	curMods := input.Modifiers(0)

	// --- callbacks, set BEFORE the event/frame loop starts ---

	win.SetCursorPosCallback(func(_ *glfw.Window, xpos, ypos float64) {
		if dragging {
			if win.GetMouseButton(glfw.MouseButtonLeft) != glfw.Press {
				// Self-heal: the left button was released without this
				// host ever seeing a matching Release callback (focus lost
				// mid-drag, an XWayland event hiccup, etc.) — rather than
				// trust the button-release callback below as the ONLY way
				// out of dragging, check the live button state on every
				// move and bail the moment it's no longer held, so a
				// missed release can never wedge the window into
				// following the cursor forever.
				dragging = false
				return
			}
			wx, wy := win.GetPos()
			win.SetPos(wx+int(xpos-dragStartX), wy+int(ypos-dragStartY))
			return
		}
		p := render.Point{X: float32(xpos), Y: float32(ypos)}
		cur := router.PointerMove(p, curMods)
		if cur != curCursor {
			curCursor = cur
			win.SetCursor(cursors.get(cur))
		}
	})

	win.SetMouseButtonCallback(func(_ *glfw.Window, button glfw.MouseButton, action glfw.Action, mods glfw.ModifierKey) {
		curMods = modsFrom(mods)
		if dragging && button == glfw.MouseButtonLeft && action == glfw.Release {
			dragging = false
		}
		// Falls through to the ordinary router dispatch below even for the
		// release that just ended a drag — intentional: it fires at the
		// cursor's current (post-move) position, which is harmless today
		// (nothing holds a pointer capture across a titlebar drag), but
		// worth flagging for whoever wires capture-driven dragging behind
		// DragRegion later.
		mx, my := win.GetCursorPos()
		p := render.Point{X: float32(mx), Y: float32(my)}
		router.PointerButton(buttonFrom(button), action == glfw.Press, p, curMods)
	})

	win.SetFocusCallback(func(_ *glfw.Window, focused bool) {
		if !focused {
			// Losing focus mid-drag (e.g. Alt+Tab, or the WM stealing focus
			// for some other reason) has no reliable matching button-release
			// event on this platform — clear dragging directly rather than
			// wait for a release that may never come.
			dragging = false
		}
	})

	win.SetScrollCallback(func(_ *glfw.Window, xoff, yoff float64) {
		mx, my := win.GetCursorPos()
		p := render.Point{X: float32(mx), Y: float32(my)}
		// Raw notch deltas — no per-pixel scaling here; a ScrollViewer (or
		// other consumer) applies its own step size to these units.
		delta := render.Point{X: float32(xoff), Y: float32(yoff)}
		router.PointerWheel(delta, p, curMods)
	})

	win.SetKeyCallback(func(_ *glfw.Window, key glfw.Key, _ int, action glfw.Action, mods glfw.ModifierKey) {
		curMods = modsFrom(mods)
		switch action {
		case glfw.Press, glfw.Repeat:
			// glfw.Repeat is treated as another KeyDown (no separate repeat
			// concept on the input.Router side).
			router.KeyDown(input.Key(key), 0, curMods)
		case glfw.Release:
			router.KeyUp(input.Key(key), curMods)
		}
	})

	win.SetCharCallback(func(_ *glfw.Window, r rune) {
		// glfw's char callback carries no modifier state of its own; the
		// closest available signal is the mods last seen from a key/mouse
		// event, but char input is text composition (mods are meaningless
		// to it — e.g. Shift is already reflected in the produced rune), so
		// 0 is passed rather than curMods.
		router.KeyDown(0, r, 0)
	})

	for !win.ShouldClose() {
		glfw.PollEvents()

		fbW, fbH := win.GetFramebufferSize()
		winW, winH := win.GetSize()

		var scale float32 = 1
		if winW != 0 {
			scale = float32(fbW) / float32(winW)
		}

		gl.Viewport(0, 0, int32(fbW), int32(fbH))
		gl.ClearColor(0.125, 0.125, 0.14, 1)
		gl.Clear(gl.COLOR_BUFFER_BIT)

		// surf never has a root set here — Run's frame callback owns
		// layout/render itself via Ctx (see fluo-demo/gallery) — so this
		// call's only effect is advancing surf's Timers queue, same as the
		// bare queue.Advance(time.Now()) this replaced.
		surf.Frame(r, winW, winH, fbW, fbH)

		mx, my := win.GetCursorPos()
		ctx := &Ctx{
			R:     r,
			Size:  render.Size{W: float32(winW), H: float32(winH)},
			Scale: scale,
			Mouse: MouseState{
				Pos:  render.Point{X: float32(mx), Y: float32(my)}, // raw cursor pos: same coord space as Size
				Down: win.GetMouseButton(glfw.MouseButtonLeft) == glfw.Press,
			},
			Input:  surf.Router(),
			Timers: surf.Timers(),
			Close:  closeFn,

			Minimize:       minimizeFn,
			ToggleMaximize: toggleMaximizeFn,
			BeginDrag:      beginDragFn,
		}

		r.Begin(fbW, fbH, scale)
		frame(ctx)
		r.End()

		win.SwapBuffers()
	}

	return nil
}
