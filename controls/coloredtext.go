package controls

import (
	"sort"

	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
)

// drawColoredRuns draws one visual line's runes as a sequence of color runs,
// shared by TextBox (editable, with a selection) and TextView (read-only). It
// cuts the line at the union of the selection edges and every ColorSpan edge
// that falls inside it, drawing each sub-run via face.Draw at the x that
// xAt(col) yields, in the color colorAtRun resolves.
//
//   - runes:        the line's LOCAL runes.
//   - lineLo:       the buffer index of the line's first rune, so spans (which
//                   are absolute) map to local columns.
//   - selLo, selHi: the selection in LOCAL columns; selLo >= selHi means none.
//   - defaultColor: the color outside every span and the selection.
//   - selColor:     the color inside the selection (wins over any span).
//   - spans:        the (Start-sorted) color spans; nil for none.
//   - xAt:          local column -> absolute x (folds in the caller's base +
//                   hscroll); xAt(0) is the line's left edge.
//
// With no spans AND no selection this emits exactly ONE face.Draw of the whole
// line — byte-identical (kerning included) to a single face.Draw(line), so an
// un-colored, unselected line renders unchanged. Cutting a line loses the kern
// across each cut, the same loss a selection split already accepts; only
// colored/selected lines ever see it.
func drawColoredRuns(r render.Renderer, face *text.Face, runes []rune, lineLo, selLo, selHi int, y float32, defaultColor, selColor render.Color, spans []ColorSpan, xAt func(col int) float32) {
	n := len(runes)
	if face == nil || n == 0 {
		return
	}
	// Fast path: nothing splits the line -> one draw, identical to before.
	if len(spans) == 0 && selLo >= selHi {
		face.Draw(r, render.Point{X: xAt(0), Y: y}, string(runes), defaultColor)
		return
	}

	cuts := []int{0, n}
	if selLo < selHi {
		if selLo > 0 && selLo < n {
			cuts = append(cuts, selLo)
		}
		if selHi > 0 && selHi < n {
			cuts = append(cuts, selHi)
		}
	}
	for _, sp := range spans {
		if s := sp.Start - lineLo; s > 0 && s < n {
			cuts = append(cuts, s)
		}
		if e := sp.End - lineLo; e > 0 && e < n {
			cuts = append(cuts, e)
		}
	}
	sort.Ints(cuts)

	segStart := 0
	for _, c := range cuts {
		if c <= segStart { // skip the leading 0 and any duplicate cut
			continue
		}
		col := colorAtRun(spans, lineLo, segStart, selLo, selHi, defaultColor, selColor)
		face.Draw(r, render.Point{X: xAt(segStart), Y: y}, string(runes[segStart:c]), col)
		segStart = c
	}
}

// normalizeColorSpans copies spans, clamps each to [0,n], drops the empty/
// inverted ones, and Start-sorts the rest — the shared normalization both
// TextBox.SetColorSpans and TextView.SetColorSpans apply. Returns nil for an
// empty input.
func normalizeColorSpans(spans []ColorSpan, n int) []ColorSpan {
	if len(spans) == 0 {
		return nil
	}
	out := make([]ColorSpan, 0, len(spans))
	for _, sp := range spans {
		sp.Start = clampInt(sp.Start, 0, n)
		sp.End = clampInt(sp.End, 0, n)
		if sp.End <= sp.Start {
			continue
		}
		out = append(out, sp)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}

// colorAtRun resolves the color of the run starting at LOCAL column col in a
// line whose first rune is buffer index lineLo. Selection wins first (the whole
// [selLo,selHi) range draws selColor so selected text stays legible), then the
// first covering ColorSpan (spans are Start-sorted and meant to be
// non-overlapping — see SetColorSpans), else defaultColor.
func colorAtRun(spans []ColorSpan, lineLo, col, selLo, selHi int, defaultColor, selColor render.Color) render.Color {
	if col >= selLo && col < selHi {
		return selColor
	}
	abs := lineLo + col
	for _, sp := range spans {
		if abs >= sp.Start && abs < sp.End {
			return sp.Color
		}
	}
	return defaultColor
}
