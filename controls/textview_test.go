package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/render"
)

// TestTextViewIncrementalAppendMatchesFullWrap is the correctness guard for the
// cheap-append optimization: building text via many small Appends (re-wrapping
// only the tail each time) must yield EXACTLY the same wrapped rows as setting
// the whole string at once. If the stable-under-append reasoning were wrong,
// the two would diverge.
func TestTextViewIncrementalAppendMatchesFullWrap(t *testing.T) {
	face := buttonFace(t)
	full := "the quick brown fox jumps over the lazy dog and then some more words to force several wrapped rows here"
	const width = float32(120)

	a := NewTextView(face)
	a.SetText(full)
	a.MeasureContent(render.Size{W: width})

	b := NewTextView(face)
	for i := 0; i < len(full); i += 3 { // stream in 3-char deltas
		end := i + 3
		if end > len(full) {
			end = len(full)
		}
		b.Append(full[i:end])
		b.MeasureContent(render.Size{W: width}) // layout between deltas, like streaming
	}

	if a.Text() != b.Text() {
		t.Fatalf("text: full=%q incremental=%q", a.Text(), b.Text())
	}
	if len(a.rows) != len(b.rows) {
		t.Fatalf("row count: full=%d incremental=%d", len(a.rows), len(b.rows))
	}
	for i := range a.rows {
		if a.rows[i] != b.rows[i] {
			t.Fatalf("row %d differs: full=%v incremental=%v (incremental wrap must match full wrap)", i, a.rows[i], b.rows[i])
		}
	}
}

// TestTextViewWrapsToWidth: long text wraps into multiple rows and the measured
// height is rows*lineHeight.
func TestTextViewWrapsToWidth(t *testing.T) {
	tv := NewTextView(buttonFace(t))
	tv.SetText("aaaa bbbb cccc dddd eeee ffff gggg hhhh")

	sz := tv.MeasureContent(render.Size{W: 60})
	if len(tv.rows) < 2 {
		t.Fatalf("expected wrapping into >=2 rows at width 60, got %d", len(tv.rows))
	}
	if want := float32(len(tv.rows)) * tv.lineHeight(); sz.H != want {
		t.Fatalf("measured H = %v, want rows*lineHeight = %v", sz.H, want)
	}
}

// TestTextViewNewlinesSplitRows: hard newlines each start a new row even when
// the text is far narrower than the width.
func TestTextViewNewlinesSplitRows(t *testing.T) {
	tv := NewTextView(buttonFace(t))
	tv.SetText("a\nb\nc")
	tv.MeasureContent(render.Size{W: 300})
	if len(tv.rows) != 3 {
		t.Fatalf("rows = %d, want 3 (one per hard line)", len(tv.rows))
	}
}

// TestTextViewAppendGrowsHeight: appending more content increases the measured
// height (the container-relayout signal).
func TestTextViewAppendGrowsHeight(t *testing.T) {
	tv := NewTextView(buttonFace(t))
	tv.SetText("line one")
	h1 := tv.MeasureContent(render.Size{W: 300}).H
	tv.Append("\nline two\nline three")
	h2 := tv.MeasureContent(render.Size{W: 300}).H
	if h2 <= h1 {
		t.Fatalf("height after append = %v, want > %v", h2, h1)
	}
}

// TestTextViewEmptyAppendIsNoop: Append("") changes nothing.
func TestTextViewEmptyAppendIsNoop(t *testing.T) {
	tv := NewTextView(buttonFace(t))
	tv.SetText("abc")
	tv.MeasureContent(render.Size{W: 300})
	before := len(tv.rows)
	tv.Append("")
	if tv.Text() != "abc" || len(tv.rows) != before {
		t.Fatal("Append(\"\") must be a no-op")
	}
}

// TestTextViewColorSpansRender: spans color the right runs through the shared
// colored-run path.
func TestTextViewColorSpansRender(t *testing.T) {
	tv := NewTextView(buttonFace(t))
	tv.SetText("abcdef")
	tv.SetColorSpans([]ColorSpan{{Start: 2, End: 4, Color: spanRed}})

	core.MeasureWidget(tv, render.Size{W: 300, H: 100})
	core.ArrangeWidget(tv, render.Rect{X: 0, Y: 0, W: 300, H: 100})
	var rec recordRenderer
	core.RenderWidget(tv, &rec)

	def := tv.color
	want := []glyphRun{{2, def}, {2, spanRed}, {2, def}}
	if len(rec.glyphRuns) != len(want) {
		t.Fatalf("glyph runs = %+v, want 3 [ab|cd|ef]", rec.glyphRuns)
	}
	for i := range want {
		if rec.glyphRuns[i] != want[i] {
			t.Fatalf("run %d = %+v, want %+v", i, rec.glyphRuns[i], want[i])
		}
	}
}
