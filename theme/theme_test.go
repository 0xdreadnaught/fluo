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
	if Active().Name != "classic-light" {
		t.Fatalf("default = %q", Active().Name)
	}
}

func TestSetActiveSwaps(t *testing.T) {
	SetActive(Dark())
	defer SetActive(nil)
	if Active().Name != "classic-dark" {
		t.Fatalf("got %q", Active().Name)
	}
}

func TestLightDarkDiffer(t *testing.T) {
	l, d := Light(), Dark()
	if l.Color.ButtonFace == d.Color.ButtonFace {
		t.Fatal("ButtonFace identical")
	}
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
	a, b := Dark(), Dark()
	a.Color.Accent = render.RGB(1, 2, 3)
	if b.Color.Accent == a.Color.Accent {
		t.Fatal("constructors share state")
	}

	c, e := Light(), Light()
	c.Color.ButtonFace = render.RGB(4, 5, 6)
	if e.Color.ButtonFace == c.Color.ButtonFace {
		t.Fatal("constructors share state")
	}
}

func TestBevelWidth(t *testing.T) {
	if Light().Metric.BevelWidth != 2 {
		t.Fatalf("light BevelWidth = %v, want 2", Light().Metric.BevelWidth)
	}
	if Dark().Metric.BevelWidth != 2 {
		t.Fatalf("dark BevelWidth = %v, want 2", Dark().Metric.BevelWidth)
	}
}

func TestCornerRadiiAreZero(t *testing.T) {
	for _, th := range []*Theme{Light(), Dark()} {
		if th.Metric.CornerRadius != 0 {
			t.Fatalf("%s CornerRadius = %v, want 0", th.Name, th.Metric.CornerRadius)
		}
		if th.Metric.ControlCornerRadius != 0 {
			t.Fatalf("%s ControlCornerRadius = %v, want 0", th.Name, th.Metric.ControlCornerRadius)
		}
	}
}

func TestSelectionForegroundPresent(t *testing.T) {
	l := Light()
	d := Dark()

	// Verify the tokens exist and are set
	_ = l.Color.SelectionForeground
	_ = d.Color.SelectionForeground

	// Dark: RGB(255, 255, 255)
	expectedDark := render.RGB(255, 255, 255)
	if d.Color.SelectionForeground != expectedDark {
		t.Fatalf("dark SelectionForeground = %v, want %v", d.Color.SelectionForeground, expectedDark)
	}

	// Light: RGB(255, 255, 255)
	expectedLight := render.RGB(255, 255, 255)
	if l.Color.SelectionForeground != expectedLight {
		t.Fatalf("light SelectionForeground = %v, want %v", l.Color.SelectionForeground, expectedLight)
	}
}

func TestScrimBackgroundPresent(t *testing.T) {
	l := Light()
	d := Dark()

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

func TestScrimBackgroundLightDarkDiffer(t *testing.T) {
	l := Light()
	d := Dark()

	if l.Color.ScrimBackground == d.Color.ScrimBackground {
		t.Fatal("ScrimBackground should differ between light and dark")
	}
}

func TestAcrylicTintPresent(t *testing.T) {
	l := Light()
	d := Dark()

	// Dark: RGBA(58, 58, 58, 180)
	expectedDark := render.RGBA(58, 58, 58, 180)
	if d.Color.AcrylicTint != expectedDark {
		t.Fatalf("dark AcrylicTint = %v, want %v", d.Color.AcrylicTint, expectedDark)
	}

	// Light: RGBA(212, 208, 200, 180)
	expectedLight := render.RGBA(212, 208, 200, 180)
	if l.Color.AcrylicTint != expectedLight {
		t.Fatalf("light AcrylicTint = %v, want %v", l.Color.AcrylicTint, expectedLight)
	}
}

func TestSeverityTokensDiffer(t *testing.T) {
	for _, th := range []*Theme{Light(), Dark()} {
		c := th.Color
		pairs := []struct {
			name string
			a, b render.Color
		}{
			{"Info vs Success", c.SeverityInfo, c.SeveritySuccess},
			{"Info vs Warning", c.SeverityInfo, c.SeverityWarning},
			{"Info vs Error", c.SeverityInfo, c.SeverityError},
			{"Success vs Warning", c.SeveritySuccess, c.SeverityWarning},
			{"Success vs Error", c.SeveritySuccess, c.SeverityError},
			{"Warning vs Error", c.SeverityWarning, c.SeverityError},
		}
		for _, p := range pairs {
			if p.a == p.b {
				t.Fatalf("%s: %s severity tokens identical (%v)", th.Name, p.name, p.a)
			}
		}
	}
}

func TestAcrylicTintLightDarkDiffer(t *testing.T) {
	l := Light()
	d := Dark()

	if l.Color.AcrylicTint == d.Color.AcrylicTint {
		t.Fatal("AcrylicTint should differ between light and dark")
	}
}

func TestClassicToneFieldsSet(t *testing.T) {
	l, d := Light(), Dark()

	wantLight := ColorTokens{
		ButtonFace:       render.RGB(212, 208, 200),
		ButtonHighlight:  render.RGB(255, 255, 255),
		ButtonLight:      render.RGB(232, 228, 220),
		ButtonShadow:     render.RGB(128, 128, 128),
		ButtonDarkShadow: render.RGB(64, 64, 64),
		WindowWell:       render.RGB(255, 255, 255),
		WindowText:       render.RGB(0, 0, 0),
		GrayText:         render.RGB(128, 128, 128),
		Highlight:        render.RGB(0, 0, 128),
		HighlightText:    render.RGB(255, 255, 255),
		CaptionFrom:      render.RGB(10, 36, 106),
		CaptionTo:        render.RGB(166, 202, 240),
		CaptionText:      render.RGB(255, 255, 255),
		InactiveCaption:  render.RGB(128, 128, 128),
	}
	if l.Color.ButtonFace != wantLight.ButtonFace || l.Color.ButtonHighlight != wantLight.ButtonHighlight ||
		l.Color.ButtonLight != wantLight.ButtonLight || l.Color.ButtonShadow != wantLight.ButtonShadow ||
		l.Color.ButtonDarkShadow != wantLight.ButtonDarkShadow || l.Color.WindowWell != wantLight.WindowWell ||
		l.Color.WindowText != wantLight.WindowText || l.Color.GrayText != wantLight.GrayText ||
		l.Color.Highlight != wantLight.Highlight || l.Color.HighlightText != wantLight.HighlightText ||
		l.Color.CaptionFrom != wantLight.CaptionFrom || l.Color.CaptionTo != wantLight.CaptionTo ||
		l.Color.CaptionText != wantLight.CaptionText || l.Color.InactiveCaption != wantLight.InactiveCaption {
		t.Fatalf("light classic tokens = %+v, want %+v", l.Color, wantLight)
	}

	wantDark := ColorTokens{
		ButtonFace:       render.RGB(58, 58, 58),
		ButtonHighlight:  render.RGB(92, 92, 92),
		ButtonLight:      render.RGB(70, 70, 70),
		ButtonShadow:     render.RGB(32, 32, 32),
		ButtonDarkShadow: render.RGB(0, 0, 0),
		WindowWell:       render.RGB(30, 30, 30),
		WindowText:       render.RGB(240, 240, 240),
		GrayText:         render.RGB(110, 110, 110),
		Highlight:        render.RGB(42, 77, 143),
		HighlightText:    render.RGB(255, 255, 255),
		CaptionFrom:      render.RGB(16, 33, 74),
		CaptionTo:        render.RGB(58, 110, 165),
		CaptionText:      render.RGB(255, 255, 255),
		InactiveCaption:  render.RGB(42, 42, 42),
	}
	if d.Color.ButtonFace != wantDark.ButtonFace || d.Color.ButtonHighlight != wantDark.ButtonHighlight ||
		d.Color.ButtonLight != wantDark.ButtonLight || d.Color.ButtonShadow != wantDark.ButtonShadow ||
		d.Color.ButtonDarkShadow != wantDark.ButtonDarkShadow || d.Color.WindowWell != wantDark.WindowWell ||
		d.Color.WindowText != wantDark.WindowText || d.Color.GrayText != wantDark.GrayText ||
		d.Color.Highlight != wantDark.Highlight || d.Color.HighlightText != wantDark.HighlightText ||
		d.Color.CaptionFrom != wantDark.CaptionFrom || d.Color.CaptionTo != wantDark.CaptionTo ||
		d.Color.CaptionText != wantDark.CaptionText || d.Color.InactiveCaption != wantDark.InactiveCaption {
		t.Fatalf("dark classic tokens = %+v, want %+v", d.Color, wantDark)
	}
}
