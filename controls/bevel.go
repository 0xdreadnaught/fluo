package controls

import (
	"github.com/0xdreadnaught/fluo/render"
	"github.com/0xdreadnaught/fluo/theme"
)

// drawRaised paints a 2px four-tone RAISED bevel with a flat `face` interior:
// outer top+left = ButtonHighlight, inner top+left = ButtonLight, inner
// bottom+right = ButtonShadow, outer bottom+right = ButtonDarkShadow. The
// interior is filled with `face` first, then the eight 1px edge rects are
// painted over it (top/left own the full width/height; bottom/right are
// drawn full-span too, so the highlight wins the top-left corner and the
// dark shadow wins the bottom-right corner, matching classic Windows-2000
// chrome).
func drawRaised(r render.Renderer, rect render.Rect, face render.Color, c theme.ColorTokens) {
	r.FillRect(rect, face)

	// outer edges
	r.FillRect(render.Rect{X: rect.X, Y: rect.Y, W: rect.W, H: 1}, c.ButtonHighlight)
	r.FillRect(render.Rect{X: rect.X, Y: rect.Y, W: 1, H: rect.H}, c.ButtonHighlight)
	r.FillRect(render.Rect{X: rect.X, Y: rect.Y + rect.H - 1, W: rect.W, H: 1}, c.ButtonDarkShadow)
	r.FillRect(render.Rect{X: rect.X + rect.W - 1, Y: rect.Y, W: 1, H: rect.H}, c.ButtonDarkShadow)

	// inner edges, inset by 1
	r.FillRect(render.Rect{X: rect.X + 1, Y: rect.Y + 1, W: rect.W - 2, H: 1}, c.ButtonLight)
	r.FillRect(render.Rect{X: rect.X + 1, Y: rect.Y + 1, W: 1, H: rect.H - 2}, c.ButtonLight)
	r.FillRect(render.Rect{X: rect.X + 1, Y: rect.Y + rect.H - 2, W: rect.W - 2, H: 1}, c.ButtonShadow)
	r.FillRect(render.Rect{X: rect.X + rect.W - 2, Y: rect.Y + 1, W: 1, H: rect.H - 2}, c.ButtonShadow)
}

// drawSunken paints the inverted 2px bevel (a recessed well): outer top+left
// = ButtonShadow, inner top+left = ButtonDarkShadow, inner bottom+right =
// ButtonLight, outer bottom+right = ButtonHighlight. The interior is filled
// with `fill` first, then the eight 1px edge rects are painted over it, in
// the same order/geometry as drawRaised with the tone roles swapped.
func drawSunken(r render.Renderer, rect render.Rect, fill render.Color, c theme.ColorTokens) {
	r.FillRect(rect, fill)

	// outer edges
	r.FillRect(render.Rect{X: rect.X, Y: rect.Y, W: rect.W, H: 1}, c.ButtonShadow)
	r.FillRect(render.Rect{X: rect.X, Y: rect.Y, W: 1, H: rect.H}, c.ButtonShadow)
	r.FillRect(render.Rect{X: rect.X, Y: rect.Y + rect.H - 1, W: rect.W, H: 1}, c.ButtonHighlight)
	r.FillRect(render.Rect{X: rect.X + rect.W - 1, Y: rect.Y, W: 1, H: rect.H}, c.ButtonHighlight)

	// inner edges, inset by 1
	r.FillRect(render.Rect{X: rect.X + 1, Y: rect.Y + 1, W: rect.W - 2, H: 1}, c.ButtonDarkShadow)
	r.FillRect(render.Rect{X: rect.X + 1, Y: rect.Y + 1, W: 1, H: rect.H - 2}, c.ButtonDarkShadow)
	r.FillRect(render.Rect{X: rect.X + 1, Y: rect.Y + rect.H - 2, W: rect.W - 2, H: 1}, c.ButtonLight)
	r.FillRect(render.Rect{X: rect.X + rect.W - 2, Y: rect.Y + 1, W: 1, H: rect.H - 2}, c.ButtonLight)
}

// drawGroove paints a 2px etched separator line down the center of rect: a
// ButtonShadow line then a ButtonHighlight line just past center.
// horizontal=true draws a horizontal rule spanning rect's width; otherwise a
// vertical rule spanning rect's height.
func drawGroove(r render.Renderer, rect render.Rect, horizontal bool, c theme.ColorTokens) {
	if horizontal {
		cy := rect.Y + rect.H/2 - 1
		r.FillRect(render.Rect{X: rect.X, Y: cy, W: rect.W, H: 1}, c.ButtonShadow)
		r.FillRect(render.Rect{X: rect.X, Y: cy + 1, W: rect.W, H: 1}, c.ButtonHighlight)
		return
	}
	cx := rect.X + rect.W/2 - 1
	r.FillRect(render.Rect{X: cx, Y: rect.Y, W: 1, H: rect.H}, c.ButtonShadow)
	r.FillRect(render.Rect{X: cx + 1, Y: rect.Y, W: 1, H: rect.H}, c.ButtonHighlight)
}

// drawScrollThumb paints a classic scrollbar's track and thumb, replacing
// the pre-restyle translucent rounded thumb (drawn alone, over whatever
// background showed through the gutter) shared by ScrollViewer and the
// virtualizer behind ListView/DataGrid: the track is a flat `c.ButtonFace`
// fill with a 1px `c.ButtonShadow` groove line on its inner edge (the edge
// bordering the scrolled content, i.e. `track`'s own left edge for a
// right-side vertical scrollbar) so it reads as a shallow well, then the
// thumb is painted as a solid raised bevel (drawRaised) so it stands out
// against the flat track. Geometry (both rects) is entirely the caller's —
// this only changes how they are painted.
func drawScrollThumb(r render.Renderer, track, thumb render.Rect, c theme.ColorTokens) {
	r.FillRect(track, c.ButtonFace)
	r.FillRect(render.Rect{X: track.X, Y: track.Y, W: 1, H: track.H}, c.ButtonShadow)
	drawRaised(r, thumb, c.ButtonFace, c)
}

// drawScrollThumbH is the horizontal analogue of drawScrollThumb: the opaque
// track fill, a 1px shadow along the track's TOP edge (where the vertical one
// puts it on the left), and the raised thumb.
func drawScrollThumbH(r render.Renderer, track, thumb render.Rect, c theme.ColorTokens) {
	r.FillRect(track, c.ButtonFace)
	r.FillRect(render.Rect{X: track.X, Y: track.Y, W: track.W, H: 1}, c.ButtonShadow)
	drawRaised(r, thumb, c.ButtonFace, c)
}

// drawRaisedRounded paints a raised 3D look on a rounded rect (pill/circle
// button chrome): a `face`-filled interior with a light top-left / dark
// bottom-right 1px bevel. StrokeRoundedRect only strokes a single flat
// color, so directional lighting can't come from a stroke — instead it is
// faked by layering three offset FillRoundedRect calls: (1) the full rect in
// ButtonDarkShadow, which ends up exposed only along the bottom+right edge
// once step 2 paints over the rest; (2) the rect shifted (-1,-1) in
// ButtonHighlight, which — because it's offset up-left — covers the
// top+left edge plus the interior but falls 1px short of the bottom+right
// edge, letting step 1's dark shadow show through there; (3) the rect inset
// by 1px (radius shrunk by 1, floored at 0 so a small radius never goes
// negative) filled with `face`, painting over everything but the 1px
// directional rim left by steps 1-2. Net result: a light top-left / dark
// bottom-right 1px ring around a flat `face` interior, matching the classic
// square drawRaised's corner directionality on a rounded shape.
func drawRaisedRounded(r render.Renderer, rect render.Rect, radius float32, face render.Color, c theme.ColorTokens) {
	r.FillRoundedRect(rect, radius, c.ButtonDarkShadow)
	r.FillRoundedRect(render.Rect{X: rect.X - 1, Y: rect.Y - 1, W: rect.W, H: rect.H}, radius, c.ButtonHighlight)

	innerRadius := radius - 1
	if innerRadius < 0 {
		innerRadius = 0
	}
	r.FillRoundedRect(rect.Inset(render.Uniform(1)), innerRadius, face)
}

// drawSunkenRounded paints the inverted rounded bevel — a recessed pill/
// circle, for a pressed or checked-toggle button — using the same layered
// offset-fill technique as drawRaisedRounded so it reads as pushed in rather
// than raised: dark top-left / light bottom-right (the same corner
// directionality as the classic square drawSunken). Structurally this is
// drawRaisedRounded with the offset layer's sign flipped from (-1,-1) to
// (+1,+1) — shifting the ButtonHighlight layer down-right instead of
// up-left mirrors which edge each color is exposed along, inverting the
// bevel's read without needing to also swap which color each step uses
// (swapping both the offset AND the two colors would cancel back out to the
// raised look, which is why only the offset flips here).
func drawSunkenRounded(r render.Renderer, rect render.Rect, radius float32, fill render.Color, c theme.ColorTokens) {
	r.FillRoundedRect(rect, radius, c.ButtonDarkShadow)
	r.FillRoundedRect(render.Rect{X: rect.X + 1, Y: rect.Y + 1, W: rect.W, H: rect.H}, radius, c.ButtonHighlight)

	innerRadius := radius - 1
	if innerRadius < 0 {
		innerRadius = 0
	}
	r.FillRoundedRect(rect.Inset(render.Uniform(1)), innerRadius, fill)
}

// drawFocusRect paints the classic 1px inset focus rectangle (Highlight
// color), one pixel inside rect, as four 1px-thick edge FillRects forming
// the border of the inset rectangle. A solid line; the classic dotted XOR
// pattern is deferred to a later version.
func drawFocusRect(r render.Renderer, rect render.Rect, c theme.ColorTokens) {
	x, y, w, h := rect.X+1, rect.Y+1, rect.W-2, rect.H-2
	r.FillRect(render.Rect{X: x, Y: y, W: w, H: 1}, c.Highlight)
	r.FillRect(render.Rect{X: x, Y: y + h - 1, W: w, H: 1}, c.Highlight)
	r.FillRect(render.Rect{X: x, Y: y, W: 1, H: h}, c.Highlight)
	r.FillRect(render.Rect{X: x + w - 1, Y: y, W: 1, H: h}, c.Highlight)
}
