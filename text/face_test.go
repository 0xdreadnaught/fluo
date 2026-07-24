package text

import (
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/0xdreadnaught/fluo/render"
)

// recordingRenderer is a minimal render.Renderer stub for exercising
// Face.Draw's crisp HD-text path without a GPU: it records DrawGlyphs and
// DrawSDFQuads calls (so a test can assert which one Draw actually used)
// and reports a fixed scale, matching how app.Surface would report the
// current frame's device-pixels-per-logical-pixel factor. Every other
// method is a no-op; Draw only calls Scale, DrawGlyphs, and (were it ever
// reached) DrawSDFQuads.
type recordingRenderer struct {
	scale      float32
	glyphCalls int
	glyphQuads []render.GlyphQuad
	sdfCalls   int
}

func (r *recordingRenderer) Begin(fbWidth, fbHeight int, scale float32)                       {}
func (r *recordingRenderer) End()                                                             {}
func (r *recordingRenderer) FillRect(rect render.Rect, c render.Color)                        {}
func (r *recordingRenderer) FillRoundedRect(rect render.Rect, radius float32, c render.Color) {}
func (r *recordingRenderer) DrawGradientRect(rect render.Rect, from, to render.Color, horizontal bool) {
}
func (r *recordingRenderer) StrokeRoundedRect(rect render.Rect, radius, width float32, c render.Color) {
}
func (r *recordingRenderer) DrawShadow(rect render.Rect, radius, blur float32, c render.Color) {}
func (r *recordingRenderer) DrawBackdropBlur(rect render.Rect, radius float32, tint render.Color) {
}
func (r *recordingRenderer) CreateTexture(w, h int, rgba []byte) render.TextureID           { return 1 }
func (r *recordingRenderer) UpdateTexture(id render.TextureID, x, y, w, h int, rgba []byte) {}
func (r *recordingRenderer) DeleteTexture(id render.TextureID)                              {}
func (r *recordingRenderer) DrawQuad(dst, src render.Rect, tex render.TextureID, tint render.Color) {
}
func (r *recordingRenderer) DrawSDFQuads(quads []render.GlyphQuad, tex render.TextureID, c render.Color) {
	r.sdfCalls++
}
func (r *recordingRenderer) DrawGlyphs(quads []render.GlyphQuad, tex render.TextureID, c render.Color) {
	r.glyphCalls++
	r.glyphQuads = quads
}
func (r *recordingRenderer) Scale() float32 {
	if r.scale <= 0 {
		return 1
	}
	return r.scale
}
func (r *recordingRenderer) PushClip(rect render.Rect) {}
func (r *recordingRenderer) PopClip()                  {}

var _ render.Renderer = (*recordingRenderer)(nil)

// visibleGlyphCount returns the number of runes of s that have non-empty
// coverage at fa's device px for the given scale — i.e. the quad count
// Face.Draw is expected to emit (spaces and other blank glyphs advance the
// pen but contribute no quad).
func visibleGlyphCount(t *testing.T, fa *Face, s string, scale float32) int {
	t.Helper()
	atlas := fa.Font.sharedAtlas()
	px := int(fa.SizePx*scale + 0.5)
	if px < 1 {
		px = 1
	}
	n := 0
	for _, ch := range s {
		gi, _ := fa.Font.glyphIndex(ch)
		e, err := atlas.glyphCoverage(gi, px)
		if err == nil && !e.empty {
			n++
		}
	}
	return n
}

// TestFaceDrawUsesDrawGlyphs is the HD-text Face.Draw test: it asserts Draw
// calls DrawGlyphs (never DrawSDFQuads — SDF is retained for future
// scaled/animated text but is no longer Face.Draw's default path), the
// emitted quad count equals the visible-glyph count (spaces skipped), and
// every quad's device origin (Dst.X*scale, Dst.Y*scale) is pixel-snapped
// (integer-valued, within float rounding) at both scale 1 and scale 2.
func TestFaceDrawUsesDrawGlyphs(t *testing.T) {
	f, err := Load(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	fa := NewFace(f, 14)
	const s = "Hi fluo"

	for _, scale := range []float32{1, 2} {
		rr := &recordingRenderer{scale: scale}
		fa.Draw(rr, render.Point{X: 3.25, Y: 5.5}, s, render.RGB(255, 255, 255))

		if rr.sdfCalls != 0 {
			t.Errorf("scale=%v: DrawSDFQuads called %d times; Face.Draw must use DrawGlyphs, not the SDF path", scale, rr.sdfCalls)
		}
		if rr.glyphCalls != 1 {
			t.Fatalf("scale=%v: DrawGlyphs called %d times, want 1", scale, rr.glyphCalls)
		}

		want := visibleGlyphCount(t, fa, s, scale)
		if len(rr.glyphQuads) != want {
			t.Errorf("scale=%v: quad count = %d, want %d (visible glyphs)", scale, len(rr.glyphQuads), want)
		}

		for i, q := range rr.glyphQuads {
			dx, dy := q.Dst.X*scale, q.Dst.Y*scale
			if rdx := dx - float32(int(dx+0.5)); rdx > 1e-3 || rdx < -1e-3 {
				t.Errorf("scale=%v quad[%d]: Dst.X*scale = %v not pixel-snapped", scale, i, dx)
			}
			if rdy := dy - float32(int(dy+0.5)); rdy > 1e-3 || rdy < -1e-3 {
				t.Errorf("scale=%v quad[%d]: Dst.Y*scale = %v not pixel-snapped", scale, i, dy)
			}
		}
	}
}

func TestMeasure(t *testing.T) {
	f, _ := Load(goregular.TTF)
	fa := NewFace(f, 16)
	m1, m2 := fa.Measure("M"), fa.Measure("MM")
	if m1.W <= 0 || m2.W <= m1.W {
		t.Errorf("M=%v MM=%v", m1, m2)
	}
	if lh := fa.LineHeight(); lh < 16 || lh > 26 {
		t.Errorf("LineHeight=%v", lh)
	}
	if fa.Measure("").W != 0 {
		t.Error("empty string width != 0")
	}
}
