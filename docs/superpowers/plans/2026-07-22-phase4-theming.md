# fluo Phase 4: Theming Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A token-based theme system with Fluent Light + Dark, all controls defaulting from tokens (no hard-coded styling), a theme-toggling gallery, and the phase milestone: a themed, clickable Fluent-styled button composite proven in a GL golden and live.

**Architecture:** Package `theme` (pure Go): a `Theme` struct of color/metric/typography token groups, `FluentLight()`/`FluentDark()` constructors, and a package-level active theme (`Active()`/`SetActive`). Widgets read tokens AT CONSTRUCTION for their defaults; explicit fluent setters still override. Runtime theme switching = rebuild the widget tree (documented v0 contract; dynamic re-theming is a Phase 8 candidate). The real `Button` control is Phase 5 — this phase's milestone button is a token-driven composite in the gallery + a GL golden, proving tokens flow end-to-end.

**Tech Stack:** Pinned deps only. `theme` is GL-free.

## Global Constraints

- All prior constraints bind (module path; go.mod PINNED; WSL-only Go with GOTOOLCHAIN=local; keyed literals; vet + gofmt gates; doc comments on exported identifiers; per-task commits with trailer `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`).
- `theme` imports `render` + stdlib ONLY.
- EXISTING GL GOLDENS MUST NOT CHANGE: every golden test passes explicit colors, so re-defaulting constructors must not affect them. If a golden fails, the wiring is wrong — never `-update` an existing golden in this phase.
- Controls keep working with zero theme setup: `theme.Active()` must never return nil (defaults to FluentDark — matches fluo's existing visual identity).
- No hard-coded style VALUES may remain in `controls` (colors, radii, the scroll gutter/thumb metrics move to tokens). Structural constants (thumb min-height 24, wheel step 48) are behavior, not style — they stay.

## File Structure

```
theme/
├── theme.go        Theme, ColorTokens, MetricTokens, TypeTokens; Active/SetActive
├── fluent.go       FluentLight(), FluentDark()
└── theme_test.go
controls/           textblock.go, scrollviewer.go re-defaulted from tokens
render/gl/renderer_test.go  + TestFluentButton milestone golden (fluent_button.png)
cmd/fluo-gallery/main.go    token-driven restyle + T-key theme toggle + demo button
```

---

### Task 1: theme package

**Files:** Create `theme/theme.go`, `theme/fluent.go`; Test `theme/theme_test.go`

**Interfaces:**
- Consumes: `render.Color` only.
- Produces (exact):

```go
package theme

type ColorTokens struct {
	WindowBackground, LayerBackground, CardBackground        render.Color
	TextPrimary, TextSecondary, TextDisabled                 render.Color
	Accent, AccentHover, AccentPressed, AccentText           render.Color
	ControlFill, ControlFillHover, ControlFillPressed        render.Color
	ControlStroke, FocusStroke                               render.Color
	ScrollThumb                                              render.Color
	Shadow                                                   render.Color
}
type MetricTokens struct {
	CornerRadius, ControlCornerRadius float32 // cards vs buttons/inputs
	StrokeWidth, FocusStrokeWidth     float32
	PaddingS, PaddingM, PaddingL      float32 // 4, 8, 16 scale
	ScrollGutter                      float32
	ShadowBlur                        float32
}
type TypeTokens struct {
	CaptionSize, BodySize, SubtitleSize, TitleSize float32 // px
}
type Theme struct {
	Name   string // "fluent-light" | "fluent-dark"
	Color  ColorTokens
	Metric MetricTokens
	Type   TypeTokens
}

func FluentLight() *Theme
func FluentDark() *Theme
func Active() *Theme      // never nil; defaults to FluentDark()
func SetActive(t *Theme)  // nil resets to the default
```

Token values (normative — Fluent/WinUI-flavored):
- Shared metrics: CornerRadius 8, ControlCornerRadius 4, StrokeWidth 1, FocusStrokeWidth 2, PaddingS 4 / PaddingM 8 / PaddingL 16, ScrollGutter 12, ShadowBlur 16. Type: Caption 12, Body 14, Subtitle 20, Title 28.
- Dark colors: WindowBackground RGB(32,32,36)... use exactly: Window (32,32,36), Layer (40,40,46), Card (44,44,50), TextPrimary (255,255,255), TextSecondary RGBA(255,255,255,160), TextDisabled RGBA(255,255,255,90), Accent (0,120,215), AccentHover (16,132,226), AccentPressed (0,100,180), AccentText (255,255,255), ControlFill RGBA(255,255,255,16), ControlFillHover RGBA(255,255,255,28), ControlFillPressed RGBA(255,255,255,10), ControlStroke RGBA(255,255,255,24), FocusStroke (0,120,215), ScrollThumb RGBA(255,255,255,60), Shadow RGBA(0,0,0,120).
- Light colors: Window (243,243,243), Layer (251,251,253), Card (255,255,255), TextPrimary (26,26,26), TextSecondary RGBA(0,0,0,150), TextDisabled RGBA(0,0,0,90), Accent (0,103,192), AccentHover (25,118,199), AccentPressed (0,86,163), AccentText (255,255,255), ControlFill RGBA(0,0,0,10), ControlFillHover RGBA(0,0,0,18), ControlFillPressed RGBA(0,0,0,6), ControlStroke RGBA(0,0,0,30), FocusStroke (0,103,192), ScrollThumb RGBA(0,0,0,70), Shadow RGBA(0,0,0,50).

- [ ] **Step 1: failing tests** — `theme/theme_test.go`:

```go
package theme

import "testing"

func TestActiveNeverNil(t *testing.T) {
	SetActive(nil)
	if Active() == nil { t.Fatal("Active returned nil") }
	if Active().Name != "fluent-dark" { t.Fatalf("default = %q", Active().Name) }
}

func TestSetActiveSwaps(t *testing.T) {
	SetActive(FluentLight())
	defer SetActive(nil)
	if Active().Name != "fluent-light" { t.Fatalf("got %q", Active().Name) }
}

func TestLightDarkDiffer(t *testing.T) {
	l, d := FluentLight(), FluentDark()
	if l.Color.WindowBackground == d.Color.WindowBackground { t.Fatal("backgrounds identical") }
	if l.Color.TextPrimary == d.Color.TextPrimary { t.Fatal("text identical") }
	if l.Metric != d.Metric { t.Fatal("metrics should be SHARED between variants") }
	if l.Type != d.Type { t.Fatal("type ramp should be shared") }
}

func TestConstructorsReturnFreshCopies(t *testing.T) {
	a, b := FluentDark(), FluentDark()
	a.Color.Accent = render.RGB(1, 2, 3)
	if b.Color.Accent == a.Color.Accent { t.Fatal("constructors share state") }
}
```

(add the `render` import; keyed literals)

- [ ] **Step 2:** FAIL → implement (constructors build fresh structs each call; `active` package var + guard). **Step 3:** green; suite+vet+gofmt. **Step 4: commit** `feat(theme): token model with Fluent light/dark`

---

### Task 2: wire controls to tokens

**Files:** Modify `controls/textblock.go`, `controls/scrollviewer.go`; Test additions in each _test.go

**Normative wiring (construction-time defaults, explicit setters still win):**
- `NewTextBlock(face, s)`: default color = `theme.Active().Color.TextPrimary` (was hard-coded white). `SetColor` overrides as today.
- `NewScrollViewer()`: gutter = `Metric.ScrollGutter`, thumb color = `Color.ScrollThumb`, thumb radius = `Metric.ControlCornerRadius` captured at construction (fields, not consts). Structural constants stay (min thumb 24, wheel 48).
- `Border`/`Fixed`/panels: no implicit visual defaults today (transparent/none) — nothing to wire; confirm no hard-coded style VALUES remain anywhere in `controls` (grep for `RGB(`/`RGBA(` outside tests must return nothing).
- Document on each wired constructor: "styled from theme.Active() at construction; rebuild to re-theme."

- [ ] **Step 1: failing tests** — in `controls/textblock_test.go`: `TestTextBlockThemeDefault` (SetActive(FluentLight()) → NewTextBlock color == light TextPrimary; SetActive(FluentDark()) → dark TextPrimary; defer SetActive(nil); read color via a new `Color() render.Color` getter — add it). In `controls/scrollviewer_test.go`: `TestScrollViewerThemeMetrics` (under Light vs Dark the gutter matches Active().Metric.ScrollGutter — observable via desired size math: child measured width = available.W − gutter).
- [ ] **Step 2:** FAIL → wire → green. **Step 3:** FULL suite — existing GL goldens MUST pass untouched (they set colors explicitly). grep check: `grep -rn "RGB(\|RGBA(" controls/*.go | grep -v _test.go` → only theme-token references (no literal channel values except inside... expect ZERO literal color constructions; move any found). **Step 4:** vet+gofmt; commit `feat(controls): default styling from theme tokens`

---

### Task 3: milestone golden + themed gallery

**Files:** `render/gl/renderer_test.go` (+ golden fluent_button.png), `cmd/fluo-gallery/main.go`, `README.md`, `ROADMAP.md`

- [ ] **Step 1: milestone golden** — `TestFluentButton`: 200×80 frame; `theme.SetActive(theme.FluentLight())` (defer reset). Compose from tokens ONLY (no literal colors): Card-background Border (CornerRadius, Card color) filling the frame inset 8; inside, a button composite: Border with ControlCornerRadius, Accent background, padding {PaddingL, PaddingM}, child TextBlock "Accept" (AccentText... TextBlock default would be TextPrimary — SetColor(th.Color.AccentText) is a TOKEN reference, allowed) — laid out via StackPanel/alignment, measured/arranged/rendered through core. `-update`, READ fluent_button.png: light card, blue rounded button, white centered label, crisp. Plain run passes; ALL prior goldens untouched.
- [ ] **Step 2: gallery** — token-driven restyle + toggle:
  - All chrome colors/paddings/radii from `theme.Active()` (build the tree in a `buildUI(th *theme.Theme, ...) core.Widget` function).
  - Theme toggle: pressing T (`input.Key('T')` == 84 — add const `input.KeyT Key = 84`... controller decision: add `KeyT` to input/events.go consts with doc) toggles Light/Dark: `theme.SetActive(next)`, rebuild via buildUI, `ctx.Input.SetRoot(newRoot)` (SetRoot resets input state by design). Root KeyHandler: make the ROOT DockPanel wrapper a tiny gallery-local type implementing input.KeyHandler for the toggle.
  - Demo Fluent button in the content column (above the swatches): gallery-local `button` type — Border-like composite: token Accent/AccentHover/AccentPressed fills by state (hover via Enter/Leave, pressed via Press/Release with capture), ControlCornerRadius, PaddingL/PaddingM, label TextBlock, click increments a counter TextBlock beside it ("Clicked N times"). This is the LIVE milestone.
- [ ] **Step 3: live verify BOTH themes** — launch (timeout 60, harness background), screenshot `fluo-gallery-p4-dark.png`, READ (dark chrome + blue button + counter text). Then... toggling needs a keypress — send via WSL? Simplest: launch a SECOND run with an env var (`FLUO_THEME=light go run ...` — buildUI reads os.Getenv at startup ONLY for this manual verification; document the env var in main.go as a dev convenience). Screenshot `fluo-gallery-p4-light.png`, READ (light chrome, dark text, accent button). Kill both; confirm dead.
- [ ] **Step 4: docs** — ROADMAP: tick all four Phase 4 boxes + the MILESTONE line. README: theming paragraph (tokens, FluentLight/Dark, rebuild-to-retheme contract, FLUO_THEME dev var).
- [ ] **Step 5:** full suite+vet+gofmt; commit `feat(gallery,theme): themed gallery with toggle + Fluent button milestone; complete Phase 4`

---

## Self-review notes

- Phase 4 ROADMAP coverage: resource model→1, Light→1, Dark→1, wire widgets→2, milestone→3.
- Goldens: Task 1-2 touch no rendering; Task 3 adds one NEW golden only.
- KeyT addition is the only `input` change (const + doc, value 84 = GLFW's key code for T).
- The gallery `button` composite intentionally previews Phase 5's Button semantics (press-capture-release-inside) — its logic seeds the future `Clickable` helper noted in the ledger.
