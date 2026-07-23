package bind

import (
	"github.com/0xdreadnaught/fluo/controls"
	"github.com/0xdreadnaught/fluo/core"
)

// List is an observable slice for collection binding. Not goroutine-safe.
type List[T any] struct {
	items  []T
	subs   map[int]func()
	nextID int
}

// NewList creates a new List with the provided initial items.
func NewList[T any](items ...T) *List[T] {
	l := &List[T]{
		items: make([]T, len(items)),
	}
	copy(l.items, items)
	return l
}

// Len returns the number of items in the list.
func (l *List[T]) Len() int {
	return len(l.items)
}

// At returns the item at index i. Panics if i is out of range (fail-fast, documented).
func (l *List[T]) At(i int) T {
	if i < 0 || i >= len(l.items) {
		panic("List: index out of range")
	}
	return l.items[i]
}

// Add appends items to the list and notifies subscribers (if len(items) > 0).
func (l *List[T]) Add(items ...T) {
	if len(items) == 0 {
		return
	}
	l.items = append(l.items, items...)
	l.notify()
}

// Insert inserts item at index i. Panics if i is out of range.
func (l *List[T]) Insert(i int, item T) {
	if i < 0 || i > len(l.items) {
		panic("List: index out of range")
	}
	l.items = append(l.items[:i], append([]T{item}, l.items[i:]...)...)
	l.notify()
}

// RemoveAt removes the item at index i. Panics if i is out of range.
func (l *List[T]) RemoveAt(i int) {
	if i < 0 || i >= len(l.items) {
		panic("List: index out of range")
	}
	l.items = append(l.items[:i], l.items[i+1:]...)
	l.notify()
}

// Set replaces the item at index i with a new value and notifies subscribers.
func (l *List[T]) Set(i int, item T) {
	if i < 0 || i >= len(l.items) {
		panic("List: index out of range")
	}
	l.items[i] = item
	l.notify()
}

// Replace replaces the entire list with new items and notifies subscribers (once).
func (l *List[T]) Replace(items ...T) {
	l.items = make([]T, len(items))
	copy(l.items, items)
	l.notify()
}

// OnChanged registers a subscriber to be called when the list changes.
// Returns a cancel function that removes the subscriber (idempotent).
func (l *List[T]) OnChanged(f func()) func() {
	if l.subs == nil {
		l.subs = make(map[int]func())
	}
	id := l.nextID
	l.nextID++
	l.subs[id] = f

	return func() {
		delete(l.subs, id)
	}
}

// notify calls all registered subscribers.
func (l *List[T]) notify() {
	for _, f := range l.subs {
		f()
	}
}

// Items binds a list to a panel: on ANY list change, panel.Clear() then Add(make(item))
// for each item, in order. v0 = full rebuild (virtualization arrives Phase 7).
func Items[T any](l *List[T], panel *controls.StackPanel, make func(item T, index int) core.Widget) func() {
	// Initial population
	rebuild := func() {
		panel.Clear()
		for i := 0; i < l.Len(); i++ {
			item := l.At(i)
			widget := make(item, i)
			panel.Add(widget)
		}
	}
	rebuild()

	// Subscribe to list changes
	cancel := l.OnChanged(rebuild)
	return cancel
}
