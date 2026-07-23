// Package theme holds fluo's Fluent/WinUI-inspired design tokens. A Theme
// bundles ColorTokens (semantic colors — backgrounds, text, accent, control
// fill/stroke, selection, shadow, and states like hover/pressed/disabled),
// MetricTokens (corner radii, stroke widths, the 4/8/16 padding scale, and
// scroll gutter/shadow blur), and TypeTokens (caption/body/subtitle/title
// pixel sizes). FluentLight and FluentDark are the two built-in variants;
// Active returns the process-wide current theme (defaulting to
// FluentDark), and SetActive changes it. Every controls widget captures the
// tokens it needs from theme.Active() at construction time rather than
// live-reskinning, so re-theming an existing app means calling SetActive
// and rebuilding the widget tree from scratch — see cmd/fluo-gallery's
// T-key toggle for the reference pattern.
package theme
