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

func TestSelectionForegroundPresent(t *testing.T) {
	l := FluentLight()
	d := FluentDark()

	// Verify the tokens exist and are set
	_ = l.Color.SelectionForeground
	_ = d.Color.SelectionForeground

	// Dark: RGB(255, 255, 255)
	expectedDark := render.RGB(255, 255, 255)
	if d.Color.SelectionForeground != expectedDark {
		t.Fatalf("dark SelectionForeground = %v, want %v", d.Color.SelectionForeground, expectedDark)
	}

	// Light: RGB(26, 26, 26)
	expectedLight := render.RGB(26, 26, 26)
	if l.Color.SelectionForeground != expectedLight {
		t.Fatalf("light SelectionForeground = %v, want %v", l.Color.SelectionForeground, expectedLight)
	}
}

func TestScrimBackgroundPresent(t *testing.T) {
	l := FluentLight()
	d := FluentDark()

	// Verify the tokens exist and are set
	_ = l.Color.ScrimBackground
	_ = d.Color.ScrimBackground

	// Dark: RGBA(0, 0, 0, 120)
	expectedDark := render.RGBA(0, 0, 0, 120)
	if d.Color.ScrimBackground != expectedDark {
		t.Fatalf("dark ScrimBackground = %v, want %v", d.Color.ScrimBackground, expectedDark)
	}

	// Light: RGBA(0, 0, 0, 90)
	expectedLight := render.RGBA(0, 0, 0, 90)
	if l.Color.ScrimBackground != expectedLight {
		t.Fatalf("light ScrimBackground = %v, want %v", l.Color.ScrimBackground, expectedLight)
	}
}

func TestSelectionForegroundLightDarkDiffer(t *testing.T) {
	l := FluentLight()
	d := FluentDark()

	if l.Color.SelectionForeground == d.Color.SelectionForeground {
		t.Fatal("SelectionForeground should differ between light and dark")
	}
}

func TestScrimBackgroundLightDarkDiffer(t *testing.T) {
	l := FluentLight()
	d := FluentDark()

	if l.Color.ScrimBackground == d.Color.ScrimBackground {
		t.Fatal("ScrimBackground should differ between light and dark")
	}
}
