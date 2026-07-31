package controls

import (
	"math"
	"strings"
	"time"
	"unicode"

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

// preeditUnderlineThickness is the drawn height of the thin underline rule
// beneath an active IME composition's provisional (uncommitted) preedit run
// (see OnComposition/renderComposing) — the conventional "this text isn't
// committed yet" cue most IME-aware text fields draw themselves, since
// fluo's text.Face has no underline/strikethrough styling of its own to
// lean on. Matches caretWidth's own convention of a hairline rule sized to
// stay visible after SDF/AA rounding.
const preeditUnderlineThickness float32 = 1.5

// tabInsertSpaces is how many space runes a Tab press inserts when
// SetTabInserts is on (see its doc comment) — and the matching cap on how
// many leading spaces a Shift+Tab press removes. A literal '\t' is never
// inserted: spaces need no tab-stop rendering support, so they work with the
// existing monospace/coverage text path unchanged.
const tabInsertSpaces = 4

// pageRowsFallback is the number of rows/lines PageUp/PageDown move the
// caret by (see pageRows) when the viewport's own row count can't be
// determined — a nil face (lineHeight() <= 0) or a box that hasn't been
// arranged yet (contentHeight() <= 0) — a sane "text area" page size chosen
// the same way textBoxMultilineDefaultLines was: enough to read as a real
// page jump rather than a token amount.
const pageRowsFallback = 10

// TextBox is a focusable, token-styled text input, single-line by default.
// The data model (text/caret/selection, rune-indexed) and rendering (chrome,
// selection highlight, caret, horizontal scroll, placeholder) were built in
// Phase 5 Task 5; Task 6 added the interaction layer: OnKey (the normative
// keyboard map — rune insertion, Backspace/Delete, arrow/Home/End caret
// movement with Shift-extend, Ctrl+A/C/X/V) and OnPointer (click-to-caret,
// drag-to-select, CursorShaper's CursorIBeam). TextBox implements
// input.Focusable and input.FocusHandler (AcceptsFocus/OnFocusChanged) since
// focus also drives the focus-ring overlay and the focused border color.
// A later editor-ergonomics pass added Ctrl+Home/End (buffer-bounds jump),
// Ctrl+Left/Right (word-wise motion), Ctrl+Backspace/Delete (word
// deletion), and PageUp/PageDown (viewport-height caret motion, multi-line
// only) — see OnKey's own doc comment for the full keyboard map.
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

	// wordWrap toggles opt-in soft word-wrap (see SetWordWrap); false (the
	// zero value) is the original behavior, unchanged — only meaningful when
	// multiline is also true (see wrapping()).
	wordWrap bool

	// tabInserts toggles opt-in Tab-inserts-indentation (see SetTabInserts);
	// false (the zero value) is the original behavior, unchanged — only
	// meaningful when multiline is also true (see OnKey's KeyTab case),
	// mirroring wordWrap's own multiline-only gating.
	tabInserts bool

	// rev counts real mutations to runes (SetText's real-change path,
	// replaceRange) — the "text revision" half of the visual-rows cache key
	// (see visualRows/rowsValid). Never decreases; overflow is not a concern
	// at realistic edit counts.
	rev uint64

	// rows, rowsWidth, rowsRev, and rowsValid together cache the visual-rows
	// word-wrap layout (see visualRows/computeVisualRows): rows is valid
	// (reusable without recomputing) exactly when rowsValid is true AND
	// rowsWidth/rowsRev both still match the width/rev the cache was last
	// computed for. Only ever populated while wrapping() is true; read by
	// every row-based caret/selection/hit-test/render helper below.
	rows      []visualRow
	rowsWidth float32
	rowsRev   uint64
	rowsValid bool

	// vScrollShown caches computeShowVScroll's decision for the current
	// arrange bounds — refreshed once at the top of every ArrangeContent
	// call, then read (not recomputed) by contentWidth, vScrollTrack,
	// RenderOverlay, and OnPointer's thumb hit-test for the rest of that
	// layout pass. Always false outside multiline mode.
	vScrollShown bool

	// vDragging and vDragGrab track an in-progress vertical-thumb drag (see
	// dragVScroll/OnPointer): vDragging is true only while this TextBox
	// holds the router's pointer capture for a thumb drag specifically (as
	// opposed to the pre-existing caret-drag-to-select capture); vDragGrab
	// is the y-offset (logical px) between the pointer and the thumb's own
	// top edge at the moment the drag began, so the thumb tracks the
	// pointer at a fixed grab point rather than snapping its edge to the
	// cursor — the same convention ScrollViewer.dragGrabY documents.
	vDragging bool
	vDragGrab float32

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
	onSubmit  func(string)

	// preedit is the active IME composition's provisional (uncommitted)
	// string, spliced in for display at t.caret while composing is true
	// (see OnComposition/renderComposing) — entirely separate from runes:
	// the committed text (Text()) is never touched by an in-progress
	// composition, only by its eventual commit (via insertText, the same
	// path every other user edit goes through). preeditCaret is the caret's
	// RUNE offset WITHIN preedit (not into runes) — see CaretScreenRect,
	// which places the caret inside the preedit run rather than at t.caret
	// while composing. Phase B of Task 6 (inline preedit rendering), built
	// on Task 5's OS candidate-window anchoring (app/ime.go) — see also
	// input.CompositionHandler, which this type implements.
	preedit      []rune
	preeditCaret int
	composing    bool

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
	t.rev++
	t.InvalidateArrange()
	if t.wrapping() {
		// A wrapped box's desired height depends on the text's own row
		// count (see MeasureContent), unlike the fixed-height unwrapped
		// case this method's own doc comment describes — so, only while
		// wrapping, a real text change must also invalidate measure.
		t.InvalidateMeasure()
	}
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

// SetWordWrap toggles opt-in soft word-wrap; false (the default) is
// SetMultiline's original "hard-wrap only, scroll horizontally" behavior,
// byte-for-byte unchanged. Only meaningful when Multiline() is also true —
// a harmless no-op flag otherwise (single-line mode never consults it; see
// wrapping()). No '\n' is ever inserted into the buffer when wrap is on:
// wrapping is a purely DISPLAY-time re-flow of each real ('\n'-delimited)
// line into one or more visual rows at the box's current content width (see
// visualRows/computeVisualRows) — Text() and OnChanged are completely
// unaffected. Horizontal scrolling (hscroll) is disabled while wrap is on,
// since wrapped rows never exceed the content width by construction (see
// updateHScroll); vertical scrolling (vscroll) instead counts visual rows
// rather than logical lines (see updateVScroll).
//
// Calls InvalidateMeasure, like SetMultiline: it can change MeasureContent's
// answer, since a wrapped box's height depends on the resulting visual-row
// count rather than the fixed textBoxMultilineDefaultLines default.
func (t *TextBox) SetWordWrap(v bool) *TextBox {
	t.wordWrap = v
	t.InvalidateMeasure()
	return t
}

// WordWrap reports whether word-wrap is enabled (see SetWordWrap).
func (t *TextBox) WordWrap() bool {
	return t.wordWrap
}

// wrapping reports whether word-wrap is ACTIVE right now: wordWrap is only
// consulted at all while multiline is also true, so this is the single
// guard every wrap-aware helper below branches on.
func (t *TextBox) wrapping() bool {
	return t.multiline && t.wordWrap
}

// SetTabInserts toggles whether a focused Tab press inserts indentation
// instead of bubbling to the Router's focus-navigation; false (the default)
// leaves Tab byte-for-byte unhandled by TextBox, exactly as before this
// feature existed — the Router's own Tab/Shift+Tab focus-cycling (see
// input/router.go) is what moves focus in that case.
//
// Only effective in MULTILINE mode (see SetMultiline and OnKey's KeyTab
// case): a single-line TextBox always leaves Tab as focus-nav regardless of
// this flag — indenting a single-line input has no sensible meaning, the
// same multiline-only gating SetWordWrap already uses (see wrapping()).
//
// When enabled and multiline, a focused Tab (without Shift) inserts
// tabInsertSpaces spaces at the caret via the same insertText path plain
// typing uses — so OnChanged, selection-replace, and caret-advance all
// behave exactly like typing those spaces would — and is CONSUMED
// (e.Handled = true) so it never reaches the Router's focus cycling.
// Shift+Tab removes up to tabInsertSpaces leading spaces from the caret's
// current line (see unindentCurrentLine); like every other recognized
// combination in OnKey, it is marked handled even when there is nothing to
// remove (a no-op edit, the same convention Backspace-at-position-0 already
// follows), so Shift+Tab never leaks through to focus-nav while this flag is
// on either. No literal '\t' is ever inserted (see tabInsertSpaces).
//
// The single-line-selection behavior above only holds when the CURRENT
// selection is contained within one logical line (or there is none). When
// the selection spans more than one logical line (see blockSelection), Tab
// and Shift+Tab instead indent/outdent every touched line in place
// (indentSelectedLines/outdentSelectedLines) — the "select several lines,
// press Tab to indent the block" editor convention — rather than deleting
// the selected text, which the plain insertText/single-line path would
// otherwise do. The selection is restored afterward to span the same
// block, so repeated Tab presses keep indenting further.
func (t *TextBox) SetTabInserts(v bool) *TextBox {
	t.tabInserts = v
	return t
}

// TabInserts reports whether Tab-inserts-indentation is enabled (see
// SetTabInserts).
func (t *TextBox) TabInserts() bool {
	return t.tabInserts
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

// OnSubmit sets the callback fired with the current text when the user
// presses Enter in SINGLE-LINE mode (see OnKey). Multi-line mode is
// unaffected — Enter there always inserts a '\n' (see SetMultiline) and
// never calls this callback, regardless of whether one is set. Like
// OnChanged, this only ever fires from a user keypress, never from a
// programmatic SetText — fluo's uniform setter convention (programmatic
// setters are silent). Replaces any previously set callback; a nil fn is a
// valid, silent no-op (Enter is then simply unhandled in single-line mode,
// matching the pre-OnSubmit behavior).
func (t *TextBox) OnSubmit(fn func(string)) *TextBox {
	t.onSubmit = fn
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
	t.rev++
	t.InvalidateArrange()
	if t.wrapping() {
		// See SetText's matching comment: only while wrapping does a text
		// change potentially change the desired (wrapped) height.
		t.InvalidateMeasure()
	}
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

// runeCharClass classifies r for word-boundary purposes (see
// prevWordBoundary/nextWordBoundary): whitespace (runeClassSpace, which
// includes '\n' — a newline is just another whitespace rune here, so word
// motion crosses line boundaries for free, the same way plain Left/Right
// already does), "word" runes (runeClassWord — letters, digits, and '_',
// the conventional identifier-character set), or everything else
// (runeClassPunct) — a run of punctuation is its own word, the standard
// editor convention (e.g. "foo," is two word-motion stops: "foo" then ",").
type runeCharClass uint8

const (
	runeClassSpace runeCharClass = iota
	runeClassWord
	runeClassPunct
)

func runeClassOf(r rune) runeCharClass {
	switch {
	case unicode.IsSpace(r):
		return runeClassSpace
	case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_':
		return runeClassWord
	default:
		return runeClassPunct
	}
}

// nextWordBoundary returns the rune index Ctrl+Right should move the caret
// to from i (see OnKey): skip any run of whitespace starting at i, then skip
// the following run of same-class runes (word or punctuation — see
// runeClassOf) — i.e. stop at the end of the next "word", where a run of
// punctuation counts as its own word. Returns len(t.runes) once there is
// nothing left to skip.
func (t *TextBox) nextWordBoundary(i int) int {
	n := len(t.runes)
	for i < n && runeClassOf(t.runes[i]) == runeClassSpace {
		i++
	}
	if i >= n {
		return n
	}
	class := runeClassOf(t.runes[i])
	for i < n && runeClassOf(t.runes[i]) == class {
		i++
	}
	return i
}

// prevWordBoundary is nextWordBoundary's mirror image for Ctrl+Left: skip
// any run of whitespace immediately before i, then skip the preceding run of
// same-class runes, landing at the START of the previous "word". Returns 0
// once there is nothing left to skip.
func (t *TextBox) prevWordBoundary(i int) int {
	for i > 0 && runeClassOf(t.runes[i-1]) == runeClassSpace {
		i--
	}
	if i <= 0 {
		return 0
	}
	class := runeClassOf(t.runes[i-1])
	for i > 0 && runeClassOf(t.runes[i-1]) == class {
		i--
	}
	return i
}

// deleteWordBackward implements Ctrl+Backspace: delete the selection if one
// is active (the same selection-first convention plain Backspace uses — see
// deleteBackward), else delete from prevWordBoundary(caret) to the caret.
// A no-op at the very start of the text with no selection.
func (t *TextBox) deleteWordBackward() {
	if start, end := t.Selection(); start != end {
		t.replaceRange(start, end, "")
		return
	}
	if t.caret > 0 {
		t.replaceRange(t.prevWordBoundary(t.caret), t.caret, "")
	}
}

// deleteWordForward is deleteWordBackward's Ctrl+Delete counterpart: delete
// the selection if one is active, else delete from the caret to
// nextWordBoundary(caret). A no-op at the very end of the text with no
// selection.
func (t *TextBox) deleteWordForward() {
	if start, end := t.Selection(); start != end {
		t.replaceRange(start, end, "")
		return
	}
	if t.caret < len(t.runes) {
		t.replaceRange(t.caret, t.nextWordBoundary(t.caret), "")
	}
}

// pageRows returns the number of rows/lines a PageUp/PageDown press should
// move the caret by (see OnKey): as many full lineHeight() rows as fit
// within the box's current content (viewport) height, so a page jump lands
// the caret roughly where the previous page's edge was — or
// pageRowsFallback when that can't be computed (a nil face, or a box that
// hasn't been arranged yet). Always at least 1.
func (t *TextBox) pageRows() int {
	lh := t.lineHeight()
	h := t.contentHeight()
	if lh <= 0 || h <= 0 {
		return pageRowsFallback
	}
	rows := int(h / lh)
	if rows < 1 {
		rows = 1
	}
	return rows
}

// homeTarget returns the rune index Home should move the caret to: 0 in
// single-line mode — the whole text's start, unchanged — or, in multi-line
// mode, the start of the caret's OWN line (see SetMultiline); while
// wrapping (see SetWordWrap), "line" here means the caret's own VISUAL row
// instead of its logical ('\n'-delimited) line — Home stops at a soft wrap
// point exactly as it would at a hard one.
func (t *TextBox) homeTarget() int {
	if !t.multiline {
		return 0
	}
	if t.wrapping() {
		row, _ := t.rowCol(t.caret)
		return t.rowStart(row)
	}
	line, _ := t.lineCol(t.caret)
	return t.lineStart(line)
}

// endTarget is homeTarget's End counterpart: len(t.runes) in single-line
// mode (unchanged), or the end of the caret's own line in multi-line mode
// (its own visual row, while wrapping — see homeTarget's doc comment).
func (t *TextBox) endTarget() int {
	if !t.multiline {
		return len(t.runes)
	}
	if t.wrapping() {
		row, _ := t.rowCol(t.caret)
		return t.rowEnd(row)
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

// OnComposition implements input.CompositionHandler, the Task 6 Phase B
// inline-preedit half of IME support (candidate-window anchoring is Phase A,
// see app/ime.go): while e.Active, it stores e.Preedit/e.CaretPos for
// renderComposing to splice in at the caret — the committed buffer (runes,
// Text()) is left completely untouched for the whole composition, so a
// user's still-in-progress CJK candidate never corrupts what OnChanged has
// already reported. When the composition ends (e.Active false), the preedit
// is cleared unconditionally, and — unless e.Canceled — e.Committed is
// inserted at the caret via insertText, the SAME mutation path every other
// user edit (typing, Backspace/Delete, Ctrl+X, Ctrl+V) already goes through:
// this fires OnChanged, InvalidateArrange, and restartBlink exactly as a
// typed character would, with no special-casing needed here for that. A
// canceled composition (e.Canceled true, e.g. Escape) or one that ends with
// an empty commit does no mutation at all beyond clearing the preedit —
// InvalidateArrange/restartBlink are still called directly on THAT path
// (insertText's replaceRange only runs, and only calls them itself, when
// there is actually a commit to insert) so the caret redraws immediately
// rather than possibly showing a stale preedit remnant until the next
// unrelated invalidation.
//
// Ignored entirely while disabled or unfocused, matching OnKey's own guard.
// Also ignored while NOT currently composing and e reports the composition
// has already ended (e.Active false) with nothing to commit — a defensive
// no-op against a redundant end notification (see the Windows decoder's own
// doc comment on WM_IME_ENDCOMPOSITION bookkeeping), so a second such
// notification never re-fires InvalidateArrange/restartBlink for no reason.
func (t *TextBox) OnComposition(e input.CompositionEvent) {
	if !t.enabled || !t.focused {
		return
	}

	if e.Active {
		t.preedit = []rune(e.Preedit)
		t.preeditCaret = clampInt(e.CaretPos, 0, len(t.preedit))
		t.composing = true
		t.InvalidateArrange()
		t.restartBlink()
		return
	}

	if !t.composing && e.Committed == "" {
		return
	}

	t.composing = false
	t.preedit = nil
	t.preeditCaret = 0

	if !e.Canceled && e.Committed != "" {
		t.insertText(e.Committed)
		return
	}
	t.InvalidateArrange()
	t.restartBlink()
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

// CaretScreenRect implements input.CaretRector: the caret's rectangle in
// window logical coordinates, for a host anchoring platform UI to it (e.g.
// app's Windows IME candidate-window anchor). false while unfocused —
// mirroring caretShown()'s own "never while unfocused" rule, though unlike
// caretShown() this does NOT further depend on the blink phase: the anchored
// UI should track the caret's position regardless of whether the drawn caret
// bar happens to be mid-blink-off this particular frame.
//
// The math mirrors Render/renderMultiline's own caret placement exactly: in
// single-line mode, bounds origin + padding + (xOf(caret)-hscroll,
// vertically centered) by lineHeight(); in multi-line mode, bounds origin +
// padding + (xOfInLine(line,col)-hscroll, line*lineHeight()-vscroll) by
// lineHeight() — or, while wrapping (see SetWordWrap), the same shape over
// the caret's own visual (row,col) via rowCol/xOfInRow instead of
// lineCol/xOfInLine (hscroll is always 0 there — see updateHScroll) — see
// caretX/xOfInLine/xOfInRow and their doc comments.
//
// While an IME composition is active (t.composing), the reported rect is
// shifted to the caret's position INSIDE the preedit run (preeditMeasure of
// preeditCaret runes further along) rather than the plain t.caret position —
// so a host anchoring platform UI to this rect (e.g. the Windows candidate
// window, see app/ime_windows.go) keeps tracking the caret as the user moves
// it within an in-progress composition, not just the point where the
// composition began.
func (t *TextBox) CaretScreenRect() (render.Rect, bool) {
	if !t.focused {
		return render.Rect{}, false
	}
	bounds := t.Bounds()
	pad := t.metrics.PaddingM
	lh := t.lineHeight()
	textX := bounds.X + pad - t.hscroll

	if !t.multiline {
		textY := bounds.Y + (bounds.H-lh)/2
		cx := textX + t.xOf(t.caret)
		if t.composing {
			cx += t.preeditMeasure(t.preeditCaret)
		}
		return render.Rect{X: cx, Y: textY, W: caretWidth, H: lh}, true
	}

	var row, col int
	var cx float32
	if t.wrapping() {
		row, col = t.rowCol(t.caret)
		cx = textX + t.xOfInRow(row, col)
	} else {
		row, col = t.lineCol(t.caret)
		cx = textX + t.xOfInLine(row, col)
	}
	if t.composing {
		cx += t.preeditMeasure(t.preeditCaret)
	}
	cy := bounds.Y + pad - t.vscroll + float32(row)*lh
	return render.Rect{X: cx, Y: cy, W: caretWidth, H: lh}, true
}

// preeditMeasure returns the pixel width of the first n runes of the active
// preedit buffer (0 for a nil face, matching xOf's own nil-face convention)
// — shared by CaretScreenRect and renderComposing/renderComposingMultiline
// to place the caret INSIDE the preedit run at its own preeditCaret offset.
func (t *TextBox) preeditMeasure(n int) float32 {
	if t.face == nil {
		return 0
	}
	return t.face.Measure(string(t.preedit[:n])).W
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

// unindentCurrentLine implements Shift+Tab's unindent step (see
// SetTabInserts): removes up to tabInsertSpaces leading space runes from the
// start of the caret's current logical line — only that leading run, never
// touching spaces elsewhere on the line — and reports whether anything was
// removed (the caller marks the key handled regardless, per OnKey's own
// no-op convention; the bool is purely for this function's own tests). A
// no-op (no mutation) when the line has no leading spaces at all.
//
// The caret is kept sensible relative to the edit rather than snapping to
// the line start: replaceRange itself always lands the caret at start (the
// deletion point), so a caret that was further into the line — past the
// removed run — is walked back out by however many runes were actually
// removed; a caret that was WITHIN the removed run (e.g. sitting among the
// leading spaces) simply ends up at the line's new start, since those runes
// are gone.
func (t *TextBox) unindentCurrentLine() bool {
	line, _ := t.lineCol(t.caret)
	start := t.lineStart(line)
	end := start
	for end < len(t.runes) && end-start < tabInsertSpaces && t.runes[end] == ' ' {
		end++
	}
	if end == start {
		return false
	}
	removed := end - start
	oldCaret := t.caret
	t.replaceRange(start, end, "")
	if newCaret := oldCaret - removed; newCaret > start {
		t.SetCaret(newCaret)
	}
	return true
}

// blockSelection reports whether the range [start,end) — already normalized
// by Selection() — is a MULTI-LINE selection: start != end, and start/end
// fall on different logical lines (equivalently, the selected text contains
// at least one '\n'). OnKey's KeyTab case checks this to decide whether Tab/
// Shift+Tab should indent/outdent every touched line in place
// (indentSelectedLines/outdentSelectedLines) rather than run the original
// single-line behavior (insertText's selection-replace, or
// unindentCurrentLine) — a selection collapsed to a caret, or contained
// within one logical line, is never a block selection and keeps that
// original behavior unchanged.
func (t *TextBox) blockSelection(start, end int) bool {
	if start == end {
		return false
	}
	startLine, _ := t.lineCol(start)
	endLine, _ := t.lineCol(end)
	return startLine != endLine
}

// touchedLines returns the [first,last] (inclusive, 0-based) logical lines a
// block selection [start,end) (see blockSelection) touches for indent/
// outdent purposes: every logical line the range enters, EXCEPT the range's
// own last line when end sits exactly at THAT line's own column 0 — i.e. the
// selection runs through the previous line's newline into the very start of
// the next line without selecting any of the next line's own text. That
// trailing line is excluded in that case, the standard editor convention
// that selecting up to (but not into) a line leaves it untouched by
// block-indent — e.g. selecting one whole line plus its trailing newline
// indents only that one line, never the line after it too. Only meaningful
// when blockSelection(start, end) is true.
func (t *TextBox) touchedLines(start, end int) (first, last int) {
	first, _ = t.lineCol(start)
	var endCol int
	last, endCol = t.lineCol(end)
	if endCol == 0 && last > first {
		last--
	}
	return first, last
}

// indentSelectedLines implements Tab's block-indent step for a multi-line
// selection (see blockSelection): inserts tabInsertSpaces spaces at the
// start of every touched line (touchedLines), leaving all existing text
// intact — unlike insertText's single-line selection-replace, no selected
// text is ever deleted. Restores the selection afterward to span the whole
// touched block (including the newly-inserted indentation), so a repeated
// Tab press keeps indenting further.
//
// Applied as a SINGLE replaceRange over the whole touched block
// [lineStart(first), lineEnd(last)) — computed once, up front, by splitting
// that span's own text on '\n' (touchedLines guarantees exactly one element
// per touched line, since the span excludes any real newline outside it)
// and rejoining with pad prepended to every line — rather than one
// replaceRange per line: a block Tab press is one edit from the caller's
// perspective, so it fires OnChanged (and invalidates arrange/measure) once,
// not once per line.
func (t *TextBox) indentSelectedLines() {
	start, end := t.Selection()
	first, last := t.touchedLines(start, end)
	pad := strings.Repeat(" ", tabInsertSpaces)

	blockStart, blockEnd := t.lineStart(first), t.lineEnd(last)
	lines := strings.Split(string(t.runes[blockStart:blockEnd]), "\n")
	newText := pad + strings.Join(lines, "\n"+pad)

	t.replaceRange(blockStart, blockEnd, newText)
	t.Select(blockStart, blockStart+len([]rune(newText)))
}

// outdentSelectedLines implements Shift+Tab's block-outdent step for a
// multi-line selection (see blockSelection): removes up to tabInsertSpaces
// leading space runes from the start of every touched line (touchedLines) —
// unindentCurrentLine's own leading-run removal, just applied to every
// touched line instead of only the caret's own. A line with fewer than
// tabInsertSpaces leading spaces loses only what it has; a line with none is
// left untouched. Restores the selection afterward to span the whole
// touched block.
//
// Applied as a SINGLE replaceRange over the whole touched block, the same
// coalescing indentSelectedLines uses (see its own doc comment for why):
// the block's text is split on '\n', each line has its own leading-space
// run stripped (up to tabInsertSpaces runes), and the result is rejoined
// and swapped in with one replaceRange — one OnChanged per block Shift+Tab
// press that actually removes something, not one per line. Unlike
// indentSelectedLines (which always inserts pad, so always a real change),
// a block with no leading spaces anywhere is a genuine no-op: replaceRange
// is skipped entirely so no OnChanged fires, matching the original
// per-line loop's own behavior (it never called replaceRange for a line
// with nothing to remove) — but the selection is still restored to span
// the touched block either way.
func (t *TextBox) outdentSelectedLines() {
	start, end := t.Selection()
	first, last := t.touchedLines(start, end)

	blockStart, blockEnd := t.lineStart(first), t.lineEnd(last)
	orig := string(t.runes[blockStart:blockEnd])
	lines := strings.Split(orig, "\n")
	for i, line := range lines {
		j := 0
		for j < len(line) && j < tabInsertSpaces && line[j] == ' ' {
			j++
		}
		lines[i] = line[j:]
	}
	newText := strings.Join(lines, "\n")

	if newText != orig {
		t.replaceRange(blockStart, blockEnd, newText)
	}
	t.Select(blockStart, blockStart+len([]rune(newText)))
}

// --- Word wrap (opt-in; see SetWordWrap) ---

// visualRow is one displayed row of multi-line text while wrapping (see
// wrapping/SetWordWrap): [start,end) is a rune-index range into t.runes —
// the row's own text, excluding a trailing real '\n' (matching lineStart/
// lineEnd's own convention) and excluding any soft-wrap space dropped at a
// word-boundary break (see wrapLogicalLine). Only ever populated/consulted
// while wrapping() is true.
type visualRow struct {
	start, end int
}

// computeVisualRows builds the full visual-rows layout for runes at content
// width (logical px, already padding-inset — see contentWidth): every real
// '\n' is always a row boundary (exactly like lineStart/lineEnd), and each
// resulting logical line is independently broken into one or more rows by
// wrapLogicalLine. A nil face has no glyph widths to wrap by, so it
// degrades to one row per logical line — the same layout wrapping() off
// would produce — matching every other nil-face convention in this file.
// Always returns at least one row (a single empty one for an empty runes).
func computeVisualRows(runes []rune, face *text.Face, width float32) []visualRow {
	if face == nil {
		return logicalLineRows(runes)
	}
	var rows []visualRow
	lineStart := 0
	for i := 0; i <= len(runes); i++ {
		if i == len(runes) || runes[i] == '\n' {
			rows = append(rows, wrapLogicalLine(runes, lineStart, i, face, width)...)
			lineStart = i + 1
		}
	}
	return rows
}

// logicalLineRows splits runes into one visualRow per real '\n'-delimited
// logical line, with no soft wrapping at all — computeVisualRows' fallback
// for a nil face, and (conceptually) what wrapLogicalLine degenerates to
// when a whole line already fits within width.
func logicalLineRows(runes []rune) []visualRow {
	var rows []visualRow
	lineStart := 0
	for i := 0; i <= len(runes); i++ {
		if i == len(runes) || runes[i] == '\n' {
			rows = append(rows, visualRow{start: lineStart, end: i})
			lineStart = i + 1
		}
	}
	return rows
}

// wrapLogicalLine breaks the single logical line runes[start:end) (no '\n'
// inside it) into one or more visual rows at width, preferring to break
// after the last space seen so far (a word boundary) and falling back to a
// character break only when a single "word" is itself wider than width.
// Relies on face.Measure's width being prefix-monotonic (more runes never
// narrower), and — like every other index-based helper in this file (see
// the type doc comment's "v0 simplification" note) — walks and re-measures
// substrings rather than keeping a per-rune advance cache; O(n) per row
// boundary, fine for realistic multi-line content.
//
// Three cases, checked in order, whenever the row-so-far plus rune i
// exceeds width:
//  1. The row is still empty (i == its own start): rune i is kept anyway —
//     nothing narrower is possible — and the row ends right after it, so a
//     single rune wider than width still gets exactly one row instead of an
//     infinite loop.
//  2. Rune i itself is the space that overflowed: the row ends BEFORE it
//     and the next row starts right after it — the space is dropped from
//     display entirely (shown as neither trailing nor leading whitespace),
//     the conventional "eat the wrap-point space" rule.
//  3. Otherwise: break after the last space seen in this row, if any (a
//     word boundary); with no space at all in the row, character-break
//     right at i.
func wrapLogicalLine(runes []rune, start, end int, face *text.Face, width float32) []visualRow {
	if start == end {
		return []visualRow{{start: start, end: end}}
	}

	var rows []visualRow
	rowStart := start
	lastSpace := -1

	for i := rowStart; i < end; i++ {
		w := face.Measure(string(runes[rowStart : i+1])).W
		if runes[i] == ' ' {
			lastSpace = i
		}
		if w <= width {
			continue
		}

		switch {
		case i == rowStart:
			rows = append(rows, visualRow{start: rowStart, end: i + 1})
			rowStart = i + 1
		case runes[i] == ' ':
			rows = append(rows, visualRow{start: rowStart, end: i})
			rowStart = i + 1
		case lastSpace >= rowStart:
			rows = append(rows, visualRow{start: rowStart, end: lastSpace + 1})
			rowStart = lastSpace + 1
		default:
			rows = append(rows, visualRow{start: rowStart, end: i})
			rowStart = i
		}
		lastSpace = -1
		i = rowStart - 1 // resume scanning from the new row's own start
	}

	// Only add a trailing row if there is unconsumed text left after the
	// last break: a break inside the loop can, in a sufficiently narrow
	// (even zero-width) degenerate case, land exactly on rowStart==end —
	// e.g. two runes in a row each too wide for width alone (case 1 twice
	// in succession) — and an unconditional append here would then tack on
	// a spurious empty row after content that already covers the whole
	// line.
	if rowStart < end {
		rows = append(rows, visualRow{start: rowStart, end: end})
	}
	return rows
}

// fullContentWidth returns the padding-inset content width WITHOUT
// subtracting the vertical-scroll thumb's gutter, even when the thumb is
// currently shown — used only by computeShowVScroll to decide whether to
// show the thumb in the first place (see its own doc comment for why that
// decision must use this ungated width rather than contentWidth() below).
func (t *TextBox) fullContentWidth() float32 {
	w := t.Bounds().W - 2*t.metrics.PaddingM
	if w < 0 {
		w = 0
	}
	return w
}

// contentHeight returns the padding-inset content (viewport) height — the
// height axis has no gutter of its own to further subtract (the vertical
// thumb's gutter is horizontal-only), so this is also exactly the value
// ArrangeContent computes as innerH.
func (t *TextBox) contentHeight() float32 {
	h := t.Bounds().H - 2*t.metrics.PaddingM
	if h < 0 {
		h = 0
	}
	return h
}

// contentWidth returns the current inner content width in logical px that
// TEXT actually gets to use — fullContentWidth, further reduced by the
// vertical-scroll thumb's gutter whenever vScrollShown is true (see
// computeShowVScroll/ArrangeContent, which caches that decision once per
// arrange pass) — used to key the visual-rows layout at runtime (see
// visualRows) and to clamp horizontal scroll (updateHScroll), so both
// always reflect the box's actual, thumb-aware arranged width.
func (t *TextBox) contentWidth() float32 {
	w := t.fullContentWidth()
	if t.vScrollShown {
		w -= t.metrics.ScrollGutter
		if w < 0 {
			w = 0
		}
	}
	return w
}

// totalContentHeight returns the current total (unclipped) content height:
// row/line count times lineHeight, using the visual-rows count while
// wrapping (at the current, gutter-aware contentWidth — safe to call here,
// unlike inside computeShowVScroll, since every caller of this method runs
// AFTER vScrollShown has already been decided for this arrange pass) or the
// logical-line count otherwise. NOT used by computeShowVScroll itself (see
// its own doc comment on why that needs the ungated width instead); this
// one backs updateVScroll's maxScroll clamp and the thumb's own geometry
// (vScrollThumbRect/dragVScroll).
func (t *TextBox) totalContentHeight() float32 {
	lh := t.lineHeight()
	var rows int
	if t.wrapping() {
		rows = t.rowCount()
	} else {
		rows = t.lineCount()
	}
	return float32(rows) * lh
}

// computeShowVScroll decides whether the vertical scroll thumb should be
// shown (and its gutter reserved) for the box's CURRENT arrange bounds:
// true only in multiline mode, only when the content's total row/line
// height exceeds the viewport height (contentHeight). Deliberately uses
// fullContentWidth — the width BEFORE any gutter is subtracted — to compute
// the wrapped row count while wrapping, rather than the gutter-aware
// contentWidth(): reserving the gutter is itself a consequence of this
// decision, so using the already-gutter-reduced width here would make the
// decision depend on its own prior result. This is safe and never
// flip-flops: a NARROWER width can only produce the same or MORE wrapped
// rows, never fewer (see wrapLogicalLine), so content that already overflows
// at the full (ungated) width is guaranteed to still overflow — never
// LESS — once the gutter narrows things further. Called once per arrange
// pass (ArrangeContent caches the result in vScrollShown); every other
// method in this file that cares reads that cached field rather than
// recomputing, so a wrapped, overflowing box does not re-run this (and the
// extra, ungated visualRows computation it implies) on every render/hit-test
// call.
func (t *TextBox) computeShowVScroll() bool {
	if !t.multiline {
		return false
	}
	viewportH := t.contentHeight()
	if viewportH <= 0 {
		return false
	}
	lh := t.lineHeight()
	var rows int
	if t.wrapping() {
		rows = len(t.visualRows(t.fullContentWidth()))
	} else {
		rows = t.lineCount()
	}
	return float32(rows)*lh > viewportH
}

// wrapMeasureWidth resolves the content width MeasureContent should wrap
// against from the available OUTER size the layout engine offers (already
// reflecting any explicit SetWidth, via core.MeasureWidget's explicit-size
// precedence — so in the common explicit-width case, this is exactly the
// width that was requested): available.W minus padding, or — if available.W
// is unconstrained (+Inf, e.g. a container offering infinite cross-axis
// space) — textBoxDefaultWidth's own content width, so a wrapped box with
// no explicit width still gets a deterministic, finite measurement rather
// than wrapping against infinity (which would degenerate to "never wrap").
func (t *TextBox) wrapMeasureWidth(availableW float32) float32 {
	if math.IsInf(float64(availableW), 1) {
		availableW = textBoxDefaultWidth
	}
	w := availableW - 2*t.metrics.PaddingM
	if w < 0 {
		w = 0
	}
	return w
}

// visualRows returns the current wrap layout for width, recomputing it only
// when width or the text has changed since the last call (rowsWidth/rowsRev
// vs. width/t.rev) — the single cache MeasureContent, ArrangeContent, every
// row-based caret/hit-test helper, and renderMultilineWrapped all share, so
// a wrapped TextBox re-flows once per real change (a text edit or a width
// change) rather than every frame or every call.
func (t *TextBox) visualRows(width float32) []visualRow {
	if t.rowsValid && t.rowsWidth == width && t.rowsRev == t.rev {
		return t.rows
	}
	t.rows = computeVisualRows(t.runes, t.face, width)
	t.rowsWidth = width
	t.rowsRev = t.rev
	t.rowsValid = true
	return t.rows
}

// rowCount returns the number of visual rows in the current wrap layout (at
// the box's current contentWidth) — the wrapping analogue of lineCount().
func (t *TextBox) rowCount() int {
	return len(t.visualRows(t.contentWidth()))
}

// rowStart and rowEnd are rowCol's inverse building blocks — the wrapping
// analogues of lineStart/lineEnd — returning row idx's own [start,end)
// rune-index range in the current wrap layout; an out-of-range idx (callers
// always clamp first, but a defensive check costs nothing) returns
// len(t.runes).
func (t *TextBox) rowStart(idx int) int {
	rows := t.visualRows(t.contentWidth())
	if idx < 0 || idx >= len(rows) {
		return len(t.runes)
	}
	return rows[idx].start
}

func (t *TextBox) rowEnd(idx int) int {
	rows := t.visualRows(t.contentWidth())
	if idx < 0 || idx >= len(rows) {
		return len(t.runes)
	}
	return rows[idx].end
}

// indexOfRowCol is rowCol's inverse: the rune index of column col on visual
// row idx, with col clamped to that row's own length — the wrapping
// analogue of indexOfLineCol, used the same way by moveCaretVertical's
// desiredCol logic and by caretIndexAtPos's click hit-testing.
func (t *TextBox) indexOfRowCol(idx, col int) int {
	start, end := t.rowStart(idx), t.rowEnd(idx)
	return start + clampInt(col, 0, end-start)
}

// rowCol maps rune index i (0..len(runes)) to its 0-based (row, col) in the
// current wrap layout — the wrapping analogue of lineCol. A caret exactly
// at a row boundary (soft OR hard — computeVisualRows makes no distinction
// once the rows exist) is reported at the END of the UPPER row, col ==
// that row's own length: the same tie-break lineCol already applies at a
// real '\n' (see its own doc comment, "just before the '\n': end of that
// line"), kept consistent between hard and soft breaks so Up/Down/Home/End
// behave identically at either kind.
func (t *TextBox) rowCol(i int) (row, col int) {
	rows := t.visualRows(t.contentWidth())
	for idx := range rows {
		if i <= rows[idx].end {
			return idx, i - rows[idx].start
		}
	}
	last := len(rows) - 1
	return last, i - rows[last].start
}

// xOfInRow is xOfInLine's wrapping analogue: the x-offset (logical px, from
// the start of row idx's own text) of column col within it.
func (t *TextBox) xOfInRow(idx, col int) float32 {
	if t.face == nil {
		return 0
	}
	rows := t.visualRows(t.contentWidth())
	if idx < 0 || idx >= len(rows) {
		return 0
	}
	start := rows[idx].start
	return t.face.Measure(string(t.runes[start : start+col])).W
}

// rowAtY is lineAtY's wrapping analogue: the 0-based visual row index
// nearest window-space y, clamped to [0, rowCount()-1].
func (t *TextBox) rowAtY(windowY float32) int {
	lh := t.lineHeight()
	if lh <= 0 {
		return 0
	}
	row := int(t.localTextY(windowY) / lh)
	return clampInt(row, 0, t.rowCount()-1)
}

// colAtXInRow is colAtX's wrapping analogue: the column within visual row
// idx nearest local x-coordinate x, using the same "compare against each
// boundary's midpoint" rule, bounded to idx's own rune count.
func (t *TextBox) colAtXInRow(idx int, x float32) int {
	n := t.rowEnd(idx) - t.rowStart(idx)
	for i := 0; i < n; i++ {
		mid := (t.xOfInRow(idx, i) + t.xOfInRow(idx, i+1)) / 2
		if x < mid {
			return i
		}
	}
	return n
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
// mode it is xOfInLine of the caret's own (line, col) — or, while wrapping
// (see SetWordWrap), xOfInRow of its own (row, col) — since multi-line
// hscroll is keyed off the caret's own line/row alone (see the SetMultiline
// doc comment's horizontal-scroll simplification). Only reachable in
// practice for the unwrapped path: updateHScroll, caretX's only caller,
// pins hscroll at 0 and returns before ever calling this while wrapping —
// the wrapping branch here exists so caretX stays correct (and safely
// callable) even if that changes.
func (t *TextBox) caretX() float32 {
	if !t.multiline {
		return t.xOf(t.caret)
	}
	if t.wrapping() {
		row, col := t.rowCol(t.caret)
		return t.xOfInRow(row, col)
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
	if t.wrapping() {
		// Same desiredCol bookkeeping as below, but moving between VISUAL
		// rows instead of logical lines — see rowCol/indexOfRowCol.
		row, col := t.rowCol(t.caret)
		dc := col
		if t.desiredColValid {
			dc = t.desiredCol
		}

		newRow := clampInt(row+delta, 0, t.rowCount()-1)
		idx := t.indexOfRowCol(newRow, dc)

		if extend {
			t.Select(t.anchor, idx)
		} else {
			t.SetCaret(idx)
		}

		t.desiredCol = dc
		t.desiredColValid = true
		return
	}

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
// the column within that row via colAtX, combined through indexOfLineCol —
// or, while wrapping (see SetWordWrap), the same shape but over VISUAL rows
// (rowAtY/colAtXInRow/indexOfRowCol), so a click inside a wrapped line lands
// the caret at the right buffer index.
func (t *TextBox) caretIndexAtPos(x, y float32) int {
	if !t.multiline {
		return t.caretIndexAtX(t.localTextX(x))
	}
	if t.wrapping() {
		row := t.rowAtY(y)
		col := t.colAtXInRow(row, t.localTextX(x))
		return t.indexOfRowCol(row, col)
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
// available is never consulted in either of those cases.
//
// While wrapping (see SetWordWrap), available IS consulted — this is the
// one case where the box's desired size actually depends on it, WPF-style:
// the content width to wrap against is resolved from available.W
// (wrapMeasureWidth — already reflecting an explicit SetWidth, per
// core.MeasureWidget's precedence, in the common case), the visual-rows
// layout is computed (and cached — see visualRows) at that width, and the
// reported height is exactly that row count times lineHeight() plus
// padding, so a wrapped box measures as tall as its wrapped content
// actually needs. ArrangeContent re-resolves the SAME layout against the
// box's final arranged width (via contentWidth/visualRows sharing the
// identical cache), so the two only disagree — leaving the wrapped content
// taller than the assigned bounds, handled by vertical scrolling exactly
// like any other over-long multi-line content — in the atypical case where
// arrange assigns a different width than measure was given (e.g. a
// Stretch-aligned box in a container that resizes between passes); no
// second measure pass is triggered for that, by design (see ArrangeContent).
func (t *TextBox) MeasureContent(available render.Size) render.Size {
	lh := t.lineHeight()

	if t.wrapping() {
		width := t.wrapMeasureWidth(available.W)
		rows := t.visualRows(width)
		h := lh*float32(len(rows)) + 2*t.metrics.PaddingM
		return render.Size{W: width + 2*t.metrics.PaddingM, H: h}
	}

	h := lh + 2*t.metrics.PaddingM
	if t.multiline {
		h = lh*textBoxMultilineDefaultLines + 2*t.metrics.PaddingM
	}
	return render.Size{W: textBoxDefaultWidth, H: h}
}

// ArrangeContent is the single source of truth for hscroll/vscroll clamping
// (the ScrollViewer clamp-in-arrange pattern applied to each scroll axis)
// AND for the vertical-scroll thumb's show/hide decision (computeShowVScroll,
// cached into vScrollShown): it recomputes the padding-inset inner
// width/height from the arranged bounds and clamps hscroll (always) and
// vscroll (multi-line mode only — updateVScroll is a no-op-to-zero in
// single-line mode) so the caret stays visible within them.
//
// vScrollShown is refreshed FIRST, before anything else runs, because
// contentWidth() — which updateHScroll's width argument, and every
// wrap-aware helper invoked for the rest of this pass, depend on — reduces
// by the thumb's gutter exactly when vScrollShown is true. A non-overflowing
// box (vScrollShown false) sees contentWidth() == fullContentWidth() (no
// gutter reserved), so this whole mechanism is invisible to a box that never
// needed to scroll — existing goldens with no vertical overflow render
// byte-for-byte as before this feature.
//
// While wrapping (see SetWordWrap), this is also where the visual-rows
// layout gets re-flowed against the FINAL arranged (and now possibly
// gutter-reduced) width if it differs from whatever width MeasureContent
// last wrapped against: updateVScroll (via rowCount/rowCol) calls
// contentWidth(), which reads bounds.W straight from t.Bounds() — already
// set to this exact bounds by core.ArrangeWidget before calling here — and
// visualRows' own cache (keyed on that width) transparently recomputes on a
// mismatch. No separate "re-wrap" step is needed beyond that cache lookup.
func (t *TextBox) ArrangeContent(bounds render.Rect) {
	// Refreshed first: contentWidth (used by updateHScroll below, and by
	// every wrap-aware helper for the rest of this pass) depends on it.
	t.vScrollShown = t.computeShowVScroll()

	t.updateHScroll(t.contentWidth())

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
//
// While wrapping (see SetWordWrap), horizontal scrolling is disabled
// outright: wrapped rows never exceed innerW by construction, so hscroll is
// simply pinned at 0 rather than computed at all.
func (t *TextBox) updateHScroll(innerW float32) {
	if t.wrapping() {
		t.hscroll = 0
		return
	}

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
// also capped at max(0, lineCount()*lineHeight()-innerH)). While wrapping
// (see SetWordWrap), "line" here means VISUAL row throughout — the caret's
// own row (rowCol) and the total row count (rowCount), both at the box's
// current contentWidth — rather than the logical-line equivalents.
func (t *TextBox) updateVScroll(innerH float32) {
	if !t.multiline {
		t.vscroll = 0
		return
	}

	lh := t.lineHeight()
	var row int
	if t.wrapping() {
		row, _ = t.rowCol(t.caret)
	} else {
		row, _ = t.lineCol(t.caret)
	}
	caretY := float32(row) * lh

	if caretY-t.vscroll < 0 {
		t.vscroll = caretY
	}
	if caretY-t.vscroll > innerH-lh {
		t.vscroll = caretY - (innerH - lh)
	}
	if t.vscroll < 0 {
		t.vscroll = 0
	}

	maxScroll := t.totalContentHeight() - innerH
	if maxScroll < 0 {
		maxScroll = 0
	}
	if t.vscroll > maxScroll {
		t.vscroll = maxScroll
	}
}

// --- Vertical scroll thumb (shown only while content overflows) ---

// vScrollTrack returns the vertical thumb's track rect — a scrollGutter-wide
// strip along the right inner edge, inset by PaddingM on all four sides like
// the text itself (so the thumb sits comfortably inside the sunken well
// rather than overlapping its bevel) — and ok==false whenever vScrollShown
// is false (see computeShowVScroll/ArrangeContent) or the box has no usable
// height to show a track in.
func (t *TextBox) vScrollTrack() (render.Rect, bool) {
	if !t.vScrollShown {
		return render.Rect{}, false
	}
	bounds := t.Bounds()
	pad := t.metrics.PaddingM
	gutter := t.metrics.ScrollGutter
	h := bounds.H - 2*pad
	if h <= 0 {
		return render.Rect{}, false
	}
	return render.Rect{
		X: bounds.Right() - pad - gutter,
		Y: bounds.Y + pad,
		W: gutter,
		H: h,
	}, true
}

// vScrollThumbRect returns the thumb's current on-screen rect — track
// position plus the offset proportional to the current vscroll — reusing
// ScrollViewer's own shared thumb-sizing/positioning math (scrollThumbLength/
// scrollThumbPos, scrollviewer.go) so it looks and behaves identically.
// ok==false exactly when vScrollTrack is (nothing to scroll, so nothing to
// draw or hit-test against).
func (t *TextBox) vScrollThumbRect() (render.Rect, bool) {
	track, ok := t.vScrollTrack()
	if !ok {
		return render.Rect{}, false
	}
	total := t.totalContentHeight()
	thumbH := scrollThumbLength(track.H, total)
	maxOffset := total - track.H
	thumbY := scrollThumbPos(track.Y, track.H, thumbH, t.vscroll, maxOffset)
	return render.Rect{X: track.X, Y: thumbY, W: track.W, H: thumbH}, true
}

// dragVScroll recomputes t.vscroll directly from a vertical-thumb drag's
// current pointer y-position, via scrollDragOffset — the same drag math
// ScrollViewer.dragTo and virtualizer.dragTo already share (scrollviewer.go),
// keeping the pointer at the same relative grab point within the thumb
// (vDragGrab) it was at when the drag began.
//
// Deliberately does NOT call InvalidateArrange, unlike every other
// vscroll-affecting path in this file: updateVScroll's caret-follow clamp
// (see its own doc comment) starts from whatever t.vscroll ALREADY is and
// only adjusts it if the caret has scrolled out of view — so invalidating
// arrange here would let the very next layout pass immediately snap the
// drag right back to the caret's position, undoing it. fluo only re-runs
// Measure/Arrange when NeedsLayout is dirty (see app.Surface.Frame) or the
// window resizes, and no OTHER path in this file marks arrange dirty on a
// pointer Move — so setting vscroll directly here is enough for it to take
// effect on the very next Render call and simply persist, exactly like a
// real editor's "manual scroll sticks until you type or move the caret
// again": the next actual caret-moving mutation (SetCaret/Select/
// replaceRange/SetText, all of which DO invalidate arrange) naturally
// re-syncs the view via that same existing clamp.
func (t *TextBox) dragVScroll(posY float32) {
	track, ok := t.vScrollTrack()
	if !ok {
		return
	}
	thumb, ok := t.vScrollThumbRect()
	if !ok {
		return
	}
	maxOffset := t.totalContentHeight() - track.H
	if maxOffset <= 0 {
		return
	}
	t.vscroll = scrollDragOffset(track.Y, track.H, thumb.H, posY, t.vDragGrab, maxOffset)
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

	if t.composing {
		t.renderComposing(r, textX, textY, lh)
		return
	}

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

// renderComposing is Render's single-line body while an IME composition is
// active (t.composing): the committed text before the caret, then the
// provisional preedit run spliced in with a thin underline beneath it
// (preeditUnderlineThickness — see its own doc comment), then the committed
// text after the caret, and finally the caret itself — positioned INSIDE
// the preedit run at preeditCaret (see preeditMeasure/CaretScreenRect's
// matching math) rather than at t.caret. Deliberately draws no selection
// highlight: starting a composition is treated like any other edit that
// collapses a prior selection (OnComposition never itself touches
// anchor/caret directly, but every real IME input path that reaches it goes
// through a fresh caret position first), so there is no meaningful
// selection range left to intersect against the spliced preedit run.
func (t *TextBox) renderComposing(r render.Renderer, textX, textY, lh float32) {
	c := t.colors
	color := c.WindowText
	if !t.enabled {
		color = c.GrayText
	}

	if pre := string(t.runes[:t.caret]); t.face != nil && pre != "" {
		t.face.Draw(r, render.Point{X: textX, Y: textY}, pre, color)
	}
	preeditX := textX + t.xOf(t.caret)
	x := preeditX

	if preeditStr := string(t.preedit); t.face != nil && preeditStr != "" {
		t.face.Draw(r, render.Point{X: preeditX, Y: textY}, preeditStr, color)
		w := t.preeditMeasure(len(t.preedit))
		underlineY := textY + lh - preeditUnderlineThickness
		r.FillRect(render.Rect{X: preeditX, Y: underlineY, W: w, H: preeditUnderlineThickness}, color)
		x = preeditX + w
	}

	if post := string(t.runes[t.caret:]); t.face != nil && post != "" {
		t.face.Draw(r, render.Point{X: x, Y: textY}, post, color)
	}

	if t.caretShown() {
		cx := preeditX + t.preeditMeasure(t.preeditCaret)
		r.FillRect(render.Rect{X: cx, Y: textY, W: caretWidth, H: lh}, c.WindowText)
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
//
// While wrapping (see SetWordWrap) and t.runes is non-empty, this delegates
// entirely to renderMultilineWrapped instead — the logical-line body below
// never runs for that case. Two carve-outs from that dispatch, both
// deliberate v0 simplifications: an EMPTY box (showing its placeholder, a
// plain string with no wrap layout of its own — see visualRows, which is
// always keyed off t.runes) always renders via the unwrapped body below,
// same as if wrapping were off; and an ACTIVE IME composition always
// renders via renderComposingMultiline's own logical-line body, regardless
// of wrapping — a composition is transient and never contains a '\n' itself
// (see OnComposition), so it temporarily shows unwrapped (with hscroll
// re-enabled for that one frame's rendering, since updateHScroll's pin only
// applies through the normal arrange pass) rather than reflowing rows
// specifically for it.
func (t *TextBox) renderMultiline(r render.Renderer, bounds render.Rect) {
	c := t.colors
	pad := t.metrics.PaddingM
	lh := t.lineHeight()
	textX := bounds.X + pad - t.hscroll

	s, color := t.displayText()

	if t.composing {
		lines := strings.Split(s, "\n")
		t.renderComposingMultiline(r, bounds, textX, lh, lines, color)
		return
	}

	if t.wrapping() && len(t.runes) > 0 {
		t.renderMultilineWrapped(r, bounds, color)
		return
	}

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

// renderMultilineWrapped is renderMultiline's body while wrapping (see
// SetWordWrap) with non-empty text: the same per-row draw/select/caret
// shape as renderMultiline's own logical-line loop, but over the current
// visual-rows layout (t.visualRows(t.contentWidth())) and reading each
// row's text directly from t.runes (row.start:row.end) rather than from a
// strings.Split slice — rows carry no '\n' of their own to split on, soft
// breaks aren't in the buffer at all. hscroll is always 0 here (see
// updateHScroll), so textX has no scroll term to subtract.
func (t *TextBox) renderMultilineWrapped(r render.Renderer, bounds render.Rect, color render.Color) {
	c := t.colors
	pad := t.metrics.PaddingM
	lh := t.lineHeight()
	textX := bounds.X + pad

	rows := t.visualRows(t.contentWidth())
	start, end := t.Selection()

	for i, row := range rows {
		rowY := bounds.Y + pad - t.vscroll + float32(i)*lh
		lo, hi := row.start, row.end

		selStart, selEnd := clampInt(start, lo, hi), clampInt(end, lo, hi)
		hasSel := selStart < selEnd

		if hasSel {
			x0 := textX + t.xOfInRow(i, selStart-lo)
			x1 := textX + t.xOfInRow(i, selEnd-lo)
			r.FillRect(render.Rect{X: x0, Y: rowY, W: x1 - x0, H: lh}, c.Highlight)
		}

		if line := string(t.runes[lo:hi]); t.face != nil && line != "" {
			if hasSel {
				t.drawLineWithSelection(r, []rune(line), selStart-lo, selEnd-lo, textX, rowY, color)
			} else {
				t.face.Draw(r, render.Point{X: textX, Y: rowY}, line, color)
			}
		}
	}

	if t.caretShown() {
		row, col := t.rowCol(t.caret)
		cx := textX + t.xOfInRow(row, col)
		cy := bounds.Y + pad - t.vscroll + float32(row)*lh
		r.FillRect(render.Rect{X: cx, Y: cy, W: caretWidth, H: lh}, c.WindowText)
	}
}

// renderComposingMultiline is renderMultiline's counterpart to
// renderComposing (see its doc comment for why no selection highlight is
// drawn): lines/color are the same displayText()/strings.Split split
// renderMultiline itself computed. Only the caret's own line has the
// preedit spliced in — an IME composition string never itself contains a
// newline (see OnComposition/CaretScreenRect, which make the same
// assumption) — every other line draws exactly as renderMultiline's
// unselected branch would.
func (t *TextBox) renderComposingMultiline(r render.Renderer, bounds render.Rect, textX, lh float32, lines []string, color render.Color) {
	pad := t.metrics.PaddingM
	caretLine, caretCol := t.lineCol(t.caret)

	for i, line := range lines {
		lineY := bounds.Y + pad - t.vscroll + float32(i)*lh

		if i != caretLine {
			if t.face != nil && line != "" {
				t.face.Draw(r, render.Point{X: textX, Y: lineY}, line, color)
			}
			continue
		}

		runes := []rune(line)
		if pre := string(runes[:caretCol]); t.face != nil && pre != "" {
			t.face.Draw(r, render.Point{X: textX, Y: lineY}, pre, color)
		}
		preeditX := textX + t.xOfInLine(i, caretCol)
		x := preeditX

		if preeditStr := string(t.preedit); t.face != nil && preeditStr != "" {
			t.face.Draw(r, render.Point{X: preeditX, Y: lineY}, preeditStr, color)
			w := t.preeditMeasure(len(t.preedit))
			underlineY := lineY + lh - preeditUnderlineThickness
			r.FillRect(render.Rect{X: preeditX, Y: underlineY, W: w, H: preeditUnderlineThickness}, color)
			x = preeditX + w
		}

		if post := string(runes[caretCol:]); t.face != nil && post != "" {
			t.face.Draw(r, render.Point{X: x, Y: lineY}, post, color)
		}

		if t.caretShown() {
			cx := preeditX + t.preeditMeasure(t.preeditCaret)
			r.FillRect(render.Rect{X: cx, Y: lineY, W: caretWidth, H: lh}, t.colors.WindowText)
		}
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

// RenderOverlay draws no separate focus ring: classic Windows textboxes
// don't (unlike every other focusable control in this package, per
// drawFocusRing's doc comment in clickable.go) — the caret and the sunken
// well already read as "this is the focused field," and the box has no
// raised/sunken chrome swap to signal focus with either (unlike Button's
// press-sunken state).
//
// It DOES draw the vertical scroll thumb (drawScrollThumb, the same classic
// track+raised-thumb chrome ScrollViewer and the ListView/DataGrid
// virtualizer already share — see bevel.go) whenever vScrollTrack reports
// one (i.e. vScrollShown is true — the content overflows the viewport
// vertically; see computeShowVScroll). Drawn last, after core.RenderWidget's
// own clip push/pop around this leaf's (empty) Children() list, so — like
// every other OverlayRenderer in this package — it is never itself clipped.
func (t *TextBox) RenderOverlay(r render.Renderer) {
	if track, ok := t.vScrollTrack(); ok {
		thumb, _ := t.vScrollThumbRect()
		drawScrollThumb(r, track, thumb, t.colors)
	}
}

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
// Up/Down are multi-line-only (see SetMultiline): in single-line mode they
// fall through unhandled exactly as before multi-line mode existed, simply
// not part of the single-line keyboard map. Enter is multi-line-only too in
// the sense that it never inserts text in single-line mode — but there it
// instead fires OnSubmit (if one is set) with the current text; with no
// OnSubmit set, it falls through unhandled exactly as before OnSubmit
// existed. Home/End are handled in both modes, but target the whole text in
// single-line mode and the caret's own line in multi-line mode
// (homeTarget/endTarget). Tab is opt-in and multi-line-only (see
// SetTabInserts): with that flag off (the default), or in single-line mode
// regardless of the flag, Tab falls through unhandled exactly as before that
// feature existed, bubbling to the Router's own Tab/Shift+Tab focus-nav.
//
// Ctrl adds several editor-ergonomics combinations, checked before the plain
// keyboard map below (so a bare Left/Right/Home/End/Backspace/Delete still
// works exactly as before whenever Ctrl ISN'T held): Ctrl+Home/End jump to
// the very start/end of the whole buffer (0/len(runes)) — in single-line
// mode this is the same target plain Home/End already use, but in
// multi-line mode it is the one way to reach the buffer's true bounds,
// since plain Home/End there stop at the caret's own line (homeTarget/
// endTarget). Ctrl+Left/Right move by whole words (prevWordBoundary/
// nextWordBoundary) instead of one rune, crossing line boundaries for free
// (a newline counts as whitespace there). Ctrl+Backspace/Delete delete the
// previous/next word (deleteWordBackward/deleteWordForward) — or the active
// selection instead, same selection-first convention as plain Backspace/
// Delete. Every Ctrl+<key> combination above accepts Shift too, extending
// the selection from the current anchor exactly like its plain counterpart
// (see moveCaretTo).
//
// PageUp/PageDown are multi-line-only (see SetMultiline), like Up/Down:
// they move the caret by pageRows() lines/rows via moveCaretVertical,
// preserving its own desired-column tracking, and fall through unhandled in
// single-line mode.
func (t *TextBox) OnKey(e *input.KeyEvent) {
	if !t.enabled || !t.focused || e.Action != input.Press {
		return
	}

	// An active IME composition owns keyboard input until it ends (see
	// OnComposition): on Windows, WM_CHAR/normal key messages simply don't
	// arrive for the keystrokes composing a CJK candidate in the first
	// place, so this guard mainly defends against programmatic misuse (a
	// caller driving OnComposition directly — e.g. a test — while also
	// dispatching an ordinary KeyDown) mutating the committed buffer out
	// from under an unfinished composition.
	if t.composing {
		return
	}

	shift := e.Mods&input.ModShift != 0

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
		case input.KeyHome:
			// The whole-buffer jump plain Home never reaches in multi-line
			// mode (homeTarget stops at the caret's own line there) — see
			// SetMultiline. In single-line mode this is exactly homeTarget's
			// own answer (0), so no separate single-line case is needed.
			t.moveCaretTo(0, shift)
			e.Handled = true
			return
		case input.KeyEnd:
			t.moveCaretTo(len(t.runes), shift)
			e.Handled = true
			return
		case input.KeyLeft:
			t.moveCaretTo(t.prevWordBoundary(t.caret), shift)
			e.Handled = true
			return
		case input.KeyRight:
			t.moveCaretTo(t.nextWordBoundary(t.caret), shift)
			e.Handled = true
			return
		case input.KeyBackspace:
			t.deleteWordBackward()
			e.Handled = true
			return
		case input.KeyDelete:
			t.deleteWordForward()
			e.Handled = true
			return
		}
	}

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
	case input.KeyTab:
		if t.multiline && t.tabInserts {
			start, end := t.Selection()
			block := t.blockSelection(start, end)
			switch {
			case shift && block:
				t.outdentSelectedLines()
			case shift:
				t.unindentCurrentLine()
			case block:
				t.indentSelectedLines()
			default:
				t.insertText(strings.Repeat(" ", tabInsertSpaces))
			}
			e.Handled = true
			return
		}
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
	case input.KeyPageUp:
		if t.multiline {
			t.moveCaretVertical(-t.pageRows(), shift)
			e.Handled = true
			return
		}
	case input.KeyPageDown:
		if t.multiline {
			t.moveCaretVertical(t.pageRows(), shift)
			e.Handled = true
			return
		}
	case input.KeyEnter:
		if t.multiline {
			t.insertText("\n")
			e.Handled = true
			return
		}
		if t.onSubmit != nil {
			t.onSubmit(string(t.runes))
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
// A Press landing inside the current vertical thumb rect (vScrollThumbRect,
// only ever non-empty while vScrollShown — see computeShowVScroll) starts a
// THUMB drag instead of the caret-drag-to-select described below: it
// records the pointer's grab offset within the thumb (vDragGrab) and sets
// vDragging, checked first (mirroring ScrollViewer.OnPointer's own
// thumb-vs-content priority). Move, while captured with vDragging set,
// scrolls via dragVScroll instead of extending the selection; Release clears
// vDragging alongside releasing capture, matching the caret-drag path.
//
// Otherwise (no thumb, or the click missed it): Press moves the caret to
// the nearest rune boundary to the click position (via caretIndexAtPos — x
// only in single-line mode, x AND y in multi-line mode, see its doc
// comment) — which also clears any existing selection and sets the drag
// anchor, since SetCaret sets anchor==caret — and captures the pointer so
// the drag survives leaving the TextBox's bounds. Move, only while this
// TextBox holds the capture, extends the selection from that same anchor to
// the new nearest boundary (Select(t.anchor, idx): t.anchor is left
// untouched by Select's own reassignment since the same value is passed
// back in, so it stays pinned at the press position across an entire drag).
// Release, only while captured, ends the drag.
func (t *TextBox) OnPointer(e *input.PointerEvent) {
	if !t.enabled {
		if e.Router != nil && e.Router.Captured() == t {
			e.Router.Release()
			t.vDragging = false
		}
		return
	}
	switch e.Action {
	case input.Press:
		if rect, ok := t.vScrollThumbRect(); ok && rect.Contains(e.Pos) {
			t.vDragging = true
			t.vDragGrab = e.Pos.Y - rect.Y
			e.Router.Capture(t)
			e.Handled = true
			return
		}
		idx := t.caretIndexAtPos(e.Pos.X, e.Pos.Y)
		t.SetCaret(idx)
		e.Router.Capture(t)
		e.Handled = true
	case input.Move:
		if e.Router.Captured() == t {
			if t.vDragging {
				t.dragVScroll(e.Pos.Y)
			} else {
				idx := t.caretIndexAtPos(e.Pos.X, e.Pos.Y)
				t.Select(t.anchor, idx)
			}
			e.Handled = true
		}
	case input.Release:
		if e.Router.Captured() == t {
			e.Router.Release()
			t.vDragging = false
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
