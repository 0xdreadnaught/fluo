package render

// Color represents an RGBA color with components in the range [0, 1].
type Color struct {
	R, G, B, A float32
}

// RGB creates a color from 8-bit red, green, and blue components with full alpha.
func RGB(r, g, b uint8) Color {
	return RGBA(r, g, b, 255)
}

// RGBA creates a color from 8-bit red, green, blue, and alpha components.
func RGBA(r, g, b, a uint8) Color {
	return Color{
		R: float32(r) / 255,
		G: float32(g) / 255,
		B: float32(b) / 255,
		A: float32(a) / 255,
	}
}

// Point represents a 2D point with X and Y coordinates.
type Point struct {
	X, Y float32
}

// Size represents a 2D size with width and height.
type Size struct {
	W, H float32
}

// Rect represents a rectangle with top-left position and size.
type Rect struct {
	X, Y, W, H float32
}

// Right returns the x-coordinate of the right edge.
func (r Rect) Right() float32 {
	return r.X + r.W
}

// Bottom returns the y-coordinate of the bottom edge.
func (r Rect) Bottom() float32 {
	return r.Y + r.H
}

// Contains checks if a point is inside the rectangle (half-open interval [X, Right) × [Y, Bottom)).
func (r Rect) Contains(p Point) bool {
	return p.X >= r.X && p.X < r.Right() && p.Y >= r.Y && p.Y < r.Bottom()
}

// Intersect computes the intersection of two rectangles.
// Returns an empty rectangle (W/H=0) if the rectangles do not overlap.
func (r Rect) Intersect(o Rect) Rect {
	left := r.X
	if o.X > left {
		left = o.X
	}
	top := r.Y
	if o.Y > top {
		top = o.Y
	}
	right := r.Right()
	if o.Right() < right {
		right = o.Right()
	}
	bottom := r.Bottom()
	if o.Bottom() < bottom {
		bottom = o.Bottom()
	}

	w := right - left
	h := bottom - top
	if w <= 0 || h <= 0 {
		return Rect{}
	}
	return Rect{left, top, w, h}
}

// Inflate returns a new rectangle with all sides grown by the given amount.
func (r Rect) Inflate(d float32) Rect {
	return Rect{
		X: r.X - d,
		Y: r.Y - d,
		W: r.W + 2*d,
		H: r.H + 2*d,
	}
}

// Empty checks if the rectangle has zero or negative width or height.
func (r Rect) Empty() bool {
	return r.W <= 0 || r.H <= 0
}

// Thickness represents insets on all four sides.
type Thickness struct {
	Left, Top, Right, Bottom float32
}

// Uniform creates a thickness with the same value on all sides.
func Uniform(v float32) Thickness {
	return Thickness{v, v, v, v}
}

// Inset returns a new rectangle inset by the given thickness.
func (r Rect) Inset(t Thickness) Rect {
	return Rect{
		X: r.X + t.Left,
		Y: r.Y + t.Top,
		W: r.W - t.Left - t.Right,
		H: r.H - t.Top - t.Bottom,
	}
}

