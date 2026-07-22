// Package gl implements render.Renderer on top of OpenGL 3.3 core. It is
// the second package (after gltest) allowed to import go-gl/gl; the rest
// of fluo stays windowing/graphics-API agnostic.
package gl

import (
	"fmt"

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

// FillRoundedRect fills a rectangle with rounded corners with a solid color.
func (rd *Renderer) FillRoundedRect(r render.Rect, radius float32, c render.Color) {
	panic("gl: FillRoundedRect not implemented (Task 5)")
}

// StrokeRoundedRect draws the stroke of a rectangle with rounded corners.
func (rd *Renderer) StrokeRoundedRect(r render.Rect, radius, width float32, c render.Color) {
	panic("gl: StrokeRoundedRect not implemented (Task 5)")
}

// DrawShadow draws a soft shadow of a rounded rectangle.
func (rd *Renderer) DrawShadow(r render.Rect, radius, blur float32, c render.Color) {
	panic("gl: DrawShadow not implemented (Task 5)")
}

// CreateTexture creates a new texture from RGBA8 data.
func (rd *Renderer) CreateTexture(w, h int, rgba []byte) render.TextureID {
	panic("gl: CreateTexture not implemented (Task 6)")
}

// UpdateTexture updates a region of an existing texture.
func (rd *Renderer) UpdateTexture(id render.TextureID, x, y, w, h int, rgba []byte) {
	panic("gl: UpdateTexture not implemented (Task 6)")
}

// DrawQuad draws a textured quad with a tint color.
func (rd *Renderer) DrawQuad(dst, src render.Rect, tex render.TextureID, tint render.Color) {
	panic("gl: DrawQuad not implemented (Task 6)")
}

// DrawSDFQuads draws glyphs from an SDF alpha atlas with the given color.
func (rd *Renderer) DrawSDFQuads(quads []render.GlyphQuad, tex render.TextureID, c render.Color) {
	panic("gl: DrawSDFQuads not implemented (Task 7)")
}

// PushClip pushes a new clip rectangle that intersects with the current clip.
func (rd *Renderer) PushClip(r render.Rect) {
	panic("gl: PushClip not implemented (Task 5)")
}

// PopClip pops the current clip rectangle.
func (rd *Renderer) PopClip() {
	panic("gl: PopClip not implemented (Task 5)")
}
