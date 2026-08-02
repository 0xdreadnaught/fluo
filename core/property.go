package core

// Property[T] is a reactive value: Set notifies subscribers on real changes.
// Not goroutine-safe; intended for single-threaded UI loops.
//
// Notification order: subscribers are notified in the order they registered
// via OnChange (registration order), deterministically — never in Go map
// order. Canceling a subscription (calling the func returned by OnChange)
// during a notification is safe: the canceled subscriber is skipped for the
// remainder of the in-flight notification and every later one. Subscribing a
// new callback during a notification may or may not be observed by that same
// in-flight notification.
//
// Reentrant Set (a callback that Sets a different value mid-notification) is
// allowed and SUPERSEDES the notification it interrupts: the reentrant Set
// runs to completion first, delivering its own (old, new) pair to every
// subscriber, and the outer notification — whose (old, new) pair is now stale
// — abandons its remaining deliveries rather than handing subscribers a value
// the property no longer holds. This is what a coercing/validating subscriber
// relies on: after it reentrantly Sets the coerced value, both the model
// (Get) and every subscriber settle on that coerced value, and the outcome
// does not depend on subscription order.
type Property[T comparable] struct {
	value T

	// subs holds subscribers in registration order. Canceling a subscriber
	// nils its fn (a tombstone) rather than removing the entry mid-iteration;
	// tombstones are skipped during notification and compacted out once no
	// notification is in flight (see compact). id is the stable handle the
	// cancel closure matches on.
	subs   []subscriber[T]
	nextID int

	// gen increments on every Set that passes the equality gate, BEFORE its
	// notification loop. A notification captures gen up front and bails the
	// moment it observes gen has moved on — the signal that a reentrant Set
	// superseded it (see Set).
	gen int

	// notifying is the current nesting depth of active notification loops.
	// Cancel defers compaction while any loop is live (> 0) so it never
	// mutates the slice a loop is walking.
	notifying int
}

type subscriber[T comparable] struct {
	id int
	fn func(old, new T)
}

// Get returns the current value.
func (p *Property[T]) Get() T {
	return p.value
}

// Set assigns v if it differs from the current value and notifies all
// subscribers in registration order. If v equals the current value, this is a
// no-op (no assignment, no notification) — the equality gate that also makes a
// subscriber's echo (SetText → OnChanged → Set of the same value) terminate
// rather than loop.
//
// See the type doc comment for reentrancy: a callback that Sets a different
// value supersedes this notification, and this loop then stops early instead
// of delivering its now-stale (old, v) pair.
func (p *Property[T]) Set(v T) {
	if v == p.value {
		return
	}
	old := p.value
	p.value = v

	p.gen++
	gen := p.gen

	p.notifying++
	defer p.endNotify()

	for _, s := range p.subs {
		if s.fn == nil {
			continue // canceled subscriber (tombstone)
		}
		s.fn(old, v)
		if p.gen != gen {
			// A reentrant Set ran during s.fn and already delivered the newer
			// value to every subscriber. Our (old, v) pair is stale; stop
			// rather than hand out a superseded value.
			return
		}
	}
}

// endNotify closes one notification frame; when the outermost one ends it
// compacts away any subscribers canceled while notification was in flight.
func (p *Property[T]) endNotify() {
	p.notifying--
	if p.notifying == 0 {
		p.compact()
	}
}

// compact drops tombstoned (canceled) subscribers, preserving registration
// order. Only ever called with no notification in flight, so it never disturbs
// an active loop's iteration.
func (p *Property[T]) compact() {
	kept := p.subs[:0]
	for _, s := range p.subs {
		if s.fn != nil {
			kept = append(kept, s)
		}
	}
	// Clear the vacated tail so canceled closures can be garbage-collected.
	for i := len(kept); i < len(p.subs); i++ {
		p.subs[i] = subscriber[T]{}
	}
	p.subs = kept
}

// OnChange registers a subscriber to be called when the value changes, and
// returns a cancel function that removes it (idempotent — calling it more than
// once, or after the subscriber already fired for the last time, is harmless).
// Subscribers are notified in the order they were registered here.
func (p *Property[T]) OnChange(f func(old, new T)) func() {
	id := p.nextID
	p.nextID++
	p.subs = append(p.subs, subscriber[T]{id: id, fn: f})

	return func() {
		for i := range p.subs {
			if p.subs[i].id == id {
				p.subs[i].fn = nil
				if p.notifying == 0 {
					p.compact()
				}
				return
			}
		}
	}
}
