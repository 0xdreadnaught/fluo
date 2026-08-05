package controls

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
	"github.com/0xdreadnaught/fluo/theme"
)

// TextView is a READ-ONLY, word-wrapping, colorable text display — the wrapping
// read-only counterpart TextBlock never became (TextBlock stays the simple
// single-line label; TextBox stays the editable field). It carries none of
// TextBox's editable semantics — no caret, no click-to-position, no IME, no
// editing — because the content it shows (a chat transcript, streamed model
// output, a log) is read, not typed into.
//
// It is built for cheap streaming APPEND: the wrapped-row layout is cached, and
// because greedy word-wrap is stable under append (adding text never moves an
// earlier row's break point), Append re-wraps only from the LAST cached row —
// O(one line) per delta, not O(whole buffer) — so hundreds of small deltas in a
// long response don't stutter. Text selection and markdown are intentionally
// out of scope for now (selection is a tracked fast-follow).
type TextView struct {
	core.Element

	face  *text.Face
	runes []rune
	spans []ColorSpan
	color render.Color // default text color

	// Wrapped-row cache. rows[i] is the rune range [start,end) drawn on visual
	// row i. wrapWidth is the width they were wrapped to (-1 == none yet).
	// wrappedUpTo is the rune index rows are complete up to; Append rewinds it
	// to the last row's start so ensureWrapped re-wraps only the tail.
	rows        []textViewRow
	wrapWidth   float32
	wrappedUpTo int
}

type textViewRow struct{ start, end int }

// NewTextView returns an empty read-only text view drawing with face (nil is
// valid — it then measures/draws nothing, matching TextBlock's convention). The
// default text color is the theme's WindowText; override with SetColor.
func NewTextView(face *text.Face) *TextView {
	return &TextView{
		face:      face,
		color:     theme.Active().Color.WindowText,
		wrapWidth: -1,
	}
}

// SetText replaces the whole contents and returns v. Invalidates the wrap cache
// and measure (the wrapped height changes).
func (v *TextView) SetText(s string) *TextView {
	v.runes = []rune(s)
	v.invalidateWrapFrom(0)
	v.InvalidateMeasure()
	return v
}

// Append adds s to the end and returns v — the streaming primitive. It rewinds
// the wrap only to the last cached row's start, so the next layout re-wraps just
// the tail line, not the whole buffer (see the type doc). Invalidates measure
// because the height grows (the container must learn the new extent).
func (v *TextView) Append(s string) *TextView {
	if s == "" {
		return v
	}
	if n := len(v.rows); n > 0 {
		v.wrappedUpTo = v.rows[n-1].start
		v.rows = v.rows[:n-1]
	} else {
		v.wrappedUpTo = 0
	}
	v.runes = append(v.runes, []rune(s)...)
	v.InvalidateMeasure()
	return v
}

// Text returns the current contents.
func (v *TextView) Text() string { return string(v.runes) }

// SetColor sets the default text color (the color outside any span) and returns
// v. Presentation-only — no relayout.
func (v *TextView) SetColor(c render.Color) *TextView {
	v.color = c
	return v
}

// SetColorSpans sets presentation-only per-range colors (see ColorSpan), copied
// and normalized (clamped to the buffer, Start-sorted). Colors do not affect
// wrapping, so this triggers no relayout — the next frame draws the new colors.
func (v *TextView) SetColorSpans(spans []ColorSpan) *TextView {
	v.spans = normalizeColorSpans(spans, len(v.runes))
	return v
}

// ColorSpans returns a copy of the normalized spans, or nil.
func (v *TextView) ColorSpans() []ColorSpan {
	if len(v.spans) == 0 {
		return nil
	}
	return append([]ColorSpan(nil), v.spans...)
}

func (v *TextView) lineHeight() float32 {
	if v.face == nil {
		return 0
	}
	return v.face.LineHeight()
}

// invalidateWrapFrom drops the cached wrap back to rune index `from` (0 == all),
// so the next ensureWrapped rebuilds from there.
func (v *TextView) invalidateWrapFrom(from int) {
	if from <= 0 {
		v.rows = v.rows[:0]
		v.wrappedUpTo = 0
		return
	}
	// Keep rows whose range ends at or before `from`; re-wrap the rest.
	keep := 0
	for keep < len(v.rows) && v.rows[keep].end <= from {
		keep++
	}
	v.rows = v.rows[:keep]
	if keep > 0 {
		v.wrappedUpTo = v.rows[keep-1].end
	} else {
		v.wrappedUpTo = 0
	}
}

// ensureWrapped makes rows a complete wrap of the whole buffer at `width`. A
// width change forces a full re-wrap; otherwise it resumes from wrappedUpTo, so
// an Append (which rewinds wrappedUpTo one row) re-wraps only the tail.
func (v *TextView) ensureWrapped(width float32) {
	if v.face == nil {
		v.rows = v.rows[:0]
		return
	}
	if width != v.wrapWidth {
		v.rows = v.rows[:0]
		v.wrappedUpTo = 0
		v.wrapWidth = width
	}
	if v.wrappedUpTo >= len(v.runes) && (len(v.rows) > 0 || len(v.runes) == 0) {
		return // already wrapped to the end
	}
	i := v.wrappedUpTo
	for i <= len(v.runes) {
		lineEnd := i
		for lineEnd < len(v.runes) && v.runes[lineEnd] != '\n' {
			lineEnd++
		}
		v.wrapLine(i, lineEnd, width)
		if lineEnd >= len(v.runes) {
			break
		}
		i = lineEnd + 1 // skip the '\n'
	}
	v.wrappedUpTo = len(v.runes)
}

// wrapLine greedily wraps the logical line [start,end) (no '\n' inside) to
// width, appending one or more rows — at least one, so an empty line still
// takes a row. Mirrors the TextBox word-wrap: break at the last space, else
// hard-break an over-long word.
func (v *TextView) wrapLine(start, end int, width float32) {
	if start == end {
		v.rows = append(v.rows, textViewRow{start, end})
		return
	}
	rowStart := start
	lastSpace := -1
	for i := rowStart; i < end; i++ {
		w := v.face.Measure(string(v.runes[rowStart : i+1])).W
		if v.runes[i] == ' ' {
			lastSpace = i
		}
		if w <= width {
			continue
		}
		switch {
		case i == rowStart:
			v.rows = append(v.rows, textViewRow{rowStart, i + 1})
			rowStart = i + 1
		case v.runes[i] == ' ':
			v.rows = append(v.rows, textViewRow{rowStart, i})
			rowStart = i + 1
		case lastSpace >= rowStart:
			v.rows = append(v.rows, textViewRow{rowStart, lastSpace + 1})
			rowStart = lastSpace + 1
		default:
			v.rows = append(v.rows, textViewRow{rowStart, i})
			rowStart = i
		}
		lastSpace = -1
		i = rowStart - 1
	}
	if rowStart < end {
		v.rows = append(v.rows, textViewRow{rowStart, end})
	}
}

// MeasureContent wraps to the available width and reports that width by the
// wrapped row count times the line height. Cheap on the streaming path: an
// unchanged width re-wraps only the tail Append rewound.
func (v *TextView) MeasureContent(available render.Size) render.Size {
	if v.face == nil {
		return render.Size{}
	}
	v.ensureWrapped(available.W)
	return render.Size{W: available.W, H: float32(len(v.rows)) * v.lineHeight()}
}

// Render draws each wrapped row via the shared colored-run primitive
// (drawColoredRuns): the default color outside spans, span colors inside. No
// selection (read-only), so the selection color is unused.
func (v *TextView) Render(r render.Renderer) {
	if v.face == nil || len(v.runes) == 0 {
		return
	}
	b := v.Bounds()
	v.ensureWrapped(b.W)
	lh := v.lineHeight()

	for i, row := range v.rows {
		if row.start >= row.end {
			continue // blank line: nothing to draw, but it still took vertical space
		}
		y := b.Y + float32(i)*lh
		line := v.runes[row.start:row.end]
		rowStart := row.start
		drawColoredRuns(r, v.face, line, rowStart, 0, 0, y, v.color, v.color, v.spans, func(col int) float32 {
			return b.X + v.face.Measure(string(v.runes[rowStart:rowStart+col])).W
		})
	}
}
