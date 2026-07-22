package input

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
)

// Router owns the widget tree's pointer-dispatch state: which widgets are
// currently hovered (for Enter/Leave) and which widget, if any, holds an
// active pointer capture. Keyboard/focus/tab-navigation state is added in a
// later task.
type Router struct {
	root core.Widget

	// hover is the last-computed hit-test path (root→leaf), used to diff
	// against the next PointerMove's path and emit Enter/Leave. Untouched
	// while a capture is active (see Capture).
	hover []core.Widget

	// captured is the widget currently holding an exclusive pointer grab, or
	// nil if none. While set, all pointer events skip hit-testing and go
	// straight to this widget via deliverDirect (see Capture/Release).
	captured core.Widget
}

// NewRouter creates an empty Router. Call SetRoot before dispatching events.
func NewRouter() *Router {
	return &Router{}
}

// SetRoot sets the root widget for this router.
func (r *Router) SetRoot(w core.Widget) {
	r.root = w
}

// Root returns the root widget for this router.
func (r *Router) Root() core.Widget {
	return r.root
}

// Capture routes all subsequent pointer events to w exclusively, bypassing
// hit-testing and hover, until Release is called.
func (r *Router) Capture(w core.Widget) {
	r.captured = w
}

// Release ends the active pointer capture, if any. The next PointerMove
// recomputes the hover path from scratch (hover was left untouched during
// capture, so it is diffed against whatever it was before capture began).
func (r *Router) Release() {
	r.captured = nil
}

// Captured returns the widget currently holding the pointer capture, or nil.
func (r *Router) Captured() core.Widget {
	return r.captured
}

// deliverDirect delivers e to w's OnPointer with no bubbling, if w
// implements PointerHandler. Used for capture delivery and for Enter/Leave,
// both of which are point-to-point, not bubbled (e.Handled is ignored by the
// caller in both cases).
func deliverDirect(w core.Widget, e *PointerEvent) {
	if h, ok := w.(PointerHandler); ok {
		h.OnPointer(e)
	}
}

// dispatchBubble delivers e leaf→root along path: every widget on path that
// implements PointerHandler receives e (Target is pre-set to the leaf, the
// same *PointerEvent instance is reused for the whole walk so a handler's
// e.Handled is visible to the loop), stopping as soon as e.Handled is set.
// A nil/empty path is a no-op.
func dispatchBubble(path []core.Widget, e *PointerEvent) {
	if len(path) == 0 {
		return
	}
	e.Target = path[len(path)-1]
	for i := len(path) - 1; i >= 0; i-- {
		if h, ok := path[i].(PointerHandler); ok {
			h.OnPointer(e)
			if e.Handled {
				return
			}
		}
	}
}

// cursorForPath walks path leaf→root and returns the first CursorShaper's
// cursor, or CursorArrow if none of the path's widgets shape a cursor.
func cursorForPath(path []core.Widget) Cursor {
	for i := len(path) - 1; i >= 0; i-- {
		if cs, ok := path[i].(CursorShaper); ok {
			return cs.Cursor()
		}
	}
	return CursorArrow
}

// containsIdentity reports whether w appears in path, compared by identity
// (==), not by any notion of equivalent bounds/state.
func containsIdentity(path []core.Widget, w core.Widget) bool {
	for _, x := range path {
		if x == w {
			return true
		}
	}
	return false
}

// updateHover diffs r.hover (the previous path) against newPath (identity
// comparison) and delivers direct, non-bubbling Enter/Leave events for the
// widgets that changed hover state, then stores newPath as the new hover
// path. Order choice (undocumented by the spec beyond "diff old vs new"):
// Leave is delivered old-path order (leaf-most departure last) for widgets
// no longer on the path, then Enter is delivered new-path order (root-most
// arrival first) for widgets newly on the path. Both sets are independent
// (a widget can't be in both, since path identity determines membership),
// so the relative order between the two sets is not otherwise significant.
func (r *Router) updateHover(newPath []core.Widget) {
	for _, w := range r.hover {
		if !containsIdentity(newPath, w) {
			deliverDirect(w, &PointerEvent{Action: Leave, Target: w, Router: r})
		}
	}
	for _, w := range newPath {
		if !containsIdentity(r.hover, w) {
			deliverDirect(w, &PointerEvent{Action: Enter, Target: w, Router: r})
		}
	}
	r.hover = newPath
}

// PointerMove routes a pointer-move at p (logical px). While captured, the
// event goes only to the captured widget (no hover, no bubbling) and the
// cursor is the captured widget's own (CursorArrow if it doesn't shape one).
// Otherwise it hit-tests, updates hover (Enter/Leave), bubbles a Move event
// leaf→root, and returns the cursor from the new hover path.
func (r *Router) PointerMove(p render.Point, mods Modifiers) Cursor {
	if r.captured != nil {
		e := &PointerEvent{Action: Move, Pos: p, Mods: mods, Target: r.captured, Router: r}
		deliverDirect(r.captured, e)
		if cs, ok := r.captured.(CursorShaper); ok {
			return cs.Cursor()
		}
		return CursorArrow
	}

	path := HitPath(r.root, p)
	r.updateHover(path)
	dispatchBubble(path, &PointerEvent{Action: Move, Pos: p, Mods: mods, Router: r})
	return cursorForPath(path)
}

// PointerButton routes a press or release at p (logical px). While
// captured, the event goes only to the captured widget, Target = captured,
// no bubbling — including when p falls outside the captured widget's
// bounds. Otherwise it hit-tests and bubbles leaf→root, stopping at the
// first handler that sets e.Handled.
func (r *Router) PointerButton(b Button, press bool, p render.Point, mods Modifiers) {
	action := Release
	if press {
		action = Press
	}

	if r.captured != nil {
		e := &PointerEvent{Action: action, Pos: p, Button: b, Mods: mods, Target: r.captured, Router: r}
		deliverDirect(r.captured, e)
		return
	}

	path := HitPath(r.root, p)
	dispatchBubble(path, &PointerEvent{Action: action, Pos: p, Button: b, Mods: mods, Router: r})
}

// PointerWheel routes a wheel/scroll event at p (logical px), bubbling
// leaf→root like a pointer event (Action: Wheel), or going only to the
// captured widget if one holds the pointer grab.
func (r *Router) PointerWheel(delta render.Point, p render.Point, mods Modifiers) {
	if r.captured != nil {
		e := &PointerEvent{Action: Wheel, Pos: p, Delta: delta, Mods: mods, Target: r.captured, Router: r}
		deliverDirect(r.captured, e)
		return
	}

	path := HitPath(r.root, p)
	dispatchBubble(path, &PointerEvent{Action: Wheel, Pos: p, Delta: delta, Mods: mods, Router: r})
}
