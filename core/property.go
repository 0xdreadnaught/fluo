package core

// Property[T] is a reactive value: Set notifies subscribers on real changes.
// Not goroutine-safe; intended for single-threaded UI loops.
type Property[T comparable] struct {
	value  T
	subs   map[int]func(old, new T)
	nextID int
}

// Get returns the current value.
func (p *Property[T]) Get() T {
	return p.value
}

// Set assigns v if it differs from the current value and notifies all subscribers.
// If v equals the current value, this is a no-op (no assignment, no notification).
func (p *Property[T]) Set(v T) {
	if v == p.value {
		return
	}
	old := p.value
	p.value = v
	for _, f := range p.subs {
		f(old, v)
	}
}

// OnChange registers a subscriber to be called when the value changes.
// Returns a cancel function that removes the subscriber (idempotent).
func (p *Property[T]) OnChange(f func(old, new T)) func() {
	if p.subs == nil {
		p.subs = make(map[int]func(old, new T))
	}
	id := p.nextID
	p.nextID++
	p.subs[id] = f

	return func() {
		delete(p.subs, id)
	}
}
