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

// KeyT identifies the T key. Unlike the named keys above (which GLFW numbers
// starting at 256), GLFW numbers printable/letter keys by their ASCII code,
// so 'T' == 84. Added for the gallery's Light/Dark theme-toggle shortcut.
const KeyT Key = 84

// KeySpace, KeyA, KeyC, KeyV, KeyX, KeyY, and KeyZ identify their respective
// keys. Like KeyT above, GLFW numbers these by ASCII code rather than the
// 256+ scheme used for the named keys further up (KeyEscape..KeyEnd): Space
// == 32, and the letters follow their uppercase ASCII values (A == 65, C ==
// 67, V == 86, X == 88, Y == 89, Z == 90). Added for clipboard shortcuts
// (Ctrl+C/V/X), undo/redo (Ctrl+Z/Y), select-all (Ctrl+A), and Space-as-
// activate on focused controls.
const (
	KeySpace Key = 32
	KeyA     Key = 65
	KeyC     Key = 67
	KeyV     Key = 86
	KeyX     Key = 88
	KeyY     Key = 89
	KeyZ     Key = 90
)

// Clipboard is the host-provided system clipboard access.
type Clipboard interface {
	Get() string
	Set(s string)
}

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

// CaretRector is an optional interface for focusable widgets that expose a
// text caret's on-screen rectangle, so a host can anchor platform UI to it —
// e.g. the Windows OS IME candidate/composition window (see app.Run),
// positioned at the focused text caret rather than a default screen corner.
// See Router.FocusedCaretRect, which type-asserts the currently focused
// widget against this interface.
type CaretRector interface {
	// CaretScreenRect returns the caret's rectangle in window logical
	// coordinates — the same space PointerEvent.Pos and core.Widget bounds
	// are expressed in — and false when the widget currently has no caret
	// to report (e.g. it isn't focused).
	CaretScreenRect() (render.Rect, bool)
}

// CompositionEvent carries an IME composition update or its end (commit or
// cancellation) — see CompositionHandler and Router.CompositionUpdate/
// CompositionCommit/CompositionCancel, the platform-agnostic entry points a
// Windows imm32 decoder (or a test) drives this from.
//
// A composition is in progress while Active is true: Preedit is the current,
// provisional (not yet committed) composition string, and CaretPos is the
// caret's RUNE offset within Preedit (not within the widget's own committed
// text) — e.g. a CJK IME reporting the user is midway through typing a
// candidate. Active becomes false exactly once, when the composition ends:
// either committed (Canceled is false and Committed holds the finalized text
// to insert at the caret) or canceled (Canceled is true; Committed is
// meaningless and nothing should be inserted — e.g. the user pressed Escape,
// or switched focus away mid-composition).
type CompositionEvent struct {
	// Preedit is the provisional composition string, valid while Active is
	// true.
	Preedit string
	// CaretPos is the caret's rune offset within Preedit, valid while Active
	// is true.
	CaretPos int
	// Committed is the finalized text to insert, valid when Active is false
	// and Canceled is false.
	Committed string
	// Active reports whether a composition is currently in progress.
	Active bool
	// Canceled reports whether the composition ended without a commit.
	// Meaningless while Active is true.
	Canceled bool
}

// CompositionHandler is an optional interface for focusable widgets that
// accept IME composition input (inline preedit text, e.g. for CJK entry) —
// see Router.CompositionUpdate/CompositionCommit/CompositionCancel, which
// type-assert the currently focused widget against this interface exactly as
// FocusedCaretRect does against CaretRector.
type CompositionHandler interface {
	OnComposition(e CompositionEvent)
}
