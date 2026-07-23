package theme

import "github.com/0xdreadnaught/fluo/render"

// ColorTokens holds the semantic color values for a theme.
//
// The classic fields (ButtonFace .. InactiveCaption) are the Windows-2000
// style four-tone bevel palette introduced in v0.2: controls render their
// raised/sunken chrome from ButtonFace/ButtonHighlight/ButtonLight/
// ButtonShadow/ButtonDarkShadow, plain content areas from WindowWell/
// WindowText/GrayText, selection from Highlight/HighlightText, and title
// bars from the CaptionFrom→CaptionTo gradient (CaptionText/
// InactiveCaption for the caption label and the unfocused-window variant).
//
// The remaining fields are the pre-v0.2 flat token set, kept only until
// each control is migrated to the classic fields above (see the
// "deprecated" comment on each); new code should not read them.
type ColorTokens struct {
	// ButtonFace is the flat fill of buttons, panels, and other 3D chrome.
	ButtonFace render.Color
	// ButtonHighlight is the brightest bevel edge (top/left of a raised
	// control, bottom/right of a sunken one).
	ButtonHighlight render.Color
	// ButtonLight is the secondary lit bevel edge, one step in from
	// ButtonHighlight.
	ButtonLight render.Color
	// ButtonShadow is the secondary dark bevel edge, one step in from
	// ButtonDarkShadow.
	ButtonShadow render.Color
	// ButtonDarkShadow is the darkest bevel edge (bottom/right of a raised
	// control, top/left of a sunken one).
	ButtonDarkShadow render.Color
	// WindowWell is the recessed content background used by text/list/tree
	// controls (the "well" a document or list sits in).
	WindowWell render.Color
	// WindowText is the default text color over WindowWell/ButtonFace.
	WindowText render.Color
	// GrayText is disabled/secondary text.
	GrayText render.Color
	// Highlight is the selection fill (selected list rows, selected text).
	Highlight render.Color
	// HighlightText is the text/foreground color over a Highlight fill.
	HighlightText render.Color
	// CaptionFrom and CaptionTo are the two stops of the active title bar's
	// left-to-right gradient.
	CaptionFrom, CaptionTo render.Color
	// CaptionText is the title bar label color.
	CaptionText render.Color
	// InactiveCaption is the flat fill an unfocused window's title bar uses
	// in place of the CaptionFrom/CaptionTo gradient.
	InactiveCaption render.Color

	// WindowBackground, LayerBackground, and CardBackground are surface
	// fills at increasing elevation.
	//
	// deprecated: migrating to classic tokens; removed after control restyle
	WindowBackground, LayerBackground, CardBackground render.Color
	// TextPrimary, TextSecondary, and TextDisabled are body text at
	// decreasing emphasis.
	//
	// deprecated: migrating to classic tokens; removed after control restyle
	TextPrimary, TextSecondary, TextDisabled render.Color
	// Accent, AccentHover, AccentPressed, and AccentText are the brand
	// accent color and its interaction states.
	//
	// deprecated: migrating to classic tokens; removed after control restyle
	Accent, AccentHover, AccentPressed, AccentText render.Color
	// ControlFill, ControlFillHover, and ControlFillPressed are a control's
	// fill at rest and under pointer interaction.
	//
	// deprecated: migrating to classic tokens; removed after control restyle
	ControlFill, ControlFillHover, ControlFillPressed render.Color
	// ControlStroke is a control's default border; FocusStroke is the
	// keyboard-focus ring color.
	//
	// deprecated: migrating to classic tokens; removed after control restyle
	ControlStroke, FocusStroke render.Color
	// ScrollThumb is a scrollbar/scroll-gutter thumb's fill.
	//
	// deprecated: migrating to classic tokens; removed after control restyle
	ScrollThumb render.Color
	// SelectionBackground and SelectionText for selected text/content regions.
	//
	// deprecated: migrating to classic tokens; removed after control restyle
	SelectionBackground, SelectionText render.Color
	// ControlFillDisabled, ControlStrokeDisabled, and AccentDisabled for disabled control states.
	//
	// deprecated: migrating to classic tokens; removed after control restyle
	ControlFillDisabled, ControlStrokeDisabled, AccentDisabled render.Color
	// Shadow is a drop-shadow tint for elevated surfaces (menus, dialogs).
	//
	// deprecated: migrating to classic tokens; removed after control restyle
	Shadow render.Color
	// SelectionForeground and ScrimBackground for text selection and scrim overlays.
	//
	// deprecated: migrating to classic tokens; removed after control restyle
	SelectionForeground, ScrimBackground render.Color
	// CloseButtonHover is the distinct red a window's close caption button
	// uses for its hover/pressed fill (a common close-button convention), same value in
	// both bundled themes — RGBA(232,17,35,255).
	//
	// deprecated: migrating to classic tokens; removed after control restyle
	CloseButtonHover render.Color
	// AcrylicTint is the translucent tint composited over a backdrop-blur
	// acrylic/mica surface (see controls.AcrylicSurface).
	//
	// deprecated: migrating to classic tokens; removed after control restyle
	AcrylicTint render.Color
}

// MetricTokens holds the layout and sizing values for a theme.
type MetricTokens struct {
	CornerRadius, ControlCornerRadius float32 // square in the classic themes: both 0
	// BevelWidth is the thickness, in pixels, of each step of the classic
	// four-tone 3D bevel (ButtonHighlight/ButtonLight/ButtonShadow/
	// ButtonDarkShadow) drawn around raised/sunken chrome.
	BevelWidth                    float32
	StrokeWidth, FocusStrokeWidth float32
	PaddingS, PaddingM, PaddingL  float32 // 4, 8, 16 scale
	ScrollGutter                  float32
	ShadowBlur                    float32
}

// TypeTokens holds the typography sizes for a theme.
type TypeTokens struct {
	CaptionSize, BodySize, SubtitleSize, TitleSize float32 // px
}

// Theme represents a complete color, metric, and typography design system.
type Theme struct {
	Name   string // "classic-light" | "classic-dark"
	Color  ColorTokens
	Metric MetricTokens
	Type   TypeTokens
}

var active *Theme

// Active returns the currently active theme, never nil (defaults to Light).
// Not safe for concurrent use; fluo v0 assumes a single UI goroutine and one active theme per process.
func Active() *Theme {
	if active == nil {
		active = Light()
	}
	return active
}

// SetActive sets the active theme. If t is nil, resets to the default (Light).
// Not safe for concurrent use; fluo v0 assumes a single UI goroutine and one active theme per process.
func SetActive(t *Theme) {
	if t == nil {
		active = nil
	} else {
		active = t
	}
}
