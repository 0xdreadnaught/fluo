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
