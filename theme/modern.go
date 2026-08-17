package theme

import "github.com/0xdreadnaught/fluo/render"

var modernMetrics = MetricTokens{
	CornerRadius:        8,
	ControlCornerRadius: 6,
	BevelWidth:          0,
	StrokeWidth:         1,
	FocusStrokeWidth:    2,
	PaddingS:            4,
	PaddingM:            8,
	PaddingL:            16,
	ScrollGutter:        8,
	ShadowBlur:          12,
}

var modernType = TypeTokens{
	CaptionSize:  12,
	BodySize:     14,
	SubtitleSize: 20,
	TitleSize:    28,
}

// ModernDark returns a fresh Theme with a contemporary dark palette
// inspired by GitHub Dark / wails.io. Bevel highlight and shadow colors
// are kept very close to ButtonFace so the existing four-tone bevel
// helpers produce a near-flat appearance; CornerRadius and
// ControlCornerRadius are non-zero for controls that adopt them.
func ModernDark() *Theme {
	return &Theme{
		Name: "modern-dark",
		Color: ColorTokens{
			ButtonFace:       render.RGB(33, 38, 45),
			ButtonHighlight:  render.RGB(45, 51, 59),
			ButtonLight:      render.RGB(45, 51, 59),
			ButtonShadow:     render.RGB(27, 31, 36),
			ButtonDarkShadow: render.RGB(20, 24, 30),
			WindowWell:       render.RGB(13, 17, 23),
			WindowText:       render.RGB(230, 237, 243),
			GrayText:         render.RGB(125, 133, 144),
			Highlight:        render.RGB(47, 129, 247),
			HighlightText:    render.RGB(255, 255, 255),
			CaptionFrom:      render.RGB(27, 35, 50),
			CaptionTo:        render.RGB(31, 50, 81),
			CaptionText:      render.RGB(230, 237, 243),
			InactiveCaption:  render.RGB(22, 27, 34),

			SeverityInfo:    render.RGB(88, 166, 255),
			SeveritySuccess: render.RGB(63, 185, 80),
			SeverityWarning: render.RGB(210, 153, 34),
			SeverityError:   render.RGB(248, 81, 73),

			AcrylicTint: render.RGBA(13, 17, 23, 180),
		},
		Metric: modernMetrics,
		Type:   modernType,
	}
}
