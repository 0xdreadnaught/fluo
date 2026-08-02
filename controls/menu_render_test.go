package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/theme"
)

// openMenuPopupRows opens bar's top-level menu idx through the real
// router-driven path (a press on its cell, exactly as
// TestMenuBarClickOpensPopup does), re-lays the host so the popup's rows get
// real arranged bounds, and returns the popup card together with its rows.
// The render tests below all need a popup whose geometry came from a
// genuine layout pass — a hand-built row would render against a zero rect
// and pin nothing.
func openMenuPopupRows(t *testing.T, bar *MenuBar, host *OverlayHost, r *input.Router, idx int) (*menuPopupCard, []core.Widget) {
	t.Helper()

	clickAt(r, rectCenter(bar.cellRect(idx)))
	layoutOverlay(host, 300, 300)

	card, ok := bar.popup.(*menuPopupCard)
	if !ok {
		t.Fatalf("bar.popup type = %T, want *menuPopupCard", bar.popup)
	}
	return card, menuPopupStackRows(t, bar.popup)
}

// TestMenuPopupCardRenderDrawsRaisedFrame pins the popup's own chrome: a
// raised ButtonFace bevel filling the card exactly. The face fill must come
// FIRST (the eight 1px edge rects are painted over it — see drawRaised), it
// must cover the card's whole bounds, and nothing may overhang: the card is
// the topmost thing on the overlay stack, so an overhanging bevel edge
// paints straight over the app content beside the menu with no clip
// anywhere to stop it.
func TestMenuPopupCardRenderDrawsRaisedFrame(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	bar, host, r, _ := newTestMenuBar(t)
	card, _ := openMenuPopupRows(t, bar, host, r, 0) // File

	bounds := card.Bounds()
	if bounds.Empty() {
		t.Fatal("fixture: the popup card has empty bounds, so nothing was laid out")
	}

	rr := newClipRecorder()
	card.Render(rr)

	if got := rr.fills[0]; got.rect != bounds || got.color != card.colors.ButtonFace {
		t.Fatalf("first fill = %+v, want the ButtonFace %v face fill over the card's bounds %+v",
			got, card.colors.ButtonFace, bounds)
	}
	assertPaintedWithinBounds(t, rr, bounds, "menuPopupCard.Render")

	// The bevel's directional edges are what make it read as raised rather
	// than a flat panel; pin one tone per direction.
	var lightTop, darkBottom bool
	for _, f := range rr.fills {
		if f.rect.H != 1 {
			continue
		}
		if f.rect.Y == bounds.Y && f.color == card.colors.ButtonHighlight {
			lightTop = true
		}
		if f.rect.Y == bounds.Bottom()-1 && f.color == card.colors.ButtonDarkShadow {
			darkBottom = true
		}
	}
	if !lightTop {
		t.Error("no ButtonHighlight edge along the card's top: the popup does not read as raised")
	}
	if !darkBottom {
		t.Error("no ButtonDarkShadow edge along the card's bottom: the popup does not read as raised")
	}
}

// TestMenuSeparatorRowRenderInsetGroove pins the separator's two-tone etched
// rule: exactly two 1px lines, stacked, each inset PaddingM from BOTH of the
// row's edges. The inset is the whole point — a groove drawn across the
// row's full width would run straight into the popup card's own bevel on
// either side and read as a break in the frame rather than a divider
// between item groups. The rule also has to sit inside the row vertically,
// or it collides with the item rows above and below it.
func TestMenuSeparatorRowRenderInsetGroove(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	bar, host, r, _ := newTestMenuBar(t)
	_, rows := openMenuPopupRows(t, bar, host, r, 0) // File: New, Open, separator, Exit

	sep, ok := rows[2].(*menuSeparatorRow)
	if !ok {
		t.Fatalf("rows[2] type = %T, want *menuSeparatorRow", rows[2])
	}
	bounds := core.BoundsOf(sep)
	if bounds.Empty() {
		t.Fatal("fixture: the separator row has empty bounds, so nothing was laid out")
	}

	rr := newClipRecorder()
	sep.Render(rr)

	if len(rr.fills) != 2 {
		t.Fatalf("separator emitted %d fills, want exactly 2 (shadow line + highlight line)", len(rr.fills))
	}
	assertPaintedWithinBounds(t, rr, bounds, "menuSeparatorRow.Render")

	pad := sep.metrics.PaddingM
	wantX := bounds.X + pad
	wantW := bounds.W - 2*pad
	for i, f := range rr.fills {
		if f.rect.X != wantX || f.rect.W != wantW {
			t.Errorf("groove line %d spans %v..%v, want %v..%v (inset PaddingM=%v on both sides)",
				i, f.rect.X, f.rect.Right(), wantX, wantX+wantW, pad)
		}
		if f.rect.H != 1 {
			t.Errorf("groove line %d has H=%v, want a 1px rule", i, f.rect.H)
		}
	}
	if got := rr.fills[0].color; got != sep.colors.ButtonShadow {
		t.Errorf("first groove line color = %v, want ButtonShadow %v", got, sep.colors.ButtonShadow)
	}
	if got := rr.fills[1].color; got != sep.colors.ButtonHighlight {
		t.Errorf("second groove line color = %v, want ButtonHighlight %v", got, sep.colors.ButtonHighlight)
	}
	if rr.fills[1].rect.Y != rr.fills[0].rect.Y+1 {
		t.Errorf("groove lines at y=%v and y=%v, want them stacked 1px apart",
			rr.fills[0].rect.Y, rr.fills[1].rect.Y)
	}
}

// TestMenuSeparatorRowRenderSkipsDegenerateGroove covers Render's width
// guard: a row too narrow to hold PaddingM on both sides would otherwise
// hand drawGroove a zero-or-negative-width rect, which paints a rule
// running the wrong way out of the row. Nothing should be drawn at all.
func TestMenuSeparatorRowRenderSkipsDegenerateGroove(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)
	th := theme.Active()

	sep := newMenuSeparatorRow(th.Color, th.Metric)
	// Exactly 2*PaddingM wide: the insets consume the whole row, leaving a
	// zero-width rule.
	narrow := render.Rect{X: 0, Y: 0, W: 2 * th.Metric.PaddingM, H: menuSeparatorHeight}
	core.MeasureWidget(sep, render.Size{W: narrow.W, H: narrow.H})
	core.ArrangeWidget(sep, narrow)

	rr := newClipRecorder()
	sep.Render(rr)

	if len(rr.fills) != 0 {
		t.Fatalf("a %v-wide separator emitted %d fills, want none (the groove degenerates to zero width)",
			narrow.W, len(rr.fills))
	}
}

// TestMenuItemRowRenderHoverBand pins the hover half of menuItemRow.Render,
// which no headless test reached before: the band must cover the row's
// bounds EXACTLY. Too narrow and it stops short of the popup's inner edge,
// leaving an unhighlighted strip beside the hovered item; too wide and it
// paints over the card's bevel frame. The label's color flips with the
// band, since WindowText on the navy Highlight is unreadable. Hover is
// driven through the live router, so this also pins that the row the
// pointer hit-tests into is the row that lights up.
func TestMenuItemRowRenderHoverBand(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	bar, host, r, _ := newTestMenuBar(t)
	_, rows := openMenuPopupRows(t, bar, host, r, 0) // File

	row, ok := rows[1].(*menuItemRow) // "Open"
	if !ok {
		t.Fatalf("rows[1] type = %T, want *menuItemRow", rows[1])
	}
	bounds := core.BoundsOf(row)

	// Resting: no band at all, plain WindowText label.
	rest := newClipRecorder()
	row.Render(rest)
	if len(rest.fills) != 0 {
		t.Fatalf("an unhovered item row emitted %d fills, want none (the card's face shows through)", len(rest.fills))
	}
	if got := row.label.Color(); got != row.colors.WindowText {
		t.Fatalf("unhovered label color = %v, want WindowText %v", got, row.colors.WindowText)
	}

	// Hovered, via a real pointer move over the row.
	r.PointerMove(rectCenter(bounds), 0)
	if !row.click.Hover() {
		t.Fatal("fixture: the row did not register hover, so the hover branch was never taken")
	}

	hover := newClipRecorder()
	row.Render(hover)
	if len(hover.fills) != 1 {
		t.Fatalf("a hovered item row emitted %d fills, want exactly 1 (the highlight band)", len(hover.fills))
	}
	if got := hover.fills[0]; got.rect != bounds || got.color != row.colors.Highlight {
		t.Fatalf("hover band = %+v, want Highlight %v over the row's whole bounds %+v",
			got, row.colors.Highlight, bounds)
	}
	if got := row.label.Color(); got != row.colors.HighlightText {
		t.Fatalf("hovered label color = %v, want HighlightText %v", got, row.colors.HighlightText)
	}
}

// TestMenuSubRowRenderHoverRecolorsLabelAndChevron is the submenu-trigger
// counterpart of the item-row hover test, and pins the one thing unique to
// menuSubRow.Render: the row has TWO children, and BOTH have to flip to
// HighlightText with the band. Recoloring only the label leaves a
// WindowText chevron sitting on the navy band — the exact bug a
// label-only recolor produces, invisible to any test that checks the fill
// alone.
func TestMenuSubRowRenderHoverRecolorsLabelAndChevron(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	bar, host, r, _ := newTestMenuBar(t)
	bar.entries[1].items.AddSub("Recent").Add("a.txt", nil)
	_, rows := openMenuPopupRows(t, bar, host, r, 1) // Edit: Undo, Recent >

	row, ok := rows[1].(*menuSubRow)
	if !ok {
		t.Fatalf("rows[1] type = %T, want *menuSubRow", rows[1])
	}
	bounds := core.BoundsOf(row)

	rest := newClipRecorder()
	row.Render(rest)
	if len(rest.fills) != 0 {
		t.Fatalf("an unhovered sub row emitted %d fills, want none", len(rest.fills))
	}
	if got := row.chevron.Color(); got != row.colors.WindowText {
		t.Fatalf("unhovered chevron color = %v, want WindowText %v", got, row.colors.WindowText)
	}

	r.PointerMove(rectCenter(bounds), 0)
	if !row.click.Hover() {
		t.Fatal("fixture: the sub row did not register hover, so the hover branch was never taken")
	}

	hover := newClipRecorder()
	row.Render(hover)
	if len(hover.fills) != 1 {
		t.Fatalf("a hovered sub row emitted %d fills, want exactly 1 (the highlight band)", len(hover.fills))
	}
	if got := hover.fills[0]; got.rect != bounds || got.color != row.colors.Highlight {
		t.Fatalf("hover band = %+v, want Highlight %v over the row's whole bounds %+v",
			got, row.colors.Highlight, bounds)
	}
	if got := row.label.Color(); got != row.colors.HighlightText {
		t.Errorf("hovered label color = %v, want HighlightText %v", got, row.colors.HighlightText)
	}
	if got := row.chevron.Color(); got != row.colors.HighlightText {
		t.Errorf("hovered chevron color = %v, want HighlightText %v", got, row.colors.HighlightText)
	}
}

// TestMenuBarRenderCellChromeMatchesHitTesting is the load-bearing MenuBar
// render test. Render walks its own x accumulator over m.cellWidths to place
// each cell's chrome, while hit-testing (cellAt) and popup anchoring
// (cellRect) walk the SAME widths independently. Nothing in the type ties
// the two together, so a change to either walk silently desynchronizes them
// — and the symptom is the nastiest kind: the hover band lights up one
// title while the pointer is actually over its neighbour, and the popup
// opens under a third. Pinning the painted chrome against cellRect keeps
// the two walks honest.
//
// Both branches are exercised in one frame: cell 0's menu is OPEN (sunken,
// the pressed look) while cell 1 is HOVERED (navy band + HighlightText).
// hoverIdx is set directly for the second half — with a popup open the
// OverlayHost holds the pointer capture and forwards moves to the popup, so
// a router-driven Move over the bar would never reach MenuBar.OnPointer;
// the hover path itself is already covered by the menu behavior tests.
func TestMenuBarRenderCellChromeMatchesHitTesting(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	bar, host, r, _ := newTestMenuBar(t)
	openMenuPopupRows(t, bar, host, r, 0) // File open: openIdx == 0

	if bar.openIdx != 0 {
		t.Fatalf("openIdx = %d, want 0", bar.openIdx)
	}
	bar.hoverIdx = 1 // Edit hovered
	bounds := bar.Bounds()

	rr := newClipRecorder()
	bar.Render(rr)

	if got := rr.fills[0]; got.rect != bounds || got.color != bar.colors.ButtonFace {
		t.Fatalf("first fill = %+v, want the bar's ButtonFace %v backing fill over %+v",
			got, bar.colors.ButtonFace, bounds)
	}
	assertPaintedWithinBounds(t, rr, bounds, "MenuBar.Render")

	openCell := bar.cellRect(0)
	hoverCell := bar.cellRect(1)

	// The hovered cell's band must land exactly on cellRect(1) — the same
	// rect cellAt maps a pointer into.
	var band *filledRect
	for i := range rr.fills {
		f := rr.fills[i]
		if f.color == bar.colors.Highlight && f.rect.W > 1 && f.rect.H > 1 {
			band = &rr.fills[i]
			break
		}
	}
	if band == nil {
		t.Fatal("no Highlight band drawn for the hovered cell")
	}
	if band.rect != hoverCell {
		t.Fatalf("hover band = %+v, want cellRect(1) = %+v — Render's cell walk has drifted from cellAt's",
			band.rect, hoverCell)
	}

	// The open cell gets the sunken look instead: a ButtonFace face fill
	// covering exactly cellRect(0), and no Highlight band anywhere over it.
	var sunkenFace bool
	for _, f := range rr.fills {
		if f.rect == openCell && f.color == bar.colors.ButtonFace {
			sunkenFace = true
		}
		if f.color == bar.colors.Highlight && f.rect.X >= openCell.X && f.rect.Right() <= openCell.Right() {
			t.Fatalf("Highlight fill %+v painted over the OPEN cell %+v, want the sunken look only",
				f.rect, openCell)
		}
	}
	if !sunkenFace {
		t.Fatalf("no ButtonFace face fill at the open cell %+v: the pressed/sunken look is missing", openCell)
	}

	// Sunken and raised differ only in which tone lands on which edge;
	// pin the sunken direction so a swapped drawSunken/drawRaised is caught.
	var shadowTop bool
	for _, f := range rr.fills {
		if f.rect.X == openCell.X && f.rect.Y == openCell.Y && f.rect.H == 1 &&
			f.rect.W == openCell.W && f.color == bar.colors.ButtonShadow {
			shadowTop = true
		}
	}
	if !shadowTop {
		t.Error("no ButtonShadow edge along the open cell's top: it reads raised, not pressed in")
	}
}

// TestMenuBarRenderNilFaceDrawsNothing covers Render's nil-face early
// return, the package-wide "a nil face renders nothing" convention
// (TextBlock, tabStrip). Without the guard the loop would still paint every
// cell's backing chrome under titles that were never drawn, leaving a bar
// of blank buttons.
func TestMenuBarRenderNilFaceDrawsNothing(t *testing.T) {
	theme.SetActive(theme.Light())
	defer theme.SetActive(nil)

	bar := NewMenuBar(nil)
	bar.AddMenu("File").Add("New", nil)
	bar.AddMenu("Edit").Add("Undo", nil)
	bar.hoverIdx = 0

	core.MeasureWidget(bar, render.Size{W: 300, H: 300})
	core.ArrangeWidget(bar, render.Rect{X: 0, Y: 0, W: 300, H: 24})

	rr := newClipRecorder()
	bar.Render(rr)

	if len(rr.fills) != 0 {
		t.Fatalf("a nil-face MenuBar emitted %d fills, want none", len(rr.fills))
	}
}
