package gl

import (
	"fmt"

	"github.com/go-gl/gl/v3.3-core/gl"
)

const vertexShaderSrc = `#version 330 core
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
` + "\x00"

const fragmentShaderSrc = `#version 330 core
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
    } else if (vMode == 7) {
        // Direct grayscale-AA text: the coverage mask already IS the
        // antialiasing (no smoothstep/fwidth needed, unlike mode 2's SDF).
        c.a *= texture(uTex, vUV).r;
    } else if (vMode >= 3) {
        float d = sdRoundBox(vPos - vRect.xy, vRect.zw, vExtra.x);
        // AA band width is derived from the screen-space derivative of d
        // (fwidth), the same principle mode 2's SDF text path uses, rather
        // than a fixed logical-unit width — so rounded-rect/stroke/shadow
        // edges stay ~1 device pixel wide at any scale (crisp at scale>1,
        // no sub-pixel aliasing at scale<1).
        float aa = fwidth(d);
        if (vMode == 3) c.a *= clamp(0.5 - d/aa, 0.0, 1.0);
        else if (vMode == 4) {
            float w = vExtra.y;
            float ds = abs(d + w*0.5) - w*0.5;
            c.a *= clamp(0.5 - ds/fwidth(ds), 0.0, 1.0);
        }
        else if (vMode == 5) {
            // Shadow's blur (vExtra.y) is an intentional, resolution-
            // independent soft falloff and stays as the transition
            // half-width; it's only floored at the screen-space AA width so
            // a near-zero blur still antialiases instead of producing a
            // hard, scale-dependent edge.
            float halfWidth = max(vExtra.y, aa);
            c.a *= 1.0 - smoothstep(-halfWidth, halfWidth, d);
        }
        else {
            // Mode 6: backdrop-blur acrylic composite. uTex holds the
            // already-blurred backdrop snapshot; mix in the tint color by
            // its own alpha, then use the rounded-rect SDF only to mask the
            // corners (the panel itself is opaque wherever it covers).
            vec4 tex = texture(uTex, vUV);
            c.rgb = mix(tex.rgb, vColor.rgb, vColor.a);
            c.a = clamp(0.5 - d/aa, 0.0, 1.0);
        }
    }
    if (c.a <= 0.001) discard;
    frag = c;
}
` + "\x00"

// compile compiles a shader of the given kind (gl.VERTEX_SHADER or
// gl.FRAGMENT_SHADER) from src, which must be a NUL-terminated GLSL source
// string. On failure it returns an error including the GL info log.
func compile(src string, kind uint32) (uint32, error) {
	shader := gl.CreateShader(kind)
	csrc, free := gl.Strs(src)
	defer free()
	length := int32(len(src) - 1) // exclude the trailing NUL
	gl.ShaderSource(shader, 1, csrc, &length)
	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLen)
		gl.DeleteShader(shader)
		if logLen == 0 {
			return 0, fmt.Errorf("gl: shader compile failed: no info log")
		}
		buf := make([]byte, logLen+1)
		gl.GetShaderInfoLog(shader, logLen, nil, &buf[0])
		return 0, fmt.Errorf("gl: shader compile failed: %s", string(buf[:logLen]))
	}
	return shader, nil
}

// link links a vertex shader and fragment shader into a program, deleting
// the shaders afterward. On failure it returns an error including the GL
// info log.
func link(vs, fs uint32) (uint32, error) {
	prog := gl.CreateProgram()
	gl.AttachShader(prog, vs)
	gl.AttachShader(prog, fs)
	gl.LinkProgram(prog)
	gl.DeleteShader(vs)
	gl.DeleteShader(fs)

	var status int32
	gl.GetProgramiv(prog, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetProgramiv(prog, gl.INFO_LOG_LENGTH, &logLen)
		gl.DeleteProgram(prog)
		if logLen == 0 {
			return 0, fmt.Errorf("gl: program link failed: no info log")
		}
		buf := make([]byte, logLen+1)
		gl.GetProgramInfoLog(prog, logLen, nil, &buf[0])
		return 0, fmt.Errorf("gl: program link failed: %s", string(buf[:logLen]))
	}
	return prog, nil
}
