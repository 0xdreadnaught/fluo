// Package render defines fluo's abstract 2D drawing surface: the Renderer
// interface (FillRect/FillRoundedRect/StrokeRoundedRect/DrawShadow/
// DrawBackdropBlur/DrawQuad/DrawGlyphs plus texture management and clip
// push/pop) and the plain-data geometry types every other package builds
// on — Point, Size, Rect (with Thickness-based Inset), and Color (0..1
// float components, with RGB/RGBA byte-based constructors). It has no
// dependency on any particular graphics API; render/gl is the OpenGL 3.3
// implementation of Renderer that the app package wires up, and core's
// layout engine, text's glyph drawing, and every controls widget speak
// only in terms of the types and interface defined here.
package render
