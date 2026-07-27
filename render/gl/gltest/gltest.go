// Package gltest is a golden-image test harness for OpenGL rendering code.
// It creates a hidden GLFW window with a 3.3-core context, gives the caller
// an offscreen FBO to render into, and can compare the rendered pixels
// against a golden PNG on disk.
//
// This is the only package in fluo allowed to import go-gl/glfw and
// go-gl/gl: production rendering code must stay windowing-library agnostic,
// but tests need a real context to exercise real GL calls.
//
// Threading: GL contexts are thread-affine. Run locks the calling
// goroutine to its OS thread for the duration of the test and must not be
// used with t.Parallel(). On macOS, GLFW additionally requires all
// windowing/event calls to happen on the process's main thread, which
// LockOSThread alone does not guarantee (that needs runtime.GOMAXPROCS-style
// main-thread pinning, e.g. via mainthread helpers) — this harness does not
// implement that yet, so it is not expected to work on macOS. On
// Windows, Linux, and WSLg (X11/XWayland) a locked, non-main thread is
// sufficient and this harness works as-is.
package gltest

import (
	"flag"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-gl/gl/v3.3-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

var update = flag.Bool("update", false, "rewrite golden images")

// Framebuffer is an offscreen render target set up by Run.
type Framebuffer struct {
	W, H     int
	FBO, Tex uint32
}

// Image reads back the framebuffer's color attachment as an *image.RGBA.
// GL's framebuffer origin is bottom-left, so rows are flipped to put the
// image in standard top-down image-space order. Alpha is forced to 255:
// an RGBA8 FBO attachment can hold junk/undefined alpha depending on the
// driver, and golden comparisons should only care about color.
func (f *Framebuffer) Image() *image.RGBA {
	gl.BindFramebuffer(gl.FRAMEBUFFER, f.FBO)

	buf := make([]byte, f.W*f.H*4)
	gl.ReadPixels(0, 0, int32(f.W), int32(f.H), gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(&buf[0]))

	img := image.NewRGBA(image.Rect(0, 0, f.W, f.H))
	stride := f.W * 4
	for y := 0; y < f.H; y++ {
		srcRow := buf[(f.H-1-y)*stride : (f.H-1-y)*stride+stride]
		dstRow := img.Pix[y*img.Stride : y*img.Stride+stride]
		copy(dstRow, srcRow)
		for x := 0; x < f.W; x++ {
			dstRow[x*4+3] = 255
		}
	}
	return img
}

// Run creates a hidden 3.3-core GL context and a w x h offscreen FBO, then
// calls frame with a Framebuffer describing it. The context and window are
// torn down when Run returns. If no GL context can be created (no display,
// no driver, etc.) the test is skipped rather than failed.
func Run(t *testing.T, w, h int, frame func(fb *Framebuffer)) {
	t.Helper()
	runtime.LockOSThread() // GL is thread-affine; tests must not use t.Parallel()

	// Context bring-up fails on a headless machine (no display, no GL driver).
	// This glfw binding logs platform errors from Init and returns nil rather
	// than an error (go-gl/glfw issue 127), so a failed Init only surfaces
	// when the next call panics with "not initialized". Guard the whole setup
	// phase and skip — rather than fail — on any such failure. frame runs only
	// after ctxReady is set, so genuine panics in the render body are never
	// swallowed here.
	ctxReady := false
	defer func() {
		if !ctxReady {
			if r := recover(); r != nil {
				t.Skipf("no GL context available: %v", r)
			}
		}
	}()

	if err := glfw.Init(); err != nil {
		t.Skipf("no GL context available: %v", err)
	}
	defer glfw.Terminate()

	glfw.WindowHint(glfw.ContextVersionMajor, 3)
	glfw.WindowHint(glfw.ContextVersionMinor, 3)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
	glfw.WindowHint(glfw.Visible, glfw.False)

	win, err := glfw.CreateWindow(64, 64, "gltest", nil, nil)
	if err != nil {
		t.Skipf("no GL window: %v", err)
	}
	defer win.Destroy()

	win.MakeContextCurrent()

	if err := gl.Init(); err != nil {
		t.Fatalf("gl.Init: %v", err)
	}

	var fbo, tex uint32
	gl.GenFramebuffers(1, &fbo)
	gl.BindFramebuffer(gl.FRAMEBUFFER, fbo)

	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, int32(w), int32(h), 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, tex, 0)

	if gl.CheckFramebufferStatus(gl.FRAMEBUFFER) != gl.FRAMEBUFFER_COMPLETE {
		t.Fatal("FBO incomplete")
	}

	gl.Viewport(0, 0, int32(w), int32(h))

	ctxReady = true
	frame(&Framebuffer{W: w, H: h, FBO: fbo, Tex: tex})
}

// CheckGolden compares got against the golden image stored at
// testdata/<name>.png, relative to the test package's working directory.
//
// With -update, the golden is (re)written from got and the check always
// passes. Otherwise the golden is decoded and compared to got: dimensions
// must match exactly, and every channel of every pixel must be within 3 of
// the golden's value. On any mismatch, got is written to
// testdata/<name>.got.png (already git-ignored) for inspection, and the
// test fails reporting the first differing pixel.
func CheckGolden(t *testing.T, name string, got *image.RGBA) {
	t.Helper()

	path := filepath.Join("testdata", name+".png")

	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("MkdirAll testdata: %v", err)
		}
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("create golden %s: %v", path, err)
		}
		defer f.Close()
		if err := png.Encode(f, got); err != nil {
			t.Fatalf("encode golden %s: %v", path, err)
		}
		t.Logf("golden written: %s", path)
		return
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("golden missing: run with -update (%v)", err)
	}
	defer f.Close()

	wantImg, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode golden %s: %v", path, err)
	}
	want, ok := wantImg.(*image.RGBA)
	if !ok {
		nrgba := image.NewRGBA(wantImg.Bounds())
		for y := wantImg.Bounds().Min.Y; y < wantImg.Bounds().Max.Y; y++ {
			for x := wantImg.Bounds().Min.X; x < wantImg.Bounds().Max.X; x++ {
				nrgba.Set(x, y, wantImg.At(x, y))
			}
		}
		want = nrgba
	}

	gb := got.Bounds()
	wb := want.Bounds()
	if gb.Dx() != wb.Dx() || gb.Dy() != wb.Dy() {
		t.Errorf("golden %s: dimension mismatch: got %dx%d, want %dx%d", name, gb.Dx(), gb.Dy(), wb.Dx(), wb.Dy())
		writeGot(t, name, got)
		return
	}

	const tol = 3
	mismatch := false
	var fx, fy int
	var fgot, fwant [4]uint8
	for y := 0; y < gb.Dy(); y++ {
		for x := 0; x < gb.Dx(); x++ {
			gi := got.PixOffset(gb.Min.X+x, gb.Min.Y+y)
			wi := want.PixOffset(wb.Min.X+x, wb.Min.Y+y)
			gp := got.Pix[gi : gi+4]
			wp := want.Pix[wi : wi+4]
			for c := 0; c < 4; c++ {
				if absDiff(gp[c], wp[c]) > tol {
					if !mismatch {
						mismatch = true
						fx, fy = x, y
						copy(fgot[:], gp)
						copy(fwant[:], wp)
					}
				}
			}
		}
	}

	if mismatch {
		t.Errorf("golden %s: pixel mismatch at (%d,%d): got %v, want %v", name, fx, fy, fgot, fwant)
		writeGot(t, name, got)
	}
}

func writeGot(t *testing.T, name string, got *image.RGBA) {
	t.Helper()
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Errorf("MkdirAll testdata: %v", err)
		return
	}
	gotPath := filepath.Join("testdata", name+".got.png")
	f, err := os.Create(gotPath)
	if err != nil {
		t.Errorf("create %s: %v", gotPath, err)
		return
	}
	defer f.Close()
	if err := png.Encode(f, got); err != nil {
		t.Errorf("encode %s: %v", gotPath, err)
		return
	}
	t.Logf("wrote diff image: %s", gotPath)
}

func absDiff(a, b uint8) int {
	if a > b {
		return int(a) - int(b)
	}
	return int(b) - int(a)
}
