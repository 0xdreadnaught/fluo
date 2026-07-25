// Package app provides fluo's desktop window host: Run opens a GLFW window
// with an OpenGL 3.3 core context, wires up render/gl's Renderer, and
// drives a frame(*Ctx) callback every vsync until the window closes. Ctx
// carries everything a frame callback needs — R (the Renderer), Size/Scale
// (logical window size and device-pixel scale factor), Mouse/Input (the
// input.Router driving the widget tree), Timers, and Close/Minimize/
// ToggleMaximize/BeginDrag hooks for wiring a controls.TitleBar when
// Config.Undecorated opts out of OS window chrome. Surface is the lower-
// level building block Run itself is built on: it owns layout, rendering,
// an input.Router, and a timers.Queue against a caller-supplied GL context,
// for embedding fluo into a host that already owns its own window and frame
// loop instead of calling Run directly.
package app
