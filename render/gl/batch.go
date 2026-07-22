package gl

import (
	"github.com/go-gl/gl/v3.3-core/gl"

	"github.com/0xdreadnaught/fluo/render"
)

const (
	floatsPerVert = 15
	maxVerts      = 16384
)

// Renderer is a batching OpenGL 3.3-core implementation of render.Renderer.
// It accumulates vertices for a single uber-shader program and flushes them
// in triangle batches whenever the bound texture changes, the clip changes,
// the vertex buffer fills up, or the frame ends.
type Renderer struct {
	prog, vao, vbo        uint32
	locViewport, locScale int32
	verts                 []float32
	curTex, whiteTex      uint32
	fbW, fbH              int
	scale                 float32
	clips                 []render.Rect
}

var _ render.Renderer = (*Renderer)(nil)

// vert appends one interleaved vertex (15 floats) to the pending batch.
func (rd *Renderer) vert(pos, uv [2]float32, c render.Color, rect [4]float32, extra [2]float32, mode float32) {
	rd.verts = append(rd.verts,
		pos[0], pos[1],
		uv[0], uv[1],
		c.R, c.G, c.B, c.A,
		rect[0], rect[1], rect[2], rect[3],
		extra[0], extra[1],
		mode,
	)
}

// quad emits two triangles (TL,BL,BR + TL,BR,TR) covering dst in position
// space and src in UV space, flushing first if the texture is changing or
// the buffer is full. Winding order is unspecified and irrelevant: face
// culling is never enabled.
func (rd *Renderer) quad(mode float32, dst, src render.Rect, c render.Color, rc [4]float32, ex [2]float32, tex uint32) {
	if tex != rd.curTex {
		rd.flush()
		rd.curTex = tex
	}
	if len(rd.verts)/floatsPerVert+6 > maxVerts {
		rd.flush()
	}

	dstTL := [2]float32{dst.X, dst.Y}
	dstTR := [2]float32{dst.Right(), dst.Y}
	dstBL := [2]float32{dst.X, dst.Bottom()}
	dstBR := [2]float32{dst.Right(), dst.Bottom()}

	uvTL := [2]float32{src.X, src.Y}
	uvTR := [2]float32{src.Right(), src.Y}
	uvBL := [2]float32{src.X, src.Bottom()}
	uvBR := [2]float32{src.Right(), src.Bottom()}

	rd.vert(dstTL, uvTL, c, rc, ex, mode)
	rd.vert(dstBL, uvBL, c, rc, ex, mode)
	rd.vert(dstBR, uvBR, c, rc, ex, mode)

	rd.vert(dstTL, uvTL, c, rc, ex, mode)
	rd.vert(dstBR, uvBR, c, rc, ex, mode)
	rd.vert(dstTR, uvTR, c, rc, ex, mode)
}

// flush uploads the pending vertices and issues a single draw call, then
// resets the batch. It is a no-op when there is nothing pending.
func (rd *Renderer) flush() {
	if len(rd.verts) == 0 {
		return
	}

	gl.BindVertexArray(rd.vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, rd.vbo)
	gl.BufferSubData(gl.ARRAY_BUFFER, 0, len(rd.verts)*4, gl.Ptr(rd.verts))

	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, rd.curTex)

	gl.DrawArrays(gl.TRIANGLES, 0, int32(len(rd.verts)/floatsPerVert))

	rd.verts = rd.verts[:0]
}
