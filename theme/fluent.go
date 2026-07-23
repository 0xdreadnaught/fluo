package theme

import "github.com/0xdreadnaught/fluo/render"

// Shared metrics across both light and dark variants.
var sharedMetrics = MetricTokens{
	CornerRadius:        8,
	ControlCornerRadius: 4,
	StrokeWidth:         1,
	FocusStrokeWidth:    2,
	PaddingS:            4,
	PaddingM:            8,
	PaddingL:            16,
	ScrollGutter:        12,
	ShadowBlur:          16,
}

// Shared type ramp across both light and dark variants.
var sharedType = TypeTokens{
	CaptionSize:  12,
	BodySize:     14,
	SubtitleSize: 20,
	TitleSize:    28,
}

// FluentLight returns a fresh Theme with light Fluent colors.
func FluentLight() *Theme {
	return &Theme{
		Name: "fluent-light",
		Color: ColorTokens{
			WindowBackground:      render.RGB(243, 243, 243),
			LayerBackground:       render.RGB(251, 251, 253),
			CardBackground:        render.RGB(255, 255, 255),
			TextPrimary:           render.RGB(26, 26, 26),
			TextSecondary:         render.RGBA(0, 0, 0, 150),
			TextDisabled:          render.RGBA(0, 0, 0, 90),
			Accent:                render.RGB(0, 103, 192),
			AccentHover:           render.RGB(25, 118, 199),
			AccentPressed:         render.RGB(0, 86, 163),
			AccentText:            render.RGB(255, 255, 255),
			ControlFill:           render.RGBA(0, 0, 0, 10),
			ControlFillHover:      render.RGBA(0, 0, 0, 18),
			ControlFillPressed:    render.RGBA(0, 0, 0, 6),
			ControlStroke:         render.RGBA(0, 0, 0, 30),
			FocusStroke:           render.RGB(0, 103, 192),
			ScrollThumb:           render.RGBA(0, 0, 0, 70),
			SelectionBackground:   render.RGBA(0, 103, 192, 70),
			SelectionText:         render.RGB(26, 26, 26),
			ControlFillDisabled:   render.RGBA(0, 0, 0, 5),
			ControlStrokeDisabled: render.RGBA(0, 0, 0, 14),
			AccentDisabled:        render.RGBA(0, 0, 0, 55),
			Shadow:                render.RGBA(0, 0, 0, 50),
			SelectionForeground:   render.RGB(26, 26, 26),
			ScrimBackground:       render.RGBA(0, 0, 0, 90),
		},
		Metric: sharedMetrics,
		Type:   sharedType,
	}
}

// FluentDark returns a fresh Theme with dark Fluent colors.
func FluentDark() *Theme {
	return &Theme{
		Name: "fluent-dark",
		Color: ColorTokens{
			WindowBackground:      render.RGB(32, 32, 36),
			LayerBackground:       render.RGB(40, 40, 46),
			CardBackground:        render.RGB(44, 44, 50),
			TextPrimary:           render.RGB(255, 255, 255),
			TextSecondary:         render.RGBA(255, 255, 255, 160),
			TextDisabled:          render.RGBA(255, 255, 255, 90),
			Accent:                render.RGB(0, 120, 215),
			AccentHover:           render.RGB(16, 132, 226),
			AccentPressed:         render.RGB(0, 100, 180),
			AccentText:            render.RGB(255, 255, 255),
			ControlFill:           render.RGBA(255, 255, 255, 16),
			ControlFillHover:      render.RGBA(255, 255, 255, 28),
			ControlFillPressed:    render.RGBA(255, 255, 255, 10),
			ControlStroke:         render.RGBA(255, 255, 255, 24),
			FocusStroke:           render.RGB(0, 120, 215),
			ScrollThumb:           render.RGBA(255, 255, 255, 60),
			SelectionBackground:   render.RGBA(0, 120, 215, 90),
			SelectionText:         render.RGB(255, 255, 255),
			ControlFillDisabled:   render.RGBA(255, 255, 255, 8),
			ControlStrokeDisabled: render.RGBA(255, 255, 255, 12),
			AccentDisabled:        render.RGBA(255, 255, 255, 40),
			Shadow:                render.RGBA(0, 0, 0, 120),
			SelectionForeground:   render.RGB(255, 255, 255),
			ScrimBackground:       render.RGBA(0, 0, 0, 120),
		},
		Metric: sharedMetrics,
		Type:   sharedType,
	}
}
