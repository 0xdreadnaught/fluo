package controls

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
)

// Dock specifies the edge to which a DockPanel child is docked.
type Dock uint8

const (
	// DockLeft docks a child to the left edge.
	DockLeft Dock = iota
	// DockTop docks a child to the top edge.
	DockTop
	// DockRight docks a child to the right edge.
	DockRight
	// DockBottom docks a child to the bottom edge.
	DockBottom
)

// dockItem represents a child widget and its docking position.
type dockItem struct {
	child core.Widget
	dock  Dock
}

// DockPanel is a container widget that arranges children along its edges
// using the Dock enumeration. The last child can optionally fill the remaining space.
type DockPanel struct {
	core.Element

	items     []dockItem
	lastFills bool
}

// NewDockPanel returns a new DockPanel with lastFills defaulting to true.
func NewDockPanel() *DockPanel {
	return &DockPanel{lastFills: true}
}

// Add appends a child to this DockPanel with the given Dock value,
// re-parenting it and invalidating measure.
func (d *DockPanel) Add(w core.Widget, dock Dock) *DockPanel {
	d.items = append(d.items, dockItem{child: w, dock: dock})
	core.SetParent(w, d)
	d.InvalidateMeasure()
	return d
}

// SetLastChildFill sets whether the last child fills the remaining space.
// Layout-relevant: invalidates measure.
func (d *DockPanel) SetLastChildFill(v bool) *DockPanel {
	d.lastFills = v
	d.InvalidateMeasure()
	return d
}

// Children returns the slice of child widgets.
func (d *DockPanel) Children() []core.Widget {
	children := make([]core.Widget, len(d.items))
	for i, item := range d.items {
		children[i] = item.child
	}
	return children
}

// MeasureContent measures all children and computes the desired size using
// WPF accumulation: for Left/Right items track maxH = max(maxH, usedH+d.H),
// usedW += d.W; Top/Bottom mirrored; final desired = (max(maxW, usedW), max(maxH, usedH)).
func (d *DockPanel) MeasureContent(available render.Size) render.Size {
	// Measure all children with available space; hidden children short-circuit.
	for _, item := range d.items {
		core.MeasureWidget(item.child, available)
	}

	// First pass: accumulate total used width and height
	usedW := float32(0)  // sum of widths of Left/Right items
	usedH := float32(0)  // sum of heights of Top/Bottom items

	for _, item := range d.items {
		if !core.IsVisible(item.child) {
			continue
		}

		desired := core.DesiredSizeOf(item.child)

		switch item.dock {
		case DockLeft, DockRight:
			usedW += desired.W
		case DockTop, DockBottom:
			usedH += desired.H
		}
	}

	// Second pass: compute max dimensions considering both dimensions
	maxH := float32(0)   // max height needed (considering Left/Right items)
	maxW := float32(0)   // max width needed (considering Top/Bottom items)

	for _, item := range d.items {
		if !core.IsVisible(item.child) {
			continue
		}

		desired := core.DesiredSizeOf(item.child)

		switch item.dock {
		case DockLeft, DockRight:
			// For Left/Right items: track max height needed considering Top/Bottom items
			if usedH+desired.H > maxH {
				maxH = usedH + desired.H
			}
		case DockTop, DockBottom:
			// For Top/Bottom items: track max width needed considering Left/Right items
			if usedW+desired.W > maxW {
				maxW = usedW + desired.W
			}
		}
	}

	w := maxW
	if usedW > w {
		w = usedW
	}
	h := maxH
	if usedH > h {
		h = usedH
	}

	return render.Size{W: w, H: h}
}

// ArrangeContent arranges children by carving space from the bounds per dock edge.
// The last visible child gets the whole remainder when lastFills is true.
func (d *DockPanel) ArrangeContent(bounds render.Rect) {
	remaining := bounds

	// Find the last visible child index
	lastVisibleIdx := -1
	for i, item := range d.items {
		if core.IsVisible(item.child) {
			lastVisibleIdx = i
		}
	}

	for i, item := range d.items {
		desired := core.DesiredSizeOf(item.child)
		isLast := i == lastVisibleIdx

		var slot render.Rect

		if isLast && d.lastFills {
			// Last visible child fills the entire remaining space
			slot = remaining
		} else {
			// Carve space from the appropriate edge
			switch item.dock {
			case DockLeft:
				slot = render.Rect{
					X: remaining.X,
					Y: remaining.Y,
					W: desired.W,
					H: remaining.H,
				}
				remaining.X += desired.W
				remaining.W -= desired.W

			case DockRight:
				slot = render.Rect{
					X: remaining.X + remaining.W - desired.W,
					Y: remaining.Y,
					W: desired.W,
					H: remaining.H,
				}
				remaining.W -= desired.W

			case DockTop:
				slot = render.Rect{
					X: remaining.X,
					Y: remaining.Y,
					W: remaining.W,
					H: desired.H,
				}
				remaining.Y += desired.H
				remaining.H -= desired.H

			case DockBottom:
				slot = render.Rect{
					X: remaining.X,
					Y: remaining.Y + remaining.H - desired.H,
					W: remaining.W,
					H: desired.H,
				}
				remaining.H -= desired.H
			}
		}

		core.ArrangeWidget(item.child, slot)
	}
}
