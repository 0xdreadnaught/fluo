package timers

import (
	"sort"
	"time"
)

// Queue is a frame-driven timer service: the host calls Advance(now) once per
// frame; due timers fire on that call, in due-time order. Not goroutine-safe.
type Queue struct {
	items []*Timer
	now   time.Time
}

// Timer represents a scheduled callback, either one-shot (After) or repeating (Every).
type Timer struct {
	due     time.Time
	period  time.Duration // 0 for one-shot (After), >0 for repeating (Every)
	fn      func()
	stopped bool
	queue   *Queue // back-reference to remove from queue on Stop
}

// NewQueue creates a new timer queue starting at the given time.
func NewQueue(start time.Time) *Queue {
	return &Queue{
		items: make([]*Timer, 0),
		now:   start,
	}
}

// After schedules a one-shot callback to fire after duration d.
func (q *Queue) After(d time.Duration, fn func()) *Timer {
	due := q.now.Add(d)
	tm := &Timer{
		due:     due,
		period:  0, // zero period means one-shot
		fn:      fn,
		stopped: false,
		queue:   q,
	}
	q.items = append(q.items, tm)
	return tm
}

// Every schedules a repeating callback to fire every duration d (first fire at start+d).
// If d <= 0, treated as After (one-shot).
func (q *Queue) Every(d time.Duration, fn func()) *Timer {
	if d <= 0 {
		// Treat as After
		return q.After(d, fn)
	}
	due := q.now.Add(d)
	tm := &Timer{
		due:     due,
		period:  d,
		fn:      fn,
		stopped: false,
		queue:   q,
	}
	q.items = append(q.items, tm)
	return tm
}

// Advance fires all timers due at or before the given time, in due-time order.
// Timers added during callbacks participate in the same Advance.
func (q *Queue) Advance(now time.Time) {
	// Keep firing timers while there are items due at or before 'now'.
	// We loop because new timers added during callbacks may be due in this window.
	for len(q.items) > 0 {
		// Sort to find the earliest due timer
		sort.Slice(q.items, func(i, j int) bool {
			return q.items[i].due.Before(q.items[j].due)
		})

		// Check if the earliest timer is due
		if q.items[0].due.After(now) {
			break // No more timers due in this window
		}

		// Fire the earliest timer
		tm := q.items[0]
		if !tm.stopped {
			// Update q.now to the timer's due time before calling the callback
			// so that timers added during the callback use the correct base time
			q.now = tm.due
			tm.fn()
		}

		// Handle rescheduling or removal with a single index-of scan (by
		// identity, in case the callback stopped/removed tm itself): find
		// tm's current index once, then either reschedule it in place or
		// splice it out — no separate "is it still present" pass followed by
		// a second identity scan to remove it.
		idx := -1
		for i, t := range q.items {
			if t == tm {
				idx = i
				break
			}
		}
		switch {
		case idx == -1:
			// Already removed (by Stop() or other means) during the callback.
		case tm.period > 0 && !tm.stopped:
			// Repeating timer that's still in queue and not stopped: reschedule for next period
			tm.due = tm.due.Add(tm.period)
			// Re-sort and continue
		default:
			// One-shot or stopped timer that's still in queue: splice it out by index
			q.items = append(q.items[:idx], q.items[idx+1:]...)
		}
	}

	// Final update to reflect the actual advance time
	q.now = now
}

// NextDue returns the time of the next due timer, or false if the queue is empty.
func (q *Queue) NextDue() (time.Time, bool) {
	if len(q.items) == 0 {
		return time.Time{}, false
	}

	// Sort to find earliest
	sort.Slice(q.items, func(i, j int) bool {
		return q.items[i].due.Before(q.items[j].due)
	})

	return q.items[0].due, true
}

// Len returns the number of pending timers in the queue.
func (q *Queue) Len() int {
	return len(q.items)
}

// remove removes a timer by identity from the queue.
// If the timer is not found, it does nothing.
func (q *Queue) remove(tm *Timer) {
	for i, timer := range q.items {
		if timer == tm {
			q.items = append(q.items[:i], q.items[i+1:]...)
			return
		}
	}
}

// Stop cancels the timer and removes it from the queue immediately. It is idempotent.
func (t *Timer) Stop() {
	if t.stopped {
		return
	}
	t.stopped = true

	// Remove from queue immediately
	if t.queue != nil {
		t.queue.remove(t)
	}
}
