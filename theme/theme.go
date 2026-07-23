package theme

import "github.com/0xdreadnaught/fluo/render"

// ColorTokens holds the semantic color values for a theme.
type ColorTokens struct {
	WindowBackground, LayerBackground, CardBackground render.Color
	TextPrimary, TextSecondary, TextDisabled          render.Color
	Accent, AccentHover, AccentPressed, AccentText    render.Color
	ControlFill, ControlFillHover, ControlFillPressed render.Color
	ControlStroke, FocusStroke                        render.Color
	ScrollThumb                                       render.Color
	// SelectionBackground and SelectionText for selected text/content regions.
	SelectionBackground, SelectionText render.Color
	// ControlFillDisabled, ControlStrokeDisabled, and AccentDisabled for disabled control states.
	ControlFillDisabled, ControlStrokeDisabled, AccentDisabled render.Color
	Shadow                                                     render.Color
	// SelectionForeground and ScrimBackground for text selection and scrim overlays.
	SelectionForeground, ScrimBackground render.Color
}

// MetricTokens holds the layout and sizing values for a theme.
type MetricTokens struct {
	CornerRadius, ControlCornerRadius float32 // cards vs buttons/inputs
	StrokeWidth, FocusStrokeWidth     float32
	PaddingS, PaddingM, PaddingL      float32 // 4, 8, 16 scale
	ScrollGutter                      float32
	ShadowBlur                        float32
}

// TypeTokens holds the typography sizes for a theme.
type TypeTokens struct {
	CaptionSize, BodySize, SubtitleSize, TitleSize float32 // px
}

// Theme represents a complete color, metric, and typography design system.
type Theme struct {
	Name   string // "fluent-light" | "fluent-dark"
	Color  ColorTokens
	Metric MetricTokens
	Type   TypeTokens
}

var active *Theme

// Active returns the currently active theme, never nil (defaults to FluentDark).
// Not safe for concurrent use; fluo v0 assumes a single UI goroutine and one active theme per process.
func Active() *Theme {
	if active == nil {
		active = FluentDark()
	}
	return active
}

// SetActive sets the active theme. If t is nil, resets to the default (FluentDark).
// Not safe for concurrent use; fluo v0 assumes a single UI goroutine and one active theme per process.
func SetActive(t *Theme) {
	if t == nil {
		active = nil
	} else {
		active = t
	}
}
