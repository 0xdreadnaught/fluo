package input_test

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
)

// compositionProbe is a minimal input.CompositionHandler: a bare
// core.Element that records every CompositionEvent delivered to it, plus
// the AcceptsFocus toggle every focus-dependent test in this package needs
// to actually get it focused. Deliberately its own type rather than an
// extension of router_test.go's probe — Task 6 Phase B's dispatch tests
// don't need any of probe's pointer/hit-test machinery, just focus +
// composition.
type compositionProbe struct {
	core.Element
	focusable bool
	events    []input.CompositionEvent
}

func (c *compositionProbe) AcceptsFocus() bool { return c.focusable }

func (c *compositionProbe) OnComposition(e input.CompositionEvent) {
	c.events = append(c.events, e)
}

// nonCompositionProbe is a focusable widget that does NOT implement
// CompositionHandler — TestCompositionDispatch_NotACompositionHandler's
// no-op case.
type nonCompositionProbe struct {
	core.Element
}

func (nonCompositionProbe) AcceptsFocus() bool { return true }

func TestCompositionUpdateDispatchesToFocusedHandler(t *testing.T) {
	r := input.NewRouter()
	c := &compositionProbe{focusable: true}
	r.SetRoot(c)
	r.Focus(c)

	r.CompositionUpdate("か", 1)

	if len(c.events) != 1 {
		t.Fatalf("events = %d, want 1", len(c.events))
	}
	got := c.events[0]
	if !got.Active || got.Preedit != "か" || got.CaretPos != 1 || got.Canceled {
		t.Fatalf("event = %+v, want Active=true Preedit=%q CaretPos=1 Canceled=false", got, "か")
	}
}

func TestCompositionCommitDispatchesToFocusedHandler(t *testing.T) {
	r := input.NewRouter()
	c := &compositionProbe{focusable: true}
	r.SetRoot(c)
	r.Focus(c)

	r.CompositionCommit("日本語")

	if len(c.events) != 1 {
		t.Fatalf("events = %d, want 1", len(c.events))
	}
	got := c.events[0]
	if got.Active || got.Canceled || got.Committed != "日本語" {
		t.Fatalf("event = %+v, want Active=false Canceled=false Committed=%q", got, "日本語")
	}
}

func TestCompositionCancelDispatchesToFocusedHandler(t *testing.T) {
	r := input.NewRouter()
	c := &compositionProbe{focusable: true}
	r.SetRoot(c)
	r.Focus(c)

	r.CompositionCancel()

	if len(c.events) != 1 {
		t.Fatalf("events = %d, want 1", len(c.events))
	}
	got := c.events[0]
	if got.Active || !got.Canceled || got.Committed != "" {
		t.Fatalf("event = %+v, want Active=false Canceled=true Committed=\"\"", got)
	}
}

func TestCompositionDispatch_NothingFocused(t *testing.T) {
	r := input.NewRouter()
	c := &compositionProbe{focusable: true}
	r.SetRoot(c)
	// Deliberately never focused.

	r.CompositionUpdate("x", 0)
	r.CompositionCommit("x")
	r.CompositionCancel()

	if len(c.events) != 0 {
		t.Fatalf("events = %d, want 0 (nothing focused)", len(c.events))
	}
}

// TestCompositionDispatch_NotACompositionHandler is the "focused widget
// isn't composition-aware" no-op case — mirrors FocusedCaretRect's own
// "focused but doesn't implement the optional interface" branch. Nothing to
// assert beyond "no panic", since nonCompositionProbe records nothing.
func TestCompositionDispatch_NotACompositionHandler(t *testing.T) {
	r := input.NewRouter()
	w := &nonCompositionProbe{}
	r.SetRoot(w)
	r.Focus(w)

	r.CompositionUpdate("x", 0)
	r.CompositionCommit("x")
	r.CompositionCancel()
}
