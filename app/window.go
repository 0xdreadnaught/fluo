// Package app provides a minimal desktop window host: it opens a GLFW
// window with an OpenGL 3.3 core context, wires up render/gl's Renderer,
// and drives a frame(*Ctx) callback every vsync. It is intentionally thin
// — layout, input routing, and window chrome are later phases; this is
// just enough surface for the fluo-demo app and for Phase 2/3 to build on.
package app

import (
	"fmt"
	"runtime"

	"github.com/go-gl/gl/v3.3-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"

	"github.com/0xdreadnaught/fluo/render"
	glr "github.com/0xdreadnaught/fluo/render/gl"
)

func init() { runtime.LockOSThread() }

// Config describes the window to open.
type Config struct {
	Title         string
	Width, Height int // logical px
}

// MouseState is the mouse position and left-button state for the current
// frame, in logical pixels.
type MouseState struct {
	Pos  render.Point // logical px
	Down bool         // left button
}

// Ctx is passed to the frame callback every vsync.
type Ctx struct {
	R     render.Renderer
	Size  render.Size // logical px
	Scale float32
	Mouse MouseState
	Close func() // request app exit
}

// Run opens the window and calls frame every vsync until closed. Blocks.
// Must be called from main(); locks the OS thread.
func Run(cfg Config, frame func(*Ctx)) error {
	if err := glfw.Init(); err != nil {
		return fmt.Errorf("glfw: %w", err)
	}
	defer glfw.Terminate()

	glfw.WindowHint(glfw.ContextVersionMajor, 3)
	glfw.WindowHint(glfw.ContextVersionMinor, 3)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)

	win, err := glfw.CreateWindow(cfg.Width, cfg.Height, cfg.Title, nil, nil)
	if err != nil {
		return err
	}
	defer win.Destroy()

	win.MakeContextCurrent()
	if err := gl.Init(); err != nil {
		return err
	}
	glfw.SwapInterval(1)

	r, err := glr.New()
	if err != nil {
		return err
	}

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
			Size:  render.Size{W: float32(fbW) / sx, H: float32(fbH) / sx},
			Scale: sx,
			Mouse: MouseState{
				Pos:  render.Point{X: float32(mx), Y: float32(my)}, // glfw cursor pos is logical on Windows
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
