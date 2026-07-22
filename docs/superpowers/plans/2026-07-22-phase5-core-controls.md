# fluo Phase 5: Core Controls Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The core control set: popup/overlay layer, Button family (Button/ToggleButton/CheckBox/RadioButton/ToggleSwitch) on a shared ClickBehavior, TextBox (caret/selection/editing/clipboard/blink), Slider, ProgressBar, ComboBox, ToolTip — all token-styled, focusable, keyboard-accessible, tested headless + goldens, live in the gallery.

**Architecture:** Everything follows the established widget idiom. New input capabilities land first (Key consts, Clipboard abstraction, Router.Detach). Popups are children of an `OverlayHost` root widget found by parent-chain walk (`OverlayHostFor`). One shared `ClickBehavior` implements press-capture-release-inside for every button-like control. TextBox is split across two tasks (model/render, then interaction). Controls read tokens at construction (rebuild-to-retheme).

**Tech Stack:** Pinned deps only. All new controls headless-testable; goldens only for visual proof.

## Global Constraints

- All prior constraints bind (module path; go.mod PINNED; WSL-only Go, GOTOOLCHAIN=local; keyed literals; vet + gofmt gates; doc comments; per-task commits + trailer `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`; existing goldens never `-update`d).
- No glfw types outside `app`. No literal style colors in `controls` non-test code — tokens only (shape data like a checkmark's stroke geometry is fine).
- Every interactive control: implements `input.Focusable` (AcceptsFocus true unless disabled), shows a focus ring when focused (FocusStroke color, FocusStrokeWidth, drawn via RenderOverlay inset −2 outside the control rect... normative: ring = StrokeRoundedRect on control bounds INFLATED by 2, radius = control radius + 2), and supports keyboard activation as specified per control.
- Disabled state on every control: `SetEnabled(bool)`; disabled ⇒ AcceptsFocus false, pointer events ignored (not handled), token colors ControlFillDisabled/ControlStrokeDisabled/TextDisabled/AccentDisabled.
- New goldens use `theme.SetActive(theme.FluentLight())` + defer reset (light shows strokes best) unless stated.
- State changes that alter layout call InvalidateMeasure; visual-only state (hover/pressed/checked) needs no invalidation (host redraws every frame).

## File Structure

```
input/events.go        + KeySpace(32), KeyA(65), KeyC(67), KeyV(86), KeyX(88), KeyY(89), KeyZ(90); Clipboard interface
input/router.go        + SetClipboard/Clipboard; Detach(w)
app/window.go          + glfw-backed clipboard wired into the router
controls/overlay.go    OverlayHost, OverlayHostFor, popup stack
controls/clickable.go  ClickBehavior (shared press/hover state machine)
controls/button.go     Button (+accent), ToggleButton
controls/checkbox.go   CheckBox, RadioButton (+RadioGroup)
controls/toggleswitch.go
controls/textbox.go    TextBox
controls/slider.go     Slider
controls/progressbar.go
controls/combobox.go
controls/tooltip.go    ToolTipArea
cmd/fluo-gallery/main.go  controls page
render/gl/renderer_test.go + goldens: buttons.png, toggles.png, textbox.png, slider_progress.png, combo_open.png
```

Declared v0 cuts (scope, not gaps): TextBox single-line, no undo/redo (KeyY/KeyZ consts land now, feature later), no double-click word-select (no click-count synthesis), prefix-width caret math is O(n²) per layout (fine single-line); ProgressBar determinate only; Slider horizontal only; ToolTip shows on Enter with 600ms delay only when a timers queue is wired, else immediately; no animations (Phase 8).

---

### Task 1: input additions + clipboard plumbing

**Files:** `input/events.go`, `input/router.go`, `input/router_test.go`, `app/window.go`

**Produces (exact):**
```go
// events.go additions
const (KeySpace Key = 32; KeyA Key = 65; KeyC Key = 67; KeyV Key = 86; KeyX Key = 88; KeyY Key = 89; KeyZ Key = 90)
// Clipboard is the host-provided system clipboard access.
type Clipboard interface { Get() string; Set(s string) }
// router.go additions
func (r *Router) SetClipboard(c Clipboard)
func (r *Router) Clipboard() Clipboard // may be nil (headless); callers must nil-check
// Detach clears hover/capture/focus references that point at w OR any widget
// in w's subtree (walk via Children). Call before removing a subtree (popup close).
func (r *Router) Detach(w core.Widget)
```
app wiring: unexported `glfwClipboard{win}` implementing Get/Set via win.GetClipboardString/SetClipboardString; `router.SetClipboard(...)` before the loop.

- [ ] TDD: `TestDetachClearsSubtreeState` (focus+capture+hover a leaf inside a Canvas subtree; Detach(subtree root); all three cleared — focused cleared WITHOUT OnFocusChanged(false)?? Normative: Detach fires OnFocusChanged(false) on the detached focused widget, then clears), `TestDetachUnrelatedUntouched` (state on sibling survives), `TestClipboardAccessors` (fake clipboard set/get roundtrip through the router). FAIL → implement → green; suite+vet+gofmt.
- [ ] Commit: `feat(input,app): key consts, clipboard abstraction, Router.Detach`

### Task 2: OverlayHost (popup layer)

**Files:** `controls/overlay.go`, `controls/overlay_test.go`

**Produces (exact):**
```go
// OverlayHost hosts the app content plus a stack of popups rendered above it.
// It should be the root (or near-root) widget; controls find it via OverlayHostFor.
func NewOverlayHost() *OverlayHost
func (h *OverlayHost) SetContent(w core.Widget) *OverlayHost
// ShowPopup places popup near anchor (screen-space rect, typically the opener's
// bounds): preferred position = below anchor, left-aligned; flips above when the
// popup would overflow the host bottom; horizontally clamped into host bounds.
// onDismiss (may be nil) fires when the popup is light-dismissed or closed.
func (h *OverlayHost) ShowPopup(popup core.Widget, anchor render.Rect, onDismiss func())
func (h *OverlayHost) ClosePopup(popup core.Widget)  // idempotent; fires onDismiss; Router-detached by the CALLER? No: normative below
func (h *OverlayHost) CloseTopPopup()
func (h *OverlayHost) PopupCount() int
// OverlayHostFor walks core.ParentOf from w; returns nil if none found.
func OverlayHostFor(w core.Widget) *OverlayHost
```
Normative: Children = [content, popups...] (popups after ⇒ hit-test topmost & render above). Measure: content with full available; popups with full available too (they size to content). Arrange: content = full bounds; each popup = desired size at computed position. **Light dismiss:** OverlayHost implements input.PointerHandler; on Press whose hit path does NOT include the topmost popup: close the topmost popup and mark the event handled (swallow). This works because the host is an ancestor of everything, so unhandled presses bubble to it. **Router detach:** ClosePopup needs the router to clear stale focus/capture into the popup — but the host has no router ref. Normative: OverlayHost gets `SetRouter(r *input.Router)` (the gallery wires ctx.Input once); ClosePopup calls r.Detach(popup) when r != nil. Document.

- [ ] TDD headless: placement below-anchor (popup bounds under anchor), flip-above when near bottom, horizontal clamp, light-dismiss closes ONLY topmost + swallows (probe pattern with a Router), ClosePopup idempotent + onDismiss once, PopupCount, OverlayHostFor finds through nesting / nil without. FAIL → implement → green.
- [ ] Commit: `feat(controls): OverlayHost popup layer`

### Task 3: ClickBehavior + Button + ToggleButton

**Files:** `controls/clickable.go`, `controls/button.go`, tests, golden `buttons.png`

**Produces (exact):**
```go
// ClickBehavior implements the standard press state machine for button-like
// widgets: press captures, releasing inside fires. Embed by value.
type ClickBehavior struct{ /* hover, pressed bool; OnClick func() */ }
func (c *ClickBehavior) HandlePointer(e *input.PointerEvent, owner core.Widget) // uses core.BoundsOf(owner)
func (c *ClickBehavior) Hover() bool
func (c *ClickBehavior) Pressed() bool
// Activate fires OnClick (keyboard activation path).
func (c *ClickBehavior) Activate()

func NewButton(face *text.Face, label string) *Button
func (b *Button) OnClick(fn func()) *Button
func (b *Button) SetAccent(a bool) *Button
func (b *Button) SetEnabled(v bool) *Button
func (b *Button) Label() *TextBlock // for tests/customization
func NewToggleButton(face *text.Face, label string) *ToggleButton
func (t *ToggleButton) Checked() bool
func (t *ToggleButton) SetChecked(v bool) *ToggleButton
func (t *ToggleButton) OnChanged(fn func(bool)) *ToggleButton
```
Button visuals (normative): fill = ControlFill/Hover/Pressed (accent: Accent/AccentHover/AccentPressed); stroke ControlStroke width StrokeWidth (accent: none); radius ControlCornerRadius; padding L horizontal / M vertical; label TextPrimary (accent: AccentText; disabled: TextDisabled). Focus ring per global constraint. Keyboard: Space or Enter KeyDown when focused → Activate. ToggleButton: checked state renders like accent-on (Accent fill) regardless of accent flag; click toggles then fires OnChanged.

- [ ] TDD headless: click fires via router press+release-inside; release-outside doesn't; disabled ignores; Space/Enter activate; toggle toggles + callback; hover/pressed observable. Golden `buttons.png` (light): 3 buttons side by side — default, accent, disabled.
- [ ] Commit: `feat(controls): ClickBehavior, Button, ToggleButton`

### Task 4: CheckBox, RadioButton, ToggleSwitch

**Files:** `controls/checkbox.go`, `controls/toggleswitch.go`, tests, golden `toggles.png`

**Produces:** `NewCheckBox(face, label)`, `NewRadioButton(face, label)`, `NewRadioGroup()` (+`group.Add(rb)` — checking one unchecks others, group OnChanged(index)), `NewToggleSwitch()` — all with Checked/SetChecked/OnChanged/SetEnabled per ToggleButton's pattern, ClickBehavior-driven, Space toggles when focused (Enter too).
Visuals (normative): CheckBox 18×18 box radius 4: unchecked = ControlFill fill + ControlStroke; checked = Accent fill + checkmark. Checkmark: try goregular '✓' (U+2713) via the face — IF the font lacks the glyph (glyphIndex miss), fallback = white inner rounded square inset 5. The implementer MUST report which path shipped. RadioButton 18×18 radius 9 (circle); checked = Accent ring (stroke width 5... normative: outer stroke Accent width 2 + inner Accent circle radius 4.5 centered — approximated as a filled rounded-rect radius 4.5 of size 9×9). ToggleSwitch 40×20 pill (radius 10): off = ControlFill + ControlStroke + TextSecondary thumb circle 12 at left inset 4; on = Accent fill + AccentText thumb at right. Labels sit right of the glyph with PaddingM gap (StackPanel-free: composite MeasureContent/Render directly).

- [ ] TDD headless per control + radio-group exclusivity + golden `toggles.png` (light): row of checkbox unchecked/checked, radio off/on, switch off/on.
- [ ] Commit: `feat(controls): CheckBox, RadioButton, ToggleSwitch`

### Task 5: TextBox — model + rendering

**Files:** `controls/textbox.go`, tests, golden `textbox.png`

**Produces (exact):**
```go
func NewTextBox(face *text.Face) *TextBox
func (t *TextBox) Text() string
func (t *TextBox) SetText(s string) *TextBox        // resets caret to end, clears selection
func (t *TextBox) SetPlaceholder(s string) *TextBox // TextDisabled color when empty+unfocused
func (t *TextBox) SetEnabled(v bool) *TextBox
func (t *TextBox) OnChanged(fn func(string)) *TextBox
func (t *TextBox) SetTimers(q *timers.Queue) *TextBox // enables caret blink (530ms); nil = solid caret
func (t *TextBox) Caret() int                        // rune index
func (t *TextBox) Selection() (start, end int)       // rune indices, start<=end; ==caret,caret when empty
func (t *TextBox) SetCaret(i int) *TextBox           // clamped; clears selection
func (t *TextBox) Select(anchor, caret int) *TextBox
```
Rendering (normative): fixed height = face.LineHeight() + 2*PaddingM; fill ControlFill (disabled: ControlFillDisabled), stroke ControlStroke (focused: FocusStroke... WinUI uses bottom accent bar — v0: focused stroke = FocusStroke width StrokeWidth, plus the standard focus ring), radius ControlCornerRadius. Text PaddingM-inset, vertically centered, TextPrimary. Selection: SelectionBackground rect(s) behind the selected rune range (prefix-width math: xOf(i) = face.Measure(runes[:i]).W). Caret: 1px-wide... normative 1.5px FillRect, Accent color, at xOf(caret) − hscroll, visible when focused && (no timers || blink phase on). Horizontal scroll: hscroll clamped so caret always visible within the inner width; text clipped via ClipProvider (own bounds). Measure: desired = {explicit-or-160 default width... normative: content desired W = 160 (a sane default; explicit SetWidth overrides via Element), H = line height + 2*PaddingM}.

- [ ] TDD headless: caret/selection accessors + clamping; xOf math against face.Measure; hscroll keeps caret visible (set narrow width, caret at end ⇒ offset > 0); placeholder state logic; SetText resets. Golden `textbox.png` (light): 200×40 focused TextBox containing "Hello fluo" with runes 2..7 selected, caret at 7 (blink-off path: no timers wired ⇒ solid caret), selection highlight visible.
- [ ] Commit: `feat(controls): TextBox model and rendering`

### Task 6: TextBox — interaction

**Files:** `controls/textbox.go` (extend), tests

Normative keyboard map (KeyDown when focused; all editing ops clear-or-use selection per convention: typing/Delete/Backspace with active selection first delete the selection):
- Rune input (e.Rune != 0, no Ctrl): insert at caret.
- Backspace/Delete: delete selection, else rune before/after caret.
- Left/Right: move caret (Shift: extend selection from anchor); Ctrl+Left/Right NOT in v0.
- Home/End: caret to 0/len (Shift extends).
- Ctrl+A: select all. Ctrl+C: copy selection (no-op if none/nil clipboard). Ctrl+X: copy+delete. Ctrl+V: paste replacing selection (strip \n\r: single-line).
- All handled events set Handled; OnChanged fires on every text mutation.
Mouse: Press sets caret to nearest rune boundary by x (use xOf midpoints), captures; Move-captured extends selection; Release releases. Cursor: CursorIBeam (CursorShaper).

- [ ] TDD headless with a fake clipboard on a real Router: typing builds text; selection editing semantics (each bullet above gets an assertion); click-to-caret math (click at midpoint boundaries); drag-select; copy/cut/paste roundtrip; disabled ignores keys.
- [ ] Commit: `feat(controls): TextBox editing, selection, clipboard`

### Task 7: Slider + ProgressBar

**Files:** `controls/slider.go`, `controls/progressbar.go`, tests, golden `slider_progress.png`

**Produces:** `NewSlider()` — Min/Max/Value float32 (SetRange, SetValue clamped, Value(), OnChanged(func(float32))), horizontal; track height 4 rounded (ControlFill + ControlStroke hairline), filled portion Accent, thumb: 16×16 circle (radius 8) Accent (hover: AccentHover; disabled: AccentDisabled), drag via capture (value from pointer x proportion), click-on-track jumps, Left/Right arrows ±(Max−Min)/100 (Shift: /10) when focused, focus ring. Desired {160, 24}. `NewProgressBar()` — SetValue 0..1 clamped, Value(); track 4 rounded ControlFill, fill Accent; NOT focusable, no input. Desired {160, 8}.

- [ ] TDD headless: value clamping; drag math (press at 75% of track ⇒ value ≈ Min+0.75*range); arrows; OnChanged; progress clamp. Golden (light): slider at 0.6 + progress at 0.3 stacked.
- [ ] Commit: `feat(controls): Slider, ProgressBar`

### Task 8: ComboBox + ToolTipArea

**Files:** `controls/combobox.go`, `controls/tooltip.go`, tests, golden `combo_open.png`

**Produces:** `NewComboBox(face)` — `SetItems([]string)`, `SelectedIndex() int` (−1 none), `SetSelectedIndex(i)`, `OnChanged(func(int))`. Field renders like a Button (ControlFill family) showing selected text (or placeholder TextSecondary "Select…") + a "v" glyph TextSecondary right-aligned (literal 'v' character in the face — cheap chevron v0). Click (or Space/Enter/Down when focused) opens popup via OverlayHostFor: popup = Card-background Border radius CornerRadius + shadow? (DrawShadow token Shadow, blur ShadowBlur — popups are the first shadow consumer) containing a vertical StackPanel of item rows (TextBlock in a hover-highlighting row widget — row: ControlFillHover on hover, SelectionBackground when == selected). Item click: select, OnChanged, close. Esc closes. No open ⇒ no popup leak: closing via light-dismiss fires onDismiss which resets the combo's open flag. `NewToolTipArea(child core.Widget, face *text.Face, tip string)` — wrapper widget (Border-like, transparent, no visuals) implementing PointerHandler: Enter starts 600ms timer when a timers.Queue is wired via SetTimers, else shows immediately; Leave cancels/hides. Tip popup: small Card Border + Caption TextBlock, anchored at the child's bounds, closed on Leave (not light-dismiss-dependent).

- [ ] TDD headless: combo opens popup in host (PopupCount 1), item click selects+closes+fires, Esc closes, light-dismiss resets open flag (second click reopens), tooltip enter/leave with fake timers (Advance past 600ms shows; Leave before doesn't). Golden `combo_open.png` (light, 220×160): open combo with 3 items, second selected-highlighted, shadow visible.
- [ ] Commit: `feat(controls): ComboBox, ToolTipArea`

### Task 9: gallery controls page + docs

**Files:** `cmd/fluo-gallery/main.go`, `README.md`, `ROADMAP.md`

- Root becomes OverlayHost (SetRouter wired) containing the existing chrome; content scroll column gains a "Controls" section above the swatches: real Button (replaces the Phase-4 demo button, keeping the counter), accent Button, ToggleButton, CheckBox, two RadioButtons in a group, ToggleSwitch, TextBox (SetTimers(ctx.Timers) — first live blink) with placeholder "Type here…", Slider wired to a ProgressBar (slider OnChanged drives progress SetValue), ComboBox with the 6 swatch color names (+ToolTipArea on the accent button: "Accent button"). Remove the Phase-4 gallery-local button type.
- Live verify: launch, winshot `fluo-gallery-p5.png`, READ: controls page visible and correct (both themes NOT required this time — dark only). INTERACTION check via winshot after driving the mouse: use Windows-side python `ctypes` SetCursorPos+mouse_event to click the TextBox and type? — out of scope for the subagent; static capture suffices (headless tests cover interaction). Kill+confirm.
- Docs: ROADMAP tick ALL Phase 5 boxes; README: controls list + clipboard/timers wiring notes.
- [ ] Commit: `feat(gallery): controls page; complete Phase 5`

---

## Self-review notes

- ROADMAP Phase 5 coverage: overlay/popup→2, Button family→3-4, TextBox→5-6, Slider/ProgressBar→7, ComboBox→8, ToolTip→8, gallery→9. Clipboard (ROADMAP "Clipboard cut/copy/paste")→1+6. Focus-visual/disabled constraints are global.
- Ledger prep notes consumed: Clickable helper (Task 3), Key consts (Task 1), Router.Detach (Task 1, used by OverlayHost in 2), scroll-chaining still deferred (no new nesting introduced).
- Type consistency: ClickBehavior methods used identically in Tasks 3-4-7-8; OverlayHostFor + SetRouter contract fixed in Task 2 before consumers in 8-9; TextBox SetTimers pattern shared with ToolTipArea.
