# controls — value-input controls

This page covers fluo's four value-input controls: `TextBox` (free-form
text), `Slider` (continuous numeric value over a range), `ProgressBar`
(non-interactive progress display), and `ComboBox` (single selection from a
list, via a popup). All four are styled from `theme.Active()` captured at
construction time (see [theming](theming.md)), and all four follow the
package's uniform contract: **programmatic setters are silent — they never
fire `OnChanged` — while `OnChanged` fires only for user-driven changes**
(typing, dragging, clicking, or the relevant keyboard interaction). Reach for
these when you need the user to supply or observe a scalar value rather than
toggle a boolean (`CheckBox`/`ToggleSwitch`) or trigger an action (`Button`).

**Import:** `github.com/0xdreadnaught/fluo/controls`

## Contents
- [TextBox](#textbox)
- [Slider](#slider)
- [ProgressBar](#progressbar)
- [ComboBox](#combobox)

---

## TextBox

`TextBox` is a focusable, token-styled text input — single-line by default,
multi-line via `SetMultiline`. The data model (text/caret/selection,
rune-indexed) and rendering (chrome, selection highlight, caret, scrolling,
placeholder) are paired with a full interaction layer: `OnKey` implements the
normative keyboard map (rune insertion, Backspace/Delete, caret movement with
Shift-extend, word-wise motion, Ctrl+A/C/X/V) and `OnPointer` implements
click-to-caret, drag-to-select, and double/triple-click word and line
selection. `TextBox` implements `input.Focusable` and `input.FocusHandler`
since focus drives both the focus ring and caret visibility, and
`input.CompositionHandler`/`input.CaretRector` for IME input (inline preedit
plus candidate-window anchoring).

Multi-line mode is additive: every flag below defaults to off, and a
default-constructed `TextBox` behaves exactly like the original single-line
control. Four opt-in flags build on it:

| Flag | Effect |
|---|---|
| [SetMultiline](#textboxsetmultiline) | Enter inserts `\n`; Up/Down/PageUp/PageDown move between lines; paste keeps newlines; a vertical scrollbar appears as needed. |
| [SetWordWrap](#textboxsetwordwrap) | Soft-wraps long lines at the content width for display only — no `\n` is ever inserted into the buffer. Multi-line only. |
| [SetLineNumbers](#textboxsetlinenumbers) | Reserves a right-aligned line-number gutter along the left inner edge. Multi-line only. |
| [SetTabInserts](#textboxsettabinserts) | Tab indents instead of moving focus, including block indent/outdent over a multi-line selection. Multi-line only. |

Single-line boxes gain [OnSubmit](#textboxonsubmit), fired when the user
presses Enter.

All rune-index parameters and returns (`Caret`, `Selection`, `SetCaret`,
`Select`) are **rune indices** into `Text()`, not byte offsets — text is
stored as `[]rune` internally so multibyte characters never split a
codepoint.

**Constructor**

```go
func NewTextBox(face *text.Face) *TextBox
```

Returns an enabled, empty, unfocused `TextBox` drawing text with `face`
(`face` may be `nil`, in which case it measures/draws no text — a degenerate
but valid state, matching `TextBlock`'s nil-face convention). Colors and
metrics are captured from `theme.Active()` at construction.

**Example**

```go
box := controls.NewTextBox(face)
box.SetPlaceholder("Search…")
box.OnChanged(func(s string) {
    fmt.Println("query:", s)
})
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [Text](#textboxtext) | `Text() string` | Returns the current text content. |
| [SetText](#textboxsettext) | `SetText(s string) *TextBox` | Replaces the text, resets the caret to the end, clears selection. |
| [SetPlaceholder](#textboxsetplaceholder) | `SetPlaceholder(s string) *TextBox` | Sets the placeholder shown whenever `Text() == ""`. |
| [SetEnabled](#textboxsetenabled) | `SetEnabled(v bool) *TextBox` | Toggles whether the box accepts focus and editing input. |
| [SetMultiline](#textboxsetmultiline) | `SetMultiline(v bool) *TextBox` | Toggles multi-line mode (off by default). |
| [Multiline](#textboxsetmultiline) | `Multiline() bool` | Reports whether multi-line mode is enabled. |
| [SetWordWrap](#textboxsetwordwrap) | `SetWordWrap(v bool) *TextBox` | Toggles display-time soft word-wrap. Multi-line only. |
| [WordWrap](#textboxsetwordwrap) | `WordWrap() bool` | Reports whether word-wrap is enabled. |
| [SetLineNumbers](#textboxsetlinenumbers) | `SetLineNumbers(v bool) *TextBox` | Toggles the line-number gutter. Multi-line only. |
| [LineNumbers](#textboxsetlinenumbers) | `LineNumbers() bool` | Reports whether the gutter is enabled. |
| [SetTabInserts](#textboxsettabinserts) | `SetTabInserts(v bool) *TextBox` | Toggles Tab-inserts-indentation. Multi-line only. |
| [TabInserts](#textboxsettabinserts) | `TabInserts() bool` | Reports whether Tab-inserts-indentation is enabled. |
| [OnChanged](#textboxonchanged) | `OnChanged(fn func(string)) *TextBox` | Sets the callback fired with the new text on every user edit. |
| [OnSubmit](#textboxonsubmit) | `OnSubmit(fn func(string)) *TextBox` | Sets the callback fired when the user presses Enter in single-line mode. |
| [SetTimers](#textboxsettimers) | `SetTimers(q *timers.Queue) *TextBox` | Wires a `timers.Queue` to drive caret blinking. |
| [Caret](#textboxcaret) | `Caret() int` | Returns the current caret rune index. |
| [Selection](#textboxselection) | `Selection() (start, end int)` | Returns the selected rune range, normalized so `start<=end`. |
| [SetCaret](#textboxsetcaret) | `SetCaret(i int) *TextBox` | Moves the caret to rune index `i`, clearing any selection. |
| [Select](#textboxselect) | `Select(anchor, caret int) *TextBox` | Sets the selection to `[anchor, caret)`. |
| [AcceptsFocus](#textboxacceptsfocus) | `AcceptsFocus() bool` | Implements `input.Focusable`; `false` while disabled. |
| [OnFocusChanged](#textboxonfocuschanged) | `OnFocusChanged(focused bool)` | Implements `input.FocusHandler`; tracks focus state. |
| [MeasureContent](#textboxmeasurecontent) | `MeasureContent(available render.Size) render.Size` | Reports the content size (taller in multi-line mode). |
| [ArrangeContent](#textboxarrangecontent) | `ArrangeContent(bounds render.Rect)` | Re-clamps the scroll offsets for the arranged size. |
| [ClipRect](#textboxcliprect) | `ClipRect() (render.Rect, bool)` | Implements `core.ClipProvider`; clips to the box's own bounds. |
| [Render](#textboxrender) | `Render(r render.Renderer)` | Paints the sunken well, selection, text, gutter, and caret. |
| [RenderOverlay](#textboxrenderoverlay) | `RenderOverlay(r render.Renderer)` | Draws the vertical scroll thumb when one is shown. |
| [OnKey](#textboxonkey) | `OnKey(e *input.KeyEvent)` | Implements `input.KeyHandler`, the normative keyboard map. |
| [OnPointer](#textboxonpointer) | `OnPointer(e *input.PointerEvent)` | Implements `input.PointerHandler`: click-to-caret, drag-to-select, multi-click. |
| [OnComposition](#textboxoncomposition) | `OnComposition(e input.CompositionEvent)` | Implements `input.CompositionHandler`: inline IME preedit and commit. |
| [CaretScreenRect](#textboxcaretscreenrect) | `CaretScreenRect() (render.Rect, bool)` | Implements `input.CaretRector`, so a host can anchor the OS IME window at the caret. |
| [Cursor](#textboxcursor) | `Cursor() input.Cursor` | Implements `input.CursorShaper`; always an I-beam. |

#### TextBox.Text

Returns the current text content.

**Syntax**

```go
func (t *TextBox) Text() string
```

**Returns** — `string`, the full text (built from the internal `[]rune`).

**Example**

```go
query := box.Text()
```

**See also** — [SetText](#textboxsettext)

#### TextBox.SetText

Replaces the text content, resets the caret to the end, and clears any
selection.

**Syntax**

```go
func (t *TextBox) SetText(s string) *TextBox
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `s` | `string` | The new text content. |

**Returns** — `*TextBox` for chaining.

**Example**

```go
box.SetText("hello world")
```

**Notes** — A complete no-op (no invalidation, caret/selection untouched)
when `s` already equals the current text — safe for an `OnChanged` handler to
call `SetText` with the value it was just notified of without triggering
pointless re-layout. `SetText` is **silent**: it never fires `OnChanged`
regardless of whether the text actually changes, per the package's uniform
setter convention (`OnChanged` reports only user-driven changes — typing,
Backspace/Delete, Ctrl+X, Ctrl+V). When the text does change, it also
restarts the caret blink phase.

**See also** — [Text](#textboxtext), [OnChanged](#textboxonchanged)

#### TextBox.SetPlaceholder

Sets the text shown (in `GrayText`) whenever `Text() == ""`, regardless of
focus state.

**Syntax**

```go
func (t *TextBox) SetPlaceholder(s string) *TextBox
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `s` | `string` | The placeholder string. |

**Returns** — `*TextBox` for chaining.

**Example**

```go
box.SetPlaceholder("Enter your name")
```

**Notes** — Simpler and more common than hiding the placeholder while
focused; this is the normative choice for the control.

#### TextBox.SetEnabled

Toggles whether the box accepts focus and pointer/keyboard editing.

**Syntax**

```go
func (t *TextBox) SetEnabled(v bool) *TextBox
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `bool` | `true` to enable (the default); `false` to disable. |

**Returns** — `*TextBox` for chaining.

**Example**

```go
box.SetEnabled(false)
```

**Notes** — `OnKey` and `OnPointer` both ignore all input while disabled.
Purely visual/behavioral: no invalidation needed.

#### TextBox.SetMultiline

Toggles multi-line mode; `false` (the default) is the single-line behavior.

**Syntax**

```go
func (t *TextBox) SetMultiline(v bool) *TextBox
func (t *TextBox) Multiline() bool
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `bool` | `true` for multi-line, `false` (the default) for single-line. |

**Returns** — `*TextBox` for chaining.

**Example**

```go
editor := controls.NewTextBox(face).
    SetMultiline(true).
    SetWordWrap(true).
    SetLineNumbers(true)
```

**Notes** — Enabling multi-line mode changes:

- Enter inserts a `\n` at the caret instead of being ignored (and never
  fires [OnSubmit](#textboxonsubmit)).
- Up/Down move the caret between lines, preserving a desired column across
  lines shorter than it; PageUp/PageDown move by a visible page. Home/End
  operate on the caret's **current** line rather than the whole text — use
  Ctrl+Home/Ctrl+End for the buffer's true bounds.
- Ctrl+V keeps newlines (normalized to `\n`) instead of stripping them.
- `MeasureContent` reports a taller default, and a vertical scrollbar is
  drawn and driven alongside the existing horizontal one.

Text still hard-wraps only at an explicit `\n` unless
[SetWordWrap](#textboxsetwordwrap) is also on; a line wider than the box
scrolls horizontally, keyed off the caret's own line. Unlike most setters
here, this one calls `InvalidateMeasure`, since it changes
`MeasureContent`'s answer.

**See also** — [SetWordWrap](#textboxsetwordwrap), [SetLineNumbers](#textboxsetlinenumbers), [SetTabInserts](#textboxsettabinserts)

#### TextBox.SetWordWrap

Toggles opt-in soft word-wrap; `false` (the default) is
[SetMultiline](#textboxsetmultiline)'s hard-wrap-and-scroll behavior.

**Syntax**

```go
func (t *TextBox) SetWordWrap(v bool) *TextBox
func (t *TextBox) WordWrap() bool
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `bool` | `true` to soft-wrap at the content width; `false` (the default) to hard-wrap at `\n` only. |

**Returns** — `*TextBox` for chaining.

**Notes** — Only meaningful when `Multiline()` is also true; a harmless
no-op flag otherwise. **No `\n` is ever inserted into the buffer** — wrapping
is a purely display-time re-flow of each real (`\n`-delimited) line into one
or more visual rows at the box's current content width, so `Text()` and
`OnChanged` are unaffected. Horizontal scrolling is disabled while wrap is
on (wrapped rows never exceed the content width by construction), and
vertical scrolling counts visual rows rather than logical lines. Calls
`InvalidateMeasure`.

**See also** — [SetMultiline](#textboxsetmultiline)

#### TextBox.SetLineNumbers

Toggles the opt-in line-number gutter.

**Syntax**

```go
func (t *TextBox) SetLineNumbers(v bool) *TextBox
func (t *TextBox) LineNumbers() bool
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `bool` | `true` to reserve and draw the gutter; `false` is the default. |

**Returns** — `*TextBox` for chaining.

**Notes** — Multi-line only; a single-line box ignores the flag entirely. A
fixed-width column is reserved along the **left** inner edge, wide enough
for the widest line number the current text can produce (measured with the
box's own `Face`), and the text content is narrowed and shifted right by
exactly that width — the mirror image of the vertical scroll thumb's gutter
on the right. Numbers are drawn right-aligned in `GrayText`, one per
**logical** line, scrolled in sync with the text, with a thin vertical rule
separating gutter from text. While word-wrap is on, only a logical line's
first visual row carries its number; continuation rows are left blank, so
the numbers keep counting real lines rather than displayed rows. Calls
`InvalidateMeasure`.

**See also** — [SetMultiline](#textboxsetmultiline), [SetWordWrap](#textboxsetwordwrap)

#### TextBox.SetTabInserts

Toggles whether a focused Tab press inserts indentation instead of bubbling
to the router's focus navigation.

**Syntax**

```go
func (t *TextBox) SetTabInserts(v bool) *TextBox
func (t *TextBox) TabInserts() bool
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `bool` | `true` to indent on Tab; `false` (the default) leaves Tab to focus navigation. |

**Returns** — `*TextBox` for chaining.

**Notes** — Multi-line only: a single-line box always leaves Tab as
focus-nav regardless of this flag. When enabled:

| Keys | Effect |
|---|---|
| Tab | Inserts spaces at the caret, through the same path plain typing uses. |
| Shift+Tab | Removes up to one indent's worth of leading spaces from the caret's current line. |
| Tab / Shift+Tab with a selection spanning **more than one** logical line | Indents/outdents every touched line in place, restoring the selection over the same block afterward, so repeated presses keep indenting. |

No literal `\t` is ever inserted. Every recognized combination is marked
handled even when the edit is a no-op, so Tab never leaks through to
focus-nav while this flag is on.

**See also** — [SetMultiline](#textboxsetmultiline), [OnKey](#textboxonkey)

#### TextBox.OnChanged

Sets the callback fired with the new text whenever the **user** changes it.

**Syntax**

```go
func (t *TextBox) OnChanged(fn func(string)) *TextBox
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `fn` | `func(string)` | Called with the full new text after every user edit. `nil` is a valid, silent no-op. |

**Returns** — `*TextBox` for chaining.

**Example**

```go
box.OnChanged(func(s string) {
    fmt.Println("changed:", s)
})
```

**Notes** — Fires for every editing mutation — typing, Backspace/Delete,
Ctrl+X, Ctrl+V — but never for a programmatic `SetText` (the package's
uniform setter convention: programmatic setters are silent, `OnChanged`
reports only user-driven changes). Replaces any previously set callback.

**See also** — [SetText](#textboxsettext), [OnSubmit](#textboxonsubmit)

#### TextBox.OnSubmit

Sets the callback fired with the current text when the user presses Enter in
**single-line** mode.

**Syntax**

```go
func (t *TextBox) OnSubmit(fn func(string)) *TextBox
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `fn` | `func(string)` | Called with the full current text on Enter. `nil` is a valid, silent no-op. |

**Returns** — `*TextBox` for chaining.

**Example**

```go
search := controls.NewTextBox(face)
search.SetPlaceholder("Search…")
search.OnSubmit(func(q string) {
    runSearch(q)
})
```

**Notes** — Multi-line mode is unaffected: Enter there always inserts a
`\n` and never calls this callback, whether or not one is set. Like
`OnChanged`, it fires only from a user keypress, never from a programmatic
`SetText`. With no callback set, Enter is simply left unhandled in
single-line mode. Replaces any previously set callback.

**See also** — [OnChanged](#textboxonchanged), [SetMultiline](#textboxsetmultiline)

#### TextBox.SetTimers

Wires `q` as the caret-blink driver.

**Syntax**

```go
func (t *TextBox) SetTimers(q *timers.Queue) *TextBox
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `q` | `*timers.Queue` | The timer queue to schedule a repeating blink callback on. `nil` detaches any previously wired queue. |

**Returns** — `*TextBox` for chaining.

**Example**

```go
box.SetTimers(app.Timers())
```

**Notes** — Schedules a repeating callback every 530ms that flips caret
visibility. Passing `nil` reverts to a solid (always-visible-while-focused)
caret. Calling `SetTimers` again always stops the previously scheduled timer
first, so a superseded queue can never keep toggling this box's caret after
the fact.

#### TextBox.Caret

Returns the current caret rune index.

**Syntax**

```go
func (t *TextBox) Caret() int
```

**Returns** — `int` in `[0, len(runes)]` — the raw caret position, which may
be either endpoint of the current selection after `Select`.

**Example**

```go
pos := box.Caret()
```

**See also** — [SetCaret](#textboxsetcaret), [Selection](#textboxselection)

#### TextBox.Selection

Returns the selected rune range, normalized so `start<=end`.

**Syntax**

```go
func (t *TextBox) Selection() (start, end int)
```

**Returns** — `(start, end int)`, the normalized selection range. Returns
`(caret, caret)` when there is no selection.

**Example**

```go
start, end := box.Selection()
```

**See also** — [Select](#textboxselect), [Caret](#textboxcaret)

#### TextBox.SetCaret

Moves the caret to rune index `i` and clears any selection.

**Syntax**

```go
func (t *TextBox) SetCaret(i int) *TextBox
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `i` | `int` | Target rune index, clamped to `[0, len(runes)]`. |

**Returns** — `*TextBox` for chaining.

**Example**

```go
box.SetCaret(0) // move to the start
```

**Notes** — Anchor becomes equal to the new caret (no selection). Restarts
the caret blink phase so the caret is immediately visible at its new
position.

**See also** — [Select](#textboxselect)

#### TextBox.Select

Sets the selection to `[anchor, caret)`.

**Syntax**

```go
func (t *TextBox) Select(anchor, caret int) *TextBox
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `anchor` | `int` | The fixed end of the selection, clamped to `[0, len(runes)]`. |
| `caret` | `int` | The moving end of the selection (the actual caret afterward), clamped to `[0, len(runes)]`. |

**Returns** — `*TextBox` for chaining.

**Example**

```go
box.Select(0, box.Caret()) // select from start to current caret
```

**Notes** — `anchor` may be greater than `caret`; `Selection()` always
normalizes the pair, but `Caret()` reports the raw `caret` argument
afterward — the actual caret position, not necessarily the normalized range
start. Restarts the caret blink phase.

**See also** — [Selection](#textboxselection), [SetCaret](#textboxsetcaret)

#### TextBox.AcceptsFocus

Implements `input.Focusable`.

**Syntax**

```go
func (t *TextBox) AcceptsFocus() bool
```

**Returns** — `bool` — `false` whenever the box is disabled; otherwise `true`.

#### TextBox.OnFocusChanged

Implements `input.FocusHandler`, tracking focus for the focused border color,
the focus-ring overlay, and caret visibility.

**Syntax**

```go
func (t *TextBox) OnFocusChanged(focused bool)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `focused` | `bool` | The box's new focus state. |

**Notes** — Gaining focus restarts the blink phase so the caret starts out
solidly visible. Losing focus stops the blink timer outright.

#### TextBox.MeasureContent

Reports the fixed content size.

**Syntax**

```go
func (t *TextBox) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | The space available to the box. Never consulted — the desired size is fixed. |

**Returns** — `render.Size` — width `160` (an internal default; an explicit
`SetWidth` overrides this through `core.MeasureWidget`'s normal explicit-size
precedence) by `lineHeight()+2*PaddingM`.

#### TextBox.ArrangeContent

Re-clamps the horizontal scroll offset for the newly arranged width — the
single source of truth for keeping the caret visible within the box.

**Syntax**

```go
func (t *TextBox) ArrangeContent(bounds render.Rect)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `bounds` | `render.Rect` | The box's newly arranged bounds. |

**Notes** — Recomputes the padding-inset inner width from `bounds` and
clamps the scroll offset so the caret's display position stays within it,
never scrolling past the point where the end of the text would leave a gap
on the right.

#### TextBox.ClipRect

Implements `core.ClipProvider`.

**Syntax**

```go
func (t *TextBox) ClipRect() (render.Rect, bool)
```

**Returns** — `(render.Rect, bool)` — the box's own full bounds (chrome
included) and `true`, always. The same rect `Render` itself clips
text/selection/caret drawing to.

#### TextBox.Render

Paints the classic sunken input well, then the selection highlight, main
text run (or placeholder), and caret.

**Syntax**

```go
func (t *TextBox) Render(r render.Renderer)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | The renderer to draw into. |

**Notes** — The well fills `WindowWell` (`ButtonFace` while disabled, the
classic "grayed-out field" look). Content is clipped to `ClipRect`. In
multi-line mode this also draws each line (or, while wrapping, each visual
row) and the line-number gutter when one is enabled. `TextBox` draws no
separate focus ring — `PaddingM` already clears the 2px sunken bevel, and
the caret plus sunken well already read as "this is the focused field."

**See also** — [RenderOverlay](#textboxrenderoverlay)

#### TextBox.RenderOverlay

Draws the vertical scroll thumb when one is shown.

**Syntax**

```go
func (t *TextBox) RenderOverlay(r render.Renderer)
```

**Notes** — Paints nothing at all in single-line mode, or whenever the
content fits — the vertical scrollbar only exists once multi-line content
overflows the box. No focus ring is drawn here: classic Windows textboxes
have none, unlike every other focusable control in this package.

#### TextBox.OnKey

Implements `input.KeyHandler`, the normative keyboard map.

**Syntax**

```go
func (t *TextBox) OnKey(e *input.KeyEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.KeyEvent` | The key event; `e.Handled` is set `true` for every recognized combination below. |

**Notes** — Ignored entirely (no mutation, `Handled` left `false`) while
disabled or unfocused, and for anything but `Action==Press` — held-key
auto-repeat arrives as repeated Press events from the host, not from
`TextBox` itself. Recognized combinations:

| Keys | Effect |
|---|---|
| Ctrl+A | Select all (`Select(0, len(runes))`). |
| Ctrl+C | Copy the selection to the clipboard (no-op if none, or no clipboard wired). |
| Ctrl+X | Cut: copy then delete the selection (same no-op conditions as Ctrl+C). |
| Ctrl+V | Paste the clipboard text at the caret/over the selection. Newlines are stripped in single-line mode and normalized to `\n` in multi-line mode. |
| Backspace | Delete the selection if one is active, else the rune before the caret. |
| Delete | Delete the selection if one is active, else the rune after the caret. |
| Ctrl+Backspace / Ctrl+Delete | Delete the previous/next **word** — or the active selection, same selection-first rule as plain Backspace/Delete. |
| Left / Right | Move the caret by one rune; with Shift held, extends the selection from the current anchor instead. With no Shift and an active selection, collapses to the selection's near edge instead of stepping past it (desktop-standard convention). |
| Ctrl+Left / Ctrl+Right | Move the caret by whole **words**, crossing line boundaries (a newline counts as whitespace). |
| Home / End | Move the caret to the start/end of the text — or, in multi-line mode, of the caret's own line. |
| Ctrl+Home / Ctrl+End | Jump to the start/end of the whole buffer. In single-line mode this is the same target plain Home/End already use; in multi-line mode it is the only way to reach the buffer's true bounds. |
| Enter | Multi-line: inserts a `\n`. Single-line: fires [OnSubmit](#textboxonsubmit) if one is set, else unhandled. |
| Up / Down | **Multi-line only**: move between lines, preserving a desired column across shorter lines. Unhandled in single-line mode. |
| PageUp / PageDown | **Multi-line only**: move by a visible page, with the same desired-column tracking. Unhandled in single-line mode. |
| Tab / Shift+Tab | **Multi-line and [SetTabInserts](#textboxsettabinserts) only**: indent/outdent (block-wise over a multi-line selection). Otherwise unhandled, bubbling to the router's focus navigation. |
| any printable rune (no Ctrl) | Inserts the rune, replacing the current selection if any. |

Every caret-movement combination above also accepts Shift, extending the
selection from the current anchor exactly like its unshifted counterpart.

Every recognized combination sets `e.Handled = true` even when the specific
operation ends up a no-op (e.g. Ctrl+C with no selection) — a focused
`TextBox` owns all of these keys and must not let them bubble further. Keys
listed as unhandled in a given mode fall through exactly as they did before
the corresponding feature existed.

An active IME composition owns keyboard input until it ends; see
[OnComposition](#textboxoncomposition).

**See also** — [OnPointer](#textboxonpointer), [SetMultiline](#textboxsetmultiline)

#### TextBox.OnPointer

Implements `input.PointerHandler`: click-to-caret and drag-to-select.

**Syntax**

```go
func (t *TextBox) OnPointer(e *input.PointerEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.PointerEvent` | The pointer event. |

**Notes** — Ignored entirely while disabled (not handled, so pointer events
bubble past a disabled box) — except a `SetEnabled(false)` landing mid-drag
first releases the router capture this box already holds, so the pointer
isn't permanently wedged with no reachable widget. Press moves the caret to
the nearest rune boundary to the click position (x only in single-line mode,
x **and** y in multi-line mode), clearing any selection, and captures the
pointer so the drag survives leaving the box's bounds. Move, only while this
box holds the capture, extends the selection from the press position to the
new nearest boundary. Release, only while captured, ends the drag.

A press landing inside the vertical scroll thumb starts a **thumb** drag
instead, checked first: Move then scrolls rather than extending the
selection, and Release clears it alongside the capture.

A press arriving as part of a click run (`input.PointerEvent.ClickCount`,
which the router assigns from press timing and distance) selects rather than
merely placing the caret:

| ClickCount | Effect |
|---|---|
| 1 | Plain caret placement. Also the value on every press when the host installed no time source, which is what keeps this additive. |
| 2 | Selects the word under the pointer, falling back to caret placement when the pointer sits in whitespace with no word either side. |
| 3 or more | Selects the whole **logical** line, stopping short of the terminating `\n`. While wrapping, this spans every visual row of that line rather than just the one clicked. The run keeps counting up past 3 rather than wrapping, so a fourth same-spot press holds the line selection. |

Dragging on from a double- or triple-click extends per-rune from the new
selection's start, not at word or line granularity — the selection stays
coherent, it just loses the wider granularity once the drag begins.

**See also** — [OnKey](#textboxonkey)

#### TextBox.OnComposition

Implements `input.CompositionHandler`: inline IME preedit and commit.

**Syntax**

```go
func (t *TextBox) OnComposition(e input.CompositionEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `input.CompositionEvent` | The composition update, commit, or cancellation. |

**Notes** — While `e.Active`, the preedit string is stored and spliced in at
the caret for rendering only — the committed buffer (`Text()`) is left
untouched for the whole composition, so a still-in-progress candidate never
corrupts what `OnChanged` has already reported. When the composition ends,
the preedit is cleared, and (unless `e.Canceled`) `e.Committed` is inserted
at the caret through the same path every other user edit uses, firing
`OnChanged` exactly as typing would. A canceled composition, or one ending
with an empty commit, mutates nothing. Ignored while disabled or unfocused,
matching `OnKey`'s own guard.

**See also** — [CaretScreenRect](#textboxcaretscreenrect), [OnKey](#textboxonkey)

#### TextBox.CaretScreenRect

Implements `input.CaretRector`, so a host can anchor platform UI at the
caret.

**Syntax**

```go
func (t *TextBox) CaretScreenRect() (render.Rect, bool)
```

**Returns** — the caret's rectangle in window logical coordinates (the same
space `PointerEvent.Pos` and widget bounds use), and `false` when there is
no caret to report — e.g. while unfocused.

**Notes** — `app.Run` polls this each frame via
`input.Router.FocusedCaretRect` and forwards the result to the OS IME
anchor, so a CJK candidate window opens **at** the caret rather than in a
default screen corner. On platforms with no OS-level IME window to anchor,
that step is a no-op.

**See also** — [OnComposition](#textboxoncomposition)

#### TextBox.Cursor

Implements `input.CursorShaper`.

**Syntax**

```go
func (t *TextBox) Cursor() input.Cursor
```

**Returns** — `input.Cursor` — always `input.CursorIBeam`, independent of
enabled/focused state.

---

## Slider

`Slider` is a focusable, token-styled continuous value picker over `[Min,
Max]` (defaulting to `[0, 1]`). `Value` is reported and set as a plain
`float32`; `SetRange`/`SetValue` both clamp into the current range.

Orientation defaults to `Horizontal` and generalizes the slider's geometry
onto whichever axis is the MAIN axis (X for Horizontal, Y for Vertical).
Horizontal: the filled portion of the track runs from the track's left edge
to the thumb's center; Min is at the left, Max at the right. **Vertical: Max
is at the TOP, Min is at the bottom** (the reverse of a naive top-to-bottom
mapping) — a thumb centered at the track's top edge is `Value==Max`, one
centered at the bottom edge is `Value==Min`; the fill mirrors horizontal's
"fill the Min side," running from the thumb's center down to the track's
bottom edge.

**Constructor**

```go
func NewSlider() *Slider
```

Returns an enabled, `Horizontal` `Slider` ranging over `[0, 1]` with `Value`
`0`.

**Example**

```go
vol := controls.NewSlider()
vol.SetRange(0, 100)
vol.SetValue(50)
vol.OnChanged(func(v float32) {
    fmt.Println("volume:", v)
})
```

**Notes** — The usable track span is inset by the thumb's radius (half the
16px thumb) at each end: the thumb's center is only ever placed within
`[thumbRadius, mainAxisLength-thumbRadius]`, never at the raw bounds edge, so
a thumb centered at that inset extreme sits flush against the track edge
without its own edge overhanging the track. The value-mapping (position ↔
`Value`) divides by `mainAxisLength - 2*thumbRadius` for the same reason — a
press exactly at a rendered thumb's center reproduces its current value
exactly.

### Methods

| Method | Signature | Description |
|---|---|---|
| [SetOrientation](#slidersetorientation) | `SetOrientation(o Orientation) *Slider` | Sets Horizontal (default) or Vertical layout. |
| [Min](#slidermin) | `Min() float32` | Returns the current minimum of the range. |
| [Max](#slidermax) | `Max() float32` | Returns the current maximum of the range. |
| [Value](#slidervalue) | `Value() float32` | Returns the current value. |
| [SetRange](#slidersetrange) | `SetRange(min, max float32) *Slider` | Sets `[min, max]` and re-clamps the current value. |
| [SetValue](#slidersetvalue) | `SetValue(v float32) *Slider` | Sets the value programmatically, clamped into `[Min, Max]`. |
| [OnChanged](#slideronchanged) | `OnChanged(fn func(float32)) *Slider` | Sets the callback fired with the new value on user-driven change. |
| [SetEnabled](#slidersetenabled) | `SetEnabled(v bool) *Slider` | Toggles whether the slider accepts focus and input. |
| [MeasureContent](#slidermeasurecontent) | `MeasureContent(available render.Size) render.Size` | Reports the fixed desired size for the current orientation. |
| [ArrangeContent](#sliderarrangecontent) | `ArrangeContent(bounds render.Rect)` | No-op — `Slider` has no children. |
| [Children](#sliderchildren) | `Children() []core.Widget` | Returns `nil` — `Slider` is a leaf widget. |
| [Render](#sliderrender) | `Render(r render.Renderer)` | Paints the track, fill, and thumb. |
| [RenderOverlay](#sliderrenderoverlay) | `RenderOverlay(r render.Renderer)` | Draws the focus ring while focused. |
| [AcceptsFocus](#slideracceptsfocus) | `AcceptsFocus() bool` | Implements `input.Focusable`; `false` while disabled. |
| [OnFocusChanged](#slideronfocuschanged) | `OnFocusChanged(focused bool)` | Implements `input.FocusHandler`. |
| [OnPointer](#slideronpointer) | `OnPointer(e *input.PointerEvent)` | Click-on-track and drag-to-set. |
| [OnKey](#slideronkey) | `OnKey(e *input.KeyEvent)` | Left/Right nudge the value. |

#### Slider.SetOrientation

Sets the slider's orientation.

**Syntax**

```go
func (s *Slider) SetOrientation(o Orientation) *Slider
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `o` | `Orientation` | `Horizontal` (the default) lays the track left-to-right with Min at the left edge and Max at the right; `Vertical` lays the track top-to-bottom with **Max at the top and Min at the bottom**. |

**Returns** — `*Slider` for chaining.

**Example**

```go
vol.SetOrientation(controls.Vertical)
```

**Notes** — Takes effect on the next Measure/Arrange/Render pass. `Vertical`
also swaps `MeasureContent`'s desired size to tall-narrow.

**See also** — [ProgressBar.SetOrientation](#progressbarsetorientation)

#### Slider.Min

Returns the current minimum of the range.

**Syntax**

```go
func (s *Slider) Min() float32
```

**Returns** — `float32`, the current `Min`.

**See also** — [Max](#slidermax), [SetRange](#slidersetrange)

#### Slider.Max

Returns the current maximum of the range.

**Syntax**

```go
func (s *Slider) Max() float32
```

**Returns** — `float32`, the current `Max`.

**See also** — [Min](#slidermin), [SetRange](#slidersetrange)

#### Slider.Value

Returns the current value.

**Syntax**

```go
func (s *Slider) Value() float32
```

**Returns** — `float32`, always within `[Min, Max]`.

**See also** — [SetValue](#slidersetvalue)

#### Slider.SetRange

Sets the `[min, max]` range and re-clamps the current value into it.

**Syntax**

```go
func (s *Slider) SetRange(min, max float32) *Slider
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `min` | `float32` | The new minimum. |
| `max` | `float32` | The new maximum. Passing `max < min` is not guarded — the underlying clamp primitive resolves that degenerate case by collapsing to `min`. |

**Returns** — `*Slider` for chaining.

**Example**

```go
vol.SetRange(0, 11)
```

**Notes** — **Silent**, matching `SetValue` and the package's uniform setter
convention — never fires `OnChanged`, even when the re-clamp actually moves
the value (e.g. shrinking `Max` below the current `Value`).

**See also** — [SetValue](#slidersetvalue)

#### Slider.SetValue

Sets the value programmatically, clamped into `[Min, Max]`.

**Syntax**

```go
func (s *Slider) SetValue(v float32) *Slider
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `float32` | The new value; clamped into `[Min, Max]`. |

**Returns** — `*Slider` for chaining.

**Example**

```go
vol.SetValue(75)
```

**Notes** — **Silent**: never fires `OnChanged`, even when the clamped
result differs from the current value. Only user-driven paths — drag,
click-on-track, and the arrow-key handler — fire `OnChanged`, and only when
the clamped result actually differs from the value beforehand.

**See also** — [OnChanged](#slideronchanged), [SetRange](#slidersetrange)

#### Slider.OnChanged

Sets the callback fired with the new value whenever the user changes it.

**Syntax**

```go
func (s *Slider) OnChanged(fn func(float32)) *Slider
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `fn` | `func(float32)` | Called with the new value after drag, click-on-track, or an arrow-key nudge. `nil` is a valid, silent no-op. |

**Returns** — `*Slider` for chaining.

**Example**

```go
vol.OnChanged(func(v float32) { applyVolume(v) })
```

**Notes** — Never fires for a programmatic `SetValue` or `SetRange`.
Replaces any previously set callback.

#### Slider.SetEnabled

Toggles whether the slider accepts focus and pointer/keyboard input.

**Syntax**

```go
func (s *Slider) SetEnabled(v bool) *Slider
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `bool` | `true` to enable (the default); `false` to disable. |

**Returns** — `*Slider` for chaining.

**Example**

```go
vol.SetEnabled(false)
```

**Notes** — Purely visual/behavioral: no invalidation needed.

#### Slider.MeasureContent

Reports the fixed desired size for the current orientation.

**Syntax**

```go
func (s *Slider) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | The space available to the slider. Not consulted — the desired size is fixed. |

**Returns** — `render.Size` — `{160, 24}` for `Horizontal` (the default),
swapped to `{24, 160}` (tall-narrow) for `Vertical`. An explicit
`SetWidth`/`SetHeight` overrides this through `core.MeasureWidget`'s normal
explicit-size precedence.

#### Slider.ArrangeContent

No-op.

**Syntax**

```go
func (s *Slider) ArrangeContent(bounds render.Rect)
```

**Notes** — `Slider` has no children to position.

#### Slider.Children

Returns `nil`.

**Syntax**

```go
func (s *Slider) Children() []core.Widget
```

**Returns** — `[]core.Widget`, always `nil` — `Slider` is a leaf widget.

#### Slider.Render

Paints the classic trackbar: a thin sunken groove, a Highlight-filled band,
and a square 16x16 raised thumb.

**Syntax**

```go
func (s *Slider) Render(r render.Renderer)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | The renderer to draw into. |

**Notes** — The groove runs the slider's full main-axis length (`ButtonFace`
sunken fill). The fill spans the track's Min side up to the thumb's center.
The thumb is `ButtonLight` while hovered, `ButtonFace` at rest — both raised.
Horizontal: groove left-to-right, fill from the left edge. Vertical: groove
top-to-bottom, fill from the thumb center down to the bottom edge.

#### Slider.RenderOverlay

Draws the focus ring while focused.

**Syntax**

```go
func (s *Slider) RenderOverlay(r render.Renderer)
```

**Notes** — Per the global focus-ring convention shared by every focusable
control in this package.

#### Slider.AcceptsFocus

Implements `input.Focusable`.

**Syntax**

```go
func (s *Slider) AcceptsFocus() bool
```

**Returns** — `bool` — `false` while disabled.

#### Slider.OnFocusChanged

Implements `input.FocusHandler`, tracking focus for the focus-ring overlay
and keyboard arrow-key handling.

**Syntax**

```go
func (s *Slider) OnFocusChanged(focused bool)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `focused` | `bool` | The slider's new focus state. |

#### Slider.OnPointer

Implements `input.PointerHandler`: click-on-track jumps the value straight to
the clicked position; drag continues updating it.

**Syntax**

```go
func (s *Slider) OnPointer(e *input.PointerEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.PointerEvent` | The pointer event. |

**Notes** — The pointer is captured on Press so a subsequent drag survives
leaving the slider's own bounds; Move only updates the value while this
slider holds the capture. Ignored entirely while disabled (not handled, so
events bubble past a disabled slider) — except a `SetEnabled(false)` landing
mid-drag first releases the capture this slider already holds.

#### Slider.OnKey

Left/Right nudge the value.

**Syntax**

```go
func (s *Slider) OnKey(e *input.KeyEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.KeyEvent` | The key event. |

**Notes** — On Press only: Left decreases and Right increases the value by
`(Max-Min)/100`, or `(Max-Min)/10` with Shift held — regardless of
orientation (Left always decreases, Right always increases; the arrow-key
step is not remapped for `Vertical`). `OnKey` is only ever invoked while this
slider is focused or an ancestor of the focused widget, so there is no
separate focused check.

---

## ProgressBar

`ProgressBar` is a non-interactive, token-styled progress indicator over
`[0, 1]`. Unlike every other control in this package, it implements none of
`input.Focusable`/`PointerHandler`/`KeyHandler`/`FocusHandler` — a bare
`core.Element` embed already reports no such methods, so `ProgressBar` is
simply never a candidate for router focus and never receives pointer/key
events.

Orientation defaults to `Horizontal`: the track runs left-to-right and the
fill grows left-to-right. **Vertical runs top-to-bottom and the fill grows
bottom-to-top.** The fill is either the default "chunked" look (discrete
`Highlight` blocks, the Windows-2000 marching-blocks look) or, via
`SetSolid(true)`, a single solid `Highlight` fill spanning the value
proportion with no gaps. `ProgressBar` never has a thumb.

**Constructor**

```go
func NewProgressBar() *ProgressBar
```

Returns a `Horizontal`, chunked `ProgressBar` at `Value` `0`.

**Example**

```go
bar := controls.NewProgressBar()
bar.SetValue(0.3)
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [Value](#progressbarvalue) | `Value() float32` | Returns the current progress value, in `[0, 1]`. |
| [SetValue](#progressbarsetvalue) | `SetValue(v float32) *ProgressBar` | Sets the progress value, clamped into `[0, 1]`. |
| [SetOrientation](#progressbarsetorientation) | `SetOrientation(o Orientation) *ProgressBar` | Sets Horizontal (default) or Vertical layout. |
| [SetSolid](#progressbarsetsolid) | `SetSolid(v bool) *ProgressBar` | Switches between chunked (default) and solid fill. |
| [MeasureContent](#progressbarmeasurecontent) | `MeasureContent(available render.Size) render.Size` | Reports the fixed desired size for the current orientation. |
| [ArrangeContent](#progressbararrangecontent) | `ArrangeContent(bounds render.Rect)` | No-op — `ProgressBar` has no children. |
| [Children](#progressbarchildren) | `Children() []core.Widget` | Returns `nil` — `ProgressBar` is a leaf widget. |
| [Render](#progressbarrender) | `Render(r render.Renderer)` | Paints the sunken well and the value-proportion fill. |

#### ProgressBar.Value

Returns the current progress value.

**Syntax**

```go
func (p *ProgressBar) Value() float32
```

**Returns** — `float32`, in `[0, 1]`.

**See also** — [SetValue](#progressbarsetvalue)

#### ProgressBar.SetValue

Sets the progress value, clamped into `[0, 1]`.

**Syntax**

```go
func (p *ProgressBar) SetValue(v float32) *ProgressBar
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `float32` | The new value; clamped into `[0, 1]`. |

**Returns** — `*ProgressBar` for chaining.

**Example**

```go
bar.SetValue(0.75)
```

**Notes** — `ProgressBar` is display-only: there is no `OnChanged` and no
user-driven mutation path, so this is the only way its value ever changes.

#### ProgressBar.SetOrientation

Sets the progress bar's orientation.

**Syntax**

```go
func (p *ProgressBar) SetOrientation(o Orientation) *ProgressBar
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `o` | `Orientation` | `Horizontal` (the default) fills left-to-right; `Vertical` fills **bottom-to-top**. |

**Returns** — `*ProgressBar` for chaining.

**Example**

```go
bar.SetOrientation(controls.Vertical)
```

**Notes** — Takes effect on the next Measure/Arrange/Render pass. `Vertical`
also swaps `MeasureContent`'s desired size to tall-narrow.

**See also** — [Slider.SetOrientation](#slidersetorientation)

#### ProgressBar.SetSolid

Sets whether the fill is a single solid bar or the default chunked blocks.

**Syntax**

```go
func (p *ProgressBar) SetSolid(v bool) *ProgressBar
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `bool` | `true` for a single solid `Highlight` fill; `false` (the default) for discrete chunked blocks. |

**Returns** — `*ProgressBar` for chaining.

**Example**

```go
bar.SetSolid(true)
```

**Notes** — Chunked mode's block size is a square on the cross axis (as deep
as the inset well) with a 2px gap to the next block; only whole blocks that
fit entirely within the filled region are drawn, so the fill grows one whole
chunk at a time as `Value` increases rather than ever drawing a partial
block.

#### ProgressBar.MeasureContent

Reports the fixed desired size for the current orientation.

**Syntax**

```go
func (p *ProgressBar) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | The space available to the bar. Not consulted — the desired size is fixed. |

**Returns** — `render.Size` — `{160, 8}` for `Horizontal` (the default),
swapped to `{8, 160}` (tall-narrow) for `Vertical`. An explicit
`SetWidth`/`SetHeight` overrides this through `core.MeasureWidget`'s normal
explicit-size precedence.

#### ProgressBar.ArrangeContent

No-op.

**Syntax**

```go
func (p *ProgressBar) ArrangeContent(bounds render.Rect)
```

**Notes** — `ProgressBar` has no children to position.

#### ProgressBar.Children

Returns `nil`.

**Syntax**

```go
func (p *ProgressBar) Children() []core.Widget
```

**Returns** — `[]core.Widget`, always `nil` — `ProgressBar` is a leaf
widget.

#### ProgressBar.Render

Paints the classic sunken well and, inside it, the value-proportion fill.

**Syntax**

```go
func (p *ProgressBar) Render(r render.Renderer)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | The renderer to draw into. |

**Notes** — The well is a `WindowWell` sunken fill with a 2px bevel. The
fill is inset by that 2px on every side and dispatches to the chunked or
solid renderer per `SetSolid`, each honoring the current orientation:
Horizontal grows left-to-right, Vertical grows **bottom-to-top**.

---

## ComboBox

`ComboBox` is a clickable, focusable, token-styled dropdown: a Button-like
field showing the selected item's text (or a placeholder) plus a chevron,
which opens a popup listing every item as a clickable row — on click, or on
Space/Enter/Down while focused. The popup renders through the overlay
system; see [overlays](controls-overlays.md) for the `OverlayHost`/`OverlayHostFor`
mechanics that host it (`ComboBox` is a modal-popup consumer alongside
`ToolTipArea`). The field stays focused the entire time the popup is open,
so Esc naturally reaches `ComboBox.OnKey` via the router's focused-widget key
dispatch.

**Constructor**

```go
func NewComboBox(face *text.Face) *ComboBox
```

Returns an enabled, closed `ComboBox` with no items and no selection
(`SelectedIndex() == -1`, showing the placeholder `"Select…"`), drawing text
with `face` (`face` may be `nil`, per `TextBlock`).

**Example**

```go
combo := controls.NewComboBox(face)
combo.SetItems([]string{"Small", "Medium", "Large"})
combo.OnChanged(func(i int) {
    fmt.Println("selected:", i)
})
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [SetItems](#comboboxsetitems) | `SetItems(items []string) *ComboBox` | Replaces the item list, re-clamping the current selection. |
| [SelectedIndex](#comboboxselectedindex) | `SelectedIndex() int` | Returns the current selection, or `-1` if none. |
| [SetSelectedIndex](#comboboxsetselectedindex) | `SetSelectedIndex(i int) *ComboBox` | Sets the selection programmatically. |
| [OnChanged](#comboboxonchanged) | `OnChanged(fn func(int)) *ComboBox` | Sets the callback fired with the new index on user selection. |
| [SetEnabled](#comboboxsetenabled) | `SetEnabled(v bool) *ComboBox` | Toggles whether the combo accepts focus and input. |
| [IsOpen](#comboboxisopen) | `IsOpen() bool` | Reports whether the popup is currently showing. |
| [MeasureContent](#comboboxmeasurecontent) | `MeasureContent(available render.Size) render.Size` | Measures the chevron and label to compute the field's desired size. |
| [ArrangeContent](#comboboxarrangecontent) | `ArrangeContent(bounds render.Rect)` | Places the label in the well and the chevron in the drop button. |
| [Children](#comboboxchildren) | `Children() []core.Widget` | Returns the label and chevron. |
| [Render](#comboboxrender) | `Render(r render.Renderer)` | Paints the field chrome and recolors the label/chevron for state. |
| [RenderOverlay](#comboboxrenderoverlay) | `RenderOverlay(r render.Renderer)` | Draws the focus ring while focused. |
| [AcceptsFocus](#comboboxacceptsfocus) | `AcceptsFocus() bool` | Implements `input.Focusable`; `false` while disabled. |
| [OnFocusChanged](#comboboxonfocuschanged) | `OnFocusChanged(focused bool)` | Implements `input.FocusHandler`. |
| [OnPointer](#comboboxonpointer) | `OnPointer(e *input.PointerEvent)` | Delegates to the field's click behavior. |
| [OnKey](#comboboxonkey) | `OnKey(e *input.KeyEvent)` | Space/Enter/Down open the popup; Escape closes it. |

#### ComboBox.SetItems

Replaces the item list, re-clamping the current selection into the new
range and refreshing the field's label.

**Syntax**

```go
func (c *ComboBox) SetItems(items []string) *ComboBox
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `items` | `[]string` | The new item list, copied internally. |

**Returns** — `*ComboBox` for chaining.

**Example**

```go
combo.SetItems([]string{"Red", "Green", "Blue"})
```

**Notes** — **Silent**: does not fire `OnChanged`, even if the re-clamp
changes `SelectedIndex()` (e.g. shrinking below the current selection) —
programmatic setters are silent, `OnChanged` reports only user-driven
changes.

**See also** — [SelectedIndex](#comboboxselectedindex)

#### ComboBox.SelectedIndex

Returns the current selection, or `-1` if none.

**Syntax**

```go
func (c *ComboBox) SelectedIndex() int
```

**Returns** — `int`, the selected item's index, or `-1`.

**See also** — [SetSelectedIndex](#comboboxsetselectedindex)

#### ComboBox.SetSelectedIndex

Sets the selection programmatically, clamped into `[-1, len(items)-1]`.

**Syntax**

```go
func (c *ComboBox) SetSelectedIndex(i int) *ComboBox
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `i` | `int` | The new selected index. `-1` is always a valid, explicit "no selection" value, not merely a clamp target. |

**Returns** — `*ComboBox` for chaining.

**Example**

```go
combo.SetSelectedIndex(1)
```

**Notes** — **Silent**: never fires `OnChanged`, matching the
`CheckBox`/`ToggleButton`/`RadioButton` `SetChecked` convention —
programmatic changes are silent; only user-driven ones (a click, or
Space/Enter/Down then a click) notify.

**See also** — [OnChanged](#comboboxonchanged)

#### ComboBox.OnChanged

Sets the callback fired with the new index whenever the user selects a
(different) item by clicking a popup row.

**Syntax**

```go
func (c *ComboBox) OnChanged(fn func(int)) *ComboBox
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `fn` | `func(int)` | Called with the newly selected index. `nil` is a valid, silent no-op. |

**Returns** — `*ComboBox` for chaining.

**Example**

```go
combo.OnChanged(func(i int) { applySize(i) })
```

**Notes** — Never fires for a programmatic `SetSelectedIndex` or `SetItems`.
Re-clicking the already-selected row is a no-op notification — `OnChanged`
fires only when the selection actually changes, matching `Slider.SetValue`'s
"notify only on real change" convention rather than `CheckBox`'s
always-fire-on-user-action one. Replaces any previously set callback.

#### ComboBox.SetEnabled

Toggles whether the combo accepts focus and pointer/keyboard input.

**Syntax**

```go
func (c *ComboBox) SetEnabled(v bool) *ComboBox
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `bool` | `true` to enable (the default); `false` to disable. |

**Returns** — `*ComboBox` for chaining.

**Example**

```go
combo.SetEnabled(false)
```

**Notes** — Both `OnPointer` and `OnKey` ignore all input while disabled,
and `AcceptsFocus` returns `false`. Purely visual/behavioral: no
invalidation needed.

#### ComboBox.IsOpen

Reports whether the popup is currently showing.

**Syntax**

```go
func (c *ComboBox) IsOpen() bool
```

**Returns** — `bool`, `true` while the dropdown popup is open.

#### ComboBox.MeasureContent

Measures the chevron and label to compute the field's desired size.

**Syntax**

```go
func (c *ComboBox) MeasureContent(available render.Size) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `available` | `render.Size` | The space available to the field. |

**Returns** — `render.Size`, the combined width of the label and chevron
(plus a gap and padding) and the taller of the two heights (plus padding).

**Notes** — The chevron is measured first and never shrinks to make room for
the label; the label is then measured in whatever width remains.

#### ComboBox.ArrangeContent

Places the label inside the sunken well and centers the chevron glyph within
the raised drop-button strip.

**Syntax**

```go
func (c *ComboBox) ArrangeContent(bounds render.Rect)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `bounds` | `render.Rect` | The field's newly arranged bounds. |

**Notes** — The field splits into a sunken text well (left) and a raised,
square drop-button strip (right, as wide as the field is tall) — the
classic Windows combo proportion.

#### ComboBox.Children

Returns the label and chevron.

**Syntax**

```go
func (c *ComboBox) Children() []core.Widget
```

**Returns** — `[]core.Widget{label, chevron}`.

#### ComboBox.Render

Paints the field chrome and recolors the label/chevron for the current
state.

**Syntax**

```go
func (c *ComboBox) Render(r render.Renderer)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | The renderer to draw into. |

**Notes** — The text well is sunken `WindowWell`; the drop-button strip is
raised `ButtonFace`, drawn sunken instead while pressed (v0 has no separate
press target for just the button — the whole field opens the popup). The
label recolors to `GrayText` while showing the placeholder or disabled, else
`WindowText`; the chevron recolors to `GrayText` while disabled, else
`WindowText`.

#### ComboBox.RenderOverlay

Draws the focus ring while focused.

**Syntax**

```go
func (c *ComboBox) RenderOverlay(r render.Renderer)
```

**Notes** — Per the global focus-ring convention shared by every focusable
control in this package.

#### ComboBox.AcceptsFocus

Implements `input.Focusable`.

**Syntax**

```go
func (c *ComboBox) AcceptsFocus() bool
```

**Returns** — `bool` — `false` while disabled.

#### ComboBox.OnFocusChanged

Implements `input.FocusHandler`, tracking focus for the focus-ring overlay
and keyboard activation.

**Syntax**

```go
func (c *ComboBox) OnFocusChanged(focused bool)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `focused` | `bool` | The combo's new focus state. |

#### ComboBox.OnPointer

Implements `input.PointerHandler`, delegating to the field's click behavior.

**Syntax**

```go
func (c *ComboBox) OnPointer(e *input.PointerEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.PointerEvent` | The pointer event. |

**Notes** — Disabled ignores pointer input outright (`e.Handled` left
`false` so the event keeps bubbling). The click behavior's click callback —
wired to open the popup — fires on release-inside, after the pointer capture
has already been released, per the overlay system's documented convention
(see [overlays](controls-overlays.md)).

#### ComboBox.OnKey

Space, Enter, or Down open the popup; Escape closes it.

**Syntax**

```go
func (c *ComboBox) OnKey(e *input.KeyEvent)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `e` | `*input.KeyEvent` | The key event. |

**Notes** — Ignored entirely while disabled or for anything but
`Action==Press`. Escape closes the popup only if it is currently open.
Space/Enter/Down open it only if not already open (each sets `e.Handled =
true` regardless).

**See also** — [IsOpen](#comboboxisopen)

---

## Notes

- **Silent setters:** every programmatic setter across these four types
  (`TextBox.SetText`, `Slider.SetValue`/`SetRange`, `ProgressBar.SetValue`,
  `ComboBox.SetSelectedIndex`/`SetItems`) never fires `OnChanged` — only
  user-driven interaction (typing, dragging, clicking, or the type's
  keyboard map) does. This is a library-wide convention shared with
  `CheckBox`/`ToggleSwitch`/`ToggleButton`.
- **Slider's usable-track-span inset:** the thumb's center is confined to
  `[thumbRadius, mainAxisLength-thumbRadius]` rather than the raw track
  span, so a 16px-diameter thumb's edge never overhangs the track at either
  extreme. `Slider`'s value-mapping methods (`thumbCenter`,
  `valueFromLocal`) both divide by `mainAxisLength - 2*thumbRadius`
  accordingly.
- **Vertical conventions differ by type:** a vertical `Slider` puts `Max`
  at the top; a vertical `ProgressBar` fills bottom-to-top from `Value` 0.
  Neither is the mirror of the other — check the type's own doc comment
  before assuming.
- `ComboBox`'s popup content (`comboPopupCard`, `comboRow`) is built
  internally and re-created on every open from the current items/selection;
  neither type is exported, and both are hosted through the overlay system
  documented on the [overlays](controls-overlays.md) page.
