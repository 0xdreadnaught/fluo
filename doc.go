// Package fluo is the module root for github.com/0xdreadnaught/fluo, a
// retained-mode, classic Windows-styled GUI toolkit for OpenGL apps in Go. It
// declares no exported symbols of its own — every usable package lives in
// a subdirectory (render, text, core, input, theme, controls, bind, anim,
// timers, app) — this file exists solely to give `go doc .`/pkg.go.dev a
// landing-page overview, quickstart, and architecture map for the module
// as a whole.
//
// # Quickstart
//
// A minimal program: load a font, build one widget, and drive it from
// app.Run's per-frame callback.
//
//	font, _ := text.Load(goregular.TTF)
//	root := controls.NewTextBlock(text.NewFace(font, 16), "Hello, fluo!")
//
//	log.Fatal(app.Run(app.Config{Title: "hello", Width: 320, Height: 200}, func(c *app.Ctx) {
//		c.Input.SetRoot(root)
//		core.MeasureWidget(root, c.Size)
//		core.ArrangeWidget(root, render.Rect{W: c.Size.W, H: c.Size.H})
//		core.RenderWidget(root, c.R)
//	}))
//
// See examples/counter, examples/form, and examples/todo for complete,
// runnable programs, and cmd/fluo-gallery for every control composed
// together.
//
// # Architecture
//
// fluo is layered bottom to top; each layer only depends on the ones
// below it:
//
//	render    — the Renderer interface and geometry primitives (Color/
//	            Point/Size/Rect); render/gl implements it on OpenGL 3.3.
//	text      — font loading and SDF glyph rasterization on top of render.
//	core      — the Widget/Element layout engine (Measure/Arrange/Render)
//	            and the reactive Property[T].
//	input     — hit-testing, event bubbling, capture, and focus over a
//	            core.Widget tree.
//	theme     — the color/metric/typography token model (Light/Dark).
//	controls  — the built-in widget set, styled from theme and wired to
//	            input.
//	bind      — one-way/two-way binding between core.Property[T]/
//	            bind.List[T] and controls.
//	anim/     — easing/Tween animation and the frame-tick timer service
//	timers      controls use for caret blink, tooltip dwell, and
//	            cross-fades.
//	app       — Run/Surface, the glfw window host (or embeddable surface)
//	            that ties every layer above together into a running
//	            program.
//
// go doc on any of the above (e.g. `go doc ./controls`) gives a
// one-paragraph summary of its role and key types.
package fluo
