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

// tooltipDelay is the hover dwell time before the tip shows, once a
// timers.Queue has been wired via SetTimers (normative: 600ms, per the Phase
// 5 Task 8 spec).
const tooltipDelay = 600 * time.Millisecond

// ToolTipArea is a transparent, single-child wrapper widget (Border-like:
// MeasureContent/ArrangeContent/Children all delegate entirely to child, so
// the wrapper adds no chrome and child's arranged bounds always equal the
// wrapper's own — see the type's golden test for this "transparent to
// layout" property) that shows a small tip popup after the pointer dwells
// over it.
//
// Implements input.PointerHandler (Enter/Leave only) but deliberately never
// sets e.Handled — a tooltip must not swallow hover events that child (or
// anything wrapping this ToolTipArea) also wants to see.
//
// With a timers.Queue wired (SetTimers), Enter starts a one-shot tooltipDelay
// timer that shows the tip when it fires; Leave (or a second Enter, though
// that shouldn't happen without an intervening Leave) cancels any pending
// timer. With no queue wired (the zero value), Enter shows the tip
// immediately — matching TextBox's "no queue == no timing behavior, but the
// underlying feature (blink there, delay here) still degrades to something
// reasonable" convention.
//
// The tip popup, once shown, is closed on Leave — not on light-dismiss: it
// carries no onDismiss-driven state of its own beyond the same open/popup
// bookkeeping ComboBox uses (see showTip/hideTip), and is itself
// non-interactive (see newTipPopup), so a stray outside press closing it via
// OverlayHost's light-dismiss swallow is a harmless side effect, never the
// intended close path.
type ToolTipArea struct {
	core.Element

	child core.Widget

	face *text.Face
	tip  string

	timerQueue   *timers.Queue
	pendingTimer *timers.Timer

	open  bool
	popup core.Widget

	colors  theme.ColorTokens
	metrics theme.MetricTokens
}

// NewToolTipArea returns a ToolTipArea wrapping child (re-parented to it),
// showing tip in face (face may be nil, per TextBlock) when hovered.
func NewToolTipArea(child core.Widget, face *text.Face, tip string) *ToolTipArea {
	th := theme.Active()
	ta := &ToolTipArea{
		child:   child,
		face:    face,
		tip:     tip,
		colors:  th.Color,
		metrics: th.Metric,
	}
	core.SetParent(child, ta)
	return ta
}

// SetTimers wires q as the hover-delay driver: Enter now starts a
// tooltipDelay one-shot timer (via q.After) rather than showing the tip
// immediately. Passing nil detaches any previously wired queue and reverts
// to immediate-show-on-Enter. Calling SetTimers again always stops whatever
// timer is currently pending first, matching TextBox.SetTimers' "a
// superseded queue can never keep affecting this widget" guarantee.
func (ta *ToolTipArea) SetTimers(q *timers.Queue) *ToolTipArea {
	if ta.pendingTimer != nil {
		ta.pendingTimer.Stop()
		ta.pendingTimer = nil
	}
	ta.timerQueue = q
	return ta
}

// MeasureContent measures child with the full available space and reports
// its desired size unchanged — ToolTipArea adds no chrome/padding.
func (ta *ToolTipArea) MeasureContent(available render.Size) render.Size {
	core.MeasureWidget(ta.child, available)
	return core.DesiredSizeOf(ta.child)
}

// ArrangeContent arranges child to fill the wrapper's own bounds exactly.
func (ta *ToolTipArea) ArrangeContent(bounds render.Rect) {
	core.ArrangeWidget(ta.child, bounds)
}

// Children returns the single wrapped child.
func (ta *ToolTipArea) Children() []core.Widget {
	return []core.Widget{ta.child}
}

// showTip opens the tip popup (a no-op if already open, or if this
// ToolTipArea isn't attached beneath an OverlayHost), anchored at the
// wrapper's own bounds (equal to child's, per the type doc comment).
func (ta *ToolTipArea) showTip() {
	if ta.open {
		return
	}
	host := OverlayHostFor(ta)
	if host == nil {
		return
	}

	ta.open = true
	popup := newTipPopup(ta.face, ta.tip, ta.colors, ta.metrics)
	ta.popup = popup

	host.ShowPopup(popup, ta.Bounds(), func() {
		ta.open = false
		ta.popup = nil
	})
}

// hideTip explicitly closes the tip popup via OverlayHost.ClosePopup (whose
// onDismiss resets ta.open/ta.popup, matching ComboBox's closePopup
// convention). A no-op if not currently open.
func (ta *ToolTipArea) hideTip() {
	if !ta.open {
		return
	}
	host := OverlayHostFor(ta)
	if host != nil && ta.popup != nil {
		host.ClosePopup(ta.popup)
	}
}

// OnPointer implements input.PointerHandler: Enter arms the show (either
// immediately or after tooltipDelay, per whether a timers.Queue is wired —
// see the type doc comment); Leave cancels any pending timer and hides the
// tip if it's already showing. Never sets e.Handled (see the type doc
// comment).
func (ta *ToolTipArea) OnPointer(e *input.PointerEvent) {
	switch e.Action {
	case input.Enter:
		if ta.pendingTimer != nil {
			ta.pendingTimer.Stop()
			ta.pendingTimer = nil
		}
		if ta.timerQueue == nil {
			ta.showTip()
			return
		}
		ta.pendingTimer = ta.timerQueue.After(tooltipDelay, func() {
			ta.pendingTimer = nil
			ta.showTip()
		})
	case input.Leave:
		if ta.pendingTimer != nil {
			ta.pendingTimer.Stop()
			ta.pendingTimer = nil
		}
		ta.hideTip()
	}
}

// newTipPopup builds the tip's popup content: a small, non-interactive
// CardBackground Border (radius CornerRadius, PaddingS padding) around a
// plain TextBlock. Non-interactive means literally that — Border implements
// no PointerHandler/KeyHandler, so a forwarded event inside it (per
// OverlayHost's capture-forwarding while open) simply finds no handler on
// its hit path and goes nowhere.
func newTipPopup(face *text.Face, tip string, colors theme.ColorTokens, metrics theme.MetricTokens) core.Widget {
	label := NewTextBlock(face, tip)
	label.SetColor(colors.TextPrimary)

	return NewBorder().
		SetBackground(colors.CardBackground).
		SetRadius(metrics.CornerRadius).
		SetPadding(render.Uniform(metrics.PaddingS)).
		SetChild(label)
}
