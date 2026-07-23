package bind

import (
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/0xdreadnaught/fluo/controls"
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/text"
)

// --- List[T] ops ---

func TestNewListInitializesWithItems(t *testing.T) {
	l := NewList[int](1, 2, 3)
	if l.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", l.Len())
	}
	if l.At(0) != 1 || l.At(1) != 2 || l.At(2) != 3 {
		t.Fatalf("items %v %v %v, want 1 2 3", l.At(0), l.At(1), l.At(2))
	}
}

func TestNewListEmptyIsValid(t *testing.T) {
	l := NewList[int]()
	if l.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", l.Len())
	}
}

func TestListAtPanicsOutOfRange(t *testing.T) {
	l := NewList[int](1, 2)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("At(2) did not panic, want panic")
		}
	}()
	l.At(2)
}

func TestListAtPanicsNegativeIndex(t *testing.T) {
	l := NewList[int](1, 2)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("At(-1) did not panic, want panic")
		}
	}()
	l.At(-1)
}

func TestListAddAppends(t *testing.T) {
	l := NewList[int](1, 2)
	l.Add(3, 4)
	if l.Len() != 4 {
		t.Fatalf("Len() = %d, want 4", l.Len())
	}
	if l.At(0) != 1 || l.At(1) != 2 || l.At(2) != 3 || l.At(3) != 4 {
		t.Fatalf("items %v %v %v %v, want 1 2 3 4", l.At(0), l.At(1), l.At(2), l.At(3))
	}
}

func TestListInsertPanicsOutOfRange(t *testing.T) {
	l := NewList[int](1, 2)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Insert(3, ...) did not panic, want panic")
		}
	}()
	l.Insert(3, 99)
}

func TestListInsertPanicsNegativeIndex(t *testing.T) {
	l := NewList[int](1, 2)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Insert(-1, ...) did not panic, want panic")
		}
	}()
	l.Insert(-1, 99)
}

func TestListInsert(t *testing.T) {
	l := NewList[int](1, 3)
	l.Insert(1, 2)
	if l.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", l.Len())
	}
	if l.At(0) != 1 || l.At(1) != 2 || l.At(2) != 3 {
		t.Fatalf("items %v %v %v, want 1 2 3", l.At(0), l.At(1), l.At(2))
	}
}

func TestListInsertAtStart(t *testing.T) {
	l := NewList[int](2, 3)
	l.Insert(0, 1)
	if l.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", l.Len())
	}
	if l.At(0) != 1 || l.At(1) != 2 || l.At(2) != 3 {
		t.Fatalf("items %v %v %v, want 1 2 3", l.At(0), l.At(1), l.At(2))
	}
}

func TestListRemoveAtPanicsOutOfRange(t *testing.T) {
	l := NewList[int](1, 2)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("RemoveAt(2) did not panic, want panic")
		}
	}()
	l.RemoveAt(2)
}

func TestListRemoveAtPanicsNegativeIndex(t *testing.T) {
	l := NewList[int](1, 2)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("RemoveAt(-1) did not panic, want panic")
		}
	}()
	l.RemoveAt(-1)
}

func TestListRemoveAt(t *testing.T) {
	l := NewList[int](1, 2, 3)
	l.RemoveAt(1)
	if l.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", l.Len())
	}
	if l.At(0) != 1 || l.At(1) != 3 {
		t.Fatalf("items %v %v, want 1 3", l.At(0), l.At(1))
	}
}

func TestListSet(t *testing.T) {
	l := NewList[int](1, 2, 3)
	l.Set(1, 99)
	if l.At(1) != 99 {
		t.Fatalf("At(1) = %d, want 99", l.At(1))
	}
	if l.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", l.Len())
	}
}

func TestListReplace(t *testing.T) {
	l := NewList[int](1, 2, 3)
	l.Replace(10, 20)
	if l.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", l.Len())
	}
	if l.At(0) != 10 || l.At(1) != 20 {
		t.Fatalf("items %v %v, want 10 20", l.At(0), l.At(1))
	}
}

func TestListReplaceToEmpty(t *testing.T) {
	l := NewList[int](1, 2, 3)
	l.Replace()
	if l.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", l.Len())
	}
}

// --- OnChanged notifications ---

func TestListOnChangedFiresOnAdd(t *testing.T) {
	l := NewList[int](1)
	fires := 0
	l.OnChanged(func() { fires++ })
	l.Add(2)
	if fires != 1 {
		t.Fatalf("fires = %d, want 1", fires)
	}
}

func TestListOnChangedFiresOnInsert(t *testing.T) {
	l := NewList[int](1, 3)
	fires := 0
	l.OnChanged(func() { fires++ })
	l.Insert(1, 2)
	if fires != 1 {
		t.Fatalf("fires = %d, want 1", fires)
	}
}

func TestListOnChangedFiresOnRemoveAt(t *testing.T) {
	l := NewList[int](1, 2, 3)
	fires := 0
	l.OnChanged(func() { fires++ })
	l.RemoveAt(1)
	if fires != 1 {
		t.Fatalf("fires = %d, want 1", fires)
	}
}

func TestListOnChangedFiresOnSet(t *testing.T) {
	l := NewList[int](1, 2, 3)
	fires := 0
	l.OnChanged(func() { fires++ })
	l.Set(1, 99)
	if fires != 1 {
		t.Fatalf("fires = %d, want 1", fires)
	}
}

func TestListOnChangedFiresOncePerReplace(t *testing.T) {
	l := NewList[int](1, 2, 3)
	fires := 0
	l.OnChanged(func() { fires++ })
	l.Replace(10, 20, 30)
	if fires != 1 {
		t.Fatalf("fires = %d, want 1 (Replace fires once)", fires)
	}
}

func TestListOnChangedDoesNotFireOnNoOp(t *testing.T) {
	l := NewList[int](1)
	fires := 0
	l.OnChanged(func() { fires++ })
	l.Add() // empty Add is a no-op
	if fires != 0 {
		t.Fatalf("fires = %d, want 0 (no-op Add should not fire)", fires)
	}
}

func TestListOnChangedCancelStopsNotifications(t *testing.T) {
	l := NewList[int](1)
	fires := 0
	cancel := l.OnChanged(func() { fires++ })
	l.Add(2)
	if fires != 1 {
		t.Fatalf("fires = %d, want 1 (before cancel)", fires)
	}
	cancel()
	l.Add(3)
	if fires != 1 {
		t.Fatalf("fires = %d after cancel, want 1 (no new notifications)", fires)
	}
}

func TestListOnChangedCancelIsIdempotent(t *testing.T) {
	l := NewList[int](1)
	cancel := l.OnChanged(func() {})
	cancel()
	cancel() // must not panic
}

func TestListOnChangedMultipleSubscribers(t *testing.T) {
	l := NewList[int](1)
	fires1 := 0
	fires2 := 0
	l.OnChanged(func() { fires1++ })
	l.OnChanged(func() { fires2++ })
	l.Add(2)
	if fires1 != 1 || fires2 != 1 {
		t.Fatalf("fires1=%d fires2=%d, want both 1", fires1, fires2)
	}
}

// --- Items binding ---

func TestItemsInitialPopulation(t *testing.T) {
	face, err := text.Load(goregular.TTF)
	if err != nil {
		t.Fatalf("text.Load: %v", err)
	}
	textFace := text.NewFace(face, 14)

	l := NewList[string]("Alice", "Bob", "Charlie")
	panel := controls.NewStackPanel(controls.Vertical)

	makeCount := 0
	Items[string](l, panel, func(item string, index int) core.Widget {
		makeCount++
		tb := controls.NewTextBox(textFace)
		tb.SetText(item)
		return tb
	})

	if makeCount != 3 {
		t.Fatalf("make called %d times, want 3", makeCount)
	}
	if len(panel.Children()) != 3 {
		t.Fatalf("panel.Children() = %d, want 3", len(panel.Children()))
	}
}

func TestItemsEmptyListCreatesEmptyPanel(t *testing.T) {
	l := NewList[int]()
	panel := controls.NewStackPanel(controls.Vertical)

	Items[int](l, panel, func(item int, index int) core.Widget {
		return controls.NewCanvas()
	})

	if len(panel.Children()) != 0 {
		t.Fatalf("panel.Children() = %d, want 0", len(panel.Children()))
	}
}

func TestItemsIndexPassedCorrectly(t *testing.T) {
	l := NewList[string]("A", "B", "C")
	panel := controls.NewStackPanel(controls.Vertical)

	var indices []int
	Items[string](l, panel, func(item string, index int) core.Widget {
		indices = append(indices, index)
		return controls.NewCanvas()
	})

	if len(indices) != 3 || indices[0] != 0 || indices[1] != 1 || indices[2] != 2 {
		t.Fatalf("indices = %v, want [0 1 2]", indices)
	}
}

func TestItemsAddUpdatesPanel(t *testing.T) {
	face, err := text.Load(goregular.TTF)
	if err != nil {
		t.Fatalf("text.Load: %v", err)
	}
	textFace := text.NewFace(face, 14)

	l := NewList[string]("Alice")
	panel := controls.NewStackPanel(controls.Vertical)

	Items[string](l, panel, func(item string, index int) core.Widget {
		tb := controls.NewTextBox(textFace)
		tb.SetText(item)
		return tb
	})

	if len(panel.Children()) != 1 {
		t.Fatalf("initial panel.Children() = %d, want 1", len(panel.Children()))
	}

	l.Add("Bob")

	if len(panel.Children()) != 2 {
		t.Fatalf("after Add panel.Children() = %d, want 2", len(panel.Children()))
	}
}

func TestItemsRemoveAtUpdatesPanel(t *testing.T) {
	face, err := text.Load(goregular.TTF)
	if err != nil {
		t.Fatalf("text.Load: %v", err)
	}
	textFace := text.NewFace(face, 14)

	l := NewList[string]("Alice", "Bob", "Charlie")
	panel := controls.NewStackPanel(controls.Vertical)

	Items[string](l, panel, func(item string, index int) core.Widget {
		tb := controls.NewTextBox(textFace)
		tb.SetText(item)
		return tb
	})

	if len(panel.Children()) != 3 {
		t.Fatalf("initial panel.Children() = %d, want 3", len(panel.Children()))
	}

	l.RemoveAt(1)

	if len(panel.Children()) != 2 {
		t.Fatalf("after RemoveAt panel.Children() = %d, want 2", len(panel.Children()))
	}
}

func TestItemsSetUpdatesPanel(t *testing.T) {
	face, err := text.Load(goregular.TTF)
	if err != nil {
		t.Fatalf("text.Load: %v", err)
	}
	textFace := text.NewFace(face, 14)

	l := NewList[string]("Alice", "Bob")
	panel := controls.NewStackPanel(controls.Vertical)

	makeCount := 0
	Items[string](l, panel, func(item string, index int) core.Widget {
		makeCount++
		tb := controls.NewTextBox(textFace)
		tb.SetText(item)
		return tb
	})

	if makeCount != 2 {
		t.Fatalf("initial make count = %d, want 2", makeCount)
	}

	l.Set(0, "Charlie")

	if makeCount != 4 {
		t.Fatalf("after Set make count = %d, want 4 (full rebuild)", makeCount)
	}
	if len(panel.Children()) != 2 {
		t.Fatalf("after Set panel.Children() = %d, want 2", len(panel.Children()))
	}
}

func TestItemsReplaceUpdatesPanel(t *testing.T) {
	face, err := text.Load(goregular.TTF)
	if err != nil {
		t.Fatalf("text.Load: %v", err)
	}
	textFace := text.NewFace(face, 14)

	l := NewList[string]("Alice", "Bob")
	panel := controls.NewStackPanel(controls.Vertical)

	Items[string](l, panel, func(item string, index int) core.Widget {
		tb := controls.NewTextBox(textFace)
		tb.SetText(item)
		return tb
	})

	if len(panel.Children()) != 2 {
		t.Fatalf("initial panel.Children() = %d, want 2", len(panel.Children()))
	}

	l.Replace("David", "Eve", "Frank")

	if len(panel.Children()) != 3 {
		t.Fatalf("after Replace panel.Children() = %d, want 3", len(panel.Children()))
	}
}

func TestItemsCancelStopsUpdates(t *testing.T) {
	face, err := text.Load(goregular.TTF)
	if err != nil {
		t.Fatalf("text.Load: %v", err)
	}
	textFace := text.NewFace(face, 14)

	l := NewList[string]("Alice")
	panel := controls.NewStackPanel(controls.Vertical)

	cancel := Items[string](l, panel, func(item string, index int) core.Widget {
		tb := controls.NewTextBox(textFace)
		tb.SetText(item)
		return tb
	})

	if len(panel.Children()) != 1 {
		t.Fatalf("initial panel.Children() = %d, want 1", len(panel.Children()))
	}

	cancel()
	l.Add("Bob")

	if len(panel.Children()) != 1 {
		t.Fatalf("after cancel + Add panel.Children() = %d, want 1 (no update)", len(panel.Children()))
	}
}

func TestItemsCancelIsIdempotent(t *testing.T) {
	l := NewList[string]()
	panel := controls.NewStackPanel(controls.Vertical)

	cancel := Items[string](l, panel, func(item string, index int) core.Widget {
		return controls.NewCanvas()
	})

	cancel()
	cancel() // must not panic
}

func TestItemsIdentityPerIndexAfterAdd(t *testing.T) {
	face, err := text.Load(goregular.TTF)
	if err != nil {
		t.Fatalf("text.Load: %v", err)
	}
	textFace := text.NewFace(face, 14)

	l := NewList[string]("Alice", "Bob")
	panel := controls.NewStackPanel(controls.Vertical)

	// Record (widget, item, index) for each rebuild
	type record struct {
		widget core.Widget
		item   string
		index  int
	}
	var records []record

	Items[string](l, panel, func(item string, index int) core.Widget {
		tb := controls.NewTextBox(textFace)
		tb.SetText(item)
		records = append(records, record{tb, item, index})
		return tb
	})

	if len(records) != 2 || records[0].item != "Alice" || records[1].item != "Bob" {
		t.Fatalf("initial records incorrect")
	}

	// Add an item -> full rebuild
	records = records[:0]
	l.Add("Charlie")

	if len(records) != 3 {
		t.Fatalf("after Add: make called %d times, want 3 (full rebuild)", len(records))
	}
	if records[0].item != "Alice" || records[0].index != 0 {
		t.Fatalf("record[0]: item=%s index=%d, want Alice 0", records[0].item, records[0].index)
	}
	if records[1].item != "Bob" || records[1].index != 1 {
		t.Fatalf("record[1]: item=%s index=%d, want Bob 1", records[1].item, records[1].index)
	}
	if records[2].item != "Charlie" || records[2].index != 2 {
		t.Fatalf("record[2]: item=%s index=%d, want Charlie 2", records[2].item, records[2].index)
	}

	// Verify panel has 3 children with correct order
	panelChildren := panel.Children()
	if len(panelChildren) != 3 {
		t.Fatalf("panel.Children() = %d, want 3", len(panelChildren))
	}

	// CRITICAL: verify widget identity (not just item/index order)
	if panelChildren[0] != records[0].widget {
		t.Fatalf("panelChildren[0] != records[0].widget (widget identity mismatch)")
	}
	if panelChildren[1] != records[1].widget {
		t.Fatalf("panelChildren[1] != records[1].widget (widget identity mismatch)")
	}
	if panelChildren[2] != records[2].widget {
		t.Fatalf("panelChildren[2] != records[2].widget (widget identity mismatch)")
	}
}

func TestItemsIdentityPerIndexAfterRemove(t *testing.T) {
	face, err := text.Load(goregular.TTF)
	if err != nil {
		t.Fatalf("text.Load: %v", err)
	}
	textFace := text.NewFace(face, 14)

	l := NewList[string]("Alice", "Bob", "Charlie")
	panel := controls.NewStackPanel(controls.Vertical)

	type record struct {
		item  string
		index int
	}
	var records []record

	Items[string](l, panel, func(item string, index int) core.Widget {
		tb := controls.NewTextBox(textFace)
		tb.SetText(item)
		records = append(records, record{item, index})
		return tb
	})

	if len(records) != 3 {
		t.Fatalf("initial: make called %d times, want 3", len(records))
	}

	// Remove middle item -> full rebuild
	records = records[:0]
	l.RemoveAt(1)

	if len(records) != 2 {
		t.Fatalf("after RemoveAt(1): make called %d times, want 2 (full rebuild)", len(records))
	}
	if records[0].item != "Alice" || records[0].index != 0 {
		t.Fatalf("record[0]: item=%s index=%d, want Alice 0", records[0].item, records[0].index)
	}
	if records[1].item != "Charlie" || records[1].index != 1 {
		t.Fatalf("record[1]: item=%s index=%d, want Charlie 1", records[1].item, records[1].index)
	}
}

func TestItemsIdentityPerIndexAfterSet(t *testing.T) {
	face, err := text.Load(goregular.TTF)
	if err != nil {
		t.Fatalf("text.Load: %v", err)
	}
	textFace := text.NewFace(face, 14)

	l := NewList[string]("Alice", "Bob")
	panel := controls.NewStackPanel(controls.Vertical)

	type record struct {
		item  string
		index int
	}
	var records []record

	Items[string](l, panel, func(item string, index int) core.Widget {
		tb := controls.NewTextBox(textFace)
		tb.SetText(item)
		records = append(records, record{item, index})
		return tb
	})

	if len(records) != 2 {
		t.Fatalf("initial: make called %d times, want 2", len(records))
	}

	// Set triggers full rebuild
	records = records[:0]
	l.Set(0, "Changed")

	if len(records) != 2 {
		t.Fatalf("after Set: make called %d times, want 2 (full rebuild)", len(records))
	}
	if records[0].item != "Changed" || records[0].index != 0 {
		t.Fatalf("record[0]: item=%s index=%d, want Changed 0", records[0].item, records[0].index)
	}
	if records[1].item != "Bob" || records[1].index != 1 {
		t.Fatalf("record[1]: item=%s index=%d, want Bob 1", records[1].item, records[1].index)
	}
}

func TestItemsReentrancyGuard(t *testing.T) {
	face, err := text.Load(goregular.TTF)
	if err != nil {
		t.Fatalf("text.Load: %v", err)
	}
	textFace := text.NewFace(face, 14)

	l := NewList[string]("Alice")
	panel := controls.NewStackPanel(controls.Vertical)

	var rebuildCount int
	var mutatedInMake bool

	Items[string](l, panel, func(item string, index int) core.Widget {
		tb := controls.NewTextBox(textFace)
		tb.SetText(item)

		// Mutate during the subscribed rebuild (after Items binding).
		// We detect "subscribed" by tracking rebuild invocations.
		// First rebuild is the initial one (pre-subscription).
		// Second rebuild is triggered externally (subscription live).
		if item == "Alice" && rebuildCount > 0 && !mutatedInMake {
			mutatedInMake = true
			l.Add("Bob") // Mutation during subscribed rebuild → sets pending
		}

		return tb
	})

	rebuildCount++ // Count the initial rebuild

	// Verify initial state
	if len(panel.Children()) != 1 {
		t.Fatalf("initial panel.Children() = %d, want 1", len(panel.Children()))
	}

	// Trigger external mutation while subscription is live
	l.Add("Charlie")
	rebuildCount++ // This triggers a rebuild subscription

	// After external Add:
	// 1. rebuild() snapshots ["Alice", "Charlie"]
	// 2. make("Alice") is called, which mutates via l.Add("Bob") → pending=true
	// 3. make("Charlie") is called
	// 4. defer runs: rebuilding=false, pending=true
	// 5. rebuild() called again, snapshots ["Alice", "Bob", "Charlie"] and rebuilds

	// Verify final state
	panelChildren := panel.Children()
	if len(panelChildren) != 3 {
		t.Fatalf("final panel.Children() = %d, want 3 (full rebuild after reentrancy)", len(panelChildren))
	}

	if l.Len() != 3 {
		t.Fatalf("l.Len() = %d, want 3", l.Len())
	}

	// Verify correct order: Add("Bob") appends, so after ["Alice", "Charlie"],
	// Add("Bob") yields ["Alice", "Charlie", "Bob"]
	if l.At(0) != "Alice" || l.At(1) != "Charlie" || l.At(2) != "Bob" {
		t.Fatalf("list order incorrect: got [%s, %s, %s], want [Alice, Charlie, Bob]",
			l.At(0), l.At(1), l.At(2))
	}

	// Verify mutation was attempted
	if !mutatedInMake {
		t.Fatal("mutation during make() did not occur")
	}
}

// --- OnChange (granular) events ---

func TestListOnChangeAddSingleItem(t *testing.T) {
	l := NewList[int](1)
	var changes []Change
	l.OnChange(func(c Change) { changes = append(changes, c) })
	l.Add(2)

	if len(changes) != 1 {
		t.Fatalf("changes count = %d, want 1", len(changes))
	}
	if changes[0].Kind != ChangeAdd || changes[0].Index != 1 {
		t.Fatalf("change[0] = {Kind: %d, Index: %d}, want {Kind: %d, Index: 1}", changes[0].Kind, changes[0].Index, ChangeAdd)
	}
}

func TestListOnChangeAddMultipleItems(t *testing.T) {
	l := NewList[int](1)
	var changes []Change
	l.OnChange(func(c Change) { changes = append(changes, c) })
	l.Add(2, 3, 4)

	if len(changes) != 3 {
		t.Fatalf("changes count = %d, want 3", len(changes))
	}
	for i, wantIdx := range []int{1, 2, 3} {
		if changes[i].Kind != ChangeAdd || changes[i].Index != wantIdx {
			t.Fatalf("changes[%d] = {Kind: %d, Index: %d}, want {Kind: %d, Index: %d}", i, changes[i].Kind, changes[i].Index, ChangeAdd, wantIdx)
		}
	}
}

func TestListOnChangeInsert(t *testing.T) {
	l := NewList[int](1, 3)
	var changes []Change
	l.OnChange(func(c Change) { changes = append(changes, c) })
	l.Insert(1, 2)

	if len(changes) != 1 {
		t.Fatalf("changes count = %d, want 1", len(changes))
	}
	if changes[0].Kind != ChangeAdd || changes[0].Index != 1 {
		t.Fatalf("change[0] = {Kind: %d, Index: %d}, want {Kind: %d, Index: 1}", changes[0].Kind, changes[0].Index, ChangeAdd)
	}
}

func TestListOnChangeRemoveAt(t *testing.T) {
	l := NewList[int](1, 2, 3)
	var changes []Change
	l.OnChange(func(c Change) { changes = append(changes, c) })
	l.RemoveAt(1)

	if len(changes) != 1 {
		t.Fatalf("changes count = %d, want 1", len(changes))
	}
	if changes[0].Kind != ChangeRemove || changes[0].Index != 1 {
		t.Fatalf("change[0] = {Kind: %d, Index: %d}, want {Kind: %d, Index: 1}", changes[0].Kind, changes[0].Index, ChangeRemove)
	}
}

func TestListOnChangeSet(t *testing.T) {
	l := NewList[int](1, 2, 3)
	var changes []Change
	l.OnChange(func(c Change) { changes = append(changes, c) })
	l.Set(1, 99)

	if len(changes) != 1 {
		t.Fatalf("changes count = %d, want 1", len(changes))
	}
	if changes[0].Kind != ChangeReplace || changes[0].Index != 1 {
		t.Fatalf("change[0] = {Kind: %d, Index: %d}, want {Kind: %d, Index: 1}", changes[0].Kind, changes[0].Index, ChangeReplace)
	}
}

func TestListOnChangeReplace(t *testing.T) {
	l := NewList[int](1, 2, 3)
	var changes []Change
	l.OnChange(func(c Change) { changes = append(changes, c) })
	l.Replace(10, 20)

	if len(changes) != 1 {
		t.Fatalf("changes count = %d, want 1", len(changes))
	}
	if changes[0].Kind != ChangeReset || changes[0].Index != -1 {
		t.Fatalf("change[0] = {Kind: %d, Index: %d}, want {Kind: %d, Index: -1}", changes[0].Kind, changes[0].Index, ChangeReset)
	}
}

func TestListOnChangeDoesNotFireOnNoOp(t *testing.T) {
	l := NewList[int](1)
	var changes []Change
	l.OnChange(func(c Change) { changes = append(changes, c) })
	l.Add() // empty Add is a no-op

	if len(changes) != 0 {
		t.Fatalf("changes count = %d, want 0 (no-op Add should not fire)", len(changes))
	}
}

func TestListOnChangeCancelStopsNotifications(t *testing.T) {
	l := NewList[int](1)
	var changes []Change
	cancel := l.OnChange(func(c Change) { changes = append(changes, c) })
	l.Add(2)

	if len(changes) != 1 {
		t.Fatalf("changes count before cancel = %d, want 1", len(changes))
	}

	cancel()
	l.Add(3)

	if len(changes) != 1 {
		t.Fatalf("changes count after cancel = %d, want 1 (no new notifications)", len(changes))
	}
}

func TestListOnChangeCancelIsIdempotent(t *testing.T) {
	l := NewList[int](1)
	cancel := l.OnChange(func(Change) {})
	cancel()
	cancel() // must not panic
}

func TestListOnChangeAndOnChangedBothFire(t *testing.T) {
	l := NewList[int](1)
	coarseCount := 0
	var granularChanges []Change

	l.OnChanged(func() { coarseCount++ })
	l.OnChange(func(c Change) { granularChanges = append(granularChanges, c) })

	l.Add(2)

	if coarseCount != 1 {
		t.Fatalf("coarse count = %d, want 1", coarseCount)
	}
	if len(granularChanges) != 1 {
		t.Fatalf("granular changes count = %d, want 1", len(granularChanges))
	}
}

func TestListOnChangeCancelIndependentFromOnChanged(t *testing.T) {
	l := NewList[int](1)
	coarseCount := 0
	var granularChanges []Change

	coarseCancel := l.OnChanged(func() { coarseCount++ })
	granularCancel := l.OnChange(func(c Change) { granularChanges = append(granularChanges, c) })

	l.Add(2)
	if coarseCount != 1 || len(granularChanges) != 1 {
		t.Fatalf("before cancel: coarse=%d, granular=%d, want 1, 1", coarseCount, len(granularChanges))
	}

	// Cancel only the granular channel
	granularCancel()
	l.Add(3)

	if coarseCount != 2 {
		t.Fatalf("after granular cancel: coarse count = %d, want 2", coarseCount)
	}
	if len(granularChanges) != 1 {
		t.Fatalf("after granular cancel: granular count = %d, want 1 (no new events)", len(granularChanges))
	}

	// Cancel only the coarse channel
	l.Add(4)
	if coarseCount != 3 {
		t.Fatalf("before coarse cancel: coarse count = %d, want 3", coarseCount)
	}

	coarseCancel()
	l.Add(5)

	if coarseCount != 3 {
		t.Fatalf("after coarse cancel: coarse count = %d, want 3 (no new events)", coarseCount)
	}
}

func TestListOnChangeMultipleSubscribers(t *testing.T) {
	l := NewList[int](1)
	var changes1 []Change
	var changes2 []Change

	l.OnChange(func(c Change) { changes1 = append(changes1, c) })
	l.OnChange(func(c Change) { changes2 = append(changes2, c) })

	l.Add(2)

	if len(changes1) != 1 || len(changes2) != 1 {
		t.Fatalf("changes1=%d changes2=%d, want both 1", len(changes1), len(changes2))
	}
}
