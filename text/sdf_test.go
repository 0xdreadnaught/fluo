package text

import (
	"image"
	"image/color"
	"testing"
)

func TestSDFFromMask(t *testing.T) {
	// synthetic 32×32 mask with a filled 16×16 square centered
	m := image.NewAlpha(image.Rect(0, 0, 32, 32))
	for y := 8; y < 24; y++ {
		for x := 8; x < 24; x++ {
			m.SetAlpha(x, y, color.Alpha{A: 255})
		}
	}
	s := sdfFromMask(m)
	if c := s.AlphaAt(16, 16).A; c < 200 {
		t.Errorf("center=%d, want >200", c)
	}
	if c := s.AlphaAt(1, 1).A; c > 60 {
		t.Errorf("far corner=%d, want <60", c)
	}
	e := s.AlphaAt(8, 16).A // on the edge
	if e < 108 || e > 148 {
		t.Errorf("edge=%d, want ~128", e)
	}
}

func TestSDFFromMaskEmpty(t *testing.T) {
	m := image.NewAlpha(image.Rect(0, 0, 8, 8))
	s := sdfFromMask(m)
	if s.Bounds() != m.Bounds() {
		t.Errorf("bounds = %v, want %v", s.Bounds(), m.Bounds())
	}
	for _, a := range s.Pix {
		if a != 0 {
			t.Errorf("expected all-zero output for empty mask, got %d", a)
			break
		}
	}
}
