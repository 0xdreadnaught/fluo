package timers

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestAfterFiresOnceInOrder(t *testing.T) {
	q := NewQueue(t0)
	var got []int
	q.After(30*time.Millisecond, func() { got = append(got, 30) })
	q.After(10*time.Millisecond, func() { got = append(got, 10) })
	q.Advance(t0.Add(20 * time.Millisecond))
	if len(got) != 1 || got[0] != 10 {
		t.Fatalf("got %v", got)
	}
	q.Advance(t0.Add(40 * time.Millisecond))
	if len(got) != 2 || got[1] != 30 {
		t.Fatalf("got %v", got)
	}
	q.Advance(t0.Add(100 * time.Millisecond)) // one-shots do not refire
	if len(got) != 2 {
		t.Fatalf("refired: %v", got)
	}
}

func TestEveryRepeats(t *testing.T) {
	q := NewQueue(t0)
	n := 0
	q.Every(10*time.Millisecond, func() { n++ })
	q.Advance(t0.Add(35 * time.Millisecond)) // catches up: fires at 10,20,30
	if n != 3 {
		t.Fatalf("n=%d", n)
	}
}

func TestStop(t *testing.T) {
	q := NewQueue(t0)
	n := 0
	tm := q.Every(10*time.Millisecond, func() { n++ })
	q.Advance(t0.Add(10 * time.Millisecond))
	if q.Len() != 1 {
		t.Fatalf("timer not in queue before stop")
	}
	tm.Stop()
	if q.Len() != 0 {
		t.Fatalf("timer still in queue after stop")
	}
	tm.Stop() // idempotent
	if q.Len() != 0 {
		t.Fatalf("timer back in queue after idempotent stop")
	}
	q.Advance(t0.Add(50 * time.Millisecond))
	if n != 1 {
		t.Fatalf("n=%d", n)
	}
	if q.Len() != 0 {
		t.Fatalf("stopped timer still queued")
	}
}

func TestNextDue(t *testing.T) {
	q := NewQueue(t0)
	if _, ok := q.NextDue(); ok {
		t.Fatal("empty queue has NextDue")
	}
	q.After(20*time.Millisecond, func() {})
	q.After(10*time.Millisecond, func() {})
	due, ok := q.NextDue()
	if !ok || !due.Equal(t0.Add(10*time.Millisecond)) {
		t.Fatalf("due=%v ok=%v", due, ok)
	}
}

func TestAddDuringFire(t *testing.T) {
	q := NewQueue(t0)
	n := 0
	q.After(10*time.Millisecond, func() {
		q.After(5*time.Millisecond, func() { n++ }) // due at 15, within same Advance window
	})
	q.Advance(t0.Add(30 * time.Millisecond))
	if n != 1 {
		t.Fatalf("timer added during fire did not fire in same Advance: n=%d", n)
	}
}

func TestSelfStopDuringFire(t *testing.T) {
	q := NewQueue(t0)
	var tm *Timer
	n := 0
	tm = q.Every(10*time.Millisecond, func() {
		n++
		if n == 3 {
			tm.Stop()
		}
	})
	q.Advance(t0.Add(50 * time.Millisecond))
	if n != 3 {
		t.Fatalf("n=%d", n)
	}
	if q.Len() != 0 {
		t.Fatalf("len=%d", q.Len())
	}
}

func TestEqualDueFiresInInsertionOrder(t *testing.T) {
	// Timers with the same due time must fire in the order they were created,
	// stably, every frame — the property the old unstable sort did not give.
	// Run several fresh queues to make an accidental ordering unlikely to pass
	// by chance.
	for trial := 0; trial < 20; trial++ {
		q := NewQueue(t0)
		var got []int
		for i := 0; i < 5; i++ {
			i := i
			q.After(10*time.Millisecond, func() { got = append(got, i) })
		}
		q.Advance(t0.Add(10 * time.Millisecond))
		want := []int{0, 1, 2, 3, 4}
		if len(got) != len(want) {
			t.Fatalf("trial %d: got %v, want %v", trial, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("trial %d: equal-due order = %v, want %v (insertion order)", trial, got, want)
			}
		}
	}
}

func TestEqualDueRepeatingStableAcrossFrames(t *testing.T) {
	// Two repeating timers sharing a period and phase tie on due time every
	// frame; the tie must break the same way (insertion order) on each fire,
	// not drift frame to frame.
	q := NewQueue(t0)
	var got []int
	q.Every(10*time.Millisecond, func() { got = append(got, 0) })
	q.Every(10*time.Millisecond, func() { got = append(got, 1) })
	for f := 1; f <= 3; f++ {
		q.Advance(t0.Add(time.Duration(f) * 10 * time.Millisecond))
	}
	want := []int{0, 1, 0, 1, 0, 1}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("repeating equal-due order = %v, want %v", got, want)
		}
	}
}

func TestStopDuringAdvanceCancelsUnfiredTimer(t *testing.T) {
	// An earlier timer's callback stops a later timer that is also due in this
	// same Advance window: the stopped timer must not fire, and the queue must
	// end empty.
	q := NewQueue(t0)
	var later *Timer
	firedEarly, firedLate := 0, 0
	q.After(10*time.Millisecond, func() {
		firedEarly++
		later.Stop() // cancel the not-yet-fired timer mid-Advance
	})
	later = q.After(20*time.Millisecond, func() { firedLate++ })
	q.Advance(t0.Add(30 * time.Millisecond))
	if firedEarly != 1 {
		t.Fatalf("firedEarly=%d", firedEarly)
	}
	if firedLate != 0 {
		t.Fatalf("timer stopped mid-Advance still fired: firedLate=%d", firedLate)
	}
	if q.Len() != 0 {
		t.Fatalf("queue not empty after stop-during-advance: len=%d", q.Len())
	}
}

func TestRepeatingCatchesUpAfterStall(t *testing.T) {
	// After a stalled frame, a repeating timer replays elapsed/period
	// iterations in one Advance, in due order, and reschedules cleanly.
	q := NewQueue(t0)
	var fires []time.Duration
	q.Every(10*time.Millisecond, func() {
		fires = append(fires, q.now.Sub(t0))
	})
	// A long stall: one Advance jumps 55ms, so fires are due at 10,20,30,40,50.
	q.Advance(t0.Add(55 * time.Millisecond))
	want := []time.Duration{10, 20, 30, 40, 50}
	if len(fires) != len(want) {
		t.Fatalf("catch-up fired %d times, want %d (%v)", len(fires), len(want), fires)
	}
	for i := range want {
		if fires[i] != want[i]*time.Millisecond {
			t.Fatalf("catch-up fire %d at %v, want %v", i, fires[i], want[i]*time.Millisecond)
		}
	}
	// It keeps repeating on the next frame at the next period boundary (60ms).
	fires = nil
	q.Advance(t0.Add(65 * time.Millisecond))
	if len(fires) != 1 || fires[0] != 60*time.Millisecond {
		t.Fatalf("post-stall fires = %v, want [60ms]", fires)
	}
}

func TestSelfStopWithUnrelatedTimer(t *testing.T) {
	q := NewQueue(t0)
	var tm1 *Timer
	n1 := 0
	n2 := 0
	tm1 = q.After(10*time.Millisecond, func() {
		n1++
		tm1.Stop() // self-stop during callback
	})
	q.After(20*time.Millisecond, func() {
		n2++ // unrelated timer must still fire
	})
	q.Advance(t0.Add(50 * time.Millisecond))
	if n1 != 1 {
		t.Fatalf("n1=%d", n1)
	}
	if n2 != 1 {
		t.Fatalf("unrelated timer did not fire: n2=%d", n2)
	}
	if q.Len() != 0 {
		t.Fatalf("queue not empty: len=%d", q.Len())
	}
}
