package render

import "testing"

func TestRectContains(t *testing.T) {
	r := Rect{10, 10, 20, 20}
	for _, tc := range []struct {
		p    Point
		want bool
	}{
		{Point{10, 10}, true}, {Point{29.9, 29.9}, true},
		{Point{30, 30}, false}, {Point{9, 15}, false},
	} {
		if got := r.Contains(tc.p); got != tc.want {
			t.Errorf("%v: got %v", tc.p, got)
		}
	}
}

func TestRectIntersect(t *testing.T) {
	a := Rect{0, 0, 10, 10}
	if got := a.Intersect(Rect{5, 5, 10, 10}); got != (Rect{5, 5, 5, 5}) {
		t.Errorf("got %v", got)
	}
	if got := a.Intersect(Rect{20, 20, 5, 5}); !got.Empty() {
		t.Errorf("want empty, got %v", got)
	}
}

func TestRectInset(t *testing.T) {
	got := Rect{0, 0, 100, 50}.Inset(Thickness{5, 10, 15, 20})
	if got != (Rect{5, 10, 80, 20}) {
		t.Errorf("got %v", got)
	}
}

func TestRGB(t *testing.T) {
	c := RGB(255, 0, 128)
	if c.R != 1 || c.A != 1 || c.B < 0.5 || c.B > 0.51 {
		t.Errorf("got %+v", c)
	}
}
