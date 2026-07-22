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

	// captureStack is the NESTED pointer-capture stack: the last element (if
	// any) is the widget currently holding the exclusive pointer grab, and
	// every element before it is a captor that was itself active when a
	// later Capture call pushed over it. While non-empty, all pointer events
	// skip hit-testing and go straight to the top widget via deliverDirect
	// (see Capture/Release). Nesting exists so a widget that captures WHILE
	// already inside someone else's capture (e.g. a ScrollViewer thumb drag
	// started from a pointer event an OverlayHost forwarded into an open
	// popup while the host itself holds a modal capture) restores the outer
	// capture on Release rather than clearing it outright.
	captureStack []core.Widget

	// focused is the widget currently holding keyboard focus, or nil if none.
	// Set by Focus (directly, or indirectly via press-to-focus in
	// PointerButton and Tab handling in KeyDown).
	focused core.Widget

	// focusing guards Focus against reentrancy: set true for the duration of
	// the OnFocusChanged callback-firing section, so a callback that itself
	// calls Focus/FocusNext/FocusPrev is ignored rather than recursing (see
	// Focus's doc comment and TestFocusReentrancyIgnored).
	focusing bool

	// clipboard is the host-provided system clipboard access, or nil if the
	// host hasn't wired one (e.g. headless/test routers). Set via
	// SetClipboard; see Clipboard.
	clipboard Clipboard
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
	r.captureStack = nil
	r.Focus(nil)
}

// Root returns the root widget for this router.
func (r *Router) Root() core.Widget {
	return r.root
}

// SetClipboard installs the host-provided system clipboard access. Passing
// nil (the zero value) puts the router back into headless mode.
func (r *Router) SetClipboard(c Clipboard) {
	r.clipboard = c
}

// Clipboard returns the host-provided system clipboard access, or nil if
// none was set (headless/test routers). Callers must nil-check before use.
func (r *Router) Clipboard() Clipboard {
	return r.clipboard
}

// Capture routes all subsequent pointer events to w exclusively, bypassing
// hit-testing and hover, until a matching Release is called.
//
// Capture NESTS rather than simply overwriting: calling Capture while
// another widget already holds the grab pushes w on top, and a later
// Release pops back to that previous captor (see Release) instead of
// clearing capture outright. This matters whenever a capture can begin from
// inside an event that's itself being delivered under someone else's
// capture — e.g. an OverlayHost holds a modal capture while a popup is open
// and forwards pointer events into the popup's own subtree (see
// controls.OverlayHost.OnPointer); if a widget inside that popup (a
// ScrollViewer thumb, say) captures for its own drag, releasing that drag
// must restore the host's modal capture, not silently drop it.
//
// Capture is idempotent when w already IS the current top of the stack: it
// does nothing rather than pushing a second identical entry. Without this, a
// caller that re-asserts its own capture repeatedly (e.g. OverlayHost.
// ShowPopup calling r.Capture(h) again for every additional popup opened
// while h already holds the grab from the first) would accumulate one stack
// entry per call, and a single matching Release would only pop one of them
// — leaving Captured() == w even after the caller believes it fully
// released, permanently wedging pointer dispatch. Capture with a genuinely
// DIFFERENT widget than the current top still nests normally.
func (r *Router) Capture(w core.Widget) {
	if len(r.captureStack) > 0 && r.captureStack[len(r.captureStack)-1] == w {
		return
	}
	r.captureStack = append(r.captureStack, w)
}

// Release ends the CURRENT (topmost) pointer capture, if any, restoring
// whichever capture (if any) was active before it — see Capture's doc
// comment on nesting. Calling Release with no capture active is a no-op.
// Only once the stack is fully unwound (Captured() == nil) does the next
// PointerMove recompute the hover path from scratch (hover is left
// untouched for the entire time ANY capture, nested or not, is active, so it
// is diffed against whatever it was before the outermost capture began).
func (r *Router) Release() {
	if len(r.captureStack) == 0 {
		return
	}
	r.captureStack = r.captureStack[:len(r.captureStack)-1]
}

// Captured returns the widget currently holding the pointer capture (the top
// of the nested capture stack — see Capture), or nil if none.
func (r *Router) Captured() core.Widget {
	if len(r.captureStack) == 0 {
		return nil
	}
	return r.captureStack[len(r.captureStack)-1]
}

// subtreeContainsIdentity reports whether target is w itself, or appears
// anywhere within w's subtree, walked via Children() (compared by identity,
// ==). Used by Detach to determine subtree membership — deliberately a
// Children() walk rather than core.ParentOf, since a widget being detached
// (e.g. a closing popup) may already be unparented by the time Detach runs.
func subtreeContainsIdentity(w, target core.Widget) bool {
	if w == target {
		return true
	}
	for _, c := range w.Children() {
		if subtreeContainsIdentity(c, target) {
			return true
		}
	}
	return false
}

// Detach clears hover/capture/focus references that point at w OR any
// widget in w's subtree (walk via Children). Call before removing a subtree
// (popup close), so the router doesn't keep dispatching to, or holding
// capture/focus on, widgets that are about to become unreachable.
//
// Focus is cleared through Focus(nil) when the focused widget is w or in
// w's subtree, so OnFocusChanged(false) still fires on it (and Focus's
// reentrancy guard still applies) exactly as it does for SetRoot. Capture
// and hover, by contrast, are cleared silently — capture has no
// release-notification concept, and hover's Enter/Leave pair is meaningless
// for a widget that's being torn down rather than merely un-hovered.
//
// Capture is a STACK (see Capture's doc comment): every entry in it — not
// just the current top — is checked against w's subtree and filtered out if
// it falls inside it, so a detach that happens to remove a middle entry
// (an outer capture whose holder is being torn down while an inner one is
// still active) doesn't leave a dangling reference behind it either. After
// filtering, the new top of the surviving stack (if any) becomes the active
// capture — e.g. if w's subtree held the current (innermost) captor but not
// an outer one further down the stack, the outer capture is what Captured()
// reports afterward, exactly as if that widget's own Release had already run.
func (r *Router) Detach(w core.Widget) {
	if w == nil {
		return
	}
	if r.focused != nil && subtreeContainsIdentity(w, r.focused) {
		r.Focus(nil)
	}
	if len(r.captureStack) > 0 {
		kept := r.captureStack[:0:0]
		for _, c := range r.captureStack {
			if !subtreeContainsIdentity(w, c) {
				kept = append(kept, c)
			}
		}
		r.captureStack = kept
	}
	if len(r.hover) > 0 {
		kept := r.hover[:0:0]
		for _, h := range r.hover {
			if !subtreeContainsIdentity(w, h) {
				kept = append(kept, h)
			}
		}
		r.hover = kept
	}
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
// button/delta don't apply to their action) targeted at the CURRENT (top of
// stack) captured widget, and delivers it directly (no hit-testing, no
// bubbling), returning the event so callers that need the captured widget's
// own cursor (PointerMove) can inspect it afterward. Shared by
// PointerMove/PointerButton/PointerWheel's captured branch, which is
// otherwise identical across all three: build an event with Target =
// Captured(), deliver it direct, done — even when the event's position falls
// outside the captured widget's bounds. Note the captured widget's own
// OnPointer may itself call Capture/Release during this delivery (nesting or
// unwinding the stack) — deliverCaptured reads Captured() once, up front, so
// that mid-call change doesn't retroactively affect which widget THIS event
// was delivered to.
func (r *Router) deliverCaptured(action Action, pos render.Point, button Button, delta render.Point, mods Modifiers) *PointerEvent {
	top := r.Captured()
	e := &PointerEvent{Action: action, Pos: pos, Button: button, Delta: delta, Mods: mods, Target: top, Router: r}
	deliverDirect(top, e)
	return e
}

// Bubble delivers e leaf→root along path: every widget on path that
// implements PointerHandler receives e (Target is pre-set to the leaf, the
// same *PointerEvent instance is reused for the whole walk so a handler's
// e.Handled is visible to the loop), stopping as soon as e.Handled is set.
// A nil/empty path is a no-op.
//
// Exported so callers outside this package can replay the same leaf→root
// delivery over a path they compute themselves — e.g. controls.OverlayHost
// forwards captured pointer events into an open popup's subtree via
// HitPath(popup, e.Pos) + Bubble, since it can't reach Router's private
// dispatch while it holds the pointer capture. Router's own dispatch
// (PointerMove/PointerButton/PointerWheel) uses this same function
// internally; behavior is unchanged.
func Bubble(path []core.Widget, e *PointerEvent) {
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
	if top := r.Captured(); top != nil {
		r.deliverCaptured(Move, p, 0, render.Point{}, mods)
		if cs, ok := top.(CursorShaper); ok {
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
	Bubble(path, &PointerEvent{Action: Move, Pos: p, Mods: mods, Router: r})
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

	if r.Captured() != nil {
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
	Bubble(path, &PointerEvent{Action: action, Pos: p, Button: b, Mods: mods, Router: r})
}

// PointerWheel routes a wheel/scroll event at p (logical px), bubbling
// leaf→root like a pointer event (Action: Wheel), or going only to the
// captured widget if one holds the pointer grab.
func (r *Router) PointerWheel(delta render.Point, p render.Point, mods Modifiers) {
	if r.Captured() != nil {
		r.deliverCaptured(Wheel, p, 0, delta, mods)
		return
	}
	// No root set yet and nothing captured: nothing to hit-test.
	if r.root == nil {
		return
	}

	path := HitPath(r.root, p)
	Bubble(path, &PointerEvent{Action: Wheel, Pos: p, Delta: delta, Mods: mods, Router: r})
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
