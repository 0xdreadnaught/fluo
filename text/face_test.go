package text

import (
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

func TestMeasure(t *testing.T) {
	f, _ := Load(goregular.TTF)
	fa := NewFace(f, 16)
	m1, m2 := fa.Measure("M"), fa.Measure("MM")
	if m1.W <= 0 || m2.W <= m1.W {
		t.Errorf("M=%v MM=%v", m1, m2)
	}
	if lh := fa.LineHeight(); lh < 16 || lh > 26 {
		t.Errorf("LineHeight=%v", lh)
	}
	if fa.Measure("").W != 0 {
		t.Error("empty string width != 0")
	}
}
