// Package gl implements render.Renderer on top of OpenGL 3.3 core. It is
// the second package (after gltest) allowed to import go-gl/gl; the rest
// of fluo stays windowing/graphics-API agnostic.
package gl

import (
	"fmt"
	"unsafe"

	"github.com/go-gl/gl/v3.3-core/gl"

	"github.com/0xdreadnaught/fluo/render"
)

// New compiles the uber-shader, links the program, allocates the VAO/VBO
// for batched vertices, and creates the 1x1 white texture used for
// non-textured draw modes. It requires a current OpenGL 3.3-core context
// on the calling thread (e.g. one set up by gltest.Run).
func New() (*Renderer, error) {
	vs, err := compile(vertexShaderSrc, gl.VERTEX_SHADER)
	if err != nil {
		return nil, fmt.Errorf("gl: compile vertex shader: %w", err)
	}
	fs, err := compile(fragmentShaderSrc, gl.FRAGMENT_SHADER)
	if err != nil {
		return nil, fmt.Errorf("gl: compile fragment shader: %w", err)
	}
	prog, err := link(vs, fs)
	if err != nil {
		return nil, fmt.Errorf("gl: link program: %w", err)
	}

	rd := &Renderer{
		prog:  prog,
		verts: make([]float32, 0, maxVerts*floatsPerVert),
	}

	rd.locViewport = gl.GetUniformLocation(rd.prog, gl.Str("uViewport\x00"))
	rd.locScale = gl.GetUniformLocation(rd.prog, gl.Str("uScale\x00"))
	locTex := gl.GetUniformLocation(rd.prog, gl.Str("uTex\x00"))

	gl.GenVertexArrays(1, &rd.vao)
	gl.GenBuffers(1, &rd.vbo)

	gl.BindVertexArray(rd.vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, rd.vbo)
	gl.BufferData(gl.ARRAY_BUFFER, maxVerts*floatsPerVert*4, nil, gl.DYNAMIC_DRAW)

	const stride = int32(floatsPerVert * 4)
	gl.EnableVertexAttribArray(0) // aPos
	gl.VertexAttribPointerWithOffset(0, 2, gl.FLOAT, false, stride, 0)
	gl.EnableVertexAttribArray(1) // aUV
	gl.VertexAttribPointerWithOffset(1, 2, gl.FLOAT, false, stride, 2*4)
	gl.EnableVertexAttribArray(2) // aColor
	gl.VertexAttribPointerWithOffset(2, 4, gl.FLOAT, false, stride, 4*4)
	gl.EnableVertexAttribArray(3) // aRect
	gl.VertexAttribPointerWithOffset(3, 4, gl.FLOAT, false, stride, 8*4)
	gl.EnableVertexAttribArray(4) // aExtra
	gl.VertexAttribPointerWithOffset(4, 2, gl.FLOAT, false, stride, 12*4)
	gl.EnableVertexAttribArray(5) // aMode
	gl.VertexAttribPointerWithOffset(5, 1, gl.FLOAT, false, stride, 14*4)

	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	gl.BindVertexArray(0)

	gl.UseProgram(rd.prog)
	gl.Uniform1i(locTex, 0)
	gl.UseProgram(0)

	rd.whiteTex = newWhiteTexture()
	rd.curTex = rd.whiteTex

	return rd, nil
}

// newWhiteTexture creates a 1x1 opaque white RGBA texture used to keep the
// sampler valid for non-textured (solid/rounded/shadow) draw modes.
func newWhiteTexture() uint32 {
	var tex uint32
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	white := [4]byte{255, 255, 255, 255}
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, 1, 1, 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(&white[0]))
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	return tex
}

// Begin starts a new frame: binds the shader program, sets the viewport
// and scale uniforms, configures blending/depth/scissor state, and resets
// the batch and clip stack.
func (rd *Renderer) Begin(fbWidth, fbHeight int, scale float32) {
	rd.fbW, rd.fbH = fbWidth, fbHeight
	rd.scale = scale

	gl.UseProgram(rd.prog)
	gl.Uniform2f(rd.locViewport, float32(fbWidth), float32(fbHeight))
	gl.Uniform1f(rd.locScale, scale)

	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.Disable(gl.DEPTH_TEST)
	gl.Disable(gl.SCISSOR_TEST)

	rd.curTex = rd.whiteTex
	rd.verts = rd.verts[:0]
	rd.clips = rd.clips[:0]
}

// End flushes any pending batched vertices.
func (rd *Renderer) End() {
	rd.flush()
}

// FillRect fills a rectangle with a solid color.
func (rd *Renderer) FillRect(r render.Rect, c render.Color) {
	rd.quad(0, r, render.Rect{}, c, [4]float32{}, [2]float32{}, rd.whiteTex)
}

// rectParams returns the rect center and half-dimensions for shader SDF calculations.
func rectParams(r render.Rect) [4]float32 {
	return [4]float32{r.X + r.W/2, r.Y + r.H/2, r.W / 2, r.H / 2}
}

// clampRadius clamps radius so a rounded shape derived from r never exceeds
// half its width or height.
func clampRadius(r render.Rect, radius float32) float32 {
	if half := r.W / 2; radius > half {
		radius = half
	}
	if half := r.H / 2; radius > half {
		radius = half
	}
	return radius
}

// FillRoundedRect fills a rectangle with rounded corners with a solid color.
func (rd *Renderer) FillRoundedRect(r render.Rect, radius float32, c render.Color) {
	radius = clampRadius(r, radius)
	rd.quad(3, r, render.Rect{}, c, rectParams(r), [2]float32{radius, 0}, rd.whiteTex)
}

// StrokeRoundedRect draws the stroke of a rectangle with rounded corners.
func (rd *Renderer) StrokeRoundedRect(r render.Rect, radius, width float32, c render.Color) {
	radius = clampRadius(r, radius)
	rd.quad(4, r, render.Rect{}, c, rectParams(r), [2]float32{radius, width}, rd.whiteTex)
}

// DrawShadow draws a soft shadow of a rounded rectangle.
func (rd *Renderer) DrawShadow(r render.Rect, radius, blur float32, c render.Color) {
	radius = clampRadius(r, radius)
	rd.quad(5, r.Inflate(blur), render.Rect{}, c, rectParams(r), [2]float32{radius, blur}, rd.whiteTex)
}

// CreateTexture creates a new texture from RGBA8 data.
func (rd *Renderer) CreateTexture(w, h int, rgba []byte) render.TextureID {
	if rgba != nil && len(rgba) < w*h*4 {
		panic("render/gl: rgba too short for w*h*4")
	}
	var id uint32
	gl.GenTextures(1, &id)
	gl.BindTexture(gl.TEXTURE_2D, id)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	var ptr unsafe.Pointer
	if rgba != nil {
		ptr = gl.Ptr(rgba)
	}
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, int32(w), int32(h), 0, gl.RGBA, gl.UNSIGNED_BYTE, ptr)
	gl.BindTexture(gl.TEXTURE_2D, rd.curTex) // restore batch binding
	return render.TextureID(id)
}

// UpdateTexture updates a region of an existing texture.
func (rd *Renderer) UpdateTexture(id render.TextureID, x, y, w, h int, rgba []byte) {
	if len(rgba) < w*h*4 {
		panic("render/gl: rgba too short for w*h*4")
	}
	rd.flush()
	gl.BindTexture(gl.TEXTURE_2D, uint32(id))
	gl.TexSubImage2D(gl.TEXTURE_2D, 0, int32(x), int32(y), int32(w), int32(h), gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(rgba))
	gl.BindTexture(gl.TEXTURE_2D, rd.curTex)
}

// DeleteTexture frees a texture created by CreateTexture; NoTexture is a
// no-op.
func (rd *Renderer) DeleteTexture(id render.TextureID) {
	if id == render.NoTexture {
		return
	}
	rd.flush() // pending quads may reference this texture
	tex := uint32(id)
	gl.DeleteTextures(1, &tex)
	if tex == rd.curTex {
		rd.curTex = rd.whiteTex
		gl.BindTexture(gl.TEXTURE_2D, rd.curTex)
	}
}

// DrawQuad draws a textured quad with a tint color.
func (rd *Renderer) DrawQuad(dst, src render.Rect, tex render.TextureID, tint render.Color) {
	rd.quad(1, dst, src, tint, [4]float32{}, [2]float32{}, uint32(tex))
}

// DrawSDFQuads draws glyphs from an SDF alpha atlas with the given color.
func (rd *Renderer) DrawSDFQuads(quads []render.GlyphQuad, tex render.TextureID, c render.Color) {
	for _, q := range quads {
		rd.quad(2, q.Dst, q.Src, c, [4]float32{}, [2]float32{}, uint32(tex))
	}
}

// PushClip pushes a new clip rectangle that intersects with the current clip.
func (rd *Renderer) PushClip(r render.Rect) {
	rd.flush()
	if n := len(rd.clips); n > 0 {
		r = r.Intersect(rd.clips[n-1])
	}
	rd.clips = append(rd.clips, r)
	rd.applyClip()
}

// PopClip pops the current clip rectangle.
func (rd *Renderer) PopClip() {
	rd.flush()
	rd.clips = rd.clips[:len(rd.clips)-1]
	rd.applyClip()
}

func (rd *Renderer) applyClip() {
	if len(rd.clips) == 0 {
		gl.Disable(gl.SCISSOR_TEST)
		return
	}
	c := rd.clips[len(rd.clips)-1]
	gl.Enable(gl.SCISSOR_TEST)
	x := int32(c.X * rd.scale)
	y := int32(float32(rd.fbH) - (c.Y+c.H)*rd.scale) // GL scissor origin = bottom-left
	gl.Scissor(x, y, int32(c.W*rd.scale), int32(c.H*rd.scale))
}
