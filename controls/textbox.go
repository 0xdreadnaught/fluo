package controls

import (
	"strings"
	"time"

	"github.com/0xdreadnaught/fluo/core"
	"github.com/0xdreadnaught/fluo/input"
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/text"
	"github.com/0xdreadnaught/fluo/theme"
	"github.com/0xdreadnaught/fluo/timers"
)

// textBoxDefaultWidth is the desired content width reported by
// MeasureContent when no explicit width has been set on the TextBox (via
// core.Element.SetWidth) — a sane single-line-input default, per the Phase 5
// Task 5 spec. An explicit SetWidth overrides it through the normal
// core.MeasureWidget explicit-size precedence (see core/widget.go); TextBox's
// own MeasureContent never needs to look at the available size to honor
// that.
const textBoxDefaultWidth float32 = 160

// caretBlinkPeriod is the interval at which caretVisible toggles once a
// timers.Queue has been wired via SetTimers (normative: 530ms, matching
// common desktop UI caret-blink cadences).
const caretBlinkPeriod = 530 * time.Millisecond

// caretWidth is the drawn width of the caret bar (normative: 1.5, a hair
// wider than a hairline 1px rule so it stays visible after SDF/AA rounding).
const caretWidth float32 = 1.5

// TextBox is a focusable, token-styled text input, single-line by default.
// The data model (text/caret/selection, rune-indexed) and rendering (chrome,
// selection highlight, caret, horizontal scroll, placeholder) were built in
// Phase 5 Task 5; Task 6 added the interaction layer: OnKey (the normative
// keyboard map — rune insertion, Backspace/Delete, arrow/Home/End caret
// movement with Shift-extend, Ctrl+A/C/X/V) and OnPointer (click-to-caret,
// drag-to-select, CursorShaper's CursorIBeam). TextBox implements
// input.Focusable and input.FocusHandler (AcceptsFocus/OnFocusChanged) since
// focus also drives the focus-ring overlay and the focused border color.
// SetMultiline opts into multi-line mode (default off — single-line
// behavior is unchanged either way); see its doc comment for the full list
// of what changes.
//
// Rune indices throughout (Caret, Selection, SetCaret, Select) are RUNE
// indices into Text(), not byte offsets — text is stored as a SINGLE []rune
// buffer internally (multi-line included: lines are '\n'-delimited runs
// within it, not separate strings) so multibyte characters (e.g. "héllo")
// never split a codepoint. v0 simplification: every index-based operation,
// including the multi-line line/column mapping (lineCol, lineStart, lineEnd,
// indexOfLineCol), is O(n) (converts via []rune / string(runes[...]) as
// needed); fine for realistic input lengths.
type TextBox struct {
	core.Element

	face *text.Face

	runes       []rune
	placeholder string

	// caret is the actual caret position; anchor is the other end of the
	// selection (equal to caret when there is no selection). Selection()
	// normalizes these into start<=end; Caret() always reports the raw
	// caret value (which may be either endpoint after Select).
	caret, anchor int

	enabled bool
	focused bool

	// hscroll is the horizontal text-scroll offset in logical px, clamped
	// during ArrangeContent (see updateHScroll) so the caret always stays
	// within the inner (padding-inset) width — the ScrollViewer
	// clamp-in-arrange pattern applied to a single scroll axis. In multi-line
	// mode this offset is shared by every line (see caretX/caretLineWidth):
	// only the CARET's own line determines it, matching the "no wrap, scroll
	// like single-line, per line" simplification described on SetMultiline.
	hscroll float32

	// multiline toggles multi-line mode (see SetMultiline); false (the
	// zero value) is the original single-line behavior, unchanged.
	multiline bool

	// vscroll is the vertical text-scroll offset in logical px, the
	// multi-line analogue of hscroll: clamped during ArrangeContent (see
	// updateVScroll) so the caret's line stays within the inner
	// (padding-inset) height. Always 0 in single-line mode.
	vscroll float32

	// desiredCol and desiredColValid track the "desired column" Up/Down
	// navigation preserves across lines shorter than it (see
	// moveCaretVertical): desiredColValid is false whenever the caret last
	// moved horizontally (typing, Left/Right, Home/End, a click — anything
	// that goes through SetCaret/Select/replaceRange directly), so the next
	// Up/Down recomputes it from the caret's actual column; a run of
	// consecutive Up/Down presses instead keeps reusing the same
	// desiredCol, the standard desktop-editor caret-column convention.
	desiredCol      int
	desiredColValid bool

	// timerQueue and blinkTimer drive caret blinking once SetTimers wires a
	// non-nil queue; nil timerQueue means "solid caret" (caretShown ignores
	// caretVisible entirely in that case). blinkTimer is stopped and
	// replaced whenever SetTimers is called again, so a superseded queue can
	// never keep toggling a textbox that has moved on to a different one
	// (or none).
	timerQueue   *timers.Queue
	blinkTimer   *timers.Timer
	caretVisible bool

	onChanged func(string)

	colors  theme.ColorTokens
	metrics theme.MetricTokens
}

// NewTextBox returns an enabled, empty, unfocused TextBox drawing text with
// face (face may be nil, in which case it measures/draws no text — a
// degenerate but valid state, matching TextBlock's nil-face convention).
// Colors and metrics are captured from theme.Active() at construction
// (rebuild to re-theme, matching every other control in this package).
func NewTextBox(face *text.Face) *TextBox {
	th := theme.Active()
	return &TextBox{
		face:         face,
		enabled:      true,
		caretVisible: true,
		colors:       th.Color,
		metrics:      th.Metric,
	}
}

// Text returns the current text content.
func (t *TextBox) Text() string {
	return string(t.runes)
}

// SetText replaces the text content, resets the caret to the end, and
// clears any selection. Task 6 carry-in fix B: SetText is a complete no-op —
// no invalidation, caret/selection left untouched — when s already equals
// the current text; this makes it safe for an OnChanged handler to call
// SetText with the same value it was just notified of (a common
// data-binding pattern) without triggering pointless re-layout. SetText is
// SILENT (never fires OnChanged) regardless of whether the text actually
// changes — fluo's uniform setter convention (matching CheckBox/
// ToggleSwitch/ToggleButton/ComboBox/Slider): programmatic setters are
// silent, OnChanged reports only user-driven changes (typing, Backspace/
// Delete, Ctrl+X, Ctrl+V — every path that funnels through replaceRange).
// When the text does change, SetText calls InvalidateArrange (carry-in fix
// A) so hscroll re-clamps to the new caret position on the next layout
// pass; InvalidateMeasure is NOT needed since MeasureContent's desired size
// (textBoxDefaultWidth, fixed) never depends on the text content. Also
// restarts the caret blink phase (restartBlink), matching every other
// caret-moving mutation — but only on the real-change path; the
// no-op-on-equal path above leaves the blink phase (like everything else)
// untouched.
func (t *TextBox) SetText(s string) *TextBox {
	if s == string(t.runes) {
		return t
	}
	t.runes = []rune(s)
	t.caret = len(t.runes)
	t.anchor = t.caret
	t.InvalidateArrange()
	t.restartBlink()
	return t
}

// SetPlaceholder sets the text shown (in TextDisabled color) whenever
// Text() == "", regardless of focus state — simpler and more common than
// hiding it while focused, and the normative choice for this control.
func (t *TextBox) SetPlaceholder(s string) *TextBox {
	t.placeholder = s
	return t
}

// SetMultiline toggles multi-line mode; false (the default) is the original
// single-line TextBox behavior, byte-for-byte unchanged. Enabling it changes:
//   - Enter inserts a '\n' at the caret instead of being ignored (see OnKey).
//   - Up/Down move the caret between lines, preserving a "desired column"
//     across lines shorter than it (see moveCaretVertical); Home/End operate
//     on the caret's CURRENT line rather than the whole text; Left/Right
//     already cross line boundaries for free, since '\n' is just another rune
//     in the single underlying []rune buffer.
//   - Ctrl+V paste keeps newlines (normalized to '\n') instead of stripping
//     them (see pasteClipboard); Ctrl+C/X/select-all/backspace/delete are
//     unaffected — they already operate on rune ranges that may contain '\n'.
//   - MeasureContent reports a taller default (a few line-heights instead of
//     one), and Render/ArrangeContent draw/scroll a second, vertical axis
//     (vscroll) alongside the existing horizontal one (hscroll) — see
//     renderMultiline and updateVScroll. Content still hard-wraps only at an
//     explicit '\n' (no word-wrap); a line wider than the box scrolls
//     horizontally exactly like the single-line box did, keyed off the
//     CARET's own line (see caretX/caretLineWidth).
//
// Purely a mode flag: does not itself touch existing text/caret/selection/
// scroll state, but — unlike most setters in this file — it DOES call
// InvalidateMeasure (not just InvalidateArrange), since it can change
// MeasureContent's answer (the taller multi-line default height).
func (t *TextBox) SetMultiline(v bool) *TextBox {
	t.multiline = v
	t.InvalidateMeasure()
	return t
}

// Multiline reports whether multi-line mode is enabled (see SetMultiline).
func (t *TextBox) Multiline() bool {
	return t.multiline
}

// SetEnabled toggles whether the box accepts focus and pointer/keyboard
// editing (OnKey/OnPointer both ignore all input while disabled). Purely
// visual/behavioral: no invalidation needed.
func (t *TextBox) SetEnabled(v bool) *TextBox {
	t.enabled = v
	return t
}

// OnChanged sets the callback fired with the new text whenever the USER
// changes it — every editing mutation (typing, Backspace/Delete, Ctrl+X,
// Ctrl+V) — but never for a programmatic SetText (fluo's uniform setter
// convention: programmatic setters are silent, OnChanged reports only
// user-driven changes). Replaces any previously set callback; a nil fn is a
// valid, silent no-op.
func (t *TextBox) OnChanged(fn func(string)) *TextBox {
	t.onChanged = fn
	return t
}

// SetTimers wires q as the caret-blink driver: SetTimers schedules a
// repeating callback every caretBlinkPeriod that flips caretVisible, and the
// caret is drawn only while caretVisible is true. Passing nil detaches any
// previously wired queue and reverts to a solid (always-visible-while-
// focused) caret. Calling SetTimers again (with a different queue, or nil)
// always stops the previously scheduled timer first, so a superseded queue
// can never keep toggling this textbox's caret after the fact.
func (t *TextBox) SetTimers(q *timers.Queue) *TextBox {
	t.timerQueue = q
	t.restartBlink()
	return t
}

// restartBlink resets caretVisible to true and, if a timers.Queue is
// currently wired (t.timerQueue), stops whatever blink timer is running and
// starts a fresh one — restarting the blink phase from "visible" rather
// than wherever it happened to be. Called from SetTimers (wiring a new
// queue), OnFocusChanged(true) (regaining focus), and every caret-affecting
// mutation (SetCaret, Select, replaceRange, and therefore SetText) so the
// caret is always visible the instant it moves, never left mid-blink-off
// from before the input landed. A nil t.timerQueue leaves blinkTimer nil
// (solid-caret mode, caretVisible is set true but caretShown() ignores it
// regardless per its own doc comment) — restartBlink is safe to call
// unconditionally.
func (t *TextBox) restartBlink() {
	if t.blinkTimer != nil {
		t.blinkTimer.Stop()
		t.blinkTimer = nil
	}
	t.caretVisible = true
	if t.timerQueue != nil {
		t.blinkTimer = t.timerQueue.Every(caretBlinkPeriod, func() {
			t.caretVisible = !t.caretVisible
		})
	}
}

// Caret returns the current caret rune index (0..len(runes)).
func (t *TextBox) Caret() int {
	return t.caret
}

// Selection returns the selected rune range, normalized so start<=end.
// Returns (caret,caret) when there is no selection (anchor==caret).
func (t *TextBox) Selection() (start, end int) {
	if t.anchor < t.caret {
		return t.anchor, t.caret
	}
	return t.caret, t.anchor
}

// SetCaret moves the caret to rune index i (clamped to [0, len(runes)]) and
// clears any selection (anchor becomes equal to the new caret). Carry-in fix
// A: calls InvalidateArrange so hscroll re-clamps to the new caret position
// on the next layout pass, under the gallery's NeedsLayout guard. Also
// restarts the caret blink phase (restartBlink) so the caret is immediately
// visible at its new position rather than possibly stuck mid-blink-off.
func (t *TextBox) SetCaret(i int) *TextBox {
	i = clampInt(i, 0, len(t.runes))
	t.caret = i
	t.anchor = i
	t.desiredColValid = false
	t.InvalidateArrange()
	t.restartBlink()
	return t
}

// Select sets the selection to [anchor, caret) (each independently clamped
// to [0, len(runes)]); anchor may be greater than caret (Selection() always
// normalizes), and Caret() reports the raw caret argument afterward — it is
// the actual caret position, which is not necessarily the normalized range
// start. Carry-in fix A: calls InvalidateArrange so hscroll re-clamps to the
// new caret position on the next layout pass. Also restarts the caret blink
// phase (restartBlink), matching SetCaret.
func (t *TextBox) Select(anchor, caret int) *TextBox {
	t.anchor = clampInt(anchor, 0, len(t.runes))
	t.caret = clampInt(caret, 0, len(t.runes))
	t.desiredColValid = false
	t.InvalidateArrange()
	t.restartBlink()
	return t
}

// replaceRange replaces runes[start:end] with s (both rune indices; callers
// are responsible for passing a valid 0<=start<=end<=len(runes) range —
// every call site below derives start/end from Selection() or an
// already-clamped caret, so no further clamping happens here), moves the
// caret to just after the inserted text and clears the selection (anchor
// follows caret), invalidates arrange (carry-in fix A, so hscroll re-clamps
// next layout pass), and fires OnChanged with the new full text — every
// editing mutation is USER-driven (the keyboard/pointer handlers below are
// replaceRange's only callers), so it notifies unconditionally, per fluo's
// uniform setter convention (OnChanged reports user-driven changes;
// programmatic setters like SetText are silent — see SetText's doc
// comment).
//
// replaceRange is the single mutation primitive every editing operation
// below is built from: plain insertion is start==end (nothing removed),
// plain deletion is s=="" (nothing inserted), and typing/paste-over-a-
// selection is both at once — which is exactly the "selection-first delete"
// convention the Task 6 keyboard map specifies (delete-or-replace the
// selection is the SAME code path as an unselected edit, just with a
// non-empty [start,end)).
//
// Callers only invoke replaceRange when a real change is happening (e.g.
// deleteBackward checks there is something to delete before calling it), so
// OnChanged is fired unconditionally here rather than re-checking for a
// no-op — unlike SetText, which the caller invokes for arbitrary (possibly
// unchanged) input and must guard its own caret/invalidation side effects
// (though never OnChanged, which it never fires at all).
//
// Also restarts the caret blink phase (restartBlink): every edit moves the
// caret, so — matching SetCaret/Select — the caret must be immediately
// visible at its new position rather than possibly stuck mid-blink-off from
// before the keystroke landed.
func (t *TextBox) replaceRange(start, end int, s string) {
	ins := []rune(s)
	next := make([]rune, 0, len(t.runes)-(end-start)+len(ins))
	next = append(next, t.runes[:start]...)
	next = append(next, ins...)
	next = append(next, t.runes[end:]...)
	t.runes = next
	t.caret = start + len(ins)
	t.anchor = t.caret
	t.desiredColValid = false
	t.InvalidateArrange()
	t.restartBlink()
	if t.onChanged != nil {
		t.onChanged(string(t.runes))
	}
}

// insertText replaces the current selection (if any; a no-op range [caret,
// caret) when there is none) with s — the shared implementation for both
// rune-input insertion (s is a single rune) and Ctrl+V paste (s is the
// clipboard text, newlines already stripped by the caller).
func (t *TextBox) insertText(s string) {
	start, end := t.Selection()
	t.replaceRange(start, end, s)
}

// deleteBackward implements Backspace: delete the selection if one is
// active (selection-first convention), else delete the single rune before
// the caret; a no-op (no mutation, no OnChanged) at the very start of the
// text with no selection.
func (t *TextBox) deleteBackward() {
	if start, end := t.Selection(); start != end {
		t.replaceRange(start, end, "")
		return
	}
	if t.caret > 0 {
		t.replaceRange(t.caret-1, t.caret, "")
	}
}

// deleteForward implements Delete: delete the selection if one is active
// (selection-first convention), else delete the single rune after the
// caret; a no-op at the very end of the text with no selection.
func (t *TextBox) deleteForward() {
	if start, end := t.Selection(); start != end {
		t.replaceRange(start, end, "")
		return
	}
	if t.caret < len(t.runes) {
		t.replaceRange(t.caret, t.caret+1, "")
	}
}

// moveCaret shifts the caret by delta runes (clamped to [0, len(runes)]):
// with extend==false (no Shift) it collapses to the new position via
// SetCaret; with extend==true (Shift held) it extends the selection from
// the CURRENT anchor to the new position via Select, so anchor stays fixed
// across a run of Shift+Left/Right presses (the "extend selection from
// anchor" bullet of the keyboard map).
//
// WPF-parity carry-in fix: with extend==false AND an active selection
// (start != end), Left/Right (delta < 0 / > 0 respectively) collapses
// straight to that selection's start/end instead of moving delta runes from
// the raw caret — the standard desktop-text-box convention where an
// unshifted arrow key on a selection dismisses it at the near edge rather
// than additionally stepping past it. Only the FIRST unshifted arrow press
// after a selection exists behaves this way; once collapsed (start==end),
// subsequent presses fall through to the plain delta-from-caret move below.
func (t *TextBox) moveCaret(delta int, extend bool) {
	if !extend {
		if start, end := t.Selection(); start != end {
			if delta < 0 {
				t.SetCaret(start)
			} else {
				t.SetCaret(end)
			}
			return
		}
	}
	t.moveCaretTo(t.caret+delta, extend)
}

// moveCaretTo moves the caret to rune index pos (clamped by SetCaret/Select),
// either collapsing (extend==false) or extending the selection from the
// current anchor (extend==true) — shared by Left/Right (delta ±1) and
// Home/End (pos 0/len(runes)).
func (t *TextBox) moveCaretTo(pos int, extend bool) {
	if extend {
		t.Select(t.anchor, pos)
		return
	}
	t.SetCaret(pos)
}

// homeTarget returns the rune index Home should move the caret to: 0 in
// single-line mode — the whole text's start, unchanged — or, in multi-line
// mode, the start of the caret's OWN line (see SetMultiline).
func (t *TextBox) homeTarget() int {
	if !t.multiline {
		return 0
	}
	line, _ := t.lineCol(t.caret)
	return t.lineStart(line)
}

// endTarget is homeTarget's End counterpart: len(t.runes) in single-line
// mode (unchanged), or the end of the caret's own line in multi-line mode.
func (t *TextBox) endTarget() int {
	if !t.multiline {
		return len(t.runes)
	}
	line, _ := t.lineCol(t.caret)
	return t.lineEnd(line)
}

// stripCRLF removes \r and \n from s — Ctrl+V paste strips newlines per the
// keyboard map's "single-line" rule, so a multi-line clipboard string pastes
// as one run rather than being silently truncated at the first line or
// corrupting the single-line layout.
func stripCRLF(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, s)
}

// normalizeNewlines converts CRLF and lone-CR line endings in s to a bare
// '\n' — the multi-line paste path's counterpart to stripCRLF: a
// multi-line TextBox keeps newlines (unlike the single-line stripCRLF rule)
// but still stores exactly one line-ending rune per line internally, so a
// Windows-style CRLF (or old-Mac-style lone-CR) clipboard source pastes as
// the same single '\n'-delimited model every other mutation assumes.
func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// copySelection implements Ctrl+C: copies the selected text to r's
// clipboard. No-op (no mutation either way — Copy never edits the text) if
// there is no active selection or the router has no clipboard wired
// (headless/test routers, or a host that hasn't set one up yet).
func (t *TextBox) copySelection(r *input.Router) {
	clip := r.Clipboard()
	if clip == nil {
		return
	}
	start, end := t.Selection()
	if start == end {
		return
	}
	clip.Set(string(t.runes[start:end]))
}

// cutSelection implements Ctrl+X: copies the selected text to r's clipboard,
// then deletes it. No-op (nothing copied, nothing deleted) under the same
// conditions as copySelection (no selection, or no clipboard) — a cut that
// can't reach the clipboard must not destroy the selection.
func (t *TextBox) cutSelection(r *input.Router) {
	clip := r.Clipboard()
	if clip == nil {
		return
	}
	start, end := t.Selection()
	if start == end {
		return
	}
	clip.Set(string(t.runes[start:end]))
	t.replaceRange(start, end, "")
}

// pasteClipboard implements Ctrl+V: reads r's clipboard and replaces the
// current selection (or inserts at the caret when there is none) with the
// result — strips \r\n (single-line rule) in single-line mode, else
// normalizes CRLF/lone-CR to '\n' (multi-line mode keeps newlines; see
// SetMultiline). No-op if the router has no clipboard wired. Also a no-op —
// no mutation, no OnChanged — when the clipboard text is empty AND there is
// no selection to collapse: pasting nothing onto nothing is not a change.
func (t *TextBox) pasteClipboard(r *input.Router) {
	clip := r.Clipboard()
	if clip == nil {
		return
	}
	s := clip.Get()
	if t.multiline {
		s = normalizeNewlines(s)
	} else {
		s = stripCRLF(s)
	}
	start, end := t.Selection()
	if s == "" && start == end {
		return
	}
	t.replaceRange(start, end, s)
}

// AcceptsFocus implements input.Focusable: a disabled textbox never accepts
// focus.
func (t *TextBox) AcceptsFocus() bool {
	return t.enabled
}

// OnFocusChanged implements input.FocusHandler, tracking focus for the
// focused border color, the focus-ring overlay, and caret visibility.
// Gaining focus restarts the blink phase (restartBlink) so the caret starts
// out solidly visible rather than picking up mid-blink; losing focus stops
// the blink timer outright (caretShown() already returns false while
// unfocused regardless, but there is no reason to keep a hidden control's
// timer ticking every caretBlinkPeriod, and a bare Stop — without also
// clearing blinkTimer — would leave a stale, already-stopped *Timer
// reference behind that the next restartBlink call would redundantly
// re-Stop). A later refocus (with a timers.Queue still wired) restarts the
// timer fresh via restartBlink.
func (t *TextBox) OnFocusChanged(focused bool) {
	t.focused = focused
	if focused {
		t.restartBlink()
		return
	}
	if t.blinkTimer != nil {
		t.blinkTimer.Stop()
		t.blinkTimer = nil
	}
}

// lineHeight returns face.LineHeight(), or 0 for a nil face (matching
// TextBlock's nil-face convention).
func (t *TextBox) lineHeight() float32 {
	if t.face == nil {
		return 0
	}
	return t.face.LineHeight()
}

// xOf returns the x-offset (in logical px, from the start of the text) of
// rune index i: face.Measure(runes[:i]).W. i must already be within
// [0, len(runes)] (every caller clamps via SetCaret/Select first). Returns 0
// for a nil face.
func (t *TextBox) xOf(i int) float32 {
	if t.face == nil {
		return 0
	}
	return t.face.Measure(string(t.runes[:i])).W
}

// caretIndexAtX returns the rune index whose boundary is nearest to x, an
// x-coordinate already in the same "local text" space as xOf (logical px
// from the start of the text, i.e. hscroll already subtracted — see
// localTextX, which converts a pointer event's window-space x into this
// space). Implements the keyboard-map's click-to-caret rule: for each
// candidate rune boundary i (0..len(runes)-1), compare x against the
// midpoint between xOf(i) and xOf(i+1) — a click left of that midpoint is
// nearer to i, so i is returned; a click past every midpoint lands after the
// last rune, so len(runes) (the end) is returned. Returns 0 for a nil face
// (xOf(anything) is 0 there, every midpoint is 0, so any x>=0 falls through
// to len(runes) instead — deliberately not special-cased, since a nil-face
// TextBox has no glyphs to click between anyway).
func (t *TextBox) caretIndexAtX(x float32) int {
	n := len(t.runes)
	for i := 0; i < n; i++ {
		mid := (t.xOf(i) + t.xOf(i+1)) / 2
		if x < mid {
			return i
		}
	}
	return n
}

// lineCol maps rune index i (0..len(runes)) to its 0-based (line, col): line
// counts the '\n' runes strictly before i, and col is the rune offset since
// the most recent '\n' (or the start of the text if there is none). i must
// already be within [0, len(runes)]. O(n): fine for realistic multi-line
// TextBox content, matching every other index-based operation in this file
// (see the type doc comment).
func (t *TextBox) lineCol(i int) (line, col int) {
	for idx := 0; idx < i; idx++ {
		if t.runes[idx] == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	return line, col
}

// lineCount returns the number of lines: the number of '\n' runes plus one
// (an empty text is 1 line; a single trailing '\n' makes 2, the second
// being empty — matching strings.Split's convention, which renderMultiline
// also relies on).
func (t *TextBox) lineCount() int {
	n := 1
	for _, r := range t.runes {
		if r == '\n' {
			n++
		}
	}
	return n
}

// lineStart returns the rune index of the first rune of line (0-based,
// clamped to [0, lineCount()-1] by the caller — an out-of-range line above
// the last returns len(runes), i.e. an empty trailing range).
func (t *TextBox) lineStart(line int) int {
	if line <= 0 {
		return 0
	}
	n := 0
	for idx, r := range t.runes {
		if r == '\n' {
			n++
			if n == line {
				return idx + 1
			}
		}
	}
	return len(t.runes)
}

// lineEnd returns the rune index just before line's terminating '\n' (or
// len(runes) for the last line) — i.e. [lineStart(line), lineEnd(line)) is
// that line's own text, excluding the newline itself.
func (t *TextBox) lineEnd(line int) int {
	start := t.lineStart(line)
	for idx := start; idx < len(t.runes); idx++ {
		if t.runes[idx] == '\n' {
			return idx
		}
	}
	return len(t.runes)
}

// indexOfLineCol is lineCol's inverse: the rune index of column col on line
// line, with col clamped to that line's own length (so a shorter target
// line — e.g. during Up/Down navigation with a desiredCol from a longer
// line — lands at its end rather than spilling onto the next line).
func (t *TextBox) indexOfLineCol(line, col int) int {
	start, end := t.lineStart(line), t.lineEnd(line)
	return start + clampInt(col, 0, end-start)
}

// xOfInLine returns the x-offset (logical px, from the start of line's own
// text) of column col within line — the multi-line analogue of xOf, which
// measures from the start of the WHOLE text: each line is drawn at its own
// origin, so its glyph positions are relative to that line, not the buffer.
// Returns 0 for a nil face (matching xOf's own convention).
func (t *TextBox) xOfInLine(line, col int) float32 {
	if t.face == nil {
		return 0
	}
	start := t.lineStart(line)
	return t.face.Measure(string(t.runes[start : start+col])).W
}

// caretX returns the caret's current x-offset in "local text" space: for a
// single-line box this is exactly xOf(caret) (unchanged); in multi-line
// mode it is xOfInLine of the caret's own (line, col), since multi-line
// hscroll is keyed off the caret's line alone (see the SetMultiline doc
// comment's horizontal-scroll simplification).
func (t *TextBox) caretX() float32 {
	if !t.multiline {
		return t.xOf(t.caret)
	}
	line, col := t.lineCol(t.caret)
	return t.xOfInLine(line, col)
}

// caretLineWidth returns the total width hscroll must not scroll past the
// end of: xOf(len(runes)) (the whole text) for a single-line box, unchanged;
// in multi-line mode, the width of the CARET's own line only — every other
// line scrolls by the same shared hscroll regardless of its own width (see
// the SetMultiline doc comment).
func (t *TextBox) caretLineWidth() float32 {
	if !t.multiline {
		return t.xOf(len(t.runes))
	}
	line, _ := t.lineCol(t.caret)
	return t.xOfInLine(line, t.lineEnd(line)-t.lineStart(line))
}

// moveCaretVertical implements Up/Down (delta -1/+1) in multi-line mode:
// moves the caret to the same column (see desiredCol/desiredColValid) on
// the line above/below, clamped to [0, lineCount()-1] (Up on the first line
// or Down on the last is a no-op-ish clamp, matching moveCaret's own
// edge-clamping convention). extend mirrors moveCaret/moveCaretTo: false
// collapses via SetCaret, true extends the selection from the current
// anchor via Select.
//
// desiredCol bookkeeping: SetCaret/Select both clear desiredColValid (any
// OTHER caret-moving operation invalidates the tracked column), so this
// method captures the column to preserve (the caret's actual column if
// desiredColValid was already false, else the still-valid previously
// tracked column) BEFORE calling SetCaret/Select, then restores
// desiredCol/desiredColValid afterward — letting a run of consecutive
// Up/Down presses keep reusing the same desired column across lines
// shorter than it, the standard desktop-editor convention, while any
// intervening horizontal move (typing, Left/Right, Home/End, a click)
// still resets it on its own next SetCaret/Select call.
func (t *TextBox) moveCaretVertical(delta int, extend bool) {
	line, col := t.lineCol(t.caret)
	dc := col
	if t.desiredColValid {
		dc = t.desiredCol
	}

	newLine := clampInt(line+delta, 0, t.lineCount()-1)
	idx := t.indexOfLineCol(newLine, dc)

	if extend {
		t.Select(t.anchor, idx)
	} else {
		t.SetCaret(idx)
	}

	t.desiredCol = dc
	t.desiredColValid = true
}

// localTextY converts a pointer event's window-space y (e.Pos.Y) into the
// "local text" space lineAtY operates in — the multi-line analogue of
// localTextX: the padding-inset text top is at bounds.Y+PaddingM-vscroll
// (see renderMultiline's lineY), so subtracting that from the window-space
// y yields the y offset from the top of the text.
func (t *TextBox) localTextY(windowY float32) float32 {
	return windowY - t.Bounds().Y - t.metrics.PaddingM + t.vscroll
}

// lineAtY returns the 0-based line index nearest window-space y — the
// multi-line click-to-caret row lookup, paired with colAtX for the column.
// Clamped to [0, lineCount()-1] so a click above the first line or below
// the last still lands somewhere editable, matching caretIndexAtX's
// past-the-end convention. A nil/zero-height face (lineHeight()<=0) always
// resolves to line 0, avoiding a divide-by-zero.
func (t *TextBox) lineAtY(windowY float32) int {
	lh := t.lineHeight()
	if lh <= 0 {
		return 0
	}
	line := int(t.localTextY(windowY) / lh)
	return clampInt(line, 0, t.lineCount()-1)
}

// colAtX is caretIndexAtX's per-line analogue: the column within line
// nearest local x-coordinate x (already hscroll-subtracted, see
// localTextX), using the same "compare against each boundary's midpoint"
// rule, but bounded to line's own rune count via xOfInLine instead of the
// whole buffer.
func (t *TextBox) colAtX(line int, x float32) int {
	n := t.lineEnd(line) - t.lineStart(line)
	for i := 0; i < n; i++ {
		mid := (t.xOfInLine(line, i) + t.xOfInLine(line, i+1)) / 2
		if x < mid {
			return i
		}
	}
	return n
}

// caretIndexAtPos resolves a pointer event's window-space position to a
// rune index: in single-line mode this is exactly caretIndexAtX(localTextX(x))
// (unchanged); in multi-line mode it also resolves the row via lineAtY, then
// the column within that row via colAtX, combined through indexOfLineCol.
func (t *TextBox) caretIndexAtPos(x, y float32) int {
	if !t.multiline {
		return t.caretIndexAtX(t.localTextX(x))
	}
	line := t.lineAtY(y)
	col := t.colAtX(line, t.localTextX(x))
	return t.indexOfLineCol(line, col)
}

// localTextX converts a pointer event's window-space x (e.Pos.X) into the
// "local text" space xOf/caretIndexAtX operate in: the padding-inset text
// origin is at bounds.X+PaddingM-hscroll (see Render's textX), so subtracting
// that from the window-space x yields the x offset from the start of the
// text, in the same units xOf returns.
func (t *TextBox) localTextX(windowX float32) float32 {
	return windowX - t.Bounds().X - t.metrics.PaddingM + t.hscroll
}

// displayText resolves what Render actually draws as the main text run and
// in what color: the placeholder (GrayText) whenever there is no text,
// regardless of focus; otherwise the real text, GrayText if disabled
// else WindowText.
func (t *TextBox) displayText() (s string, color render.Color) {
	if len(t.runes) == 0 {
		return t.placeholder, t.colors.GrayText
	}
	if !t.enabled {
		return string(t.runes), t.colors.GrayText
	}
	return string(t.runes), t.colors.WindowText
}

// caretShown reports whether the caret should be drawn this frame: never
// while unfocused; while focused, always if no timers.Queue is wired (solid
// caret), else per the current blink phase (caretVisible).
func (t *TextBox) caretShown() bool {
	if !t.focused {
		return false
	}
	if t.timerQueue == nil {
		return true
	}
	return t.caretVisible
}

// textBoxMultilineDefaultLines is the number of lines' worth of height
// MeasureContent reports for a multi-line TextBox with no explicit height
// (via core.Element.SetHeight) — enough to read as a text area rather than
// a single input line, while still small enough to fit comfortably in a
// typical form; an explicit SetHeight overrides it exactly like
// textBoxDefaultWidth's explicit-SetWidth precedence.
const textBoxMultilineDefaultLines = 4

// MeasureContent reports the fixed content size: textBoxDefaultWidth by
// lineHeight()+2*PaddingM in single-line mode (unchanged), or by
// textBoxMultilineDefaultLines*lineHeight()+2*PaddingM in multi-line mode
// (see SetMultiline). An explicit SetWidth/SetHeight overrides either
// through core.MeasureWidget's normal explicit-size precedence, so
// available is never consulted.
func (t *TextBox) MeasureContent(available render.Size) render.Size {
	lh := t.lineHeight()
	h := lh + 2*t.metrics.PaddingM
	if t.multiline {
		h = lh*textBoxMultilineDefaultLines + 2*t.metrics.PaddingM
	}
	return render.Size{W: textBoxDefaultWidth, H: h}
}

// ArrangeContent is the single source of truth for hscroll/vscroll clamping
// (the ScrollViewer clamp-in-arrange pattern applied to each scroll axis):
// it recomputes the padding-inset inner width/height from the arranged
// bounds and clamps hscroll (always) and vscroll (multi-line mode only —
// updateVScroll is a no-op-to-zero in single-line mode) so the caret stays
// visible within them.
func (t *TextBox) ArrangeContent(bounds render.Rect) {
	innerW := bounds.W - 2*t.metrics.PaddingM
	if innerW < 0 {
		innerW = 0
	}
	t.updateHScroll(innerW)

	innerH := bounds.H - 2*t.metrics.PaddingM
	if innerH < 0 {
		innerH = 0
	}
	t.updateVScroll(innerH)
}

// updateHScroll clamps hscroll into a range that keeps the caret's display
// x-position (caretX()-hscroll) within [0, innerW], while never scrolling
// past the point where the end of the caret's line would leave a gap on the
// right (hscroll is also capped at max(0, caretLineWidth()-innerW)).
// caretX/caretLineWidth reduce to xOf(caret)/xOf(len(runes)) in single-line
// mode (unchanged behavior); in multi-line mode they key off the caret's
// own line only (see their doc comments and SetMultiline).
func (t *TextBox) updateHScroll(innerW float32) {
	caretX := t.caretX()

	if caretX-t.hscroll < 0 {
		t.hscroll = caretX
	}
	if caretX-t.hscroll > innerW {
		t.hscroll = caretX - innerW
	}
	if t.hscroll < 0 {
		t.hscroll = 0
	}

	maxScroll := t.caretLineWidth() - innerW
	if maxScroll < 0 {
		maxScroll = 0
	}
	if t.hscroll > maxScroll {
		t.hscroll = maxScroll
	}
}

// updateVScroll is updateHScroll's vertical analogue, multi-line only: a
// no-op that pins vscroll at 0 in single-line mode (so a single-line box
// never scrolls vertically, and renderMultiline's y math is never even
// consulted for one). In multi-line mode it clamps vscroll so the caret's
// own line stays fully visible within [0, innerH] — never scrolling past
// the point where the last line would leave a gap at the bottom (vscroll is
// also capped at max(0, lineCount()*lineHeight()-innerH)).
func (t *TextBox) updateVScroll(innerH float32) {
	if !t.multiline {
		t.vscroll = 0
		return
	}

	lh := t.lineHeight()
	line, _ := t.lineCol(t.caret)
	caretY := float32(line) * lh

	if caretY-t.vscroll < 0 {
		t.vscroll = caretY
	}
	if caretY-t.vscroll > innerH-lh {
		t.vscroll = caretY - (innerH - lh)
	}
	if t.vscroll < 0 {
		t.vscroll = 0
	}

	maxScroll := float32(t.lineCount())*lh - innerH
	if maxScroll < 0 {
		maxScroll = 0
	}
	if t.vscroll > maxScroll {
		t.vscroll = maxScroll
	}
}

// ClipRect implements core.ClipProvider, clipping to the textbox's own full
// bounds (the entire chrome rect, stroke included) — the same rect Render
// itself clips text/selection/caret drawing to (see Render).
func (t *TextBox) ClipRect() (render.Rect, bool) {
	return t.Bounds(), true
}

// Render paints the classic sunken input well (drawSunken, WindowWell fill —
// ButtonFace while disabled, per the classic "grayed-out field" convention),
// then the selection highlight, main text run (or placeholder, split around
// any active selection so the selected glyphs recolor to HighlightText), and
// caret, all clipped to the textbox's own bounds (see ClipRect) since TextBox
// draws its content directly rather than through a child widget that
// RenderWidget would clip via ClipProvider on its own.
//
// No separate focus ring: PaddingM (8px) already clears the 2px sunken
// bevel drawn by drawSunken on every edge, so the existing text/caret origin
// needs no further draw-only inset — see RenderOverlay's doc comment for why
// TextBox alone, among the focusable controls in this package, paints no
// focus ring at all.
func (t *TextBox) Render(r render.Renderer) {
	c := t.colors
	bounds := t.Bounds()

	fill := c.WindowWell
	if !t.enabled {
		fill = c.ButtonFace
	}
	drawSunken(r, bounds, fill, c)

	rect, clip := t.ClipRect()
	if clip {
		r.PushClip(rect)
		defer r.PopClip()
	}

	if t.multiline {
		t.renderMultiline(r, bounds)
		return
	}

	pad := t.metrics.PaddingM
	lh := t.lineHeight()
	textY := bounds.Y + (bounds.H-lh)/2
	textX := bounds.X + pad - t.hscroll

	start, end := t.Selection()
	hasSel := start != end

	if hasSel {
		selX0 := textX + t.xOf(start)
		selX1 := textX + t.xOf(end)
		r.FillRect(render.Rect{X: selX0, Y: textY, W: selX1 - selX0, H: lh}, c.Highlight)
	}

	if s, color := t.displayText(); t.face != nil && s != "" {
		if hasSel {
			t.drawTextWithSelection(r, []rune(s), start, end, textX, textY, color)
		} else {
			t.face.Draw(r, render.Point{X: textX, Y: textY}, s, color)
		}
	}

	if t.caretShown() {
		cx := textX + t.xOf(t.caret)
		r.FillRect(render.Rect{X: cx, Y: textY, W: caretWidth, H: lh}, c.WindowText)
	}
}

// drawTextWithSelection draws runes in three pieces — before [start,end),
// inside it (recolored to HighlightText over the Highlight selection band),
// and after — so the selected glyphs read correctly over the band instead of
// the whole run drawing uniformly in color. textX/textY is the same
// unselected-run origin Render itself would have used; each piece's x
// offset derives from t.xOf, matching how selX0/selX1 above were computed.
func (t *TextBox) drawTextWithSelection(r render.Renderer, runes []rune, start, end int, textX, textY float32, color render.Color) {
	if pre := string(runes[:start]); pre != "" {
		t.face.Draw(r, render.Point{X: textX, Y: textY}, pre, color)
	}
	if sel := string(runes[start:end]); sel != "" {
		x := textX + t.xOf(start)
		t.face.Draw(r, render.Point{X: x, Y: textY}, sel, t.colors.HighlightText)
	}
	if post := string(runes[end:]); post != "" {
		x := textX + t.xOf(end)
		t.face.Draw(r, render.Point{X: x, Y: textY}, post, color)
	}
}

// renderMultiline is Render's multi-line body (see SetMultiline), invoked
// in place of Render's single-line drawing (never both — see Render's early
// return): draws each '\n'-delimited line of displayText() at its own
// baseline, lineHeight() apart, offset by vscroll vertically (see
// updateVScroll) and by the single shared hscroll horizontally (see
// updateHScroll's caretX/caretLineWidth) — every line shares one hscroll
// offset rather than each scrolling independently, the "mirror single-line…
// per line" horizontal simplification SetMultiline's doc comment describes.
//
// Selection highlight is drawn per line by intersecting the whole-text
// [start,end) rune range with each line's own [lineStart,lineEnd) range
// (both derived from t.runes, which is safe here: displayText() only
// diverges from string(t.runes) — the placeholder — when t.runes is empty,
// in which case Selection() is always (0,0) anyway, so the intersection math
// is a no-op regardless). The caret is drawn once, at its own (line, col).
func (t *TextBox) renderMultiline(r render.Renderer, bounds render.Rect) {
	c := t.colors
	pad := t.metrics.PaddingM
	lh := t.lineHeight()
	textX := bounds.X + pad - t.hscroll

	s, color := t.displayText()
	lines := strings.Split(s, "\n")

	start, end := t.Selection()

	for i, line := range lines {
		lineY := bounds.Y + pad - t.vscroll + float32(i)*lh
		lo, hi := t.lineStart(i), t.lineEnd(i)

		selStart, selEnd := clampInt(start, lo, hi), clampInt(end, lo, hi)
		hasSel := selStart < selEnd

		if hasSel {
			x0 := textX + t.xOfInLine(i, selStart-lo)
			x1 := textX + t.xOfInLine(i, selEnd-lo)
			r.FillRect(render.Rect{X: x0, Y: lineY, W: x1 - x0, H: lh}, c.Highlight)
		}

		if t.face != nil && line != "" {
			if hasSel {
				t.drawLineWithSelection(r, []rune(line), selStart-lo, selEnd-lo, textX, lineY, color)
			} else {
				t.face.Draw(r, render.Point{X: textX, Y: lineY}, line, color)
			}
		}
	}

	if t.caretShown() {
		line, col := t.lineCol(t.caret)
		cx := textX + t.xOfInLine(line, col)
		cy := bounds.Y + pad - t.vscroll + float32(line)*lh
		r.FillRect(render.Rect{X: cx, Y: cy, W: caretWidth, H: lh}, c.WindowText)
	}
}

// drawLineWithSelection is renderMultiline's per-line analogue of
// drawTextWithSelection: runes is one line's runes (no '\n'), start/end are
// LINE-RELATIVE rune offsets into it (already intersected with the line's
// own range by the caller), and lineX/lineY is that line's own draw origin
// — the same three-piece before/inside/after split, with each piece's x
// offset measured from the start of THIS line (via face.Measure) rather
// than the whole buffer (unlike xOf, which drawTextWithSelection uses).
func (t *TextBox) drawLineWithSelection(r render.Renderer, runes []rune, start, end int, lineX, lineY float32, color render.Color) {
	if pre := string(runes[:start]); pre != "" {
		t.face.Draw(r, render.Point{X: lineX, Y: lineY}, pre, color)
	}
	if sel := string(runes[start:end]); sel != "" {
		x := lineX + t.face.Measure(string(runes[:start])).W
		t.face.Draw(r, render.Point{X: x, Y: lineY}, sel, t.colors.HighlightText)
	}
	if post := string(runes[end:]); post != "" {
		x := lineX + t.face.Measure(string(runes[:end])).W
		t.face.Draw(r, render.Point{X: x, Y: lineY}, post, color)
	}
}

// RenderOverlay is a deliberate no-op: classic Windows textboxes draw no
// separate focus ring (unlike every other focusable control in this
// package, per drawFocusRing's doc comment in clickable.go) — the caret and
// the sunken well already read as "this is the focused field," and the box
// has no raised/sunken chrome swap to signal focus with either (unlike
// Button's press-sunken state). TextBox still implements OverlayRenderer so
// core.RenderWidget's overlay dispatch (see core/widget.go) finds a stable
// method here rather than silently falling through, but it paints nothing.
func (t *TextBox) RenderOverlay(r render.Renderer) {}

// OnKey implements input.KeyHandler, the normative Task 6 keyboard map.
// Ignored entirely (no mutation, Handled left false) while disabled or
// unfocused, and for anything but Action==Press — glfw's Repeat action
// arrives as a second Press while a key is held, so held Backspace/Delete/
// arrows correctly auto-repeat via the host's own key-repeat timer, not
// anything TextBox does itself.
//
// Every recognized combination below sets e.Handled = true, even when the
// specific operation ends up being a no-op (e.g. Ctrl+C with no selection,
// Backspace at position 0, Ctrl+V with no clipboard wired) — a focused
// TextBox owns all of these keys and must not let them bubble further
// (e.g. into a Router.KeyDown Tab-navigation-style fallback) just because
// there happened to be nothing to do.
//
// Enter/Up/Down are multi-line-only (see SetMultiline): in single-line mode
// they fall through unhandled exactly as before multi-line mode existed —
// Enter does not commit/submit anything here (TextBox has no such concept;
// a host wanting that behavior sees the unhandled KeyEnter bubble past it),
// and Up/Down are simply not part of the single-line keyboard map. Home/End
// are handled in both modes, but target the whole text in single-line mode
// and the caret's own line in multi-line mode (homeTarget/endTarget).
func (t *TextBox) OnKey(e *input.KeyEvent) {
	if !t.enabled || !t.focused || e.Action != input.Press {
		return
	}

	if e.Mods&input.ModCtrl != 0 {
		switch e.Key {
		case input.KeyA:
			t.Select(0, len(t.runes))
			e.Handled = true
			return
		case input.KeyC:
			t.copySelection(e.Router)
			e.Handled = true
			return
		case input.KeyX:
			t.cutSelection(e.Router)
			e.Handled = true
			return
		case input.KeyV:
			t.pasteClipboard(e.Router)
			e.Handled = true
			return
		}
	}

	shift := e.Mods&input.ModShift != 0
	switch e.Key {
	case input.KeyBackspace:
		t.deleteBackward()
		e.Handled = true
		return
	case input.KeyDelete:
		t.deleteForward()
		e.Handled = true
		return
	case input.KeyLeft:
		t.moveCaret(-1, shift)
		e.Handled = true
		return
	case input.KeyRight:
		t.moveCaret(1, shift)
		e.Handled = true
		return
	case input.KeyHome:
		t.moveCaretTo(t.homeTarget(), shift)
		e.Handled = true
		return
	case input.KeyEnd:
		t.moveCaretTo(t.endTarget(), shift)
		e.Handled = true
		return
	case input.KeyUp:
		if t.multiline {
			t.moveCaretVertical(-1, shift)
			e.Handled = true
			return
		}
	case input.KeyDown:
		if t.multiline {
			t.moveCaretVertical(1, shift)
			e.Handled = true
			return
		}
	case input.KeyEnter:
		if t.multiline {
			t.insertText("\n")
			e.Handled = true
			return
		}
	}

	// Rune input: any produced character not accompanied by Ctrl (Ctrl+<key>
	// combos above either matched and returned already, or aren't part of the
	// v0 keyboard map — either way, a bare Ctrl+letter must not ALSO insert
	// the letter as text).
	if e.Rune != 0 && e.Mods&input.ModCtrl == 0 {
		t.insertText(string(e.Rune))
		e.Handled = true
	}
}

// OnPointer implements input.PointerHandler: click-to-caret and drag-to-
// select. Ignored entirely while disabled (not handled, so pointer events
// bubble past a disabled TextBox rather than being swallowed by it) — but a
// SetEnabled(false) landing MID-DRAG (this box still holds the router's
// capture from an earlier Press) releases that capture first, before the
// disabled early-return, matching Slider's OnPointer: otherwise every
// subsequent pointer event would keep routing here via deliverCaptured
// (never hit-testing) and find a disabled TextBox unwilling to do anything
// with it — a permanent wedge with no widget reachable by the pointer at
// all, not merely this one ignoring input as intended.
//
// Press moves the caret to the nearest rune boundary to the click position
// (via caretIndexAtPos — x only in single-line mode, x AND y in multi-line
// mode, see its doc comment) — which also clears any existing selection and
// sets the drag anchor, since SetCaret sets anchor==caret — and captures the
// pointer so the drag survives leaving the TextBox's bounds. Move, only
// while this TextBox holds the capture, extends the selection from that
// same anchor to the new nearest boundary (Select(t.anchor, idx): t.anchor
// is left untouched by Select's own reassignment since the same value is
// passed back in, so it stays pinned at the press position across an entire
// drag). Release, only while captured, ends the drag.
func (t *TextBox) OnPointer(e *input.PointerEvent) {
	if !t.enabled {
		if e.Router != nil && e.Router.Captured() == t {
			e.Router.Release()
		}
		return
	}
	switch e.Action {
	case input.Press:
		idx := t.caretIndexAtPos(e.Pos.X, e.Pos.Y)
		t.SetCaret(idx)
		e.Router.Capture(t)
		e.Handled = true
	case input.Move:
		if e.Router.Captured() == t {
			idx := t.caretIndexAtPos(e.Pos.X, e.Pos.Y)
			t.Select(t.anchor, idx)
			e.Handled = true
		}
	case input.Release:
		if e.Router.Captured() == t {
			e.Router.Release()
			e.Handled = true
		}
	}
}

// Cursor implements input.CursorShaper: TextBox always shapes an I-beam
// cursor over its bounds (matching the platform convention for text-input
// fields), independent of enabled/focused state.
func (t *TextBox) Cursor() input.Cursor {
	return input.CursorIBeam
}

// clampInt clamps v into [lo, hi].
func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
