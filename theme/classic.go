package theme

import "github.com/0xdreadnaught/fluo/render"

// Shared metrics across both light and dark variants. Both classic themes
// are square (zero corner radii) and share the same 2px bevel width.
var sharedMetrics = MetricTokens{
	CornerRadius:        0,
	ControlCornerRadius: 0,
	BevelWidth:          2,
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

// Light returns a fresh Theme with the classic light (Windows-2000
// "Standard") four-tone bevel palette.
func Light() *Theme {
	return &Theme{
		Name: "classic-light",
		Color: ColorTokens{
			// Classic tokens.
			ButtonFace:       render.RGB(212, 208, 200),
			ButtonHighlight:  render.RGB(255, 255, 255),
			ButtonLight:      render.RGB(232, 228, 220),
			ButtonShadow:     render.RGB(128, 128, 128),
			ButtonDarkShadow: render.RGB(64, 64, 64),
			WindowWell:       render.RGB(255, 255, 255),
			WindowText:       render.RGB(0, 0, 0),
			GrayText:         render.RGB(128, 128, 128),
			Highlight:        render.RGB(0, 0, 128),
			HighlightText:    render.RGB(255, 255, 255),
			CaptionFrom:      render.RGB(10, 36, 106),
			CaptionTo:        render.RGB(166, 202, 240),
			CaptionText:      render.RGB(255, 255, 255),
			InactiveCaption:  render.RGB(128, 128, 128),

			// Severity accents: muted enough to sit next to the classic
			// bevel palette, but success/error stay unmistakably apart at
			// a glance.
			SeverityInfo:    render.RGB(0, 90, 158),
			SeveritySuccess: render.RGB(0, 128, 0),
			SeverityWarning: render.RGB(196, 130, 0),
			SeverityError:   render.RGB(178, 0, 0),

			// Deprecated pre-v0.2 fields, mapped onto the classic palette
			// so unmigrated controls still render in classic colors.
			WindowBackground:      render.RGB(212, 208, 200),
			LayerBackground:       render.RGB(212, 208, 200),
			CardBackground:        render.RGB(212, 208, 200),
			TextPrimary:           render.RGB(0, 0, 0),
			TextSecondary:         render.RGB(128, 128, 128),
			TextDisabled:          render.RGB(128, 128, 128),
			Accent:                render.RGB(0, 0, 128),
			AccentHover:           render.RGB(0, 0, 168),
			AccentPressed:         render.RGB(0, 0, 96),
			AccentText:            render.RGB(255, 255, 255),
			ControlFill:           render.RGB(212, 208, 200),
			ControlFillHover:      render.RGB(232, 228, 220),
			ControlFillPressed:    render.RGB(212, 208, 200),
			ControlStroke:         render.RGB(128, 128, 128),
			SelectionBackground:   render.RGB(0, 0, 128),
			ControlFillDisabled:   render.RGB(212, 208, 200),
			ControlStrokeDisabled: render.RGB(128, 128, 128),
			AccentDisabled:        render.RGB(128, 128, 128),
			SelectionForeground:   render.RGB(255, 255, 255),
			ScrimBackground:       render.RGBA(0, 0, 0, 90),
			AcrylicTint:           render.RGBA(212, 208, 200, 180),
		},
		Metric: sharedMetrics,
		Type:   sharedType,
	}
}

// Dark returns a fresh Theme with the classic dark four-tone bevel palette.
func Dark() *Theme {
	return &Theme{
		Name: "classic-dark",
		Color: ColorTokens{
			// Classic tokens.
			ButtonFace:       render.RGB(58, 58, 58),
			ButtonHighlight:  render.RGB(92, 92, 92),
			ButtonLight:      render.RGB(70, 70, 70),
			ButtonShadow:     render.RGB(32, 32, 32),
			ButtonDarkShadow: render.RGB(0, 0, 0),
			WindowWell:       render.RGB(30, 30, 30),
			WindowText:       render.RGB(240, 240, 240),
			GrayText:         render.RGB(110, 110, 110),
			Highlight:        render.RGB(42, 77, 143),
			HighlightText:    render.RGB(255, 255, 255),
			CaptionFrom:      render.RGB(16, 33, 74),
			CaptionTo:        render.RGB(58, 110, 165),
			CaptionText:      render.RGB(255, 255, 255),
			InactiveCaption:  render.RGB(42, 42, 42),

			// Severity accents: brighter than the light variant so each
			// still reads clearly against the darker button face.
			SeverityInfo:    render.RGB(70, 130, 190),
			SeveritySuccess: render.RGB(60, 170, 60),
			SeverityWarning: render.RGB(214, 158, 0),
			SeverityError:   render.RGB(214, 60, 60),

			// Deprecated pre-v0.2 fields, mapped onto the classic palette
			// so unmigrated controls still render in classic colors.
			WindowBackground:      render.RGB(58, 58, 58),
			LayerBackground:       render.RGB(58, 58, 58),
			CardBackground:        render.RGB(58, 58, 58),
			TextPrimary:           render.RGB(240, 240, 240),
			TextSecondary:         render.RGB(110, 110, 110),
			TextDisabled:          render.RGB(110, 110, 110),
			Accent:                render.RGB(42, 77, 143),
			AccentHover:           render.RGB(66, 103, 173),
			AccentPressed:         render.RGB(26, 56, 110),
			AccentText:            render.RGB(255, 255, 255),
			ControlFill:           render.RGB(58, 58, 58),
			ControlFillHover:      render.RGB(70, 70, 70),
			ControlFillPressed:    render.RGB(58, 58, 58),
			ControlStroke:         render.RGB(32, 32, 32),
			SelectionBackground:   render.RGB(42, 77, 143),
			ControlFillDisabled:   render.RGB(58, 58, 58),
			ControlStrokeDisabled: render.RGB(32, 32, 32),
			AccentDisabled:        render.RGB(110, 110, 110),
			SelectionForeground:   render.RGB(255, 255, 255),
			ScrimBackground:       render.RGBA(0, 0, 0, 120),
			AcrylicTint:           render.RGBA(58, 58, 58, 180),
		},
		Metric: sharedMetrics,
		Type:   sharedType,
	}
}
