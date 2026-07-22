package theme

import "github.com/0xdreadnaught/fluo/render"

type ColorTokens struct {
	WindowBackground, LayerBackground, CardBackground render.Color
	TextPrimary, TextSecondary, TextDisabled          render.Color
	Accent, AccentHover, AccentPressed, AccentText    render.Color
	ControlFill, ControlFillHover, ControlFillPressed render.Color
	ControlStroke, FocusStroke                        render.Color
	ScrollThumb                                       render.Color
	Shadow                                            render.Color
}

type MetricTokens struct {
	CornerRadius, ControlCornerRadius float32 // cards vs buttons/inputs
	StrokeWidth, FocusStrokeWidth     float32
	PaddingS, PaddingM, PaddingL      float32 // 4, 8, 16 scale
	ScrollGutter                      float32
	ShadowBlur                        float32
}

type TypeTokens struct {
	CaptionSize, BodySize, SubtitleSize, TitleSize float32 // px
}

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
