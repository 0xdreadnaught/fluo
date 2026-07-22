package theme

import (
	"testing"

	"github.com/0xdreadnaught/fluo/render"
)

func TestActiveNeverNil(t *testing.T) {
	SetActive(nil)
	if Active() == nil {
		t.Fatal("Active returned nil")
	}
	if Active().Name != "fluent-dark" {
		t.Fatalf("default = %q", Active().Name)
	}
}

func TestSetActiveSwaps(t *testing.T) {
	SetActive(FluentLight())
	defer SetActive(nil)
	if Active().Name != "fluent-light" {
		t.Fatalf("got %q", Active().Name)
	}
}

func TestLightDarkDiffer(t *testing.T) {
	l, d := FluentLight(), FluentDark()
	if l.Color.WindowBackground == d.Color.WindowBackground {
		t.Fatal("backgrounds identical")
	}
	if l.Color.TextPrimary == d.Color.TextPrimary {
		t.Fatal("text identical")
	}
	if l.Color.SelectionBackground == d.Color.SelectionBackground {
		t.Fatal("selection background should differ between variants")
	}
	if l.Metric != d.Metric {
		t.Fatal("metrics should be SHARED between variants")
	}
	if l.Type != d.Type {
		t.Fatal("type ramp should be shared")
	}
}

func TestConstructorsReturnFreshCopies(t *testing.T) {
	a, b := FluentDark(), FluentDark()
	a.Color.Accent = render.RGB(1, 2, 3)
	if b.Color.Accent == a.Color.Accent {
		t.Fatal("constructors share state")
	}
}
