package controls

import (
	"testing"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
)

// layoutSlider measures then arranges w at the given absolute rect — same
// pattern as layoutButton (button_test.go), duplicated under a
// slider-specific name only because layoutButton's doc comment is written
// in terms of buttons; the implementation is identical (and could equally
// well be reused directly, but a descriptive name at each call site reads
// better in the slider tests below).
func layoutSlider(w core.Widget, bounds render.Rect) {
	core.MeasureWidget(w, render.Size{W: bounds.W, H: bounds.H})
	core.ArrangeWidget(w, bounds)
}

func TestSliderDefaultRangeAndValue(t *testing.T) {
	s := NewSlider()
	if s.Min() != 0 || s.Max() != 1 || s.Value() != 0 {
		t.Fatalf("NewSlider() = {Min:%v Max:%v Value:%v}, want {0 1 0}", s.Min(), s.Max(), s.Value())
	}
}

func TestSliderMeasuresToFixedDesiredSize(t *testing.T) {
	s := NewSlider()
	core.MeasureWidget(s, render.Size{W: 1000, H: 1000})
	d := core.DesiredSizeOf(s)
	if d.W != 160 || d.H != 24 {
		t.Fatalf("DesiredSize() = %v, want {160 24}", d)
	}
}

func TestSliderSetValueClampsBeyondRange(t *testing.T) {
	s := NewSlider().SetRange(0, 10)

	s.SetValue(-5)
	if s.Value() != 0 {
		t.Fatalf("SetValue(-5) with range [0,10] = %v, want 0", s.Value())
	}

	s.SetValue(50)
	if s.Value() != 10 {
		t.Fatalf("SetValue(50) with range [0,10] = %v, want 10", s.Value())
	}

	s.SetValue(4)
	if s.Value() != 4 {
		t.Fatalf("SetValue(4) with range [0,10] = %v, want 4", s.Value())
	}
}

func TestSliderSetRangeReClampsValue(t *testing.T) {
	s := NewSlider().SetRange(0, 10)
	s.SetValue(8)

	s.SetRange(0, 5)
	if s.Value() != 5 {
		t.Fatalf("Value() after shrinking Max below current Value = %v, want 5 (re-clamped)", s.Value())
	}

	s.SetRange(2, 5)
	if s.Value() != 5 {
		t.Fatalf("Value() after raising Min (still <= current Value) = %v, want 5 (unaffected)", s.Value())
	}

	s.SetRange(6, 10)
	if s.Value() != 6 {
		t.Fatalf("Value() after raising Min above current Value = %v, want 6 (re-clamped)", s.Value())
	}
}

// TestSliderSetValueAndSetRangeAreSilent is the Phase 5 final-fix regression
// for the uniform silent-setter convention (Important #3, controller
// decision option A): Slider joins CheckBox/ToggleSwitch/ToggleButton/
// ComboBox/TextBox in never firing OnChanged from a programmatic setter —
// SetValue and SetRange's re-clamp are both silent even when they produce a
// REAL change to Value().
func TestSliderSetValueAndSetRangeAreSilent(t *testing.T) {
	s := NewSlider().SetRange(0, 10)

	var got []float32
	s.OnChanged(func(v float32) { got = append(got, v) })

	s.SetValue(3)    // programmatic, real change: silent
	s.SetValue(3)    // programmatic, no-op: silent
	s.SetRange(0, 2) // re-clamp, real change (3 -> 2): silent

	if len(got) != 0 {
		t.Fatalf("OnChanged calls = %v, want none (SetValue/SetRange are silent)", got)
	}
	if s.Value() != 2 {
		t.Fatalf("Value() after SetValue(3) then SetRange(0,2) = %v, want 2 (still clamps/mutates, just silently)", s.Value())
	}
}

// TestSliderOnChangedFiresOnUserDrivenChange proves the OTHER half of the
// uniform contract: user-driven paths (here, click-on-track) still fire
// OnChanged, only when the value actually changes.
func TestSliderOnChangedFiresOnUserDrivenChange(t *testing.T) {
	s := NewSlider().SetRange(0, 200)
	layoutSlider(s, render.Rect{X: 0, Y: 0, W: 160, H: 24})

	var got []float32
	s.OnChanged(func(v float32) { got = append(got, v) })

	r := input.NewRouter()
	r.SetRoot(s)
	r.PointerButton(input.ButtonLeft, true, render.Point{X: 8, Y: 12}, 0) // left edge: 0 (no-op, already 0)
	r.PointerButton(input.ButtonLeft, false, render.Point{X: 8, Y: 12}, 0)
	r.PointerButton(input.ButtonLeft, true, render.Point{X: 152, Y: 12}, 0) // right edge: 200 (real change)
	r.PointerButton(input.ButtonLeft, false, render.Point{X: 152, Y: 12}, 0)

	if len(got) != 1 || got[0] != 200 {
		t.Fatalf("OnChanged calls = %v, want [200] (fires once, only on the real user-driven change)", got)
	}
}

// TestSliderPressAtSeventyFivePercentJumpsValue is the drag-math golden
// case from the task brief: pressing at 75% across the USABLE track span
// (which insets by thumbRadius on each side, per the type doc comment)
// jumps the value to Min + 0.75*(Max-Min), within a 0.01 tolerance.
func TestSliderPressAtSeventyFivePercentJumpsValue(t *testing.T) {
	s := NewSlider().SetRange(0, 200)
	layoutSlider(s, render.Rect{X: 0, Y: 0, W: 160, H: 24})

	// Usable span = width - 2*thumbRadius = 160 - 16 = 144.
	// 75% across that span, offset by the left inset: 8 + 0.75*144 = 116.
	r := input.NewRouter()
	r.SetRoot(s)
	pos := render.Point{X: 116, Y: 12}
	r.PointerButton(input.ButtonLeft, true, pos, 0)

	want := float32(0.75 * 200)
	if diff := s.Value() - want; diff > 0.01 || diff < -0.01 {
		t.Fatalf("Value() after press at 75%% = %v, want ~%v (tolerance 0.01)", s.Value(), want)
	}
}

func TestSliderPressAtEdgesClampsToMinMax(t *testing.T) {
	s := NewSlider().SetRange(0, 200)
	layoutSlider(s, render.Rect{X: 0, Y: 0, W: 160, H: 24})

	r := input.NewRouter()
	r.SetRoot(s)

	r.PointerButton(input.ButtonLeft, true, render.Point{X: 0, Y: 12}, 0)
	r.PointerButton(input.ButtonLeft, false, render.Point{X: 0, Y: 12}, 0)
	if s.Value() != 0 {
		t.Fatalf("Value() after press at track's left edge = %v, want 0", s.Value())
	}

	// Bounds.Contains is half-open ([X, Right)), so the rightmost hittable
	// point is just under the bounds' own Right() (160), not 160 itself.
	r.PointerButton(input.ButtonLeft, true, render.Point{X: 159.99, Y: 12}, 0)
	r.PointerButton(input.ButtonLeft, false, render.Point{X: 159.99, Y: 12}, 0)
	if s.Value() != 200 {
		t.Fatalf("Value() after press at track's right edge = %v, want 200", s.Value())
	}
}

// TestSliderCapturedDragUpdatesContinuously proves Move events, delivered
// while the slider holds the router's pointer capture, keep updating the
// value as the pointer moves — not just the initial Press.
func TestSliderCapturedDragUpdatesContinuously(t *testing.T) {
	s := NewSlider().SetRange(0, 100)
	layoutSlider(s, render.Rect{X: 0, Y: 0, W: 160, H: 24})

	r := input.NewRouter()
	r.SetRoot(s)

	r.PointerButton(input.ButtonLeft, true, render.Point{X: 8, Y: 12}, 0) // left edge: 0
	if s.Value() != 0 {
		t.Fatalf("Value() after initial press = %v, want 0", s.Value())
	}

	r.PointerMove(render.Point{X: 152, Y: 12}, 0) // right edge: 100
	if s.Value() != 100 {
		t.Fatalf("Value() after captured Move to right edge = %v, want 100", s.Value())
	}

	r.PointerMove(render.Point{X: 80, Y: 12}, 0) // midpoint: 50
	if diff := s.Value() - 50; diff > 0.01 || diff < -0.01 {
		t.Fatalf("Value() after captured Move to midpoint = %v, want ~50", s.Value())
	}

	r.PointerButton(input.ButtonLeft, false, render.Point{X: 80, Y: 12}, 0)
	if r.Captured() != nil {
		t.Fatal("Captured() after Release, want nil")
	}
}

func TestSliderArrowKeysStepByOneAndTenPercentWhenFocused(t *testing.T) {
	s := NewSlider().SetRange(0, 100)
	layoutSlider(s, render.Rect{X: 0, Y: 0, W: 160, H: 24})
	s.SetValue(50)

	r := input.NewRouter()
	r.SetRoot(s)
	r.Focus(s)

	r.KeyDown(input.KeyRight, 0, 0)
	if s.Value() != 51 {
		t.Fatalf("Value() after Right = %v, want 51 (+1%%)", s.Value())
	}

	r.KeyDown(input.KeyLeft, 0, 0)
	if s.Value() != 50 {
		t.Fatalf("Value() after Left = %v, want 50 (-1%%)", s.Value())
	}

	r.KeyDown(input.KeyRight, 0, input.ModShift)
	if s.Value() != 60 {
		t.Fatalf("Value() after Shift+Right = %v, want 60 (+10%%)", s.Value())
	}

	r.KeyDown(input.KeyLeft, 0, input.ModShift)
	if s.Value() != 50 {
		t.Fatalf("Value() after Shift+Left = %v, want 50 (-10%%)", s.Value())
	}
}

// TestSliderVerticalArrowKeysStepAlongMainAxis pins the orientation branch
// OnKey used to be missing: the handler only ever looked at Left/Right, so a
// focused VERTICAL slider — whose thumb travels up and down — could not be
// moved with the arrow keys that point along its own track, while the two
// that point across it worked. Up is the increasing direction, since Max sits
// at the top of a vertical track.
func TestSliderVerticalArrowKeysStepAlongMainAxis(t *testing.T) {
	s := NewSlider().SetOrientation(Vertical).SetRange(0, 100)
	layoutSlider(s, render.Rect{X: 0, Y: 0, W: 24, H: 160})
	s.SetValue(50)

	r := input.NewRouter()
	r.SetRoot(s)
	r.Focus(s)

	r.KeyDown(input.KeyUp, 0, 0)
	if s.Value() != 51 {
		t.Fatalf("Value() after Up = %v, want 51 (+1%%, Up moves toward Max at the top)", s.Value())
	}

	r.KeyDown(input.KeyDown, 0, 0)
	if s.Value() != 50 {
		t.Fatalf("Value() after Down = %v, want 50 (-1%%)", s.Value())
	}

	r.KeyDown(input.KeyUp, 0, input.ModShift)
	if s.Value() != 60 {
		t.Fatalf("Value() after Shift+Up = %v, want 60 (+10%%)", s.Value())
	}

	r.KeyDown(input.KeyDown, 0, input.ModShift)
	if s.Value() != 50 {
		t.Fatalf("Value() after Shift+Down = %v, want 50 (-10%%)", s.Value())
	}
}

// TestSliderArrowKeysIgnoreTheOffAxisPair proves each orientation answers
// only the arrow pair running along its own track, leaving the other pair
// unhandled so it keeps bubbling to whatever else might want it.
func TestSliderArrowKeysIgnoreTheOffAxisPair(t *testing.T) {
	vert := NewSlider().SetOrientation(Vertical).SetRange(0, 100)
	layoutSlider(vert, render.Rect{X: 0, Y: 0, W: 24, H: 160})
	vert.SetValue(50)

	for _, k := range []input.Key{input.KeyLeft, input.KeyRight} {
		e := &input.KeyEvent{Action: input.Press, Key: k}
		vert.OnKey(e)
		if vert.Value() != 50 || e.Handled {
			t.Fatalf("vertical slider after %v: Value()=%v Handled=%v, want 50 and unhandled", k, vert.Value(), e.Handled)
		}
	}

	horiz := NewSlider().SetRange(0, 100)
	layoutSlider(horiz, render.Rect{X: 0, Y: 0, W: 160, H: 24})
	horiz.SetValue(50)

	for _, k := range []input.Key{input.KeyUp, input.KeyDown} {
		e := &input.KeyEvent{Action: input.Press, Key: k}
		horiz.OnKey(e)
		if horiz.Value() != 50 || e.Handled {
			t.Fatalf("horizontal slider after %v: Value()=%v Handled=%v, want 50 and unhandled", k, horiz.Value(), e.Handled)
		}
	}
}

func TestSliderArrowKeysIgnoredWhenNotFocused(t *testing.T) {
	s := NewSlider().SetRange(0, 100)
	s.SetValue(50)
	host := NewStackPanel(Horizontal).Add(s)

	r := input.NewRouter()
	r.SetRoot(host)
	layoutSlider(host, render.Rect{X: 0, Y: 0, W: 200, H: 24})

	r.KeyDown(input.KeyRight, 0, 0)
	if s.Value() != 50 {
		t.Fatalf("Value() after Right with no focus = %v, want 50 (unaffected)", s.Value())
	}
}

func TestSliderDisabledIgnoresPointerAndKeyAndFocus(t *testing.T) {
	s := NewSlider().SetRange(0, 100).SetEnabled(false)
	layoutSlider(s, render.Rect{X: 0, Y: 0, W: 160, H: 24})

	if s.AcceptsFocus() {
		t.Fatal("AcceptsFocus() = true for a disabled slider, want false")
	}

	press := &input.PointerEvent{Action: input.Press, Pos: render.Point{X: 116, Y: 12}}
	s.OnPointer(press)
	if press.Handled {
		t.Fatal("Press on a disabled slider set Handled = true, want false")
	}
	if s.Value() != 0 {
		t.Fatalf("Value() after Press on disabled slider = %v, want 0 (unaffected)", s.Value())
	}

	key := &input.KeyEvent{Action: input.Press, Key: input.KeyRight}
	s.OnKey(key)
	if key.Handled {
		t.Fatal("Right key on a disabled slider set Handled = true, want false")
	}
	if s.Value() != 0 {
		t.Fatalf("Value() after Right key on disabled slider = %v, want 0 (unaffected)", s.Value())
	}
}

// TestSliderSetEnabledFalseMidDragReleasesCapture is the Phase 5 final-fix
// regression for the mid-drag disable wedge (Important #2, joint with
// TextBox): capturing a drag via Press, then disabling the slider WHILE
// still captured, must release the router's capture — otherwise every
// subsequent pointer event keeps routing to this now-disabled, unwilling
// slider forever (deliverCaptured, never hit-testing again), wedging the
// whole app's pointer input. Proven end-to-end: after the disable+release,
// Captured() is nil and a fresh press actually reaches an unrelated probe.
func TestSliderSetEnabledFalseMidDragReleasesCapture(t *testing.T) {
	s := NewSlider().SetRange(0, 100)
	probe := &ovProbe{}
	probe.SetWidth(50)
	probe.SetHeight(50)

	canvas := NewCanvas().Add(s, 0, 0).Add(probe, 200, 0)

	r := input.NewRouter()
	r.SetRoot(canvas)
	layoutSlider(canvas, render.Rect{X: 0, Y: 0, W: 400, H: 60})

	r.PointerButton(input.ButtonLeft, true, render.Point{X: 80, Y: 12}, 0) // press on the slider: captures
	if got := r.Captured(); got != core.Widget(s) {
		t.Fatalf("Captured() after Press = %v, want slider", got)
	}

	s.SetEnabled(false) // disabled WHILE still mid-drag

	r.PointerButton(input.ButtonLeft, false, render.Point{X: 80, Y: 12}, 0) // Release, delivered captured
	if got := r.Captured(); got != nil {
		t.Fatalf("Captured() after Release while disabled mid-drag = %v, want nil (no wedge)", got)
	}

	r.PointerButton(input.ButtonLeft, true, render.Point{X: 210, Y: 10}, 0) // must reach the probe now
	if got := probe.events; len(got) != 1 || got[0] != "press" {
		t.Fatalf("probe.events = %v, want [press] (pointer flow restored, not permanently wedged)", got)
	}
}

func TestSliderFocusRingTracked(t *testing.T) {
	s := NewSlider()
	if s.focused {
		t.Fatal("focused = true before any focus change, want false")
	}
	s.OnFocusChanged(true)
	if !s.focused {
		t.Fatal("focused = false after OnFocusChanged(true), want true")
	}
	s.OnFocusChanged(false)
	if s.focused {
		t.Fatal("focused = true after OnFocusChanged(false), want false")
	}
}

// TestSliderDefaultOrientationIsHorizontal pins the opt-in invariant: a
// bare NewSlider() is Horizontal, matching every existing test/golden in
// this file (none of which call SetOrientation).
func TestSliderDefaultOrientationIsHorizontal(t *testing.T) {
	s := NewSlider()
	if s.orientation != Horizontal {
		t.Fatalf("NewSlider().orientation = %v, want Horizontal", s.orientation)
	}
}

// TestSliderVerticalMeasuresToSwappedDesiredSize proves Vertical's
// MeasureContent swaps the fixed {160, 24} Horizontal desired size to a
// tall-narrow {24, 160}, per the type doc comment.
func TestSliderVerticalMeasuresToSwappedDesiredSize(t *testing.T) {
	s := NewSlider().SetOrientation(Vertical)
	core.MeasureWidget(s, render.Size{W: 1000, H: 1000})
	d := core.DesiredSizeOf(s)
	if d.W != 24 || d.H != 160 {
		t.Fatalf("DesiredSize() = %v, want {24 160}", d)
	}
}

// TestSliderThumbCenterVerticalMaxAtTopMinAtBottom pins the vertical value-
// to-position mapping directly: thumbCenter (the Y coordinate the thumb is
// painted at and the exact inverse valueFromLocal reproduces) sits at the
// TOP edge inset (thumbRadius) when Value==Max, and at the BOTTOM edge
// inset (axisLength-thumbRadius) when Value==Min — the reverse of the
// Horizontal convention, per the type doc comment.
func TestSliderThumbCenterVerticalMaxAtTopMinAtBottom(t *testing.T) {
	s := NewSlider().SetOrientation(Vertical).SetRange(0, 100)
	layoutSlider(s, render.Rect{X: 0, Y: 0, W: 24, H: 160})

	s.SetValue(100)
	if got, want := s.thumbCenter(), float32(8); got != want {
		t.Fatalf("thumbCenter() at Value=Max(100) = %v, want %v (top edge inset)", got, want)
	}

	s.SetValue(0)
	if got, want := s.thumbCenter(), float32(152); got != want {
		t.Fatalf("thumbCenter() at Value=Min(0) = %v, want %v (bottom edge inset)", got, want)
	}
}

// TestSliderVerticalPressAtTopAndBottomClampsToMaxAndMin is the Vertical
// counterpart of TestSliderPressAtEdgesClampsToMinMax: pressing the track's
// TOP edge jumps to Max, and the bottom edge jumps to Min — reversed from
// Horizontal's left-edge-is-Min/right-edge-is-Max, per the type doc
// comment's vertical convention.
func TestSliderVerticalPressAtTopAndBottomClampsToMaxAndMin(t *testing.T) {
	s := NewSlider().SetOrientation(Vertical).SetRange(0, 200)
	layoutSlider(s, render.Rect{X: 0, Y: 0, W: 24, H: 160})

	r := input.NewRouter()
	r.SetRoot(s)

	r.PointerButton(input.ButtonLeft, true, render.Point{X: 12, Y: 0}, 0)
	r.PointerButton(input.ButtonLeft, false, render.Point{X: 12, Y: 0}, 0)
	if s.Value() != 200 {
		t.Fatalf("Value() after press at track's top edge = %v, want 200 (Max)", s.Value())
	}

	// Bounds.Contains is half-open, so the bottom-most hittable point is
	// just under Bounds().Bottom() (160), not 160 itself — mirroring
	// TestSliderPressAtEdgesClampsToMinMax's 159.99 on the X axis.
	r.PointerButton(input.ButtonLeft, true, render.Point{X: 12, Y: 159.99}, 0)
	r.PointerButton(input.ButtonLeft, false, render.Point{X: 12, Y: 159.99}, 0)
	if s.Value() != 0 {
		t.Fatalf("Value() after press at track's bottom edge = %v, want 0 (Min)", s.Value())
	}
}

// TestSliderVerticalPressAtSeventyFivePercentJumpsTowardMin is the Vertical
// counterpart of TestSliderPressAtSeventyFivePercentJumpsValue: pressing
// 75% of the way DOWN the usable span (top-anchored, since Max is at the
// top) lands the value 75% of the way from Max toward Min.
func TestSliderVerticalPressAtSeventyFivePercentJumpsTowardMin(t *testing.T) {
	s := NewSlider().SetOrientation(Vertical).SetRange(0, 200)
	layoutSlider(s, render.Rect{X: 0, Y: 0, W: 24, H: 160})

	// Usable span = height - 2*thumbRadius = 160 - 16 = 144. 75% down from
	// the top, offset by the top inset: 8 + 0.75*144 = 116.
	r := input.NewRouter()
	r.SetRoot(s)
	r.PointerButton(input.ButtonLeft, true, render.Point{X: 12, Y: 116}, 0)

	want := float32(200 - 0.75*200) // 75% of the way from Max(200) toward Min(0)
	if diff := s.Value() - want; diff > 0.01 || diff < -0.01 {
		t.Fatalf("Value() after press at 75%% down = %v, want ~%v (tolerance 0.01)", s.Value(), want)
	}
}

// TestSliderVerticalRenderFillsMinSideBelowThumb proves Render's fill
// geometry for Vertical: the Highlight band runs from the thumb's center
// DOWN to the track's bottom edge (the Min side), mirroring Horizontal's
// left-of-thumb fill — per the type doc comment.
func TestSliderVerticalRenderFillsMinSideBelowThumb(t *testing.T) {
	s := NewSlider().SetOrientation(Vertical).SetRange(0, 100).SetValue(50)
	layoutSlider(s, render.Rect{X: 0, Y: 0, W: 24, H: 160})

	rr := &recordRenderer{}
	s.Render(rr)

	thumbPos := s.thumbCenter()
	bounds := s.Bounds()

	var filled filledRect
	found := false
	for _, f := range rr.fills {
		if f.color == s.colors.Highlight {
			filled = f
			found = true
		}
	}
	if !found {
		t.Fatal("Render emitted no Highlight fill")
	}
	if filled.rect.Y != thumbPos {
		t.Fatalf("fill Y = %v, want %v (the thumb's center — fill starts there and extends down)", filled.rect.Y, thumbPos)
	}
	if wantH := bounds.Y + bounds.H - thumbPos; filled.rect.H != wantH {
		t.Fatalf("fill H = %v, want %v (extends to the track's bottom edge)", filled.rect.H, wantH)
	}
}

func TestSliderHoverTracked(t *testing.T) {
	s := NewSlider()
	layoutSlider(s, render.Rect{X: 0, Y: 0, W: 160, H: 24})

	rest := &recordRenderer{}
	s.Render(rest)

	s.OnPointer(&input.PointerEvent{Action: input.Enter})
	if !s.hover {
		t.Fatal("hover = false after Enter, want true")
	}
	hover := &recordRenderer{}
	s.Render(hover)

	// Only the thumb's face fill reacts to hover; the groove, the Highlight
	// band, and every bevel edge are painted identically either way.
	if len(rest.fills) != len(hover.fills) {
		t.Fatalf("hovered Render emitted %d FillRect calls, want %d (same as rest)", len(hover.fills), len(rest.fills))
	}
	var changed []int
	for i := range rest.fills {
		if rest.fills[i] != hover.fills[i] {
			changed = append(changed, i)
		}
	}
	if len(changed) != 1 {
		t.Fatalf("hover changed %d fills, want exactly 1 (the thumb face)", len(changed))
	}
	i := changed[0]
	if rest.fills[i].color != s.colors.ButtonFace || hover.fills[i].color != s.colors.ButtonLight {
		t.Fatalf("thumb face rest/hover = %v/%v, want ButtonFace %v / ButtonLight %v",
			rest.fills[i].color, hover.fills[i].color, s.colors.ButtonFace, s.colors.ButtonLight)
	}
	s.OnPointer(&input.PointerEvent{Action: input.Leave})
	if s.hover {
		t.Fatal("hover = true after Leave, want false")
	}
}
