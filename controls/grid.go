package controls

import (
	"fmt"
	"math"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
)

// trackKind distinguishes the three ways a Track can be sized.
type trackKind uint8

const (
	trackPx trackKind = iota
	trackAuto
	trackStar
)

// Track defines the sizing behavior of one grid row or column: a fixed
// pixel size (Px), sized-to-content (AutoTrack), or a proportional share of
// the remaining space (Star).
type Track struct {
	kind  trackKind
	value float32 // px size for Px tracks, weight for Star tracks; unused for Auto.
}

// Px returns a track with a fixed size of v.
func Px(v float32) Track {
	return Track{kind: trackPx, value: v}
}

// AutoTrack returns a track sized to the largest desired extent (on that
// axis) of the children assigned to it.
func AutoTrack() Track {
	return Track{kind: trackAuto}
}

// Star returns a track that receives a proportional share of the space
// remaining after Px and Auto tracks are resolved, weighted by weight
// against the other Star tracks on the same axis.
func Star(weight float32) Track {
	return Track{kind: trackStar, value: weight}
}

// gridCell associates a child widget with the (row, col) track it occupies.
type gridCell struct {
	child    core.Widget
	row, col int
}

// Grid arranges children into a fixed set of rows and columns, each an
// independently-sized Track (Px/Auto/Star).
//
// v0 limitations (by design, not bugs):
//   - No cell spans: every child occupies exactly one row and one column.
//   - Children are measured exactly once per measure pass, with per-axis
//     available space of the Px value for Px tracks and +Inf for Auto/Star
//     tracks. They are never re-measured against the final resolved track
//     size, so a child placed in a Star track keeps its Inf-measured desired
//     size; ArrangeWidget's stretch/alignment absorbs the difference between
//     that desired size and the cell it is arranged into.
//   - When Rows or Cols is never called, that axis defaults to a single
//     Star(1) track.
//   - Rows/Cols re-validate every already-Added cell against the new track
//     count, panicking rather than letting a later layout pass index out of
//     range.
type Grid struct {
	core.Element

	rows, cols []Track
	cells      []gridCell
}

// NewGrid returns an empty Grid with no rows, columns, or children.
func NewGrid() *Grid {
	return &Grid{}
}

// Rows sets the row tracks for this axis, replacing any previously set
// rows. Layout-relevant: invalidates measure. Panics if any already-Added
// cell's row index no longer fits the new track count.
func (g *Grid) Rows(tracks ...Track) *Grid {
	g.rows = tracks
	n := len(g.effectiveRows())
	for _, c := range g.cells {
		if c.row >= n {
			panic(fmt.Sprintf("controls: Grid.Rows(%d tracks) invalidates existing cell at row %d", n, c.row))
		}
	}
	g.InvalidateMeasure()
	return g
}

// Cols sets the column tracks for this axis, replacing any previously set
// columns. Layout-relevant: invalidates measure. Panics if any already-Added
// cell's col index no longer fits the new track count.
func (g *Grid) Cols(tracks ...Track) *Grid {
	g.cols = tracks
	n := len(g.effectiveCols())
	for _, c := range g.cells {
		if c.col >= n {
			panic(fmt.Sprintf("controls: Grid.Cols(%d tracks) invalidates existing cell at col %d", n, c.col))
		}
	}
	g.InvalidateMeasure()
	return g
}

// Add places w at (row, col), re-parenting it to this Grid and invalidating
// measure. row/col are validated against the effective row/col track counts
// (which fall back to a single default Star(1) track per axis); an
// out-of-range index panics.
func (g *Grid) Add(w core.Widget, row, col int) *Grid {
	rows := g.effectiveRows()
	cols := g.effectiveCols()

	if row < 0 || row >= len(rows) {
		panic(fmt.Sprintf("controls: Grid.Add row %d out of range (%d rows)", row, len(rows)))
	}
	if col < 0 || col >= len(cols) {
		panic(fmt.Sprintf("controls: Grid.Add col %d out of range (%d cols)", col, len(cols)))
	}

	g.cells = append(g.cells, gridCell{child: w, row: row, col: col})
	core.SetParent(w, g)
	g.InvalidateMeasure()
	return g
}

// Children returns the widgets added to this Grid, in Add order. Returns a
// copy; mutating it does not affect the panel.
func (g *Grid) Children() []core.Widget {
	out := make([]core.Widget, len(g.cells))
	for i, c := range g.cells {
		out[i] = c.child
	}
	return out
}

// effectiveRows returns the row tracks in effect: g.rows, or a single
// Star(1) track if Rows was never called.
func (g *Grid) effectiveRows() []Track {
	if len(g.rows) == 0 {
		return []Track{Star(1)}
	}
	return g.rows
}

// effectiveCols returns the column tracks in effect: g.cols, or a single
// Star(1) track if Cols was never called.
func (g *Grid) effectiveCols() []Track {
	if len(g.cols) == 0 {
		return []Track{Star(1)}
	}
	return g.cols
}

// trackChildAvail returns the per-axis available space a child in track t
// is measured with: t's fixed value for a Px track, +Inf for Auto/Star.
func trackChildAvail(t Track) float32 {
	if t.kind == trackPx {
		return t.value
	}
	return float32(math.Inf(1))
}

// resolveTracks resolves tracks against the total available space on one
// axis, given a callback that reports the max desired extent (on that axis,
// over visible children) of the track at index i. It is the single
// resolution routine shared by MeasureContent (resolving against the
// available space offered to the grid) and ArrangeContent (re-resolving
// against the grid's actual final extent):
//
//  1. A Px track resolves to its fixed value.
//  2. An Auto track resolves to maxDesired(i) (0 if no children).
//  3. Remaining space (avail minus the sum of Px and Auto tracks, floored at
//  0. is split across Star tracks proportional to weight/Σweights. If
//     avail is +Inf, each Star track instead resolves like Auto (maxDesired).
func resolveTracks(tracks []Track, avail float32, maxDesired func(i int) float32) []float32 {
	resolved := make([]float32, len(tracks))

	var sumFixed, sumWeight float32
	infAvail := math.IsInf(float64(avail), 1)

	for i, t := range tracks {
		switch t.kind {
		case trackPx:
			resolved[i] = t.value
			sumFixed += resolved[i]
		case trackAuto:
			resolved[i] = maxDesired(i)
			sumFixed += resolved[i]
		case trackStar:
			if infAvail {
				resolved[i] = maxDesired(i)
			} else {
				sumWeight += t.value
			}
		}
	}

	if !infAvail && sumWeight > 0 {
		remaining := avail - sumFixed
		if remaining < 0 {
			remaining = 0
		}
		for i, t := range tracks {
			if t.kind == trackStar {
				resolved[i] = remaining * (t.value / sumWeight)
			}
		}
	}
	// If there are Star tracks but sumWeight == 0 (e.g. all Star(0)), no
	// weight exists to divide the remaining space by, so those tracks stay
	// at their zero-value resolved size and the leftover space is simply
	// left unallocated. This is a deliberate v0 choice, not a bug: a
	// zero-weight Star track is a degenerate input, and silently treating it
	// as "give it everything" or "give it nothing but still consume space"
	// would both be surprising. Callers who want a track to consume all
	// remaining space should give it a positive weight instead.

	return resolved
}

// sumF32 returns the sum of xs.
func sumF32(xs []float32) float32 {
	var total float32
	for _, x := range xs {
		total += x
	}
	return total
}

// trackMaxDesired returns a callback reporting, for track index i, the
// largest desired extent (along axisW: true for width, false for height)
// among visible children whose cell index (col if axisW, row otherwise)
// equals i. Hidden children are excluded, per the engine's convention that
// hidden widgets contribute nothing to a parent's Auto/Star sizing.
func (g *Grid) trackMaxDesired(axisW bool) func(i int) float32 {
	return func(i int) float32 {
		var max float32
		for _, c := range g.cells {
			idx := c.row
			if axisW {
				idx = c.col
			}
			if idx != i || !core.IsVisible(c.child) {
				continue
			}
			d := core.DesiredSizeOf(c.child)
			v := d.H
			if axisW {
				v = d.W
			}
			if v > max {
				max = v
			}
		}
		return max
	}
}

// MeasureContent measures every child exactly once (per-axis available is
// the Px value for a Px track, +Inf for Auto/Star), then resolves the row
// and column tracks against the space available to the grid. The desired
// size is the sum of the resolved column widths and row heights.
func (g *Grid) MeasureContent(available render.Size) render.Size {
	rows := g.effectiveRows()
	cols := g.effectiveCols()

	for _, c := range g.cells {
		childAvail := render.Size{
			W: trackChildAvail(cols[c.col]),
			H: trackChildAvail(rows[c.row]),
		}
		core.MeasureWidget(c.child, childAvail)
	}

	resolvedCols := resolveTracks(cols, available.W, g.trackMaxDesired(true))
	resolvedRows := resolveTracks(rows, available.H, g.trackMaxDesired(false))

	return render.Size{W: sumF32(resolvedCols), H: sumF32(resolvedRows)}
}

// ArrangeContent re-resolves the row and column tracks against the grid's
// actual final extent (bounds), then arranges each child into the rect of
// the track intersection (row, col) it occupies. ArrangeWidget handles
// stretch/alignment of the child within that cell rect.
func (g *Grid) ArrangeContent(bounds render.Rect) {
	rows := g.effectiveRows()
	cols := g.effectiveCols()

	resolvedCols := resolveTracks(cols, bounds.W, g.trackMaxDesired(true))
	resolvedRows := resolveTracks(rows, bounds.H, g.trackMaxDesired(false))

	colOffsets := make([]float32, len(resolvedCols))
	x := bounds.X
	for i, w := range resolvedCols {
		colOffsets[i] = x
		x += w
	}

	rowOffsets := make([]float32, len(resolvedRows))
	y := bounds.Y
	for i, h := range resolvedRows {
		rowOffsets[i] = y
		y += h
	}

	for _, c := range g.cells {
		cell := render.Rect{
			X: colOffsets[c.col],
			Y: rowOffsets[c.row],
			W: resolvedCols[c.col],
			H: resolvedRows[c.row],
		}
		core.ArrangeWidget(c.child, cell)
	}
}
