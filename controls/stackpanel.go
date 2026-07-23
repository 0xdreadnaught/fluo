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

// Clear removes all children, detaching each (core.SetParent(child, nil), so
// none of them keep this panel recorded as their layout parent) and
// invalidates measure. Returns s for chaining, matching Add/SetGap. A no-op
// (still invalidates measure) when the panel already has no children.
func (s *StackPanel) Clear() *StackPanel {
	for _, child := range s.children {
		core.SetParent(child, nil)
	}
	s.children = nil
	s.InvalidateMeasure()
	return s
}

// Children returns a copy of the children slice; mutating it does not
// affect the panel.
func (s *StackPanel) Children() []core.Widget {
	out := make([]core.Widget, len(s.children))
	copy(out, s.children)
	return out
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

		// Only count visible children for gap/extent contribution.
		if core.IsVisible(child) {
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

		// Only count visible children for gap/extent contribution.
		if core.IsVisible(child) {
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

		// Add gap before this child if there was a previous visible child and this child is visible.
		if core.IsVisible(child) && lastContributedIdx >= 0 {
			y += s.gap
		}

		// Arrange all children; hidden children short-circuit cheaply inside ArrangeWidget.
		slot := render.Rect{X: bounds.X, Y: y, W: bounds.W, H: desired.H}
		core.ArrangeWidget(child, slot)

		// Track y position and last visible child for gap calculation.
		if core.IsVisible(child) {
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

		// Add gap before this child if there was a previous visible child and this child is visible.
		if core.IsVisible(child) && lastContributedIdx >= 0 {
			x += s.gap
		}

		// Arrange all children; hidden children short-circuit cheaply inside ArrangeWidget.
		slot := render.Rect{X: x, Y: bounds.Y, W: desired.W, H: bounds.H}
		core.ArrangeWidget(child, slot)

		// Track x position and last visible child for gap calculation.
		if core.IsVisible(child) {
			x += desired.W
			lastContributedIdx = i
		}
	}
}
