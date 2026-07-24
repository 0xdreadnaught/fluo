package render

// TextureID uniquely identifies a texture, with 0 meaning no texture.
type TextureID uint32

// NoTexture is a TextureID representing no texture.
const NoTexture TextureID = 0

// GlyphQuad represents a glyph quad with destination and source rectangles.
type GlyphQuad struct {
	Dst, Src Rect
}

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

	// DrawGradientRect fills r with a linear gradient from `from` to `to`:
	// horizontal (left->right) when horizontal is true, else vertical (top->bottom).
	DrawGradientRect(r Rect, from, to Color, horizontal bool)

	// StrokeRoundedRect draws the stroke of a rectangle with rounded corners (stroke inside edge).
	StrokeRoundedRect(r Rect, radius, width float32, c Color)

	// DrawShadow draws a soft shadow of a rounded rectangle.
	DrawShadow(r Rect, radius, blur float32, c Color)

	// DrawBackdropBlur draws an acrylic/mica-style backdrop-blur surface:
	// it snapshots whatever has already been drawn beneath r, blurs it, and
	// composites the result back into r (rounded by radius) tinted by c —
	// approximating WinUI's acrylic material. Because it samples already-
	// rendered content, callers must invoke it AFTER painting whatever
	// should show through, and BEFORE drawing anything meant to sit on top
	// of the acrylic surface (e.g. its children). An implementation that
	// cannot obtain a true mid-frame snapshot may instead degrade to a
	// flat, tinted, translucent rounded fill (equivalent to
	// FillRoundedRect(r, radius, c)); any such degrade must be prominently
	// documented at the implementation site.
	DrawBackdropBlur(r Rect, radius float32, tint Color)

	// CreateTexture creates a new texture from RGBA8 data.
	// The rgba slice must have length w*h*4; if rgba is non-nil and shorter
	// than that, implementations panic. A nil rgba allocates storage without
	// uploading pixels.
	CreateTexture(w, h int, rgba []byte) TextureID

	// UpdateTexture updates a region of an existing texture.
	// The rgba slice must have length w*h*4; implementations panic if it is
	// shorter (rgba must not be nil here, unlike CreateTexture).
	UpdateTexture(id TextureID, x, y, w, h int, rgba []byte)

	// DeleteTexture frees a texture created by CreateTexture; NoTexture is a
	// no-op.
	DeleteTexture(id TextureID)

	// DrawQuad draws a textured quad with the source rectangle in UV coordinates (0..1) and a tint color.
	DrawQuad(dst, src Rect, tex TextureID, tint Color)

	// DrawSDFQuads draws glyphs from an SDF alpha atlas with the given color.
	DrawSDFQuads(quads []GlyphQuad, tex TextureID, c Color)

	// PushClip pushes a new clip rectangle that intersects with the current clip.
	PushClip(r Rect)

	// PopClip pops the current clip rectangle.
	PopClip()
}
