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
	Shadow                                            render.Color
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
func Active() *Theme {
	if active == nil {
		active = FluentDark()
	}
	return active
}

// SetActive sets the active theme. If t is nil, resets to the default (FluentDark).
func SetActive(t *Theme) {
	if t == nil {
		active = nil
	} else {
		active = t
	}
}
