# theme

The `theme` package holds fluo's design tokens: a `Theme` bundles a color
palette (`ColorTokens`), a layout/sizing scale (`MetricTokens`), and a
typography ramp (`TypeTokens`). Both built-in variants (`Light`, `Dark`)
implement the classic Windows-2000-style four-tone bevel look. Reach for this
package when you need to read the current palette to paint a custom widget,
or when you want to swap the whole app's look by building a modified `Theme`
and calling `SetActive`.

**Import:** `github.com/0xdreadnaught/fluo/theme`

## Contents
- [Theme](#theme-1)
- [ColorTokens](#colortokens)
- [MetricTokens](#metrictokens)
- [TypeTokens](#typetokens)

---

## Theme

`Theme` represents a complete color, metric, and typography design system:
a `Name` identifying the variant, plus one instance each of `ColorTokens`,
`MetricTokens`, and `TypeTokens`. There is no `NewTheme` constructor — themes
are produced by the package-level `Light`/`Dark` functions (or built by
copying and mutating one of them) and installed process-wide with
`SetActive`. Every `controls` widget reads `theme.Active()` once, at
construction time, and copies out the tokens it needs — themes are not
live-reskinned, so changing the active theme only affects widgets built
afterward (see `cmd/fluo-gallery`'s T-key toggle for the reference
re-theming pattern: call `SetActive`, then rebuild the widget tree).

### Fields

| Name | Type | Description |
|---|---|---|
| `Name` | `string` | Identifies the variant. The built-in themes use `"classic-light"` and `"classic-dark"`. |
| `Color` | `ColorTokens` | The theme's color palette. |
| `Metric` | `MetricTokens` | The theme's layout and sizing scale. |
| `Type` | `TypeTokens` | The theme's typography sizes. |

### Functions

| Function | Signature | Description |
|---|---|---|
| [Light](#light) | `func Light() *Theme` | Returns a fresh Theme with the classic light four-tone bevel palette. |
| [Dark](#dark) | `func Dark() *Theme` | Returns a fresh Theme with the classic dark four-tone bevel palette. |
| [Active](#active) | `func Active() *Theme` | Returns the process-wide active theme, defaulting to Light. |
| [SetActive](#setactive) | `func SetActive(t *Theme)` | Sets the process-wide active theme. |

#### Light

Returns a fresh Theme with the classic light (Windows-2000 "Standard")
four-tone bevel palette.

**Syntax**

```go
func Light() *Theme
```

**Returns** — `*Theme` named `"classic-light"`. Each call allocates a new
`Theme` value (not a shared singleton); `Metric` and `Type` are the palette
values shared by both built-in variants (see the [MetricTokens](#metrictokens)
and [TypeTokens](#typetokens) value tables).

**Example**

```go
theme.SetActive(theme.Light())
```

**See also** — [Dark](#dark), [Active](#active), [SetActive](#setactive)

#### Dark

Returns a fresh Theme with the classic dark four-tone bevel palette.

**Syntax**

```go
func Dark() *Theme
```

**Returns** — `*Theme` named `"classic-dark"`. Each call allocates a new
`Theme` value; `Metric` and `Type` match [Light](#light)'s.

**Example**

```go
theme.SetActive(theme.Dark())
```

**See also** — [Light](#light), [Active](#active), [SetActive](#setactive)

#### Active

Returns the currently active theme, never nil (defaults to Light).

**Syntax**

```go
func Active() *Theme
```

**Returns** — `*Theme`, the process-wide current theme. If no theme has
been set (or it was reset via `SetActive(nil)`), returns `Light()`.

**Example**

```go
c := theme.Active().Color
label.SetColor(c.WindowText)
```

**Notes** — Not safe for concurrent use; fluo v0 assumes a single UI
goroutine and one active theme per process.

**See also** — [SetActive](#setactive), [Light](#light), [Dark](#dark)

#### SetActive

Sets the active theme. If `t` is nil, resets to the default (Light).

**Syntax**

```go
func SetActive(t *Theme)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `t` | `*Theme` | The theme to install process-wide. `nil` resets to `Light()`. |

**Notes** — Not safe for concurrent use; fluo v0 assumes a single UI
goroutine and one active theme per process. Because widgets capture tokens
from `theme.Active()` at construction time rather than live-reskinning,
calling `SetActive` only changes the theme for widgets built *after* the
call — rebuild the widget tree to re-theme an existing app.

**Example — building a custom theme**

Copy a built-in theme, override the tokens you want to change, and install
it before constructing any widgets:

```go
custom := theme.Dark()
custom.Name = "midnight"
custom.Color.Highlight = render.RGB(255, 140, 0)     // orange selection
custom.Color.HighlightText = render.RGB(0, 0, 0)
custom.Color.CaptionFrom = render.RGB(40, 20, 60)
custom.Color.CaptionTo = render.RGB(90, 40, 120)

theme.SetActive(custom)

// Build the widget tree after SetActive so it picks up the new palette.
root := controls.NewButton("OK")
```

**See also** — [Active](#active), [ColorTokens](#colortokens)

---

## ColorTokens

`ColorTokens` holds the semantic color values for a theme. The classic
fields (`ButtonFace` … `InactiveCaption`) are the Windows-2000-style
four-tone bevel palette introduced in v0.2: controls render their
raised/sunken chrome from `ButtonFace`/`ButtonHighlight`/`ButtonLight`/
`ButtonShadow`/`ButtonDarkShadow`, plain content areas from
`WindowWell`/`WindowText`/`GrayText`, selection from
`Highlight`/`HighlightText`, and title bars from the `CaptionFrom`→`CaptionTo`
gradient (`CaptionText`/`InactiveCaption` for the caption label and the
unfocused-window variant). Two smaller groups sit alongside them: the four
`Severity*` accents added in v0.8 for `Toast`'s colored stripe, and
`AcrylicTint`, the translucent tint `controls.AcrylicSurface` composites over
a backdrop blur.

The pre-v0.2 flat token set (`Accent`, `TextPrimary`, `ControlFill`, and the
rest) was removed in v0.15.0. If you are porting code that read those, see
[Replacements for the removed flat tokens](#replacements-for-the-removed-flat-tokens).

### Fields

**Classic tokens**

| Name | Type | Description |
|---|---|---|
| `ButtonFace` | `render.Color` | Flat fill of buttons, panels, and other 3D chrome. |
| `ButtonHighlight` | `render.Color` | Brightest bevel edge (top/left of a raised control, bottom/right of a sunken one). |
| `ButtonLight` | `render.Color` | Secondary lit bevel edge, one step in from `ButtonHighlight`. |
| `ButtonShadow` | `render.Color` | Secondary dark bevel edge, one step in from `ButtonDarkShadow`. |
| `ButtonDarkShadow` | `render.Color` | Darkest bevel edge (bottom/right of a raised control, top/left of a sunken one). |
| `WindowWell` | `render.Color` | Recessed content background used by text/list/tree controls. |
| `WindowText` | `render.Color` | Default text color over `WindowWell`/`ButtonFace`. |
| `GrayText` | `render.Color` | Disabled/secondary text. |
| `Highlight` | `render.Color` | Selection fill (selected list rows, selected text). |
| `HighlightText` | `render.Color` | Text/foreground color over a `Highlight` fill. |
| `CaptionFrom` | `render.Color` | Left stop of the active title bar's left-to-right gradient. |
| `CaptionTo` | `render.Color` | Right stop of the active title bar's left-to-right gradient. |
| `CaptionText` | `render.Color` | Title bar label color. |
| `InactiveCaption` | `render.Color` | Flat fill an unfocused window's title bar uses in place of the `CaptionFrom`/`CaptionTo` gradient. |

**Severity accents**

| Name | Type | Description |
|---|---|---|
| `SeverityInfo` | `render.Color` | Accent for an informational condition. Deliberately unused by `Toast`'s default rendering (see below), but set to a real color so anything else can use it. |
| `SeveritySuccess` | `render.Color` | Accent for a condition that completed as intended. |
| `SeverityWarning` | `render.Color` | Accent for a condition worth attention but not a failure. |
| `SeverityError` | `render.Color` | Accent for a failed or broken condition. |

**Surface effects**

| Name | Type | Description |
|---|---|---|
| `AcrylicTint` | `render.Color` | Translucent tint composited over a backdrop-blur acrylic/mica surface (see `controls.AcrylicSurface`). The one token in the palette carrying partial alpha. |

### Classic color tokens

Each entry below names the control(s) that currently consume the token
(grep-verified against `controls/`), so the description reflects real usage
rather than the intent alone.

#### ColorTokens.ButtonFace

Flat fill of buttons, panels, and other 3D chrome.

The interior fill passed to `drawRaised`/`drawSunken` (see `controls/bevel.go`)
for a control's rest-state chrome. Consumed by `Button` (rect and pill/circle
shapes), `TabControl` (header cells and the body panel), `Menu` popup cards,
`Dialog` cards, `ToolTipArea` popup cards, `ComboBox` (drop button and popup
card), `Expander`'s header, and `ScrollViewer`'s track/thumb (via
`drawScrollThumb`). `TextBox` also swaps its `WindowWell` fill to
`ButtonFace` while disabled (the classic "grayed-out field" look).

**Example**

```go
c := theme.Active().Color
r.FillRect(bounds, c.ButtonFace)
```

#### ColorTokens.ButtonHighlight

Brightest bevel edge (top/left of a raised control, bottom/right of a
sunken one).

Painted by `drawRaised`/`drawSunken`/`drawRaisedRounded`/`drawSunkenRounded`
as the lit outer edge, and by `drawGroove` as the second (lit) line of an
etched separator. `Button` also uses it for the disabled-label "engrave"
effect: the label glyphs are drawn once in `ButtonHighlight`, offset
(+1,+1), beneath the normal `GrayText` glyphs.

#### ColorTokens.ButtonLight

Secondary lit bevel edge, one step in from `ButtonHighlight`.

Used as the inner lit edge in `drawRaised`/`drawSunken`. Several controls
also use it directly as the hover-state face for a raised control in place
of `ButtonFace`: `Button`, `TabControl` header cells, and `Expander`'s
header all switch to `drawRaised(..., ButtonLight, ...)` on hover. `Slider`
uses it as the thumb's hover fill.

#### ColorTokens.ButtonShadow

Secondary dark bevel edge, one step in from `ButtonDarkShadow`.

Used as the inner dark edge in `drawRaised`/`drawSunken`. Also painted
directly as a 1px groove/rule: `drawScrollThumb`'s inner track edge,
`RadioButton`'s outer ring stroke, and `DataGrid`'s per-row horizontal grid
line all use `ButtonShadow` as a flat 1px line rather than as part of a
bevel.

#### ColorTokens.ButtonDarkShadow

Darkest bevel edge (bottom/right of a raised control, top/left of a sunken
one).

Used as the outer dark edge in `drawRaised`/`drawSunken`. Also drawn as the
extra 1px ring `drawOuterBorder` paints just outside an accent ("default")
`Button`'s bevel — the classic default-button marker.

#### ColorTokens.WindowWell

Recessed content background used by text/list/tree controls (the "well" a
document or list sits in).

Filled by `drawSunken` as the base for `TextBox`'s input well, `ListView`
and `TreeView`'s scrollable frame, `DataGrid`'s outer frame, and the
sunken box behind `CheckBox`/`RadioButton`. `ComboBox`'s popup card fills
its inner (bevel-inset) list area with `WindowWell` as well.

#### ColorTokens.WindowText

Default text color over `WindowWell`/`ButtonFace`.

The baseline label/glyph color across the library: `Button` labels (when
enabled), `TextBox` text and caret, `ListView`/`TreeView`/`DataGrid` row
text (when not selected), `Menu` item/title labels and chevrons (when not
hovered), `TitleBar`... *(see `CaptionText` for the title bar label, which
uses its own token)*, `TabControl` tab titles, `ComboBox` chevron and label
(when not showing the placeholder or disabled), and the checkmark/dot glyph
in `CheckBox`/`RadioButton`.

#### ColorTokens.GrayText

Disabled/secondary text.

Used for disabled labels (`Button`, `CheckBox`) and for placeholder text:
`TextBox` draws its placeholder string in `GrayText` regardless of focus,
and `ComboBox` uses it for its placeholder label and for the label/chevron
while disabled.

#### ColorTokens.Highlight

Selection fill (selected list rows, selected text).

The library's one accent color in the classic palette. Paints the selected
row band in `ListView`, `TreeView`, and `DataGrid`; the hovered row in
`Menu`; the current-selection/hovered row in `ComboBox`'s popup; the
selected-text band in `TextBox`; the filled portion of `Slider` and
`ProgressBar` (both chunked and solid fill modes); the "on" state of
`ToggleSwitch`'s track; and the classic 1px inset focus rectangle drawn by
`drawFocusRect`.

#### ColorTokens.HighlightText

Text/foreground color over a `Highlight` fill.

Paired with `Highlight` wherever it's used as a selection band: selected
row text in `ListView`/`TreeView`/`DataGrid`, hovered item/title text in
`Menu`, selected/hovered row text in `ComboBox`'s popup, and selected glyph
color in `TextBox`.

#### ColorTokens.CaptionFrom

Left stop of the active title bar's left-to-right gradient (with
`CaptionTo`).

Passed to `render.Renderer.DrawGradientRect` by `TitleBar.Render` to paint
the caption bar background.

#### ColorTokens.CaptionTo

Right stop of the active title bar's left-to-right gradient (with
`CaptionFrom`).

Passed to `render.Renderer.DrawGradientRect` alongside `CaptionFrom`; see
`TitleBar.Render`.

#### ColorTokens.CaptionText

Title bar label color.

Set as the title `TextBlock`'s color in `TitleBar`.

#### ColorTokens.InactiveCaption

Flat fill an unfocused window's title bar uses in place of the
`CaptionFrom`/`CaptionTo` gradient.

**Notes** — Reserved for the unfocused-window caption state; `TitleBar` in
the current control set only renders the focused/active gradient, so this
token is not yet consumed anywhere.

### Severity accents

Added in v0.8.0 alongside `Toast`'s `Severity` field. `Toast` maps its
`Severity` to one of these and paints it as an accent stripe down the card's
left inner edge, inset by `BevelWidth` so it sits inside the raised bevel
rather than covering it.

#### ColorTokens.SeverityInfo

Accent for an informational condition.

`controls.SeverityInfo` is `Severity`'s zero value, and a toast carrying it
renders with **no** color cue at all — a plain bevelled card — so this token
has no reader in the current control set. Both themes still set it to a real
color so anything classifying conditions by severity has a full four-color
set to draw from.

#### ColorTokens.SeveritySuccess

Accent for a condition that completed as intended. Painted as `Toast`'s
stripe for `controls.SeveritySuccess`.

#### ColorTokens.SeverityWarning

Accent for a condition worth the user's attention but not a failure.
Painted as `Toast`'s stripe for `controls.SeverityWarning`.

#### ColorTokens.SeverityError

Accent for a failed or broken condition. Painted as `Toast`'s stripe for
`controls.SeverityError`.

### Surface effects

#### ColorTokens.AcrylicTint

Translucent tint composited over a backdrop-blur acrylic/mica surface.

Read by `controls.NewAcrylicSurface` as its default tint (overridable per
instance with `SetTint`). The only token in the palette with partial alpha —
both themes set it to their own `ButtonFace` at alpha 180. The classic themes
keep it even though nothing else in their chrome uses translucency.

**Example**

```go
c := theme.Active().Color
surface := controls.NewAcrylicSurface().SetTint(c.AcrylicTint)
```

### Replacements for the removed flat tokens

The pre-v0.2 flat tokens were removed in v0.15.0, after the last control
still reading one was migrated onto the classic palette. Code written against
the old names will not compile against v0.15+; this is the mapping to port
by. Where a token has no classic equivalent, the effect it named is gone from
the classic chrome rather than renamed.

| Removed field | Read instead |
|---|---|
| `WindowBackground`, `LayerBackground`, `CardBackground` | `ButtonFace` |
| `TextPrimary` | `WindowText` |
| `TextSecondary`, `TextDisabled` | `GrayText` |
| `Accent`, `SelectionBackground` | `Highlight` |
| `AccentText`, `SelectionForeground` | `HighlightText` |
| `AccentHover`, `AccentPressed` | *(none — the classic palette has one selection color, not a hover/press ramp)* |
| `ControlFill`, `ControlFillPressed`, `ControlFillDisabled` | `ButtonFace` |
| `ControlFillHover` | `ButtonLight` (the classic hover face — see [ButtonLight](#colortokensbuttonlight)) |
| `ControlStroke`, `ControlStrokeDisabled` | `ButtonShadow` |
| `AccentDisabled` | `GrayText` |
| `ScrimBackground` | *(none — `Dialog`'s scrim is painted invisibly, matching classic Win2000 modals)* |

**Notes** — `AcrylicTint` was listed with this group before v0.15 but was
never part of it: it is a live token, still read by `AcrylicSurface`, and it
survives unchanged (see [above](#colortokensacrylictint)).

---

## MetricTokens

`MetricTokens` holds the layout and sizing values for a theme. Both classic
themes (`Light` and `Dark`) share the exact same `MetricTokens` value —
square corners and a 2px bevel, per `classic.go`'s `sharedMetrics`.

### Fields

| Name | Type | Description |
|---|---|---|
| `CornerRadius` | `float32` | Corner radius for large surfaces (cards, popups). Square (`0`) in the classic themes. |
| `ControlCornerRadius` | `float32` | Corner radius for controls (buttons, inputs). Square (`0`) in the classic themes. |
| `BevelWidth` | `float32` | Thickness, in pixels, of each step of the classic four-tone 3D bevel (`ButtonHighlight`/`ButtonLight`/`ButtonShadow`/`ButtonDarkShadow`) drawn around raised/sunken chrome. |
| `StrokeWidth` | `float32` | Default stroke width for single-tone borders and rings. |
| `FocusStrokeWidth` | `float32` | Stroke width of the focus indicator. |
| `PaddingS` | `float32` | Small padding step (4/8/16 scale). |
| `PaddingM` | `float32` | Medium padding step (4/8/16 scale). |
| `PaddingL` | `float32` | Large padding step (4/8/16 scale). |
| `ScrollGutter` | `float32` | Width/height reserved for a scrollbar gutter. |
| `ShadowBlur` | `float32` | Blur radius for drop shadows. |

### Values (shared by Light and Dark)

| Field | Value |
|---|---|
| `CornerRadius` | `0` |
| `ControlCornerRadius` | `0` |
| `BevelWidth` | `2` |
| `StrokeWidth` | `1` |
| `FocusStrokeWidth` | `2` |
| `PaddingS` | `4` |
| `PaddingM` | `8` |
| `PaddingL` | `16` |
| `ScrollGutter` | `12` |
| `ShadowBlur` | `16` |

**See also** — [ColorTokens](#colortokens), [TypeTokens](#typetokens)

---

## TypeTokens

`TypeTokens` holds the typography sizes for a theme, in pixels. Both classic
themes share the exact same `TypeTokens` value, per `classic.go`'s
`sharedType`.

### Fields

| Name | Type | Description |
|---|---|---|
| `CaptionSize` | `float32` | Font size, in px, for captions and other small/secondary labels. |
| `BodySize` | `float32` | Font size, in px, for body text — the default control label size. |
| `SubtitleSize` | `float32` | Font size, in px, for subtitles. |
| `TitleSize` | `float32` | Font size, in px, for titles. |

### Values (shared by Light and Dark)

| Field | Value (px) |
|---|---|
| `CaptionSize` | `12` |
| `BodySize` | `14` |
| `SubtitleSize` | `20` |
| `TitleSize` | `28` |

**See also** — [ColorTokens](#colortokens), [MetricTokens](#metrictokens)

---

## Palette reference

The full `ColorTokens` values set by [Light](#light) and [Dark](#dark),
read directly from `classic.go`. Colors are constructed with
`render.RGB(r, g, b)` (full alpha) unless noted otherwise.

### Classic tokens

| Field | Light() | Dark() |
|---|---|---|
| `ButtonFace` | `RGB(212, 208, 200)` | `RGB(58, 58, 58)` |
| `ButtonHighlight` | `RGB(255, 255, 255)` | `RGB(92, 92, 92)` |
| `ButtonLight` | `RGB(232, 228, 220)` | `RGB(70, 70, 70)` |
| `ButtonShadow` | `RGB(128, 128, 128)` | `RGB(32, 32, 32)` |
| `ButtonDarkShadow` | `RGB(64, 64, 64)` | `RGB(0, 0, 0)` |
| `WindowWell` | `RGB(255, 255, 255)` | `RGB(30, 30, 30)` |
| `WindowText` | `RGB(0, 0, 0)` | `RGB(240, 240, 240)` |
| `GrayText` | `RGB(128, 128, 128)` | `RGB(110, 110, 110)` |
| `Highlight` | `RGB(0, 0, 128)` | `RGB(42, 77, 143)` |
| `HighlightText` | `RGB(255, 255, 255)` | `RGB(255, 255, 255)` |
| `CaptionFrom` | `RGB(10, 36, 106)` | `RGB(16, 33, 74)` |
| `CaptionTo` | `RGB(166, 202, 240)` | `RGB(58, 110, 165)` |
| `CaptionText` | `RGB(255, 255, 255)` | `RGB(255, 255, 255)` |
| `InactiveCaption` | `RGB(128, 128, 128)` | `RGB(42, 42, 42)` |

### Severity accents

Brighter in `Dark()` than in `Light()`, so each still reads clearly against
the darker button face.

| Field | Light() | Dark() |
|---|---|---|
| `SeverityInfo` | `RGB(0, 90, 158)` | `RGB(70, 130, 190)` |
| `SeveritySuccess` | `RGB(0, 128, 0)` | `RGB(60, 170, 60)` |
| `SeverityWarning` | `RGB(196, 130, 0)` | `RGB(214, 158, 0)` |
| `SeverityError` | `RGB(178, 0, 0)` | `RGB(214, 60, 60)` |

### Surface effects

| Field | Light() | Dark() |
|---|---|---|
| `AcrylicTint` | `RGBA(212, 208, 200, 180)` | `RGBA(58, 58, 58, 180)` |
