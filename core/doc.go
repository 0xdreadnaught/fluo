// Package core is fluo's layout engine and widget foundation. Widget is the
// interface every control satisfies by embedding Element (which supplies
// margin/size/alignment/visibility state and a zero-value-valid default
// implementation of MeasureContent/ArrangeContent/Render/Children);
// MeasureWidget and ArrangeWidget are the only entry points that run the
// measure and arrange passes (available space in, desired size out; a final
// rect in, absolute bounds out), and RenderWidget draws a widget and its
// children in the correct order, honoring the optional ClipProvider and
// OverlayRenderer interfaces. DesiredSizeOf/BoundsOf/IsVisible/ParentOf read
// back layout results computed by those entry points, and SetParent records
// the parent/child edge containers use for invalidation and ancestor walks.
// Property[T] is core's other pillar: a reactive value whose Set notifies
// subscribers on real changes, the mechanism package bind builds two-way and
// one-way binding on top of. Every other fluo package (controls, input,
// bind, app) is built on the Widget/Element contract and Property defined
// here.
package core
