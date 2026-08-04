package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/input"
)

// exercises typeAhead.feed directly — no widget, no clock, just the matching
// contract (see typeahead.go). Times are passed explicitly so the reset window
// is deterministic.

func taLabel(items []string) func(int) string {
	return func(i int) string { return items[i] }
}

func TestTypeAheadAccumulatesPrefix(t *testing.T) {
	items := []string{"apple", "apricot", "banana", "cherry"}
	label := taLabel(items)
	var ta typeAhead

	if i, ok := ta.feed(1.0, 'a', len(items), -1, label); !ok || i != 0 {
		t.Fatalf("feed 'a' = %d,%v, want 0,true (apple)", i, ok)
	}
	// 'p' within the window extends to "ap"; apple still matches, so selection
	// holds rather than jumping.
	if i, ok := ta.feed(1.1, 'p', len(items), 0, label); !ok || i != 0 {
		t.Fatalf("feed 'p' = %d,%v, want 0,true (apple still matches 'ap')", i, ok)
	}
	// 'r' extends to "apr" — apple no longer matches, apricot does.
	if i, ok := ta.feed(1.2, 'r', len(items), 0, label); !ok || i != 1 {
		t.Fatalf("feed 'r' = %d,%v, want 1,true (apricot)", i, ok)
	}
}

func TestTypeAheadRepeatCharCycles(t *testing.T) {
	items := []string{"apple", "apricot", "avocado", "banana"}
	label := taLabel(items)
	var ta typeAhead

	cur := -1
	for step, want := range []int{0, 1, 2, 0} { // 'a' four times: apple→apricot→avocado→wrap
		i, ok := ta.feed(1.0+float64(step)*0.1, 'a', len(items), cur, label)
		if !ok || i != want {
			t.Fatalf("repeat 'a' #%d = %d,%v, want %d,true", step+1, i, ok, want)
		}
		cur = i
	}
}

func TestTypeAheadWindowResets(t *testing.T) {
	items := []string{"apple", "apricot"}
	label := taLabel(items)
	var ta typeAhead

	ta.feed(1.0, 'a', len(items), -1, label) // buffer "a"
	// 'p' AFTER the reset window: the buffer resets to "p" (not "ap"), and no
	// item starts with "p", so there is no match.
	if i, ok := ta.feed(1.0+typeAheadResetSeconds+0.1, 'p', len(items), 0, label); ok {
		t.Fatalf("feed 'p' after the window = %d,%v, want -1,false (buffer reset)", i, ok)
	}
}

func TestTypeAheadCaseInsensitive(t *testing.T) {
	items := []string{"Apple", "Banana"}
	label := taLabel(items)
	var ta typeAhead

	if i, ok := ta.feed(1.0, 'a', len(items), -1, label); !ok || i != 0 {
		t.Fatalf("lowercase 'a' vs 'Apple' = %d,%v, want 0,true", i, ok)
	}
}

func TestTypeAheadNoMatchLeavesSelection(t *testing.T) {
	items := []string{"apple", "banana"}
	if i, ok := (&typeAhead{}).feed(1.0, 'z', len(items), 0, taLabel(items)); ok || i != -1 {
		t.Fatalf("feed 'z' = %d,%v, want -1,false (no z item, selection untouched)", i, ok)
	}
}

func TestTypeAheadEmptyList(t *testing.T) {
	if i, ok := (&typeAhead{}).feed(1.0, 'a', 0, -1, func(int) string { return "" }); ok || i != -1 {
		t.Fatalf("feed into empty list = %d,%v, want -1,false", i, ok)
	}
}

// --- per-widget integration (one keystroke proves the OnKey wiring) -----

func TestComboBoxTypeAheadSelects(t *testing.T) {
	combo, _, r := newTestCombo(t, []string{"Apple", "Banana", "Cherry"})
	r.Focus(combo)

	r.KeyDown(input.Key(0), 'b', 0) // char event, no Ctrl
	if got := combo.SelectedIndex(); got != 1 {
		t.Fatalf("ComboBox type-ahead 'b': SelectedIndex = %d, want 1 (Banana)", got)
	}
}

func TestListViewTypeAheadSelects(t *testing.T) {
	l := NewListView(nil, newFakeListItems("alpha", "bravo", "charlie")).SetRowHeight(20)
	r := input.NewRouter()
	r.SetRoot(l)
	layoutListView(l, 0, 0, 200, 200)
	r.Focus(l)

	r.KeyDown(input.Key(0), 'c', 0)
	if l.selected != 2 {
		t.Fatalf("ListView type-ahead 'c': selected = %d, want 2 (charlie)", l.selected)
	}
}

func TestTreeViewTypeAheadSelects(t *testing.T) {
	tv := NewTreeView(nil, NewTreeNode("apple"), NewTreeNode("banana"), NewTreeNode("cherry"))
	r := input.NewRouter()
	r.SetRoot(tv)
	layoutTreeView(tv, 0, 0, 200, 200)
	r.Focus(tv)

	r.KeyDown(input.Key(0), 'b', 0)
	if tv.selected == nil || tv.selected.Label != "banana" {
		t.Fatalf("TreeView type-ahead 'b': selected = %v, want the banana node", tv.selected)
	}
}
