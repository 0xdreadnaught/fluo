package controls

import (
	"math"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
)

// WrapPanel is a container widget that arranges children horizontally in rows,
// wrapping to a new row when there is not enough space. Gap specifies spacing
// between items in a row and between rows. Hidden children are measured and
// arranged but excluded from flow (no slot, no gap contribution).
type WrapPanel struct {
	core.Element

	children []core.Widget
	gap      float32
}

// row represents a single row of wrapped children.
type row struct {
	children []core.Widget
	width    float32
	height   float32
}

// NewWrapPanel returns a new WrapPanel.
func NewWrapPanel() *WrapPanel {
	return &WrapPanel{}
}

// Add appends children to this WrapPanel, re-parenting each one and
// invalidating measure.
func (w *WrapPanel) Add(children ...core.Widget) *WrapPanel {
	for _, child := range children {
		w.children = append(w.children, child)
		core.SetParent(child, w)
	}
	w.InvalidateMeasure()
	return w
}

// SetGap sets the spacing between items in a row and between rows.
// Layout-relevant: invalidates measure.
func (w *WrapPanel) SetGap(g float32) *WrapPanel {
	w.gap = g
	w.InvalidateMeasure()
	return w
}

// Children returns the slice of children.
func (w *WrapPanel) Children() []core.Widget {
	return w.children
}

// MeasureContent measures all children and computes desired size using
// horizontal flow: children are measured with (available.W, +Inf),
// wrapped to new rows when cursor+childW > available.W (first item never wraps).
// Desired = (max row width, sum of row heights + gaps).
func (w *WrapPanel) MeasureContent(available render.Size) render.Size {
	if len(w.children) == 0 {
		return render.Size{}
	}

	// Measure all children with available width and infinite height.
	infAvail := render.Size{W: available.W, H: float32(math.Inf(1))}
	for _, child := range w.children {
		core.MeasureWidget(child, infAvail)
	}

	// Flow children into rows
	rows := w.flowRows(available.W)

	// Compute desired size
	maxRowWidth := float32(0)
	totalHeight := float32(0)

	for i, r := range rows {
		if r.width > maxRowWidth {
			maxRowWidth = r.width
		}
		totalHeight += r.height

		// Add gap between rows (not after the last row)
		if i < len(rows)-1 {
			totalHeight += w.gap
		}
	}

	return render.Size{W: maxRowWidth, H: totalHeight}
}

// ArrangeContent arranges children in rows, recomputing flow against bounds.W.
// Each child slot = {rowX, rowY, childDesired.W, rowHeight}.
// Hidden children are arranged with an empty rect; core.ArrangeWidget handles the short-circuit.
func (w *WrapPanel) ArrangeContent(bounds render.Rect) {
	rows := w.flowRows(bounds.W)

	// Build a set of visible children for quick lookup
	visibleSet := make(map[core.Widget]bool)
	for _, r := range rows {
		for _, child := range r.children {
			visibleSet[child] = true
		}
	}

	y := bounds.Y
	for _, r := range rows {
		x := bounds.X
		for _, child := range r.children {
			desired := core.DesiredSizeOf(child)
			slot := render.Rect{
				X: x,
				Y: y,
				W: desired.W,
				H: r.height,
			}
			core.ArrangeWidget(child, slot)
			x += desired.W + w.gap
		}
		y += r.height + w.gap
	}

	// Arrange hidden children with empty rect
	for _, child := range w.children {
		if !visibleSet[child] {
			core.ArrangeWidget(child, render.Rect{})
		}
	}
}

// flowRows computes the rows of children given an available width.
func (w *WrapPanel) flowRows(availWidth float32) []row {
	if len(w.children) == 0 {
		return nil
	}

	var rows []row
	var currentRow row

	for _, child := range w.children {
		if !core.IsVisible(child) {
			continue
		}

		desired := core.DesiredSizeOf(child)

		// Check if we need to wrap
		if len(currentRow.children) > 0 &&
			currentRow.width+w.gap+desired.W > availWidth {
			// Wrap to next row
			rows = append(rows, currentRow)
			currentRow = row{}
		}

		// Add child to current row
		if len(currentRow.children) > 0 {
			currentRow.width += w.gap
		}
		currentRow.width += desired.W
		if desired.H > currentRow.height {
			currentRow.height = desired.H
		}
		currentRow.children = append(currentRow.children, child)
	}

	// Add the last row if it has children
	if len(currentRow.children) > 0 {
		rows = append(rows, currentRow)
	}

	return rows
}
