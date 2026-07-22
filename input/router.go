package input

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
)

// Router owns the widget tree's input-dispatch state: which widgets are
// currently hovered (for Enter/Leave), which widget (if any) holds an active
// pointer capture, and which widget (if any) holds keyboard focus. Pointer
// events (PointerMove/PointerButton/PointerWheel) hit-test and bubble along
// the resulting path, or go straight to the captured widget while one holds
// the grab. Key events (KeyDown/KeyUp) instead bubble from the focused
// widget up its core.ParentOf ancestor chain, since the focused widget need
// not be under the pointer at all; an unhandled Tab/Shift+Tab KeyDown moves
// focus via FocusNext/FocusPrev. See Capture/Release for pointer capture and
// Focus/Focused/FocusNext/FocusPrev for focus and tab navigation.
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

	// focused is the widget currently holding keyboard focus, or nil if none.
	// Set by Focus (directly, or indirectly via press-to-focus in
	// PointerButton and Tab handling in KeyDown).
	focused core.Widget

	// focusing guards Focus against reentrancy: set true for the duration of
	// the OnFocusChanged callback-firing section, so a callback that itself
	// calls Focus/FocusNext/FocusPrev is ignored rather than recursing (see
	// Focus's doc comment and TestFocusReentrancyIgnored).
	focusing bool
}

// NewRouter creates an empty Router. Call SetRoot before dispatching events.
func NewRouter() *Router {
	return &Router{}
}

// SetRoot sets the root widget for this router. SetRoot resets hover,
// capture, and focus: the previous tree's hover path and pointer capture are
// discarded directly (no Enter/Leave or capture-release notifications fire
// for widgets that are about to become unreachable), while focus is cleared
// via Focus(nil) so OnFocusChanged(false) still fires normally on whatever
// widget previously held it.
func (r *Router) SetRoot(w core.Widget) {
	r.root = w
	r.hover = nil
	r.captured = nil
	r.Focus(nil)
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

// deliverCaptured builds a PointerEvent for action (with the given pos,
// button, delta, and mods — callers pass the zero value for whichever of
// button/delta don't apply to their action) targeted at r.captured, and
// delivers it directly (no hit-testing, no bubbling), returning the event so
// callers that need the captured widget's own cursor (PointerMove) can
// inspect it afterward. Shared by PointerMove/PointerButton/PointerWheel's
// captured branch, which is otherwise identical across all three: build an
// event with Target = r.captured, deliver it direct, done — even when the
// event's position falls outside the captured widget's bounds.
func (r *Router) deliverCaptured(action Action, pos render.Point, button Button, delta render.Point, mods Modifiers) *PointerEvent {
	e := &PointerEvent{Action: action, Pos: pos, Button: button, Delta: delta, Mods: mods, Target: r.captured, Router: r}
	deliverDirect(r.captured, e)
	return e
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
		r.deliverCaptured(Move, p, 0, render.Point{}, mods)
		if cs, ok := r.captured.(CursorShaper); ok {
			return cs.Cursor()
		}
		return CursorArrow
	}
	// No root set yet (e.g. a glfw host wiring callbacks before its first
	// frame calls SetRoot) and nothing captured: nothing to hit-test.
	if r.root == nil {
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
		r.deliverCaptured(action, p, b, render.Point{}, mods)
		return
	}
	// No root set yet and nothing captured: nothing to hit-test or focus.
	if r.root == nil {
		return
	}

	path := HitPath(r.root, p)
	if press {
		r.focusFromPath(path)
	}
	dispatchBubble(path, &PointerEvent{Action: action, Pos: p, Button: b, Mods: mods, Router: r})
}

// PointerWheel routes a wheel/scroll event at p (logical px), bubbling
// leaf→root like a pointer event (Action: Wheel), or going only to the
// captured widget if one holds the pointer grab.
func (r *Router) PointerWheel(delta render.Point, p render.Point, mods Modifiers) {
	if r.captured != nil {
		r.deliverCaptured(Wheel, p, 0, delta, mods)
		return
	}
	// No root set yet and nothing captured: nothing to hit-test.
	if r.root == nil {
		return
	}

	path := HitPath(r.root, p)
	dispatchBubble(path, &PointerEvent{Action: Wheel, Pos: p, Delta: delta, Mods: mods, Router: r})
}

// focusFromPath implements press-to-focus: given the hit-test path (root→
// leaf) of an uncaptured press, it walks leaf→root and focuses the first
// widget that implements Focusable with AcceptsFocus() == true. If no widget
// on the path qualifies — including an empty path, i.e. a press that hit
// nothing — focus is RETAINED: pressing on a non-focusable widget (or empty
// space) leaves whatever is currently focused alone rather than clearing it.
// Focus(nil) remains available for callers that want to clear focus
// explicitly. Focus itself is a no-op when the target is already focused.
func (r *Router) focusFromPath(path []core.Widget) {
	for i := len(path) - 1; i >= 0; i-- {
		if f, ok := path[i].(Focusable); ok && f.AcceptsFocus() {
			r.Focus(path[i])
			return
		}
	}
}

// Focus sets w as the focused widget, or clears focus entirely when w is
// nil. A no-op if w is already focused. Otherwise fires OnFocusChanged(false)
// on the previously focused widget (if it implements FocusHandler), then
// OnFocusChanged(true) on w (likewise), in that order.
//
// Focus is not reentrant: focus changes requested from within an
// OnFocusChanged callback (via Focus, FocusNext, or FocusPrev, from either
// the blurring or the focusing widget) are ignored. Without this guard, two
// widgets that each try to reclaim focus from within OnFocusChanged could
// recurse into each other unboundedly.
func (r *Router) Focus(w core.Widget) {
	if r.focusing {
		return
	}
	if r.focused == w {
		return
	}
	old := r.focused
	r.focused = w

	r.focusing = true
	defer func() { r.focusing = false }()

	if old != nil {
		if fh, ok := old.(FocusHandler); ok {
			fh.OnFocusChanged(false)
		}
	}
	if w != nil {
		if fh, ok := w.(FocusHandler); ok {
			fh.OnFocusChanged(true)
		}
	}
}

// Focused returns the widget currently holding keyboard focus, or nil.
func (r *Router) Focused() core.Widget {
	return r.focused
}

// focusableList returns the widgets under (and including) w that are both
// visible (core.IsVisible) and implement Focusable with AcceptsFocus() ==
// true, in DFS document order (a widget before its children, children in
// Children() order). A hidden widget's entire subtree is skipped, matching
// HitPath's and RenderWidget's treatment of hidden subtrees.
func focusableList(w core.Widget) []core.Widget {
	if w == nil || !core.IsVisible(w) {
		return nil
	}
	var out []core.Widget
	if f, ok := w.(Focusable); ok && f.AcceptsFocus() {
		out = append(out, w)
	}
	for _, c := range w.Children() {
		out = append(out, focusableList(c)...)
	}
	return out
}

// indexOfIdentity returns the index of w in list (compared by identity, ==),
// or -1 if absent.
func indexOfIdentity(list []core.Widget, w core.Widget) int {
	for i, x := range list {
		if x == w {
			return i
		}
	}
	return -1
}

// FocusNext moves focus to the next focusable+visible widget in document
// order (DFS from the root), wrapping from the last back to the first. If
// nothing is currently focused (or the focused widget is no longer in the
// list), it focuses the first entry. A no-op if there are no focusable
// widgets at all.
func (r *Router) FocusNext() {
	list := focusableList(r.root)
	if len(list) == 0 {
		return
	}
	idx := indexOfIdentity(list, r.focused)
	if idx == -1 {
		r.Focus(list[0])
		return
	}
	r.Focus(list[(idx+1)%len(list)])
}

// FocusPrev moves focus to the previous focusable+visible widget in document
// order, wrapping from the first back to the last. If nothing is currently
// focused (or the focused widget is no longer in the list), it focuses the
// last entry. A no-op if there are no focusable widgets at all.
func (r *Router) FocusPrev() {
	list := focusableList(r.root)
	if len(list) == 0 {
		return
	}
	idx := indexOfIdentity(list, r.focused)
	if idx == -1 {
		r.Focus(list[len(list)-1])
		return
	}
	r.Focus(list[(idx-1+len(list))%len(list)])
}

// keyChain returns the widget chain leaf→root, starting at w (inclusive) and
// following core.ParentOf links up to (and including) the root. Used to
// bubble key events along the ANCESTOR chain, unlike pointer events which
// bubble along a hit-test path — the focused widget need not be under the
// pointer at all.
func keyChain(w core.Widget) []core.Widget {
	var chain []core.Widget
	for w != nil {
		chain = append(chain, w)
		w = core.ParentOf(w)
	}
	return chain
}

// dispatchKey delivers e to the focused widget and bubbles it up the parent
// chain (core.ParentOf), stopping as soon as e.Handled is set. With no
// focused widget, delivery is to the root only (if it implements KeyHandler
// and a root is set). The root==nil branch is deliberately guarded (a Router
// used before SetRoot, or with an empty tree, must produce an empty chain,
// not a []core.Widget{nil} that would panic on the KeyHandler type
// assertion below).
func (r *Router) dispatchKey(e *KeyEvent) {
	var chain []core.Widget
	if r.focused != nil {
		chain = keyChain(r.focused)
	} else if r.root != nil {
		chain = []core.Widget{r.root}
	}
	for _, w := range chain {
		if h, ok := w.(KeyHandler); ok {
			h.OnKey(e)
			if e.Handled {
				return
			}
		}
	}
}

// KeyDown routes a key-press. rn is the produced character for char-input
// events, else 0. The event bubbles from the focused widget up through its
// parent chain (see dispatchKey). If, after that, KeyTab remains unhandled,
// the router itself consumes it: ModShift moves focus to the previous
// focusable widget, otherwise to the next, and the event is marked handled
// (no re-dispatch — this is router-internal bookkeeping, not delivered to
// any widget).
func (r *Router) KeyDown(k Key, rn rune, mods Modifiers) {
	e := &KeyEvent{Action: Press, Key: k, Rune: rn, Mods: mods, Router: r}
	r.dispatchKey(e)
	if !e.Handled && k == KeyTab {
		if mods&ModShift != 0 {
			r.FocusPrev()
		} else {
			r.FocusNext()
		}
		e.Handled = true
	}
}

// KeyUp routes a key-release the same way KeyDown does (focused widget,
// bubbling up the parent chain), with no Tab handling.
func (r *Router) KeyUp(k Key, mods Modifiers) {
	e := &KeyEvent{Action: Release, Key: k, Mods: mods, Router: r}
	r.dispatchKey(e)
}
