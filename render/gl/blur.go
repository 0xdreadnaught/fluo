package gl

import (
	"fmt"

	"github.com/go-gl/gl/v3.3-core/gl"

	"github.com/0xdreadnaught/fluo/render"
)

// DrawBackdropBlur implements render.Renderer's acrylic/mica backdrop-blur
// surface with a REAL mid-frame snapshot-and-blur, not a tint degrade:
//
//  1. Flush the pending batch so whatever has already been drawn beneath r
//     is actually present in the bound framebuffer.
//  2. glCopyTexImage2D the device-pixel region under r out of that
//     framebuffer into a plain "snapshot" texture (no read-back to the CPU;
//     it stays a GPU texture the whole time).
//  3. Run a separable 9-tap Gaussian blur (horizontal pass, then vertical)
//     into two small scratch FBOs sized exactly to the snapshot.
//  4. Composite the blurred result back into r via a dedicated uber-shader
//     mode (6, see shader.go) that rounds the corners by radius and mixes
//     in tint by the tint's own alpha, giving the "frosted glass" look.
//
// This only degrades (to FillRoundedRect(r, radius, tint), i.e. a flat
// tinted fill with no blur) in two narrow cases, both cheaper to check than
// to crash a frame over: the blur shader itself fails to compile/link on
// the current driver (extremely unlikely for GLSL this simple on a 3.3-core
// context), or one of the two scratch FBOs allocated below comes back
// incomplete (e.g. a driver refusing RGBA8 as a color-renderable format —
// also extremely unlikely, but checked rather than assumed). Either sets
// rd.blurFailed so every subsequent call on this Renderer takes the degrade
// path directly, skipping the GPU work above. On any GL 3.3-core context
// this has actually been exercised on (see TestAcrylic's golden), the real
// blur path runs.
func (rd *Renderer) DrawBackdropBlur(r render.Rect, radius float32, tint render.Color) {
	radius = clampRadius(r, radius)

	// Whatever should show through the acrylic must already be on the GPU
	// framebuffer before we snapshot it.
	rd.flush()

	scale := rd.scale
	x0 := int32(r.X * scale)
	y0 := int32(float32(rd.fbH) - (r.Y+r.H)*scale) // bottom-left GL origin, matches applyClip
	w := int32(r.W * scale)
	h := int32(r.H * scale)

	// Clamp the source region to the framebuffer; a rect straddling or
	// outside it just samples whatever fraction is valid, and a rect
	// entirely outside it has nothing to draw.
	if x0 < 0 {
		w += x0
		x0 = 0
	}
	if y0 < 0 {
		h += y0
		y0 = 0
	}
	if x0+w > int32(rd.fbW) {
		w = int32(rd.fbW) - x0
	}
	if y0+h > int32(rd.fbH) {
		h = int32(rd.fbH) - y0
	}
	if w <= 0 || h <= 0 {
		return
	}

	if rd.blur == nil && !rd.blurFailed {
		bp, err := newBlurProgram()
		if err != nil {
			rd.blurFailed = true
		} else {
			rd.blur = bp
		}
	}
	if rd.blurFailed {
		rd.FillRoundedRect(r, radius, tint)
		return
	}

	var prevFBO int32
	gl.GetIntegerv(gl.FRAMEBUFFER_BINDING, &prevFBO)

	// 1. Snapshot the framebuffer region under r into a plain texture.
	var snapTex uint32
	gl.GenTextures(1, &snapTex)
	gl.BindTexture(gl.TEXTURE_2D, snapTex)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.CopyTexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, x0, y0, w, h, 0)

	// 2. Two scratch FBOs for the horizontal/vertical separable passes.
	fboA, texA, completeA := makeScratchFBO(w, h)
	fboB, texB, completeB := makeScratchFBO(w, h)

	if !completeA || !completeB {
		// Driver rejected one of the scratch FBOs (e.g. RGBA8 not
		// color-renderable on this GL implementation) — same degrade as a
		// blur-program compile/link failure, latched so later calls skip
		// straight to it. Tear down everything allocated above first: the
		// framebuffer binding is still fboB (makeScratchFBO leaves its FBO
		// bound), so restore prevFBO before anyone draws again.
		rd.blurFailed = true
		gl.BindFramebuffer(gl.FRAMEBUFFER, uint32(prevFBO))
		gl.DeleteFramebuffers(1, &fboA)
		gl.DeleteFramebuffers(1, &fboB)
		gl.DeleteTextures(1, &snapTex)
		gl.DeleteTextures(1, &texA)
		gl.DeleteTextures(1, &texB)
		rd.FillRoundedRect(r, radius, tint)
		return
	}

	gl.Disable(gl.SCISSOR_TEST) // the blur passes render into their own small FBOs, unclipped
	gl.Disable(gl.BLEND)        // opaque resample; no blending wanted

	// spread is the texel spacing between taps: a fixed softening amount
	// (DrawBackdropBlur's radius parameter is the corner radius, not a blur
	// kernel radius, so the blur strength itself is a constant).
	const spread = 2.5
	rd.blur.pass(snapTex, fboA, w, h, [2]float32{spread / float32(w), 0})
	rd.blur.pass(texA, fboB, w, h, [2]float32{0, spread / float32(h)})

	// Restore the real frame target and the batch's GL state. In
	// particular, blurProgram.pass switched the active program away from
	// the main uber-shader (rd.prog) and left program 0 bound afterward —
	// re-bind rd.prog or the composite quad below draws with no program
	// bound at all.
	gl.BindFramebuffer(gl.FRAMEBUFFER, uint32(prevFBO))
	gl.Viewport(0, 0, int32(rd.fbW), int32(rd.fbH))
	gl.UseProgram(rd.prog)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	rd.applyClip()

	// 3. Composite: draw texB (the blurred backdrop) into r, rounded by
	// radius, mixing in tint (mode 6). The snapshot/blur textures were
	// filled via CopyTexImage2D straight from the (bottom-up) framebuffer,
	// the opposite vertical convention from CreateTexture-uploaded textures
	// (whose row 0 is understood to be the visual top everywhere else in
	// this renderer) — so the source rect's V axis is flipped here
	// (Y:1, H:-1) to undo that and land the blurred image right-side up.
	src := render.Rect{X: 0, Y: 1, W: 1, H: -1}
	rd.quad(6, r, src, tint, rectParams(r), [2]float32{radius, 0}, texB)
	rd.flush() // issue the draw now: texA/texB/snapTex are about to be deleted

	gl.DeleteFramebuffers(1, &fboA)
	gl.DeleteFramebuffers(1, &fboB)
	gl.DeleteTextures(1, &snapTex)
	gl.DeleteTextures(1, &texA)
	gl.DeleteTextures(1, &texB)

	// rd.curTex is left pointing at the now-deleted texB; reset it to the
	// always-valid white texture so no later CreateTexture/UpdateTexture
	// restore-bind (or batch flush) rebinds a stale name.
	rd.curTex = rd.whiteTex
	gl.BindTexture(gl.TEXTURE_2D, rd.curTex)
}

// makeScratchFBO allocates a w x h RGBA8 texture and an FBO with it bound as
// COLOR_ATTACHMENT0, for use as an intermediate blur-pass render target.
// complete reports whether the driver actually accepted the attachment
// (gl.CheckFramebufferStatus == gl.FRAMEBUFFER_COMPLETE) — the caller must
// check it and degrade rather than render into (or sample from) an
// incomplete FBO, whose contents are undefined.
func makeScratchFBO(w, h int32) (fbo, tex uint32, complete bool) {
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, w, h, 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

	gl.GenFramebuffers(1, &fbo)
	gl.BindFramebuffer(gl.FRAMEBUFFER, fbo)
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, tex, 0)
	status := gl.CheckFramebufferStatus(gl.FRAMEBUFFER)
	return fbo, tex, status == gl.FRAMEBUFFER_COMPLETE
}

// blurProgram is a small standalone GL program (separate from the main
// uber-shader) that runs one direction of a separable 9-tap Gaussian blur
// over a fullscreen NDC quad. It is compiled lazily, once, on a Renderer's
// first DrawBackdropBlur call.
type blurProgram struct {
	prog           uint32
	vao, vbo       uint32
	locTex, locDir int32
}

const blurVertexShaderSrc = `#version 330 core
layout(location=0) in vec2 aPos; // NDC, covers the whole viewport
layout(location=1) in vec2 aUV;
out vec2 vUV;
void main(){
    vUV = aUV;
    gl_Position = vec4(aPos, 0.0, 1.0);
}
` + "\x00"

const blurFragmentShaderSrc = `#version 330 core
in vec2 vUV;
uniform sampler2D uTex;
uniform vec2 uDir; // per-tap texel offset, e.g. (spread/w, 0) or (0, spread/h)
out vec4 frag;
void main(){
    vec4 result = texture(uTex, vUV) * 0.2270270270;
    result += texture(uTex, vUV + uDir * 1.0) * 0.1945945946;
    result += texture(uTex, vUV - uDir * 1.0) * 0.1945945946;
    result += texture(uTex, vUV + uDir * 2.0) * 0.1216216216;
    result += texture(uTex, vUV - uDir * 2.0) * 0.1216216216;
    result += texture(uTex, vUV + uDir * 3.0) * 0.0540540541;
    result += texture(uTex, vUV - uDir * 3.0) * 0.0540540541;
    result += texture(uTex, vUV + uDir * 4.0) * 0.0162162162;
    result += texture(uTex, vUV - uDir * 4.0) * 0.0162162162;
    frag = result;
}
` + "\x00"

// newBlurProgram compiles and links the blur shader and allocates a static
// fullscreen NDC quad (position+UV interleaved) to draw it with.
func newBlurProgram() (*blurProgram, error) {
	vs, err := compile(blurVertexShaderSrc, gl.VERTEX_SHADER)
	if err != nil {
		return nil, fmt.Errorf("gl: compile blur vertex shader: %w", err)
	}
	fs, err := compile(blurFragmentShaderSrc, gl.FRAGMENT_SHADER)
	if err != nil {
		return nil, fmt.Errorf("gl: compile blur fragment shader: %w", err)
	}
	prog, err := link(vs, fs)
	if err != nil {
		return nil, fmt.Errorf("gl: link blur program: %w", err)
	}

	bp := &blurProgram{prog: prog}
	bp.locTex = gl.GetUniformLocation(prog, gl.Str("uTex\x00"))
	bp.locDir = gl.GetUniformLocation(prog, gl.Str("uDir\x00"))

	// pos.xy (NDC), uv.xy — 2 triangles covering the whole viewport.
	verts := []float32{
		-1, 1, 0, 1,
		-1, -1, 0, 0,
		1, -1, 1, 0,
		-1, 1, 0, 1,
		1, -1, 1, 0,
		1, 1, 1, 1,
	}
	gl.GenVertexArrays(1, &bp.vao)
	gl.GenBuffers(1, &bp.vbo)
	gl.BindVertexArray(bp.vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, bp.vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(verts)*4, gl.Ptr(verts), gl.STATIC_DRAW)
	const stride = int32(4 * 4)
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(0, 2, gl.FLOAT, false, stride, 0)
	gl.EnableVertexAttribArray(1)
	gl.VertexAttribPointerWithOffset(1, 2, gl.FLOAT, false, stride, 2*4)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	gl.BindVertexArray(0)

	gl.UseProgram(prog)
	gl.Uniform1i(bp.locTex, 0)
	gl.UseProgram(0)

	return bp, nil
}

// pass renders one direction of the separable blur: it samples srcTex and
// writes the result into dstFBO (a w x h scratch FBO from makeScratchFBO),
// offsetting each tap by dir texels.
func (bp *blurProgram) pass(srcTex, dstFBO uint32, w, h int32, dir [2]float32) {
	gl.BindFramebuffer(gl.FRAMEBUFFER, dstFBO)
	gl.Viewport(0, 0, w, h)

	gl.UseProgram(bp.prog)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, srcTex)
	gl.Uniform2f(bp.locDir, dir[0], dir[1])

	gl.BindVertexArray(bp.vao)
	gl.DrawArrays(gl.TRIANGLES, 0, 6)
	gl.BindVertexArray(0)
	gl.UseProgram(0)
}
