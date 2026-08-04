package controls

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/timers"
)

// popupEntry is one entry in an OverlayHost's popup stack: the popup widget
// itself, the screen-space anchor rect it was opened against (re-used every
// arrange pass to recompute its position), the callback (may be nil) to fire
// when it's dismissed, whether it's MODAL (opened via ShowPopup) or not
// (opened via ShowPopupNonModal) — see the type doc comment's "Modal vs
// non-modal popups" paragraph — and whether it TRAPS KEYBOARD FOCUS for as
// long as it's open (see the "Focus trapping" paragraph). Every OTHER aspect
// of a popup (rendering, hit-testing z-order, Detach-on-close) is identical
// regardless of either flag; only capture engagement and light-dismiss
// depend on modal, and only the router's focus-scope stack depends on
// focusScope.
type popupEntry struct {
	w         core.Widget
	anchor    render.Rect
	onDismiss func()
	modal     bool

	// owner is the widget this popup belongs to (e.g. the ComboBox that opened
	// its dropdown), or nil for owner-less popups. When non-nil, ArrangeContent
	// auto-closes the popup once the owner leaves the live tree — hidden by a
	// tab switch, or detached by SetContent (see ownerLive). Without this an
	// orphaned modal popup would keep the host capture engaged and the owner's
	// own open-state stuck (e.g. ComboBox.open never resets, so the box is dead
	// for life). The public ShowPopup* entry points pass nil (never swept);
	// same-package controls that ARE owned (ComboBox) call showPopup directly.
	owner core.Widget

	// focusScope records that showPopup pushed an input.Router focus scope
	// for this popup, so ClosePopup knows to pop it again — see
	// showPopupTrapFocus.
	focusScope bool
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
// Modal vs non-modal popups: ShowPopup opens a MODAL popup (ComboBox's
// dropdown); ShowPopupNonModal opens one that is NOT (ToolTipArea's tip).
// The two are otherwise identical — same stack, same z-order for hit-testing
// and rendering, same Detach-on-close via ClosePopup — but only a modal
// popup engages the capture-based light-dismiss machinery described below;
// a non-modal popup relies entirely on the router's ORDINARY (uncaptured)
// hit-testing and hover-diffing to place it above content (it hit-tests and
// renders topmost purely because of its position in Children(), same as
// any popup) and to close it (its owner, e.g. ToolTipArea, closes it itself
// on Leave — see ToolTipArea's own doc comment). If a modal popup is ALSO
// open at the same time, a non-modal one behaves exactly as it would if it
// were modal for the purpose of EVENT delivery: the modal capture governs
// all pointer routing regardless of which popup happens to be topmost, per
// the paragraph below.
//
// Focus trapping is a SEPARATE opt-in from modality, and a narrower one:
// ShowModalPopup (which is also how ShowDialog opens its own surface)
// pushes an input.Router focus scope rooted at the popup, so Tab/Shift+Tab
// cycle only the popup's own focusable widgets and keys never dispatch into
// the content behind it; ClosePopup pops that scope again. Being modal is
// NOT enough to trap focus, because the two modal popup families this
// package already ships deliberately keep focus on their OPENER, which sits
// outside the popup: a ComboBox's dropdown leaves the ComboBox field focused
// for the popup's whole lifetime (that is how its Esc/arrow keys keep
// working — see ComboBox's own type doc comment), and a MenuBar likewise
// stays focused while its menu popup is open. Trapping focus inside those
// popups would strand their keyboard handling in a subtree that has no
// handler for it. A dialog is the opposite case: nothing outside it should
// be keyboard-reachable while it's up, and its Escape-to-close lives on the
// popup itself.
//
// Light dismiss (see OnPointer) needs a way to stop a stray press from
// reaching content underneath an open MODAL popup, and content's own
// widgets are always deeper in the bubble path than OverlayHost (the
// root), so they'd receive a Press before OverlayHost — the standard
// bubble order — ever ran. To pre-empt that, OverlayHost captures the
// router (SetRouter must be wired) for as long as at least one MODAL popup
// is open: every pointer event routes directly to OverlayHost.OnPointer
// while a modal popup is showing, completely bypassing hit-testing into
// content. OnPointer re-hit-tests the WHOLE stack, top-down, for the popup
// actually containing the event (see popupAt) — not just the last one — and
// forwards into THAT popup's own subtree (input.HitPath + input.Bubble),
// closing whatever popups sit above it first (see OnPointer's own doc
// comment for the full chain-aware (a)/(b)/(c) semantics), so popup-internal
// controls (ComboBox item rows, a menu's own sibling rows or nested
// submenu, etc.) still receive Press/Release/Move/Wheel normally regardless
// of which level of an open chain they belong to. Content is the part that
// stays inert while a modal popup is open — only the currently-containing
// popup's interior and the dismiss/auto-close decisions above it are live.
// A non-modal popup with no modal popup open engages none of this: it is
// never light-dismissed (nothing captures on its account), and the
// ordinary uncaptured dispatch path below (hit-testing, hover-diffing,
// bubbling) reaches it and everything else exactly as if it weren't a
// popup at all.
//
// Hover (Enter/Leave) is the one exception to "forwarded like any other
// event" while a MODAL popup holds the capture: input.Router's own
// hover-diffing (the mechanism that normally turns hit-test-path changes
// between successive Move events into Enter/Leave pairs) lives entirely in
// Router.PointerMove's UNCAPTURED branch, so it never runs at all while
// OverlayHost holds the capture — a captured Move only ever reaches
// OverlayHost.OnPointer as a bare Move, with no Enter/Leave synthesized
// anywhere. Popup-internal controls that drive their hover state off
// Enter/Leave (e.g. ComboBox's item rows, a menuSubRow's hover-opens-submenu)
// would therefore never actually hover from real mouse movement, only from
// Press/Release — so OnPointer replicates Router.updateHover's
// diff-and-notify algorithm itself, scoped to whichever popup's own subtree
// currently CONTAINS the pointer (not necessarily the topmost one — see
// popupAt), via popupHover/diffPopupHover. Moving the pointer into a
// DIFFERENT (lower) popup than the one previously hovered auto-closes
// whatever popup(s) sat above it, exactly like a Press would (see
// OnPointer's own doc comment). A non-modal popup with no modal capture
// engaged needs none of this replication: Router's own hover-diffing runs
// normally (it was never bypassed), so Enter/Leave reach it exactly as they
// would any other widget in the tree — this is what restores ToolTipArea's
// documented close-on-Leave path (see its own type doc comment).
type OverlayHost struct {
	core.Element

	content core.Widget
	popups  []popupEntry
	router  *input.Router

	// toasts is the stack of currently open transient notifications — see
	// ShowToast (controls/toast.go). Kept entirely separate from popups: a
	// toast never engages the modal capture/light-dismiss machinery above
	// (it isn't opened via showPopup at all), and is stacked/positioned by
	// its own corner-anchored layout (arrangeToasts) rather than placePopup's
	// anchor-relative one. It IS included in Children() (after content and
	// every popup, so it hit-tests and renders topmost of all) so a toast's
	// own OnPointer (click-to-dismiss) and Render are reached normally.
	toasts []*toastEntry

	// timerQueue, wired via SetTimers, drives ShowToast's auto-dismiss
	// timers. nil (the zero value, or after SetTimers(nil)) disables
	// auto-dismiss for any toast shown from then on — see ShowToast's doc
	// comment for the degradation. Mirrors ToolTipArea/TextBox/Button's own
	// SetTimers convention, except a superseded queue's effect on toasts
	// ALREADY showing is left alone (there is no single pending timer to
	// stop up front, unlike those widgets — a host may have many open
	// toasts, each with its own independent timer).
	timerQueue *timers.Queue

	// popupHover is the last hit-test path (root→leaf, rooted at whichever
	// popup OnPointer's popupAt found as the CONTAINING popup for the most
	// recent forwarded Move — not necessarily the topmost one; see
	// OnPointer's chain-aware doc comment) computed by that Move, used to
	// diff against the next one and synthesize Enter/Leave for
	// popup-internal widgets — see diffPopupHover and the type doc comment's
	// "Hover" paragraph. Reset to nil (silently — no Leave fired, matching
	// input.Router.Detach's no-notification-on-teardown convention) whenever
	// any popup opens (ShowPopup/ShowPopupNonModal) or closes (ClosePopup):
	// either invalidates whatever it was tracking, since OnPointer always
	// closes every popup ABOVE the containing one before recomputing this —
	// so by the time diffPopupHover runs again, a change of containing popup
	// has already reset popupHover to nil via that closing, with no separate
	// "which popup does this belong to" field needed to detect the switch.
	popupHover []core.Widget

	// modalOpenObs holds observers notified the instant a MODAL popup opens
	// (see showPopup/notifyModalOpen). Its only consumer is ToolTipArea: a
	// content tooltip relies on the router's ordinary (uncaptured) hover
	// diffing for its Leave, but a modal popup captures the router and silences
	// that diffing (see this type's "Hover" paragraph), so a pending or showing
	// content tip would otherwise strand beside — or, if its dwell timer is
	// still pending, pop OVER — the modal at stale bounds. Each observer cancels
	// one such tooltip. Keyed by id so a ToolTipArea can unregister when it
	// disarms (onModalOpen's returned cancel); non-modal popups (a tooltip's own
	// tip) never fire it, since they engage no capture.
	modalOpenObs   map[int]func()
	modalOpenObsID int

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
//
// w may be nil, which simply CLEARS the content: MeasureContent,
// ArrangeContent and Children all already skip a host with no content (a
// popup-only host is a perfectly ordinary state — see Children's own doc
// comment), and the nil is kept away from core.SetParent, which dereferences
// its child argument. Same nil handling as ScrollViewer.SetChild.
func (h *OverlayHost) SetContent(w core.Widget) *OverlayHost {
	if h.content != nil {
		core.SetParent(h.content, nil)
	}
	h.content = w
	if w != nil {
		core.SetParent(w, h)
	}
	h.InvalidateMeasure()
	return h
}

// SetRouter wires the input.Router that dispatches to this tree. It is used
// for two things: capturing pointer input for the duration any MODAL popup is
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

// SetTimers wires q as the driver for ShowToast's auto-dismiss timers (see
// its doc comment). Passing nil detaches any previously wired queue; toasts
// shown from then on simply have no auto-dismiss (see ShowToast). SetTimers
// is normally called once, e.g. by the host application right after
// SetRouter — same as every other control's SetTimers method.
func (h *OverlayHost) SetTimers(q *timers.Queue) *OverlayHost {
	h.timerQueue = q
	return h
}

// ShowPopup opens popup as a MODAL popup, placing it near anchor (a
// screen-space rect, typically the opener's own bounds): the preferred
// position is directly below anchor, left-aligned with it; the popup flips
// to sit above anchor instead when it would overflow the host's bottom
// edge, and is always clamped horizontally so it stays within the host's
// bounds (see placePopup for the exact placement math). popup becomes the
// new topmost popup (last in the stack), so it hit-tests and renders above
// every existing popup and the content.
//
// onDismiss (may be nil) fires exactly once, when popup is later removed via
// ClosePopup/CloseTopPopup — whether that happens via light-dismiss or an
// explicit close call made by whoever opened the popup.
//
// While at least one MODAL popup is open, ShowPopup captures the wired
// router (see SetRouter) on this host, so pointer input routes to
// OnPointer instead of hit-testing into content — re-engaging the capture
// is a no-op if it's already held by this host (see input.Router.Capture's
// doc comment on idempotence, and its caveat about a capture stack shaped
// h→w→h). Use ShowPopupNonModal for a popup that should NOT engage any of
// this (e.g. ToolTipArea's tip) — see the type doc comment's "Modal vs
// non-modal popups" paragraph.
// A popup shown this way does NOT trap keyboard focus — see the type doc
// comment's "Focus trapping" paragraph, and ShowModalPopup for the
// dialog-shaped popup that does.
func (h *OverlayHost) ShowPopup(popup core.Widget, anchor render.Rect, onDismiss func()) {
	h.showPopup(popup, anchor, onDismiss, true, false, nil)
}

// ShowPopupNonModal opens popup exactly like ShowPopup — same placement,
// same stacking, same onDismiss-once-on-close contract, same
// Detach-on-close — except it never engages the router: no capture is
// taken on this host on its account, so it is never light-dismissed (it
// hit-tests/renders topmost purely from its position in the popup stack,
// and closing it is entirely up to its owner, e.g. ToolTipArea closing its
// own tip on Leave). If a MODAL popup is ALSO open (from a prior or later
// ShowPopup call), that modal capture still governs event routing for
// EVERY popup regardless of which is topmost — see the type doc comment's
// "Modal vs non-modal popups" paragraph.
func (h *OverlayHost) ShowPopupNonModal(popup core.Widget, anchor render.Rect, onDismiss func()) {
	h.showPopup(popup, anchor, onDismiss, false, false, nil)
}

// ShowModalPopup opens popup as a MODAL popup that ALSO TRAPS KEYBOARD FOCUS
// for as long as it is open. Placement, stacking, the light-dismiss capture
// and the onDismiss-fires-once-on-close contract are all exactly ShowPopup's
// (see its doc comment); the difference is entirely the focus trap: a focus
// scope rooted at popup is pushed on the wired router (see SetRouter) when it
// opens and popped again when it closes, so while it is up, Tab/Shift+Tab
// cycle only popup's own focusable widgets, widgets behind it are not
// keyboard-reachable (no key event dispatches into the content, even if
// something out there is still focused), and Escape reaches popup itself —
// pushing the scope also homes focus onto popup, which is what makes it the
// key target even when it holds no focusable widget at all. Closing it
// returns focus to whatever held it when the popup opened (typically the
// control that opened it), per the focus-scope contract — see
// input.Router.PushFocusScope/PopFocusScope for the exact rules, including
// what happens when that opener is gone by the time it closes.
//
// Which of the three to use:
//
//   - ShowPopup — modal, does NOT trap focus. For a popup whose keyboard
//     handling lives on the OPENER outside it (a ComboBox dropdown, a MenuBar
//     menu) — see the type doc comment's "Focus trapping" paragraph for why
//     trapping those would strand their Escape/arrow keys.
//   - ShowPopupNonModal — neither modal nor trapping (a tooltip).
//   - ShowModalPopup — both, for a dialog-shaped surface: nothing behind it
//     should be reachable until it closes. ShowDialog is the ready-made one
//     built on this; call ShowModalPopup directly to trap focus in a
//     scrim-backed modal surface of your own.
func (h *OverlayHost) ShowModalPopup(popup core.Widget, anchor render.Rect, onDismiss func()) {
	h.showPopupTrapFocus(popup, anchor, onDismiss)
}

// showPopupTrapFocus opens popup as a MODAL popup that ALSO traps keyboard
// focus for as long as it is open: the wired router (see SetRouter) gets a
// focus scope rooted at popup pushed on it here and popped again by
// ClosePopup, so while it's up, Tab/Shift+Tab cycle only popup's own
// focusable widgets and no key event dispatches into the content behind it
// (see input.Router.PushFocusScope for the full contract, including its
// focusing popup itself on push). Everything else is exactly ShowPopup.
//
// Shared by the two surfaces that want a trap: ShowModalPopup (the public
// spelling of it) and ShowDialog, whose whole point is that nothing behind it
// is reachable until it closes. The other modal popups in this package must
// NOT trap — see the type doc comment's "Focus trapping" paragraph for why
// keeping focus on the opener is load-bearing for ComboBox and MenuBar.
func (h *OverlayHost) showPopupTrapFocus(popup core.Widget, anchor render.Rect, onDismiss func()) {
	h.showPopup(popup, anchor, onDismiss, true, true, nil)
}

// showPopup is ShowPopup/ShowPopupNonModal/showPopupTrapFocus's shared
// implementation: append the entry (recording modal so ClosePopup's
// capture-release decision and hasModalPopup can tell modal and non-modal
// popups apart, and trapFocus so ClosePopup knows whether it owes a
// PopFocusScope), re-parent, reset the stale popupHover (see the field's doc
// comment and popupHoverGen's), invalidate measure, and — modal only —
// capture the wired router, plus push its focus scope when trapping.
func (h *OverlayHost) showPopup(popup core.Widget, anchor render.Rect, onDismiss func(), modal, trapFocus bool, owner core.Widget) {
	h.popups = append(h.popups, popupEntry{w: popup, anchor: anchor, onDismiss: onDismiss, modal: modal, owner: owner, focusScope: trapFocus})
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
	if modal && h.router != nil {
		h.router.Capture(h)
	}
	// Pushed AFTER the capture and BEFORE notifyModalOpen, so an observer
	// that reacts by closing its own (non-modal, non-trapping) popup sees the
	// same fully-engaged state a later event would. PushFocusScope also homes
	// focus onto popup itself when focus was outside it — which is what makes
	// the first Tab enter the popup and Escape reach it even when it holds no
	// focusable widget at all (see input.Router.PushFocusScope).
	if trapFocus && h.router != nil {
		h.router.PushFocusScope(popup)
	}
	h.InvalidateMeasure()
	// A modal popup has just captured the router, silencing the hover diffing
	// content tooltips depend on — tell them to drop any pending/showing tip
	// (see modalOpenObs). Done last, after the popup is fully on the stack and
	// the capture is engaged, so an observer that closes its own tip popup
	// (hideTip -> ClosePopup) sees consistent state.
	if modal {
		h.notifyModalOpen()
	}
}

// onModalOpen registers f to be invoked whenever a MODAL popup opens on this
// host, returning a cancel that unregisters it (idempotent — a second call is a
// no-op). See the modalOpenObs field's doc comment for why ToolTipArea needs
// it.
func (h *OverlayHost) onModalOpen(f func()) (cancel func()) {
	if h.modalOpenObs == nil {
		h.modalOpenObs = make(map[int]func())
	}
	id := h.modalOpenObsID
	h.modalOpenObsID++
	h.modalOpenObs[id] = f
	return func() { delete(h.modalOpenObs, id) }
}

// notifyModalOpen invokes every modal-open observer over a snapshot, so an
// observer that unregisters itself (disarm) or mutates the popup stack
// (ToolTipArea.hideTip -> ClosePopup) during the call cannot corrupt the walk.
func (h *OverlayHost) notifyModalOpen() {
	if len(h.modalOpenObs) == 0 {
		return
	}
	fns := make([]func(), 0, len(h.modalOpenObs))
	for _, f := range h.modalOpenObs {
		fns = append(fns, f)
	}
	for _, f := range fns {
		f()
	}
}

// hasModalPopup reports whether any popup currently on the stack is modal
// (opened via ShowPopup, as opposed to ShowPopupNonModal) — used by
// ClosePopup to decide whether releasing this host's router capture is due
// yet: it must stay engaged as long as even one modal popup remains, not
// merely until the stack is entirely empty (a non-modal popup, e.g. a
// tooltip, may still be open and must not have the capture linger on its
// account).
func (h *OverlayHost) hasModalPopup() bool {
	for _, p := range h.popups {
		if p.modal {
			return true
		}
	}
	return false
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
// measure is invalidated. If NO popup on the stack is modal afterward (see
// hasModalPopup — a stack that's merely empty qualifies trivially, but so
// does one that still holds non-modal popups, e.g. a tooltip left open),
// and this host currently holds the router's pointer capture (see
// ShowPopup), the capture is released so ordinary hit-testing into content
// resumes. A popup that pushed a focus scope when it opened (see
// showPopupTrapFocus) has it popped here, restoring whatever scope — or
// unrestricted traversal — was in effect beneath it.
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

	// A trapping popup's focus scope ends with the popup. Popped BEFORE the
	// Detach below so that, when an OUTER scope is restored (a nested dialog
	// closing), PopFocusScope's own re-homing runs while the closing popup
	// still holds focus: focus lands on the surface beneath, and Detach then
	// finds nothing of its own left to clear. With no outer scope the pop
	// changes no focus at all and Detach clears it exactly as it always did.
	if entry.focusScope && h.router != nil {
		h.router.PopFocusScope()
	}

	core.SetParent(popup, nil)

	// focusClearedByClose records whether detaching this popup strips keyboard
	// focus from the whole tree — true only when the focused widget lived
	// inside popup's subtree, which is exactly when Detach clears it via
	// Focus(nil) (e.g. the dialog scrim ShowDialog focuses). Captured BEFORE
	// Detach, compared AFTER, so a popup whose owner keeps focus elsewhere (a
	// ComboBox dropdown, a menu — their opener field stays focused, outside the
	// closing subtree) leaves this false and is left untouched below.
	var focusClearedByClose bool
	if h.router != nil {
		hadFocus := h.router.Focused() != nil
		h.router.Detach(popup)
		focusClearedByClose = hadFocus && h.router.Focused() == nil
	}
	if entry.onDismiss != nil {
		entry.onDismiss()
	}
	h.InvalidateMeasure()

	if !h.hasModalPopup() && h.router != nil && h.router.Captured() == core.Widget(h) {
		h.router.Release()
	}

	// If closing this popup left the tree unfocused AND another popup is still
	// open beneath it, route focus to the now-topmost popup so it stays
	// keyboard-reachable. Without this, OverlayHost.OnKey — which delegates to
	// CONTENT whenever Focused() == nil — would send Escape (and every other
	// unfocused key) to content instead of the surviving dialog, stranding a
	// button-less dialog whose only close path is Escape. Mirrors how
	// ShowDialog focuses a single dialog's scrim.
	if focusClearedByClose && h.router != nil && len(h.popups) > 0 {
		h.router.Focus(h.popups[len(h.popups)-1].w)
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

// CloseAllPopups closes every open popup, topmost first, via repeated
// CloseTopPopup calls — so each popup's onDismiss fires exactly once, in
// topmost-to-bottommost order, as if the caller had manually closed each one
// from the top down. A no-op when no popup is open.
//
// Added for the menu family (MenuBar/ContextMenu, controls/menu.go): a
// menu-item click needs to collapse an arbitrarily deep stack of nested
// submenu popups (each opened via its own ShowPopup call — see
// menuPopupCard.openSub) in a single call, not just the one topmost popup
// CloseTopPopup closes. This is a v0 simplification for callers that know
// every popup currently open belongs to the same menu/submenu chain: it
// closes EVERY popup on this host indiscriminately, including any unrelated
// one (e.g. a ComboBox dropdown left open elsewhere) that happens to also be
// open at the same time — acceptable because in normal use nothing else
// holds a popup open while a menu is engaged (a menu popup is modal, so
// opening one already dismisses whatever else was open first — see
// MenuBar.openMenu). Document, don't special-case, this limitation.
func (h *OverlayHost) CloseAllPopups() {
	for len(h.popups) > 0 {
		h.CloseTopPopup()
	}
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

// MeasureContent measures content, every open popup, and every open toast,
// each with the full available space (popups and toasts size to their own
// content, same as content itself; none is narrowed on the host's account).
// The host's own desired size is content's desired size — popups and toasts
// are overlay-positioned and never affect it.
func (h *OverlayHost) MeasureContent(available render.Size) render.Size {
	var desired render.Size
	if h.content != nil {
		core.MeasureWidget(h.content, available)
		desired = core.DesiredSizeOf(h.content)
	}
	for _, p := range h.popups {
		core.MeasureWidget(p.w, available)
	}
	for _, tst := range h.toasts {
		core.MeasureWidget(tst.w, available)
	}
	return desired
}

// ArrangeContent arranges content to fill the host's full bounds, then
// places each popup at its computed position (see placePopup) sized to
// exactly its own desired size — never stretched or otherwise touched by
// the popup widget's own alignment, since the rect handed to ArrangeWidget
// already equals its desired size on both axes — and finally stacks every
// open toast in the host's corner (see arrangeToasts).
// ownerLive reports whether a popup's owner is still in the live tree:
// reachable from this OverlayHost through ancestors that are ALL visible. A
// hidden ancestor (a tab switch that hid the owner's subtree) fails the
// visibility check; a detached owner (SetContent removed its subtree, so its
// parent chain no longer reaches h) walks off to a nil parent without ever
// hitting h. Either way the owner is gone and its popup is orphaned.
func (h *OverlayHost) ownerLive(w core.Widget) bool {
	for n := w; n != nil; n = core.ParentOf(n) {
		if !core.IsVisible(n) {
			return false
		}
		if n == core.Widget(h) {
			return true
		}
	}
	return false
}

func (h *OverlayHost) ArrangeContent(bounds render.Rect) {
	// Close any OWNED popup whose owner has left the live tree (hidden or
	// detached) before laying out. ClosePopup fires onDismiss — resetting the
	// owner's open-state (e.g. ComboBox.open) — and releases the host capture
	// once the last modal popup goes, so an orphaned dropdown can't strand the
	// box or the capture. Snapshot first: ClosePopup mutates h.popups.
	if len(h.popups) > 0 {
		for _, p := range append([]popupEntry(nil), h.popups...) {
			if p.owner != nil && !h.ownerLive(p.owner) {
				h.ClosePopup(p.w)
			}
		}
	}
	if h.content != nil {
		core.ArrangeWidget(h.content, bounds)
	}
	for _, p := range h.popups {
		desired := core.DesiredSizeOf(p.w)
		core.ArrangeWidget(p.w, placePopup(p.anchor, desired, bounds))
	}
	h.arrangeToasts(bounds)
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
// an inverted range. A NaN v also resolves to lo: both of the comparisons
// below are false for NaN, so without this it would pass through unclamped
// and place the popup at a nonsensical position (see clampScrollOffset,
// which shares the same rule).
func clampF(v, lo, hi float32) float32 {
	if hi < lo {
		hi = lo
	}
	if v != v { // NaN
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Children returns [content, popups..., toasts...] (content first, popups in
// stack order, toasts last in show order) — see the OverlayHost doc comment
// for why this order matters to both hit-testing and rendering; toasts sit
// after popups so a toast always hit-tests/renders above even an open modal
// popup. A host with no content yet set simply omits it (no nil entries).
func (h *OverlayHost) Children() []core.Widget {
	out := make([]core.Widget, 0, 1+len(h.popups)+len(h.toasts))
	if h.content != nil {
		out = append(out, h.content)
	}
	for _, p := range h.popups {
		out = append(out, p.w)
	}
	for _, tst := range h.toasts {
		out = append(out, tst.w)
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

// popupAt returns the index (into h.popups) of the topmost popup whose
// bounds contain p, searching the stack TOP-DOWN — the first (highest-index)
// match wins, mirroring input.HitPath's own topmost-child-wins convention:
// a nested submenu, opened later and therefore later in the stack, is found
// before the parent popup beneath it even where the two would overlap.
// Returns -1 if p falls inside no popup on the stack at all.
func (h *OverlayHost) popupAt(p render.Point) int {
	for i := len(h.popups) - 1; i >= 0; i-- {
		if core.BoundsOf(h.popups[i].w).Contains(p) {
			return i
		}
	}
	return -1
}

// OnPointer implements input.PointerHandler. See the OverlayHost doc comment
// for why the router is captured while any MODAL popup is open, which is
// what makes this the exclusive receiver of every pointer event during that
// window: with no root set (or no modal popup open), this is never called
// via capture at all — only ever via ordinary bubbling, if it happens to be
// on the hit path, in which case there's nothing to do (the hasModalPopup
// guard below short-circuits it).
//
// CHAIN-AWARE forwarding/dismissal (Phase 8 Task 1, replacing the earlier
// "only the topmost popup exists" v0 simplification): every popup on the
// stack is a candidate, not just the last one. popupAt searches top-down for
// the first (i.e. topmost) popup whose bounds contain e.Pos:
//
//   - (a) e.Pos is inside NO popup at all (popupAt returns -1): a Move first
//     clears popupHover (diffPopupHover(nil) delivers Leave to everything it
//     was tracking); a Press closes EVERY popup on the stack, topmost first
//     (via CloseAllPopups, so each one's onDismiss fires in that order) and
//     marks e.Handled, swallowing it. Every other outside action
//     (Move/Release/Wheel) is swallowed silently beyond that hover clear.
//
//   - (b)/(c) e.Pos is inside popups[idx]'s bounds for some idx: any popup
//     ABOVE idx — opened later, e.g. a deeper submenu in the same chain that
//     e.Pos no longer falls within — is now stale relative to where the
//     pointer actually is, and is closed first (topmost first, via
//     CloseTopPopup, each firing its own onDismiss) before anything is
//     delivered. This is what makes a lower popup's sibling rows reachable
//     again — hovering a parent menu's row auto-closes whatever submenu
//     chain was open above it — and what collapses an out-of-order chain
//     down to the level a Press actually landed in, in one step, rather than
//     needing one dismiss per stale level.
//
//     Once popups[idx] (call it target) is topmost, the event is forwarded
//     into its own subtree exactly as single-popup forwarding always worked:
//     input.HitPath(target, e.Pos) finds the widget(s) under the point, and
//     input.Bubble replays the same leaf→root delivery Router's own dispatch
//     would have done had it not been bypassed by the capture — so
//     popup-internal controls (buttons, item rows, ...) receive
//     Press/Release/Move/Wheel much as if the router had hit-tested
//     normally. A Move specifically is ALSO run through diffPopupHover first
//     (see the type doc comment's "Hover" paragraph and diffPopupHover's own
//     doc comment for why this needs no extra "which popup is this rooted
//     at" bookkeeping beyond what closing the stale levels above already
//     gives it), so popup-internal widgets that rely on Enter/Leave (e.g.
//     ComboBox's item rows, a menuSubRow's hover-opens-submenu) receive
//     proper hover transitions from real mouse movement. Nothing here is
//     dismissed by virtue of landing inside target, regardless of whether
//     the forwarded delivery itself ends up Handled — only popups ABOVE
//     target are ever closed by this branch.
//
// One real difference from ordinary (uncaptured) dispatch: this host's own
// Capture(h) is still the one on top of the router's capture stack for the
// duration of a forwarded call. If a forwarded widget itself captures — e.g.
// a ScrollViewer thumb starting a drag inside a popup — that Capture NESTS
// on top of the host's (see input.Router.Capture), and the widget's own
// matching Release pops back to the host's capture rather than clearing it:
// light dismiss stays armed for the rest of the popup's lifetime even after
// an inner drag completes.
func (h *OverlayHost) OnPointer(e *input.PointerEvent) {
	// Guard on hasModalPopup, NOT merely len(h.popups) == 0: OverlayHost is
	// the root, so it sits on EVERY hit-test path and is reached via the
	// ordinary (uncaptured) bubble whenever nothing before it in the path
	// sets e.Handled — including whenever only NON-modal popups are open
	// (no capture is ever engaged on their account, so this call, when it
	// happens at all, is exactly that ordinary bubble, not the captured
	// forwarding call the rest of this method is written for). Without this
	// guard, a plain uncaptured press elsewhere in content would still
	// reach here, find e.Pos outside every popup, and wrongly light-dismiss
	// them via the outside-Press branch below — exactly the swallow this
	// whole feature exists to avoid. Once hasModalPopup() is true, this IS
	// always the captured-forwarding call (ShowPopup guarantees the capture
	// is held for as long as any modal popup remains), so the rest of the
	// method proceeds exactly as before.
	if !h.hasModalPopup() {
		return
	}

	idx := h.popupAt(e.Pos)

	if idx == -1 {
		if e.Action == input.Move {
			h.diffPopupHover(nil)
		}
		if e.Action == input.Press {
			h.CloseAllPopups()
			// O5: a light-dismiss press moves focus like an ordinary click —
			// onto whatever it hit in content, or clear — so the just-dismissed
			// opener (e.g. a ComboBox) doesn't keep focus and keep eating keys
			// (Space reopening a just-closed dropdown). FocusFromPath only
			// FOCUSES; it never delivers the press, so the dismiss can't also
			// activate what's underneath.
			if h.router != nil && h.content != nil {
				if !h.router.FocusFromPath(input.HitPath(h.content, e.Pos)) {
					h.router.Focus(nil)
				}
			}
			e.Handled = true
		}
		return
	}

	target := h.popups[idx].w
	// Indiscriminate, like CloseAllPopups: if a closing popup's onDismiss
	// synchronously opened a NEW popup, that new popup would be closed too
	// (it's now above target, same as anything else this loop finds) — no
	// current consumer's onDismiss does this.
	for len(h.popups) > 0 && h.popups[len(h.popups)-1].w != target {
		h.CloseTopPopup()
	}

	path := input.HitPath(target, e.Pos)
	if e.Action == input.Move {
		h.diffPopupHover(path)
	}
	// N1: run press-to-focus for a press forwarded into a modal-captured popup.
	// The captured path bypasses the router's own uncaptured press-to-focus, so
	// without this a focusable widget INSIDE a modal popup (e.g. a TextBox in a
	// ShowModalPopup surface) can be clicked but never focused (Tab works via
	// the focus scope, click didn't). Focus before Bubble, like the router's
	// own uncaptured press path.
	if e.Action == input.Press && h.router != nil {
		h.router.FocusFromPath(path)
	}
	input.Bubble(path, e)
}

// diffPopupHover diffs h.popupHover (the hit-test path, rooted at whichever
// popup was the CONTAINING popup as of the previous forwarded Move — see the
// field's own doc comment) against newPath, delivering a direct
// (non-bubbling; e.Handled is not consulted, matching input.Router.updateHover)
// Leave to every widget that dropped off the path and an Enter to every
// widget newly on it, then stores newPath as the new popupHover. Passing nil
// (as OnPointer does once the pointer has moved outside every popup on the
// stack) delivers Leave to every currently-tracked widget and Enter to none
// — this replicates input.Router.updateHover's own diff-and-notify algorithm
// (that method is private to the input package, and in any case diffs
// against the ROOT tree's hover state, which OverlayHost's modal capture
// bypasses entirely while any popup is open — see the type doc comment's
// "Hover" paragraph for why Router's own mechanism never runs during that
// window). Because OnPointer always closes every popup above the containing
// one BEFORE calling this (see its own doc comment), a Move that finds a
// DIFFERENT containing popup than the previous one already arrives here with
// old (== h.popupHover) reset to nil — that reset happened silently, as a
// side effect of closing the now-stale popup(s) above (see the field's own
// doc comment), not as a Leave delivered here. This diff then simply has
// nothing to deliver a Leave for (old is empty) and delivers Enter to every
// widget on newPath, with no separate "did the containing popup change"
// check needed to get that right.
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
