package controls

import (
	"testing"
	"time"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/theme"
	"github.com/0xdreadnaught/fluo/timers"
)

// newTestToastHost returns a fresh OverlayHost with a plain Fixed content,
// laid out at 300x300 — the setup every test below shares.
func newTestToastHost(t *testing.T) *OverlayHost {
	t.Helper()
	host := NewOverlayHost()
	host.SetContent(NewFixed(300, 300, render.RGB(1, 2, 3)))
	layoutOverlay(host, 300, 300)
	return host
}

func TestShowToastAutoTimeoutRemovesAfterDuration(t *testing.T) {
	start := time.Now()
	q := timers.NewQueue(start)

	host := newTestToastHost(t)
	host.SetTimers(q)

	host.ShowToast(ToastSpec{Message: "hi", Timeout: 5 * time.Second})
	if got := host.PopupCount(); got != 0 {
		t.Fatalf("PopupCount = %d, want 0 (toasts aren't popups)", got)
	}
	if got := len(host.toasts); got != 1 {
		t.Fatalf("len(toasts) right after ShowToast = %d, want 1", got)
	}

	q.Advance(start.Add(2 * time.Second))
	if got := len(host.toasts); got != 1 {
		t.Fatalf("len(toasts) before timeout = %d, want 1 (not yet due)", got)
	}

	q.Advance(start.Add(5 * time.Second))
	if got := len(host.toasts); got != 0 {
		t.Fatalf("len(toasts) after Advance(Timeout) = %d, want 0 (auto-dismissed)", got)
	}
}

func TestShowToastStackingDistinctOffsets(t *testing.T) {
	host := newTestToastHost(t)

	host.ShowToast(ToastSpec{Message: "one"})
	host.ShowToast(ToastSpec{Message: "two"})
	host.ShowToast(ToastSpec{Message: "three"})

	layoutOverlay(host, 300, 300)

	if got := len(host.toasts); got != 3 {
		t.Fatalf("len(toasts) = %d, want 3", got)
	}

	var seen []render.Rect
	for _, e := range host.toasts {
		b := core.BoundsOf(e.w)
		for _, s := range seen {
			if s.Y == b.Y {
				t.Fatalf("two toasts share Y=%v: bounds %+v", b.Y, host.toasts)
			}
			if overlapsRect(s, b) {
				t.Fatalf("toasts overlap: %+v and %+v", s, b)
			}
		}
		seen = append(seen, b)
	}

	// All toasts are the same size (identical nil-face message), so the
	// expected Y positions follow directly from PaddingM/PaddingS, walked
	// newest-first (see arrangeToasts) — verify the concrete stack, not just
	// pairwise non-overlap.
	m := theme.Active().Metric
	h := core.DesiredSizeOf(host.toasts[0].w).H
	bounds := render.Rect{X: 0, Y: 0, W: 300, H: 300}

	wantNewest := bounds.Bottom() - m.PaddingM - h
	wantMiddle := wantNewest - m.PaddingS - h
	wantOldest := wantMiddle - m.PaddingS - h

	if got := core.BoundsOf(host.toasts[2].w).Y; got != wantNewest {
		t.Fatalf("newest (third shown) toast Y = %v, want %v", got, wantNewest)
	}
	if got := core.BoundsOf(host.toasts[1].w).Y; got != wantMiddle {
		t.Fatalf("middle toast Y = %v, want %v", got, wantMiddle)
	}
	if got := core.BoundsOf(host.toasts[0].w).Y; got != wantOldest {
		t.Fatalf("oldest (first shown) toast Y = %v, want %v", got, wantOldest)
	}
}

func TestShowToastDismissingMiddleReflows(t *testing.T) {
	host := newTestToastHost(t)

	host.ShowToast(ToastSpec{Message: "one"})
	dismissTwo := host.ShowToast(ToastSpec{Message: "two"})
	host.ShowToast(ToastSpec{Message: "three"})
	layoutOverlay(host, 300, 300)

	dismissTwo()
	layoutOverlay(host, 300, 300)

	if got := len(host.toasts); got != 2 {
		t.Fatalf("len(toasts) after dismissing the middle one = %d, want 2", got)
	}

	m := theme.Active().Metric
	h := core.DesiredSizeOf(host.toasts[0].w).H
	bounds := render.Rect{X: 0, Y: 0, W: 300, H: 300}

	wantNewest := bounds.Bottom() - m.PaddingM - h // "three", still newest
	wantOldest := wantNewest - m.PaddingS - h      // "one", moved down to fill the gap
	if got := core.BoundsOf(host.toasts[1].w).Y; got != wantNewest {
		t.Fatalf("remaining newest toast Y after reflow = %v, want %v", got, wantNewest)
	}
	if got := core.BoundsOf(host.toasts[0].w).Y; got != wantOldest {
		t.Fatalf("remaining oldest toast Y after reflow = %v, want %v (should move toward the corner)", got, wantOldest)
	}
}

func TestShowToastManualDismissRemovesAndIsIdempotent(t *testing.T) {
	host := newTestToastHost(t)

	dismiss := host.ShowToast(ToastSpec{Message: "hi"})
	if got := len(host.toasts); got != 1 {
		t.Fatalf("len(toasts) after ShowToast = %d, want 1", got)
	}

	dismiss()
	if got := len(host.toasts); got != 0 {
		t.Fatalf("len(toasts) after dismiss() = %d, want 0", got)
	}

	dismiss() // idempotent: must not panic or go negative
	if got := len(host.toasts); got != 0 {
		t.Fatalf("len(toasts) after second dismiss() = %d, want 0 (idempotent)", got)
	}
}

func TestShowToastClickToDismiss(t *testing.T) {
	host := newTestToastHost(t)

	host.ShowToast(ToastSpec{Message: "hi"})
	if got := len(host.toasts); got != 1 {
		t.Fatalf("len(toasts) = %d, want 1", got)
	}

	toast := host.toasts[0].w
	e := &input.PointerEvent{Action: input.Press, Target: toast}
	toast.(input.PointerHandler).OnPointer(e)

	if !e.Handled {
		t.Fatal("Press on a toast did not set e.Handled")
	}
	if got := len(host.toasts); got != 0 {
		t.Fatalf("len(toasts) after clicking the toast = %d, want 0", got)
	}
}

func TestShowToastNilTimersDegradesToNoAutoDismiss(t *testing.T) {
	host := newTestToastHost(t) // no SetTimers call: nil queue.

	dismiss := host.ShowToast(ToastSpec{Message: "hi", Timeout: time.Second})
	if got := len(host.toasts); got != 1 {
		t.Fatalf("len(toasts) after ShowToast with no timers wired = %d, want 1 (shown regardless)", got)
	}

	// Nothing to advance (no queue was ever wired) — the toast simply stays
	// open indefinitely; only an explicit dismiss removes it.
	if got := len(host.toasts); got != 1 {
		t.Fatalf("len(toasts) with no timers wired, before any manual dismiss = %d, want 1 (no auto-dismiss ever fires)", got)
	}
	dismiss()
	if got := len(host.toasts); got != 0 {
		t.Fatalf("len(toasts) after manual dismiss = %d, want 0", got)
	}
}

func TestShowToastSeverityInfoRendersLikeDefault(t *testing.T) {
	host := newTestToastHost(t)

	host.ShowToast(ToastSpec{Message: "hi"})
	host.ShowToast(ToastSpec{Message: "hi", Severity: SeverityInfo})

	plain := host.toasts[0].w.(*Toast)
	explicit := host.toasts[1].w.(*Toast)

	if plain.severity != SeverityInfo {
		t.Fatalf("no-Severity toast defaulted to %v, want SeverityInfo", plain.severity)
	}
	if _, ok := severityColor(plain.severity, plain.colors); ok {
		t.Fatal("SeverityInfo (zero value) should have no accent color")
	}
	if _, ok := severityColor(explicit.severity, explicit.colors); ok {
		t.Fatal("explicit SeverityInfo should have no accent color")
	}
}

func TestShowToastSeverityUsesMatchingThemeToken(t *testing.T) {
	th := theme.Active()

	cases := []struct {
		severity Severity
		want     render.Color
	}{
		{SeveritySuccess, th.Color.SeveritySuccess},
		{SeverityWarning, th.Color.SeverityWarning},
		{SeverityError, th.Color.SeverityError},
	}

	for _, c := range cases {
		host := newTestToastHost(t)
		host.ShowToast(ToastSpec{Message: "hi", Severity: c.severity})

		toast := host.toasts[0].w.(*Toast)
		if toast.severity != c.severity {
			t.Fatalf("toast.severity = %v, want %v", toast.severity, c.severity)
		}
		got, ok := severityColor(toast.severity, toast.colors)
		if !ok {
			t.Fatalf("severity %v: expected an accent color", c.severity)
		}
		if got != c.want {
			t.Fatalf("severity %v: accent color = %v, want %v", c.severity, got, c.want)
		}
	}
}

// overlapsRect reports whether a and b (both axis-aligned rects) overlap.
func overlapsRect(a, b render.Rect) bool {
	if a.X+a.W <= b.X || b.X+b.W <= a.X {
		return false
	}
	if a.Y+a.H <= b.Y || b.Y+b.H <= a.Y {
		return false
	}
	return true
}
