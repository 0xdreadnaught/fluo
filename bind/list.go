package bind

import (
	"fmt"

	"github.com/0xdreadnaught/fluo/controls"
	"github.com/0xdreadnaught/fluo/core"
)

// ChangeKind enumerates types of list mutations.
type ChangeKind uint8

const (
	ChangeAdd ChangeKind = iota
	ChangeRemove
	ChangeReplace
	ChangeReset
)

// Change represents a granular list mutation event.
type Change struct {
	Kind  ChangeKind
	Index int // -1 for ChangeReset
}

// List is an observable slice for collection binding. Not goroutine-safe.
type List[T any] struct {
	items      []T
	subs       map[int]func()
	nextID     int
	granSubs   map[int]func(Change)
	granNextID int
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
		panic(fmt.Sprintf("bind: List.At index %d out of range [0, %d)", i, len(l.items)))
	}
	return l.items[i]
}

// Add appends items to the list and notifies subscribers (if len(items) > 0).
func (l *List[T]) Add(items ...T) {
	if len(items) == 0 {
		return
	}
	startIndex := len(l.items)
	l.items = append(l.items, items...)
	l.notify()
	for i := range items {
		l.notifyGran(Change{Kind: ChangeAdd, Index: startIndex + i})
	}
}

// Insert inserts item at index i. Panics if i is out of range.
func (l *List[T]) Insert(i int, item T) {
	if i < 0 || i > len(l.items) {
		panic(fmt.Sprintf("bind: List.Insert index %d out of range [0, %d]", i, len(l.items)))
	}
	l.items = append(l.items[:i], append([]T{item}, l.items[i:]...)...)
	l.notify()
	l.notifyGran(Change{Kind: ChangeAdd, Index: i})
}

// RemoveAt removes the item at index i. Panics if i is out of range.
func (l *List[T]) RemoveAt(i int) {
	if i < 0 || i >= len(l.items) {
		panic(fmt.Sprintf("bind: List.RemoveAt index %d out of range [0, %d)", i, len(l.items)))
	}
	l.items = append(l.items[:i], l.items[i+1:]...)
	l.notify()
	l.notifyGran(Change{Kind: ChangeRemove, Index: i})
}

// Set replaces the item at index i with a new value and notifies subscribers.
// Panics if i is out of range.
func (l *List[T]) Set(i int, item T) {
	if i < 0 || i >= len(l.items) {
		panic(fmt.Sprintf("bind: List.Set index %d out of range [0, %d)", i, len(l.items)))
	}
	l.items[i] = item
	l.notify()
	l.notifyGran(Change{Kind: ChangeReplace, Index: i})
}

// Replace replaces the entire list with new items and notifies subscribers (once).
func (l *List[T]) Replace(items ...T) {
	l.items = make([]T, len(items))
	copy(l.items, items)
	l.notify()
	l.notifyGran(Change{Kind: ChangeReset, Index: -1})
}

// OnChanged registers a subscriber to be called when the list changes.
// Returns a cancel function that removes the subscriber (idempotent).
func (l *List[T]) OnChanged(f func()) (cancel func()) {
	if l.subs == nil {
		l.subs = make(map[int]func())
	}
	id := l.nextID
	l.nextID++
	l.subs[id] = f

	cancel = func() {
		delete(l.subs, id)
	}
	return
}

// OnChange registers a subscriber to be called with granular change details.
// Returns a cancel function that removes the subscriber (idempotent).
func (l *List[T]) OnChange(f func(Change)) (cancel func()) {
	if l.granSubs == nil {
		l.granSubs = make(map[int]func(Change))
	}
	id := l.granNextID
	l.granNextID++
	l.granSubs[id] = f

	cancel = func() {
		delete(l.granSubs, id)
	}
	return
}

// notify calls all registered subscribers.
func (l *List[T]) notify() {
	for _, f := range l.subs {
		f()
	}
}

// notifyGran calls all registered granular subscribers with a Change event.
func (l *List[T]) notifyGran(c Change) {
	for _, f := range l.granSubs {
		f(c)
	}
}

// snapshotListItems returns a copy of all items in the list, copied
// directly from the internal slice (not via repeated At calls), so Items
// can iterate a rebuild pass without aliasing the live slice.
func snapshotListItems[T any](l *List[T]) []T {
	snapshot := make([]T, len(l.items))
	copy(snapshot, l.items)
	return snapshot
}

// Items binds a list to a panel: on ANY list change, panel.Clear() then
// Add(makeItem(item)) for each item, in order. v0 = full rebuild
// (virtualization arrives Phase 7).
//
// Reentrancy: if a list mutation occurs during makeItem() (while
// rebuilding), the rebuild coalesces into one additional rebuild after the
// outer completes, matching Property's own reentrancy semantics. Each pass
// snapshots the current items and iterates the snapshot; mutations
// discovered during the pass set a pending flag, and after the outer loop
// completes, one additional rebuild pass runs if pending, ensuring
// convergence (each pass has bounded iteration). Unsupported: a makeItem()
// that mutates on EVERY invocation across every pass will not converge.
func Items[T any](l *List[T], panel *controls.StackPanel, makeItem func(item T, index int) core.Widget) (cancel func()) {
	var rebuilding bool
	var pending bool
	var rebuild func()

	rebuild = func() {
		if rebuilding {
			pending = true
			return
		}
		rebuilding = true
		defer func() {
			rebuilding = false
			if pending {
				pending = false
				rebuild() // tail-recursive: one coalesced rebuild after outer completes
			}
		}()

		// Snapshot items at start of pass to bound iteration.
		// Any mutation during this pass will set pending=true via guard.
		snapshot := snapshotListItems(l)

		panel.Clear()
		for i := 0; i < len(snapshot); i++ {
			item := snapshot[i]
			widget := makeItem(item, i)
			panel.Add(widget)
		}
	}
	rebuild()

	// Subscribe to list changes
	cancel = l.OnChanged(rebuild)
	return
}
