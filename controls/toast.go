package controls

import (
	"time"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
	"github.com/0xdreadnaught/fluo/theme"
	"github.com/0xdreadnaught/fluo/timers"
)

// Severity classifies the kind of condition a Toast is communicating,
// driving which theme.ColorTokens severity accent (if any) it renders as a
// glanceable color cue — see Toast.Render.
type Severity int

const (
	// SeverityInfo is the zero value and the default for a ToastSpec that
	// doesn't set Severity. It renders with NO color cue at all — a plain
	// neutral toast, byte-identical to every toast before severities
	// existed — so existing ShowToast callers are unaffected by this field
	// existing.
	SeverityInfo Severity = iota
	// SeveritySuccess marks a condition that completed as intended.
	SeveritySuccess
	// SeverityWarning marks a condition worth the user's attention but not
	// necessarily broken.
	SeverityWarning
	// SeverityError marks a failed or broken condition.
	SeverityError
)

// ToastSpec configures one transient notification shown via
// OverlayHost.ShowToast.
type ToastSpec struct {
	// Face renders Message; nil measures to zero and renders nothing (see
	// TextBlock's own "face may be nil" contract) — a toast built that way
	// still occupies its bevel chrome's minimum size but shows no text, so
	// callers should normally pass a real face.
	Face *text.Face
	// Message is the text shown in the toast body.
	Message string
	// Severity is the toast's kind, SeverityInfo (no color cue) by default —
	// see Severity's doc comment.
	Severity Severity
	// Timeout is how long the toast stays open before auto-dismissing, once
	// a timers.Queue is wired via OverlayHost.SetTimers. Timeout <= 0, or no
	// queue wired, disables auto-dismiss entirely — see ShowToast's doc
	// comment.
	Timeout time.Duration
}

// toastEntry is one entry in an OverlayHost's toast stack (h.toasts): the
// toast widget itself and its pending auto-dismiss timer (nil if none was
// scheduled — see ShowToast). Removed by identity scan in removeToast,
// mirroring popupEntry/ClosePopup's own convention.
type toastEntry struct {
	w     core.Widget
	timer *timers.Timer
}

// ShowToast opens a new Toast (see the type's doc comment for its chrome)
// displaying spec.Message, added to the host's toast stack and stacked in
// its bottom-right corner (see arrangeToasts): the NEWEST toast always sits
// at the fixed corner slot, and every older toast is pushed further from the
// corner to make room. It engages none of the modal/light-dismiss machinery
// ShowPopup/ShowPopupNonModal do (see OverlayHost's own "Modal vs non-modal
// popups" doc comment) — a toast is never dismissed by an outside press,
// only by its own timeout, a click on itself (see Toast.OnPointer), or the
// returned dismiss func. One consequence of staying outside that machinery:
// while a MODAL popup is also open and holds the router's capture, a press
// landing on a toast is NOT forwarded to it by OverlayHost.OnPointer (which
// only ever searches h.popups) — it falls into that method's
// outside-every-popup branch instead. This is an accepted v0 limitation:
// toasts and modal popups are not expected to need simultaneous interactive
// overlap in practice.
//
// With a timers.Queue wired via SetTimers and spec.Timeout > 0, the toast
// auto-removes itself after spec.Timeout via a one-shot timer on that queue.
// With no queue wired (the zero value) or spec.Timeout <= 0, the toast shows
// with no auto-dismiss at all — degrading like ToolTipArea's own "no queue
// == no timing behavior, but the underlying feature still shows something
// reasonable" convention (see its type doc comment): it simply stays open
// until manually dismissed.
//
// The returned dismiss func removes this toast early. It is idempotent — a
// second call (or one after the toast already auto-dismissed) is a no-op.
func (h *OverlayHost) ShowToast(spec ToastSpec) (dismiss func()) {
	th := theme.Active()
	toast := newToast(spec.Face, spec.Message, spec.Severity, th.Color, th.Metric)

	entry := &toastEntry{w: toast}
	h.toasts = append(h.toasts, entry)
	core.SetParent(toast, h)

	dismissed := false
	dismiss = func() {
		if dismissed {
			return
		}
		dismissed = true
		h.removeToast(toast)
	}
	toast.dismiss = dismiss

	if h.timerQueue != nil && spec.Timeout > 0 {
		entry.timer = h.timerQueue.After(spec.Timeout, func() {
			entry.timer = nil
			dismiss()
		})
	}

	h.InvalidateMeasure()
	return dismiss
}

// removeToast removes toast from h.toasts, wherever it sits in the stack (a
// plain by-identity scan, mirroring ClosePopup — not assumed to be the
// newest/oldest entry). A no-op if toast isn't currently in the stack, which
// is what makes ShowToast's returned dismiss func idempotent on its own,
// even before that func's own dismissed-bool guard is considered.
//
// On an actual removal: any pending auto-dismiss timer is stopped (a no-op
// if it already fired — this IS how it fires, calling dismiss itself, so by
// the time removeToast runs on that path entry.timer is already nil), toast
// is detached (parent cleared, and — if a router is wired — Detach clears
// any stale focus/capture/hover into its subtree, matching ClosePopup), and
// measure is invalidated so the remaining toasts reflow toward the corner on
// the next arrange pass (see arrangeToasts — reflow is not a separate step,
// it falls out of that method recomputing every position from current
// h.toasts order on every call).
func (h *OverlayHost) removeToast(toast core.Widget) {
	idx := -1
	for i, e := range h.toasts {
		if e.w == toast {
			idx = i
			break
		}
	}
	if idx == -1 {
		return
	}

	entry := h.toasts[idx]
	h.toasts = append(h.toasts[:idx], h.toasts[idx+1:]...)

	if entry.timer != nil {
		entry.timer.Stop()
	}

	core.SetParent(toast, nil)
	if h.router != nil {
		h.router.Detach(toast)
	}
	h.InvalidateMeasure()
}

// arrangeToasts places every currently open toast (h.toasts, oldest first —
// see ShowToast's append) stacked in the host's bottom-right corner, inset
// from both edges by PaddingM with PaddingS between consecutive toasts. The
// stack is walked NEWEST FIRST (from the end of h.toasts): the newest toast
// is anchored at the fixed corner slot, and each older toast is placed
// further away (upward), so an older toast visibly moves up to make room
// each time a newer one arrives. Because this recomputes every position
// unconditionally from current h.toasts order on every ArrangeContent call,
// dismissing any one toast (see removeToast) reflows every toast that was
// above it back down toward the corner on the very next pass — no separate
// reflow step is needed.
func (h *OverlayHost) arrangeToasts(bounds render.Rect) {
	if len(h.toasts) == 0 {
		return
	}

	m := theme.Active().Metric
	margin := m.PaddingM
	gap := m.PaddingS

	y := bounds.Bottom() - margin
	for i := len(h.toasts) - 1; i >= 0; i-- {
		w := h.toasts[i].w
		desired := core.DesiredSizeOf(w)
		x := bounds.Right() - margin - desired.W
		y -= desired.H
		core.ArrangeWidget(w, render.Rect{X: x, Y: y, W: desired.W, H: desired.H})
		y -= gap
	}
}

// Toast is the popup content ShowToast shows: a small, classic raised
// ButtonFace bevel card — identical chrome to ToolTipArea's own tip popup
// (see tipCard) — framing a single-line text message. Unlike tipCard, Toast
// is exported (ShowToast's stacking lives on OverlayHost, one package level
// up from tipCard's own tooltip-only usage) and optionally dismisses itself
// on a Press, via a dismiss func ShowToast wires onto it — see OnPointer.
//
// A non-SeverityInfo toast additionally renders a colored accent stripe
// down its left inner edge (see Render and severityColor) — a glanceable
// cue for the toast's kind. SeverityInfo renders no stripe at all, so a
// plain ToastSpec (Severity left at its zero value) looks exactly as it did
// before Severity existed.
type Toast struct {
	core.Element

	child core.Widget

	// dismiss is wired by ShowToast to this toast's own returned dismiss
	// func, letting OnPointer's click-to-dismiss reuse the exact same
	// idempotent removal path a caller-held dismiss func would.
	dismiss func()

	severity Severity
	colors   theme.ColorTokens
	metrics  theme.MetricTokens
}

// newToast returns a Toast wrapping a TextBlock showing message in face,
// colored WindowText — matches newTipPopup/tipCard's own label styling —
// and carrying severity for Render's accent-stripe cue.
func newToast(face *text.Face, message string, severity Severity, colors theme.ColorTokens, metrics theme.MetricTokens) *Toast {
	label := NewTextBlock(face, message)
	label.SetColor(colors.WindowText)

	t := &Toast{child: label, severity: severity, colors: colors, metrics: metrics}
	core.SetParent(label, t)
	return t
}

// severityColor returns severity's theme accent color and ok=true, or
// ok=false for SeverityInfo — which intentionally has no accent cue at all,
// so Toast.Render draws nothing for it (see Toast's own doc comment).
func severityColor(severity Severity, colors theme.ColorTokens) (c render.Color, ok bool) {
	switch severity {
	case SeveritySuccess:
		return colors.SeveritySuccess, true
	case SeverityWarning:
		return colors.SeverityWarning, true
	case SeverityError:
		return colors.SeverityError, true
	default:
		return render.Color{}, false
	}
}

// chrome returns the inset on every side: the bevel width plus PaddingS
// breathing room around the message — matches tipCard.chrome exactly.
func (t *Toast) chrome() render.Thickness {
	inset := t.metrics.BevelWidth + t.metrics.PaddingS
	return render.Uniform(inset)
}

// MeasureContent measures child within the available space reduced by
// chrome, then adds chrome back to its desired size — matches
// tipCard.MeasureContent.
func (t *Toast) MeasureContent(available render.Size) render.Size {
	c := t.chrome()

	availW := available.W - c.Left - c.Right
	if availW < 0 {
		availW = 0
	}
	availH := available.H - c.Top - c.Bottom
	if availH < 0 {
		availH = 0
	}

	core.MeasureWidget(t.child, render.Size{W: availW, H: availH})
	d := core.DesiredSizeOf(t.child)
	return render.Size{W: d.W + c.Left + c.Right, H: d.H + c.Top + c.Bottom}
}

// ArrangeContent arranges child within bounds inset by chrome.
func (t *Toast) ArrangeContent(bounds render.Rect) {
	inner := bounds.Inset(t.chrome())
	if inner.W < 0 {
		inner.W = 0
	}
	if inner.H < 0 {
		inner.H = 0
	}
	core.ArrangeWidget(t.child, inner)
}

// Children returns the single wrapped child (the message TextBlock).
func (t *Toast) Children() []core.Widget {
	return []core.Widget{t.child}
}

// Render draws the classic raised ButtonFace bevel framing the message —
// matches tipCard.Render exactly — then, for a non-SeverityInfo toast, a
// severity accent stripe down the card's left inner edge: PaddingS wide,
// inset by BevelWidth from the top/left/bottom so it sits inside the raised
// bevel rather than covering it. SeverityInfo (the default) draws no
// stripe, leaving the card identical to a plain pre-Severity toast.
func (t *Toast) Render(r render.Renderer) {
	bounds := t.Bounds()
	drawRaised(r, bounds, t.colors.ButtonFace, t.colors)

	if c, ok := severityColor(t.severity, t.colors); ok {
		bw := t.metrics.BevelWidth
		stripe := render.Rect{
			X: bounds.X + bw,
			Y: bounds.Y + bw,
			W: t.metrics.PaddingS,
			H: bounds.H - 2*bw,
		}
		r.FillRect(stripe, c)
	}
}

// OnPointer implements input.PointerHandler: a Press dismisses the toast
// early (click-to-dismiss) via the dismiss func ShowToast wired on
// construction, and marks e.Handled so the click doesn't fall through to
// whatever (if anything) sits beneath the toast. dismiss is itself
// idempotent (see ShowToast), so this is safe even if somehow reached again
// after the toast already closed.
func (t *Toast) OnPointer(e *input.PointerEvent) {
	if e.Action == input.Press && t.dismiss != nil {
		e.Handled = true
		t.dismiss()
	}
}
