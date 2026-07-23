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

	var makeCount int
	var addedInMake bool

	Items[string](l, panel, func(item string, index int) core.Widget {
		makeCount++
		tb := controls.NewTextBox(textFace)
		tb.SetText(item)

		// On the first rebuild of the initial item, trigger a mutation
		if item == "Alice" && !addedInMake {
			addedInMake = true
			l.Add("Bob") // This should queue a pending rebuild, not panic
		}

		return tb
	})

	// makeCount should be 2: first rebuild of ["Alice"], then
	// the queued rebuild of ["Alice", "Bob"] after make() for "Alice" completes.
	if makeCount != 2 {
		t.Fatalf("makeCount = %d, want 2 (initial rebuild + queued rebuild)", makeCount)
	}

	// Final panel should have both items
	panelChildren := panel.Children()
	if len(panelChildren) != 2 {
		t.Fatalf("panel.Children() = %d, want 2", len(panelChildren))
	}

	if l.Len() != 2 {
		t.Fatalf("l.Len() = %d, want 2", l.Len())
	}
}
