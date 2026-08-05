package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
)

var spanRed = render.Color{R: 220, G: 40, B: 40, A: 255}

// renderGlyphRuns renders tb through a recording renderer and returns the
// per-DrawGlyphs-call (glyphCount, color) runs, in draw order.
func renderGlyphRuns(tb *TextBox) []glyphRun {
	var rec recordRenderer
	core.RenderWidget(tb, &rec)
	return rec.glyphRuns
}

// TestTextBoxColorSpansSegmentRuns: a span splits the line into colored runs
// (default outside, span color inside), in order.
func TestTextBoxColorSpansSegmentRuns(t *testing.T) {
	tb, _ := newFocusedTextBox(t, "abcdef")
	tb.SetColorSpans([]ColorSpan{{Start: 2, End: 4, Color: spanRed}})

	runs := renderGlyphRuns(tb)
	def := tb.colors.WindowText
	want := []glyphRun{{2, def}, {2, spanRed}, {2, def}}
	if len(runs) != len(want) {
		t.Fatalf("glyph runs = %+v, want 3 runs [ab|cd|ef]", runs)
	}
	for i := range want {
		if runs[i].n != want[i].n || runs[i].color != want[i].color {
			t.Fatalf("run %d = {n:%d %v}, want {n:%d %v}", i, runs[i].n, runs[i].color, want[i].n, want[i].color)
		}
	}
}

// TestTextBoxNoSpansIsSingleRun: with no spans and no selection the whole line
// is one draw (the byte-identical fast path).
func TestTextBoxNoSpansIsSingleRun(t *testing.T) {
	tb, _ := newFocusedTextBox(t, "abcdef")
	runs := renderGlyphRuns(tb)
	if len(runs) != 1 || runs[0].n != 6 || runs[0].color != tb.colors.WindowText {
		t.Fatalf("un-colored runs = %+v, want a single 6-glyph WindowText run", runs)
	}
}

// TestTextBoxSelectionOverridesColorSpan: within a selection the HighlightText
// color wins over a span, so selected text stays legible.
func TestTextBoxSelectionOverridesColorSpan(t *testing.T) {
	tb, _ := newFocusedTextBox(t, "abcdef")
	tb.SetColorSpans([]ColorSpan{{Start: 2, End: 4, Color: spanRed}})
	tb.Select(1, 5) // covers the whole span

	for _, run := range renderGlyphRuns(tb) {
		if run.color == spanRed {
			t.Fatalf("a run drew in the span color inside the selection: %+v — selection must override", run)
		}
	}
}

// TestTextBoxColorSpansClearedRestoresSingleRun: clearing spans returns to the
// single-run fast path.
func TestTextBoxColorSpansClearedRestoresSingleRun(t *testing.T) {
	tb, _ := newFocusedTextBox(t, "abcdef")
	tb.SetColorSpans([]ColorSpan{{Start: 2, End: 4, Color: spanRed}})
	if len(renderGlyphRuns(tb)) != 3 {
		t.Fatal("precondition: colored box should render 3 runs")
	}
	tb.SetColorSpans(nil)
	if runs := renderGlyphRuns(tb); len(runs) != 1 {
		t.Fatalf("after clearing spans: %d runs, want 1", len(runs))
	}
}

// TestTextBoxColorSpansNormalized: SetColorSpans clamps to the buffer, drops
// empty/out-of-range spans, and Start-sorts.
func TestTextBoxColorSpansNormalized(t *testing.T) {
	tb, _ := newFocusedTextBox(t, "abcdef") // len 6
	tb.SetColorSpans([]ColorSpan{
		{Start: 4, End: 5, Color: spanRed},   // in order after sort
		{Start: 0, End: 2, Color: spanRed},   // should sort before the first
		{Start: 10, End: 20, Color: spanRed}, // fully past the buffer -> dropped
		{Start: 3, End: 3, Color: spanRed},   // empty -> dropped
		{Start: 5, End: 100, Color: spanRed}, // End clamped to 6
	})
	got := tb.ColorSpans()
	want := []ColorSpan{{0, 2, spanRed}, {4, 5, spanRed}, {5, 6, spanRed}}
	if len(got) != len(want) {
		t.Fatalf("normalized spans = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("span %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestTextBoxColorSpansGetterCopy: ColorSpans returns a copy; mutating it does
// not affect the box.
func TestTextBoxColorSpansGetterCopy(t *testing.T) {
	tb, _ := newFocusedTextBox(t, "abcdef")
	tb.SetColorSpans([]ColorSpan{{Start: 1, End: 3, Color: spanRed}})
	got := tb.ColorSpans()
	got[0].Start = 99 // mutate the caller's copy
	if tb.ColorSpans()[0].Start != 1 {
		t.Fatal("ColorSpans() did not return an independent copy")
	}
}

// TestTextBoxColorSpansMultilineMapToLines: a span is applied by ABSOLUTE
// buffer index, so it colors the right run on the right logical line (the
// renderMultiline lineLo->local mapping).
func TestTextBoxColorSpansMultilineMapToLines(t *testing.T) {
	tb := NewTextBox(buttonFace(t))
	tb.SetMultiline(true)
	tb.SetText("ab\ncd") // ab=0,1  \n=2  cd=3,4
	tb.SetWidth(300)
	tb.SetHeight(60)
	r := input.NewRouter()
	r.SetRoot(tb)
	layoutButton(tb, render.Rect{X: 0, Y: 0, W: 300, H: 60})
	r.Focus(tb)

	tb.SetColorSpans([]ColorSpan{{Start: 3, End: 5, Color: spanRed}}) // "cd" only

	runs := renderGlyphRuns(tb)
	def := tb.colors.WindowText
	// Line 1 "ab" default (1 run), line 2 "cd" red (1 run).
	if len(runs) != 2 {
		t.Fatalf("multiline runs = %+v, want 2 (ab default, cd red)", runs)
	}
	if runs[0].color != def || runs[1].color != spanRed {
		t.Fatalf("multiline run colors = [%v, %v], want [default, red] — span must map to line 2 only", runs[0].color, runs[1].color)
	}
}

// TestTextBoxColorSpanOverTab: a span covering a run that contains a tab colors
// it in one draw without splitting on the tab (the tab draws no glyph but is
// advanced over inside face.Draw).
func TestTextBoxColorSpanOverTab(t *testing.T) {
	tb, _ := newFocusedTextBox(t, "a\tb") // a=0 \t=1 b=2
	tb.SetColorSpans([]ColorSpan{{Start: 0, End: 3, Color: spanRed}})

	runs := renderGlyphRuns(tb)
	if len(runs) != 1 || runs[0].color != spanRed {
		t.Fatalf("tab-spanning runs = %+v, want a single red run over a\tb", runs)
	}
	if runs[0].n != 2 {
		t.Fatalf("tab-spanning run glyph count = %d, want 2 (a and b; the tab draws no glyph)", runs[0].n)
	}
}
