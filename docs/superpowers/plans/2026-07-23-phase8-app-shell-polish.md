# fluo Phase 8: App Shell & Polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish fluo to a publishable v0.1: chain-aware popup forwarding (closes the Phase-7 menu gap), a frame-driven animation system wired into control state transitions, an embeddable host API (drive fluo from a caller-owned GL loop), a custom Fluent titlebar + window chrome, a translucent acrylic/mica surface (best-effort backdrop blur), a high-DPI audit, package docs + example programs, and publish-readiness prep. **The actual GitHub repo creation / push / tag is OUT OF SCOPE for the subagents — the final task prepares everything and STOPS; the operator performs the publish.**

**Architecture:** Animation is a `anim` package built on `timers.Queue` (tween + easing; controls opt in by animating their state colors). Embeddable host splits `app.Run`'s glfw-owning loop from a `Surface` that renders+routes into a caller-provided context. Titlebar is an undecorated-glfw window + a `TitleBar` widget with drag/min/max/close hit regions. Acrylic adds a mid-frame framebuffer snapshot+blur to `render/gl`, exposed via a new `Renderer.DrawBackdropBlur` (interface addition — done now while `*gl.Renderer` is the only impl). Chain-aware forwarding generalizes OverlayHost to walk the whole popup stack for outside-press dismissal and hover delivery.

**Tech Stack:** Pinned deps. `anim` GL-free. GL work in `render/gl` + `app`.

## Global Constraints

- All prior constraints bind (go.mod PINNED; WSL-only Go, GOTOOLCHAIN=local; keyed literals; vet+gofmt gates; doc comments; per-task commits + trailer `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`; existing goldens never `-update`d; tokens only; controls idiom).
- No glfw types outside `app`. New goldens: light theme + defer SetActive(nil).
- Interface additions to `render.Renderer` are allowed this phase (single impl); every addition gets a `*gl.Renderer` implementation + the `var _ render.Renderer` assertion stays green.
- Declared v0 cuts: acrylic backdrop-blur is best-effort (degrades to tinted translucency if blur is impractical — document which shipped); titlebar is Windows/Linux (glfw undecorated) — macOS traffic-lights not handled; animation is opt-in per control, no global animation clock config; high-DPI audit is code-audit + scale-parameterized tests (a live 2x display isn't available in WSLg — document the residual manual-verification gap); IME + accessibility remain stretch/deferred.

## File Structure

```
controls/overlay.go        chain-aware forwarding (whole-stack outside-press + hover)
anim/anim.go               Tween, Easing, Animator (timers-driven)
anim/anim_test.go
controls/animation.go      colorAnim helper; wire into Button/CheckBox/etc. state fills
app/surface.go             Surface (embeddable: render+route into a caller context)
app/window.go              Run refactored onto Surface; undecorated-window option
controls/titlebar.go       TitleBar widget (drag region + min/max/close buttons)
render/renderer.go         + DrawBackdropBlur (interface)
render/gl/blur.go          framebuffer snapshot + separable blur
render/gl/renderer.go       DrawBackdropBlur impl
controls/acrylic.go        AcrylicSurface widget
doc.go + per-package doc.go package documentation
examples/                  minimal example programs (counter, form, todo)
cmd/fluo-gallery/main.go   titlebar + acrylic + animated controls + Advanced menu slide
CHANGELOG.md, README.md, ROADMAP.md
render/gl/renderer_test.go + goldens: acrylic.png (best-effort)
```

---

### Task 1: chain-aware popup forwarding (Phase-7 backlog)

**Files:** `controls/overlay.go`(+tests)

Normative: replace "forwarding/dismiss targets only the topmost popup" with whole-chain awareness. (a) An outside Press (position inside NO popup in the stack) closes ALL popups top-down (each fires onDismiss) + swallows. (b) A Press inside popup at stack level k (not topmost) closes popups above k, then delivers to level k. (c) Hover synthesis + Move forwarding target the popup whose bounds contain the pointer (search stack top-down), not just `[len-1]`. Preserve: modal capture engaged while ≥1 modal popup open; non-modal (tooltip) unaffected; the gen-guarded reentrancy latch. Update the pinned Phase-7 menu tests (`TestMenuOutsidePressWithSubmenuClosesOnlyTopmost` → now closes all; `TestMenuSiblingRowUnreachableWhileSubmenuOpen` → now reachable, submenu auto-closes on sibling hover) to the NEW behavior + add File→Edit-slide equivalent (open menu A, hover bar entry B → A closes, B opens... that's MenuBar-level, keep in menu_test if the bar drives it; here just prove overlay delivers to the containing popup).
- [ ] TDD the (a)/(b)/(c) semantics with probe popups on a real Router; update menu tests; verify combo/dialog/tooltip tests still pass (their single-popup / scrim / non-modal cases are unaffected). Golden untouched (menu_open.png is a static open state). Commit `fix(controls): chain-aware popup forwarding and dismissal`

### Task 2: animation system

**Files:** `anim/anim.go`(+tests), `controls/animation.go`(+tests)

**Produces (exact):**
```go
package anim
type Easing func(t float32) float32   // t,return in [0,1]
func Linear(t float32) float32
func EaseOut(t float32) float32        // cubic ease-out
func EaseInOut(t float32) float32
// Tween interpolates 0→1 over duration on a timers.Queue; calls onUpdate(v) each
// advance and onDone once. Not goroutine-safe.
type Tween struct{ /* ... */ }
func NewTween(q *timers.Queue, d time.Duration, ease Easing, onUpdate func(float32), onDone func()) *Tween
func (t *Tween) Stop()                 // idempotent; halts, no onDone
```
`controls/animation.go`: `colorAnim` helper interpolates render.Color A→B over a tween (lerp per channel); a control embeds it to smoothly cross-fade hover/press fills instead of snapping. Wire into Button (hover/press fill) behind an opt-in `SetAnimated(bool)` (default OFF so all existing goldens/tests are byte-identical — the neutral rest-state color is unchanged; verify goldens untouched). Document: animation needs a timers.Queue via the existing SetTimers-style wiring; without it, instant (current behavior).
- [ ] TDD: easing curves (endpoints 0→0,1→1; EaseOut monotonic, EaseOut(0.5)>0.5); Tween advance interpolation (fake clock: half-duration → ~ease(0.5)); Stop idempotent+no onDone; colorAnim channel lerp; Button SetAnimated cross-fade (advance mid-tween → fill between rest and hover); default-off leaves color logic identical. Commit `feat(anim,controls): tween/easing animation; opt-in animated buttons`

### Task 3: embeddable Surface host

**Files:** `app/surface.go`, `app/window.go`(refactor)(+tests where headless-possible)

**Produces (exact):**
```go
// Surface renders a fluo widget tree and routes input, into a caller-owned GL
// context. The caller owns the glfw window, GL context, and frame loop; Surface
// owns layout, rendering, the Router, and a Timers queue.
func NewSurface() *Surface
func (s *Surface) SetRoot(w core.Widget) *Surface
func (s *Surface) Router() *input.Router
func (s *Surface) Timers() *timers.Queue
// Frame draws one frame: advances timers, (re)lays out if size changed or dirty,
// renders via r. fbW/fbH device px; winW/winH logical px; scale = fbW/winW.
func (s *Surface) Frame(r render.Renderer, winW, winH, fbW, fbH int)
// Input feed (logical px) — caller translates its own events (or app.Run does it):
func (s *Surface) Pointer(...); func (s *Surface) Key(...)  // mirror Router entry points
```
`app.Run` is refactored to construct a `Surface` internally and feed glfw events into it — behavior identical (Ctx unchanged; existing demo/gallery untouched). Surface makes fluo usable inside someone else's engine loop.
- [ ] TDD (headless where possible: a mock render.Renderer + synthetic sizes/events assert Frame lays out + routes; the glfw parts stay in app.Run and are covered by the gallery live-run). Build+gallery still work. Commit `feat(app): embeddable Surface host; Run refactored onto it`

### Task 4: custom Fluent titlebar + window chrome

**Files:** `controls/titlebar.go`(+tests), `app/window.go`(undecorated option)

**Produces:**
```go
func NewTitleBar(face *text.Face, title string) *TitleBar
func (t *TitleBar) SetTitle(s string) *TitleBar
func (t *TitleBar) OnMinimize(fn func()) *TitleBar
func (t *TitleBar) OnMaximize(fn func()) *TitleBar
func (t *TitleBar) OnClose(fn func()) *TitleBar
// DragRegion reports whether point p (logical, within the bar) is in the draggable
// area (not over a caption button) — the host uses this to move the window.
func (t *TitleBar) DragRegion(p render.Point) bool
```
Normative: full-width bar, height ~32; title left (BodySize TextPrimary), 3 caption buttons right (min/max/close — glyphs: "–","☐","✕" or simple drawn shapes; hover ControlFillHover, close-hover a red RGBA(232,17,35,...) token-free accent OK as a documented one-off or add a token; pressed states). `app.Config` gains `Undecorated bool`; when set, `app.Run` uses `glfw.WindowHint(glfw.Decorated, glfw.False)` and: caption button callbacks wired to `win.Iconify`/maximize-toggle/`win.SetShouldClose`; dragging in a DragRegion moves the window via cursor-delta + `win.SetPos`. Document the WSLg/X11 drag approach.
- [ ] TDD headless: DragRegion math (over button → false, over title → true); caption callbacks fire on click (router); layout. Live-verify the undecorated window + drag + buttons via the gallery (Task 8). Golden `titlebar.png` (light, 300x40: title + 3 buttons, close hovered). Commit `feat(controls,app): custom Fluent titlebar and undecorated window`

### Task 5: acrylic/mica surface (best-effort backdrop blur)

**Files:** `render/renderer.go`(+DrawBackdropBlur), `render/gl/blur.go`, `render/gl/renderer.go`, `controls/acrylic.go`(+tests), golden `acrylic.png`

Normative: `render.Renderer` gains `DrawBackdropBlur(r render.Rect, radius float32, tint render.Color)`. gl impl: snapshot the current framebuffer region under `r` to a texture, separable-gaussian blur it (a few taps), draw it into `r` (rounded by radius) with `tint` composited over — approximating WinUI acrylic. If a true mid-frame snapshot proves impractical in the batched pipeline within reasonable effort, DEGRADE: implement DrawBackdropBlur as a tinted translucent FillRoundedRect over the window background (no real blur) and document the degrade clearly — the interface + AcrylicSurface widget ship either way. `controls.AcrylicSurface` = a Border-like container that calls DrawBackdropBlur for its background then renders children; tint from a new `theme.Color.AcrylicTint` token (dark: RGBA(32,32,36,180); light: RGBA(243,243,243,180)).
- [ ] TDD: AcrylicSurface layout (Border-like, delegates child measure/arrange); the renderer path is golden-only. Golden `acrylic.png` (light, 200x120: an acrylic panel over some colored blocks — blurred-or-tinted per what shipped; READ + document which). Commit `feat(render,controls): backdrop-blur acrylic surface (or documented tint fallback)`

### Task 6: high-DPI audit

**Files:** audit across `render/gl`, `controls`, `app`; `app/surface_test.go` or a dedicated `controls/dpi_test.go`

Normative: no live 2x display in WSLg — do a CODE audit + scale-parameterized tests. (a) Confirm the renderer's `scale` (fbW/winW) flows to every device-px conversion (scissor clip, SDF text quads, line widths) — grep for hardcoded 1:1 assumptions. (b) A test that lays out + hit-tests a small tree and asserts logical coordinates are scale-independent (hit at logical (x,y) selects the same widget whether the harness pretends scale=1 or scale=2 — since layout/hit-test are pure logical, this should hold trivially; the test PINS that invariant). (c) Render one existing golden scenario through the FBO harness at a 2x framebuffer with scale=2 and assert it produces a 2x-resolution image of the same layout (new golden `text_2x.png` — proves SDF text + clip scale correctly). Document the residual: real per-monitor DPI-change events untested (no hardware).
- [ ] TDD per above; fix any hardcoded-scale bug found (report if none). Commit `test: high-DPI audit and scale-parameterized rendering`

### Task 7: docs + examples

**Files:** `doc.go`, per-package `doc.go` (render/text/core/input/controls/theme/bind/anim/app/timers), `examples/counter/main.go`, `examples/form/main.go`, `examples/todo/main.go`, `README.md`, `CHANGELOG.md`

Normative: a top-level `doc.go` with a package overview (what fluo is, quickstart, the architecture layers); one-paragraph `doc.go` per package summarizing its role + the key types; three small example programs (counter = button+label+binding; form = TextBox/CheckBox/Slider bound to a model; todo = List+ListView+add/remove) each runnable (`go run ./examples/todo`) and each ≤120 lines as consumer-facing reference code; README expanded to a real project README (features, install, quickstart, architecture, examples, status, license); CHANGELOG.md with a v0.1.0 unreleased section listing the phase deliverables.
- [ ] Build all examples (they don't need live-run in CI, but must compile + vet + gofmt). `go doc ./...` sanity. Commit `docs: package documentation, example programs, README, CHANGELOG`

### Task 8: gallery polish + publish prep (STOP before publish)

**Files:** `cmd/fluo-gallery/main.go`, `README.md`, `ROADMAP.md`, `go.mod` (verify only)

- Gallery: opt into the undecorated titlebar (app.Config.Undecorated + a TitleBar wired to the window controls), wrap the content in an AcrylicSurface, enable SetAnimated on the demo buttons, and confirm the Advanced menu now slides File→Edit (chain-aware forwarding). Live-verify via winshot `fluo-gallery-p8.png` (READ: titlebar with caption buttons, acrylic panel, controls). Kill+confirm.
- Publish-readiness (DO NOT create the repo / push / tag): verify `go.mod` module path `github.com/0xdreadnaught/fluo` + go 1.23; LICENSE present (MIT, 0xdreadnaught); every package has a doc.go; `go vet ./...`, `gofmt -l .` clean; full `go test ./...` green; `go build ./...` clean; the golden suite passes. Write a `docs/superpowers/RELEASE-CHECKLIST.md` enumerating the exact operator steps to publish (create github repo, `git remote add origin`, `git push -u origin main`, `git tag v0.1.0`, `git push --tags`, optional `go list -m` proof) — but perform NONE of them.
- Docs: ROADMAP tick ALL Phase 8 boxes + mark the project v0.1-ready; README status → "v0.1 ready to publish".
- [ ] Commit `feat(gallery): titlebar+acrylic+animation; v0.1 publish prep; complete Phase 8`. Then STOP — report the release checklist to the operator for the actual publish.

---

## Self-review notes
- ROADMAP Phase 8 coverage: embedding API→3, titlebar→4, acrylic/mica→5, animation→2, high-DPI→6, docs/examples→7, v0.1 publish PREP→8 (publish itself = operator), IME/a11y remain stretch (not built). Phase-7 menu backlog→1.
- Interface addition (DrawBackdropBlur) is done while *gl.Renderer is the only impl.
- Every risky GL/glfw task (5 titlebar-drag, 5 acrylic) has an explicit documented degrade path so the phase completes even if the ideal proves impractical.
- The publish gate is honored: Task 8 prepares and STOPS; no outward-facing action is taken by any subagent.
