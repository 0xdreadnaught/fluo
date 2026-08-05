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
	KeyPageUp    Key = 266
	KeyPageDown  Key = 267
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

// MultiClickInterval is the longest gap, in seconds, allowed between two
// consecutive presses of the same button for the second to continue the same
// click run (making it a double-click, then a triple-click) rather than
// starting a fresh one — see Router.PointerButton and PointerEvent.ClickCount.
// 0.4s sits just under the usual platform default (Win32's own double-click
// time starts at 500ms), erring tight so two deliberately separate clicks are
// less likely to be merged into one run.
const MultiClickInterval = 0.4

// MultiClickDistance is the furthest a press may land from the previous one,
// in logical px on EITHER axis, and still continue the same click run: a
// press outside this box starts a new run even when it falls well inside
// MultiClickInterval, so a click-move-click never reads as a double-click.
// The rectangular (per-axis, not radial) test mirrors Win32's own
// SM_CXDOUBLECLK/SM_CYDOUBLECLK slop, which likewise defaults to 4px.
const MultiClickDistance = 4

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

	// Time is the event's timestamp in monotonic seconds, taken from the
	// clock the host installed with Router.SetTimeSource (a glfw host passes
	// glfw.GetTime — see app.Run). It is 0 on every event when no host clock
	// was installed, which is also the zero value a synthetic event built by
	// hand (a test, or a widget forwarding an event of its own) carries unless
	// it sets one — so treat 0 as "unknown", not as "the beginning of time".
	// Only the three real dispatch entry points stamp it (PointerMove,
	// PointerButton, PointerWheel); the Enter/Leave pairs updateHover derives
	// from a move are notifications about a state change rather than distinct
	// hardware events, and are left at 0.
	Time float64

	// ClickCount is the position of this press within its click run: 1 for a
	// standalone click, 2 for the second press of a double-click, 3 for the
	// third of a triple-click, and upward from there for as long as presses
	// keep arriving within MultiClickInterval and MultiClickDistance of one
	// another (the Router does not wrap or cap the run — a widget that only
	// defines behavior up to 3 should treat anything beyond as "3 or more"
	// rather than assume the count resets). Only Press events carry it —
	// Release, Move, Wheel, Enter, and Leave leave it 0 — and every press
	// carries at least 1, including presses dispatched with no host clock
	// installed: a run cannot be timed without one, so such a press is always
	// reported as standalone rather than guessed at. See Router.PointerButton.
	ClickCount int
}

// KeyEvent represents a keyboard event.
type KeyEvent struct {
	Action Action // Press or Release
	Key    Key    // keyboard key code
	Rune   rune   // character code for char input events; else 0
	Mods   Modifiers

	// Time is the event's timestamp in monotonic seconds, from the same clock
	// PointerEvent.Time uses (Router.SetTimeSource; a glfw host passes
	// glfw.GetTime — see app.Run). It is the keyboard twin of that field: it
	// lets a handler time keystroke runs the way PointerEvent.Time times click
	// runs — e.g. list type-ahead resetting its prefix buffer after a pause. It
	// is 0 ("unknown", not "the beginning of time") on every event when no host
	// clock is installed, and on a synthetic event built by hand that does not
	// set one. Stamped by both real dispatch entry points, KeyDown and KeyUp.
	Time float64

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

// TabStop is an optional interface a Focusable widget may implement to opt OUT
// of the Tab-navigation cycle while STAYING click-focusable. A widget whose
// TabStop() returns false still accepts focus on a press (see FocusFromPath) and
// via Focus(), but FocusNext/FocusPrev skip it — for content that must be
// focusable to be interacted with (a selectable read-only transcript message)
// yet should not clutter the Tab order (a 50-message transcript would otherwise
// be 50 unwanted Tab stops). A Focusable that does not implement TabStop is a
// Tab stop, exactly as before.
type TabStop interface {
	TabStop() bool
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
