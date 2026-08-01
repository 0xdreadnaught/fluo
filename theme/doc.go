// Package theme holds fluo's classic Windows-2000-inspired design tokens. A
// Theme bundles ColorTokens (the four-tone bevel palette — ButtonFace/
// ButtonHighlight/ButtonLight/ButtonShadow/ButtonDarkShadow — plus
// WindowWell/WindowText/GrayText, Highlight/HighlightText selection, and the
// CaptionFrom/CaptionTo title bar gradient, the four SeverityInfo/Success/
// Warning/Error accents a Toast classifies itself by, and the AcrylicTint
// composited over a backdrop blur), MetricTokens (corner radii
// pinned to 0, BevelWidth, stroke widths, the 4/8/16 padding scale, and
// scroll gutter/shadow blur), and TypeTokens (caption/body/subtitle/title
// pixel sizes). Light and Dark are the two built-in variants; Active
// returns the process-wide current theme (defaulting to Light), and
// SetActive changes it. Every controls widget captures the
// tokens it needs from theme.Active() at construction time rather than
// live-reskinning, so re-theming an existing app means calling SetActive
// and rebuilding the widget tree from scratch — see cmd/fluo-gallery's
// T-key toggle for the reference pattern.
package theme
