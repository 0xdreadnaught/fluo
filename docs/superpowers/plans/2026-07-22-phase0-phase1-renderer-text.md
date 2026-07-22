# fluo Phase 0 + Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Scaffold the fluo repo and build the entire visual bottom layer: a batched GL renderer (rects, rounded rects, strokes, shadows, images, clipping), SDF text, and a minimal demo host window.

**Architecture:** Pure-Go `render` package defines the `Renderer` interface and primitives; `render/gl` implements it with one uber-shader batching all draw modes; `text` rasterizes TTF outlines → SDF atlas → glyph quads consumed through the interface; `app` is a thin glfw host loop. Only `render/gl` and `app` touch GL/glfw.

**Tech Stack:** Go 1.23, OpenGL 3.3 core via `github.com/go-gl/gl/v3.3-core/gl`, `github.com/go-gl/glfw/v3.3/glfw`, `golang.org/x/image` (sfnt outlines, vector rasterizer, gofont test fonts).

## Global Constraints

- Module path: `github.com/0xdreadnaught/fluo` — exact, everywhere.
- License: MIT, copyright `2026 0xdreadnaught`.
- Go: `go 1.23` in go.mod.
- Allowed deps: `github.com/go-gl/gl`, `github.com/go-gl/glfw/v3.3/glfw`, `golang.org/x/image`. Nothing else without a spec change.
- GL profile: 3.3 core, forward-compatible hint. No other GL versions.
- Packages `render`, `text` must NOT import any GL or glfw package (headless-testable). Only `render/gl` (+ its `gltest`) and `app` may.
- All public API coordinates are **logical pixels, y-down, origin top-left**. The renderer maps logical→device px via the `scale` passed to `Begin`.
- GL tests auto-skip when a context can't be created (`t.Skipf`), never fail CI for lack of GPU.
- Golden tests: regenerate with `go test ./render/gl/... -run TestName -update`, visually inspect the PNG, commit it. Tolerance ±3 per channel.
- Windows dev box: GL tests need a real session; run them locally, not in CI.
- Commit at the end of every task (small, one-concern commits).

## File Structure

```
fluo/
├── go.mod, LICENSE, README.md, .gitignore
├── .github/workflows/ci.yml
├── ROADMAP.md                      (exists — tick boxes as tasks land)
├── render/
│   ├── primitives.go               Color, Point, Size, Rect, Thickness + math
│   ├── primitives_test.go
│   └── renderer.go                 Renderer interface, TextureID, GlyphQuad
├── render/gl/
│   ├── shader.go                   GLSL sources, compile/link helpers
│   ├── renderer.go                 Renderer impl: state, textures, clip stack
│   ├── batch.go                    vertex batching + flush
│   ├── renderer_test.go            golden tests (all primitives)
│   ├── testdata/*.png              goldens
│   └── gltest/gltest.go            hidden-window+FBO harness, CheckGolden
├── text/
│   ├── font.go                     Font: sfnt load, metrics, glyph lookup
│   ├── raster.go                   outline → alpha mask (x/image/vector)
│   ├── sdf.go                      alpha mask → SDF field
│   ├── atlas.go                    shelf-packed SDF atlas, lazy GPU upload
│   ├── face.go                     Face: Measure / LineHeight / Draw
│   └── *_test.go                   headless unit tests per file
├── app/
│   └── window.go                   Run(cfg, frame): glfw loop + input pump
└── cmd/fluo-demo/main.go           visual smoke: card, button, text
```

---

### Task 1: Repo scaffolding + CI

**Files:**
- Create: `go.mod`, `LICENSE`, `README.md`, `.gitignore`, `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: nothing.
- Produces: buildable module `github.com/0xdreadnaught/fluo`; CI running `go vet` + `go test ./...` on ubuntu.

- [ ] **Step 1: go.mod**

```
module github.com/0xdreadnaught/fluo

go 1.23
```

(Deps get added by `go get` in later tasks — do not pre-add.)

- [ ] **Step 2: LICENSE** — standard MIT text, header line: `Copyright (c) 2026 0xdreadnaught`

- [ ] **Step 3: README.md**

```markdown
# fluo

A retained-mode, Fluent/WinUI-styled GUI toolkit for OpenGL apps in Go.
Pre-alpha — see [ROADMAP.md](ROADMAP.md) and `docs/superpowers/specs/` for the design.
```

- [ ] **Step 4: .gitignore**

```
*.exe
*.got.png
```

- [ ] **Step 5: CI** — `.github/workflows/ci.yml`

```yaml
name: ci
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.23' }
      - run: sudo apt-get update && sudo apt-get install -y libgl1-mesa-dev xorg-dev
      - run: go vet ./...
      - run: go test ./...
```

(GL golden tests skip themselves headless; apt deps are for cgo *compilation* of glfw.)

- [ ] **Step 6: Verify + commit**

Run: `go build ./...` → no output, exit 0.
```bash
git add -A && git commit -m "chore: scaffold module, MIT license, CI"
```

---

### Task 2: render primitives + Renderer interface

**Files:**
- Create: `render/primitives.go`, `render/renderer.go`
- Test: `render/primitives_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces (used by every later task — exact):

```go
package render

type Color struct{ R, G, B, A float32 }              // 0..1, straight alpha
func RGB(r, g, b uint8) Color                        // A=1
func RGBA(r, g, b, a uint8) Color
type Point struct{ X, Y float32 }
type Size struct{ W, H float32 }
type Rect struct{ X, Y, W, H float32 }               // top-left + size
func (r Rect) Right() float32
func (r Rect) Bottom() float32
func (r Rect) Contains(p Point) bool                 // [X,Right) × [Y,Bottom)
func (r Rect) Intersect(o Rect) Rect                 // empty (W/H=0) if disjoint
func (r Rect) Inflate(d float32) Rect                // grow all sides by d
func (r Rect) Empty() bool                           // W<=0 || H<=0
type Thickness struct{ Left, Top, Right, Bottom float32 }
func Uniform(v float32) Thickness
func (r Rect) Inset(t Thickness) Rect

type TextureID uint32                                // 0 = no texture
type GlyphQuad struct{ Dst, Src Rect }               // Dst logical px, Src UV 0..1

type Renderer interface {
    Begin(fbWidth, fbHeight int, scale float32)      // start frame; fb size in DEVICE px
    End()                                            // flush frame
    FillRect(r Rect, c Color)
    FillRoundedRect(r Rect, radius float32, c Color)
    StrokeRoundedRect(r Rect, radius, width float32, c Color) // stroke inside edge
    DrawShadow(r Rect, radius, blur float32, c Color)         // soft shadow of rounded rect
    CreateTexture(w, h int, rgba []byte) TextureID            // RGBA8, len=w*h*4
    UpdateTexture(id TextureID, x, y, w, h int, rgba []byte)
    DrawQuad(dst, src Rect, tex TextureID, tint Color)        // src in UV 0..1
    DrawSDFQuads(quads []GlyphQuad, tex TextureID, c Color)   // tex is an SDF alpha atlas
    PushClip(r Rect)                                          // intersects with current
    PopClip()
}
```

- [ ] **Step 1: Write failing tests** — `render/primitives_test.go`, table tests:

```go
func TestRectContains(t *testing.T) {
    r := Rect{10, 10, 20, 20}
    for _, tc := range []struct{ p Point; want bool }{
        {Point{10, 10}, true}, {Point{29.9, 29.9}, true},
        {Point{30, 30}, false}, {Point{9, 15}, false},
    } {
        if got := r.Contains(tc.p); got != tc.want { t.Errorf("%v: got %v", tc.p, got) }
    }
}
func TestRectIntersect(t *testing.T) {
    a := Rect{0, 0, 10, 10}
    if got := a.Intersect(Rect{5, 5, 10, 10}); got != (Rect{5, 5, 5, 5}) { t.Errorf("got %v", got) }
    if got := a.Intersect(Rect{20, 20, 5, 5}); !got.Empty() { t.Errorf("want empty, got %v", got) }
}
func TestRectInset(t *testing.T) {
    got := Rect{0, 0, 100, 50}.Inset(Thickness{5, 10, 15, 20})
    if got != (Rect{5, 10, 80, 20}) { t.Errorf("got %v", got) }
}
func TestRGB(t *testing.T) {
    c := RGB(255, 0, 128)
    if c.R != 1 || c.A != 1 || c.B < 0.5 || c.B > 0.51 { t.Errorf("got %+v", c) }
}
```

- [ ] **Step 2: Run** `go test ./render/` — expect FAIL (undefined symbols).
- [ ] **Step 3: Implement** `primitives.go` (straightforward math; `Intersect` clamps to max(left), min(right) and returns `Rect{}` when W or H ≤ 0) and `renderer.go` (the interface block above, verbatim).
- [ ] **Step 4: Run** `go test ./render/` — expect PASS. Also `go vet ./...`.
- [ ] **Step 5: Commit** `feat(render): primitives and Renderer interface`

---

### Task 3: GL test harness (hidden window + FBO + goldens)

**Files:**
- Create: `render/gl/gltest/gltest.go`
- Test: `render/gl/gltest/gltest_test.go`, golden `render/gl/gltest/testdata/clear.png`

**Interfaces:**
- Consumes: nothing from fluo.
- Produces (used by Tasks 4–8, 11):

```go
package gltest

// Run creates a hidden 3.3-core context + wxh offscreen FBO, calls frame on the
// locked OS thread, then tears everything down. Skips the test if no GL.
func Run(t *testing.T, w, h int, frame func(fb *Framebuffer))
type Framebuffer struct{ W, H int; FBO, Tex uint32 }
func (f *Framebuffer) Image() *image.RGBA            // ReadPixels, y-flipped to image space
// CheckGolden compares got to testdata/<name>.png (±3/channel). -update writes it.
// On mismatch writes testdata/<name>.got.png and fails.
func CheckGolden(t *testing.T, name string, got *image.RGBA)
```

- [ ] **Step 1: deps** — `go get github.com/go-gl/gl/v3.3-core/gl github.com/go-gl/glfw/v3.3/glfw`
- [ ] **Step 2: failing test** — `gltest_test.go`:

```go
func TestClearGolden(t *testing.T) {
    gltest.Run(t, 64, 64, func(fb *gltest.Framebuffer) {
        gl.ClearColor(1, 0.5, 0, 1)
        gl.Clear(gl.COLOR_BUFFER_BIT)
        gltest.CheckGolden(t, "clear", fb.Image())
    })
}
```

- [ ] **Step 3: implement** — key parts of `gltest.go`:

```go
var update = flag.Bool("update", false, "rewrite golden images")

func Run(t *testing.T, w, h int, frame func(fb *Framebuffer)) {
    t.Helper()
    runtime.LockOSThread() // GL is thread-affine; tests must not use t.Parallel()
    if err := glfw.Init(); err != nil { t.Skipf("no GL context available: %v", err) }
    defer glfw.Terminate()
    glfw.WindowHint(glfw.ContextVersionMajor, 3)
    glfw.WindowHint(glfw.ContextVersionMinor, 3)
    glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
    glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
    glfw.WindowHint(glfw.Visible, glfw.False)
    win, err := glfw.CreateWindow(64, 64, "gltest", nil, nil)
    if err != nil { t.Skipf("no GL window: %v", err) }
    defer win.Destroy()
    win.MakeContextCurrent()
    if err := gl.Init(); err != nil { t.Fatalf("gl.Init: %v", err) }
    // FBO with RGBA8 color attachment, w x h
    var fbo, tex uint32
    gl.GenFramebuffers(1, &fbo); gl.BindFramebuffer(gl.FRAMEBUFFER, fbo)
    gl.GenTextures(1, &tex); gl.BindTexture(gl.TEXTURE_2D, tex)
    gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, int32(w), int32(h), 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)
    gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
    gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, tex, 0)
    if gl.CheckFramebufferStatus(gl.FRAMEBUFFER) != gl.FRAMEBUFFER_COMPLETE { t.Fatal("FBO incomplete") }
    gl.Viewport(0, 0, int32(w), int32(h))
    frame(&Framebuffer{W: w, H: h, FBO: fbo, Tex: tex})
}
```

`Image()`: `gl.ReadPixels(0,0,W,H,RGBA,UNSIGNED_BYTE,ptr)` into a buffer, then copy rows bottom-up into `image.RGBA` (GL origin is bottom-left).
`CheckGolden`: if `*update` → `os.MkdirAll("testdata")` + encode PNG, log "golden written", return. Else decode `testdata/<name>.png` (missing → `t.Fatalf("golden missing: run with -update")`), compare dims + every channel with `abs(a-b)<=3`; on failure encode `<name>.got.png` and `t.Errorf` with first-diff coordinates.

- [ ] **Step 4: generate + verify** — `go test ./render/gl/gltest/ -update` (writes clear.png, inspect: solid orange 64×64), then `go test ./render/gl/gltest/` → PASS.
- [ ] **Step 5: Commit** (including `testdata/clear.png`) `feat(render/gl): offscreen golden-image test harness`

---

### Task 4: uber-shader + batching + FillRect

**Files:**
- Create: `render/gl/shader.go`, `render/gl/batch.go`, `render/gl/renderer.go`
- Test: `render/gl/renderer_test.go` (+ golden `fill_rect.png`)

**Interfaces:**
- Consumes: `render.Renderer` (Task 2), `gltest` (Task 3).
- Produces: `func New() (*Renderer, error)` in package `gl` (import path `github.com/0xdreadnaught/fluo/render/gl`), a `*Renderer` satisfying `render.Renderer`. All later renderer tasks fill in methods on this type.

Design (fixed now, reused by Tasks 5–8):
- One shader program, one VAO/VBO, interleaved vertices of **15 float32**: `pos(2) uv(2) color(4) rect(4: center.xy, halfSize.xy) extra(2: radius, width|blur) mode(1)`.
- Modes: `0` solid, `1` texture, `2` SDF text, `3` rounded fill, `4` rounded stroke, `5` shadow.
- Batch flushes when: texture binding changes, clip changes, buffer full (16384 verts), or `End()`.
- A 1×1 white texture stays bound for non-texture modes so the sampler is always valid.
- Blending: `gl.Enable(gl.BLEND); gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)`; depth test off.

- [ ] **Step 1: failing golden test**

```go
func testFrame(t *testing.T, name string, w, h int, draw func(r *glr.Renderer)) {
    gltest.Run(t, w, h, func(fb *gltest.Framebuffer) {
        r, err := glr.New()
        if err != nil { t.Fatalf("New: %v", err) }
        gl.ClearColor(0.12, 0.12, 0.14, 1); gl.Clear(gl.COLOR_BUFFER_BIT)
        r.Begin(fb.W, fb.H, 1)
        draw(r)
        r.End()
        gltest.CheckGolden(t, name, fb.Image())
    })
}
func TestFillRect(t *testing.T) {
    testFrame(t, "fill_rect", 128, 96, func(r *glr.Renderer) {
        r.FillRect(render.Rect{8, 8, 60, 40}, render.RGB(0, 120, 215))
        r.FillRect(render.Rect{40, 30, 60, 40}, render.RGBA(255, 255, 255, 128)) // blend check
    })
}
```

Run: `go test ./render/gl/ -run TestFillRect` → FAIL (package doesn't compile / New undefined).

- [ ] **Step 2: shader.go** — full GLSL now (all modes; later tasks only add Go methods):

```glsl
// vertex
#version 330 core
layout(location=0) in vec2 aPos;   // logical px
layout(location=1) in vec2 aUV;
layout(location=2) in vec4 aColor;
layout(location=3) in vec4 aRect;  // center.xy, halfSize.xy (logical px)
layout(location=4) in vec2 aExtra; // x: radius, y: strokeWidth|blur
layout(location=5) in float aMode;
uniform vec2 uViewport;            // device px
uniform float uScale;
out vec2 vUV; out vec4 vColor; out vec4 vRect; out vec2 vExtra; out vec2 vPos;
flat out int vMode;
void main(){
    vec2 p = aPos * uScale;
    gl_Position = vec4(2.0*p.x/uViewport.x - 1.0, 1.0 - 2.0*p.y/uViewport.y, 0.0, 1.0);
    vUV=aUV; vColor=aColor; vRect=aRect; vExtra=aExtra; vPos=aPos; vMode=int(aMode);
}
```

```glsl
// fragment
#version 330 core
in vec2 vUV; in vec4 vColor; in vec4 vRect; in vec2 vExtra; in vec2 vPos;
flat in int vMode;
uniform sampler2D uTex;
out vec4 frag;
float sdRoundBox(vec2 p, vec2 b, float r){
    vec2 q = abs(p) - b + vec2(r);
    return length(max(q, vec2(0.0))) + min(max(q.x, q.y), 0.0) - r;
}
void main(){
    vec4 c = vColor;
    if (vMode == 1) {
        c *= texture(uTex, vUV);
    } else if (vMode == 2) {
        float s = texture(uTex, vUV).r;
        float w = fwidth(s);
        c.a *= smoothstep(0.5 - w, 0.5 + w, s);
    } else if (vMode >= 3) {
        float d = sdRoundBox(vPos - vRect.xy, vRect.zw, vExtra.x);
        if (vMode == 3) c.a *= clamp(0.5 - d, 0.0, 1.0);
        else if (vMode == 4) { float w = vExtra.y; c.a *= clamp(0.5 - (abs(d + w*0.5) - w*0.5), 0.0, 1.0); }
        else c.a *= 1.0 - smoothstep(-vExtra.y, vExtra.y, d);
    }
    if (c.a <= 0.001) discard;
    frag = c;
}
```

Plus `compile(src string, kind uint32) (uint32, error)` and `link(vs, fs uint32) (uint32, error)` with info-log error returns.

- [ ] **Step 3: batch.go + renderer.go**

```go
const (
    floatsPerVert = 15
    maxVerts      = 16384
)
type Renderer struct {
    prog, vao, vbo    uint32
    locViewport, locScale int32
    verts             []float32
    curTex, whiteTex  uint32
    fbW, fbH          int
    scale             float32
    clips             []render.Rect
}
```

`New()`: compile+link program, get uniform locations, create VAO/VBO (dynamic draw, maxVerts*floatsPerVert*4 bytes), set the 6 attrib pointers with stride 60, create whiteTex (1×1 RGBA {255,255,255,255}).
`Begin(w,h,scale)`: store; `gl.UseProgram`; set uniforms; enable blend, disable depth+scissor; `curTex = whiteTex`; `verts = verts[:0]`; `clips = clips[:0]`.
`End()`: flush.
`flush()`: if len(verts)==0 return; bind VAO/VBO, `BufferSubData`, bind curTex, `DrawArrays(TRIANGLES)`, reset slice.
`vert(...)` appends 15 floats; `quad(mode float32, dst, src render.Rect, c render.Color, rc [4]float32, ex [2]float32, tex uint32)`: if tex != curTex or full → flush + set curTex; emit 2 triangles (6 verts) with per-corner pos/uv.
`FillRect(r, c)`: `quad(0, r, render.Rect{}, c, [4]float32{}, [2]float32{}, rd.whiteTex)`.
Stub every other interface method with `panic("not implemented")` for now so the type satisfies `render.Renderer` (each later task replaces one panic; `var _ render.Renderer = (*Renderer)(nil)` compile check).

- [ ] **Step 4: generate + inspect + pass** — `-update`, inspect `fill_rect.png` (blue rect, translucent white overlap on dark gray), then plain run → PASS.
- [ ] **Step 5: Commit** `feat(render/gl): uber-shader, vertex batching, FillRect`

---

### Task 5: clip stack

**Files:**
- Modify: `render/gl/renderer.go` (replace PushClip/PopClip panics)
- Test: golden `clip.png` in `render/gl/renderer_test.go`

**Interfaces:** Consumes Task 4's batcher. Produces working `PushClip/PopClip`.

- [ ] **Step 1: failing golden test** — draw a big rect clipped to a window:

```go
func TestClip(t *testing.T) {
    testFrame(t, "clip", 128, 96, func(r *glr.Renderer) {
        r.PushClip(render.Rect{20, 20, 50, 30})
        r.FillRect(render.Rect{0, 0, 128, 96}, render.RGB(0, 120, 215)) // fills only the clip
        r.PushClip(render.Rect{0, 0, 40, 96})                            // nested: intersects
        r.FillRect(render.Rect{0, 0, 128, 96}, render.RGB(255, 185, 0))
        r.PopClip()
        r.PopClip()
        r.FillRect(render.Rect{100, 70, 40, 40}, render.RGB(16, 124, 16)) // unclipped again
    })
}
```

- [ ] **Step 2: implement**

```go
func (rd *Renderer) PushClip(r render.Rect) {
    rd.flush()
    if n := len(rd.clips); n > 0 { r = r.Intersect(rd.clips[n-1]) }
    rd.clips = append(rd.clips, r)
    rd.applyClip()
}
func (rd *Renderer) PopClip() {
    rd.flush()
    rd.clips = rd.clips[:len(rd.clips)-1]
    rd.applyClip()
}
func (rd *Renderer) applyClip() {
    if len(rd.clips) == 0 { gl.Disable(gl.SCISSOR_TEST); return }
    c := rd.clips[len(rd.clips)-1]
    gl.Enable(gl.SCISSOR_TEST)
    x := int32(c.X * rd.scale)
    y := int32(float32(rd.fbH) - (c.Y+c.H)*rd.scale) // GL scissor origin = bottom-left
    gl.Scissor(x, y, int32(c.W*rd.scale), int32(c.H*rd.scale))
}
```

- [ ] **Step 3: generate + inspect + pass** — blue only inside 20,20,50,30; yellow only in the 20..40 x-band of it; green square bottom-right intact.
- [ ] **Step 4: Commit** `feat(render/gl): scissor clip stack`

---

### Task 6: rounded-rect fill + stroke

**Files:**
- Modify: `render/gl/renderer.go`
- Test: goldens `rounded_fill.png`, `rounded_stroke.png`

**Interfaces:** Produces working `FillRoundedRect`, `StrokeRoundedRect`.

- [ ] **Step 1: failing golden tests**

```go
func TestRoundedFill(t *testing.T) {
    testFrame(t, "rounded_fill", 128, 96, func(r *glr.Renderer) {
        r.FillRoundedRect(render.Rect{10, 10, 80, 50}, 8, render.RGB(0, 120, 215))
        r.FillRoundedRect(render.Rect{60, 40, 50, 50}, 25, render.RGB(255, 185, 0)) // circle
    })
}
func TestRoundedStroke(t *testing.T) {
    testFrame(t, "rounded_stroke", 128, 96, func(r *glr.Renderer) {
        r.StrokeRoundedRect(render.Rect{10, 10, 100, 70}, 8, 2, render.RGB(255, 255, 255))
    })
}
```

- [ ] **Step 2: implement** — both are one `quad` call; the SDF work is already in the shader:

```go
func rectParams(r render.Rect) [4]float32 {
    return [4]float32{r.X + r.W/2, r.Y + r.H/2, r.W / 2, r.H / 2}
}
func (rd *Renderer) FillRoundedRect(r render.Rect, radius float32, c render.Color) {
    rd.quad(3, r, render.Rect{}, c, rectParams(r), [2]float32{radius, 0}, rd.whiteTex)
}
func (rd *Renderer) StrokeRoundedRect(r render.Rect, radius, width float32, c render.Color) {
    rd.quad(4, r, render.Rect{}, c, rectParams(r), [2]float32{radius, width}, rd.whiteTex)
}
```

(Radius must be clamped: `radius = min(radius, r.W/2, r.H/2)` — do it in both methods.)

- [ ] **Step 3: generate + inspect + pass** — smooth anti-aliased corners, the 25-radius one reads as a circle, stroke is a crisp 2px inner outline.
- [ ] **Step 4: Commit** `feat(render/gl): SDF rounded-rect fill and stroke`

---

### Task 7: drop shadow

**Files:**
- Modify: `render/gl/renderer.go`
- Test: golden `shadow.png`

**Interfaces:** Produces working `DrawShadow`.

- [ ] **Step 1: failing golden test**

```go
func TestShadow(t *testing.T) {
    testFrame(t, "shadow", 128, 96, func(r *glr.Renderer) {
        card := render.Rect{24, 20, 80, 56}
        r.DrawShadow(card, 8, 12, render.RGBA(0, 0, 0, 140))
        r.FillRoundedRect(card, 8, render.RGB(243, 243, 243))
    })
}
```

- [ ] **Step 2: implement** — geometry must cover the blur falloff, so inflate the quad; SDF params stay on the *original* rect:

```go
func (rd *Renderer) DrawShadow(r render.Rect, radius, blur float32, c render.Color) {
    rd.quad(5, r.Inflate(blur), render.Rect{}, c, rectParams(r), [2]float32{radius, blur}, rd.whiteTex)
}
```

- [ ] **Step 3: generate + inspect + pass** — light card floating on dark bg with a soft halo, no hard edges.
- [ ] **Step 4: Commit** `feat(render/gl): soft drop shadows`

---

### Task 8: textures + DrawQuad + DrawSDFQuads

**Files:**
- Modify: `render/gl/renderer.go`
- Test: golden `texture.png`

**Interfaces:** Produces working `CreateTexture`, `UpdateTexture`, `DrawQuad`, `DrawSDFQuads` — the complete `render.Renderer` (delete the compile-check panics; keep `var _ render.Renderer = (*Renderer)(nil)`).

- [ ] **Step 1: failing golden test** — procedural 8×8 checkerboard, drawn scaled + tinted + sub-rect:

```go
func TestTexture(t *testing.T) {
    testFrame(t, "texture", 128, 96, func(r *glr.Renderer) {
        px := make([]byte, 8*8*4)
        for y := 0; y < 8; y++ { for x := 0; x < 8; x++ {
            v := byte(255); if (x+y)%2 == 0 { v = 40 }
            i := (y*8 + x) * 4
            px[i], px[i+1], px[i+2], px[i+3] = v, v, v, 255
        }}
        id := r.CreateTexture(8, 8, px)
        r.DrawQuad(render.Rect{8, 8, 48, 48}, render.Rect{0, 0, 1, 1}, id, render.RGB(255, 255, 255))
        r.DrawQuad(render.Rect{64, 8, 48, 48}, render.Rect{0, 0, 0.5, 0.5}, id, render.RGB(0, 120, 215)) // sub-rect + tint
    })
}
```

- [ ] **Step 2: implement**

```go
func (rd *Renderer) CreateTexture(w, h int, rgba []byte) render.TextureID {
    var id uint32
    gl.GenTextures(1, &id)
    gl.BindTexture(gl.TEXTURE_2D, id)
    gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
    gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
    gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
    gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
    var ptr unsafe.Pointer
    if rgba != nil { ptr = gl.Ptr(rgba) }
    gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, int32(w), int32(h), 0, gl.RGBA, gl.UNSIGNED_BYTE, ptr)
    gl.BindTexture(gl.TEXTURE_2D, rd.curTex) // restore batch binding
    return render.TextureID(id)
}
func (rd *Renderer) UpdateTexture(id render.TextureID, x, y, w, h int, rgba []byte) {
    rd.flush()
    gl.BindTexture(gl.TEXTURE_2D, uint32(id))
    gl.TexSubImage2D(gl.TEXTURE_2D, 0, int32(x), int32(y), int32(w), int32(h), gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(rgba))
    gl.BindTexture(gl.TEXTURE_2D, rd.curTex)
}
func (rd *Renderer) DrawQuad(dst, src render.Rect, tex render.TextureID, tint render.Color) {
    rd.quad(1, dst, src, tint, [4]float32{}, [2]float32{}, uint32(tex))
}
func (rd *Renderer) DrawSDFQuads(quads []render.GlyphQuad, tex render.TextureID, c render.Color) {
    for _, q := range quads { rd.quad(2, q.Dst, q.Src, c, [4]float32{}, [2]float32{}, uint32(tex)) }
}
```

- [ ] **Step 3: generate + inspect + pass** — left: full checkerboard; right: 2× zoom of its top-left quarter, blue-tinted. (DrawSDFQuads gets its golden in Task 11 with a real atlas.)
- [ ] **Step 4: Commit** `feat(render/gl): textures, image quads, SDF quad batch — Renderer complete`

---

### Task 9: text — font loading + glyph rasterization

**Files:**
- Create: `text/font.go`, `text/raster.go`
- Test: `text/font_test.go` (headless — no GL)

**Interfaces:**
- Consumes: `golang.org/x/image/font/sfnt`, `golang.org/x/image/vector`, `golang.org/x/image/math/fixed`; test font `golang.org/x/image/font/gofont/goregular` (embedded TTF, no asset files).
- Produces:

```go
package text

type Font struct{ /* sf *sfnt.Font; buf sfnt.Buffer; mu sync.Mutex */ }
func Load(ttf []byte) (*Font, error)
func (f *Font) glyphIndex(r rune) (sfnt.GlyphIndex, bool)
func (f *Font) advance(gi sfnt.GlyphIndex, sizePx float32) float32
func (f *Font) kern(a, b sfnt.GlyphIndex, sizePx float32) float32
func (f *Font) metrics(sizePx float32) (ascent, descent, lineGap float32)
// rasterGlyph renders the outline at sizePx into a padded alpha mask.
// bearingX/bearingY: offset from pen position (baseline) to the mask's top-left, logical px.
func (f *Font) rasterGlyph(gi sfnt.GlyphIndex, sizePx float32, pad int) (mask *image.Alpha, bearingX, bearingY float32, err error)
```

- [ ] **Step 1: dep** — `go get golang.org/x/image`
- [ ] **Step 2: failing tests**

```go
func TestLoadAndMetrics(t *testing.T) {
    f, err := Load(goregular.TTF)
    if err != nil { t.Fatal(err) }
    asc, desc, _ := f.metrics(16)
    if asc <= 8 || asc >= 20 || desc <= 0 { t.Errorf("asc=%v desc=%v", asc, desc) }
    if _, ok := f.glyphIndex('A'); !ok { t.Error("no glyph for A") }
    if a := f.advance(mustGlyph(t, f, 'M'), 16); a <= 4 || a >= 20 { t.Errorf("advance=%v", a) }
}
func TestRasterGlyph(t *testing.T) {
    f, _ := Load(goregular.TTF)
    mask, bx, by, err := f.rasterGlyph(mustGlyph(t, f, 'A'), 48, 4)
    if err != nil { t.Fatal(err) }
    b := mask.Bounds()
    if b.Dx() < 20 || b.Dy() < 30 { t.Errorf("mask too small: %v", b) }
    sum := 0
    for _, a := range mask.Pix { sum += int(a) }
    if sum == 0 { t.Error("mask is empty") }
    if by >= 0 { t.Errorf("bearingY should be negative (above baseline), got %v", by) }
    _ = bx
}
```

- [ ] **Step 3: implement** — `Load` = `sfnt.Parse`. Metrics/advance/kern via `sfnt` calls with `fixed.Int26_6(sizePx * 64)` ppem and `font.HintingNone`, guarded by the mutex (sfnt.Buffer is not concurrent-safe). `rasterGlyph`:

```go
segs, err := f.sf.LoadGlyph(&f.buf, gi, fixed.Int26_6(sizePx*64), nil)
// 1) bounds: min/max over every seg.Args point (26_6 → float32 /64)
// 2) mask size: ceil(bounds) + 2*pad each axis
// 3) rasterize with vector.NewRasterizer(w, h), DrawOp=draw.Src:
//    MoveTo/LineTo/QuadTo/CubeTo with (x - minX + pad, y - minY + pad)
//    (sfnt segment coords are already y-down; no flip needed)
// 4) rast.Draw(mask, mask.Bounds(), image.Opaque, image.Point{})
// 5) bearingX = minX - pad; bearingY = minY - pad  (minY is negative above baseline)
```

- [ ] **Step 4: Run** `go test ./text/` → PASS. Confirm headless: `go vet ./text/` and check imports contain no `go-gl`.
- [ ] **Step 5: Commit** `feat(text): sfnt font loading and outline rasterization`

---

### Task 10: text — SDF generation + atlas packing

**Files:**
- Create: `text/sdf.go`, `text/atlas.go`
- Test: `text/sdf_test.go`, `text/atlas_test.go` (headless)

**Interfaces:**
- Consumes: Task 9 (`rasterGlyph`).
- Produces:

```go
package text

const sdfPad = 8       // raster padding AND sdf spread, px
const sdfRasterPx = 48 // glyphs rasterized once at this size, scaled at draw time

// sdfFromMask converts a coverage mask into an SDF: 128 at the edge,
// >128 inside, <128 outside, saturating at ±sdfPad px.
func sdfFromMask(m *image.Alpha) *image.Alpha

type atlasEntry struct {
    uv                 render.Rect // 0..1 within atlas
    w, h               int         // px in atlas
    bearingX, bearingY float32     // at sdfRasterPx scale
    advance            float32     // at sdfRasterPx scale
}
type Atlas struct{ /* img *image.Alpha (1024×1024), shelf packer state, entries map[sfnt.GlyphIndex]atlasEntry, dirty regions, tex render.TextureID */ }
func NewAtlas(f *Font) *Atlas
func (a *Atlas) glyph(gi sfnt.GlyphIndex) (atlasEntry, error)      // rasterize+SDF+pack on miss
func (a *Atlas) ensureTexture(r render.Renderer) render.TextureID  // create + upload dirty regions
```

- [ ] **Step 1: failing tests**

```go
func TestSDFFromMask(t *testing.T) {
    // synthetic 32×32 mask with a filled 16×16 square centered
    m := image.NewAlpha(image.Rect(0, 0, 32, 32))
    for y := 8; y < 24; y++ { for x := 8; x < 24; x++ { m.SetAlpha(x, y, color.Alpha{255}) } }
    s := sdfFromMask(m)
    if c := s.AlphaAt(16, 16).A; c < 200 { t.Errorf("center=%d, want >200", c) }
    if c := s.AlphaAt(1, 1).A; c > 60 { t.Errorf("far corner=%d, want <60", c) }
    e := s.AlphaAt(8, 16).A // on the edge
    if e < 108 || e > 148 { t.Errorf("edge=%d, want ~128", e) }
}
func TestAtlasPacking(t *testing.T) {
    f, _ := Load(goregular.TTF)
    a := NewAtlas(f)
    seen := map[render.Rect]bool{}
    for _, r := range "AbgQ" {
        gi, _ := f.glyphIndex(r)
        e, err := a.glyph(gi)
        if err != nil { t.Fatal(err) }
        if e.uv.W <= 0 || e.uv.Right() > 1 || e.uv.Bottom() > 1 { t.Errorf("%c: bad uv %v", r, e.uv) }
        if seen[e.uv] { t.Errorf("%c: uv reused", r) }
        seen[e.uv] = true
    }
    gi, _ := f.glyphIndex('A')
    e1, _ := a.glyph(gi)
    e2, _ := a.glyph(gi)
    if e1 != e2 { t.Error("glyph not cached") }
}
```

- [ ] **Step 2: implement sdf.go** — brute force against the edge set (fast enough at 48px, O(pixels × edge-pixels)):

```go
// inside(x,y) = mask alpha >= 128. An edge pixel is an inside pixel with any
// 4-neighbor outside (treat out-of-bounds as outside).
// For each pixel: d = sqrt(min squared distance to any edge pixel), sign = inside ? + : -
// value = clamp(128 + d*128/sdfPad * sign, 0, 255)  → edge ≈ 128, saturates at ±sdfPad
// No edge pixels at all (empty mask) → all zeros.
```

- [ ] **Step 3: implement atlas.go** — shelf packer: cursor `(x, y)`, `rowH`; place entry at cursor if `x+w <= 1024`, else new row (`y += rowH; x = 0`); error `"atlas full"` if `y+h > 1024` (growth is a later-phase concern; document it). `glyph`: on miss → `rasterGlyph(gi, sdfRasterPx, sdfPad)` → `sdfFromMask` → copy into `img`, record dirty rect + entry (bearing/advance from Task 9 calls at `sdfRasterPx`). `ensureTexture`: first call `CreateTexture(1024, 1024, nil)`; then for each dirty rect expand alpha→RGBA bytes (`r=g=b=a=alpha value` — shader reads `.r`) and `UpdateTexture`; clear dirty list.
- [ ] **Step 4: Run** `go test ./text/` → PASS.
- [ ] **Step 5: Commit** `feat(text): SDF glyph fields and shelf-packed atlas`

---

### Task 11: text — Face, Measure, Draw (+ the text golden)

**Files:**
- Create: `text/face.go`
- Test: `text/face_test.go` (headless measure), golden `text.png` in `render/gl/renderer_test.go`

**Interfaces:**
- Consumes: Tasks 9–10; `render.Renderer`.
- Produces (this is what Phase 2's `TextBlock` will consume):

```go
package text

type Face struct{ Font *Font; SizePx float32 /* atlas shared per Font */ }
func NewFace(f *Font, sizePx float32) *Face
func (fa *Face) Measure(s string) render.Size     // width incl. advances+kerning; height = LineHeight (single line for now)
func (fa *Face) LineHeight() float32              // ascent + descent + lineGap
func (fa *Face) Ascent() float32
// Draw renders s with at = TOP-LEFT of the text box (baseline = at.Y + Ascent()).
func (fa *Face) Draw(r render.Renderer, at render.Point, s string, c render.Color)
```

Note: one `Atlas` is shared per `Font` (glyphs are size-independent SDFs); `NewFace` reuses it via a map on `Font`. Scale factor at draw: `k := fa.SizePx / sdfRasterPx` applied to bearings and glyph dims; advances/kerning computed directly at `fa.SizePx`.

- [ ] **Step 1: failing headless tests**

```go
func TestMeasure(t *testing.T) {
    f, _ := Load(goregular.TTF)
    fa := NewFace(f, 16)
    m1, m2 := fa.Measure("M"), fa.Measure("MM")
    if m1.W <= 0 || m2.W <= m1.W { t.Errorf("M=%v MM=%v", m1, m2) }
    if lh := fa.LineHeight(); lh < 16 || lh > 26 { t.Errorf("LineHeight=%v", lh) }
    if fa.Measure("").W != 0 { t.Error("empty string width != 0") }
}
```

- [ ] **Step 2: implement face.go** — Measure: iterate runes, `advance(gi, SizePx)` + `kern(prev, gi, SizePx)`; unknown rune → use `.notdef` glyph index 0. Draw: same walk; per rune get `atlas.glyph(gi)`, emit `render.GlyphQuad{ Dst: Rect{penX + e.bearingX*k, at.Y + Ascent() + e.bearingY*k, float32(e.w)*k, float32(e.h)*k}, Src: e.uv }`; after the walk `r.DrawSDFQuads(quads, atlas.ensureTexture(r), c)`. Skip glyphs with w==0 (spaces) but still advance the pen.
- [ ] **Step 3: Run** `go test ./text/` → PASS.
- [ ] **Step 4: golden** — add to `render/gl/renderer_test.go`:

```go
func TestText(t *testing.T) {
    testFrame(t, "text", 256, 96, func(r *glr.Renderer) {
        f, err := text.Load(goregular.TTF)
        if err != nil { t.Fatal(err) }
        text.NewFace(f, 14).Draw(r, render.Point{8, 8}, "Hello, fluo!", render.RGB(255, 255, 255))
        text.NewFace(f, 28).Draw(r, render.Point{8, 40}, "SDF text 0123", render.RGB(0, 120, 215))
    })
}
```

`-update`, inspect (crisp readable text at both sizes, no boxy artifacts, correct baseline alignment), then plain run → PASS.

- [ ] **Step 5: Commit** `feat(text): Face measure/draw with shared SDF atlas`

---

### Task 12: minimal demo host + demo app + docs

**Files:**
- Create: `app/window.go`, `cmd/fluo-demo/main.go`
- Modify: `README.md`, `ROADMAP.md`

**Interfaces:**
- Consumes: everything above.
- Produces (Phase 2/3 build on this; polish deferred to Phase 8):

```go
package app

type Config struct {
    Title         string
    Width, Height int     // logical px
}
type MouseState struct {
    Pos     render.Point  // logical px
    Down    bool          // left button
}
type Ctx struct {
    R     render.Renderer
    Size  render.Size     // logical px
    Scale float32
    Mouse MouseState
    Close func()          // request app exit
}
// Run opens the window and calls frame every vsync until closed. Blocks.
// Must be called from main(); locks the OS thread.
func Run(cfg Config, frame func(*Ctx)) error
```

- [ ] **Step 1: implement app/window.go**

```go
func init() { runtime.LockOSThread() }

func Run(cfg Config, frame func(*Ctx)) error {
    if err := glfw.Init(); err != nil { return fmt.Errorf("glfw: %w", err) }
    defer glfw.Terminate()
    glfw.WindowHint(glfw.ContextVersionMajor, 3)
    glfw.WindowHint(glfw.ContextVersionMinor, 3)
    glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
    glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
    win, err := glfw.CreateWindow(cfg.Width, cfg.Height, cfg.Title, nil, nil)
    if err != nil { return err }
    defer win.Destroy()
    win.MakeContextCurrent()
    if err := gl.Init(); err != nil { return err }
    glfw.SwapInterval(1)
    r, err := glr.New()
    if err != nil { return err }
    for !win.ShouldClose() {
        glfw.PollEvents()
        fbW, fbH := win.GetFramebufferSize()
        sx, _ := win.GetContentScale()
        gl.Viewport(0, 0, int32(fbW), int32(fbH))
        gl.ClearColor(0.125, 0.125, 0.14, 1)
        gl.Clear(gl.COLOR_BUFFER_BIT)
        mx, my := win.GetCursorPos()
        ctx := &Ctx{
            R:     r,
            Size:  render.Size{float32(fbW) / sx, float32(fbH) / sx},
            Scale: sx,
            Mouse: MouseState{
                Pos:  render.Point{float32(mx), float32(my)}, // glfw cursor pos is logical on Windows
                Down: win.GetMouseButton(glfw.MouseButtonLeft) == glfw.Press,
            },
            Close: func() { win.SetShouldClose(true) },
        }
        r.Begin(fbW, fbH, sx)
        frame(ctx)
        r.End()
        win.SwapBuffers()
    }
    return nil
}
```

- [ ] **Step 2: cmd/fluo-demo/main.go** — exercises every primitive, Fluent-flavored:

```go
func main() {
    font, err := text.Load(goregular.TTF)
    if err != nil { log.Fatal(err) }
    title, body := text.NewFace(font, 24), text.NewFace(font, 14)
    err = app.Run(app.Config{Title: "fluo demo", Width: 480, Height: 320}, func(c *app.Ctx) {
        card := render.Rect{40, 40, c.Size.W - 80, c.Size.H - 80}
        c.R.DrawShadow(card, 8, 16, render.RGBA(0, 0, 0, 120))
        c.R.FillRoundedRect(card, 8, render.RGB(32, 32, 36))
        c.R.StrokeRoundedRect(card, 8, 1, render.RGBA(255, 255, 255, 24))
        title.Draw(c.R, render.Point{card.X + 24, card.Y + 20}, "fluo", render.RGB(255, 255, 255))
        body.Draw(c.R, render.Point{card.X + 24, card.Y + 56}, "Phase 1: renderer + SDF text",
            render.RGBA(255, 255, 255, 160))
        // hover-reactive accent button (manual for now — this becomes controls.Button in Phase 5)
        btn := render.Rect{card.X + 24, card.Bottom() - 56, 120, 32}
        bg := render.RGB(0, 120, 215)
        if btn.Contains(c.Mouse.Pos) {
            bg = render.RGB(16, 132, 226)
            if c.Mouse.Down { bg = render.RGB(0, 100, 180) }
        }
        c.R.FillRoundedRect(btn, 4, bg)
        label := "Click me"
        w := body.Measure(label).W
        body.Draw(c.R, render.Point{btn.X + (btn.W-w)/2, btn.Y + (btn.H-body.LineHeight())/2},
            label, render.RGB(255, 255, 255))
    })
    if err != nil { log.Fatal(err) }
}
```

- [ ] **Step 3: manual verify** — `go run ./cmd/fluo-demo`: shadowed dark card on darker bg, crisp title + body text, button changes shade on hover and press. Resize the window — layout follows, nothing stretches. Close the window (leave no process running).
- [ ] **Step 4: full suite** — `go test ./...` → all PASS; `go vet ./...` clean.
- [ ] **Step 5: docs** — README: add Requirements (Go 1.23, C compiler for cgo, GL 3.3), `go run ./cmd/fluo-demo`, golden-test `-update` workflow. ROADMAP.md: tick every Phase 0 and Phase 1 checkbox this plan delivered.
- [ ] **Step 6: Commit** `feat(app): minimal demo host + fluo-demo; complete Phase 0/1`

---

## Self-review notes (resolved inline)

- Spec coverage: every Phase 0/1 roadmap node maps to a task (scaffolding→1, harness→3, interface/primitives→2, quad pipeline→4, clip→5, rounded→6, shadow→7, image/texture→8, SDF text→9-11, goldens→per-task, DPI scale→plumbed via `Begin`/`uScale` from Task 4, demo host→12).
- Type consistency: `quad(mode, dst, src, c, rectParams, extra, tex)` signature identical across Tasks 4-8; `render.GlyphQuad{Dst,Src}` matches Task 11's emission; `atlasEntry` fields used in `face.go` match Task 10's declaration.
- Known deferred items (deliberate, not gaps): atlas growth/eviction, kerning-off fast path, multi-line text, mouse wheel/keyboard input (Phase 3), macOS main-thread caveat documented in gltest comment.
