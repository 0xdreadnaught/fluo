package bind

import (
	"fmt"

	"github.com/0xdreadnaught/fluo/controls"
	"github.com/0xdreadnaught/fluo/core"
)

// ChangeKind enumerates types of list mutations. It is a type alias for
// controls.ListChangeKind, not an independent definition: Phase 7 Task 2
// needs controls.ListView to declare a ListItems interface whose OnChange
// method names this payload type, and controls cannot import bind (bind
// already imports controls, for Items and, from Task 3, ListSelected — the
// reverse edge would be an import cycle). Declaring the canonical type in
// controls and aliasing it here keeps ChangeKind's published name and
// values exactly as they were before Task 2 (this is a real alias, not a
// same-shaped redefinition, so ChangeKind and controls.ListChangeKind are
// the identical type, and *List[T] satisfies controls.ListItems — see the
// static assertion below) while breaking the cycle. See
// controls/listview.go's ListChange doc comment for the full rationale.
type ChangeKind = controls.ListChangeKind

const (
	ChangeAdd     = controls.ListChangeAdd
	ChangeRemove  = controls.ListChangeRemove
	ChangeReplace = controls.ListChangeReplace
	ChangeReset   = controls.ListChangeReset
)

// Change represents a granular list mutation event. Like ChangeKind above,
// this is a type alias for controls.ListChange (see ChangeKind's doc
// comment for why).
type Change = controls.ListChange

// subEntry pairs a coarse OnChanged subscriber with the id its cancel closes
// over. Subscribers are held in registration order (a slice, not a map) so
// notify delivers to them deterministically — see notify.
type subEntry struct {
	id int
	fn func()
}

// granSubEntry is subEntry's granular (OnChange) counterpart.
type granSubEntry struct {
	id int
	fn func(Change)
}

// List is an observable slice for collection binding. Not goroutine-safe.
type List[T any] struct {
	items      []T
	subs       []subEntry
	nextID     int
	granSubs   []granSubEntry
	granNextID int
}

// Compile-time proof that *List[string] satisfies controls.ListItems (Len,
// At(int) string, OnChange(func(ListChange)) func()) — the whole point of
// ChangeKind/Change being aliases above: controls.NewListView can accept a
// *bind.List[string] directly without controls importing bind. If this
// assertion ever fails to compile, ListView's ListItems interface and
// List[T]'s method set have drifted apart; do not silence it, fix the
// mismatch.
var _ controls.ListItems = (*List[string])(nil)

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
	if len(items) == 1 {
		l.notifyGran(Change{Kind: ChangeAdd, Index: startIndex})
		return
	}
	// A multi-item Add coalesces into ONE Reset rather than one ChangeAdd per
	// appended item. Firing them in a loop meant a subscriber that mutates on
	// an early event (e.g. a cap-enforcing RemoveAt(0)) shifted every later
	// index out from under the remaining Change{Index} events, so they named
	// the wrong rows — or an out-of-range one a consumer's At would panic on.
	// A single Reset carries no index to go stale and tells consumers to
	// rebuild from the final list state, which is exactly what the in-tree
	// consumer (ListView) already does for any change. Single Add/Remove/Set
	// keep their precise granular event (see Insert/RemoveAt/Set below).
	l.notifyGran(Change{Kind: ChangeReset, Index: -1})
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
	id := l.nextID
	l.nextID++
	l.subs = append(l.subs, subEntry{id: id, fn: f})

	cancel = func() {
		for i := range l.subs {
			if l.subs[i].id == id {
				l.subs = append(l.subs[:i], l.subs[i+1:]...)
				return
			}
		}
	}
	return
}

// OnChange registers a subscriber to be called with granular change details.
// Granular events fire after the coarse OnChanged notification, and only after
// the mutation is fully applied — a subscriber processing a change already
// observes the final list state. A single-item Add/Insert/RemoveAt/Set emits
// one precise Change; a multi-item Add emits a single ChangeReset (see Add) so
// no index can go stale if the subscriber mutates the list mid-notification.
// Subscribers are notified in registration order (see notify/notifyGran).
// Returns a cancel function that removes the subscriber (idempotent).
func (l *List[T]) OnChange(f func(Change)) (cancel func()) {
	id := l.granNextID
	l.granNextID++
	l.granSubs = append(l.granSubs, granSubEntry{id: id, fn: f})

	cancel = func() {
		for i := range l.granSubs {
			if l.granSubs[i].id == id {
				l.granSubs = append(l.granSubs[:i], l.granSubs[i+1:]...)
				return
			}
		}
	}
	return
}

// notify calls all registered subscribers in stable registration order.
// It walks a snapshot copy of the subscriber slice, not the live one: a
// subscriber may add or cancel subscribers while this runs — including
// reentrantly, via a nested list mutation whose own notify recurses — and
// the copy keeps that from corrupting this walk (a cancel rewrites the live
// slice's backing array in place) or making delivery order depend on which
// subscribers a mid-walk mutation happened to add.
func (l *List[T]) notify() {
	subs := make([]subEntry, len(l.subs))
	copy(subs, l.subs)
	for _, s := range subs {
		s.fn()
	}
}

// notifyGran calls all registered granular subscribers with a Change event,
// in stable registration order and over a snapshot copy for the same
// reentrancy/ordering reasons as notify (see its doc comment).
func (l *List[T]) notifyGran(c Change) {
	subs := make([]granSubEntry, len(l.granSubs))
	copy(subs, l.granSubs)
	for _, s := range subs {
		s.fn(c)
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
	// Subscribe BEFORE the initial rebuild so a list mutation made from within
	// makeItem on that very first pass is not lost. The rebuilding/pending
	// coalescing (above) only engages via this subscription: a makeItem that
	// mutates the list fires notify, which — only if we are already listening —
	// re-enters rebuild, sees rebuilding==true, and sets pending so one more
	// pass runs after the outer completes. Subscribing AFTER the first rebuild
	// (the prior order) meant that first-pass mutation reached no subscriber,
	// set no pending flag, and left the panel permanently stale. OnChanged only
	// registers the callback (it never invokes rebuild itself), so the explicit
	// rebuild() below still runs exactly once — no double build.
	cancel = l.OnChanged(rebuild)
	rebuild()
	return
}
