// Package anim provides easing functions and a frame-driven Tween that
// interpolates a normalized progress value 0→1 over a duration, using a
// timers.Queue as its clock. It is the shared foundation controls use to
// smoothly cross-fade colors (see controls/animation.go) instead of
// snapping between states. Linear, EaseOut, and EaseInOut are the built-in
// Easing curves; NewTween starts a Tween immediately, calling onUpdate on
// every internal tick (at least once, even for a zero duration) and onDone
// exactly once when progress reaches 1 — Stop cancels a Tween early without
// firing onDone.
package anim
