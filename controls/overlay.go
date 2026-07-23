package controls

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
)

// popupEntry is one entry in an OverlayHost's popup stack: the popup widget
// itself, the screen-space anchor rect it was opened against (re-used every
// arrange pass to recompute its position), and the callback (may be nil) to
// fire when it's dismissed.
type popupEntry struct {
	w         core.Widget
	anchor    render.Rect
	onDismiss func()
}

// OverlayHost hosts the app content plus a stack of popups rendered above it.
// It should be the root (or near-root) widget; controls find it via
// OverlayHostFor.
//
// Children are ordered [content, popups...] (last popup = topmost): this
// ordering makes input.HitPath's last-to-first child scan hit-test the
// topmost popup before content, and makes RenderWidget paint it last (i.e.
// above content and every popup beneath it). OverlayHost itself draws
// nothing — Render is the inherited core.Element no-op; content and popups
// render entirely as children.
//
// Light dismiss (see OnPointer) needs a way to stop a stray press from
// reaching content underneath an open popup, and content's own widgets are
// always deeper in the bubble path than OverlayHost (the root), so they'd
// receive a Press before OverlayHost — the standard bubble order — ever ran.
// To pre-empt that, OverlayHost captures the router (SetRouter must be
// wired) for as long as at least one popup is open: every pointer event
// routes directly to OverlayHost.OnPointer while a popup is showing,
// completely bypassing hit-testing into content. OnPointer re-hit-tests and
// forwards into the topmost popup's own subtree (input.HitPath + input.Bubble)
// when the event falls inside it, so popup-internal controls (ComboBox item
// rows, etc.) still receive Press/Release/Move/Wheel normally. Content (and
// any popup beneath the topmost one) is the part that stays inert while a
// popup is open — only the topmost popup's interior and the
// dismiss-on-outside-Press decision are live. Documented v0 simplification,
// not an oversight.
//
// Hover (Enter/Leave) is the one exception to "forwarded like any other
// event": input.Router's own hover-diffing (the mechanism that normally
// turns hit-test-path changes between successive Move events into Enter/
// Leave pairs) lives entirely in Router.PointerMove's UNCAPTURED branch, so
// it never runs at all while OverlayHost holds the capture — a captured Move
// only ever reaches OverlayHost.OnPointer as a bare Move, with no Enter/Leave
// synthesized anywhere. Popup-internal controls that drive their hover state
// off Enter/Leave (e.g. ComboBox's item rows) would therefore never actually
// hover from real mouse movement, only from Press/Release — so OnPointer
// replicates Router.updateHover's diff-and-notify algorithm itself, scoped to
// the topmost popup's own subtree, via popupHover/diffPopupHover.
type OverlayHost struct {
	core.Element

	content core.Widget
	popups  []popupEntry
	router  *input.Router

	// popupHover is the last hit-test path (root→leaf, rooted at the topmost
	// popup) computed by a forwarded Move, used to diff against the next one
	// and synthesize Enter/Leave for popup-internal widgets — see
	// diffPopupHover and the type doc comment's "Hover" paragraph. Reset to
	// nil (silently — no Leave fired, matching input.Router.Detach's
	// no-notification-on-teardown convention) whenever the topmost popup
	// changes: a new popup opens (ShowPopup) or any popup closes
	// (ClosePopup), since either invalidates whatever it was tracking.
	popupHover []core.Widget

	// popupHoverGen counts every write to popupHover (both the resets in
	// ShowPopup/ClosePopup and diffPopupHover's own final assignment), used
	// by diffPopupHover to detect REENTRANT mutation: delivering a
	// synthesized Enter/Leave can itself call ShowPopup/ClosePopup
	// synchronously (e.g. a ToolTipArea with no timers.Queue wired shows or
	// hides its tip immediately, from inside OnPointer), which would
	// otherwise be clobbered by diffPopupHover's own stale final write once
	// the delivery loop returns to it — see diffPopupHover's doc comment.
	popupHoverGen int
}

// NewOverlayHost returns an empty OverlayHost with no content and no popups.
func NewOverlayHost() *OverlayHost {
	return &OverlayHost{}
}

// SetContent sets (replacing any existing) the single always-visible content
// widget, re-parenting it to this host and invalidating measure. Any
// previously set content is detached (its parent cleared) first, matching
// the SetChild convention used elsewhere in controls.
func (h *OverlayHost) SetContent(w core.Widget) *OverlayHost {
	if h.content != nil {
		core.SetParent(h.content, nil)
	}
	h.content = w
	core.SetParent(w, h)
	h.InvalidateMeasure()
	return h
}

// SetRouter wires the input.Router that dispatches to this tree. It is used
// for two things: capturing pointer input for the duration any popup is
// open (see OnPointer's doc comment) and clearing stale focus/capture/hover
// into a popup's subtree via router.Detach when it closes (see ClosePopup).
// SetRouter is normally called once, e.g. by the host application right
// after constructing its input.Router. A nil router (the zero value) is
// valid and simply disables both behaviors — ClosePopup and CloseTopPopup
// still work, they just skip the Detach call, and light-dismiss falls back
// to whatever the ordinary (uncaptured) bubble happens to deliver.
func (h *OverlayHost) SetRouter(r *input.Router) {
	h.router = r
}

// ShowPopup opens popup, placing it near anchor (a screen-space rect,
// typically the opener's own bounds): the preferred position is directly
// below anchor, left-aligned with it; the popup flips to sit above anchor
// instead when it would overflow the host's bottom edge, and is always
// clamped horizontally so it stays within the host's bounds (see
// placePopup for the exact placement math). popup becomes the new topmost
// popup (last in the stack), so it hit-tests and renders above every
// existing popup and the content.
//
// onDismiss (may be nil) fires exactly once, when popup is later removed via
// ClosePopup/CloseTopPopup — whether that happens via light-dismiss or an
// explicit close call made by whoever opened the popup.
//
// While at least one popup is open, ShowPopup captures the wired router (see
// SetRouter) on this host, so pointer input routes to OnPointer instead of
// hit-testing into content — re-engaging the capture is a no-op if it's
// already held by this host.
func (h *OverlayHost) ShowPopup(popup core.Widget, anchor render.Rect, onDismiss func()) {
	h.popups = append(h.popups, popupEntry{w: popup, anchor: anchor, onDismiss: onDismiss})
	core.SetParent(popup, h)
	// popup is now topmost: whatever popupHover was tracking (if anything)
	// belonged to the previous topmost (or nothing, if this is the first
	// popup) and is stale either way — reset silently, per the field's doc
	// comment. Bumping popupHoverGen here is what lets a diffPopupHover call
	// further up the stack (if ShowPopup was itself called from a
	// synthesized Enter/Leave — see popupHoverGen's doc comment) detect that
	// its own pending final write is now stale and must not clobber this.
	h.popupHover = nil
	h.popupHoverGen++
	if h.router != nil {
		h.router.Capture(h)
	}
	h.InvalidateMeasure()
}

// ClosePopup removes popup from the stack, wherever it sits in it (not just
// the top). Idempotent: if popup isn't currently in the stack (already
// closed, or never opened), it's a no-op — in particular onDismiss does NOT
// fire again on a repeat call.
//
// On an actual close: popup is detached (its parent cleared), the wired
// router (if any, see SetRouter) has Detach(popup) called on it — clearing
// any stale focus/capture/hover the router held into popup's subtree, since
// popup is about to become unreachable — onDismiss fires (if non-nil), and
// measure is invalidated. If this empties the stack entirely, and this host
// currently holds the router's pointer capture (see ShowPopup), the capture
// is released so ordinary hit-testing into content resumes.
func (h *OverlayHost) ClosePopup(popup core.Widget) {
	idx := -1
	for i, p := range h.popups {
		if p.w == popup {
			idx = i
			break
		}
	}
	if idx == -1 {
		return
	}

	entry := h.popups[idx]
	h.popups = append(h.popups[:idx], h.popups[idx+1:]...)
	// The closed popup (or, if idx wasn't the top, the new topmost) may
	// differ from whatever popupHover was tracking — reset silently, per the
	// field's doc comment. Bumping popupHoverGen (see its doc comment) is
	// what lets a diffPopupHover call further up the stack detect this and
	// skip its own now-stale final write.
	h.popupHover = nil
	h.popupHoverGen++

	core.SetParent(popup, nil)
	if h.router != nil {
		h.router.Detach(popup)
	}
	if entry.onDismiss != nil {
		entry.onDismiss()
	}
	h.InvalidateMeasure()

	if len(h.popups) == 0 && h.router != nil && h.router.Captured() == core.Widget(h) {
		h.router.Release()
	}
}

// CloseTopPopup closes only the topmost (last-shown) popup, per ClosePopup's
// contract. A no-op when no popup is open.
func (h *OverlayHost) CloseTopPopup() {
	if len(h.popups) == 0 {
		return
	}
	h.ClosePopup(h.popups[len(h.popups)-1].w)
}

// PopupCount returns how many popups are currently open.
func (h *OverlayHost) PopupCount() int {
	return len(h.popups)
}

// OverlayHostFor walks core.ParentOf from w, looking for the nearest
// ancestor (inclusive of w itself) that is an *OverlayHost. Returns nil if
// w has no OverlayHost ancestor — e.g. it isn't attached to a tree rooted at
// one.
func OverlayHostFor(w core.Widget) *OverlayHost {
	for w != nil {
		if h, ok := w.(*OverlayHost); ok {
			return h
		}
		w = core.ParentOf(w)
	}
	return nil
}

// MeasureContent measures content and every open popup, each with the full
// available space (popups size to their own content, same as content
// itself; neither is narrowed on the host's account). The host's own
// desired size is content's desired size — popups are overlay-positioned
// and never affect it.
func (h *OverlayHost) MeasureContent(available render.Size) render.Size {
	var desired render.Size
	if h.content != nil {
		core.MeasureWidget(h.content, available)
		desired = core.DesiredSizeOf(h.content)
	}
	for _, p := range h.popups {
		core.MeasureWidget(p.w, available)
	}
	return desired
}

// ArrangeContent arranges content to fill the host's full bounds, then
// places each popup at its computed position (see placePopup) sized to
// exactly its own desired size — never stretched or otherwise touched by
// the popup widget's own alignment, since the rect handed to ArrangeWidget
// already equals its desired size on both axes.
func (h *OverlayHost) ArrangeContent(bounds render.Rect) {
	if h.content != nil {
		core.ArrangeWidget(h.content, bounds)
	}
	for _, p := range h.popups {
		desired := core.DesiredSizeOf(p.w)
		core.ArrangeWidget(p.w, placePopup(p.anchor, desired, bounds))
	}
}

// placePopup computes a popup's absolute placement rect given the anchor it
// was opened against, its own desired size, and the host's bounds:
//
//   - x = anchor.X, clamped into [bounds.X, bounds.Right()-desired.W] (a
//     popup wider than bounds simply pins to bounds.X — clampF handles that).
//   - y = anchor.Bottom() (directly below, the preferred position); flips to
//     anchor.Y-desired.H (directly above) when the preferred position would
//     overflow bounds.Bottom(). After a flip, y is additionally clamped to be
//     >= bounds.Y (a popup taller than the space above the anchor pins to the
//     host's top rather than running off it).
func placePopup(anchor render.Rect, desired render.Size, bounds render.Rect) render.Rect {
	x := clampF(anchor.X, bounds.X, bounds.Right()-desired.W)

	y := anchor.Bottom()
	if y+desired.H > bounds.Bottom() {
		y = anchor.Y - desired.H
		if y < bounds.Y {
			y = bounds.Y
		}
	}

	return render.Rect{X: x, Y: y, W: desired.W, H: desired.H}
}

// clampF clamps v into [lo, hi]. If hi < lo (the content being placed is
// larger than the space available), lo wins outright rather than producing
// an inverted range.
func clampF(v, lo, hi float32) float32 {
	if hi < lo {
		hi = lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Children returns [content, popups...] (content first, popups in stack
// order, topmost last) — see the OverlayHost doc comment for why this order
// matters to both hit-testing and rendering. A host with no content yet set
// simply omits it (no nil entries).
func (h *OverlayHost) Children() []core.Widget {
	out := make([]core.Widget, 0, 1+len(h.popups))
	if h.content != nil {
		out = append(out, h.content)
	}
	for _, p := range h.popups {
		out = append(out, p.w)
	}
	return out
}

// OnKey implements input.KeyHandler: it delegates to content's own OnKey
// (if content implements input.KeyHandler), but ONLY when no widget in the
// tree currently holds keyboard focus (e.Router.Focused() == nil).
//
// OverlayHost is a purely structural root — it draws nothing itself
// (Render is the inherited core.Element no-op) and exists to give popups
// somewhere to hit-test and render above content. But input.Router.dispatchKey
// delivers an unfocused key event to the bare router root ALONE, walking no
// further into the tree (see its doc comment); for an app whose root is an
// OverlayHost, that means a window-level accelerator hosted on content (or
// somewhere in content's own subtree, reachable via content's OnKey) would
// silently never fire until something happened to be focused. Delegating
// here restores that: whatever content would have received had IT been the
// router's root gets delivered exactly as before.
//
// The focused-widget case needs no such forwarding — dispatchKey already
// bubbles a focused key event up the ancestor chain from the focused widget
// to the root, and content sits on that chain (between the focused widget
// and this host, being its parent) whenever the focused widget is under
// content at all — so content already receives the event once through the
// ordinary bubble. Delegating unconditionally here (without the
// Focused()==nil guard) would deliver that same event to content a SECOND
// time. The guard is what keeps this a pure "unfocused-only" fallback rather
// than a duplicate delivery path.
//
// Popups are deliberately excluded: this never forwards into h.popups. A
// popup that needs Esc-to-close (ComboBox) or similar already gets it via
// the ordinary focused path, since the field that opened the popup stays
// focused for the popup's entire lifetime (see ComboBox's type doc comment)
// — there is no unfocused-popup-key scenario that needs a separate route.
func (h *OverlayHost) OnKey(e *input.KeyEvent) {
	if e.Router != nil && e.Router.Focused() != nil {
		return
	}
	if kh, ok := h.content.(input.KeyHandler); ok {
		kh.OnKey(e)
	}
}

// OnPointer implements input.PointerHandler. See the OverlayHost doc comment
// for why the router is captured while any popup is open, which is what
// makes this the exclusive receiver of every pointer event during that
// window: with no root set (or no popups open), this is never called via
// capture at all — only ever via ordinary bubbling, if it happens to be on
// the hit path, in which case there's nothing to do (len(h.popups) == 0
// short-circuits below).
//
// When e.Pos falls inside the topmost popup's bounds, the event is forwarded
// into that popup's own subtree — input.HitPath(topmost, e.Pos) finds the
// widget(s) under the point, and input.Bubble replays the same leaf→root
// delivery Router's own dispatch would have done had it not been bypassed by
// the capture — so popup-internal controls (buttons, item rows, ...) receive
// Press/Release/Move/Wheel much as if the router had hit-tested normally. A
// Move specifically is ALSO run through diffPopupHover first (see the type
// doc comment's "Hover" paragraph), so popup-internal widgets that rely on
// Enter/Leave (e.g. ComboBox's item rows) receive proper hover transitions
// from real mouse movement, not just from this forwarded Move. Nothing here
// is dismissed in this branch, regardless of whether the forwarded delivery
// itself ends up Handled.
//
// One real difference from ordinary (uncaptured) dispatch: this host's own
// Capture(h) is still the one on top of the router's capture stack for the
// duration of that forwarded call. If a forwarded widget itself captures —
// e.g. a ScrollViewer thumb starting a drag inside a popup — that Capture
// NESTS on top of the host's (see input.Router.Capture), and the widget's
// own matching Release pops back to the host's capture rather than clearing
// it: light dismiss stays armed for the rest of the popup's lifetime even
// after an inner drag completes.
//
// When e.Pos falls outside the topmost popup's bounds, a Move first clears
// popupHover (diffPopupHover(nil) delivers Leave to everything it was
// tracking — the pointer just left the popup's interior entirely, whether or
// not the popup itself stays open), then only Press matters: it closes that
// popup (via CloseTopPopup, so its onDismiss fires) and marks e.Handled,
// swallowing it. Every other outside action (Move/Release/Wheel) is
// swallowed silently with no forwarding beyond that hover clear — content
// (and any popup beneath the topmost one) still gets no hover/move feedback
// while a popup is open; only the topmost popup's own interior and the
// dismiss-on-Press decision are live. This is a documented v0
// simplification, not an oversight.
func (h *OverlayHost) OnPointer(e *input.PointerEvent) {
	if len(h.popups) == 0 {
		return
	}
	top := h.popups[len(h.popups)-1].w

	if core.BoundsOf(top).Contains(e.Pos) {
		path := input.HitPath(top, e.Pos)
		if e.Action == input.Move {
			h.diffPopupHover(path)
		}
		input.Bubble(path, e)
		return
	}

	if e.Action == input.Move {
		h.diffPopupHover(nil)
	}

	if e.Action == input.Press {
		h.CloseTopPopup()
		e.Handled = true
	}
}

// diffPopupHover diffs h.popupHover (the hit-test path, rooted at the
// topmost popup, as of the previous forwarded Move) against newPath,
// delivering a direct (non-bubbling; e.Handled is not consulted, matching
// input.Router.updateHover) Leave to every widget that dropped off the path
// and an Enter to every widget newly on it, then stores newPath as the new
// popupHover. Passing nil (as OnPointer does once the pointer has moved
// outside the popup entirely) delivers Leave to every currently-tracked
// widget and Enter to none — this replicates input.Router.updateHover's own
// diff-and-notify algorithm (that method is private to the input package,
// and in any case diffs against the ROOT tree's hover state, which
// OverlayHost's modal capture bypasses entirely while any popup is open —
// see the type doc comment's "Hover" paragraph for why Router's own
// mechanism never runs during that window).
//
// REENTRANCY: delivering a synthesized Enter/Leave below can itself call
// ShowPopup/ClosePopup synchronously — e.g. a ToolTipArea with no
// timers.Queue wired shows (Enter) or hides (Leave) its tip immediately,
// right there inside OnPointer — and either one resets h.popupHover (and
// bumps popupHoverGen) out from under this call, for whatever is now the
// (possibly entirely different) topmost popup. So: every read of "the
// previous path" below uses old, a snapshot taken at entry, NEVER h.popupHover
// directly (which may have already been overwritten by the time a later
// iteration of either loop runs) — and the final write only actually commits
// newPath if h.popupHoverGen still equals the snapshot taken at entry, i.e.
// nothing nested changed it in the meantime. Skipping that write when it
// doesn't hold is load-bearing: committing newPath (rooted at THIS call's
// popup) over whatever a nested ShowPopup/ClosePopup just installed (rooted
// at a different popup, or nil) would desync popupHover from reality — the
// concrete failure this guards is a phantom Leave closing a tooltip that a
// nested ShowPopup just opened, on the very next Move.
func (h *OverlayHost) diffPopupHover(newPath []core.Widget) {
	old := h.popupHover
	startGen := h.popupHoverGen

	for _, w := range old {
		if !containsPopupWidget(newPath, w) {
			deliverPopupHoverEvent(w, input.Leave, h.router)
		}
	}
	for _, w := range newPath {
		if !containsPopupWidget(old, w) {
			deliverPopupHoverEvent(w, input.Enter, h.router)
		}
	}

	if h.popupHoverGen == startGen {
		h.popupHover = newPath
		h.popupHoverGen++
	}
}

// deliverPopupHoverEvent delivers a direct (non-bubbling) Enter or Leave to
// w, if w implements input.PointerHandler — the same direct-delivery shape
// input.Router's own hover diffing uses (deliverDirect, private to the input
// package). router (the OverlayHost's own, possibly nil) is attached to the
// synthesized event for parity with a normally-dispatched Enter/Leave, even
// though neither action reads it in practice (ClickBehavior.HandlePointer,
// for one, only touches e.Router on Press/Release).
func deliverPopupHoverEvent(w core.Widget, action input.Action, router *input.Router) {
	if ph, ok := w.(input.PointerHandler); ok {
		ph.OnPointer(&input.PointerEvent{Action: action, Target: w, Router: router})
	}
}

// containsPopupWidget reports whether w appears in path, compared by
// identity (==) — the same comparison input.Router's own hover diffing uses
// (containsIdentity, private to the input package).
func containsPopupWidget(path []core.Widget, w core.Widget) bool {
	for _, x := range path {
		if x == w {
			return true
		}
	}
	return false
}
