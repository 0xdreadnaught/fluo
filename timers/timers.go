package timers

import (
	"container/heap"
	"time"
)

// Queue is a frame-driven timer service: the host calls Advance(now) once per
// frame; due timers fire on that call, in due-time order. Not goroutine-safe.
//
// Pending timers are held in a min-heap keyed by due time, so firing k of n
// timers costs O(k log n) rather than the O(k·n log n) a re-sort-per-fire would.
// Timers with the same due time fire in insertion order: each timer carries a
// monotonic sequence number assigned at creation, used as the heap's tie-break,
// so equal-due ordering is stable frame to frame (a plain due-only sort left it
// unspecified).
type Queue struct {
	h   timerHeap
	now time.Time
	seq uint64 // next insertion sequence number, for stable equal-due ties
}

// Timer represents a scheduled callback, either one-shot (After) or repeating (Every).
type Timer struct {
	due     time.Time
	period  time.Duration // 0 for one-shot (After), >0 for repeating (Every)
	fn      func()
	stopped bool
	queue   *Queue // back-reference to remove from queue on Stop
	seq     uint64 // insertion order, the stable tie-break for equal due times
	index   int    // position in queue.h, or -1 when not currently heaped
}

// NewQueue creates a new timer queue starting at the given time.
func NewQueue(start time.Time) *Queue {
	return &Queue{now: start}
}

// After schedules a one-shot callback to fire after duration d.
func (q *Queue) After(d time.Duration, fn func()) *Timer {
	tm := &Timer{
		due:    q.now.Add(d),
		period: 0, // zero period means one-shot
		fn:     fn,
		queue:  q,
		seq:    q.seq,
		index:  -1,
	}
	q.seq++
	heap.Push(&q.h, tm)
	return tm
}

// Every schedules a repeating callback to fire every duration d (first fire at start+d).
// If d <= 0, treated as After (one-shot).
func (q *Queue) Every(d time.Duration, fn func()) *Timer {
	if d <= 0 {
		// Treat as After
		return q.After(d, fn)
	}
	tm := &Timer{
		due:    q.now.Add(d),
		period: d,
		fn:     fn,
		queue:  q,
		seq:    q.seq,
		index:  -1,
	}
	q.seq++
	heap.Push(&q.h, tm)
	return tm
}

// Advance fires all timers due at or before the given time, in due-time order.
// Timers added during callbacks participate in the same Advance.
func (q *Queue) Advance(now time.Time) {
	// Keep firing while the earliest pending timer is due at or before 'now'.
	// The loop re-reads the heap root each pass, so timers added during a
	// callback (and repeating timers rescheduled below) are picked up in the
	// same Advance if they too fall due within this window.
	for q.h.Len() > 0 {
		tm := q.h[0]
		if tm.due.After(now) {
			break // earliest timer is beyond the window: nothing more is due
		}

		// A stopped timer should never sit at the root — Stop removes it from
		// the heap immediately — but drop it defensively rather than fire it.
		if tm.stopped {
			heap.Pop(&q.h)
			continue
		}

		// Fire. The timer stays in the heap for the duration of its callback
		// (matching the single-threaded frame model where Len/NextDue observed
		// mid-callback still count it), so a self-Stop during the callback goes
		// through the same heap.Remove path as any other Stop. Advance q.now to
		// this timer's due time first, so timers scheduled inside the callback
		// measure their delay from the correct base.
		q.now = tm.due
		tm.fn()

		switch {
		case tm.index < 0:
			// The callback stopped tm (Stop -> heap.Remove cleared index): it
			// is already gone, so there is nothing to reschedule or remove.
		case tm.period > 0 && !tm.stopped:
			// Repeating timer still live: advance its due by one period and
			// re-establish heap order in place. Firing repeatedly here (rather
			// than once per frame) is what lets a repeating timer catch up on
			// elapsed/period iterations after a stalled frame.
			tm.due = tm.due.Add(tm.period)
			heap.Fix(&q.h, tm.index)
		default:
			// One-shot that fired (or a timer stopped without leaving the heap):
			// remove it.
			heap.Remove(&q.h, tm.index)
		}
	}

	// Final update to reflect the actual advance time.
	q.now = now
}

// NextDue returns the time of the next due timer, or false if the queue is
// empty. It is a pure read: the heap root is always the earliest timer, so no
// sorting (and no mutation) is needed.
func (q *Queue) NextDue() (time.Time, bool) {
	if q.h.Len() == 0 {
		return time.Time{}, false
	}
	return q.h[0].due, true
}

// Len returns the number of pending timers in the queue.
func (q *Queue) Len() int {
	return q.h.Len()
}

// Stop cancels the timer and removes it from the queue immediately. It is
// idempotent, and safe to call from within the timer's own callback (the timer
// is heaped during its callback, so this removes it before it can reschedule).
func (t *Timer) Stop() {
	if t.stopped {
		return
	}
	t.stopped = true

	// Remove from the heap immediately if still present. index < 0 means the
	// timer is not currently heaped (already fired/removed), so there is
	// nothing to pull out.
	if t.queue != nil && t.index >= 0 {
		heap.Remove(&t.queue.h, t.index)
	}
}

// timerHeap is a min-heap of *Timer ordered by due time, with the insertion
// sequence number as a stable tie-break for equal due times. It implements
// heap.Interface; each timer's index field is kept in sync by Swap/Push/Pop so
// an arbitrary timer can be removed in O(log n) via heap.Remove (used by Stop).
type timerHeap []*Timer

func (h timerHeap) Len() int { return len(h) }

func (h timerHeap) Less(i, j int) bool {
	if h[i].due.Equal(h[j].due) {
		return h[i].seq < h[j].seq // stable: earlier-created fires first
	}
	return h[i].due.Before(h[j].due)
}

func (h timerHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *timerHeap) Push(x any) {
	tm := x.(*Timer)
	tm.index = len(*h)
	*h = append(*h, tm)
}

func (h *timerHeap) Pop() any {
	old := *h
	n := len(old)
	tm := old[n-1]
	old[n-1] = nil // release the reference for GC
	tm.index = -1
	*h = old[:n-1]
	return tm
}
