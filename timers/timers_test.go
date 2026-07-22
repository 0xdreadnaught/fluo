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
