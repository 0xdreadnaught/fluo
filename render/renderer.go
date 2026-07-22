package render

// Renderer is the interface for rendering graphics operations.
type Renderer interface {
	// Begin starts a new frame with the given framebuffer dimensions (in device pixels) and scale factor.
	Begin(fbWidth, fbHeight int, scale float32)

	// End flushes the current frame.
	End()

	// FillRect fills a rectangle with a solid color.
	FillRect(r Rect, c Color)

	// FillRoundedRect fills a rectangle with rounded corners with a solid color.
	FillRoundedRect(r Rect, radius float32, c Color)

	// StrokeRoundedRect draws the stroke of a rectangle with rounded corners (stroke inside edge).
	StrokeRoundedRect(r Rect, radius, width float32, c Color)

	// DrawShadow draws a soft shadow of a rounded rectangle.
	DrawShadow(r Rect, radius, blur float32, c Color)

	// CreateTexture creates a new texture from RGBA8 data.
	// The rgba slice must have length w*h*4.
	CreateTexture(w, h int, rgba []byte) TextureID

	// UpdateTexture updates a region of an existing texture.
	UpdateTexture(id TextureID, x, y, w, h int, rgba []byte)

	// DrawQuad draws a textured quad with the source rectangle in UV coordinates (0..1) and a tint color.
	DrawQuad(dst, src Rect, tex TextureID, tint Color)

	// DrawSDFQuads draws glyphs from an SDF alpha atlas with the given color.
	DrawSDFQuads(quads []GlyphQuad, tex TextureID, c Color)

	// PushClip pushes a new clip rectangle that intersects with the current clip.
	PushClip(r Rect)

	// PopClip pops the current clip rectangle.
	PopClip()
}
