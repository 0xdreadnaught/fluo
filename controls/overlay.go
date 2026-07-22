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
// popup is open — it gets no hover/move feedback, only the topmost popup's
// interior and the dismiss-on-outside-Press decision are live. Documented v0
// simplification, not an oversight.
type OverlayHost struct {
	core.Element

	content core.Widget
	popups  []popupEntry
	router  *input.Router
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
// Press/Release/Move/Wheel exactly as if the router had hit-tested normally.
// Nothing here is dismissed in this branch, regardless of whether the
// forwarded delivery itself ends up Handled.
//
// When e.Pos falls outside the topmost popup's bounds, only Press matters:
// it closes that popup (via CloseTopPopup, so its onDismiss fires) and marks
// e.Handled, swallowing it. Every other outside action (Move/Release/Wheel)
// is swallowed silently with no forwarding — content (and, for now, any
// popup beneath the topmost one) gets no hover/move feedback while a popup
// is open; only the topmost popup's own interior and the dismiss-on-Press
// decision are live. This is a documented v0 simplification, not an
// oversight.
func (h *OverlayHost) OnPointer(e *input.PointerEvent) {
	if len(h.popups) == 0 {
		return
	}
	top := h.popups[len(h.popups)-1].w

	if core.BoundsOf(top).Contains(e.Pos) {
		path := input.HitPath(top, e.Pos)
		input.Bubble(path, e)
		return
	}

	if e.Action == input.Press {
		h.CloseTopPopup()
		e.Handled = true
	}
}
