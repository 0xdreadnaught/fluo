# render / text

`render` defines fluo's abstract 2D drawing surface: the `Renderer`
interface (fill/stroke/gradient/shadow/blur/texture/glyph/clip operations)
plus the plain-data geometry and color types every other package builds
on — `Point`, `Size`, `Rect` (with `Thickness`-based `Inset`), and `Color`.
It has no dependency on any particular graphics API; `render/gl` is the
OpenGL 3.3 implementation the `app` package wires up. `text` builds on
`render` to provide pure-Go font loading and single-line text layout:
`Load` parses TTF/OTF bytes into a `Font`, `NewFace` pairs a `Font` with a
pixel size to produce a `Face` — the type callers actually measure and
draw with. Reach for `render` when implementing a new graphics backend or
painting a custom widget's chrome; reach for `text` whenever you need to
size or draw a string.

**Import:** `github.com/0xdreadnaught/fluo/render`, `github.com/0xdreadnaught/fluo/text`

## Contents
- [Renderer](#renderer)
- [TextureID](#textureid)
- [GlyphQuad](#glyphquad)
- [Color](#color)
- [Point](#point)
- [Size](#size)
- [Rect](#rect)
- [Thickness](#thickness)
- [Font](#font)
- [Atlas](#atlas)
- [Face](#face)

---

## Renderer

`Renderer` is fluo's backend seam: the interface `core`'s layout engine,
`text`'s glyph drawing, and every `controls` widget speak in terms of —
never a concrete backend type. `render/gl.Renderer` (OpenGL 3.3) is the
implementation the `app` package currently wires up; porting fluo to
another backend means implementing exactly this interface and nothing
more.

**The scale contract.** Every coordinate passed into a `Renderer` method —
every `Rect`, every `Point` — is authored in **logical pixels**. Device-
pixel conversion happens exactly once, on the GPU, driven by the `scale`
factor passed to [Begin](#rendererbegin); no CPU-side caller other than
`text` should ever multiply a coordinate by [Scale](#rendererscale). The
one exception is text: `text.Face.Draw` calls `Scale()` to rasterize
glyphs at true device resolution and pixel-snap their baseline and draw
origin, then hands back logical-px `Dst` rectangles in the `GlyphQuad`s it
submits — so the contract holds even for text, from the renderer's point
of view.

### Methods

| Method | Signature | Description |
|---|---|---|
| [Begin](#rendererbegin) | `Begin(fbWidth, fbHeight int, scale float32)` | Starts a new frame with the given framebuffer dimensions and scale factor. |
| [End](#rendererend) | `End()` | Flushes the current frame. |
| [FillRect](#rendererfillrect) | `FillRect(r Rect, c Color)` | Fills a rectangle with a solid color. |
| [FillRoundedRect](#rendererfillroundedrect) | `FillRoundedRect(r Rect, radius float32, c Color)` | Fills a rectangle with rounded corners with a solid color. |
| [DrawGradientRect](#rendererdrawgradientrect) | `DrawGradientRect(r Rect, from, to Color, horizontal bool)` | Fills a rectangle with a linear gradient. |
| [StrokeRoundedRect](#rendererstrokeroundedrect) | `StrokeRoundedRect(r Rect, radius, width float32, c Color)` | Draws the stroke of a rounded rectangle (inside edge). |
| [DrawShadow](#rendererdrawshadow) | `DrawShadow(r Rect, radius, blur float32, c Color)` | Draws a soft shadow of a rounded rectangle. |
| [DrawBackdropBlur](#rendererdrawbackdropblur) | `DrawBackdropBlur(r Rect, radius float32, tint Color)` | Draws an acrylic/mica-style backdrop-blur surface. |
| [CreateTexture](#renderercreatetexture) | `CreateTexture(w, h int, rgba []byte) TextureID` | Creates a new texture from RGBA8 data. |
| [UpdateTexture](#rendererupdatetexture) | `UpdateTexture(id TextureID, x, y, w, h int, rgba []byte)` | Updates a region of an existing texture. |
| [DeleteTexture](#rendererdeletetexture) | `DeleteTexture(id TextureID)` | Frees a texture created by `CreateTexture`. |
| [DrawQuad](#rendererdrawquad) | `DrawQuad(dst, src Rect, tex TextureID, tint Color)` | Draws a textured quad. |
| [DrawSDFQuads](#rendererdrawsdfquads) | `DrawSDFQuads(quads []GlyphQuad, tex TextureID, c Color)` | Draws glyphs from an SDF alpha atlas. |
| [DrawGlyphs](#rendererdrawglyphs) | `DrawGlyphs(quads []GlyphQuad, tex TextureID, c Color)` | Draws grayscale-coverage glyph quads from a coverage atlas. |
| [Scale](#rendererscale) | `Scale() float32` | Returns the current frame's device-pixels-per-logical-pixel factor. |
| [PushClip](#rendererpushclip) | `PushClip(r Rect)` | Pushes a new clip rectangle that intersects with the current clip. |
| [PopClip](#rendererpopclip) | `PopClip()` | Pops the current clip rectangle. |

#### Renderer.Begin

Starts a new frame with the given framebuffer dimensions (in device
pixels) and scale factor.

**Syntax**

```go
Begin(fbWidth, fbHeight int, scale float32)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `fbWidth` | `int` | Framebuffer width, in device pixels. |
| `fbHeight` | `int` | Framebuffer height, in device pixels. |
| `scale` | `float32` | Device-pixels-per-logical-pixel factor for this frame. Retrievable for the rest of the frame via `Scale`. |

**Notes** — Must be paired with a matching `End` call; every other
`Renderer` method is only meaningful between `Begin` and `End`.

**See also** — [Renderer.End](#rendererend), [Renderer.Scale](#rendererscale)

#### Renderer.End

Flushes the current frame.

**Syntax**

```go
End()
```

**See also** — [Renderer.Begin](#rendererbegin)

#### Renderer.FillRect

Fills a rectangle with a solid color.

**Syntax**

```go
FillRect(r Rect, c Color)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `Rect` | The rectangle to fill, in logical px. |
| `c` | `Color` | The fill color. |

**Example**

```go
r.FillRect(render.Rect{X: 0, Y: 0, W: 100, H: 24}, render.RGB(212, 208, 200))
```

**See also** — [Renderer.FillRoundedRect](#rendererfillroundedrect), [Rect](#rect), [Color](#color)

#### Renderer.FillRoundedRect

Fills a rectangle with rounded corners with a solid color.

**Syntax**

```go
FillRoundedRect(r Rect, radius float32, c Color)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `Rect` | The rectangle to fill, in logical px. |
| `radius` | `float32` | Corner radius, in logical px. |
| `c` | `Color` | The fill color. |

**Example**

```go
r.FillRoundedRect(card, 8, render.RGB(32, 32, 36))
```

**See also** — [Renderer.FillRect](#rendererfillrect), [Renderer.StrokeRoundedRect](#rendererstrokeroundedrect)

#### Renderer.DrawGradientRect

Fills `r` with a linear gradient from `from` to `to`: horizontal
(left→right) when `horizontal` is true, else vertical (top→bottom).

**Syntax**

```go
DrawGradientRect(r Rect, from, to Color, horizontal bool)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `Rect` | The rectangle to fill, in logical px. |
| `from` | `Color` | Gradient start color (left edge if horizontal, top edge otherwise). |
| `to` | `Color` | Gradient end color (right edge if horizontal, bottom edge otherwise). |
| `horizontal` | `bool` | `true` for a left→right gradient, `false` for top→bottom. |

**Example**

```go
r.DrawGradientRect(caption, theme.Active().Color.CaptionFrom, theme.Active().Color.CaptionTo, true)
```

**See also** — [Renderer.FillRect](#rendererfillrect)

#### Renderer.StrokeRoundedRect

Draws the stroke of a rectangle with rounded corners (stroke inside edge).

**Syntax**

```go
StrokeRoundedRect(r Rect, radius, width float32, c Color)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `Rect` | The rectangle whose border to stroke, in logical px. |
| `radius` | `float32` | Corner radius, in logical px. |
| `width` | `float32` | Stroke thickness, in logical px, drawn inside `r`'s edge. |
| `c` | `Color` | The stroke color. |

**See also** — [Renderer.FillRoundedRect](#rendererfillroundedrect)

#### Renderer.DrawShadow

Draws a soft shadow of a rounded rectangle.

**Syntax**

```go
DrawShadow(r Rect, radius, blur float32, c Color)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `Rect` | The rectangle the shadow is cast from, in logical px. |
| `radius` | `float32` | Corner radius matching the shadowed shape, in logical px. |
| `blur` | `float32` | Blur radius, in logical px. |
| `c` | `Color` | The shadow color (typically translucent black). |

**See also** — [MetricTokens](theming.md#metrictokens) (`ShadowBlur`)

#### Renderer.DrawBackdropBlur

Draws an acrylic/mica-style backdrop-blur surface: it snapshots whatever
has already been drawn beneath `r`, blurs it, and composites the result
back into `r` (rounded by `radius`) tinted by `c` — approximating WinUI's
acrylic material.

**Syntax**

```go
DrawBackdropBlur(r Rect, radius float32, tint Color)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `Rect` | The region to blur and composite over, in logical px. |
| `radius` | `float32` | Corner radius, in logical px. |
| `tint` | `Color` | Color tinting the blurred backdrop. |

**Notes** — Because it samples already-rendered content, callers must
invoke it *after* painting whatever should show through and *before*
drawing anything meant to sit on top of the acrylic surface (e.g. its
children). An implementation that cannot obtain a true mid-frame snapshot
may instead degrade to a flat, tinted, translucent rounded fill
(equivalent to `FillRoundedRect(r, radius, c)`); any such degrade must be
prominently documented at the implementation site.

**See also** — [Renderer.FillRoundedRect](#rendererfillroundedrect)

#### Renderer.CreateTexture

Creates a new texture from RGBA8 data.

**Syntax**

```go
CreateTexture(w, h int, rgba []byte) TextureID
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `w` | `int` | Texture width, in pixels. |
| `h` | `int` | Texture height, in pixels. |
| `rgba` | `[]byte` | Pixel data, tightly packed RGBA8. Must have length `w*h*4` when non-nil. A `nil` slice allocates storage without uploading pixels. |

**Returns** — `TextureID`, a handle for later `UpdateTexture`/`DeleteTexture`/draw calls.

**Notes** — Implementations panic if `rgba` is non-nil and shorter than `w*h*4`.

**See also** — [TextureID](#textureid), [Renderer.UpdateTexture](#rendererupdatetexture), [Renderer.DeleteTexture](#rendererdeletetexture)

#### Renderer.UpdateTexture

Updates a region of an existing texture.

**Syntax**

```go
UpdateTexture(id TextureID, x, y, w, h int, rgba []byte)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `id` | `TextureID` | The texture to update, as returned by `CreateTexture`. |
| `x` | `int` | Left edge of the region to update, in texture pixels. |
| `y` | `int` | Top edge of the region to update, in texture pixels. |
| `w` | `int` | Region width, in pixels. |
| `h` | `int` | Region height, in pixels. |
| `rgba` | `[]byte` | Pixel data, tightly packed RGBA8, length `w*h*4`. |

**Notes** — Unlike `CreateTexture`, `rgba` must not be nil here; implementations panic if it is shorter than `w*h*4`.

**See also** — [Renderer.CreateTexture](#renderercreatetexture)

#### Renderer.DeleteTexture

Frees a texture created by `CreateTexture`.

**Syntax**

```go
DeleteTexture(id TextureID)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `id` | `TextureID` | The texture to free. `NoTexture` is a no-op. |

**See also** — [TextureID](#textureid), [Renderer.CreateTexture](#renderercreatetexture)

#### Renderer.DrawQuad

Draws a textured quad with the source rectangle in UV coordinates (0..1) and a tint color.

**Syntax**

```go
DrawQuad(dst, src Rect, tex TextureID, tint Color)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `dst` | `Rect` | Destination rectangle, in logical px. |
| `src` | `Rect` | Source rectangle within the texture, in UV coordinates (0..1). |
| `tex` | `TextureID` | The texture to sample. |
| `tint` | `Color` | Color multiplied into the sampled texel. |

**See also** — [TextureID](#textureid), [Renderer.DrawGlyphs](#rendererdrawglyphs)

#### Renderer.DrawSDFQuads

Draws glyphs from an SDF alpha atlas with the given color. Retained for
future scaled/animated text; `text.Face.Draw`'s default (HD) path uses
`DrawGlyphs` instead.

**Syntax**

```go
DrawSDFQuads(quads []GlyphQuad, tex TextureID, c Color)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `quads` | `[]GlyphQuad` | Destination/source rectangle pairs for the glyphs to draw. |
| `tex` | `TextureID` | The SDF atlas texture. |
| `c` | `Color` | Text color. |

**Notes** — Not the path `text.Face.Draw` uses today (see
[DrawGlyphs](#rendererdrawglyphs)); kept for a future scaled/animated-text
feature that needs resolution-independent glyphs.

**See also** — [Renderer.DrawGlyphs](#rendererdrawglyphs), [GlyphQuad](#glyphquad), [Face.Draw](#facedraw)

#### Renderer.DrawGlyphs

Draws grayscale-coverage glyph quads from a coverage atlas (alpha =
`texture.r`) tinted with `c`.

**Syntax**

```go
DrawGlyphs(quads []GlyphQuad, tex TextureID, c Color)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `quads` | `[]GlyphQuad` | Destination/source rectangle pairs for the glyphs to draw. `Dst` is in logical px (the vertex shader applies `uScale`); `Src` is 0..1 atlas UV. |
| `tex` | `TextureID` | The coverage atlas texture. |
| `c` | `Color` | Text color. |

**Notes** — Used by `text.Face` for crisp, direct grayscale-AA UI text —
the path `Face.Draw` uses by default today. Distinct from
[DrawSDFQuads](#rendererdrawsdfquads) (the SDF path, retained for future
scaled/animated text).

**See also** — [Renderer.DrawSDFQuads](#rendererdrawsdfquads), [GlyphQuad](#glyphquad), [Face.Draw](#facedraw)

#### Renderer.Scale

Returns the current frame's device-pixels-per-logical-pixel factor (as
passed to `Begin`). Valid between `Begin` and `End`.

**Syntax**

```go
Scale() float32
```

**Returns** — `float32`, the scale factor passed to the current frame's `Begin` call.

**Notes** — The text layer uses it to rasterize glyphs at device
resolution and to pixel-snap; no other code should multiply by scale —
the vertex shader applies `uScale` exactly once. See the
[scale contract](#renderer) above.

**See also** — [Renderer.Begin](#rendererbegin), [Face.Draw](#facedraw)

#### Renderer.PushClip

Pushes a new clip rectangle that intersects with the current clip.

**Syntax**

```go
PushClip(r Rect)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `Rect` | The rectangle to intersect with the current clip, in logical px. |

**Notes** — Clips nest: each `PushClip` narrows drawing to the
intersection of `r` and whatever was already clipped. Every push must be
matched by a `PopClip`; `core.RenderWidget` does this automatically around
a widget's children when the widget implements clipping.

**See also** — [Renderer.PopClip](#rendererpopclip), [Rect.Intersect](#rectintersect)

#### Renderer.PopClip

Pops the current clip rectangle.

**Syntax**

```go
PopClip()
```

**See also** — [Renderer.PushClip](#rendererpushclip)

---

## TextureID

`TextureID` uniquely identifies a texture created by
[Renderer.CreateTexture](#renderercreatetexture), with `0` meaning no
texture.

### Underlying type

```go
type TextureID uint32
```

### Constants

| Name | Type | Description |
|---|---|---|
| `NoTexture` | `TextureID` | A `TextureID` representing no texture (the zero value). Passing it to `DeleteTexture` is a no-op. |

**See also** — [Renderer.CreateTexture](#renderercreatetexture), [Renderer.DeleteTexture](#rendererdeletetexture)

---

## GlyphQuad

`GlyphQuad` represents a glyph quad with destination and source
rectangles. It is the unit `Face.Draw` batches and submits to
[Renderer.DrawGlyphs](#rendererdrawglyphs) (or, for the SDF path,
[Renderer.DrawSDFQuads](#rendererdrawsdfquads)).

### Fields

| Name | Type | Description |
|---|---|---|
| `Dst` | `Rect` | Destination rectangle, in logical px. |
| `Src` | `Rect` | Source rectangle within the glyph atlas, in UV coordinates (0..1). |

**See also** — [Renderer.DrawGlyphs](#rendererdrawglyphs), [Renderer.DrawSDFQuads](#rendererdrawsdfquads), [Face.Draw](#facedraw)

---

## Color

`Color` represents an RGBA color with components in the range [0, 1].

### Fields

| Name | Type | Description |
|---|---|---|
| `R` | `float32` | Red component, 0..1. |
| `G` | `float32` | Green component, 0..1. |
| `B` | `float32` | Blue component, 0..1. |
| `A` | `float32` | Alpha component, 0..1. |

### Functions

| Function | Signature | Description |
|---|---|---|
| [RGB](#rgb) | `func RGB(r, g, b uint8) Color` | Creates a color from 8-bit red, green, and blue components with full alpha. |
| [RGBA](#rgba) | `func RGBA(r, g, b, a uint8) Color` | Creates a color from 8-bit red, green, blue, and alpha components. |

#### RGB

Creates a color from 8-bit red, green, and blue components with full alpha.

**Syntax**

```go
func RGB(r, g, b uint8) Color
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `uint8` | Red component, 0-255. |
| `g` | `uint8` | Green component, 0-255. |
| `b` | `uint8` | Blue component, 0-255. |

**Returns** — `Color` with `A` set to 1 (full opacity). Equivalent to `RGBA(r, g, b, 255)`.

**Example**

```go
bg := render.RGB(0, 120, 215)
```

**See also** — [RGBA](#rgba)

#### RGBA

Creates a color from 8-bit red, green, blue, and alpha components.

**Syntax**

```go
func RGBA(r, g, b, a uint8) Color
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `uint8` | Red component, 0-255. |
| `g` | `uint8` | Green component, 0-255. |
| `b` | `uint8` | Blue component, 0-255. |
| `a` | `uint8` | Alpha component, 0-255. |

**Returns** — `Color` with each component divided by 255 into the library's 0..1 range.

**Example**

```go
scrim := render.RGBA(0, 0, 0, 90)
```

**See also** — [RGB](#rgb)

---

## Point

`Point` represents a 2D point with X and Y coordinates, in logical px.

### Fields

| Name | Type | Description |
|---|---|---|
| `X` | `float32` | X coordinate, in logical px. |
| `Y` | `float32` | Y coordinate, in logical px. |

**Example**

```go
title.Draw(r, render.Point{X: card.X + 24, Y: card.Y + 20}, "fluo", render.RGB(255, 255, 255))
```

**See also** — [Rect](#rect), [Face.Draw](#facedraw)

---

## Size

`Size` represents a 2D size with width and height, in logical px.

### Fields

| Name | Type | Description |
|---|---|---|
| `W` | `float32` | Width, in logical px. |
| `H` | `float32` | Height, in logical px. |

**See also** — [Rect](#rect), [Face.Measure](#facemeasure)

---

## Rect

`Rect` represents a rectangle with top-left position and size, in logical
px, plus geometry helpers used throughout layout and hit-testing.

### Fields

| Name | Type | Description |
|---|---|---|
| `X` | `float32` | Left edge, in logical px. |
| `Y` | `float32` | Top edge, in logical px. |
| `W` | `float32` | Width, in logical px. |
| `H` | `float32` | Height, in logical px. |

### Methods

| Method | Signature | Description |
|---|---|---|
| [Right](#rectright) | `func (r Rect) Right() float32` | Returns the x-coordinate of the right edge. |
| [Bottom](#rectbottom) | `func (r Rect) Bottom() float32` | Returns the y-coordinate of the bottom edge. |
| [Contains](#rectcontains) | `func (r Rect) Contains(p Point) bool` | Checks if a point is inside the rectangle. |
| [Intersect](#rectintersect) | `func (r Rect) Intersect(o Rect) Rect` | Computes the intersection of two rectangles. |
| [Inflate](#rectinflate) | `func (r Rect) Inflate(d float32) Rect` | Returns a new rectangle with all sides grown by the given amount. |
| [Empty](#rectempty) | `func (r Rect) Empty() bool` | Checks if the rectangle has zero or negative width or height. |
| [Inset](#rectinset) | `func (r Rect) Inset(t Thickness) Rect` | Returns a new rectangle inset by the given thickness. |

#### Rect.Right

Returns the x-coordinate of the right edge.

**Syntax**

```go
func (r Rect) Right() float32
```

**Returns** — `float32`, equal to `r.X + r.W`.

**See also** — [Rect.Bottom](#rectbottom)

#### Rect.Bottom

Returns the y-coordinate of the bottom edge.

**Syntax**

```go
func (r Rect) Bottom() float32
```

**Returns** — `float32`, equal to `r.Y + r.H`.

**See also** — [Rect.Right](#rectright)

#### Rect.Contains

Checks if a point is inside the rectangle (half-open interval `[X,
Right)` × `[Y, Bottom)`).

**Syntax**

```go
func (r Rect) Contains(p Point) bool
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `p` | `Point` | The point to test, in the same coordinate space as `r`. |

**Returns** — `bool`, true when `p.X >= r.X && p.X < r.Right() && p.Y >= r.Y && p.Y < r.Bottom()`.

**Notes** — The right and bottom edges are exclusive; a point exactly on
`Right()` or `Bottom()` is not contained.

**See also** — [Point](#point)

#### Rect.Intersect

Computes the intersection of two rectangles.

**Syntax**

```go
func (r Rect) Intersect(o Rect) Rect
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `o` | `Rect` | The other rectangle. |

**Returns** — `Rect`, the overlapping region. Returns an empty rectangle (`W`/`H` = 0) if the rectangles do not overlap.

**Example**

```go
visible := contentRect.Intersect(clipRect)
if !visible.Empty() {
    r.FillRect(visible, c)
}
```

**See also** — [Rect.Empty](#rectempty), [Renderer.PushClip](#rendererpushclip)

#### Rect.Inflate

Returns a new rectangle with all sides grown by the given amount.

**Syntax**

```go
func (r Rect) Inflate(d float32) Rect
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `d` | `float32` | Amount to grow each side by, in logical px. A negative value shrinks the rectangle. |

**Returns** — `Rect`, positioned at `(r.X-d, r.Y-d)` with size `(r.W+2d, r.H+2d)`.

**See also** — [Rect.Inset](#rectinset)

#### Rect.Empty

Checks if the rectangle has zero or negative width or height.

**Syntax**

```go
func (r Rect) Empty() bool
```

**Returns** — `bool`, true when `r.W <= 0 || r.H <= 0`.

**See also** — [Rect.Intersect](#rectintersect)

#### Rect.Inset

Returns a new rectangle inset by the given thickness.

**Syntax**

```go
func (r Rect) Inset(t Thickness) Rect
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `t` | `Thickness` | Insets to apply to each edge. |

**Returns** — `Rect`, positioned at `(r.X+t.Left, r.Y+t.Top)` with size `(r.W-t.Left-t.Right, r.H-t.Top-t.Bottom)`.

**Example**

```go
content := bounds.Inset(render.Uniform(th.Metric.PaddingM))
```

**See also** — [Thickness](#thickness), [Uniform](#uniform)

---

## Thickness

`Thickness` represents insets on all four sides, in logical px. Used by
[Rect.Inset](#rectinset).

### Fields

| Name | Type | Description |
|---|---|---|
| `Left` | `float32` | Left inset, in logical px. |
| `Top` | `float32` | Top inset, in logical px. |
| `Right` | `float32` | Right inset, in logical px. |
| `Bottom` | `float32` | Bottom inset, in logical px. |

### Functions

| Function | Signature | Description |
|---|---|---|
| [Uniform](#uniform) | `func Uniform(v float32) Thickness` | Creates a thickness with the same value on all sides. |

#### Uniform

Creates a thickness with the same value on all sides.

**Syntax**

```go
func Uniform(v float32) Thickness
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `v` | `float32` | Inset applied to `Left`, `Top`, `Right`, and `Bottom`, in logical px. |

**Returns** — `Thickness{v, v, v, v}`.

**Example**

```go
content := bounds.Inset(render.Uniform(8))
```

**See also** — [Rect.Inset](#rectinset)

---

## Font

`Font` wraps a parsed `sfnt.Font` (from `golang.org/x/image/font/sfnt`)
together with the scratch buffer sfnt requires for its calls. It is the
starting point for the `text` API: parse bytes once with `Load`, then
build one or more [Face](#face) values from it at whatever sizes you
need.

**Constructor**

```go
func Load(ttf []byte) (*Font, error)
```

Parses raw TrueType/OpenType font bytes into a `Font`. Returns an error if
`sfnt.Parse` cannot parse `ttf`.

**Example**

```go
font, err := text.Load(goregular.TTF)
if err != nil {
    log.Fatal(err)
}
body := text.NewFace(font, 14)
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [HasGlyph](#fonthasglyph) | `func (f *Font) HasGlyph(r rune) bool` | Reports whether f actually has a glyph for r. |

#### Font.HasGlyph

Reports whether `f` actually has a glyph for `r`, as opposed to silently
falling back to glyph index 0 (`.notdef`) the way `Face.Measure` and
`Face.Draw` do.

**Syntax**

```go
func (f *Font) HasGlyph(r rune) bool
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `rune` | The character to check. |

**Returns** — `bool`, true when `f` has a real glyph for `r` (not the `.notdef` fallback).

**Notes** — Callers that want to draw a specific rune (e.g. a Unicode
symbol used as an icon) only when the font can really render it —
falling back to a drawn shape otherwise — should gate on this rather than
`Measure`'s advance width, since a `.notdef` glyph can still have a
nonzero advance.

**Example**

```go
if font.HasGlyph('✓') {
    face.Draw(r, at, "✓", checkColor)
} else {
    drawFallbackCheckmark(r, at, checkColor)
}
```

**See also** — [Face.Measure](#facemeasure), [Face.Draw](#facedraw)

---

## Atlas

`Atlas` packs SDF glyph masks for a single `Font` into one square alpha
image (1024×1024), uploading it to a GPU texture lazily and
incrementally. Every `Face` built from a given `Font` (via `NewFace`)
draws from that font's one shared `Atlas` — glyph SDFs are rasterized
once, at a fixed 48px raster size, and reused at any draw size — so
creating many `Face`s at different sizes for the same `Font` does not
duplicate the atlas. Internally the same `Atlas` also owns a second,
independent backing image/texture for the direct grayscale-coverage
glyphs `Face.Draw`'s default path uses (coverage masks are rasterized at
their exact device-pixel size rather than being resolution-independent
like the SDF path, so a new size means a new raster). Callers do not pack
glyphs directly; `Font.sharedAtlas` and `Face.Draw` do that internally.

**Constructor**

```go
func NewAtlas(f *Font) *Atlas
```

Creates an empty `Atlas` backed by `f`. Glyphs are rasterized and packed
lazily as they are first requested.

**Notes** — `Atlas` is not safe for concurrent use. It exposes no exported
methods — `NewFace` calls `Font.sharedAtlas()` to obtain (and lazily
create) the one `Atlas` a `Font`'s `Face`s share; application code never
constructs or drives an `Atlas` directly beyond that.

**See also** — [Font](#font), [Face](#face)

---

## Face

`Face` renders text from a `Font` at a fixed pixel size. The glyph
coverage masks it draws from (see `Draw`) live in the `Font`'s shared
`Atlas`, so creating many `Face`s at different sizes for the same `Font`
does not duplicate the atlas itself — though each distinct (glyph, device
px) pair is rasterized once at that size, since coverage masks aren't
resolution-independent the way the retained SDF path is. `Face` is the
layout-facing text API: `controls` widgets depend on `Measure`,
`LineHeight`, and `Ascent` to size and position themselves, then call
`Draw` to paint.

**Constructor**

```go
func NewFace(f *Font, sizePx float32) *Face
```

Returns a `Face` for `f` at `sizePx`, ensuring `f`'s shared glyph atlas
exists.

### Fields

| Name | Type | Description |
|---|---|---|
| `Font` | `*Font` | The font this face draws from. |
| `SizePx` | `float32` | The face's fixed pixel size, in logical px. |

**Example**

```go
font, _ := text.Load(goregular.TTF)
body := text.NewFace(font, th.Type.BodySize)
```

### Methods

| Method | Signature | Description |
|---|---|---|
| [LineHeight](#facelineheight) | `func (fa *Face) LineHeight() float32` | Returns the recommended distance between successive baselines. |
| [Ascent](#faceascent) | `func (fa *Face) Ascent() float32` | Returns the distance from the baseline up to the top of the line. |
| [Measure](#facemeasure) | `func (fa *Face) Measure(s string) render.Size` | Returns the size s occupies when drawn with fa. |
| [Draw](#facedraw) | `func (fa *Face) Draw(r render.Renderer, at render.Point, s string, c render.Color)` | Renders s with fa. |

#### Face.LineHeight

Returns the recommended distance between successive baselines when text
is set with `fa`: ascent + descent + lineGap, all at `fa.SizePx`.

**Syntax**

```go
func (fa *Face) LineHeight() float32
```

**Returns** — `float32`, in logical px.

**See also** — [Face.Ascent](#faceascent), [Face.Measure](#facemeasure)

#### Face.Ascent

Returns the distance from the baseline up to the top of the line at
`fa.SizePx`.

**Syntax**

```go
func (fa *Face) Ascent() float32
```

**Returns** — `float32`, in logical px.

**Notes** — `Face.Draw` places the baseline at `at.Y + fa.Ascent()`.

**See also** — [Face.LineHeight](#facelineheight), [Face.Draw](#facedraw)

#### Face.Measure

Returns the size `s` occupies when drawn with `fa`: width is the sum of
glyph advances plus kerning between consecutive glyphs, height is
`LineHeight` (text is laid out on a single line for now).

**Syntax**

```go
func (fa *Face) Measure(s string) render.Size
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `s` | `string` | The text to measure. |

**Returns** — `render.Size`, in logical px: `W` is the laid-out width, `H` is `fa.LineHeight()`.

**Notes** — Runes with no glyph in the font fall back to glyph index 0
(`.notdef`), matching `Draw`. Text is single-line only — `Measure` does
not wrap or account for newlines.

**Example**

```go
size := body.Measure("Hello, fluo!")
box := render.Rect{X: at.X, Y: at.Y, W: size.W, H: size.H}
```

**See also** — [Face.Draw](#facedraw), [Font.HasGlyph](#fonthasglyph)

#### Face.Draw

Renders `s` with `fa`, with `at` as the top-left corner of the text box;
the baseline sits at `at.Y + fa.Ascent()`.

**Syntax**

```go
func (fa *Face) Draw(r render.Renderer, at render.Point, s string, c render.Color)
```

**Parameters**

| Name | Type | Description |
|---|---|---|
| `r` | `render.Renderer` | The renderer to draw into. Must be between `Begin`/`End`. |
| `at` | `render.Point` | Top-left corner of the text box, in logical px. |
| `s` | `string` | The text to draw. |
| `c` | `render.Color` | Text color. |

**Notes** — This is the crisp HD-text path: each glyph is rasterized
directly (grayscale-AA coverage, no SDF) at the exact device-pixel size
for the current frame's scale (from `r.Scale()`), and both the baseline
and each glyph's draw origin are snapped to whole device pixels for a
sharp result. Layout stays exact: advances and kerning are computed from
the unsnapped logical-px metrics path, and the pen is never snapped
cumulatively — only each glyph's own draw origin is — so accumulated
advance/`Measure` width never drifts from the pixels actually drawn.
Glyph quads are gathered from the font's shared coverage atlas and
submitted to `r` in a single [Renderer.DrawGlyphs](#rendererdrawglyphs)
batch (not `DrawSDFQuads` — that path is retained for future
scaled/animated text). Runes with no glyph in the font fall back to glyph
index 0 (`.notdef`); glyphs with no visible coverage (e.g. space) are
skipped but still advance the pen.

**Example**

```go
title.Draw(r, render.Point{X: card.X + 24, Y: card.Y + 20}, "fluo", render.RGB(255, 255, 255))
```

**See also** — [Renderer.DrawGlyphs](#rendererdrawglyphs), [Renderer.DrawSDFQuads](#rendererdrawsdfquads), [Renderer.Scale](#rendererscale), [Face.Measure](#facemeasure)
