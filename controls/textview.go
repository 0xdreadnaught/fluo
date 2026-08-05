package controls

import (
	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
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

	// Selection (read-only: select + copy, never edit). anchor/caret are rune
	// indices; the selection is [min,max). They are NOT a caret — nothing is
	// ever drawn at them and no editing keys move them; they only mark a
	// selected range for the Highlight band and Ctrl+C. focused/dragging track
	// interaction. See OnPointer/OnKey.
	anchor, caret int
	focused       bool
	dragging      bool

	colors theme.ColorTokens

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
	th := theme.Active()
	return &TextView{
		face:      face,
		color:     th.Color.WindowText,
		colors:    th.Color,
		wrapWidth: -1,
	}
}

// SetText replaces the whole contents and returns v. Invalidates the wrap cache
// and measure (the wrapped height changes).
func (v *TextView) SetText(s string) *TextView {
	v.runes = []rune(s)
	v.anchor, v.caret = 0, 0 // a fresh document has no selection
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
// (drawColoredRuns): the default color outside spans, span colors inside, and —
// where a selection intersects the row — a Highlight band under it with the
// selected runes recolored HighlightText. Still no caret (read-only). With no
// selection AND no spans a row is one plain face.Draw, so an unselected TextView
// renders byte-identically to before selection existed.
func (v *TextView) Render(r render.Renderer) {
	if v.face == nil || len(v.runes) == 0 {
		return
	}
	b := v.Bounds()
	v.ensureWrapped(b.W)
	lh := v.lineHeight()
	selStart, selEnd := v.Selection()

	for i, row := range v.rows {
		if row.start >= row.end {
			continue // blank line: nothing to draw, but it still took vertical space
		}
		y := b.Y + float32(i)*lh
		rowStart := row.start
		line := v.runes[rowStart:row.end]
		xAt := func(col int) float32 {
			return b.X + v.face.Measure(string(v.runes[rowStart:rowStart+col])).W
		}

		// The selection intersected with this row, in LOCAL columns.
		selLo := clampInt(selStart, rowStart, row.end) - rowStart
		selHi := clampInt(selEnd, rowStart, row.end) - rowStart
		if selLo < selHi {
			r.FillRect(render.Rect{X: xAt(selLo), Y: y, W: xAt(selHi) - xAt(selLo), H: lh}, v.colors.Highlight)
		}
		drawColoredRuns(r, v.face, line, rowStart, selLo, selHi, y, v.color, v.colors.HighlightText, v.spans, xAt)
	}
}

// AcceptsFocus makes the TextView click-focusable so it can receive Ctrl+C to
// copy a selection. It stays READ-ONLY — focus only enables selection + copy,
// never a caret, editing, or IME.
func (v *TextView) AcceptsFocus() bool { return true }

// TabStop returns false: a TextView accepts focus on CLICK but is NOT in the Tab
// cycle (see input.TabStop). A transcript is many TextViews; putting each in the
// Tab order would make Tab a maze, so Tab flows past them to the real controls
// while a click still focuses a message to select/copy it.
func (v *TextView) TabStop() bool { return false }

// Selection returns the selected rune range [start,end) (start<=end), or an
// empty range at 0 when nothing is selected.
func (v *TextView) Selection() (start, end int) {
	if v.anchor <= v.caret {
		return v.anchor, v.caret
	}
	return v.caret, v.anchor
}

// Select sets the selected rune range programmatically (each endpoint clamped
// to the buffer) and returns v — for a search hit, a select-all, or restoring a
// selection. It only marks the range; it moves no focus and copies nothing.
func (v *TextView) Select(start, end int) *TextView {
	v.anchor = v.clampRune(start)
	v.caret = v.clampRune(end)
	return v
}

// SelectedText returns the currently selected text (empty if none).
func (v *TextView) SelectedText() string {
	s, e := v.Selection()
	if s == e {
		return ""
	}
	return string(v.runes[s:e])
}

func (v *TextView) clampRune(i int) int { return clampInt(i, 0, len(v.runes)) }

// runeAtPoint maps an absolute pointer position to the nearest rune index,
// across the wrapped rows: the row from the y offset, the column from the x
// offset within that row.
func (v *TextView) runeAtPoint(p render.Point) int {
	b := v.Bounds()
	v.ensureWrapped(b.W)
	if len(v.rows) == 0 {
		return 0
	}
	lh := v.lineHeight()
	row := 0
	if lh > 0 {
		row = int((p.Y - b.Y) / lh)
	}
	if row < 0 {
		row = 0
	}
	if row >= len(v.rows) {
		row = len(v.rows) - 1
	}
	rr := v.rows[row]
	col := columnAtX(v.face, v.runes[rr.start:rr.end], p.X-b.X)
	return rr.start + col
}

// columnAtX returns the caret-style column (0..len) in runes nearest x (x is
// relative to the run's left edge). Mirrors TextBox's own hit-test.
func columnAtX(face *text.Face, runes []rune, x float32) int {
	n := len(runes)
	if face == nil {
		if x < 0 {
			return 0
		}
		return n
	}
	widths := face.PrefixWidths(runes)
	for i := 0; i < n; i++ {
		if x < (widths[i]+widths[i+1])/2 {
			return i
		}
	}
	return n
}

// OnPointer implements click-drag selection: press sets the anchor at the hit
// rune and captures the pointer; drag extends the caret end; release ends the
// drag. A press with no drag collapses the selection (anchor==caret). Read-only
// throughout — this only moves the selection range, never a caret.
func (v *TextView) OnPointer(e *input.PointerEvent) {
	switch e.Action {
	case input.Press:
		i := v.clampRune(v.runeAtPoint(e.Pos))
		v.anchor, v.caret = i, i
		v.dragging = true
		e.Router.Capture(v)
		e.Handled = true
	case input.Move:
		if v.dragging && e.Router.Captured() == core.Widget(v) {
			v.caret = v.clampRune(v.runeAtPoint(e.Pos))
			e.Handled = true
		}
	case input.Release:
		if v.dragging {
			v.dragging = false
			e.Router.Release()
			e.Handled = true
		}
	}
}

// OnKey implements the read-only keyboard: Ctrl+C copies the selection, Ctrl+A
// selects all. No other key does anything (never edits). Inert unless focused.
func (v *TextView) OnKey(e *input.KeyEvent) {
	if !v.focused || e.Action != input.Press || e.Mods&input.ModCtrl == 0 {
		return
	}
	switch e.Key {
	case input.KeyC:
		v.copySelection(e.Router)
		e.Handled = true
	case input.KeyA:
		v.anchor, v.caret = 0, len(v.runes)
		e.Handled = true
	}
}

// OnFocusChanged tracks focus. Losing focus keeps the selection (so a copy that
// moved focus to a menu still has something to copy) but disables the keyboard
// path until re-focused.
func (v *TextView) OnFocusChanged(f bool) { v.focused = f }

func (v *TextView) copySelection(r *input.Router) {
	if r == nil {
		return
	}
	clip := r.Clipboard()
	s, e := v.Selection()
	if clip == nil || s == e {
		return
	}
	clip.Set(string(v.runes[s:e]))
}
