package input

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
)

// Button identifies a mouse button.
type Button uint8

const (
	ButtonNone Button = iota
	ButtonLeft
	ButtonRight
	ButtonMiddle
)

// Action describes the event action for pointer and key events.
type Action uint8

const (
	Press Action = iota
	Release
	Move
	Wheel
	Enter
	Leave
)

// Modifiers represents keyboard modifier states as bitflags.
type Modifiers uint8

const (
	ModShift Modifiers = 1 << iota
	ModCtrl
	ModAlt
	ModSuper
)

// Key identifies a keyboard key (values match GLFW keycodes numerically).
type Key int32

const (
	KeyEscape    Key = 256
	KeyEnter     Key = 257
	KeyTab       Key = 258
	KeyBackspace Key = 259
	KeyDelete    Key = 261
	KeyRight     Key = 262
	KeyLeft      Key = 263
	KeyDown      Key = 264
	KeyUp        Key = 265
	KeyHome      Key = 268
	KeyEnd       Key = 269
)

// Cursor identifies a cursor shape.
type Cursor uint8

const (
	CursorArrow Cursor = iota
	CursorIBeam
	CursorHand
	CursorHResize
	CursorVResize
)

// PointerEvent represents a mouse or touch pointer event.
type PointerEvent struct {
	Action  Action       // Press, Release, Move, Wheel, Enter, or Leave
	Pos     render.Point // logical px in window space
	Button  Button       // Press/Release only
	Delta   render.Point // Wheel only (x,y scroll delta)
	Mods    Modifiers    // keyboard modifier states
	Target  core.Widget  // hit leaf widget (or captured widget)
	Router  *Router      // for Capture/Focus calls from handlers
	Handled bool         // set by handlers to prevent propagation
}

// KeyEvent represents a keyboard event.
type KeyEvent struct {
	Action  Action // Press or Release
	Key     Key    // keyboard key code
	Rune    rune   // character code for char input events; else 0
	Mods    Modifiers
	Router  *Router
	Handled bool
}

// PointerHandler is an optional interface for widgets that handle pointer events.
type PointerHandler interface {
	OnPointer(e *PointerEvent)
}

// KeyHandler is an optional interface for widgets that handle keyboard events.
type KeyHandler interface {
	OnKey(e *KeyEvent)
}

// FocusHandler is an optional interface for widgets that react to focus changes.
type FocusHandler interface {
	OnFocusChanged(focused bool)
}

// Focusable is an optional interface for widgets that can accept keyboard focus.
type Focusable interface {
	AcceptsFocus() bool
}

// CursorShaper is an optional interface for widgets that define a custom cursor.
type CursorShaper interface {
	Cursor() Cursor
}
