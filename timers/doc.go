// Package timers provides a frame-driven timer service: Queue holds a set
// of pending Timer callbacks, and the host calls Advance(now) once per
// frame to fire whichever are due, in due-time order. After schedules a
// one-shot callback; Every schedules a repeating one; both return a *Timer
// whose Stop cancels it. Queue is not goroutine-safe and carries no
// wall-clock dependency of its own — Advance is driven entirely by the
// caller's notion of "now" — which is why app.Surface owns one and advances
// it every Frame, and why controls like TextBox (caret blink) and
// ToolTipArea (hover-dwell) accept a *timers.Queue via SetTimers rather
// than starting their own goroutine timers.
package timers
