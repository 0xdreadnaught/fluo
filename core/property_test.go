package core

import "testing"

func TestPropertySetGet(t *testing.T) {
	var p Property[int]
	if p.Get() != 0 {
		t.Fatalf("zero value: got %d", p.Get())
	}
	p.Set(5)
	if p.Get() != 5 {
		t.Fatalf("got %d", p.Get())
	}
}

func TestPropertyNotify(t *testing.T) {
	var p Property[string]
	var gotOld, gotNew string
	calls := 0
	p.OnChange(func(o, n string) { gotOld, gotNew = o, n; calls++ })
	p.Set("a")
	if calls != 1 || gotOld != "" || gotNew != "a" {
		t.Fatalf("calls=%d old=%q new=%q", calls, gotOld, gotNew)
	}
	p.Set("a") // same value: no notify
	if calls != 1 {
		t.Fatalf("no-op Set notified: calls=%d", calls)
	}
}

func TestPropertyCancel(t *testing.T) {
	var p Property[int]
	calls := 0
	cancel := p.OnChange(func(_, _ int) { calls++ })
	p.Set(1)
	cancel()
	p.Set(2)
	if calls != 1 {
		t.Fatalf("calls=%d after cancel", calls)
	}
	// cancel is idempotent
	cancel()
}

func TestPropertyMultipleSubs(t *testing.T) {
	var p Property[int]
	a, b := 0, 0
	p.OnChange(func(_, _ int) { a++ })
	p.OnChange(func(_, _ int) { b++ })
	p.Set(7)
	if a != 1 || b != 1 {
		t.Fatalf("a=%d b=%d", a, b)
	}
}

// TestPropertyNotifiesInRegistrationOrder locks the deterministic ordering:
// subscribers fire in the order they were registered, never in map order.
func TestPropertyNotifiesInRegistrationOrder(t *testing.T) {
	var p Property[int]
	var order []int
	for i := 0; i < 6; i++ {
		i := i
		p.OnChange(func(_, _ int) { order = append(order, i) })
	}
	p.Set(1)
	for i, v := range order {
		if v != i {
			t.Fatalf("notification order = %v, want ascending registration order", order)
		}
	}
	if len(order) != 6 {
		t.Fatalf("saw %d notifications, want 6", len(order))
	}
}

// TestPropertyReentrantCoercionSettlesDeterministically is the C2 core case: a
// validator subscriber that reentrantly Sets a coerced value, plus a view
// subscriber, must leave BOTH the model and the view at the coerced value —
// regardless of which subscriber registered first. Before the fix, the outer
// notification could deliver its now-stale pair to the view after the
// reentrant Set had already settled a different value, and the winner depended
// on Go map iteration order.
func TestPropertyReentrantCoercionSettlesDeterministically(t *testing.T) {
	for _, validatorFirst := range []bool{true, false} {
		var p Property[int]
		var view int

		validator := func(_, n int) {
			if n > 10 {
				p.Set(10) // coerce: clamp to 10
			}
		}
		observe := func(_, n int) { view = n }

		if validatorFirst {
			p.OnChange(validator)
			p.OnChange(observe)
		} else {
			p.OnChange(observe)
			p.OnChange(validator)
		}

		p.Set(50)

		if p.Get() != 10 {
			t.Fatalf("validatorFirst=%v: model=%d, want 10", validatorFirst, p.Get())
		}
		if view != 10 {
			t.Fatalf("validatorFirst=%v: view=%d, want 10 (view must agree with model)", validatorFirst, view)
		}
	}
}

// TestPropertyCancelDuringNotify exercises cancel semantics under the ordered-
// slice representation: a subscriber that cancels another mid-notification
// keeps that other from firing (this notification and every later one), and
// the surviving subscribers still fire in order. It also confirms the canceled
// entry does not resurrect on a later Set (tombstone compaction is clean).
func TestPropertyCancelDuringNotify(t *testing.T) {
	var p Property[int]
	var aCalls, cCalls int
	var cancelB func()

	p.OnChange(func(_, _ int) { aCalls++; cancelB() })
	cancelB = p.OnChange(func(_, _ int) { t.Fatal("B was canceled before it should have fired") })
	p.OnChange(func(_, _ int) { cCalls++ })

	p.Set(1)
	if aCalls != 1 || cCalls != 1 {
		t.Fatalf("after first Set: aCalls=%d cCalls=%d, want 1/1", aCalls, cCalls)
	}

	p.Set(2)
	if aCalls != 2 || cCalls != 2 {
		t.Fatalf("after second Set: aCalls=%d cCalls=%d, want 2/2 (B must stay gone)", aCalls, cCalls)
	}
}

// TestPropertyReentrantSetOnlyDeliversFinalValue guards echo-safety and the
// supersede rule together: the equality gate stops a reentrant Set of the same
// value from looping, and a reentrant Set of a new value supersedes the outer
// one so no subscriber is ever handed the intermediate value after the fact.
func TestPropertyReentrantSetOnlyDeliversFinalValue(t *testing.T) {
	var p Property[int]
	var seen []int
	coerced := false

	p.OnChange(func(_, n int) {
		if !coerced && n == 1 {
			coerced = true
			p.Set(2) // supersede the in-flight (0,1) notification
		}
	})
	p.OnChange(func(_, n int) { seen = append(seen, n) })

	p.Set(1)

	// The observer must never see the stale 1 delivered after 2 settled.
	if len(seen) == 0 || seen[len(seen)-1] != 2 {
		t.Fatalf("observer saw %v, want it to end at 2 (the coerced value)", seen)
	}
	for _, v := range seen {
		if v == 1 {
			t.Fatalf("observer saw the superseded intermediate value 1: %v", seen)
		}
	}
	if p.Get() != 2 {
		t.Fatalf("model=%d, want 2", p.Get())
	}
}
