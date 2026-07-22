package controls

import (
	"math"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
)

// Orientation specifies the direction in which a StackPanel stacks its children.
type Orientation uint8

const (
	// Vertical stacks children top to bottom.
	Vertical Orientation = iota
	// Horizontal stacks children left to right.
	Horizontal
)

// StackPanel is a container widget that arranges children in a row (Horizontal)
// or column (Vertical) with optional spacing between them.
type StackPanel struct {
	core.Element

	orient   Orientation
	children []core.Widget
	gap      float32
}

// NewStackPanel returns a new StackPanel with the given orientation.
func NewStackPanel(o Orientation) *StackPanel {
	return &StackPanel{orient: o}
}

// Add appends children to this StackPanel, re-parenting each one and
// invalidating measure.
func (s *StackPanel) Add(children ...core.Widget) *StackPanel {
	for _, child := range children {
		s.children = append(s.children, child)
		core.SetParent(child, s)
	}
	s.InvalidateMeasure()
	return s
}

// SetGap sets the spacing between adjacent children. Layout-relevant:
// invalidates measure.
func (s *StackPanel) SetGap(g float32) *StackPanel {
	s.gap = g
	s.InvalidateMeasure()
	return s
}

// Children returns the slice of children.
func (s *StackPanel) Children() []core.Widget {
	return s.children
}

// MeasureContent measures all children and computes the total size.
// For Vertical: children are measured with (available.W, +Inf);
// desired size is (max child W, sum child H + gaps between visible children).
// For Horizontal: children are measured with (+Inf, available.H);
// desired size is (sum child W + gaps between visible children, max child H).
func (s *StackPanel) MeasureContent(available render.Size) render.Size {
	if s.orient == Vertical {
		return s.measureVertical(available)
	}
	return s.measureHorizontal(available)
}

func (s *StackPanel) measureVertical(available render.Size) render.Size {
	// Measure each child with available.W and infinite height.
	infAvail := render.Size{W: available.W, H: float32(math.Inf(1))}

	maxW := float32(0)
	totalH := float32(0)
	lastContributedIdx := -1

	for i, child := range s.children {
		core.MeasureWidget(child, infAvail)
		desired := core.DesiredSizeOf(child)

		// Track maximum width.
		if desired.W > maxW {
			maxW = desired.W
		}

		// Only count children with non-zero height (visible children).
		if desired.H > 0 {
			// Add gap before this child if there was a previous visible child.
			if lastContributedIdx >= 0 {
				totalH += s.gap
			}
			totalH += desired.H
			lastContributedIdx = i
		}
	}

	return render.Size{W: maxW, H: totalH}
}

func (s *StackPanel) measureHorizontal(available render.Size) render.Size {
	// Measure each child with infinite width and available.H.
	infAvail := render.Size{W: float32(math.Inf(1)), H: available.H}

	totalW := float32(0)
	maxH := float32(0)
	lastContributedIdx := -1

	for i, child := range s.children {
		core.MeasureWidget(child, infAvail)
		desired := core.DesiredSizeOf(child)

		// Track maximum height.
		if desired.H > maxH {
			maxH = desired.H
		}

		// Only count children with non-zero width (visible children).
		if desired.W > 0 {
			// Add gap before this child if there was a previous visible child.
			if lastContributedIdx >= 0 {
				totalW += s.gap
			}
			totalW += desired.W
			lastContributedIdx = i
		}
	}

	return render.Size{W: totalW, H: maxH}
}

// ArrangeContent arranges children in the stack.
// For Vertical: each child gets slot {bounds.X, y, bounds.W, childDesired.H};
// the cross-axis alignment is handled by ArrangeWidget.
// For Horizontal: each child gets slot {x, bounds.Y, childDesired.W, bounds.H}.
func (s *StackPanel) ArrangeContent(bounds render.Rect) {
	if s.orient == Vertical {
		s.arrangeVertical(bounds)
	} else {
		s.arrangeHorizontal(bounds)
	}
}

func (s *StackPanel) arrangeVertical(bounds render.Rect) {
	y := bounds.Y
	lastContributedIdx := -1

	for i, child := range s.children {
		desired := core.DesiredSizeOf(child)

		// Only arrange children with non-zero height (visible children).
		if desired.H > 0 {
			// Add gap before this child if there was a previous visible child.
			if lastContributedIdx >= 0 {
				y += s.gap
			}

			// Arrange the child with full width of bounds and its desired height.
			slot := render.Rect{X: bounds.X, Y: y, W: bounds.W, H: desired.H}
			core.ArrangeWidget(child, slot)

			y += desired.H
			lastContributedIdx = i
		}
	}
}

func (s *StackPanel) arrangeHorizontal(bounds render.Rect) {
	x := bounds.X
	lastContributedIdx := -1

	for i, child := range s.children {
		desired := core.DesiredSizeOf(child)

		// Only arrange children with non-zero width (visible children).
		if desired.W > 0 {
			// Add gap before this child if there was a previous visible child.
			if lastContributedIdx >= 0 {
				x += s.gap
			}

			// Arrange the child with its desired width and full height of bounds.
			slot := render.Rect{X: x, Y: bounds.Y, W: desired.W, H: bounds.H}
			core.ArrangeWidget(child, slot)

			x += desired.W
			lastContributedIdx = i
		}
	}
}
